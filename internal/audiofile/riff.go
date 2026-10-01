package audiofile

import (
	"bytes"
	"fmt"
	"math"
	"strings"
)

// eachChunk walks RIFF (little-endian sizes) or IFF/AIFF (big-endian) chunks
// from off. A chunk that claims more bytes than the file has is cut at the end
// of the file (common in files written while recording).
func eachChunk(s source, off int64, bigEndian bool, fn func(id string, body, n int64) error) error {
	for off+8 <= s.size {
		h, err := s.read(off, 8)
		if err != nil {
			return err
		}
		n := int64(le32(h[4:]))
		if bigEndian {
			n = int64(be32(h[4:]))
		}
		body := off + 8
		if body+n > s.size {
			n = s.size - body
		}
		if err := fn(string(h[:4]), body, n); err != nil {
			return err
		}
		off = body + n + n%2
	}
	return nil
}

var wavInfoFields = map[string]string{
	"INAM": "title", "IART": "artist", "IPRD": "album", "ICRD": "date", "IGNR": "genre", "ITRK": "tracknumber", "IPRT": "tracknumber",
}

func readWAV(s source) (Info, error) {
	info := Info{Format: "wav"}
	h, err := s.read(0, 12)
	if err != nil {
		return info, err
	}
	if string(h[:4]) != "RIFF" || string(h[8:12]) != "WAVE" {
		return info, fmt.Errorf("%w: no es un WAV", ErrFormat)
	}
	var byteRate int64
	err = eachChunk(s, 12, false, func(id string, body, n int64) error {
		switch id {
		case "fmt ":
			if n < 16 {
				return fmt.Errorf("%w: bloque fmt corto", ErrFormat)
			}
			d, err := s.read(body, 16)
			if err != nil {
				return err
			}
			tag := le16(d)
			info.Channels, info.SampleRate, info.BitDepth = int(le16(d[2:])), int(le32(d[4:])), int(le16(d[14:]))
			byteRate = int64(le32(d[8:]))
			switch tag {
			case 1, 0xFFFE:
				info.Codec, info.Lossless = "pcm", true
			case 3:
				info.Codec, info.Lossless = "pcm_float", true
			default:
				info.Codec, info.BitDepth = fmt.Sprintf("wav-0x%04x", tag), 0
			}
		case "data":
			info.Payload = Range{body, n}
		case "LIST":
			d, err := s.read(body, int(min(n, maxRead)))
			if err != nil {
				return err
			}
			if len(d) >= 4 && string(d[:4]) == "INFO" {
				readRIFFInfo(d[4:], &info.Tags)
			}
		case "id3 ", "ID3 ":
			_, err := readID3v2(s, body, &info.Tags)
			return err
		}
		return nil
	})
	if err != nil {
		return info, err
	}
	if info.Codec == "" || info.Payload.Length == 0 {
		return info, fmt.Errorf("%w: WAV sin formato o sin datos", ErrFormat)
	}
	if byteRate > 0 {
		info.DurationMs = info.Payload.Length * 1000 / byteRate
		info.Bitrate = int(byteRate * 8 / 1000)
	}
	return info, nil
}

func readRIFFInfo(d []byte, t *Tags) {
	for p := 0; p+8 <= len(d); {
		id, n := string(d[p:p+4]), int(le32(d[p+4:]))
		p += 8
		if n < 0 || p+n > len(d) {
			return
		}
		if field, ok := wavInfoFields[id]; ok {
			v := d[p : p+n]
			if i := bytes.IndexByte(v, 0); i >= 0 {
				v = v[:i]
			}
			t.set(field, text8(v))
		}
		p += n + n%2
	}
}

func readAIFF(s source) (Info, error) {
	info := Info{Format: "aiff"}
	h, err := s.read(0, 12)
	if err != nil {
		return info, err
	}
	kind := string(h[8:12])
	if string(h[:4]) != "FORM" || (kind != "AIFF" && kind != "AIFC") {
		return info, fmt.Errorf("%w: no es un AIFF", ErrFormat)
	}
	var frames int64
	err = eachChunk(s, 12, true, func(id string, body, n int64) error {
		switch id {
		case "COMM":
			if n < 18 {
				return fmt.Errorf("%w: bloque COMM corto", ErrFormat)
			}
			d, err := s.read(body, int(min(n, 22)))
			if err != nil {
				return err
			}
			info.Channels, frames, info.BitDepth = int(be16(d)), int64(be32(d[2:])), int(be16(d[6:]))
			info.SampleRate = int(ext80(d[8:18]))
			info.Codec, info.Lossless = "pcm", true
			if kind == "AIFC" && len(d) >= 22 {
				switch c := string(d[18:22]); c {
				case "NONE", "sowt", "twos":
				case "fl32", "FL32", "fl64", "FL64":
					info.Codec = "pcm_float"
				default:
					info.Codec, info.Lossless = strings.ToLower(strings.TrimSpace(c)), false
				}
			}
		case "SSND":
			if n < 8 {
				return fmt.Errorf("%w: bloque SSND corto", ErrFormat)
			}
			d, err := s.read(body, 8)
			if err != nil {
				return err
			}
			if skip := int64(be32(d)); 8+skip <= n {
				info.Payload = Range{body + 8 + skip, n - 8 - skip}
			}
		case "NAME", "AUTH":
			d, err := s.read(body, int(min(n, 4096)))
			if err != nil {
				return err
			}
			field := "title"
			if id == "AUTH" {
				field = "artist"
			}
			info.Tags.set(field, text8(d))
		case "ID3 ", "id3 ":
			_, err := readID3v2(s, body, &info.Tags)
			return err
		}
		return nil
	})
	if err != nil {
		return info, err
	}
	if info.Codec == "" || info.Payload.Length == 0 {
		return info, fmt.Errorf("%w: AIFF sin formato o sin datos", ErrFormat)
	}
	if info.SampleRate > 0 {
		info.DurationMs = frames * 1000 / int64(info.SampleRate)
	}
	info.Bitrate = info.SampleRate * info.Channels * info.BitDepth / 1000
	return info, nil
}

// ext80 decodes an 80-bit IEEE 754 extended float (AIFF sample rates).
func ext80(b []byte) float64 {
	exp := int(b[0]&0x7F)<<8 | int(b[1])
	mant := be64(b[2:10])
	if exp == 0 && mant == 0 {
		return 0
	}
	f := float64(mant) * math.Pow(2, float64(exp-16383-63))
	if b[0]&0x80 != 0 {
		f = -f
	}
	return f
}
