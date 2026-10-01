package audiofile

import "fmt"

func readFLAC(s source) (Info, error) {
	info := Info{Format: "flac", Codec: "flac", Lossless: true}
	off, err := readID3v2(s, 0, &info.Tags) // some taggers put ID3 before "fLaC"
	if err != nil {
		return info, err
	}
	magic, err := s.read(off, 4)
	if err != nil {
		return info, err
	}
	if string(magic) != "fLaC" {
		return info, fmt.Errorf("%w: falta la marca fLaC", ErrFormat)
	}
	pos := off + 4
	var samples int64
	for last := false; !last; {
		h, err := s.read(pos, 4)
		if err != nil {
			return info, err
		}
		last = h[0]&0x80 != 0
		typ, n := h[0]&0x7F, int(h[1])<<16|int(h[2])<<8|int(h[3])
		pos += 4
		switch typ {
		case 0: // STREAMINFO
			if n < 34 {
				return info, fmt.Errorf("%w: STREAMINFO corto", ErrFormat)
			}
			d, err := s.read(pos, 34)
			if err != nil {
				return info, err
			}
			info.SampleRate = int(d[10])<<12 | int(d[11])<<4 | int(d[12])>>4
			info.Channels = int((d[12]>>1)&7) + 1
			info.BitDepth = (int(d[12]&1)<<4 | int(d[13]>>4)) + 1
			samples = int64(d[13]&0x0F)<<32 | int64(be32(d[14:18]))
		case 4: // VORBIS_COMMENT
			d, err := s.read(pos, n)
			if err != nil {
				return info, err
			}
			if err := parseVorbisComment(d, &info.Tags, s.pic); err != nil {
				return info, err
			}
		case 6: // PICTURE
			info.Tags.HasCover = true
			if s.pic != nil && n >= 4 {
				typ, err := s.read(pos, 4)
				if err != nil {
					return info, err
				}
				s.pic.offer(be32(typ) == 3, nil, "", Range{pos, int64(n)}, true)
			}
		case 127:
			return info, fmt.Errorf("%w: bloque FLAC inválido", ErrFormat)
		}
		pos += int64(n)
	}
	if pos > s.size {
		return info, fmt.Errorf("%w: metadatos FLAC truncados", ErrFormat)
	}
	info.Payload = Range{pos, s.size - pos}
	if info.SampleRate > 0 {
		info.DurationMs = samples * 1000 / int64(info.SampleRate)
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(info.Payload.Length * 8 / info.DurationMs)
	}
	return info, nil
}
