package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type stationRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	ArtistGap int    `json:"artistGap"`
	AvoidDays int    `json:"avoidDays"`
	Source    struct {
		Folder string `json:"folder"`
		View   string `json:"view"`
	} `json:"source"`
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

func TestStationsEndpoints(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	call := func(method, target, body string) *httptest.ResponseRecorder {
		return do(t, h, method, target, body, "Content-Type", jsonCT)
	}
	rec := call("POST", "/api/stations", `{"name":"Locura","source":{"folder":"../musica/Virus - Locura"},"mode":"albumes","artistGap":3}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var st stationRow
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.ID == 0 || st.Name != "Locura" || st.Mode != "albumes" || st.ArtistGap != 3 || st.Source.Folder != "../musica/Virus - Locura" || st.CreatedAt == 0 {
		t.Fatalf("created %+v", st)
	}
	var list []stationRow
	decode(t, call("GET", "/api/stations", ""), &list)
	if len(list) != 1 || list[0].ID != st.ID || list[0].Mode != "albumes" {
		t.Fatalf("list %+v", list)
	}
	if rec := call("PUT", "/api/stations/"+itoa(st.ID), `{"name":"Virus","source":{},"mode":"pistas"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	list = nil
	decode(t, call("GET", "/api/stations", ""), &list)
	if len(list) != 1 || list[0].Name != "Virus" || list[0].Mode != "pistas" || list[0].Source.Folder != "" {
		t.Fatalf("after update %+v", list)
	}
	if rec := call("POST", "/api/stations", `{"name":" "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name: %d", rec.Code)
	}
	if rec := call("PUT", "/api/stations/"+itoa(st.ID), `{"name":" "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank rename: %d", rec.Code)
	}
	if rec := call("PUT", "/api/stations/9999", `{"name":"x"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown update: %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/stations", `{"name":"x"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("not JSON: %d", rec.Code)
	}
	s.ReadOnly = true
	hro := s.Handler()
	for _, c := range [][3]string{{"POST", "/api/stations", `{"name":"x"}`}, {"PUT", "/api/stations/" + itoa(st.ID), `{"name":"x"}`}, {"DELETE", "/api/stations/" + itoa(st.ID), ""}} {
		if rec := do(t, hro, c[0], c[1], c[2], "Content-Type", jsonCT); rec.Code != http.StatusConflict {
			t.Fatalf("%s read-only: %d", c[0], rec.Code)
		}
	}
	if rec := do(t, hro, "GET", "/api/stations", ""); rec.Code != http.StatusOK {
		t.Fatalf("list read-only: %d", rec.Code)
	}
	s.ReadOnly = false
	if rec := call("DELETE", "/api/stations/"+itoa(st.ID), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := call("DELETE", "/api/stations/"+itoa(st.ID), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete again: %d", rec.Code)
	}
}

func TestStationNext(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	next := func(def, recent string) *httptest.ResponseRecorder {
		q := url.Values{}
		if def != "" {
			q.Set("def", def)
		}
		if recent != "" {
			q.Set("recientes", recent)
		}
		return do(t, h, "GET", "/api/station/next?"+q.Encode(), "")
	}
	var out struct {
		Items []struct {
			ID   int64  `json:"id"`
			MIME string `json:"mime"`
		} `json:"items"`
		Station struct {
			Mode string `json:"mode"`
		} `json:"station"`
	}
	// The only playable track fills the batch: the rules relax down to repeating it.
	onlyMP3 := func(what string) {
		t.Helper()
		if len(out.Items) == 0 {
			t.Fatalf("%s: no items", what)
		}
		for _, it := range out.Items {
			if it.ID != ids[0] || it.MIME == "" {
				t.Fatalf("%s: %+v", what, out)
			}
		}
	}
	decode(t, next(`{"source":{}}`, ""), &out)
	onlyMP3("next")
	if out.Station.Mode != "pistas" {
		t.Fatalf("station %+v", out.Station)
	}
	out.Items = nil
	decode(t, next(`{"source":{}}`, itoa(ids[0])+",x,-3"), &out)
	onlyMP3("with recent")
	for _, def := range []string{"", "{", `[1]`} {
		if rec := next(def, ""); rec.Code != http.StatusBadRequest {
			t.Fatalf("def %q: %d", def, rec.Code)
		}
	}
	if rec := next(`{"source":{"playlist":9999}}`, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown playlist: %d", rec.Code)
	}
	pl, err := s.Store.CreatePlaylist("Todo")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Store.AddToPlaylist(pl, ids); err != nil {
		t.Fatal(err)
	}
	out.Items = nil
	decode(t, next(`{"source":{"playlist":`+itoa(pl)+`}}`, ""), &out)
	onlyMP3("playlist")
}
