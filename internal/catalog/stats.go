package catalog

import (
	"cmp"
	"slices"

	"tordo/internal/store"
)

// TrackPlays is a track with how many times it was played.
type TrackPlays struct {
	Track
	Plays int `json:"plays"`
}

// AlbumPlays is an album with how many times its tracks were played.
type AlbumPlays struct {
	Album
	Plays int `json:"plays"`
}

// ArtistPlays is an artist with how many times their tracks were played.
type ArtistPlays struct {
	Key   string `json:"key"` // "" when the artist has no page
	Name  string `json:"name"`
	Plays int    `json:"plays"`
}

// ValuePlays is a facet value (a country, a genre) with its plays.
type ValuePlays struct {
	Value string `json:"value"`
	Plays int    `json:"plays"`
}

// Stats ranks what was played in a period.
type Stats struct {
	Plays      int           `json:"plays"` // of the period
	ListenedMs int64         `json:"listenedMs"`
	Played     int           `json:"played"` // distinct tracks
	Tracks     []TrackPlays  `json:"tracks"`
	Albums     []AlbumPlays  `json:"albums"`
	Artists    []ArtistPlays `json:"artists"`
	Countries  []ValuePlays  `json:"countries"`
	Genres     []ValuePlays  `json:"genres"`
	Never      int           `json:"never"` // albums never played (all time)
	Stale      int           `json:"stale"` // albums not played in over a year
}

// rank sorts by plays descending, then by name, and cuts to limit.
func rank[T any](xs []T, plays func(T) int, name func(T) string, limit int) []T {
	slices.SortFunc(xs, func(a, b T) int { return cmp.Or(cmp.Compare(plays(b), plays(a)), cmp.Compare(name(a), name(b))) })
	return xs[:min(max(limit, 0), len(xs))]
}

// Stats ranks the tally t, at most limit of each list. p (all time) gives
// the albums never played or not in over a year.
func (c *Catalog) Stats(t store.PlayTally, p *store.Personal, limit int) Stats {
	s := Stats{Plays: t.Plays, ListenedMs: t.ListenedMs, Tracks: []TrackPlays{}, Albums: []AlbumPlays{}, Artists: []ArtistPlays{},
		Countries: []ValuePlays{}, Genres: []ValuePlays{}}
	var tracks []TrackPlays
	albums := map[int64]int{}
	artists := map[string]*ArtistPlays{}
	countries, genres := map[string]int{}, map[string]int{}
	for id, n := range t.Counts {
		tr, ok := c.Track(id)
		if !ok || n <= 0 {
			continue
		}
		s.Played++
		tracks = append(tracks, TrackPlays{*tr, n})
		if tr.AlbumID != 0 {
			albums[tr.AlbumID] += n
		}
		key, name := tr.ArtistKey, tr.Artist
		group := key
		if key != "" {
			if a, ok := c.Artist(key); ok {
				name = a.Name
			}
		} else if group = "\x00" + tr.anorm; tr.anorm == "" {
			group = ""
		}
		if group != "" {
			a := artists[group]
			if a == nil {
				a = &ArtistPlays{Key: key, Name: name}
				artists[group] = a
			} else if key == "" && tr.Artist < a.Name {
				a.Name = tr.Artist // the same spelling whatever the map order
			}
			a.Plays += n
		}
		for _, v := range tr.values[FacetCountry] {
			countries[v] += n
		}
		for _, v := range tr.values[FacetGenre] {
			genres[v] += n
		}
	}
	s.Tracks = rank(tracks, func(x TrackPlays) int { return x.Plays },
		func(x TrackPlays) string { return x.anorm + "\x00" + x.norm + "\x00" + idKey(x.ID) }, limit)
	if s.Tracks == nil {
		s.Tracks = []TrackPlays{}
	}
	var ap []AlbumPlays
	for id, n := range albums {
		if a, ok := c.Album(id); ok {
			ap = append(ap, AlbumPlays{*a, n})
		}
	}
	s.Albums = append(s.Albums, rank(ap, func(x AlbumPlays) int { return x.Plays },
		func(x AlbumPlays) string { return x.anorm + "\x00" + x.norm + "\x00" + idKey(x.ID) }, limit)...)
	var arts []ArtistPlays
	for _, a := range artists {
		arts = append(arts, *a)
	}
	s.Artists = append(s.Artists, rank(arts, func(x ArtistPlays) int { return x.Plays },
		func(x ArtistPlays) string { return x.Name + "\x00" + x.Key }, limit)...)
	s.Countries = append(s.Countries, rankValues(countries, limit)...)
	s.Genres = append(s.Genres, rankValues(genres, limit)...)

	l := c.listening(p)
	for i := range c.Albums {
		switch v := l.albumValues(c.Albums[i].ID); {
		case slices.Contains(v, ListenedNever):
			s.Never++
		case slices.Contains(v, ListenedStale):
			s.Stale++
		}
	}
	return s
}

func rankValues(m map[string]int, limit int) []ValuePlays {
	vs := make([]ValuePlays, 0, len(m))
	for v, n := range m {
		vs = append(vs, ValuePlays{v, n})
	}
	return rank(vs, func(x ValuePlays) int { return x.Plays }, func(x ValuePlays) string { return x.Value }, limit)
}

// ArtistTop lists an artist's most played tracks (their albums and their
// appearances), most played first; at most limit, only played ones.
func (c *Catalog) ArtistTop(key string, p *store.Personal, limit int) []TrackPlays {
	out := []TrackPlays{}
	if p == nil || key == "" {
		return out
	}
	for i := range c.Tracks {
		t := &c.Tracks[i]
		n := p.Plays[t.ID].Count
		if n <= 0 {
			continue
		}
		if al, ok := c.Album(t.AlbumID); t.ArtistKey == key || (ok && al.ArtistKey == key) {
			out = append(out, TrackPlays{*t, n})
		}
	}
	slices.SortFunc(out, func(a, b TrackPlays) int {
		return cmp.Or(cmp.Compare(b.Plays, a.Plays), cmp.Compare(a.order, b.order), cmp.Compare(a.Disc, b.Disc),
			cmp.Compare(a.TrackView.Track, b.TrackView.Track), cmp.Compare(a.ID, b.ID))
	})
	return out[:min(max(limit, 0), len(out))]
}
