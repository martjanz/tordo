package audiofile

import (
	"errors"
	"reflect"
	"testing"
)

func TestM4A(t *testing.T) {
	payload := make([]byte, 16000)
	file := m4aFile(true, payload,
		mp4Text("\xa9nam", "Alcachofa"),
		mp4Text("\xa9ART", "Ricardo Villalobos"),
		mp4Text("aART", "Ricardo Villalobos"),
		mp4Text("\xa9alb", "Alcachofa"),
		mp4Text("\xa9day", "2003-05-12"),
		mkAtom("trkn", mp4Data(0, []byte{0, 0, 0, 3, 0, 12, 0, 0})),
		mkAtom("disk", mp4Data(0, []byte{0, 0, 0, 1, 0, 2})),
		mkAtom("gnre", mp4Data(0, u16be(53))), // ID3 index 52 + 1
		mkAtom("cpil", mp4Data(21, []byte{1})),
		mkAtom("covr", mp4Data(13, []byte{0xFF, 0xD8})),
		mkAtom("----", mkAtom("mean", u32be(0), []byte("com.apple.iTunes")),
			mkAtom("name", u32be(0), []byte("MusicBrainz Album Id")), mp4Data(1, []byte("album-mbid"))),
	)
	info, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "mp4" || info.Codec != "aac" || info.Lossless || info.Channels != 2 || info.SampleRate != 44100 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 5000 || info.Bitrate != 25 {
		t.Errorf("DurationMs = %d, Bitrate = %d", info.DurationMs, info.Bitrate)
	}
	if info.Payload != (Range{int64(len(file) - len(payload)), int64(len(payload))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	tg := info.Tags
	if tg.Title != "Alcachofa" || tg.Artist != "Ricardo Villalobos" || tg.AlbumArtist != "Ricardo Villalobos" || tg.Year != 2003 {
		t.Errorf("tags = %+v", tg)
	}
	if tg.Track != 3 || tg.TrackTotal != 12 || tg.Disc != 1 || tg.DiscTotal != 2 || !tg.Compilation || !tg.HasCover || tg.MBAlbumID != "album-mbid" {
		t.Errorf("tags = %+v", tg)
	}
	if !reflect.DeepEqual(tg.Genres, []string{"Electronic"}) {
		t.Errorf("Genres = %q", tg.Genres)
	}
}

func TestM4AQuickTimeMeta(t *testing.T) {
	info, err := readMP4(src(m4aFile(false, make([]byte, 10), mp4Text("\xa9nam", "Sin versión"))))
	if err != nil {
		t.Fatal(err)
	}
	if info.Tags.Title != "Sin versión" {
		t.Errorf("Title = %q", info.Tags.Title)
	}
}

func TestFragmentedM4A(t *testing.T) {
	first, second := make([]byte, 30000), make([]byte, 20000)
	file := fragmentedM4A(first, second)
	info, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Codec != "flac" || !info.Lossless || info.BitDepth != 16 || info.SampleRate != 48000 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 5000 || info.Bitrate != 80 { // (30000 + 20000) bytes * 8 / 5000 ms
		t.Errorf("DurationMs = %d, Bitrate = %d", info.DurationMs, info.Bitrate)
	}
	if info.Payload.Length != 30000 {
		t.Errorf("Payload = %+v, want the largest mdat", info.Payload)
	}
}

func TestMP4WithoutMoov(t *testing.T) {
	file := cat(mkAtom("ftyp", []byte("M4A "), u32be(0)), mkAtom("mdat", make([]byte, 10)))
	if _, err := readMP4(src(file)); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
