package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"tordo/internal/grouping"
)

// m1Catalog writes a catalog file shaped like M1's (user_version 0).
func m1Catalog(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "tordo.db")
	db, err := sql.Open("sqlite", file)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT NOT NULL UNIQUE, size INTEGER NOT NULL,
			mtime INTEGER NOT NULL, kind TEXT NOT NULL, fingerprint TEXT NOT NULL DEFAULT '', missing INTEGER NOT NULL DEFAULT 0,
			first_seen INTEGER NOT NULL);
		INSERT INTO files (path, size, mtime, kind, first_seen) VALUES ('../musica/a.mp3', 1, 1, 'audio', 1);
		CREATE TABLE albums (id INTEGER PRIMARY KEY, key TEXT NOT NULL UNIQUE, title TEXT NOT NULL);
		INSERT INTO albums (key, title) VALUES ('k', 'Viejo');`); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestOpenMigratesM1Catalog(t *testing.T) {
	file := m1Catalog(t)
	s, err := Open(file, false)
	if err != nil {
		t.Fatal(err)
	}
	var files, albums, version int
	s.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&files)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM albums WHERE status = ''`).Scan(&albums); err != nil {
		t.Fatalf("albums has no status column: %v", err)
	}
	s.db.QueryRow(`PRAGMA user_version`).Scan(&version)
	if files != 1 || albums != 0 || version != schemaVersion {
		t.Fatalf("files %d albums %d version %d", files, albums, version)
	}
	if _, err := s.db.Exec(`INSERT INTO folders (path, parent, name, tracks, albums, duration_ms, collection)
		VALUES ('../musica/x', '../musica', 'x', 0, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	// Make it a v2 catalog: no not_collections yet.
	if _, err := s.db.Exec(`DROP TABLE not_collections; PRAGMA user_version = 2`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(file, false) // v2 to v3 only adds a table: nothing is dropped
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var folders int
	s.db.QueryRow(`SELECT COUNT(*) FROM folders`).Scan(&folders)
	if folders != 1 {
		t.Fatalf("a v2 catalog lost its folders on open")
	}
	if err := s.SetCollection("../musica/x", false); err != nil {
		t.Fatalf("no not_collections table: %v", err)
	}
}

func TestOpenAddsUserTables(t *testing.T) {
	file := m1Catalog(t)
	s, err := Open(file, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TABLE marks; DROP TABLE plays; DROP TABLE playlist_items; DROP TABLE playlists;
		DROP TABLE loudness; DROP TABLE stations; PRAGMA user_version = 3`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO albums (key, dir, title, artist, year, compilation, discs, tracks, duration_ms, position,
		cover_path, cover_file_id, genres) VALUES ('k2', '../musica/k2', 'Kept', '', 0, 0, 1, 1, 0, 0, '', 0, '[]')`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(file, false) // v3 to v6 only adds tables
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version, albums int
	s.db.QueryRow(`PRAGMA user_version`).Scan(&version)
	s.db.QueryRow(`SELECT COUNT(*) FROM albums WHERE key = 'k2'`).Scan(&albums)
	if version != 7 || albums != 1 {
		t.Fatalf("version %d, kept albums %d", version, albums)
	}
	for _, name := range []string{"marks", "plays", "playlists", "playlist_items", "loudness", "stations", "sessions"} {
		var n int
		s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&n)
		if n != 1 {
			t.Errorf("table %s missing", name)
		}
	}
}

func TestReadOnlyOpenRejectsOldCatalog(t *testing.T) {
	if _, err := Open(m1Catalog(t), true); !errors.Is(err, ErrOldCatalog) {
		t.Fatalf("err = %v, want ErrOldCatalog", err)
	}
}

func TestSelectionIsStored(t *testing.T) {
	s := open(t)
	if err := s.ReplaceGrouping(grouping.Result{Albums: []grouping.Album{
		{Key: "k", Dir: "../musica/baulera", Title: "baulera", Tracks: 1, Discs: 1, Selection: true},
	}}); err != nil {
		t.Fatal(err)
	}
	var sel bool
	if err := s.db.QueryRow(`SELECT selection FROM albums WHERE key = 'k'`).Scan(&sel); err != nil || !sel {
		t.Fatalf("selection = %v, err %v", sel, err)
	}
}

func TestOpenMigratesV6Sessions(t *testing.T) {
	// Build a v6 catalog
	file := filepath.Join(t.TempDir(), "tordo.db")
	db, err := sql.Open("sqlite", file)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE sessions (
		id         INTEGER PRIMARY KEY,
		token_hash TEXT NOT NULL UNIQUE,
		device     TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		last_seen  INTEGER NOT NULL
	);
	INSERT INTO sessions (id, token_hash, device, created_at, last_seen) VALUES (1, 'hash-old', 'Viejo', 1000, 2000);
	PRAGMA user_version = 6`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Reopen -> user_version 7
	s, err := Open(file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 7 {
		t.Fatalf("version = %d, want 7, err %v", version, err)
	}

	// Old row reads with Host == false
	se, ok, err := s.SessionByToken("hash-old")
	if err != nil || !ok || se.Host != false || se.Device != "Viejo" {
		t.Fatalf("SessionByToken: %+v %v %v", se, ok, err)
	}

	// SetSessionHost works on it
	found, err := s.SetSessionHost(se.ID, true)
	if !found || err != nil {
		t.Fatalf("SetSessionHost: %v %v", found, err)
	}
	se, ok, err = s.SessionByToken("hash-old")
	if err != nil || !ok || se.Host != true {
		t.Fatalf("SessionByToken after set: %+v %v %v", se, ok, err)
	}
}
