// Package server exposes the catalog as a JSON API, the tracks' audio and
// pictures, and the embedded web app. It only answers requests addressed to
// localhost (LAN access and its PIN come in M5).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"tordo/internal/auth"
	"tordo/internal/config"
	"tordo/internal/covers"
	"tordo/internal/engine"
	"tordo/internal/jukebox"
	"tordo/internal/loudness"
	"tordo/internal/scan"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// Loudness is what the API needs from the loudness runner.
type Loudness interface {
	Prioritize(ids []int64)
	Status() loudness.Status
}

type Server struct {
	Store      *store.Store
	Scanner    *scan.Scanner   // nil in browse-only mode; ignored with an Engine
	Identifier Identifier      // nil in browse-only mode; ignored with an Engine
	Loudness   Loudness        // nil: no loudness analysis
	Normalize  bool            // the player evens out volume (config.json); ignored with an Engine
	AppDir     string          // where catalog and cache paths are relative to
	Streamer   stream.Streamer // FFmpeg "" when there is none
	Covers     *covers.Covers  // nil: no pictures
	Static     fs.FS           // the web app; nil: the embedded build
	ReadOnly   bool

	// Engine holds what config.json decides; nil (tests): the fixed fields
	// Scanner, Identifier and Normalize, and no settings.
	Engine *engine.Engine
	// VerifyToken asks Discogs whether it takes a token; nil: asking for a
	// well-known release once.
	VerifyToken func(ctx context.Context, token string) error

	// interfaces gives the host's IP addresses and name for the LAN addresses;
	// nil: the real ones. now is the clock of the LAN gate; nil: time.Now.
	interfaces func() ([]net.IP, string)
	now        func() time.Time

	cat  catalogState
	pers personalState

	muxOnce sync.Once
	mux     http.Handler

	lan     lanListener
	limiter auth.Limiter // one for every device: there is a single user
	loginMu sync.Mutex   // logins one at a time, so parallel guesses cannot skip the limiter

	// jukeboxOnce builds the Rockola's hub on first use; beat is the SSE
	// heartbeat (0: 20 s; tests shorten it).
	jukeboxOnce sync.Once
	jukeboxHub  *jukebox.Hub
	beat        time.Duration
}

// parts are the pieces config.json decides, read once per request.
type parts struct {
	scanner    *scan.Scanner // nil: browse-only mode
	identifier Identifier    // nil: browse-only mode
	normalize  bool
	lan        config.LAN
	scan       func() // starts a scan in the background
	listenMode bool   // config.json: curation routes answer 403
}

func (s *Server) parts() parts {
	if s.Engine == nil {
		p := parts{scanner: s.Scanner, identifier: s.Identifier, normalize: s.Normalize}
		if sc := s.Scanner; sc != nil {
			p.scan = func() {
				go func() {
					if err := sc.Run(context.Background()); err != nil && !errors.Is(err, scan.ErrBusy) {
						log.Printf("escaneo: %v", err)
					}
				}()
			}
		}
		return p
	}
	rt := s.Engine.Current()
	if rt == nil {
		return parts{normalize: true}
	}
	p := parts{scanner: rt.Scanner, normalize: rt.Config.Normalizing(), lan: rt.Config.LAN, scan: rt.Scan, listenMode: rt.Config.ListenMode}
	if rt.Identifier != nil {
		p.identifier = rt.Identifier
	}
	return p
}

// readOnly tells whether nothing can be written to the app folder.
func (s *Server) readOnly() bool { return s.ReadOnly || (s.Engine != nil && s.Engine.ReadOnly) }

// Handler serves the local listener: only requests addressed to localhost.
func (s *Server) Handler() http.Handler { return localOnly(s.routes()) }

// routes is the mux of both listeners, built once.
func (s *Server) routes() http.Handler {
	s.muxOnce.Do(func() { s.mux = s.buildMux() })
	return s.mux
}

// route is one pattern of the mux and its handler.
type route struct {
	pattern string
	handler http.HandlerFunc
}

// routeTable lists every route, in one place: the mux is built from it and the
// LAN gate test walks it, so a route added here is checked against the gate.
func (s *Server) routeTable() []route {
	return []route{
		{"GET /api/auth", s.authState},
		{"POST /api/auth/login", jsonOnly(s.login)},
		{"POST /api/auth/logout", jsonOnly(s.logout)},
		{"GET /api/sessions", settings(s.sessions)},
		{"DELETE /api/sessions/{id}", settings(s.closeSession)},
		{"PUT /api/sessions/{id}/host", settings(jsonOnly(s.setSessionHost))},
		{"GET /api/jukebox/speaker", speakerOnly(s.jukeboxSpeaker)},
		{"POST /api/jukebox/state", speakerOnly(jsonOnly(s.jukeboxState))},
		{"POST /api/jukebox/notice", speakerOnly(jsonOnly(s.jukeboxNotice))},
		{"GET /api/jukebox/events", s.jukeboxEvents},
		{"POST /api/jukebox/command", jsonOnly(s.jukeboxCommand)},
		{"GET /api/lan", settings(s.lanInfo)},
		{"GET /api/lan/qr", settings(s.lanQR)},
		{"GET /api/status", s.status},
		{"GET /api/config", settings(s.withEngine(s.getConfig))},
		{"PUT /api/config", settings(jsonOnly(s.withEngine(s.putConfig)))},
		{"POST /api/config/root", settings(jsonOnly(s.withEngine(s.checkRoot)))},
		{"POST /api/config/token", settings(jsonOnly(s.withEngine(s.checkToken)))},
		{"POST /api/scan", s.curation(jsonOnly(s.scan))},
		{"GET /api/albums", s.albums},
		{"GET /api/albums/{id}", s.album},
		{"GET /api/folders", s.folder},
		{"GET /api/folders/tree", s.folderTree},
		{"GET /api/folders/tree/search", s.searchFolderTree},
		{"GET /api/collections", s.collections},
		{"GET /api/explore", s.explore},
		{"GET /api/marks", s.marks},
		{"PUT /api/likes", jsonOnly(s.setLike)},
		{"PUT /api/ratings", jsonOnly(s.setRating)},
		{"POST /api/plays", jsonOnly(s.recordPlay)},
		{"GET /api/liked", s.liked},
		{"GET /api/playlists", s.playlists},
		{"POST /api/playlists", jsonOnly(s.createPlaylist)},
		{"GET /api/playlists/{id}", s.playlist},
		{"PUT /api/playlists/{id}", jsonOnly(s.renamePlaylist)},
		{"DELETE /api/playlists/{id}", s.deletePlaylist},
		{"GET /api/stations", s.stations},
		{"POST /api/stations", jsonOnly(s.createStation)},
		{"PUT /api/stations/{id}", jsonOnly(s.updateStation)},
		{"DELETE /api/stations/{id}", s.deleteStation},
		{"GET /api/station/next", s.stationNext},
		{"POST /api/playlists/{id}/items", jsonOnly(s.addToPlaylist)},
		{"DELETE /api/playlists/{id}/items/{item}", s.removeFromPlaylist},
		{"PUT /api/playlists/{id}/order", jsonOnly(s.reorderPlaylist)},
		{"GET /api/home", s.home},
		{"GET /api/stats", s.stats},
		{"GET /api/artists/{key}", s.artist},
		{"GET /api/search", s.find},
		{"PUT /api/folders/collection", s.curation(jsonOnly(s.setCollection))},
		{"POST /api/identify", s.curation(jsonOnly(s.identify))},
		{"GET /api/review/unidentified", s.unidentified},
		{"GET /api/review/duplicates", s.duplicates},
		{"GET /api/review/formats", s.formatsToConvert},
		{"PUT /api/albums/{id}/identification", s.curation(jsonOnly(s.setIdentification))},
		{"DELETE /api/albums/{id}/identification", s.curation(s.resetIdentification)},
		{"GET /api/genres", s.genreTaxonomy},
		{"GET /api/genres/unmapped", s.unmappedGenres},
		{"GET /api/genres/aliases", s.genreAliases},
		{"PUT /api/genres/aliases", s.curation(jsonOnly(s.setGenreAlias))},
		{"DELETE /api/genres/aliases", s.curation(s.deleteGenreAlias)},
		{"GET /api/play", s.play},
		{"GET /api/gains", s.gains},
		{"GET /api/covers/album/{id}", s.albumCover},
		{"GET /api/covers/track/{id}", s.trackCover},
		{"GET /stream/{id}", s.streamTrack},
		{"GET /", s.static().ServeHTTP},
	}
}

func (s *Server) buildMux() http.Handler {
	mux := http.NewServeMux()
	for _, rt := range s.routeTable() {
		mux.HandleFunc(rt.pattern, rt.handler)
	}
	return mux
}

// localOnly rejects requests whose Host is not localhost (DNS rebinding) and
// requests made by other sites' pages, which could otherwise read the catalog
// through the user's browser. Navigating to the app from a link still works.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "prohibido", http.StatusForbidden)
			return
		}
		if !sameSiteOnly(w, r) {
			return
		}
		next.ServeHTTP(w, withLocal(r))
	})
}

// sameSiteOnly rejects (403) requests made by other sites' pages, except plain
// navigations, and marks the answer same-origin only. It reports whether the
// request may go on.
func sameSiteOnly(w http.ResponseWriter, r *http.Request) bool {
	site := r.Header.Get("Sec-Fetch-Site")
	navigation := r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Mode") == "navigate"
	if (site == "cross-site" || site == "same-site") && !navigation {
		http.Error(w, "prohibido", http.StatusForbidden)
		return false
	}
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	return true
}

type localKey struct{}

// withLocal marks the request as coming from the computer that runs Tordo.
func withLocal(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), localKey{}, true))
}

// localRequest tells whether the request comes from the computer that runs
// Tordo (see decision 11); set by localOnly (always) and by the LAN gate.
func localRequest(r *http.Request) bool {
	v, _ := r.Context().Value(localKey{}).(bool)
	return v
}

// settings wraps the handlers that change or reveal the configuration: 403
// when the request is not local.
func settings(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localRequest(r) {
			http.Error(w, "Los ajustes solo se cambian en la computadora que corre Tordo.", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// curation wraps the handlers that change the catalog's curation (scan,
// identification, collections, genre aliases): 403 while listen mode is on.
func (s *Server) curation(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.parts().listenMode {
			http.Error(w, "Modo escucha activo: la curaduría está bloqueada.", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// withEngine answers 404 when there is no engine (no settings).
func (s *Server) withEngine(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Engine == nil {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}
}

// jsonOnly requires a JSON content type, which forces a CORS preflight on
// cross-site requests and so blocks other pages from calling write endpoints.
func jsonOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "se requiere application/json", http.StatusUnsupportedMediaType)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) { writeJSONStatus(w, http.StatusOK, v) }

// writeJSONStatus sets the content type before the status, or it is lost.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("respuesta: %v", err)
	}
}

func fail(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "error interno", http.StatusInternalServerError)
}

func (s *Server) jukebox() *jukebox.Hub {
	s.jukeboxOnce.Do(func() { s.jukeboxHub = jukebox.New() })
	return s.jukeboxHub
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	p := s.parts()
	var st scan.Status
	if p.scanner != nil {
		st = p.scanner.Status()
	}
	counts, err := s.Store.Counts()
	if err != nil {
		fail(w, err)
		return
	}
	var ident any = map[string]string{"state": "off"}
	if p.identifier != nil {
		ident = p.identifier.Status()
	}
	var loud any = map[string]string{"state": "off"}
	if s.Loudness != nil {
		loud = s.Loudness.Status()
	}
	writeJSON(w, map[string]any{"readOnly": s.readOnly(), "setupPending": s.Engine != nil && s.Engine.SetupPending(),
		"scan": st, "identify": ident, "loudness": loud, "normalize": p.normalize, "listenMode": p.listenMode, "counts": counts,
		"jukebox": map[string]bool{"speaker": s.jukebox().Speaking()}})
}

// setupPending answers 409 while the first-use assistant has not saved: nothing
// is scanned or identified before it does (decision 8).
func (s *Server) setupPending(w http.ResponseWriter) bool {
	if s.Engine != nil && s.Engine.SetupPending() {
		http.Error(w, "completá primero el asistente", http.StatusConflict)
		return true
	}
	return false
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	if s.setupPending(w) {
		return
	}
	p := s.parts()
	if p.scanner == nil {
		http.Error(w, "modo consulta: no se puede escanear", http.StatusConflict)
		return
	}
	if p.scanner.Status().Running {
		http.Error(w, scan.ErrBusy.Error(), http.StatusConflict)
		return
	}
	p.scan()
	writeJSONStatus(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Server) albums(w http.ResponseWriter, r *http.Request) {
	as, err := s.Store.Albums(r.URL.Query().Get("folder"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, as)
}

func (s *Server) album(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r); ok {
		s.writeAlbum(w, id)
	}
}

func (s *Server) collections(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.Collections()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, cs)
}
