package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"tordo/internal/genres"
	"tordo/internal/httpx"
	"tordo/internal/identify"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// Identifier is what the API needs from the identification runner.
type Identifier interface {
	Status() identify.Status
	Trigger()
	Apply() error
	SetRelease(ctx context.Context, albumID int64, releaseID string) error
	Ignore(albumID int64) error
	Reset(albumID int64) error
}

const maxBody = 1 << 16

func noIdentifier(w http.ResponseWriter, p parts) bool {
	if p.identifier == nil {
		writeError(w, http.StatusConflict, codeReadOnly, "browse-only mode: cannot identify or correct", nil)
		return true
	}
	return false
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidID, "invalid id", nil)
		return 0, false
	}
	return id, true
}

func (s *Server) identify(w http.ResponseWriter, r *http.Request) {
	if s.setupPending(w) {
		return
	}
	p := s.parts()
	if noIdentifier(w, p) {
		return
	}
	p.identifier.Trigger()
	writeJSONStatus(w, http.StatusAccepted, map[string]bool{"started": true})
}

func (s *Server) unidentified(w http.ResponseWriter, r *http.Request) {
	us, err := s.Store.Unidentified()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, us)
}

// releaseIDFrom accepts an MBID or a musicbrainz.org release URL.
func releaseIDFrom(s string) string {
	s = strings.TrimSpace(s)
	if _, rest, ok := strings.Cut(s, "musicbrainz.org/release/"); ok {
		if i := strings.IndexAny(rest, "/?#"); i >= 0 {
			rest = rest[:i]
		}
		return rest
	}
	return s
}

func (s *Server) setIdentification(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	p := s.parts()
	if !ok || noIdentifier(w, p) {
		return
	}
	var body struct {
		ReleaseID string `json:"releaseId"`
		Ignore    bool   `json:"ignore"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	var err error
	switch {
	case body.Ignore:
		err = p.identifier.Ignore(id)
	case strings.TrimSpace(body.ReleaseID) != "":
		err = p.identifier.SetRelease(r.Context(), id, releaseIDFrom(body.ReleaseID))
	default:
		writeError(w, http.StatusBadRequest, codeMissingRelease, "releaseId or ignore is required", nil)
		return
	}
	s.afterCorrection(w, p, id, err)
}

func (s *Server) resetIdentification(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	p := s.parts()
	if !ok || noIdentifier(w, p) {
		return
	}
	s.afterCorrection(w, p, id, p.identifier.Reset(id))
}

// afterCorrection answers a correction with the updated album and starts a
// run, so the artist, genres and cover of a chosen release follow.
func (s *Server) afterCorrection(w http.ResponseWriter, p parts, id int64, err error) {
	switch {
	case errors.Is(err, musicbrainz.ErrInvalidID):
		writeError(w, http.StatusBadRequest, codeInvalidMBID, "invalid MusicBrainz id", nil)
		return
	case errors.Is(err, musicbrainz.ErrNotFound):
		writeError(w, http.StatusNotFound, codeReleaseNotFound, "release not found in MusicBrainz", nil)
		return
	case errors.Is(err, identify.ErrNoAlbum):
		writeError(w, http.StatusNotFound, codeAlbumNotFound, "album not found", nil)
		return
	case errors.Is(err, httpx.ErrOffline):
		writeError(w, http.StatusServiceUnavailable, codeMusicBrainzOffline, "MusicBrainz is unreachable", nil)
		return
	case err != nil:
		fail(w, err)
		return
	}
	p.identifier.Trigger()
	s.writeAlbum(w, id)
}

func (s *Server) genreTaxonomy(w http.ResponseWriter, r *http.Request) { writeJSON(w, genres.Taxonomy) }

func (s *Server) unmappedGenres(w http.ResponseWriter, r *http.Request) {
	us, err := s.Store.UnmappedGenres()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, us)
}

func (s *Server) genreAliases(w http.ResponseWriter, r *http.Request) {
	as, err := s.Store.GenreAliases()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, as)
}

func (s *Server) setGenreAlias(w http.ResponseWriter, r *http.Request) {
	p := s.parts()
	if noIdentifier(w, p) {
		return
	}
	var a store.GenreAlias
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	if genres.Key(a.Raw) == "" {
		writeError(w, http.StatusBadRequest, codeMissingRaw, "raw value is required", nil)
		return
	}
	if (a.Genre == "" && a.Style != "") || (a.Genre != "" && !genres.Valid(a.Genre, a.Style)) {
		writeError(w, http.StatusBadRequest, codeUnknownGenre, "unknown genre or style", nil)
		return
	}
	a.Raw = genres.Key(a.Raw)
	if err := s.Store.SetGenreAlias(a); err != nil {
		fail(w, err)
		return
	}
	if err := p.identifier.Apply(); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, a)
}

// deleteGenreAlias forgets the user's mapping of ?raw=.
func (s *Server) deleteGenreAlias(w http.ResponseWriter, r *http.Request) {
	p := s.parts()
	if noIdentifier(w, p) {
		return
	}
	raw := genres.Key(r.URL.Query().Get("raw"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, codeMissingRaw, "raw value is required", nil)
		return
	}
	if err := s.Store.DeleteGenreAlias(raw); err != nil {
		fail(w, err)
		return
	}
	if err := p.identifier.Apply(); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
