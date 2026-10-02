package catalog

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"tordo/internal/search"
	"tordo/internal/store"
)

// Search document keys: a kind and the item's id.
const (
	keyArtist = "artist:"
	keyAlbum  = "album:"
	keyTrack  = "track:"
	keyFolder = "folder:"
)

// Folder is a folder found by the search: one that is not an album's own.
type Folder struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Collection bool   `json:"collection"`
	Albums     int    `json:"albums"`
	Tracks     int    `json:"tracks"`
}

// SearchDocs gives the search index one document per artist, album, track
// and folder (other than albums' own folders, which their albums stand for).
func (c *Catalog) SearchDocs() []search.Doc {
	docs := make([]search.Doc, 0, len(c.Artists)+len(c.Albums)+len(c.Tracks))
	for _, a := range c.Artists {
		docs = append(docs, search.Doc{Key: keyArtist + a.Key, Name: a.Name, Context: strings.Join(a.names, " ")})
	}
	for _, a := range c.Albums {
		docs = append(docs, search.Doc{Key: keyAlbum + strconv.FormatInt(a.ID, 10), Name: a.Title,
			Context: a.Artist + " " + a.tagged})
	}
	for _, t := range c.Tracks {
		ctx := t.Artist + " " + t.Album
		if t.TagTitle != t.Title {
			ctx += " " + t.TagTitle
		}
		docs = append(docs, search.Doc{Key: keyTrack + strconv.FormatInt(t.ID, 10), Name: t.Title, Context: ctx})
	}
	albumDirs := map[string]bool{}
	for _, a := range c.Albums {
		albumDirs[a.Dir] = true
	}
	for _, f := range c.sortedFolders() {
		if !albumDirs[f.Path] && f.TrackCount > 0 {
			docs = append(docs, search.Doc{Key: keyFolder + f.Path, Name: f.Name, Context: c.folders[f.Parent].Name})
		}
	}
	return docs
}

func (c *Catalog) sortedFolders() []store.SnapFolder {
	out := make([]store.SnapFolder, 0, len(c.folders))
	for _, f := range c.folders {
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b store.SnapFolder) int { return cmp.Compare(a.Path, b.Path) })
	return out
}

// Found is what a search finds, by kind, best first: at most limit of each,
// with how many there are in all.
type Found struct {
	Artists []Artist       `json:"artists"`
	Albums  []Album        `json:"albums"`
	Tracks  []Track        `json:"tracks"`
	Folders []Folder       `json:"folders"`
	Totals  map[string]int `json:"totals"` // by kind: artists, albums, tracks, folders
}

// Found turns the index's keys into items. Artists come by their number of
// albums: their names all match, and the one with most albums is likely
// the one sought.
func (c *Catalog) Found(keys []string, limit int) Found {
	f := Found{Artists: []Artist{}, Albums: []Album{}, Tracks: []Track{}, Folders: []Folder{}, Totals: map[string]int{}}
	// room counts one more item of a kind and tells whether it is shown.
	room := func(kind string, shown int) bool {
		f.Totals[kind]++
		return shown < limit
	}
	for _, k := range keys {
		kind, id, _ := strings.Cut(k, ":")
		n, _ := strconv.ParseInt(id, 10, 64)
		switch kind + ":" {
		case keyArtist:
			if a, ok := c.Artist(id); ok {
				f.Totals["artists"]++
				f.Artists = append(f.Artists, *a)
			}
		case keyAlbum:
			if a, ok := c.Album(n); ok && room("albums", len(f.Albums)) {
				f.Albums = append(f.Albums, *a)
			}
		case keyTrack:
			if t, ok := c.Track(n); ok && room("tracks", len(f.Tracks)) {
				f.Tracks = append(f.Tracks, *t)
			}
		case keyFolder:
			if d, ok := c.folders[id]; ok && room("folders", len(f.Folders)) {
				f.Folders = append(f.Folders, Folder{Path: d.Path, Name: d.Name, Collection: d.Collection,
					Albums: d.AlbumCount, Tracks: d.TrackCount})
			}
		}
	}
	slices.SortStableFunc(f.Artists, func(a, b Artist) int { return cmp.Compare(b.Albums, a.Albums) })
	f.Artists = f.Artists[:min(limit, len(f.Artists))]
	return f
}
