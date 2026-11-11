package server

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"tordo/internal/catalog"
	"tordo/internal/startcache"
	"tordo/internal/store"
)

// ghostCache writes a startup cache with one album the store does not have.
func ghostCache(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "cache", "catalogo.gob.gz")
	err := startcache.Save(file, startcache.Data{
		Snapshot: store.Snapshot{
			Albums:        []store.AlbumView{{ID: 77, Title: "Fantasma", Artist: "Nadie", Tracks: 1}},
			Artists:       map[string]store.Artist{},
			ReleaseGroups: map[string]string{},
		},
		Personal: store.Personal{Tracks: map[int64]store.Mark{}, Albums: map[int64]store.Mark{77: {Liked: true, LikedAt: 1}},
			Artists: map[string]int64{}, Collections: map[string]int64{}, Plays: map[int64]store.Plays{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// prepare starts Prepare and, when the test ends, waits for it and for the
// startup cache writes it started: they write into the test's folder.
func prepare(t *testing.T, s *Server) <-chan struct{} {
	t.Helper()
	ready := s.Prepare()
	t.Cleanup(func() {
		<-ready
		for {
			s.cat.saveMu.Lock()
			saving := s.cat.saving
			s.cat.saveMu.Unlock()
			if !saving {
				return
			}
			time.Sleep(time.Millisecond)
		}
	})
	return ready
}

func albumTitles(t *testing.T, s *Server) []string {
	t.Helper()
	var r catalog.Result
	decode(t, do(t, s.Handler(), "GET", "/api/explore", ""), &r)
	var out []string
	for _, a := range r.Albums {
		out = append(out, a.Title)
	}
	return out
}

func waitPrepared(t *testing.T, s *Server) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for statusWithin(t, s).Preparing {
		if time.Now().After(deadline) {
			t.Fatal("still preparing")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPrepareAnswersFromTheStartupCacheUntilTheStoreIsRead(t *testing.T) {
	s := &Server{Store: newStore(t), CatalogCache: ghostCache(t)}
	s.cat.build.Lock() // reading the store cannot end yet
	ready := prepare(t, s)
	waitPrepared(t, s) // the cache is enough for the pages

	if got := albumTitles(t, s); len(got) != 1 || got[0] != "Fantasma" {
		t.Fatalf("while the store is read, the cached catalog: %v", got)
	}
	var marks struct {
		Albums map[int64]store.Mark `json:"albums"`
	}
	decode(t, do(t, s.Handler(), "GET", "/api/marks", ""), &marks)
	if !marks.Albums[77].Liked {
		t.Fatalf("and the cached marks: %+v", marks)
	}
	select {
	case <-ready:
		t.Fatal("the scan still waits for the store's catalog")
	default:
	}

	s.cat.build.Unlock()
	<-ready
	if got := albumTitles(t, s); len(got) != 1 || got[0] != "Locura" {
		t.Fatalf("then the store's catalog: %v", got)
	}
	marks.Albums = nil
	decode(t, do(t, s.Handler(), "GET", "/api/marks", ""), &marks)
	if len(marks.Albums) != 0 {
		t.Fatalf("and the store's marks: %+v", marks)
	}
}

func TestPrepareWritesTheStartupCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache", "catalogo.gob.gz")
	s := &Server{Store: newStore(t), CatalogCache: file}
	<-prepare(t, s)
	d := waitCache(t, file, func(d startcache.Data) bool { return true })
	if len(d.Snapshot.Albums) != 1 || d.Snapshot.Albums[0].Title != "Locura" || d.Personal.Albums == nil {
		t.Fatalf("cached %+v", d)
	}
}

func TestTheStartupCacheFollowsLaterBuilds(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	st := newStore(t)
	s := &Server{Store: st, CatalogCache: file, cacheEvery: time.Millisecond}
	<-prepare(t, s)
	before, _ := st.Changes()
	if _, err := st.SyncFiles([]store.FileRow{
		{Path: "../musica/Virus - Locura/01.mp3", Size: 1, MTime: 1, Kind: "audio"},
		{Path: "../musica/Virus - Locura/02.mp3", Size: 1, MTime: 1, Kind: "audio"},
	}, []string{"../musica"}); err != nil {
		t.Fatal(err)
	}
	now, _ := st.Changes()
	if now == before {
		t.Fatal("the store did not change")
	}
	albumTitles(t, s) // a page rebuilds the catalog
	waitCache(t, file, func(d startcache.Data) bool { return d.Snapshot.Changes == now })
}

// waitCache waits until the startup cache is written and ok accepts it.
func waitCache(t *testing.T, file string, ok func(startcache.Data) bool) startcache.Data {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		d, err := startcache.Load(file)
		if err == nil && ok(d) {
			return d
		}
		if time.Now().After(deadline) {
			t.Fatalf("the startup cache was not written: %+v, %v", d.Snapshot, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPrepareWithABrokenStartupCacheReadsTheStore(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	os.WriteFile(file, []byte("roto"), 0o644)
	s := &Server{Store: newStore(t), CatalogCache: file}
	s.cat.build.Lock()
	prepare(t, s)
	if !statusWithin(t, s).Preparing {
		t.Fatal("without a usable cache the pages wait for the store")
	}
	s.cat.build.Unlock()
	waitPrepared(t, s)
	if got := albumTitles(t, s); len(got) != 1 || got[0] != "Locura" {
		t.Fatalf("albums %v", got)
	}
}

func TestReadOnlyDoesNotWriteTheStartupCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	s := &Server{Store: newStore(t), CatalogCache: file, ReadOnly: true}
	<-prepare(t, s)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("browse-only mode writes nothing: %v", err)
	}
}

func TestTheFirstPagesWaitForTheStartupCacheNotForTheStore(t *testing.T) {
	s := &Server{Store: newStore(t), CatalogCache: ghostCache(t)}
	s.cat.build.Lock() // reading the store cannot end
	defer s.cat.build.Unlock()
	prepare(t, s)
	got := make(chan []string, 1)
	go func() { got <- albumTitles(t, s) }() // at once: the cache may not be loaded yet
	select {
	case titles := <-got:
		if len(titles) != 1 || titles[0] != "Fantasma" {
			t.Fatalf("albums %v", titles)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the first page waited for the store")
	}
}

func TestTheStatusAnswersTheCachedCountsUntilTheStoreIsRead(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	if err := startcache.Save(file, startcache.Data{Counts: store.Counts{Albums: 77}}); err != nil {
		t.Fatal(err)
	}
	var counted atomic.Bool
	s := &Server{Store: newStore(t), CatalogCache: file}
	s.countStore = func() (store.Counts, error) {
		counted.Store(true)
		return store.Counts{Albums: 1}, nil
	}
	s.cat.build.Lock()
	ready := prepare(t, s)
	waitPrepared(t, s)
	if a := statusWithin(t, s); a.Counts == nil || a.Counts.Albums != 77 {
		t.Fatalf("the cached counts: %+v", a.Counts)
	}
	if counted.Load() {
		t.Fatal("counting reads the whole store: not before it is read")
	}
	s.cat.build.Unlock()
	<-ready
	if a := statusWithin(t, s); a.Counts == nil || a.Counts.Albums != 1 {
		t.Fatalf("then the store's: %+v", a.Counts)
	}
}

func TestTheStartupCacheKeepsTheCounts(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	s := &Server{Store: newStore(t), CatalogCache: file}
	<-prepare(t, s)
	waitCache(t, file, func(d startcache.Data) bool { return d.Counts.Albums == 1 && d.Counts.Tracks == 1 })
}
