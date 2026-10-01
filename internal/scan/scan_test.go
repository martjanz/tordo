package scan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tordo/internal/audiofile"
	"tordo/internal/store"
)

// fakeRead treats a file's content as JSON tags ("BAD" is an unreadable format).
func fakeRead(ctx context.Context, p string) (audiofile.Info, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return audiofile.Info{}, fmt.Errorf("%w: %v", audiofile.ErrIO, err)
	}
	var tags audiofile.Tags
	if string(b) == "BAD" || json.Unmarshal(b, &tags) != nil {
		return audiofile.Info{}, fmt.Errorf("%w: prueba", audiofile.ErrFormat)
	}
	return audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: tags, Reader: "native",
		Payload: audiofile.Range{Offset: 0, Length: int64(len(b))}}, nil
}

type lib struct {
	t    *testing.T
	root string
	st   *store.Store
	sc   *Scanner
}

func newLib(t *testing.T) *lib {
	parent := t.TempDir()
	app, root := filepath.Join(parent, "tordo"), filepath.Join(parent, "musica")
	for _, d := range []string{app, root} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	sc := &Scanner{AppDir: app, Roots: []string{"../musica"}, Store: st, Read: fakeRead, Env: store.ReadEnv{Version: 1}}
	return &lib{t: t, root: root, st: st, sc: sc}
}

func (l *lib) write(rel, content string) {
	l.t.Helper()
	p := filepath.Join(l.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		l.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		l.t.Fatal(err)
	}
}

func tags(title, artist, album string, track int) string {
	b, _ := json.Marshal(audiofile.Tags{Title: title, Artist: artist, Album: album, Track: track})
	return string(b)
}

func (l *lib) run() Status {
	l.t.Helper()
	if err := l.sc.Run(context.Background()); err != nil {
		l.t.Fatal(err)
	}
	return l.sc.Status()
}

const magos = "Rata Blanca - (1990) Magos"

func (l *lib) sample() {
	l.write(magos+"/01 - Mujer amante.mp3", tags("Mujer amante", "Rata Blanca", "Magos, Espadas y Rosas", 1))
	l.write(magos+"/02 - La leyenda del hada y el mago.mp3", tags("La leyenda del hada y el mago", "Rata Blanca", "Magos, Espadas y Rosas", 2))
	l.write(magos+"/cover.jpg", "jpeg")
	l.write(magos+"/roto.mp3", "BAD")
	l.write("Ableton/sample.mp3", tags("x", "x", "x", 1))
	l.write("Soulseek/downloading/half.mp3", tags("x", "x", "x", 1))
	l.write(".hidden/y.mp3", tags("y", "y", "y", 1))
	l.write("libro.epub", "epub")
}

func TestScanBuildsCatalog(t *testing.T) {
	l := newLib(t)
	l.sample()
	st := l.run()
	if st.Running || st.Files != 4 || st.ToRead != 3 || st.Read != 3 || st.Tracks != 2 || st.Albums != 1 || st.LastError != "" {
		t.Fatalf("status = %+v", st)
	}
	c, _ := l.st.Counts()
	if c.Audio != 3 || c.Unreadable != 1 || c.Tracks != 2 || c.Albums != 1 {
		t.Fatalf("counts = %+v", c)
	}
	albums, _ := l.st.Albums("")
	if len(albums) != 1 || albums[0].Title != "Magos, Espadas y Rosas" || albums[0].CoverPath != "../musica/"+magos+"/cover.jpg" {
		t.Fatalf("albums = %+v", albums)
	}
}

func TestRescanIsIncremental(t *testing.T) {
	l := newLib(t)
	l.sample()
	l.run()
	if st := l.run(); st.ToRead != 0 {
		t.Fatalf("second scan reads %d files, want 0", st.ToRead)
	}
	p := magos + "/01 - Mujer amante.mp3"
	l.write(p, tags("Mujer amante (remaster)", "Rata Blanca", "Magos, Espadas y Rosas", 1))
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(l.root, filepath.FromSlash(p)), later, later); err != nil {
		t.Fatal(err)
	}
	if st := l.run(); st.ToRead != 1 {
		t.Fatalf("after a change, reads %d files, want 1", st.ToRead)
	}
	albums, _ := l.st.Albums("")
	d, _, _ := l.st.Album(albums[0].ID)
	if d.Tracks[0].Title != "Mujer amante (remaster)" {
		t.Fatalf("title = %q", d.Tracks[0].Title)
	}
}

func TestDeletedFileLeavesTheCatalog(t *testing.T) {
	l := newLib(t)
	l.sample()
	l.run()
	if err := os.Remove(filepath.Join(l.root, magos, "02 - La leyenda del hada y el mago.mp3")); err != nil {
		t.Fatal(err)
	}
	if st := l.run(); st.Tracks != 1 {
		t.Fatalf("tracks = %d, want 1", st.Tracks)
	}
}

func TestUnavailableRootKeepsTheCatalog(t *testing.T) {
	l := newLib(t)
	l.sample()
	l.run()
	if err := os.Rename(l.root, l.root+"-desenchufado"); err != nil {
		t.Fatal(err)
	}
	if st := l.run(); st.Tracks != 2 || st.Albums != 1 {
		t.Fatalf("status = %+v, want the previous catalog", st)
	}
}

func TestBusy(t *testing.T) {
	l := newLib(t)
	l.sample()
	started, release := make(chan struct{}), make(chan struct{})
	l.sc.Read = func(ctx context.Context, p string) (audiofile.Info, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		return fakeRead(ctx, p)
	}
	done := make(chan error)
	go func() { done <- l.sc.Run(context.Background()) }()
	<-started
	if err := l.sc.Run(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
