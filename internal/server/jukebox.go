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
			http.Error(w, "El audio solo se activa en la computadora que corre Tordo.", http.StatusForbidden)
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
			log.Printf("rockola: %v", err)
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
		http.Error(w, "prohibido", http.StatusForbidden)
		return
	}
	c, err := s.jukebox().AttachRemote(session, role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
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
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	if err := s.jukebox().Publish(in.Token, in.State); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) jukeboxNotice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token  string `json:"token"`
		Remote string `json:"remote"`
		Text   string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(in.Text)
	if utf8.RuneCountInString(text) > 300 {
		text = string([]rune(text)[:300])
	}
	if text == "" {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	if err := s.jukebox().Notice(in.Token, in.Remote, text); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) jukeboxCommand(w http.ResponseWriter, r *http.Request) {
	session, role, ok := requestRole(r)
	if !ok {
		http.Error(w, "prohibido", http.StatusForbidden)
		return
	}
	var in struct {
		Remote  string          `json:"remote"`
		Command string          `json:"command"`
		Args    json.RawMessage `json:"args"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	err := s.jukebox().Command(in.Remote, session, role, in.Command, in.Args)
	status := http.StatusAccepted
	switch {
	case err == nil:
	case errors.Is(err, jukebox.ErrUnknownCommand):
		status = http.StatusBadRequest
	case errors.Is(err, jukebox.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, jukebox.ErrUnknownRemote):
		status = http.StatusNotFound
	case errors.Is(err, jukebox.ErrNoSpeaker):
		status = http.StatusConflict
	case errors.Is(err, jukebox.ErrBusy):
		status = http.StatusServiceUnavailable
	default:
		fail(w, err)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}
	w.WriteHeader(status)
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
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	found, err := s.Store.SetSessionHost(id, *in.Host)
	if err != nil {
		fail(w, err)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	role := jukebox.Guest
	if *in.Host {
		role = jukebox.Host
	}
	s.jukebox().SetRole(id, role)
	w.WriteHeader(http.StatusNoContent)
}
