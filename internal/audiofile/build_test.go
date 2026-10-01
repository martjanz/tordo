package audiofile

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// Builders of tiny, hand-made audio files for the tests. Payload bytes are
// filler: the readers never decode audio.

func src(b []byte) source { return source{bytes.NewReader(b), int64(len(b))} }

func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func u16be(v int) []byte   { return binary.BigEndian.AppendUint16(nil, uint16(v)) }
func u32be(v int) []byte   { return binary.BigEndian.AppendUint32(nil, uint32(v)) }
func u16le(v int) []byte   { return binary.LittleEndian.AppendUint16(nil, uint16(v)) }
func u32le(v int) []byte   { return binary.LittleEndian.AppendUint32(nil, uint32(v)) }
func u64le(v int64) []byte { return binary.LittleEndian.AppendUint64(nil, uint64(v)) }

func ssafe(n int) []byte {
	return []byte{byte(n>>21) & 0x7F, byte(n>>14) & 0x7F, byte(n>>7) & 0x7F, byte(n) & 0x7F}
}

// id3Frame builds an ID3v2.3 or v2.4 frame (v2.4 sizes are synchsafe).
func id3Frame(major byte, id string, data []byte) []byte {
	size := u32be(len(data))
	if major == 4 {
		size = ssafe(len(data))
	}
	return cat([]byte(id), size, []byte{0, 0}, data)
}

// id3TextFrame encodes values as UTF-8 (v2.4) or UTF-16 with BOM (v2.3).
func id3TextFrame(major byte, id string, values ...string) []byte {
	if major == 4 {
		return id3Frame(4, id, cat([]byte{3}, []byte(strings.Join(values, "\x00"))))
	}
	b := []byte{1}
	for i, v := range values {
		if i > 0 {
			b = append(b, 0, 0)
		}
		b = append(b, 0xFF, 0xFE)
		for _, u := range utf16.Encode([]rune(v)) {
			b = binary.LittleEndian.AppendUint16(b, u)
		}
	}
	return id3Frame(3, id, b)
}

// id3Tag wraps frames in an ID3v2 header, with 16 bytes of padding.
func id3Tag(major byte, frames ...[]byte) []byte {
	body := cat(append(frames, make([]byte, 16))...)
	return cat([]byte{'I', 'D', '3', major, 0, 0}, ssafe(len(body)), body)
}

func id3v1(title, artist, album, year string, track, genre byte) []byte {
	b := make([]byte, 128)
	copy(b, "TAG")
	copy(b[3:33], title)
	copy(b[33:63], artist)
	copy(b[63:93], album)
	copy(b[93:97], year)
	b[126], b[127] = track, genre
	return b
}

// mp3Frame is one MPEG-1 layer III frame: 128 kbps, 44.1 kHz, stereo, no
// padding (417 bytes), filled with fill (avoid 0xFF).
func mp3Frame(fill byte) []byte {
	f := bytes.Repeat([]byte{fill}, 417)
	f[0], f[1], f[2], f[3] = 0xFF, 0xFB, 0x90, 0x00
	return f
}

func mp3Frames(n int, fill byte) []byte {
	var b []byte
	for i := 0; i < n; i++ {
		b = append(b, mp3Frame(fill)...)
	}
	return b
}

// xingFrame is a first frame carrying a Xing (or "Info") header with the
// frame count and, when bytes > 0, the byte count.
func xingFrame(tag string, frames, bytes int) []byte {
	f := mp3Frame(0)
	copy(f[36:], tag) // 4-byte header + 32 bytes of side info (MPEG-1 stereo)
	flags := 1
	if bytes > 0 {
		flags |= 2
		copy(f[48:], u32be(bytes))
	}
	copy(f[40:], u32be(flags))
	copy(f[44:], u32be(frames))
	return f
}

// mp3FramesAt builds n MPEG-1 layer III 44.1 kHz stereo frames at a bitrate
// index (1 = 32 kbps, 9 = 128 kbps, 11 = 192 kbps).
func mp3FramesAt(n, brIdx int, fill byte) []byte {
	br := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192}[brIdx]
	var b []byte
	for i := 0; i < n; i++ {
		f := bytes.Repeat([]byte{fill}, 144*br*1000/44100)
		f[0], f[1], f[2], f[3] = 0xFF, 0xFB, byte(brIdx<<4), 0x00
		b = append(b, f...)
	}
	return b
}

func flacBlock(typ byte, last bool, data []byte) []byte {
	if last {
		typ |= 0x80
	}
	return cat([]byte{typ, byte(len(data) >> 16), byte(len(data) >> 8), byte(len(data))}, data)
}

func streamInfo(rate, channels, bits int, samples int64) []byte {
	d := make([]byte, 34)
	d[10] = byte(rate >> 12)
	d[11] = byte(rate >> 4)
	d[12] = byte(rate&0x0F)<<4 | byte(channels-1)<<1 | byte(bits-1)>>4
	d[13] = (byte(bits-1)&0x0F)<<4 | byte(samples>>32)&0x0F
	binary.BigEndian.PutUint32(d[14:], uint32(samples))
	return d
}

// vorbisComment builds a comment block: vendor, then "KEY=value" entries.
func vorbisComment(vendor string, entries ...string) []byte {
	b := cat(u32le(len(vendor)), []byte(vendor), u32le(len(entries)))
	for _, e := range entries {
		b = cat(b, u32le(len(e)), []byte(e))
	}
	return b
}

// mkOggRawPage builds a page from explicit lacing values (CRC left at zero:
// the reader does not check it).
func mkOggRawPage(headerType byte, granule int64, serial, seq int, lacing, data []byte) []byte {
	return cat([]byte("OggS"), []byte{0, headerType}, u64le(granule), u32le(serial), u32le(seq), u32le(0),
		[]byte{byte(len(lacing))}, lacing, data)
}

// mkOggPage builds a page holding whole packets.
func mkOggPage(headerType byte, granule int64, serial, seq int, packets ...[]byte) []byte {
	var lacing, data []byte
	for _, p := range packets {
		n := len(p)
		for ; n >= 255; n -= 255 {
			lacing = append(lacing, 255)
		}
		lacing = append(lacing, byte(n))
		data = append(data, p...)
	}
	return mkOggRawPage(headerType, granule, serial, seq, lacing, data)
}

func opusHead(channels, preSkip int) []byte {
	return cat([]byte("OpusHead"), []byte{1, byte(channels)}, u16le(preSkip), u32le(48000), u16le(0), []byte{0})
}

func vorbisID(channels, rate int) []byte {
	return cat([]byte{1}, []byte("vorbis"), u32le(0), []byte{byte(channels)}, u32le(rate),
		u32le(0), u32le(128000), u32le(0), []byte{0xB8, 1})
}

func mkAtom(typ string, parts ...[]byte) []byte {
	body := cat(parts...)
	return cat(u32be(8+len(body)), []byte(typ), body)
}

func mp4Data(kind int, value []byte) []byte { return mkAtom("data", u32be(kind), u32be(0), value) }

func mp4Text(typ, v string) []byte { return mkAtom(typ, mp4Data(1, []byte(v))) }

// m4aFile builds an AAC M4A: 44.1 kHz stereo, 5 s, with the given ilst items.
func m4aFile(fullMeta bool, payload []byte, items ...[]byte) []byte {
	mvhd := mkAtom("mvhd", u32be(0), u32be(0), u32be(0), u32be(1000), u32be(5000), make([]byte, 80))
	entry := mkAtom("mp4a", make([]byte, 6), u16be(1), make([]byte, 8), u16be(2), u16be(16), u16be(0), u16be(0), u32be(44100<<16))
	stsd := mkAtom("stsd", u32be(0), u32be(1), entry)
	trak := mkAtom("trak", mkAtom("mdia", mkAtom("minf", mkAtom("stbl", stsd))))
	var meta []byte
	if fullMeta {
		meta = mkAtom("meta", u32be(0), mkAtom("hdlr", make([]byte, 25)), mkAtom("ilst", items...))
	} else {
		meta = mkAtom("meta", mkAtom("hdlr", make([]byte, 25)), mkAtom("ilst", items...))
	}
	moov := mkAtom("moov", mvhd, trak, mkAtom("udta", meta))
	return cat(mkAtom("ftyp", []byte("M4A "), u32be(0)), moov, mkAtom("mdat", payload))
}

// fragmentedM4A builds a fragmented FLAC-in-MP4: mvhd duration 0, the real
// duration (5 s at timescale 1000) in mvex/mehd, and two moof/mdat pairs.
func fragmentedM4A(first, second []byte) []byte {
	mvhd := mkAtom("mvhd", u32be(0), u32be(0), u32be(0), u32be(1000), u32be(0), make([]byte, 80))
	entry := mkAtom("fLaC", make([]byte, 6), u16be(1), make([]byte, 8), u16be(2), u16be(16), u16be(0), u16be(0), u32be(48000<<16))
	trak := mkAtom("trak", mkAtom("mdia", mkAtom("minf", mkAtom("stbl", mkAtom("stsd", u32be(0), u32be(1), entry)))))
	mvex := mkAtom("mvex", mkAtom("mehd", u32be(0), u32be(5000)), mkAtom("trex", make([]byte, 24)))
	moof := mkAtom("moof", mkAtom("mfhd", make([]byte, 8)))
	return cat(mkAtom("ftyp", []byte("iso5"), u32be(0)), mkAtom("moov", mvhd, trak, mvex),
		moof, mkAtom("mdat", first), moof, mkAtom("mdat", second))
}

func asfObject(id [16]byte, body ...[]byte) []byte {
	b := cat(body...)
	return cat(id[:], u64le(int64(24+len(b))), b)
}

func utf16z(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return append(b, 0, 0)
}

func asfDesc(name string, typ int, value []byte) []byte {
	n := utf16z(name)
	return cat(u16le(len(n)), n, u16le(typ), u16le(len(value)), value)
}

// wmaFile builds a WMA v2 file: 44.1 kHz stereo, 128 kbps, 5 s (plus a 3 s preroll).
func wmaFile(payload []byte) (file []byte, headerLen int) {
	fileProps := asfObject(asfFileProps, make([]byte, 40), u64le((5000+3000)*10000), u64le(0), u64le(3000), make([]byte, 16))
	wfx := cat(u16le(0x0161), u16le(2), u32le(44100), u32le(16000), u16le(4096), u16le(16))
	streamProps := asfObject(asfStreamProps, asfAudioMedia[:], make([]byte, 16), u64le(0), u32le(len(wfx)), u32le(0), u16le(1), u32le(0), wfx)
	title, author := utf16z("Guitarra negra"), utf16z("Alfredo Zitarrosa")
	content := asfObject(asfContentDesc, u16le(len(title)), u16le(len(author)), u16le(0), u16le(0), u16le(0), title, author)
	ext := asfObject(asfExtContent, u16le(4),
		asfDesc("WM/AlbumTitle", 0, utf16z("Guitarra negra")),
		asfDesc("WM/TrackNumber", 3, u32le(5)),
		asfDesc("WM/Genre", 0, utf16z("Folklore")),
		asfDesc("WM/Picture", 1, []byte{1, 2, 3}))
	objects := cat(fileProps, streamProps, content, ext)
	header := cat(asfHeader[:], u64le(int64(30+len(objects))), u32le(4), []byte{1, 2}, objects)
	data := cat(asfData[:], u64le(int64(50+len(payload))), make([]byte, 26), payload)
	return cat(header, data), len(header)
}

func riffChunk(id string, body []byte) []byte {
	b := cat([]byte(id), u32le(len(body)), body)
	if len(body)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

func iffChunk(id string, body []byte) []byte {
	b := cat([]byte(id), u32be(len(body)), body)
	if len(body)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

// wavFile builds a 16-bit stereo 44.1 kHz WAV of one second, with INFO tags.
func wavFile() []byte {
	chunks := cat(
		riffChunk("fmt ", cat(u16le(1), u16le(2), u32le(44100), u32le(176400), u16le(4), u16le(16))),
		riffChunk("LIST", cat([]byte("INFO"), riffChunk("INAM", []byte("Candombe\x00")), riffChunk("IART", []byte("Rubén Rada\x00")))),
		riffChunk("data", make([]byte, 176400)),
	)
	return cat([]byte("RIFF"), u32le(4+len(chunks)), []byte("WAVE"), chunks)
}

var ext44100 = []byte{0x40, 0x0E, 0xAC, 0x44, 0, 0, 0, 0, 0, 0}

func aiffFile() []byte {
	chunks := cat(
		iffChunk("COMM", cat(u16be(2), u32be(44100), u16be(16), ext44100)),
		iffChunk("SSND", cat(u32be(0), u32be(0), make([]byte, 176400))),
		iffChunk("ID3 ", id3Tag(3, id3TextFrame(3, "TIT2", "Tango del este"))),
	)
	return cat([]byte("FORM"), u32be(4+len(chunks)), []byte("AIFF"), chunks)
}
