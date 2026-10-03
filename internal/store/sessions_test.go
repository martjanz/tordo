package store

import (
	"strings"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	s := open(t)
	clock := time.UnixMilli(1_000_000)
	now = func() time.Time { return clock }
	defer func() { now = time.Now }()

	a, err := s.CreateSession("hash-a", "Teléfono")
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	b, err := s.CreateSession("hash-b", "Notebook")
	if err != nil {
		t.Fatal(err)
	}
	for hash, want := range map[string]string{"hash-a": "Teléfono", "hash-b": "Notebook"} {
		se, ok, err := s.SessionByToken(hash)
		if err != nil || !ok || se.Device != want || se.CreatedAt != se.LastSeen || se.CreatedAt <= 0 {
			t.Fatalf("%s: %+v %v %v", hash, se, ok, err)
		}
	}
	if _, ok, err := s.SessionByToken("nope"); ok || err != nil {
		t.Fatalf("unknown hash: %v %v", ok, err)
	}
	list, _ := s.Sessions()
	if len(list) != 2 || list[0].ID != b {
		t.Fatalf("newest not first: %+v", list)
	}

	clock = clock.Add(time.Second)
	if err := s.TouchSession(a); err != nil {
		t.Fatal(err)
	}
	se, _, _ := s.SessionByToken("hash-a")
	if se.LastSeen <= se.CreatedAt {
		t.Fatalf("last seen did not move: %+v", se)
	}
	list, _ = s.Sessions()
	if list[0].ID != a {
		t.Fatalf("touched not first: %+v", list)
	}

	if _, err := s.CreateSession("hash-a", "Otro"); err == nil {
		t.Fatal("a repeated hash was accepted")
	}

	ok, err := s.DeleteSession(a)
	if !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if _, found, _ := s.SessionByToken("hash-a"); found {
		t.Fatal("deleted session still found")
	}
	if ok, err := s.DeleteSession(a); ok || err != nil {
		t.Fatalf("second delete: %v %v", ok, err)
	}

	if err := s.DeleteSessions(); err != nil {
		t.Fatal(err)
	}
	list, err = s.Sessions()
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("%#v %v", list, err)
	}
}

func TestSessionDevice(t *testing.T) {
	s := open(t)
	for i, c := range []struct{ device, want string }{
		{"  ", "Dispositivo"},
		{strings.Repeat("ñ", 80), strings.Repeat("ñ", 60)},
	} {
		id, err := s.CreateSession(string(rune('a'+i)), c.device)
		if err != nil {
			t.Fatal(err)
		}
		list, _ := s.Sessions()
		for _, se := range list {
			if se.ID == id && se.Device != c.want {
				t.Fatalf("device %q, want %q", se.Device, c.want)
			}
		}
	}
}

func TestPruneSessions(t *testing.T) {
	s := open(t)
	day := func(n int) time.Time { return time.UnixMilli(0).Add(time.Duration(n) * 24 * time.Hour) }
	defer func() { now = time.Now }()
	now = func() time.Time { return day(1) }
	s.CreateSession("old", "Viejo")
	now = func() time.Time { return day(100) }
	s.CreateSession("new", "Nuevo")

	n, err := s.PruneSessions(day(10))
	if err != nil || n != 1 {
		t.Fatalf("pruned %d, %v", n, err)
	}
	if _, ok, _ := s.SessionByToken("old"); ok {
		t.Fatal("old session kept")
	}
	if _, ok, _ := s.SessionByToken("new"); !ok {
		t.Fatal("new session pruned")
	}
}

func TestSessionWritesAreSide(t *testing.T) {
	s := sample(t)
	changes, _ := s.Changes()
	personal := s.PersonalVersion()
	id, err := s.CreateSession("h", "Teléfono")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TouchSession(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteSession(id); err != nil {
		t.Fatal(err)
	}
	after, _ := s.Changes()
	if after != changes || s.PersonalVersion() != personal {
		t.Fatalf("changes %d -> %d, personal %d -> %d", changes, after, personal, s.PersonalVersion())
	}
}
