package audiofile

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// guid turns the textual form of a GUID into its on-disk bytes (the first
// three fields are little-endian).
func guid(s string) [16]byte {
	h, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if err != nil || len(h) != 16 {
		panic("guid inválido: " + s)
	}
	var g [16]byte
	g[0], g[1], g[2], g[3] = h[3], h[2], h[1], h[0]
	g[4], g[5] = h[5], h[4]
	g[6], g[7] = h[7], h[6]
	copy(g[8:], h[8:])
	return g
}

var (
	asfHeader      = guid("75B22630-668E-11CF-A6D9-00AA0062CE6C")
	asfData        = guid("75B22636-668E-11CF-A6D9-00AA0062CE6C")
	asfFileProps   = guid("8CABDCA1-A947-11CF-8EE4-00C00C205365")
	asfStreamProps = guid("B7DC0791-A9B7-11CF-8EE6-00C00C205365")
	asfContentDesc = guid("75B22633-668E-11CF-A6D9-00AA0062CE6C")
	asfExtContent  = guid("D2D0A440-E307-11D2-97F0-00A0C95EA850")
	asfAudioMedia  = guid("F8699E40-5B4D-11CF-A8FD-00805F5C442B")
)

var asfCodecs = map[uint16]struct {
	name     string
	lossless bool
}{
	0x0160: {"wmav1", false}, 0x0161: {"wma", false}, 0x0162: {"wmapro", false}, 0x0163: {"wmalossless", true},
	0x000A: {"wmavoice", false}, 0x0055: {"mp3", false}, 0x0001: {"pcm", true},
}

func readASF(s source) (Info, error) {
	info := Info{Format: "asf"}
	h, err := s.read(0, 30)
	if err != nil {
		return info, err
	}
	if [16]byte(h[:16]) != asfHeader {
		return info, fmt.Errorf("%w: falta la cabecera ASF", ErrFormat)
	}
	hsize, count := int64(le64(h[16:24])), int(le32(h[24:28]))
	if hsize < 30 || hsize > s.size {
		return info, fmt.Errorf("%w: cabecera ASF truncada", ErrFormat)
	}
	off := int64(30)
	for i := 0; i < count && off+24 <= hsize; i++ {
		oh, err := s.read(off, 24)
		if err != nil {
			return info, err
		}
		size := int64(le64(oh[16:24]))
		if size < 24 || off+size > hsize {
			return info, fmt.Errorf("%w: objeto ASF inválido", ErrFormat)
		}
		body, n := off+24, size-24
		switch [16]byte(oh[:16]) {
		case asfFileProps:
			if n >= 80 {
				d, err := s.read(body, 80)
				if err != nil {
					return info, err
				}
				play, preroll := int64(le64(d[40:48])), int64(le64(d[56:64]))
				info.DurationMs = max(play/10000-preroll, 0)
			}
		case asfStreamProps:
			if n >= 70 && info.Codec == "" {
				d, err := s.read(body, 70)
				if err != nil {
					return info, err
				}
				if [16]byte(d[:16]) == asfAudioMedia {
					tag := le16(d[54:56])
					c, ok := asfCodecs[tag]
					if !ok {
						c.name = fmt.Sprintf("asf-0x%04x", tag)
					}
					info.Codec, info.Lossless = c.name, c.lossless
					info.Channels, info.SampleRate = int(le16(d[56:58])), int(le32(d[58:62]))
					info.Bitrate = int(le32(d[62:66])) * 8 / 1000
					if c.lossless {
						info.BitDepth = int(le16(d[68:70]))
					}
				}
			}
		case asfContentDesc:
			d, err := s.read(body, int(n))
			if err != nil {
				return info, err
			}
			readASFContent(d, &info.Tags)
		case asfExtContent:
			d, err := s.read(body, int(n))
			if err != nil {
				return info, err
			}
			readASFExtContent(d, &info.Tags)
		}
		off += size
	}
	if info.Codec == "" {
		return info, fmt.Errorf("%w: sin pista de audio", ErrFormat)
	}
	if d, err := s.read(hsize, 24); err == nil && [16]byte(d[:16]) == asfData {
		if size := int64(le64(d[16:24])); size >= 50 && hsize+size <= s.size {
			info.Payload = Range{hsize + 50, size - 50}
		}
	}
	if info.Payload.Length == 0 {
		info.Payload = Range{hsize, s.size - hsize}
	}
	return info, nil
}

// readASFContent reads the Content Description object: five lengths, then
// title, author, copyright, description and rating in UTF-16LE.
func readASFContent(d []byte, t *Tags) {
	if len(d) < 10 {
		return
	}
	var vals [5]string
	p := 10
	for i := range vals {
		l := int(le16(d[2*i:]))
		if p+l > len(d) {
			return
		}
		vals[i] = utf16Text(d[p:p+l], false)
		p += l
	}
	t.set("title", vals[0])
	t.set("artist", vals[1])
}

// readASFExtContent reads the Extended Content Description object: named
// attributes such as WM/AlbumTitle, WM/TrackNumber or MusicBrainz/Album Id.
func readASFExtContent(d []byte, t *Tags) {
	if len(d) < 2 {
		return
	}
	n, p := int(le16(d)), 2
	for i := 0; i < n; i++ {
		if p+2 > len(d) {
			return
		}
		nl := int(le16(d[p:]))
		p += 2
		if p+nl+4 > len(d) {
			return
		}
		name := clean(utf16Text(d[p:p+nl], false))
		p += nl
		typ, vl := le16(d[p:]), int(le16(d[p+2:]))
		p += 4
		if p+vl > len(d) {
			return
		}
		v := d[p : p+vl]
		p += vl
		switch typ {
		case 0: // string
			t.set(name, utf16Text(v, false))
		case 1: // bytes
			if name == "WM/Picture" {
				t.HasCover = true
			}
		case 2, 3: // bool, dword
			if len(v) >= 4 {
				t.set(name, strconv.Itoa(int(le32(v))))
			}
		case 4: // qword
			if len(v) >= 8 {
				t.set(name, strconv.FormatUint(le64(v), 10))
			}
		case 5: // word
			if len(v) >= 2 {
				t.set(name, strconv.Itoa(int(le16(v))))
			}
		}
	}
}
