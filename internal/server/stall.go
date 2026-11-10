package server

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

// A browser opens at most six connections to a server, shared by all its
// tabs. A tab that stops reading an answer (frozen in the background, or
// closed mid-answer) keeps one busy until the write fails, which without a
// deadline is the operating system's TCP timeout: minutes. With six of them
// busy every page waits in the browser on "Cargando…" and the server sees
// nothing.
const (
	stallTimeout = 20 * time.Second // a write that makes no progress for this long gives up
	slowRequest  = 5 * time.Second  // a request still running after this is logged
	idleTimeout  = 2 * time.Minute  // keep-alive connections with no request are closed
)

// longLived tells the answers that last by design: the audio (a paused track
// stops reading while it is paused) and the live updates, which bound their
// own writes.
func longLived(path string) bool {
	return strings.HasPrefix(path, "/stream/") || path == "/api/jukebox/events" || path == "/api/jukebox/speaker"
}

// HTTPServer is the server for a listener of h. No WriteTimeout: it would cut
// every song and the live updates; guard bounds the other answers' writes.
func (s *Server) HTTPServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: idleTimeout}
}

func (s *Server) guard(next http.Handler) http.Handler {
	stall, slow := s.stall, s.slow
	if stall <= 0 {
		stall = stallTimeout
	}
	if slow <= 0 {
		slow = slowRequest
	}
	return guard(next, stall, slow)
}

// guard bounds every write of an answer that is not long-lived to stall
// without progress, and logs the requests that take longer than slow: once
// while they still run and once when they end.
func guard(next http.Handler, stall, slow time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if longLived(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		what := r.Method + " " + r.URL.RequestURI()
		start := time.Now()
		note := time.AfterFunc(slow, func() {
			log.Printf("pedido sin terminar tras %v: %s", slow, what)
		})
		defer func() {
			if !note.Stop() {
				log.Printf("pedido terminado tras %v: %s", time.Since(start).Round(time.Millisecond), what)
			}
		}()
		next.ServeHTTP(&boundedWriter{ResponseWriter: w, rc: http.NewResponseController(w), stall: stall}, r)
	})
}

// boundedWriter moves the connection's write deadline before every write, so
// a write fails once the client stopped reading for stall. The server clears
// the deadline when the answer is done, before the connection is reused.
type boundedWriter struct {
	http.ResponseWriter
	rc    *http.ResponseController
	stall time.Duration
}

func (b *boundedWriter) extend() {
	if err := b.rc.SetWriteDeadline(time.Now().Add(b.stall)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		log.Printf("respuesta: %v", err)
	}
}

func (b *boundedWriter) WriteHeader(code int) {
	b.extend()
	b.ResponseWriter.WriteHeader(code)
}

func (b *boundedWriter) Write(p []byte) (int, error) {
	b.extend()
	return b.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController (and http.ServeContent's checks) reach
// the real writer.
func (b *boundedWriter) Unwrap() http.ResponseWriter { return b.ResponseWriter }
