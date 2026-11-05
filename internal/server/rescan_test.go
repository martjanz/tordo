package server

import (
	"net/http"
	"testing"
	"time"
)

func TestPutConfigRescanMinutes(t *testing.T) {
	s := settingsServer(t, false)
	if rec := putConfig(t, s, `{"roots":["../musica"],"lan":{"port":4747}}`); rec.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body)
	}
	var v struct {
		RescanMinutes int `json:"rescanMinutes"`
	}
	decode(t, do(t, s.Handler(), "GET", "/api/config", ""), &v)
	if v.RescanMinutes != 60 {
		t.Fatalf("default = %d, want 60", v.RescanMinutes)
	}
	decode(t, putConfig(t, s, `{"rescanMinutes":180}`), &v)
	if got := saved(t, s).RescanEvery(); v.RescanMinutes != 180 || got != 3*time.Hour {
		t.Fatalf("view %d, saved %v", v.RescanMinutes, got)
	}
	decode(t, putConfig(t, s, `{"listenMode":false}`), &v)
	if v.RescanMinutes != 180 {
		t.Fatalf("an absent value changed it to %d", v.RescanMinutes)
	}
	decode(t, putConfig(t, s, `{"rescanMinutes":0}`), &v)
	if v.RescanMinutes != 0 || saved(t, s).RescanEvery() != 0 {
		t.Fatalf("off: view %d, saved %v", v.RescanMinutes, saved(t, s).RescanEvery())
	}
	for _, bad := range []string{"-1", "10081"} {
		if rec := putConfig(t, s, `{"rescanMinutes":`+bad+`}`); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", bad, rec.Code, rec.Body)
		}
	}
}
