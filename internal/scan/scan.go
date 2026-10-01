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
	Running   bool      `json:"running"`
	Files     int64     `json:"files"`  // catalogued files seen by the walk
	ToRead    int       `json:"toRead"` // audio files this run reads
	Read      int       `json:"read"`
	Tracks    int       `json:"tracks"`
	Albums    int       `json:"albums"`
	LastError string    `json:"lastError"`
	Finished  time.Time `json:"finished"`
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

	mu     sync.Mutex
	status Status
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run performs one incremental scan. A call while another runs returns ErrBusy.
func (s *Scanner) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.status = Status{Running: true}
	s.mu.Unlock()

	err := s.run(ctx)

	s.mu.Lock()
	s.status.Running = false
	s.status.Finished = time.Now()
	if err != nil {
		s.status.LastError = err.Error()
	}
	s.mu.Unlock()
	return err
}

func (s *Scanner) run(ctx context.Context) error {
	attempted, clean, seen, err := s.walk(ctx)
	if err != nil {
		return err
	}
	if err := s.Store.SyncFiles(seen, clean); err != nil {
		return err
	}
	if err := s.Store.MarkOutsideRoots(s.Roots); err != nil {
		return err
	}
	readErr := s.readAll(ctx, attempted)
	if err := ctx.Err(); err != nil {
		return err
	}
	// The catalog is regrouped even when reading stopped early: what was read
	// is already saved.
	return errors.Join(readErr, s.group())
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
			s.status.Files++
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
	read, env := s.Read, s.Env
	if read == nil {
		read, env = audiofile.Read, store.ReadEnv{Version: audiofile.Version, FFprobe: audiofile.HasFFprobe()}
	}
	pending, err := s.Store.PendingReads(env)
	if err != nil {
		return err
	}
	var targets []store.ReadTarget
	for _, t := range pending {
		if rootOf(t.Path, roots) != "" {
			targets = append(targets, t)
		}
	}
	s.mu.Lock()
	s.status.ToRead = len(targets)
	s.mu.Unlock()

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
	}
	return flush()
}

// group rebuilds albums, tracks and folders from the whole catalog.
func (s *Scanner) group() error {
	tracks, images, err := s.Store.GroupingInput()
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
