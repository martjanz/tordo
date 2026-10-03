package discogs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"tordo/internal/httpx"
)

func newTest(t *testing.T, token string) (*Client, *[]*http.Request) {
	t.Helper()
	var reqs []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs = append(reqs, r)
		if r.Header.Get("Authorization") == "Discogs token=malo" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/oauth/identity" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{}"))
			return
		}
		files := map[string]string{"/releases/1234": "release-1234.json", "/masters/5678": "master-5678.json",
			"/database/search": "search-signos.json"}
		name, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, _ := os.ReadFile("testdata/" + name)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	c := New(token)
	c.Base = srv.URL
	c.HTTP.Limiter, c.HTTP.Backoff = nil, time.Millisecond
	return c, &reqs
}

func TestReleaseAndMaster(t *testing.T) {
	c, reqs := newTest(t, "")
	info, err := c.Release(context.Background(), 1234)
	if err != nil || !reflect.DeepEqual(info, Info{ID: 1234, MasterID: 5678, Genres: []string{"Rock"}, Styles: []string{"Pop Rock", "New Wave"}}) {
		t.Fatalf("release = %+v, err %v", info, err)
	}
	if r := (*reqs)[0]; r.Header.Get("User-Agent") != httpx.UserAgent || r.Header.Get("Authorization") != "" {
		t.Fatalf("headers = %v", r.Header)
	}
	m, err := c.Master(context.Background(), 5678)
	if err != nil || !reflect.DeepEqual(m, Info{MasterID: 5678, Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}) {
		t.Fatalf("master = %+v, err %v", m, err)
	}
	if _, err := c.Release(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSearch(t *testing.T) {
	c, reqs := newTest(t, "secreto")
	rs, err := c.Search(context.Background(), "Soda Stereo", "Signos")
	want := []Result{{ID: 1234, MasterID: 5678, Artist: "Soda Stereo", Title: "Signos", Year: 1986,
		Genres: []string{"Rock"}, Styles: []string{"Pop Rock", "New Wave"}}}
	if err != nil || !reflect.DeepEqual(rs, want) {
		t.Fatalf("results = %+v, err %v", rs, err)
	}
	r := (*reqs)[0]
	q := r.URL.Query()
	if r.Header.Get("Authorization") != "Discogs token=secreto" || q.Get("type") != "release" ||
		q.Get("artist") != "Soda Stereo" || q.Get("release_title") != "Signos" {
		t.Fatalf("request %s, headers %v", r.URL, r.Header)
	}
}

func TestSearchNeedsToken(t *testing.T) {
	c, reqs := newTest(t, "")
	if c.HasToken() {
		t.Fatal("HasToken without a token")
	}
	if _, err := c.Search(context.Background(), "Soda Stereo", "Signos"); !errors.Is(err, ErrNoToken) || len(*reqs) != 0 {
		t.Fatalf("err = %v, requests %d", err, len(*reqs))
	}
}

func TestBadToken(t *testing.T) {
	c, _ := newTest(t, "malo")
	if _, err := c.Search(context.Background(), "Soda Stereo", "Signos"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestNewRates(t *testing.T) {
	if r := New("").HTTP.Limiter.Rate; r != 25.0/60 {
		t.Fatalf("anonymous rate = %v", r)
	}
	if r := New("x").HTTP.Limiter.Rate; r != 55.0/60 {
		t.Fatalf("token rate = %v", r)
	}
}

func TestIdentity(t *testing.T) {
	c, reqs := newTest(t, "bueno")
	err := c.Identity(context.Background())
	if err != nil {
		t.Fatalf("Identity with good token: err %v", err)
	}
	if len(*reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(*reqs))
	}
	if (*reqs)[0].URL.Path != "/oauth/identity" {
		t.Fatalf("expected /oauth/identity, got %s", (*reqs)[0].URL.Path)
	}

	c, reqs = newTest(t, "malo")
	err = c.Identity(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Identity with bad token: err %v, want ErrUnauthorized", err)
	}
}

func TestParseURL(t *testing.T) {
	cases := []struct {
		in   string
		kind string
		id   int
		ok   bool
	}{
		{"https://www.discogs.com/release/1234", "release", 1234, true},
		{"https://www.discogs.com/release/1234-Soda-Stereo-Signos", "release", 1234, true},
		{"https://www.discogs.com/Soda-Stereo-Signos/master/5678", "master", 5678, true},
		{"https://www.discogs.com/es/master/5678-Signos", "master", 5678, true},
		{"https://www.discogs.com/artist/99-Soda-Stereo", "", 0, false},
		{"https://example.com/release/1", "", 0, false},
	}
	for _, c := range cases {
		kind, id, ok := ParseURL(c.in)
		if kind != c.kind || id != c.id || ok != c.ok {
			t.Errorf("ParseURL(%q) = %q %d %v", c.in, kind, id, ok)
		}
	}
}
