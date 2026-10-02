package store

import (
	"fmt"
	"reflect"
	"testing"
)

func chainFolders() []treeFolder {
	return []treeFolder{
		{Path: "r", Name: "r", Tracks: 3},
		{Path: "r/a", Parent: "r", Name: "a", Tracks: 2},
		{Path: "r/a/b", Parent: "r/a", Name: "b", Tracks: 2},
		{Path: "r/a/b/c", Parent: "r/a/b", Name: "c", Tracks: 2, Own: 2},
		{Path: "r/z", Parent: "r", Name: "z", Tracks: 1, Own: 1},
		{Path: "r/z/y", Parent: "r/z", Name: "y"},
	}
}

func TestTreeChains(t *testing.T) {
	tr := newFolderTree(chainFolders())
	got, ok := tr.level("")
	if !ok || len(got) != 2 {
		t.Fatalf("level(\"\") = %+v, %v", got, ok)
	}
	if n := got[0]; n.Path != "r/a/b/c" || n.Label != "a › b › c" || n.Name != "c" || n.HasChildren {
		t.Errorf("row 0 = %+v", n)
	}
	if n := got[1]; n.Path != "r/z" || n.Label != "z" || !n.HasChildren {
		t.Errorf("row 1 = %+v", n)
	}
	got, ok = tr.level("r/a")
	if !ok || len(got) != 1 || got[0].Path != "r/a/b/c" || got[0].Label != "b › c" {
		t.Errorf("level(r/a) = %+v, %v", got, ok)
	}
	if _, ok := tr.level("nope"); ok {
		t.Error("level(nope) should be unknown")
	}
	// Zero own tracks but two children: not merged.
	tr = newFolderTree([]treeFolder{
		{Path: "r", Name: "r"},
		{Path: "r/p", Parent: "r", Name: "p"},
		{Path: "r/p/x", Parent: "r/p", Name: "x", Own: 1},
		{Path: "r/p/y", Parent: "r/p", Name: "y", Own: 1},
	})
	got, _ = tr.level("")
	if len(got) != 1 || got[0].Path != "r/p" || got[0].Label != "p" || !got[0].HasChildren {
		t.Errorf("two-children level = %+v", got)
	}
}

func TestTreeSeveralRoots(t *testing.T) {
	tr := newFolderTree([]treeFolder{
		{Path: "r1", Name: "r1"},
		{Path: "r1/a", Parent: "r1", Name: "a", Own: 1},
		{Path: "r2", Name: "r2", Own: 1},
		{Path: "r2/b", Parent: "r2", Name: "b", Own: 1},
	})
	got, _ := tr.level("")
	if len(got) != 2 || got[0].Path != "r1/a" || got[0].Label != "r1 › a" || got[1].Path != "r2" || got[1].Label != "r2" {
		t.Errorf("level(\"\") = %+v", got)
	}
}

func TestTreeSearchMore(t *testing.T) {
	fs := []treeFolder{{Path: "r", Name: "r"}}
	for i := 0; i < 205; i++ {
		p := fmt.Sprintf("r/disco %03d", i)
		fs = append(fs, treeFolder{Path: p, Parent: "r", Name: fmt.Sprintf("disco %03d", i), Own: 1})
	}
	res := newFolderTree(fs).search("disco", 200)
	if len(res.Matches) != 200 || !res.More {
		t.Errorf("matches=%d more=%v", len(res.Matches), res.More)
	}
}

func TestFolderTree(t *testing.T) {
	s := explorer(t)
	top, ok, err := s.FolderTree("")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	labels := []string{}
	for _, n := range top {
		labels = append(labels, n.Label)
	}
	want := []string{"Brasil › Música Popular", "Soulseek › complete", "Uruguay › Jaime Roos"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %q", labels)
	}
	if n := top[1]; n.Path != sk || n.Tracks != 6 || n.SizeBytes != 62 || !n.HasChildren || n.Album != 0 {
		t.Errorf("soulseek row = %+v", n)
	}
	kids, ok, err := s.FolderTree(sk)
	if err != nil || !ok || len(kids) != 2 {
		t.Fatalf("%+v %v %v", kids, ok, err)
	}
	if n := kids[0]; n.Label != "benteveo › Zitarrosa › Canta" || n.Path != sk+"/benteveo/Zitarrosa/Canta" ||
		n.Album == 0 || n.HasChildren || n.SizeBytes != 20 {
		t.Errorf("canta row = %+v", n)
	}
	if n := kids[1]; n.Label != "Low_ID" || !n.HasChildren || n.Tracks != 4 {
		t.Errorf("low_id row = %+v", n)
	}
	if _, ok, err := s.FolderTree("../nope"); err != nil || ok {
		t.Errorf("nope: %v %v", ok, err)
	}
}

func TestSearchFolderTree(t *testing.T) {
	s := explorer(t)
	canta := sk + "/benteveo/Zitarrosa/Canta"
	search := func(q string) TreeSearch {
		t.Helper()
		r, err := s.SearchFolderTree(q)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	paths := func(ns []TreeNode) []string {
		out := []string{}
		for _, n := range ns {
			out = append(out, n.Path)
		}
		return out
	}

	r := search("zitar")
	if !reflect.DeepEqual(r.Matches, []string{canta}) ||
		!reflect.DeepEqual(paths(r.Levels[""]), []string{sk}) ||
		!reflect.DeepEqual(paths(r.Levels[sk]), []string{canta}) {
		t.Errorf("zitar = %+v", r)
	}
	if r.Levels[sk][0].SizeBytes != 20 {
		t.Errorf("size not filled: %+v", r.Levels[sk][0])
	}

	nfd := "../musica/Brasil/Música Popular"
	r = search("musica")
	if !reflect.DeepEqual(r.Matches, []string{nfd}) || len(r.Levels[""]) != 1 || r.Levels[""][0].Path != nfd {
		t.Errorf("musica = %+v", r)
	}

	crusades := sk + "/Low_ID/music of the crusades"
	for _, q := range []string{"music of", "of music"} {
		if r := search(q); !reflect.DeepEqual(r.Matches, []string{crusades}) {
			t.Errorf("%q = %+v", q, r.Matches)
		}
	}
	if r := search("lhasa sel"); len(r.Matches) != 1 {
		t.Errorf("lhasa sel = %+v", r.Matches)
	}
	if r := search("lhas de"); len(r.Matches) != 0 {
		t.Errorf("lhas de = %+v", r.Matches)
	}
	r = search("m")
	if len(r.Matches) != 0 || len(r.Levels) != 0 {
		t.Errorf("m = %+v", r)
	}
	r = search("lo")
	if !reflect.DeepEqual(r.Matches, []string{sk + "/Low_ID"}) || len(r.Levels[sk]) != 1 {
		t.Errorf("lo = %+v", r)
	}
	r = search("complete")
	if !reflect.DeepEqual(r.Matches, []string{sk}) {
		t.Errorf("complete = %+v", r)
	}
}
