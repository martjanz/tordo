package store

import (
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
)

// catalog builds a store from tagged files through the real grouping.
func catalog(t *testing.T, files map[string]audiofile.Tags, images ...string) *Store {
	t.Helper()
	s := open(t)
	var rows []FileRow
	for p := range files {
		rows = append(rows, audioRow(p, 10, 1))
	}
	for _, p := range images {
		rows = append(rows, FileRow{Path: p, Size: 1, MTime: 1, Kind: "image"})
	}
	if err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	env := ReadEnv{Version: 1}
	pending, _ := s.PendingReads(env)
	var rs []ReadResult
	for _, p := range pending {
		rs = append(rs, ReadResult{Target: p, Env: env, Fingerprint: "fp" + p.Path,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", Bitrate: 320, SampleRate: 44100, DurationMs: 1000, Tags: files[p.Path]}})
	}
	if err := s.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	return s
}

func regroup(t *testing.T, s *Store) {
	t.Helper()
	tracks, images, err := s.GroupingInput()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGrouping(grouping.Build(tracks, images, root)); err != nil {
		t.Fatal(err)
	}
}

const la = "../musica/Latinoamérica 600"

func sample(t *testing.T) *Store {
	return catalog(t, map[string]audiofile.Tags{
		la + "/[LA-10] Soda Stereo - Signos/01.mp3":          {Title: "Signos", Artist: "Soda Stereo", Album: "Signos", Track: 1, Year: 1986},
		la + "/[LA-10] Soda Stereo - Signos/02.mp3":          {Title: "No existes", Artist: "Soda Stereo", Album: "Signos", Track: 2, Year: 1986},
		la + "/[LA-9] Charly García - Clics modernos/01.mp3": {Title: "Nos siguen pegando abajo", Artist: "Charly García", Album: "Clics modernos", Year: 1983},
		la + "/[LA-11] Virus - Locura/01.mp3":                {Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", Year: 1985},
		"../musica/suelto.mp3":                               {Title: "Suelto"},
	}, la+"/[LA-10] Soda Stereo - Signos/cover.jpg")
}

func TestAlbumsAndAlbum(t *testing.T) {
	s := sample(t)
	all, err := s.Albums("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Artist != "Charly García" || all[2].Artist != "Virus" {
		t.Fatalf("albums = %+v", all)
	}
	var signos AlbumView
	for _, a := range all {
		if a.Title == "Signos" {
			signos = a
		}
	}
	if signos.Tracks != 2 || signos.Year != 1986 || signos.Position != 10 || signos.CoverPath != la+"/[LA-10] Soda Stereo - Signos/cover.jpg" || signos.RawGenres == nil {
		t.Fatalf("signos = %+v", signos)
	}
	d, ok, err := s.Album(signos.ID)
	if err != nil || !ok {
		t.Fatalf("Album = %v, %v", ok, err)
	}
	if len(d.Tracks) != 2 || d.Tracks[0].Title != "Signos" || d.Tracks[1].Track != 2 || d.Tracks[0].Bitrate != 320 || d.Tracks[0].Fingerprint == "" {
		t.Fatalf("tracks = %+v", d.Tracks)
	}
	if _, ok, _ := s.Album(9999); ok {
		t.Fatal("Album(9999) found")
	}
	if under, _ := s.Albums(la + "/[LA-9] Charly García - Clics modernos"); len(under) != 1 {
		t.Fatalf("Albums(folder) = %+v", under)
	}
}

func TestAlbumIDsAreStable(t *testing.T) {
	s := sample(t)
	before, _ := s.Albums("")
	regroup(t, s)
	after, _ := s.Albums("")
	for i := range before {
		if before[i].ID != after[i].ID {
			t.Fatalf("album %q changed id: %d → %d", before[i].Title, before[i].ID, after[i].ID)
		}
	}
}

func TestFolderView(t *testing.T) {
	s := sample(t)
	top, ok, err := s.Folder("")
	if err != nil || !ok || len(top.Children) != 1 || top.Children[0].Path != "../musica" {
		t.Fatalf("roots = %+v, %v, %v", top, ok, err)
	}
	r, _, _ := s.Folder("../musica")
	if r.TrackCount != 5 || r.AlbumCount != 3 || len(r.Loose) != 1 || r.Loose[0].Title != "Suelto" || len(r.Children) != 1 {
		t.Fatalf("root = %+v", r)
	}
	l, _, _ := s.Folder(la)
	if !l.Collection || len(l.Children) != 3 || l.Children[0].Name != "[LA-9] Charly García - Clics modernos" || l.Parent != "../musica" {
		t.Fatalf("collection folder = %+v", l)
	}
	if _, ok, _ := s.Folder("../nope"); ok {
		t.Fatal("unknown folder found")
	}
}

func TestCollectionsInOrder(t *testing.T) {
	s := sample(t)
	cs, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Path != la || len(cs[0].Albums) != 3 {
		t.Fatalf("collections = %+v", cs)
	}
	got := []int{cs[0].Albums[0].Position, cs[0].Albums[1].Position, cs[0].Albums[2].Position}
	if got[0] != 9 || got[1] != 10 || got[2] != 11 {
		t.Fatalf("positions = %v", got)
	}
	c, _ := s.Counts()
	if c.Albums != 3 || c.Tracks != 5 || c.Collections != 1 {
		t.Fatalf("counts = %+v", c)
	}
}
