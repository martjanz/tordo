package server

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticApp(t *testing.T) {
	s := &Server{Store: newStore(t), Static: fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>Tordo</title>")},
		"assets/index-abc1.js": {Data: []byte("console.log(1)")},
		"favicon.svg":          {Data: []byte("<svg/>")},
		".gitkeep":             {Data: []byte("")},
	}}
	h := s.Handler()
	cases := []struct {
		path, body, cache string
		code              int
	}{
		{"/", "<title>Tordo</title>", "no-cache", 200},
		{"/carpetas?ruta=../musica", "<title>Tordo</title>", "no-cache", 200},
		{"/album/12", "<title>Tordo</title>", "no-cache", 200},
		{"/assets/index-abc1.js", "console.log(1)", "public, max-age=31536000, immutable", 200},
		{"/favicon.svg", "<svg/>", "", 200},
		{"/assets/missing.js", "", "", http.StatusNotFound},
		{"/.gitkeep", "", "", http.StatusNotFound},
		{"/api/nada", "", "", http.StatusNotFound},
		{"/stream/x", "", "", http.StatusBadRequest}, // the stream route answers, not the app
	}
	for _, c := range cases {
		rec := do(t, h, "GET", c.path, "")
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.body) || rec.Header().Get("Cache-Control") != c.cache {
			t.Errorf("%s: %d %q (Cache-Control %q), want %d %q (%q)", c.path, rec.Code, rec.Body, rec.Header().Get("Cache-Control"),
				c.code, c.body, c.cache)
		}
	}
}

func TestStaticWithoutBuild(t *testing.T) {
	s := &Server{Store: newStore(t), Static: fstest.MapFS{".gitkeep": {Data: []byte("")}}}
	rec := do(t, s.Handler(), "GET", "/carpetas", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Falta la interfaz") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestEmbeddedApp(t *testing.T) {
	// Whether or not the web app was built, the embedded files answer a page.
	rec := do(t, (&Server{Store: newStore(t)}).Handler(), "GET", "/", "")
	if rec.Code != 200 || !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("status %d: %.200s", rec.Code, rec.Body)
	}
}
