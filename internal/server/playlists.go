package server

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"tordo/internal/store"
)

// what is a play query as a JSON body: {"album": 3}, {"folder": "…"},
// {"artist": "…"}, {"tracks": [1, 2]}, {"playlist": 4}.
type what struct {
	Album    *int64  `json:"album"`
	Folder   *string `json:"folder"`
	Artist   *string `json:"artist"`
	Tracks   []int64 `json:"tracks"`
	Playlist *int64  `json:"playlist"`
}

// values turns the body into the query resolve reads.
func (w what) values() url.Values {
	q := url.Values{}
	if w.Album != nil {
		q.Set("album", strconv.FormatInt(*w.Album, 10))
	}
	if w.Folder != nil {
		q.Set("folder", *w.Folder)
	}
	if w.Artist != nil {
		q.Set("artist", *w.Artist)
	}
	for _, id := range w.Tracks {
		q.Add("track", strconv.FormatInt(id, 10))
	}
	if w.Playlist != nil {
		q.Set("playlist", strconv.FormatInt(*w.Playlist, 10))
	}
	return q
}

// trackIDs resolves a body to the ids of the tracks it names.
func (s *Server) trackIDs(w what) ([]int64, error) {
	items, err := s.resolve(w.values())
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids, nil
}

// writable answers 409 in browse-only mode and tells whether to go on.
func (s *Server) writable(w http.ResponseWriter) bool {
	if s.ReadOnly {
		http.Error(w, readOnlyMsg, http.StatusConflict)
		return false
	}
	return true
}

func playlistID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

const playlistGone = "lista no encontrada"

func (s *Server) playlists(w http.ResponseWriter, r *http.Request) {
	ps, err := s.Store.Playlists()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, ps)
}

// playlistErr answers the errors the store reports for bad playlist input.
func playlistErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrPlaylistName), errors.Is(err, store.ErrPlaylistFull), errors.Is(err, store.ErrPlaylistOrder):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		answer(w, err)
	}
}

func (s *Server) createPlaylist(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	var b struct {
		Name string `json:"name"`
		Add  *what  `json:"add"`
	}
	if !readBody(w, r, &b) {
		return
	}
	// Resolve before creating, so a bad "add" creates nothing.
	var ids []int64
	if b.Add != nil {
		var err error
		if ids, err = s.trackIDs(*b.Add); err != nil {
			playlistErr(w, err)
			return
		}
	}
	id, err := s.Store.CreatePlaylist(b.Name)
	if err != nil {
		playlistErr(w, err)
		return
	}
	if len(ids) > 0 {
		if _, _, err := s.Store.AddToPlaylist(id, ids); err != nil {
			s.Store.DeletePlaylist(id) // do not leave a half-made list behind
			playlistErr(w, err)
			return
		}
	}
	p, ok, err := s.Store.Playlist(id)
	if err != nil || !ok {
		fail(w, errors.Join(err, errors.New("playlist vanished after creation")))
		return
	}
	writeJSONStatus(w, http.StatusCreated, p.PlaylistSummary)
}

func (s *Server) playlist(w http.ResponseWriter, r *http.Request) {
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	p, found, err := s.Store.Playlist(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !found {
		http.Error(w, playlistGone, http.StatusNotFound)
		return
	}
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	type entry struct {
		store.PlaylistEntry
		Playable  bool   `json:"playable"`
		ArtistKey string `json:"artistKey"`
	}
	entries := make([]entry, len(p.Entries))
	for i, e := range p.Entries {
		entries[i].PlaylistEntry = e
		if t, ok := c.Track(e.TrackID); ok && e.TrackID != 0 {
			entries[i].Playable = s.playable(t.TrackView)
			entries[i].ArtistKey = t.ArtistKey
		}
	}
	writeJSON(w, struct {
		store.PlaylistSummary
		Entries []entry `json:"entries"`
	}{p.PlaylistSummary, entries})
}

func (s *Server) renamePlaylist(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	var b struct {
		Name string `json:"name"`
	}
	if !readBody(w, r, &b) {
		return
	}
	found, err := s.Store.RenamePlaylist(id, b.Name)
	if err != nil {
		playlistErr(w, err)
		return
	}
	written(w, found, nil, playlistGone)
}

func (s *Server) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	found, err := s.Store.DeletePlaylist(id)
	written(w, found, err, playlistGone)
}

func (s *Server) addToPlaylist(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	var b what
	if !readBody(w, r, &b) {
		return
	}
	ids, err := s.trackIDs(b)
	if err != nil {
		playlistErr(w, err)
		return
	}
	added, exists, err := s.Store.AddToPlaylist(id, ids)
	if err != nil {
		playlistErr(w, err)
		return
	}
	if !exists {
		http.Error(w, playlistGone, http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]int{"added": added})
}

func (s *Server) removeFromPlaylist(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	item, err := strconv.ParseInt(r.PathValue("item"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	found, err := s.Store.RemoveFromPlaylist(id, item)
	written(w, found, err, "pista no encontrada en la lista")
}

func (s *Server) reorderPlaylist(w http.ResponseWriter, r *http.Request) {
	if !s.writable(w) {
		return
	}
	id, ok := playlistID(w, r)
	if !ok {
		return
	}
	var b struct {
		Items []int64 `json:"items"`
	}
	if !readBody(w, r, &b) {
		return
	}
	found, err := s.Store.ReorderPlaylist(id, b.Items)
	if err != nil {
		playlistErr(w, err)
		return
	}
	written(w, found, nil, playlistGone)
}
