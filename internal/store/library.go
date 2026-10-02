package store

import (
	"cmp"
	"database/sql"
	"errors"
	"path"
	"slices"
	"strings"

	"tordo/internal/natsort"
)

// TrackFile is what serving a track's audio needs.
type TrackFile struct {
	ID       int64
	Path     string // relative to the app folder
	Format   string
	Codec    string
	Lossless bool
}

// TrackFile returns a present, readable track's file.
func (s *Store) TrackFile(id int64) (TrackFile, bool, error) {
	f := TrackFile{ID: id}
	err := s.db.QueryRow(`SELECT f.path, a.format, a.codec, a.lossless FROM tracks t
		JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
		WHERE t.file_id = ? AND f.missing = 0`, id).Scan(&f.Path, &f.Format, &f.Codec, &f.Lossless)
	if errors.Is(err, sql.ErrNoRows) {
		return TrackFile{}, false, nil
	}
	return f, err == nil, err
}

// PlayItem is a track as the player queues it.
type PlayItem struct {
	ID         int64  `json:"id"` // the file id
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	AlbumID    int64  `json:"albumId"` // 0: a loose track
	DurationMs int64  `json:"durationMs"`
	Format     string `json:"format"`
	Codec      string `json:"codec"`
	Lossless   bool   `json:"lossless"`
	dir        string
	path       string
	disc       int
	track      int
}

const playQuery = `SELECT t.file_id, CASE WHEN t.mb_title != '' THEN t.mb_title ELSE t.title END, t.artist,
	COALESCE(CASE WHEN al.mb_title != '' THEN al.mb_title ELSE al.title END, ''), COALESCE(t.album_id, 0), t.duration_ms,
	a.format, a.codec, a.lossless, t.dir, f.path, t.disc, t.track
	FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
	LEFT JOIN albums al ON al.id = t.album_id `

func (s *Store) playItems(where string, args ...any) ([]PlayItem, error) {
	rows, err := s.db.Query(playQuery+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayItem{}
	for rows.Next() {
		var p PlayItem
		if err := rows.Scan(&p.ID, &p.Title, &p.Artist, &p.Album, &p.AlbumID, &p.DurationMs, &p.Format, &p.Codec,
			&p.Lossless, &p.dir, &p.path, &p.disc, &p.track); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// albumOrder sorts an album's tracks: disc, track number, then file name.
func albumOrder(a, b PlayItem) int {
	return cmp.Or(cmp.Compare(a.disc, b.disc), cmp.Compare(a.track, b.track), natCompare(a.path, b.path))
}

func natCompare(a, b string) int {
	switch {
	case natsort.Less(a, b):
		return -1
	case natsort.Less(b, a):
		return 1
	}
	return 0
}

// PlayAlbum returns an album's tracks in order.
func (s *Store) PlayAlbum(id int64) ([]PlayItem, error) {
	items, err := s.playItems(`WHERE t.album_id = ?`, id)
	slices.SortStableFunc(items, albumOrder)
	return items, err
}

// PlayTracks returns the given tracks in the given order, skipping unknown ids.
func (s *Store) PlayTracks(ids []int64) ([]PlayItem, error) {
	out := []PlayItem{}
	for _, id := range ids {
		items, err := s.playItems(`WHERE t.file_id = ?`, id)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

// PlayFolder returns every track inside a folder, in the order its page
// shows them: subfolders first (natural order, recursively), then the
// folder's albums (collection order) with their tracks in order, then its
// loose tracks. An album whose discs are in subfolders (CD1, CD2) belongs to
// the folder above them, so it is played once, whole; inside a disc folder
// itself, that disc's tracks are played.
func (s *Store) PlayFolder(p string) ([]PlayItem, error) {
	items, err := s.playItems(`WHERE t.dir = ?1 OR substr(t.dir, 1, length(?1) + 1) = ?1 || '/'`, p)
	if err != nil {
		return nil, err
	}
	albums, err := s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE dir = ?1 OR substr(dir, 1, length(?1) + 1) = ?1 || '/'`, p)
	if err != nil {
		return nil, err
	}
	inside := map[int64]bool{}
	for _, a := range albums {
		inside[a.ID] = true
	}
	byAlbum := map[int64][]PlayItem{}
	looseIn := map[string][]PlayItem{}
	strayIn := map[string][]PlayItem{} // tracks of an album whose folder is above p (a disc folder)
	for _, it := range items {
		switch {
		case it.AlbumID != 0 && inside[it.AlbumID]:
			byAlbum[it.AlbumID] = append(byAlbum[it.AlbumID], it)
		case it.AlbumID != 0:
			strayIn[it.dir] = append(strayIn[it.dir], it)
		default:
			looseIn[it.dir] = append(looseIn[it.dir], it)
		}
	}
	albumsIn := map[string][]AlbumView{}
	children := map[string][]string{}
	addDir := func(d string) {
		for d != p && strings.HasPrefix(d, p+"/") {
			parent := path.Dir(d)
			if !slices.Contains(children[parent], d) {
				children[parent] = append(children[parent], d)
			}
			d = parent
		}
	}
	for _, a := range albums {
		albumsIn[a.Dir] = append(albumsIn[a.Dir], a)
		addDir(a.Dir)
	}
	for d := range looseIn {
		addDir(d)
	}
	for d := range strayIn {
		addDir(d)
	}
	out := []PlayItem{}
	var walk func(d string)
	walk = func(d string) {
		kids := children[d]
		slices.SortFunc(kids, func(a, b string) int { return natCompare(path.Base(a), path.Base(b)) })
		for _, k := range kids {
			walk(k)
		}
		as := albumsIn[d]
		sortByPosition(as)
		for _, a := range as {
			tracks := byAlbum[a.ID]
			slices.SortStableFunc(tracks, albumOrder)
			out = append(out, tracks...)
		}
		stray := strayIn[d]
		slices.SortStableFunc(stray, albumOrder)
		out = append(out, stray...)
		loose := looseIn[d]
		slices.SortFunc(loose, func(a, b PlayItem) int { return natCompare(a.path, b.path) })
		out = append(out, loose...)
	}
	walk(p)
	return out, nil
}

// AlbumPictureFiles returns the paths of an album's tracks with an embedded
// picture, the album's cover track first: some files claim a picture but
// hold an empty one, so the next one can stand in.
func (s *Store) AlbumPictureFiles(albumID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT f.path FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
		JOIN albums al ON al.id = t.album_id
		WHERE t.album_id = ? AND json_extract(a.tags, '$.hasCover')
		ORDER BY t.file_id != al.cover_file_id, t.disc, t.track, f.path`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CoverFiles are the image files an album's picture can come from, as
// paths relative to the app folder ("" when there is none).
type CoverFiles struct {
	Folder string // the folder image the grouping chose
	Cache  string // the Cover Art Archive image the identification downloaded
}

// AlbumCover returns an album's image files and whether the album exists.
func (s *Store) AlbumCover(albumID int64) (CoverFiles, bool, error) {
	var c CoverFiles
	err := s.db.QueryRow(`SELECT cover_path, cover_cache FROM albums WHERE id = ?`, albumID).Scan(&c.Folder, &c.Cache)
	if errors.Is(err, sql.ErrNoRows) {
		return CoverFiles{}, false, nil
	}
	return c, err == nil, err
}

// FolderArt returns, for each subfolder of p, up to four of the albums with
// a picture inside it (at any depth, in natural order of their folders):
// what its card shows. A subfolder that is one album's own folder gets just
// that album, picture or not.
func (s *Store) FolderArt(p string) (map[string][]int64, error) {
	out := map[string][]int64{}
	if p == "" {
		return out, nil // the top level lists the roots: they get no art
	}
	prefix := p + "/"
	rows, err := s.db.Query(`SELECT id, dir, `+hasPicture+` FROM albums
		WHERE substr(dir, 1, length(?1)) = ?1`, prefix)
	if err != nil {
		return nil, err
	}
	type entry struct {
		id      int64
		dir     string
		picture bool
	}
	byChild := map[string][]entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.dir, &e.picture); err != nil {
			rows.Close()
			return nil, err
		}
		child := prefix + strings.SplitN(strings.TrimPrefix(e.dir, prefix), "/", 2)[0]
		byChild[child] = append(byChild[child], e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for child, es := range byChild {
		slices.SortFunc(es, func(a, b entry) int { return natCompare(a.dir, b.dir) })
		if es[0].dir == child {
			out[child] = []int64{es[0].id}
			continue
		}
		for _, e := range es {
			if e.picture && len(out[child]) < 4 {
				out[child] = append(out[child], e.id)
			}
		}
	}
	return out, nil
}

// Crumb is one folder on the way from a root to a folder.
type Crumb struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// FolderTrail returns the folders from p's root down to p itself.
func (s *Store) FolderTrail(p string) ([]Crumb, error) {
	out := []Crumb{}
	for p != "" && len(out) < 256 {
		var c Crumb
		var parent string
		err := s.db.QueryRow(`SELECT path, name, parent FROM folders WHERE path = ?`, p).Scan(&c.Path, &c.Name, &parent)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append([]Crumb{c}, out...)
		p = parent
	}
	return out, nil
}
