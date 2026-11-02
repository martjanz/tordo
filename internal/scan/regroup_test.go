package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tordo/internal/audiofile"
)

// A long read regroups the catalog as it goes: albums read early show up
// before the last file is read.
func TestReadingRegroupsAsItGoes(t *testing.T) {
	l := newLib(t)
	l.sc.GroupEvery = time.Nanosecond
	l.write("A/01.mp3", tags("Uno", "Alfa", "Primero", 1))
	l.write("B/01.mp3", tags("Dos", "Beta", "Segundo", 1))
	albumsAtB := -1
	l.sc.Read = func(ctx context.Context, p string) (audiofile.Info, error) {
		if strings.Contains(filepath.ToSlash(p), "/B/") {
			albumsAtB = l.sc.Status().Albums
		}
		return slowRead(ctx, p)
	}
	if st := l.run(); st.Albums != 2 {
		t.Fatalf("status = %+v", st)
	}
	if albumsAtB != 1 {
		t.Fatalf("albums when reading B = %d, want 1", albumsAtB)
	}
}

// A regroup while reading keeps the albums of changed files not read yet,
// with their ids, from what was read before.
func TestRegroupWhileReadingKeepsChangedAlbums(t *testing.T) {
	l := newLib(t)
	l.write("B/01.mp3", tags("Dos", "Beta", "Segundo", 1))
	l.run()
	before, _ := l.st.Albums("")
	l.sc.GroupEvery = time.Nanosecond
	l.write("A/01.mp3", tags("Uno", "Alfa", "Primero", 1))
	p := filepath.Join(l.root, "B", "01.mp3")
	l.write("B/01.mp3", tags("Dos (remaster)", "Beta", "Segundo", 1))
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	var atB []int64
	l.sc.Read = func(ctx context.Context, path string) (audiofile.Info, error) {
		if strings.Contains(filepath.ToSlash(path), "/B/") {
			albums, _ := l.st.Albums("")
			for _, a := range albums {
				atB = append(atB, a.ID)
			}
		}
		return slowRead(ctx, path)
	}
	l.run()
	if len(atB) != 2 || (atB[0] != before[0].ID && atB[1] != before[0].ID) {
		t.Fatalf("albums when reading B = %v, want 2 with id %d", atB, before[0].ID)
	}
}

// slowRead is fakeRead taking long enough for the clock to move (it is coarse
// on Windows), so a GroupEvery of 1 ns regroups after every file.
func slowRead(ctx context.Context, p string) (audiofile.Info, error) {
	time.Sleep(20 * time.Millisecond)
	return fakeRead(ctx, p)
}

// While reading, the status estimates the time left from the pace so far,
// once there is enough of it to go by.
func TestReadingEstimatesTimeLeft(t *testing.T) {
	l := newLib(t)
	for i := range 4 {
		l.write(fmt.Sprintf("A/%02d.mp3", i+1), tags("x", "Alfa", "Primero", i+1))
	}
	clock := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	l.sc.now = func() time.Time { return clock }
	var etas []int
	l.sc.Read = func(ctx context.Context, p string) (audiofile.Info, error) {
		etas = append(etas, l.sc.Status().EtaSeconds)
		clock = clock.Add(20 * time.Second)
		return fakeRead(ctx, p)
	}
	l.sc.etaAfter = 2
	if st := l.run(); st.EtaSeconds != 0 {
		t.Fatalf("eta after the scan = %d, want 0", st.EtaSeconds)
	}
	// Reading the 3rd file: 2 read in 40 s, 2 left. The 4th: 3 in 60 s, 1 left.
	if want := []int{0, 0, 40, 20}; fmt.Sprint(etas) != fmt.Sprint(want) {
		t.Fatalf("etas = %v, want %v", etas, want)
	}
}
