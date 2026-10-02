package store

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"tordo/internal/audiofile"
)

func trackNamed(t *testing.T, s *Store, title string) int64 {
	t.Helper()
	var id int64
	if err := s.db.QueryRow(`SELECT file_id FROM tracks WHERE title = ?`, title).Scan(&id); err != nil {
		t.Fatalf("no track %q: %v", title, err)
	}
	return id
}

func personal(t *testing.T, s *Store) Personal {
	t.Helper()
	p, err := s.Personal()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLikeAndRateTrack(t *testing.T) {
	s := library(t)
	id := trackNamed(t, s, "Signos")
	if ok, err := s.LikeTrack(id, true); !ok || err != nil {
		t.Fatalf("like = %v, %v", ok, err)
	}
	m := personal(t, s).Tracks[id]
	if !m.Liked || m.LikedAt <= 0 {
		t.Fatalf("mark = %+v", m)
	}
	first := m.LikedAt
	now = func() time.Time { return time.UnixMilli(first + 5000) }
	defer func() { now = time.Now }()
	s.LikeTrack(id, true)
	if got := personal(t, s).Tracks[id].LikedAt; got != first {
		t.Fatalf("liked again: LikedAt %d, want %d", got, first)
	}
	if ok, err := s.RateTrack(id, 4); !ok || err != nil {
		t.Fatalf("rate = %v, %v", ok, err)
	}
	if m := personal(t, s).Tracks[id]; m.Rating != 4 || !m.Liked {
		t.Fatalf("mark = %+v", m)
	}
	s.LikeTrack(id, false)
	m, in := personal(t, s).Tracks[id]
	if !in || m.Liked || m.Rating != 4 || m.LikedAt != 0 {
		t.Fatalf("unliked = %+v, in %v", m, in)
	}
	s.RateTrack(id, 0)
	if _, in := personal(t, s).Tracks[id]; in {
		t.Fatal("still marked")
	}
	if n := countRows(t, s, "marks"); n != 0 {
		t.Fatalf("%d marks left", n)
	}
	if _, err := s.RateTrack(id, 6); err != ErrBadRating {
		t.Fatalf("rating 6: %v", err)
	}
	if _, err := s.RateTrack(id, -1); err != ErrBadRating {
		t.Fatalf("rating -1: %v", err)
	}
	if ok, err := s.LikeTrack(99999, true); ok || err != nil {
		t.Fatalf("unknown = %v, %v", ok, err)
	}
	if ok, err := s.RateTrack(99999, 3); ok || err != nil {
		t.Fatalf("unknown rate = %v, %v", ok, err)
	}
}

func TestMarksFollowTheAudio(t *testing.T) {
	s := library(t)
	old := la + "/[LA-10] Soda Stereo - Signos/01.mp3"
	id := trackNamed(t, s, "Signos")
	s.LikeTrack(id, true)

	moved := la + "/[LA-10] Soda Stereo - Signos/renamed.mp3"
	var rows []FileRow
	rs, err := s.db.Query(`SELECT path FROM files WHERE kind = 'audio' AND path != ?`, old)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var p string
		rs.Scan(&p)
		rows = append(rows, audioRow(p, 10, 1))
	}
	rs.Close()
	rows = append(rows, audioRow(moved, 10, 1))
	if err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	env := ReadEnv{Version: 1}
	pending, _ := s.PendingReads(env)
	var res []ReadResult
	for _, p := range pending {
		if p.Path != moved {
			t.Fatalf("unexpected pending %q", p.Path)
		}
		res = append(res, ReadResult{Target: p, Env: env, Fingerprint: "fp" + old,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: audiofile.Tags{Title: "Signos", Artist: "Soda Stereo", Album: "Signos", Track: 1, Year: 1986}}})
	}
	if err := s.SaveReads(res); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	var newID int64
	if err := s.db.QueryRow(`SELECT id FROM files WHERE path = ?`, moved).Scan(&newID); err != nil {
		t.Fatal(err)
	}
	p := personal(t, s)
	if !p.Tracks[newID].Liked {
		t.Fatalf("the moved file is not liked: %+v", p.Tracks)
	}
	if _, in := p.Tracks[id]; in {
		t.Fatal("the missing file is still listed")
	}
}

func TestAlbumMarks(t *testing.T) {
	s := library(t)
	id := albumNamed(t, s, "Obras completas")
	if ok, err := s.LikeAlbum(id, true); !ok || err != nil {
		t.Fatalf("like = %v, %v", ok, err)
	}
	if ok, err := s.RateAlbum(id, 5); !ok || err != nil {
		t.Fatalf("rate = %v, %v", ok, err)
	}
	p := personal(t, s)
	if m := p.Albums[id]; !m.Liked || m.Rating != 5 || m.LikedAt <= 0 {
		t.Fatalf("album = %+v", m)
	}
	if len(p.Tracks) != 0 {
		t.Fatalf("tracks = %+v", p.Tracks)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM marks WHERE kind = 'album'`).Scan(&n)
	if n != 3 {
		t.Fatalf("%d album rows", n)
	}
	s.LikeAlbum(id, false)
	if m := personal(t, s).Albums[id]; m.Liked || m.Rating != 5 {
		t.Fatalf("unliked = %+v", m)
	}
	if ok, err := s.LikeAlbum(99999, true); ok || err != nil {
		t.Fatalf("unknown = %v, %v", ok, err)
	}
	if _, err := s.RateAlbum(id, 9); err != ErrBadRating {
		t.Fatalf("rating 9: %v", err)
	}
}

func TestArtistAndCollectionLikes(t *testing.T) {
	s := library(t)
	if err := s.LikeArtist("soda-stereo", true); err != nil {
		t.Fatal(err)
	}
	if err := s.LikeArtist("", true); err == nil {
		t.Fatal("empty key accepted")
	}
	if ok, err := s.LikeCollection(la, true); !ok || err != nil {
		t.Fatalf("collection = %v, %v", ok, err)
	}
	if ok, _ := s.LikeCollection("../musica/nope", true); ok {
		t.Fatal("liked a folder that does not exist")
	}
	p := personal(t, s)
	if p.Artists["soda-stereo"] <= 0 || p.Collections[la] <= 0 || len(p.Collections) != 1 {
		t.Fatalf("personal = %+v", p)
	}
	s.LikeArtist("soda-stereo", false)
	s.LikeCollection(la, false)
	p = personal(t, s)
	if len(p.Artists) != 0 || len(p.Collections) != 0 {
		t.Fatalf("personal = %+v", p)
	}
	if n := countRows(t, s, "marks"); n != 0 {
		t.Fatalf("%d marks left", n)
	}
}

func TestRecordPlay(t *testing.T) {
	s := library(t)
	signos, sur := trackNamed(t, s, "Signos"), trackNamed(t, s, "Sur")
	t1 := time.UnixMilli(1_700_000_000_000)
	t2 := t1.Add(time.Hour)
	for _, c := range []struct {
		id int64
		at time.Time
	}{{signos, t2}, {signos, t1}, {sur, t1}} {
		if ok, err := s.RecordPlay(c.id, c.at, 1000, "  Mac · Safari  "); !ok || err != nil {
			t.Fatalf("play = %v, %v", ok, err)
		}
	}
	p := personal(t, s).Plays
	if p[signos] != (Plays{Count: 2, Last: t2.UnixMilli()}) || p[sur] != (Plays{Count: 1, Last: t1.UnixMilli()}) {
		t.Fatalf("plays = %+v", p)
	}
	var dev string
	s.db.QueryRow(`SELECT device FROM plays LIMIT 1`).Scan(&dev)
	if dev != "Mac · Safari" {
		t.Fatalf("device = %q", dev)
	}
	long := strings.Repeat("é", 50) // 100 bytes
	s.RecordPlay(sur, t1, 1, long)
	s.db.QueryRow(`SELECT device FROM plays ORDER BY id DESC LIMIT 1`).Scan(&dev)
	if len(dev) > 64 || !utf8.ValidString(dev) || dev == "" {
		t.Fatalf("device = %q (%d bytes)", dev, len(dev))
	}
	if ok, err := s.RecordPlay(99999, t1, 1, ""); ok || err != nil {
		t.Fatalf("unknown = %v, %v", ok, err)
	}
}

func TestPersonalWritesKeepTheCatalog(t *testing.T) {
	s := library(t)
	id := trackNamed(t, s, "Signos")
	before, _ := s.Changes()
	v := s.PersonalVersion()
	s.LikeTrack(id, true)
	s.RateTrack(id, 3)
	s.LikeAlbum(albumNamed(t, s, "Obras completas"), true)
	s.RecordPlay(id, time.Now(), 1000, "")
	after, _ := s.Changes()
	if before != after {
		t.Fatalf("changes %d -> %d", before, after)
	}
	if got := s.PersonalVersion() - v; got != 4 {
		t.Fatalf("version grew by %d", got)
	}
}

func TestLikedTrackIDs(t *testing.T) {
	s := library(t)
	signos, sur := trackNamed(t, s, "Signos"), trackNamed(t, s, "Sur")
	at := int64(1_700_000_000_000)
	defer func() { now = time.Now }()
	now = func() time.Time { return time.UnixMilli(at) }
	s.LikeTrack(sur, true)
	now = func() time.Time { return time.UnixMilli(at + 10) }
	s.LikeTrack(signos, true)
	s.RateTrack(trackNamed(t, s, "Suelto"), 5) // rated, not liked
	got, err := s.LikedTrackIDs()
	if err != nil || !reflect.DeepEqual(got, []int64{signos, sur}) {
		t.Fatalf("ids = %v, err %v", got, err)
	}
}
