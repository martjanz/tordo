// Package audiofile reads the tags and technical data of audio files in one
// pass: natively for MP3, FLAC, Ogg (Opus, Vorbis), MP4/M4A, ASF/WMA, WAV and
// AIFF, and with ffprobe (when installed) for anything else. It only reads.
package audiofile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Version identifies the readers' behavior: files read by an older version
// are read again on the next scan.
const Version = 1

var (
	ErrUnsupported = errors.New("formato sin lector propio")
	ErrFormat      = errors.New("formato inválido")
	ErrIO          = errors.New("error de lectura")
)

// Tags are the metadata the catalog uses, whatever the format called them.
type Tags struct {
	Title         string   `json:"title,omitempty"`
	Artist        string   `json:"artist,omitempty"` // several artists joined with "; "
	AlbumArtist   string   `json:"albumArtist,omitempty"`
	Album         string   `json:"album,omitempty"`
	Track         int      `json:"track,omitempty"`
	TrackTotal    int      `json:"trackTotal,omitempty"`
	Disc          int      `json:"disc,omitempty"`
	DiscTotal     int      `json:"discTotal,omitempty"`
	Year          int      `json:"year,omitempty"`
	OriginalYear  int      `json:"originalYear,omitempty"`
	Genres        []string `json:"genres,omitempty"` // raw values, normalized in M2
	Compilation   bool     `json:"compilation,omitempty"`
	MBRecordingID string   `json:"mbRecordingId,omitempty"`
	MBAlbumID     string   `json:"mbAlbumId,omitempty"`
	MBArtistID    string   `json:"mbArtistId,omitempty"`
	TrackGain     *float64 `json:"trackGain,omitempty"` // ReplayGain, dB
	AlbumGain     *float64 `json:"albumGain,omitempty"`
	HasCover      bool     `json:"hasCover,omitempty"` // an embedded picture
}

// Range is a byte range of a file.
type Range struct {
	Offset int64 `json:"offset"`
	Length int64 `json:"length"`
}

type Info struct {
	Format     string // container: mp3, flac, ogg, mp4, asf, wav, aiff, or ffprobe's format name
	Codec      string // mp3, flac, opus, vorbis, aac, alac, wma, wmapro, wmalossless, pcm…
	Lossless   bool
	DurationMs int64
	Bitrate    int // kbps
	SampleRate int
	BitDepth   int // lossless formats only
	Channels   int
	Tags       Tags
	Payload    Range  // the audio data without tags or pictures: what gets fingerprinted
	Reader     string // "native" or "ffprobe"
}

// maxRead caps a single read, so a corrupt length field cannot allocate gigabytes.
const maxRead = 32 << 20

type source struct {
	r    io.ReaderAt
	size int64
	pic  *picture // set by Picture: the readers keep the embedded picture
}

// read returns n bytes at off. Reading past the end of the file is a format
// error (some length field lies); a failing read is an I/O error.
func (s source) read(off int64, n int) ([]byte, error) {
	if off < 0 || n < 0 || n > maxRead || off+int64(n) > s.size {
		return nil, fmt.Errorf("%w: %d bytes en %d, fuera del archivo", ErrFormat, n, off)
	}
	b := make([]byte, n)
	k, err := s.r.ReadAt(b, off)
	if k == n {
		return b, nil
	}
	return nil, fmt.Errorf("%w: %v", ErrIO, err)
}

func be16(b []byte) uint16 { return binary.BigEndian.Uint16(b) }
func be32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }
func be64(b []byte) uint64 { return binary.BigEndian.Uint64(b) }
func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func le64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }

// syncsafe decodes a 28-bit ID3v2 "synchsafe" integer (7 bits per byte).
func syncsafe(b []byte) int64 {
	return int64(b[0]&0x7F)<<21 | int64(b[1]&0x7F)<<14 | int64(b[2]&0x7F)<<7 | int64(b[3]&0x7F)
}
