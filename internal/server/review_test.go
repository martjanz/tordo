package server

import (
	"strings"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/catalog"
)

// copiesServer holds Locura twice (the same fingerprints, two folders) and a
// WMA track in a third album.
func copiesServer(t *testing.T) *Server {
	t.Helper()
	info := func(title string, n int) audiofile.Info {
		return audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000,
			Tags: audiofile.Tags{Title: title, Artist: "Virus", Album: "Locura", Track: n}}
	}
	s, _ := mediaServerFingerprints(t, "", map[string]audiofile.Info{
		"A/01.mp3": info("Una luna", 1),
		"B/01.mp3": info("Una luna", 1),
		"C/01.wma": {Format: "asf", Codec: "wma", DurationMs: 5000,
			Tags: audiofile.Tags{Title: "Otra", Artist: "Soda", Album: "Dynamo", Track: 1}},
	}, func(path string) string {
		if strings.HasSuffix(path, "/01.mp3") {
			return "same"
		}
		return path
	})
	return s
}

type duplicatesResponse struct {
	Counts      struct{ Exact, Formats, Suspect int } `json:"counts"`
	Recoverable int64                                 `json:"recoverable"`
	Groups      []catalog.DupGroup                    `json:"groups"`
}

func TestReviewDuplicates(t *testing.T) {
	h := copiesServer(t).Handler()
	var d duplicatesResponse
	decode(t, do(t, h, "GET", "/api/review/duplicates", ""), &d)
	if d.Counts.Exact != 1 || d.Counts.Formats != 0 || d.Counts.Suspect != 0 || d.Recoverable <= 0 ||
		len(d.Groups) != 1 || len(d.Groups[0].Files) != 2 || d.Groups[0].Kind != catalog.DupExact {
		t.Fatalf("exact: %+v", d)
	}
	rec := do(t, h, "GET", "/api/review/duplicates?tipo=sospechas", "")
	if !strings.Contains(rec.Body.String(), `"groups":[]`) {
		t.Fatalf("suspects must be an empty list: %s", rec.Body)
	}
	var raro duplicatesResponse
	decode(t, do(t, h, "GET", "/api/review/duplicates?tipo=raro", ""), &raro)
	if len(raro.Groups) != 1 || raro.Groups[0].Kind != catalog.DupExact {
		t.Fatalf("unknown kind: %+v", raro)
	}
}

func TestReviewFormats(t *testing.T) {
	h := copiesServer(t).Handler()
	var c catalog.ToConvert
	decode(t, do(t, h, "GET", "/api/review/formats", ""), &c)
	if c.Files != 1 || len(c.Formats) != 1 || c.Formats[0].Key != "wma" || len(c.Items) != 1 || c.Items[0].Title != "Dynamo" {
		t.Fatalf("formats to convert: %+v", c)
	}
}

func TestAlbumVersions(t *testing.T) {
	s := copiesServer(t)
	h := s.Handler()
	c, err := s.catalog()
	if err != nil {
		t.Fatal(err)
	}
	d := c.Duplicates()
	a, b := d.Exact[0].Files[0].AlbumID, d.Exact[0].Files[1].AlbumID
	var page struct {
		Versions []catalog.Version `json:"versions"`
	}
	decode(t, do(t, h, "GET", "/api/albums/"+itoa(a), ""), &page)
	if len(page.Versions) != 1 || page.Versions[0].AlbumID != b || page.Versions[0].Kind != catalog.DupExact || page.Versions[0].Shared != 1 {
		t.Fatalf("versions: %+v", page.Versions)
	}
	other := int64(0)
	for _, f := range c.Tracks {
		if f.AlbumID != a && f.AlbumID != b {
			other = f.AlbumID
		}
	}
	rec := do(t, h, "GET", "/api/albums/"+itoa(other), "")
	if !strings.Contains(rec.Body.String(), `"versions":[]`) {
		t.Fatalf("an album without copies: %s", rec.Body)
	}
}
