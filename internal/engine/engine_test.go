package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"tordo/internal/config"
	"tordo/internal/discogs"
	"tordo/internal/identify"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

var errTest = errors.New("prueba")

// failMB is a MusicBrainz that fails every call: nothing touches the network.
type failMB struct{}

func (failMB) SearchReleases(context.Context, musicbrainz.SearchQuery) ([]musicbrainz.ReleaseHit, error) {
	return nil, errTest
}
func (failMB) Release(context.Context, string) (musicbrainz.Release, error) {
	return musicbrainz.Release{}, errTest
}
func (failMB) ReleaseGroupLinks(context.Context, string) (musicbrainz.GroupLinks, error) {
	return musicbrainz.GroupLinks{}, errTest
}
func (failMB) Artist(context.Context, string) (musicbrainz.Artist, error) {
	return musicbrainz.Artist{}, errTest
}
func (failMB) Cover(context.Context, string, string) ([]byte, error) { return nil, errTest }

// fakeDiscogs only remembers the token it was built with.
type fakeDiscogs struct{ token string }

func (f *fakeDiscogs) HasToken() bool { return f.token != "" }
func (f *fakeDiscogs) Release(context.Context, int) (discogs.Info, error) {
	return discogs.Info{}, errTest
}
func (f *fakeDiscogs) Master(context.Context, int) (discogs.Info, error) {
	return discogs.Info{}, errTest
}
func (f *fakeDiscogs) Search(context.Context, string, string) ([]discogs.Result, error) {
	return nil, errTest
}

// lanHook records the settings it is called with and fails when err is set.
type lanHook struct {
	mu    sync.Mutex
	calls []config.LAN
	err   error
}

func (h *lanHook) call(l config.LAN) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, l)
	if h.err != nil && l.Enabled {
		return h.err
	}
	return nil
}

func (h *lanHook) got() []config.LAN {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]config.LAN(nil), h.calls...)
}

// mp3 is a tiny but real MP3: an ID3v2.4 tag with title, artist and album,
// then a few MPEG frames.
func mp3(title string) []byte {
	text := func(id, v string) []byte {
		data := append([]byte{3}, v...)
		size := len(data)
		f := append([]byte(id), byte(size>>21)&0x7F, byte(size>>14)&0x7F, byte(size>>7)&0x7F, byte(size)&0x7F, 0, 0)
		return append(f, data...)
	}
	var body []byte
	body = append(body, text("TIT2", title)...)
	body = append(body, text("TPE1", "Soda Stereo")...)
	body = append(body, text("TALB", "Signos")...)
	body = append(body, make([]byte, 16)...)
	n := len(body)
	out := []byte{'I', 'D', '3', 4, 0, 0, byte(n>>21) & 0x7F, byte(n>>14) & 0x7F, byte(n>>7) & 0x7F, byte(n) & 0x7F}
	out = append(out, body...)
	for i := 0; i < 4; i++ {
		f := make([]byte, 417)
		for j := range f {
			f[j] = 0x11
		}
		f[0], f[1], f[2], f[3] = 0xFF, 0xFB, 0x90, 0x00
		out = append(out, f...)
	}
	return out
}

// newEngine makes <tmp>/tordo next to musica/ and otra/, each with a track,
// and an engine over a fresh catalog.
func newEngine(t *testing.T) (*Engine, *lanHook, *[]string) {
	t.Helper()
	disk := t.TempDir()
	appDir := filepath.Join(disk, "tordo")
	if err := os.Mkdir(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"musica/Signos/01.mp3", "otra/Otro/01.mp3"} {
		p := filepath.Join(disk, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, mp3(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	hook := &lanHook{}
	var mu sync.Mutex
	tokens := &[]string{}
	e := &Engine{AppDir: appDir, Store: st, MB: failMB{}, LAN: hook.call,
		Discogs: func(token string) identify.Discogs {
			mu.Lock()
			defer mu.Unlock()
			*tokens = append(*tokens, token)
			return &fakeDiscogs{token}
		}}
	t.Cleanup(e.Close)
	return e, hook, tokens
}

// waitScan waits for the current runtime's scan to finish.
func waitScan(t *testing.T, e *Engine) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for st := e.Current().Scanner.Status(); st.Running || st.Finished.IsZero(); st = e.Current().Scanner.Status() {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not finish: %+v", st)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func tracks(t *testing.T, e *Engine) int {
	t.Helper()
	c, err := e.Store.Counts()
	if err != nil {
		t.Fatal(err)
	}
	return c.Tracks
}

func cfgOf(roots ...string) config.Config {
	return config.Config{Roots: roots, Ignore: []string{}, LAN: config.LAN{Port: config.DefaultPort}}
}

func TestStartPending(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(cfgOf("../musica"), true)
	time.Sleep(100 * time.Millisecond)
	if !e.SetupPending() || !e.Current().Scanner.Status().Finished.IsZero() || tracks(t, e) != 0 {
		t.Fatal("scanned while the setup was pending")
	}
	if _, err := os.Stat(filepath.Join(e.AppDir, config.FileName)); err == nil {
		t.Fatal("config.json written")
	}
}

func TestStartScans(t *testing.T) {
	e, hook, _ := newEngine(t)
	cfg := cfgOf("../musica")
	cfg.LAN.Enabled = true
	e.Start(cfg, false)
	waitScan(t, e)
	if tracks(t, e) != 1 || e.SetupPending() {
		t.Fatalf("tracks %d", tracks(t, e))
	}
	if got := hook.got(); len(got) != 1 || !got[0].Enabled {
		t.Fatalf("LAN calls %+v", got)
	}
}

func TestApplyFirstUse(t *testing.T) {
	e, _, _ := newEngine(t)
	cfg := cfgOf("../musica")
	e.Start(cfg, true)
	old := e.Current()
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if e.SetupPending() || e.Current() == old {
		t.Fatal("runtime not replaced")
	}
	saved, created, err := config.Load(e.AppDir)
	if err != nil || created || !reflect.DeepEqual(saved, cfg) {
		t.Fatalf("saved %+v created=%v err=%v", saved, created, err)
	}
	waitScan(t, e)
	if tracks(t, e) != 1 {
		t.Fatalf("tracks %d", tracks(t, e))
	}
	old.Scan()
	if old.Scanner.Status().Running {
		t.Fatal("a stopped runtime scanned")
	}
}

func TestUpdateKeepsPartsForNormalize(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(cfgOf("../musica"), false)
	waitScan(t, e)
	old := e.Current()
	off := false
	cfg := cfgOf("../musica")
	cfg.Normalize = &off
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	rt := e.Current()
	if rt.Scanner != old.Scanner || rt.Identifier != old.Identifier || rt.Config.Normalizing() {
		t.Fatalf("runtime %+v", rt)
	}
	if saved, _, _ := config.Load(e.AppDir); saved.Normalizing() {
		t.Fatalf("saved %+v", saved)
	}
}

func TestUpdateRootsRescans(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(cfgOf("../musica"), false)
	waitScan(t, e)
	old := e.Current()
	if err := e.Apply(cfgOf("../musica", "../otra")); err != nil {
		t.Fatal(err)
	}
	if e.Current().Scanner == old.Scanner {
		t.Fatal("scanner not replaced")
	}
	waitScan(t, e)
	if got := tracks(t, e); got != 2 {
		t.Fatalf("tracks %d, want 2", got)
	}
	if !reflect.DeepEqual(e.Current().Scanner.Roots, []string{"../musica", "../otra"}) {
		t.Fatalf("roots %v", e.Current().Scanner.Roots)
	}
}

func TestUpdateTokenRebuildsIdentifier(t *testing.T) {
	e, _, tokens := newEngine(t)
	e.Start(cfgOf("../musica"), false)
	waitScan(t, e)
	old := e.Current()
	cfg := cfgOf("../musica")
	cfg.DiscogsToken = "nuevo"
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	rt := e.Current()
	if rt.Identifier == old.Identifier || rt.Identifier.Discogs.(*fakeDiscogs).token != "nuevo" {
		t.Fatalf("identifier %+v", rt.Identifier)
	}
	if want := []string{"", "nuevo"}; !reflect.DeepEqual(*tokens, want) {
		t.Fatalf("tokens %v, want %v", *tokens, want)
	}
	time.Sleep(50 * time.Millisecond)
	if !rt.Scanner.Status().Finished.IsZero() || rt.Scanner.Status().Running {
		t.Fatal("a token change scanned")
	}
}

func TestUpdateLAN(t *testing.T) {
	e, hook, _ := newEngine(t)
	e.Start(cfgOf("../musica"), false)
	waitScan(t, e)
	old := e.Current()

	cfg := cfgOf("../musica")
	cfg.LAN = config.LAN{Enabled: true, Port: 5000, PinHash: "h1"}
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if got := hook.got(); !reflect.DeepEqual(got, []config.LAN{cfg.LAN}) {
		t.Fatalf("calls %+v", got)
	}
	if rt := e.Current(); rt.Scanner != old.Scanner || !rt.Config.LAN.Enabled {
		t.Fatalf("runtime %+v", rt)
	}

	// Only the PIN changes: the hook still runs (it is a no-op for a listener
	// already on that port), so a listener that failed to open is retried.
	cfg.LAN.PinHash = "h2"
	if err := e.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	if got := hook.got(); len(got) != 2 || got[1] != cfg.LAN {
		t.Fatalf("calls %+v", got)
	}

	// The port is compared with its default: 0 means DefaultPort.
	other := cfg
	other.LAN = config.LAN{Enabled: true, Port: 0, PinHash: "h2"}
	if err := e.Apply(other); err != nil {
		t.Fatal(err)
	}
	if got := hook.got(); len(got) != 3 {
		t.Fatalf("calls %+v", got)
	}

	// Moving to a port that cannot be opened: nothing is saved.
	hook.err = errTest
	bad := other
	bad.LAN.Port = 6000
	err := e.Apply(bad)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Error() != "no se pudo abrir el puerto 6000: prueba" {
		t.Fatalf("err %v", err)
	}
	if saved, _, _ := config.Load(e.AppDir); !reflect.DeepEqual(saved, other) {
		t.Fatalf("saved %+v", saved)
	}
	if !reflect.DeepEqual(e.Current().Config, other) {
		t.Fatalf("current %+v", e.Current().Config)
	}
}

func TestUpdateLANUndoneWhenSaveFails(t *testing.T) {
	e, hook, _ := newEngine(t)
	e.Start(cfgOf("../musica"), false)
	waitScan(t, e)
	if err := os.Mkdir(filepath.Join(e.AppDir, config.FileName), 0o755); err != nil { // Save cannot write a directory
		t.Fatal(err)
	}
	cfg := cfgOf("../musica")
	cfg.LAN = config.LAN{Enabled: true, Port: 5000, PinHash: "h"}
	if err := e.Apply(cfg); err == nil {
		t.Fatal("expected a save error")
	}
	got := hook.got()
	if len(got) != 2 || !got[0].Enabled || got[1].Enabled {
		t.Fatalf("calls %+v", got)
	}
	if e.Current().Config.LAN.Enabled {
		t.Fatal("config changed")
	}
}

func TestUpdateValidation(t *testing.T) {
	e, _, _ := newEngine(t)
	e.Start(cfgOf("../musica"), false)
	waitScan(t, e)
	old := e.Current()
	want := &ValidationError{errTest}
	err := e.Update(func(config.Config) (config.Config, error) { return config.Config{}, want })
	if err != error(want) {
		t.Fatalf("err %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(e.AppDir, config.FileName)); statErr == nil {
		t.Fatal("config.json written")
	}
	if e.Current() != old {
		t.Fatal("runtime replaced")
	}
}

func TestReadOnly(t *testing.T) {
	e, hook, _ := newEngine(t)
	e.ReadOnly = true
	cfg := cfgOf("../musica")
	cfg.LAN.Enabled = true
	e.Start(cfg, false)
	rt := e.Current()
	if rt.Scanner != nil || rt.Identifier != nil || len(hook.got()) != 0 {
		t.Fatalf("browse-only runtime %+v, LAN %v", rt, hook.got())
	}
	rt.Scan() // nothing
	if err := e.Apply(cfg); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("err %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.AppDir, config.FileName)); err == nil {
		t.Fatal("config.json written")
	}
}

func TestLoudnessAfterScan(t *testing.T) {
	e, _, _ := newEngine(t)
	l := &trig{ch: make(chan struct{}, 4)}
	e.Loudness = l
	e.Start(cfgOf("../musica"), false)
	select {
	case <-l.ch:
	case <-time.After(10 * time.Second):
		t.Fatal("loudness not triggered after the scan")
	}
}

type trig struct{ ch chan struct{} }

func (t *trig) Trigger() { t.ch <- struct{}{} }

func TestUpdateRetriesLANWhenEnabledAndUnchanged(t *testing.T) {
	e, hook, _ := newEngine(t)
	cfg := cfgOf("../musica")
	cfg.LAN = config.LAN{Enabled: true, Port: 5000, PinHash: "h"}
	hook.err = errTest // the listener fails to open at start
	e.Start(cfg, false)
	waitScan(t, e)
	if got := hook.got(); len(got) != 1 {
		t.Fatalf("start calls %+v", got)
	}
	hook.err = nil
	if err := e.Apply(cfg); err != nil { // nothing changed
		t.Fatal(err)
	}
	if got := hook.got(); len(got) != 2 || got[1] != cfg.LAN {
		t.Fatalf("calls after an unchanged save: %+v", got)
	}
	// With the LAN off the hook is not called for an unchanged save.
	off := cfgOf("../musica")
	if err := e.Apply(off); err != nil {
		t.Fatal(err)
	}
	n := len(hook.got())
	if err := e.Apply(off); err != nil {
		t.Fatal(err)
	}
	if got := hook.got(); len(got) != n {
		t.Fatalf("calls with the LAN off: %+v", got)
	}
}

func TestSamePartsNilAndEmptyAreEqual(t *testing.T) {
	a := config.Config{Roots: []string{"../m"}, Ignore: nil}
	b := config.Config{Roots: []string{"../m"}, Ignore: []string{}}
	if !sameParts(a, b) || !sameParts(b, a) {
		t.Fatal("nil and empty ignore differ")
	}
	b.Ignore = []string{"x"}
	if sameParts(a, b) {
		t.Fatal("different ignore patterns are the same parts")
	}
	b.Ignore = nil
	b.DiscogsToken = "t"
	if sameParts(a, b) {
		t.Fatal("a token change is the same parts")
	}
}
