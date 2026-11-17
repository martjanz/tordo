package server

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"tordo/internal/store"
)

// personalState is the user's data resolved to the catalog's ids, kept while
// neither the catalog (ids move with scans) nor the data changed.
type personalState struct {
	mu      sync.Mutex // guards the fields; never held while loading
	load    sync.Mutex // one load at a time
	changes int64
	version int64
	p       *store.Personal
}

// current is the last data loaded (nil before the first) and its version.
func (st *personalState) current() (*store.Personal, int64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.p, st.version
}

func (st *personalState) fresh(changes, version int64) *store.Personal {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.p != nil && st.changes == changes && st.version == version {
		return st.p
	}
	return nil
}

// personal returns the user's data, reloaded when the store's change counter
// or the personal version moved; the startup cache's while the store has not
// been read yet and the user changed nothing since.
func (s *Server) personal() (*store.Personal, error) {
	s.waitStartCache()
	if s.cat.provisional.Load() {
		if p, version := s.pers.current(); p != nil && version == s.Store.PersonalVersion() {
			return p, nil
		}
	}
	p, _, err := s.freshPersonal()
	return p, err
}

// freshPersonal returns the user's data as the store has it now, and the
// store's change counter it was resolved at.
func (s *Server) freshPersonal() (*store.Personal, int64, error) {
	changes, err := s.Store.Changes()
	if err != nil {
		return nil, 0, err
	}
	version := s.Store.PersonalVersion()
	st := &s.pers
	if p := st.fresh(changes, version); p != nil {
		return p, changes, nil
	}
	st.load.Lock()
	defer st.load.Unlock()
	if p := st.fresh(changes, version); p != nil {
		return p, changes, nil
	}
	p, err := s.Store.Personal()
	if err != nil {
		return nil, 0, err
	}
	st.mu.Lock()
	st.p, st.changes, st.version = &p, changes, version
	st.mu.Unlock()
	return &p, changes, nil
}

// readBody decodes a JSON body into v, answering 400 when it cannot.
func readBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return false
	}
	return true
}

// written answers the error (a bad rating is 400, anything unknown 500), 404
// with missing when a write found nothing, and 204 otherwise.
func written(w http.ResponseWriter, ok bool, err error, missing *httpError) {
	switch {
	case err != nil:
		fail(w, err)
	case !ok:
		fail(w, missing)
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
		writeError(w, http.StatusConflict, codeReadOnly, "browse-only mode: changes cannot be saved", nil)
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
			writeError(w, http.StatusBadRequest, codeMissingID, "id is required", nil)
			return
		}
	case "artist", "collection":
		if b.Key == "" {
			writeError(w, http.StatusBadRequest, codeMissingKey, "key is required", nil)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, codeInvalidKind, "invalid kind", nil)
		return
	}
	switch b.Kind {
	case "track":
		ok, err := s.Store.LikeTrack(b.ID, b.Liked)
		written(w, ok, err, errTrackNotFound)
	case "album":
		ok, err := s.Store.LikeAlbum(b.ID, b.Liked)
		written(w, ok, err, errAlbumNotFound)
	case "artist":
		written(w, true, s.Store.LikeArtist(b.Key, b.Liked), nil)
	default:
		ok, err := s.Store.LikeCollection(b.Key, b.Liked)
		written(w, ok, err, errFolderNotFound)
	}
}

func (s *Server) setRating(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly {
		writeError(w, http.StatusConflict, codeReadOnly, "browse-only mode: changes cannot be saved", nil)
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
		writeError(w, http.StatusBadRequest, codeInvalidKind, "invalid kind", nil)
		return
	}
	if b.ID == 0 {
		writeError(w, http.StatusBadRequest, codeMissingID, "id is required", nil)
		return
	}
	if b.Kind == "track" {
		ok, err := s.Store.RateTrack(b.ID, b.Stars)
		written(w, ok, err, errTrackNotFound)
		return
	}
	ok, err := s.Store.RateAlbum(b.ID, b.Stars)
	written(w, ok, err, errAlbumNotFound)
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
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	if s.ReadOnly {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ok, err := s.Store.RecordPlay(b.ID, time.Now(), b.ListenedMs, b.Device)
	written(w, ok, err, errTrackNotFound)
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
