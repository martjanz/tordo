package scan

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// failing walks normally but reports the folders named in names as unreadable.
func failing(names ...string) func(string, fs.WalkDirFunc) error {
	return func(root string, fn fs.WalkDirFunc) error {
		return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				for _, n := range names {
					if d.Name() == n {
						return fn(p, d, errors.New("sin permiso"))
					}
				}
			}
			return fn(p, d, err)
		})
	}
}

func (l *lib) trackPaths() map[string]bool {
	l.t.Helper()
	tracks, _, err := l.st.GroupingInput()
	if err != nil {
		l.t.Fatal(err)
	}
	out := map[string]bool{}
	for _, tr := range tracks {
		out[tr.Path] = true
	}
	return out
}

func TestUnreadableFolderDoesNotBlockMissing(t *testing.T) {
	l := newLib(t)
	l.write("bien/a.mp3", tags("A", "x", "Uno", 1))
	l.write("bien/borrado.mp3", tags("B", "x", "Uno", 2))
	l.write("rota/c.mp3", tags("C", "x", "Dos", 1))
	l.run()

	if err := os.Remove(filepath.Join(l.root, "bien", "borrado.mp3")); err != nil {
		t.Fatal(err)
	}
	l.sc.walkDir = failing("rota")
	st := l.run()

	got := l.trackPaths()
	if got["../musica/bien/borrado.mp3"] {
		t.Error("the deleted file is still in the catalog")
	}
	if !got["../musica/bien/a.mp3"] || !got["../musica/rota/c.mp3"] {
		t.Errorf("tracks = %v, want a.mp3 and what the unreadable folder held", got)
	}
	if st.Skipped != 1 || len(st.SkippedPaths) != 1 || st.SkippedPaths[0] != "../musica/rota" {
		t.Errorf("skipped = %d %v", st.Skipped, st.SkippedPaths)
	}

	// Readable again: it is no longer reported.
	l.sc.walkDir = nil
	if st := l.run(); st.Skipped != 0 || len(st.SkippedPaths) != 0 {
		t.Errorf("skipped after recovering = %d %v", st.Skipped, st.SkippedPaths)
	}
}

func TestSkippedPathsAreCapped(t *testing.T) {
	l := newLib(t)
	var names []string
	for i := range maxSkippedPaths + 5 {
		name := fmt.Sprintf("rota%02d", i)
		names = append(names, name)
		l.write(name+"/a.mp3", tags("A", "x", name, 1))
	}
	l.sc.walkDir = failing(names...)
	st := l.run()
	if st.Skipped != maxSkippedPaths+5 || len(st.SkippedPaths) != maxSkippedPaths {
		t.Errorf("skipped = %d, %d paths", st.Skipped, len(st.SkippedPaths))
	}
}

func TestFolderNameEndingInSpaceIsCatalogued(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows trims names")
	}
	l := newLib(t)
	l.write("normal/a.mp3", tags("A", "x", "Uno", 1))
	for _, dir := range []string{"Flac ", "Etc."} {
		long := `\\?\` + filepath.Join(l.root, dir)
		if err := os.Mkdir(long, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(long+`\b.mp3`, []byte(tags("B", "y", dir, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			os.Remove(long + `\b.mp3`)
			os.Remove(long)
		})
	}
	st := l.run()
	if st.Skipped != 0 {
		t.Errorf("skipped = %d %v", st.Skipped, st.SkippedPaths)
	}
	got := l.trackPaths()
	if !got["../musica/Flac /b.mp3"] || !got["../musica/Etc./b.mp3"] || !got["../musica/normal/a.mp3"] {
		t.Errorf("tracks = %v", got)
	}
}

// A timed check that finds changes starts a visible run: the folders it could
// not read have to stay named.
func TestRefreshWithChangesKeepsTheSkippedFolders(t *testing.T) {
	l := newLib(t)
	l.write("bien/a.mp3", tags("A", "x", "Uno", 1))
	l.write("rota/c.mp3", tags("C", "x", "Dos", 1))
	l.run()

	l.write("bien/nuevo.mp3", tags("N", "x", "Uno", 2))
	l.sc.walkDir = failing("rota")
	st := l.refresh()
	if st.ToRead != 1 || st.Skipped != 1 || len(st.SkippedPaths) != 1 || st.SkippedPaths[0] != "../musica/rota" {
		t.Fatalf("status = %+v", st)
	}
}

// With nothing else to do the check stays quiet, but what it could read is
// still news: the status follows the folder going bad and coming back.
func TestQuietRefreshFollowsTheSkippedFolders(t *testing.T) {
	l := newLib(t)
	l.write("bien/a.mp3", tags("A", "x", "Uno", 1))
	l.write("rota/c.mp3", tags("C", "x", "Dos", 1))
	l.run()
	done := 0
	l.sc.OnDone = func() { done++ }

	l.sc.walkDir = failing("rota")
	if st := l.refresh(); st.Skipped != 1 || st.Tracks != 2 {
		t.Fatalf("with the folder unreadable: status = %+v", st)
	}
	l.sc.walkDir = nil
	if st := l.refresh(); st.Skipped != 0 || len(st.SkippedPaths) != 0 || st.Tracks != 2 {
		t.Fatalf("with the folder back: status = %+v", st)
	}
	if done != 0 {
		t.Fatalf("OnDone ran %d times for an unchanged catalog", done)
	}
}
