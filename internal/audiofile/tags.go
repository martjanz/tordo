package audiofile

import (
	"regexp"
	"strconv"
	"strings"
)

// fold reduces a tag name to lowercase letters and digits, so "ALBUM ARTIST",
// "album_artist" and "AlbumArtist" are the same, and "WM/AlbumTitle" becomes
// "wmalbumtitle".
func fold(name string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(name) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// set applies one tag by name. Names from every format (Vorbis comments,
// ID3 TXXX descriptions, MP4 freeform names, ASF attributes, ffprobe keys)
// are folded and mapped to fields; unknown names are ignored. The first value
// of a field wins, except artist and genre, which keep every distinct value.
func (t *Tags) set(name, value string) {
	value = clean(value)
	if value == "" {
		return
	}
	switch fold(name) {
	case "title":
		first(&t.Title, value)
	case "artist", "author":
		t.Artist = addValue(t.Artist, value)
	case "albumartist", "wmalbumartist":
		first(&t.AlbumArtist, value)
	case "album", "wmalbumtitle":
		first(&t.Album, value)
	case "tracknumber", "track", "wmtracknumber":
		n, total := parsePair(value)
		firstInt(&t.Track, n)
		firstInt(&t.TrackTotal, total)
	case "tracktotal", "totaltracks":
		firstInt(&t.TrackTotal, atoi(value))
	case "discnumber", "disc", "wmpartofset":
		n, total := parsePair(value)
		firstInt(&t.Disc, n)
		firstInt(&t.DiscTotal, total)
	case "disctotal", "totaldiscs":
		firstInt(&t.DiscTotal, atoi(value))
	case "date", "year", "wmyear":
		firstInt(&t.Year, parseYear(value))
	case "originaldate", "originalyear", "wmoriginalreleaseyear":
		firstInt(&t.OriginalYear, parseYear(value))
	case "genre", "wmgenre":
		t.addGenre(value)
	case "compilation", "wmiscompilation":
		t.Compilation = t.Compilation || value == "1" || strings.EqualFold(value, "true")
	case "musicbrainztrackid":
		first(&t.MBRecordingID, value)
	case "musicbrainzalbumid":
		first(&t.MBAlbumID, value)
	case "musicbrainzartistid":
		first(&t.MBArtistID, value)
	case "replaygaintrackgain":
		if t.TrackGain == nil {
			t.TrackGain = parseGain(value)
		}
	case "replaygainalbumgain":
		if t.AlbumGain == nil {
			t.AlbumGain = parseGain(value)
		}
	}
}

func first(dst *string, v string) {
	if *dst == "" {
		*dst = v
	}
}

func firstInt(dst *int, v int) {
	if *dst == 0 {
		*dst = v
	}
}

const valueSep = "; "

// addValue appends v to a "; "-joined list unless it is already there.
func addValue(cur, v string) string {
	if cur == "" {
		return v
	}
	for _, x := range strings.Split(cur, valueSep) {
		if strings.EqualFold(x, v) {
			return cur
		}
	}
	return cur + valueSep + v
}

func (t *Tags) addGenre(v string) {
	v = clean(v)
	if v == "" {
		return
	}
	for _, g := range t.Genres {
		if strings.EqualFold(g, v) {
			return
		}
	}
	t.Genres = append(t.Genres, v)
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// parsePair reads "3/12" (number and total) or "3".
func parsePair(s string) (int, int) {
	a, b, _ := strings.Cut(s, "/")
	return atoi(a), atoi(b)
}

var yearRe = regexp.MustCompile(`(?:^|\D)(1\d{3}|20\d{2})(?:\d{4})?(?:\D|$)`)

// parseYear finds a year (1000–2099) in a date: "1990", "2008-05-01",
// "20231105" (yt-dlp writes upload dates like that).
func parseYear(s string) int {
	m := yearRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// parseGain reads a ReplayGain value: "-6.54 dB".
func parseGain(s string) *float64 {
	s = strings.TrimSpace(s)
	if len(s) > 2 && strings.EqualFold(s[len(s)-2:], "db") {
		s = strings.TrimSpace(s[:len(s)-2])
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}
