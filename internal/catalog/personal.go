package catalog

import (
	"cmp"
	"slices"

	"tordo/internal/store"
)

// ratingValues[r] are the Puntaje values of an item rated r: "1" to r.
var ratingValues = [6][]string{nil, {"1"}, {"1", "2"}, {"1", "2", "3"}, {"1", "2", "3", "4"}, {"1", "2", "3", "4", "5"}}

// personalValues gives an item's Me gusta and Puntaje values, nil when it
// has none.
func personalValues(m store.Mark, ok bool) map[string][]string {
	if !ok {
		return nil
	}
	var out map[string][]string
	if m.Liked {
		out = map[string][]string{FacetLiked: {LikedYes}}
	}
	if m.Rating >= 1 && m.Rating <= 5 {
		if out == nil {
			out = map[string][]string{}
		}
		out[FacetRating] = ratingValues[m.Rating]
	}
	return out
}

// trackMark and albumMark look a mark up; p may be nil.
func trackMark(p *store.Personal, id int64) (store.Mark, bool) {
	if p == nil {
		return store.Mark{}, false
	}
	m, ok := p.Tracks[id]
	return m, ok
}

func albumMark(p *store.Personal, id int64) (store.Mark, bool) {
	if p == nil {
		return store.Mark{}, false
	}
	m, ok := p.Albums[id]
	return m, ok
}

// trackPlays gives each track's play count, only when q sorts by plays.
func (c *Catalog) trackPlays(q Query, p *store.Personal) map[int64]int {
	if q.Order != OrderPlays || p == nil {
		return nil
	}
	out := make(map[int64]int, len(p.Plays))
	for id, pl := range p.Plays {
		out[id] = pl.Count
	}
	return out
}

// albumPlays sums the plays of each album's tracks, only when q sorts by
// plays.
func (c *Catalog) albumPlays(q Query, p *store.Personal) map[int64]int {
	if q.Order != OrderPlays || p == nil {
		return nil
	}
	out := map[int64]int{}
	for i := range c.Tracks {
		t := &c.Tracks[i]
		if t.AlbumID != 0 {
			out[t.AlbumID] += p.Plays[t.ID].Count
		}
	}
	return out
}

// artistPlays sums the plays of each artist's albums, only when q sorts by
// plays.
func (c *Catalog) artistPlays(q Query, p *store.Personal) map[string]int {
	if q.Order != OrderPlays || p == nil {
		return nil
	}
	out := map[string]int{}
	for id, n := range c.albumPlays(q, p) {
		if a, ok := c.Album(id); ok && a.ArtistKey != "" {
			out[a.ArtistKey] += n
		}
	}
	return out
}

// LikedLists are the user's likes as the Listas page shows them, most
// recently liked first. Only what is in the catalog.
type LikedLists struct {
	Tracks      []Track               `json:"tracks"`
	Albums      []Album               `json:"albums"`
	Artists     []Artist              `json:"artists"`
	Collections []store.FolderSummary `json:"collections"`
}

// liked is an item with when it was liked and a key to break ties.
type liked[T any] struct {
	item T
	at   int64
	key  string
}

// sortLiked lists items most recently liked first, then by key.
func sortLiked[T any](xs []liked[T]) []T {
	slices.SortFunc(xs, func(a, b liked[T]) int { return cmp.Or(cmp.Compare(b.at, a.at), cmp.Compare(a.key, b.key)) })
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		out = append(out, x.item)
	}
	return out
}

// idKey is an id as a fixed-width string, so ids sort numerically.
func idKey(id int64) string {
	b := []byte("0000000000000000000")
	for i := len(b) - 1; id > 0 && i >= 0; i, id = i-1, id/10 {
		b[i] = byte('0' + id%10)
	}
	return string(b)
}

// Liked lists the user's liked tracks, albums, artists and collections that
// are in the catalog. p may be nil.
func (c *Catalog) Liked(p *store.Personal) LikedLists {
	var tracks []liked[Track]
	var albums []liked[Album]
	var artists []liked[Artist]
	var colls []liked[store.FolderSummary]
	if p != nil {
		for id, m := range p.Tracks {
			if t, ok := c.Track(id); ok && m.Liked {
				tracks = append(tracks, liked[Track]{*t, m.LikedAt, idKey(id)})
			}
		}
		for id, m := range p.Albums {
			if a, ok := c.Album(id); ok && m.Liked {
				albums = append(albums, liked[Album]{*a, m.LikedAt, idKey(id)})
			}
		}
		for key, at := range p.Artists {
			if a, ok := c.Artist(key); ok {
				artists = append(artists, liked[Artist]{*a, at, key})
			}
		}
		for path, at := range p.Collections {
			if f, ok := c.folders[path]; ok {
				colls = append(colls, liked[store.FolderSummary]{f.FolderSummary, at, path})
			}
		}
	}
	return LikedLists{Tracks: sortLiked(tracks), Albums: sortLiked(albums), Artists: sortLiked(artists), Collections: sortLiked(colls)}
}
