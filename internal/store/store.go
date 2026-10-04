// Package store persists the catalog in a SQLite file in the app folder.
package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct {
	db   *sql.DB
	side side
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

// schemaVersion is kept in PRAGMA user_version. Version 2 (M2) added columns
// to the derived tables: below it they are dropped and the next scan
// rebuilds them. Version 3 (M3b) only added not_collections. The other
// tables are only ever added to. Version 4 (M4a) added the user's tables
// (marks, plays, playlists) and loudness. Version 5 (M4b) added saved stations. Version 6 (M5) added the LAN sessions.
// Version 7 (M6) added sessions.host (the Rockola's anfitriones).
const schemaVersion = 7

// derivedVersion is the last version that changed the derived tables.
const derivedVersion = 2

// ErrOldCatalog means a catalog from an older version was opened read-only,
// so it could not be updated.
var ErrOldCatalog = errors.New("el catálogo es de una versión anterior: abrí tordo una vez con la carpeta escribible para actualizarlo")

func openDB(dsn string, writable bool) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if writable {
		err = migrate(db)
	} else {
		err = checkVersion(db)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v < derivedVersion {
		if _, err := db.Exec(`DROP TABLE IF EXISTS tracks; DROP TABLE IF EXISTS album_genres;
			DROP TABLE IF EXISTS albums; DROP TABLE IF EXISTS folders`); err != nil {
			return err
		}
	}
	if v == 6 {
		// The ALTER and the version bump are not atomic: a previous run may have died in between.
		var has int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name = 'host'`).Scan(&has); err != nil {
			return err
		}
		if has == 0 {
			if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN host INTEGER NOT NULL DEFAULT 0`); err != nil {
				return err
			}
		}
	}
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion))
	return err
}

func checkVersion(db *sql.DB) error {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v < schemaVersion {
		return ErrOldCatalog
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }
