package grouping

import (
	"path"
	"reflect"
	"strconv"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/nameparse"
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
		// Real tags on other discs of the same set: the disc folder, not the tag, names the album.
		tr(3, base+"/Noel Rosa - Noel pela primeira vez Cd 03 -x/05 - z.mp3", audiofile.Tags{Title: "z", Artist: "Noel Rosa", Album: "Noel Pela Primeira Vez (03)", Track: 5}),
		tr(4, base+"/Noel Rosa - Noel pela primeira vez Cd 05 -x/05 - w.mp3", audiofile.Tags{Title: "w", Artist: "Noel Rosa", Album: "Noel Pela Primeira Vez - disc", Track: 5}),
	}, nil, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := r.Albums[0]
	if a.Dir != base || a.Discs != 5 || a.Tracks != 4 || a.Title != "Noel pela primeira vez" {
		t.Fatalf("album = %+v", a)
	}
	if r.Tracks[0].Disc != 1 || r.Tracks[1].Disc != 2 || r.Tracks[2].Disc != 3 || r.Tracks[3].Disc != 5 {
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

func TestStrayAlbumTagsJoinTheFolderAlbum(t *testing.T) {
	d := "../musica/Brasil/Elza Soares/(1964-1965) Na Roda Do Samba , Um Show De Elza"
	main := audiofile.Tags{Artist: "Elza Soares", AlbumArtist: "Elza Soares", Album: "Na Roda do Samba / Um Show de Elza"}
	var tracks []Track
	for i := int64(1); i <= 5; i++ {
		tracks = append(tracks, tr(i, d+"/0"+string(rune('0'+i))+".mp3", main))
	}
	tracks = append(tracks,
		tr(6, d+"/06.mp3", audiofile.Tags{Artist: "Elza Soares", Album: "Na Roda do Samba"}),
		tr(7, d+"/07.mp3", audiofile.Tags{Artist: "Elza Soares", Album: "Um Show de Elza"}))
	r := Build(tracks, nil, roots)
	if len(r.Albums) != 1 || r.Albums[0].Tracks != 7 || r.Albums[0].Title != "Na Roda do Samba / Um Show de Elza" {
		t.Fatalf("albums = %+v", r.Albums)
	}
	for _, x := range r.Tracks {
		if x.AlbumKey != r.Albums[0].Key {
			t.Fatalf("track %d in %q", x.FileID, x.AlbumKey)
		}
	}
}

func TestSeveralRealAlbumsInOneFolderStaySeparate(t *testing.T) {
	d := "../musica/USA/Anthology of American Folk Music, Volumes 1-3"
	var tracks []Track
	for i, vol := range []string{"Volume 1: Ballads", "Volume 2: Social Music", "Volume 3: Songs"} {
		for j := 0; j < 3; j++ {
			id := int64(i*3 + j + 1)
			tracks = append(tracks, tr(id, d+"/"+strconv.FormatInt(id, 10)+".flac", audiofile.Tags{Artist: "x", AlbumArtist: "Various Artists", Album: "Anthology of American Folk Music, " + vol}))
		}
	}
	if r := Build(tracks, nil, roots); len(r.Albums) != 3 {
		t.Fatalf("albums = %+v", r.Albums)
	}
}

func TestSelectionFolderIsOneAlbum(t *testing.T) {
	d := "../musica/Uruguay/Selección de murgas"
	r := Build([]Track{
		tr(1, d+"/a1.mp3", audiofile.Tags{Artist: "Asaltantes con Patente", Album: "Asaltantes"}),
		tr(2, d+"/a2.mp3", audiofile.Tags{Artist: "Asaltantes con Patente", Album: "Asaltantes"}),
		tr(3, d+"/a3.mp3", audiofile.Tags{Artist: "Asaltantes con Patente", Album: "Asaltantes"}),
		tr(4, d+"/m1.mp3", audiofile.Tags{Artist: "La Nueva ola", AlbumArtist: "La Nueva ola", Album: "Carnaval uruguayo vol 3 (macondo)"}),
		tr(5, d+"/m2.mp3", audiofile.Tags{Artist: "Curtidores", AlbumArtist: "Curtidores", Album: "Carnaval uruguayo vol 3 (macondo)"}),
		tr(6, d+"/b.mp3", audiofile.Tags{Artist: "Araca la Cana", Album: "Araca la Cana 1973"}),
		tr(7, d+"/c.mp3", audiofile.Tags{Artist: "Don Timoteo", Album: "Don Timoteo 1985"}),
		tr(8, d+"/e.mp3", audiofile.Tags{Artist: "La Mojigata", Album: "La Mojigata 2002"}),
	}, nil, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := r.Albums[0]
	if a.Title != "Selección de murgas" || a.Artist != Various || !a.Compilation || a.Tracks != 8 {
		t.Fatalf("album = %+v", a)
	}
}

func TestOneTrackAlbumTagsMakeOneFolderAlbum(t *testing.T) {
	d := "../musica/Uruguay/Alfredo Zitarrosa/[1980-1981] Casi en privado (programa de Radio Educación - Mexico)"
	r := Build([]Track{
		tr(1, d+"/01.mp3", audiofile.Tags{Artist: "Alfredo Zitarrosa", Album: "P.1"}),
		tr(2, d+"/02.mp3", audiofile.Tags{Artist: "Alfredo Zitarrosa", Album: "P.2"}),
		tr(3, d+"/03.mp3", audiofile.Tags{Artist: "Alfredo Zitarrosa", Album: "P.3"}),
	}, nil, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := r.Albums[0]
	if want := nameparse.ParseFolder(path.Base(d)).Album; a.Title != want || a.Artist != "Alfredo Zitarrosa" || a.Compilation || a.Tracks != 3 {
		t.Fatalf("album = %+v, want title %q", a, want)
	}
}

func TestDiscFolderNotesDoNotSplitTheSet(t *testing.T) {
	base := "../musica/Uruguay"
	r := Build([]Track{
		tr(1, base+"/Varios - Candombe de llamadas CD 1 (selección, falta tema 20)/01.mp3", audiofile.Tags{Artist: "Senegal", Album: "Candombe de llamadas CD 1"}),
		tr(2, base+"/Varios - Candombe de llamadas CD 2/01.mp3", audiofile.Tags{Artist: "Nicolás Arnicho", Album: "Candombe de llamadas CD 2"}),
	}, nil, roots)
	if len(r.Albums) != 1 || r.Albums[0].Discs != 2 || r.Albums[0].Artist != Various {
		t.Fatalf("albums = %+v", r.Albums)
	}
}
