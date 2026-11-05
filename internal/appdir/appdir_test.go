package appdir

import (
	"path/filepath"
	"runtime"
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

func TestVerbatimOnlyWhereNeeded(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows trims names")
	}
	app := `D:\tordo`
	for _, c := range []struct{ rel, want string }{
		{"../musica/Uruguay/Zitarrosa.mp3", `D:\musica\Uruguay\Zitarrosa.mp3`},
		{"../musica/Vol. 1/a.mp3", `D:\musica\Vol. 1\a.mp3`},
		{"../musica/Flac /a.mp3", `\\?\D:\musica\Flac \a.mp3`},
		{"../musica/Etc./a.mp3", `\\?\D:\musica\Etc.\a.mp3`},
	} {
		if got := Abs(app, c.rel); got != c.want {
			t.Errorf("Abs(%q) = %q, want %q", c.rel, got, c.want)
		}
	}
	if got := Abs(`\\nas\musica\tordo`, "../Flac /a.mp3"); got != `\\?\UNC\nas\musica\Flac \a.mp3` {
		t.Errorf("Abs on a network share = %q", got)
	}
}

func TestVerbatimRoundTrip(t *testing.T) {
	app := filepath.Join(t.TempDir(), "tordo")
	want := "../musica/Flac /01.mp3"
	got, err := Rel(app, Abs(app, want))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Rel = %q, want %q", got, want)
	}
	// A walk starts from Verbatim, so every path it gives is in that form.
	got, err = Rel(app, filepath.Join(Verbatim(filepath.Join(filepath.Dir(app), "musica")), "a.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "../musica/a.mp3" {
		t.Fatalf("Rel of a walked path = %q", got)
	}
}
