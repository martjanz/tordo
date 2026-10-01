package audiofile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadNativeDispatch(t *testing.T) {
	wma, _ := wmaFile(make([]byte, 100))
	files := map[string][]byte{
		"a.mp3":  cat(id3Tag(3, id3TextFrame(3, "TIT2", "mp3")), mp3Frames(3, 0x11)),
		"b.FLAC": flacFile(make([]byte, 10)),
		"c.m4a":  m4aFile(true, make([]byte, 10)),
		"d.wma":  wma,
		"e.wav":  wavFile(),
		"f.aiff": aiffFile(),
		"g.opus": cat(mkOggPage(2, 0, 1, 0, opusHead(2, 0)), mkOggPage(0, 0, 1, 1, cat([]byte("OpusTags"), vorbisComment("v"))), mkOggPage(4, 48000, 1, 2, []byte{1})),
	}
	want := map[string]string{"a.mp3": "mp3", "b.FLAC": "flac", "c.m4a": "mp4", "d.wma": "asf", "e.wav": "wav", "f.aiff": "aiff", "g.opus": "ogg"}
	for name, data := range files {
		info, err := ReadNative(writeFile(t, name, data))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if info.Format != want[name] || info.Reader != "native" {
			t.Errorf("%s: Format = %q, Reader = %q", name, info.Format, info.Reader)
		}
	}
}

func TestReadNativeUnsupportedAndMissing(t *testing.T) {
	if _, err := ReadNative(writeFile(t, "x.ape", []byte("MAC "))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("ape: err = %v, want ErrUnsupported", err)
	}
	if _, err := ReadNative(filepath.Join(t.TempDir(), "missing.mp3")); !errors.Is(err, ErrIO) {
		t.Errorf("missing: err = %v, want ErrIO", err)
	}
}

func TestReadFallsBackToFFprobe(t *testing.T) {
	if !HasFFprobe() {
		t.Skip("ffprobe no está instalado")
	}
	// A WAV with a .ape extension: no native reader, ffprobe understands it.
	p := writeFile(t, "really-a-wav.ape", wavFile())
	info, err := Read(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Reader != "ffprobe" || info.Codec != "pcm_s16le" || !info.Lossless || info.DurationMs != 1000 {
		t.Errorf("info = %+v", info)
	}
	if info.Tags.Title != "Candombe" {
		t.Errorf("Title = %q", info.Tags.Title)
	}
	if info.Payload != (Range{0, int64(len(wavFile()))}) {
		t.Errorf("Payload = %+v (ffprobe: the whole file)", info.Payload)
	}
}
