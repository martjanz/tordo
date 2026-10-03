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

type ConvertItem struct {
	AlbumID    int64  `json:"albumId"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Dir        string `json:"dir"`
	Format     string `json:"format"`
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
	DurationMs int64  `json:"durationMs"`
}

type ToConvert struct {
	Files   int             `json:"files"`
	Bytes   int64           `json:"bytes"`
	Formats []ConvertFormat `json:"formats"`
	Items   []ConvertItem   `json:"items"`
}

func (c *Catalog) ToConvert() ToConvert {
	type itemKey struct {
		albumID int64
		format  string
		dir     string
	}

	formatStats := map[string]*ConvertFormat{}
	itemMap := map[itemKey]*ConvertItem{}

	for _, t := range c.Tracks {
		fk := FormatKey(t.Format, t.Codec)
		if _, isConvert := ConvertLabels[fk]; !isConvert {
			continue
		}

		var key itemKey
		var title, artist, dir string
		var albumID int64

		if t.AlbumID == 0 {
			key = itemKey{format: fk, dir: t.Dir}
			title = path.Base(t.Dir)
		} else {
			al, ok := c.Album(t.AlbumID)
			if !ok {
				key = itemKey{format: fk, dir: t.Dir}
				title = path.Base(t.Dir)
			} else {
				albumID = t.AlbumID
				key = itemKey{albumID: albumID, format: fk}
				title = al.Title
				artist = al.Artist
				dir = al.Dir
			}
		}

		if _, ok := formatStats[fk]; !ok {
			formatStats[fk] = &ConvertFormat{Key: fk, Label: ConvertLabels[fk]}
		}
		formatStats[fk].Files++
		formatStats[fk].Bytes += t.SizeBytes

		if _, ok := itemMap[key]; !ok {
			itemMap[key] = &ConvertItem{AlbumID: albumID, Title: title, Artist: artist, Dir: dir, Format: fk}
		}
		itemMap[key].Files++
		itemMap[key].Bytes += t.SizeBytes
		itemMap[key].DurationMs += t.DurationMs
	}

	result := ToConvert{Formats: []ConvertFormat{}, Items: []ConvertItem{}}
	for _, fk := range ConvertKeys {
		if f, ok := formatStats[fk]; ok {
			result.Formats = append(result.Formats, *f)
			result.Files += f.Files
			result.Bytes += f.Bytes
		}
	}

	for _, item := range itemMap {
		result.Items = append(result.Items, *item)
	}

	formatIndex := map[string]int{}
	for i, fk := range ConvertKeys {
		formatIndex[fk] = i
	}

	slices.SortFunc(result.Items, func(a, b ConvertItem) int {
		if c := cmp.Compare(b.Bytes, a.Bytes); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Title, b.Title); c != 0 {
			return c
		}
		if c := cmp.Compare(formatIndex[a.Format], formatIndex[b.Format]); c != 0 {
			return c
		}
		if c := cmp.Compare(a.AlbumID, b.AlbumID); c != 0 {
			return c
		}
		return cmp.Compare(a.Dir, b.Dir)
	})

	return result
}
