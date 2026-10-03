package catalog

import (
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"tordo/internal/store"
)

const (
	root = "../musica"
	la   = root + "/Latinoamérica 600"
	soda = "0ab49580-c84f-44d4-875f-d83760ea2cfe" // a MusicBrainz artist id
)

type fixtureAlbum struct {
	album  store.AlbumView
	tracks []store.SnapTrack
}

func tr(id int64, title, artist, format, codec string, lossless bool) store.SnapTrack {
	return store.SnapTrack{TrackView: store.TrackView{ID: id, Title: title, Artist: artist, Format: format, Codec: codec,
		Lossless: lossless, Track: int(id % 100), Path: "x", Fingerprint: "fp" + strconv.FormatInt(id, 10)}, Added: 1000 + id}
}

// fixture is a small catalog:
//   - Signos: identified (Soda Stereo's MusicBrainz id, credited with a guest), 1986, Latin / Nueva Cancion
//     and Rock / Pop Rock, in the collection, FLAC;
//   - Doble vida: by "soda stereo", not identified, 1988 (joins Signos' artist by MusicBrainz's name), MP3;
//   - Clics modernos and Bocanada: by "Charly Garcia" and "Charly García", no id, 1983 and 1999;
//     Bocanada in the queue (unmatched);
//   - Rock nacional: a compilation with a Soda Stereo track, 1995, WMA;
//   - a loose track in youtube/, 2020, Opus.
func fixture() store.Snapshot {
	albums := []fixtureAlbum{
		{store.AlbumView{ID: 1, Title: "Signos", Artist: "Soda Stereo & Andrea Echeverri", TagArtist: "Soda", Year: 1990, OriginalYear: 1986,
			Status: store.StatusAuto, ArtistID: soda, Country: "AR", Dir: la + "/[LA-10] Soda Stereo - Signos", Picture: true,
			Genres: []store.GenreLink{{Genre: "Latin", Style: "Nueva Cancion", Source: "discogs"},
				{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}},
			[]store.SnapTrack{tr(101, "Signos", "Soda Stereo", "flac", "flac", true), tr(102, "No existes", "Soda Stereo", "flac", "flac", true)}},
		{store.AlbumView{ID: 2, Title: "Doble vida", Artist: "soda stereo", TagArtist: "soda stereo", Year: 1988,
			Status: store.StatusAuto, Dir: root + "/Soda Stereo/Doble vida",
			Genres: []store.GenreLink{{Genre: "Rock", Source: "tag"}}},
			[]store.SnapTrack{tr(201, "En la ciudad de la furia", "Soda Stereo", "mp3", "mp3", false)}},
		{store.AlbumView{ID: 3, Title: "Clics modernos", Artist: "Charly Garcia", TagArtist: "Charly Garcia", Year: 1983,
			Dir: la + "/[LA-9] Charly García - Clics modernos"},
			[]store.SnapTrack{tr(301, "Nos siguen pegando abajo", "Charly García", "mp3", "mp3", false)}},
		{store.AlbumView{ID: 4, Title: "Bocanada", Artist: "Charly García", TagArtist: "Charly García", Year: 1999,
			Status: store.StatusUnmatched, Dir: root + "/Charly García/Bocanada"},
			[]store.SnapTrack{tr(401, "Bocanada", "Charly García", "mp3", "mp3", false)}},
		{store.AlbumView{ID: 5, Title: "Rock nacional", Artist: "Varios artistas", Compilation: true, Year: 1995,
			Dir: root + "/Compilados/Rock nacional", Picture: true},
			[]store.SnapTrack{tr(501, "Persiana americana", "Soda Stereo", "asf", "wmav2", false),
				tr(502, "Rasguña las piedras", "Sui Generis", "asf", "wmav2", false)}},
	}
	snap := store.Snapshot{Changes: 7, Artists: map[string]store.Artist{
		soda: {ID: soda, Name: "Soda Stereo", Country: "AR", Type: "Group", Begin: "1982", End: "2007"}}}
	for _, f := range []struct{ path, parent, name string }{
		{root, "", "musica"}, {la, root, "Latinoamérica 600"},
		{la + "/[LA-10] Soda Stereo - Signos", la, "[LA-10] Soda Stereo - Signos"},
		{la + "/[LA-9] Charly García - Clics modernos", la, "[LA-9] Charly García - Clics modernos"},
		{root + "/Soda Stereo", root, "Soda Stereo"}, {root + "/Soda Stereo/Doble vida", root + "/Soda Stereo", "Doble vida"},
		{root + "/Charly García", root, "Charly García"}, {root + "/Charly García/Bocanada", root + "/Charly García", "Bocanada"},
		{root + "/Compilados", root, "Compilados"}, {root + "/Compilados/Rock nacional", root + "/Compilados", "Rock nacional"},
		{root + "/youtube", root, "youtube"},
	} {
		snap.Folders = append(snap.Folders, store.SnapFolder{FolderSummary: store.FolderSummary{Path: f.path, Name: f.name,
			TrackCount: 1, Collection: f.path == la}, Parent: f.parent})
	}
	for _, a := range albums {
		a.album.Tracks = len(a.tracks)
		snap.Albums = append(snap.Albums, a.album)
		for _, t := range a.tracks {
			t.AlbumID, t.Dir = a.album.ID, a.album.Dir
			t.Year = a.album.Year
			snap.Tracks = append(snap.Tracks, t)
		}
	}
	loose := tr(901, "Juízo Final", "Nelson Cavaquinho", "ogg", "opus", false)
	loose.Dir, loose.Year = root+"/youtube", 2020
	snap.Tracks = append(snap.Tracks, loose)
	return snap
}

// wmaNeedsFFmpeg is the Playable of a server without ffmpeg.
func wmaNeedsFFmpeg(t store.TrackView) bool { return t.Format != "asf" }

func build() *Catalog { return Build(fixture(), Options{Playable: wmaNeedsFFmpeg}) }

func albumTitles(as []Album) []string {
	out := []string{}
	for _, a := range as {
		out = append(out, a.Title)
	}
	return out
}

func TestArtistKeys(t *testing.T) {
	c := build()
	key := func(id int64) string { a, _ := c.Album(id); return a.ArtistKey }
	if key(1) != soda || key(2) != soda {
		t.Fatalf("Soda Stereo: %q and %q", key(1), key(2))
	}
	if key(3) != "charly-garcia" || key(4) != "charly-garcia" {
		t.Fatalf("Charly García: %q and %q", key(3), key(4))
	}
	if al, _ := c.Album(5); al.ArtistKey != "" || al.Artist != "Varios artistas" || !al.Compilation {
		t.Fatalf("compilation = %+v", al)
	}
	if len(c.Artists) != 2 {
		t.Fatalf("artists = %+v", c.Artists)
	}
	ch, _ := c.Artist("charly-garcia")
	if ch.Albums != 2 || ch.Name != "Charly Garcia" || ch.Cover != 0 {
		t.Fatalf("Charly = %+v", ch) // as many albums with each spelling: the first in byte order
	}
	s, _ := c.Artist(soda)
	if s.Name != "Soda Stereo" || s.Country != "AR" || s.Albums != 2 || s.Cover != 1 {
		t.Fatalf("Soda = %+v", s)
	}
}

func TestAlbumFacetValues(t *testing.T) {
	c := build()
	signos, _ := c.Album(1)
	want := map[string][]string{
		FacetDecade: {"1980"}, FacetYear: {"1986"}, FacetArtist: {soda}, FacetCountry: {"AR"},
		FacetGenre: {"Latin", "Rock"}, FacetStyle: {"Nueva Cancion", "Pop Rock"}, FacetCollection: {la}, FacetFolder: {root, la},
		FacetQuality: {QualityLossless}, FacetFormat: {"flac"},
	}
	if !reflect.DeepEqual(signos.values, want) {
		t.Fatalf("Signos:\n got %v\nwant %v", signos.values, want)
	}
	if signos.Year != 1986 {
		t.Fatalf("year = %d, want the original", signos.Year)
	}
	comp, _ := c.Album(5)
	if !reflect.DeepEqual(comp.values[FacetFormat], []string{"wma"}) || !reflect.DeepEqual(comp.values[FacetState], []string{StateUnplayable}) {
		t.Fatalf("compilation: %v", comp.values)
	}
	boca, _ := c.Album(4)
	if !reflect.DeepEqual(boca.values[FacetState], []string{StateUnidentified}) {
		t.Fatalf("Bocanada: %v", boca.values)
	}
	if c.Albums[0].Added != 1101 {
		t.Fatalf("added = %d", c.Albums[0].Added)
	}
}

func TestExploreAlbums(t *testing.T) {
	c := build()
	r := c.Explore(ParseQuery(url.Values{"decada": {"1980"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Doble vida", "Signos", "Clics modernos"}) || r.Total != 3 {
		t.Fatalf("1980s, newest first: %q (total %d)", got, r.Total)
	}
	// A menu counts over the other facets: the decade menu shows every
	// decade, the others only what the 1980s have.
	if got := r.Facets[FacetDecade]; len(got) != 2 || got[0] != (FacetValue{"1990", "1990", 2}) || got[1] != (FacetValue{"1980", "1980", 3}) {
		t.Fatalf("decades = %+v", got)
	}
	if got := r.Facets[FacetArtist]; len(got) != 2 || got[0] != (FacetValue{soda, "Soda Stereo", 2}) || got[1].Label != "Charly Garcia" {
		t.Fatalf("artists = %+v", got)
	}
	if got := r.Facets[FacetCollection]; len(got) != 1 || got[0] != (FacetValue{la, "Latinoamérica 600", 2}) {
		t.Fatalf("collections = %+v", got)
	}
	if got := r.Facets[FacetFolder]; got[0] != (FacetValue{root, "musica", 3}) || got[1].Label != "musica/Latinoamérica 600" {
		t.Fatalf("folders = %+v", got)
	}
	// Choosing a decade keeps its menu whole even with a year chosen.
	r = c.Explore(ParseQuery(url.Values{"decada": {"1980"}, "anio": {"1983"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Clics modernos"}) || len(r.Facets[FacetDecade]) != 2 {
		t.Fatalf("1983: %q, decades %+v", got, r.Facets[FacetDecade])
	}
	r = c.Explore(ParseQuery(url.Values{"genero": {"Rock"}, "estilo": {"Pop Rock"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Signos"}) || len(r.Facets[FacetGenre]) != 2 || r.Facets[FacetGenre][0] != (FacetValue{"Rock", "Rock", 2}) {
		t.Fatalf("Pop Rock: %q, genres %+v", got, r.Facets[FacetGenre])
	}
	// Signos is Latin too: the style menu lists only Rock's styles.
	if got := r.Facets[FacetStyle]; len(got) != 1 || got[0] != (FacetValue{"Pop Rock", "Pop Rock", 1}) {
		t.Fatalf("styles = %+v", got)
	}
	r = c.Explore(ParseQuery(url.Values{"estado": {StateUnplayable}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Rock nacional"}) {
		t.Fatalf("unplayable: %q", got)
	}
	r = c.Explore(ParseQuery(url.Values{"orden": {"titulo"}}), 1, 2)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Clics modernos", "Doble vida"}) || r.Total != 5 || r.Offset != 1 {
		t.Fatalf("page 2 by title: %q, total %d", got, r.Total)
	}
	r = c.Explore(ParseQuery(url.Values{"orden": {"artista"}, "dir": {"desc"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Rock nacional", "Signos", "Doble vida", "Bocanada", "Clics modernos"}) {
		t.Fatalf("by artist, Z to A: %q", got)
	}
	if r := c.Explore(ParseQuery(nil), 10, 100); r.Albums == nil || len(r.Albums) != 0 || r.Total != 5 {
		t.Fatalf("past the end: %+v", r)
	}
}

func TestExploreArtistsAndTracks(t *testing.T) {
	c := build()
	r := c.Explore(ParseQuery(url.Values{"ver": {"artistas"}, "decada": {"1980"}}), 0, 100)
	if len(r.Artists) != 2 || r.Artists[0].Name != "Charly Garcia" || r.Artists[0].Albums != 1 || r.Artists[1].Albums != 2 {
		t.Fatalf("artists of the 1980s: %+v", r.Artists)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"artistas"}, "orden": {"cantidad"}}), 0, 100)
	if r.Artists[0].Key != "charly-garcia" || r.Artists[1].Key != soda { // as many albums: by name
		t.Fatalf("by number of albums: %+v", r.Artists)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"pistas"}, "artista": {soda}}), 0, 100)
	got := []string{}
	for _, tr := range r.Tracks {
		got = append(got, tr.Title)
	}
	// Soda Stereo's tracks, in the compilation too; by artist, then year.
	if want := []string{"Signos", "No existes", "En la ciudad de la furia", "Persiana americana"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tracks = %q, want %q", got, want)
	}
	if tr := r.Tracks[3]; tr.Album != "Rock nacional" || tr.ArtistKey != soda || tr.Path != "" || tr.Year != 1995 {
		t.Fatalf("compilation track = %+v", tr)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"pistas"}, "carpeta": {root + "/youtube"}}), 0, 100)
	if len(r.Tracks) != 1 || r.Tracks[0].Title != "Juízo Final" || r.Tracks[0].AlbumID != 0 || r.Tracks[0].Dir != root+"/youtube" {
		t.Fatalf("loose track: %+v", r.Tracks)
	}
	if got := r.Facets[FacetDecade]; len(got) != 1 || got[0].Value != "2020" {
		t.Fatalf("loose track decade: %+v", got)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"pistas"}, "formato": {"wma"}}), 0, 100)
	if len(r.Tracks) != 2 || r.Facets[FacetState][0] != (FacetValue{StateUnplayable, StateUnplayable, 2}) {
		t.Fatalf("WMA tracks: %d, states %+v", len(r.Tracks), r.Facets[FacetState])
	}
}

func TestParseQuery(t *testing.T) {
	q := ParseQuery(url.Values{"decada": {"1985"}, "anio": {"1986"}, "pais": {"ar"}, "calidad": {"mala"}, "x": {"y"},
		"orden": {"cantidad"}, "dir": {"asc"}})
	want := Query{View: ViewAlbums, Facets: map[string]string{FacetYear: "1986"}, Order: OrderYear, Dir: Asc}
	if !reflect.DeepEqual(q, want) {
		t.Fatalf("q = %+v", q)
	}
	q = ParseQuery(url.Values{"ver": {"artistas"}, "orden": {"cantidad"}})
	if q.View != ViewArtists || q.Order != OrderCount || q.Dir != Desc {
		t.Fatalf("artists = %+v", q)
	}
	if q := ParseQuery(url.Values{"ver": {"pistas"}}); q.Order != OrderArtist || q.Dir != Asc {
		t.Fatalf("tracks = %+v", q)
	}
}

func TestFormatKey(t *testing.T) {
	for _, c := range []struct{ format, codec, want string }{
		{"mp3", "mp3", "mp3"}, {"asf", "wmapro", "wma"}, {"wav", "pcm_s16le", "wav"}, {"aiff", "pcm_float", "aiff"},
		{"mp4", "alac", "alac"}, {"ogg", "", "ogg"},
	} {
		if got := FormatKey(c.format, c.codec); got != c.want {
			t.Errorf("FormatKey(%q, %q) = %q", c.format, c.codec, got)
		}
	}
}
