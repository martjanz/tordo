package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tordo/internal/audiofile"
)

const hada = "02 - La leyenda del hada y el mago.mp3"

// scanned is a library scanned once, counting what later rounds set off.
func scanned(t *testing.T) (*lib, *int) {
	t.Helper()
	l := newLib(t)
	l.sample()
	l.run()
	done := 0
	l.sc.OnDone = func() { done++ }
	return l, &done
}

func (l *lib) refresh() Status {
	l.t.Helper()
	if err := l.sc.Refresh(context.Background()); err != nil {
		l.t.Fatal(err)
	}
	return l.sc.Status()
}

func TestRefreshWithoutChangesStopsAfterTheWalk(t *testing.T) {
	l, done := scanned(t)
	l.sc.status.Finished = time.Time{} // the clock can be too coarse to compare two rounds
	st := l.refresh()
	if *done != 0 {
		t.Fatal("identification was set off for an unchanged disk")
	}
	if st.Tracks != 2 || st.Albums != 1 {
		t.Fatalf("status lost the catalog figures, which would reload the pages: %+v", st)
	}
	if st.Finished.IsZero() {
		t.Fatal("the round did not count as finished")
	}
}

func TestRefreshPicksUpANewFile(t *testing.T) {
	l, done := scanned(t)
	l.write(magos+"/03 - Otra.mp3", tags("Otra", "Rata Blanca", "Magos, Espadas y Rosas", 3))
	st := l.refresh()
	if st.ToRead != 1 || st.Tracks != 3 || *done != 1 {
		t.Fatalf("status = %+v, OnDone %d", st, *done)
	}
}

func TestRefreshPicksUpADeletedFile(t *testing.T) {
	l, done := scanned(t)
	if err := os.Remove(filepath.Join(l.root, magos, hada)); err != nil {
		t.Fatal(err)
	}
	st := l.refresh()
	if st.ToRead != 0 || st.Tracks != 1 || *done != 1 {
		t.Fatalf("status = %+v, OnDone %d", st, *done)
	}
}

// Corrections are kept by fingerprint, so a renamed folder has to keep them.
func TestRefreshPicksUpARenamedFolderKeepingTheFingerprints(t *testing.T) {
	l, done := scanned(t)
	fingerprints := func() map[string]bool {
		t.Helper()
		albums, err := l.st.Albums("")
		if err != nil || len(albums) != 1 {
			t.Fatalf("albums = %+v, err = %v", albums, err)
		}
		fps, err := l.st.AlbumFingerprints(albums[0].ID)
		if err != nil || len(fps) != 2 {
			t.Fatalf("fingerprints = %v, err = %v", fps, err)
		}
		set := map[string]bool{}
		for _, fp := range fps {
			set[fp] = true
		}
		return set
	}
	was := fingerprints()
	if err := os.Rename(filepath.Join(l.root, magos), filepath.Join(l.root, "Renombrada")); err != nil {
		t.Fatal(err)
	}
	st := l.refresh()
	if st.ToRead != 3 || st.Tracks != 2 || st.Albums != 1 || *done != 1 { // 3: the unreadable file too
		t.Fatalf("status = %+v, OnDone %d", st, *done)
	}
	for fp := range fingerprints() {
		if !was[fp] {
			t.Fatalf("fingerprint %s is new after the rename", fp)
		}
	}
}

func TestRefreshKeepsTheCatalogOfAnUnpluggedDrive(t *testing.T) {
	l, done := scanned(t)
	if err := os.Rename(l.root, l.root+"-desenchufado"); err != nil {
		t.Fatal(err)
	}
	st := l.refresh()
	if st.Tracks != 2 || st.Albums != 1 || *done != 0 {
		t.Fatalf("status = %+v, OnDone %d", st, *done)
	}
}

func TestRefreshRetriesAFileThatFailedToRead(t *testing.T) {
	l := newLib(t)
	l.sample()
	failing := true
	l.sc.Read = func(ctx context.Context, p string) (audiofile.Info, error) {
		if failing && filepath.Base(p) == "01 - Mujer amante.mp3" {
			return audiofile.Info{}, fmt.Errorf("%w: prueba", audiofile.ErrIO)
		}
		return fakeRead(ctx, p)
	}
	l.sc.Run(context.Background())
	failing = false
	if st := l.refresh(); st.ToRead != 1 || st.Tracks != 2 {
		t.Fatalf("status = %+v", st)
	}
}

// A scan cut short may have saved changes it never grouped: the disk matching
// the catalog does not mean the albums are current.
func TestRefreshFinishesWhatACancelledScanLeft(t *testing.T) {
	l, done := scanned(t)
	if err := os.Remove(filepath.Join(l.root, magos, hada)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.sc.afterSync = cancel
	l.sc.Run(ctx)
	l.sc.afterSync = nil
	st := l.refresh()
	if st.Tracks != 1 || *done != 1 {
		t.Fatalf("status = %+v, OnDone %d", st, *done)
	}
}

// A scanner that has not grouped yet (the settings just rebuilt it) does the
// whole job on its first round.
func TestFirstRefreshOfAScannerDoesTheWholeJob(t *testing.T) {
	l, _ := scanned(t)
	done := 0
	fresh := &Scanner{AppDir: l.sc.AppDir, Roots: l.sc.Roots, Store: l.st, Read: fakeRead, Env: l.sc.Env, OnDone: func() { done++ }}
	if err := fresh.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := fresh.Status(); st.Tracks != 2 || done != 1 {
		t.Fatalf("status = %+v, OnDone %d", st, done)
	}
}

// The status poll reloads the pages when a run ends, so checking the disk
// cannot look like a run.
func TestRefreshIsNotVisibleWhileItChecks(t *testing.T) {
	l, _ := scanned(t)
	before := l.sc.Status()
	checked := false
	l.sc.afterSync = func() {
		checked = true
		if st := l.sc.Status(); st.Running || st.Tracks != 2 || st.Files != before.Files {
			t.Errorf("status while checking = %+v", st)
		}
		if !l.sc.Busy() {
			t.Error("not busy while checking")
		}
		if err := l.sc.Run(context.Background()); !errors.Is(err, ErrBusy) {
			t.Errorf("Run while checking: err = %v, want ErrBusy", err)
		}
	}
	l.refresh()
	if !checked || l.sc.Busy() {
		t.Fatalf("checked = %v, busy after = %v", checked, l.sc.Busy())
	}
}

func TestRefreshBecomesVisibleOnceItFindsChanges(t *testing.T) {
	l, _ := scanned(t)
	l.write(magos+"/03 - Otra.mp3", tags("Otra", "Rata Blanca", "Magos, Espadas y Rosas", 3))
	running := false
	l.sc.Read = func(ctx context.Context, p string) (audiofile.Info, error) {
		running = l.sc.Status().Running
		return fakeRead(ctx, p)
	}
	if st := l.refresh(); !running || st.Running {
		t.Fatalf("running while reading = %v; status after = %+v", running, st)
	}
}

// A failed check must not read "the last scan failed" for ever: later rounds
// that end quietly are what clears it.
func TestQuietRefreshClearsAnEarlierError(t *testing.T) {
	l, _ := scanned(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.sc.Refresh(ctx); err == nil {
		t.Fatal("no error from a cancelled round")
	}
	if l.sc.Status().LastError == "" {
		t.Fatal("the failed round left no error")
	}
	if st := l.refresh(); st.LastError != "" {
		t.Fatalf("LastError = %q", st.LastError)
	}
}
