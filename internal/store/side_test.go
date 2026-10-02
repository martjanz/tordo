package store

import (
	"database/sql"
	"errors"
	"testing"
)

func TestSideWritesAreNotCatalogChanges(t *testing.T) {
	s := sample(t)
	c0, err := s.Changes()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.sideWrite(true, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO marks (kind, ref, liked, liked_at, updated_at) VALUES ('track', 'r', 1, 5, 5)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.sideWrite(false, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO loudness (ref, lufs, measured_at) VALUES ('r', -14, 5)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Changes(); c != c0 {
		t.Fatalf("Changes = %d after side writes, want %d", c, c0)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Changes != c0 {
		t.Fatalf("Snapshot.Changes = %d, want %d", snap.Changes, c0)
	}
	if v := s.PersonalVersion(); v != 1 {
		t.Fatalf("PersonalVersion = %d, want 1", v)
	}
	if err := s.SetCollection(la, false); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Changes(); c == c0 {
		t.Fatalf("a catalog write did not move Changes (%d)", c)
	}
}

func TestFailedSideWriteIsNotCounted(t *testing.T) {
	s := sample(t)
	c0, _ := s.Changes()
	boom := errors.New("boom")
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO marks (kind, ref, updated_at) VALUES ('track', 'r', 5)`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM marks`).Scan(&n)
	if n != 0 {
		t.Fatalf("the failed write left %d marks", n)
	}
	if c, _ := s.Changes(); c != c0 {
		t.Fatalf("Changes = %d, want %d", c, c0)
	}
	if v := s.PersonalVersion(); v != 0 {
		t.Fatalf("PersonalVersion = %d, want 0", v)
	}
}
