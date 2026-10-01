package identify

import (
	"context"
	"errors"
	"log"
	"math"
	"sort"

	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// API is the part of the MusicBrainz client the identification uses.
type API interface {
	SearchReleases(ctx context.Context, q musicbrainz.SearchQuery) ([]musicbrainz.ReleaseHit, error)
	Release(ctx context.Context, id string) (musicbrainz.Release, error)
	ReleaseGroupLinks(ctx context.Context, id string) (musicbrainz.GroupLinks, error)
	Artist(ctx context.Context, id string) (musicbrainz.Artist, error)
	Cover(ctx context.Context, kind, id string) ([]byte, error)
}

// maxLookups is how many releases one album may look up (a request each).
const maxLookups = 4

// maxCandidates is how many candidates an unidentified album keeps.
const maxCandidates = 5

// Identify matches an album against MusicBrainz. A release id from the
// tags is trusted when MusicBrainz has it; otherwise each variant is
// searched in turn and the hits that could still reach the threshold are
// looked up (at most maxLookups per album) until one does. Releases it looks
// up are added to fetched. Only fatal errors (offline, cancelled) are
// returned: a rejected query leaves the album unmatched.
func Identify(ctx context.Context, api API, q Query, fetched map[string]musicbrainz.Release) (store.Match, error) {
	m := store.Match{Query: q.Key(), Status: store.StatusUnmatched, MatcherVersion: MatcherVersion}
	if q.ReleaseID != "" {
		rel, err := lookup(ctx, api, q.ReleaseID, fetched)
		if fatal(err) {
			return m, err
		}
		if err == nil {
			m.Status, m.ReleaseID, m.Confidence = store.StatusAuto, rel.ID, 1
			m.Candidates = []store.Candidate{candidateOf(rel, 1)}
			return m, nil
		}
	}
	best := map[string]store.Candidate{}
	lookedUp := make(map[string]bool)
	keep := func(c store.Candidate, isLookup bool) {
		if isLookup {
			// A looked-up score always replaces any hit score
			best[c.ReleaseID] = c
			lookedUp[c.ReleaseID] = true
		} else if !lookedUp[c.ReleaseID] {
			// For hit scores, keep the max only if not already looked up
			if old, ok := best[c.ReleaseID]; !ok || c.Score > old.Score {
				best[c.ReleaseID] = c
			}
		}
	}
	lookups := 0
variants:
	for _, v := range q.Variants {
		hits, err := api.SearchReleases(ctx, musicbrainz.SearchQuery{Title: v.Title, Artist: v.Artist, Various: v.Various})
		if fatal(err) {
			return m, err
		}
		if err != nil {
			log.Printf("búsqueda en MusicBrainz de %q: %v", v.Title, err)
			continue
		}
		scores := make(map[string]float64, len(hits))
		for _, h := range hits {
			scores[h.ID] = hitScore(v, q, h)
			keep(candidateOfHit(h, scores[h.ID]), false)
		}
		// Sort a copy to avoid mutating the API's returned slice
		sortedHits := make([]musicbrainz.ReleaseHit, len(hits))
		copy(sortedHits, hits)
		sort.SliceStable(sortedHits, func(i, j int) bool { return scores[sortedHits[i].ID] > scores[sortedHits[j].ID] })
		for _, h := range sortedHits {
			if scores[h.ID]+durationsWeight < Threshold {
				break
			}
			if _, ok := fetched[h.ID]; !ok {
				if lookups >= maxLookups {
					break
				}
				lookups++
			}
			rel, err := lookup(ctx, api, h.ID, fetched)
			if fatal(err) {
				return m, err
			}
			if err != nil {
				continue
			}
			s := releaseScore(v, q, rel)
			keep(candidateOf(rel, s), true)
			if s >= Threshold {
				m.Status, m.ReleaseID, m.Confidence = store.StatusAuto, rel.ID, round(s)
				break variants
			}
		}
	}
	m.Candidates = top(best)
	if m.Status != store.StatusAuto && len(m.Candidates) > 0 {
		m.Confidence = m.Candidates[0].Score
	}
	return m, nil
}

// lookup gets a release, from fetched when this run already looked it up.
func lookup(ctx context.Context, api API, id string, fetched map[string]musicbrainz.Release) (musicbrainz.Release, error) {
	if r, ok := fetched[id]; ok {
		return r, nil
	}
	r, err := api.Release(ctx, id)
	if err == nil {
		fetched[id] = r
	}
	return r, err
}

// fatal errors stop a run: no network, or cancellation. Other errors only
// skip the item at hand.
func fatal(err error) bool {
	return errors.Is(err, httpx.ErrOffline) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func candidateOfHit(h musicbrainz.ReleaseHit, score float64) store.Candidate {
	return store.Candidate{ReleaseID: h.ID, Title: h.Title, Artist: h.Artist, Year: h.Year, Country: h.Country,
		Tracks: h.Tracks, Score: round(score)}
}

func candidateOf(r musicbrainz.Release, score float64) store.Candidate {
	return store.Candidate{ReleaseID: r.ID, Title: r.Title, Artist: r.Artist, Year: r.Year, Country: r.Country,
		Tracks: len(r.Tracks), Score: round(score)}
}

func round(s float64) float64 { return math.Round(s*100) / 100 }

func top(best map[string]store.Candidate) []store.Candidate {
	out := make([]store.Candidate, 0, len(best))
	for _, c := range best {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ReleaseID < out[j].ReleaseID
	})
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out
}
