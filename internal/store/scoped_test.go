package store

import (
	"reflect"
	"sort"
	"testing"
)

func TestScopedLookups(t *testing.T) {
	s := sample(t)
	signos, locura := albumID(t, s, "Signos"), albumID(t, s, "Locura")
	dir := la + "/[LA-10] Soda Stereo - Signos"

	ids, err := s.AlbumsWithFingerprints([]string{"fp" + dir + "/02.mp3", "fp-nowhere"})
	if err != nil || !reflect.DeepEqual(ids, []int64{signos}) {
		t.Fatalf("albums with fingerprints = %v, err %v", ids, err)
	}
	if ids, _ := s.AlbumsWithFingerprints(nil); len(ids) != 0 {
		t.Fatalf("no fingerprints: %v", ids)
	}

	ts, err := s.IdentifyTargetsOf([]int64{locura, signos, 9999})
	if err != nil || len(ts) != 2 {
		t.Fatalf("targets = %+v, err %v", ts, err)
	}
	all, _ := s.IdentifyTargets()
	if want := targetByTitle(t, all, "Signos"); !reflect.DeepEqual(targetByTitle(t, ts, "Signos"), want) {
		t.Fatalf("scoped target %+v, want %+v", targetByTitle(t, ts, "Signos"), want)
	}
	if ts, _ := s.IdentifyTargetsOf(nil); len(ts) != 0 {
		t.Fatalf("no albums: %+v", ts)
	}

	if err := s.SaveMatches([]Match{{Query: "q1", Status: StatusAuto, ReleaseID: "r1"}, {Query: "q2", Status: StatusUnmatched}}); err != nil {
		t.Fatal(err)
	}
	if ms, err := s.MatchesOf([]string{"q2", "q9"}); err != nil || len(ms) != 1 || ms["q2"].Status != StatusUnmatched {
		t.Fatalf("matches = %+v, err %v", ms, err)
	}
	if err := s.SetCorrection([]string{"fp1", "fp2"}, Correction{Status: StatusIgnored}); err != nil {
		t.Fatal(err)
	}
	if cs, err := s.CorrectionsOf([]string{"fp2", "fp9"}); err != nil || len(cs) != 1 || cs["fp2"].Status != StatusIgnored {
		t.Fatalf("corrections = %+v, err %v", cs, err)
	}
	for _, id := range []string{"r1", "r2"} {
		if err := s.SaveRelease(Release{ID: id, Title: id}); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveArtist(Artist{ID: "a-" + id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	if rs, err := s.ReleasesOf([]string{"r2", "r9"}); err != nil || len(rs) != 1 || rs["r2"].Title != "r2" {
		t.Fatalf("releases = %+v, err %v", rs, err)
	}
	if as, err := s.ArtistsOf([]string{"a-r1"}); err != nil || len(as) != 1 || as["a-r1"].Name != "r1" {
		t.Fatalf("artists = %+v, err %v", as, err)
	}
}

func unmappedMap(t *testing.T, s *Store) map[string]int {
	t.Helper()
	un, err := s.UnmappedGenres()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, u := range un {
		out[u.Raw] = u.Albums
	}
	return out
}

func TestApplyAlbumIdentitiesTouchesOnlyItsAlbums(t *testing.T) {
	s := sample(t)
	signos, locura, clics := albumID(t, s, "Signos"), albumID(t, s, "Locura"), albumID(t, s, "Clics modernos")
	sig, _, _ := s.Album(signos)
	loc, _, _ := s.Album(locura)
	tag := []GenreLink{{Genre: "Rock", Source: "tag"}}
	if err := s.ApplyIdentities([]AlbumIdentity{
		{AlbumID: signos, Query: "qs", Status: StatusUnmatched, Genres: tag},
		{AlbumID: locura, Query: "ql", Status: StatusAuto, ReleaseID: "rl", Title: "Locura MB", Genres: tag,
			Tracks: []TrackIdentity{{FileID: loc.Tracks[0].ID, Title: "Luna MB", RecordingID: "rec-l"}}},
		{AlbumID: clics, Query: "qc", Status: StatusUnmatched},
	}, map[string]int{"Chamamé": 2, "Cumbia": 1}); err != nil {
		t.Fatal(err)
	}

	// Signos alone: identified by hand, its tags no longer count Chamamé, and
	// it brings a new unmapped value. Locura is left as it was, even though it
	// is not in ids.
	err := s.ApplyAlbumIdentities([]int64{signos}, []AlbumIdentity{{AlbumID: signos, Query: "qs", Status: StatusManual,
		ReleaseID: "rs", Confidence: 1, Title: "Signos MB", Genres: []GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "mb"}},
		Tracks: []TrackIdentity{{FileID: sig.Tracks[1].ID, Title: "No existes MB", RecordingID: "rec-2"}}}},
		map[string]int{"Chamamé": -1, "Cumbia": -1, "Murga": 1})
	if err != nil {
		t.Fatal(err)
	}
	a, _, _ := s.Album(signos)
	if a.Status != StatusManual || a.Title != "Signos MB" || a.Tracks[1].Title != "No existes MB" || a.Tracks[0].Title != "Signos" ||
		!reflect.DeepEqual(a.Genres, []GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "mb"}}) {
		t.Fatalf("signos = %+v", a)
	}
	b, _, _ := s.Album(locura)
	if b.Status != StatusAuto || b.Title != "Locura MB" || b.Tracks[0].Title != "Luna MB" || !reflect.DeepEqual(b.Genres, tag) {
		t.Fatalf("locura was touched: %+v", b)
	}
	if got, want := unmappedMap(t, s), map[string]int{"Chamamé": 1, "Murga": 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unmapped = %v, want %v", got, want)
	}

	// An id outside the scope is not written.
	if err := s.ApplyAlbumIdentities([]int64{signos}, []AlbumIdentity{{AlbumID: clics, Status: StatusIgnored}}, nil); err != nil {
		t.Fatal(err)
	}
	if c, _, _ := s.Album(clics); c.Status != StatusUnmatched {
		t.Fatalf("clics written from outside the scope: %q", c.Status)
	}
	// A scoped album missing from ids loses its track titles and genres, as
	// with the full apply.
	if err := s.ApplyAlbumIdentities([]int64{signos}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if a, _, _ := s.Album(signos); a.Tracks[1].Title != "No existes" || len(a.Genres) != 0 {
		t.Fatalf("signos not cleared: %+v", a)
	}
	keys := []string{}
	for k := range unmappedMap(t, s) {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, []string{"Chamamé", "Murga"}) {
		t.Fatalf("unmapped changed without a delta: %v", keys)
	}
}
