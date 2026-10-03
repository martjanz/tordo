package loudness

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"

	"tordo/internal/appdir"
)

// Runner states, as the API shows them.
const (
	StateIdle    = "idle"
	StateRunning = "running"
)

// batchSize is how many pending tracks a pass takes at a time.
const batchSize = 20

// Catalog is what the runner needs from the store.
type Catalog interface {
	LoudnessPending(limit int) ([]Job, error)
	LoudnessJobs(ids []int64) ([]Job, error)
	LoudnessPendingCount() (int, error)
	SaveLoudness(ref string, lufs *float64, errText string) error
}

type Status struct {
	State   string `json:"state"` // idle | running
	Done    int    `json:"done"`  // measured since the program started
	Pending int    `json:"pending"`
}

// Runner measures the loudness of the tracks that have none, in the
// background, one at a time. The tracks the player asks for go first.
type Runner struct {
	Catalog Catalog
	AppDir  string
	FFmpeg  string
	// Analyze measures one file; nil means the package's Analyze.
	Analyze func(ctx context.Context, ffmpeg, file string) (float64, error)

	mu      sync.Mutex
	running bool
	again   bool    // a Trigger arrived during a pass
	queue   []int64 // prioritized file ids not yet taken
	done    int
	pending int
	ctx     context.Context
	cancel  context.CancelFunc
	closed  bool
	wg      sync.WaitGroup
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := Status{State: StateIdle, Done: r.done, Pending: r.pending}
	if r.running {
		st.State = StateRunning
	}
	return st
}

// Trigger starts a pass when none is running. A trigger during a pass makes
// it look again before it ends. After Close it does nothing.
func (r *Runner) Trigger() {
	r.start(nil)
}

// Prioritize makes the pass measure these tracks (file ids) first, starting
// one when idle. After Close it does nothing.
func (r *Runner) Prioritize(ids []int64) {
	if len(ids) == 0 {
		return
	}
	r.start(ids)
}

func (r *Runner) start(ids []int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.queue = append(r.queue, ids...)
	if r.running {
		r.again = true
		return
	}
	r.running = true
	if r.ctx == nil {
		r.ctx, r.cancel = context.WithCancel(context.Background())
	}
	ctx := r.ctx
	r.wg.Add(1)
	go r.loop(ctx)
}

// loop runs passes until nothing asks for another.
func (r *Runner) loop(ctx context.Context) {
	defer r.wg.Done()
	for {
		ok := r.pass(ctx)
		r.mu.Lock()
		if !ok { // a store error ends the loop: the next Trigger starts afresh
			r.queue = nil
			r.running = false
			r.again = false
			r.mu.Unlock()
			return
		}
		if (!r.again && len(r.queue) == 0) || r.closed || ctx.Err() != nil {
			r.running = false
			r.again = false
			r.mu.Unlock()
			return
		}
		r.again = false
		r.mu.Unlock()
	}
}

// takeQueue empties the prioritized ids.
func (r *Runner) takeQueue() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := r.queue
	r.queue = nil
	return q
}

// pass measures until nothing is left. false: it ended on a store error.
func (r *Runner) pass(ctx context.Context) bool {
	n, err := r.Catalog.LoudnessPendingCount()
	if err != nil {
		log.Printf("volumen: %v", err)
		return false
	}
	r.mu.Lock()
	r.pending = n
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		if ctx.Err() == nil {
			r.pending = 0
		}
		r.mu.Unlock()
	}()
	analyze := r.Analyze
	if analyze == nil {
		analyze = Analyze
	}
	for ctx.Err() == nil {
		var batch []Job
		if q := r.takeQueue(); len(q) > 0 {
			batch, err = r.Catalog.LoudnessJobs(q)
		} else {
			batch, err = r.Catalog.LoudnessPending(batchSize)
		}
		if err != nil {
			log.Printf("volumen: %v", err)
			return false
		}
		if len(batch) == 0 {
			// Prioritized tracks that need nothing do not end the pass.
			if r.hasQueue() {
				continue
			}
			return true
		}
		seen := map[string]bool{}
		for len(batch) > 0 {
			if q := r.takeQueue(); len(q) > 0 {
				first, err := r.Catalog.LoudnessJobs(q)
				if err != nil {
					log.Printf("volumen: %v", err)
					return false
				}
				batch = append(first, batch...)
			}
			job := batch[0]
			batch = batch[1:]
			if seen[job.Ref] {
				continue
			}
			seen[job.Ref] = true
			if ok, abort := r.measure(ctx, analyze, job); !ok {
				return ctx.Err() == nil && !abort
			}
		}
	}
	return true
}

func (r *Runner) hasQueue() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.queue) > 0
}

// measure analyzes and saves one job. ok false: the pass must end. abort
// true: it ended because the failure may be temporary (the file is gone or
// unreadable, ffmpeg could not start), so nothing was saved and the track
// stays pending for the next scan or Prioritize.
func (r *Runner) measure(ctx context.Context, analyze func(context.Context, string, string) (float64, error), job Job) (ok, abort bool) {
	file := appdir.Abs(r.AppDir, job.Path)
	v, err := analyze(ctx, r.FFmpeg, file)
	if ctx.Err() != nil {
		return false, false
	}
	measured := err == nil
	if measured {
		err = r.Catalog.SaveLoudness(job.Ref, &v, "")
	} else {
		if errors.Is(err, ErrNotStarted) {
			log.Printf("volumen: %v", err)
			return false, true
		}
		if temporaryRead(err) {
			log.Printf("volumen: error de lectura en %s: %v", job.Path, err)
			return false, true
		}
		f, oerr := os.Open(file)
		if oerr != nil {
			log.Printf("volumen: no se puede leer %s: %v", job.Path, oerr)
			return false, true
		}
		f.Close()
		err = r.Catalog.SaveLoudness(job.Ref, nil, err.Error())
	}
	if err != nil {
		log.Printf("volumen: %v", err)
		return false, false
	}
	r.mu.Lock()
	if measured {
		r.done++
	}
	if r.pending > 0 {
		r.pending--
	}
	r.mu.Unlock()
	return true, false
}

// temporaryRead tells whether an Analyze error comes from reading the file
// (a flaky drive, a permission) rather than from its content.
func temporaryRead(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Input/output error") || strings.Contains(msg, "Permission denied")
}

// Close stops the passes for good: it cancels the one in progress and waits
// for it to end. What it saved stays.
func (r *Runner) Close() {
	r.mu.Lock()
	r.closed = true
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
	r.wg.Wait()
}
