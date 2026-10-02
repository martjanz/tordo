package store

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sameJSON(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(got, &a); err != nil {
		t.Fatalf("def %s: %v", got, err)
	}
	json.Unmarshal([]byte(want), &b)
	ga, _ := json.Marshal(a)
	gb, _ := json.Marshal(b)
	if string(ga) != string(gb) {
		t.Fatalf("def %s, want %s", got, want)
	}
}

func TestStationLifecycle(t *testing.T) {
	s := open(t)
	clock := time.UnixMilli(1_000_000)
	now = func() time.Time { return clock }
	defer func() { now = time.Now }()

	a, err := s.CreateStation("  Brasil  ", []byte(`{"mode":"pistas"}`))
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.Stations()
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	if list[0].Name != "Brasil" || list[0].CreatedAt <= 0 || list[0].CreatedAt != list[0].UpdatedAt {
		t.Fatalf("%+v", list[0])
	}
	sameJSON(t, list[0].Def, `{"mode":"pistas"}`)

	clock = clock.Add(time.Second)
	b, _ := s.CreateStation("Jazz", []byte(`{}`))
	list, _ = s.Stations()
	if len(list) != 2 || list[0].ID != b {
		t.Fatalf("second not first: %+v", list)
	}

	clock = clock.Add(time.Second)
	ok, err := s.UpdateStation(a, "Brasil 2", []byte(`{"mode":"albumes"}`))
	if !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	list, _ = s.Stations()
	if list[0].ID != a || list[0].Name != "Brasil 2" || list[0].UpdatedAt <= list[0].CreatedAt {
		t.Fatalf("%+v", list[0])
	}
	got, found, err := s.Station(a)
	if !found || err != nil || got.Name != "Brasil 2" {
		t.Fatalf("%+v %v %v", got, found, err)
	}
	sameJSON(t, got.Def, `{"mode":"albumes"}`)

	ok, err = s.DeleteStation(a)
	if !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if _, found, _ := s.Station(a); found {
		t.Fatal("deleted station still there")
	}
	list, _ = s.Stations()
	if len(list) != 1 || list[0].ID != b {
		t.Fatalf("%+v", list)
	}
	if ok, err := s.UpdateStation(999, "x", []byte(`{}`)); ok || err != nil {
		t.Fatalf("update unknown: %v %v", ok, err)
	}
	if ok, err := s.DeleteStation(999); ok || err != nil {
		t.Fatalf("delete unknown: %v %v", ok, err)
	}
}

func TestStationsEmptyListIsNotNil(t *testing.T) {
	list, err := open(t).Stations()
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("%#v %v", list, err)
	}
}

func TestStationValidation(t *testing.T) {
	s := open(t)
	if _, err := s.CreateStation("   ", []byte(`{}`)); err != ErrStationName {
		t.Fatalf("create: %v", err)
	}
	id, _ := s.CreateStation("Ok", []byte(`{}`))
	if _, err := s.UpdateStation(id, "", []byte(`{}`)); err != ErrStationName {
		t.Fatalf("update: %v", err)
	}
	if _, err := s.CreateStation("Mala", []byte(`{`)); err == nil {
		t.Fatal("invalid def accepted on create")
	}
	if _, err := s.UpdateStation(id, "Otra", []byte(`{`)); err == nil {
		t.Fatal("invalid def accepted on update")
	}
	list, _ := s.Stations()
	if len(list) != 1 || list[0].Name != "Ok" {
		t.Fatalf("something was written: %+v", list)
	}
	long, _ := s.CreateStation(strings.Repeat("é", 300), []byte(`{}`))
	got, _, _ := s.Station(long)
	if n := len([]rune(got.Name)); n != 200 {
		t.Fatalf("name has %d runes", n)
	}
}

func TestStationWritesKeepTheCatalog(t *testing.T) {
	s := sample(t)
	before, _ := s.Changes()
	v := s.PersonalVersion()
	id, _ := s.CreateStation("A", []byte(`{}`))
	s.UpdateStation(id, "B", []byte(`{}`))
	s.DeleteStation(id)
	after, _ := s.Changes()
	if before != after {
		t.Fatalf("changes %d -> %d", before, after)
	}
	if got := s.PersonalVersion() - v; got != 3 {
		t.Fatalf("version grew by %d", got)
	}
}
