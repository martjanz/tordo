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

// oneJobPerRef selects (id, ref, path) of the lowest file id of each ref
// that needs measuring, with its newest first_seen as mx. extra filters the
// outer query.
func oneJobPerRef(extra string) string {
	return `SELECT id, ref, path FROM (
		SELECT f.id AS id, ` + trackRef + ` AS ref, f.path AS path,
			ROW_NUMBER() OVER (PARTITION BY ` + trackRef + ` ORDER BY f.id) AS rn,
			MAX(f.first_seen) OVER (PARTITION BY ` + trackRef + `) AS mx ` + needsMeasure + `
	) WHERE rn = 1 ` + extra
}

// LoudnessPending lists the tracks still to measure, one per ref (the lowest
// file id), newest first.
func (s *Store) LoudnessPending(limit int) ([]loudness.Job, error) {
	return s.loudnessJobs(oneJobPerRef(`ORDER BY mx DESC, id DESC LIMIT ?`), limit)
}

// LoudnessJobs lists, among ids, the tracks still to measure (one per ref,
// its lowest file id).
func (s *Store) LoudnessJobs(ids []int64) ([]loudness.Job, error) {
	var out []loudness.Job
	seen := map[string]bool{}
	for _, chunk := range chunks(ids) {
		jobs, err := s.loudnessJobs(oneJobPerRef(`AND ref IN (SELECT `+trackRef+` FROM files f WHERE f.id IN (`+
			placeholders(len(chunk))+`)) ORDER BY id`), int64Args(chunk)...)
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
			json_extract(a.tags, '$.trackGain'), json_extract(a.tags, '$.albumGain'), l.lufs, l.ref IS NOT NULL
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
			var hasRow bool
			if err := rows.Scan(&id, &t.AlbumID, &t.DurationMs, &tt, &ta, &lu, &hasRow); err != nil {
				rows.Close()
				return nil, err
			}
			t.TagTrack, t.TagAlbum, t.LUFS = nullPtr(tt), nullPtr(ta), nullPtr(lu)
			t.Failed = hasRow && t.LUFS == nil
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

func chunks[T any](ids []T) [][]T {
	var out [][]T
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
