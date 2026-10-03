// Package catalog turns a snapshot of the store into what Explorar, the
// artist pages and the search show: albums, artists and tracks with the
// values of their facets. Everything here is pure: no SQL, no network.
package catalog

import (
	"cmp"
	"path"
	"slices"
	"strconv"
	"strings"

	"tordo/internal/grouping"
	"tordo/internal/nameparse"
	"tordo/internal/store"
)

// Album is one album card of Explorar.
type Album struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	ArtistKey   string `json:"artistKey"` // "" for compilations and albums without an artist
	Year        int    `json:"year"`      // the original year when known, else the tags'
	Compilation bool   `json:"compilation"`
	Tracks      int    `json:"tracks"`
	DurationMs  int64  `json:"durationMs"`
	Picture     bool   `json:"picture"`
	Country     string `json:"country"`
	Status      string `json:"status"`
	Dir         string `json:"dir"`
	Added       int64  `json:"added"` // the first time one of its files was seen, unix ms

	values map[string][]string // by facet
	labels map[string]string   // facet + "\x00" + value → label, for values whose label is not the value
	norm   string              // title as compared when sorting
	anorm  string              // artist as compared when sorting
	tagged string              // the title and artist in its tags, when they differ (for the search)
}

// Artist is one artist card of Explorar: the album artist of one or more
// albums (compilations have none).
type Artist struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Albums  int    `json:"albums"`
	Cover   int64  `json:"cover"` // an album with a picture, for the card; 0 without one
	Added   int64  `json:"added"`

	mbid  string
	norm  string
	years []int
	names []string // every spelling of its name in the albums
}

// Track is one row of Explorar's Pistas.
type Track struct {
	store.TrackView
	AlbumID   int64  `json:"albumId"` // 0: a loose track
	Album     string `json:"album"`
	Dir       string `json:"dir"`
	ArtistKey string `json:"artistKey"` // the track artist's page, when there is one
	Added     int64  `json:"added"`

	values map[string][]string
	labels map[string]string
	norm   string
	anorm  string
	order  string // album, disc and track, for the album order of equal years
}

// Options are what building the catalog needs besides the snapshot.
type Options struct {
	// Playable tells whether the server can deliver a track (formats that
	// need ffmpeg are not playable without it). Nil: everything is.
	Playable func(store.TrackView) bool
}

// Catalog is a snapshot arranged for the pages.
type Catalog struct {
	Albums  []Album  // by id
	Artists []Artist // by name
	Tracks  []Track  // by file id
	Changes int64    // the snapshot's change counter

	albumIndex  map[int64]int
	artistIndex map[string]int
	trackIndex  map[int64]int
	artistNames map[string]string // normalized name → artist key, for track artists
	folders     map[string]store.SnapFolder
	snapArtists map[string]store.Artist
	dups        Duplicates
}

// Build arranges a snapshot.
func Build(snap store.Snapshot, opt Options) *Catalog {
	c := &Catalog{Changes: snap.Changes, albumIndex: map[int64]int{}, artistIndex: map[string]int{}, trackIndex: map[int64]int{},
		folders: map[string]store.SnapFolder{}, snapArtists: snap.Artists}
	for _, f := range snap.Folders {
		c.folders[f.Path] = f
	}
	tracksOf := map[int64][]store.SnapTrack{}
	for _, t := range snap.Tracks {
		if t.AlbumID != 0 {
			tracksOf[t.AlbumID] = append(tracksOf[t.AlbumID], t)
		}
	}
	keys := artistKeys(snap.Albums, snap.Artists)
	c.Albums = make([]Album, 0, len(snap.Albums))
	for _, a := range snap.Albums {
		c.albumIndex[a.ID] = len(c.Albums)
		c.Albums = append(c.Albums, c.album(a, keys[a.ID], tracksOf[a.ID], opt))
	}
	c.buildArtists(snap.Artists)
	c.Tracks = make([]Track, 0, len(snap.Tracks))
	for _, t := range snap.Tracks {
		c.trackIndex[t.ID] = len(c.Tracks)
		c.Tracks = append(c.Tracks, c.track(t, opt))
	}
	c.computeDuplicates(snap)
	return c
}

// Track returns a track by id.
func (c *Catalog) Track(id int64) (*Track, bool) {
	i, ok := c.trackIndex[id]
	if !ok {
		return nil, false
	}
	return &c.Tracks[i], true
}

// Album returns an album by id.
func (c *Catalog) Album(id int64) (*Album, bool) {
	i, ok := c.albumIndex[id]
	if !ok {
		return nil, false
	}
	return &c.Albums[i], true
}

// Artist returns an artist by key.
func (c *Catalog) Artist(key string) (*Artist, bool) {
	i, ok := c.artistIndex[key]
	if !ok {
		return nil, false
	}
	return &c.Artists[i], true
}

// isCompilation tells albums credited to no one artist.
func isCompilation(a store.AlbumView) bool {
	return a.Compilation || a.Artist == grouping.Various || nameparse.IsVarious(a.Artist)
}

// nameKey is the key of an artist known only by name: its folded name with
// dashes ("fito-paez"). "" when the name has no letters or digits.
func nameKey(name string) string {
	return strings.ReplaceAll(nameparse.Norm(name), " ", "-")
}

// artistKeys gives each album's artist a key: the MusicBrainz id when the
// album is identified, else the folded name. An album known only by name
// joins the MusicBrainz artist whose name (MusicBrainz's, or as their albums
// credit or tag it) is that name, when there is exactly one.
func artistKeys(albums []store.AlbumView, mb map[string]store.Artist) map[int64]string {
	byName := map[string]map[string]bool{} // folded name → MusicBrainz ids
	note := func(name, id string) {
		n := nameKey(name)
		if n == "" {
			return
		}
		if byName[n] == nil {
			byName[n] = map[string]bool{}
		}
		byName[n][id] = true
	}
	for _, a := range albums {
		if a.ArtistID != "" && !isCompilation(a) {
			note(a.Artist, a.ArtistID)
			note(a.TagArtist, a.ArtistID)
			note(mb[a.ArtistID].Name, a.ArtistID)
		}
	}
	out := map[int64]string{}
	for _, a := range albums {
		switch {
		case isCompilation(a):
		case a.ArtistID != "":
			out[a.ID] = a.ArtistID
		default:
			n := nameKey(a.Artist)
			if ids := byName[n]; len(ids) == 1 {
				for id := range ids {
					out[a.ID] = id
				}
			} else if n != "" {
				out[a.ID] = n
			}
		}
	}
	return out
}

func (c *Catalog) album(a store.AlbumView, key string, tracks []store.SnapTrack, opt Options) Album {
	year := a.OriginalYear
	if year == 0 {
		year = a.Year
	}
	artist := a.Artist
	if isCompilation(a) {
		artist = grouping.Various
	}
	out := Album{ID: a.ID, Title: a.Title, Artist: artist, ArtistKey: key, Year: year, Compilation: isCompilation(a),
		Tracks: a.Tracks, DurationMs: a.DurationMs, Picture: a.Picture, Country: a.Country, Status: a.Status, Dir: a.Dir,
		values: map[string][]string{}, labels: map[string]string{},
		norm: nameparse.Norm(a.Title), anorm: nameparse.Norm(artist)}
	if a.TagTitle != a.Title || a.TagArtist != a.Artist {
		out.tagged = a.TagTitle + " " + a.TagArtist
	}
	for _, t := range tracks {
		if out.Added == 0 || (t.Added > 0 && t.Added < out.Added) {
			out.Added = t.Added
		}
	}
	add := out.add
	if year > 0 {
		add(FacetDecade, strconv.Itoa(year/10*10), "")
		add(FacetYear, strconv.Itoa(year), "")
	}
	if key != "" {
		add(FacetArtist, key, "") // labelled with the artist's name once artists are built
	}
	if a.Country != "" {
		add(FacetCountry, a.Country, "")
	}
	for _, g := range a.Genres {
		add(FacetGenre, g.Genre, "")
		if g.Style != "" {
			add(FacetStyle, g.Style, "")
		}
	}
	for _, p := range c.collectionsOf(path.Dir(a.Dir)) {
		add(FacetCollection, p, c.folders[p].Name)
	}
	for _, p := range c.places(a.Dir) {
		add(FacetFolder, p, c.placeLabel(p))
	}
	unplayable := false
	for _, t := range tracks {
		c.formatValues(add, t.TrackView)
		if opt.Playable != nil && !opt.Playable(t.TrackView) {
			unplayable = true
		}
	}
	if a.Status == store.StatusUnmatched {
		add(FacetState, StateUnidentified, "")
	}
	if unplayable {
		add(FacetState, StateUnplayable, "")
	}
	return out
}

func (c *Catalog) formatValues(add func(facet, value, label string), t store.TrackView) {
	if t.Lossless {
		add(FacetQuality, QualityLossless, "")
	} else {
		add(FacetQuality, QualityLossy, "")
	}
	if k := FormatKey(t.Format, t.Codec); k != "" {
		add(FacetFormat, k, "")
	}
}

// add records a facet value once, with its label when it is not the value.
func (a *Album) add(facet, value, label string) { addValue(a.values, a.labels, facet, value, label) }
func (t *Track) add(facet, value, label string) { addValue(t.values, t.labels, facet, value, label) }

func addValue(values map[string][]string, labels map[string]string, facet, value, label string) {
	if slices.Contains(values[facet], value) {
		return
	}
	values[facet] = append(values[facet], value)
	if label != "" && label != value {
		labels[facet+"\x00"+value] = label
	}
}

// collectionsOf lists the collections among dir and the folders above it.
func (c *Catalog) collectionsOf(dir string) []string {
	var out []string
	for f, ok := c.folders[dir]; ok; f, ok = c.folders[f.Parent] {
		if f.Collection {
			out = append(out, f.Path)
		}
	}
	return out
}

// places gives the Carpeta values of a folder: the root it is under and,
// below the root, its first-level folder.
func (c *Catalog) places(dir string) []string {
	var chain []string // from dir up to its root
	for f, ok := c.folders[dir]; ok; f, ok = c.folders[f.Parent] {
		chain = append(chain, f.Path)
	}
	slices.Reverse(chain)
	return chain[:min(2, len(chain))]
}

// placeLabel names a Carpeta value: "musica", "musica/Brasil".
func (c *Catalog) placeLabel(p string) string {
	f := c.folders[p]
	if f.Parent == "" {
		return f.Name
	}
	return c.folders[f.Parent].Name + "/" + f.Name
}

// buildArtists makes one artist per album artist key, named as MusicBrainz
// names it or, without an id, as most of its albums spell it.
func (c *Catalog) buildArtists(mb map[string]store.Artist) {
	type acc struct {
		Artist
		spellings map[string]int
	}
	byKey := map[string]*acc{}
	var order []string
	for i := range c.Albums {
		al := &c.Albums[i]
		if al.ArtistKey == "" {
			continue
		}
		a := byKey[al.ArtistKey]
		if a == nil {
			a = &acc{Artist: Artist{Key: al.ArtistKey}, spellings: map[string]int{}}
			if m, ok := mb[al.ArtistKey]; ok {
				a.mbid, a.Name, a.Country = m.ID, m.Name, m.Country
			}
			byKey[al.ArtistKey] = a
			order = append(order, al.ArtistKey)
		}
		a.Albums++
		a.spellings[al.Artist]++
		if a.Cover == 0 && al.Picture {
			a.Cover = al.ID
		}
		if a.Country == "" {
			a.Country = al.Country
		}
		if al.Added > a.Added {
			a.Added = al.Added
		}
		if al.Year > 0 {
			a.years = append(a.years, al.Year)
		}
	}
	c.artistNames = map[string]string{}
	for _, k := range order {
		a := byKey[k]
		if a.Name == "" {
			best := 0
			for name, n := range a.spellings {
				if n > best || (n == best && name < a.Name) {
					a.Name, best = name, n
				}
			}
		}
		a.norm = nameparse.Norm(a.Name)
		for name := range a.spellings {
			c.artistNames[nameparse.Norm(name)] = k
			if name != a.Name {
				a.names = append(a.names, name)
			}
		}
		slices.Sort(a.names)
		c.Artists = append(c.Artists, a.Artist)
		c.artistNames[a.norm] = k
	}
	slices.SortFunc(c.Artists, func(x, y Artist) int { return cmp.Or(cmp.Compare(x.norm, y.norm), cmp.Compare(x.Key, y.Key)) })
	for i := range c.Artists {
		c.artistIndex[c.Artists[i].Key] = i
	}
	for i := range c.Albums {
		if k := c.Albums[i].ArtistKey; k != "" {
			c.Albums[i].labels[FacetArtist+"\x00"+k] = byKey[k].Name
		}
	}
}

func (c *Catalog) track(t store.SnapTrack, opt Options) Track {
	out := Track{TrackView: t.TrackView, AlbumID: t.AlbumID, Dir: t.Dir, Added: t.Added, values: map[string][]string{},
		labels: map[string]string{}, norm: nameparse.Norm(t.Title), anorm: nameparse.Norm(t.Artist)}
	out.TrackView.Path = "" // not needed by the pages
	out.TrackView.Fingerprint = ""
	out.ArtistKey = c.artistNames[out.anorm]
	al, ok := c.Album(t.AlbumID)
	if ok {
		out.Album = al.Title
		out.order = al.norm + "\x00" + strconv.Itoa(al.Year)
		for f, vs := range al.values {
			if f == FacetQuality || f == FacetFormat || f == FacetState {
				continue // the track's own
			}
			for _, v := range vs {
				out.add(f, v, al.labels[f+"\x00"+v])
			}
		}
	} else {
		if t.Year > 0 {
			out.add(FacetDecade, strconv.Itoa(t.Year/10*10), "")
			out.add(FacetYear, strconv.Itoa(t.Year), "")
		}
		for _, p := range c.collectionsOf(t.Dir) {
			out.add(FacetCollection, p, c.folders[p].Name)
		}
		for _, p := range c.places(t.Dir) {
			out.add(FacetFolder, p, c.placeLabel(p))
		}
	}
	if out.ArtistKey != "" {
		if a, ok := c.Artist(out.ArtistKey); ok {
			out.add(FacetArtist, out.ArtistKey, a.Name)
		}
	}
	if ok && al.Year > 0 {
		out.Year = al.Year // the album's (original) year, as the album shows it
	}
	out.order += "\x00" + strconv.Itoa(1000+t.Disc) + strconv.Itoa(10000+t.Track)
	c.formatValues(out.add, t.TrackView)
	if ok && al.Status == store.StatusUnmatched {
		out.add(FacetState, StateUnidentified, "")
	}
	if opt.Playable != nil && !opt.Playable(t.TrackView) {
		out.add(FacetState, StateUnplayable, "")
	}
	return out
}

// FormatKey names a track's format for the Formato facet: its codec, its
// container for PCM ("wav", "aiff"), "wma" for every WMA flavor.
func FormatKey(format, codec string) string {
	c := strings.ToLower(codec)
	switch {
	case strings.HasPrefix(c, "pcm"):
		return strings.ToLower(format)
	case strings.HasPrefix(c, "wma"):
		return "wma"
	case c == "":
		return strings.ToLower(format)
	}
	return c
}
