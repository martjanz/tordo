package audiofile

import (
	"errors"
	"reflect"
	"testing"
)

func flacFile(audio []byte) []byte {
	return cat([]byte("fLaC"),
		flacBlock(0, false, streamInfo(44100, 2, 16, 441000)),
		flacBlock(4, false, vorbisComment("reference libFLAC",
			"TITLE=Camelô", "ARTIST=Billy Blanco", "ARTIST=Miltinho", "ALBUMARTIST=Billy Blanco",
			"ALBUM=História da Música Popular Brasileira", "TRACKNUMBER=6", "TRACKTOTAL=12",
			"DATE=1971", "GENRE=Samba", "GENRE=MPB", "COMPILATION=1",
			"MUSICBRAINZ_TRACKID=rec-mbid", "REPLAYGAIN_TRACK_GAIN=-3.10 dB", "METADATA_BLOCK_PICTURE=AAAA")),
		flacBlock(6, false, []byte{0, 0, 0, 3}),
		flacBlock(1, true, make([]byte, 100)), // PADDING
		audio)
}

func TestFLAC(t *testing.T) {
	audio := make([]byte, 5000)
	file := flacFile(audio)
	info, err := readFLAC(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Codec != "flac" || !info.Lossless || info.SampleRate != 44100 || info.Channels != 2 || info.BitDepth != 16 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 10000 {
		t.Errorf("DurationMs = %d, want 10000", info.DurationMs)
	}
	if info.Payload != (Range{int64(len(file) - len(audio)), int64(len(audio))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	tg := info.Tags
	if tg.Title != "Camelô" || tg.Artist != "Billy Blanco; Miltinho" || tg.AlbumArtist != "Billy Blanco" || tg.Track != 6 || tg.TrackTotal != 12 || tg.Year != 1971 {
		t.Errorf("tags = %+v", tg)
	}
	if !reflect.DeepEqual(tg.Genres, []string{"Samba", "MPB"}) || !tg.Compilation || tg.MBRecordingID != "rec-mbid" || !tg.HasCover {
		t.Errorf("tags = %+v", tg)
	}
	if tg.TrackGain == nil || *tg.TrackGain != -3.10 {
		t.Errorf("TrackGain = %v", tg.TrackGain)
	}
}

func TestFLACWithLeadingID3(t *testing.T) {
	tag := id3Tag(3, id3TextFrame(3, "TIT2", "From ID3"))
	info, err := readFLAC(src(cat(tag, flacFile(make([]byte, 10)))))
	if err != nil {
		t.Fatal(err)
	}
	if info.Tags.Title != "From ID3" { // ID3 is read first: its value wins
		t.Errorf("Title = %q", info.Tags.Title)
	}
}

func TestFLACBadMagic(t *testing.T) {
	if _, err := readFLAC(src([]byte("OggS not a flac file"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestVorbisCommentTruncated(t *testing.T) {
	b := vorbisComment("v", "TITLE=x")
	var tg Tags
	if err := parseVorbisComment(b[:len(b)-3], &tg); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
