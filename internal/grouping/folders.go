package grouping

import (
	"path"
	"sort"
	"strings"

	"tordo/internal/nameparse"
	"tordo/internal/natsort"
)

type folderTotals struct {
	tracks, albums int
	duration       int64
}

func buildFolders(res Result, isRoot map[string]bool) []Folder {
	totals := map[string]*folderTotals{}
	// up applies fn to dir and every folder above it, up to its root. Folders
	// outside every root are ignored.
	up := func(dir string, fn func(*folderTotals)) {
		if rootOf(dir, isRoot) == "" {
			return
		}
		for d := dir; ; d = path.Dir(d) {
			t := totals[d]
			if t == nil {
				t = &folderTotals{}
				totals[d] = t
			}
			fn(t)
			if isRoot[d] {
				return
			}
		}
	}
	for _, tr := range res.Tracks {
		up(tr.Dir, func(t *folderTotals) { t.tracks++; t.duration += tr.DurationMs })
	}
	for _, a := range res.Albums {
		up(a.Dir, func(t *folderTotals) { t.albums++ })
	}

	artists := map[string]bool{}
	for _, tr := range res.Tracks {
		artists[nameparse.Norm(tr.Artist)] = true
	}
	direct := map[string][]Album{} // folder → albums in its child folders
	for _, a := range res.Albums {
		artists[nameparse.Norm(a.Artist)] = true
		if !isRoot[a.Dir] {
			direct[path.Dir(a.Dir)] = append(direct[path.Dir(a.Dir)], a)
		}
	}
	delete(artists, nameparse.Norm(Various))
	delete(artists, "")

	out := make([]Folder, 0, len(totals))
	for p, t := range totals {
		f := Folder{Path: p, Name: path.Base(p), Tracks: t.tracks, Albums: t.albums, DurationMs: t.duration}
		if !isRoot[p] {
			f.Parent = path.Dir(p)
			f.Collection = isCollection(f.Name, direct[p], artists)
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return natsort.Less(out[i].Path, out[j].Path) })
	return out
}

// isCollection: at least 3 albums in child folders, by at least 2 different
// album artists, under a name that is not an artist's.
func isCollection(name string, albums []Album, artists map[string]bool) bool {
	if len(albums) < 3 {
		return false
	}
	distinct := map[string]bool{}
	for _, a := range albums {
		distinct[nameparse.Norm(a.Artist)] = true
	}
	return len(distinct) >= 2 && !artists[nameparse.Norm(name)]
}

// rootOf returns the root that contains dir (or is dir), or "".
func rootOf(dir string, isRoot map[string]bool) string {
	for r := range isRoot {
		if dir == r || strings.HasPrefix(dir, r+"/") {
			return r
		}
	}
	return ""
}
