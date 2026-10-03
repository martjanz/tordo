package catalog

import (
	"cmp"
	"math/rand/v2"
	"slices"

	"tordo/internal/store"
)

// HomeRowSize is how many albums an Inicio row shows.
const HomeRowSize = 12

// rotatingRows is how many rotating rows Inicio draws.
const rotatingRows = 3

// minRowAlbums is how many albums a rotating row needs to be drawn (the
// random row needs one); tests lower it.
var minRowAlbums = 4

// Kinds of Inicio's rotating rows.
const (
	RowDecade     = "decada"
	RowCountry    = "pais"
	RowGenre      = "genero"
	RowStyle      = "estilo"
	RowCollection = "coleccion"
	RowNever      = "nunca"
	RowStale      = "hace-un-anio"
	RowRandom     = "azar"
)

// HomeRow is a rotating row: a facet value and some of its albums.
type HomeRow struct {
	Kind   string            `json:"kind"`
	Value  string            `json:"value"`  // "1980", "BR", "Rock", a folder path; "" for nunca, hace-un-anio, azar
	Label  string            `json:"label"`  // the value's label when it is not the value (a collection's name)
	Facets map[string]string `json:"facets"` // Explorar's, for "ver todas" and the row's station (view albumes); {} for azar
	Albums []Album           `json:"albums"`
}

// Home is Inicio's rows (saved stations are the server's to add).
type Home struct {
	Recent   []Album   `json:"recent"`   // newest first
	Continue []Album   `json:"continue"` // by their latest play
	Liked    []Album   `json:"liked"`    // most recently liked first
	Rotating []HomeRow `json:"rotating"`
}

// candidate is a possible rotating row: its kind, value and the albums that
// have it.
type candidate struct {
	kind, value, label string
	facets             map[string]string
	albums             []Album
}

// Home answers Inicio: the fixed rows and three rotating ones drawn with
// seed (the same seed gives the same rows). p may be nil.
func (c *Catalog) Home(p *store.Personal, seed uint64) Home {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	l := c.listening(p)
	h := Home{Recent: []Album{}, Continue: []Album{}, Liked: []Album{}, Rotating: []HomeRow{}}

	recent := slices.Clone(c.Albums)
	slices.SortFunc(recent, func(a, b Album) int { return cmp.Or(cmp.Compare(b.Added, a.Added), cmp.Compare(b.ID, a.ID)) })
	h.Recent = recent[:min(HomeRowSize, len(recent))]

	if l != nil {
		var played []Album
		for _, a := range c.Albums {
			if pl := l.albums[a.ID]; pl.Count > 0 {
				played = append(played, a)
			}
		}
		slices.SortFunc(played, func(a, b Album) int {
			return cmp.Or(cmp.Compare(l.albums[b.ID].Last, l.albums[a.ID].Last), cmp.Compare(a.ID, b.ID))
		})
		h.Continue = append(h.Continue, played[:min(HomeRowSize, len(played))]...)
	}
	liked := c.Liked(p).Albums
	h.Liked = append(h.Liked, liked[:min(HomeRowSize, len(liked))]...)

	kinds := []string{RowDecade, RowCountry, RowGenre, RowCollection, RowNever, RowStale, RowRandom}
	r.Shuffle(len(kinds), func(i, j int) { kinds[i], kinds[j] = kinds[j], kinds[i] })
	for _, kind := range kinds {
		cands := c.candidates(kind, p, l)
		if len(cands) == 0 {
			continue
		}
		cd := cands[r.IntN(len(cands))]
		row := HomeRow{Kind: cd.kind, Value: cd.value, Label: cd.label, Facets: cd.facets, Albums: []Album{}}
		for _, i := range r.Perm(len(cd.albums))[:min(HomeRowSize, len(cd.albums))] {
			row.Albums = append(row.Albums, cd.albums[i])
		}
		h.Rotating = append(h.Rotating, row)
		if len(h.Rotating) == rotatingRows {
			break
		}
	}
	return h
}

// candidates lists the rows of a kind that qualify, in a fixed order.
func (c *Catalog) candidates(kind string, p *store.Personal, l *listening) []candidate {
	// albumsWith lists the albums that match facets, with their personal values.
	albumsWith := func(facets map[string]string) []Album {
		var out []Album
		for i := range c.Albums {
			a := &c.Albums[i]
			m, ok := albumMark(p, a.ID)
			if matches(a.values, personalValues(m, ok, l.albumValues(a.ID)), facets, "") {
				out = append(out, *a)
			}
		}
		return out
	}
	// byValue makes a candidate of each value of a catalog facet that enough
	// albums have; one pass groups the albums by the values they carry.
	byValue := func(rowKind, facet string) []candidate {
		groups := map[string][]Album{}
		var values []string
		for i := range c.Albums {
			a := &c.Albums[i]
			for j, v := range a.values[facet] {
				if slices.Contains(a.values[facet][:j], v) {
					continue
				}
				if _, ok := groups[v]; !ok {
					values = append(values, v)
				}
				groups[v] = append(groups[v], *a)
			}
		}
		slices.Sort(values)
		var out []candidate
		for _, v := range values {
			if albums := groups[v]; len(albums) >= minRowAlbums {
				cd := candidate{kind: rowKind, value: v, facets: map[string]string{facet: v}, albums: albums}
				if kind == RowCollection {
					cd.label = c.folders[v].Name
				}
				out = append(out, cd)
			}
		}
		return out
	}
	switch kind {
	case RowDecade:
		return byValue(RowDecade, FacetDecade)
	case RowCountry:
		return byValue(RowCountry, FacetCountry)
	case RowGenre:
		return append(byValue(RowGenre, FacetGenre), byValue(RowStyle, FacetStyle)...)
	case RowCollection:
		return byValue(RowCollection, FacetCollection)
	case RowNever, RowStale:
		facets := map[string]string{FacetListened: ListenedNever}
		if kind == RowStale {
			facets[FacetListened] = ListenedStale
		}
		if albums := albumsWith(facets); len(albums) >= minRowAlbums {
			return []candidate{{kind: kind, facets: facets, albums: albums}}
		}
	case RowRandom:
		if len(c.Albums) >= 1 {
			return []candidate{{kind: RowRandom, facets: map[string]string{}, albums: c.Albums}}
		}
	}
	return nil
}
