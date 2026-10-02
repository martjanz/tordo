package loudness

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

type save struct {
	Ref  string
	LUFS *float64
	Err  string
}

// fakeCatalog keeps jobs in memory: a saved job is no longer pending.
type fakeCatalog struct {
	mu    sync.Mutex
	jobs  []Job
	saved []save
}

func (c *fakeCatalog) isSaved(ref string) bool {
	for _, s := range c.saved {
		if s.Ref == ref {
			return true
		}
	}
	return false
}

func (c *fakeCatalog) pending() []Job {
	var out []Job
	for _, j := range c.jobs {
		if !c.isSaved(j.Ref) {
			out = append(out, j)
		}
	}
	return out
}

func (c *fakeCatalog) LoudnessPending(limit int) ([]Job, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := c.pending()
	if len(p) > limit {
		p = p[:limit]
	}
	return p, nil
}

func (c *fakeCatalog) LoudnessJobs(ids []int64) ([]Job, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Job
	for _, j := range c.pending() {
		if slices.Contains(ids, j.ID) {
			out = append(out, j)
		}
	}
	return out, nil
}

func (c *fakeCatalog) LoudnessPendingCount() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending()), nil
}

func (c *fakeCatalog) SaveLoudness(ref string, lufs *float64, errText string) error {
	c.mu.Lock()
	s := save{ref, lufs, errText}
	c.saved = append(c.saved, s)
	c.mu.Unlock()
	return nil
}

func (c *fakeCatalog) refs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, s := range c.saved {
		out = append(out, s.Ref)
	}
	return out
}

func jobsOf(n int) []Job {
	var out []Job
	for i := 1; i <= n; i++ {
		out = append(out, Job{ID: int64(i), Ref: fmt.Sprintf("r%d", i), Path: fmt.Sprintf("m/t%d.mp3", i)})
	}
	return out
}

func waitIdle(t *testing.T, r *Runner) {
	t.Helper()
	r.wg.Wait()
	if s := r.Status(); s.State != "idle" {
		t.Fatalf("state = %q", s.State)
	}
}

func TestRunnerMeasuresAll(t *testing.T) {
	cat := &fakeCatalog{jobs: jobsOf(5)}
	var files []string
	var mu sync.Mutex
	r := &Runner{Catalog: cat, AppDir: "/app", FFmpeg: "ff", Analyze: func(ctx context.Context, ff, file string) (float64, error) {
		mu.Lock()
		files = append(files, file)
		mu.Unlock()
		if ff != "ff" {
			t.Errorf("ffmpeg = %q", ff)
		}
		if filepath.Base(file) == "t3.mp3" {
			return 0, ErrNoMeasure
		}
		return -10, nil
	}}
	if s := r.Status(); s.State != "idle" {
		t.Fatalf("initial state = %q", s.State)
	}
	r.Trigger()
	waitIdle(t, r)
	if got := cat.refs(); !slices.Equal(got, []string{"r1", "r2", "r3", "r4", "r5"}) {
		t.Fatalf("saved = %v", got)
	}
	for _, s := range cat.saved {
		if s.Ref == "r3" {
			if s.LUFS != nil || s.Err != ErrNoMeasure.Error() {
				t.Fatalf("failed job saved as %+v", s)
			}
		} else if s.LUFS == nil || *s.LUFS != -10 || s.Err != "" {
			t.Fatalf("job saved as %+v", s)
		}
	}
	if files[0] != filepath.Join("/app", "m", "t1.mp3") {
		t.Fatalf("file = %q", files[0])
	}
	if st := r.Status(); st.Done != 4 || st.Pending != 0 {
		t.Fatalf("status = %+v", st)
	}
	r.Close()
}

func TestRunnerStatusRunning(t *testing.T) {
	cat := &fakeCatalog{jobs: jobsOf(2)}
	started := make(chan string)
	release := make(chan struct{})
	r := &Runner{Catalog: cat, AppDir: "/app", Analyze: func(ctx context.Context, ff, file string) (float64, error) {
		started <- filepath.Base(file)
		<-release
		return -10, nil
	}}
	r.Trigger()
	<-started
	if s := r.Status(); s.State != "running" || s.Pending != 2 {
		t.Fatalf("status = %+v", s)
	}
	release <- struct{}{}
	<-started
	if s := r.Status(); s.State != "running" || s.Done != 1 || s.Pending != 1 {
		t.Fatalf("status = %+v", s)
	}
	release <- struct{}{}
	waitIdle(t, r)
	if s := r.Status(); s.Done != 2 || s.Pending != 0 {
		t.Fatalf("status = %+v", s)
	}
	r.Close()
}

func TestRunnerPrioritize(t *testing.T) {
	cat := &fakeCatalog{jobs: jobsOf(5)}
	started := make(chan string)
	release := make(chan struct{})
	r := &Runner{Catalog: cat, AppDir: "/app", Analyze: func(ctx context.Context, ff, file string) (float64, error) {
		if filepath.Base(file) == "t2.mp3" {
			started <- "slow"
			select {
			case <-release:
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		return -10, nil
	}}
	r.Trigger()
	<-started
	r.Prioritize([]int64{5})
	release <- struct{}{}
	waitIdle(t, r)
	if got := cat.refs(); !slices.Equal(got, []string{"r1", "r2", "r5", "r3", "r4"}) {
		t.Fatalf("order = %v", got)
	}
	r.Close()
}

func TestRunnerPrioritizeStartsIdle(t *testing.T) {
	cat := &fakeCatalog{jobs: jobsOf(3)}
	r := &Runner{Catalog: cat, AppDir: "/app", Analyze: func(ctx context.Context, ff, file string) (float64, error) {
		return -10, nil
	}}
	r.Prioritize([]int64{3})
	waitIdle(t, r)
	if got := cat.refs(); len(got) != 3 || got[0] != "r3" {
		t.Fatalf("order = %v", got)
	}
	r.Close()
}

func TestRunnerCloseDuringJob(t *testing.T) {
	cat := &fakeCatalog{jobs: jobsOf(3)}
	started := make(chan struct{})
	r := &Runner{Catalog: cat, AppDir: "/app", Analyze: func(ctx context.Context, ff, file string) (float64, error) {
		if filepath.Base(file) == "t2.mp3" {
			close(started)
			<-ctx.Done()
			return 0, ctx.Err()
		}
		return -10, nil
	}}
	r.Trigger()
	<-started
	closed := make(chan struct{})
	go func() { r.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return")
	}
	if got := cat.refs(); !slices.Equal(got, []string{"r1"}) {
		t.Fatalf("saved = %v", got)
	}
	r.Trigger()
	r.Prioritize([]int64{3})
	r.wg.Wait()
	if got := cat.refs(); len(got) != 1 {
		t.Fatalf("saved after Close = %v", got)
	}
}

func TestRunnerEndsOnStoreError(t *testing.T) {
	cat := &errCatalog{fakeCatalog: fakeCatalog{jobs: jobsOf(3)}}
	n := 0
	r := &Runner{Catalog: cat, AppDir: "/app", Analyze: func(ctx context.Context, ff, file string) (float64, error) {
		n++
		return -10, nil
	}}
	r.Trigger()
	waitIdle(t, r)
	if n != 1 {
		t.Fatalf("analyzed %d, want the pass to end after the first store error", n)
	}
	r.Close()
}

type errCatalog struct{ fakeCatalog }

func (c *errCatalog) SaveLoudness(string, *float64, string) error { return errors.New("disco lleno") }
