// Package scan walks the library roots and refreshes the catalog. It only
// reads the library; the only writes go to the catalog database.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tordo/internal/appdir"
	"tordo/internal/audiofile"
	"tordo/internal/fingerprint"
	"tordo/internal/grouping"
	"tordo/internal/mediafile"
	"tordo/internal/store"
)

var ErrBusy = errors.New("ya hay un escaneo en curso")

type Status struct {
	Running bool  `json:"running"`
	Files   int64 `json:"files"`  // catalogued files seen by the walk
	ToRead  int   `json:"toRead"` // audio files this run reads
	Read    int   `json:"read"`
	Tracks  int   `json:"tracks"`
	Albums  int   `json:"albums"`
	// EtaSeconds estimates the reading time left from the pace so far; 0
	// while unknown (too few files read) or not reading.
	EtaSeconds int       `json:"etaSeconds"`
	LastError  string    `json:"lastError"`
	Finished   time.Time `json:"finished"`
}

type Scanner struct {
	AppDir string
	Roots  []string // catalog form, cleaned
	Ignore []string // extra folder patterns from config.json
	Store  *store.Store
	// Read reads one audio file; nil means audiofile.Read (and Env from the
	// audiofile package).
	Read func(ctx context.Context, path string) (audiofile.Info, error)
	Env  store.ReadEnv

	// OnDone is called after every scan that was not cancelled (the
	// identification runner's Trigger); nil for none.
	OnDone func()

	// GroupEvery is how often a long read regroups the catalog, so the music
	// shows up as it is read; 0 means 30 s. A regroup that takes long spaces
	// out the next ones.
	GroupEvery time.Duration

	now       func() time.Time // nil means time.Now; tests replace it
	etaAfter  int              // files read before estimating; 0 means 200
	afterSync func()           // nil in production; tests run it once the walk is saved

	mu        sync.Mutex
	status    Status
	readStart time.Time
	busy      bool // a scan or a Refresh is under way
	checking  bool // a Refresh that has found nothing to do so far
	grouped   bool // the albums reflect the catalog of files
}

func (s *Scanner) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status
	after := s.etaAfter
	if after <= 0 {
		after = 200
	}
	if st.Running && st.Read >= after && st.Read < st.ToRead {
		elapsed := s.clock().Sub(s.readStart)
		st.EtaSeconds = int(elapsed.Seconds() * float64(st.ToRead-st.Read) / float64(st.Read))
	}
	return st
}

// Busy reports whether a scan or a Refresh is under way, also one that the
// status does not show.
func (s *Scanner) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy
}

// Run performs one incremental scan. A call while another runs returns ErrBusy.
func (s *Scanner) Run(ctx context.Context) error { return s.start(ctx, false) }

// Refresh checks whether the disk differs from the catalog and only then does
// what Run does. While it checks the status stays as it was, and with nothing
// to do it ends there: the albums are not rebuilt, OnDone is not called and
// the pages, which reload when a run ends, do not notice. A call while another
// runs returns ErrBusy.
func (s *Scanner) Refresh(ctx context.Context) error { return s.start(ctx, true) }

func (s *Scanner) start(ctx context.Context, check bool) error {
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return ErrBusy
	}
	s.busy, s.checking = true, check
	if !check {
		s.status = Status{Running: true}
	}
	s.mu.Unlock()

	err := s.run(ctx)

	s.mu.Lock()
	worked := !s.checking
	s.busy, s.checking = false, false
	s.status.Running = false
	s.status.Finished = time.Now()
	if err != nil {
		s.status.LastError = err.Error()
	}
	s.mu.Unlock()
	if worked && ctx.Err() == nil && s.OnDone != nil {
		s.OnDone()
	}
	return err
}

func (s *Scanner) run(ctx context.Context) error {
	attempted, clean, seen, err := s.walk(ctx)
	if err != nil {
		return err
	}
	changed, err := s.Store.SyncFiles(seen, clean)
	if err != nil {
		return err
	}
	if err := s.Store.MarkOutsideRoots(s.Roots); err != nil {
		return err
	}
	if changed {
		s.mu.Lock()
		s.grouped = false
		s.mu.Unlock()
	}
	if s.afterSync != nil {
		s.afterSync()
	}
	if done, err := s.settle(attempted, len(seen)); done || err != nil {
		return err
	}
	readErr := s.readAll(ctx, attempted)
	if err := ctx.Err(); err != nil {
		return err
	}
	// The catalog is regrouped even when reading stopped early: what was read
	// is already saved.
	groupErr := s.group(false)
	if readErr == nil && groupErr == nil {
		s.mu.Lock()
		s.grouped = true
		s.mu.Unlock()
	}
	return errors.Join(readErr, groupErr)
}

// settle ends a Refresh that has nothing to do (done), or else turns it into
// a run the status shows. From here until the albums are rebuilt they are
// behind the files, whatever interrupts this.
func (s *Scanner) settle(roots []string, files int) (done bool, err error) {
	s.mu.Lock()
	checking := s.checking
	s.mu.Unlock()
	var pending []store.ReadTarget
	if checking {
		_, env := s.reader()
		if pending, err = s.pendingUnder(env, roots); err != nil {
			return false, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if checking && s.grouped && len(pending) == 0 {
		s.status.LastError = ""
		return true, nil
	}
	if checking {
		s.status = Status{Running: true, Files: int64(files)}
		s.checking = false
	}
	s.grouped = false
	return false, nil
}

// walk lists the catalogued files of every available root. attempted are the
// roots walked at all; clean those walked without read errors (only under
// those may unseen files be marked missing).
func (s *Scanner) walk(ctx context.Context) (attempted, clean []string, seen []store.FileRow, err error) {
	for _, root := range s.Roots {
		abs := appdir.Abs(s.AppDir, root)
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			log.Printf("raíz no disponible, se omite: %s", root)
			continue
		}
		attempted = append(attempted, root)
		rootHadError := false
		werr := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				rootHadError = true
				log.Printf("no se puede leer %s: %v", p, err)
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() {
				if p == abs {
					return nil
				}
				rel, err := filepath.Rel(abs, p)
				if err != nil {
					return err
				}
				if mediafile.IgnoredDir(filepath.ToSlash(rel), s.Ignore) {
					return fs.SkipDir
				}
				return nil
			}
			kind := mediafile.Classify(d.Name())
			if kind == mediafile.None {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				rootHadError = true
				log.Printf("no se puede leer %s: %v", p, err)
				return nil
			}
			rel, err := appdir.Rel(s.AppDir, p)
			if err != nil {
				return err
			}
			seen = append(seen, store.FileRow{Path: rel, Size: info.Size(), MTime: info.ModTime().UnixMilli(), Kind: string(kind)})
			s.mu.Lock()
			if !s.checking {
				s.status.Files++
			}
			s.mu.Unlock()
			return nil
		})
		if werr != nil {
			return nil, nil, nil, werr
		}
		// A root that vanished mid-walk means the drive was unplugged: keep
		// the previous catalog instead of marking everything missing.
		if _, err := os.Stat(abs); err != nil {
			return nil, nil, nil, fmt.Errorf("la raíz %s desapareció durante el escaneo", root)
		}
		if rootHadError {
			log.Printf("raíz leída en parte por errores de lectura; no se marcan ausentes bajo ella: %s", root)
			continue
		}
		clean = append(clean, root)
	}
	return attempted, clean, seen, nil
}

// readBatch is how many results are committed at once, so an interrupted
// scan keeps most of its work.
const readBatch = 100

// readAll reads the pending audio files under roots: tags, technical data and
// fingerprint. Files that cannot be read now are left for the next scan;
// files read but not understood are stored with their error.
func (s *Scanner) readAll(ctx context.Context, roots []string) error {
	read, env := s.reader()
	targets, err := s.pendingUnder(env, roots)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.status.ToRead = len(targets)
	s.readStart = s.clock()
	s.mu.Unlock()

	every := s.GroupEvery
	if every <= 0 {
		every = 30 * time.Second
	}
	nextGroup := time.Now().Add(every)

	var batch []store.ReadResult
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := s.Store.SaveReads(batch)
		batch = batch[:0]
		return err
	}
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		abs := appdir.Abs(s.AppDir, t.Path)
		info, err := read(ctx, abs)
		r := store.ReadResult{Target: t, Info: info, Env: env}
		if err == nil {
			fp, ferr := fingerprint.Of(abs, info.Payload.Offset, info.Payload.Length, info.Reader == "native" && info.Format == "ogg")
			if ferr != nil {
				err = fmt.Errorf("%w: %v", audiofile.ErrIO, ferr)
			} else {
				r.Fingerprint = fp
			}
		}
		if err != nil && ctx.Err() == nil {
			root := rootOf(t.Path, roots)
			if _, statErr := os.Stat(appdir.Abs(s.AppDir, root)); statErr != nil {
				return errors.Join(fmt.Errorf("la raíz %s desapareció durante la lectura", root), flush())
			}
		}
		switch {
		case err == nil:
			batch = append(batch, r)
		case ctx.Err() != nil:
			return errors.Join(ctx.Err(), flush())
		case errors.Is(err, audiofile.ErrIO):
			log.Printf("no se puede leer %s, se reintentará: %v", t.Path, err)
		default:
			r.Info, r.Err = audiofile.Info{}, err.Error()
			batch = append(batch, r)
		}
		s.mu.Lock()
		s.status.Read++
		s.mu.Unlock()
		if len(batch) >= readBatch {
			if err := flush(); err != nil {
				return err
			}
		}
		if time.Now().After(nextGroup) {
			if err := flush(); err != nil {
				return err
			}
			start := time.Now()
			if err := s.group(true); err != nil {
				return err
			}
			nextGroup = time.Now().Add(max(every, 10*time.Since(start)))
		}
	}
	return flush()
}

func (s *Scanner) reader() (func(context.Context, string) (audiofile.Info, error), store.ReadEnv) {
	if s.Read != nil {
		return s.Read, s.Env
	}
	return audiofile.Read, store.ReadEnv{Version: audiofile.Version, FFprobe: audiofile.HasFFprobe()}
}

// pendingUnder lists the audio files under roots that have to be read.
func (s *Scanner) pendingUnder(env store.ReadEnv, roots []string) ([]store.ReadTarget, error) {
	pending, err := s.Store.PendingReads(env)
	if err != nil {
		return nil, err
	}
	var targets []store.ReadTarget
	for _, t := range pending {
		if rootOf(t.Path, roots) != "" {
			targets = append(targets, t)
		}
	}
	return targets, nil
}

// group rebuilds albums, tracks and folders from the whole catalog; interim
// while reading (see store.InterimGroupingInput).
func (s *Scanner) group(interim bool) error {
	input := s.Store.GroupingInput
	if interim {
		input = s.Store.InterimGroupingInput
	}
	tracks, images, err := input()
	if err != nil {
		return err
	}
	res := grouping.Build(tracks, images, s.Roots)
	if err := s.Store.ReplaceGrouping(res); err != nil {
		return err
	}
	s.mu.Lock()
	s.status.Tracks, s.status.Albums = len(res.Tracks), len(res.Albums)
	s.mu.Unlock()
	return nil
}

// rootOf returns the root (catalog form) that contains p, or "".
func rootOf(p string, roots []string) string {
	for _, r := range roots {
		if strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/") {
			return r
		}
	}
	return ""
}
