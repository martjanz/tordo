package server

import (
	"encoding/json"
	"io"
	"net/http"

	"tordo/internal/store"
	"tordo/internal/stream"
)

// folderFile is a file of a folder as the page gets it.
type folderFile struct {
	store.FolderFile
	Playable bool `json:"playable"` // a track the server can deliver (false: not a track, or it needs ffmpeg)
}

// folderTree answers one level of the Carpetas tree (no path: the top).
func (s *Server) folderTree(w http.ResponseWriter, r *http.Request) {
	nodes, ok, err := s.Store.FolderTree(r.URL.Query().Get("path"))
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "carpeta no encontrada", http.StatusNotFound)
		return
	}
	if nodes == nil {
		nodes = []store.TreeNode{}
	}
	writeJSON(w, nodes)
}

// searchFolderTree filters the tree by folder name.
func (s *Server) searchFolderTree(w http.ResponseWriter, r *http.Request) {
	res, err := s.Store.SearchFolderTree(r.URL.Query().Get("q"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, res)
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
	files := make([]folderFile, len(f.Files))
	for i, ff := range f.Files {
		mode, _ := s.Streamer.Plan(stream.Format{Container: ff.Format, Codec: ff.Codec, Lossless: ff.Lossless})
		files[i] = folderFile{ff, ff.Track && mode != stream.Unplayable}
	}
	writeJSON(w, struct {
		store.FolderView
		Files []folderFile       `json:"files"` // shadows the embedded Files
		Art   map[string][]int64 `json:"art"`
		Trail []store.Crumb      `json:"trail"`
	}{f, files, art, trail})
}

// setCollection answers "no es una colección" (collection false) on a
// detected collection, or takes it back (true), with the folder's summary.
func (s *Server) setCollection(w http.ResponseWriter, r *http.Request) {
	if s.ReadOnly {
		http.Error(w, "modo consulta: no se pueden guardar cambios", http.StatusConflict)
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
