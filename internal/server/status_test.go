package server

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"tordo/internal/store"
)

type statusAnswer struct {
	Preparing bool          `json:"preparing"`
	Counts    *store.Counts `json:"counts"`
}

// statusWithin asks for the status and fails if it takes long: it is what
// shows the progress, so it must not wait for the store.
func statusWithin(t *testing.T, s *Server) statusAnswer {
	t.Helper()
	got := make(chan statusAnswer, 1)
	go func() {
		var a statusAnswer
		decode(t, do(t, s.Handler(), "GET", "/api/status", ""), &a)
		got <- a
	}()
	select {
	case a := <-got:
		return a
	case <-time.After(5 * time.Second):
		t.Fatal("el estado esperó al catálogo")
		return statusAnswer{}
	}
}

func TestStatusDoesNotWaitForTheStore(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var blocked atomic.Bool
	s := &Server{Store: newStore(t)}
	s.countStore = func() (store.Counts, error) {
		if blocked.Load() {
			<-release
		}
		return store.Counts{Albums: 7}, nil
	}

	if a := statusWithin(t, s); a.Counts == nil || a.Counts.Albums != 7 {
		t.Fatalf("a free store answers its counts: %+v", a)
	}
	blocked.Store(true)
	if a := statusWithin(t, s); a.Counts == nil || a.Counts.Albums != 7 {
		t.Fatalf("a busy store answers the last counts: %+v", a)
	}
	if a := statusWithin(t, s); a.Counts == nil || a.Counts.Albums != 7 {
		t.Fatalf("still busy, the same: %+v", a)
	}
}

func TestStatusWithoutCountsYet(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	s := &Server{Store: newStore(t)}
	s.countStore = func() (store.Counts, error) {
		<-release
		return store.Counts{}, nil
	}
	if a := statusWithin(t, s); a.Counts != nil {
		t.Fatalf("no counts known yet: %+v", a)
	}
}

func TestStatusCountsErrorIsReported(t *testing.T) {
	s := &Server{Store: newStore(t)}
	s.countStore = func() (store.Counts, error) { return store.Counts{}, errors.New("base rota") }
	if rec := do(t, s.Handler(), "GET", "/api/status", ""); rec.Code != 500 {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPrepareReportsTheFirstCatalog(t *testing.T) {
	s := &Server{Store: newStore(t)}
	if a := statusWithin(t, s); a.Preparing {
		t.Fatal("nothing is being prepared before Prepare")
	}

	s.cat.build.Lock() // the first build cannot end yet
	s.Prepare()
	if a := statusWithin(t, s); !a.Preparing {
		t.Fatal("the status has to tell the catalog is being prepared")
	}
	if c := s.cat.current(); c != nil {
		t.Fatal("no catalog yet")
	}
	s.cat.build.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	for statusWithin(t, s).Preparing {
		if time.Now().After(deadline) {
			t.Fatal("still preparing")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if s.cat.current() == nil {
		t.Fatal("Prepare builds the catalog")
	}
}
