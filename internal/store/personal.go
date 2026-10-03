package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Kinds of marks.
const (
	KindTrack      = "track"
	KindAlbum      = "album"
	KindArtist     = "artist"
	KindCollection = "collection"
)

// now is the clock of the user's data; tests replace it.
var now = time.Now

// ErrBadRating is returned for a rating outside 0 to 5.
var ErrBadRating = errors.New("el puntaje va de 0 a 5")

// Mark is a liked or rated track or album.
type Mark struct {
	Liked   bool  `json:"liked,omitempty"`
	LikedAt int64 `json:"likedAt,omitempty"` // unix ms
	Rating  int   `json:"rating,omitempty"`  // 1–5, 0 none
}

// Plays sums up a track's plays.
type Plays struct {
	Count int   `json:"count"`
	Last  int64 `json:"last"` // unix ms
}

// Personal is the user's data resolved to the catalog's current ids: only
// present tracks, their albums, and folders that exist.
type Personal struct {
	Tracks      map[int64]Mark   // marked tracks by file id
	Albums      map[int64]Mark   // marked albums by id
	Artists     map[string]int64 // liked artists: key → liked at
	Collections map[string]int64 // liked folders: path → liked at
	Plays       map[int64]Plays  // played tracks by file id
}

// setLike writes the like of one mark, keeping the first time it was liked.
func setLike(tx *sql.Tx, kind, ref string, liked bool) error {
	ms := now().UnixMilli()
	l := 0
	if liked {
		l = 1
	}
	if _, err := tx.Exec(`INSERT INTO marks (kind, ref, liked, liked_at, rating, updated_at) VALUES (?1, ?2, ?3, ?4, 0, ?5)
		ON CONFLICT(kind, ref) DO UPDATE SET
			liked = excluded.liked,
			liked_at = CASE WHEN excluded.liked = 0 THEN 0 WHEN marks.liked = 1 THEN marks.liked_at ELSE excluded.liked_at END,
			updated_at = excluded.updated_at`, kind, ref, l, boolMs(liked, ms), ms); err != nil {
		return err
	}
	return dropEmpty(tx, kind, ref)
}

func boolMs(liked bool, ms int64) int64 {
	if liked {
		return ms
	}
	return 0
}

// setRating writes the rating of one mark.
func setRating(tx *sql.Tx, kind, ref string, stars int) error {
	if _, err := tx.Exec(`INSERT INTO marks (kind, ref, rating, updated_at) VALUES (?1, ?2, ?3, ?4)
		ON CONFLICT(kind, ref) DO UPDATE SET rating = excluded.rating, updated_at = excluded.updated_at`,
		kind, ref, stars, now().UnixMilli()); err != nil {
		return err
	}
	return dropEmpty(tx, kind, ref)
}

// dropEmpty deletes a mark that is neither liked nor rated.
func dropEmpty(tx *sql.Tx, kind, ref string) error {
	_, err := tx.Exec(`DELETE FROM marks WHERE kind = ? AND ref = ? AND liked = 0 AND rating = 0`, kind, ref)
	return err
}

// trackRefOf is the ref of a present track; ok is false when there is none.
func trackRefOf(tx *sql.Tx, id int64) (ref string, ok bool, err error) {
	err = tx.QueryRow(`SELECT `+trackRef+` FROM files f JOIN tracks t ON t.file_id = f.id
		WHERE f.id = ? AND f.missing = 0`, id).Scan(&ref)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return ref, err == nil, err
}

// albumRefs are the refs of an album's present tracks.
func albumRefs(tx *sql.Tx, id int64) ([]string, error) {
	rows, err := tx.Query(`SELECT `+trackRef+` FROM tracks t JOIN files f ON f.id = t.file_id
		WHERE t.album_id = ? AND f.missing = 0`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

func (s *Store) markTrack(id int64, write func(tx *sql.Tx, ref string) error) (bool, error) {
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		ref, ok, err := trackRefOf(tx, id)
		if err != nil || !ok {
			return err
		}
		found = true
		return write(tx, ref)
	})
	return found && err == nil, err
}

func (s *Store) markAlbum(id int64, write func(tx *sql.Tx, ref string) error) (bool, error) {
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		refs, err := albumRefs(tx, id)
		if err != nil || len(refs) == 0 {
			return err
		}
		found = true
		for _, r := range refs {
			if err := write(tx, r); err != nil {
				return err
			}
		}
		return nil
	})
	return found && err == nil, err
}

// LikeTrack likes or unlikes a track; false when there is no such present track.
func (s *Store) LikeTrack(id int64, liked bool) (bool, error) {
	return s.markTrack(id, func(tx *sql.Tx, ref string) error { return setLike(tx, KindTrack, ref, liked) })
}

// RateTrack rates a track from 1 to 5; 0 clears the rating.
func (s *Store) RateTrack(id int64, stars int) (bool, error) {
	if stars < 0 || stars > 5 {
		return false, ErrBadRating
	}
	return s.markTrack(id, func(tx *sql.Tx, ref string) error { return setRating(tx, KindTrack, ref, stars) })
}

// LikeAlbum likes or unlikes an album (on each of its present tracks); false
// when it has none.
func (s *Store) LikeAlbum(id int64, liked bool) (bool, error) {
	return s.markAlbum(id, func(tx *sql.Tx, ref string) error { return setLike(tx, KindAlbum, ref, liked) })
}

// RateAlbum rates an album from 1 to 5; 0 clears the rating.
func (s *Store) RateAlbum(id int64, stars int) (bool, error) {
	if stars < 0 || stars > 5 {
		return false, ErrBadRating
	}
	return s.markAlbum(id, func(tx *sql.Tx, ref string) error { return setRating(tx, KindAlbum, ref, stars) })
}

// LikeArtist likes or unlikes an artist by its catalog key.
func (s *Store) LikeArtist(key string, liked bool) error {
	if key == "" {
		return errors.New("falta el artista")
	}
	return s.sideWrite(true, func(tx *sql.Tx) error { return setLike(tx, KindArtist, key, liked) })
}

// LikeCollection likes or unlikes a folder; false when it does not exist.
func (s *Store) LikeCollection(path string, liked bool) (bool, error) {
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM folders WHERE path = ?`, path).Scan(&n); err != nil || n == 0 {
			return err
		}
		found = true
		return setLike(tx, KindCollection, path, liked)
	})
	return found && err == nil, err
}

// RecordPlay stores a play of a present track; false when there is none.
func (s *Store) RecordPlay(id int64, at time.Time, listenedMs int64, device string) (bool, error) {
	device = cutBytes(strings.TrimSpace(device), 64)
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		ref, ok, err := trackRefOf(tx, id)
		if err != nil || !ok {
			return err
		}
		found = true
		_, err = tx.Exec(`INSERT INTO plays (ref, file_id, played_at, listened_ms, device) VALUES (?, ?, ?, ?, ?)`,
			ref, id, at.UnixMilli(), listenedMs, device)
		return err
	})
	return found && err == nil, err
}

// cutBytes cuts s to at most n bytes at a rune boundary.
func cutBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// Personal reads the user's data resolved to the catalog's current ids.
func (s *Store) Personal() (Personal, error) {
	p := Personal{
		Tracks:      map[int64]Mark{},
		Albums:      map[int64]Mark{},
		Artists:     map[string]int64{},
		Collections: map[string]int64{},
		Plays:       map[int64]Plays{},
	}
	tx, err := s.db.Begin()
	if err != nil {
		return p, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT t.file_id, m.liked, m.liked_at, m.rating
		FROM tracks t JOIN files f ON f.id = t.file_id
		JOIN marks m ON m.kind = 'track' AND m.ref = (` + trackRef + `)
		WHERE f.missing = 0`)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var id, at int64
		var m Mark
		var liked int
		if err := rows.Scan(&id, &liked, &at, &m.Rating); err != nil {
			rows.Close()
			return p, err
		}
		m.Liked, m.LikedAt = liked != 0, at
		p.Tracks[id] = m
	}
	if err := closeRows(rows); err != nil {
		return p, err
	}

	// An album is liked or rated only when most of its present tracks carry
	// the mark: a track shared with another album (same audio) must not mark it.
	rows, err = tx.Query(`SELECT t.album_id, COUNT(*),
			SUM(CASE WHEN m.liked = 1 THEN 1 ELSE 0 END), MAX(CASE WHEN m.liked = 1 THEN m.liked_at ELSE 0 END),
			SUM(CASE WHEN m.rating > 0 THEN 1 ELSE 0 END), MAX(COALESCE(m.rating, 0))
		FROM tracks t JOIN files f ON f.id = t.file_id
		LEFT JOIN marks m ON m.kind = 'album' AND m.ref = (` + trackRef + `)
		WHERE t.album_id IN (SELECT t2.album_id FROM tracks t2 JOIN files f2 ON f2.id = t2.file_id
				JOIN marks m2 ON m2.kind = 'album' AND m2.ref = (` + strings.ReplaceAll(trackRef, "f.", "f2.") + `)
				WHERE f2.missing = 0 AND t2.album_id IS NOT NULL)
			AND f.missing = 0
		GROUP BY t.album_id
		HAVING SUM(CASE WHEN m.ref IS NOT NULL THEN 1 ELSE 0 END) > 0`)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var id, at int64
		var total, likedN, ratedN int
		var m Mark
		if err := rows.Scan(&id, &total, &likedN, &at, &ratedN, &m.Rating); err != nil {
			rows.Close()
			return p, err
		}
		m.Liked = 2*likedN > total
		if !m.Liked {
			at = 0
		}
		m.LikedAt = at
		if 2*ratedN <= total {
			m.Rating = 0
		}
		if m.Liked || m.Rating > 0 {
			p.Albums[id] = m
		}
	}
	if err := closeRows(rows); err != nil {
		return p, err
	}

	for _, q := range []struct {
		sql string
		to  map[string]int64
	}{
		{`SELECT ref, liked_at FROM marks WHERE kind = 'artist' AND liked = 1`, p.Artists},
		{`SELECT ref, liked_at FROM marks WHERE kind = 'collection' AND liked = 1 AND ref IN (SELECT path FROM folders)`, p.Collections},
	} {
		rows, err = tx.Query(q.sql)
		if err != nil {
			return p, err
		}
		for rows.Next() {
			var ref string
			var at int64
			if err := rows.Scan(&ref, &at); err != nil {
				rows.Close()
				return p, err
			}
			q.to[ref] = at
		}
		if err := closeRows(rows); err != nil {
			return p, err
		}
	}

	rows, err = tx.Query(`SELECT t.file_id, COUNT(*), MAX(p.played_at)
		FROM tracks t JOIN files f ON f.id = t.file_id
		JOIN plays p ON p.ref = (` + trackRef + `)
		WHERE f.missing = 0
		GROUP BY t.file_id`)
	if err != nil {
		return p, err
	}
	for rows.Next() {
		var id int64
		var pl Plays
		if err := rows.Scan(&id, &pl.Count, &pl.Last); err != nil {
			rows.Close()
			return p, err
		}
		p.Plays[id] = pl
	}
	return p, closeRows(rows)
}

// closeRows closes rows and reports the iteration's error.
func closeRows(rows *sql.Rows) error {
	err := rows.Err()
	if cerr := rows.Close(); err == nil {
		err = cerr
	}
	return err
}

// LikedTrackIDs lists the present liked tracks, most recently liked first
// (one id per ref: the lowest file id).
func (s *Store) LikedTrackIDs() ([]int64, error) {
	rows, err := s.db.Query(`SELECT MIN(t.file_id)
		FROM tracks t JOIN files f ON f.id = t.file_id
		JOIN marks m ON m.kind = 'track' AND m.ref = (` + trackRef + `)
		WHERE f.missing = 0 AND m.liked = 1
		GROUP BY m.ref
		ORDER BY m.liked_at DESC, MIN(t.file_id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
