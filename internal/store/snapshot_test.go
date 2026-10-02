package store

import "testing"

func TestSnapshot(t *testing.T) {
	s := sample(t)
	id := albumID(t, s, "Signos")
	if err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Status: StatusAuto, ReleaseID: "r1", ArtistID: "a1",
		Genres: []GenreLink{{Genre: "Rock", Source: "mb"}}}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveArtist(Artist{ID: "a1", Name: "Soda Stereo", Country: "AR"}); err != nil {
		t.Fatal(err)
	}
	before, err := s.Changes()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Changes != before {
		t.Fatalf("changes %d, counter %d", snap.Changes, before)
	}
	if len(snap.Albums) != 3 || len(snap.Tracks) != 5 || len(snap.Folders) != 5 || snap.Artists["a1"].Country != "AR" {
		t.Fatalf("snapshot: %d albums, %d tracks, %d folders, artists %v", len(snap.Albums), len(snap.Tracks), len(snap.Folders), snap.Artists)
	}
	var signos AlbumView
	for _, a := range snap.Albums {
		if a.ID == id {
			signos = a
		}
	}
	if len(signos.Genres) != 1 || signos.Genres[0].Genre != "Rock" {
		t.Fatalf("genres = %+v", signos.Genres)
	}
	loose := 0
	for _, tr := range snap.Tracks {
		if tr.Added == 0 || tr.Dir == "" || tr.Format != "mp3" {
			t.Fatalf("track = %+v", tr)
		}
		if tr.AlbumID == 0 {
			loose++
		}
	}
	if loose != 1 {
		t.Fatalf("loose tracks = %d", loose)
	}
	roots := 0
	for _, f := range snap.Folders {
		if f.Parent == "" {
			roots++
		}
	}
	if roots != 1 {
		t.Fatalf("roots = %d", roots)
	}
	if err := s.SetCollection(la, false); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.Changes(); after == before {
		t.Fatal("a write did not move the counter")
	}
}
