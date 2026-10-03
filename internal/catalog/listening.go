package catalog

import (
	"time"

	"tordo/internal/store"
)

// Explorar's Escucha facet: what the user has not listened to lately.
const (
	FacetListened = "escucha"
	ListenedNever = "nunca"        // never played
	ListenedStale = "hace-un-anio" // played, but not in over a year
)

// staleAfter is how old the last play of something "sin escuchar hace un año" is.
const staleAfter = 365 * 24 * time.Hour

// now is the catalog's clock; tests replace it.
var now = time.Now

// listening is the user's plays summed up per album: computed once per call.
type listening struct {
	tracks map[int64]store.Plays // p.Plays (nil p: nil map)
	albums map[int64]store.Plays // per album: plays summed, latest play
	cutoff int64                 // unix ms: a latest play before it is stale
}

// listening sums p's plays per album. It is nil without user data, and then
// nothing has Escucha values.
func (c *Catalog) listening(p *store.Personal) *listening {
	if p == nil {
		return nil
	}
	l := &listening{tracks: p.Plays, albums: map[int64]store.Plays{}, cutoff: now().Add(-staleAfter).UnixMilli()}
	for id, pl := range p.Plays {
		t, ok := c.Track(id)
		if !ok || t.AlbumID == 0 {
			continue
		}
		a := l.albums[t.AlbumID]
		a.Count += pl.Count
		a.Last = max(a.Last, pl.Last)
		l.albums[t.AlbumID] = a
	}
	return l
}

func (l *listening) trackValues(id int64) []string {
	if l == nil {
		return nil
	}
	return l.values(l.tracks[id])
}

func (l *listening) albumValues(id int64) []string {
	if l == nil {
		return nil
	}
	return l.values(l.albums[id])
}

// values is the Escucha value of something played pl: never, or stale.
func (l *listening) values(pl store.Plays) []string {
	switch {
	case pl.Count <= 0:
		return []string{ListenedNever}
	case pl.Last < l.cutoff:
		return []string{ListenedStale}
	}
	return nil
}
