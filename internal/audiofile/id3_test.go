package audiofile

import (
	"errors"
	"testing"
)

func TestID3v23(t *testing.T) {
	tag := id3Tag(3,
		id3TextFrame(3, "TIT2", "Só pra contrariar"),
		id3TextFrame(3, "TPE1", "Noel Rosa"),
		id3TextFrame(3, "TALB", "Noel pela primeira vez"),
		id3TextFrame(3, "TRCK", "16/20"),
		id3TextFrame(3, "TPOS", "2/3"),
		id3TextFrame(3, "TYER", "2000"),
		id3TextFrame(3, "TCON", "(114)"),
		id3TextFrame(3, "TXXX", "MusicBrainz Album Id", "album-mbid"),
		id3Frame(3, "UFID", cat([]byte("http://musicbrainz.org\x00"), []byte("rec-mbid"))),
		id3Frame(3, "APIC", []byte{0, 'i', 'm', 'a', 'g', 'e', '/', 'j', 'p', 'e', 'g', 0, 3, 0, 0xFF, 0xD8}),
	)
	file := cat(tag, []byte("audio"))
	var tg Tags
	n, err := readID3v2(src(file), 0, &tg)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(tag)) {
		t.Errorf("size = %d, want %d", n, len(tag))
	}
	if tg.Title != "Só pra contrariar" || tg.Artist != "Noel Rosa" || tg.Album != "Noel pela primeira vez" {
		t.Errorf("text = %+v", tg)
	}
	if tg.Track != 16 || tg.TrackTotal != 20 || tg.Disc != 2 || tg.DiscTotal != 3 || tg.Year != 2000 {
		t.Errorf("numbers = %+v", tg)
	}
	if len(tg.Genres) != 1 || tg.Genres[0] != "Samba" {
		t.Errorf("genres = %q", tg.Genres)
	}
	if tg.MBAlbumID != "album-mbid" || tg.MBRecordingID != "rec-mbid" || !tg.HasCover {
		t.Errorf("ids/cover = %+v", tg)
	}
}

func TestID3v24MultipleValuesAndUTF8InLatin1(t *testing.T) {
	tag := id3Tag(4,
		id3TextFrame(4, "TPE1", "Caetano Veloso", "Maria Bethânia"),
		id3TextFrame(4, "TDRC", "1978-01-01"),
		id3Frame(4, "TALB", cat([]byte{0}, []byte("Tercer Mundo (Páez)"))), // UTF-8 bytes declared Latin-1
	)
	var tg Tags
	if _, err := readID3v2(src(tag), 0, &tg); err != nil {
		t.Fatal(err)
	}
	if tg.Artist != "Caetano Veloso; Maria Bethânia" {
		t.Errorf("Artist = %q", tg.Artist)
	}
	if tg.Year != 1978 {
		t.Errorf("Year = %d", tg.Year)
	}
	if tg.Album != "Tercer Mundo (Páez)" {
		t.Errorf("Album = %q", tg.Album)
	}
}

func TestID3v22(t *testing.T) {
	frame := func(id, text string) []byte {
		data := cat([]byte{0}, []byte(text))
		return cat([]byte(id), []byte{0, 0, byte(len(data))}, data)
	}
	body := cat(frame("TT2", "Milonga triste"), frame("TP1", "Alfredo Zitarrosa"), make([]byte, 10))
	tag := cat([]byte{'I', 'D', '3', 2, 0, 0}, ssafe(len(body)), body)
	var tg Tags
	if _, err := readID3v2(src(tag), 0, &tg); err != nil {
		t.Fatal(err)
	}
	if tg.Title != "Milonga triste" || tg.Artist != "Alfredo Zitarrosa" {
		t.Errorf("tags = %+v", tg)
	}
}

func TestNoID3v2(t *testing.T) {
	var tg Tags
	n, err := readID3v2(src([]byte("fLaC and more bytes")), 0, &tg)
	if n != 0 || err != nil {
		t.Fatalf("n = %d, err = %v", n, err)
	}
}

func TestTruncatedID3v2(t *testing.T) {
	tag := id3Tag(3, id3TextFrame(3, "TIT2", "x"))
	var tg Tags
	if _, err := readID3v2(src(tag[:len(tag)-5]), 0, &tg); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestID3v1FillsOnlyEmptyFields(t *testing.T) {
	file := cat([]byte("audio bytes"), id3v1("Retirada", "La Nueva Milonga", "Murgas", "1985", 7, 113))
	tg := Tags{Title: "From ID3v2"}
	n, err := readID3v1(src(file), &tg)
	if err != nil || n != 128 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	if tg.Title != "From ID3v2" || tg.Artist != "La Nueva Milonga" || tg.Album != "Murgas" || tg.Year != 1985 || tg.Track != 7 {
		t.Errorf("tags = %+v", tg)
	}
	if len(tg.Genres) != 1 || tg.Genres[0] != "Tango" {
		t.Errorf("genres = %q", tg.Genres)
	}
}

func TestAPETagSize(t *testing.T) {
	items := []byte("0123456789")
	footer := cat([]byte("APETAGEX"), u32le(2000), u32le(len(items)+32), u32le(1), u32le(1<<31), make([]byte, 8))
	header := cat([]byte("APETAGEX"), u32le(2000), u32le(len(items)+32), u32le(1), u32le(1<<31|1<<29), make([]byte, 8))
	file := cat([]byte("audio"), header, items, footer)
	n, err := apeTagSize(src(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(header)+len(items)+len(footer)) {
		t.Fatalf("n = %d", n)
	}
	if n, _ := apeTagSize(src([]byte("no tag here, just bytes, at least 32 long")), 41); n != 0 {
		t.Fatalf("n = %d without a tag", n)
	}
}
