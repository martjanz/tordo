package server

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"tordo/internal/catalog"
)

// periods maps ?period= to how many days it looks back (0: all time).
var periods = map[string]int{"week": 7, "month": 30, "year": 365, "all": 0}

// home answers Inicio: ?seed= draws the rotating rows (missing or bad:
// a random one), plus the saved stations as /api/stations lists them.
func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	seed, err := strconv.ParseUint(r.URL.Query().Get("seed"), 10, 64)
	if err != nil {
		seed = rand.Uint64()
	}
	c, p, err := s.browse()
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

// stats answers "Historial de escucha": ?period= (week, month, year, all; default
// month) and ?limit= of each ranking.
func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "month"
	}
	days, ok := periods[period]
	if !ok {
		writeError(w, http.StatusBadRequest, codeInvalidPeriod, "invalid period", nil)
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
	}{c.Stats(tally, p, intParam(r, "limit", 10, 1, 100)), period})
}
