// Package engine builds, from config.json, the parts of the app that depend
// on it (the scanner and the identification runner) and replaces them when
// the settings change. The LAN listener is opened and closed through a hook.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	"tordo/internal/config"
	"tordo/internal/discogs"
	"tordo/internal/identify"
	"tordo/internal/scan"
	"tordo/internal/store"
)

// ErrReadOnly means the app directory cannot be written: the configuration
// cannot change.
var ErrReadOnly = errors.New("modo consulta: la configuración no se puede modificar")

// ValidationError marks a rejected configuration (not an I/O failure): Update
// callers use it to tell "the request was invalid" from "saving failed".
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// Engine holds the current Runtime. Its fields are what does not depend on
// config.json; set them before Start.
type Engine struct {
	AppDir   string
	Store    *store.Store
	ReadOnly bool
	MB       identify.API                        // MusicBrainz, shared by every runtime
	Discogs  func(token string) identify.Discogs // nil: discogs.New
	Loudness interface{ Trigger() }              // nil: none; triggered after every scan
	LAN      func(config.LAN) error              // opens, moves or closes the LAN listener; nil: none

	mu      sync.Mutex // serializes Update
	current atomic.Pointer[Runtime]
	pending atomic.Bool
}

// Runtime is everything built from one configuration. Pages read it once
// per request; it does not change after it is built. A change that no part
// depends on only replaces it with a copy that keeps the same parts running.
type Runtime struct {
	Config     config.Config
	Scanner    *scan.Scanner    // nil in browse-only mode
	Identifier *identify.Runner // nil in browse-only mode

	*lifecycle // shared by those copies
}

type lifecycle struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool
	scans   sync.WaitGroup
}

// Start builds the first runtime and opens the LAN listener when it is
// enabled (an error is logged: the app works without it). setupPending means
// config.json does not exist yet: nothing is scanned until Apply saves one.
func (e *Engine) Start(cfg config.Config, setupPending bool) {
	rt := e.build(cfg)
	e.current.Store(rt)
	e.pending.Store(setupPending)
	if cfg.LAN.Enabled && !e.ReadOnly && e.LAN != nil {
		if err := e.LAN(cfg.LAN); err != nil {
			log.Print(err)
		}
	}
	if !setupPending {
		rt.Scan()
	}
}

// Current is the runtime in use.
func (e *Engine) Current() *Runtime { return e.current.Load() }

// Use replaces the runtime as it is, without stopping the previous one or
// scanning (tests build their own).
func (e *Engine) Use(rt *Runtime) {
	if rt.lifecycle == nil {
		rt.lifecycle = &lifecycle{}
	}
	e.current.Store(rt)
}

// SetupPending reports whether the first-use assistant has to run.
func (e *Engine) SetupPending() bool { return e.pending.Load() }

// Apply saves cfg as config.json and replaces the runtime as Update does.
// cfg must be valid.
func (e *Engine) Apply(cfg config.Config) error {
	return e.Update(func(config.Config) (config.Config, error) { return cfg, nil })
}

// Update resolves the next configuration against the current one under a
// lock, so a "keep the saved token" decision can never race a concurrent
// change: f receives the configuration in effect right now and returns what
// to save, or a *ValidationError if it rejects it. The LAN hook runs
// before anything is saved when the LAN switch or port changed, and on every
// save while the LAN is on; if the
// listener cannot be opened nothing is saved. Then the runtime is replaced;
// what runs next depends on what changed:
//   - only normalization or LAN: the running parts are kept as they are;
//   - the roots or the ignore patterns: the scanner is rebuilt and the
//     library is scanned again (incremental);
//   - anything else (the Discogs token): no scan, the identification runs
//     again with the new settings.
//
// On first use (the setup was pending) everything is scanned.
func (e *Engine) Update(f func(cur config.Config) (config.Config, error)) error {
	if e.ReadOnly {
		return ErrReadOnly
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	old := e.current.Load()
	var cur config.Config
	if old != nil {
		cur = old.Config
	}
	cfg, err := f(cur)
	if err != nil {
		return err
	}
	lanMoved := false
	if e.LAN != nil {
		changed := cfg.LAN.Enabled != cur.LAN.Enabled || cfg.LAN.PortOrDefault() != cur.LAN.PortOrDefault()
		// With the LAN on the hook runs on every save, so a listener that failed
		// to open at start is retried; the hook does nothing when the listener is
		// already on that port.
		if changed || cfg.LAN.Enabled {
			if err := e.LAN(cfg.LAN); err != nil {
				return &ValidationError{fmt.Errorf("no se pudo abrir el puerto %d: %w", cfg.LAN.PortOrDefault(), err)}
			}
			lanMoved = changed
		}
	}
	if err := config.Save(e.AppDir, cfg); err != nil {
		if lanMoved {
			if err2 := e.LAN(cur.LAN); err2 != nil {
				log.Printf("no se pudo restaurar el acceso desde la red: %v", err2)
			}
		}
		return err
	}
	if old == nil || e.pending.Load() {
		if old != nil {
			old.Stop()
		}
		rt := e.build(cfg)
		e.current.Store(rt)
		e.pending.Store(false)
		rt.Scan()
		return nil
	}
	if sameParts(old.Config, cfg) {
		rt := *old
		rt.Config = cfg
		e.current.Store(&rt)
		return nil
	}
	old.Stop()
	rt := e.build(cfg)
	e.current.Store(rt)
	if !slices.Equal(old.Config.Roots, cfg.Roots) || !slices.Equal(old.Config.Ignore, cfg.Ignore) {
		rt.Scan()
	} else if rt.Identifier != nil {
		rt.Identifier.Trigger()
	}
	return nil
}

// sameParts reports whether a and b differ at most in what no part of the
// runtime depends on (normalization, LAN, listen mode).
func sameParts(a, b config.Config) bool {
	a.Normalize, b.Normalize = nil, nil
	a.LAN, b.LAN = config.LAN{}, config.LAN{}
	a.ListenMode, b.ListenMode = false, false
	// nil and empty lists are the same; the lists are taken out so DeepEqual
	// judges whatever else the config holds.
	same := slices.Equal(a.Roots, b.Roots) && slices.Equal(a.Ignore, b.Ignore)
	a.Roots, b.Roots, a.Ignore, b.Ignore = nil, nil, nil, nil
	return same && reflect.DeepEqual(a, b)
}

func (e *Engine) build(cfg config.Config) *Runtime {
	rt := &Runtime{Config: cfg, lifecycle: &lifecycle{}}
	if e.ReadOnly {
		return rt
	}
	if cfg.DiscogsToken == "" {
		log.Print("sin token de Discogs: los géneros salen de MusicBrainz, de sus enlaces a Discogs y de los tags")
	}
	var dc identify.Discogs
	if e.Discogs != nil {
		dc = e.Discogs(cfg.DiscogsToken)
	} else {
		dc = discogs.New(cfg.DiscogsToken)
	}
	runner := &identify.Runner{AppDir: e.AppDir, Store: e.Store, MB: e.MB, Discogs: dc}
	roots := make([]string, len(cfg.Roots))
	for i, r := range cfg.Roots {
		roots[i] = path.Clean(r)
	}
	onDone := runner.Trigger
	if e.Loudness != nil {
		loud := e.Loudness
		onDone = func() {
			runner.Trigger()
			loud.Trigger()
		}
	}
	rt.Identifier = runner
	rt.Scanner = &scan.Scanner{AppDir: e.AppDir, Roots: roots, Ignore: cfg.Ignore, Store: e.Store, OnDone: onDone}
	return rt
}

// Scan starts a scan of every root in the background; nothing in browse-only
// mode, after Stop, or while one is running.
func (rt *Runtime) Scan() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.Scanner == nil || rt.stopped {
		return
	}
	if rt.ctx == nil {
		rt.ctx, rt.cancel = context.WithCancel(context.Background())
	}
	ctx := rt.ctx
	rt.scans.Add(1)
	go func() {
		defer rt.scans.Done()
		if err := rt.Scanner.Run(ctx); err != nil && !errors.Is(err, scan.ErrBusy) && ctx.Err() == nil {
			log.Printf("escaneo: %v", err)
		}
	}()
}

// Stop cancels the runtime's scan and identification and waits for them to
// end. What they saved stays.
func (rt *Runtime) Stop() {
	rt.mu.Lock()
	rt.stopped = true
	if rt.cancel != nil {
		rt.cancel()
	}
	rt.mu.Unlock()
	if rt.Identifier != nil {
		rt.Identifier.Close()
	}
	rt.scans.Wait()
}

// Close stops the current runtime.
func (e *Engine) Close() {
	if rt := e.current.Load(); rt != nil {
		rt.Stop()
	}
}
