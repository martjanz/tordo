package store

import (
	"reflect"
	"testing"

	"tordo/internal/audiofile"
)

func titles(items []PlayItem) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func albumID(t *testing.T, s *Store, title string) int64 {
	t.Helper()
	as, err := s.Albums("")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.Title == title {
			return a.ID
		}
	}
	t.Fatalf("no album %q", title)
	return 0
}

const tango = "../musica/Tangos/Troilo - Obras completas"

// library is sample plus a two-disc album and a loose track in a subfolder.
func library(t *testing.T) *Store {
	return catalog(t, map[string]audiofile.Tags{
		la + "/[LA-10] Soda Stereo - Signos/01.mp3":          {Title: "Signos", Artist: "Soda Stereo", Album: "Signos", Track: 1, Year: 1986, HasCover: true},
		la + "/[LA-10] Soda Stereo - Signos/02.mp3":          {Title: "No existes", Artist: "Soda Stereo", Album: "Signos", Track: 2, Year: 1986, HasCover: true},
		la + "/[LA-9] Charly García - Clics modernos/01.mp3": {Title: "Nos siguen pegando abajo", Artist: "Charly García", Album: "Clics modernos", Year: 1983},
		la + "/[LA-11] Virus - Locura/01.mp3":                {Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", Year: 1985},
		"../musica/suelto.mp3":                               {Title: "Suelto"},
		tango + "/CD2/01.mp3":                                {Title: "Sur", Artist: "Troilo", Album: "Obras completas", Disc: 2, Track: 1},
		tango + "/CD1/02.mp3":                                {Title: "Barrio de tango", Artist: "Troilo", Album: "Obras completas", Disc: 1, Track: 2},
		tango + "/CD1/01.mp3":                                {Title: "Che bandoneón", Artist: "Troilo", Album: "Obras completas", Disc: 1, Track: 1},
		"../musica/Tangos/youtube/a.opus":                    {Title: "A"}, // alone in its folder: loose
	}, la+"/[LA-10] Soda Stereo - Signos/cover.jpg")
}

func TestPlayAlbum(t *testing.T) {
	s := library(t)
	items, err := s.PlayAlbum(albumID(t, s, "Obras completas"))
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !reflect.DeepEqual(got, []string{"Che bandoneón", "Barrio de tango", "Sur"}) {
		t.Fatalf("tracks = %q", got)
	}
	it := items[0]
	if it.Artist != "Troilo" || it.Album != "Obras completas" || it.AlbumID == 0 || it.DurationMs != 1000 || it.Format != "mp3" || it.Codec != "mp3" {
		t.Fatalf("item = %+v", it)
	}
}

func TestPlayFolder(t *testing.T) {
	s := library(t)
	items, err := s.PlayFolder("../musica")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Nos siguen pegando abajo", "Signos", "No existes", "Una luna de miel en la mano", // the collection, in order
		"Che bandoneón", "Barrio de tango", "Sur", // the two-disc album, once
		"A",      // the loose track of Tangos/youtube
		"Suelto", // loose track of the folder itself, last
	}
	if got := titles(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("tracks = %q", got)
	}
	if items[len(items)-1].AlbumID != 0 {
		t.Fatalf("a loose track has album %d", items[len(items)-1].AlbumID)
	}
	items, err = s.PlayFolder(tango + "/CD1")
	if err != nil || len(items) != 0 {
		t.Fatalf("a disc folder plays its album from the folder above: %q, err %v", titles(items), err)
	}
	items, _ = s.PlayFolder("../mus") // a prefix of a name is not a folder
	if len(items) != 0 {
		t.Fatalf("prefix: %q", titles(items))
	}
}

func TestPlayTracksAndTrackFile(t *testing.T) {
	s := library(t)
	album, _ := s.PlayAlbum(albumID(t, s, "Signos"))
	items, err := s.PlayTracks([]int64{album[1].ID, 9999, album[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !reflect.DeepEqual(got, []string{"No existes", "Signos"}) {
		t.Fatalf("tracks = %q", got)
	}
	f, ok, err := s.TrackFile(album[0].ID)
	if err != nil || !ok || f.Path != la+"/[LA-10] Soda Stereo - Signos/01.mp3" || f.Format != "mp3" || f.Codec != "mp3" {
		t.Fatalf("file %+v, ok %v, err %v", f, ok, err)
	}
	if _, ok, err := s.TrackFile(9999); ok || err != nil {
		t.Fatalf("unknown track: ok %v, err %v", ok, err)
	}
}

func TestAlbumPictures(t *testing.T) {
	s := library(t)
	signos := albumID(t, s, "Signos")
	files, err := s.AlbumPictureFiles(signos)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %q", files)
	}
	cover, ok, err := s.AlbumCover(signos)
	if err != nil || !ok || cover != la+"/[LA-10] Soda Stereo - Signos/cover.jpg" {
		t.Fatalf("cover %q, ok %v, err %v", cover, ok, err)
	}
	if files, _ := s.AlbumPictureFiles(albumID(t, s, "Locura")); len(files) != 0 {
		t.Fatalf("Locura has no pictures: %q", files)
	}
	if _, ok, _ := s.AlbumCover(9999); ok {
		t.Fatal("unknown album found")
	}
}

func TestFolderArt(t *testing.T) {
	s := library(t)
	art, err := s.FolderArt("../musica")
	if err != nil {
		t.Fatal(err)
	}
	// Only Signos has a picture; Tangos holds an album without one, deeper.
	want := map[string][]int64{"../musica/Latinoamérica 600": {albumID(t, s, "Signos")}}
	if !reflect.DeepEqual(art, want) {
		t.Fatalf("art = %v, want %v", art, want)
	}
	art, _ = s.FolderArt(la)
	if got := art[la+"/[LA-11] Virus - Locura"]; !reflect.DeepEqual(got, []int64{albumID(t, s, "Locura")}) {
		t.Fatalf("an album's own folder, even without a picture: %v", got)
	}
	if art, _ := s.FolderArt(""); len(art) != 0 {
		t.Fatalf("roots: %v", art)
	}
}

func TestFolderTrail(t *testing.T) {
	s := library(t)
	trail, err := s.FolderTrail(la + "/[LA-11] Virus - Locura")
	if err != nil {
		t.Fatal(err)
	}
	want := []Crumb{{"../musica", "musica"}, {la, "Latinoamérica 600"}, {la + "/[LA-11] Virus - Locura", "[LA-11] Virus - Locura"}}
	if !reflect.DeepEqual(trail, want) {
		t.Fatalf("trail = %+v", trail)
	}
	if trail, _ := s.FolderTrail(""); len(trail) != 0 {
		t.Fatalf("top level: %+v", trail)
	}
}
