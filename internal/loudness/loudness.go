// Package loudness gives tracks a gain toward a common loudness, from their
// ReplayGain tags or measured with ffmpeg's EBU R128 filter. It only reads
// the audio files.
package loudness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Reference is the loudness tracks are brought to, in LUFS: ReplayGain 2's.
const Reference = -18.0

// Floor is the quietest loudness taken as a measure: at or below it
// (silence, a broken decode) there is nothing to normalize.
const Floor = -70.0

// ErrNoMeasure means ffmpeg gave no usable integrated loudness.
var ErrNoMeasure = errors.New("ffmpeg no midió el volumen")

// Gain is the gain in dB that brings a track of this loudness to Reference.
func Gain(lufs float64) float64 { return Reference - lufs }

// FromGain is the loudness a ReplayGain gain implies.
func FromGain(gain float64) float64 { return Reference - gain }

// Part is a track of an album: its loudness and length.
type Part struct {
	LUFS       float64
	DurationMs int64
}

// AlbumLoudness is the loudness of tracks played in a row: the mean of
// their energy, weighted by duration (EBU R128 integrates energy, not
// LUFS values). false when there are no parts.
func AlbumLoudness(parts []Part) (float64, bool) {
	var energy float64
	var total int64
	for _, p := range parts {
		d := max(p.DurationMs, 1)
		energy += float64(d) * math.Pow(10, p.LUFS/10)
		total += d
	}
	if total == 0 {
		return 0, false
	}
	return 10 * math.Log10(energy/float64(total)), true
}

var integratedRe = regexp.MustCompile(`(?m)^\s*I:\s*(-inf|-?[0-9]+(?:\.[0-9]+)?)\s*LUFS`)

// ParseEBUR128 reads the integrated loudness from the summary ffmpeg's
// ebur128 filter writes at the end.
func ParseEBUR128(out string) (float64, error) {
	i := strings.LastIndex(out, "Summary:")
	if i < 0 {
		return 0, ErrNoMeasure
	}
	m := integratedRe.FindStringSubmatch(out[i:])
	if m == nil || m[1] == "-inf" {
		return 0, ErrNoMeasure
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || v <= Floor {
		return 0, ErrNoMeasure
	}
	return v, nil
}

// Analyze measures a file's integrated loudness with ffmpeg: its first audio
// stream, one thread, nothing written.
func Analyze(ctx context.Context, ffmpeg, file string) (float64, error) {
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostats", "-hide_banner", "-nostdin", "-threads", "1",
		"-i", file, "-map", "0:a:0", "-af", "ebur128=framelog=quiet", "-f", "null", "-")
	var out bytes.Buffer
	cmd.Stderr = &out
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	v, err := ParseEBUR128(out.String())
	if err != nil && runErr != nil {
		return 0, fmt.Errorf("%w: %s", ErrNoMeasure, lastLine(out.String()))
	}
	return v, err
}

// lastLine is the last non-empty line of ffmpeg's output: usually the error.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Job is a track to measure: its file id, its fingerprint ref and its path
// relative to the app folder.
type Job struct {
	ID   int64
	Ref  string
	Path string
}

// Track is what a track's gains come from (see the store's LoudnessFacts).
type Track struct {
	AlbumID    int64
	DurationMs int64
	TagTrack   *float64 // ReplayGain tags, dB
	TagAlbum   *float64
	LUFS       *float64 // measured by Analyze
}

// TrackGain is a track's gains in dB, nil when unknown.
type TrackGain struct {
	Track *float64 `json:"trackGain,omitempty"`
	Album *float64 `json:"albumGain,omitempty"`
}

// loudnessOf is a track's known loudness: from its track gain tag, else
// measured.
func loudnessOf(t Track) (float64, bool) {
	if t.TagTrack != nil {
		return FromGain(*t.TagTrack), true
	}
	if t.LUFS != nil {
		return *t.LUFS, true
	}
	return 0, false
}

func round(v float64) *float64 {
	r := math.Round(v*100) / 100
	return &r
}

// Gains computes every track's gains. facts must hold every track of the
// albums involved: an album gain without tags needs all of them measured.
func Gains(facts map[int64]Track) map[int64]TrackGain {
	albums := map[int64][]Part{}
	unknown := map[int64]bool{}
	for _, t := range facts {
		if t.AlbumID == 0 {
			continue
		}
		if l, ok := loudnessOf(t); ok {
			albums[t.AlbumID] = append(albums[t.AlbumID], Part{l, t.DurationMs})
		} else {
			unknown[t.AlbumID] = true
		}
	}
	out := make(map[int64]TrackGain, len(facts))
	for id, t := range facts {
		var g TrackGain
		if l, ok := loudnessOf(t); ok {
			g.Track = round(Gain(l))
		}
		switch {
		case t.TagAlbum != nil:
			g.Album = round(*t.TagAlbum)
		case t.AlbumID != 0 && !unknown[t.AlbumID]:
			if l, ok := AlbumLoudness(albums[t.AlbumID]); ok {
				g.Album = round(Gain(l))
			}
		}
		out[id] = g
	}
	return out
}
