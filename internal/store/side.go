package store

import (
	"database/sql"
	"sync"
)

// trackRef is, in SQL over files f, what the user's data is keyed by: the
// audio fingerprint, so that it survives moving, renaming and re-tagging.
const trackRef = `CASE WHEN f.fingerprint != '' THEN f.fingerprint ELSE 'id:' || f.id END`

// side keeps the user's writes (and measured loudness) out of the catalog's
// change counter: they do not change what the catalog shows, and counting
// them would rebuild it on every like or play.
type side struct {
	mu       sync.Mutex // held by side writes and by reads of the counter
	rows     int64      // rows changed by side writes since the store opened
	personal int64      // personal side writes since the store opened
}

// sideWrite runs fn in a transaction whose changes are not catalog
// changes. personal marks the user's data (marks, plays, playlists) as
// changed. The lock is always taken before the connection. fn runs holding
// the side lock and the single connection: it must read and write only
// through tx (using the store's other methods would deadlock).
func (s *Store) sideWrite(personal bool, fn func(tx *sql.Tx) error) error {
	s.side.mu.Lock()
	defer s.side.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before, after int64
	if err := tx.QueryRow(`SELECT total_changes()`).Scan(&before); err != nil {
		return err
	}
	ferr := fn(tx)
	// Count what fn changed even when it failed: total_changes keeps
	// counting rows a rollback undoes.
	if err := tx.QueryRow(`SELECT total_changes()`).Scan(&after); err != nil {
		return err
	}
	s.side.rows += after - before
	if ferr != nil {
		return ferr
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if personal {
		s.side.personal++
	}
	return nil
}

// Changes is the change counter of Snapshot, read on its own: a cheap way to
// know whether what was derived from the last snapshot is still current.
// Side writes do not count.
func (s *Store) Changes() (int64, error) {
	s.side.mu.Lock()
	defer s.side.mu.Unlock()
	var n int64
	err := s.db.QueryRow(`SELECT total_changes()`).Scan(&n)
	return n - s.side.rows, err
}

// PersonalVersion grows with every change to the user's data.
func (s *Store) PersonalVersion() int64 {
	s.side.mu.Lock()
	defer s.side.mu.Unlock()
	return s.side.personal
}
