package store

// SnapTrack is a track as the catalog's pages see it.
type SnapTrack struct {
	TrackView
	AlbumID int64  `json:"albumId"` // 0: a loose track
	Dir     string `json:"dir"`
	Added   int64  `json:"added"` // when its file was first seen, unix ms
}

// SnapFolder is a folder with its parent ("" for a root).
type SnapFolder struct {
	FolderSummary
	Parent string `json:"parent"`
}

// Snapshot is the whole catalog as Explorar, the artist pages and the
// search see it, read at one point in time.
type Snapshot struct {
	Albums  []AlbumView       // by id, with their genres
	Tracks  []SnapTrack       // by file id
	Folders []SnapFolder      // by path
	Artists map[string]Artist // MusicBrainz artists by id
	// Changes counts the rows written through the catalog's connection since
	// it was opened: it differs between two snapshots when the catalog
	// changed in between (it restarts if the connection does).
	Changes int64
}

// Snapshot reads the catalog in one transaction, so that it comes from one
// state even while the scanner or the identification write. The catalog has
// a single connection, which the transaction holds: everything here reads
// through tx.
func (s *Store) Snapshot() (Snapshot, error) {
	var snap Snapshot
	s.side.mu.Lock() // before the connection, as in sideWrite
	defer s.side.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return snap, err
	}
	defer tx.Rollback()
	if err := tx.QueryRow(`SELECT total_changes()`).Scan(&snap.Changes); err != nil {
		return snap, err
	}
	snap.Changes -= s.side.rows
	if snap.Albums, err = readAlbums(tx, `SELECT `+albumCols+` FROM albums ORDER BY id`); err != nil {
		return snap, err
	}
	if snap.Tracks, err = snapTracks(tx); err != nil {
		return snap, err
	}
	if snap.Folders, err = snapFolders(tx); err != nil {
		return snap, err
	}
	if snap.Artists, err = loadDocs[Artist](tx, "artists"); err != nil {
		return snap, err
	}
	return snap, nil
}

func snapTracks(q querier) ([]SnapTrack, error) {
	rows, err := q.Query(trackQuery + ` ORDER BY t.file_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SnapTrack{}
	for rows.Next() {
		var t SnapTrack
		if err := rows.Scan(trackDest(&t.TrackView, &t.AlbumID, &t.Dir, &t.Added)...); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func snapFolders(q querier) ([]SnapFolder, error) {
	rows, err := q.Query(`SELECT ` + folderCols + `, parent FROM folders ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SnapFolder{}
	for rows.Next() {
		var f SnapFolder
		if err := rows.Scan(&f.Path, &f.Name, &f.TrackCount, &f.AlbumCount, &f.DurationMs, &f.Collection, &f.Detected, &f.Parent); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
