package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tordo/internal/store"
)

func TestPlaylistEndpoints(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	album := onlyAlbum(t, s)
	call := func(method, target, body string) *httptest.ResponseRecorder {
		return do(t, h, method, target, body, "Content-Type", jsonCT)
	}
	rec := call("POST", "/api/playlists", `{"name":"Viaje","add":{"album":`+itoa(album)+`}}`)
	var sum store.PlaylistSummary
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Name != "Viaje" || sum.Tracks != 2 {
		t.Fatalf("summary %+v", sum)
	}
	id := itoa(sum.ID)
	var list []store.PlaylistSummary
	decode(t, call("GET", "/api/playlists", ""), &list)
	if len(list) != 1 || list[0].ID != sum.ID {
		t.Fatalf("list %+v", list)
	}
	type entry struct {
		store.PlaylistEntry
		Playable  bool   `json:"playable"`
		ArtistKey string `json:"artistKey"`
	}
	var pl struct {
		store.PlaylistSummary
		Entries []entry `json:"entries"`
	}
	decode(t, call("GET", "/api/playlists/"+id, ""), &pl)
	if len(pl.Entries) != 2 || !pl.Entries[0].Playable || pl.Entries[1].Playable || pl.Entries[0].ArtistKey == "" {
		t.Fatalf("playlist %+v", pl)
	}
	var added struct{ Added int }
	decode(t, call("POST", "/api/playlists/"+id+"/items", `{"tracks":[`+itoa(ids[0])+`]}`), &added)
	if added.Added != 1 {
		t.Fatalf("added %+v", added)
	}
	decode(t, call("GET", "/api/playlists/"+id, ""), &pl)
	if len(pl.Entries) != 3 {
		t.Fatalf("entries %d", len(pl.Entries))
	}
	rev := []string{itoa(pl.Entries[2].ItemID), itoa(pl.Entries[1].ItemID), itoa(pl.Entries[0].ItemID)}
	if rec := call("PUT", "/api/playlists/"+id+"/order", `{"items":[`+strings.Join(rev, ",")+`]}`); rec.Code != http.StatusNoContent {
		t.Fatalf("order: %d %s", rec.Code, rec.Body)
	}
	decode(t, call("GET", "/api/playlists/"+id, ""), &pl)
	if itoa(pl.Entries[0].ItemID) != rev[0] {
		t.Fatalf("order not applied: %+v", pl.Entries)
	}
	if rec := call("DELETE", "/api/playlists/"+id+"/items/"+rev[0], ""); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: %d", rec.Code)
	}
	if rec := call("PUT", "/api/playlists/"+id, `{"name":"Ruta"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("rename: %d", rec.Code)
	}
	if rec := call("PUT", "/api/playlists/"+id, `{"name":" "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name: %d", rec.Code)
	}
	decode(t, call("GET", "/api/playlists/"+id, ""), &pl)
	if pl.Name != "Ruta" || len(pl.Entries) != 2 {
		t.Fatalf("after edits %+v", pl)
	}
	if rec := call("DELETE", "/api/playlists/"+id, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := call("DELETE", "/api/playlists/"+id, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete again: %d", rec.Code)
	}
}

func TestPlaylistErrors(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	call := func(method, target, body string) *httptest.ResponseRecorder {
		return do(t, h, method, target, body, "Content-Type", jsonCT)
	}
	for _, c := range [][3]string{
		{"GET", "/api/playlists/999", ""},
		{"PUT", "/api/playlists/999", `{"name":"x"}`},
		{"DELETE", "/api/playlists/999", ""},
		{"POST", "/api/playlists/999/items", `{"tracks":[` + itoa(ids[0]) + `]}`},
		{"DELETE", "/api/playlists/999/items/1", ""},
		{"PUT", "/api/playlists/999/order", `{"items":[]}`},
	} {
		if rec := call(c[0], c[1], c[2]); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: %d", c[0], c[1], rec.Code)
		}
	}
	if rec := call("POST", "/api/playlists", `{"name":"x","add":{"album":999}}`); rec.Code != http.StatusNotFound {
		t.Errorf("bad add: %d", rec.Code)
	}
	var list []store.PlaylistSummary
	decode(t, call("GET", "/api/playlists", ""), &list)
	if len(list) != 0 {
		t.Fatalf("a bad add created %+v", list)
	}
	if rec := call("POST", "/api/playlists", `{`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json: %d", rec.Code)
	}
	id, _ := s.Store.CreatePlaylist("Una")
	s.ReadOnly = true
	for _, c := range [][3]string{
		{"POST", "/api/playlists", `{"name":"x"}`},
		{"PUT", "/api/playlists/" + itoa(id), `{"name":"y"}`},
		{"DELETE", "/api/playlists/" + itoa(id), ""},
		{"POST", "/api/playlists/" + itoa(id) + "/items", `{"tracks":[1]}`},
		{"DELETE", "/api/playlists/" + itoa(id) + "/items/1", ""},
		{"PUT", "/api/playlists/" + itoa(id) + "/order", `{"items":[]}`},
	} {
		if rec := call(c[0], c[1], c[2]); rec.Code != http.StatusConflict {
			t.Errorf("read-only %s %s: %d", c[0], c[1], rec.Code)
		}
	}
	if rec := call("GET", "/api/playlists/"+itoa(id), ""); rec.Code != http.StatusOK {
		t.Errorf("read-only GET: %d", rec.Code)
	}
}
