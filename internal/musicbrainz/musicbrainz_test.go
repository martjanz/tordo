package musicbrainz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"tordo/internal/httpx"
)

const (
	signosID   = "a1b2c3d4-0000-4000-8000-000000000001"
	signosRG   = "a1b2c3d4-0000-4000-8000-000000000002"
	sodaID     = "a1b2c3d4-0000-4000-8000-000000000003"
	noelRosaID = "a1b2c3d4-0000-4000-8000-000000000006"
)

// fakeMB serves the fixtures and records the requests.
type fakeMB struct {
	mu   sync.Mutex
	reqs []*http.Request
}

func (f *fakeMB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	f.mu.Unlock()
	files := map[string]string{
		"/release":                   "search-signos.json",
		"/release/" + signosID:       "release-signos.json",
		"/release-group/" + signosRG: "release-group-signos.json",
		"/artist/" + sodaID:          "artist-soda.json",
		"/artist/" + noelRosaID:      "artist-subdivision.json",
	}
	if strings.HasPrefix(r.URL.Path, "/caa/") {
		if r.URL.Path == "/caa/release/"+signosID+"/front-500" {
			w.Write([]byte("JPEG"))
			return
		}
		http.NotFound(w, r)
		return
	}
	name, ok := files[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write(b)
}

func (f *fakeMB) last() *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func newTest(t *testing.T) (*Client, *fakeMB) {
	t.Helper()
	f := &fakeMB{}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	h := http.Header{"User-Agent": {httpx.UserAgent}}
	return &Client{Base: srv.URL, CoverBase: srv.URL + "/caa",
		HTTP:   &httpx.Client{Backoff: time.Millisecond, Header: h},
		Covers: &httpx.Client{Backoff: time.Millisecond, Header: h}}, f
}

func TestSearchReleases(t *testing.T) {
	c, f := newTest(t)
	hits, err := c.SearchReleases(context.Background(), SearchQuery{Title: "Signos (Remastered)", Artist: "Soda Stereo"})
	if err != nil {
		t.Fatal(err)
	}
	r := f.last()
	q := r.URL.Query()
	if q.Get("query") != "release:(signos remastered) AND artist:(soda stereo)" || q.Get("fmt") != "json" ||
		q.Get("limit") != "10" || r.Header.Get("User-Agent") != httpx.UserAgent {
		t.Fatalf("request %s, User-Agent %q", r.URL, r.Header.Get("User-Agent"))
	}
	want := []ReleaseHit{
		{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR", Tracks: 8, Score: 100},
		{ID: "a1b2c3d4-0000-4000-8000-000000000004", Title: "Signos", Artist: "Various Artists", ArtistNames: []string{"Various Artists"},
			Various: true, Year: 2001, Tracks: 14, Score: 91},
	}
	if !reflect.DeepEqual(hits, want) {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchVariousAndEmpty(t *testing.T) {
	c, f := newTest(t)
	if _, err := c.SearchReleases(context.Background(), SearchQuery{Title: "História da MPB", Artist: "x", Various: true}); err != nil {
		t.Fatal(err)
	}
	if got := f.last().URL.Query().Get("query"); got != "release:(história da mpb) AND arid:"+VariousArtistsID {
		t.Fatalf("query = %q", got)
	}
	n := len(f.reqs)
	if hits, err := c.SearchReleases(context.Background(), SearchQuery{Title: " ()"}); err != nil || hits != nil || len(f.reqs) != n {
		t.Fatalf("an empty title must not be searched: %v %v", hits, err)
	}
}

func TestTerms(t *testing.T) {
	cases := map[string]string{
		"Rock AND Roll":          "rock and roll",
		`AC/DC: "Live" [1992]!`:  "ac dc live 1992",
		"Magos, Espadas Y Rosas": "magos, espadas y rosas",
	}
	for in, want := range cases {
		if got := terms(in); got != want {
			t.Errorf("terms(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRelease(t *testing.T) {
	c, f := newTest(t)
	rel, err := c.Release(context.Background(), signosID)
	if err != nil {
		t.Fatal(err)
	}
	if inc := f.last().URL.Query().Get("inc"); inc != "recordings artist-credits release-groups url-rels genres" {
		t.Fatalf("inc = %q", inc)
	}
	want := Release{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistIDs: []string{sodaID}, ArtistNames: []string{"Soda Stereo"},
		Year: 1986, Country: "AR", GroupID: signosRG, OriginalYear: 1986,
		Tracks: []Track{
			{Disc: 1, Position: 1, Title: "Táctica y estrategia", LengthMs: 231000, RecordingID: "b1000000-0000-4000-8000-000000000001"},
			{Disc: 1, Position: 2, Title: "Sin sobresaltos", LengthMs: 210000, RecordingID: "b1000000-0000-4000-8000-000000000002"},
			{Disc: 1, Position: 3, Title: "Prófugos", LengthMs: 268000, RecordingID: "b1000000-0000-4000-8000-000000000003"},
		},
		Genres: []string{"rock", "new wave"}, DiscogsURLs: []string{"https://www.discogs.com/release/1234"}}
	if !reflect.DeepEqual(rel, want) {
		t.Fatalf("release = %+v", rel)
	}
}

func TestReleaseErrors(t *testing.T) {
	c, f := newTest(t)
	if _, err := c.Release(context.Background(), "a1b2c3d4-0000-4000-8000-00000000ffff"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	n := len(f.reqs)
	for _, bad := range []string{"", "../artist/x", "A1B2C3D4-0000-4000-8000-000000000001"} {
		if _, err := c.Release(context.Background(), bad); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("Release(%q) err = %v", bad, err)
		}
	}
	if len(f.reqs) != n {
		t.Fatal("an invalid id was requested")
	}
}

func TestReleaseGroupLinks(t *testing.T) {
	c, _ := newTest(t)
	g, err := c.ReleaseGroupLinks(context.Background(), signosRG)
	if err != nil || !reflect.DeepEqual(g, GroupLinks{DiscogsURLs: []string{"https://www.discogs.com/master/5678"}, Genres: []string{"rock"}}) {
		t.Fatalf("links = %+v, err %v", g, err)
	}
}

func TestArtist(t *testing.T) {
	c, _ := newTest(t)
	a, err := c.Artist(context.Background(), sodaID)
	want := Artist{ID: sodaID, Name: "Soda Stereo", SortName: "Soda Stereo", Type: "Group", Country: "AR", Area: "Argentina", Begin: "1982", End: "2007"}
	if err != nil || a != want {
		t.Fatalf("artist = %+v, err %v", a, err)
	}
	a, err = c.Artist(context.Background(), noelRosaID)
	if err != nil || a.Country != "BR" || a.Area != "Rio de Janeiro" {
		t.Fatalf("subdivision area: %+v, err %v", a, err)
	}
}

func TestCover(t *testing.T) {
	c, _ := newTest(t)
	b, err := c.Cover(context.Background(), "release", signosID)
	if err != nil || string(b) != "JPEG" {
		t.Fatalf("cover %q, err %v", b, err)
	}
	if _, err := c.Cover(context.Background(), "release-group", signosRG); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := c.Cover(context.Background(), "artist", signosID); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestNewIsPolite(t *testing.T) {
	c := New()
	if c.Base != BaseURL || c.HTTP.Limiter == nil || c.HTTP.Limiter.Rate != 1 || c.HTTP.Limiter.Burst != 1 ||
		c.HTTP.Header.Get("User-Agent") != httpx.UserAgent || c.Covers.Header.Get("User-Agent") != httpx.UserAgent {
		t.Fatalf("client = %+v", c)
	}
}
