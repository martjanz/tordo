package store

import (
	"sort"
	"strings"

	"tordo/internal/nameparse"
	"tordo/internal/natsort"
)

// TreeNode is a row of the folder tree: a folder plus the pass-through
// folders merged into it (see node).
type TreeNode struct {
	Path        string `json:"path"` // the deepest folder of the chain
	Label       string `json:"label"`
	Name        string `json:"name"` // name of the deepest folder
	Tracks      int    `json:"tracks"`
	SizeBytes   int64  `json:"sizeBytes"`
	Album       int64  `json:"album"`
	Collection  bool   `json:"collection"`
	HasChildren bool   `json:"hasChildren"`
}

// TreeSearch is what filtering the tree by folder name shows: the rows on
// the way to each match, by level, with every branch open.
type TreeSearch struct {
	Levels  map[string][]TreeNode `json:"levels"`  // level key → rows (only those leading to a match), natural order
	Matches []string              `json:"matches"` // paths of the rows that match, natural order of path
	More    bool                  `json:"more"`    // more folders matched than the limit
}

// TreeSearchLimit is the most folders a name filter reports.
const TreeSearchLimit = 200

type treeFolder struct {
	Path, Parent, Name string
	Tracks, Own        int   // recursive tracks; tracks directly in it
	Album              int64 // smallest album id whose dir it is, else 0
	Collection         bool
}

type folderTree struct {
	byPath   map[string]*treeFolder
	children map[string][]string
	roots    []string
}

func newFolderTree(fs []treeFolder) *folderTree {
	t := &folderTree{byPath: make(map[string]*treeFolder, len(fs)), children: map[string][]string{}}
	for i := range fs {
		f := &fs[i]
		t.byPath[f.Path] = f
		if f.Parent == "" {
			t.roots = append(t.roots, f.Path)
		} else {
			t.children[f.Parent] = append(t.children[f.Parent], f.Path)
		}
	}
	t.sortPaths(t.roots)
	for _, c := range t.children {
		t.sortPaths(c)
	}
	return t
}

func (t *folderTree) sortPaths(ps []string) {
	sort.SliceStable(ps, func(i, j int) bool {
		return natsort.Less(t.byPath[ps[i]].Name, t.byPath[ps[j]].Name)
	})
}

// node builds the row starting at folder start, following pass-through
// folders (no tracks of their own, a single subfolder) down the chain.
func (t *folderTree) node(start string) TreeNode {
	f := t.byPath[start]
	names := []string{f.Name}
	for f.Own == 0 && len(t.children[f.Path]) == 1 {
		f = t.byPath[t.children[f.Path][0]]
		names = append(names, f.Name)
	}
	return TreeNode{
		Path: f.Path, Label: strings.Join(names, " › "), Name: f.Name,
		Tracks: f.Tracks, Album: f.Album, Collection: f.Collection,
		HasChildren: len(t.children[f.Path]) > 0,
	}
}

func (t *folderTree) nodes(paths []string) []TreeNode {
	out := make([]TreeNode, 0, len(paths))
	for _, p := range paths {
		out = append(out, t.node(p))
	}
	return out
}

// level lists the rows of a level: "" is the top (the single root's
// subfolders, or the roots); any other key is a folder path.
func (t *folderTree) level(key string) ([]TreeNode, bool) {
	if key == "" {
		if len(t.roots) == 1 {
			return t.nodes(t.children[t.roots[0]]), true
		}
		return t.nodes(t.roots), true
	}
	if _, ok := t.byPath[key]; !ok {
		return nil, false
	}
	return t.nodes(t.children[key]), true
}

func matchesName(words []string, name string) bool {
	nw := strings.Fields(nameparse.Norm(name))
	last := words[len(words)-1]
	for _, w := range words[:len(words)-1] {
		found := false
		for _, x := range nw {
			if x == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, x := range nw {
		if strings.HasPrefix(x, last) {
			return true
		}
	}
	return false
}

// search finds the folders whose name matches q (every word but the last
// equals a word of the name, the last is a prefix of one) and the rows
// leading to them.
func (t *folderTree) search(q string, limit int) TreeSearch {
	res := TreeSearch{Levels: map[string][]TreeNode{}, Matches: []string{}}
	words := strings.Fields(nameparse.Norm(q))
	if len([]rune(strings.Join(words, ""))) < 2 {
		return res
	}
	var found []string
	for p, f := range t.byPath {
		if f.Parent == "" && len(t.roots) == 1 {
			continue
		}
		if matchesName(words, f.Name) {
			found = append(found, p)
		}
	}
	sort.Slice(found, func(i, j int) bool { return natsort.Less(found[i], found[j]) })
	if len(found) > limit {
		found, res.More = found[:limit], true
	}

	levels := map[string][]TreeNode{}
	keep := map[string]map[string]bool{}
	matched := map[string]bool{}
	for _, m := range found {
		key := ""
		for {
			nodes, ok := levels[key]
			if !ok {
				nodes, _ = t.level(key)
				levels[key] = nodes
			}
			var next string
			hit := false
			for _, n := range nodes {
				if n.Path == m || strings.HasPrefix(n.Path, m+"/") {
					t.record(keep, key, n.Path)
					matched[n.Path] = true
					hit = true
					break
				}
				if strings.HasPrefix(m, n.Path+"/") {
					t.record(keep, key, n.Path)
					next, hit = n.Path, true
					break
				}
			}
			if !hit || next == "" {
				break
			}
			key = next
		}
	}
	for key, set := range keep {
		rows := []TreeNode{}
		for _, n := range levels[key] {
			if set[n.Path] {
				rows = append(rows, n)
			}
		}
		res.Levels[key] = rows
	}
	for p := range matched {
		res.Matches = append(res.Matches, p)
	}
	sort.Slice(res.Matches, func(i, j int) bool { return natsort.Less(res.Matches[i], res.Matches[j]) })
	return res
}

func (t *folderTree) record(keep map[string]map[string]bool, key, p string) {
	if keep[key] == nil {
		keep[key] = map[string]bool{}
	}
	keep[key][p] = true
}

func (s *Store) loadFolderTree() (*folderTree, error) {
	var fs []treeFolder
	idx := map[string]int{}
	rows, err := s.db.Query(`SELECT path, parent, name, tracks, ` + isCollection + ` FROM folders`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f treeFolder
		if err := rows.Scan(&f.Path, &f.Parent, &f.Name, &f.Tracks, &f.Collection); err != nil {
			rows.Close()
			return nil, err
		}
		idx[f.Path] = len(fs)
		fs = append(fs, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT dir, COUNT(*) FROM tracks GROUP BY dir`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d string
		var n int
		if err := rows.Scan(&d, &n); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := idx[d]; ok {
			fs[i].Own = n
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT dir, MIN(id) FROM albums GROUP BY dir`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d string
		var id int64
		if err := rows.Scan(&d, &id); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := idx[d]; ok {
			fs[i].Album = id
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	return newFolderTree(fs), nil
}

func (s *Store) fillSizes(nodes []TreeNode) error {
	for i := range nodes {
		size, _, err := s.folderStats(nodes[i].Path)
		if err != nil {
			return err
		}
		nodes[i].SizeBytes = size
	}
	return nil
}

// FolderTree lists the rows of level p ("" is the top). false: p is not a
// folder of the catalog.
func (s *Store) FolderTree(p string) ([]TreeNode, bool, error) {
	t, err := s.loadFolderTree()
	if err != nil {
		return nil, false, err
	}
	nodes, ok := t.level(p)
	if !ok {
		return nil, false, nil
	}
	return nodes, true, s.fillSizes(nodes)
}

// SearchFolderTree filters the tree by folder name.
func (s *Store) SearchFolderTree(q string) (TreeSearch, error) {
	t, err := s.loadFolderTree()
	if err != nil {
		return TreeSearch{}, err
	}
	res := t.search(q, TreeSearchLimit)
	for _, nodes := range res.Levels {
		if err := s.fillSizes(nodes); err != nil {
			return TreeSearch{}, err
		}
	}
	return res, nil
}
