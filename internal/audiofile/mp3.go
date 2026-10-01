package audiofile

import "fmt"

type mpegFrame struct {
	version    int // 1, 2, or 25 (MPEG 2.5)
	layer      int // 1, 2 or 3
	bitrate    int // kbps
	sampleRate int
	channels   int
	samples    int // per frame
	length     int // bytes
}

var mpegBitrates = [5][14]int{
	{32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448}, // MPEG-1 layer I
	{32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384},    // MPEG-1 layer II
	{32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320},     // MPEG-1 layer III
	{32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256},    // MPEG-2/2.5 layer I
	{8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160},         // MPEG-2/2.5 layers II and III
}

var mpegRates = [3][3]int{{44100, 48000, 32000}, {22050, 24000, 16000}, {11025, 12000, 8000}}

// parseMPEGFrame decodes a 4-byte MPEG audio frame header. Free-format and
// reserved values are rejected.
func parseMPEGFrame(b []byte) (mpegFrame, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return mpegFrame{}, false
	}
	verBits, layerBits := (b[1]>>3)&3, (b[1]>>1)&3
	brIdx, srIdx, pad := int(b[2]>>4), int((b[2]>>2)&3), int((b[2]>>1)&1)
	if verBits == 1 || layerBits == 0 || brIdx == 0 || brIdx == 15 || srIdx == 3 {
		return mpegFrame{}, false
	}
	f := mpegFrame{layer: 4 - int(layerBits), channels: 2}
	if b[3]>>6 == 3 {
		f.channels = 1
	}
	var vi int
	switch verBits {
	case 3:
		f.version, vi = 1, 0
	case 2:
		f.version, vi = 2, 1
	default:
		f.version, vi = 25, 2
	}
	f.sampleRate = mpegRates[vi][srIdx]
	table := f.layer - 1
	if f.version != 1 {
		table = 4
		if f.layer == 1 {
			table = 3
		}
	}
	f.bitrate = mpegBitrates[table][brIdx-1]
	br := f.bitrate * 1000
	switch {
	case f.layer == 1:
		f.samples, f.length = 384, (12*br/f.sampleRate+pad)*4
	case f.layer == 2 || f.version == 1:
		f.samples, f.length = 1152, 144*br/f.sampleRate+pad
	default: // layer III of MPEG-2 and 2.5
		f.samples, f.length = 576, 72*br/f.sampleRate+pad
	}
	return f, true
}

// findMPEGFrame returns the first frame header in [start, end) (searching up
// to 256 KiB) that is followed by another frame of the same kind, or that is
// the only frame before end.
func findMPEGFrame(s source, start, end int64) (int64, mpegFrame, error) {
	n := min(end-start, 256<<10)
	if n < 4 {
		return 0, mpegFrame{}, fmt.Errorf("%w: sin audio MPEG", ErrFormat)
	}
	b, err := s.read(start, int(n))
	if err != nil {
		return 0, mpegFrame{}, err
	}
	for i := 0; i+4 <= len(b); i++ {
		f, ok := parseMPEGFrame(b[i:])
		if !ok {
			continue
		}
		next := i + f.length
		switch {
		case start+int64(next) == end:
			return start + int64(i), f, nil // the last frame of the file
		case next+4 <= len(b):
			g, ok := parseMPEGFrame(b[next:])
			if ok && g.version == f.version && g.layer == f.layer && g.sampleRate == f.sampleRate {
				return start + int64(i), f, nil
			}
		case start+int64(next) < end:
			return start + int64(i), f, nil // the next frame lies beyond the search window
		}
	}
	return 0, mpegFrame{}, fmt.Errorf("%w: sin audio MPEG", ErrFormat)
}

// vbrHeader reads the frame and byte counts of a Xing/Info or VBRI header in
// the first frame; zeros when there is none (bytes is 0 when not recorded).
func vbrHeader(s source, off int64, f mpegFrame) (frames, bytes int64) {
	side := 32 // side information after the 4-byte header
	switch {
	case f.version == 1 && f.channels == 1:
		side = 17
	case f.version != 1 && f.channels == 2:
		side = 17
	case f.version != 1:
		side = 9
	}
	b, err := s.read(off, min(f.length, 64))
	if err != nil {
		return 0, 0
	}
	if x := 4 + side; x+12 <= len(b) {
		if tag := string(b[x : x+4]); tag == "Xing" || tag == "Info" {
			flags, p := be32(b[x+4:]), x+8
			if flags&1 != 0 {
				frames = int64(be32(b[p:]))
				p += 4
			}
			if flags&2 != 0 && p+4 <= len(b) {
				bytes = int64(be32(b[p:]))
			}
			return frames, bytes
		}
	}
	if v := 4 + 32; v+18 <= len(b) && string(b[v:v+4]) == "VBRI" {
		return int64(be32(b[v+14:])), int64(be32(b[v+10:]))
	}
	return 0, 0
}

// meanBitrate samples frames at four points of the payload and returns their
// mean bitrate. A frame counts only when the next one parses too.
func meanBitrate(s source, off, end int64, first mpegFrame) int {
	same := func(b []byte) (mpegFrame, bool) {
		g, ok := parseMPEGFrame(b)
		return g, ok && g.version == first.version && g.layer == first.layer && g.sampleRate == first.sampleRate
	}
	sum, n := 0, 0
	for _, at := range []int64{off, off + (end-off)/4, off + (end-off)/2, off + 3*(end-off)/4} {
		b, err := s.read(at, int(min(end-at, 32<<10)))
		if err != nil {
			continue
		}
		for i := 0; i+4 <= len(b); {
			g, ok := same(b[i:])
			if !ok {
				i++
				continue
			}
			if i+g.length+4 > len(b) {
				break
			}
			if _, ok := same(b[i+g.length:]); !ok {
				i++
				continue
			}
			sum += g.bitrate
			n++
			i += g.length
		}
	}
	if n == 0 {
		return first.bitrate
	}
	return sum / n
}

func readMP3(s source) (Info, error) {
	info := Info{Format: "mp3"}
	start := int64(0)
	for { // some files carry several ID3v2 tags in a row
		n, err := readID3v2(s, start, &info.Tags)
		if err != nil {
			return info, err
		}
		if n == 0 {
			break
		}
		start += n
	}
	end := s.size
	n, err := readID3v1(s, &info.Tags)
	if err != nil {
		return info, err
	}
	end -= n
	if n, err = apeTagSize(s, end); err != nil {
		return info, err
	}
	end -= n
	off, f, err := findMPEGFrame(s, start, end)
	if err != nil {
		return info, err
	}
	info.Codec = [...]string{"", "mp1", "mp2", "mp3"}[f.layer]
	info.SampleRate, info.Channels = f.sampleRate, f.channels
	info.Payload = Range{off, end - off}
	frames, bytes := vbrHeader(s, off, f)
	if diff := bytes - info.Payload.Length; frames > 0 && (bytes == 0 || 10*max(diff, -diff) <= info.Payload.Length) {
		info.DurationMs = frames * int64(f.samples) * 1000 / int64(f.sampleRate)
	} else {
		info.DurationMs = info.Payload.Length * 8 / int64(meanBitrate(s, off, end, f))
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(info.Payload.Length * 8 / info.DurationMs)
	}
	return info, nil
}
