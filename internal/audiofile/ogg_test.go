package audiofile

import (
	"bytes"
	"errors"
	"testing"
)

func TestOpus(t *testing.T) {
	head := cat(
		mkOggPage(2, 0, 7, 0, opusHead(2, 312)),
		mkOggPage(0, 0, 7, 1, cat([]byte("OpusTags"), vorbisComment("yt-dlp", "TITLE=Criosho", "ARTIST=Barrios", "DATE=20231105"))),
	)
	audio := cat(
		mkOggPage(0, 48000, 7, 2, bytes.Repeat([]byte{1}, 200)),
		mkOggPage(0, 96000, 7, 3, bytes.Repeat([]byte{2}, 200)),
		mkOggPage(4, 144312, 7, 4, bytes.Repeat([]byte{3}, 200)),
	)
	info, err := readOgg(src(cat(head, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "ogg" || info.Codec != "opus" || info.Channels != 2 || info.SampleRate != 48000 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 3000 { // (144312 - 312) / 48000
		t.Errorf("DurationMs = %d, want 3000", info.DurationMs)
	}
	if info.Payload != (Range{int64(len(head)), int64(len(audio))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	if info.Tags.Title != "Criosho" || info.Tags.Artist != "Barrios" || info.Tags.Year != 2023 {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestVorbisCommentAcrossPages(t *testing.T) {
	comments := cat([]byte{3}, []byte("vorbis"), vorbisComment("Xiph", "TITLE=Tango del este", "ALBUM="+string(bytes.Repeat([]byte("x"), 400))), []byte{1})
	first, rest := comments[:255], comments[255:]
	lacingRest := []byte{}
	n := len(rest)
	for ; n >= 255; n -= 255 {
		lacingRest = append(lacingRest, 255)
	}
	lacingRest = append(lacingRest, byte(n))
	head := cat(
		mkOggPage(2, 0, 9, 0, vorbisID(2, 44100)),
		mkOggRawPage(0, 0, 9, 1, []byte{255}, first), // the packet continues on the next page
		mkOggRawPage(1, 0, 9, 2, lacingRest, rest),
	)
	audio := mkOggPage(4, 441000, 9, 3, bytes.Repeat([]byte{5}, 300))
	info, err := readOgg(src(cat(head, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Codec != "vorbis" || info.SampleRate != 44100 || info.DurationMs != 10000 {
		t.Errorf("tech = %+v", info)
	}
	if info.Tags.Title != "Tango del este" || len(info.Tags.Album) != 400 {
		t.Errorf("tags = %+v", info.Tags)
	}
	if info.Payload.Offset != int64(len(head)) {
		t.Errorf("Payload.Offset = %d, want %d", info.Payload.Offset, len(head))
	}
}

func TestOggUnknownCodec(t *testing.T) {
	file := cat(mkOggPage(2, 0, 1, 0, []byte("\x7FFLAC....")), mkOggPage(0, 0, 1, 1, []byte("x")))
	if _, err := readOgg(src(file)); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestOggNotOgg(t *testing.T) {
	if _, err := readOgg(src([]byte("RIFF....WAVEfmt and some more bytes"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
