package store

import "testing"

func totalChanges(t *testing.T, s *Store) int64 {
	t.Helper()
	var n int64
	if err := s.db.QueryRow(`SELECT total_changes()`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSyncFilesReportsWhatChanged(t *testing.T) {
	s := open(t)
	a, b := audioRow("../musica/a.mp3", 10, 1), audioRow("../musica/b.mp3", 20, 1)
	image := FileRow{Path: "../musica/a.mp3", Size: 11, MTime: 2, Kind: "image"}
	steps := []struct {
		name string
		seen []FileRow
		want bool
	}{
		{"first sight", []FileRow{a, b}, true},
		{"same files", []FileRow{a, b}, false},
		{"new size", []FileRow{audioRow("../musica/a.mp3", 11, 1), b}, true},
		{"same again", []FileRow{audioRow("../musica/a.mp3", 11, 1), b}, false},
		{"new mtime", []FileRow{audioRow("../musica/a.mp3", 11, 2), b}, true},
		{"new kind", []FileRow{image, b}, true},
		{"a file gone", []FileRow{b}, true},
		{"still gone", []FileRow{b}, false},
		{"it came back", []FileRow{image, b}, true},
		{"a new path", []FileRow{image, b, audioRow("../musica/c.mp3", 5, 1)}, true},
	}
	for _, st := range steps {
		got, err := s.SyncFiles(st.seen, root)
		if err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if got != st.want {
			t.Fatalf("%s: changed = %v, want %v", st.name, got, st.want)
		}
	}
}

func TestSyncFilesWritesNothingWhenNothingChanged(t *testing.T) {
	s := open(t)
	rows := []FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../musica/b.mp3", 20, 1)}
	if _, err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	before := totalChanges(t, s)
	if _, err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	if after := totalChanges(t, s); after != before {
		t.Fatalf("%d rows written for an unchanged disk", after-before)
	}
}

func TestSyncFilesWritesOnlyTheRowThatChanged(t *testing.T) {
	s := open(t)
	if _, err := s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../musica/b.mp3", 20, 1)}, root); err != nil {
		t.Fatal(err)
	}
	before := totalChanges(t, s)
	if _, err := s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 11, 1), audioRow("../musica/b.mp3", 20, 1)}, root); err != nil {
		t.Fatal(err)
	}
	if n := totalChanges(t, s) - before; n != 1 {
		t.Fatalf("%d rows written, want 1", n)
	}
}

// A root that was not walked (unplugged drive) has nothing gone under it.
func TestSyncFilesKeepsAnUnavailableRoot(t *testing.T) {
	s := open(t)
	if _, err := s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1)}, root); err != nil {
		t.Fatal(err)
	}
	changed, err := s.SyncFiles(nil, nil)
	if err != nil || changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
}
