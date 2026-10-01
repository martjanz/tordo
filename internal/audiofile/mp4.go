package audiofile

import "fmt"

type atom struct {
	typ       string
	off, size int64 // the whole atom
	hdr       int64 // header length: 8, or 16 with a 64-bit size
}

func (a atom) body() int64 { return a.off + a.hdr }
func (a atom) end() int64  { return a.off + a.size }

func readAtom(s source, off, limit int64) (atom, error) {
	h, err := s.read(off, 8)
	if err != nil {
		return atom{}, err
	}
	a := atom{typ: string(h[4:8]), off: off, size: int64(be32(h)), hdr: 8}
	switch a.size {
	case 1:
		x, err := s.read(off+8, 8)
		if err != nil {
			return atom{}, err
		}
		a.size, a.hdr = int64(be64(x)), 16
	case 0:
		a.size = limit - off // up to the end of the parent
	}
	if a.size < a.hdr || off+a.size > limit {
		return atom{}, fmt.Errorf("%w: átomo MP4 %q inválido", ErrFormat, a.typ)
	}
	return a, nil
}

// eachAtom calls fn for every atom in [start, end).
func eachAtom(s source, start, end int64, fn func(atom) error) error {
	for off := start; off+8 <= end; {
		a, err := readAtom(s, off, end)
		if err != nil {
			return err
		}
		if err := fn(a); err != nil {
			return err
		}
		off = a.end()
	}
	return nil
}

// findAtom follows a path of atom types down from parent; ok is false when a
// step is missing.
func findAtom(s source, parent atom, path ...string) (atom, bool, error) {
	cur := parent
	for _, typ := range path {
		var found atom
		ok := false
		err := eachAtom(s, cur.body(), cur.end(), func(a atom) error {
			if !ok && a.typ == typ {
				found, ok = a, true
			}
			return nil
		})
		if err != nil || !ok {
			return atom{}, false, err
		}
		cur = found
	}
	return cur, true, nil
}

func readMP4(s source) (Info, error) {
	info := Info{Format: "mp4"}
	var moov, mdat atom
	var mdatTotal int64
	err := eachAtom(s, 0, s.size, func(a atom) error {
		switch a.typ {
		case "moov":
			moov = a
		case "mdat":
			mdatTotal += a.size - a.hdr
			if a.size > mdat.size {
				mdat = a
			}
		}
		return nil
	})
	if err != nil {
		return info, err
	}
	if moov.size == 0 {
		return info, fmt.Errorf("%w: falta el átomo moov", ErrFormat)
	}
	var scale int64
	if mvhd, ok, err := findAtom(s, moov, "mvhd"); err != nil {
		return info, err
	} else if ok {
		if scale, err = readMVHD(s, mvhd, &info); err != nil {
			return info, err
		}
	}
	if info.DurationMs == 0 && scale > 0 { // fragmented: the duration is in mvex/mehd
		if mehd, ok, err := findAtom(s, moov, "mvex", "mehd"); err != nil {
			return info, err
		} else if ok {
			d, err := s.read(mehd.body(), int(min(mehd.size-mehd.hdr, 12)))
			if err != nil {
				return info, err
			}
			switch {
			case len(d) >= 12 && d[0] == 1:
				info.DurationMs = int64(be64(d[4:12])) * 1000 / scale
			case len(d) >= 8:
				info.DurationMs = int64(be32(d[4:8])) * 1000 / scale
			}
		}
	}
	err = eachAtom(s, moov.body(), moov.end(), func(a atom) error {
		if a.typ != "trak" || info.Codec != "" {
			return nil
		}
		stsd, ok, err := findAtom(s, a, "mdia", "minf", "stbl", "stsd")
		if err != nil || !ok {
			return err
		}
		return readSTSD(s, stsd, &info)
	})
	if err != nil {
		return info, err
	}
	if meta, ok, err := findAtom(s, moov, "udta", "meta"); err != nil {
		return info, err
	} else if ok {
		if err := readMeta(s, meta, &info.Tags); err != nil {
			return info, err
		}
	}
	if info.Codec == "" {
		return info, fmt.Errorf("%w: sin pista de audio", ErrFormat)
	}
	if mdat.size > 0 {
		info.Payload = Range{mdat.body(), mdat.size - mdat.hdr}
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(mdatTotal * 8 / info.DurationMs)
	}
	return info, nil
}

// readMVHD sets the duration and returns the movie timescale.
func readMVHD(s source, a atom, info *Info) (int64, error) {
	d, err := s.read(a.body(), int(min(a.size-a.hdr, 32)))
	if err != nil {
		return 0, err
	}
	var scale, dur int64
	switch {
	case len(d) >= 32 && d[0] == 1:
		scale, dur = int64(be32(d[20:24])), int64(be64(d[24:32]))
	case len(d) >= 20:
		scale, dur = int64(be32(d[12:16])), int64(be32(d[16:20]))
	}
	if scale > 0 {
		info.DurationMs = dur * 1000 / scale
	}
	return scale, nil
}

var mp4Codecs = map[string]struct {
	name     string
	lossless bool
}{
	"mp4a": {"aac", false}, "alac": {"alac", true}, "fLaC": {"flac", true}, "Opus": {"opus", false},
	"ac-3": {"ac3", false}, "ec-3": {"eac3", false}, ".mp3": {"mp3", false},
}

// readSTSD reads the first sample entry of a track; tracks that are not audio
// are left alone.
func readSTSD(s source, a atom, info *Info) error {
	if a.size-a.hdr < 8+36 {
		return nil
	}
	d, err := s.read(a.body()+8, 36)
	if err != nil {
		return err
	}
	c, ok := mp4Codecs[string(d[4:8])]
	if !ok {
		return nil
	}
	info.Codec, info.Lossless = c.name, c.lossless
	info.Channels = int(be16(d[24:26]))
	if c.lossless {
		info.BitDepth = int(be16(d[26:28]))
	}
	info.SampleRate = int(be32(d[32:36]) >> 16)
	return nil
}

var mp4Fields = map[string]string{
	"\xa9nam": "title", "\xa9ART": "artist", "aART": "albumartist", "\xa9alb": "album",
	"\xa9day": "date", "\xa9gen": "genre",
}

func readMeta(s source, meta atom, t *Tags) error {
	if meta.size-meta.hdr < 8 {
		return nil
	}
	start := meta.body()
	peek, err := s.read(start, 8)
	if err != nil {
		return err
	}
	if string(peek[4:8]) != "hdlr" {
		start += 4 // a full box: version and flags come first
	}
	return eachAtom(s, start, meta.end(), func(a atom) error {
		if a.typ != "ilst" {
			return nil
		}
		return eachAtom(s, a.body(), a.end(), func(item atom) error { return readItem(s, item, t) })
	})
}

func readItem(s source, item atom, t *Tags) error {
	if item.typ == "covr" {
		t.HasCover = true // read only by Picture: pictures can be large
		if s.pic == nil {
			return nil
		}
		return eachAtom(s, item.body(), item.end(), func(a atom) error {
			if a.typ == "data" && a.size-a.hdr > 8 {
				s.pic.offer(false, nil, "", Range{a.body() + 8, a.size - a.hdr - 8}, false)
			}
			return nil
		})
	}
	var name string
	var data []byte
	err := eachAtom(s, item.body(), item.end(), func(a atom) error {
		switch a.typ {
		case "name":
			b, err := s.read(a.body(), int(a.size-a.hdr))
			if err != nil {
				return err
			}
			if len(b) > 4 {
				name = string(b[4:])
			}
		case "data":
			b, err := s.read(a.body(), int(a.size-a.hdr))
			if err != nil {
				return err
			}
			if len(b) >= 8 {
				data = b[8:] // after the type indicator and the locale
			}
		}
		return nil
	})
	if err != nil || data == nil {
		return err
	}
	switch item.typ {
	case "----":
		t.set(name, text8(data))
	case "trkn", "disk":
		if len(data) >= 6 {
			n, total := int(be16(data[2:4])), int(be16(data[4:6]))
			if item.typ == "trkn" {
				firstInt(&t.Track, n)
				firstInt(&t.TrackTotal, total)
			} else {
				firstInt(&t.Disc, n)
				firstInt(&t.DiscTotal, total)
			}
		}
	case "gnre":
		if len(data) >= 2 {
			if g := int(be16(data)) - 1; g >= 0 && g < len(genreNames) {
				t.addGenre(genreNames[g])
			}
		}
	case "cpil":
		if len(data) >= 1 && data[0] != 0 {
			t.Compilation = true
		}
	default:
		if field, ok := mp4Fields[item.typ]; ok {
			t.set(field, text8(data))
		}
	}
	return nil
}
