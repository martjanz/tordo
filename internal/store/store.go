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
	db      *sql.DB
	file    string // the catalog file, "" in memory
	side    side
	pending pendingList // guarded by side.mu
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
	s, err := openDB(dsn, !readOnly)
	if err != nil {
		return nil, err
	}
	s.file = file
	return s, nil
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
// Version 7 (M6) added sessions.host (the Rockola's anfitriones). grouping_stamp came
// later without a new version: it is created on the next writable open, and a
// read-only one never regroups.
const schemaVersion = 8

// derivedVersion is the last version that changed the derived tables.
const derivedVersion = 2

// ErrOldCatalog means a catalog from an older version was opened read-only,
// so it could not be updated.
var ErrOldCatalog = errors.New("the catalog is from an older version: open tordo once with a writable folder to update it")

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
			DROP TABLE IF EXISTS albums; DROP TABLE IF EXISTS folders; DROP TABLE IF EXISTS grouping_stamp`); err != nil {
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
	if v < 8 {
		if err := upgradeVocabulary(db); err != nil {
			return err
		}
	}
	_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion))
	return err
}

// upgradeVocabulary is the v8 step: saved stations move to the English
// vocabulary and the Various Artists marker changes its text. The grouping
// stamp goes so the next scan regroups. It all happens in one transaction.
func upgradeVocabulary(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	type row struct {
		id  int64
		def []byte
	}
	rows, err := tx.Query(`SELECT id, def FROM stations`)
	if err != nil {
		return err
	}
	var changed []row
	for rows.Next() {
		var r row
		var def string
		if err := rows.Scan(&r.id, &def); err != nil {
			rows.Close()
			return err
		}
		if up, ok := upgradeStationDef([]byte(def)); ok {
			r.def = up
			changed = append(changed, r)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, r := range changed {
		if _, err := tx.Exec(`UPDATE stations SET def = ? WHERE id = ?`, string(r.def), r.id); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`UPDATE albums SET artist = 'Various Artists' WHERE artist = 'Varios artistas'`,
		`UPDATE albums SET mb_artist = 'Various Artists' WHERE mb_artist = 'Varios artistas'`,
		`DELETE FROM grouping_stamp`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return tx.Commit()
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
