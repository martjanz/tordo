// Package nameparse reads what folder and file names say about music:
// artist, album, year, disc, track and position in a collection. It only
// fills what the tags lack.
package nameparse

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// Norm folds a name for comparisons: lowercase, without accents or
// punctuation, single spaces. "Fito Páez" and "fito paez!" are equal.
func Norm(s string) string {
	s = accents.Replace(strings.ToLower(s))
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return b.String()
}

var various = map[string]bool{
	"va": true, "v a": true, "aavv": true, "vvaa": true, "varios": true, "various": true,
	"various artists": true, "varios artistas": true, "varios interpretes": true,
}

// IsVarious reports whether an artist name means "various artists".
func IsVarious(s string) bool { return various[Norm(s)] }

var (
	reDisc      = regexp.MustCompile(`(?i)[\s\-_,(\[]*\b(?:cd|disc|disco|disk|disque)\s*0*(\d{1,2})\b[)\]]?`)
	reShortTail = regexp.MustCompile(`^\s*-?\s*\w?\s*$`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// StripDisc removes a disc marker ("CD 2", "(Disc 1)", "Disco 3", "Cd 02 -x")
// and returns the rest and the disc number (0 when there is none). What
// follows the marker is kept, unless it is only a stray letter or dash.
func StripDisc(s string) (string, int) {
	m := reDisc.FindStringSubmatchIndex(s)
	if m == nil {
		return s, 0
	}
	disc, _ := strconv.Atoi(s[m[2]:m[3]])
	tail := s[m[1]:]
	if reShortTail.MatchString(tail) {
		tail = ""
	}
	return tidy(s[:m[0]] + " " + tail), disc
}

// tidy collapses spaces and trims separators from both ends.
func tidy(s string) string {
	return strings.Trim(reSpaces.ReplaceAllString(s, " "), " -–_,")
}
