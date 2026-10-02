package catalog

import (
	"reflect"
	"testing"
)

func TestArtistPage(t *testing.T) {
	c := build()
	p, ok := c.ArtistPage(soda)
	if !ok {
		t.Fatal("no page")
	}
	if p.MBID != soda || p.Type != "Group" || p.Begin != "1982" || p.FirstYear != 1986 || p.LastYear != 1988 || p.Country != "AR" {
		t.Fatalf("page = %+v", p)
	}
	if got := albumTitles(p.Albums); !reflect.DeepEqual(got, []string{"Signos", "Doble vida"}) {
		t.Fatalf("albums = %q", got)
	}
	if len(p.Appearances) != 1 || p.Appearances[0].Album.Title != "Rock nacional" || len(p.Appearances[0].Tracks) != 1 ||
		p.Appearances[0].Tracks[0].Title != "Persiana americana" {
		t.Fatalf("appearances = %+v", p.Appearances)
	}
	ch, _ := c.ArtistPage("charly-garcia")
	if ch.MBID != "" || len(ch.Albums) != 2 || len(ch.Appearances) != 0 {
		t.Fatalf("Charly = %+v", ch)
	}
	if _, ok := c.ArtistPage("nadie"); ok {
		t.Fatal("unknown artist found")
	}
}
