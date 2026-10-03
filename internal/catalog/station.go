package catalog

import (
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"tordo/internal/store"
)

// Station modes.
const (
	ModeTracks = "pistas"  // track by track
	ModeAlbums = "albumes" // a whole album, then another
)

// StationBatch is how many tracks a station adds at a time.
const StationBatch = 10

// Defaults and limits of the rules.
const (
	DefaultArtistGap = 3
	DefaultAvoidDays = 7
	MaxArtistGap     = 20
	MaxAvoidDays     = 365
	MaxRecent        = 200 // the most recent tracks Next takes into account
)

// maxStationName is the longest station name, in runes.
const maxStationName = 200

// Source is what a station plays from: one of an Explorar query (View and
// Facets; no facets: the whole library), a folder and everything under it,
// a playlist, or the liked music.
type Source struct {
	View     string            `json:"view,omitempty"` // ViewTracks or ViewAlbums
	Facets   map[string]string `json:"facets,omitempty"`
	Folder   string            `json:"folder,omitempty"`
	Playlist int64             `json:"playlist,omitempty"`
	Liked    bool              `json:"liked,omitempty"`
}

// Station is a source plus the rules that pace it.
type Station struct {
	Name      string `json:"name,omitempty"`
	Source    Source `json:"source"`
	Mode      string `json:"mode"`
	ArtistGap int    `json:"artistGap"` // no artist again within this many tracks; 0: no rule
	AvoidDays int    `json:"avoidDays"` // leave out what was played in the last days; 0: no rule
}

// Clean returns the station as Next reads it: one kind of source (folder,
// then playlist, then liked, then the Explorar query), well formed facets
// only, a known mode and view (pistas by default), rules within limits, the
// name trimmed and cut to 200 runes.
func (s Station) Clean() Station {
	out := Station{Mode: ModeTracks, ArtistGap: min(max(s.ArtistGap, 0), MaxArtistGap), AvoidDays: min(max(s.AvoidDays, 0), MaxAvoidDays)}
	if s.Mode == ModeAlbums {
		out.Mode = ModeAlbums
	}
	if name := []rune(strings.TrimSpace(s.Name)); len(name) > maxStationName {
		out.Name = string(name[:maxStationName])
	} else {
		out.Name = string(name)
	}
	src := s.Source
	switch folder := strings.TrimSpace(src.Folder); {
	case folder != "":
		out.Source.Folder = folder
	case src.Playlist > 0:
		out.Source.Playlist = src.Playlist
	case src.Liked:
		out.Source.Liked = true
	default:
		out.Source.View = ViewTracks
		if src.View == ViewAlbums || src.View == ViewArtists {
			out.Source.View = ViewAlbums
		}
		for _, f := range FacetNames {
			if v, ok := src.Facets[f]; ok && valid(f, v) {
				if out.Source.Facets == nil {
					out.Source.Facets = map[string]string{}
				}
				out.Source.Facets[f] = v
			}
		}
	}
	return out
}

// StationInput is what the next tracks depend on besides the station.
type StationInput struct {
	Personal *store.Personal // nil: no user data (no weights, nothing to avoid)
	Playlist []int64         // the playlist's available tracks, for a playlist source
	Recent   []int64         // the tracks the station queued so far, oldest first (the last MaxRecent count)
	Rand     *rand.Rand      // math/rand/v2
}

// Next picks a station's next tracks: about count of them, by album in
// albums mode. Empty when the source has nothing playable. The station is
// taken as is (the caller cleans it).
func (c *Catalog) Next(st Station, in StationInput, count int) []int64 {
	if count <= 0 {
		return nil
	}
	if len(in.Recent) > MaxRecent {
		in.Recent = in.Recent[len(in.Recent)-MaxRecent:]
	}
	if in.Rand == nil {
		in.Rand = rand.New(rand.NewPCG(uint64(now().UnixNano()), 1))
	}
	pool := c.pool(st.Source, in.Personal, in.Playlist)
	if len(pool) == 0 {
		return nil
	}
	if st.Mode == ModeAlbums {
		return c.nextAlbums(st, in, pool, count)
	}
	return c.nextTracks(st, in, pool, count)
}

// pool lists the catalog's tracks of a source that can be played, by file id.
func (c *Catalog) pool(src Source, p *store.Personal, playlist []int64) []*Track {
	var out []*Track
	add := func(t *Track) {
		if !slices.Contains(t.values[FacetState], StateUnplayable) {
			out = append(out, t)
		}
	}
	switch {
	case src.Folder != "":
		for i := range c.Tracks {
			if t := &c.Tracks[i]; t.Dir == src.Folder || strings.HasPrefix(t.Dir, src.Folder+"/") {
				add(t)
			}
		}
	case src.Playlist > 0:
		seen := map[int64]bool{}
		for _, id := range playlist {
			if t, ok := c.Track(id); ok && !seen[id] {
				seen[id] = true
				add(t)
			}
		}
	case src.Liked:
		if p == nil {
			return nil
		}
		for i := range c.Tracks {
			if t := &c.Tracks[i]; c.likedTrack(p, t) {
				add(t)
			}
		}
	case src.View == ViewAlbums:
		l := c.listening(p)
		for i := range c.Tracks {
			t := &c.Tracks[i]
			a, ok := c.Album(t.AlbumID)
			if !ok {
				continue
			}
			m, ok := albumMark(p, a.ID)
			if matches(a.values, personalValues(m, ok, l.albumValues(a.ID)), src.Facets, "") {
				add(t)
			}
		}
	default:
		l := c.listening(p)
		for i := range c.Tracks {
			t := &c.Tracks[i]
			m, ok := trackMark(p, t.ID)
			if matches(t.values, personalValues(m, ok, l.trackValues(t.ID)), src.Facets, "") {
				add(t)
			}
		}
	}
	return out
}

// likedTrack tells whether the liked music has t: the track, its album, its
// artist (or its album's) or a collection above it is liked.
func (c *Catalog) likedTrack(p *store.Personal, t *Track) bool {
	if m, ok := p.Tracks[t.ID]; ok && m.Liked {
		return true
	}
	if likedArtist(p, t.ArtistKey) {
		return true
	}
	if a, ok := c.Album(t.AlbumID); ok {
		if m, ok := p.Albums[a.ID]; ok && m.Liked {
			return true
		}
		if likedArtist(p, a.ArtistKey) {
			return true
		}
	}
	for path := range p.Collections {
		if t.Dir == path || strings.HasPrefix(t.Dir, path+"/") {
			return true
		}
	}
	return false
}

// ratingWeight[r] scales the weight of something rated r (0: not rated).
var ratingWeight = [6]float64{1, 0.25, 0.5, 1, 2, 3}

// weight is what liking and rating make of a base weight of 1: ×3 liked,
// ×2 by a liked artist, and by the best rating.
func weight(likedItem, likedArtist bool, rating int) float64 {
	w := 1.0
	if likedItem {
		w *= 3
	}
	if likedArtist {
		w *= 2
	}
	if rating >= 0 && rating < len(ratingWeight) {
		w *= ratingWeight[rating]
	}
	return w
}

func likedArtist(p *store.Personal, key string) bool {
	_, ok := p.Artists[key]
	return key != "" && ok
}

// trackWeight is how likely a track is to be picked: the track's and its
// album's like and rating, and its artist. 1 without user data.
func trackWeight(p *store.Personal, c *Catalog, t *Track) float64 {
	if p == nil {
		return 1
	}
	tm := p.Tracks[t.ID]
	liked, rating, artist := tm.Liked, tm.Rating, likedArtist(p, t.ArtistKey)
	if a, ok := c.Album(t.AlbumID); ok {
		am := p.Albums[a.ID]
		liked, rating = liked || am.Liked, max(rating, am.Rating)
		artist = artist || likedArtist(p, a.ArtistKey)
	}
	return weight(liked, artist, rating)
}

// albumWeight is how likely an album is to be picked (albums mode).
func albumWeight(p *store.Personal, a *Album) float64 {
	if p == nil {
		return 1
	}
	m := p.Albums[a.ID]
	return weight(m.Liked, likedArtist(p, a.ArtistKey), m.Rating)
}

// artistOf identifies a track's artist for the gap: its key, else its name
// (an artist the catalog has no page for).
func artistOf(t *Track) string {
	if t.ArtistKey != "" {
		return t.ArtistKey
	}
	return "~" + t.anorm
}

// albumArtistOf identifies the artist of a track's album (albums mode);
// compilations and albums without an artist are their own.
func (c *Catalog) albumArtistOf(t *Track) string {
	a, ok := c.Album(t.AlbumID)
	if !ok {
		return artistOf(t)
	}
	if a.ArtistKey != "" {
		return a.ArtistKey
	}
	return "album:" + idKey(a.ID)
}

// playedSince gives the tracks played in the last days.
func (c *Catalog) playedSince(p *store.Personal, days int) map[int64]bool {
	out := map[int64]bool{}
	if p == nil || days <= 0 {
		return out
	}
	cutoff := now().Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()
	for id, pl := range p.Plays {
		if pl.Count > 0 && pl.Last >= cutoff {
			out[id] = true
		}
	}
	return out
}

// nextTracks picks count tracks of pool one at a time, each from the first
// non-empty tier of candidates: every rule, then without the artist gap,
// then without avoiding recent plays, then allowing what the station
// already queued (but not the last track, when there is another).
func (c *Catalog) nextTracks(st Station, in StationInput, pool []*Track, count int) []int64 {
	queued := map[int64]bool{}
	var history []string // artists of the queued tracks, oldest first
	for _, id := range in.Recent {
		queued[id] = true
		if t, ok := c.Track(id); ok {
			history = append(history, artistOf(t))
		}
	}
	last := int64(0)
	if len(in.Recent) > 0 {
		last = in.Recent[len(in.Recent)-1]
	}
	weights := make([]float64, len(pool))
	for i, t := range pool {
		weights[i] = trackWeight(in.Personal, c, t)
	}
	avoid := c.playedSince(in.Personal, st.AvoidDays)
	var out []int64
	for len(out) < count {
		gap := map[string]bool{}
		for _, a := range history[max(0, len(history)-st.ArtistGap):] {
			gap[a] = true
		}
		tiers := []func(t *Track) bool{
			func(t *Track) bool { return !queued[t.ID] && !avoid[t.ID] && !gap[artistOf(t)] },
			func(t *Track) bool { return !queued[t.ID] && !avoid[t.ID] },
			func(t *Track) bool { return !queued[t.ID] },
			func(t *Track) bool { return t.ID != last || len(pool) == 1 },
		}
		i := -1
		for _, ok := range tiers {
			if i = pick(pool, weights, ok, in.Rand); i >= 0 {
				break
			}
		}
		if i < 0 {
			break
		}
		t := pool[i]
		out = append(out, t.ID)
		queued[t.ID], last = true, t.ID
		history = append(history, artistOf(t))
	}
	return out
}

// pick chooses an index of items among those ok accepts, with probability
// proportional to its weight; -1 when ok accepts none.
func pick[T any](items []T, weights []float64, ok func(T) bool, r *rand.Rand) int {
	total := 0.0
	for i, t := range items {
		if ok(t) {
			total += weights[i]
		}
	}
	if total <= 0 {
		return -1
	}
	x := r.Float64() * total
	for i, t := range items {
		if ok(t) {
			if x -= weights[i]; x < 0 {
				return i
			}
		}
	}
	for i := len(items) - 1; i >= 0; i-- { // rounding: the last accepted one
		if ok(items[i]) {
			return i
		}
	}
	return -1
}

// albumGroup is the pool's tracks of one album, in album order (a loose
// track: a group of its own).
type albumGroup struct {
	tracks []*Track
	artist string
}

// nextAlbums picks whole albums, by the same tiers as nextTracks over
// albums, until the batch holds at least count tracks.
func (c *Catalog) nextAlbums(st Station, in StationInput, pool []*Track, count int) []int64 {
	var groups []*albumGroup
	byAlbum := map[int64]*albumGroup{}
	of := map[int64]*albumGroup{} // track id → its group
	for _, t := range pool {
		g := byAlbum[t.AlbumID]
		if g == nil || t.AlbumID == 0 {
			g = &albumGroup{artist: c.albumArtistOf(t)}
			if t.AlbumID != 0 {
				byAlbum[t.AlbumID] = g
			}
			groups = append(groups, g)
		}
		g.tracks = append(g.tracks, t)
		of[t.ID] = g
	}
	weights := make([]float64, len(groups))
	for i, g := range groups {
		weights[i] = trackWeight(in.Personal, c, g.tracks[0])
		if a, ok := c.Album(g.tracks[0].AlbumID); ok {
			weights[i] = albumWeight(in.Personal, a)
		}
		slices.SortStableFunc(g.tracks, func(a, b *Track) int { return strings.Compare(a.order, b.order) })
	}
	avoidTracks := c.playedSince(in.Personal, st.AvoidDays)
	avoid := map[*albumGroup]bool{}
	for id := range avoidTracks {
		if g := of[id]; g != nil {
			avoid[g] = true
		}
	}
	queued := map[*albumGroup]bool{}
	var history []string
	var lastGroup *albumGroup
	for _, id := range in.Recent {
		if t, ok := c.Track(id); ok {
			history = append(history, c.albumArtistOf(t))
		}
		if g := of[id]; g != nil {
			queued[g], lastGroup = true, g
		} else {
			lastGroup = nil
		}
	}
	var out []int64
	for len(out) < count {
		gap := map[string]bool{}
		for _, a := range history[max(0, len(history)-st.ArtistGap):] {
			gap[a] = true
		}
		tiers := []func(g *albumGroup) bool{
			func(g *albumGroup) bool { return !queued[g] && !avoid[g] && !gap[g.artist] },
			func(g *albumGroup) bool { return !queued[g] && !avoid[g] },
			func(g *albumGroup) bool { return !queued[g] },
			func(g *albumGroup) bool { return g != lastGroup || len(groups) == 1 },
		}
		i := -1
		for _, ok := range tiers {
			if i = pick(groups, weights, ok, in.Rand); i >= 0 {
				break
			}
		}
		if i < 0 {
			break
		}
		g := groups[i]
		queued[g], lastGroup = true, g
		for _, t := range g.tracks {
			out = append(out, t.ID)
			history = append(history, g.artist)
		}
	}
	return out
}
