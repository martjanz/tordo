package identify

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"tordo/internal/discogs"
	"tordo/internal/musicbrainz"
)

const (
	signosID    = "a1b2c3d4-0000-4000-8000-000000000001"
	signosGroup = "a1b2c3d4-0000-4000-8000-000000000002"
	sodaID      = "a1b2c3d4-0000-4000-8000-000000000003"
	otherID     = "a1b2c3d4-0000-4000-8000-000000000004"
)

func signosRelease() musicbrainz.Release {
	return musicbrainz.Release{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistIDs: []string{sodaID},
		ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR", GroupID: signosGroup, OriginalYear: 1986,
		Tracks: []musicbrainz.Track{
			{Disc: 1, Position: 1, Title: "Táctica y estrategia", LengthMs: 231000, RecordingID: "rec-1"},
			{Disc: 1, Position: 2, Title: "Sin sobresaltos", LengthMs: 210000, RecordingID: "rec-2"},
		},
		Genres: []string{"rock"}, DiscogsURLs: []string{"https://www.discogs.com/release/1234-Soda-Stereo-Signos"}}
}

func signosHit() musicbrainz.ReleaseHit {
	return musicbrainz.ReleaseHit{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistNames: []string{"Soda Stereo"},
		Year: 1986, Country: "AR", Tracks: 2, Score: 100}
}

// otherHit looks like Signos by title only.
func otherHit() musicbrainz.ReleaseHit {
	return musicbrainz.ReleaseHit{ID: otherID, Title: "Signos", Artist: "Los Signos", ArtistNames: []string{"Los Signos"},
		Year: 2001, Tracks: 12, Score: 80}
}

// fakeMB answers from maps and records every call. Search keys are
// "title|artist|various".
type fakeMB struct {
	mu       sync.Mutex
	search   map[string][]musicbrainz.ReleaseHit
	releases map[string]musicbrainz.Release
	groups   map[string]musicbrainz.GroupLinks
	artists  map[string]musicbrainz.Artist
	covers   map[string][]byte // "kind/id"
	err      error             // returned by every call when set
	calls    []string
}

func (f *fakeMB) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeMB) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeMB) SearchReleases(ctx context.Context, q musicbrainz.SearchQuery) ([]musicbrainz.ReleaseHit, error) {
	key := fmt.Sprintf("%s|%s|%t", q.Title, q.Artist, q.Various)
	if err := f.record("search " + key); err != nil {
		return nil, err
	}
	return f.search[key], nil
}

func (f *fakeMB) Release(ctx context.Context, id string) (musicbrainz.Release, error) {
	if err := f.record("release " + id); err != nil {
		return musicbrainz.Release{}, err
	}
	r, ok := f.releases[id]
	if !ok {
		return r, musicbrainz.ErrNotFound
	}
	return r, nil
}

func (f *fakeMB) ReleaseGroupLinks(ctx context.Context, id string) (musicbrainz.GroupLinks, error) {
	if err := f.record("group " + id); err != nil {
		return musicbrainz.GroupLinks{}, err
	}
	return f.groups[id], nil
}

func (f *fakeMB) Artist(ctx context.Context, id string) (musicbrainz.Artist, error) {
	if err := f.record("artist " + id); err != nil {
		return musicbrainz.Artist{}, err
	}
	a, ok := f.artists[id]
	if !ok {
		return a, musicbrainz.ErrNotFound
	}
	return a, nil
}

func (f *fakeMB) Cover(ctx context.Context, kind, id string) ([]byte, error) {
	if err := f.record("cover " + kind + "/" + id); err != nil {
		return nil, err
	}
	b, ok := f.covers[kind+"/"+id]
	if !ok {
		return nil, musicbrainz.ErrNotFound
	}
	return b, nil
}

// fakeDiscogs answers from maps. Search keys are "artist|title".
type fakeDiscogs struct {
	mu       sync.Mutex
	token    bool
	releases map[int]discogs.Info
	masters  map[int]discogs.Info
	search   map[string][]discogs.Result
	err      error
	calls    []string
}

func (f *fakeDiscogs) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeDiscogs) HasToken() bool { return f.token }

func (f *fakeDiscogs) Release(ctx context.Context, id int) (discogs.Info, error) {
	if err := f.record(fmt.Sprintf("release %d", id)); err != nil {
		return discogs.Info{}, err
	}
	i, ok := f.releases[id]
	if !ok {
		return i, discogs.ErrNotFound
	}
	return i, nil
}

func (f *fakeDiscogs) Master(ctx context.Context, id int) (discogs.Info, error) {
	if err := f.record(fmt.Sprintf("master %d", id)); err != nil {
		return discogs.Info{}, err
	}
	i, ok := f.masters[id]
	if !ok {
		return i, discogs.ErrNotFound
	}
	return i, nil
}

func (f *fakeDiscogs) Search(ctx context.Context, artist, title string) ([]discogs.Result, error) {
	if !f.token {
		return nil, discogs.ErrNoToken
	}
	if err := f.record("search " + artist + "|" + title); err != nil {
		return nil, err
	}
	return f.search[artist+"|"+title], nil
}
