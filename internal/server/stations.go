package server

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"tordo/internal/catalog"
	"tordo/internal/store"
)

const stationGone = "estación no encontrada"

// savedStation is a saved station as listed: its stored name and what it plays.
type savedStation struct {
	ID int64 `json:"id"`
	catalog.Station
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// listed shows a stored station: its def decoded back (one that fails to
// decode plays the whole library), under the stored name.
func listed(ss store.SavedStation) savedStation {
	var st catalog.Station
	if json.Unmarshal(ss.Def, &st) != nil {
		st = catalog.Station{}
	}
	st = st.Clean()
	st.Name = ss.Name
	return savedStation{ID: ss.ID, Station: st, CreatedAt: ss.CreatedAt, UpdatedAt: ss.UpdatedAt}
}

// stationDef splits a station into the name and the def to store: the
// cleaned station without its name.
func stationDef(st catalog.Station) (string, []byte, error) {
	st = st.Clean()
	name := st.Name
	st.Name = ""
	def, err := json.Marshal(st)
	return name, def, err
}

func stationErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrStationName) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	answer(w, err)
}

// savedStations lists the saved stations, most recently updated first.
func (s *Server) savedStations() ([]savedStation, error) {
	all, err := s.Store.Stations()
	if err != nil {
		return nil, err
	}
	out := make([]savedStation, len(all))
	for i, ss := range all {
		out[i] = listed(ss)
	}
	return out, nil
}

func (s *Server) stations(w http.ResponseWriter, r *http.Request) {
	out, err := s.savedStations()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, out)
}

func (s *Server) createStation(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var b catalog.Station
	if !readBody(w, r, &b) {
		return
	}
	name, def, err := stationDef(b)
	if err != nil {
		fail(w, err)
		return
	}
	id, err := s.Store.CreateStation(name, def)
	if err != nil {
		stationErr(w, err)
		return
	}
	ss, ok, err := s.Store.Station(id)
	if err != nil || !ok {
		fail(w, errors.Join(err, errors.New("la estación desapareció al crearla")))
		return
	}
	writeJSONStatus(w, http.StatusCreated, listed(ss))
}

func (s *Server) updateStation(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	var b catalog.Station
	if !readBody(w, r, &b) {
		return
	}
	name, def, err := stationDef(b)
	if err != nil {
		fail(w, err)
		return
	}
	found, err := s.Store.UpdateStation(id, name, def)
	if err != nil {
		stationErr(w, err)
		return
	}
	written(w, found, nil, stationGone)
}

func (s *Server) deleteStation(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	found, err := s.Store.DeleteStation(id)
	written(w, found, err, stationGone)
}

// stationNext answers GET /api/station/next?def=<station JSON>&recientes=1,2:
// the next batch of a station, shaped like /api/play, plus the station as the
// server applied it. The station travels whole, so the server keeps no state
// per listener.
func (s *Server) stationNext(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var st catalog.Station
	if err := json.Unmarshal([]byte(q.Get("def")), &st); err != nil {
		http.Error(w, "estación inválida", http.StatusBadRequest)
		return
	}
	st = st.Clean()
	in := catalog.StationInput{Rand: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
	if st.Source.Playlist > 0 {
		items, ok, err := s.Store.PlayPlaylist(st.Source.Playlist)
		if err != nil {
			fail(w, err)
			return
		}
		if !ok {
			http.Error(w, playlistGone, http.StatusNotFound)
			return
		}
		for _, it := range items {
			in.Playlist = append(in.Playlist, it.ID)
		}
	}
	for _, v := range strings.Split(q.Get("recientes"), ",") {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			in.Recent = append(in.Recent, id)
		}
	}
	if len(in.Recent) > catalog.MaxRecent {
		in.Recent = in.Recent[len(in.Recent)-catalog.MaxRecent:]
	}
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	if in.Personal, err = s.personal(); err != nil {
		fail(w, err)
		return
	}
	items, err := s.Store.PlayTracks(c.Next(st, in, catalog.StationBatch))
	if err != nil {
		fail(w, err)
		return
	}
	out, err := s.deliver(items)
	if err != nil {
		fail(w, err)
		return
	}
	out["station"] = st
	writeJSON(w, out)
}
