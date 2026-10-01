package audiofile

import (
	"errors"
	"testing"
)

func TestMP3CBRWithID3v2(t *testing.T) {
	tag := id3Tag(3, id3TextFrame(3, "TIT2", "Fue amor"), id3TextFrame(3, "TPE1", "Fito Páez"))
	audio := mp3Frames(100, 0x11)
	info, err := readMP3(src(cat(tag, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "mp3" || info.Codec != "mp3" || info.Lossless {
		t.Errorf("format = %+v", info)
	}
	if info.SampleRate != 44100 || info.Channels != 2 || info.Bitrate != 128 {
		t.Errorf("tech = %d Hz, %d ch, %d kbps", info.SampleRate, info.Channels, info.Bitrate)
	}
	if info.DurationMs != 2606 { // 41700 bytes * 8 / 128 kbps
		t.Errorf("DurationMs = %d, want 2606", info.DurationMs)
	}
	if info.Payload != (Range{int64(len(tag)), int64(len(audio))}) {
		t.Errorf("Payload = %+v, want {%d %d}", info.Payload, len(tag), len(audio))
	}
	if info.Tags.Title != "Fue amor" || info.Tags.Artist != "Fito Páez" {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestMP3XingDuration(t *testing.T) {
	info, err := readMP3(src(cat(xingFrame("Xing", 100, 0), mp3Frames(100, 0x22))))
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationMs != 2612 { // 100 frames * 1152 samples / 44100 Hz
		t.Errorf("DurationMs = %d, want 2612", info.DurationMs)
	}
}

func TestMP3InconsistentInfoHeaderIsIgnored(t *testing.T) {
	// The header says 100 frames in 10000 bytes; the file has 101 frames
	// (42117 bytes): it was edited after encoding.
	info, err := readMP3(src(cat(xingFrame("Info", 100, 10000), mp3Frames(100, 0x22))))
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationMs != 2632 { // 42117 bytes * 8 / 128 kbps
		t.Errorf("DurationMs = %d, want 2632", info.DurationMs)
	}
}

func TestMP3VBRWithoutHeaderUsesMeanBitrate(t *testing.T) {
	// 20 frames of 32 kbps silence, then 200 frames at 192 kbps: 220 frames,
	// 5747 ms. The first frame alone would say 31.8 s.
	info, err := readMP3(src(cat(mp3FramesAt(20, 1, 0x11), mp3FramesAt(200, 11, 0x22))))
	if err != nil {
		t.Fatal(err)
	}
	if d := info.DurationMs - 5747; d > 170 || d < -170 {
		t.Errorf("DurationMs = %d, want about 5747", info.DurationMs)
	}
}

func TestMP3TrailingTagsAreNotPayload(t *testing.T) {
	audio := mp3Frames(10, 0x33)
	items := []byte("0123456789")
	ape := cat(items, []byte("APETAGEX"), u32le(2000), u32le(len(items)+32), u32le(1), u32le(0), make([]byte, 8))
	v1 := id3v1("Retirada", "La Nueva Milonga", "", "1985", 3, 113)
	info, err := readMP3(src(cat(audio, ape, v1)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Payload != (Range{0, int64(len(audio))}) {
		t.Errorf("Payload = %+v, want {0 %d}", info.Payload, len(audio))
	}
	if info.Tags.Title != "Retirada" || info.Tags.Track != 3 {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestMP3SkipsFalseSync(t *testing.T) {
	junk := []byte{0xFF, 0xFB, 0x90, 0x00, 1, 2, 3} // a header not followed by another frame
	audio := mp3Frames(5, 0x44)
	info, err := readMP3(src(cat(junk, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Payload.Offset != int64(len(junk)) {
		t.Errorf("Payload.Offset = %d, want %d", info.Payload.Offset, len(junk))
	}
}

func TestMP3NotMPEG(t *testing.T) {
	if _, err := readMP3(src([]byte("this is not an mp3 file at all"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestParseMPEGFrame(t *testing.T) {
	f, ok := parseMPEGFrame([]byte{0xFF, 0xFB, 0x90, 0x00})
	if !ok || f.version != 1 || f.layer != 3 || f.bitrate != 128 || f.sampleRate != 44100 || f.length != 417 || f.samples != 1152 {
		t.Errorf("MPEG1 L3 = %+v, %v", f, ok)
	}
	// MPEG-2 layer III, 64 kbps, 22.05 kHz, mono, padded: 72*64000/22050 + 1 = 209
	f, ok = parseMPEGFrame([]byte{0xFF, 0xF3, 0x82, 0xC0})
	if !ok || f.version != 2 || f.bitrate != 64 || f.sampleRate != 22050 || f.channels != 1 || f.length != 209 || f.samples != 576 {
		t.Errorf("MPEG2 L3 = %+v, %v", f, ok)
	}
	if _, ok := parseMPEGFrame([]byte{0xFF, 0xFB, 0xF0, 0x00}); ok {
		t.Error("bitrate index 15 must be rejected")
	}
}
