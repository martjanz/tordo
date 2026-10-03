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

func TestSizeAndReleaseGroups(t *testing.T) {
	s := sample(t)
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	// Check that tracks have their file sizes from SyncFiles
	// sample() creates files with audioRow(p, 10, 1) where 10 is the size
	for _, tr := range snap.Tracks {
		if tr.SizeBytes != 10 {
			t.Fatalf("track %q: sizeBytes = %d, want 10", tr.Path, tr.SizeBytes)
		}
	}
	// Check that Album() tracks also have the size
	id := albumID(t, s, "Signos")
	detail, ok, err := s.Album(id)
	if err != nil || !ok {
		t.Fatalf("Album = %v, %v", ok, err)
	}
	for _, tr := range detail.Tracks {
		if tr.SizeBytes != 10 {
			t.Fatalf("album track %q: sizeBytes = %d, want 10", tr.Path, tr.SizeBytes)
		}
	}
	// Check that ReleaseGroups starts as empty but non-nil
	if snap.ReleaseGroups == nil {
		t.Fatal("ReleaseGroups is nil, want empty map")
	}
	if len(snap.ReleaseGroups) != 0 {
		t.Fatalf("ReleaseGroups = %+v, want empty map", snap.ReleaseGroups)
	}
	// Save releases with and without group IDs
	if err := s.SaveRelease(Release{ID: "r1", GroupID: "g1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRelease(Release{ID: "r2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRelease(Release{ID: "r3", GroupID: ""}); err != nil {
		t.Fatal(err)
	}
	// Snapshot should map r1 to g1, but skip r2 and r3
	snap, err = s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.ReleaseGroups == nil {
		t.Fatal("ReleaseGroups is nil after SaveRelease, want map")
	}
	if len(snap.ReleaseGroups) != 1 || snap.ReleaseGroups["r1"] != "g1" {
		t.Fatalf("ReleaseGroups = %+v, want {\"r1\": \"g1\"}", snap.ReleaseGroups)
	}
}
