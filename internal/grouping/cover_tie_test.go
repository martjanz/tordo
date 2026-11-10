package grouping

import (
	"testing"

	"tordo/internal/audiofile"
)

// Two equally good covers (same name, same size) in a two-disc album: the one
// in the album's own folder wins, on every run. Picking by map order made
// every scan store a different cover, so the catalog never stayed the same.
func TestCoverTieIsDecidedTheSameWayEveryTime(t *testing.T) {
	d := "../musica/OST/Thats All Folks - 2 CDs"
	tags := func(disc int, title string) audiofile.Tags {
		return audiofile.Tags{Title: title, Artist: "Varios", Album: "That's All Folks", Disc: disc, Track: 1}
	}
	tracks := []Track{
		tr(1, d+"/Disc 1/01.mp3", tags(1, "A")),
		tr(2, d+"/Disc 2/01.mp3", tags(2, "B")),
	}
	images := []Image{
		{Path: d + "/Disc 2/Folder.jpg", Size: 50},
		{Path: d + "/Disc 1/Folder.jpg", Size: 50},
		{Path: d + "/Folder.jpg", Size: 50},
	}
	for i := 0; i < 50; i++ {
		r := Build(tracks, images, roots)
		if len(r.Albums) != 1 {
			t.Fatalf("albums = %+v", r.Albums)
		}
		if got := r.Albums[0].CoverPath; got != d+"/Folder.jpg" {
			t.Fatalf("run %d: cover = %q", i, got)
		}
	}
	// Without one in the album's folder, the first disc's.
	for i := 0; i < 50; i++ {
		r := Build(tracks, images[:2], roots)
		if got := r.Albums[0].CoverPath; got != d+"/Disc 1/Folder.jpg" {
			t.Fatalf("run %d: cover = %q", i, got)
		}
	}
}
