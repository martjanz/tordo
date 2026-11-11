package startcache

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"tordo/internal/store"
)

func sample() Data {
	return Data{
		Snapshot: store.Snapshot{
			Albums:        []store.AlbumView{{ID: 3, Title: "Locura", Artist: "Virus", Genres: []store.GenreLink{{Genre: "rock"}}}},
			Tracks:        []store.SnapTrack{{TrackView: store.TrackView{ID: 9, Title: "Una luna de miel en la mano"}, AlbumID: 3, Dir: "../musica/Virus - Locura", Added: 5}},
			Folders:       []store.SnapFolder{{FolderSummary: store.FolderSummary{Path: "../musica", Name: "musica"}}},
			Artists:       map[string]store.Artist{"mb1": {Name: "Virus"}},
			ReleaseGroups: map[string]string{"r1": "g1"},
			Changes:       42,
		},
		Personal: store.Personal{
			Tracks:      map[int64]store.Mark{9: {Liked: true, LikedAt: 7}},
			Albums:      map[int64]store.Mark{3: {Rating: 4}},
			Artists:     map[string]int64{"mb1": 8},
			Collections: map[string]int64{},
			Plays:       map[int64]store.Plays{9: {Count: 2, Last: 6}},
		},
		Counts: store.Counts{Files: 10, Tracks: 1, Albums: 1},
	}
}

func TestSaveThenLoadGivesTheSameData(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cache", "catalogo.gob.gz")
	want := sample()
	if err := Save(file, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSaveLeavesOnlyTheCacheFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "catalogo.gob.gz")
	for range 2 {
		if err := Save(file, sample()); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "catalogo.gob.gz" {
		t.Fatalf("files left: %v", entries)
	}
}

func TestLoadWithoutACacheIsNotExist(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nada.gob.gz")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsAnUnreadableCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	os.WriteFile(file, []byte("no es un catálogo"), 0o644)
	if _, err := Load(file); err == nil {
		t.Fatal("a broken cache must not load")
	}
}

func TestLoadRejectsAnotherFormatVersion(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	if err := save(file, version+1, sample()); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil {
		t.Fatal("a cache of another version must not load")
	}
}

// Windows does not replace a file someone has open: Save waits a moment
// for the reader instead of losing the write.
func TestSaveWaitsForAReaderOfTheCache(t *testing.T) {
	file := filepath.Join(t.TempDir(), "catalogo.gob.gz")
	if err := Save(file, Data{}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		f.Close()
	}()
	if err := Save(file, sample()); err != nil {
		t.Fatal(err)
	}
	if d, err := Load(file); err != nil || d.Snapshot.Changes != 42 {
		t.Fatalf("Load = %+v, %v", d.Snapshot, err)
	}
}
