package catalog

import (
	"net/url"
	"reflect"
	"slices"
	"testing"
	"time"

	"tordo/internal/store"
)

// clockAt fixes the catalog's clock until the test ends.
func clockAt(t *testing.T, at time.Time) {
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
}

func TestParseQueryListened(t *testing.T) {
	q := ParseQuery(url.Values{"escucha": {"nunca"}})
	if q.Facets[FacetListened] != ListenedNever {
		t.Fatalf("nunca: %v", q.Facets)
	}
	q = ParseQuery(url.Values{"escucha": {"hace-un-anio"}})
	if q.Facets[FacetListened] != ListenedStale {
		t.Fatalf("hace-un-anio: %v", q.Facets)
	}
	if q = ParseQuery(url.Values{"escucha": {"x"}}); len(q.Facets) != 0 {
		t.Fatalf("x kept: %v", q.Facets)
	}
}

func TestExploreListened(t *testing.T) {
	at := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	clockAt(t, at)
	c := build()
	p := &store.Personal{Plays: map[int64]store.Plays{
		101: {Count: 3, Last: at.Add(-24 * time.Hour).UnixMilli()},
		201: {Count: 1, Last: at.Add(-2 * 365 * 24 * time.Hour).UnixMilli()},
	}}
	albums := func(v string, p *store.Personal) []string {
		r := c.ExploreFor(Query{View: ViewAlbums, Facets: map[string]string{FacetListened: v}, Order: OrderTitle, Dir: Asc}, p, 0, 50)
		return albumTitles(r.Albums)
	}
	if got, want := albums(ListenedNever, p), []string{"Bocanada", "Clics modernos", "Rock nacional"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nunca: %v want %v", got, want)
	}
	if got, want := albums(ListenedStale, p), []string{"Doble vida"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hace-un-anio: %v want %v", got, want)
	}
	if got := albums(ListenedNever, nil); len(got) != 0 {
		t.Fatalf("nil p: %v", got)
	}
	r := c.ExploreFor(Query{View: ViewAlbums, Facets: map[string]string{}, Order: OrderTitle, Dir: Asc}, p, 0, 50)
	want := []FacetValue{{ListenedNever, ListenedNever, 3}, {ListenedStale, ListenedStale, 1}}
	got := r.Facets[FacetListened]
	slices.SortFunc(got, func(a, b FacetValue) int { return -len(a.Value) + len(b.Value) })
	if len(got) != 2 || got[0].Count+got[1].Count != 4 {
		t.Fatalf("menu %v want %v", got, want)
	}
	for _, v := range got {
		if (v.Value == ListenedNever && v.Count != 3) || (v.Value == ListenedStale && v.Count != 1) {
			t.Fatalf("menu %v", got)
		}
	}
	r = c.ExploreFor(Query{View: ViewTracks, Facets: map[string]string{FacetListened: ListenedNever}, Order: OrderTitle, Dir: Asc}, p, 0, 50)
	ids := []int64{}
	for _, tr := range r.Tracks {
		ids = append(ids, tr.ID)
	}
	slices.Sort(ids)
	if want := []int64{102, 301, 401, 501, 502, 901}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("tracks: %v want %v", ids, want)
	}
}
