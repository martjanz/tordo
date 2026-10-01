package audiofile

import (
	"bytes"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
)

// Smallest headers http.DetectContentType recognizes, plus filler.
var (
	jpeg = []byte("\xFF\xD8\xFFfront-cover")
	png  = []byte("\x89PNG\x0D\x0A\x1A\x0Aback-cover")
)

// flacPictureBlock builds a FLAC PICTURE block body.
func flacPictureBlock(typ int, mime string, data []byte) []byte {
	return cat(u32be(typ), u32be(len(mime)), []byte(mime), u32be(0), make([]byte, 16), u32be(len(data)), data)
}

func apic(typ byte, mime string, data []byte) []byte {
	return cat([]byte{0}, []byte(mime), []byte{0, typ}, []byte("desc\x00"), data)
}

func TestPicture(t *testing.T) {
	wmPicture := func(typ byte, data []byte) []byte {
		return cat([]byte{typ}, u32le(len(data)), utf16z("image/jpeg"), utf16z(""), data)
	}
	wma, _ := wmaFileWith(make([]byte, 100), wmPicture(3, jpeg))
	cases := []struct {
		name string
		file []byte
		want []byte
		mime string
	}{
		{"front cover after another picture.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", apic(4, "image/png", png)),
			id3Frame(3, "APIC", apic(3, "image/jpeg", jpeg))), mp3Frames(20, 0x11)), jpeg, "image/jpeg"},
		{"utf-16 description.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", cat([]byte{1}, []byte("image/png\x00"), []byte{3}, []byte{0xFF, 0xFE, 'd', 0, 0, 0}, png))),
			mp3Frames(20, 0x11)), png, "image/png"},
		{"first picture without a front cover.mp3", cat(id3Tag(4,
			id3Frame(4, "APIC", apic(0, "image/png", png)),
			id3Frame(4, "APIC", apic(4, "image/jpeg", jpeg))), mp3Frames(20, 0x11)), png, "image/png"},
		{"block.flac", cat([]byte("fLaC"),
			flacBlock(0, false, streamInfo(44100, 2, 16, 441000)),
			flacBlock(6, true, flacPictureBlock(3, "image/jpeg", jpeg)), make([]byte, 100)), jpeg, "image/jpeg"},
		{"vorbis comment.opus", cat(
			mkOggPage(2, 0, 7, 0, opusHead(2, 312)),
			mkOggPage(0, 0, 7, 1, cat([]byte("OpusTags"), vorbisComment("yt-dlp",
				"METADATA_BLOCK_PICTURE="+base64.StdEncoding.EncodeToString(flacPictureBlock(3, "image/png", png))))),
			mkOggPage(4, 48312, 7, 2, make([]byte, 200))), png, "image/png"},
		{"covr.m4a", m4aFile(true, make([]byte, 100), mkAtom("covr", mp4Data(13, jpeg))), jpeg, "image/jpeg"},
		{"wm-picture.wma", wma, jpeg, "image/jpeg"},
		{"mime sniffed when the tag lies.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", apic(3, "jpg", png))), mp3Frames(20, 0x11)), png, "image/png"},
		{"upper-case mime.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", apic(3, "image/PNG", png))), mp3Frames(20, 0x11)), png, "image/png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, mime, err := Picture(writeFile(t, c.name, c.file))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, c.want) || mime != c.mime {
				t.Fatalf("got %q (%s), want %q (%s)", data, mime, c.want, c.mime)
			}
		})
	}
}

func TestPictureMissing(t *testing.T) {
	cases := map[string][]byte{
		"no picture.mp3":  cat(id3Tag(3, id3TextFrame(3, "TIT2", "Sin tapa")), mp3Frames(20, 0x11)),
		"a link.mp3":      cat(id3Tag(3, id3Frame(3, "APIC", apic(3, "-->", []byte("http://example.com/x.jpg")))), mp3Frames(20, 0x11)),
		"unsupported.ape": []byte("MAC "),
		"empty.mp3":       cat(id3Tag(3, id3Frame(3, "APIC", []byte("\x00image/jpg\x00\x00\x00"))), mp3Frames(20, 0x11)),
	}
	for name, b := range cases {
		if _, _, err := Picture(writeFile(t, name, b)); !errors.Is(err, ErrNoPicture) {
			t.Errorf("%s: err = %v, want ErrNoPicture", name, err)
		}
	}
	if _, _, err := Picture(filepath.Join(t.TempDir(), "gone.mp3")); !errors.Is(err, ErrIO) {
		t.Errorf("missing file: err = %v, want ErrIO", err)
	}
}
