package stream

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain doubles as a fake ffmpeg: run with TORDO_FAKE_FFMPEG=1, the test
// binary prints its arguments instead of converting anything.
func TestMain(m *testing.M) {
	if os.Getenv("TORDO_FAKE_FFMPEG") == "1" {
		fmt.Print("ffmpeg " + strings.Join(os.Args[1:], " "))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeFFmpeg(t *testing.T) Streamer {
	t.Setenv("TORDO_FAKE_FFMPEG", "1")
	return Streamer{FFmpeg: os.Args[0]}
}

func TestPlan(t *testing.T) {
	with, without := Streamer{FFmpeg: "ffmpeg"}, Streamer{}
	cases := []struct {
		f    Format
		mode Mode
		mime string
	}{
		{Format{"mp3", "mp3", false}, Direct, "audio/mpeg"},
		{Format{"flac", "flac", true}, Direct, "audio/flac"},
		{Format{"ogg", "opus", false}, Direct, "audio/ogg; codecs=opus"},
		{Format{"ogg", "vorbis", false}, Direct, "audio/ogg; codecs=vorbis"},
		{Format{"mp4", "aac", false}, Direct, "audio/mp4"},
		{Format{"mp4", "flac", true}, Direct, "audio/mp4; codecs=flac"},
		{Format{"wav", "pcm", true}, Direct, "audio/wav"},
		{Format{"wav", "pcm_s24le", true}, Direct, "audio/wav"}, // ffprobe's name
		{Format{"mp4", "alac", true}, Transcode, "audio/flac"},
		{Format{"asf", "wma", false}, Transcode, "audio/mpeg"},
		{Format{"asf", "wmalossless", true}, Transcode, "audio/flac"},
		{Format{"aiff", "pcm", true}, Transcode, "audio/flac"},
		{Format{"ape", "ape", true}, Transcode, "audio/flac"},
	}
	for _, c := range cases {
		if mode, mime := with.Plan(c.f); mode != c.mode || mime != c.mime {
			t.Errorf("%+v: %v %q, want %v %q", c.f, mode, mime, c.mode, c.mime)
		}
		if c.mode == Transcode {
			if mode, _ := without.Plan(c.f); mode != Unplayable {
				t.Errorf("%+v without ffmpeg: %v", c.f, mode)
			}
		}
	}
}

func audioFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.mp3")
	if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func serve(s Streamer, path string, f Format, transcode bool, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/stream/1", nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	s.Serve(rec, req, path, f, transcode)
	return rec
}

func TestServeDirectWithRange(t *testing.T) {
	p := audioFile(t)
	mp3 := Format{"mp3", "mp3", false}
	rec := serve(Streamer{}, p, mp3, false)
	if rec.Code != http.StatusOK || rec.Body.String() != "0123456789" || rec.Header().Get("Content-Type") != "audio/mpeg" ||
		rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("whole file: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	rec = serve(Streamer{}, p, mp3, false, "Range", "bytes=2-5")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "2345" || rec.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if rec := serve(Streamer{}, filepath.Join(t.TempDir(), "gone.mp3"), mp3, false); rec.Code != http.StatusNotFound {
		t.Fatalf("missing file: %d", rec.Code)
	}
}

func TestServeTranscodes(t *testing.T) {
	p := audioFile(t)
	s := fakeFFmpeg(t)
	rec := serve(s, p, Format{"asf", "wma", false}, false)
	want := "ffmpeg -nostdin -v error -i " + p + " -map 0:a:0 -vn -c:a libmp3lame -q:a 2 -f mp3 pipe:1"
	if rec.Code != http.StatusOK || rec.Body.String() != want || rec.Header().Get("Content-Type") != "audio/mpeg" ||
		rec.Header().Get("Accept-Ranges") != "" {
		t.Fatalf("wma: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	rec = serve(s, p, Format{"mp4", "alac", true}, false)
	if !strings.Contains(rec.Body.String(), "-c:a flac -compression_level 0 -f flac pipe:1") || rec.Header().Get("Content-Type") != "audio/flac" {
		t.Fatalf("alac: %q %v", rec.Body, rec.Header())
	}
	rec = serve(s, p, Format{"ogg", "opus", false}, true) // the browser cannot play Ogg
	if !strings.Contains(rec.Body.String(), "libmp3lame") {
		t.Fatalf("asked conversion: %q", rec.Body)
	}
}

func TestServeWithoutFFmpeg(t *testing.T) {
	p := audioFile(t)
	if rec := serve(Streamer{}, p, Format{"asf", "wma", false}, false); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wma without ffmpeg: %d", rec.Code)
	}
	// Asking for a conversion without ffmpeg sends the file as it is.
	if rec := serve(Streamer{}, p, Format{"ogg", "opus", false}, true); rec.Code != http.StatusOK || rec.Body.String() != "0123456789" {
		t.Fatalf("opus: %d %q", rec.Code, rec.Body)
	}
}
