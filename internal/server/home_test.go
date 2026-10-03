package server

import (
	"net/http"
	"testing"
	"time"

	"tordo/internal/catalog"
)

type homeBody struct {
	Recent   []catalog.Album `json:"recent"`
	Continue []catalog.Album `json:"continue"`
	Liked    []catalog.Album `json:"liked"`
	Stations []stationRow    `json:"stations"`
}

func TestHomeEndpoint(t *testing.T) {
	st := newStore(t)
	h := (&Server{Store: st}).Handler()
	tr, al := ids(t, st)

	a := do(t, h, "GET", "/api/home?semilla=7", "")
	b := do(t, h, "GET", "/api/home?semilla=7", "")
	if a.Code != http.StatusOK || a.Body.String() != b.Body.String() {
		t.Fatalf("same seed: %d\n%s\n%s", a.Code, a.Body, b.Body)
	}
	var home homeBody
	decode(t, a, &home)
	if len(home.Recent) != 1 || home.Recent[0].ID != al || home.Continue == nil || len(home.Continue) != 0 ||
		home.Liked == nil || len(home.Liked) != 0 || home.Stations == nil || len(home.Stations) != 0 {
		t.Fatalf("empty home = %+v\n%s", home, a.Body)
	}
	if rec := do(t, h, "GET", "/api/home?semilla=nope", ""); rec.Code != http.StatusOK {
		t.Fatalf("bad seed: %d", rec.Code)
	}

	if _, err := st.RecordPlay(tr, time.Now(), 500, "test"); err != nil {
		t.Fatal(err)
	}
	if rec := put(t, h, "POST", "/api/stations", `{"name":"Todo","source":{},"mode":"pistas"}`); rec.Code != http.StatusCreated {
		t.Fatalf("station: %d %s", rec.Code, rec.Body)
	}
	home = homeBody{}
	decode(t, do(t, h, "GET", "/api/home?semilla=7", ""), &home)
	if len(home.Continue) != 1 || home.Continue[0].ID != al {
		t.Fatalf("continue = %+v", home.Continue)
	}
	if len(home.Stations) != 1 || home.Stations[0].Name != "Todo" || home.Stations[0].Mode != "pistas" {
		t.Fatalf("stations = %+v", home.Stations)
	}
}

func TestStatsEndpoint(t *testing.T) {
	st := newStore(t)
	h := (&Server{Store: st}).Handler()
	tr, _ := ids(t, st)
	if _, err := st.RecordPlay(tr, time.Now(), 500, "test"); err != nil {
		t.Fatal(err)
	}
	var s struct {
		Period string `json:"period"`
		Plays  int    `json:"plays"`
		Tracks []struct {
			ID    int64 `json:"id"`
			Plays int   `json:"plays"`
		} `json:"tracks"`
	}
	decode(t, do(t, h, "GET", "/api/stats?periodo=semana", ""), &s)
	if s.Period != "semana" || s.Plays != 1 || len(s.Tracks) != 1 || s.Tracks[0].ID != tr || s.Tracks[0].Plays != 1 {
		t.Fatalf("semana = %+v", s)
	}
	for _, p := range []string{"mes", "anio", "siempre"} {
		if rec := do(t, h, "GET", "/api/stats?periodo="+p+"&limite=3", ""); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", p, rec.Code)
		}
	}
	if rec := do(t, h, "GET", "/api/stats?periodo=x", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad period: %d", rec.Code)
	}
	s.Period = ""
	decode(t, do(t, h, "GET", "/api/stats", ""), &s)
	if s.Period != "mes" {
		t.Fatalf("default period = %q", s.Period)
	}
}

func TestArtistTop(t *testing.T) {
	st := newStore(t)
	h := (&Server{Store: st}).Handler()
	tr, _ := ids(t, st)
	type top struct {
		Top []struct {
			ID    int64 `json:"id"`
			Plays int   `json:"plays"`
		} `json:"top"`
	}
	rec := do(t, h, "GET", "/api/artists/virus", "")
	var p top
	decode(t, rec, &p)
	if rec.Code != http.StatusOK || p.Top == nil || len(p.Top) != 0 {
		t.Fatalf("no plays: %d %s", rec.Code, rec.Body)
	}
	if _, err := st.RecordPlay(tr, time.Now(), 500, "test"); err != nil {
		t.Fatal(err)
	}
	p = top{}
	decode(t, do(t, h, "GET", "/api/artists/virus", ""), &p)
	if len(p.Top) != 1 || p.Top[0].ID != tr || p.Top[0].Plays != 1 {
		t.Fatalf("top = %+v", p.Top)
	}
}
