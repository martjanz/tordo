package catalog

import (
	"cmp"
	"slices"
	"strconv"

	"tordo/internal/nameparse"
	"tordo/internal/store"
)

// Kinds of duplicate groups, as Revisar names them.
const (
	DupExact   = "exacta"   // the same audio (fingerprint) in several files
	DupFormats = "formatos" // the same recording in one album, in several files
	DupSuspect = "sospecha" // the same artist, title and length in different albums
)

// dupGapMs is the most two lengths of one recording may differ by.
const dupGapMs = 2000

// DupFile is one file of a duplicate group.
type DupFile struct {
	ID         int64  `json:"id"` // the file id
	Path       string `json:"path"`
	Dir        string `json:"dir"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	AlbumID    int64  `json:"albumId"` // 0: a loose track
	Album      string `json:"album"`
	Format     string `json:"format"` // FormatKey
	Lossless   bool   `json:"lossless"`
	Bitrate    int    `json:"bitrate"`
	SampleRate int    `json:"sampleRate"`
	BitDepth   int    `json:"bitDepth"`
	DurationMs int64  `json:"durationMs"`
	SizeBytes  int64  `json:"sizeBytes"`
	Best       bool   `json:"best"` // MEJOR: strictly better than every other file of its group
}

// DupGroup is a set of files that hold, or may hold, the same music.
type DupGroup struct {
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`  // of its first file
	Artist      string    `json:"artist"` // of its first file
	Files       []DupFile `json:"files"`
	SizeBytes   int64     `json:"sizeBytes"`   // all its files
	Recoverable int64     `json:"recoverable"` // exact copies: the bytes freed keeping one; else 0
}

// Duplicates are the three kinds of groups of the catalog.
type Duplicates struct {
	Exact       []DupGroup
	Formats     []DupGroup
	Suspect     []DupGroup
	Recoverable int64 // the exact copies' recoverable bytes
}

// Version is another album that holds some of an album's tracks.
type Version struct {
	AlbumID int64    `json:"albumId"`
	Title   string   `json:"title"`
	Artist  string   `json:"artist"`
	Year    int      `json:"year"`
	Formats []string `json:"formats"` // FormatKey of its shared tracks, sorted, distinct
	Shared  int      `json:"shared"`  // this album's tracks with a counterpart there
	Kind    string   `json:"kind"`    // DupExact when every shared track is an exact copy, else DupFormats
}

// Duplicates returns the groups computed by Build; callers must not modify them.
func (c *Catalog) Duplicates() Duplicates { return c.dups }

// AlbumVersions lists the other albums that share an exact or formats group
// with album id, the one sharing most first.
func (c *Catalog) AlbumVersions(id int64) []Version {
	type acc struct {
		formats map[string]bool
		tracks  map[int64]bool // this album's tracks shared with the other album
		exact   map[int64]bool // those with an exact copy there
	}
	byAlbum := map[int64]*acc{}
	for _, groups := range [][]DupGroup{c.dups.Exact, c.dups.Formats} {
		for _, g := range groups {
			var mine, others []DupFile
			for _, f := range g.Files {
				switch {
				case f.AlbumID == id:
					mine = append(mine, f)
				case f.AlbumID != 0:
					others = append(others, f)
				}
			}
			for _, o := range others {
				a := byAlbum[o.AlbumID]
				if a == nil {
					a = &acc{formats: map[string]bool{}, tracks: map[int64]bool{}, exact: map[int64]bool{}}
					byAlbum[o.AlbumID] = a
				}
				for _, m := range mine {
					a.formats[o.Format] = true
					a.tracks[m.ID] = true
					if g.Kind == DupExact {
						a.exact[m.ID] = true
					}
				}
			}
		}
	}
	out := []Version{}
	for albumID, a := range byAlbum {
		if len(a.tracks) == 0 {
			continue
		}
		al, ok := c.Album(albumID)
		if !ok {
			continue
		}
		v := Version{AlbumID: albumID, Title: al.Title, Artist: al.Artist, Year: al.Year, Shared: len(a.tracks), Kind: DupFormats}
		for f := range a.formats {
			v.Formats = append(v.Formats, f)
		}
		slices.Sort(v.Formats)
		if len(a.exact) == len(a.tracks) {
			v.Kind = DupExact
		}
		out = append(out, v)
	}
	slices.SortFunc(out, func(x, y Version) int {
		return cmp.Or(cmp.Compare(y.Shared, x.Shared), cmp.Compare(nameparse.Norm(x.Title), nameparse.Norm(y.Title)), cmp.Compare(x.AlbumID, y.AlbumID))
	})
	return out
}

// dupItem is a present track as the duplicate rules see it.
type dupItem struct {
	file  DupFile
	fp    string
	rec   string // MusicBrainz recording id
	album string // the album key; "track:<id>" for a loose track
	name  string // normalized artist and title; "" without a title
}

// albumKey tells when two albums are the same: the same MusicBrainz release
// group, else the same release, else the album itself.
func albumKey(a store.AlbumView, groups map[string]string) string {
	switch {
	case a.ReleaseID == "":
		return "al:" + strconv.FormatInt(a.ID, 10)
	case groups[a.ReleaseID] != "":
		return "rg:" + groups[a.ReleaseID]
	}
	return "rel:" + a.ReleaseID
}

// better is positive when a is better than b: lossless over lossy; among
// lossless, bit depth then sample rate; among lossy, bitrate.
func better(a, b DupFile) int {
	if a.Lossless != b.Lossless {
		if a.Lossless {
			return 1
		}
		return -1
	}
	if a.Lossless {
		return cmp.Or(cmp.Compare(a.BitDepth, b.BitDepth), cmp.Compare(a.SampleRate, b.SampleRate))
	}
	return cmp.Compare(a.Bitrate, b.Bitrate)
}

// computeDuplicates finds the three kinds of groups of a snapshot.
func (c *Catalog) computeDuplicates(snap store.Snapshot) {
	albums := map[int64]store.AlbumView{}
	for _, a := range snap.Albums {
		albums[a.ID] = a
	}
	items := make([]dupItem, 0, len(snap.Tracks))
	for _, t := range snap.Tracks {
		f := DupFile{ID: t.ID, Path: t.Path, Dir: t.Dir, Title: t.Title, Artist: t.Artist, AlbumID: t.AlbumID,
			Format: FormatKey(t.Format, t.Codec), Lossless: t.Lossless, Bitrate: t.Bitrate, SampleRate: t.SampleRate,
			BitDepth: t.BitDepth, DurationMs: t.DurationMs, SizeBytes: t.SizeBytes}
		it := dupItem{fp: t.Fingerprint, rec: t.RecordingID, album: "track:" + strconv.FormatInt(t.ID, 10)}
		if al, ok := albums[t.AlbumID]; ok && t.AlbumID != 0 {
			f.Album = al.Title
			it.album = albumKey(al, snap.ReleaseGroups)
			if f.Artist == "" {
				f.Artist = al.Artist
			}
		}
		if title := nameparse.Norm(t.Title); title != "" {
			it.name = nameparse.Norm(f.Artist) + "\x00" + title
		}
		it.file = f
		items = append(items, it)
	}

	// Exact copies: one fingerprint, several files.
	byFP := map[string][]int{}
	for i, it := range items {
		if it.fp != "" {
			byFP[it.fp] = append(byFP[it.fp], i)
		}
	}
	d := Duplicates{Exact: []DupGroup{}, Formats: []DupGroup{}, Suspect: []DupGroup{}}
	for _, idx := range byFP {
		if len(idx) < 2 {
			continue
		}
		g := makeGroup(DupExact, items, idx)
		var largest int64
		for _, f := range g.Files {
			largest = max(largest, f.SizeBytes)
		}
		g.Recoverable = g.SizeBytes - largest
		d.Recoverable += g.Recoverable
		d.Exact = append(d.Exact, g)
	}
	slices.SortFunc(d.Exact, func(x, y DupGroup) int {
		return cmp.Or(cmp.Compare(y.Recoverable, x.Recoverable), cmp.Compare(x.Files[0].Path, y.Files[0].Path))
	})

	// The same recording in one album: a recording id, or one name and length.
	uf := make([]int, len(items))
	for i := range uf {
		uf[i] = i
	}
	var find func(i int) int
	find = func(i int) int {
		for uf[i] != i {
			uf[i] = uf[uf[i]]
			i = uf[i]
		}
		return i
	}
	union := func(a, b int) { uf[find(a)] = find(b) }
	byAlbum := map[string][]int{}
	for i, it := range items {
		if it.file.AlbumID != 0 {
			byAlbum[it.album] = append(byAlbum[it.album], i)
		}
	}
	for _, idx := range byAlbum {
		if len(idx) < 2 {
			continue
		}
		byRec := map[string]int{}
		for _, i := range idx {
			if rec := items[i].rec; rec != "" {
				if j, ok := byRec[rec]; ok {
					union(i, j)
				} else {
					byRec[rec] = i
				}
			}
		}
		for _, run := range nameRuns(items, idx) {
			for _, i := range run[1:] {
				union(run[0], i)
			}
		}
	}
	comps := map[int][]int{}
	var roots []int
	for _, idx := range byAlbum {
		for _, i := range idx {
			r := find(i)
			if comps[r] == nil {
				roots = append(roots, r)
			}
			comps[r] = append(comps[r], i)
		}
	}
	for _, r := range roots {
		if idx := comps[r]; len(idx) >= 2 && distinctFingerprints(items, idx) >= 2 {
			d.Formats = append(d.Formats, makeGroup(DupFormats, items, idx))
		}
	}

	// Suspects: one name and length across albums, with different audio.
	byName := map[string][]int{}
	for i, it := range items {
		if it.name != "" {
			byName[it.name] = append(byName[it.name], i)
		}
	}
	for _, idx := range byName {
		if len(idx) < 2 {
			continue
		}
		for _, run := range nameRuns(items, idx) {
			keys := map[string]bool{}
			for _, i := range run {
				keys[items[i].album] = true
			}
			if len(run) >= 2 && len(keys) >= 2 && distinctFingerprints(items, run) >= 2 {
				d.Suspect = append(d.Suspect, makeGroup(DupSuspect, items, run))
			}
		}
	}
	byName2 := func(x, y DupGroup) int {
		return cmp.Or(cmp.Compare(nameparse.Norm(x.Artist), nameparse.Norm(y.Artist)), cmp.Compare(nameparse.Norm(x.Title), nameparse.Norm(y.Title)),
			cmp.Compare(x.Files[0].Path, y.Files[0].Path), cmp.Compare(x.Files[0].ID, y.Files[0].ID))
	}
	slices.SortFunc(d.Formats, byName2)
	slices.SortFunc(d.Suspect, byName2)
	c.dups = d

	// Estado "con duplicados": tracks of exact and formats groups, and their albums.
	for _, groups := range [][]DupGroup{d.Exact, d.Formats} {
		for _, g := range groups {
			for _, f := range g.Files {
				if i, ok := c.trackIndex[f.ID]; ok {
					c.Tracks[i].add(FacetState, StateDuplicated, "")
				}
				if al, ok := c.Album(f.AlbumID); ok {
					al.add(FacetState, StateDuplicated, "")
				}
			}
		}
	}
}

// nameRuns splits the items of idx by name and by length: sorted by length,
// a gap over dupGapMs starts another run. Runs of one item are dropped.
func nameRuns(items []dupItem, idx []int) [][]int {
	sorted := slices.Clone(idx)
	slices.SortFunc(sorted, func(a, b int) int {
		return cmp.Or(cmp.Compare(items[a].name, items[b].name), cmp.Compare(items[a].file.DurationMs, items[b].file.DurationMs), cmp.Compare(a, b))
	})
	var runs [][]int
	var run []int
	flush := func() {
		if len(run) >= 2 {
			runs = append(runs, run)
		}
		run = nil
	}
	for _, i := range sorted {
		if items[i].name == "" {
			continue
		}
		if len(run) > 0 {
			last := items[run[len(run)-1]]
			if last.name != items[i].name || items[i].file.DurationMs-last.file.DurationMs > dupGapMs {
				flush()
			}
		}
		run = append(run, i)
	}
	flush()
	return runs
}

// distinctFingerprints counts the different audio of the items: a file
// without a fingerprint is distinct from every other.
func distinctFingerprints(items []dupItem, idx []int) int {
	seen := map[string]bool{}
	n := 0
	for _, i := range idx {
		if fp := items[i].fp; fp == "" {
			n++
		} else if !seen[fp] {
			seen[fp] = true
			n++
		}
	}
	return n
}

// makeGroup builds a group: its files best first, then by path. The best
// gets the badge only when strictly better than all the others; exact
// copies have none.
func makeGroup(kind string, items []dupItem, idx []int) DupGroup {
	g := DupGroup{Kind: kind, Files: make([]DupFile, 0, len(idx))}
	for _, i := range idx {
		g.Files = append(g.Files, items[i].file)
		g.SizeBytes += items[i].file.SizeBytes
	}
	slices.SortFunc(g.Files, func(a, b DupFile) int {
		return cmp.Or(better(b, a), cmp.Compare(a.Path, b.Path), cmp.Compare(a.ID, b.ID))
	})
	if kind != DupExact && better(g.Files[0], g.Files[1]) > 0 {
		g.Files[0].Best = true
	}
	g.Title, g.Artist = g.Files[0].Title, g.Files[0].Artist
	return g
}
