package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"tordo/internal/covers"
	"tordo/internal/httpx"
	"tordo/internal/identify"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

type fakeIdentifier struct {
	triggered, applied int
	set                map[int64]string
	ignored, reset     []int64
	err                error
}

func (f *fakeIdentifier) Status() identify.Status {
	return identify.Status{State: identify.StateIdle, Discogs: identify.DiscogsNoToken}
}
func (f *fakeIdentifier) Trigger()     { f.triggered++ }
func (f *fakeIdentifier) Apply() error { f.applied++; return nil }
func (f *fakeIdentifier) SetRelease(ctx context.Context, id int64, rel string) error {
	if f.set == nil {
		f.set = map[int64]string{}
	}
	f.set[id] = rel
	return f.err
}
func (f *fakeIdentifier) Ignore(id int64) error { f.ignored = append(f.ignored, id); return f.err }
func (f *fakeIdentifier) Reset(id int64) error  { f.reset = append(f.reset, id); return f.err }

func identifyServer(t *testing.T) (*Server, *fakeIdentifier, int64) {
	t.Helper()
	st := newStore(t)
	as, err := st.Albums("")
	if err != nil || len(as) != 1 {
		t.Fatalf("albums = %+v, err %v", as, err)
	}
	f := &fakeIdentifier{}
	return &Server{Store: st, Identifier: f, AppDir: t.TempDir()}, f, as[0].ID
}

const (
	jsonCT = "application/json"
	respCT = "application/json; charset=utf-8"
)

func TestStatusIncludesIdentify(t *testing.T) {
	s, _, _ := identifyServer(t)
	var got struct {
		Identify identify.Status `json:"identify"`
	}
	rec := do(t, s.Handler(), "GET", "/api/status", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Identify.State != identify.StateIdle {
		t.Fatalf("status %s, err %v", rec.Body, err)
	}
	s.Identifier = nil
	rec = do(t, s.Handler(), "GET", "/api/status", "")
	var off struct {
		Identify map[string]string `json:"identify"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &off); off.Identify["state"] != "off" {
		t.Fatalf("browse-only status %s", rec.Body)
	}
}

func TestIdentifyEndpoint(t *testing.T) {
	s, f, _ := identifyServer(t)
	if rec := do(t, s.Handler(), "POST", "/api/identify", "{}"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("without JSON content type: %d", rec.Code)
	}
	if rec := do(t, s.Handler(), "POST", "/api/identify", "{}", "Content-Type", jsonCT); rec.Code != http.StatusAccepted || f.triggered != 1 || rec.Result().Header.Get("Content-Type") != respCT {
		t.Fatalf("code %d, triggered %d, content type %q", rec.Code, f.triggered, rec.Result().Header.Get("Content-Type"))
	}
	s.Identifier = nil
	if rec := do(t, s.Handler(), "POST", "/api/identify", "{}", "Content-Type", jsonCT); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only: %d", rec.Code)
	}
}

func TestSetIdentification(t *testing.T) {
	s, f, id := identifyServer(t)
	url := "/api/albums/" + strconv.FormatInt(id, 10) + "/identification"
	const mbid = "a1b2c3d4-0000-4000-8000-000000000001"
	rec := do(t, s.Handler(), "PUT", url, `{"releaseId": "https://musicbrainz.org/release/`+mbid+`?x=1"}`, "Content-Type", jsonCT)
	var a store.AlbumDetail
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &a) != nil || a.ID != id || f.set[id] != mbid || f.triggered != 1 {
		t.Fatalf("code %d body %s set %v triggered %d", rec.Code, rec.Body, f.set, f.triggered)
	}
	if rec := do(t, s.Handler(), "PUT", url, `{"ignore": true}`, "Content-Type", jsonCT); rec.Code != http.StatusOK || len(f.ignored) != 1 {
		t.Fatalf("ignore: %d %v", rec.Code, f.ignored)
	}
	if rec := do(t, s.Handler(), "DELETE", url, ""); rec.Code != http.StatusOK || len(f.reset) != 1 {
		t.Fatalf("reset: %d %v", rec.Code, f.reset)
	}
	if rec := do(t, s.Handler(), "PUT", url, `{}`, "Content-Type", jsonCT); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body: %d", rec.Code)
	}
	errs := []struct {
		err  error
		code int
	}{
		{musicbrainz.ErrInvalidID, http.StatusBadRequest},
		{musicbrainz.ErrNotFound, http.StatusNotFound},
		{identify.ErrNoAlbum, http.StatusNotFound},
		{fmt.Errorf("%w: prueba", httpx.ErrOffline), http.StatusServiceUnavailable},
		{errors.New("otro"), http.StatusInternalServerError},
	}
	for _, e := range errs {
		f.err = e.err
		if rec := do(t, s.Handler(), "PUT", url, `{"releaseId": "x"}`, "Content-Type", jsonCT); rec.Code != e.code {
			t.Errorf("%v: code %d, want %d", e.err, rec.Code, e.code)
		}
	}
}

func TestUnidentifiedEndpoint(t *testing.T) {
	s, _, id := identifyServer(t)
	cands := []store.Candidate{{ReleaseID: "r1", Title: "Locura", Artist: "Virus", Score: 0.7}}
	if err := s.Store.SaveMatches([]store.Match{{Query: "q", Status: store.StatusUnmatched, Candidates: cands, MatcherVersion: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ApplyIdentities([]store.AlbumIdentity{{AlbumID: id, Query: "q", Status: store.StatusUnmatched}}, nil); err != nil {
		t.Fatal(err)
	}
	rec := do(t, s.Handler(), "GET", "/api/review/unidentified", "")
	var got []store.UnidentifiedView
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got) != 1 || got[0].Candidates[0].ReleaseID != "r1" {
		t.Fatalf("code %d body %s", rec.Code, rec.Body)
	}
}

// TestCachedCover: an album without a picture of its own gets the Cover Art
// Archive image the identification downloaded, through the covers endpoint.
func TestCachedCover(t *testing.T) {
	s, _, id := identifyServer(t)
	s.Covers = covers.New(s.Store, s.AppDir, "")
	url := "/api/covers/album/" + strconv.FormatInt(id, 10)
	if rec := do(t, s.Handler(), "GET", url, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no cover: %d", rec.Code)
	}
	name := filepath.Join(s.AppDir, "cache", "covers", "r1.jpg")
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	jpeg := []byte("\xff\xd8\xff\xe0 jpeg")
	if err := os.WriteFile(name, jpeg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ApplyIdentities([]store.AlbumIdentity{{AlbumID: id, Status: store.StatusAuto, ReleaseID: "r1",
		CoverCache: "cache/covers/r1.jpg"}}, nil); err != nil {
		t.Fatal(err)
	}
	rec := do(t, s.Handler(), "GET", url, "")
	if rec.Code != http.StatusOK || rec.Body.String() != string(jpeg) || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("cached cover: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if rec := do(t, s.Handler(), "GET", "/api/albums/"+strconv.FormatInt(id, 10)+"/cover", ""); rec.Code != http.StatusNotFound {
		t.Fatal("the M2 cover route is gone: /api/covers/album serves every picture")
	}
}

func TestGenreEndpoints(t *testing.T) {
	s, f, _ := identifyServer(t)
	rec := do(t, s.Handler(), "GET", "/api/genres", "")
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) || len(rec.Body.String()) < 100 {
		t.Fatalf("taxonomy: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, s.Handler(), "GET", "/api/genres/unmapped", ""); rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Fatalf("unmapped: %d %q", rec.Code, rec.Body)
	}
	bad := []string{`{"raw": "Chamamé", "genre": "Cumbia"}`, `{"raw": "Chamamé", "genre": "Latin", "style": "Pop Rock"}`,
		`{"raw": "", "genre": "Latin"}`, `{"raw": "x", "style": "Samba"}`}
	for _, b := range bad {
		if rec := do(t, s.Handler(), "PUT", "/api/genres/aliases", b, "Content-Type", jsonCT); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", b, rec.Code)
		}
	}
	rec = do(t, s.Handler(), "PUT", "/api/genres/aliases", `{"raw": "Chamamé", "genre": "Folk, World, & Country", "style": "Folk"}`,
		"Content-Type", jsonCT)
	if rec.Code != http.StatusOK || f.applied != 1 {
		t.Fatalf("alias: %d %s, applied %d", rec.Code, rec.Body, f.applied)
	}
	as, _ := s.Store.GenreAliases()
	if len(as) != 1 || as[0].Raw != "chamame" {
		t.Fatalf("aliases = %+v", as)
	}
	if rec := do(t, s.Handler(), "GET", "/api/genres/aliases", ""); rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("list aliases: %d", rec.Code)
	}
}

func TestReleaseIDFrom(t *testing.T) {
	cases := map[string]string{
		" a1b2c3d4-0000-4000-8000-000000000001 ":                                            "a1b2c3d4-0000-4000-8000-000000000001",
		"https://musicbrainz.org/release/a1b2c3d4-0000-4000-8000-000000000001":              "a1b2c3d4-0000-4000-8000-000000000001",
		"https://musicbrainz.org/release/a1b2c3d4-0000-4000-8000-000000000001/recordings?x": "a1b2c3d4-0000-4000-8000-000000000001",
		"cualquier cosa": "cualquier cosa",
	}
	for in, want := range cases {
		if got := releaseIDFrom(in); got != want {
			t.Errorf("releaseIDFrom(%q) = %q, want %q", in, got, want)
		}
	}
}
