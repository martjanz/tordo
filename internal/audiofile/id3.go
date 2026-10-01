package audiofile

import (
	"bytes"
	"fmt"
)

// v22Names maps ID3v2.2 three-letter frame ids to their v2.3/2.4 names.
var v22Names = map[string]string{
	"TT2": "TIT2", "TP1": "TPE1", "TP2": "TPE2", "TAL": "TALB", "TRK": "TRCK", "TPA": "TPOS",
	"TYE": "TYER", "TOR": "TORY", "TCO": "TCON", "TCP": "TCMP", "TXX": "TXXX", "PIC": "APIC", "UFI": "UFID",
}

// id3Fields maps ID3 text frames to Tags.set names.
var id3Fields = map[string]string{
	"TIT2": "title", "TPE1": "artist", "TPE2": "albumartist", "TALB": "album", "TRCK": "tracknumber",
	"TPOS": "discnumber", "TYER": "date", "TDRC": "date", "TDOR": "originaldate", "TORY": "originaldate",
	"TCMP": "compilation",
}

// readID3v2 parses the ID3v2 tag at off into t and returns its total size
// (header, frames, padding and footer), or 0 when there is no tag at off.
// Compressed and encrypted frames are skipped.
func readID3v2(s source, off int64, t *Tags) (int64, error) {
	if off+10 > s.size {
		return 0, nil
	}
	h, err := s.read(off, 10)
	if err != nil {
		return 0, err
	}
	if string(h[:3]) != "ID3" {
		return 0, nil
	}
	major, flags := h[3], h[5]
	size := syncsafe(h[6:10])
	total := 10 + size
	if flags&0x10 != 0 {
		total += 10 // footer
	}
	if off+total > s.size {
		return 0, fmt.Errorf("%w: etiqueta ID3v2 truncada", ErrFormat)
	}
	if major < 2 || major > 4 {
		return total, nil // unknown version: skip it whole
	}
	body, err := s.read(off+10, int(size))
	if err != nil {
		return 0, err
	}
	if major < 4 && flags&0x80 != 0 {
		body = unsync(body)
	}
	pos := 0
	if flags&0x40 != 0 && major >= 3 && len(body) >= 4 { // extended header
		if major == 3 {
			pos = 4 + int(be32(body))
		} else {
			pos = int(syncsafe(body))
		}
	}
	idLen, hdrLen := 4, 10
	if major == 2 {
		idLen, hdrLen = 3, 6
	}
	for pos >= 0 && pos+hdrLen <= len(body) {
		fh := body[pos : pos+hdrLen]
		if !validFrameID(fh[:idLen]) {
			break // padding, or garbage
		}
		id := string(fh[:idLen])
		var n int
		var fflags uint16
		switch major {
		case 2:
			n = int(fh[3])<<16 | int(fh[4])<<8 | int(fh[5])
		case 3:
			n, fflags = int(be32(fh[4:])), be16(fh[8:])
		case 4:
			n, fflags = int(syncsafe(fh[4:])), be16(fh[8:])
		}
		pos += hdrLen
		if n < 0 || pos+n > len(body) {
			break
		}
		data := body[pos : pos+n]
		pos += n
		switch major {
		case 3:
			if fflags&0x00C0 != 0 {
				continue // compressed or encrypted
			}
			if fflags&0x0020 != 0 && len(data) > 0 {
				data = data[1:] // group id
			}
		case 4:
			if fflags&0x000C != 0 {
				continue // compressed or encrypted
			}
			if fflags&0x0040 != 0 && len(data) > 0 {
				data = data[1:] // group id
			}
			if fflags&0x0001 != 0 && len(data) >= 4 {
				data = data[4:] // data length indicator
			}
			if fflags&0x0002 != 0 {
				data = unsync(data)
			}
		}
		if v2, ok := v22Names[id]; ok {
			id = v2
		}
		t.id3Frame(id, data)
	}
	return total, nil
}

func validFrameID(id []byte) bool {
	for _, c := range id {
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func (t *Tags) id3Frame(id string, data []byte) {
	switch id {
	case "APIC":
		t.HasCover = true
	case "UFID":
		owner, rest, ok := bytes.Cut(data, []byte{0})
		if ok && string(owner) == "http://musicbrainz.org" {
			t.set("musicbrainztrackid", string(rest))
		}
	case "TXXX":
		if v := id3Text(data); len(v) >= 2 {
			t.set(v[0], v[1])
		}
	case "TCON":
		for _, v := range id3Text(data) {
			for _, g := range id3Genres(v) {
				t.addGenre(g)
			}
		}
	default:
		if name, ok := id3Fields[id]; ok {
			for _, v := range id3Text(data) {
				t.set(name, v)
			}
		}
	}
}

// id3Text decodes a text frame: an encoding byte, then one or more values
// separated by terminators (v2.4 allows several values).
func id3Text(data []byte) []string {
	if len(data) < 1 {
		return nil
	}
	enc, b := data[0], data[1:]
	var parts [][]byte
	if enc == 1 || enc == 2 {
		start := 0
		for i := 0; i+1 < len(b); i += 2 {
			if b[i] == 0 && b[i+1] == 0 {
				parts = append(parts, b[start:i])
				start = i + 2
			}
		}
		parts = append(parts, b[start:])
	} else {
		parts = bytes.Split(b, []byte{0})
	}
	var out []string
	for _, p := range parts {
		var s string
		switch enc {
		case 1:
			s = utf16Text(p, false)
		case 2:
			s = utf16Text(p, true)
		default: // 0 (Latin-1, often UTF-8 in practice) and 3 (UTF-8)
			s = text8(p)
		}
		out = append(out, clean(s))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// unsync undoes ID3 unsynchronisation: every 0xFF 0x00 becomes 0xFF.
func unsync(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		out = append(out, b[i])
		if b[i] == 0xFF && i+1 < len(b) && b[i+1] == 0x00 {
			i++
		}
	}
	return out
}

// readID3v1 fills the fields of t that are still empty from an ID3v1 tag at
// the end of the file, and returns the tag's size (128) or 0 when there is none.
func readID3v1(s source, t *Tags) (int64, error) {
	if s.size < 128 {
		return 0, nil
	}
	b, err := s.read(s.size-128, 128)
	if err != nil {
		return 0, err
	}
	if string(b[:3]) != "TAG" {
		return 0, nil
	}
	field := func(f []byte) string {
		if i := bytes.IndexByte(f, 0); i >= 0 {
			f = f[:i]
		}
		return clean(text8(f))
	}
	first(&t.Title, field(b[3:33]))
	first(&t.Artist, field(b[33:63]))
	first(&t.Album, field(b[63:93]))
	firstInt(&t.Year, parseYear(field(b[93:97])))
	if b[125] == 0 && b[126] != 0 {
		firstInt(&t.Track, int(b[126]))
	}
	if len(t.Genres) == 0 && int(b[127]) < len(genreNames) {
		t.Genres = []string{genreNames[b[127]]}
	}
	return 128, nil
}

// apeTagSize returns the size of an APE tag that ends at end (its 32-byte
// footer starts with "APETAGEX"), including its header when it has one, or 0.
func apeTagSize(s source, end int64) (int64, error) {
	if end < 32 {
		return 0, nil
	}
	f, err := s.read(end-32, 32)
	if err != nil {
		return 0, err
	}
	if string(f[:8]) != "APETAGEX" {
		return 0, nil
	}
	n := int64(le32(f[12:16])) // items and footer
	if le32(f[20:24])&(1<<31) != 0 {
		n += 32 // header
	}
	if n > end {
		return 0, fmt.Errorf("%w: etiqueta APE inválida", ErrFormat)
	}
	return n, nil
}
