package store

import (
	"testing"

	"tordo/internal/audiofile"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var root = []string{"../musica"}

func audioRow(p string, size, mtime int64) FileRow {
	return FileRow{Path: p, Size: size, MTime: mtime, Kind: "audio"}
}

func save(t *testing.T, s *Store, env ReadEnv, fp string, info audiofile.Info) {
	t.Helper()
	pending, err := s.PendingReads(env)
	if err != nil {
		t.Fatal(err)
	}
	var rs []ReadResult
	for _, p := range pending {
		rs = append(rs, ReadResult{Target: p, Info: info, Fingerprint: fp + p.Path, Env: env})
	}
	if err := s.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
}

func TestPendingReadsFollowChanges(t *testing.T) {
	s := open(t)
	env := ReadEnv{Version: 1}
	if err := s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../musica/b.mp3", 20, 1), {Path: "../musica/cover.jpg", Size: 5, MTime: 1, Kind: "image"}}, root); err != nil {
		t.Fatal(err)
	}
	pending, _ := s.PendingReads(env)
	if len(pending) != 2 {
		t.Fatalf("pending = %+v, want the 2 audio files", pending)
	}
	save(t, s, env, "fp-", audiofile.Info{Format: "mp3", DurationMs: 1000})
	if pending, _ := s.PendingReads(env); len(pending) != 0 {
		t.Fatalf("pending after saving = %+v", pending)
	}
	// b.mp3 changes: read again; a newer reader version: read everything again.
	if err := s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../musica/b.mp3", 21, 2)}, root); err != nil {
		t.Fatal(err)
	}
	if pending, _ := s.PendingReads(env); len(pending) != 1 || pending[0].Path != "../musica/b.mp3" {
		t.Fatalf("pending = %+v, want b.mp3", pending)
	}
	if pending, _ := s.PendingReads(ReadEnv{Version: 2}); len(pending) != 2 {
		t.Fatalf("pending with a new version = %+v", pending)
	}
}

func TestFailedReadsRetryWhenFFprobeAppears(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{audioRow("../musica/x.ape", 10, 1)}, root)
	pending, _ := s.PendingReads(ReadEnv{Version: 1})
	if err := s.SaveReads([]ReadResult{{Target: pending[0], Err: "formato sin lector propio", Env: ReadEnv{Version: 1}}}); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.PendingReads(ReadEnv{Version: 1}); len(p) != 0 {
		t.Fatalf("a failure is not retried without ffprobe: %+v", p)
	}
	if p, _ := s.PendingReads(ReadEnv{Version: 1, FFprobe: true}); len(p) != 1 {
		t.Fatalf("a failure is retried once ffprobe exists: %+v", p)
	}
	c, _ := s.Counts()
	if c.Unreadable != 1 || c.Audio != 1 {
		t.Fatalf("counts = %+v", c)
	}
}

func TestMissingFiles(t *testing.T) {
	s := open(t)
	env := ReadEnv{Version: 1}
	s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../otra/b.mp3", 10, 1)}, []string{"../musica", "../otra"})
	save(t, s, env, "", audiofile.Info{})
	// ../otra was not scanned (unavailable): b stays. a disappeared from ../musica.
	if err := s.SyncFiles(nil, []string{"../musica"}); err != nil {
		t.Fatal(err)
	}
	tracks, _, _ := s.GroupingInput()
	if len(tracks) != 1 || tracks[0].Path != "../otra/b.mp3" {
		t.Fatalf("tracks = %+v", tracks)
	}
	// ../otra removed from the configuration: its files go missing too.
	if err := s.MarkOutsideRoots([]string{"../musica"}); err != nil {
		t.Fatal(err)
	}
	if tracks, _, _ := s.GroupingInput(); len(tracks) != 0 {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func TestGroupingInputCarriesTags(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), {Path: "../musica/folder.jpg", Size: 7, MTime: 1, Kind: "image"}}, root)
	save(t, s, ReadEnv{Version: 1}, "", audiofile.Info{DurationMs: 1234, Tags: audiofile.Tags{Title: "Fue amor", Genres: []string{"Rock"}}})
	tracks, images, err := s.GroupingInput()
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Tags.Title != "Fue amor" || tracks[0].DurationMs != 1234 || tracks[0].Tags.Genres[0] != "Rock" {
		t.Fatalf("tracks = %+v", tracks)
	}
	if len(images) != 1 || images[0].Path != "../musica/folder.jpg" || images[0].Size != 7 {
		t.Fatalf("images = %+v", images)
	}
}

func TestMovedFileKeepsFirstSeen(t *testing.T) {
	s := open(t)
	env := ReadEnv{Version: 1}
	s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1)}, root)
	p, _ := s.PendingReads(env)
	s.SaveReads([]ReadResult{{Target: p[0], Fingerprint: "same", Env: env}})
	var before int64
	s.db.QueryRow(`SELECT first_seen FROM files WHERE path = '../musica/a.mp3'`).Scan(&before)
	s.db.Exec(`UPDATE files SET first_seen = 1000 WHERE path = '../musica/a.mp3'`)

	s.SyncFiles([]FileRow{audioRow("../musica/moved/a.mp3", 10, 1)}, root)
	p, _ = s.PendingReads(env)
	s.SaveReads([]ReadResult{{Target: p[0], Fingerprint: "same", Env: env}})
	var after int64
	s.db.QueryRow(`SELECT first_seen FROM files WHERE path = '../musica/moved/a.mp3'`).Scan(&after)
	if after != 1000 {
		t.Fatalf("first_seen = %d, want 1000 (inherited)", after)
	}
}
