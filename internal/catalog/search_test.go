package catalog

import (
	"testing"

	"tordo/internal/search"
	"tordo/internal/store"
)

func TestSearch(t *testing.T) {
	c := build()
	x, err := search.New()
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	if err := x.Refresh(c.Changes, c.SearchDocs); err != nil {
		t.Fatal(err)
	}
	find := func(q string, limit int) Found {
		keys, err := x.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		return c.Found(keys, limit)
	}
	f := find("soda", 10)
	if len(f.Artists) != 1 || f.Artists[0].Key != soda || len(f.Albums) != 2 || len(f.Tracks) != 4 || f.Totals["tracks"] != 4 {
		t.Fatalf("soda: %+v", f)
	}
	f = find("garcia", 1) // both spellings, one key
	if len(f.Artists) != 1 || f.Totals["albums"] != 2 || len(f.Albums) != 1 || f.Totals["artists"] != 1 {
		t.Fatalf("garcia: %+v", f)
	}
	f = find("latinoamérica", 10)
	if len(f.Folders) != 1 || f.Folders[0].Path != la || !f.Folders[0].Collection || len(f.Albums) != 0 {
		t.Fatalf("collection: %+v", f)
	}
	if f := find("signos", 10); len(f.Folders) != 0 {
		t.Fatalf("an album's own folder is the album: %+v", f.Folders)
	}
	if f := find("juizo", 10); len(f.Tracks) != 1 || f.Tracks[0].AlbumID != 0 {
		t.Fatalf("loose track: %+v", f)
	}
}

func TestFoundArtistsByAlbums(t *testing.T) {
	snap := fixture()
	snap.Albums = append(snap.Albums, store.AlbumView{ID: 6, Title: "Demo", Artist: "Charly Sosa", Dir: root + "/Charly Sosa", Tracks: 1})
	c := Build(snap, Options{})
	f := c.Found([]string{"artist:charly-sosa", "artist:charly-garcia"}, 1)
	if len(f.Artists) != 1 || f.Artists[0].Key != "charly-garcia" || f.Totals["artists"] != 2 {
		t.Fatalf("found = %+v", f)
	}
}
