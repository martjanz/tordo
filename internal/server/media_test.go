package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/covers"
	"tordo/internal/grouping"
	"tordo/internal/loudness"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// mediaServer catalogs an album with an MP3 and a WMA track whose files
// exist under <tmp>/musica, served from the app folder <tmp>/tordo.
func mediaServer(t *testing.T, ffmpeg string) (*Server, []int64) {
	return mediaServerFiles(t, ffmpeg, map[string]audiofile.Info{
		"Virus - Locura/01.mp3": {Format: "mp3", Codec: "mp3", DurationMs: 1000,
			Tags: audiofile.Tags{Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", Track: 1}},
		"Virus - Locura/02.wma": {Format: "asf", Codec: "wma", DurationMs: 1000,
			Tags: audiofile.Tags{Title: "Pecados para dos", Artist: "Virus", Album: "Locura", Track: 2}},
	})
}

// mediaServerFiles builds a server over the given files (paths relative to
// the music folder); the returned ids are in folder order.
func mediaServerFiles(t *testing.T, ffmpeg string, files map[string]audiofile.Info) (*Server, []int64) {
	t.Helper()
	base := t.TempDir()
	app := filepath.Join(base, "tordo")
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

// fakeLoudness records what the server asks to measure first.
type fakeLoudness struct {
	mu          sync.Mutex
	prioritized [][]int64
}

func (f *fakeLoudness) Prioritize(ids []int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prioritized = append(f.prioritized, slices.Clone(ids))
}

func (f *fakeLoudness) Status() loudness.Status { return loudness.Status{State: "idle"} }

func (f *fakeLoudness) calls() [][]int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.prioritized)
}

func TestPlayGains(t *testing.T) {
	s, ids := mediaServer(t, "ffmpeg") // with ffmpeg the WMA track is playable
	fake := &fakeLoudness{}
	s.Loudness = fake
	h := s.Handler()
	mp3, wma := ids[0], ids[1]
	v := -10.0
	if err := s.Store.SaveLoudness("../musica/Virus - Locura/01.mp3", &v, ""); err != nil {
		t.Fatal(err)
	}

	var p struct {
		Items []struct {
			ID        int64    `json:"id"`
			TrackGain *float64 `json:"trackGain"`
			AlbumGain *float64 `json:"albumGain"`
		} `json:"items"`
	}
	decode(t, do(t, h, "GET", "/api/play?album="+itoa(onlyAlbum(t, s)), ""), &p)
	if len(p.Items) != 2 {
		t.Fatalf("items = %+v", p)
	}
	for _, it := range p.Items {
		if it.AlbumGain != nil {
			t.Fatalf("album gain with an unmeasured track: %+v", p)
		}
		if it.ID == mp3 && (it.TrackGain == nil || *it.TrackGain != -8) {
			t.Fatalf("mp3 = %+v", it)
		}
		if it.ID == wma && it.TrackGain != nil {
			t.Fatalf("wma = %+v", it)
		}
	}
	if got := fake.calls(); len(got) != 1 || !slices.Equal(got[0], []int64{wma}) {
		t.Fatalf("prioritized = %v", got)
	}

	var gains map[string]loudness.TrackGain
	decode(t, do(t, h, "GET", "/api/gains?track="+itoa(mp3), ""), &gains)
	if g := gains[itoa(mp3)]; len(gains) != 1 || g.Track == nil || *g.Track != -8 || g.Album != nil {
		t.Fatalf("gains = %+v", gains)
	}
	fake.mu.Lock()
	fake.prioritized = nil
	fake.mu.Unlock()
	decode(t, do(t, h, "GET", "/api/gains?track="+itoa(wma)+"&track="+itoa(mp3), ""), &gains)
	if got := fake.calls(); len(got) != 1 || !slices.Equal(got[0], []int64{wma}) {
		t.Fatalf("prioritized by /api/gains = %v", got)
	}

	var q []string
	for i := 1; i <= 101; i++ {
		q = append(q, "track="+itoa(int64(i)))
	}
	if rec := do(t, h, "GET", "/api/gains?"+strings.Join(q, "&"), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("101 ids: %d", rec.Code)
	}
	for _, bad := range []string{"/api/gains", "/api/gains?track=x"} {
		if rec := do(t, h, "GET", bad, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
	// With no runner the gains still come out and nothing is asked.
	s.Loudness = nil
	decode(t, do(t, h, "GET", "/api/gains?track="+itoa(mp3), ""), &gains)
}

func TestPlayUnmeasurable(t *testing.T) {
	s, ids := mediaServer(t, "ffmpeg")
	fake := &fakeLoudness{}
	s.Loudness = fake
	h := s.Handler()
	wma := ids[1]
	if err := s.Store.SaveLoudness("../musica/Virus - Locura/02.wma", nil, "roto"); err != nil {
		t.Fatal(err)
	}
	var p struct {
		Items []struct {
			ID           int64 `json:"id"`
			Unmeasurable bool  `json:"unmeasurable"`
		} `json:"items"`
	}
	decode(t, do(t, h, "GET", "/api/play?album="+itoa(onlyAlbum(t, s)), ""), &p)
	var found bool
	for _, it := range p.Items {
		if it.ID == wma {
			found = true
			if !it.Unmeasurable {
				t.Fatalf("wma = %+v", it)
			}
		} else if it.Unmeasurable {
			t.Fatalf("other track unmeasurable: %+v", it)
		}
	}
	if !found {
		t.Fatalf("items = %+v", p)
	}
	for _, call := range fake.calls() {
		if slices.Contains(call, wma) {
			t.Fatalf("the unmeasurable track was prioritized: %v", call)
		}
	}
}

func TestPlayPlaylistAndLiked(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	pid, err := s.Store.CreatePlaylist("Lista")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Store.AddToPlaylist(pid, ids); err != nil {
		t.Fatal(err)
	}
	var p playResponse
	decode(t, do(t, h, "GET", "/api/play?playlist="+itoa(pid), ""), &p)
	if len(p.Items) != 1 || p.Skipped != 1 {
		t.Fatalf("playlist: %+v", p)
	}
	if ok, err := s.Store.LikeTrack(ids[0], true); !ok || err != nil {
		t.Fatalf("like: %v %v", ok, err)
	}
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?liked=1", ""), &p)
	if len(p.Items) != 1 || p.Items[0].ID != ids[0] {
		t.Fatalf("liked: %+v", p)
	}
	if rec := do(t, h, "GET", "/api/play?playlist=999", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown playlist: %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/play", "")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "playlist o liked") {
		t.Errorf("nothing: %d %s", rec.Code, rec.Body)
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

func TestGainsCompilationHasNoAlbumGain(t *testing.T) {
	mp3 := func(artist, album, title string, n int, comp bool) audiofile.Info {
		return audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000,
			Tags: audiofile.Tags{Artist: artist, Album: album, Title: title, Track: n, Compilation: comp}}
	}
	files := map[string]audiofile.Info{
		"Selección de murgas/01.mp3": mp3("Agarrate Catalina", "Selección de murgas", "Uno", 1, true),
		"Selección de murgas/02.mp3": mp3("Falta y Resto", "Selección de murgas", "Dos", 2, true),
		"Virus - Locura/01.mp3":      mp3("Virus", "Locura", "Una luna", 1, false),
		"Virus - Locura/02.mp3":      mp3("Virus", "Locura", "Pecados", 2, false),
	}
	s, _ := mediaServerFiles(t, "", files)
	h := s.Handler()
	lufs := -24.0
	for name := range files {
		v := lufs
		lufs += 6
		if err := s.Store.SaveLoudness("../musica/"+name, &v, ""); err != nil {
			t.Fatal(err)
		}
	}
	type gain struct {
		ID        int64    `json:"id"`
		TrackGain *float64 `json:"trackGain"`
		AlbumGain *float64 `json:"albumGain"`
	}
	check := func(what string, g gain, wantAlbum bool) {
		t.Helper()
		if g.TrackGain == nil || (g.AlbumGain != nil) != wantAlbum {
			t.Errorf("%s %d: track %v album %v, want album gain %v", what, g.ID, g.TrackGain, g.AlbumGain, wantAlbum)
		}
	}
	var p struct{ Items []gain }
	var ids []string
	for _, c := range []struct {
		folder string
		want   bool
	}{{"../musica/Selección de murgas", false}, {"../musica/Virus - Locura", true}} {
		decode(t, do(t, h, "GET", "/api/play?"+url.Values{"folder": {c.folder}}.Encode(), ""), &p)
		if len(p.Items) != 2 {
			t.Fatalf("%s: %+v", c.folder, p)
		}
		for _, it := range p.Items {
			check("play", it, c.want)
			ids = append(ids, "track="+itoa(it.ID))
		}
	}
	var gains map[string]loudness.TrackGain
	decode(t, do(t, h, "GET", "/api/gains?"+strings.Join(ids, "&"), ""), &gains)
	if len(gains) != 4 {
		t.Fatalf("gains = %+v", gains)
	}
	for i, id := range ids { // the first two are the compilation's
		g := gains[strings.TrimPrefix(id, "track=")]
		check("gains", gain{TrackGain: g.Track, AlbumGain: g.Album}, i >= 2)
	}
}
