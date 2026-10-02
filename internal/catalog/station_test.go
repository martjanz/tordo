package catalog

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"tordo/internal/store"
)

var stationNow = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

func fixNow(t *testing.T) {
	old := now
	now = func() time.Time { return stationNow }
	t.Cleanup(func() { now = old })
}

func rng() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

func ids(ts []*Track) []int64 {
	out := []int64{}
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

func TestStationClean(t *testing.T) {
	st := Station{Name: "  " + strings.Repeat("ñ", 300) + " ", Mode: "x", ArtistGap: -1, AvoidDays: 9999,
		Source: Source{Folder: " a/b ", Playlist: 3, Liked: true, View: ViewTracks, Facets: map[string]string{"pais": "AR"}}}
	got := st.Clean()
	if got.Source.Folder != "a/b" || got.Source.Playlist != 0 || got.Source.Liked || got.Source.View != "" || got.Source.Facets != nil ||
		got.Mode != ModeTracks || got.ArtistGap != 0 || got.AvoidDays != MaxAvoidDays {
		t.Fatalf("folder: %+v", got)
	}
	if n := len([]rune(got.Name)); n != 200 {
		t.Fatalf("name has %d runes", n)
	}
	if got := (Station{Source: Source{Playlist: 3, Liked: true}, ArtistGap: 99}).Clean(); got.Source.Playlist != 3 || got.Source.Liked || got.ArtistGap != MaxArtistGap {
		t.Fatalf("playlist: %+v", got)
	}
	if got := (Station{Source: Source{Liked: true, View: ViewAlbums}, Mode: ModeAlbums}).Clean(); !got.Source.Liked || got.Source.View != "" || got.Mode != ModeAlbums {
		t.Fatalf("liked: %+v", got)
	}
	got = Station{Source: Source{View: ViewArtists, Facets: map[string]string{"pais": "AR", "decada": "1985", "nope": "x"}}}.Clean()
	if got.Source.View != ViewAlbums || !reflect.DeepEqual(got.Source.Facets, map[string]string{"pais": "AR"}) {
		t.Fatalf("query: %+v", got.Source)
	}
	got = Station{Source: Source{Facets: map[string]string{"nope": "x"}}}.Clean()
	if got.Source.View != ViewTracks || got.Source.Facets != nil {
		t.Fatalf("empty query: %+v", got.Source)
	}
}

func TestStationPool(t *testing.T) {
	c := build()
	p := &store.Personal{Albums: map[int64]store.Mark{1: {Liked: true}}, Artists: map[string]int64{nameKey("Charly García"): 1}}
	for _, tc := range []struct {
		name string
		src  Source
		p    *store.Personal
		list []int64
		want []int64
	}{
		{"artist tracks", Source{View: ViewTracks, Facets: map[string]string{"artista": soda}}, nil, nil, []int64{101, 102, 201}},
		{"albums", Source{View: ViewAlbums, Facets: map[string]string{"pais": "AR"}}, nil, nil, []int64{101, 102}},
		{"folder", Source{Folder: la}, nil, nil, []int64{101, 102, 301}},
		{"playlist", Source{Playlist: 1}, nil, []int64{301, 999, 301}, []int64{301}},
		{"liked", Source{Liked: true}, p, nil, []int64{101, 102, 301, 401}},
		{"liked without data", Source{Liked: true}, nil, nil, []int64{}},
		{"everything", Source{}, nil, nil, []int64{101, 102, 201, 301, 401, 901}},
	} {
		if got := ids(c.pool(tc.src, tc.p, tc.list)); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	// A liked collection brings what is under it.
	pc := &store.Personal{Collections: map[string]int64{la: 1}}
	if got := ids(c.pool(Source{Liked: true}, pc, nil)); !slices.Equal(got, []int64{101, 102, 301}) {
		t.Errorf("liked collection: %v", got)
	}
}

func TestNextTracksRules(t *testing.T) {
	fixNow(t)
	c := build()
	st := Station{Mode: ModeTracks}
	out := c.Next(st, StationInput{Rand: rng()}, 10)
	if len(out) != 10 {
		t.Fatalf("got %v", out)
	}
	first := slices.Clone(out[:6])
	slices.Sort(first)
	if !slices.Equal(first, []int64{101, 102, 201, 301, 401, 901}) {
		t.Fatalf("first six: %v", out)
	}
	for i := 1; i < len(out); i++ {
		if out[i] == out[i-1] {
			t.Fatalf("repeated back to back: %v", out)
		}
	}
	for seed := range uint64(20) {
		if got := c.Next(st, StationInput{Rand: rand.New(rand.NewPCG(seed, 2)), Recent: []int64{101}}, 1); got[0] == 101 {
			t.Fatalf("seed %d: the first pick repeats the last: %v", seed, got)
		}
	}
}

func TestNextArtistGap(t *testing.T) {
	fixNow(t)
	c := build()
	st := Station{Source: Source{View: ViewTracks, Facets: map[string]string{"decada": "1980"}}, Mode: ModeTracks, ArtistGap: 1}
	for seed := range uint64(20) {
		out := c.Next(st, StationInput{Rand: rand.New(rand.NewPCG(seed, 2)), Recent: []int64{101}}, 2)
		if len(out) != 2 || out[0] != 301 || out[1] == 301 {
			t.Fatalf("seed %d: %v", seed, out)
		}
	}
}

func TestNextAvoidsRecentPlays(t *testing.T) {
	fixNow(t)
	c := build()
	yesterday := stationNow.Add(-24 * time.Hour).UnixMilli()
	p := &store.Personal{Plays: map[int64]store.Plays{101: {Count: 1, Last: yesterday}, 102: {Count: 1, Last: yesterday}}}
	st := Station{Source: Source{View: ViewTracks, Facets: map[string]string{"estilo": "Pop Rock"}}, Mode: ModeTracks, AvoidDays: 7}
	out := c.Next(st, StationInput{Personal: p, Rand: rng()}, 2)
	slices.Sort(out)
	if !slices.Equal(out, []int64{101, 102}) {
		t.Fatalf("relaxed: %v", out)
	}
	st.Source = Source{View: ViewTracks, Facets: map[string]string{"decada": "1980"}}
	out = c.Next(st, StationInput{Personal: p, Rand: rng()}, 2)
	slices.Sort(out)
	if !slices.Equal(out, []int64{201, 301}) {
		t.Fatalf("avoiding: %v", out)
	}
}

func TestNextWeights(t *testing.T) {
	fixNow(t)
	c := build()
	p := &store.Personal{Tracks: map[int64]store.Mark{101: {Liked: true, Rating: 5}}}
	st := Station{Source: Source{View: ViewTracks, Facets: map[string]string{"decada": "1980"}}, Mode: ModeTracks}
	r := rng()
	n := 0
	for range 3000 {
		if c.Next(st, StationInput{Personal: p, Rand: r}, 1)[0] == 101 {
			n++
		}
	}
	if n < 1950 || n > 2550 {
		t.Fatalf("101 chosen %d of 3000 times, want about 75%%", n)
	}
}

func TestNextAlbums(t *testing.T) {
	fixNow(t)
	c := build()
	st := Station{Mode: ModeAlbums}
	for seed := range uint64(20) {
		out := c.Next(st, StationInput{Rand: rand.New(rand.NewPCG(seed, 2))}, 3)
		if len(out) < 3 {
			t.Fatalf("seed %d: %v", seed, out)
		}
		for i, id := range out {
			if id == 101 && (i+1 >= len(out) || out[i+1] != 102) {
				t.Fatalf("seed %d: Signos split: %v", seed, out)
			}
		}
		out = c.Next(st, StationInput{Rand: rand.New(rand.NewPCG(seed, 2)), Recent: []int64{101, 102}}, 3)
		if out[0] == 101 || out[0] == 102 {
			t.Fatalf("seed %d: repeats the last album: %v", seed, out)
		}
	}
}

func TestNextEmpty(t *testing.T) {
	c := build()
	if got := c.Next(Station{Source: Source{View: ViewTracks, Facets: map[string]string{"pais": "BR"}}, Mode: ModeTracks}, StationInput{Rand: rng()}, 10); len(got) != 0 {
		t.Fatalf("nothing: %v", got)
	}
	if got := c.Next(Station{Source: Source{Playlist: 1}, Mode: ModeAlbums}, StationInput{Rand: rng()}, 10); len(got) != 0 {
		t.Fatalf("empty playlist: %v", got)
	}
}
