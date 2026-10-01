package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/covers"
	"tordo/internal/grouping"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// mediaServer catalogs an album with an MP3 and a WMA track whose files
// exist under <tmp>/musica, served from the app folder <tmp>/tordo.
func mediaServer(t *testing.T, ffmpeg string) (*Server, []int64) {
	t.Helper()
	base := t.TempDir()
	app := filepath.Join(base, "tordo")
	files := map[string]audiofile.Info{
		"Virus - Locura/01.mp3": {Format: "mp3", Codec: "mp3", DurationMs: 1000,
			Tags: audiofile.Tags{Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", Track: 1}},
		"Virus - Locura/02.wma": {Format: "asf", Codec: "wma", DurationMs: 1000,
			Tags: audiofile.Tags{Title: "Pecados para dos", Artist: "Virus", Album: "Locura", Track: 2}},
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	var rows []store.FileRow
	for name := range files {
		p := filepath.Join(base, "musica", filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, store.FileRow{Path: "../musica/" + name, Size: 10, MTime: 1, Kind: "audio"})
	}
	roots := []string{"../musica"}
	if err := st.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	pending, _ := st.PendingReads(env)
	var rs []store.ReadResult
	for _, p := range pending {
		rs = append(rs, store.ReadResult{Target: p, Env: env, Fingerprint: p.Path, Info: files[p.Path[len("../musica/"):]]})
	}
	if err := st.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	tracks, images, _ := st.GroupingInput()
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
	items, _ := st.PlayFolder("../musica")
	ids := []int64{}
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return &Server{Store: st, AppDir: app, Streamer: stream.Streamer{FFmpeg: ffmpeg}, Covers: covers.New(st, app, "")}, ids
}

type playResponse struct {
	Items []struct {
		store.PlayItem
		MIME       string `json:"mime"`
		Transcoded bool   `json:"transcoded"`
	} `json:"items"`
	Skipped   int  `json:"skipped"`
	Truncated int  `json:"truncated"`
	FFmpeg    bool `json:"ffmpeg"`
}

func TestPlayEndpoint(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	var p playResponse
	decode(t, do(t, h, "GET", "/api/play?folder=../musica", ""), &p)
	if len(p.Items) != 1 || p.Skipped != 1 || p.FFmpeg || p.Items[0].Title != "Una luna de miel en la mano" ||
		p.Items[0].MIME != "audio/mpeg" || p.Items[0].Transcoded {
		t.Fatalf("without ffmpeg: %+v", p)
	}
	s.Streamer.FFmpeg = "ffmpeg"
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?album="+itoa(onlyAlbum(t, s)), ""), &p)
	if len(p.Items) != 2 || p.Skipped != 0 || p.Truncated != 0 || !p.FFmpeg || !p.Items[1].Transcoded || p.Items[1].MIME != "audio/mpeg" {
		t.Fatalf("with ffmpeg: %+v", p)
	}
	maxPlay := MaxPlay
	MaxPlay = 1
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?album="+itoa(onlyAlbum(t, s)), ""), &p)
	MaxPlay = maxPlay
	if len(p.Items) != 1 || p.Truncated != 1 {
		t.Fatalf("capped: %d items, truncated %d", len(p.Items), p.Truncated)
	}
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?track="+itoa(ids[1])+"&track="+itoa(ids[0])+"&shuffle=0", ""), &p)
	if len(p.Items) != 2 || p.Items[0].ID != ids[1] {
		t.Fatalf("tracks: %+v", p)
	}
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?folder=../musica&shuffle=1", ""), &p)
	if len(p.Items) != 2 {
		t.Fatalf("shuffled: %+v", p)
	}
	for _, bad := range []string{"/api/play", "/api/play?album=x", "/api/play?track=1&track=y"} {
		if rec := do(t, h, "GET", bad, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
}

// onlyAlbum returns the id of the catalog's only album.
func onlyAlbum(t *testing.T, s *Server) int64 {
	t.Helper()
	as, err := s.Store.Albums("")
	if err != nil || len(as) != 1 {
		t.Fatalf("albums %+v, err %v", as, err)
	}
	return as[0].ID
}

func TestStreamEndpoint(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	rec := do(t, h, "GET", "/stream/"+itoa(ids[0]), "", "Range", "bytes=0-3")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "0123" || rec.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("mp3: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if rec := do(t, h, "GET", "/stream/"+itoa(ids[1]), ""); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wma without ffmpeg: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/stream/9999", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown track: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/stream/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}
}

func TestCoverEndpoints(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	if rec := do(t, h, "GET", "/api/covers/track/"+itoa(ids[0]), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a track without a picture: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/covers/album/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}
	s.Covers = nil
	if rec := do(t, s.Handler(), "GET", "/api/covers/album/1", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("without covers: %d", rec.Code)
	}
}

func TestFolderArt(t *testing.T) {
	s, _ := mediaServer(t, "")
	var f struct {
		store.FolderView
		Art   map[string][]int64 `json:"art"`
		Trail []store.Crumb      `json:"trail"`
	}
	decode(t, do(t, s.Handler(), "GET", "/api/folders?path=../musica", ""), &f)
	if len(f.Children) != 1 || len(f.Art["../musica/Virus - Locura"]) != 1 || len(f.Trail) != 1 || f.Trail[0].Name != "musica" {
		t.Fatalf("folder = %+v", f)
	}
}
