package server

import (
	"math/rand/v2"
	"net/http"
	"strconv"

	"tordo/internal/appdir"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// MaxPlay caps the tracks one play request returns: the player keeps its
// queue in the browser's storage. A variable so tests can lower it.
var MaxPlay = 5000

// playItem is a track as the player gets it: how the server will deliver it.
type playItem struct {
	store.PlayItem
	MIME       string `json:"mime"`       // what /stream/{id} sends
	Transcoded bool   `json:"transcoded"` // converted by ffmpeg: no seeking
}

func trackFormat(f store.TrackFile) stream.Format {
	return stream.Format{Container: f.Format, Codec: f.Codec, Lossless: f.Lossless}
}

// streamTrack answers GET /stream/{id}: the track's audio, by id (never by
// path). ?transcode=1 asks for a conversion of a format the browser cannot
// play.
func (s *Server) streamTrack(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	f, ok, err := s.Store.TrackFile(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "pista no encontrada", http.StatusNotFound)
		return
	}
	s.Streamer.Serve(w, r, appdir.Abs(s.AppDir, f.Path), trackFormat(f), r.URL.Query().Get("transcode") == "1")
}

// play answers GET /api/play: the tracks to queue for an album
// (?album=id), a folder or collection (?folder=path, recursive, in order)
// or some tracks (?track=id, repeatable), shuffled with ?shuffle=1. Tracks
// that cannot play (they need ffmpeg) are left out and counted.
func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var items []store.PlayItem
	var err error
	switch {
	case q.Has("album"):
		id, perr := strconv.ParseInt(q.Get("album"), 10, 64)
		if perr != nil {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}
		items, err = s.Store.PlayAlbum(id)
	case q.Has("folder"):
		items, err = s.Store.PlayFolder(q.Get("folder"))
	case q.Has("track"):
		var ids []int64
		for _, v := range q["track"] {
			id, perr := strconv.ParseInt(v, 10, 64)
			if perr != nil {
				http.Error(w, "id inválido", http.StatusBadRequest)
				return
			}
			ids = append(ids, id)
		}
		items, err = s.Store.PlayTracks(ids)
	default:
		http.Error(w, "falta qué reproducir: album, folder o track", http.StatusBadRequest)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	if q.Get("shuffle") == "1" {
		rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	}
	out := []playItem{}
	skipped, truncated := 0, 0
	for _, it := range items {
		mode, mime := s.Streamer.Plan(stream.Format{Container: it.Format, Codec: it.Codec, Lossless: it.Lossless})
		if mode == stream.Unplayable {
			skipped++
			continue
		}
		if len(out) < MaxPlay {
			out = append(out, playItem{PlayItem: it, MIME: mime, Transcoded: mode == stream.Transcode})
		} else {
			truncated++
		}
	}
	writeJSON(w, map[string]any{"items": out, "skipped": skipped, "truncated": truncated, "ffmpeg": s.Streamer.FFmpeg != ""})
}

func (s *Server) albumCover(w http.ResponseWriter, r *http.Request) {
	s.coverByID(w, r, s.Covers.ServeAlbum)
}

func (s *Server) trackCover(w http.ResponseWriter, r *http.Request) {
	s.coverByID(w, r, s.Covers.ServeTrack)
}

func (s *Server) coverByID(w http.ResponseWriter, r *http.Request, serve func(http.ResponseWriter, *http.Request, int64)) {
	if s.Covers == nil {
		http.Error(w, "sin carátula", http.StatusNotFound)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	serve(w, r, id)
}
