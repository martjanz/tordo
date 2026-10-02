package store

import (
	"database/sql"
	"errors"
	"slices"
	"strings"
)

// PlaylistSummary is a playlist as the Listas page shows it.
type PlaylistSummary struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Tracks     int     `json:"tracks"`     // entries, available or not
	DurationMs int64   `json:"durationMs"` // of every entry, as added
	Covers     []int64 `json:"covers"`     // up to 4 distinct albums with a picture, in list order
	UpdatedAt  int64   `json:"updatedAt"`
}

// PlaylistEntry is one entry: TrackID 0 when its track is no longer in the
// catalog (it keeps how it was called when added).
type PlaylistEntry struct {
	ItemID     int64  `json:"itemId"`
	TrackID    int64  `json:"trackId"`
	AlbumID    int64  `json:"albumId"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMs int64  `json:"durationMs"`
	AddedAt    int64  `json:"addedAt"`
}

type Playlist struct {
	PlaylistSummary
	Entries []PlaylistEntry `json:"entries"`
}

var (
	ErrPlaylistName  = errors.New("escribí un nombre para la lista")
	ErrPlaylistFull  = errors.New("la lista llegó al máximo de pistas")
	ErrPlaylistOrder = errors.New("el orden no coincide con las pistas de la lista")
)

// MaxPlaylist is the most entries a playlist holds (a variable so tests can
// lower it).
var MaxPlaylist = 5000

const maxPlaylistName = 200

func playlistName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrPlaylistName
	}
	if r := []rune(name); len(r) > maxPlaylistName {
		name = strings.TrimSpace(string(r[:maxPlaylistName]))
	}
	return name, nil
}

// rawEntry is a stored entry, before it is resolved to a track.
type rawEntry struct {
	playlist int64
	ref      string
	savedID  int64
	PlaylistEntry
}

// loadEntries reads the entries of one playlist (or of all when id is 0) in
// list order, then resolves each to a present track: the saved file when it
// still has the entry's ref, else the lowest present file with that ref,
// else none.
func loadEntries(q querier, id int64) ([]rawEntry, error) {
	rows, err := q.Query(`SELECT id, playlist_id, ref, file_id, title, artist, album, duration_ms, added_at
		FROM playlist_items WHERE ?1 = 0 OR playlist_id = ?1 ORDER BY playlist_id, pos, id`, id)
	if err != nil {
		return nil, err
	}
	var out []rawEntry
	for rows.Next() {
		var e rawEntry
		if err := rows.Scan(&e.ItemID, &e.playlist, &e.ref, &e.savedID, &e.Title, &e.Artist, &e.Album, &e.DurationMs, &e.AddedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	if err := closeRows(rows); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	type cand struct{ file, album int64 }
	byRef := map[string][]cand{} // present files by ref, lowest id first
	rows, err = q.Query(`SELECT `+trackRef+`, f.id, COALESCE(t.album_id, 0)
		FROM files f JOIN tracks t ON t.file_id = f.id
		WHERE f.missing = 0 AND (`+trackRef+`) IN (SELECT ref FROM playlist_items WHERE ?1 = 0 OR playlist_id = ?1)
		ORDER BY f.id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ref string
		var c cand
		if err := rows.Scan(&ref, &c.file, &c.album); err != nil {
			rows.Close()
			return nil, err
		}
		byRef[ref] = append(byRef[ref], c)
	}
	if err := closeRows(rows); err != nil {
		return nil, err
	}
	for i := range out {
		e := &out[i]
		cs := byRef[e.ref]
		if len(cs) == 0 {
			continue
		}
		pick := cs[0]
		for _, c := range cs {
			if c.file == e.savedID {
				pick = c
				break
			}
		}
		e.TrackID, e.AlbumID = pick.file, pick.album
	}
	return out, nil
}

// albumsWithPicture is the set of albums that have a picture.
func albumsWithPicture(q querier) (map[int64]bool, error) {
	rows, err := q.Query(`SELECT id FROM albums WHERE ` + hasPicture)
	if err != nil {
		return nil, err
	}
	pics := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		pics[id] = true
	}
	return pics, closeRows(rows)
}

// summarize fills the counts and covers of a playlist from its entries.
func summarize(sum *PlaylistSummary, entries []rawEntry, pics map[int64]bool) {
	sum.Tracks = len(entries)
	sum.DurationMs = 0
	sum.Covers = []int64{}
	for _, e := range entries {
		sum.DurationMs += e.DurationMs
		if len(sum.Covers) < 4 && e.AlbumID != 0 && pics[e.AlbumID] && !slices.Contains(sum.Covers, e.AlbumID) {
			sum.Covers = append(sum.Covers, e.AlbumID)
		}
	}
}

// Playlists lists the playlists, most recently updated first.
func (s *Store) Playlists() ([]PlaylistSummary, error) {
	rows, err := s.db.Query(`SELECT id, name, updated_at FROM playlists ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	out := []PlaylistSummary{}
	for rows.Next() {
		var p PlaylistSummary
		if err := rows.Scan(&p.ID, &p.Name, &p.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := closeRows(rows); err != nil {
		return nil, err
	}
	entries, err := loadEntries(s.db, 0)
	if err != nil {
		return nil, err
	}
	pics, err := albumsWithPicture(s.db)
	if err != nil {
		return nil, err
	}
	by := map[int64][]rawEntry{}
	for _, e := range entries {
		by[e.playlist] = append(by[e.playlist], e)
	}
	for i := range out {
		summarize(&out[i], by[out[i].ID], pics)
	}
	return out, nil
}

// Playlist reads one playlist with its entries.
func (s *Store) Playlist(id int64) (Playlist, bool, error) {
	var p Playlist
	err := s.db.QueryRow(`SELECT id, name, updated_at FROM playlists WHERE id = ?`, id).Scan(&p.ID, &p.Name, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	entries, err := loadEntries(s.db, id)
	if err != nil {
		return p, false, err
	}
	pics, err := albumsWithPicture(s.db)
	if err != nil {
		return p, false, err
	}
	summarize(&p.PlaylistSummary, entries, pics)
	p.Entries = make([]PlaylistEntry, len(entries))
	for i, e := range entries {
		p.Entries[i] = e.PlaylistEntry
	}
	return p, true, nil
}

// CreatePlaylist makes an empty playlist.
func (s *Store) CreatePlaylist(name string) (int64, error) {
	name, err := playlistName(name)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.sideWrite(true, func(tx *sql.Tx) error {
		ms := now().UnixMilli()
		res, err := tx.Exec(`INSERT INTO playlists (name, created_at, updated_at) VALUES (?, ?, ?)`, name, ms, ms)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

// touch sets the playlist's updated_at; false when it does not exist.
func touch(tx *sql.Tx, id int64) (bool, error) {
	res, err := tx.Exec(`UPDATE playlists SET updated_at = ? WHERE id = ?`, now().UnixMilli(), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// RenamePlaylist renames a playlist; false when it does not exist.
func (s *Store) RenamePlaylist(id int64, name string) (bool, error) {
	name, err := playlistName(name)
	if err != nil {
		return false, err
	}
	found := false
	err = s.sideWrite(true, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE playlists SET name = ?, updated_at = ? WHERE id = ?`, name, now().UnixMilli(), id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		found = n > 0
		return err
	})
	return found && err == nil, err
}

// DeletePlaylist deletes a playlist and its entries; false when it does not exist.
func (s *Store) DeletePlaylist(id int64) (bool, error) {
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM playlists WHERE id = ?`, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		found = n > 0
		return err
	})
	return found && err == nil, err
}

// AddToPlaylist appends present tracks to a playlist, skipping unknown or
// missing ids. It returns how many were added and whether the playlist exists.
func (s *Store) AddToPlaylist(id int64, trackIDs []int64) (int, bool, error) {
	added, found := 0, false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		var count, last int
		err := tx.QueryRow(`SELECT COUNT(*), COALESCE(MAX(pos), -1) FROM playlist_items WHERE playlist_id = ?`, id).Scan(&count, &last)
		if err != nil {
			return err
		}
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM playlists WHERE id = ?`, id).Scan(&exists); err != nil || exists == 0 {
			return err
		}
		found = true
		type row struct {
			ref, title, artist, album string
			file, duration            int64
		}
		var rows []row
		for _, tid := range trackIDs {
			r := row{file: tid}
			err := tx.QueryRow(`SELECT `+trackRef+`, CASE WHEN t.mb_title != '' THEN t.mb_title ELSE t.title END, t.artist,
				COALESCE(CASE WHEN al.mb_title != '' THEN al.mb_title ELSE al.title END, ''), t.duration_ms
				FROM tracks t JOIN files f ON f.id = t.file_id LEFT JOIN albums al ON al.id = t.album_id
				WHERE f.id = ? AND f.missing = 0`, tid).Scan(&r.ref, &r.title, &r.artist, &r.album, &r.duration)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				return err
			}
			rows = append(rows, r)
		}
		if count+len(rows) > MaxPlaylist {
			return ErrPlaylistFull
		}
		ms := now().UnixMilli()
		for _, r := range rows {
			last++
			if _, err := tx.Exec(`INSERT INTO playlist_items (playlist_id, pos, ref, file_id, title, artist, album, duration_ms, added_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, last, r.ref, r.file, r.title, r.artist, r.album, r.duration, ms); err != nil {
				return err
			}
		}
		added = len(rows)
		if added == 0 {
			return nil
		}
		_, err = touch(tx, id)
		return err
	})
	if err != nil {
		return 0, found, err
	}
	return added, found, nil
}

// itemIDs lists a playlist's entry ids in order.
func itemIDs(tx *sql.Tx, id int64) ([]int64, error) {
	rows, err := tx.Query(`SELECT id FROM playlist_items WHERE playlist_id = ? ORDER BY pos, id`, id)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var i int64
		if err := rows.Scan(&i); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, i)
	}
	return ids, closeRows(rows)
}

// writePositions sets pos 0…n−1 following ids.
func writePositions(tx *sql.Tx, ids []int64) error {
	for pos, itemID := range ids {
		if _, err := tx.Exec(`UPDATE playlist_items SET pos = ? WHERE id = ?`, pos, itemID); err != nil {
			return err
		}
	}
	return nil
}

// RemoveFromPlaylist deletes an entry and renumbers the rest; false when the
// playlist has no such entry.
func (s *Store) RemoveFromPlaylist(id, itemID int64) (bool, error) {
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM playlist_items WHERE id = ? AND playlist_id = ?`, itemID, id)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return err
		}
		found = true
		ids, err := itemIDs(tx, id)
		if err != nil {
			return err
		}
		if err := writePositions(tx, ids); err != nil {
			return err
		}
		_, err = touch(tx, id)
		return err
	})
	return found && err == nil, err
}

// ReorderPlaylist sets the order of a playlist's entries; itemIDs must be
// exactly its entry ids. False when the playlist does not exist.
func (s *Store) ReorderPlaylist(id int64, itemIDList []int64) (bool, error) {
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM playlists WHERE id = ?`, id).Scan(&exists); err != nil || exists == 0 {
			return err
		}
		found = true
		have, err := itemIDs(tx, id)
		if err != nil {
			return err
		}
		want := slices.Clone(itemIDList)
		slices.Sort(have)
		slices.Sort(want)
		if !slices.Equal(have, want) {
			return ErrPlaylistOrder
		}
		if err := writePositions(tx, itemIDList); err != nil {
			return err
		}
		_, err = touch(tx, id)
		return err
	})
	return found && err == nil, err
}

// PlayPlaylist returns the playable entries of a playlist, in order.
func (s *Store) PlayPlaylist(id int64) ([]PlayItem, bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM playlists WHERE id = ?`, id).Scan(&n); err != nil || n == 0 {
		return nil, false, err
	}
	entries, err := loadEntries(s.db, id)
	if err != nil {
		return nil, false, err
	}
	ids := []int64{}
	for _, e := range entries {
		if e.TrackID != 0 {
			ids = append(ids, e.TrackID)
		}
	}
	items, err := s.PlayTracks(ids)
	return items, err == nil, err
}
