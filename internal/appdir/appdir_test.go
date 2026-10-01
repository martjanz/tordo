package appdir

import (
	"path/filepath"
	"testing"
)

func TestRelAbsRoundTrip(t *testing.T) {
	app := filepath.Join(t.TempDir(), "tordo")
	movie := filepath.Join(filepath.Dir(app), "musica", "Uruguay", "Zitarrosa.mp3")

	rel, err := Rel(app, movie)
	if err != nil {
		t.Fatal(err)
	}
	if rel != "../musica/Uruguay/Zitarrosa.mp3" {
		t.Fatalf("Rel = %q", rel)
	}
	if got := Abs(app, rel); got != movie {
		t.Fatalf("Abs = %q, want %q", got, movie)
	}
}

func TestWritable(t *testing.T) {
	if !Writable(t.TempDir()) {
		t.Fatal("temp dir should be writable")
	}
	if Writable(filepath.Join(t.TempDir(), "does-not-exist")) {
		t.Fatal("missing dir must not be writable")
	}
}

func TestResolveOverride(t *testing.T) {
	dir := t.TempDir()
	got, err := Resolve(dir)
	if err != nil || got != dir {
		t.Fatalf("Resolve(%q) = %q, %v", dir, got, err)
	}
}
