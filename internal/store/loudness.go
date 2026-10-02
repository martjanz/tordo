package store

import (
	"database/sql"
	"strings"

	"tordo/internal/loudness"
)

// loudnessChunk is how many ids go in one query.
const loudnessChunk = 500

// needsMeasure selects the present tracks with no track gain tag and no
// loudness row (measured or failed).
const needsMeasure = `FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
	WHERE f.missing = 0 AND json_extract(a.tags, '$.trackGain') IS NULL
	AND NOT EXISTS (SELECT 1 FROM loudness l WHERE l.ref = ` + trackRef + `)`

// LoudnessPending lists the tracks still to measure, one per ref (the lowest
// file id), newest first.
func (s *Store) LoudnessPending(limit int) ([]loudness.Job, error) {
	return s.loudnessJobs(`SELECT MIN(f.id), `+trackRef+`, f.path `+needsMeasure+`
		GROUP BY `+trackRef+` ORDER BY MAX(f.first_seen) DESC, MIN(f.id) DESC LIMIT ?`, limit)
}

// LoudnessJobs lists, among ids, the tracks still to measure (one per ref).
func (s *Store) LoudnessJobs(ids []int64) ([]loudness.Job, error) {
	var out []loudness.Job
	seen := map[string]bool{}
	for _, chunk := range chunks(ids) {
		jobs, err := s.loudnessJobs(`SELECT MIN(f.id), `+trackRef+`, f.path `+needsMeasure+`
			AND f.id IN (`+placeholders(len(chunk))+`) GROUP BY `+trackRef+` ORDER BY MIN(f.id)`, int64Args(chunk)...)
		if err != nil {
			return nil, err
		}
		for _, j := range jobs {
			if !seen[j.Ref] {
				seen[j.Ref] = true
				out = append(out, j)
			}
		}
	}
	return out, nil
}

func (s *Store) loudnessJobs(query string, args ...any) ([]loudness.Job, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []loudness.Job
	for rows.Next() {
		var j loudness.Job
		if err := rows.Scan(&j.ID, &j.Ref, &j.Path); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// LoudnessPendingCount is how many tracks are still to measure.
func (s *Store) LoudnessPendingCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(DISTINCT ` + trackRef + `) ` + needsMeasure).Scan(&n)
	return n, err
}

// SaveLoudness records a measure, or (lufs nil) why there is none, so the
// track is not tried again. It does not count as a catalog change.
func (s *Store) SaveLoudness(ref string, lufs *float64, errText string) error {
	return s.sideWrite(false, func(tx *sql.Tx) error {
		var v any
		if lufs != nil {
			v = *lufs
		}
		_, err := tx.Exec(`INSERT INTO loudness (ref, lufs, error, measured_at) VALUES (?1, ?2, ?3, ?4)
			ON CONFLICT(ref) DO UPDATE SET lufs = excluded.lufs, error = excluded.error, measured_at = excluded.measured_at`,
			ref, v, errText, now().UnixMilli())
		return err
	})
}

// LoudnessFacts returns what the gains of the given present tracks come
// from, together with every present track of their albums.
func (s *Store) LoudnessFacts(ids []int64) (map[int64]loudness.Track, error) {
	out := map[int64]loudness.Track{}
	for _, chunk := range chunks(ids) {
		ph := placeholders(len(chunk))
		args := int64Args(chunk)
		rows, err := s.db.Query(`SELECT t.file_id, COALESCE(t.album_id, 0), t.duration_ms,
			json_extract(a.tags, '$.trackGain'), json_extract(a.tags, '$.albumGain'), l.lufs
			FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
			LEFT JOIN loudness l ON l.ref = `+trackRef+`
			WHERE f.missing = 0 AND (t.file_id IN (`+ph+`)
				OR t.album_id IN (SELECT album_id FROM tracks WHERE file_id IN (`+ph+`)))`,
			append(args, args...)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			var t loudness.Track
			var tt, ta, lu sql.NullFloat64
			if err := rows.Scan(&id, &t.AlbumID, &t.DurationMs, &tt, &ta, &lu); err != nil {
				rows.Close()
				return nil, err
			}
			t.TagTrack, t.TagAlbum, t.LUFS = nullPtr(tt), nullPtr(ta), nullPtr(lu)
			out[id] = t
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func nullPtr(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

func chunks(ids []int64) [][]int64 {
	var out [][]int64
	for len(ids) > 0 {
		n := min(len(ids), loudnessChunk)
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	return out
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func int64Args(ids []int64) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
