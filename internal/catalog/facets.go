package catalog

import (
	"cmp"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"tordo/internal/genres"
	"tordo/internal/store"
)

// Facet names, as they appear in Explorar's URL.
const (
	FacetDecade     = "decada"
	FacetYear       = "anio"
	FacetArtist     = "artista"   // an artist key
	FacetCountry    = "pais"      // ISO 3166-1 alpha-2
	FacetGenre      = "genero"    // a genre of the taxonomy
	FacetStyle      = "estilo"    // a style of the taxonomy
	FacetCollection = "coleccion" // a collection's folder path
	FacetFolder     = "carpeta"   // a root, or a first-level folder under it
	FacetQuality    = "calidad"   // QualityLossless or QualityLossy
	FacetFormat     = "formato"   // FormatKey
	FacetState      = "estado"    // StateUnidentified or StateUnplayable
	FacetLiked      = "megusta"   // LikedYes: the album or track itself is liked
	FacetRating     = "puntaje"   // "1"–"5": rated that many stars or more
)

// FacetNames lists the facets in the order of Explorar's bar.
var FacetNames = []string{FacetDecade, FacetYear, FacetArtist, FacetCountry, FacetGenre, FacetStyle, FacetCollection,
	FacetFolder, FacetQuality, FacetFormat, FacetState, FacetLiked, FacetRating, FacetListened}

// Values of FacetQuality and FacetState.
const (
	QualityLossless   = "sin-perdida"
	QualityLossy      = "con-perdida"
	StateUnidentified = "sin-identificar"
	StateUnplayable   = "no-reproducible"
	LikedYes          = "si"
)

// Explorar's tabs.
const (
	ViewAlbums  = "albumes"
	ViewArtists = "artistas"
	ViewTracks  = "pistas"
)

// Sort orders and directions. Artists sort by name (OrderTitle) or by
// their number of albums (OrderCount).
const (
	OrderYear   = "anio"
	OrderTitle  = "titulo"
	OrderArtist = "artista"
	OrderAdded  = "agregado"
	OrderCount  = "cantidad"
	OrderPlays  = "escuchado"
	Asc         = "asc"
	Desc        = "desc"
)

// Query is what Explorar asks for: a tab, one value per facet, and an order.
type Query struct {
	View   string            `json:"view"`
	Facets map[string]string `json:"facets"`
	Order  string            `json:"order"`
	Dir    string            `json:"dir"`
}

var (
	numberRe  = regexp.MustCompile(`^[1-9][0-9]{0,3}$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
)

// valid reports whether value is well formed for a facet. A well formed value
// that matches nothing is still applied (and finds nothing).
func valid(facet, value string) bool {
	switch facet {
	case FacetDecade:
		n, err := strconv.Atoi(value)
		return err == nil && numberRe.MatchString(value) && n%10 == 0
	case FacetYear:
		return numberRe.MatchString(value)
	case FacetCountry:
		return countryRe.MatchString(value)
	case FacetQuality:
		return value == QualityLossless || value == QualityLossy
	case FacetState:
		return value == StateUnidentified || value == StateUnplayable
	case FacetLiked:
		return value == LikedYes
	case FacetRating:
		return len(value) == 1 && value[0] >= '1' && value[0] <= '5'
	case FacetListened:
		return value == ListenedNever || value == ListenedStale
	}
	return strings.TrimSpace(value) != "" && len(value) <= 1024
}

// orders lists the orders of a tab, its default first.
func orders(view string) []string {
	switch view {
	case ViewArtists:
		return []string{OrderTitle, OrderCount, OrderAdded, OrderPlays}
	case ViewTracks:
		return []string{OrderArtist, OrderYear, OrderTitle, OrderAdded, OrderPlays}
	}
	return []string{OrderYear, OrderTitle, OrderArtist, OrderAdded, OrderPlays}
}

// naturalDir is an order's usual direction: A to Z for names, newest or
// largest first otherwise.
func naturalDir(order string) string {
	if order == OrderTitle || order == OrderArtist {
		return Asc
	}
	return Desc
}

// ParseQuery reads Explorar's parameters: ver, the facets, orden and dir.
// Unknown parameters and malformed values are dropped; each tab has its
// default order, and each order its natural direction.
func ParseQuery(v url.Values) Query {
	q := Query{View: ViewAlbums, Facets: map[string]string{}}
	if w := v.Get("ver"); w == ViewArtists || w == ViewTracks {
		q.View = w
	}
	for _, f := range FacetNames {
		if val := v.Get(f); valid(f, val) {
			q.Facets[f] = val
		}
	}
	os := orders(q.View)
	q.Order = os[0]
	if o := v.Get("orden"); slices.Contains(os, o) {
		q.Order = o
	}
	q.Dir = naturalDir(q.Order)
	if d := v.Get("dir"); d == Asc || d == Desc {
		q.Dir = d
	}
	return q
}

// matches reports whether values, or extra (the user's data), has every
// facet but except.
func matches(values, extra map[string][]string, facets map[string]string, except string) bool {
	for f, want := range facets {
		// The decade's menu also ignores the year, and the genre's the
		// style: choosing another decade or genre drops them.
		if f == except || (except == FacetDecade && f == FacetYear) || (except == FacetGenre && f == FacetStyle) {
			continue
		}
		if !slices.Contains(values[f], want) && !slices.Contains(extra[f], want) {
			return false
		}
	}
	return true
}

// FacetValue is one option of a facet's menu.
type FacetValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// counter counts facet values over the items of one tab.
type counter struct {
	counts map[string]map[string]int
	labels map[string]string
}

func newCounter() *counter {
	return &counter{counts: map[string]map[string]int{}, labels: map[string]string{}}
}

// add counts an item's values for every facet whose menu it belongs to: the
// item matches every other facet.
func (k *counter) add(values, extra map[string][]string, labels map[string]string, facets map[string]string) {
	for _, f := range FacetNames {
		if !matches(values, extra, facets, f) {
			continue
		}
		if k.counts[f] == nil {
			k.counts[f] = map[string]int{}
		}
		for _, vs := range [][]string{values[f], extra[f]} {
			for _, v := range vs {
				k.counts[f][v]++
				if l, ok := labels[f+"\x00"+v]; ok {
					k.labels[f+"\x00"+v] = l
				}
			}
		}
	}
}

// result lists each facet's values: years and decades newest first, the
// rest most frequent first.
func (k *counter) result() map[string][]FacetValue {
	out := map[string][]FacetValue{}
	for _, f := range FacetNames {
		vs := []FacetValue{}
		for v, n := range k.counts[f] {
			l, ok := k.labels[f+"\x00"+v]
			if !ok {
				l = v
			}
			vs = append(vs, FacetValue{Value: v, Label: l, Count: n})
		}
		slices.SortFunc(vs, func(a, b FacetValue) int {
			if f == FacetDecade || f == FacetYear {
				return cmp.Compare(b.Value, a.Value)
			}
			return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label)), cmp.Compare(a.Value, b.Value))
		})
		out[f] = vs
	}
	return out
}

// Result is a page of Explorar: the items of the tab that match the query
// (Albums, Artists or Tracks), how many there are in all, and every facet's
// values counted over the other facets.
type Result struct {
	Query   Query                   `json:"query"`
	Total   int                     `json:"total"`
	Offset  int                     `json:"offset"`
	Albums  []Album                 `json:"albums,omitempty"`
	Artists []Artist                `json:"artists,omitempty"`
	Tracks  []Track                 `json:"tracks,omitempty"`
	Facets  map[string][]FacetValue `json:"facets"`
}

// Explore answers Explorar without the user's data.
func (c *Catalog) Explore(q Query, offset, limit int) Result {
	return c.ExploreFor(q, nil, offset, limit)
}

// ExploreFor answers Explorar: the items of q's tab from offset, at most
// limit, with the user's data (nil: none) for the Me gusta and Puntaje
// facets and the Más escuchado order. The Artistas tab lists the album
// artists of the matching albums, with how many of them each has; its menus
// count albums.
func (c *Catalog) ExploreFor(q Query, p *store.Personal, offset, limit int) Result {
	r := Result{Query: q, Offset: offset}
	k := newCounter()
	l := c.listening(p)
	switch q.View {
	case ViewTracks:
		var found []Track
		for i := range c.Tracks {
			t := &c.Tracks[i]
			m, ok := trackMark(p, t.ID)
			extra := personalValues(m, ok, l.trackValues(t.ID))
			k.add(t.values, extra, t.labels, q.Facets)
			if matches(t.values, extra, q.Facets, "") {
				found = append(found, *t)
			}
		}
		sortTracks(found, q.Order, q.Dir, c.trackPlays(q, p))
		r.Total, r.Tracks = len(found), page(found, offset, limit)
	default:
		var found []Album
		for i := range c.Albums {
			a := &c.Albums[i]
			m, ok := albumMark(p, a.ID)
			extra := personalValues(m, ok, l.albumValues(a.ID))
			k.add(a.values, extra, a.labels, q.Facets)
			if matches(a.values, extra, q.Facets, "") {
				found = append(found, *a)
			}
		}
		if q.View == ViewArtists {
			artists := c.artistsOf(found)
			sortArtists(artists, q.Order, q.Dir, c.artistPlays(q, p))
			r.Total, r.Artists = len(artists), page(artists, offset, limit)
		} else {
			sortAlbums(found, q.Order, q.Dir, c.albumPlays(q, p))
			r.Total, r.Albums = len(found), page(found, offset, limit)
		}
	}
	r.Facets = k.result()
	if g := q.Facets[FacetGenre]; g != "" {
		// An album may have several genres: list only the chosen one's styles.
		r.Facets[FacetStyle] = slices.DeleteFunc(r.Facets[FacetStyle], func(v FacetValue) bool { return !genres.Valid(g, v.Value) })
	}
	return r
}

// artistsOf lists the album artists of albums, counting their albums.
func (c *Catalog) artistsOf(albums []Album) []Artist {
	n := map[string]int{}
	var keys []string
	for _, a := range albums {
		if a.ArtistKey == "" {
			continue
		}
		if n[a.ArtistKey] == 0 {
			keys = append(keys, a.ArtistKey)
		}
		n[a.ArtistKey]++
	}
	out := make([]Artist, 0, len(keys))
	for _, key := range keys {
		if a, ok := c.Artist(key); ok {
			cp := *a
			cp.Albums = n[key]
			out = append(out, cp)
		}
	}
	return out
}

// page returns items[offset:offset+limit], never nil.
func page[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return []T{}
	}
	return items[offset:min(len(items), offset+limit)]
}

// direction applies dir to a comparison.
func direction(c int, dir string) int {
	if dir == Desc {
		return -c
	}
	return c
}

// sortAlbums orders albums; albums without a year go last when sorting by
// year. Ties fall back to artist, year, title and id, so pages are stable.
func sortAlbums(as []Album, order, dir string, plays map[int64]int) {
	slices.SortStableFunc(as, func(a, b Album) int {
		if order == OrderYear && (a.Year == 0) != (b.Year == 0) {
			return cmp.Compare(b.Year, a.Year) // the one with a year first
		}
		var c int
		switch order {
		case OrderTitle:
			c = cmp.Compare(a.norm, b.norm)
		case OrderArtist:
			c = cmp.Or(cmp.Compare(a.anorm, b.anorm), cmp.Compare(a.Year, b.Year))
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		case OrderPlays:
			c = cmp.Compare(plays[a.ID], plays[b.ID])
		default:
			c = cmp.Compare(a.Year, b.Year)
		}
		return cmp.Or(direction(c, dir), cmp.Compare(a.anorm, b.anorm), cmp.Compare(a.Year, b.Year),
			cmp.Compare(a.norm, b.norm), cmp.Compare(a.ID, b.ID))
	})
}

func sortArtists(as []Artist, order, dir string, plays map[string]int) {
	slices.SortStableFunc(as, func(a, b Artist) int {
		var c int
		switch order {
		case OrderCount:
			c = cmp.Compare(a.Albums, b.Albums)
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		case OrderPlays:
			c = cmp.Compare(plays[a.Key], plays[b.Key])
		default:
			c = cmp.Compare(a.norm, b.norm)
		}
		return cmp.Or(direction(c, dir), cmp.Compare(a.norm, b.norm), cmp.Compare(a.Key, b.Key))
	})
}

// sortTracks orders tracks; by artist, an artist's tracks come by year and
// in album order.
func sortTracks(ts []Track, order, dir string, plays map[int64]int) {
	slices.SortStableFunc(ts, func(a, b Track) int {
		if order == OrderYear && (a.Year == 0) != (b.Year == 0) {
			return cmp.Compare(b.Year, a.Year)
		}
		var c int
		switch order {
		case OrderTitle:
			c = cmp.Compare(a.norm, b.norm)
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		case OrderYear:
			c = cmp.Compare(a.Year, b.Year)
		case OrderPlays:
			c = cmp.Compare(plays[a.ID], plays[b.ID])
		default:
			c = cmp.Compare(a.anorm, b.anorm)
		}
		return cmp.Or(direction(c, dir), cmp.Compare(a.anorm, b.anorm), cmp.Compare(a.Year, b.Year),
			cmp.Compare(a.order, b.order), cmp.Compare(a.norm, b.norm), cmp.Compare(a.ID, b.ID))
	})
}
