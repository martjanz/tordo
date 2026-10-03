package catalog

import (
	"net/url"
	"reflect"
	"testing"

	"tordo/internal/store"
)

// dt is a FLAC 16/44.1 track of 200 s for the duplicates tests.
func dt(id int64, title, fp string, album int64, size int64) store.SnapTrack {
	t := tr(id, title, "Mercedes Sosa", "flac", "flac", true)
	t.Fingerprint, t.AlbumID, t.SizeBytes, t.DurationMs = fp, album, size, 200000
	t.Path = "m/" + title + "/" + string(rune('a'+id%26))
	t.BitDepth, t.SampleRate = 16, 44100
	return t
}

func mp3(t store.SnapTrack, kbps int) store.SnapTrack {
	t.Format, t.Codec, t.Lossless, t.Bitrate, t.BitDepth, t.SampleRate = "mp3", "mp3", false, kbps, 0, 44100
	return t
}

func dupAlbum(id int64, release string) store.AlbumView {
	a := store.AlbumView{ID: id, Title: "Album " + string(rune('A'+id-1)), Artist: "Mercedes Sosa", Year: 1970 + int(id), Dir: "m/a" + string(rune('0'+id))}
	if release != "" {
		a.ReleaseID = release
		a.Status = store.StatusAuto
	}
	return a
}

func dupBuild(groups map[string]string, albums []store.AlbumView, tracks ...store.SnapTrack) *Catalog {
	if groups == nil {
		groups = map[string]string{}
	}
	return Build(store.Snapshot{Albums: albums, Tracks: tracks, ReleaseGroups: groups}, Options{})
}

func plainAlbums(n int) []store.AlbumView {
	var out []store.AlbumView
	for i := int64(1); i <= int64(n); i++ {
		out = append(out, dupAlbum(i, ""))
	}
	return out
}

func fileIDs(g DupGroup) []int64 {
	out := []int64{}
	for _, f := range g.Files {
		out = append(out, f.ID)
	}
	return out
}

func TestDuplicatesExact(t *testing.T) {
	c := dupBuild(nil, plainAlbums(3),
		dt(1, "Gracias a la vida", "fp1", 1, 10), dt(2, "Gracias a la vida", "fp1", 2, 12), dt(3, "Gracias a la vida", "fp1", 3, 11),
		dt(4, "Alfonsina y el mar", "fp2", 1, 5))
	d := c.Duplicates()
	if len(d.Exact) != 1 || len(d.Formats) != 0 || len(d.Suspect) != 0 {
		t.Fatalf("groups = %+v", d)
	}
	g := d.Exact[0]
	if g.Kind != DupExact || len(g.Files) != 3 || g.Recoverable != 21 || g.SizeBytes != 33 || d.Recoverable != 21 {
		t.Fatalf("group = %+v, total %d", g, d.Recoverable)
	}
	for _, f := range g.Files {
		if f.Best {
			t.Fatalf("exact copies have no best: %+v", f)
		}
	}
	if g.Title != "Gracias a la vida" || g.Artist != "Mercedes Sosa" || g.Files[0].Album == "" {
		t.Fatalf("group header = %+v", g)
	}

	c = dupBuild(nil, plainAlbums(3),
		dt(1, "Gracias a la vida", "fp1", 1, 10), dt(2, "Gracias a la vida", "fp1", 2, 12),
		dt(3, "Volver a los 17", "fp3", 2, 100), dt(4, "Volver a los 17", "fp3", 3, 100))
	d = c.Duplicates()
	if len(d.Exact) != 2 || d.Exact[0].Recoverable != 100 || d.Exact[1].Recoverable != 10 || d.Recoverable != 110 {
		t.Fatalf("sorted by recoverable: %+v", d)
	}
	if d := dupBuild(nil, nil).Duplicates(); d.Exact == nil || d.Formats == nil || d.Suspect == nil {
		t.Fatalf("slices must not be nil: %+v", d)
	}
}

func TestDuplicatesFormats(t *testing.T) {
	albums := []store.AlbumView{dupAlbum(1, "r1"), dupAlbum(2, "r2"), dupAlbum(3, ""), dupAlbum(4, ""), dupAlbum(5, "r5"), dupAlbum(6, "r5")}
	groups := map[string]string{"r1": "g1", "r2": "g1"}
	a := mp3(dt(1, "Zamba de mi esperanza", "fp-a", 1, 8), 320)
	b := dt(2, "Zamba de mi esperanza", "fp-b", 2, 30)
	b.DurationMs += 1500
	d := dupBuild(groups, albums, a, b).Duplicates()
	if len(d.Formats) != 1 || len(d.Exact) != 0 || len(d.Suspect) != 0 {
		t.Fatalf("groups = %+v", d)
	}
	g := d.Formats[0]
	if g.Kind != DupFormats || !reflect.DeepEqual(fileIDs(g), []int64{2, 1}) || !g.Files[0].Best || g.Files[1].Best || g.Recoverable != 0 {
		t.Fatalf("FLAC first with best: %+v", g)
	}
	if g.Files[0].Format != "flac" || g.Files[1].Format != "mp3" || g.Files[0].AlbumID != 2 || g.Files[0].Album != "Album B" {
		t.Fatalf("files = %+v", g.Files)
	}
	// Without the release group the two releases are two albums.
	if d := dupBuild(nil, albums, a, b).Duplicates(); len(d.Formats) != 0 || len(d.Suspect) != 1 {
		t.Fatalf("no known group: %+v", d)
	}

	// The same unidentified album: a WMA and a FLAC.
	wma := dt(11, "Cuando tenga la tierra", "w", 3, 4)
	wma.Format, wma.Codec, wma.Lossless, wma.BitDepth = "asf", "wmav2", false, 0
	fl := dt(12, "Cuando tenga la tierra", "f", 3, 40)
	if g := dupBuild(nil, albums, wma, fl).Duplicates().Formats; len(g) != 1 || g[0].Files[0].ID != 12 || !g[0].Files[0].Best {
		t.Fatalf("al: key: %+v", g)
	}
	// Durations 10 s apart: no group.
	far := dt(13, "Cuando tenga la tierra", "g", 3, 40)
	far.DurationMs += 10000
	if d := dupBuild(nil, albums, fl, far).Duplicates(); len(d.Formats) != 0 {
		t.Fatalf("10 s apart: %+v", d)
	}
	// The same recording id, different titles.
	x, y := dt(14, "Gracias a la vida", "p", 3, 4), dt(15, "Gracias a la vida (en vivo)", "q", 3, 4)
	x.RecordingID, y.RecordingID = "rec1", "rec1"
	y.DurationMs += 60000
	if g := dupBuild(nil, albums, x, y).Duplicates().Formats; len(g) != 1 || len(g[0].Files) != 2 {
		t.Fatalf("recording id: %+v", g)
	}
	// Two equal FLACs, different fingerprints: grouped, no best.
	e1, e2 := dt(16, "Alfonsina y el mar", "p", 3, 4), dt(17, "Alfonsina y el mar", "q", 3, 4)
	if g := dupBuild(nil, albums, e1, e2).Duplicates().Formats; len(g) != 1 || g[0].Files[0].Best || g[0].Files[1].Best {
		t.Fatalf("equal quality: %+v", g)
	}
	// The same fingerprint in one album: exact only.
	s1, s2 := dt(18, "Alfonsina y el mar", "same", 3, 4), dt(19, "Alfonsina y el mar", "same", 3, 4)
	if d := dupBuild(nil, albums, s1, s2).Duplicates(); len(d.Exact) != 1 || len(d.Formats) != 0 {
		t.Fatalf("same fingerprint: %+v", d)
	}
	// A release with no known group: albums with the same release id share a key.
	r1, r2 := mp3(dt(20, "Volver a los 17", "p", 5, 4), 192), dt(21, "Volver a los 17", "q", 6, 40)
	if g := dupBuild(nil, albums, r1, r2).Duplicates().Formats; len(g) != 1 {
		t.Fatalf("rel: key: %+v", g)
	}
	// Loose tracks are never in this kind.
	l1, l2 := dt(22, "Volver a los 17", "p", 0, 4), dt(23, "Volver a los 17", "q", 0, 4)
	if d := dupBuild(nil, albums, l1, l2).Duplicates(); len(d.Formats) != 0 {
		t.Fatalf("loose tracks: %+v", d)
	}
}

func TestDuplicatesSuspect(t *testing.T) {
	albums := plainAlbums(3)
	a, b := dt(1, "Gracias a la vida", "p", 1, 4), dt(2, "Gracias a la vida", "q", 2, 4)
	b.DurationMs += 900
	d := dupBuild(nil, albums, a, b).Duplicates()
	if len(d.Suspect) != 1 || d.Suspect[0].Kind != DupSuspect || len(d.Suspect[0].Files) != 2 || len(d.Formats) != 0 {
		t.Fatalf("two albums: %+v", d)
	}
	if d := dupBuild(nil, albums, a, dt(3, "Gracias a la vida", "r", 1, 4)).Duplicates(); len(d.Suspect) != 0 || len(d.Formats) != 1 {
		t.Fatalf("one album is formats: %+v", d)
	}
	loose := dt(4, "Gracias a la vida", "r", 0, 4)
	if d := dupBuild(nil, albums, a, loose).Duplicates(); len(d.Suspect) != 1 {
		t.Fatalf("loose + album: %+v", d)
	}
	acc, up := dt(5, "Canción", "p", 1, 4), dt(6, "CANCION", "q", 2, 4)
	if d := dupBuild(nil, albums, acc, up).Duplicates(); len(d.Suspect) != 1 {
		t.Fatalf("accents and case: %+v", d)
	}
	far := dt(7, "Gracias a la vida", "q", 2, 4)
	far.DurationMs += 3000
	if d := dupBuild(nil, albums, a, far).Duplicates(); len(d.Suspect) != 0 {
		t.Fatalf("3 s apart: %+v", d)
	}
	if d := dupBuild(nil, albums, dt(8, "Volver", "same", 1, 4), dt(9, "Volver", "same", 2, 4)).Duplicates(); len(d.Suspect) != 0 || len(d.Exact) != 1 {
		t.Fatalf("exact copies only: %+v", d)
	}
}

func TestDuplicatedFacet(t *testing.T) {
	c := dupBuild(nil, plainAlbums(4),
		dt(1, "Gracias a la vida", "fp1", 1, 10), dt(2, "Gracias a la vida", "fp1", 2, 10), // exact
		dt(3, "Volver", "p", 3, 4), dt(4, "Volver", "q", 3, 4), // formats, one album
		dt(5, "Sospecha", "r", 1, 4), dt(6, "Sospecha", "s", 4, 4), // suspects only
		dt(7, "Solo", "z", 4, 4))
	r := c.Explore(ParseQuery(url.Values{"ver": {"pistas"}, "estado": {StateDuplicated}}), 0, 100)
	ids := map[int64]bool{}
	for _, tk := range r.Tracks {
		ids[tk.ID] = true
	}
	if r.Total != 4 || !reflect.DeepEqual(ids, map[int64]bool{1: true, 2: true, 3: true, 4: true}) {
		t.Fatalf("tracks = %v, total %d", ids, r.Total)
	}
	if got := c.Explore(ParseQuery(url.Values{"ver": {"pistas"}}), 0, 100).Facets[FacetState]; len(got) != 1 || got[0] != (FacetValue{StateDuplicated, StateDuplicated, 4}) {
		t.Fatalf("facet = %+v", got)
	}
	r = c.Explore(ParseQuery(url.Values{"estado": {StateDuplicated}}), 0, 100)
	if got := albumTitles(r.Albums); r.Total != 3 || !reflect.DeepEqual(map[string]bool{got[0]: true, got[1]: true, got[2]: true},
		map[string]bool{"Album A": true, "Album B": true, "Album C": true}) {
		t.Fatalf("albums = %q", got)
	}
}

func TestAlbumVersions(t *testing.T) {
	// A and B are two releases of one release group, so "Dos" is one recording in two formats there.
	albums := []store.AlbumView{dupAlbum(1, "r1"), dupAlbum(2, "r2"), dupAlbum(3, ""), dupAlbum(4, "")}
	albums[1].Year, albums[2].Year = 1999, 1999
	a1, a2, a3 := dt(1, "Uno", "e1", 1, 10), dt(2, "Dos", "fa", 1, 10), dt(3, "Tres", "e2", 1, 10)
	b1, b2 := dt(11, "Uno", "e1", 2, 10), mp3(dt(12, "Dos", "fb", 2, 3), 320)
	c3 := dt(23, "Tres", "e2", 3, 10)
	c := dupBuild(map[string]string{"r1": "g1", "r2": "g1"}, albums, a1, a2, a3, b1, b2, c3, dt(31, "Cuatro", "zz", 4, 1))
	got := c.AlbumVersions(1)
	want := []Version{
		{AlbumID: 2, Title: "Album B", Artist: "Mercedes Sosa", Year: 1999, Formats: []string{"flac", "mp3"}, Shared: 2, Kind: DupFormats},
		{AlbumID: 3, Title: "Album C", Artist: "Mercedes Sosa", Year: 1999, Formats: []string{"flac"}, Shared: 1, Kind: DupExact},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("versions:\n got %+v\nwant %+v", got, want)
	}
	if v := c.AlbumVersions(4); v == nil || len(v) != 0 {
		t.Fatalf("no duplicates: %#v", v)
	}
	if v := c.AlbumVersions(99); v == nil || len(v) != 0 {
		t.Fatalf("unknown album: %#v", v)
	}
}
