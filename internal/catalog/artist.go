package catalog

import (
	"cmp"
	"slices"
)

// ArtistPage is what the Artista page shows.
type ArtistPage struct {
	Artist
	MBID        string       `json:"mbid"` // "" when known only by name
	Type        string       `json:"type"` // MusicBrainz's: Person, Group…
	Area        string       `json:"area"`
	Begin       string       `json:"begin"` // MusicBrainz dates: "1982", "1951-10-23"
	End         string       `json:"end"`
	FirstYear   int          `json:"firstYear"` // of its albums, 0 when unknown
	LastYear    int          `json:"lastYear"`
	Albums      []Album      `json:"albums"`      // its own, oldest first
	Appearances []Appearance `json:"appearances"` // its tracks on others' albums (compilations)
}

// Appearance is an album of others with tracks by the artist.
type Appearance struct {
	Album  Album   `json:"album"`
	Tracks []Track `json:"tracks"`
}

// ArtistPage returns an artist's page.
func (c *Catalog) ArtistPage(key string) (ArtistPage, bool) {
	a, ok := c.Artist(key)
	if !ok {
		return ArtistPage{}, false
	}
	p := ArtistPage{Artist: *a, Albums: []Album{}, Appearances: []Appearance{}}
	if m, ok := c.snapArtists[a.mbid]; ok && a.mbid != "" {
		p.MBID, p.Type, p.Area, p.Begin, p.End = m.ID, m.Type, m.Area, m.Begin, m.End
	}
	for _, al := range c.Albums {
		if al.ArtistKey == key {
			p.Albums = append(p.Albums, al)
		}
	}
	sortAlbums(p.Albums, OrderYear, Asc, nil)
	for _, y := range a.years {
		if p.FirstYear == 0 || y < p.FirstYear {
			p.FirstYear = y
		}
		p.LastYear = max(p.LastYear, y)
	}
	byAlbum := map[int64]int{}
	for _, t := range c.Tracks {
		if t.ArtistKey != key || t.AlbumID == 0 {
			continue
		}
		al, ok := c.Album(t.AlbumID)
		if !ok || al.ArtistKey == key {
			continue
		}
		i, seen := byAlbum[al.ID]
		if !seen {
			i = len(p.Appearances)
			byAlbum[al.ID] = i
			p.Appearances = append(p.Appearances, Appearance{Album: *al})
		}
		p.Appearances[i].Tracks = append(p.Appearances[i].Tracks, t)
	}
	slices.SortStableFunc(p.Appearances, func(x, y Appearance) int {
		return cmp.Or(cmp.Compare(x.Album.Year, y.Album.Year), cmp.Compare(x.Album.norm, y.Album.norm), cmp.Compare(x.Album.ID, y.Album.ID))
	})
	for _, ap := range p.Appearances {
		slices.SortStableFunc(ap.Tracks, func(x, y Track) int { return cmp.Compare(x.order, y.order) })
	}
	return p, true
}
