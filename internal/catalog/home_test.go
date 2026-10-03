package catalog

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"tordo/internal/store"
)

func lowerRows(t *testing.T, n int) {
	old := minRowAlbums
	minRowAlbums = n
	t.Cleanup(func() { minRowAlbums = old })
}

func TestHome(t *testing.T) {
	at := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	clockAt(t, at)
	c := build()
	p := &store.Personal{
		Albums: map[int64]store.Mark{1: {Liked: true, LikedAt: 5}},
		Plays: map[int64]store.Plays{
			301: {Count: 1, Last: at.Add(-time.Hour).UnixMilli()},
			101: {Count: 2, Last: at.Add(-48 * time.Hour).UnixMilli()},
		},
	}
	h := c.Home(p, 1)
	if got := albumTitles(h.Recent); len(got) != 5 || got[0] != "Rock nacional" || got[1] != "Bocanada" {
		t.Fatalf("recent = %v", got)
	}
	if got, want := albumTitles(h.Continue), []string{"Clics modernos", "Signos"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("continue = %v want %v", got, want)
	}
	if got, want := albumTitles(h.Liked), []string{"Signos"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("liked = %v want %v", got, want)
	}
	// Too few albums for the threshold: only the random row.
	for seed := uint64(1); seed <= 20; seed++ {
		h := c.Home(p, seed)
		if len(h.Rotating) != 1 || h.Rotating[0].Kind != RowRandom || len(h.Rotating[0].Albums) != 5 || len(h.Rotating[0].Facets) != 0 {
			t.Fatalf("seed %d: %+v", seed, h.Rotating)
		}
	}

	lowerRows(t, 2)
	seen := map[string]bool{}
	for seed := uint64(1); seed <= 50; seed++ {
		h := c.Home(p, seed)
		if len(h.Rotating) != 3 {
			t.Fatalf("seed %d: %d rows", seed, len(h.Rotating))
		}
		kinds := map[string]bool{}
		for _, r := range h.Rotating {
			k := r.Kind
			if k == RowStyle {
				k = RowGenre
			}
			if kinds[k] {
				t.Fatalf("seed %d: kind %s twice", seed, k)
			}
			kinds[k] = true
			seen[k] = true
			if r.Kind == RowCountry {
				t.Fatalf("seed %d: country row", seed)
			}
			if len(r.Albums) == 0 {
				t.Fatalf("seed %d: row %s without albums", seed, r.Kind)
			}
			for _, a := range r.Albums {
				for f, v := range r.Facets {
					ok := slices.Contains(a.values[f], v)
					if f == FacetListened {
						ok = v == ListenedNever && a.Title != "Signos" && a.Title != "Clics modernos"
					}
					if !ok {
						t.Fatalf("seed %d: %s in row %+v lacks %s=%s", seed, a.Title, r, f, v)
					}
				}
			}
			if r.Kind == RowCollection && r.Label != "Latinoamérica 600" {
				t.Fatalf("label = %q", r.Label)
			}
		}
		if !reflect.DeepEqual(c.Home(p, seed), h) {
			t.Fatalf("seed %d: not deterministic", seed)
		}
	}
	for _, k := range []string{RowDecade, RowGenre, RowCollection, RowNever, RowRandom} {
		if !seen[k] {
			t.Errorf("kind %s never drawn", k)
		}
	}

	h = c.Home(nil, 3)
	if h.Continue == nil || len(h.Continue) != 0 || h.Liked == nil || len(h.Liked) != 0 {
		t.Fatalf("nil p: %+v %+v", h.Continue, h.Liked)
	}
	for seed := uint64(1); seed <= 50; seed++ {
		for _, r := range c.Home(nil, seed).Rotating {
			if r.Kind == RowNever || r.Kind == RowStale {
				t.Fatalf("nil p: row %s", r.Kind)
			}
		}
	}
}
