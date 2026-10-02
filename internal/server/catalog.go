package server

import (
	"net/http"
	"strconv"
	"sync"

	"tordo/internal/catalog"
	"tordo/internal/search"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// catalogState is the catalog built from the store and its search index,
// kept between requests while the store's change counter stays: Explorar
// and typing in the search ask often, and building takes a whole snapshot.
type catalogState struct {
	mu      sync.Mutex
	cat     *catalog.Catalog
	index   *search.Index
	indexed int64 // the change counter the index was built at
}

// playable tells whether the server can deliver a track's audio.
func (s *Server) playable(t store.TrackView) bool {
	mode, _ := s.Streamer.Plan(stream.Format{Container: t.Format, Codec: t.Codec, Lossless: t.Lossless})
	return mode != stream.Unplayable
}

// catalog returns the catalog, rebuilt when the store changed.
func (s *Server) catalog() (*catalog.Catalog, error) {
	changes, err := s.Store.Changes()
	if err != nil {
		return nil, err
	}
	st := &s.cat
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.cat != nil && st.cat.Changes == changes {
		return st.cat, nil
	}
	snap, err := s.Store.Snapshot()
	if err != nil {
		return nil, err
	}
	st.cat = catalog.Build(snap, catalog.Options{Playable: s.playable})
	return st.cat, nil
}

// searchIndex returns the catalog and its search index, both current.
func (s *Server) searchIndex() (*catalog.Catalog, *search.Index, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, nil, err
	}
	st := &s.cat
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.index == nil {
		if st.index, err = search.New(); err != nil {
			return nil, nil, err
		}
	}
	if err := st.index.Refresh(c.Changes, c.SearchDocs); err != nil {
		return nil, nil, err
	}
	return c, st.index, nil
}

// intParam reads a query parameter between lo and hi, def when missing or
// out of range.
func intParam(r *http.Request, name string, def, lo, hi int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || n < lo || n > hi {
		return def
	}
	return n
}

// explore answers Explorar: ?ver= (albumes, artistas, pistas), the facets,
// ?orden= and ?dir=, a page from ?desde= of ?limite= items.
func (s *Server) explore(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	q := catalog.ParseQuery(r.URL.Query())
	writeJSON(w, c.Explore(q, intParam(r, "desde", 0, 0, 1<<30), intParam(r, "limite", 120, 1, 500)))
}

func (s *Server) artist(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	p, ok := c.ArtistPage(r.PathValue("key"))
	if !ok {
		http.Error(w, "artista no encontrado", http.StatusNotFound)
		return
	}
	writeJSON(w, p)
}

// find is the instant search: ?q=, at most ?limite= of each kind.
func (s *Server) find(w http.ResponseWriter, r *http.Request) {
	c, idx, err := s.searchIndex()
	if err != nil {
		fail(w, err)
		return
	}
	q := r.URL.Query().Get("q")
	keys, err := idx.Query(q)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, struct {
		Q string `json:"q"`
		catalog.Found
	}{q, c.Found(keys, intParam(r, "limite", 6, 1, 200))})
}

// albumTrack is a track of the album page: whether the server can deliver it.
type albumTrack struct {
	store.TrackView
	ArtistKey  string `json:"artistKey"`  // the track artist's page, when there is one
	Playable   bool   `json:"playable"`   // false: it needs ffmpeg, and there is none
	Transcoded bool   `json:"transcoded"` // converted by ffmpeg: no seeking
}

// albumPage is the album page: the album, its artist's page, its tracks and
// the MusicBrainz releases offered for it.
type albumPage struct {
	store.AlbumDetail
	ArtistKey  string            `json:"artistKey"`
	Tracks     []albumTrack      `json:"tracks"`
	Candidates []store.Candidate `json:"candidates"`
}

// writeAlbum answers an album's page.
func (s *Server) writeAlbum(w http.ResponseWriter, id int64) {
	a, ok, err := s.Store.Album(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "álbum no encontrado", http.StatusNotFound)
		return
	}
	cands, err := s.Store.AlbumCandidates(id)
	if err != nil {
		fail(w, err)
		return
	}
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	p := albumPage{AlbumDetail: a, Tracks: make([]albumTrack, 0, len(a.Tracks)), Candidates: cands}
	if al, ok := c.Album(id); ok {
		p.ArtistKey = al.ArtistKey
	}
	for _, t := range a.Tracks {
		mode, _ := s.Streamer.Plan(stream.Format{Container: t.Format, Codec: t.Codec, Lossless: t.Lossless})
		at := albumTrack{TrackView: t, Playable: mode != stream.Unplayable, Transcoded: mode == stream.Transcode}
		if ct, ok := c.Track(t.ID); ok {
			at.ArtistKey = ct.ArtistKey
		}
		p.Tracks = append(p.Tracks, at)
	}
	writeJSON(w, p)
}
