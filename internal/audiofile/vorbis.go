package audiofile

import (
	"fmt"
	"strings"
)

// parseVorbisComment reads a Vorbis comment block (FLAC, Ogg Vorbis, Opus):
// little-endian lengths, a vendor string, then "KEY=value" entries.
func parseVorbisComment(b []byte, t *Tags) error {
	bad := fmt.Errorf("%w: comentarios Vorbis truncados", ErrFormat)
	if len(b) < 4 {
		return bad
	}
	p := 4 + int64(le32(b))
	if p+4 > int64(len(b)) {
		return bad
	}
	n := int64(le32(b[p:]))
	p += 4
	for i := int64(0); i < n; i++ {
		if p+4 > int64(len(b)) {
			return bad
		}
		l := int64(le32(b[p:]))
		p += 4
		if p+l > int64(len(b)) {
			return bad
		}
		k, v, ok := strings.Cut(string(b[p:p+l]), "=")
		p += l
		if !ok {
			continue
		}
		switch strings.ToUpper(k) {
		case "METADATA_BLOCK_PICTURE", "COVERART":
			t.HasCover = true
		default:
			t.set(k, v)
		}
	}
	return nil
}
