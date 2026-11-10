package store

import (
	"reflect"
	"testing"
)

// signosIdentity identifies Signos with one track title and one genre.
func signosIdentity(t *testing.T, s *Store, title string, genres ...GenreLink) []AlbumIdentity {
	t.Helper()
	id := albumID(t, s, "Signos")
	a, _, _ := s.Album(id)
	return []AlbumIdentity{{AlbumID: id, Query: "q", Status: StatusAuto, ReleaseID: "r1", Confidence: 0.9, Title: "Signos MB",
		Genres: genres, Tracks: []TrackIdentity{{FileID: a.Tracks[0].ID, Title: title, RecordingID: "rec1"}}}}
}

// Identification applies every album's identity several times per run. With
// nothing new, rewriting them held the catalog's only connection for seconds
// on a large catalog (the pages waited) and bumped the change counter (the
// catalog was rebuilt): an identical apply writes nothing.
func TestApplyIdentitiesTwiceWritesNothingTheSecondTime(t *testing.T) {
	s := sample(t)
	rock := GenreLink{Genre: "Rock", Source: "mb"}
	ids := signosIdentity(t, s, "Signos (MB)", rock)
	unmapped := map[string]int{"Chamamé": 2}
	if err := s.ApplyIdentities(ids, unmapped); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Changes()
	if err := s.ApplyIdentities(ids, unmapped); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.Changes(); after != before {
		t.Fatalf("an identical apply changed %d rows", after-before)
	}
}

// Only what differs is written, and the result is what a full rewrite gives.
func TestApplyIdentitiesWritesWhatChanged(t *testing.T) {
	s := sample(t)
	rock, pop := GenreLink{Genre: "Rock", Source: "mb"}, GenreLink{Genre: "Pop", Source: "mb"}
	if err := s.ApplyIdentities(signosIdentity(t, s, "Signos (MB)", rock), map[string]int{"Chamamé": 2, "Zamba": 1}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Changes()
	// A new track title, a genre swapped, an unmapped value gone and one changed.
	ids := signosIdentity(t, s, "Signos (otro)", pop)
	if err := s.ApplyIdentities(ids, map[string]int{"Chamamé": 3}); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Changes()
	if n := after - before; n == 0 || n > 6 {
		t.Fatalf("changed %d rows, want the few that differ", n)
	}
	a, _, _ := s.Album(ids[0].AlbumID)
	if a.Tracks[0].Title != "Signos (otro)" || !reflect.DeepEqual(a.Genres, []GenreLink{pop}) {
		t.Fatalf("album = %+v", a)
	}
	un, _ := s.UnmappedGenres()
	if !reflect.DeepEqual(un, []UnmappedGenre{{Raw: "Chamamé", Albums: 3}}) {
		t.Fatalf("unmapped = %+v", un)
	}
	// Identities that leave Signos out clear its track titles and genres,
	// and keep its album row as it was.
	if err := s.ApplyIdentities(nil, nil); err != nil {
		t.Fatal(err)
	}
	a, _, _ = s.Album(ids[0].AlbumID)
	if a.Tracks[0].Title != "Signos" || a.Tracks[0].RecordingID != "" || len(a.Genres) != 0 || a.Status != StatusAuto {
		t.Fatalf("album = %+v", a)
	}
	if un, _ := s.UnmappedGenres(); len(un) != 0 {
		t.Fatalf("unmapped = %+v", un)
	}
}
