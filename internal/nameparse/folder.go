package nameparse

import (
	"regexp"
	"strconv"
	"strings"
)

// Folder is what a folder's name says.
type Folder struct {
	Position int // order prefix inside a collection: "[LA-599] …" → 599, "07 - …" → 7
	Artist   string
	Album    string // the whole name when there is no "Artist - Album" split
	Year     int
	Disc     int
	Various  bool // "VA - …", "AAVV - …", "Varios - …": a compilation
}

var (
	rePosBracket = regexp.MustCompile(`^\[\s*(?:[A-Za-z]{1,5}\s*[-_ ]?\s*)?(\d{1,4})\s*\]\s*`)
	rePosNumber  = regexp.MustCompile(`^(\d{1,3})\s*[.)\-_]\s*`)
	reNoise      = regexp.MustCompile(`(?i)\s*[\[(]\s*(?:\d{2,3}\s*k(?:b|bps)?|v0|v2|320|aac[^)\]]*|mp3[^)\]]*|flac[^)\]]*|ogg[^)\]]*|digital|lp|vinyl|web|cd|cdrip|hi-?res|\d{2}\s*[-/ ]\s*\d{2,3}(?:[.,]\d)?(?:\s*khz)?|\d{2}\s*bits?[^)\]]*)\s*[\])]`)
	reYearParen  = regexp.MustCompile(`\s*[\[(]((?:19|20)\d{2})[\])]`)
	reYearLead   = regexp.MustCompile(`^((?:19|20)\d{2})\s*[-–.]\s*`)
	reYearMiddle = regexp.MustCompile(`\s+[-–]\s*((?:19|20)\d{2})\s*[-–]\s*`)
	reYearTail   = regexp.MustCompile(`(?:\s*[-–]\s*|\s+)((?:19|20)\d{2})$`)
	reSeparators = regexp.MustCompile(`\s*[-–](?:\s*[-–])+\s*`)
	reSplit      = regexp.MustCompile(`\s+[-–]\s*|\s*[-–]\s+`)
)

// ParseFolder reads a folder name such as "[LA-599] Rata Blanca - (1990)
// Magos, Espadas Y Rosas" or "Noel Rosa - Noel pela primeira vez Cd 02".
func ParseFolder(name string) Folder {
	var f Folder
	s := strings.ReplaceAll(name, "_", " ")
	if m := rePosBracket.FindStringSubmatchIndex(s); m != nil {
		f.Position, _ = strconv.Atoi(s[m[2]:m[3]])
		s = s[m[1]:]
	} else if m := rePosNumber.FindStringSubmatchIndex(s); m != nil {
		f.Position, _ = strconv.Atoi(s[m[2]:m[3]])
		s = s[m[1]:]
	}
	s = reNoise.ReplaceAllString(s, "")
	s, f.Disc = StripDisc(s)
	s, f.Year = stripYear(s)
	s = tidy(reSeparators.ReplaceAllString(s, " - "))
	if loc := reSplit.FindStringIndex(s); loc != nil {
		f.Artist, f.Album = tidy(s[:loc[0]]), tidy(s[loc[1]:])
	} else {
		f.Album = s
	}
	if IsVarious(f.Artist) {
		f.Various, f.Artist = true, ""
	}
	return f
}

// stripYear takes the release year out of a name: "(1990)", "[1980]",
// "1971 - …", "… - 1961 - …" or a trailing "… 1977". Other bracketed years
// (reissues: "Beyond (2015)") are dropped.
func stripYear(s string) (string, int) {
	year := 0
	take := func(m []int) { year, _ = strconv.Atoi(s[m[2]:m[3]]) }
	if m := reYearParen.FindStringSubmatchIndex(s); m != nil {
		take(m)
		s = reYearParen.ReplaceAllString(s[:m[0]]+" "+s[m[1]:], "")
	} else if m := reYearLead.FindStringSubmatchIndex(s); m != nil {
		take(m)
		s = s[m[1]:]
	} else if m := reYearMiddle.FindStringSubmatchIndex(s); m != nil {
		take(m)
		s = s[:m[0]] + " - " + s[m[1]:]
	} else if m := reYearTail.FindStringSubmatchIndex(s); m != nil && m[0] > 0 {
		take(m)
		s = s[:m[0]]
	}
	return s, year
}
