package catalog

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"tordo/internal/store"
)

func trackIDs(ts []TrackPlays) []int64 {
	out := []int64{}
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

func TestStats(t *testing.T) {
	clockAt(t, time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC))
	c := build()
	tally := store.PlayTally{Counts: map[int64]int{101: 3, 102: 1, 201: 2, 502: 1, 9999: 1}, Plays: 8, ListenedMs: 1234}
	s := c.Stats(tally, &store.Personal{}, 10)
	if s.Plays != 8 || s.Played != 4 || s.ListenedMs != 1234 {
		t.Fatalf("totals = %d %d %d", s.Plays, s.Played, s.ListenedMs)
	}
	if got, want := trackIDs(s.Tracks), []int64{101, 201, 102, 502}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tracks = %v want %v", got, want)
	}
	var albums []string
	for _, a := range s.Albums {
		albums = append(albums, a.Title+":"+strconv.Itoa(a.Plays))
	}
	if want := []string{"Signos:4", "Doble vida:2", "Rock nacional:1"}; !reflect.DeepEqual(albums, want) {
		t.Fatalf("albums = %v want %v", albums, want)
	}
	if want := []ArtistPlays{{soda, "Soda Stereo", 6}, {"", "Sui Generis", 1}}; !reflect.DeepEqual(s.Artists, want) {
		t.Fatalf("artists = %+v", s.Artists)
	}
	if want := []ValuePlays{{"AR", 4}}; !reflect.DeepEqual(s.Countries, want) {
		t.Fatalf("countries = %+v", s.Countries)
	}
	if want := []ValuePlays{{"Rock", 6}, {"Latin", 4}}; !reflect.DeepEqual(s.Genres, want) {
		t.Fatalf("genres = %+v", s.Genres)
	}
	if s.Never != 5 || s.Stale != 0 {
		t.Fatalf("never/stale = %d/%d", s.Never, s.Stale)
	}
	s = c.Stats(tally, &store.Personal{}, 2)
	if len(s.Tracks) != 2 || len(s.Albums) != 2 || len(s.Artists) != 2 || len(s.Genres) != 2 {
		t.Fatalf("limit: %d %d %d %d", len(s.Tracks), len(s.Albums), len(s.Artists), len(s.Genres))
	}
	empty := c.Stats(store.PlayTally{Counts: map[int64]int{}}, nil, 5)
	if empty.Tracks == nil || empty.Albums == nil || empty.Artists == nil || empty.Countries == nil || empty.Genres == nil {
		t.Fatalf("nil lists: %+v", empty)
	}
}

func TestArtistTop(t *testing.T) {
	c := build()
	p := &store.Personal{Plays: map[int64]store.Plays{101: {Count: 3}, 501: {Count: 1}, 102: {Count: 0}, 301: {Count: 9}}}
	if got, want := trackIDs(c.ArtistTop(soda, p, 10)), []int64{101, 501}; !reflect.DeepEqual(got, want) {
		t.Fatalf("top = %v want %v", got, want)
	}
	if got := c.ArtistTop(soda, p, 1); len(got) != 1 || got[0].Plays != 3 {
		t.Fatalf("limit: %+v", got)
	}
	if got := c.ArtistTop("nadie", p, 10); got == nil || len(got) != 0 {
		t.Fatalf("unknown: %v", got)
	}
	if got := c.ArtistTop(soda, nil, 10); got == nil || len(got) != 0 {
		t.Fatalf("nil p: %v", got)
	}
}
