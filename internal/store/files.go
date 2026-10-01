package store

import (
	"database/sql"
	"path"
	"strings"
	"time"
)

// FileRow is a catalogued file as the walk saw it.
type FileRow struct {
	Path  string // catalog form
	Size  int64
	MTime int64 // unix ms
	Kind  string
}

// SyncFiles upserts the files seen in a scan and marks as missing the known
// files under scannedRoots that were not seen. Files under other roots keep
// their state, so an unplugged drive does not wipe its entries. A file whose
// size or mtime changed loses its fingerprint until it is read again.
func (s *Store) SyncFiles(seen []FileRow, scannedRoots []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, kind, missing, first_seen) VALUES (?1, ?2, ?3, ?4, 0, ?5)
		ON CONFLICT(path) DO UPDATE SET
			fingerprint = CASE WHEN files.size != excluded.size OR files.mtime != excluded.mtime THEN '' ELSE files.fingerprint END,
			size = excluded.size, mtime = excluded.mtime, kind = excluded.kind, missing = 0`)
	if err != nil {
		return err
	}
	defer up.Close()
	now := time.Now().UnixMilli()
	present := make(map[string]bool, len(seen))
	for _, f := range seen {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Kind, now); err != nil {
			return err
		}
		present[f.Path] = true
	}
	gone, err := presentPaths(tx, func(p string) bool { return !present[p] && underAny(p, scannedRoots) })
	if err != nil {
		return err
	}
	if err := markMissing(tx, gone); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkOutsideRoots marks as missing the present files that are not under any
// of roots (the configured ones, available or not): what a removed root
// leaves behind. They come back if the root is added again.
func (s *Store) MarkOutsideRoots(roots []string) error {
	clean := make([]string, len(roots))
	for i, r := range roots {
		clean[i] = path.Clean(r)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	gone, err := presentPaths(tx, func(p string) bool { return !underAny(p, clean) })
	if err != nil {
		return err
	}
	if err := markMissing(tx, gone); err != nil {
		return err
	}
	return tx.Commit()
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

// presentPaths returns the present files whose path satisfies keep.
func presentPaths(q querier, keep func(string) bool) ([]string, error) {
	rows, err := q.Query(`SELECT path FROM files WHERE missing = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if keep(p) {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

func markMissing(q querier, paths []string) error {
	for _, p := range paths {
		if _, err := q.Exec(`UPDATE files SET missing = 1 WHERE path = ?`, p); err != nil {
			return err
		}
	}
	return nil
}

func underAny(p string, roots []string) bool {
	for _, r := range roots {
		if strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/") {
			return true
		}
	}
	return false
}
