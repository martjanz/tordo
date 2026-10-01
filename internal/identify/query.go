// Package identify identifies albums on MusicBrainz, enriches them
// (MusicBrainz, Discogs, Cover Art Archive) and applies the result, the
// user's corrections first, to the catalog.
package identify

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"tordo/internal/musicbrainz"
	"tordo/internal/nameparse"
	"tordo/internal/store"
)

// MatcherVersion changes when matching changes: queries matched by an older
// version are matched again.
const MatcherVersion = 1

// Variant is one way of searching an album.
type Variant struct {
	Title   string
	Artist  string // "" when Various or unknown
	Various bool
}

// Query describes an album for the matcher.
type Query struct {
	Variants  []Variant // searched in order
	Year      int
	Durations []int64 // the album's tracks in disc/track order, ms
	ReleaseID string  // the tags' MusicBrainz album id, when most tracks agree on one
}

// BuildQuery describes an album: as grouped, as its folder names it, and
// both without disc markers or bracketed notes. Compilations search Various
// Artists.
func BuildQuery(t store.IdentifyTarget) Query {
	q := Query{Year: t.Year, ReleaseID: commonReleaseID(t.Tracks)}
	for _, tr := range t.Tracks {
		q.Durations = append(q.Durations, tr.DurationMs)
	}
	various := t.Compilation || nameparse.IsVarious(t.Artist)
	artist := clean(t.Artist)
	if various {
		artist = ""
	}
	f := nameparse.ParseFolder(path.Base(t.Dir))
	fVarious := various || f.Various
	fArtist := clean(f.Artist)
	if fArtist == "" {
		fArtist = artist
	}
	if fVarious {
		fArtist = ""
	}
	seen := map[string]bool{}
	add := func(title, artist string, various bool) {
		title = clean(title)
		k := fmt.Sprintf("%s\x00%s\x00%t", nameparse.Norm(title), nameparse.Norm(artist), various)
		if nameparse.Norm(title) == "" || seen[k] {
			return
		}
		seen[k] = true
		q.Variants = append(q.Variants, Variant{Title: title, Artist: artist, Various: various})
	}
	add(t.Title, artist, various)
	add(f.Album, fArtist, fVarious)
	add(stripTitle(clean(t.Title)), artist, various)
	add(stripTitle(clean(f.Album)), fArtist, fVarious)
	return q
}

// Key identifies the query: a stored match is reused while the key stays.
func (q Query) Key() string {
	var b strings.Builder
	for _, v := range q.Variants {
		fmt.Fprintf(&b, "%s/%s/%t;", nameparse.Norm(v.Title), nameparse.Norm(v.Artist), v.Various)
	}
	fmt.Fprintf(&b, "|%d|%s|", q.Year, q.ReleaseID)
	for i, d := range q.Durations {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%d", (d+500)/1000)
	}
	return b.String()
}

// clean composes a name and drops it when it is a download site or a code.
func clean(s string) string { return nameparse.CleanTag(nameparse.Compose(s)) }

var reNotes = regexp.MustCompile(`\s*[(\[][^)\]]*[)\]]`)

// stripTitle drops disc markers and bracketed notes: "Signos (Remastered)".
func stripTitle(s string) string {
	s, _ = nameparse.StripDisc(s)
	return strings.TrimSpace(reNotes.ReplaceAllString(s, ""))
}

// commonReleaseID is the MusicBrainz album id most of the tracks carry.
func commonReleaseID(ts []store.TargetTrack) string {
	n := map[string]int{}
	for _, t := range ts {
		if musicbrainz.ValidID(t.MBAlbumID) {
			n[t.MBAlbumID]++
		}
	}
	for id, c := range n {
		if c*2 > len(ts) {
			return id
		}
	}
	return ""
}
