package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"tordo/internal/jukebox"
)

// speakerOnly wraps the speaker's endpoints: only the computer that runs
// Tordo plays the Rockola.
func speakerOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localRequest(r) {
			writeError(w, http.StatusForbidden, codeAudioLocal, "audio can only be turned on at the computer running Tordo", nil)
			return
		}
		h(w, r)
	}
}

func (s *Server) heartbeat() time.Duration {
	if s.beat > 0 {
		return s.beat
	}
	return 20 * time.Second
}

// stream writes c's events as server-sent events until the client leaves or
// the hub drops c (then what is already queued is written first).
func (s *Server) stream(w http.ResponseWriter, r *http.Request, c *jukebox.Conn) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if rc.Flush() != nil {
		return
	}
	tick := time.NewTicker(s.heartbeat())
	defer tick.Stop()
	// A phone that vanishes from Wi-Fi leaves a half-open connection: bound every write.
	deadline := func() bool {
		err := rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return err == nil || errors.Is(err, http.ErrNotSupported)
	}
	write := func(e jukebox.Event) bool {
		data, err := json.Marshal(e.Data)
		if err != nil {
			log.Printf("jukebox: %v", err)
			return true
		}
		if !deadline() {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-c.Events():
			if !write(e) {
				return
			}
		case <-c.Done():
			for {
				select {
				case e := <-c.Events():
					if !write(e) {
						return
					}
				default:
					return
				}
			}
		case <-tick.C:
			if !deadline() {
				return
			}
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}

func (s *Server) jukeboxSpeaker(w http.ResponseWriter, r *http.Request) {
	c, _ := s.jukebox().AttachSpeaker()
	defer s.jukebox().DetachSpeaker(c)
	s.stream(w, r, c)
}

func (s *Server) jukeboxEvents(w http.ResponseWriter, r *http.Request) {
	session, role, ok := requestRole(r)
	if !ok {
		writeError(w, http.StatusForbidden, codeForbidden, "forbidden", nil)
		return
	}
	c, err := s.jukebox().AttachRemote(session, role)
	if err != nil {
		fail(w, err)
		return
	}
	defer s.jukebox().DetachRemote(c)
	s.stream(w, r, c)
}

func (s *Server) jukeboxState(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string          `json:"token"`
		State json.RawMessage `json:"state"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&in); err != nil || len(in.State) == 0 {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	if err := s.jukebox().Publish(in.Token, in.State); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// cleanNotice checks a notice payload: a non-empty string (trimmed, at most
// 300 characters) or an object with a string key and optional object params.
// It returns the payload to forward, or false when it is neither.
func cleanNotice(raw json.RawMessage) (json.RawMessage, bool) {
	var str string
	if json.Unmarshal(raw, &str) == nil {
		str = strings.TrimSpace(str)
		if utf8.RuneCountInString(str) > 300 {
			str = string([]rune(str)[:300])
		}
		if str == "" {
			return nil, false
		}
		out, err := json.Marshal(str)
		return out, err == nil
	}
	var obj struct {
		Key    string         `json:"key"`
		Params map[string]any `json:"params,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil || obj.Key == "" {
		return nil, false
	}
	out, err := json.Marshal(obj)
	return out, err == nil
}

func (s *Server) jukeboxNotice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token  string          `json:"token"`
		Remote string          `json:"remote"`
		Notice json.RawMessage `json:"notice"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	notice, ok := cleanNotice(in.Notice)
	if !ok {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	if err := s.jukebox().Notice(in.Token, in.Remote, notice); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) jukeboxCommand(w http.ResponseWriter, r *http.Request) {
	session, role, ok := requestRole(r)
	if !ok {
		writeError(w, http.StatusForbidden, codeForbidden, "forbidden", nil)
		return
	}
	var in struct {
		Remote  string          `json:"remote"`
		Command string          `json:"command"`
		Args    json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	if err := s.jukebox().Command(in.Remote, session, role, in.Command, in.Args); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) setSessionHost(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in struct {
		Host *bool `json:"host"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil || in.Host == nil {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	found, err := s.Store.SetSessionHost(id, *in.Host)
	if err != nil {
		fail(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, codeNotFound, "not found", nil)
		return
	}
	role := jukebox.Guest
	if *in.Host {
		role = jukebox.Host
	}
	s.jukebox().SetRole(id, role)
	w.WriteHeader(http.StatusNoContent)
}
