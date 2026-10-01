// Package store persists the catalog in a SQLite file in the app folder.
package store

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
}

// Open opens (creating it if needed) the catalog. Journal mode DELETE instead
// of WAL: safer on removable drives. readOnly opens with query_only.
func Open(file string, readOnly bool) (*Store, error) {
	dsn := file + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if readOnly {
		dsn += "&_pragma=query_only(1)"
	} else {
		dsn += "&_pragma=journal_mode(DELETE)"
	}
	return openDB(dsn, !readOnly)
}

// OpenMemory returns an empty in-memory catalog (browse-only mode without a
// catalog file).
func OpenMemory() (*Store, error) {
	return openDB(":memory:?_pragma=foreign_keys(1)", true)
}

func openDB(dsn string, migrate bool) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if migrate {
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
