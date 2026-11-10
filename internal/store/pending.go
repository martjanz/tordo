package store

import (
	"database/sql"
	"strconv"
	"strings"

	"tordo/internal/loudness"
)

// pendingList is what LoudnessPending lists, for one state of the catalog:
// the lowest file id of each ref to measure, newest first (by its newest
// first_seen, then by that id). Building it reads every track, so it is kept
// until the catalog changes: a ref's place depends only on the catalog, and
// a measure only takes a ref out. A ref measured since the list was built
// stays in ids until a read drops it; measured counts those. Guarded by
// side.mu.
type pendingList struct {
	built    bool
	changes  int64 // the catalog's change counter it was built at
	ids      []int64
	measured int
}

// withPending runs fn on a current pending list, in a transaction, holding
// the side lock (taken before the connection, as sideWrite does).
func (s *Store) withPending(fn func(tx *sql.Tx, p *pendingList) error) error {
	s.side.mu.Lock()
	defer s.side.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var total int64
	if err := tx.QueryRow(`SELECT total_changes()`).Scan(&total); err != nil {
		return err
	}
	p := &s.pending
	if c := total - s.side.rows; !p.built || p.changes != c {
		ids, err := pendingIDs(tx)
		if err != nil {
			return err
		}
		*p = pendingList{built: true, changes: c, ids: ids}
	}
	if err := fn(tx, p); err != nil {
		return err
	}
	return tx.Commit()
}

func pendingIDs(tx *sql.Tx) ([]int64, error) {
	rows, err := tx.Query(`SELECT MIN(f.id) AS id, MAX(f.first_seen) AS mx ` + needsMeasure +
		` GROUP BY ` + trackRef + ` ORDER BY mx DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id, mx int64
		if err := rows.Scan(&id, &mx); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// LoudnessPending lists the tracks still to measure, one per ref (the lowest
// file id), newest first.
func (s *Store) LoudnessPending(limit int) ([]loudness.Job, error) {
	var out []loudness.Job
	err := s.withPending(func(tx *sql.Tx, p *pendingList) error {
		var kept []int64 // what is still pending in ids[:hi]
		hi := 0
		for hi < len(p.ids) && (limit < 0 || len(out) < limit) {
			chunk := p.ids[hi:min(hi+loudnessChunk, len(p.ids))]
			jobs, err := unmeasured(tx, chunk)
			if err != nil {
				return err
			}
			for _, id := range chunk {
				j, ok := jobs[id]
				if !ok { // measured since the list was built
					p.measured--
					continue
				}
				kept = append(kept, id)
				if limit < 0 || len(out) < limit {
					out = append(out, j)
				}
			}
			hi += len(chunk)
		}
		copy(p.ids[hi-len(kept):hi], kept)
		p.ids = p.ids[hi-len(kept):]
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// unmeasured returns, by file id, the jobs of the files among ids whose ref
// has no loudness row.
func unmeasured(tx *sql.Tx, ids []int64) (map[int64]loudness.Job, error) {
	rows, err := tx.Query(`SELECT f.id, `+trackRef+`, f.path FROM files f WHERE f.id IN (`+placeholders(len(ids))+`)
		AND NOT EXISTS (SELECT 1 FROM loudness l WHERE l.ref = `+trackRef+`)`, int64Args(ids)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]loudness.Job{}
	for rows.Next() {
		var j loudness.Job
		if err := rows.Scan(&j.ID, &j.Ref, &j.Path); err != nil {
			return nil, err
		}
		out[j.ID] = j
	}
	return out, rows.Err()
}

// LoudnessPendingCount is how many tracks are still to measure.
func (s *Store) LoudnessPendingCount() (int, error) {
	var n int
	err := s.withPending(func(_ *sql.Tx, p *pendingList) error {
		n = len(p.ids) - p.measured
		return nil
	})
	return n, err
}

// SaveLoudness records a measure, or (lufs nil) why there is none, so the
// track is not tried again. It does not count as a catalog change.
func (s *Store) SaveLoudness(ref string, lufs *float64, errText string) error {
	var listed bool // a ref of the pending list got its first row
	return s.sideWriteThen(false, func(tx *sql.Tx) error {
		var v any
		if lufs != nil {
			v = *lufs
		}
		if s.pending.built {
			var err error
			if listed, err = newlyMeasured(tx, ref); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO loudness (ref, lufs, error, measured_at) VALUES (?1, ?2, ?3, ?4)
			ON CONFLICT(ref) DO UPDATE SET lufs = excluded.lufs, error = excluded.error, measured_at = excluded.measured_at`,
			ref, v, errText, now().UnixMilli())
		return err
	}, func() {
		if listed {
			s.pending.measured++
		}
	})
}

// newlyMeasured tells whether ref has no loudness row yet and is the ref of
// a present track with no gain tag: then it is in the pending list.
func newlyMeasured(tx *sql.Tx, ref string) (bool, error) {
	var id any // the file of an "id:N" ref
	if rest, ok := strings.CutPrefix(ref, "id:"); ok {
		if n, err := strconv.ParseInt(rest, 10, 64); err == nil {
			id = n
		}
	}
	var ok bool
	err := tx.QueryRow(`SELECT NOT EXISTS (SELECT 1 FROM loudness WHERE ref = ?1) AND EXISTS (SELECT 1
		FROM files f JOIN tracks t ON t.file_id = f.id JOIN audio a ON a.file_id = f.id
		WHERE (f.fingerprint = ?1 OR (f.fingerprint = '' AND f.id = ?2))
		AND f.missing = 0 AND json_extract(a.tags, '$.trackGain') IS NULL)`, ref, id).Scan(&ok)
	return ok, err
}
