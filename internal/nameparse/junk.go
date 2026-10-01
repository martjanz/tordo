package nameparse

import (
	"regexp"
	"strings"
)

var (
	reSiteNote = regexp.MustCompile(`(?i)\s*[\[(][^)\]]*(?:www\.|https?://|\.(?:com|net|org|info|biz)\b)[^)\]]*[\])]`)
	reSite     = regexp.MustCompile(`(?i)(?:\bwww\.|https?://|\.(?:com|net|org|info|biz)\b)`)
	reFileCode = regexp.MustCompile(`^[A-Za-z]{0,4}\d{2,}[A-Za-z0-9]*(?:_[A-Za-z0-9]+)+$`)
)

// CleanTag drops what download sites add to album and artist tags: a
// bracketed site name ("Tercer Mundo (LoMusical.Com)" → "Tercer Mundo"). A
// value that is only a site or a file code ("WWW.TODOMUSICACD.8K.COM",
// "M031CAS_e_01_01") comes back empty, so the folder name is used instead.
// Not meant for track titles.
func CleanTag(s string) string {
	s = strings.TrimSpace(reSiteNote.ReplaceAllString(s, ""))
	if reSite.MatchString(s) || reFileCode.MatchString(s) {
		return ""
	}
	return s
}
