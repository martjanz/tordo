package store

import (
	"reflect"
	"testing"

	"tordo/internal/audiofile"
)

func targetByTitle(t *testing.T, ts []IdentifyTarget, title string) IdentifyTarget {
	t.Helper()
	for _, x := range ts {
		if x.Title == title {
			return x
		}
	}
	t.Fatalf("no target %q in %+v", title, ts)
	return IdentifyTarget{}
}

func TestIdentifyTargets(t *testing.T) {
	s := sample(t)
	ts, err := s.IdentifyTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 3 {
		t.Fatalf("targets = %+v", ts)
	}
	signos := targetByTitle(t, ts, "Signos")
	dir := la + "/[LA-10] Soda Stereo - Signos"
	if signos.Artist != "Soda Stereo" || signos.Year != 1986 || !signos.HasCover || signos.Dir != dir || len(signos.Tracks) != 2 {
		t.Fatalf("signos = %+v", signos)
	}
	if tr := signos.Tracks[0]; tr.Fingerprint != "fp"+dir+"/01.mp3" || tr.Title != "Signos" || tr.Track != 1 || tr.DurationMs != 1000 {
		t.Fatalf("first track = %+v", tr)
	}
	if locura := targetByTitle(t, ts, "Locura"); locura.HasCover {
		t.Fatalf("locura has no cover: %+v", locura)
	}
}

func TestIdentifyTargetsCarryTagReleaseID(t *testing.T) {
	s := catalog(t, map[string]audiofile.Tags{
		"../musica/Virus - Locura/01.mp3": {Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", MBAlbumID: "mbid-locura"},
	})
	ts, _ := s.IdentifyTargets()
	if len(ts) != 1 || ts[0].Tracks[0].MBAlbumID != "mbid-locura" {
		t.Fatalf("targets = %+v", ts)
	}
	fps, err := s.AlbumFingerprints(ts[0].AlbumID)
	if err != nil || !reflect.DeepEqual(fps, []string{"fp../musica/Virus - Locura/01.mp3"}) {
		t.Fatalf("fingerprints = %q, err %v", fps, err)
	}
	if fps, _ := s.AlbumFingerprints(9999); len(fps) != 0 {
		t.Fatalf("unknown album has fingerprints %q", fps)
	}
}

func TestMatchesRoundTrip(t *testing.T) {
	s := open(t)
	m := Match{Query: "signos|soda stereo", Status: StatusAuto, ReleaseID: "r1", Confidence: 0.93, MatcherVersion: 1,
		Candidates: []Candidate{{ReleaseID: "r1", Title: "Signos", Artist: "Soda Stereo", Year: 1986, Country: "AR", Tracks: 8, Score: 0.93}}}
	if err := s.SaveMatches([]Match{m, {Query: "nada", Status: StatusUnmatched, MatcherVersion: 1}}); err != nil {
		t.Fatal(err)
	}
	m.Confidence = 0.95 // saving again updates
	if err := s.SaveMatches([]Match{m}); err != nil {
		t.Fatal(err)
	}
	ms, err := s.Matches()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ms[m.Query], m) || len(ms) != 2 || ms["nada"].Candidates == nil {
		t.Fatalf("matches = %+v", ms)
	}
}

func TestCorrectionsSetAndClear(t *testing.T) {
	s := open(t)
	manual := Correction{Status: StatusManual, ReleaseID: "r1"}
	if err := s.SetCorrection([]string{"fp1", "fp2", ""}, manual); err != nil {
		t.Fatal(err)
	}
	cs, _ := s.Corrections()
	if len(cs) != 2 || cs["fp1"] != manual {
		t.Fatalf("corrections = %+v", cs)
	}
	if err := s.SetCorrection([]string{"fp1"}, Correction{}); err != nil {
		t.Fatal(err)
	}
	cs, _ = s.Corrections()
	if _, ok := cs["fp1"]; ok || len(cs) != 1 {
		t.Fatalf("after clearing fp1: %+v", cs)
	}
}

func TestReleasesAndArtistsRoundTrip(t *testing.T) {
	s := open(t)
	r := Release{ID: "r1", GroupID: "g1", Title: "Signos", Artist: "Soda Stereo", ArtistIDs: []string{"a1"},
		ArtistNames: []string{"Soda Stereo"}, Year: 1986, OriginalYear: 1986, Country: "AR",
		Tracks:   []ReleaseTrack{{Disc: 1, Position: 1, Title: "Táctica y estrategia", LengthMs: 231000, RecordingID: "rec1"}},
		MBGenres: []string{"rock"}, DiscogsURLs: []string{"https://www.discogs.com/release/1234"},
		Discogs: DiscogsInfo{Done: true, ID: 1234, Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}, Cover: CoverNone}
	if err := s.SaveRelease(r); err != nil {
		t.Fatal(err)
	}
	r.Cover = "cache/covers/r1.jpg"
	if err := s.SaveRelease(r); err != nil {
		t.Fatal(err)
	}
	rs, err := s.Releases()
	if err != nil || !reflect.DeepEqual(rs["r1"], r) {
		t.Fatalf("releases = %+v, err %v", rs, err)
	}
	a := Artist{ID: "a1", Name: "Soda Stereo", SortName: "Soda Stereo", Type: "Group", Country: "AR", Area: "Argentina", Begin: "1982", End: "2007"}
	if err := s.SaveArtist(a); err != nil {
		t.Fatal(err)
	}
	as, err := s.Artists()
	if err != nil || as["a1"] != a {
		t.Fatalf("artists = %+v, err %v", as, err)
	}
}
