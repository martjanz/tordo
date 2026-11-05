package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tordo/internal/config"
)

func withRescan(minutes int) config.Config {
	cfg := cfgOf("../musica")
	cfg.RescanMinutes = &minutes
	return cfg
}

// addTrack puts a new track on the disk.
func addTrack(t *testing.T, e *Engine) {
	t.Helper()
	p := filepath.Join(filepath.Dir(e.AppDir), "musica", "Signos", "02.mp3")
	if err := os.WriteFile(p, mp3("nuevo"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitTracks(t *testing.T, e *Engine, want int) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); tracks(t, e) != want; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("tracks = %d, want %d", tracks(t, e), want)
		}
	}
}

// stillOneTrack fails if a round started when it should not have.
func stillOneTrack(t *testing.T, e *Engine, why string) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	if tracks(t, e) != 1 {
		t.Fatal(why)
	}
}

func TestRescanRunsOnceTheIntervalPasses(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(withRescan(30), false)
	waitScan(t, e)
	addTrack(t, e)
	base := time.Now()
	e.rescanCheck(base)
	e.rescanCheck(base.Add(29 * time.Minute))
	stillOneTrack(t, e, "rescanned before the interval")
	e.rescanCheck(base.Add(31 * time.Minute))
	waitTracks(t, e, 2)
}

func TestRescanDefaultsToAnHour(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(cfgOf("../musica"), false)
	waitScan(t, e)
	addTrack(t, e)
	base := time.Now()
	e.rescanCheck(base)
	e.rescanCheck(base.Add(59 * time.Minute))
	stillOneTrack(t, e, "rescanned before the hour")
	e.rescanCheck(base.Add(61 * time.Minute))
	waitTracks(t, e, 2)
}

func TestRescanOff(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(withRescan(0), false)
	waitScan(t, e)
	addTrack(t, e)
	base := time.Now()
	e.rescanCheck(base)
	e.rescanCheck(base.Add(1000 * time.Hour))
	stillOneTrack(t, e, "rescanned while off")
}

func TestRescanWaitsForTheSetup(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(cfgOf("../musica"), true)
	base := time.Now()
	e.rescanCheck(base)
	e.rescanCheck(base.Add(1000 * time.Hour))
	time.Sleep(100 * time.Millisecond)
	if tracks(t, e) != 0 {
		t.Fatal("rescanned while the setup was pending")
	}
}

func TestRescanCountsFromTheLastScan(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(withRescan(30), false)
	waitScan(t, e)
	finished := e.Current().Scanner.Status().Finished
	e.rescanCheck(finished.Add(-time.Hour))
	addTrack(t, e)
	e.rescanCheck(finished.Add(29 * time.Minute))
	stillOneTrack(t, e, "rescanned less than the interval after the last scan")
}

func TestChangingTheIntervalKeepsThePartsAndApplies(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(withRescan(0), false)
	waitScan(t, e)
	old := e.Current()
	base := time.Now()
	e.rescanCheck(base)
	if err := e.Apply(withRescan(30)); err != nil {
		t.Fatal(err)
	}
	if rt := e.Current(); rt.Scanner != old.Scanner || rt.Identifier != old.Identifier {
		t.Fatal("changing the interval rebuilt the parts")
	}
	addTrack(t, e)
	e.rescanCheck(base.Add(31 * time.Minute))
	waitTracks(t, e, 2)
}

func TestChangingTheIntervalDoesNotScanAtOnce(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(withRescan(30), false)
	waitScan(t, e)
	finished := e.Current().Scanner.Status().Finished
	if err := e.Apply(withRescan(45)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := e.Current().Scanner.Status().Finished; !got.Equal(finished) {
		t.Fatal("changing the interval scanned at once")
	}
}
