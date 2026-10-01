package store

import (
	"encoding/json"
	"time"

	"tordo/internal/audiofile"
)

// Album identification statuses.
const (
	StatusPending   = ""          // not looked up yet
	StatusAuto      = "auto"      // matched automatically, above the threshold
	StatusManual    = "manual"    // chosen by the user
	StatusUnmatched = "unmatched" // below the threshold: the Sin identificar queue
	StatusIgnored   = "ignored"   // the user says it is not in MusicBrainz
	StatusSelection = "selection" // a folder selection MusicBrainz does not know
)

// TargetTrack is a track of an album to identify.
type TargetTrack struct {
	FileID      int64
	Fingerprint string
	Title       string
	Disc, Track int
	DurationMs  int64
	MBAlbumID   string // from the tags
}

// IdentifyTarget is an album as the identification sees it.
type IdentifyTarget struct {
	AlbumID     int64
	Dir         string
	Title       string // as grouped: from tags or the folder name
	Artist      string
	Year        int
	Compilation bool
	Selection   bool
	HasCover    bool // a folder image or an embedded picture
	RawGenres   []string
	Tracks      []TargetTrack // in disc/track order
}

// IdentifyTargets returns every album with its tracks.
func (s *Store) IdentifyTargets() ([]IdentifyTarget, error) {
	rows, err := s.db.Query(`SELECT id, dir, title, artist, year, compilation, selection,
		cover_path != '' OR cover_file_id != 0, genres FROM albums ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []IdentifyTarget
	index := map[int64]int{}
	for rows.Next() {
		var t IdentifyTarget
		var genres string
		if err := rows.Scan(&t.AlbumID, &t.Dir, &t.Title, &t.Artist, &t.Year, &t.Compilation, &t.Selection, &t.HasCover, &genres); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(genres), &t.RawGenres); err != nil {
			rows.Close()
			return nil, err
		}
		index[t.AlbumID] = len(out)
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.db.Query(`SELECT t.album_id, t.file_id, f.fingerprint, t.title, t.disc, t.track, t.duration_ms, a.tags
		FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
		WHERE t.album_id IS NOT NULL ORDER BY t.album_id, t.disc, t.track, f.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var albumID int64
		var tr TargetTrack
		var tags string
		if err := rows.Scan(&albumID, &tr.FileID, &tr.Fingerprint, &tr.Title, &tr.Disc, &tr.Track, &tr.DurationMs, &tags); err != nil {
			return nil, err
		}
		var tg audiofile.Tags
		if err := json.Unmarshal([]byte(tags), &tg); err != nil {
			return nil, err
		}
		tr.MBAlbumID = tg.MBAlbumID
		if i, ok := index[albumID]; ok {
			out[i].Tracks = append(out[i].Tracks, tr)
		}
	}
	return out, rows.Err()
}

// AlbumFingerprints returns the fingerprints of an album's tracks (none for
// an unknown album).
func (s *Store) AlbumFingerprints(albumID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT f.fingerprint FROM tracks t JOIN files f ON f.id = t.file_id
		WHERE t.album_id = ? AND f.fingerprint != '' ORDER BY f.path`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, err
		}
		out = append(out, fp)
	}
	return out, rows.Err()
}

// Candidate is a MusicBrainz release offered for an album.
type Candidate struct {
	ReleaseID string  `json:"releaseId"`
	Title     string  `json:"title"`
	Artist    string  `json:"artist"`
	Year      int     `json:"year"`
	Country   string  `json:"country"`
	Tracks    int     `json:"tracks"`
	Score     float64 `json:"score"`
}

// Match is the result of matching one album query.
type Match struct {
	Query          string
	Status         string // StatusAuto or StatusUnmatched
	ReleaseID      string // set when StatusAuto
	Confidence     float64
	Candidates     []Candidate // best first
	MatcherVersion int
}

func (s *Store) SaveMatches(ms []Match) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(`INSERT INTO matches (query, status, release_id, confidence, candidates, matcher_version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(query) DO UPDATE SET status = excluded.status,
		release_id = excluded.release_id, confidence = excluded.confidence, candidates = excluded.candidates,
		matcher_version = excluded.matcher_version, updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer st.Close()
	now := time.Now().UnixMilli()
	for _, m := range ms {
		cands := m.Candidates
		if cands == nil {
			cands = []Candidate{}
		}
		b, err := json.Marshal(cands)
		if err != nil {
			return err
		}
		if _, err := st.Exec(m.Query, m.Status, m.ReleaseID, m.Confidence, string(b), m.MatcherVersion, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Matches returns every stored match by query.
func (s *Store) Matches() (map[string]Match, error) {
	rows, err := s.db.Query(`SELECT query, status, release_id, confidence, candidates, matcher_version FROM matches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Match{}
	for rows.Next() {
		var m Match
		var cands string
		if err := rows.Scan(&m.Query, &m.Status, &m.ReleaseID, &m.Confidence, &cands, &m.MatcherVersion); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(cands), &m.Candidates); err != nil {
			return nil, err
		}
		out[m.Query] = m
	}
	return out, rows.Err()
}

// Correction is the user's identification of a track's album.
type Correction struct {
	Status    string // StatusManual or StatusIgnored
	ReleaseID string // set when StatusManual
}

// Corrections returns every correction by fingerprint.
func (s *Store) Corrections() (map[string]Correction, error) {
	rows, err := s.db.Query(`SELECT fingerprint, status, release_id FROM corrections`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Correction{}
	for rows.Next() {
		var fp string
		var c Correction
		if err := rows.Scan(&fp, &c.Status, &c.ReleaseID); err != nil {
			return nil, err
		}
		out[fp] = c
	}
	return out, rows.Err()
}

// SetCorrection stores c for every fingerprint; an empty c.Status removes
// their corrections (back to the automatic identification).
func (s *Store) SetCorrection(fps []string, c Correction) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	for _, fp := range fps {
		if fp == "" {
			continue
		}
		if c.Status == "" {
			_, err = tx.Exec(`DELETE FROM corrections WHERE fingerprint = ?`, fp)
		} else {
			_, err = tx.Exec(`INSERT INTO corrections (fingerprint, status, release_id, updated_at) VALUES (?, ?, ?, ?)
				ON CONFLICT(fingerprint) DO UPDATE SET status = excluded.status, release_id = excluded.release_id,
				updated_at = excluded.updated_at`, fp, c.Status, c.ReleaseID, now)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
