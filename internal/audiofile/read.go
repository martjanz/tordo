package audiofile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read reads the audio file at path: natively when there is a reader for its
// extension; with ffprobe when the native reader does not understand it (or
// there is none) and ffprobe is installed. An ErrIO is returned as is: the
// file could not be read now, which ffprobe would not fix.
func Read(ctx context.Context, path string) (Info, error) {
	info, err := ReadNative(path)
	if err == nil || errors.Is(err, ErrIO) || !HasFFprobe() {
		return info, err
	}
	if fi, ferr := readFFprobe(ctx, path); ferr == nil {
		return fi, nil
	}
	return info, err
}

// ReadNative reads the file with the built-in readers only.
func ReadNative(path string) (Info, error) {
	var read func(source) (Info, error)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		read = readMP3
	case ".flac":
		read = readFLAC
	case ".ogg", ".oga", ".opus":
		read = readOgg
	case ".m4a", ".mp4":
		read = readMP4
	case ".wma", ".asf":
		read = readASF
	case ".wav":
		read = readWAV
	case ".aif", ".aiff", ".aifc":
		read = readAIFF
	default:
		return Info{}, ErrUnsupported
	}
	f, err := os.Open(path)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	info, err := read(source{f, st.Size()})
	info.Reader = "native"
	return info, err
}
