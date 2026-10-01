package audiofile

import "fmt"

type oggPage struct {
	granule int64
	serial  uint32
	lacing  []byte
	data    int64 // offset of the page's data
	length  int64 // data length
}

func (p oggPage) end() int64 { return p.data + p.length }

func readOggPage(s source, off int64) (oggPage, error) {
	h, err := s.read(off, 27)
	if err != nil {
		return oggPage{}, err
	}
	if string(h[:4]) != "OggS" {
		return oggPage{}, fmt.Errorf("%w: página Ogg inválida", ErrFormat)
	}
	lacing, err := s.read(off+27, int(h[26]))
	if err != nil {
		return oggPage{}, err
	}
	p := oggPage{granule: int64(le64(h[6:14])), serial: le32(h[14:18]), lacing: lacing, data: off + 27 + int64(len(lacing))}
	for _, l := range lacing {
		p.length += int64(l)
	}
	return p, nil
}

// readOgg reads an Ogg Opus or Ogg Vorbis file: codec and tags from the first
// two packets of the first logical stream, duration from its last granule
// position.
func readOgg(s source) (Info, error) {
	info := Info{Format: "ogg"}
	var packets [][]byte
	var cur []byte
	var serial uint32
	off := int64(0)
	for first := true; len(packets) < 2; first = false {
		p, err := readOggPage(s, off)
		if err != nil {
			return info, err
		}
		off = p.end()
		if first {
			serial = p.serial
		} else if p.serial != serial {
			continue // another logical stream
		}
		data, err := s.read(p.data, int(p.length))
		if err != nil {
			return info, err
		}
		pos := 0
		for _, l := range p.lacing {
			cur = append(cur, data[pos:pos+int(l)]...)
			pos += int(l)
			if l < 255 {
				packets = append(packets, cur)
				cur = nil
				if len(packets) == 2 {
					break
				}
			}
		}
		if len(cur) > maxRead {
			return info, fmt.Errorf("%w: paquete Ogg demasiado grande", ErrFormat)
		}
	}
	id, comments := packets[0], packets[1]
	var preSkip int64
	rate := 0
	switch {
	case len(id) >= 19 && string(id[:8]) == "OpusHead":
		info.Codec, info.Channels = "opus", int(id[9])
		preSkip, rate, info.SampleRate = int64(le16(id[10:12])), 48000, 48000
		if len(comments) < 8 || string(comments[:8]) != "OpusTags" {
			return info, fmt.Errorf("%w: faltan los comentarios Opus", ErrFormat)
		}
		if err := parseVorbisComment(comments[8:], &info.Tags, s.pic); err != nil {
			return info, err
		}
	case len(id) >= 30 && id[0] == 1 && string(id[1:7]) == "vorbis":
		info.Codec, info.Channels = "vorbis", int(id[11])
		rate = int(le32(id[12:16]))
		info.SampleRate = rate
		if len(comments) < 7 || comments[0] != 3 || string(comments[1:7]) != "vorbis" {
			return info, fmt.Errorf("%w: faltan los comentarios Vorbis", ErrFormat)
		}
		if err := parseVorbisComment(comments[7:], &info.Tags, s.pic); err != nil {
			return info, err
		}
	default:
		return info, fmt.Errorf("%w: códec Ogg sin lector propio", ErrUnsupported)
	}
	info.Payload = Range{off, s.size - off}
	last, err := lastOggGranule(s, serial)
	if err != nil {
		return info, err
	}
	if rate > 0 && last > preSkip {
		info.DurationMs = (last - preSkip) * 1000 / int64(rate)
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(info.Payload.Length * 8 / info.DurationMs)
	}
	return info, nil
}

// lastOggGranule returns the granule position of the stream's last page,
// looking at the final 64 KiB of the file.
func lastOggGranule(s source, serial uint32) (int64, error) {
	n := min(s.size, 64<<10)
	b, err := s.read(s.size-n, int(n))
	if err != nil {
		return 0, err
	}
	for i := len(b) - 27; i >= 0; i-- {
		if string(b[i:i+4]) == "OggS" && le32(b[i+14:i+18]) == serial {
			if g := int64(le64(b[i+6 : i+14])); g >= 0 {
				return g, nil
			}
		}
	}
	return 0, nil
}
