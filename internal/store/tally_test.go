package store

import (
	"testing"
	"time"

	"tordo/internal/audiofile"
)

func day(n int) time.Time { return time.Date(2026, 1, n, 12, 0, 0, 0, time.UTC) }

func recordPlays(t *testing.T, s *Store, id int64, days ...int) {
	t.Helper()
	for _, d := range days {
		if ok, err := s.RecordPlay(id, day(d), 1000, "test"); err != nil || !ok {
			t.Fatalf("RecordPlay: ok %v, err %v", ok, err)
		}
	}
}

func tally(t *testing.T, s *Store, since time.Time) PlayTally {
	t.Helper()
	p, err := s.PlayTally(since)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlayTallyByPeriod(t *testing.T) {
	s := library(t)
	signos, sur := trackNamed(t, s, "Signos"), trackNamed(t, s, "Sur")
	recordPlays(t, s, signos, 1, 9)
	recordPlays(t, s, sur, 9)

	p := tally(t, s, time.Time{})
	if p.Counts[signos] != 2 || p.Counts[sur] != 1 || len(p.Counts) != 2 || p.Plays != 3 || p.ListenedMs != 3000 {
		t.Fatalf("all: %+v", p)
	}
	p = tally(t, s, day(5))
	if p.Counts[signos] != 1 || p.Counts[sur] != 1 || p.Plays != 2 || p.ListenedMs != 2000 {
		t.Fatalf("since day 5: %+v", p)
	}
	p = tally(t, s, day(20))
	if p.Counts == nil || len(p.Counts) != 0 || p.Plays != 0 || p.ListenedMs != 0 {
		t.Fatalf("empty: %+v", p)
	}
}

func TestPlayTallyCopiesCountOnce(t *testing.T) {
	s := library(t)
	old := la + "/[LA-10] Soda Stereo - Signos/01.mp3"
	id := trackNamed(t, s, "Signos")

	copyPath := la + "/[LA-10] Soda Stereo - Signos/copy.mp3"
	var rows []FileRow
	rs, err := s.db.Query(`SELECT path FROM files WHERE kind = 'audio'`)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var p string
		rs.Scan(&p)
		rows = append(rows, audioRow(p, 10, 1))
	}
	rs.Close()
	rows = append(rows, audioRow(copyPath, 10, 1))
	if err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	env := ReadEnv{Version: 1}
	pending, _ := s.PendingReads(env)
	var res []ReadResult
	for _, p := range pending {
		if p.Path != copyPath {
			t.Fatalf("unexpected pending %q", p.Path)
		}
		res = append(res, ReadResult{Target: p, Env: env, Fingerprint: "fp" + old,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: audiofile.Tags{Title: "Signos", Artist: "Soda Stereo", Album: "Signos", Track: 1, Year: 1986}}})
	}
	if err := s.SaveReads(res); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	var copyID int64
	if err := s.db.QueryRow(`SELECT id FROM files WHERE path = ?`, copyPath).Scan(&copyID); err != nil || copyID == id {
		t.Fatalf("copy id %d (original %d), err %v", copyID, id, err)
	}

	recordPlays(t, s, id, 1, 9)
	p := tally(t, s, time.Time{})
	if p.Counts[id] != 2 || p.Counts[copyID] != 0 || p.Plays != 2 {
		t.Fatalf("both present: %+v", p)
	}

	if _, err := s.db.Exec(`UPDATE files SET missing = 1 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	p = tally(t, s, time.Time{})
	if p.Counts[copyID] != 2 || p.Counts[id] != 0 || p.Plays != 2 || p.ListenedMs != 2000 {
		t.Fatalf("original missing: %+v", p)
	}

	if _, err := s.db.Exec(`UPDATE files SET missing = 1 WHERE id = ?`, copyID); err != nil {
		t.Fatal(err)
	}
	p = tally(t, s, time.Time{})
	if len(p.Counts) != 0 || p.Plays != 2 || p.ListenedMs != 2000 {
		t.Fatalf("both missing: %+v", p)
	}
}
