package grouping

import (
	"reflect"
	"testing"

	"tordo/internal/audiofile"
)

var roots = []string{"../musica"}

func tr(id int64, p string, tags audiofile.Tags) Track {
	return Track{FileID: id, Path: p, Tags: tags, DurationMs: 1000}
}

func albumByTitle(t *testing.T, r Result, title string) Album {
	t.Helper()
	for _, a := range r.Albums {
		if a.Title == title {
			return a
		}
	}
	t.Fatalf("no album %q in %+v", title, r.Albums)
	return Album{}
}

func TestAlbumFromTags(t *testing.T) {
	d := "../musica/Soulseek/complete/radionauta/Fito Páez/(1990) Tercer Mundo"
	r := Build([]Track{
		tr(2, d+"/02 - B.mp3", audiofile.Tags{Title: "B", Artist: "Fito Páez", Album: "Tercer Mundo", Track: 2, Year: 1990, Genres: []string{"Rock"}}),
		tr(1, d+"/01 - A.mp3", audiofile.Tags{Title: "A", Artist: "Fito Páez", Album: "Tercer Mundo", Track: 1, Year: 1990, Genres: []string{"rock", "Latino"}, HasCover: true}),
	}, []Image{{Path: d + "/folder.jpg", Size: 10}, {Path: d + "/back.jpg", Size: 99}}, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := r.Albums[0]
	want := Album{Key: d + "\x00tercer mundo", Dir: d, Title: "Tercer Mundo", Artist: "Fito Páez", Year: 1990, Discs: 1, Tracks: 2,
		DurationMs: 2000, CoverPath: d + "/folder.jpg", CoverFileID: 1, Genres: []string{"rock", "Latino"}}
	if !reflect.DeepEqual(a, want) {
		t.Fatalf("got  %+v\nwant %+v", a, want)
	}
	if r.Tracks[0].FileID != 1 || r.Tracks[1].FileID != 2 || r.Tracks[0].AlbumKey != a.Key {
		t.Fatalf("tracks out of order: %+v", r.Tracks)
	}
}

func TestAlbumFromFolderNameWhenUntagged(t *testing.T) {
	d := "../musica/Soulseek/complete/morcal/Nilze_Carvalho/Nilze Carvalho - Choro de Menina - vol 2 - LP CID 1981"
	r := Build([]Track{
		tr(1, d+"/07 - Curare - Nilze Carvalho.mp3", audiofile.Tags{}),
		tr(2, d+"/08 - Doce de coco.mp3", audiofile.Tags{}),
	}, nil, roots)
	a := albumByTitle(t, r, "Choro de Menina - vol 2 - LP CID")
	if a.Artist != "Nilze Carvalho" || a.Year != 1981 || a.Tracks != 2 {
		t.Fatalf("album = %+v", a)
	}
	got := r.Tracks[0]
	if got.Title != "Curare" || got.Artist != "Nilze Carvalho" || got.Track != 7 || got.Year != 1981 {
		t.Fatalf("track = %+v", got)
	}
}

func TestDiscFoldersMergeIntoOneAlbum(t *testing.T) {
	base := "../musica/Soulseek/complete/morcal/Noel Rosa"
	r := Build([]Track{
		tr(1, base+"/Noel Rosa - Noel pela primeira vez Cd 01/01 - x.mp3", audiofile.Tags{Title: "x", Artist: "Noel Rosa", Album: "Noel pela primeira vez", Track: 1}),
		tr(2, base+"/Noel Rosa - Noel pela primeira vez Cd 02 -x/16 - y.mp3", audiofile.Tags{Title: "y", Artist: "Noel Rosa", Album: "Noel pela primeira vez CD 2", Track: 16}),
	}, nil, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := r.Albums[0]
	if a.Dir != base || a.Discs != 2 || a.Tracks != 2 || a.Title != "Noel pela primeira vez" {
		t.Fatalf("album = %+v", a)
	}
	if r.Tracks[0].Disc != 1 || r.Tracks[1].Disc != 2 {
		t.Fatalf("discs = %+v", r.Tracks)
	}
}

func TestCompilations(t *testing.T) {
	va := "../musica/Brasil/VA - História da MPB"
	mixed := "../musica/Uruguay/Selección de murgas"
	tagged := "../musica/Brasil/AAVV - Vinicius de Moraes Songbook (1993)"
	r := Build([]Track{
		tr(1, va+"/01.mp3", audiofile.Tags{Title: "a", Artist: "Billy Blanco", Album: "História da MPB"}),
		tr(2, va+"/02.mp3", audiofile.Tags{Title: "b", Artist: "Billy Blanco", Album: "História da MPB"}),
		tr(3, mixed+"/a.mp3", audiofile.Tags{Title: "a", Artist: "La Nueva Milonga", Album: "Murgas"}),
		tr(4, mixed+"/b.mp3", audiofile.Tags{Title: "b", Artist: "Falta y Resto", Album: "Murgas"}),
		tr(5, tagged+"/01.mp3", audiofile.Tags{Title: "a", Artist: "Tom Jobim", AlbumArtist: "Various Artists", Album: "Songbook"}),
	}, nil, roots)
	for _, title := range []string{"História da MPB", "Murgas", "Songbook"} {
		a := albumByTitle(t, r, title)
		if a.Artist != Various || !a.Compilation {
			t.Errorf("%s: artist %q, compilation %v", title, a.Artist, a.Compilation)
		}
	}
}

func TestLooseTracks(t *testing.T) {
	r := Build([]Track{
		tr(1, "../musica/Criosho [K-NtwAQ8Oqk].opus", audiofile.Tags{Title: "Criosho"}),
		tr(2, "../musica/youtube/Gilberto Gil - Aquele Abraço.opus", audiofile.Tags{}),
		tr(3, "../musica/Uruguay/suelto.mp3", audiofile.Tags{Title: "Suelto", Album: "Con álbum"}),
	}, nil, roots)
	if len(r.Albums) != 1 || r.Albums[0].Title != "Con álbum" {
		t.Fatalf("albums = %+v", r.Albums)
	}
	loose := map[int64]TrackOut{}
	for _, x := range r.Tracks {
		if x.AlbumKey == "" {
			loose[x.FileID] = x
		}
	}
	if len(loose) != 2 {
		t.Fatalf("loose = %+v", loose)
	}
	if g := loose[2]; g.Artist != "Gilberto Gil" || g.Title != "Aquele Abraço" {
		t.Fatalf("parsed loose track = %+v", g)
	}
}

func TestPositionFromFolderPrefix(t *testing.T) {
	d := "../musica/Latinoamérica 600/[LA-599] Rata Blanca - (1990) Magos, Espadas Y Rosas"
	r := Build([]Track{tr(1, d+"/01.mp3", audiofile.Tags{Title: "a", Artist: "Rata Blanca", Album: "Magos, Espadas y Rosas"})}, nil, roots)
	if r.Albums[0].Position != 599 {
		t.Fatalf("Position = %d", r.Albums[0].Position)
	}
}
