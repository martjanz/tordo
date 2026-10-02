package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path"
	"sort"
	"time"

	"tordo/internal/grouping"
	"tordo/internal/natsort"
)

// Counts summarizes the catalog for the status endpoint.
type Counts struct {
	Files        int `json:"files"`
	Audio        int `json:"audio"`
	Unreadable   int `json:"unreadable"`
	Tracks       int `json:"tracks"`
	Albums       int `json:"albums"`
	Folders      int `json:"folders"`
	Collections  int `json:"collections"`
	Identified   int `json:"identified"`
	Unidentified int `json:"unidentified"`
}

func (s *Store) Counts() (Counts, error) {
	var c Counts
	err := s.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM files WHERE missing = 0),
		(SELECT COUNT(*) FROM files WHERE missing = 0 AND kind = 'audio'),
		(SELECT COUNT(*) FROM files f JOIN audio a ON a.file_id = f.id WHERE f.missing = 0 AND a.error != ''),
		(SELECT COUNT(*) FROM tracks),
		(SELECT COUNT(*) FROM albums),
		(SELECT COUNT(*) FROM folders),
		(SELECT COUNT(*) FROM folders WHERE `+isCollection+`),
		(SELECT COUNT(*) FROM albums WHERE status IN ('auto', 'manual')),
		(SELECT COUNT(*) FROM albums WHERE status = 'unmatched')`).
		Scan(&c.Files, &c.Audio, &c.Unreadable, &c.Tracks, &c.Albums, &c.Folders, &c.Collections, &c.Identified, &c.Unidentified)
	return c, err
}

// ReplaceGrouping stores a grouping result. Albums are upserted by key so
// their ids survive rebuilds; albums that disappeared are deleted.
func (s *Store) ReplaceGrouping(r grouping.Result) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE albums SET seen = 0`); err != nil {
		return err
	}
	up, err := tx.Prepare(`INSERT INTO albums (key, dir, title, artist, year, compilation, discs, tracks, duration_ms,
		position, cover_path, cover_file_id, genres, selection, seen) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(key) DO UPDATE SET dir = excluded.dir, title = excluded.title, artist = excluded.artist,
		year = excluded.year, compilation = excluded.compilation, discs = excluded.discs, tracks = excluded.tracks,
		duration_ms = excluded.duration_ms, position = excluded.position, cover_path = excluded.cover_path,
		cover_file_id = excluded.cover_file_id, genres = excluded.genres, selection = excluded.selection, seen = 1
		RETURNING id`)
	if err != nil {
		return err
	}
	defer up.Close()
	ids := make(map[string]int64, len(r.Albums))
	for _, a := range r.Albums {
		genres := a.Genres
		if genres == nil {
			genres = []string{}
		}
		g, err := json.Marshal(genres)
		if err != nil {
			return err
		}
		var id int64
		if err := up.QueryRow(a.Key, a.Dir, a.Title, a.Artist, a.Year, a.Compilation, a.Discs, a.Tracks, a.DurationMs,
			a.Position, a.CoverPath, a.CoverFileID, string(g), a.Selection).Scan(&id); err != nil {
			return err
		}
		ids[a.Key] = id
	}
	if _, err := tx.Exec(`DELETE FROM tracks`); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO tracks (file_id, album_id, dir, title, artist, disc, track, year, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, t := range r.Tracks {
		var album any
		if t.AlbumKey != "" {
			album = ids[t.AlbumKey]
		}
		if _, err := ins.Exec(t.FileID, album, t.Dir, t.Title, t.Artist, t.Disc, t.Track, t.Year, t.DurationMs); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM albums WHERE seen = 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM folders`); err != nil {
		return err
	}
	fins, err := tx.Prepare(`INSERT INTO folders (path, parent, name, tracks, albums, duration_ms, collection) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer fins.Close()
	for _, f := range r.Folders {
		if _, err := fins.Exec(f.Path, f.Parent, f.Name, f.Tracks, f.Albums, f.DurationMs, f.Collection); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type AlbumView struct {
	ID           int64       `json:"id"`
	Title        string      `json:"title"` // MusicBrainz's when identified, else from tags or folder
	Artist       string      `json:"artist"`
	Year         int         `json:"year"`
	OriginalYear int         `json:"originalYear"`
	Compilation  bool        `json:"compilation"`
	Selection    bool        `json:"selection"`
	Discs        int         `json:"discs"`
	Tracks       int         `json:"tracks"`
	DurationMs   int64       `json:"durationMs"`
	Dir          string      `json:"dir"`
	Position     int         `json:"position"`
	CoverPath    string      `json:"coverPath"`
	CoverFileID  int64       `json:"coverFileId"`
	CoverCache   string      `json:"coverCache"`
	Picture      bool        `json:"picture"` // a folder image, an embedded picture or a cached one
	Genres       []GenreLink `json:"genres"`
	RawGenres    []string    `json:"rawGenres"`
	Status       string      `json:"status"`
	ReleaseID    string      `json:"releaseId"`
	Confidence   float64     `json:"confidence"`
	Country      string      `json:"country"`
	ArtistID     string      `json:"artistId"`
	TagTitle     string      `json:"tagTitle"`
	TagArtist    string      `json:"tagArtist"`
	MatchQuery   string      `json:"-"`
}

type TrackView struct {
	ID          int64  `json:"id"` // the file id
	Path        string `json:"path"`
	Title       string `json:"title"` // MusicBrainz's when paired, else from tags or file name
	TagTitle    string `json:"tagTitle"`
	RecordingID string `json:"recordingId"`
	Artist      string `json:"artist"`
	Disc        int    `json:"disc"`
	Track       int    `json:"track"`
	Year        int    `json:"year"`
	DurationMs  int64  `json:"durationMs"`
	Format      string `json:"format"`
	Codec       string `json:"codec"`
	Lossless    bool   `json:"lossless"`
	Bitrate     int    `json:"bitrate"`
	SampleRate  int    `json:"sampleRate"`
	BitDepth    int    `json:"bitDepth"`
	Fingerprint string `json:"fingerprint"`
}

type AlbumDetail struct {
	AlbumView
	Tracks []TrackView `json:"tracks"`
}

type FolderSummary struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	TrackCount int    `json:"trackCount"` // recursive
	AlbumCount int    `json:"albumCount"` // recursive
	DurationMs int64  `json:"durationMs"`
	Collection bool   `json:"collection"` // detected, and the user did not say otherwise
	Detected   bool   `json:"detected"`   // detected as a collection
}

// isCollection tells, in SQL over folders, whether a folder is a
// collection: detected, and not dismissed by the user.
const isCollection = `(collection = 1 AND path NOT IN (SELECT path FROM not_collections))`

const folderCols = `path, name, tracks, albums, duration_ms, ` + isCollection + `, collection`

type FolderView struct {
	FolderSummary
	Parent    string        `json:"parent"`
	SizeBytes int64         `json:"sizeBytes"`
	Children  []FolderChild `json:"children"`
	Albums    []AlbumView   `json:"albums"` // albums whose folder is this one
	Loose     []TrackView   `json:"loose"`  // tracks here that belong to no album
	Files     []FolderFile  `json:"files"`  // present files directly in this folder
}

type CollectionView struct {
	FolderSummary
	Albums []AlbumView `json:"albums"` // in collection order
}

const albumCols = `id, CASE WHEN mb_title != '' THEN mb_title ELSE title END,
	CASE WHEN mb_artist != '' THEN mb_artist ELSE artist END, year, original_year, compilation OR mb_compilation,
	selection, discs, tracks, duration_ms, dir, position, cover_path, cover_file_id, cover_cache, genres, status,
	release_id, confidence, country, artist_id, title, artist, match_query, ` + hasPicture

// hasPicture tells, in SQL over albums, whether an album has a picture.
const hasPicture = `(cover_path != '' OR cover_file_id != 0 OR cover_cache != '')`

func (s *Store) queryAlbums(q string, args ...any) ([]AlbumView, error) {
	return readAlbums(s.db, q, args...)
}

// readAlbums reads albums (albumCols) with their genres through q.
func readAlbums(q querier, query string, args ...any) ([]AlbumView, error) {
	out, err := scanAlbums(q, query, args...)
	if err != nil {
		return nil, err
	}
	return out, fillGenres(q, out)
}

func scanAlbums(q querier, query string, args ...any) ([]AlbumView, error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlbumView{}
	for rows.Next() {
		var a AlbumView
		var raw string
		if err := rows.Scan(&a.ID, &a.Title, &a.Artist, &a.Year, &a.OriginalYear, &a.Compilation, &a.Selection, &a.Discs,
			&a.Tracks, &a.DurationMs, &a.Dir, &a.Position, &a.CoverPath, &a.CoverFileID, &a.CoverCache, &raw, &a.Status,
			&a.ReleaseID, &a.Confidence, &a.Country, &a.ArtistID, &a.TagTitle, &a.TagArtist, &a.MatchQuery, &a.Picture); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &a.RawGenres); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// fillGenres sets the albums' genre links. It reads the whole (small)
// album_genres table once instead of once per album.
func fillGenres(q querier, as []AlbumView) error {
	if len(as) == 0 {
		return nil
	}
	at := make(map[int64]int, len(as))
	for i := range as {
		at[as[i].ID] = i
		as[i].Genres = []GenreLink{}
	}
	rows, err := q.Query(`SELECT album_id, genre, style, source FROM album_genres ORDER BY album_id, genre, style`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var g GenreLink
		if err := rows.Scan(&id, &g.Genre, &g.Style, &g.Source); err != nil {
			return err
		}
		if i, ok := at[id]; ok {
			as[i].Genres = append(as[i].Genres, g)
		}
	}
	return rows.Err()
}

const trackQuery = `SELECT t.file_id, f.path, CASE WHEN t.mb_title != '' THEN t.mb_title ELSE t.title END, t.title,
	t.recording_id, t.artist, t.disc, t.track, t.year, t.duration_ms,
	a.format, a.codec, a.lossless, a.bitrate, a.sample_rate, a.bit_depth, f.fingerprint,
	COALESCE(t.album_id, 0), t.dir, f.first_seen
	FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id `

func (s *Store) queryTracks(where string, args ...any) ([]TrackView, error) {
	rows, err := s.db.Query(trackQuery+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrackView{}
	for rows.Next() {
		var t TrackView
		var albumID, added int64
		var dir string
		if err := rows.Scan(trackDest(&t, &albumID, &dir, &added)...); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// trackDest lists where a row of trackQuery goes: the view, then its album
// id, folder and first-seen time.
func trackDest(t *TrackView, albumID *int64, dir *string, added *int64) []any {
	return []any{&t.ID, &t.Path, &t.Title, &t.TagTitle, &t.RecordingID, &t.Artist, &t.Disc, &t.Track, &t.Year,
		&t.DurationMs, &t.Format, &t.Codec, &t.Lossless, &t.Bitrate, &t.SampleRate, &t.BitDepth, &t.Fingerprint,
		albumID, dir, added}
}

// Albums lists the albums, all of them or those inside folder (at any depth).
func (s *Store) Albums(folder string) ([]AlbumView, error) {
	q := `SELECT ` + albumCols + ` FROM albums`
	var args []any
	if folder != "" {
		q += ` WHERE dir = ?1 OR substr(dir, 1, length(?1) + 1) = ?1 || '/'`
		args = append(args, folder)
	}
	return s.queryAlbums(q+` ORDER BY artist COLLATE NOCASE, year, title COLLATE NOCASE`, args...)
}

func (s *Store) Album(id int64) (AlbumDetail, bool, error) {
	as, err := s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE id = ?`, id)
	if err != nil || len(as) == 0 {
		return AlbumDetail{}, false, err
	}
	tracks, err := s.queryTracks(`WHERE t.album_id = ? ORDER BY t.disc, t.track, f.path`, id)
	if err != nil {
		return AlbumDetail{}, false, err
	}
	return AlbumDetail{AlbumView: as[0], Tracks: tracks}, true, nil
}

// Folder returns a folder with its subfolders, the albums whose folder it is,
// and its loose tracks. p == "" is the virtual top level: the roots.
func (s *Store) Folder(p string) (FolderView, bool, error) {
	v := FolderView{Children: []FolderChild{}, Albums: []AlbumView{}, Loose: []TrackView{}, Files: []FolderFile{}}
	if p != "" {
		err := s.db.QueryRow(`SELECT `+folderCols+`, parent FROM folders WHERE path = ?`, p).
			Scan(&v.Path, &v.Name, &v.TrackCount, &v.AlbumCount, &v.DurationMs, &v.Collection, &v.Detected, &v.Parent)
		if errors.Is(err, sql.ErrNoRows) {
			return FolderView{}, false, nil
		}
		if err != nil {
			return FolderView{}, false, err
		}
		v.SizeBytes, _, err = s.folderStats(p)
		if err != nil {
			return FolderView{}, false, err
		}
		if v.Files, err = s.folderFiles(p); err != nil {
			return FolderView{}, false, err
		}
	}
	children, err := s.folderSummaries(`WHERE parent = ?`, p)
	if err != nil {
		return FolderView{}, false, err
	}
	sort.SliceStable(children, func(i, j int) bool { return natsort.Less(children[i].Name, children[j].Name) })
	for _, c := range children {
		fc := FolderChild{FolderSummary: c}
		if fc.SizeBytes, fc.Codec, err = s.folderStats(c.Path); err != nil {
			return FolderView{}, false, err
		}
		var album sql.NullInt64
		if err := s.db.QueryRow(`SELECT MIN(id) FROM albums WHERE dir = ?`, c.Path).Scan(&album); err != nil {
			return FolderView{}, false, err
		}
		fc.Album = album.Int64
		v.Children = append(v.Children, fc)
	}
	if p == "" {
		return v, true, nil
	}
	if v.Albums, err = s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE dir = ?`, p); err != nil {
		return FolderView{}, false, err
	}
	sortByPosition(v.Albums)
	if v.Loose, err = s.queryTracks(`WHERE t.dir = ? AND t.album_id IS NULL ORDER BY f.path`, p); err != nil {
		return FolderView{}, false, err
	}
	return v, true, nil
}

func (s *Store) folderSummaries(where string, args ...any) ([]FolderSummary, error) {
	rows, err := s.db.Query(`SELECT `+folderCols+` FROM folders `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FolderSummary{}
	for rows.Next() {
		var f FolderSummary
		if err := rows.Scan(&f.Path, &f.Name, &f.TrackCount, &f.AlbumCount, &f.DurationMs, &f.Collection, &f.Detected); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Collections lists the detected collections with their albums in order.
func (s *Store) Collections() ([]CollectionView, error) {
	sums, err := s.folderSummaries(`WHERE ` + isCollection)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(sums, func(i, j int) bool { return natsort.Less(sums[i].Name, sums[j].Name) })
	out := []CollectionView{}
	for _, f := range sums {
		all, err := s.Albums(f.Path)
		if err != nil {
			return nil, err
		}
		albums := []AlbumView{}
		for _, a := range all {
			if path.Dir(a.Dir) == f.Path {
				albums = append(albums, a)
			}
		}
		sortByPosition(albums)
		out = append(out, CollectionView{FolderSummary: f, Albums: albums})
	}
	return out, nil
}

// SetCollection records whether a detected collection is one: false
// dismisses it ("no es una colección"), true takes that back. The decision
// is kept by path, across scans.
func (s *Store) SetCollection(path string, collection bool) error {
	var err error
	if collection {
		_, err = s.db.Exec(`DELETE FROM not_collections WHERE path = ?`, path)
	} else {
		_, err = s.db.Exec(`INSERT INTO not_collections (path, updated_at) VALUES (?, ?)
			ON CONFLICT(path) DO UPDATE SET updated_at = excluded.updated_at`, path, time.Now().UnixMilli())
	}
	return err
}

// sortByPosition orders albums by their collection position (albums without
// one last), then by folder name in natural order.
func sortByPosition(as []AlbumView) {
	sort.SliceStable(as, func(i, j int) bool {
		pi, pj := as[i].Position, as[j].Position
		if (pi == 0) != (pj == 0) {
			return pi != 0
		}
		if pi != pj {
			return pi < pj
		}
		return natsort.Less(path.Base(as[i].Dir), path.Base(as[j].Dir))
	})
}
