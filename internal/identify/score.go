package identify

import (
	"tordo/internal/musicbrainz"
	"tordo/internal/nameparse"
)

// Threshold is the confidence above which an album is identified
// automatically; below it, the album goes to Sin identificar.
const Threshold = 0.80

// Weights of the confidence score (they add up to 1).
const (
	titleWeight     = 0.30
	artistWeight    = 0.20
	yearWeight      = 0.10
	tracksWeight    = 0.15
	durationsWeight = 0.25
)

// durationSlackMs is how far apart two durations of the same track may be.
const durationSlackMs = 3000

// similarity compares two names after Norm: 1 − edit distance / longer
// length. Empty names are not similar to anything.
func similarity(a, b string) float64 {
	ra, rb := []rune(nameparse.Norm(a)), []rune(nameparse.Norm(b))
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	return 1 - float64(levenshtein(ra, rb))/float64(max(len(ra), len(rb)))
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func titleScore(v Variant, title string) float64 {
	return max(similarity(v.Title, title), similarity(stripTitle(v.Title), stripTitle(title)))
}

// artistScore compares the album's artist with the release's credit and
// each credited name. A compilation against a single-artist release (or the
// reverse) is unlikely but possible: a composer's songbook credited to him.
func artistScore(v Variant, various bool, credit string, names []string) float64 {
	switch {
	case v.Various && various:
		return 1
	case v.Various != various:
		return 0.3
	case v.Artist == "":
		return 0.5
	}
	best := similarity(v.Artist, credit)
	for _, n := range names {
		best = max(best, similarity(v.Artist, n))
	}
	return best
}

// yearScore rewards the exact year (of the release or its first release)
// and half of it for ±1. Without a year it gives a neutral half.
func yearScore(year int, candidates ...int) float64 {
	if year == 0 {
		return 0.5
	}
	best := 0.0
	for _, y := range candidates {
		switch {
		case y == year:
			return 1
		case y != 0 && (y == year-1 || y == year+1):
			best = 0.5
		}
	}
	return best
}

// tracksScore: the same count is best; an album missing tracks gets a
// share; more tracks than the release has, nothing.
func tracksScore(n, m int) float64 {
	switch {
	case n == m:
		return 1
	case n < m:
		return 0.5 * float64(n) / float64(m)
	}
	return 0
}

// durationsScore is the share of the album's tracks (with a known
// duration) that match a release track: position by position when both
// have as many tracks, else any unused track. Unknown durations give a
// neutral half.
func durationsScore(album, release []int64) float64 {
	known := 0
	for _, d := range album {
		if d > 0 {
			known++
		}
	}
	if known == 0 || len(release) == 0 {
		return 0.5
	}
	matched := 0
	if len(album) == len(release) {
		for i, d := range album {
			if near(d, release[i]) {
				matched++
			}
		}
	} else {
		used := make([]bool, len(release))
		for _, d := range album {
			for j, r := range release {
				if !used[j] && near(d, r) {
					used[j] = true
					matched++
					break
				}
			}
		}
	}
	return float64(matched) / float64(known)
}

func near(a, b int64) bool { return a > 0 && b > 0 && absDiff(a, b) <= durationSlackMs }

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

// hitScore is a search hit's score without durations (a hit has none):
// hitScore + durationsWeight is the most it can reach once looked up.
func hitScore(v Variant, q Query, h musicbrainz.ReleaseHit) float64 {
	return titleWeight*titleScore(v, h.Title) + artistWeight*artistScore(v, h.Various, h.Artist, h.ArtistNames) +
		yearWeight*yearScore(q.Year, h.Year) + tracksWeight*tracksScore(len(q.Durations), h.Tracks)
}

// releaseScore is a looked-up release's confidence score.
func releaseScore(v Variant, q Query, rel musicbrainz.Release) float64 {
	lengths := make([]int64, len(rel.Tracks))
	for i, t := range rel.Tracks {
		lengths[i] = t.LengthMs
	}
	return titleWeight*titleScore(v, rel.Title) + artistWeight*artistScore(v, rel.Various, rel.Artist, rel.ArtistNames) +
		yearWeight*yearScore(q.Year, rel.Year, rel.OriginalYear) + tracksWeight*tracksScore(len(q.Durations), len(rel.Tracks)) +
		durationsWeight*durationsScore(q.Durations, lengths)
}
