package server

import (
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"tordo/internal/catalog"
	"tordo/internal/identify"
	"tordo/internal/search"
	"tordo/internal/startcache"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// catalogState is the catalog built from the store and its search index,
// kept between requests while the store's change counter stays: Explorar
// and typing in the search ask often, and building takes a whole snapshot.
type catalogState struct {
	mu         sync.Mutex // guards cat; never held while building
	build      sync.Mutex // one build at a time
	cat        *catalog.Catalog
	refreshing atomic.Bool // a background rebuild is on its way
	preparing  atomic.Bool // Prepare has not finished
	// provisional: cat (and the user's data) came from the startup cache
	// and the store has not been read yet; the pages answer them meanwhile.
	provisional atomic.Bool
	// cacheTried closes once Prepare has tried the startup cache, so the
	// first pages wait for it (a moment) instead of for the store. Set by
	// Prepare before anything is served; nil without Prepare.
	cacheTried chan struct{}

	saveMu   sync.Mutex      // guards the three below
	toSave   *store.Snapshot // the last build not written to the startup cache
	saving   bool            // a writer is on its way
	lastSave time.Time

	indexMu sync.Mutex // guards index
	index   *search.Index
}

// current is the last catalog built, nil before the first.
func (st *catalogState) current() *catalog.Catalog {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.cat
}

// playable tells whether the server can deliver a track's audio.
func (s *Server) playable(t store.TrackView) bool {
	mode, _ := s.Streamer.Plan(stream.Format{Container: t.Format, Codec: t.Codec, Lossless: t.Lossless})
	return mode != stream.Unplayable
}

// catalog returns the catalog, rebuilt when the store changed; the
// startup cache's while the store has not been read yet.
func (s *Server) catalog() (*catalog.Catalog, error) {
	s.waitStartCache()
	if c := s.cat.current(); c != nil && s.cat.provisional.Load() {
		return c, nil
	}
	return s.freshCatalog()
}

// freshCatalog returns the catalog built from the store as it is now.
func (s *Server) freshCatalog() (*catalog.Catalog, error) {
	changes, err := s.Store.Changes()
	if err != nil {
		return nil, err
	}
	st := &s.cat
	if c := st.current(); c != nil && c.Changes == changes {
		return c, nil
	}
	st.build.Lock()
	defer st.build.Unlock()
	// Another request may have built it while this one waited.
	if c := st.current(); c != nil && c.Changes >= changes {
		return c, nil
	}
	snap, err := s.Store.Snapshot()
	if err != nil {
		return nil, err
	}
	c := catalog.Build(snap, catalog.Options{Playable: s.playable})
	st.mu.Lock()
	st.cat = c
	st.mu.Unlock()
	s.cacheLater(&snap)
	return c, nil
}

// Prepare starts building the first catalog in the background, so the status
// can tell what the first pages are waiting for, and they wait less. With a
// startup cache the pages answer from it at once, and the store is read
// behind it. The channel closes when the store's catalog is built (also when
// it failed): the startup scan waits for it, since walking the folders while
// the catalog is read from the same disk makes both slow, and the albums are
// what people wait for.
func (s *Server) Prepare() <-chan struct{} {
	s.cat.preparing.Store(true)
	ready := make(chan struct{})
	tried := make(chan struct{})
	s.cat.cacheTried = tried
	go func() {
		defer close(ready)
		defer s.cat.preparing.Store(false)
		defer s.cat.provisional.Store(false)
		if s.loadStartCache() {
			s.cat.preparing.Store(false)
		}
		close(tried)
		// Read sequentially what the snapshot would otherwise read in
		// scattered pages: seconds instead of minutes on a hard disk.
		if _, err := s.Store.Preload(); err != nil {
			log.Printf("catálogo: %v", err)
		}
		if _, err := s.freshCatalog(); err != nil {
			log.Printf("catálogo: %v", err)
		} else if _, _, err := s.freshPersonal(); err != nil {
			log.Printf("catálogo: %v", err)
		}
	}()
	return ready
}

// waitStartCache waits until Prepare has tried the startup cache.
func (s *Server) waitStartCache() {
	if ch := s.cat.cacheTried; ch != nil {
		<-ch
	}
}

// loadStartCache builds the provisional catalog from the startup cache and
// tells whether there was one to build it from.
func (s *Server) loadStartCache() bool {
	if s.CatalogCache == "" {
		return false
	}
	d, err := startcache.Load(s.CatalogCache)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Print(err)
		}
		return false
	}
	// Its counter belongs to an earlier connection: it must never pass for
	// the store's current one.
	d.Snapshot.Changes = -1
	c := catalog.Build(d.Snapshot, catalog.Options{Playable: s.playable})
	s.pers.mu.Lock()
	s.pers.p, s.pers.changes, s.pers.version = &d.Personal, -1, s.Store.PersonalVersion()
	s.pers.mu.Unlock()
	s.counts.mu.Lock()
	s.counts.last = &d.Counts
	s.counts.mu.Unlock()
	s.cat.mu.Lock()
	s.cat.cat = c
	s.cat.mu.Unlock()
	s.cat.provisional.Store(true)
	return true
}

// cacheLater writes snap to the startup cache in the background, at most
// once every cacheEvery: during a scan the catalog is rebuilt often, and
// only the last build is worth writing.
func (s *Server) cacheLater(snap *store.Snapshot) {
	if s.CatalogCache == "" || s.readOnly() {
		return
	}
	st := &s.cat
	st.saveMu.Lock()
	defer st.saveMu.Unlock()
	st.toSave = snap
	if st.saving {
		return
	}
	st.saving = true
	every := s.cacheEvery
	if every <= 0 {
		every = time.Minute
	}
	wait := time.Until(st.lastSave.Add(every))
	// One writer at a time, so an older build never lands after a newer
	// one; what was built while it wrote is written next.
	go func() {
		for {
			time.Sleep(max(wait, 0))
			st.saveMu.Lock()
			snap := st.toSave
			st.toSave = nil
			st.saveMu.Unlock()
			if err := s.saveStartCache(snap); err != nil {
				log.Printf("caché del catálogo: %v", err)
			}
			st.saveMu.Lock()
			st.lastSave = time.Now()
			if st.toSave == nil {
				st.saving = false
				st.saveMu.Unlock()
				return
			}
			wait = every
			st.saveMu.Unlock()
		}
	}()
}

// saveStartCache writes snap with the user's data resolved to its ids; nothing
// when the store changed since (a later build writes it).
func (s *Server) saveStartCache(snap *store.Snapshot) error {
	p, changes, err := s.freshPersonal()
	if err != nil || changes != snap.Changes {
		return err
	}
	counts, err := s.countFigures()
	if err != nil {
		return err
	}
	return startcache.Save(s.CatalogCache, startcache.Data{Snapshot: *snap, Personal: *p, Counts: counts})
}

// browse returns the catalog and the user's data for Inicio and Explorar.
// While a scan or an identification run keeps changing the store, a
// rebuild on every visit would keep those pages waiting (seconds on a large
// collection, holding the store's only connection while it reads): it
// answers the last ones built, without touching the store, and brings them
// up to date in the background. The pages load again as the run makes
// progress and when it ends, and by then nothing is running and the answer
// is current. A change to the user's data is never answered late.
func (s *Server) browse() (*catalog.Catalog, *store.Personal, error) {
	c := s.cat.current()
	p, version := s.pers.current()
	if c != nil && p != nil && version == s.Store.PersonalVersion() && s.running() {
		if s.cat.refreshing.CompareAndSwap(false, true) {
			go func() {
				defer s.cat.refreshing.Store(false)
				if _, err := s.catalog(); err != nil {
					log.Printf("catálogo: %v", err)
				} else if _, err := s.personal(); err != nil {
					log.Printf("catálogo: %v", err)
				}
			}()
		}
		return c, p, nil
	}
	c, err := s.catalog()
	if err != nil {
		return nil, nil, err
	}
	p, err = s.personal()
	if err != nil {
		return nil, nil, err
	}
	return c, p, nil
}

// running tells whether a scan or an identification run is under way.
func (s *Server) running() bool {
	p := s.parts()
	return (p.scanner != nil && p.scanner.Status().Running) ||
		(p.identifier != nil && p.identifier.Status().State == identify.StateRunning)
}

// searchIndex returns the catalog and its search index, both current.
func (s *Server) searchIndex() (*catalog.Catalog, *search.Index, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, nil, err
	}
	st := &s.cat
	st.indexMu.Lock()
	defer st.indexMu.Unlock()
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
	c, p, err := s.browse()
	if err != nil {
		fail(w, err)
		return
	}
	q := catalog.ParseQuery(r.URL.Query())
	writeJSON(w, c.ExploreFor(q, p, intParam(r, "desde", 0, 0, 1<<30), intParam(r, "limite", 120, 1, 500)))
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
	pers, err := s.personal()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, struct {
		catalog.ArtistPage
		Top []catalog.TrackPlays `json:"top"`
	}{p, c.ArtistTop(r.PathValue("key"), pers, 5)})
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
	Versions   []catalog.Version `json:"versions"` // other albums with the same recordings
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
	p := albumPage{AlbumDetail: a, Tracks: make([]albumTrack, 0, len(a.Tracks)), Candidates: cands, Versions: c.AlbumVersions(id)}
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
