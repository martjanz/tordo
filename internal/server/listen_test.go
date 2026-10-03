package server

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestListenModeBlocksCuration(t *testing.T) {
	s := settingsServer(t, false)
	h := s.Handler()
	setListen := func(on string) {
		t.Helper()
		body := `{"roots":["../musica"],"ignore":[],"normalize":true,"listenMode":` + on + `,"lan":{"enabled":false,"port":4747}}`
		var v configView
		decode(t, putConfig(t, s, body), &v)
		if v.ListenMode != (on == "true") || saved(t, s).ListenMode != (on == "true") {
			t.Fatalf("listenMode %s not applied: %+v", on, v)
		}
	}
	status := func() bool {
		var st struct {
			ListenMode bool `json:"listenMode"`
		}
		decode(t, do(t, h, "GET", "/api/status", ""), &st)
		return st.ListenMode
	}
	curation := [][2]string{
		{"POST", "/api/scan"}, {"POST", "/api/identify"},
		{"PUT", "/api/albums/1/identification"}, {"DELETE", "/api/albums/1/identification"},
		{"PUT", "/api/folders/collection"},
		{"PUT", "/api/genres/aliases"}, {"DELETE", "/api/genres/aliases"},
	}
	open := [][2]string{
		{"PUT", "/api/likes"}, {"PUT", "/api/ratings"}, {"POST", "/api/plays"},
		{"POST", "/api/playlists"}, {"POST", "/api/stations"},
	}
	code := func(r [2]string) int {
		return do(t, h, r[0], r[1], `{}`, "Content-Type", "application/json").Code
	}

	if status() {
		t.Fatal("listen mode on by default")
	}
	for _, r := range curation {
		if strings.Contains(r[1], "scan") || strings.Contains(r[1], "identif") {
			continue // with the mode off these would really start the work
		}
		if c := code(r); c == http.StatusForbidden {
			t.Errorf("%s %s: 403 with listen mode off", r[0], r[1])
		}
	}

	setListen("true")
	if !status() {
		t.Fatal("status does not report listenMode")
	}
	for _, r := range curation {
		rec := do(t, h, r[0], r[1], `{}`, "Content-Type", "application/json")
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "Modo escucha activo") {
			t.Errorf("%s %s: %d %s", r[0], r[1], rec.Code, rec.Body)
		}
	}
	// Even without a JSON content type the answer is 403, not 415.
	if rec := do(t, h, "POST", "/api/scan", ""); rec.Code != http.StatusForbidden {
		t.Errorf("scan without content type: %d", rec.Code)
	}
	for _, r := range open {
		if c := code(r); c == http.StatusForbidden {
			t.Errorf("%s %s: blocked by listen mode", r[0], r[1])
		}
	}

	// Settings stay reachable, so listen mode can be turned off again; a PUT
	// without listenMode keeps it.
	var v configView
	decode(t, putConfig(t, s, `{"roots":["../musica"],"ignore":[],"normalize":true,"lan":{"enabled":false,"port":4747}}`), &v)
	if !v.ListenMode {
		t.Fatal("absent listenMode must keep the saved value")
	}
	setListen("false")
	if status() {
		t.Fatal("status still reports listenMode")
	}
}

// The settings page saves listen mode on its own: a body with only
// listenMode keeps the folders and everything else.
func TestListenModeAloneKeepsTheRest(t *testing.T) {
	s := settingsServer(t, false)
	putConfig(t, s, `{"roots":["../musica"],"ignore":["tmp"],"normalize":false,"lan":{"enabled":false,"port":4800}}`)
	before := saved(t, s)
	rec := putConfig(t, s, `{"listenMode":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("listenMode alone: %d %s", rec.Code, rec.Body)
	}
	after := saved(t, s)
	if !after.ListenMode {
		t.Fatal("listenMode not saved")
	}
	after.ListenMode = before.ListenMode
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("other settings changed:\n%+v\n%+v", before, after)
	}
}

// On first use there are no folders to keep: listen mode alone cannot end
// the setup.
func TestListenModeAloneDuringSetup(t *testing.T) {
	s := settingsServer(t, false)
	if rec := putConfig(t, s, `{"listenMode":true}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("listenMode alone during setup: %d %s", rec.Code, rec.Body)
	}
}
