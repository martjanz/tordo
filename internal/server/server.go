// Package server exposes the catalog as a JSON API, the tracks' audio and
// pictures, and the embedded web app. It only answers requests addressed to
// localhost (LAN access and its PIN come in M5).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"

	"tordo/internal/covers"
	"tordo/internal/scan"
	"tordo/internal/store"
	"tordo/internal/stream"
)

type Server struct {
	Store      *store.Store
	Scanner    *scan.Scanner   // nil in browse-only mode
	Identifier Identifier      // nil in browse-only mode
	AppDir     string          // where catalog and cache paths are relative to
	Streamer   stream.Streamer // FFmpeg "" when there is none
	Covers     *covers.Covers  // nil: no pictures
	Static     fs.FS           // the web app; nil: the embedded build
	ReadOnly   bool

	cat  catalogState
	pers personalState
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("POST /api/scan", jsonOnly(s.scan))
	mux.HandleFunc("GET /api/albums", s.albums)
	mux.HandleFunc("GET /api/albums/{id}", s.album)
	mux.HandleFunc("GET /api/folders", s.folder)
	mux.HandleFunc("GET /api/collections", s.collections)
	mux.HandleFunc("GET /api/explore", s.explore)
	mux.HandleFunc("GET /api/marks", s.marks)
	mux.HandleFunc("PUT /api/likes", jsonOnly(s.setLike))
	mux.HandleFunc("PUT /api/ratings", jsonOnly(s.setRating))
	mux.HandleFunc("POST /api/plays", jsonOnly(s.recordPlay))
	mux.HandleFunc("GET /api/liked", s.liked)
	mux.HandleFunc("GET /api/playlists", s.playlists)
	mux.HandleFunc("POST /api/playlists", jsonOnly(s.createPlaylist))
	mux.HandleFunc("GET /api/playlists/{id}", s.playlist)
	mux.HandleFunc("PUT /api/playlists/{id}", jsonOnly(s.renamePlaylist))
	mux.HandleFunc("DELETE /api/playlists/{id}", s.deletePlaylist)
	mux.HandleFunc("POST /api/playlists/{id}/items", jsonOnly(s.addToPlaylist))
	mux.HandleFunc("DELETE /api/playlists/{id}/items/{item}", s.removeFromPlaylist)
	mux.HandleFunc("PUT /api/playlists/{id}/order", jsonOnly(s.reorderPlaylist))
	mux.HandleFunc("GET /api/artists/{key}", s.artist)
	mux.HandleFunc("GET /api/search", s.find)
	mux.HandleFunc("PUT /api/folders/collection", jsonOnly(s.setCollection))
	mux.HandleFunc("POST /api/identify", jsonOnly(s.identify))
	mux.HandleFunc("GET /api/review/unidentified", s.unidentified)
	mux.HandleFunc("PUT /api/albums/{id}/identification", jsonOnly(s.setIdentification))
	mux.HandleFunc("DELETE /api/albums/{id}/identification", s.resetIdentification)
	mux.HandleFunc("GET /api/genres", s.genreTaxonomy)
	mux.HandleFunc("GET /api/genres/unmapped", s.unmappedGenres)
	mux.HandleFunc("GET /api/genres/aliases", s.genreAliases)
	mux.HandleFunc("PUT /api/genres/aliases", jsonOnly(s.setGenreAlias))
	mux.HandleFunc("DELETE /api/genres/aliases", s.deleteGenreAlias)
	mux.HandleFunc("GET /api/play", s.play)
	mux.HandleFunc("GET /api/covers/album/{id}", s.albumCover)
	mux.HandleFunc("GET /api/covers/track/{id}", s.trackCover)
	mux.HandleFunc("GET /stream/{id}", s.streamTrack)
	mux.Handle("GET /", s.static())
	return localOnly(mux)
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
		site := r.Header.Get("Sec-Fetch-Site")
		navigation := r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Mode") == "navigate"
		if (site == "cross-site" || site == "same-site") && !navigation {
			http.Error(w, "prohibido", http.StatusForbidden)
			return
		}
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
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

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	var st scan.Status
	if s.Scanner != nil {
		st = s.Scanner.Status()
	}
	counts, err := s.Store.Counts()
	if err != nil {
		fail(w, err)
		return
	}
	var ident any = map[string]string{"state": "off"}
	if s.Identifier != nil {
		ident = s.Identifier.Status()
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st, "identify": ident, "counts": counts})
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	if s.Scanner == nil {
		http.Error(w, "modo consulta: no se puede escanear", http.StatusConflict)
		return
	}
	if s.Scanner.Status().Running {
		http.Error(w, scan.ErrBusy.Error(), http.StatusConflict)
		return
	}
	go func() {
		if err := s.Scanner.Run(context.Background()); err != nil && !errors.Is(err, scan.ErrBusy) {
			log.Printf("escaneo: %v", err)
		}
	}()
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

func (s *Server) folder(w http.ResponseWriter, r *http.Request) {
	f, ok, err := s.Store.Folder(r.URL.Query().Get("path"))
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "carpeta no encontrada", http.StatusNotFound)
		return
	}
	art, err := s.Store.FolderArt(f.Path)
	if err != nil {
		fail(w, err)
		return
	}
	trail, err := s.Store.FolderTrail(f.Path)
	if err != nil {
		fail(w, err)
		return
	}
	// art: up to four album ids per subfolder, for its card's picture;
	// trail: the folders from the root down to this one.
	writeJSON(w, struct {
		store.FolderView
		Art   map[string][]int64 `json:"art"`
		Trail []store.Crumb      `json:"trail"`
	}{f, art, trail})
}

// setCollection answers "no es una colección" (collection false) on a
// detected collection, or takes it back (true), with the folder's summary.
func (s *Server) setCollection(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly {
		http.Error(w, readOnlyMsg, http.StatusConflict)
		return
	}
	var body struct {
		Path       string `json:"path"`
		Collection bool   `json:"collection"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&body); err != nil || body.Path == "" {
		http.Error(w, "cuerpo inválido", http.StatusBadRequest)
		return
	}
	if _, ok, err := s.Store.Folder(body.Path); err != nil || !ok {
		if err != nil {
			fail(w, err)
		} else {
			http.Error(w, "carpeta no encontrada", http.StatusNotFound)
		}
		return
	}
	if err := s.Store.SetCollection(body.Path, body.Collection); err != nil {
		fail(w, err)
		return
	}
	f, _, err := s.Store.Folder(body.Path)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, f.FolderSummary)
}

func (s *Server) collections(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.Collections()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, cs)
}
