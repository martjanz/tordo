package loudness

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// summary is ffmpeg 9's ebur128 output for a 440 Hz tone, trimmed.
const summary = `  Stream #0:0: Audio: pcm_s16le, 44100 Hz, mono, s16, 705 kb/s
[Parsed_ebur128_0 @ 0x126005490] Summary:

  Integrated loudness:
    I:         -22.2 LUFS
    Threshold: -32.2 LUFS

  Loudness range:
    LRA:        20.0 LU
    Threshold: -42.2 LUFS
    LRA low:   -42.2 LUFS
    LRA high:  -22.2 LUFS
[out#0/null @ 0x12600b930] video:0KiB audio:258KiB subtitle:0KiB other streams:0KiB global headers:0KiB muxing overhead: unknown
`

func TestParseEBUR128(t *testing.T) {
	if v, err := ParseEBUR128(summary); err != nil || v != -22.2 {
		t.Fatalf("got %v, %v", v, err)
	}
	for name, out := range map[string]string{
		"no summary": "    I:         -22.2 LUFS\n",
		"silence":    "Summary:\n\n  Integrated loudness:\n    I:         -70.0 LUFS\n",
		"inf":        "Summary:\n    I:         -inf LUFS\n",
		"empty":      "",
	} {
		if _, err := ParseEBUR128(out); err != ErrNoMeasure {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestGainMath(t *testing.T) {
	if g := Gain(-10); g != -8 {
		t.Errorf("Gain(-10) = %v", g)
	}
	if l := FromGain(-8); l != -10 {
		t.Errorf("FromGain(-8) = %v", l)
	}
	if _, ok := AlbumLoudness(nil); ok {
		t.Error("no parts: no loudness")
	}
	// Two equal halves keep their loudness; a loud track dominates a quiet
	// one of the same length (energy, not the mean of the LUFS values).
	if l, _ := AlbumLoudness([]Part{{-12, 1000}, {-12, 1000}}); math.Abs(l+12) > 1e-9 {
		t.Errorf("equal = %v", l)
	}
	l, _ := AlbumLoudness([]Part{{-10, 1000}, {-30, 1000}})
	if math.Abs(l-(-12.97)) > 0.01 {
		t.Errorf("mixed = %v, want ≈ -12.97", l)
	}
	// Longer tracks weigh more.
	if l, _ := AlbumLoudness([]Part{{-10, 3000}, {-30, 1000}}); l < -11.3 || l > -11.2 {
		t.Errorf("weighted = %v", l)
	}
}

func f(v float64) *float64 { return &v }

func TestGains(t *testing.T) {
	facts := map[int64]Track{
		1: {AlbumID: 7, DurationMs: 1000, TagTrack: f(-6), TagAlbum: f(-5)}, // tags win
		2: {AlbumID: 8, DurationMs: 1000, LUFS: f(-10)},
		3: {AlbumID: 8, DurationMs: 1000, LUFS: f(-10)},
		4: {AlbumID: 9, DurationMs: 1000, LUFS: f(-10)},
		5: {AlbumID: 9, DurationMs: 1000},   // not measured yet: album 9 has no album gain
		6: {DurationMs: 1000, LUFS: f(-14)}, // a loose track
		7: {AlbumID: 10, DurationMs: 1000, TagTrack: f(-4)},
		8: {AlbumID: 10, DurationMs: 1000, LUFS: f(-14)}, // a tag track gain counts as a measure
	}
	g := Gains(facts)
	check := func(id int64, track, album *float64) {
		t.Helper()
		got := g[id]
		eq := func(a, b *float64) bool { return (a == nil) == (b == nil) && (a == nil || math.Abs(*a-*b) < 0.006) }
		if !eq(got.Track, track) || !eq(got.Album, album) {
			t.Errorf("%d: got %+v", id, got)
		}
	}
	check(1, f(-6), f(-5))
	check(2, f(-8), f(-8))
	check(4, f(-8), nil)
	check(5, nil, nil)
	check(6, f(-4), nil)
	check(7, f(-4), f(-4))
}

// TestAnalyze runs the real ffmpeg on a generated tone, when there is one.
func TestAnalyze(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("sin ffmpeg")
	}
	file := filepath.Join(t.TempDir(), "tone.wav")
	if out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi",
		"-i", "sine=frequency=440:duration=3", "-y", file).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	v, err := Analyze(context.Background(), ffmpeg, file)
	if err != nil || v > -15 || v < -30 {
		t.Fatalf("got %v, %v", v, err)
	}
	if err := os.WriteFile(file, []byte("not audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Analyze(context.Background(), ffmpeg, file); err == nil {
		t.Fatal("a broken file has no loudness")
	}
}
