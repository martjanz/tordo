package audiofile

import (
	"reflect"
	"testing"
)

func TestSetMapsNamesFromEveryFormat(t *testing.T) {
	var tg Tags
	tg.set("TITLE", "Fue amor")
	tg.set("title", "ignored: first value wins")
	tg.set("ARTIST", "Fito Páez")
	tg.set("artist", "fito páez") // same artist, different case: not repeated
	tg.set("ARTIST", "Charly García")
	tg.set("album_artist", "Fito Páez")
	tg.set("WM/AlbumTitle", "Tercer mundo")
	tg.set("TRACKNUMBER", "05/12")
	tg.set("DISCNUMBER", "1")
	tg.set("TOTALDISCS", "2")
	tg.set("DATE", "1990-10-01")
	tg.set("ORIGINALDATE", "1990")
	tg.set("GENRE", "Rock")
	tg.set("genre", "rock")
	tg.set("GENRE", "Latino")
	tg.set("COMPILATION", "1")
	tg.set("MusicBrainz Album Id", "album-mbid")
	tg.set("MUSICBRAINZ_TRACKID", "recording-mbid")
	tg.set("REPLAYGAIN_TRACK_GAIN", "-6.54 dB")
	tg.set("unknown", "x")
	tg.set("ALBUM", "  \x00 ")

	gain := -6.54
	want := Tags{
		Title: "Fue amor", Artist: "Fito Páez; Charly García", AlbumArtist: "Fito Páez", Album: "Tercer mundo",
		Track: 5, TrackTotal: 12, Disc: 1, DiscTotal: 2, Year: 1990, OriginalYear: 1990,
		Genres: []string{"Rock", "Latino"}, Compilation: true,
		MBAlbumID: "album-mbid", MBRecordingID: "recording-mbid", TrackGain: &gain,
	}
	if !reflect.DeepEqual(tg, want) {
		t.Fatalf("got  %+v\nwant %+v", tg, want)
	}
}

func TestParseHelpers(t *testing.T) {
	if n, total := parsePair("3/12"); n != 3 || total != 12 {
		t.Errorf("parsePair = %d, %d", n, total)
	}
	if n, total := parsePair("x"); n != 0 || total != 0 {
		t.Errorf("parsePair(x) = %d, %d", n, total)
	}
	for in, want := range map[string]int{"1990": 1990, "2008-05-01": 2008, "c. 1957": 1957, "20080501": 2008, "123456": 0, "": 0, "3000": 0} {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestID3Genres(t *testing.T) {
	cases := map[string][]string{
		"(17)":     {"Rock"},
		"(114)":    {"Samba"},
		"(17)Rock": {"Rock", "Rock"}, // addGenre removes the duplicate later
		"113":      {"Tango"},
		"(RX)":     {"Remix"},
		"Murga":    {"Murga"},
		"(999)":    nil,
	}
	for in, want := range cases {
		if got := id3Genres(in); !reflect.DeepEqual(got, want) {
			t.Errorf("id3Genres(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenreTable(t *testing.T) {
	if len(genreNames) != 192 {
		t.Fatalf("len(genreNames) = %d, want 192", len(genreNames))
	}
	for i, want := range map[int]string{17: "Rock", 86: "Latin", 113: "Tango", 114: "Samba", 115: "Folklore", 191: "Psybient"} {
		if genreNames[i] != want {
			t.Errorf("genreNames[%d] = %q, want %q", i, genreNames[i], want)
		}
	}
}
