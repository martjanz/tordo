package store

import (
	"path"
	"sort"
	"strings"

	"tordo/internal/natsort"
)

// FolderChild is a subfolder as the folder page lists it.
type FolderChild struct {
	FolderSummary
	SizeBytes int64  `json:"sizeBytes"` // every present catalogued file under it
	Album     int64  `json:"album"`     // an album whose folder it is (smallest id), else 0
	Codec     string `json:"codec"`     // the codec most of its audio files use ("" without audio)
}

// FolderFile is a present catalogued file directly in a folder.
type FolderFile struct {
	ID         int64  `json:"id"`   // the file id; for a track, also the track id
	Name       string `json:"name"` // file name
	Kind       string `json:"kind"` // audio | image | lyrics | playlist
	SizeBytes  int64  `json:"sizeBytes"`
	Track      bool   `json:"track"` // an audio file the catalog has as a track
	DurationMs int64  `json:"durationMs"`
	Format     string `json:"format"`
	Codec      string `json:"codec"`
	Lossless   bool   `json:"lossless"`
	Bitrate    int    `json:"bitrate"`
	SampleRate int    `json:"sampleRate"`
	BitDepth   int    `json:"bitDepth"`
}

// folderStats returns the bytes of every present file under folder p and
// the codec most of its audio files use (ties: the alphabetically first).
func (s *Store) folderStats(p string) (size int64, codec string, err error) {
	rows, err := s.db.Query(`SELECT COALESCE(a.codec, ''), COUNT(a.file_id), COALESCE(SUM(f.size), 0)
		FROM files f LEFT JOIN audio a ON a.file_id = f.id AND f.kind = 'audio'
		WHERE f.path >= ?1 AND f.path < ?2 AND f.missing = 0
		GROUP BY 1`, p+"/", p+"0")
	if err != nil {
		return 0, "", err
	}
	defer rows.Close()
	best := int64(0)
	for rows.Next() {
		var c string
		var n, sz int64
		if err := rows.Scan(&c, &n, &sz); err != nil {
			return 0, "", err
		}
		size += sz
		if c == "" {
			continue
		}
		if n > best || (n == best && c < codec) {
			best, codec = n, c
		}
	}
	return size, codec, rows.Err()
}

// folderFiles lists the present files directly in folder p, in natural order.
func (s *Store) folderFiles(p string) ([]FolderFile, error) {
	prefix := p + "/"
	rows, err := s.db.Query(`SELECT f.id, f.path, f.kind, f.size, t.file_id IS NOT NULL,
			COALESCE(a.duration_ms, 0), COALESCE(a.format, ''), COALESCE(a.codec, ''),
			COALESCE(a.lossless, 0), COALESCE(a.bitrate, 0), COALESCE(a.sample_rate, 0), COALESCE(a.bit_depth, 0)
		FROM files f LEFT JOIN audio a ON a.file_id = f.id LEFT JOIN tracks t ON t.file_id = f.id
		WHERE f.path >= ?1 AND f.path < ?2 AND f.missing = 0`, prefix, p+"0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FolderFile{}
	for rows.Next() {
		var f FolderFile
		var fp string
		if err := rows.Scan(&f.ID, &fp, &f.Kind, &f.SizeBytes, &f.Track, &f.DurationMs, &f.Format, &f.Codec,
			&f.Lossless, &f.Bitrate, &f.SampleRate, &f.BitDepth); err != nil {
			return nil, err
		}
		if strings.Contains(fp[len(prefix):], "/") {
			continue
		}
		f.Name = path.Base(fp)
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return natsort.Less(out[i].Name, out[j].Name) })
	return out, nil
}
