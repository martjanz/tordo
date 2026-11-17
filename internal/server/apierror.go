package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"tordo/internal/auth"
	"tordo/internal/config"
	"tordo/internal/engine"
	"tordo/internal/jukebox"
	"tordo/internal/scan"
	"tordo/internal/store"
)

// apiError is an error body: what writeError sends. The web app shows
// error.<code> in the user's language (with params) and falls back to Message,
// which is English and also fit for a log.
type apiError struct {
	Code    errCode        `json:"code"`
	Message string         `json:"message"`
	Params  map[string]any `json:"params,omitempty"`
}

// writeError answers an error as {code, message, params}; params may be nil.
func writeError(w http.ResponseWriter, status int, code errCode, message string, params map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(apiError{code, message, params}); err != nil {
		log.Printf("response: %v", err)
	}
}

// httpError is a request the server refuses: the status and the error body to
// answer. Handlers return it where they cannot answer themselves; answer and
// errorFor turn it into a response.
type httpError struct {
	status int
	code   errCode
	msg    string
	params map[string]any
}

func (e *httpError) Error() string { return e.msg }

// reject builds an *httpError.
func reject(status int, code errCode, msg string, params map[string]any) *httpError {
	return &httpError{status, code, msg, params}
}

// errorFor maps the errors that deeper packages report on purpose (sentinels
// and typed errors) to a response; ok is false for any other error.
func errorFor(err error) (status int, code errCode, message string, params map[string]any, ok bool) {
	var he *httpError
	var re *config.RootError
	var pe *engine.PortError
	switch {
	case errors.As(err, &he):
		return he.status, he.code, he.msg, he.params, true

	case errors.Is(err, jukebox.ErrNoSpeaker):
		return http.StatusConflict, codeNoSpeaker, err.Error(), nil, true
	case errors.Is(err, jukebox.ErrNotSpeaker):
		return http.StatusConflict, codeNotSpeaker, err.Error(), nil, true
	case errors.Is(err, jukebox.ErrUnknownCommand):
		return http.StatusBadRequest, codeUnknownCommand, err.Error(), nil, true
	case errors.Is(err, jukebox.ErrForbidden):
		return http.StatusForbidden, codeHostOnly, err.Error(), nil, true
	case errors.Is(err, jukebox.ErrUnknownRemote):
		return http.StatusNotFound, codeUnknownRemote, err.Error(), nil, true
	case errors.Is(err, jukebox.ErrTooMany):
		return http.StatusServiceUnavailable, codeTooManyRemotes, err.Error(), nil, true
	case errors.Is(err, jukebox.ErrBusy):
		return http.StatusServiceUnavailable, codeSpeakerBusy, err.Error(), nil, true

	case errors.Is(err, store.ErrStationName):
		return http.StatusBadRequest, codeStationNameRequired, err.Error(), nil, true
	case errors.Is(err, store.ErrPlaylistName):
		return http.StatusBadRequest, codePlaylistNameRequired, err.Error(), nil, true
	case errors.Is(err, store.ErrPlaylistFull):
		return http.StatusBadRequest, codePlaylistFull, err.Error(), nil, true
	case errors.Is(err, store.ErrPlaylistOrder):
		return http.StatusBadRequest, codePlaylistOrder, err.Error(), nil, true
	case errors.Is(err, store.ErrBadRating):
		return http.StatusBadRequest, codeInvalidRating, err.Error(), nil, true

	case errors.Is(err, auth.ErrPINFormat):
		return http.StatusBadRequest, codePINFormat, err.Error(), nil, true
	case errors.Is(err, engine.ErrReadOnly):
		return http.StatusConflict, codeReadOnly, err.Error(), nil, true
	case errors.Is(err, scan.ErrBusy):
		return http.StatusConflict, codeScanRunning, err.Error(), nil, true

	case errors.As(err, &pe):
		return http.StatusBadRequest, codePortUnavailable, err.Error(), map[string]any{"port": pe.Port}, true
	case errors.As(err, &re):
		return rootError(re)
	}
	return 0, "", "", nil, false
}

// rootError maps a rejected root to its code, with the folder(s) as params.
func rootError(re *config.RootError) (int, errCode, string, map[string]any, bool) {
	var code errCode
	switch {
	case errors.Is(re, config.ErrRootRequired):
		code = codeRootRequired
	case errors.Is(re, config.ErrRootNotFound):
		code = codeRootNotFound
	case errors.Is(re, config.ErrRootNotDir):
		code = codeRootNotDir
	case errors.Is(re, config.ErrRootOtherDisk):
		code = codeRootOtherDisk
	case errors.Is(re, config.ErrRootIsApp):
		code = codeRootIsApp
	case errors.Is(re, config.ErrRootContainsApp):
		code = codeRootContainsApp
	case errors.Is(re, config.ErrRootInsideApp):
		code = codeRootInsideApp
	case errors.Is(re, config.ErrRootsRequired):
		code = codeRootsRequired
	case errors.Is(re, config.ErrRootMalformed):
		code = codeRootMalformed
	case errors.Is(re, config.ErrRootDuplicate):
		code = codeRootDuplicate
	case errors.Is(re, config.ErrRootsNested):
		code = codeRootsNested
	default:
		return 0, "", "", nil, false
	}
	var params map[string]any
	switch {
	case re.Other != "":
		params = map[string]any{"first": re.Path, "second": re.Other}
	case re.Path != "":
		params = map[string]any{"path": re.Path}
	}
	return http.StatusBadRequest, code, re.Error(), params, true
}

// fail answers err: the status and code errorFor knows, else a logged 500.
func fail(w http.ResponseWriter, err error) {
	if status, code, msg, params, ok := errorFor(err); ok {
		writeError(w, status, code, msg, params)
		return
	}
	log.Printf("error: %v", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "internal error", nil)
}

// The not-found answers handlers share.
var (
	errAlbumNotFound         = reject(http.StatusNotFound, codeAlbumNotFound, "album not found", nil)
	errArtistNotFound        = reject(http.StatusNotFound, codeArtistNotFound, "artist not found", nil)
	errFolderNotFound        = reject(http.StatusNotFound, codeFolderNotFound, "folder not found", nil)
	errTrackNotFound         = reject(http.StatusNotFound, codeTrackNotFound, "track not found", nil)
	errPlaylistNotFound      = reject(http.StatusNotFound, codePlaylistNotFound, "playlist not found", nil)
	errPlaylistTrackNotFound = reject(http.StatusNotFound, codePlaylistTrackNotFound, "track not in the playlist", nil)
	errStationNotFound       = reject(http.StatusNotFound, codeStationNotFound, "station not found", nil)
)
