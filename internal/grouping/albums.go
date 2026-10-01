package grouping

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"tordo/internal/audiofile"
	"tordo/internal/nameparse"
	"tordo/internal/natsort"
)

type member struct {
	out      TrackOut
	path     string
	tags     audiofile.Tags
	album    string           // album title without disc marker
	albumDir string           // the album's folder
	folder   nameparse.Folder // what the album folder's name says
}

func buildAlbums(tracks []Track, images []Image, isRoot map[string]bool) Result {
	perDir := map[string]int{}
	for _, t := range tracks {
		perDir[path.Dir(t.Path)]++
	}
	parsed := map[string]nameparse.Folder{}
	parse := func(dir string) nameparse.Folder {
		if isRoot[dir] || !strings.Contains(dir, "/") {
			return nameparse.Folder{}
		}
		f, ok := parsed[dir]
		if !ok {
			f = nameparse.ParseFolder(path.Base(dir))
			parsed[dir] = f
		}
		return f
	}

	groups := map[string][]*member{}
	var order []string
	var loose []*member
	for _, t := range tracks {
		dir := path.Dir(t.Path)
		df := parse(dir)
		albumDir, af := dir, df
		if df.Disc > 0 && !isRoot[dir] {
			albumDir = path.Dir(dir)
			if df.Album == "" && df.Artist == "" { // a bare "CD1": the parent names the album
				af = parse(albumDir)
				af.Disc = df.Disc
			}
		}
		tg := t.Tags
		ff := nameparse.ParseFile(path.Base(t.Path), firstOf(tg.AlbumArtist, af.Artist))
		m := &member{
			path: t.Path, tags: tg, albumDir: albumDir, folder: af,
			out: TrackOut{
				FileID: t.FileID, Dir: dir, DurationMs: t.DurationMs,
				Title:  firstOf(tg.Title, ff.Title, stem(t.Path)),
				Artist: firstOf(tg.Artist, ff.Artist, tg.AlbumArtist, af.Artist),
				Disc:   firstNonZero(tg.Disc, af.Disc, ff.Disc),
				Track:  firstNonZero(tg.Track, ff.Track),
				Year:   firstNonZero(tg.Year, ff.Year, af.Year),
			},
		}
		title := tg.Album
		if title == "" && !isRoot[albumDir] && perDir[dir] >= 2 {
			title = af.Album
		}
		m.album, _ = nameparse.StripDisc(title)
		if m.album == "" {
			loose = append(loose, m)
			continue
		}
		key := albumDir + "\x00" + nameparse.Norm(m.album)
		m.out.AlbumKey = key
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], m)
	}

	covers := indexImages(images)
	var res Result
	for _, key := range order {
		ms := groups[key]
		sort.SliceStable(ms, func(i, j int) bool { return memberLess(ms[i], ms[j]) })
		a := makeAlbum(key, ms)
		a.CoverPath = covers.pick(a.Dir, ms)
		res.Albums = append(res.Albums, a)
		for _, m := range ms {
			res.Tracks = append(res.Tracks, m.out)
		}
	}
	sort.SliceStable(loose, func(i, j int) bool { return natsort.Less(loose[i].path, loose[j].path) })
	for _, m := range loose {
		res.Tracks = append(res.Tracks, m.out)
	}
	return res
}

func memberLess(a, b *member) bool {
	if a.out.Disc != b.out.Disc {
		return a.out.Disc < b.out.Disc
	}
	if a.out.Track != b.out.Track {
		return a.out.Track < b.out.Track
	}
	return natsort.Less(a.path, b.path)
}

func makeAlbum(key string, ms []*member) Album {
	a := Album{Key: key, Dir: ms[0].albumDir, Position: ms[0].folder.Position}
	var titles, albumArtists, artists, years counter
	compilations := 0
	for _, m := range ms {
		titles.add(m.album)
		albumArtists.add(m.tags.AlbumArtist)
		artists.add(m.out.Artist)
		if m.out.Year > 0 {
			years.add(strconv.Itoa(m.out.Year))
		}
		if m.tags.Compilation {
			compilations++
		}
		a.Tracks++
		a.DurationMs += m.out.DurationMs
		a.Discs = max(a.Discs, m.out.Disc, 1)
		for _, g := range m.tags.Genres {
			a.Genres = addGenre(a.Genres, g)
		}
		if a.CoverFileID == 0 && m.tags.HasCover {
			a.CoverFileID = m.out.FileID
		}
	}
	a.Title = titles.top()
	a.Year, _ = strconv.Atoi(years.top())
	folder := ms[0].folder
	switch aa := albumArtists.top(); {
	case aa != "" && !nameparse.IsVarious(aa):
		a.Artist = aa
	case aa != "" || folder.Various:
		a.Artist, a.Compilation = Various, true
	case folder.Artist != "":
		a.Artist = folder.Artist
	case artists.distinct() == 1:
		a.Artist = artists.top()
	case artists.distinct() > 1:
		a.Artist, a.Compilation = Various, true
	}
	if compilations*2 > len(ms) {
		a.Compilation = true
	}
	return a
}

func addGenre(gs []string, g string) []string {
	for _, x := range gs {
		if strings.EqualFold(x, g) {
			return gs
		}
	}
	return append(gs, g)
}

// counter counts values by their normalized form and remembers the first
// spelling seen of each.
type counter struct {
	keys []string
	n    map[string]int
	orig map[string]string
}

func (c *counter) add(v string) {
	if v == "" {
		return
	}
	k := nameparse.Norm(v)
	if c.n == nil {
		c.n, c.orig = map[string]int{}, map[string]string{}
	}
	if _, ok := c.n[k]; !ok {
		c.keys = append(c.keys, k)
		c.orig[k] = v
	}
	c.n[k]++
}

// top returns the most frequent value (the first seen on a tie), or "".
func (c *counter) top() string {
	best := ""
	for _, k := range c.keys {
		if best == "" || c.n[k] > c.n[best] {
			best = k
		}
	}
	return c.orig[best]
}

func (c *counter) distinct() int { return len(c.keys) }

// coverIndex finds cover images: by folder, and by parent folder for the
// image folders below an album ("Scans/", "Artwork/").
type coverIndex struct {
	byDir    map[string][]Image
	children map[string][]string // folder → its subfolders that hold images
}

func indexImages(images []Image) coverIndex {
	ix := coverIndex{byDir: map[string][]Image{}, children: map[string][]string{}}
	for _, img := range images {
		d := path.Dir(img.Path)
		if _, ok := ix.byDir[d]; !ok {
			ix.children[path.Dir(d)] = append(ix.children[path.Dir(d)], d)
		}
		ix.byDir[d] = append(ix.byDir[d], img)
	}
	return ix
}

// pick chooses the album's cover: a "cover", "folder" or "front" image is
// best; then one whose name mentions them; then the largest image in the
// album's own folders. Images named "back…" never count.
func (ix coverIndex) pick(albumDir string, ms []*member) string {
	dirs := map[string]bool{albumDir: true}
	for _, m := range ms {
		dirs[m.out.Dir] = true
	}
	best, bestScore, bestSize := "", 0, int64(-1)
	consider := func(img Image, own bool) {
		st := strings.ToLower(stem(img.Path))
		score := 0
		switch {
		case strings.Contains(st, "back") || strings.Contains(st, "contra"):
			score = 0
		case st == "cover" || st == "folder" || st == "front":
			score = 3
		case strings.Contains(st, "cover") || strings.Contains(st, "folder") || strings.Contains(st, "front"):
			score = 2
		case own:
			score = 1
		}
		if score > bestScore || (score == bestScore && score > 0 && img.Size > bestSize) {
			best, bestScore, bestSize = img.Path, score, img.Size
		}
	}
	for d := range dirs {
		for _, img := range ix.byDir[d] {
			consider(img, true)
		}
	}
	for _, sub := range ix.children[albumDir] {
		if !dirs[sub] {
			for _, img := range ix.byDir[sub] {
				consider(img, false)
			}
		}
	}
	return best
}
