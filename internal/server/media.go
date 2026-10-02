package server

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"

	"tordo/internal/appdir"
	"tordo/internal/catalog"
	"tordo/internal/loudness"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// MaxPlay caps the tracks one play request returns: the player keeps its
// queue in the browser's storage. A variable so tests can lower it.
var MaxPlay = 5000

// playItem is a track as the player gets it: how the server will deliver it.
type playItem struct {
	store.PlayItem
	loudness.TrackGain
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

// httpError is a request the server refuses, with the status and text to answer.
type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string { return e.msg }

// answer writes err as the response: an *httpError with its status, anything
// else as an internal error.
func answer(w http.ResponseWriter, err error) {
	var he *httpError
	if errors.As(err, &he) {
		http.Error(w, he.msg, he.code)
		return
	}
	fail(w, err)
}

func badID() error { return &httpError{http.StatusBadRequest, "id inválido"} }

// resolve lists the tracks a play query names: album, folder, artist,
// track (repeatable), playlist (its available entries in order) or liked=1
// (the liked tracks, most recent first). Bad or unknown references are
// *httpError (400, 404).
func (s *Server) resolve(q url.Values) ([]store.PlayItem, error) {
	switch {
	case q.Has("album"):
		id, err := strconv.ParseInt(q.Get("album"), 10, 64)
		if err != nil {
			return nil, badID()
		}
		if _, ok, err := s.Store.Album(id); err != nil {
			return nil, err
		} else if !ok {
			return nil, &httpError{http.StatusNotFound, "álbum no encontrado"}
		}
		return s.Store.PlayAlbum(id)
	case q.Has("folder"):
		return s.Store.PlayFolder(q.Get("folder"))
	case q.Has("artist"):
		items, ok, err := s.playArtist(q.Get("artist"))
		if err == nil && !ok {
			return nil, &httpError{http.StatusNotFound, "artista no encontrado"}
		}
		return items, err
	case q.Has("track"):
		var ids []int64
		for _, v := range q["track"] {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, badID()
			}
			ids = append(ids, id)
		}
		return s.Store.PlayTracks(ids)
	case q.Has("playlist"):
		id, err := strconv.ParseInt(q.Get("playlist"), 10, 64)
		if err != nil {
			return nil, badID()
		}
		items, ok, err := s.Store.PlayPlaylist(id)
		if err == nil && !ok {
			return nil, &httpError{http.StatusNotFound, "lista no encontrada"}
		}
		return items, err
	case q.Get("liked") == "1":
		ids, err := s.Store.LikedTrackIDs()
		if err != nil {
			return nil, err
		}
		return s.Store.PlayTracks(ids)
	}
	return nil, &httpError{http.StatusBadRequest, "falta qué reproducir: album, folder, artist, track, playlist o liked"}
}

// play answers GET /api/play: the tracks to queue for an album
// (?album=id), a folder or collection (?folder=path, recursive, in order),
// an artist (?artist=key: their albums by year, then their tracks on
// others' albums), some tracks (?track=id, repeatable), a playlist
// (?playlist=id) or the liked tracks (?liked=1), shuffled with ?shuffle=1.
// Tracks that cannot play (they need ffmpeg) are left out and counted.
func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := s.resolve(q)
	if err != nil {
		answer(w, err)
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
	ids := make([]int64, len(out))
	for i, it := range out {
		ids[i] = it.ID
	}
	gains, err := s.gainsFor(ids)
	if err != nil {
		fail(w, err)
		return
	}
	for i := range out {
		out[i].TrackGain = gains[out[i].ID]
	}
	s.prioritizeUnmeasured(ids, gains)
	writeJSON(w, map[string]any{"items": out, "skipped": skipped, "truncated": truncated, "ffmpeg": s.Streamer.FFmpeg != ""})
}

// playArtist lists an artist's tracks: their albums oldest first, then their
// appearances on others' albums.
func (s *Server) playArtist(key string) ([]store.PlayItem, bool, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, false, err
	}
	p, ok := c.ArtistPage(key)
	if !ok {
		return nil, false, nil
	}
	var out []store.PlayItem
	for _, a := range p.Albums {
		items, err := s.Store.PlayAlbum(a.ID)
		if err != nil {
			return nil, true, err
		}
		out = append(out, items...)
	}
	var ids []int64
	for _, ap := range p.Appearances {
		for _, t := range ap.Tracks {
			ids = append(ids, t.ID)
		}
	}
	items, err := s.Store.PlayTracks(ids)
	return append(out, items...), true, err
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

// prioritizeCount is how many unmeasured tracks of a request are sent to the
// loudness runner to measure first.
const prioritizeCount = 20

// maxGains caps the ids of one /api/gains request.
const maxGains = 100

// gainsFor computes the gains of the given tracks. Tracks of compilations get
// no album gain: their tracks come from different sources and levels, so
// levelling them to one album reference keeps the jumps between them.
func (s *Server) gainsFor(ids []int64) (map[int64]loudness.TrackGain, error) {
	facts, err := s.Store.LoudnessFacts(ids)
	if err != nil {
		return nil, err
	}
	gains := loudness.Gains(facts)
	var c *catalog.Catalog
	compilation := map[int64]bool{} // album id -> is a compilation
	for id, g := range gains {
		albumID := facts[id].AlbumID
		if g.Album == nil || albumID == 0 {
			continue
		}
		comp, ok := compilation[albumID]
		if !ok {
			if c == nil {
				if c, err = s.catalog(); err != nil {
					return nil, err
				}
			}
			a, found := c.Album(albumID)
			comp = found && a.Compilation
			compilation[albumID] = comp
		}
		if comp {
			g.Album = nil
			gains[id] = g
		}
	}
	return gains, nil
}

// prioritizeUnmeasured asks the runner to measure first the first tracks of
// ids (in order) that have no track gain.
func (s *Server) prioritizeUnmeasured(ids []int64, gains map[int64]loudness.TrackGain) {
	if s.Loudness == nil {
		return
	}
	var want []int64
	for _, id := range ids {
		if g, ok := gains[id]; ok && g.Track == nil {
			if want = append(want, id); len(want) == prioritizeCount {
				break
			}
		}
	}
	if len(want) > 0 {
		s.Loudness.Prioritize(want)
	}
}

// gains answers GET /api/gains?track=1&track=2: {id: gains} for those tracks,
// and has the unmeasured ones measured first.
func (s *Server) gains(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()["track"]
	if len(q) == 0 || len(q) > maxGains {
		http.Error(w, "se esperan entre 1 y 100 pistas", http.StatusBadRequest)
		return
	}
	ids := make([]int64, len(q))
	for i, v := range q {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}
		ids[i] = id
	}
	all, err := s.gainsFor(ids)
	if err != nil {
		fail(w, err)
		return
	}
	out := make(map[int64]loudness.TrackGain, len(ids))
	for _, id := range ids {
		if g, ok := all[id]; ok {
			out[id] = g
		}
	}
	s.prioritizeUnmeasured(ids, out)
	writeJSON(w, out)
}
