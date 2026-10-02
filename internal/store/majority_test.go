package store

import (
	"strconv"
	"testing"
	"time"

	"tordo/internal/audiofile"
)

// addCompilation adds, next to library(t), a folder holding a bit-identical
// copy of "Signos" (same fingerprint) and two tracks by other artists: it
// groups as one compilation of 3 tracks.
func addCompilation(t *testing.T, s *Store) {
	t.Helper()
	comp := "../musica/Compilados/Rock"
	tags := map[string]audiofile.Tags{
		comp + "/01.mp3": {Title: "Signos", Artist: "Soda Stereo", Album: "Rock nacional", Track: 1},
		comp + "/02.mp3": {Title: "Locura", Artist: "Virus", Album: "Rock nacional", Track: 2},
		comp + "/03.mp3": {Title: "Sur", Artist: "Troilo", Album: "Rock nacional", Track: 3},
	}
	var rows []FileRow
	rs, err := s.db.Query(`SELECT path, kind FROM files`)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var r FileRow
		rs.Scan(&r.Path, &r.Kind)
		r.Size, r.MTime = 10, 1
		if r.Kind == "image" {
			r.Size = 1
		}
		rows = append(rows, r)
	}
	rs.Close()
	for p := range tags {
		rows = append(rows, audioRow(p, 10, 1))
	}
	if err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	env := ReadEnv{Version: 1}
	pending, _ := s.PendingReads(env)
	var res []ReadResult
	for _, p := range pending {
		fp := "fp" + p.Path
		if p.Path == comp+"/01.mp3" {
			fp = "fp" + la + "/[LA-10] Soda Stereo - Signos/01.mp3"
		}
		res = append(res, ReadResult{Target: p, Env: env, Fingerprint: fp,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: tags[p.Path]}})
	}
	if len(res) != 3 {
		t.Fatalf("%d pending", len(res))
	}
	if err := s.SaveReads(res); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
}

func TestAlbumMarkNeedsMostTracks(t *testing.T) {
	s := library(t)
	addCompilation(t, s)
	signos := albumNamed(t, s, "Signos")
	comp := albumNamed(t, s, "Rock nacional")
	if d, _, _ := s.Album(comp); len(d.Tracks) != 3 {
		t.Fatalf("compilation = %+v", d)
	}
	if ok, err := s.LikeAlbum(signos, true); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if ok, err := s.RateAlbum(signos, 5); !ok || err != nil {
		t.Fatal(ok, err)
	}
	p := personal(t, s)
	if m := p.Albums[signos]; !m.Liked || m.Rating != 5 {
		t.Fatalf("signos = %+v", m)
	}
	if m, in := p.Albums[comp]; in {
		t.Fatalf("1 of 3 tracks marks the compilation: %+v", m)
	}
	if ok, err := s.LikeAlbum(comp, true); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if m := personal(t, s).Albums[comp]; !m.Liked || m.Rating != 0 {
		t.Fatalf("compilation = %+v", m)
	}
}

func TestAlbumMarkWithAMissingTrack(t *testing.T) {
	s := library(t)
	id := albumNamed(t, s, "Obras completas")
	s.LikeAlbum(id, true)
	if _, err := s.db.Exec(`UPDATE files SET missing = 1 WHERE id = (SELECT file_id FROM tracks WHERE title = 'Sur')`); err != nil {
		t.Fatal(err)
	}
	if m := personal(t, s).Albums[id]; !m.Liked {
		t.Fatalf("album = %+v", m)
	}
}

func refCount(t *testing.T, s *Store, table, ref string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE ref = ?`, ref).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIDRefsFollowTheFingerprint(t *testing.T) {
	s := open(t)
	if err := s.SyncFiles([]FileRow{audioRow("../musica/Tangos/a.mp3", 10, 1)}, root); err != nil {
		t.Fatal(err)
	}
	env := ReadEnv{Version: 1}
	pending, _ := s.PendingReads(env)
	if len(pending) != 1 {
		t.Fatalf("pending = %+v", pending)
	}
	res := ReadResult{Target: pending[0], Env: env,
		Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: audiofile.Tags{Title: "A", Artist: "X"}}}
	if err := s.SaveReads([]ReadResult{res}); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	id := pending[0].FileID
	old := "id:" + strconv.FormatInt(id, 10)
	s.LikeTrack(id, true)
	s.RecordPlay(id, time.Now(), 500, "")
	pl, _ := s.CreatePlaylist("L")
	s.AddToPlaylist(pl, []int64{id})
	if _, err := s.db.Exec(`INSERT INTO loudness (ref, lufs, error, measured_at) VALUES (?, -14, '', 1)`, old); err != nil {
		t.Fatal(err)
	}
	tables := []string{"marks", "plays", "playlist_items", "loudness"}
	for _, tbl := range tables {
		if n := refCount(t, s, tbl, old); n != 1 {
			t.Fatalf("%s: %d rows with %s", tbl, n, old)
		}
	}

	res.Fingerprint = "fp-a"
	if err := s.SaveReads([]ReadResult{res}); err != nil {
		t.Fatal(err)
	}
	for _, tbl := range tables {
		if n := refCount(t, s, tbl, old); n != 0 {
			t.Fatalf("%s still has %d id rows", tbl, n)
		}
	}
	for _, tbl := range tables[:3] {
		if n := refCount(t, s, tbl, "fp-a"); n != 1 {
			t.Fatalf("%s: %d rows with the fingerprint", tbl, n)
		}
	}
	p := personal(t, s)
	if !p.Tracks[id].Liked || p.Plays[id].Count != 1 {
		t.Fatalf("personal = %+v %+v", p.Tracks, p.Plays)
	}
}
