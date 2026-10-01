package nameparse

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// File is what a file's name says.
type File struct {
	Disc, Track, Year int
	Artist, Title     string
}

var (
	reYouTubeID = regexp.MustCompile(`\s*\[[A-Za-z0-9_-]{11}\]$`)
	reTrack     = regexp.MustCompile(`^(?:(\d)[-.](\d{1,3})|(\d{1,3}))(?:\s*[-.)_]\s*|\s+)`)
	reCredit    = regexp.MustCompile(`(?i)^(?:c-|c/|con |com |feat\.?|ft\.?)\s*`)
	reYearOnly  = regexp.MustCompile(`^(?:19|20)\d{2}$`)
	reFileSplit = regexp.MustCompile(`\s+[-–]\s+`)
)

// ParseFile reads a file name (with its extension): "05-Fue Amor.mp3",
// "1-05 Title.flac", "07 - Curare - Nilze Carvalho.mp3". folderArtist, when
// known, tells "Title - Artist" from "Artist - Title".
func ParseFile(name, folderArtist string) File {
	name = Compose(name)
	var f File
	s := strings.TrimSuffix(name, path.Ext(name))
	s = reYouTubeID.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "⧸", "/") // yt-dlp's stand-in for '/'
	s = strings.ReplaceAll(s, "_", " ")
	if m := reTrack.FindStringSubmatchIndex(s); m != nil && m[1] < len(s) {
		if m[2] >= 0 {
			f.Disc, _ = strconv.Atoi(s[m[2]:m[3]])
			f.Track, _ = strconv.Atoi(s[m[4]:m[5]])
		} else {
			f.Track, _ = strconv.Atoi(s[m[6]:m[7]])
		}
		s = s[m[1]:]
	}
	var parts []string
	for _, p := range reFileSplit.Split(s, -1) {
		if p = tidy(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) > 1 && reYearOnly.MatchString(parts[0]) {
		f.Year, _ = strconv.Atoi(parts[0])
		parts = parts[1:]
	}
	switch {
	case len(parts) == 0:
		f.Title = tidy(s)
	case len(parts) == 1:
		f.Title = parts[0]
	default:
		last := parts[len(parts)-1]
		head := strings.Join(parts[:len(parts)-1], " - ")
		switch {
		case reCredit.MatchString(last):
			f.Title, f.Artist = head, tidy(reCredit.ReplaceAllString(last, ""))
		case isArtist(last, folderArtist):
			f.Title, f.Artist = head, last
		default:
			f.Artist, f.Title = parts[0], strings.Join(parts[1:], " - ")
		}
	}
	return f
}

// isArtist reports whether s names the folder's artist (possibly with others:
// "Silvia Barrios / Daniel Barrera").
func isArtist(s, folderArtist string) bool {
	a := Norm(folderArtist)
	return a != "" && strings.HasPrefix(Norm(s), a)
}
