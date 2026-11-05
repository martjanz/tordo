package store

import (
	"database/sql"
	"path"
	"slices"
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

// SyncFiles writes what differs between the files seen in a scan and the
// catalog, and marks as missing the known files under scannedRoots that were
// not seen. Files under other roots keep their state, so an unplugged drive
// does not wipe its entries. A file whose size or mtime changed loses its
// fingerprint until it is read again. It reports whether anything changed; an
// unchanged disk writes nothing.
func (s *Store) SyncFiles(seen []FileRow, scannedRoots []string) (bool, error) {
	return s.SyncFilesExcept(seen, scannedRoots, nil)
}

// SyncFilesExcept is SyncFiles for a walk that could not read some folders or
// files: those (and what a folder holds) keep their state, since not seeing
// them says nothing.
func (s *Store) SyncFilesExcept(seen []FileRow, scannedRoots, unreadable []string) (bool, error) {
	known, err := s.knownFiles()
	if err != nil {
		return false, err
	}
	var write []FileRow
	for _, f := range seen {
		k, ok := known[f.Path]
		if !ok || k.missing || k.row != f {
			write = append(write, f)
		}
		delete(known, f.Path)
	}
	var gone []string
	for p, k := range known { // what is left was not seen
		if !k.missing && underAny(p, scannedRoots) && !slices.Contains(unreadable, p) && !underAny(p, unreadable) {
			gone = append(gone, p)
		}
	}
	if len(write) == 0 && len(gone) == 0 {
		return false, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, kind, missing, first_seen) VALUES (?1, ?2, ?3, ?4, 0, ?5)
		ON CONFLICT(path) DO UPDATE SET
			fingerprint = CASE WHEN files.size != excluded.size OR files.mtime != excluded.mtime THEN '' ELSE files.fingerprint END,
			size = excluded.size, mtime = excluded.mtime, kind = excluded.kind, missing = 0`)
	if err != nil {
		return false, err
	}
	defer up.Close()
	now := time.Now().UnixMilli()
	for _, f := range write {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Kind, now); err != nil {
			return false, err
		}
	}
	if err := markMissing(tx, gone); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

type knownFile struct {
	row     FileRow
	missing bool
}

// knownFiles returns every file in the catalog by path.
func (s *Store) knownFiles() (map[string]knownFile, error) {
	rows, err := s.db.Query(`SELECT path, size, mtime, kind, missing FROM files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]knownFile{}
	for rows.Next() {
		var k knownFile
		if err := rows.Scan(&k.row.Path, &k.row.Size, &k.row.MTime, &k.row.Kind, &k.missing); err != nil {
			return nil, err
		}
		out[k.row.Path] = k
	}
	return out, rows.Err()
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
