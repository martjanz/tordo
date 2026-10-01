package store

import (
	"reflect"
	"testing"
)

func albumID(t *testing.T, s *Store, title string) int64 {
	t.Helper()
	as, err := s.Albums("")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.TagTitle == title {
			return a.ID
		}
	}
	t.Fatalf("no album %q", title)
	return 0
}

func TestApplyIdentities(t *testing.T) {
	s := sample(t)
	id := albumID(t, s, "Signos")
	before, _, _ := s.Album(id)
	first := before.Tracks[0].ID
	genres := []GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}
	err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Query: "q", Status: StatusAuto, ReleaseID: "r1", Confidence: 0.9,
		Title: "Signos (remasterizado)", Artist: "Soda Stereo", OriginalYear: 1986, Country: "AR", ArtistID: "a1",
		CoverCache: "cache/covers/r1.jpg", Genres: genres,
		Tracks: []TrackIdentity{{FileID: first, Title: "Signos (MB)", RecordingID: "rec1"}}}},
		map[string]int{"Chamamé": 2})
	if err != nil {
		t.Fatal(err)
	}
	a, ok, err := s.Album(id)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if a.Title != "Signos (remasterizado)" || a.TagTitle != "Signos" || a.Status != StatusAuto || a.ReleaseID != "r1" ||
		a.Country != "AR" || a.OriginalYear != 1986 || a.CoverCache != "cache/covers/r1.jpg" || !reflect.DeepEqual(a.Genres, genres) {
		t.Fatalf("album = %+v", a.AlbumView)
	}
	if tr := a.Tracks[0]; tr.Title != "Signos (MB)" || tr.TagTitle != "Signos" || tr.RecordingID != "rec1" {
		t.Fatalf("track = %+v", tr)
	}
	if a.Tracks[1].Title != "No existes" {
		t.Fatalf("unaligned track lost its title: %+v", a.Tracks[1])
	}
	c, _ := s.Counts()
	if c.Identified != 1 || c.Unidentified != 0 {
		t.Fatalf("counts = %+v", c)
	}
	un, err := s.UnmappedGenres()
	if err != nil || !reflect.DeepEqual(un, []UnmappedGenre{{Raw: "Chamamé", Albums: 2}}) {
		t.Fatalf("unmapped = %+v, err %v", un, err)
	}
}

func TestIdentityKeptAcrossRegroup(t *testing.T) {
	s := sample(t)
	id := albumID(t, s, "Signos")
	a, _, _ := s.Album(id)
	if err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Status: StatusAuto, ReleaseID: "r1", Title: "Signos MB",
		Genres: []GenreLink{{Genre: "Rock", Source: "mb"}}, Tracks: []TrackIdentity{{FileID: a.Tracks[0].ID, Title: "MB"}}}}, nil); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	a, _, _ = s.Album(id)
	// The album row keeps its identity; tracks are rebuilt until the next apply.
	if a.Status != StatusAuto || a.Title != "Signos MB" || len(a.Genres) != 1 || a.Tracks[0].Title != "Signos" {
		t.Fatalf("after regroup: %+v", a)
	}
}

func TestUnidentifiedAndReleasesInUse(t *testing.T) {
	s := sample(t)
	signos, clics, locura := albumID(t, s, "Signos"), albumID(t, s, "Clics modernos"), albumID(t, s, "Locura")
	cands := []Candidate{{ReleaseID: "r9", Title: "Clics Modernos", Artist: "Charly García", Score: 0.7}}
	if err := s.SaveMatches([]Match{{Query: "q-clics", Status: StatusUnmatched, Confidence: 0.7, Candidates: cands, MatcherVersion: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyIdentities([]AlbumIdentity{
		{AlbumID: signos, Status: StatusAuto, ReleaseID: "r1"},
		{AlbumID: clics, Query: "q-clics", Status: StatusUnmatched, Confidence: 0.7},
		{AlbumID: locura, Status: StatusManual, ReleaseID: "r2"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	un, err := s.Unidentified()
	if err != nil || len(un) != 1 || un[0].ID != clics || !reflect.DeepEqual(un[0].Candidates, cands) {
		t.Fatalf("unidentified = %+v, err %v", un, err)
	}
	inUse, _ := s.ReleaseIDsInUse()
	if !reflect.DeepEqual(inUse, []string{"r1", "r2"}) {
		t.Fatalf("in use = %q", inUse)
	}
	coverless, _ := s.CoverlessReleases() // Signos has cover.jpg
	if !reflect.DeepEqual(coverless, []string{"r2"}) {
		t.Fatalf("coverless = %q", coverless)
	}
}

func TestGenreAliases(t *testing.T) {
	s := open(t)
	if err := s.SetGenreAlias(GenreAlias{Raw: "chamame", Genre: "Latin"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGenreAlias(GenreAlias{Raw: "chamame", Genre: "Folk, World, & Country", Style: "Folk"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGenreAlias(GenreAlias{Raw: "uruguay"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GenreAliases()
	want := []GenreAlias{{Raw: "chamame", Genre: "Folk, World, & Country", Style: "Folk"}, {Raw: "uruguay"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("aliases = %+v, err %v", got, err)
	}
}
