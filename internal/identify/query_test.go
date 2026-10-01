package identify

import (
	"reflect"
	"testing"

	"tordo/internal/store"
)

const validMBID = "a1b2c3d4-0000-4000-8000-000000000001"

func TestBuildQueryPlainAlbum(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Soda Stereo/Soda Stereo - (1986) Signos", Title: "Signos",
		Artist: "Soda Stereo", Year: 1986, Tracks: []store.TargetTrack{{DurationMs: 231500}, {DurationMs: 209000}}})
	want := Query{Variants: []Variant{{Title: "Signos", Artist: "Soda Stereo"}}, Year: 1986, Durations: []int64{231500, 209000}}
	if !reflect.DeepEqual(q, want) {
		t.Fatalf("query = %+v", q)
	}
}

func TestBuildQueryFolderVariant(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Fito Páez/Fito Páez - (2004) Mi vida con ellas",
		Title: "FITO PAEZ - 2004 - mi vida con ellas 1", Artist: "Fito Páez"})
	want := []Variant{{Title: "FITO PAEZ - 2004 - mi vida con ellas 1", Artist: "Fito Páez"}, {Title: "Mi vida con ellas", Artist: "Fito Páez"}}
	if !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryStripsNotes(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Soda Stereo/Signos remaster", Title: "Signos (Remastered 2007)", Artist: "Soda Stereo"})
	want := []Variant{{Title: "Signos (Remastered 2007)", Artist: "Soda Stereo"}, {Title: "Signos remaster", Artist: "Soda Stereo"},
		{Title: "Signos", Artist: "Soda Stereo"}}
	if !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryCompilation(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Brasil/VA - História da MPB", Title: "História da MPB",
		Artist: "Varios artistas", Compilation: true})
	if want := []Variant{{Title: "História da MPB", Various: true}}; !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryComposesAndDropsJunk(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/x/Impe\u0301rio", Title: "Impe\u0301rio", Artist: "www.lomusical.com"})
	if want := []Variant{{Title: "Império"}}; !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryTagReleaseID(t *testing.T) {
	tracks := []store.TargetTrack{{MBAlbumID: validMBID}, {MBAlbumID: validMBID}, {MBAlbumID: "basura"}}
	if q := BuildQuery(store.IdentifyTarget{Dir: "../musica/a/b", Title: "b", Tracks: tracks}); q.ReleaseID != validMBID {
		t.Fatalf("release id = %q", q.ReleaseID)
	}
	tracks[1].MBAlbumID = ""
	if q := BuildQuery(store.IdentifyTarget{Dir: "../musica/a/b", Title: "b", Tracks: tracks}); q.ReleaseID != "" {
		t.Fatalf("a minority id was used: %q", q.ReleaseID)
	}
}

func TestQueryKey(t *testing.T) {
	base := store.IdentifyTarget{Dir: "../musica/Soda Stereo/Signos", Title: "Signos", Artist: "Soda Stereo", Year: 1986,
		Tracks: []store.TargetTrack{{DurationMs: 231500}, {DurationMs: 209000}}}
	moved := base
	moved.Dir = "../musica/Ordenar/Signos"
	if BuildQuery(base).Key() != BuildQuery(moved).Key() {
		t.Fatal("moving the folder changed the key")
	}
	changed := base
	changed.Tracks = []store.TargetTrack{{DurationMs: 231500}, {DurationMs: 250000}}
	if BuildQuery(base).Key() == BuildQuery(changed).Key() {
		t.Fatal("other durations kept the key")
	}
}
