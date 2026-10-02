package store

import (
	"reflect"
	"testing"

	"tordo/internal/audiofile"
)

func playlistTitles(p Playlist) []string {
	out := []string{}
	for _, e := range p.Entries {
		out = append(out, e.Title)
	}
	return out
}

func mustPlaylist(t *testing.T, s *Store, id int64) Playlist {
	t.Helper()
	p, ok, err := s.Playlist(id)
	if err != nil || !ok {
		t.Fatalf("Playlist(%d) = %v, %v", id, ok, err)
	}
	return p
}

func TestPlaylistLifecycle(t *testing.T) {
	s := library(t)
	id, err := s.CreatePlaylist("  Para correr  ")
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.Playlists()
	if err != nil || len(list) != 1 || list[0].Name != "Para correr" || list[0].Tracks != 0 {
		t.Fatalf("playlists = %+v, %v", list, err)
	}
	sur, signos := trackNamed(t, s, "Sur"), trackNamed(t, s, "Signos")
	n, ok, err := s.AddToPlaylist(id, []int64{sur, signos, 99999, sur})
	if n != 3 || !ok || err != nil {
		t.Fatalf("add = %d, %v, %v", n, ok, err)
	}
	p := mustPlaylist(t, s, id)
	if got := playlistTitles(p); !reflect.DeepEqual(got, []string{"Sur", "Signos", "Sur"}) {
		t.Fatalf("entries = %q", got)
	}
	e := p.Entries[1]
	if e.TrackID != signos || e.AlbumID != albumNamed(t, s, "Signos") || e.Artist != "Soda Stereo" || e.Album != "Signos" || e.DurationMs != 1000 || e.AddedAt <= 0 {
		t.Fatalf("entry = %+v", e)
	}
	if p.Tracks != 3 || p.DurationMs != 3000 || !reflect.DeepEqual(p.Covers, []int64{albumNamed(t, s, "Signos")}) {
		t.Fatalf("summary = %+v", p.PlaylistSummary)
	}
	if ok, err := s.RenamePlaylist(id, "Mañanas"); !ok || err != nil {
		t.Fatalf("rename = %v, %v", ok, err)
	}
	if list, _ := s.Playlists(); list[0].Name != "Mañanas" {
		t.Fatalf("playlists = %+v", list)
	}
	if _, err := s.RenamePlaylist(id, "  "); err != ErrPlaylistName {
		t.Fatalf("empty rename: %v", err)
	}
	if _, err := s.CreatePlaylist(""); err != ErrPlaylistName {
		t.Fatalf("empty create: %v", err)
	}
	if ok, err := s.RenamePlaylist(999, "x"); ok || err != nil {
		t.Fatalf("unknown rename = %v, %v", ok, err)
	}
	if _, ok, _ := s.AddToPlaylist(999, []int64{sur}); ok {
		t.Fatal("added to a missing playlist")
	}
	if ok, err := s.DeletePlaylist(id); !ok || err != nil {
		t.Fatalf("delete = %v, %v", ok, err)
	}
	if _, ok, _ := s.Playlist(id); ok {
		t.Fatal("still there")
	}
	if n := countRows(t, s, "playlist_items"); n != 0 {
		t.Fatalf("%d entries left", n)
	}
	if ok, _ := s.DeletePlaylist(id); ok {
		t.Fatal("deleted twice")
	}
}

func TestPlaylistReorderAndRemove(t *testing.T) {
	s := library(t)
	id, _ := s.CreatePlaylist("x")
	s.AddToPlaylist(id, []int64{trackNamed(t, s, "Sur"), trackNamed(t, s, "Signos"), trackNamed(t, s, "Suelto")})
	p := mustPlaylist(t, s, id)
	a, b, c := p.Entries[0].ItemID, p.Entries[1].ItemID, p.Entries[2].ItemID
	if ok, err := s.ReorderPlaylist(id, []int64{c, a, b}); !ok || err != nil {
		t.Fatalf("reorder = %v, %v", ok, err)
	}
	if got := playlistTitles(mustPlaylist(t, s, id)); !reflect.DeepEqual(got, []string{"Suelto", "Sur", "Signos"}) {
		t.Fatalf("order = %q", got)
	}
	for _, bad := range [][]int64{{c, a}, {c, a, 99999}, {c, a, b, b}, {c, a, a}} {
		if _, err := s.ReorderPlaylist(id, bad); err != ErrPlaylistOrder {
			t.Fatalf("reorder %v: %v", bad, err)
		}
	}
	if got := playlistTitles(mustPlaylist(t, s, id)); !reflect.DeepEqual(got, []string{"Suelto", "Sur", "Signos"}) {
		t.Fatalf("order changed: %q", got)
	}
	if ok, err := s.RemoveFromPlaylist(id, b); !ok || err != nil {
		t.Fatalf("remove = %v, %v", ok, err)
	}
	if ok, _ := s.RemoveFromPlaylist(id, b); ok {
		t.Fatal("removed twice")
	}
	if got := playlistTitles(mustPlaylist(t, s, id)); !reflect.DeepEqual(got, []string{"Suelto", "Sur"}) {
		t.Fatalf("after remove = %q", got)
	}
	rows, _ := s.db.Query(`SELECT pos FROM playlist_items WHERE playlist_id = ? ORDER BY pos`, id)
	var pos []int
	for rows.Next() {
		var p int
		rows.Scan(&p)
		pos = append(pos, p)
	}
	rows.Close()
	if !reflect.DeepEqual(pos, []int{0, 1}) {
		t.Fatalf("pos = %v", pos)
	}
}

func TestPlaylistEntryWhoseTrackLeft(t *testing.T) {
	s := library(t)
	id, _ := s.CreatePlaylist("x")
	s.AddToPlaylist(id, []int64{trackNamed(t, s, "Sur"), trackNamed(t, s, "Signos")})
	old := tango + "/CD2/01.mp3"

	// sync rescans every present file except old, plus path when given.
	sync := func(path string) {
		t.Helper()
		var rows []FileRow
		rs, err := s.db.Query(`SELECT path FROM files WHERE kind = 'audio' AND path != ? AND missing = 0`, old)
		if err != nil {
			t.Fatal(err)
		}
		for rs.Next() {
			var p string
			rs.Scan(&p)
			rows = append(rows, audioRow(p, 10, 1))
		}
		rs.Close()
		if path != "" {
			rows = append(rows, audioRow(path, 10, 1))
		}
		if err := s.SyncFiles(rows, root); err != nil {
			t.Fatal(err)
		}
		env := ReadEnv{Version: 1}
		pending, _ := s.PendingReads(env)
		var res []ReadResult
		for _, p := range pending {
			res = append(res, ReadResult{Target: p, Env: env, Fingerprint: "fp" + old,
				Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: audiofile.Tags{Title: "Sur", Artist: "Troilo", Album: "Obras completas", Disc: 2, Track: 1}}})
		}
		if err := s.SaveReads(res); err != nil {
			t.Fatal(err)
		}
		regroup(t, s)
	}

	sync("")
	p := mustPlaylist(t, s, id)
	if p.Tracks != 2 || p.Entries[0].TrackID != 0 || p.Entries[0].Title != "Sur" || p.Entries[1].TrackID == 0 {
		t.Fatalf("playlist = %+v", p)
	}
	items, _, err := s.PlayPlaylist(id)
	if err != nil || !reflect.DeepEqual(titles(items), []string{"Signos"}) {
		t.Fatalf("play = %q, %v", titles(items), err)
	}

	moved := tango + "/CD2/moved.mp3"
	sync(moved)
	var newID int64
	if err := s.db.QueryRow(`SELECT id FROM files WHERE path = ?`, moved).Scan(&newID); err != nil {
		t.Fatal(err)
	}
	p = mustPlaylist(t, s, id)
	if p.Entries[0].TrackID != newID {
		t.Fatalf("entry = %+v, want track %d", p.Entries[0], newID)
	}
	items, _, _ = s.PlayPlaylist(id)
	if !reflect.DeepEqual(titles(items), []string{"Sur", "Signos"}) {
		t.Fatalf("play = %q", titles(items))
	}
	if _, ok, err := s.PlayPlaylist(999); ok || err != nil {
		t.Fatalf("unknown play: %v, %v", ok, err)
	}
}

func TestPlaylistLimit(t *testing.T) {
	s := library(t)
	old := MaxPlaylist
	MaxPlaylist = 3
	t.Cleanup(func() { MaxPlaylist = old })
	id, _ := s.CreatePlaylist("x")
	sur := trackNamed(t, s, "Sur")
	if n, _, err := s.AddToPlaylist(id, []int64{sur, sur}); n != 2 || err != nil {
		t.Fatalf("add = %d, %v", n, err)
	}
	if _, _, err := s.AddToPlaylist(id, []int64{sur, sur}); err != ErrPlaylistFull {
		t.Fatalf("over the limit: %v", err)
	}
	if p := mustPlaylist(t, s, id); p.Tracks != 2 {
		t.Fatalf("tracks = %d", p.Tracks)
	}
	if n, _, err := s.AddToPlaylist(id, []int64{sur}); n != 1 || err != nil {
		t.Fatalf("up to the limit = %d, %v", n, err)
	}
}

func TestPlaylistWritesKeepTheCatalog(t *testing.T) {
	s := library(t)
	before, _ := s.Changes()
	id, _ := s.CreatePlaylist("x")
	s.AddToPlaylist(id, []int64{trackNamed(t, s, "Sur"), trackNamed(t, s, "Signos")})
	p := mustPlaylist(t, s, id)
	s.ReorderPlaylist(id, []int64{p.Entries[1].ItemID, p.Entries[0].ItemID})
	s.RemoveFromPlaylist(id, p.Entries[0].ItemID)
	s.RenamePlaylist(id, "y")
	s.DeletePlaylist(id)
	if after, _ := s.Changes(); after != before {
		t.Fatalf("Changes %d -> %d", before, after)
	}
	if s.PersonalVersion() == 0 {
		t.Fatal("personal version did not move")
	}
}
