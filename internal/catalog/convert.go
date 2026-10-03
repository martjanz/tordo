package catalog

import (
	"cmp"
	"path"
	"slices"
)

// ConvertKeys are the formats Revisar suggests converting, in the order
// it lists them; ConvertLabels names them.
var ConvertKeys = []string{"wma", "wav", "aiff", "ape", "wavpack", "alac"}
var ConvertLabels = map[string]string{"wma": "WMA", "wav": "WAV", "aiff": "AIFF", "ape": "APE", "wavpack": "WavPack", "alac": "ALAC"}

type ConvertFormat struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

// ConvertItem is one album's (or, for loose tracks, one folder's) files
// in one of those formats.
type ConvertItem struct {
	AlbumID    int64  `json:"albumId"` // 0: loose tracks of Dir
	Title      string `json:"title"`   // the album's, or the folder's name
	Artist     string `json:"artist"`
	Dir        string `json:"dir"`
	Format     string `json:"format"` // a key of ConvertKeys
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
	DurationMs int64  `json:"durationMs"`
}

type ToConvert struct {
	Files   int             `json:"files"`
	Bytes   int64           `json:"bytes"`
	Formats []ConvertFormat `json:"formats"` // only those with files, in ConvertKeys order
	Items   []ConvertItem   `json:"items"`   // by bytes descending, then title
}

// ToConvert returns the formats to convert and their items.
func (c *Catalog) ToConvert() ToConvert {
	result := ToConvert{
		Formats: []ConvertFormat{},
		Items:   []ConvertItem{},
	}

	// Track which formats to convert and their counts/bytes
	formatStats := map[string]*ConvertFormat{} // key -> *ConvertFormat

	// Group items: key = (albumID, format) for albums, (0, format, dir) for loose tracks
	type itemKey struct {
		albumID int64
		format  string
		dir     string
	}
	itemMap := map[itemKey]*ConvertItem{} // key -> *ConvertItem

	// Process all tracks
	for _, t := range c.Tracks {
		fk := FormatKey(t.Format, t.Codec)

		// Check if this format is in ConvertKeys
		isConvertFormat := slices.Contains(ConvertKeys, fk)
		if !isConvertFormat {
			continue
		}

		// Update format stats
		if formatStats[fk] == nil {
			formatStats[fk] = &ConvertFormat{
				Key:   fk,
				Label: ConvertLabels[fk],
				Files: 0,
				Bytes: 0,
			}
		}
		formatStats[fk].Files++
		formatStats[fk].Bytes += t.SizeBytes
		result.Files++
		result.Bytes += t.SizeBytes

		// Determine item grouping
		var key itemKey
		var item *ConvertItem

		if t.AlbumID == 0 {
			// Loose track: group by folder and format
			key = itemKey{albumID: 0, format: fk, dir: t.Dir}
			folderName := path.Base(t.Dir)

			if itemMap[key] == nil {
				itemMap[key] = &ConvertItem{
					AlbumID:    0,
					Title:      folderName,
					Artist:     "",
					Dir:        t.Dir,
					Format:     fk,
					Files:      0,
					Bytes:      0,
					DurationMs: 0,
				}
			}
		} else {
			// Album track: group by album and format
			key = itemKey{albumID: t.AlbumID, format: fk, dir: ""}
			al, ok := c.Album(t.AlbumID)
			if !ok {
				continue
			}

			if itemMap[key] == nil {
				itemMap[key] = &ConvertItem{
					AlbumID:    t.AlbumID,
					Title:      al.Title,
					Artist:     al.Artist,
					Dir:        al.Dir,
					Format:     fk,
					Files:      0,
					Bytes:      0,
					DurationMs: 0,
				}
			}
		}

		item = itemMap[key]
		item.Files++
		item.Bytes += t.SizeBytes
		item.DurationMs += t.DurationMs
	}

	// Build formats list in ConvertKeys order, only including formats with files
	for _, fk := range ConvertKeys {
		if f, ok := formatStats[fk]; ok {
			result.Formats = append(result.Formats, *f)
		}
	}

	// Build items list from the map
	for _, item := range itemMap {
		result.Items = append(result.Items, *item)
	}

	// Sort items: by bytes descending, then title
	slices.SortFunc(result.Items, func(a, b ConvertItem) int {
		// Sort by bytes descending (reverse)
		if c := cmp.Compare(b.Bytes, a.Bytes); c != 0 {
			return c
		}
		// Then by title ascending
		return cmp.Compare(a.Title, b.Title)
	})

	return result
}
