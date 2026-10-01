package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
	"tordo/internal/scan"
	"tordo/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	roots := []string{"../musica"}
	if err := st.SyncFiles([]store.FileRow{{Path: "../musica/Virus - Locura/01.mp3", Size: 1, MTime: 1, Kind: "audio"}}, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	p, _ := st.PendingReads(env)
	st.SaveReads([]store.ReadResult{{Target: p[0], Env: env, Fingerprint: "fp",
		Info: audiofile.Info{Format: "mp3", DurationMs: 1000, Tags: audiofile.Tags{Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura"}}}})
	tracks, images, _ := st.GroupingInput()
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
	return st
}

func do(t *testing.T, h http.Handler, method, target string, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:4747"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatal(err)
	}
}

func TestReadEndpoints(t *testing.T) {
	h := (&Server{Store: newStore(t)}).Handler()

	var status struct {
		ReadOnly bool         `json:"readOnly"`
		Counts   store.Counts `json:"counts"`
	}
	decode(t, do(t, h, "GET", "/api/status", ""), &status)
	if status.Counts.Albums != 1 || status.Counts.Tracks != 1 {
		t.Fatalf("status = %+v", status)
	}

	var albums []store.AlbumView
	decode(t, do(t, h, "GET", "/api/albums", ""), &albums)
	if len(albums) != 1 || albums[0].Title != "Locura" {
		t.Fatalf("albums = %+v", albums)
	}
	var detail store.AlbumDetail
	decode(t, do(t, h, "GET", "/api/albums/"+itoa(albums[0].ID), ""), &detail)
	if len(detail.Tracks) != 1 || detail.Tracks[0].Title != "Una luna de miel en la mano" {
		t.Fatalf("album = %+v", detail)
	}
	if rec := do(t, h, "GET", "/api/albums/9999", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown album: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/albums/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}

	var top store.FolderView
	decode(t, do(t, h, "GET", "/api/folders", ""), &top)
	if len(top.Children) != 1 || top.Children[0].Path != "../musica" {
		t.Fatalf("roots = %+v", top)
	}
	if rec := do(t, h, "GET", "/api/folders?path=../nope", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown folder: %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/collections", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("collections = %d %s", rec.Code, rec.Body)
	}
}

func TestLocalOnly(t *testing.T) {
	h := (&Server{Store: newStore(t)}).Handler()
	req := httptest.NewRequest("GET", "/api/status", nil)
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign host: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/status", "", "Sec-Fetch-Site", "cross-site", "Sec-Fetch-Mode", "no-cors"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site fetch: %d", rec.Code)
	}
}

func TestScanEndpoint(t *testing.T) {
	st := newStore(t)
	if rec := do(t, (&Server{Store: st, ReadOnly: true}).Handler(), "POST", "/api/scan", "{}", "Content-Type", "application/json"); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only: %d", rec.Code)
	}
	sc := &scan.Scanner{AppDir: t.TempDir(), Store: st, Env: store.ReadEnv{Version: 1},
		Read: func(_ context.Context, _ string) (audiofile.Info, error) { return audiofile.Info{}, nil }}
	h := (&Server{Store: st, Scanner: sc}).Handler()
	if rec := do(t, h, "POST", "/api/scan", "{}"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("without JSON: %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/scan", "{}", "Content-Type", "application/json"); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(5 * time.Second)
	for sc.Status().Finished.IsZero() {
		if time.Now().After(deadline) {
			t.Fatal("the scan did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
