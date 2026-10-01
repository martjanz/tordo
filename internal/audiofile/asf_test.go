package audiofile

import (
	"errors"
	"reflect"
	"testing"
)

func TestWMA(t *testing.T) {
	payload := make([]byte, 3000)
	file, headerLen := wmaFile(payload)
	info, err := readASF(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "asf" || info.Codec != "wma" || info.Lossless || info.Channels != 2 || info.SampleRate != 44100 || info.Bitrate != 128 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 5000 {
		t.Errorf("DurationMs = %d, want 5000 (play duration minus preroll)", info.DurationMs)
	}
	if info.Payload != (Range{int64(headerLen + 50), int64(len(payload))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	tg := info.Tags
	if tg.Title != "Guitarra negra" || tg.Artist != "Alfredo Zitarrosa" || tg.Album != "Guitarra negra" || tg.Track != 5 || !tg.HasCover {
		t.Errorf("tags = %+v", tg)
	}
	if !reflect.DeepEqual(tg.Genres, []string{"Folklore"}) {
		t.Errorf("Genres = %q", tg.Genres)
	}
}

func TestGUIDByteOrder(t *testing.T) {
	g := guid("75B22630-668E-11CF-A6D9-00AA0062CE6C")
	want := [16]byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	if g != want {
		t.Fatalf("guid = % X", g)
	}
}

func TestASFNotASF(t *testing.T) {
	if _, err := readASF(src(make([]byte, 64))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
