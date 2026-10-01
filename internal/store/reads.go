package store

import (
	"encoding/json"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
)

// ReadEnv describes the reader a scan runs with.
type ReadEnv struct {
	Version int  // audiofile.Version
	FFprobe bool // audiofile.HasFFprobe()
}

// ReadTarget is an audio file to read.
type ReadTarget struct {
	FileID int64
	Path   string
	Size   int64
	MTime  int64
}

// ReadResult is what reading a target produced. Err is set when the file was
// read but not understood: it is kept so the file is not read again until it
// changes (or ffprobe appears).
type ReadResult struct {
	Target      ReadTarget
	Info        audiofile.Info
	Fingerprint string
	Err         string
	Env         ReadEnv
}

// PendingReads returns the present audio files never read, changed since,
// read by an older reader version, or that failed without ffprobe while
// ffprobe is available now.
func (s *Store) PendingReads(env ReadEnv) ([]ReadTarget, error) {
	rows, err := s.db.Query(`SELECT f.id, f.path, f.size, f.mtime FROM files f
		LEFT JOIN audio a ON a.file_id = f.id
		WHERE f.missing = 0 AND f.kind = 'audio'
		  AND (a.file_id IS NULL OR a.size != f.size OR a.mtime != f.mtime OR a.read_version < ?1
		       OR (?2 AND a.error != '' AND a.with_ffprobe = 0))
		ORDER BY f.path`, env.Version, env.FFprobe)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReadTarget
	for rows.Next() {
		var t ReadTarget
		if err := rows.Scan(&t.FileID, &t.Path, &t.Size, &t.MTime); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveReads stores a batch of results in one transaction. A file whose
// content was seen before under another path inherits that path's first_seen.
func (s *Store) SaveReads(rs []ReadResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO audio (file_id, size, mtime, read_version, with_ffprobe, reader, error, format, codec,
		lossless, duration_ms, bitrate, sample_rate, bit_depth, channels, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_id) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
		read_version = excluded.read_version, with_ffprobe = excluded.with_ffprobe, reader = excluded.reader,
		error = excluded.error, format = excluded.format, codec = excluded.codec, lossless = excluded.lossless,
		duration_ms = excluded.duration_ms, bitrate = excluded.bitrate, sample_rate = excluded.sample_rate,
		bit_depth = excluded.bit_depth, channels = excluded.channels, tags = excluded.tags`)
	if err != nil {
		return err
	}
	defer up.Close()
	fp, err := tx.Prepare(`UPDATE files SET fingerprint = ?1,
		first_seen = CASE WHEN ?1 = '' THEN first_seen ELSE
			MIN(first_seen, COALESCE((SELECT MIN(first_seen) FROM files WHERE fingerprint = ?1 AND id != ?2), first_seen)) END
		WHERE id = ?2`)
	if err != nil {
		return err
	}
	defer fp.Close()
	for _, r := range rs {
		tags, err := json.Marshal(r.Info.Tags)
		if err != nil {
			return err
		}
		i := r.Info
		if _, err := up.Exec(r.Target.FileID, r.Target.Size, r.Target.MTime, r.Env.Version, r.Env.FFprobe, i.Reader, r.Err,
			i.Format, i.Codec, i.Lossless, i.DurationMs, i.Bitrate, i.SampleRate, i.BitDepth, i.Channels, string(tags)); err != nil {
			return err
		}
		if _, err := fp.Exec(r.Fingerprint, r.Target.FileID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GroupingInput returns the present, readable audio files with their tags,
// and the present images.
func (s *Store) GroupingInput() ([]grouping.Track, []grouping.Image, error) {
	rows, err := s.db.Query(`SELECT f.id, f.path, a.duration_ms, a.tags FROM files f JOIN audio a ON a.file_id = f.id
		WHERE f.missing = 0 AND f.kind = 'audio' AND a.error = '' AND a.size = f.size AND a.mtime = f.mtime
		ORDER BY f.path`)
	if err != nil {
		return nil, nil, err
	}
	var tracks []grouping.Track
	for rows.Next() {
		var t grouping.Track
		var tags string
		if err := rows.Scan(&t.FileID, &t.Path, &t.DurationMs, &tags); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if err := json.Unmarshal([]byte(tags), &t.Tags); err != nil {
			rows.Close()
			return nil, nil, err
		}
		tracks = append(tracks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	rows, err = s.db.Query(`SELECT path, size FROM files WHERE missing = 0 AND kind = 'image'`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var images []grouping.Image
	for rows.Next() {
		var img grouping.Image
		if err := rows.Scan(&img.Path, &img.Size); err != nil {
			return nil, nil, err
		}
		images = append(images, img)
	}
	return tracks, images, rows.Err()
}
