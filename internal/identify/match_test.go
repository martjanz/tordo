package identify

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

func signosQuery() Query {
	return Query{Variants: []Variant{{Title: "Signos", Artist: "Soda Stereo"}}, Year: 1986, Durations: []int64{231500, 209000}}
}

func TestIdentifyPicksMatchingRelease(t *testing.T) {
	mb := &fakeMB{
		search:   map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {otherHit(), signosHit()}},
		releases: map[string]musicbrainz.Release{signosID: signosRelease()},
	}
	fetched := map[string]musicbrainz.Release{}
	m, err := Identify(context.Background(), mb, signosQuery(), fetched)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != store.StatusAuto || m.ReleaseID != signosID || m.Confidence < Threshold || m.MatcherVersion != MatcherVersion ||
		m.Query != signosQuery().Key() {
		t.Fatalf("match = %+v", m)
	}
	if want := []string{"search Signos|Soda Stereo|false", "release " + signosID}; !reflect.DeepEqual(mb.calls, want) {
		t.Fatalf("calls = %q", mb.calls)
	}
	if _, ok := fetched[signosID]; !ok || len(m.Candidates) != 2 || m.Candidates[0].ReleaseID != signosID {
		t.Fatalf("fetched %v, candidates %+v", fetched, m.Candidates)
	}
}

func TestIdentifyBelowThresholdKeepsCandidates(t *testing.T) {
	mb := &fakeMB{search: map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {otherHit()}}}
	m, err := Identify(context.Background(), mb, signosQuery(), map[string]musicbrainz.Release{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != store.StatusUnmatched || m.ReleaseID != "" || len(m.Candidates) != 1 || m.Candidates[0].ReleaseID != otherID ||
		m.Confidence != m.Candidates[0].Score || mb.count("release ") != 0 {
		t.Fatalf("match = %+v, calls %q", m, mb.calls)
	}
}

func TestIdentifyTriesFolderVariant(t *testing.T) {
	const miVida = "a1b2c3d4-0000-4000-8000-000000000010"
	rel := musicbrainz.Release{ID: miVida, Title: "Mi vida con ellas", Artist: "Fito Páez", ArtistNames: []string{"Fito Páez"},
		Year: 2004, Tracks: []musicbrainz.Track{{Title: "Uno", LengthMs: 200000}}}
	mb := &fakeMB{
		search: map[string][]musicbrainz.ReleaseHit{"Mi vida con ellas|Fito Páez|false": {{ID: miVida, Title: "Mi vida con ellas",
			Artist: "Fito Páez", ArtistNames: []string{"Fito Páez"}, Year: 2004, Tracks: 1}}},
		releases: map[string]musicbrainz.Release{miVida: rel},
	}
	q := Query{Variants: []Variant{{Title: "FITO PAEZ - 2004 - mi vida con ellas 1", Artist: "Fito Páez"},
		{Title: "Mi vida con ellas", Artist: "Fito Páez"}}, Year: 2004, Durations: []int64{201000}}
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusAuto || m.ReleaseID != miVida {
		t.Fatalf("match = %+v, err %v, calls %q", m, err, mb.calls)
	}
}

func TestIdentifyTrustsTagReleaseID(t *testing.T) {
	mb := &fakeMB{releases: map[string]musicbrainz.Release{signosID: signosRelease()}}
	q := signosQuery()
	q.ReleaseID = signosID
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusAuto || m.Confidence != 1 || mb.count("search ") != 0 {
		t.Fatalf("match = %+v, err %v, calls %q", m, err, mb.calls)
	}
}

func TestIdentifyUnknownTagReleaseIDSearches(t *testing.T) {
	mb := &fakeMB{
		search:   map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {signosHit()}},
		releases: map[string]musicbrainz.Release{signosID: signosRelease()},
	}
	q := signosQuery()
	q.ReleaseID = otherID
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.ReleaseID != signosID {
		t.Fatalf("match = %+v, err %v", m, err)
	}
}

func TestIdentifyVariousArtists(t *testing.T) {
	const mpb = "a1b2c3d4-0000-4000-8000-000000000011"
	rel := musicbrainz.Release{ID: mpb, Title: "História da MPB", Artist: "Various Artists", Various: true,
		ArtistIDs: []string{musicbrainz.VariousArtistsID}, Tracks: []musicbrainz.Track{{LengthMs: 180000}}}
	mb := &fakeMB{
		search: map[string][]musicbrainz.ReleaseHit{"História da MPB||true": {{ID: mpb, Title: "História da MPB",
			Artist: "Various Artists", Various: true, Tracks: 1}}},
		releases: map[string]musicbrainz.Release{mpb: rel},
	}
	q := Query{Variants: []Variant{{Title: "História da MPB", Various: true}}, Durations: []int64{181000}}
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusAuto || m.ReleaseID != mpb {
		t.Fatalf("match = %+v, err %v", m, err)
	}
}

func TestIdentifyOfflineIsFatal(t *testing.T) {
	mb := &fakeMB{err: fmt.Errorf("%w: prueba", httpx.ErrOffline)}
	if _, err := Identify(context.Background(), mb, signosQuery(), map[string]musicbrainz.Release{}); !errors.Is(err, httpx.ErrOffline) {
		t.Fatalf("err = %v", err)
	}
}

func TestIdentifyRejectedQueryIsNotFatal(t *testing.T) {
	mb := &fakeMB{err: &httpx.StatusError{Code: 400, URL: "x"}}
	m, err := Identify(context.Background(), mb, signosQuery(), map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusUnmatched {
		t.Fatalf("match = %+v, err %v", m, err)
	}
}

func TestIdentifyWithoutVariants(t *testing.T) {
	mb := &fakeMB{}
	m, err := Identify(context.Background(), mb, Query{}, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusUnmatched || len(mb.calls) != 0 {
		t.Fatalf("match = %+v, err %v, calls %q", m, err, mb.calls)
	}
}

func TestIdentifyUsesLookupScoreNotHitScore(t *testing.T) {
	const badID = "a1b2c3d4-0000-4000-8000-000000000099"
	// Hit claims 2 tracks matching the query
	badHit := musicbrainz.ReleaseHit{ID: badID, Title: "Signos", Artist: "Soda Stereo",
		ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR", Tracks: 2, Score: 100}
	// But the release actually has 12 tracks with mismatched durations
	badRelease := musicbrainz.Release{ID: badID, Title: "Signos (Extended)", Artist: "Soda Stereo",
		ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR",
		Tracks: make([]musicbrainz.Track, 12)}
	for i := 0; i < 12; i++ {
		badRelease.Tracks[i] = musicbrainz.Track{Disc: 1, Position: i + 1, LengthMs: 100000}
	}
	mb := &fakeMB{
		search:   map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {badHit}},
		releases: map[string]musicbrainz.Release{badID: badRelease},
	}
	q := Query{Variants: []Variant{{Title: "Signos", Artist: "Soda Stereo"}}, Year: 1986, Durations: []int64{231500, 209000}}
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != store.StatusUnmatched {
		t.Fatalf("expected unmatched, got status %v", m.Status)
	}
	if len(m.Candidates) == 0 {
		t.Fatalf("expected candidates, got none")
	}
	// The candidate's score should be based on releaseScore (low due to track mismatch),
	// not the hitScore (high due to 2 matching tracks)
	hitScoreVal := hitScore(q.Variants[0], q, badHit)
	lookupScoreVal := releaseScore(q.Variants[0], q, badRelease)
	if hitScoreVal+durationsWeight < Threshold || lookupScoreVal >= Threshold || hitScoreVal <= lookupScoreVal {
		t.Fatalf("precondition: hit score %v must be worth a lookup and exceed lookup score %v, which must stay below the threshold", hitScoreVal, lookupScoreVal)
	}
	if m.Candidates[0].Score != round(lookupScoreVal) {
		t.Fatalf("candidate score %v should equal round(releaseScore) %v, not round(hitScore) %v",
			m.Candidates[0].Score, round(lookupScoreVal), round(hitScoreVal))
	}
}

func TestIdentifyKeepsBestLookupScoreAcrossVariants(t *testing.T) {
	const badID = "a1b2c3d4-0000-4000-8000-000000000099"
	hit := func(title string) musicbrainz.ReleaseHit {
		return musicbrainz.ReleaseHit{ID: badID, Title: title, Artist: "Soda Stereo",
			ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR", Tracks: 2, Score: 100}
	}
	rel := musicbrainz.Release{ID: badID, Title: "Signos (Extended)", Artist: "Soda Stereo",
		ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR", Tracks: make([]musicbrainz.Track, 12)}
	for i := range rel.Tracks {
		rel.Tracks[i] = musicbrainz.Track{Disc: 1, Position: i + 1, LengthMs: 100000}
	}
	mb := &fakeMB{
		search: map[string][]musicbrainz.ReleaseHit{
			"Signos|Soda Stereo|false":     {hit("Signos")},
			"Disco Otro|Soda Stereo|false": {hit("Disco Otro")},
		},
		releases: map[string]musicbrainz.Release{badID: rel},
	}
	q := Query{Variants: []Variant{{Title: "Signos", Artist: "Soda Stereo"}, {Title: "Disco Otro", Artist: "Soda Stereo"}},
		Year: 1986, Durations: []int64{231500, 209000}}
	first, second := releaseScore(q.Variants[0], q, rel), releaseScore(q.Variants[1], q, rel)
	if first >= Threshold || second >= first {
		t.Fatalf("precondition: want second %v < first %v < threshold", second, first)
	}
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil {
		t.Fatal(err)
	}
	if mb.count("search ") != 2 || len(m.Candidates) != 1 || m.Candidates[0].Score != round(first) {
		t.Fatalf("searches %d, candidates %+v, want score %v", mb.count("search "), m.Candidates, round(first))
	}
}
