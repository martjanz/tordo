package store

import (
	"testing"

	"tordo/internal/grouping"
)

// Every startup scan regroups the whole catalog. When nothing changed on disk
// the result is the same, and rewriting it would bump the change counter
// (the pages rebuild their catalog) and wipe what identification set on the
// tracks.
func TestReplaceGroupingWithTheSameResultWritesNothing(t *testing.T) {
	s := sample(t)
	before, err := s.Changes()
	if err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	if after, _ := s.Changes(); after != before {
		t.Fatalf("an identical regroup changed %d rows", after-before)
	}
}

func TestReplaceGroupingWithADifferentResultWrites(t *testing.T) {
	s := sample(t)
	before, _ := s.Changes()
	tracks, images, err := s.GroupingInput()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGrouping(grouping.Build(tracks[1:], images, root)); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.Changes(); after == before {
		t.Fatal("a different grouping was not written")
	}
	n, _ := s.Counts()
	if n.Tracks != len(tracks)-1 {
		t.Fatalf("tracks = %d, want %d", n.Tracks, len(tracks)-1)
	}
	// And back: the stamp follows what is stored, not the first result.
	regroup(t, s)
	if n, _ := s.Counts(); n.Tracks != len(tracks) {
		t.Fatalf("tracks = %d, want %d", n.Tracks, len(tracks))
	}
}

// A catalog whose derived tables are dropped by a migration must be
// regrouped even when the result matches the one stored before.
func TestDroppedDerivedTablesForgetTheGrouping(t *testing.T) {
	s := sample(t)
	if _, err := s.db.Exec(`DELETE FROM tracks; DELETE FROM folders; DELETE FROM albums; PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(s.db); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	if n, _ := s.Counts(); n.Tracks == 0 || n.Albums == 0 {
		t.Fatalf("not regrouped after the migration: %+v", n)
	}
}
