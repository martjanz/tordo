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
