package identify

import (
	"tordo/internal/genres"
	"tordo/internal/grouping"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// world is everything resolve reads: all local, no requests.
type world struct {
	matches     map[string]store.Match      // by query key
	corrections map[string]store.Correction // by fingerprint
	releases    map[string]store.Release
	artists     map[string]store.Artist
	genres      *genres.Normalizer
}

// resolve decides an album's identity from what is stored. Raw tag genres
// nothing maps are counted in unmapped when they decide the album's genres.
func resolve(t store.IdentifyTarget, q Query, w world, unmapped map[string]int) store.AlbumIdentity {
	id := store.AlbumIdentity{AlbumID: t.AlbumID, Query: q.Key()}
	if c, ok := correctionOf(t, w.corrections); ok {
		id.Status, id.ReleaseID = c.Status, c.ReleaseID
		if c.Status == store.StatusManual {
			id.Confidence = 1
		}
	} else if m, ok := w.matches[id.Query]; ok {
		id.Status, id.Confidence = m.Status, m.Confidence
		if m.Status == store.StatusAuto {
			id.ReleaseID = m.ReleaseID
		}
	}
	if id.Status == store.StatusUnmatched && t.Selection {
		id.Status = store.StatusSelection
	}
	rel, ok := w.releases[id.ReleaseID]
	identified := ok && id.ReleaseID != ""
	if identified {
		id.Title, id.Artist, id.Compilation = rel.Title, rel.Artist, rel.Various
		if rel.Various {
			id.Artist = grouping.Various
		}
		id.OriginalYear = rel.OriginalYear
		if id.OriginalYear == 0 {
			id.OriginalYear = rel.Year
		}
		id.ArtistID = mainArtist(rel)
		id.Country = countryOf(rel, w.artists)
		if !t.HasCover && rel.Cover != "" && rel.Cover != store.CoverNone {
			id.CoverCache = rel.Cover
		}
		id.Tracks = alignTracks(t.Tracks, rel.Tracks)
	}
	id.Genres = albumGenres(t, rel, identified, w.genres, unmapped)
	return id
}

// correctionOf returns the user's correction for an album: the most common
// one when at least half of its tracks carry one.
func correctionOf(t store.IdentifyTarget, cs map[string]store.Correction) (store.Correction, bool) {
	counts := map[store.Correction]int{}
	n := 0
	for _, tr := range t.Tracks {
		if c, ok := cs[tr.Fingerprint]; ok && tr.Fingerprint != "" {
			counts[c]++
			n++
		}
	}
	if n == 0 || n*2 < len(t.Tracks) {
		return store.Correction{}, false
	}
	var best store.Correction
	bn := 0
	for c, k := range counts {
		if k > bn || (k == bn && c.ReleaseID < best.ReleaseID) {
			best, bn = c, k
		}
	}
	return best, true
}

// releaseFrom keeps what tordo uses of a MusicBrainz release.
func releaseFrom(m musicbrainz.Release) store.Release {
	rel := store.Release{ID: m.ID, GroupID: m.GroupID, Title: m.Title, Artist: m.Artist, ArtistIDs: m.ArtistIDs,
		ArtistNames: m.ArtistNames, Various: m.Various, Year: m.Year, OriginalYear: m.OriginalYear, Country: m.Country,
		MBGenres: m.Genres, DiscogsURLs: m.DiscogsURLs}
	for _, t := range m.Tracks {
		rel.Tracks = append(rel.Tracks, store.ReleaseTrack{Disc: t.Disc, Position: t.Position, Title: t.Title,
			LengthMs: t.LengthMs, RecordingID: t.RecordingID})
	}
	return rel
}

// mainArtist is the first credited artist that is not Various Artists.
func mainArtist(rel store.Release) string {
	for _, id := range rel.ArtistIDs {
		if id != musicbrainz.VariousArtistsID {
			return id
		}
	}
	return ""
}

// unspecific release countries: worldwide, Europe, unknown.
var unspecific = map[string]bool{"": true, "XW": true, "XE": true, "XU": true}

// countryOf is the album's country: its main artist's, or for Various
// Artists the release's own country when it names one.
func countryOf(rel store.Release, artists map[string]store.Artist) string {
	if rel.Various {
		if unspecific[rel.Country] {
			return ""
		}
		return rel.Country
	}
	return artists[mainArtist(rel)].Country
}

// alignTracks pairs the album's tracks with the release's: by position when
// both have as many tracks, else each with the unused release track with
// the closest title. A pair needs durations within durationSlackMs, or a
// similar title when a duration is unknown.
func alignTracks(ts []store.TargetTrack, rts []store.ReleaseTrack) []store.TrackIdentity {
	pairs := func(t store.TargetTrack, r store.ReleaseTrack) bool {
		if t.DurationMs > 0 && r.LengthMs > 0 {
			return absDiff(t.DurationMs, r.LengthMs) <= durationSlackMs
		}
		return similarity(t.Title, r.Title) >= 0.6
	}
	identity := func(t store.TargetTrack, r store.ReleaseTrack) store.TrackIdentity {
		return store.TrackIdentity{FileID: t.FileID, Title: r.Title, RecordingID: r.RecordingID}
	}
	var out []store.TrackIdentity
	if len(ts) == len(rts) {
		for i, t := range ts {
			if pairs(t, rts[i]) {
				out = append(out, identity(t, rts[i]))
			}
		}
		return out
	}
	used := make([]bool, len(rts))
	for _, t := range ts {
		best := -1
		for j, r := range rts {
			if used[j] || !pairs(t, r) {
				continue
			}
			if best < 0 || similarity(t.Title, r.Title) > similarity(t.Title, rts[best].Title) {
				best = j
			}
		}
		if best >= 0 {
			used[best] = true
			out = append(out, identity(t, rts[best]))
		}
	}
	return out
}

// albumGenres takes the genres from the best source: Discogs, MusicBrainz,
// the tags.
func albumGenres(t store.IdentifyTarget, rel store.Release, identified bool, n *genres.Normalizer, unmapped map[string]int) []store.GenreLink {
	if identified {
		if ms := genres.FromDiscogs(rel.Discogs.Genres, rel.Discogs.Styles); len(ms) > 0 {
			return links(ms, "discogs")
		}
		var ms []genres.Mapping
		for _, g := range rel.MBGenres {
			m, _ := n.Map(g)
			ms = append(ms, m...)
		}
		if len(ms) > 0 {
			return links(ms, "mb")
		}
	}
	var ms []genres.Mapping
	for _, raw := range t.RawGenres {
		m, ok := n.Map(raw)
		ms = append(ms, m...)
		if !ok {
			unmapped[raw]++
		}
	}
	return links(ms, "tag")
}

func links(ms []genres.Mapping, source string) []store.GenreLink {
	var out []store.GenreLink
	seen := map[genres.Mapping]bool{}
	for _, m := range ms {
		if !seen[m] {
			seen[m] = true
			out = append(out, store.GenreLink{Genre: m.Genre, Style: m.Style, Source: source})
		}
	}
	return out
}
