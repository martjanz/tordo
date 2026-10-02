package server

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"tordo/internal/catalog"
)

// periods maps ?periodo= to how many days it looks back (0: all time).
var periods = map[string]int{"semana": 7, "mes": 30, "anio": 365, "siempre": 0}

// home answers Inicio: ?semilla= draws the rotating rows (missing or bad:
// a random one), plus the saved stations as /api/stations lists them.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	seed, err := strconv.ParseUint(r.URL.Query().Get("semilla"), 10, 64)
	if err != nil {
		seed = rand.Uint64()
	}
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	p, err := s.personal()
	if err != nil {
		fail(w, err)
		return
	}
	stations, err := s.savedStations()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, struct {
		catalog.Home
		Stations []savedStation `json:"stations"`
	}{c.Home(p, seed), stations})
}

// stats answers "Tu escucha": ?periodo= (semana, mes, anio, siempre; default
// mes) and ?limite= of each ranking.
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("periodo")
	if period == "" {
		period = "mes"
	}
	days, ok := periods[period]
	if !ok {
		http.Error(w, "período inválido", http.StatusBadRequest)
		return
	}
	var since time.Time
	if days > 0 {
		since = time.Now().AddDate(0, 0, -days)
	}
	tally, err := s.Store.PlayTally(since)
	if err != nil {
		fail(w, err)
		return
	}
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	p, err := s.personal()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, struct {
		catalog.Stats
		Period string `json:"period"`
	}{c.Stats(tally, p, intParam(r, "limite", 10, 1, 100)), period})
}
