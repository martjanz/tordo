package config

import (
	"os"
	"path/filepath"
	"testing"
)

// disk makes <tmp>/tordo (the app folder) next to musica/1970s, and a file.
func disk(t *testing.T) (appDir, parent string) {
	t.Helper()
	parent = t.TempDir()
	appDir = filepath.Join(parent, "tordo")
	for _, d := range []string{appDir, filepath.Join(parent, "musica", "1970s"), filepath.Join(appDir, "cache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(parent, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return appDir, parent
}

func TestRoot(t *testing.T) {
	appDir, parent := disk(t)
	for _, tc := range []struct{ in, want string }{
		{"../musica", "../musica"},
		{"../musica/", "../musica"},
		{"./../musica/1970s", "../musica/1970s"},
		{filepath.Join(parent, "musica"), "../musica"},
		{"  ../musica  ", "../musica"},
	} {
		got, err := Root(appDir, tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Root(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "../nada", "../notes.txt", ".", appDir, "..", parent, "cache", filepath.Join(appDir, "cache")} {
		if got, err := Root(appDir, in); err == nil {
			t.Errorf("Root(%q) = %q, want an error", in, got)
		}
	}
}

// A case-variant spelling of an existing folder resolves to the on-disk name,
// so a case-insensitive filesystem never admits one folder under two spellings.
func TestRootCanonicalizesCase(t *testing.T) {
	parent := t.TempDir()
	appDir := filepath.Join(parent, "tordo")
	for _, d := range []string{filepath.Join(parent, "Musica", "1970s"), appDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	lower := filepath.Join(parent, "musica")
	if _, err := os.Stat(lower); err != nil {
		t.Skip("case-sensitive filesystem: no case-insensitive match to canonicalize")
	}
	if got, err := Root(appDir, lower); err != nil || got != "../Musica" {
		t.Fatalf("Root(%q) = %q, %v; want ../Musica", lower, got, err)
	}
	nested := filepath.Join(parent, "musica", "1970S")
	if got, err := Root(appDir, nested); err != nil || got != "../Musica/1970s" {
		t.Fatalf("Root(%q) = %q, %v; want ../Musica/1970s", nested, got, err)
	}
}

func TestCheckRoots(t *testing.T) {
	appDir, _ := disk(t)
	for _, roots := range [][]string{
		{"../musica"},
		{"../musica", "../gone"}, // saved, unplugged
	} {
		if err := CheckRoots(appDir, roots, []string{"../gone"}); err != nil {
			t.Errorf("%v: %v", roots, err)
		}
	}
	for _, roots := range [][]string{
		nil,
		{"../gone"},                      // new and missing
		{"../musica/"},                   // not in catalog form
		{"../musica", "../musica"},       // repeated
		{"../musica", "../musica/1970s"}, // nested
		{"../musica/1970s", "../musica"}, // nested, the other way
	} {
		if err := CheckRoots(appDir, roots, nil); err == nil {
			t.Errorf("%v: want an error", roots)
		}
	}
}

func TestAvailable(t *testing.T) {
	appDir, _ := disk(t)
	if !Available(appDir, "../musica") || Available(appDir, "../nada") || Available(appDir, "../notes.txt") {
		t.Fatal("Available")
	}
}
