package audiofile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCorpus compares the native readers with ffprobe over a real collection:
//
//	TORDO_CORPUS=/Users/martin/Music go test ./internal/audiofile -run Corpus -v -timeout 0
//
// It fails when the native readers reject more than 1% of the files ffprobe
// reads; differences in tags or duration are logged for review.
func TestCorpus(t *testing.T) {
	root := os.Getenv("TORDO_CORPUS")
	if root == "" {
		t.Skip("TORDO_CORPUS no definido")
	}
	if !HasFFprobe() {
		t.Skip("hace falta ffprobe")
	}
	var files, failed, differ int
	firstArtist := func(s string) string {
		s, _, _ = strings.Cut(s, ";")
		return strings.TrimSpace(s)
	}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".mp3", ".flac", ".ogg", ".oga", ".opus", ".m4a", ".wma", ".wav", ".aif", ".aiff":
		default:
			return nil
		}
		want, ferr := readFFprobe(context.Background(), p)
		if ferr != nil {
			return nil // ffprobe cannot read it either
		}
		files++
		got, err := ReadNative(p)
		if err != nil {
			failed++
			t.Logf("FALLA %s: %v", p, err)
			return nil
		}
		var diffs []string
		if got.Tags.Title != want.Tags.Title {
			diffs = append(diffs, fmt.Sprintf("título %q ≠ %q", got.Tags.Title, want.Tags.Title))
		}
		if firstArtist(got.Tags.Artist) != firstArtist(want.Tags.Artist) {
			diffs = append(diffs, fmt.Sprintf("artista %q ≠ %q", got.Tags.Artist, want.Tags.Artist))
		}
		if got.Tags.Album != want.Tags.Album {
			diffs = append(diffs, fmt.Sprintf("álbum %q ≠ %q", got.Tags.Album, want.Tags.Album))
		}
		if dd := got.DurationMs - want.DurationMs; dd > max(1500, want.DurationMs/100) || -dd > max(1500, want.DurationMs/100) {
			diffs = append(diffs, fmt.Sprintf("duración %d ≠ %d ms", got.DurationMs, want.DurationMs))
		}
		if len(diffs) > 0 {
			differ++
			if differ <= 300 {
				t.Logf("DIFIERE %s: %s", p, strings.Join(diffs, "; "))
			}
		}
		return nil
	})
	t.Logf("%d archivos; %d rechazados por el lector propio; %d con diferencias", files, failed, differ)
	if files > 0 && failed*100 > files {
		t.Errorf("el lector propio rechaza más del 1%% de los archivos")
	}
}

// TestCorpusPictures checks that Picture extracts the embedded picture of
// every file the readers saw one in:
//
//	TORDO_CORPUS=/Users/martin/Music go test ./internal/audiofile -run CorpusPictures -v -timeout 0
func TestCorpusPictures(t *testing.T) {
	root := os.Getenv("TORDO_CORPUS")
	if root == "" {
		t.Skip("TORDO_CORPUS no definido")
	}
	var withCover, empty, failed int
	mimes := map[string]int{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || nativeReader(p) == nil {
			return nil
		}
		info, err := ReadNative(p)
		if err != nil || !info.Tags.HasCover {
			return nil
		}
		withCover++
		_, mime, err := Picture(p)
		if errors.Is(err, ErrNoPicture) {
			empty++ // an empty or linked picture
			return nil
		}
		if err != nil {
			failed++
			t.Logf("SIN IMAGEN %s: %v", p, err)
			return nil
		}
		mimes[mime]++
		return nil
	})
	t.Logf("%d archivos con imagen; %d vacías o enlazadas; %d sin extraer; tipos %v", withCover, empty, failed, mimes)
	if withCover > 0 && failed*100 > withCover {
		t.Errorf("no se extrae la imagen de más del 1%% de los archivos que la tienen")
	}
}
