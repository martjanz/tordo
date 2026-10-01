package identify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"tordo/internal/audiofile"
	"tordo/internal/discogs"
	"tordo/internal/genres"
	"tordo/internal/grouping"
	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

var roots = []string{"../musica"}

type file struct {
	path string
	fp   string
	ms   int64
	tags audiofile.Tags
}

func newCatalog(t *testing.T, files ...file) *store.Store {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	load(t, st, files...)
	return st
}

// load makes files the catalog's only present files and regroups, as a scan would.
func load(t *testing.T, st *store.Store, files ...file) {
	t.Helper()
	var rows []store.FileRow
	byPath := map[string]file{}
	for _, f := range files {
		rows = append(rows, store.FileRow{Path: f.path, Size: 1, MTime: 1, Kind: "audio"})
		byPath[f.path] = f
	}
	if err := st.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	pending, err := st.PendingReads(env)
	if err != nil {
		t.Fatal(err)
	}
	var rs []store.ReadResult
	for _, p := range pending {
		f := byPath[p.Path]
		rs = append(rs, store.ReadResult{Target: p, Env: env, Fingerprint: f.fp,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: f.ms, Tags: f.tags}})
	}
	if err := st.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	tracks, images, err := st.GroupingInput()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
}

func signosFiles() []file {
	tags := func(title string, n int) audiofile.Tags {
		return audiofile.Tags{Title: title, Artist: "Soda Stereo", Album: "Signos", Track: n, Year: 1986, Genres: []string{"Rock nacional"}}
	}
	return []file{
		{signosDir + "/01.mp3", "fp-1", 231500, tags("Tactica y estrategia", 1)},
		{signosDir + "/02.mp3", "fp-2", 209000, tags("Sin sobresalt", 2)},
	}
}

func signosMB() *fakeMB {
	return &fakeMB{
		search:   map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {signosHit()}},
		releases: map[string]musicbrainz.Release{signosID: signosRelease()},
		artists:  map[string]musicbrainz.Artist{sodaID: {ID: sodaID, Name: "Soda Stereo", Type: "Group", Country: "AR", Area: "Argentina"}},
		covers:   map[string][]byte{"release/" + signosID: []byte("jpeg")},
	}
}

func signosDiscogs() *fakeDiscogs {
	return &fakeDiscogs{releases: map[int]discogs.Info{1234: {ID: 1234, MasterID: 99, Genres: []string{"Rock"},
		Styles: []string{"Pop Rock", "New Wave"}}}}
}

type retries struct {
	mu     sync.Mutex
	delays []time.Duration
}

func newRunner(t *testing.T, st *store.Store, mb *fakeMB, dg *fakeDiscogs) (*Runner, *retries) {
	rt := &retries{}
	r := &Runner{AppDir: t.TempDir(), Store: st, MB: mb,
		After: func(d time.Duration, f func()) func() {
			rt.mu.Lock()
			rt.delays = append(rt.delays, d)
			rt.mu.Unlock()
			return func() {}
		}}
	if dg != nil { // a nil *fakeDiscogs in the interface would not read as "no Discogs"
		r.Discogs = dg
	}
	t.Cleanup(r.Close)
	return r, rt
}

func onlyAlbum(t *testing.T, st *store.Store) store.AlbumDetail {
	t.Helper()
	as, err := st.Albums("")
	if err != nil || len(as) != 1 {
		t.Fatalf("albums = %+v, err %v", as, err)
	}
	a, _, err := st.Album(as[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRunIdentifiesAndEnriches(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb, dg := signosMB(), signosDiscogs()
	r, _ := newRunner(t, st, mb, dg)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if a.Status != store.StatusAuto || a.ReleaseID != signosID || a.Title != "Signos" || a.Country != "AR" || a.OriginalYear != 1986 {
		t.Fatalf("album = %+v", a.AlbumView)
	}
	wantGenres := []store.GenreLink{{Genre: "Rock", Style: "New Wave", Source: "discogs"}, {Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}
	if !reflect.DeepEqual(a.Genres, wantGenres) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if a.CoverCache != "cache/covers/"+signosID+".jpg" {
		t.Fatalf("cover = %q", a.CoverCache)
	}
	if b, err := os.ReadFile(filepath.Join(r.AppDir, "cache", "covers", signosID+".jpg")); err != nil || string(b) != "jpeg" {
		t.Fatalf("cover file %q, err %v", b, err)
	}
	if tr := a.Tracks[1]; tr.Title != "Sin sobresaltos" || tr.TagTitle != "Sin sobresalt" || tr.RecordingID != "rec-2" {
		t.Fatalf("track = %+v", tr)
	}
	if s := r.Status(); s.State != StateIdle || s.Identified != 1 || s.Discogs != DiscogsNoToken {
		t.Fatalf("status = %+v", s)
	}
	if un, _ := st.UnmappedGenres(); len(un) != 0 {
		t.Fatalf("unmapped = %+v", un)
	}
}

func TestSecondRunMakesNoRequests(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb, dg := signosMB(), signosDiscogs()
	r, _ := newRunner(t, st, mb, dg)
	for range 2 {
		if err := r.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(mb.calls) != 4 || len(dg.calls) != 1 { // search, release, artist, cover; Discogs release
		t.Fatalf("MusicBrainz calls %q, Discogs calls %q", mb.calls, dg.calls)
	}
}

func TestUnmatchedGoesToReview(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := &fakeMB{search: map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {otherHit()}}}
	r, _ := newRunner(t, st, mb, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	un, err := st.Unidentified()
	if err != nil || len(un) != 1 || len(un[0].Candidates) != 1 || un[0].Candidates[0].ReleaseID != otherID {
		t.Fatalf("unidentified = %+v, err %v", un, err)
	}
	if c, _ := st.Counts(); c.Unidentified != 1 || c.Identified != 0 {
		t.Fatalf("counts = %+v", c)
	}
}

func TestOfflineSchedulesRetry(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	mb.err = fmt.Errorf("%w: prueba", httpx.ErrOffline)
	r, rt := newRunner(t, st, mb, nil)
	for range 2 {
		if err := r.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if r.Status().State != StateOffline || !reflect.DeepEqual(rt.delays, []time.Duration{5 * time.Minute, 10 * time.Minute}) {
		t.Fatalf("state %q, retries %v", r.Status().State, rt.delays)
	}
	if a := onlyAlbum(t, st); a.Status != store.StatusPending {
		t.Fatalf("status = %q", a.Status)
	}
}

func TestCorrectionSurvivesMoveAndRetag(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := &fakeMB{releases: map[string]musicbrainz.Release{signosID: signosRelease()}}
	r, _ := newRunner(t, st, mb, nil)
	ctx := context.Background()
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if a.Status != store.StatusUnmatched {
		t.Fatalf("status = %q", a.Status)
	}
	if err := r.SetRelease(ctx, a.ID, signosID); err != nil {
		t.Fatal(err)
	}
	if a = onlyAlbum(t, st); a.Status != store.StatusManual || a.Title != "Signos" || a.Confidence != 1 {
		t.Fatalf("after correction: %+v", a.AlbumView)
	}
	moved := signosFiles()
	for i := range moved {
		moved[i].path = "../musica/Ordenar/cosas de soda/" + path.Base(moved[i].path)
		moved[i].tags.Album, moved[i].tags.Artist = "Disco de Soda", "Soda"
	}
	load(t, st, moved...)
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	a = onlyAlbum(t, st)
	if a.Dir != "../musica/Ordenar/cosas de soda" || a.Status != store.StatusManual || a.ReleaseID != signosID || a.Title != "Signos" {
		t.Fatalf("after moving and re-tagging: %+v", a.AlbumView)
	}
	searches := mb.count("search ")
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if mb.count("search ") != searches {
		t.Fatal("a corrected album was searched again")
	}
}

func TestSetReleaseErrors(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	id := onlyAlbum(t, st).ID
	if err := r.SetRelease(context.Background(), id, "nada"); !errors.Is(err, musicbrainz.ErrInvalidID) {
		t.Fatalf("err = %v", err)
	}
	if err := r.SetRelease(context.Background(), id, otherID); !errors.Is(err, musicbrainz.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := r.SetRelease(context.Background(), 9999, signosID); !errors.Is(err, ErrNoAlbum) {
		t.Fatalf("err = %v", err)
	}
}

func TestIgnoreAndReset(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	id := onlyAlbum(t, st).ID
	if err := r.Ignore(id); err != nil {
		t.Fatal(err)
	}
	if un, _ := st.Unidentified(); onlyAlbum(t, st).Status != store.StatusIgnored || len(un) != 0 {
		t.Fatalf("ignored album still in review: %+v", un)
	}
	if err := r.Reset(id); err != nil {
		t.Fatal(err)
	}
	if un, _ := st.Unidentified(); onlyAlbum(t, st).Status != store.StatusUnmatched || len(un) != 1 {
		t.Fatalf("reset album not back in review: %+v", un)
	}
	if err := r.Ignore(9999); !errors.Is(err, ErrNoAlbum) {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectionStaysOutOfReview(t *testing.T) {
	d := "../musica/baulera"
	st := newCatalog(t,
		file{d + "/a.mp3", "fp-a", 1000, audiofile.Tags{Title: "A", Artist: "Uno", Album: "Uno"}},
		file{d + "/b.mp3", "fp-b", 1000, audiofile.Tags{Title: "B", Artist: "Dos", Album: "Dos"}},
		file{d + "/c.mp3", "fp-c", 1000, audiofile.Tags{Title: "C", Artist: "Tres", Album: "Tres"}})
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if un, _ := st.Unidentified(); a.Status != store.StatusSelection || !a.Selection || len(un) != 0 {
		t.Fatalf("album %+v, review %+v", a.AlbumView, un)
	}
}

func TestTagGenresAndAliases(t *testing.T) {
	files := signosFiles()
	for i := range files {
		files[i].tags.Genres = []string{"Rock nacional", "Chamamé"}
	}
	st := newCatalog(t, files...)
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a := onlyAlbum(t, st); !reflect.DeepEqual(a.Genres, []store.GenreLink{{Genre: "Rock", Source: "tag"}}) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if un, _ := st.UnmappedGenres(); !reflect.DeepEqual(un, []store.UnmappedGenre{{Raw: "Chamamé", Albums: 1}}) {
		t.Fatalf("unmapped = %+v", un)
	}
	if err := st.SetGenreAlias(store.GenreAlias{Raw: genres.Key("Chamamé"), Genre: "Folk, World, & Country", Style: "Folk"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	want := []store.GenreLink{{Genre: "Folk, World, & Country", Style: "Folk", Source: "tag"}, {Genre: "Rock", Source: "tag"}}
	if a := onlyAlbum(t, st); !reflect.DeepEqual(a.Genres, want) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if un, _ := st.UnmappedGenres(); len(un) != 0 {
		t.Fatalf("unmapped = %+v", un)
	}
}

func TestDiscogsSearchWithToken(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	rel := signosRelease()
	rel.DiscogsURLs = nil
	mb.releases[signosID] = rel
	dg := &fakeDiscogs{token: true, search: map[string][]discogs.Result{"Soda Stereo|Signos": {{ID: 77, Artist: "Soda Stereo",
		Title: "Signos", Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}}}}
	r, _ := newRunner(t, st, mb, dg)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a := onlyAlbum(t, st); !reflect.DeepEqual(a.Genres, []store.GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if mb.count("group "+signosGroup) != 1 || r.Status().Discogs != DiscogsOK {
		t.Fatalf("calls %q, status %+v", mb.calls, r.Status())
	}
}

func TestDiscogsBadTokenOnlyDisablesDiscogs(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	rel := signosRelease()
	rel.DiscogsURLs = nil
	mb.releases[signosID] = rel
	dg := &fakeDiscogs{token: true, err: discogs.ErrUnauthorized}
	r, _ := newRunner(t, st, mb, dg)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if a.Status != store.StatusAuto || r.Status().Discogs != DiscogsBadToken ||
		!reflect.DeepEqual(a.Genres, []store.GenreLink{{Genre: "Rock", Source: "mb"}}) {
		t.Fatalf("album %+v, status %+v", a.AlbumView, r.Status())
	}
}

func TestBadTokenStopsDiscogsCallsEvenWithURLs(t *testing.T) {
	// Album with a release that has DiscogsURLs; run twice to verify
	// token rejection stops Discogs calls on both runs
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	rel := signosRelease() // Has DiscogsURLs: "https://www.discogs.com/release/1234-..."
	mb.releases[signosID] = rel
	// Discogs rejects the token on any call
	dg := &fakeDiscogs{token: true, err: discogs.ErrUnauthorized}
	r, _ := newRunner(t, st, mb, dg)
	// First run identifies the album and tries to enrich with Discogs,
	// which fails with 401
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Second run should not make any more Discogs calls because badToken
	// prevents needsDiscogs from returning true
	firstRunCalls := len(dg.calls)
	if firstRunCalls != 1 {
		t.Fatalf("expected 1 Discogs call in first run (the 401), got %d: %q", firstRunCalls, dg.calls)
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(dg.calls) != 1 {
		t.Fatalf("expected no additional Discogs calls in second run, got %d total: %q", len(dg.calls), dg.calls)
	}
	a := onlyAlbum(t, st)
	if a.Status != store.StatusAuto || a.ReleaseID != signosID {
		t.Fatalf("album should be auto-identified, got status=%q release=%q", a.Status, a.ReleaseID)
	}
	if r.Status().Discogs != DiscogsBadToken {
		t.Fatalf("status.Discogs should be %q, got %q", DiscogsBadToken, r.Status().Discogs)
	}
}

func TestRunAppliesBeforeNetworkWork(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	r, _ := newRunner(t, st, mb, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	// A scan regroups (wiping the MusicBrainz titles) and finds a new album.
	other := file{"../musica/Otro/01.mp3", "fp-o", 1000, audiofile.Tags{Title: "Uno", Artist: "Otro", Album: "Otro"}}
	load(t, st, append(signosFiles(), other)...)
	var during store.AlbumDetail
	mb.onSearch = func() {
		as, _ := st.Albums("")
		for _, a := range as {
			if a.Title == "Signos" {
				during, _, _ = st.Album(a.ID)
			}
		}
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(during.Tracks) != 2 || during.Tracks[1].Title != "Sin sobresaltos" || len(during.Genres) == 0 {
		t.Fatalf("identity not restored before the first request: %+v", during)
	}
}

// blockingSearch makes the fake's searches announce themselves on started and
// wait for a token on release. The searches fail offline, so every run
// searches again.
func blockingSearch(mb *fakeMB) (started chan struct{}, release chan struct{}) {
	started, release = make(chan struct{}, 10), make(chan struct{}, 10)
	mb.err = fmt.Errorf("%w: prueba", httpx.ErrOffline)
	mb.onSearch = func() {
		started <- struct{}{}
		<-release
	}
	return started, release
}

func recv(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func waitIdle(t *testing.T, r *Runner) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		r.mu.Lock()
		running := r.running
		r.mu.Unlock()
		if !running {
			return
		}
	}
	t.Fatal("the runner never went idle")
}

func TestTriggerDuringRunFollowsWithOneMoreRun(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	started, release := blockingSearch(mb)
	r, _ := newRunner(t, st, mb, nil)
	r.Trigger()
	recv(t, started, "the first run's search")
	r.Trigger() // during the run
	r.Trigger() // coalesced with the previous one
	release <- struct{}{}
	recv(t, started, "the follow-up run's search")
	release <- struct{}{}
	waitIdle(t, r)
	if n := mb.count("search "); n != 2 {
		t.Fatalf("searches = %d, want 2 (one run plus exactly one follow-up)", n)
	}
}

func TestCloseCancelsPendingRetry(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	mb.err = fmt.Errorf("%w: prueba", httpx.ErrOffline)
	r, _ := newRunner(t, st, mb, nil)
	cancelled := 0
	r.After = func(d time.Duration, f func()) func() { return func() { cancelled++ } }
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.Status().State != StateOffline {
		t.Fatalf("state = %q", r.Status().State)
	}
	r.Close()
	if cancelled != 1 {
		t.Fatalf("pending retry cancelled %d times, want 1", cancelled)
	}
}

func TestCloseWaitsForRunInProgress(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	started, release := blockingSearch(mb)
	r, _ := newRunner(t, st, mb, nil)
	r.Trigger()
	recv(t, started, "the run's search")
	closed := make(chan struct{})
	go func() { r.Close(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("Close returned while a run was in progress")
	case <-time.After(50 * time.Millisecond):
	}
	release <- struct{}{}
	recv(t, closed, "Close to return")
	if r.Status().State == StateRunning {
		t.Fatalf("state = %q after Close", r.Status().State)
	}
}

func TestTriggerAfterCloseDoesNothing(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	r, _ := newRunner(t, st, mb, nil)
	r.Close()
	r.Trigger()
	r.Close() // would wait for a run the Trigger had wrongly started
	if len(mb.calls) != 0 || r.Status().State != StateIdle {
		t.Fatalf("calls %q, state %q", mb.calls, r.Status().State)
	}
}

func TestRunDuringBackgroundRunIsBusy(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	started, release := blockingSearch(mb)
	r, _ := newRunner(t, st, mb, nil)
	r.Trigger()
	recv(t, started, "the run's search")
	if err := r.Run(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	release <- struct{}{}
	waitIdle(t, r)
}
