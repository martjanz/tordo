// Package grouping turns the catalog's audio files into tracks, albums,
// folders and collections. It is a pure function of its input: the scanner
// rebuilds the derived tables from it after every scan.
package grouping

import (
	"path"
	"strings"

	"tordo/internal/audiofile"
)

const Various = "Varios artistas"

type Track struct {
	FileID     int64
	Path       string // catalog form
	Tags       audiofile.Tags
	DurationMs int64
}

type Image struct {
	Path string
	Size int64
}

type TrackOut struct {
	FileID     int64
	AlbumKey   string // "" for a loose track
	Dir        string
	Title      string
	Artist     string
	Disc       int
	Track      int
	Year       int
	DurationMs int64
}

type Album struct {
	Key         string // album folder + "\x00" + normalized title
	Dir         string // the album's folder (the parent of its disc folders)
	Title       string
	Artist      string // album artist, or Various
	Year        int
	Compilation bool
	Discs       int
	Tracks      int
	DurationMs  int64
	Position    int    // order inside its parent collection (0: none)
	CoverPath   string // image file next to the album ("" if none)
	CoverFileID int64  // a track with an embedded picture (0 if none)
	Genres      []string
}

type Folder struct {
	Path       string
	Parent     string // "" for a root
	Name       string
	Tracks     int // recursive
	Albums     int // recursive
	DurationMs int64
	Collection bool
}

type Result struct {
	Albums  []Album
	Tracks  []TrackOut
	Folders []Folder
}

// Build groups tracks into albums, folders and collections. roots are the
// configured roots in catalog form; tracks outside them are ignored for
// folders.
func Build(tracks []Track, images []Image, roots []string) Result {
	isRoot := map[string]bool{}
	for _, r := range roots {
		isRoot[path.Clean(r)] = true
	}
	res := buildAlbums(tracks, images, isRoot)
	res.Folders = buildFolders(res, isRoot)
	return res
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func stem(p string) string { return strings.TrimSuffix(path.Base(p), path.Ext(p)) }
