package server

import (
	"net/http"
	"strings"
	"testing"

	"tordo/internal/store"
)

const virus = "../musica/Virus%20-%20Locura"

func TestFolderTreeEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	rec := do(t, h, "GET", "/api/folders/tree", "")
	var rows []store.TreeNode
	decode(t, rec, &rows)
	if len(rows) != 1 || rows[0].Label != "Virus - Locura" || rows[0].Tracks != 2 ||
		rows[0].SizeBytes != 20 || rows[0].Album == 0 || rows[0].HasChildren {
		t.Fatalf("rows = %+v", rows)
	}
	rec = do(t, h, "GET", "/api/folders/tree?path="+virus, "")
	var none []store.TreeNode
	decode(t, rec, &none)
	if len(none) != 0 || !strings.HasPrefix(rec.Body.String(), "[") {
		t.Fatalf("leaf = %q", rec.Body.String())
	}
	if rec = do(t, h, "GET", "/api/folders/tree?path=../nope", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path = %d", rec.Code)
	}
}

func TestFolderTreeSearchEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var res store.TreeSearch
	decode(t, do(t, h, "GET", "/api/folders/tree/search?q=locu", ""), &res)
	if len(res.Matches) != 1 || res.Matches[0] != "../musica/Virus - Locura" || len(res.Levels[""]) != 1 {
		t.Fatalf("res = %+v", res)
	}
	rec := do(t, h, "GET", "/api/folders/tree/search?q=x", "")
	var empty store.TreeSearch
	decode(t, rec, &empty)
	if rec.Code != http.StatusOK || len(empty.Matches) != 0 {
		t.Fatalf("short q = %d %+v", rec.Code, empty)
	}
}

func TestFolderFilesPlayable(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	get := func() map[string]folderFile {
		var v struct {
			SizeBytes int64        `json:"sizeBytes"`
			Files     []folderFile `json:"files"`
		}
		decode(t, do(t, h, "GET", "/api/folders?path="+virus, ""), &v)
		if v.SizeBytes != 20 {
			t.Fatalf("sizeBytes = %d", v.SizeBytes)
		}
		m := map[string]folderFile{}
		for _, f := range v.Files {
			m[f.Name] = f
		}
		return m
	}
	m := get()
	if !m["01.mp3"].Track || !m["01.mp3"].Playable || !m["02.wma"].Track || m["02.wma"].Playable {
		t.Fatalf("files = %+v", m)
	}
	s.Streamer.FFmpeg = "ffmpeg"
	m = get()
	if !m["01.mp3"].Playable || !m["02.wma"].Playable {
		t.Fatalf("with ffmpeg: %+v", m)
	}
}
