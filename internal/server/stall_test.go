package server

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// stalledClient sends a GET for path and never reads the answer, as a frozen
// background tab does. It returns a function that closes the connection.
func stalledClient(t *testing.T, srv *httptest.Server, path string) func() {
	t.Helper()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	// A small receive buffer, so the server's writes block soon.
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetReadBuffer(4096)
	}
	fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n", path)
	return func() { conn.Close() }
}

// bigWriter writes until a write fails and reports the error.
func bigWriter(done chan<- error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chunk := bytes.Repeat([]byte("x"), 64<<10)
		for i := 0; i < 4096; i++ { // 256 MB: far more than any socket buffer
			if _, err := w.Write(chunk); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}
}

// A client that stops reading gives up its connection: the write fails once
// it made no progress for the stall time, instead of after Windows' TCP
// timeout (minutes), while the browser keeps one of its six connections busy.
func TestStalledAnswerIsCutOff(t *testing.T) {
	done := make(chan error, 1)
	srv := httptest.NewServer(guard(bigWriter(done), 200*time.Millisecond, time.Hour))
	defer srv.Close()
	defer stalledClient(t, srv, "/api/explore")()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the whole answer was written to a client that does not read")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the write is still blocked")
	}
}

// Audio and the live updates are long by design: a paused track stops
// reading for as long as it is paused.
func TestLongLivedAnswersAreNotCutOff(t *testing.T) {
	for _, path := range []string{"/stream/7", "/api/jukebox/events", "/api/jukebox/speaker"} {
		done := make(chan error, 1)
		srv := httptest.NewServer(guard(bigWriter(done), 100*time.Millisecond, time.Hour))
		closeClient := stalledClient(t, srv, path)
		select {
		case err := <-done:
			t.Fatalf("%s: cut off: %v", path, err)
		case <-time.After(600 * time.Millisecond):
		}
		closeClient()
		srv.Close()
	}
}

// syncBuffer collects the log from several goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func captureLog(t *testing.T) *syncBuffer {
	t.Helper()
	var buf syncBuffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// A request that takes long says so in the terminal while it is still
// running, and again when it ends: a page stuck on "Cargando…" leaves a trace.
func TestSlowRequestsAreLogged(t *testing.T) {
	buf := captureLog(t)
	release := make(chan struct{})
	h := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusNoContent)
	}), time.Hour, 50*time.Millisecond)
	srv := httptest.NewServer(h)
	defer srv.Close()
	res := make(chan error, 1)
	go func() {
		r, err := http.Get(srv.URL + "/api/explore?ver=albumes")
		if err == nil {
			r.Body.Close()
		}
		res <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(buf.String(), "GET /api/explore?ver=albumes") {
		if time.Now().After(deadline) {
			t.Fatalf("no note while running; log: %q", buf.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	if err := <-res; err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "GET /api/explore?ver=albumes"); n != 2 {
		t.Fatalf("want a note while running and one at the end; log: %q", buf.String())
	}
}

func TestQuickRequestsAreNotLogged(t *testing.T) {
	buf := captureLog(t)
	srv := httptest.NewServer(guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), time.Hour, time.Second))
	defer srv.Close()
	r, err := http.Get(srv.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	time.Sleep(50 * time.Millisecond)
	if buf.String() != "" {
		t.Fatalf("log: %q", buf.String())
	}
}

// The listeners close idle connections and slow headers.
func TestHTTPServerHasTimeouts(t *testing.T) {
	s := &Server{Store: newStore(t)}
	hs := s.HTTPServer(http.NotFoundHandler())
	if hs.ReadHeaderTimeout <= 0 || hs.IdleTimeout <= 0 {
		t.Fatalf("timeouts: header %v, idle %v", hs.ReadHeaderTimeout, hs.IdleTimeout)
	}
	if hs.WriteTimeout != 0 {
		t.Fatal("a WriteTimeout would cut every song and the live updates")
	}
}
