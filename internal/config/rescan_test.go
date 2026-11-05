package config

import (
	"testing"
	"time"
)

func TestRescanEvery(t *testing.T) {
	n := func(v int) *int { return &v }
	for _, c := range []struct {
		minutes *int
		want    time.Duration
	}{{nil, time.Hour}, {n(0), 0}, {n(-5), 0}, {n(15), 15 * time.Minute}, {n(1440), 24 * time.Hour}} {
		if got := (Config{RescanMinutes: c.minutes}).RescanEvery(); got != c.want {
			t.Errorf("RescanMinutes %v: %v, want %v", c.minutes, got, c.want)
		}
	}
}

// Off (0) has to survive a save: it is not the same as absent (an hour).
func TestRescanOffRoundTrip(t *testing.T) {
	dir := t.TempDir()
	off := 0
	if err := Save(dir, Config{Roots: []string{}, Ignore: []string{}, RescanMinutes: &off}); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(dir)
	if err != nil || cfg.RescanEvery() != 0 {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}
