package catalog

import (
	"net/url"
	"reflect"
	"slices"
	"testing"

	"tordo/internal/store"
)

func TestParseQueryPersonal(t *testing.T) {
	q := ParseQuery(url.Values{"megusta": {"si"}, "puntaje": {"3"}})
	if q.Facets[FacetLiked] != "si" || q.Facets[FacetRating] != "3" {
		t.Fatalf("facets = %v", q.Facets)
	}
	for _, v := range []url.Values{{"megusta": {"no"}}, {"puntaje": {"0"}}, {"puntaje": {"6"}}, {"puntaje": {"x"}}, {"puntaje": {"33"}}} {
		if q := ParseQuery(v); len(q.Facets) != 0 {
			t.Fatalf("%v kept: %v", v, q.Facets)
		}
	}
	for _, view := range []string{ViewAlbums, ViewArtists, ViewTracks} {
		q := ParseQuery(url.Values{"ver": {view}, "orden": {"escuchado"}})
		if q.Order != OrderPlays || q.Dir != Desc {
			t.Fatalf("%s: order %q dir %q", view, q.Order, q.Dir)
		}
	}
}

func TestExploreLiked(t *testing.T) {
	c := build()
	p := &store.Personal{Albums: map[int64]store.Mark{1: {Liked: true, LikedAt: 10}, 3: {Rating: 3}}}
	r := c.ExploreFor(ParseQuery(url.Values{"megusta": {"si"}}), p, 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Signos"}) {
		t.Fatalf("megusta = %v", got)
	}
	r = c.ExploreFor(ParseQuery(url.Values{"puntaje": {"2"}}), p, 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Clics modernos"}) {
		t.Fatalf("puntaje=2 = %v", got)
	}
	if r = c.ExploreFor(ParseQuery(url.Values{"puntaje": {"4"}}), p, 0, 100); r.Total != 0 {
		t.Fatalf("puntaje=4 = %v", albumTitles(r.Albums))
	}
	r = c.ExploreFor(ParseQuery(nil), p, 0, 100)
	if got := r.Facets[FacetRating]; len(got) != 3 {
		t.Fatalf("puntaje menu = %v", got)
	}
	for _, v := range r.Facets[FacetRating] {
		if v.Count != 1 || v.Label != v.Value {
			t.Fatalf("puntaje value = %+v", v)
		}
	}
	if got := r.Facets[FacetLiked]; len(got) != 1 || got[0].Value != "si" || got[0].Count != 1 {
		t.Fatalf("megusta menu = %v", got)
	}
	q := ParseQuery(url.Values{"decada": {"1980"}, "ver": {"pistas"}})
	if !reflect.DeepEqual(c.ExploreFor(q, nil, 0, 50), c.Explore(q, 0, 50)) {
		t.Fatal("ExploreFor(nil) differs from Explore")
	}
	if r := c.ExploreFor(ParseQuery(url.Values{"megusta": {"si"}}), nil, 0, 10); r.Total != 0 {
		t.Fatalf("no data, megusta = %d", r.Total)
	}
}

func TestExploreLikedTracks(t *testing.T) {
	c := build()
	p := &store.Personal{Tracks: map[int64]store.Mark{102: {Liked: true}}, Albums: map[int64]store.Mark{2: {Liked: true}}}
	r := c.ExploreFor(ParseQuery(url.Values{"ver": {"pistas"}, "megusta": {"si"}}), p, 0, 100)
	if r.Total != 1 || r.Tracks[0].ID != 102 {
		t.Fatalf("tracks = %+v", r.Tracks)
	}
	// The liked album's own track (201) is not liked.
	for _, tr := range r.Tracks {
		if tr.ID == 201 {
			t.Fatal("a liked album's track matched")
		}
	}
}

func TestOrderByPlays(t *testing.T) {
	c := build()
	// Track 401 (album 4, Charly) 5 plays; track 201 (album 2, Soda) 2 plays.
	p := &store.Personal{Plays: map[int64]store.Plays{401: {Count: 5}, 201: {Count: 2}}}
	ids := func(r Result) []int64 {
		var out []int64
		for _, a := range r.Albums {
			out = append(out, a.ID)
		}
		return out
	}
	r := c.ExploreFor(ParseQuery(url.Values{"orden": {"escuchado"}}), p, 0, 100)
	got := ids(r)
	if !slices.Equal(got[:2], []int64{4, 2}) || len(got) != 5 {
		t.Fatalf("albums = %v", got)
	}
	// The rest by the usual tie-breakers: artist, year, title.
	if !slices.Equal(got[2:], []int64{3, 1, 5}) {
		t.Fatalf("albums tail = %v", got[2:])
	}
	r = c.ExploreFor(ParseQuery(url.Values{"orden": {"escuchado"}, "dir": {"asc"}}), p, 0, 100)
	if got := ids(r); got[len(got)-1] != 4 || got[len(got)-2] != 2 {
		t.Fatalf("asc albums = %v", got)
	}

	r = c.ExploreFor(ParseQuery(url.Values{"ver": {"pistas"}, "orden": {"escuchado"}}), p, 0, 100)
	if r.Tracks[0].ID != 401 || r.Tracks[1].ID != 201 {
		t.Fatalf("tracks = %v, %v", r.Tracks[0].ID, r.Tracks[1].ID)
	}
	r = c.ExploreFor(ParseQuery(url.Values{"ver": {"pistas"}, "orden": {"escuchado"}, "dir": {"asc"}}), p, 0, 100)
	if r.Tracks[len(r.Tracks)-1].ID != 401 {
		t.Fatalf("asc tracks last = %v", r.Tracks[len(r.Tracks)-1].ID)
	}

	// Artists: Charly 5, Soda 2.
	r = c.ExploreFor(ParseQuery(url.Values{"ver": {"artistas"}, "orden": {"escuchado"}}), p, 0, 100)
	if len(r.Artists) != 2 || r.Artists[0].Key != "charly-garcia" || r.Artists[1].Key != soda {
		t.Fatalf("artists = %+v", r.Artists)
	}
	r = c.ExploreFor(ParseQuery(url.Values{"ver": {"artistas"}, "orden": {"escuchado"}, "dir": {"asc"}}), p, 0, 100)
	if r.Artists[0].Key != soda {
		t.Fatalf("asc artists = %+v", r.Artists)
	}
}

func TestLiked(t *testing.T) {
	c := build()
	p := &store.Personal{
		Tracks:      map[int64]store.Mark{101: {Liked: true, LikedAt: 5}, 102: {Liked: true, LikedAt: 9}, 201: {Rating: 4}, 9999: {Liked: true, LikedAt: 99}},
		Albums:      map[int64]store.Mark{1: {Liked: true, LikedAt: 1}, 3: {Liked: true, LikedAt: 2}},
		Artists:     map[string]int64{soda: 3, "charly-garcia": 7, "nadie": 50},
		Collections: map[string]int64{la: 4, root + "/youtube": 8, "../nada": 60},
	}
	l := c.Liked(p)
	var tr []int64
	for _, t := range l.Tracks {
		tr = append(tr, t.ID)
	}
	if !slices.Equal(tr, []int64{102, 101}) {
		t.Fatalf("tracks = %v", tr)
	}
	if got := albumTitles(l.Albums); !reflect.DeepEqual(got, []string{"Clics modernos", "Signos"}) {
		t.Fatalf("albums = %v", got)
	}
	if len(l.Artists) != 2 || l.Artists[0].Key != "charly-garcia" || l.Artists[1].Key != soda {
		t.Fatalf("artists = %+v", l.Artists)
	}
	if len(l.Collections) != 2 || l.Collections[0].Path != root+"/youtube" || l.Collections[1].Path != la || l.Collections[1].Name != "Latinoamérica 600" {
		t.Fatalf("collections = %+v", l.Collections)
	}
	for _, e := range []LikedLists{c.Liked(&store.Personal{}), c.Liked(nil)} {
		if e.Tracks == nil || e.Albums == nil || e.Artists == nil || e.Collections == nil ||
			len(e.Tracks)+len(e.Albums)+len(e.Artists)+len(e.Collections) != 0 {
			t.Fatalf("empty = %+v", e)
		}
	}
}
