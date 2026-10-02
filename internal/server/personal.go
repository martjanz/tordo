package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"tordo/internal/store"
)

const readOnlyMsg = "modo consulta: no se pueden guardar cambios"

// personalState is the user's data resolved to the catalog's ids, kept while
// neither the catalog (ids move with scans) nor the data changed.
type personalState struct {
	mu      sync.Mutex
	changes int64
	version int64
	p       *store.Personal
}

// personal returns the user's data, reloaded when the store's change counter
// or the personal version moved.
func (s *Server) personal() (*store.Personal, error) {
	changes, err := s.Store.Changes()
	if err != nil {
		return nil, err
	}
	version := s.Store.PersonalVersion()
	st := &s.pers
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.p != nil && st.changes == changes && st.version == version {
		return st.p, nil
	}
	p, err := s.Store.Personal()
	if err != nil {
		return nil, err
	}
	st.p, st.changes, st.version = &p, changes, version
	return st.p, nil
}

// readBody decodes a JSON body into v, answering 400 when it cannot.
func readBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(v); err != nil {
		http.Error(w, "cuerpo inválido", http.StatusBadRequest)
		return false
	}
	return true
}

// written answers 400 for a bad rating, 500 on error, 404 with msg when a
// write found nothing, and 204 otherwise.
func written(w http.ResponseWriter, ok bool, err error, msg string) {
	switch {
	case errors.Is(err, store.ErrBadRating):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case err != nil:
		fail(w, err)
	case !ok:
		http.Error(w, msg, http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func sortedKeys(m map[string]int64) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// marks answers the user's likes and ratings.
func (s *Server) marks(w http.ResponseWriter, r *http.Request) {
	p, err := s.personal()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, struct {
		Tracks      map[int64]store.Mark `json:"tracks"`
		Albums      map[int64]store.Mark `json:"albums"`
		Artists     []string             `json:"artists"`
		Collections []string             `json:"collections"`
	}{p.Tracks, p.Albums, sortedKeys(p.Artists), sortedKeys(p.Collections)})
}

func (s *Server) setLike(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly {
		http.Error(w, readOnlyMsg, http.StatusConflict)
		return
	}
	var b struct {
		Kind  string `json:"kind"`
		ID    int64  `json:"id"`
		Key   string `json:"key"`
		Liked bool   `json:"liked"`
	}
	if !readBody(w, r, &b) {
		return
	}
	switch b.Kind {
	case "track", "album":
		if b.ID == 0 {
			http.Error(w, "falta el id", http.StatusBadRequest)
			return
		}
	case "artist", "collection":
		if b.Key == "" {
			http.Error(w, "falta la clave", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "tipo inválido", http.StatusBadRequest)
		return
	}
	switch b.Kind {
	case "track":
		ok, err := s.Store.LikeTrack(b.ID, b.Liked)
		written(w, ok, err, "pista no encontrada")
	case "album":
		ok, err := s.Store.LikeAlbum(b.ID, b.Liked)
		written(w, ok, err, "álbum no encontrado")
	case "artist":
		written(w, true, s.Store.LikeArtist(b.Key, b.Liked), "")
	default:
		ok, err := s.Store.LikeCollection(b.Key, b.Liked)
		written(w, ok, err, "carpeta no encontrada")
	}
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly {
		http.Error(w, readOnlyMsg, http.StatusConflict)
		return
	}
	var b struct {
		Kind  string `json:"kind"`
		ID    int64  `json:"id"`
		Stars int    `json:"stars"`
	}
	if !readBody(w, r, &b) {
		return
	}
	if b.Kind != "track" && b.Kind != "album" {
		http.Error(w, "tipo inválido", http.StatusBadRequest)
		return
	}
	if b.ID == 0 {
		http.Error(w, "falta el id", http.StatusBadRequest)
		return
	}
	if b.Kind == "track" {
		ok, err := s.Store.RateTrack(b.ID, b.Stars)
		written(w, ok, err, "pista no encontrada")
		return
	}
	ok, err := s.Store.RateAlbum(b.ID, b.Stars)
	written(w, ok, err, "álbum no encontrado")
}

// recordPlay logs a play; in browse-only mode it answers 204 and records
// nothing, so the player does not have to know the mode.
func (s *Server) recordPlay(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ID         int64  `json:"id"`
		ListenedMs int64  `json:"listenedMs"`
		Device     string `json:"device"`
	}
	if !readBody(w, r, &b) {
		return
	}
	if b.ListenedMs < 0 {
		http.Error(w, "cuerpo inválido", http.StatusBadRequest)
		return
	}
	if s.ReadOnly {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ok, err := s.Store.RecordPlay(b.ID, time.Now(), b.ListenedMs, b.Device)
	written(w, ok, err, "pista no encontrada")
}

// liked answers the Listas page's likes.
func (s *Server) liked(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, c.Liked(p))
}
