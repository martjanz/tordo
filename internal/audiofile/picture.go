package audiofile

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// ErrNoPicture means the file has no embedded picture the readers understand.
var ErrNoPicture = errors.New("el archivo no tiene imagen")

// picture collects the embedded picture while a file is read for Picture. It
// keeps the first one, or a front cover (picture type 3) over any other.
// Pictures the readers have not read yet (FLAC blocks, MP4 atoms) are kept as
// a range of the file and read at the end; FLAC PICTURE blocks (also inside
// base64 Vorbis comments) are decoded at the end too.
type picture struct {
	found bool
	front bool
	data  []byte
	mime  string
	at    Range // the picture's bytes in the file, when data is nil
	flac  bool  // data (or at) is a whole FLAC PICTURE block
}

// offer considers a picture: the first one wins unless a front cover comes
// later. Empty pictures (some taggers write them) are not pictures.
func (p *picture) offer(front bool, data []byte, mime string, at Range, flac bool) {
	if p == nil || (p.found && (p.front || !front)) || (data != nil && len(data) == 0) || (data == nil && at.Length <= 0) {
		return
	}
	*p = picture{found: true, front: front, data: data, mime: mime, at: at, flac: flac}
}

// offerID3 reads an APIC frame: encoding, MIME type (v2.2: a three-letter
// format), picture type, description, data.
func (p *picture) offerID3(data []byte, v22 bool) {
	if p == nil || len(data) < 2 {
		return
	}
	enc, rest := data[0], data[1:]
	var mime string
	if v22 {
		if len(rest) < 3 {
			return
		}
		mime = "image/" + strings.ToLower(string(rest[:3]))
		rest = rest[3:]
	} else {
		m, after, ok := bytes.Cut(rest, []byte{0})
		if !ok {
			return
		}
		mime, rest = string(m), after
	}
	if len(rest) < 1 {
		return
	}
	front := rest[0] == 3
	rest = rest[1:]
	// The description ends with a terminator of its encoding's width.
	if enc == 1 || enc == 2 {
		for i := 0; ; i += 2 {
			if i+1 >= len(rest) {
				return
			}
			if rest[i] == 0 && rest[i+1] == 0 {
				rest = rest[i+2:]
				break
			}
		}
	} else {
		_, after, ok := bytes.Cut(rest, []byte{0})
		if !ok {
			return
		}
		rest = after
	}
	p.offer(front, rest, mime, Range{}, false)
}

// offerVorbis reads a picture from a Vorbis comment: METADATA_BLOCK_PICTURE
// (a base64 FLAC PICTURE block) or the older COVERART (a base64 image).
func (p *picture) offerVorbis(key, value string) {
	if p == nil {
		return
	}
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return
	}
	if key == "COVERART" {
		p.offer(false, b, "", Range{}, false)
		return
	}
	p.offer(len(b) >= 4 && be32(b) == 3, b, "", Range{}, true)
}

// offerASF reads a WM/Picture value: picture type, data length, MIME type
// and description in UTF-16LE, then the data.
func (p *picture) offerASF(v []byte) {
	if p == nil || len(v) < 5 {
		return
	}
	front, n := v[0] == 3, int(le32(v[1:5]))
	rest := v[5:]
	var mime string
	for field := 0; field < 2; field++ {
		end := -1
		for i := 0; i+1 < len(rest); i += 2 {
			if rest[i] == 0 && rest[i+1] == 0 {
				end = i
				break
			}
		}
		if end < 0 {
			return
		}
		if field == 0 {
			mime = utf16Text(rest[:end], false)
		}
		rest = rest[end+2:]
	}
	if n > len(rest) {
		return
	}
	p.offer(front, rest[:n], mime, Range{}, false)
}

// finish reads what was left as a range and decodes FLAC PICTURE blocks.
func (p *picture) finish(s source) error {
	if !p.found {
		return ErrNoPicture
	}
	if p.data == nil {
		d, err := s.read(p.at.Offset, int(p.at.Length))
		if err != nil {
			return err
		}
		p.data = d
	}
	if p.flac {
		d, mime, ok := flacPicture(p.data)
		if !ok {
			return fmt.Errorf("%w: bloque PICTURE inválido", ErrFormat)
		}
		p.data, p.mime = d, mime
	}
	sniffed := http.DetectContentType(p.data)
	p.mime = strings.ToLower(p.mime)
	if !strings.HasPrefix(p.mime, "image/") || p.mime == "image/jpg" {
		p.mime = sniffed
	}
	if !strings.HasPrefix(sniffed, "image/") {
		return ErrNoPicture // a link ("-->") or something that is not an image
	}
	return nil
}

// flacPicture decodes a FLAC PICTURE block: type, MIME type, description,
// width, height, depth, colors, then the data, all lengths big-endian.
func flacPicture(b []byte) ([]byte, string, bool) {
	p := 4
	field := func() ([]byte, bool) {
		if p+4 > len(b) {
			return nil, false
		}
		n := int(be32(b[p:]))
		p += 4
		if n < 0 || p+n > len(b) {
			return nil, false
		}
		f := b[p : p+n]
		p += n
		return f, true
	}
	mime, ok := field()
	if !ok {
		return nil, "", false
	}
	if _, ok := field(); !ok { // description
		return nil, "", false
	}
	p += 16 // width, height, depth, colors
	data, ok := field()
	return data, string(mime), ok
}

// Picture returns the picture embedded in the audio file at path, a front
// cover when there are several, and its MIME type. It reads MP3 (ID3v2),
// FLAC, Ogg (Opus, Vorbis), MP4/M4A, ASF/WMA and AIFF/WAV with an ID3 chunk.
func Picture(path string) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrIO, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrIO, err)
	}
	read := nativeReader(path)
	if read == nil {
		return nil, "", ErrNoPicture
	}
	pic := &picture{}
	s := source{r: f, size: st.Size(), pic: pic}
	if _, err := read(s); err != nil && !pic.found {
		return nil, "", err
	}
	if err := pic.finish(s); err != nil {
		return nil, "", err
	}
	return pic.data, pic.mime, nil
}
