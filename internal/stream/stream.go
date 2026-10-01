// Package stream delivers a track's audio to the browser: the file as it is,
// with Range requests (seeking), when browsers play its format; converted on
// the fly by ffmpeg otherwise (no seeking). It only reads the file.
package stream

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Format is what deciding how to deliver a track needs.
type Format struct {
	Container string // audiofile.Info.Format: mp3, flac, ogg, mp4, asf, wav, aiff, or ffprobe's name
	Codec     string
	Lossless  bool
}

// Mode is how a track reaches the browser.
type Mode int

const (
	Direct     Mode = iota // the file as it is
	Transcode              // converted by ffmpeg
	Unplayable             // needs ffmpeg, which is not installed
)

// ErrUnplayable means the track needs ffmpeg and there is none.
var ErrUnplayable = errors.New("este formato necesita ffmpeg para reproducirse")

// Streamer serves tracks. FFmpeg is the path to ffmpeg ("" when there is
// none); see LookFFmpeg.
type Streamer struct {
	FFmpeg string
}

var (
	ffmpegOnce sync.Once
	ffmpegPath string
)

// LookFFmpeg returns the path to ffmpeg on the PATH, or "" (looked up once).
func LookFFmpeg() string {
	ffmpegOnce.Do(func() { ffmpegPath, _ = exec.LookPath("ffmpeg") })
	return ffmpegPath
}

// directMIME is the MIME type of the formats browsers play, by container and
// codec ("" for those they don't). Browsers that lack one of them (Safari
// and Ogg, for instance) ask for a conversion instead: see Serve.
func directMIME(f Format) string {
	switch f.Container {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		if f.Codec == "flac" {
			return "audio/flac"
		}
	case "ogg":
		switch f.Codec {
		case "opus", "vorbis":
			return "audio/ogg; codecs=" + f.Codec
		}
	case "mp4", "mov":
		switch f.Codec {
		case "aac":
			return "audio/mp4"
		case "flac", "opus", "mp3":
			return "audio/mp4; codecs=" + f.Codec
		}
	case "wav":
		if strings.HasPrefix(f.Codec, "pcm") {
			return "audio/wav"
		}
	}
	return ""
}

// transcodeMIME is what a conversion produces: FLAC for lossless sources (no
// quality lost), MP3 for the rest.
func transcodeMIME(f Format) string {
	if f.Lossless {
		return "audio/flac"
	}
	return "audio/mpeg"
}

// Plan says how f is delivered, and with which MIME type.
func (s Streamer) Plan(f Format) (Mode, string) {
	if m := directMIME(f); m != "" {
		return Direct, m
	}
	if s.FFmpeg == "" {
		return Unplayable, ""
	}
	return Transcode, transcodeMIME(f)
}

// Serve writes the audio of the file at path. transcode asks for a
// conversion even of a format that is usually played as it is (the browser
// said it cannot play it). Without ffmpeg, a format that needs it gets 415.
func (s Streamer) Serve(w http.ResponseWriter, r *http.Request, path string, f Format, transcode bool) {
	mode, mime := s.Plan(f)
	if transcode && mode == Direct && s.FFmpeg != "" {
		mode, mime = Transcode, transcodeMIME(f)
	}
	switch mode {
	case Unplayable:
		http.Error(w, ErrUnplayable.Error(), http.StatusUnsupportedMediaType)
	case Direct:
		serveFile(w, r, path, mime)
	case Transcode:
		s.transcode(w, r, path, f, mime)
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, path, mime string) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("audio: %v", err)
		http.Error(w, "no se pudo abrir el archivo", http.StatusNotFound)
		return
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		log.Printf("audio: %v", err)
		http.Error(w, "no se pudo abrir el archivo", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", mime)
	http.ServeContent(w, r, "", st.ModTime(), file)
}

// transcode runs ffmpeg and streams its output. Without a length the
// browser cannot seek; stopping playback cancels the request, which kills
// ffmpeg.
func (s Streamer) transcode(w http.ResponseWriter, r *http.Request, path string, f Format, mime string) {
	if _, err := os.Stat(path); err != nil {
		log.Printf("audio: %v", err)
		http.Error(w, "no se pudo abrir el archivo", http.StatusNotFound)
		return
	}
	args := []string{"-nostdin", "-v", "error", "-i", path, "-map", "0:a:0", "-vn"}
	if mime == "audio/flac" {
		args = append(args, "-c:a", "flac", "-compression_level", "0", "-f", "flac", "pipe:1")
	} else {
		args = append(args, "-c:a", "libmp3lame", "-q:a", "2", "-f", "mp3", "pipe:1")
	}
	cmd := exec.CommandContext(r.Context(), s.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = w, &stderr
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "no-store")
	if err := cmd.Run(); err != nil && r.Context().Err() == nil {
		log.Printf("ffmpeg (%s, %s): %v: %s", path, f.Codec, err, strings.TrimSpace(stderr.String()))
	}
}
