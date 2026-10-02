package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// GenreLink is one of an album's genres and where it came from.
type GenreLink struct {
	Genre  string `json:"genre"`
	Style  string `json:"style,omitempty"`
	Source string `json:"source"` // tag | mb | discogs | manual
}

// TrackIdentity is the MusicBrainz track a catalog track was paired with.
type TrackIdentity struct {
	FileID      int64
	Title       string
	RecordingID string
}

// AlbumIdentity is what the identification decided for an album.
type AlbumIdentity struct {
	AlbumID      int64
	Query        string // the album's identify query key
	Status       string
	ReleaseID    string
	Confidence   float64
	Title        string // MusicBrainz's; "" keeps the grouped one
	Artist       string
	Compilation  bool // MusicBrainz credits Various Artists
	OriginalYear int
	Country      string
	ArtistID     string
	CoverCache   string
	Genres       []GenreLink
	Tracks       []TrackIdentity
}

// ApplyIdentities writes the identity of every album, and the raw genre
// values nothing maps with how many albums use each. ids must cover all
// albums: track titles and genres not in ids are cleared.
// The identities are computed outside this transaction, so a scan may have
// regrouped meanwhile: rows for albums that are gone, and tracks that moved to
// another album, are skipped.
func (s *Store) ApplyIdentities(ids []AlbumIdentity, unmapped map[string]int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{`UPDATE tracks SET mb_title = '', recording_id = ''`, `DELETE FROM album_genres`, `DELETE FROM unmapped_genres`} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	al, err := tx.Prepare(`UPDATE albums SET match_query = ?, status = ?, release_id = ?, confidence = ?, mb_title = ?,
		mb_artist = ?, mb_compilation = ?, original_year = ?, country = ?, artist_id = ?, cover_cache = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer al.Close()
	gi, err := tx.Prepare(`INSERT OR IGNORE INTO album_genres (album_id, genre, style, source)
		SELECT ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM albums WHERE id = ?)`)
	if err != nil {
		return err
	}
	defer gi.Close()
	ti, err := tx.Prepare(`UPDATE tracks SET mb_title = ?, recording_id = ? WHERE file_id = ? AND album_id = ?`)
	if err != nil {
		return err
	}
	defer ti.Close()
	for _, id := range ids {
		if _, err := al.Exec(id.Query, id.Status, id.ReleaseID, id.Confidence, id.Title, id.Artist, id.Compilation,
			id.OriginalYear, id.Country, id.ArtistID, id.CoverCache, id.AlbumID); err != nil {
			return err
		}
		for _, g := range id.Genres {
			if _, err := gi.Exec(id.AlbumID, g.Genre, g.Style, g.Source, id.AlbumID); err != nil {
				return err
			}
		}
		for _, t := range id.Tracks {
			if _, err := ti.Exec(t.Title, t.RecordingID, t.FileID, id.AlbumID); err != nil {
				return err
			}
		}
	}
	for raw, n := range unmapped {
		if _, err := tx.Exec(`INSERT INTO unmapped_genres (raw, albums) VALUES (?, ?)`, raw, n); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const identified = `status IN ('auto', 'manual') AND release_id != ''`

// ReleaseIDsInUse lists the releases of identified albums.
func (s *Store) ReleaseIDsInUse() ([]string, error) {
	return s.column(`SELECT DISTINCT release_id FROM albums WHERE ` + identified + ` ORDER BY release_id`)
}

// CoverlessReleases lists the releases of identified albums that have
// neither a folder image nor an embedded picture.
func (s *Store) CoverlessReleases() ([]string, error) {
	return s.column(`SELECT DISTINCT release_id FROM albums WHERE ` + identified +
		` AND cover_path = '' AND cover_file_id = 0 ORDER BY release_id`)
}

func (s *Store) column(q string, args ...any) ([]string, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UnidentifiedView is an album of the Sin identificar queue.
type UnidentifiedView struct {
	AlbumView
	Candidates []Candidate `json:"candidates"`
}

// Unidentified lists the albums below the threshold with their candidates.
func (s *Store) Unidentified() ([]UnidentifiedView, error) {
	as, err := s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE status = ?
		ORDER BY artist COLLATE NOCASE, title COLLATE NOCASE`, StatusUnmatched)
	if err != nil {
		return nil, err
	}
	ms, err := s.Matches()
	if err != nil {
		return nil, err
	}
	out := make([]UnidentifiedView, 0, len(as))
	for _, a := range as {
		c := ms[a.MatchQuery].Candidates
		if c == nil {
			c = []Candidate{}
		}
		out = append(out, UnidentifiedView{AlbumView: a, Candidates: c})
	}
	return out, nil
}

// AlbumCandidates returns the MusicBrainz releases offered for an album when
// it was matched, best first (none when it was not looked up).
func (s *Store) AlbumCandidates(albumID int64) ([]Candidate, error) {
	var raw string
	err := s.db.QueryRow(`SELECT m.candidates FROM albums a JOIN matches m ON m.query = a.match_query WHERE a.id = ?`, albumID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return []Candidate{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Candidate{}
	return out, json.Unmarshal([]byte(raw), &out)
}

// GenreAlias is the user's mapping of a raw genre value.
type GenreAlias struct {
	Raw   string `json:"raw"`   // genres.Key form
	Genre string `json:"genre"` // "" = not a genre: ignore it
	Style string `json:"style"`
}

func (s *Store) SetGenreAlias(a GenreAlias) error {
	_, err := s.db.Exec(`INSERT INTO genre_alias (raw, genre, style) VALUES (?, ?, ?)
		ON CONFLICT(raw) DO UPDATE SET genre = excluded.genre, style = excluded.style`, a.Raw, a.Genre, a.Style)
	return err
}

// DeleteGenreAlias forgets a mapping: the raw value maps as built in, or
// goes back to the Géneros por mapear queue.
func (s *Store) DeleteGenreAlias(raw string) error {
	_, err := s.db.Exec(`DELETE FROM genre_alias WHERE raw = ?`, raw)
	return err
}

func (s *Store) GenreAliases() ([]GenreAlias, error) {
	rows, err := s.db.Query(`SELECT raw, genre, style FROM genre_alias ORDER BY raw`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GenreAlias{}
	for rows.Next() {
		var a GenreAlias
		if err := rows.Scan(&a.Raw, &a.Genre, &a.Style); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UnmappedGenre is an entry of the Géneros por mapear queue.
type UnmappedGenre struct {
	Raw    string `json:"raw"`
	Albums int    `json:"albums"`
}

// UnmappedGenres lists the queue, most used first.
func (s *Store) UnmappedGenres() ([]UnmappedGenre, error) {
	rows, err := s.db.Query(`SELECT raw, albums FROM unmapped_genres ORDER BY albums DESC, raw`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UnmappedGenre{}
	for rows.Next() {
		var u UnmappedGenre
		if err := rows.Scan(&u.Raw, &u.Albums); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
