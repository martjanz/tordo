package identify

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tordo/internal/discogs"
	"tordo/internal/genres"
	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// Runner states, as the API shows them.
const (
	StateIdle    = "idle"
	StateRunning = "running"
	StateOffline = "offline" // waiting to retry
)

// Discogs states.
const (
	DiscogsOK       = "ok"
	DiscogsNoToken  = "noToken"  // only MusicBrainz's links to Discogs are followed
	DiscogsBadToken = "badToken" // Discogs rejected the token
)

var (
	ErrBusy    = errors.New("ya hay una identificación en curso")
	ErrNoAlbum = errors.New("álbum no encontrado")
)

// Retry delays while offline: the first, doubled up to the last.
const (
	firstRetry = 5 * time.Minute
	maxRetry   = time.Hour
)

// saveBatch is how many matches are committed at once; applyEvery is how
// many matched albums trigger an intermediate apply.
const (
	saveBatch  = 20
	applyEvery = 100
)

type Status struct {
	State      string `json:"state"`
	ToIdentify int    `json:"toIdentify"`
	Identified int    `json:"identified"`
	ToEnrich   int    `json:"toEnrich"`
	Enriched   int    `json:"enriched"`
	Discogs    string `json:"discogs"`
}

// Discogs is the part of the Discogs client the runner uses.
type Discogs interface {
	HasToken() bool
	Release(ctx context.Context, id int) (discogs.Info, error)
	Master(ctx context.Context, id int) (discogs.Info, error)
	Search(ctx context.Context, artist, title string) ([]discogs.Result, error)
}

type Runner struct {
	AppDir  string
	Store   *store.Store
	MB      API
	Discogs Discogs // nil: no Discogs at all
	// After schedules f after d and returns a function that cancels it;
	// nil means time.AfterFunc. Tests replace it.
	After func(d time.Duration, f func()) (cancel func())

	applyMu sync.Mutex // serializes Apply (runs and corrections)

	mu       sync.Mutex
	status   Status
	running  bool
	again    bool // a Trigger arrived during a run
	backoff  time.Duration
	cancel   func() // pending retry
	badToken bool   // Discogs rejected the token
	base     context.Context
	stop     context.CancelFunc
	closed   bool
	wg       sync.WaitGroup
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.status
	if s.State == "" {
		s.State = StateIdle
	}
	switch {
	case r.Discogs == nil || !r.Discogs.HasToken():
		s.Discogs = DiscogsNoToken
	case r.badToken:
		s.Discogs = DiscogsBadToken
	default:
		s.Discogs = DiscogsOK
	}
	return s
}

// Trigger starts a run in the background. A trigger during a run makes one
// more run follow it. After Close it does nothing.
func (r *Runner) Trigger() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	if r.running {
		r.again = true
		r.mu.Unlock()
		return
	}
	r.running = true
	if r.base == nil {
		r.base, r.stop = context.WithCancel(context.Background())
	}
	ctx := r.base
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		for {
			if err := r.run(ctx); err != nil && ctx.Err() == nil {
				log.Printf("identificación: %v", err)
			}
			r.mu.Lock()
			if !r.again || r.closed {
				r.running = false
				r.mu.Unlock()
				return
			}
			r.again = false
			r.mu.Unlock()
		}
	}()
}

// Close stops the background runs for good: it cancels the one in progress
// and the pending retry, and waits for it to end. What it saved stays.
func (r *Runner) Close() {
	r.mu.Lock()
	r.closed = true
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.stop != nil {
		r.stop()
	}
	r.mu.Unlock()
	r.wg.Wait()
}

// Run performs one run synchronously; ErrBusy if one is in progress.
func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrBusy
	}
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
	return r.run(ctx)
}

func (r *Runner) run(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.status = Status{State: StateRunning}
	r.mu.Unlock()

	// Releases looked up while matching, reused by enrichment in this run.
	fetched := map[string]musicbrainz.Release{}
	err := r.identifyAll(ctx, fetched)
	if err == nil {
		err = r.Apply()
	}
	if err == nil {
		err = r.enrichAll(ctx, fetched)
	}
	if err == nil {
		err = r.coversAll(ctx)
	}
	if aerr := r.Apply(); err == nil {
		err = aerr
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.closed:
		r.status.State = StateIdle
		return nil
	case errors.Is(err, httpx.ErrOffline):
		r.status.State = StateOffline
		if r.backoff == 0 {
			r.backoff = firstRetry
		} else {
			r.backoff = min(2*r.backoff, maxRetry)
		}
		after := r.After
		if after == nil {
			after = func(d time.Duration, f func()) func() { t := time.AfterFunc(d, f); return func() { t.Stop() } }
		}
		r.cancel = after(r.backoff, r.Trigger)
		log.Printf("identificación: sin conexión, se reintenta en %v", r.backoff)
		return nil
	}
	r.status.State = StateIdle
	r.backoff = 0
	return err
}

func (r *Runner) identifyAll(ctx context.Context, fetched map[string]musicbrainz.Release) error {
	targets, err := r.Store.IdentifyTargets()
	if err != nil {
		return err
	}
	matches, err := r.Store.Matches()
	if err != nil {
		return err
	}
	corrections, err := r.Store.Corrections()
	if err != nil {
		return err
	}
	known, err := r.Store.Releases()
	if err != nil {
		return err
	}
	var jobs []Query
	queued := map[string]bool{}
	for _, t := range targets {
		if _, ok := correctionOf(t, corrections); ok {
			continue // the automatic identification never overrides a correction
		}
		q := BuildQuery(t)
		k := q.Key()
		if m, ok := matches[k]; (ok && m.MatcherVersion >= MatcherVersion) || queued[k] {
			continue
		}
		queued[k] = true
		jobs = append(jobs, q)
	}
	r.mu.Lock()
	r.status.ToIdentify = len(jobs)
	r.mu.Unlock()

	var batch []store.Match
	flush := func() error {
		for _, m := range batch {
			if _, ok := known[m.ReleaseID]; m.Status != store.StatusAuto || ok {
				continue
			}
			rel := releaseFrom(fetched[m.ReleaseID])
			if err := r.Store.SaveRelease(rel); err != nil {
				return err
			}
			known[m.ReleaseID] = rel
		}
		err := r.Store.SaveMatches(batch)
		batch = batch[:0]
		return err
	}
	for i, q := range jobs {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		m, err := Identify(ctx, r.MB, q, fetched)
		if err != nil {
			return errors.Join(err, flush())
		}
		batch = append(batch, m)
		r.mu.Lock()
		r.status.Identified++
		r.mu.Unlock()
		if len(batch) >= saveBatch {
			if err := flush(); err != nil {
				return err
			}
		}
		if (i+1)%applyEvery == 0 {
			if err := r.Apply(); err != nil {
				return err
			}
		}
	}
	return flush()
}

func (r *Runner) enrichAll(ctx context.Context, fetched map[string]musicbrainz.Release) error {
	ids, err := r.Store.ReleaseIDsInUse()
	if err != nil {
		return err
	}
	releases, err := r.Store.Releases()
	if err != nil {
		return err
	}
	artists, err := r.Store.Artists()
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.status.ToEnrich = len(ids)
	r.mu.Unlock()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, ok := releases[id]
		if !ok { // named by a correction made while offline
			mb, err := lookup(ctx, r.MB, id, fetched)
			switch {
			case fatal(err):
				return err
			case err != nil:
				log.Printf("no se pudo obtener el lanzamiento %s de MusicBrainz: %v", id, err)
				continue
			}
			rel = releaseFrom(mb)
			if err := r.Store.SaveRelease(rel); err != nil {
				return err
			}
		}
		if aid := mainArtist(rel); aid != "" {
			if _, ok := artists[aid]; !ok {
				a, err := r.fetchArtist(ctx, aid)
				if err != nil {
					return err
				}
				artists[aid] = a
			}
		}
		if r.needsDiscogs(rel) {
			if err := r.discogsFor(ctx, &rel); err != nil {
				return err
			}
			if err := r.Store.SaveRelease(rel); err != nil {
				return err
			}
		}
		r.mu.Lock()
		r.status.Enriched++
		r.mu.Unlock()
	}
	return nil
}

// fetchArtist looks an artist up and stores it; one MusicBrainz does not
// have is stored empty so it is not asked for again. Only fatal errors are
// returned.
func (r *Runner) fetchArtist(ctx context.Context, id string) (store.Artist, error) {
	a, err := r.MB.Artist(ctx, id)
	switch {
	case fatal(err):
		return store.Artist{}, err
	case errors.Is(err, musicbrainz.ErrNotFound):
		a = musicbrainz.Artist{ID: id}
	case err != nil:
		log.Printf("no se pudo obtener el artista %s de MusicBrainz: %v", id, err)
		return store.Artist{ID: id}, nil
	}
	sa := store.Artist{ID: a.ID, Name: a.Name, SortName: a.SortName, Type: a.Type, Country: a.Country, Area: a.Area,
		Begin: a.Begin, End: a.End}
	return sa, r.Store.SaveArtist(sa)
}

func (r *Runner) needsDiscogs(rel store.Release) bool {
	if r.Discogs == nil || r.tokenRejected() {
		return false
	}
	if !rel.Discogs.Done {
		return true
	}
	return len(rel.Discogs.Genres) == 0 && !rel.Discogs.Searched && r.Discogs.HasToken()
}

// discogsFor fills a release's Discogs genres and styles: from MusicBrainz's
// links to Discogs (the release's, else its group's), else from a Discogs
// search when there is a token. Only fatal errors are returned.
func (r *Runner) discogsFor(ctx context.Context, rel *store.Release) error {
	if !rel.Discogs.Done {
		if len(rel.DiscogsURLs) == 0 && !rel.GroupChecked && rel.GroupID != "" {
			g, err := r.MB.ReleaseGroupLinks(ctx, rel.GroupID)
			if fatal(err) {
				return err
			}
			if err != nil {
				log.Printf("no se pudo obtener el grupo %s de MusicBrainz: %v", rel.GroupID, err)
			} else {
				rel.DiscogsURLs = g.DiscogsURLs
				if len(rel.MBGenres) == 0 {
					rel.MBGenres = g.Genres
				}
			}
			rel.GroupChecked = true
		}
		info, err := r.followDiscogs(ctx, rel.DiscogsURLs)
		switch {
		case fatal(err):
			return err
		case errors.Is(err, discogs.ErrUnauthorized):
			r.rejectToken()
			return nil // Done stays false: retried once the token is fixed
		case err != nil:
			log.Printf("Discogs, %s: %v", rel.Title, err)
		}
		rel.Discogs.ID, rel.Discogs.MasterID, rel.Discogs.Genres, rel.Discogs.Styles = info.ID, info.MasterID, info.Genres, info.Styles
		rel.Discogs.Done = true
	}
	if len(rel.Discogs.Genres) > 0 || rel.Discogs.Searched || !r.Discogs.HasToken() || r.tokenRejected() {
		return nil
	}
	results, err := r.Discogs.Search(ctx, searchArtist(*rel), rel.Title)
	switch {
	case fatal(err):
		return err
	case errors.Is(err, discogs.ErrUnauthorized):
		r.rejectToken()
		return nil
	case err != nil:
		log.Printf("búsqueda en Discogs de %q: %v", rel.Title, err)
		return nil
	}
	if res, ok := bestResult(*rel, results); ok {
		rel.Discogs.ID, rel.Discogs.MasterID, rel.Discogs.Genres, rel.Discogs.Styles = res.ID, res.MasterID, res.Genres, res.Styles
	}
	rel.Discogs.Searched = true
	return nil
}

// followDiscogs reads genres and styles from the first Discogs link that
// has them; a release without genres falls back to its master.
func (r *Runner) followDiscogs(ctx context.Context, urls []string) (discogs.Info, error) {
	for _, u := range urls {
		kind, id, ok := discogs.ParseURL(u)
		if !ok {
			continue
		}
		var info discogs.Info
		var err error
		if kind == "release" {
			info, err = r.Discogs.Release(ctx, id)
			if err == nil && len(info.Genres) == 0 && info.MasterID != 0 {
				m, merr := r.Discogs.Master(ctx, info.MasterID)
				if fatal(merr) || errors.Is(merr, discogs.ErrUnauthorized) {
					return discogs.Info{}, merr
				}
				if merr == nil {
					info.Genres, info.Styles = m.Genres, m.Styles
				}
			}
		} else {
			info, err = r.Discogs.Master(ctx, id)
		}
		if errors.Is(err, discogs.ErrNotFound) {
			continue
		}
		if err != nil {
			return discogs.Info{}, err
		}
		if len(info.Genres) > 0 {
			return info, nil
		}
	}
	return discogs.Info{}, nil
}

func (r *Runner) rejectToken() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.badToken {
		log.Print("Discogs rechazó el token: se sigue sin Discogs; revisá discogsToken en config.json")
	}
	r.badToken = true
}

func (r *Runner) tokenRejected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.badToken
}

func searchArtist(rel store.Release) string {
	if rel.Various {
		return "Various"
	}
	if len(rel.ArtistNames) > 0 {
		return rel.ArtistNames[0]
	}
	return rel.Artist
}

// bestResult is the first search result with the release's title (and
// artist, unless a compilation). Discogs adds " (2)" to homonyms; the
// similarity tolerates it.
func bestResult(rel store.Release, rs []discogs.Result) (discogs.Result, bool) {
	for _, res := range rs {
		if similarity(res.Title, rel.Title) >= 0.8 && (rel.Various || similarity(res.Artist, searchArtist(rel)) >= 0.6) {
			return res, true
		}
	}
	return discogs.Result{}, false
}

func (r *Runner) coversAll(ctx context.Context) error {
	ids, err := r.Store.CoverlessReleases()
	if err != nil {
		return err
	}
	releases, err := r.Store.Releases()
	if err != nil {
		return err
	}
	for _, id := range ids {
		rel, ok := releases[id]
		if !ok || rel.Cover != "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := r.MB.Cover(ctx, "release", id)
		if errors.Is(err, musicbrainz.ErrNotFound) && rel.GroupID != "" {
			b, err = r.MB.Cover(ctx, "release-group", rel.GroupID)
		}
		switch {
		case fatal(err):
			return err
		case errors.Is(err, musicbrainz.ErrNotFound):
			rel.Cover = store.CoverNone
		case err != nil:
			log.Printf("carátula de %s: %v", id, err)
			continue
		default:
			if rel.Cover, err = r.saveCover(id, b); err != nil {
				return err
			}
		}
		if err := r.Store.SaveRelease(rel); err != nil {
			return err
		}
	}
	return nil
}

// saveCover writes a cover into cache/covers/ through a temporary file and
// returns its path relative to the app folder.
func (r *Runner) saveCover(id string, b []byte) (string, error) {
	rel := "cache/covers/" + id + ".jpg"
	name := filepath.Join(r.AppDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(name), id+".*.tmp")
	if err != nil {
		return "", err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), name)
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return rel, nil
}

// Apply recomputes every album's identity from what is stored (no requests)
// and writes it.
func (r *Runner) Apply() error {
	r.applyMu.Lock()
	defer r.applyMu.Unlock()
	targets, err := r.Store.IdentifyTargets()
	if err != nil {
		return err
	}
	var w world
	if w.matches, err = r.Store.Matches(); err != nil {
		return err
	}
	if w.corrections, err = r.Store.Corrections(); err != nil {
		return err
	}
	if w.releases, err = r.Store.Releases(); err != nil {
		return err
	}
	if w.artists, err = r.Store.Artists(); err != nil {
		return err
	}
	aliases, err := r.Store.GenreAliases()
	if err != nil {
		return err
	}
	user := map[string]genres.Mapping{}
	for _, a := range aliases {
		user[a.Raw] = genres.Mapping{Genre: a.Genre, Style: a.Style}
	}
	w.genres = genres.New(user)
	unmapped := map[string]int{}
	ids := make([]store.AlbumIdentity, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, resolve(t, BuildQuery(t), w, unmapped))
	}
	return r.Store.ApplyIdentities(ids, unmapped)
}

// SetRelease identifies an album by hand. The release is looked up first
// (which validates the id) unless already stored; the correction goes on
// every track's fingerprint. The caller should Trigger a run afterwards so
// the artist, Discogs genres and cover follow.
func (r *Runner) SetRelease(ctx context.Context, albumID int64, releaseID string) error {
	if !musicbrainz.ValidID(releaseID) {
		return musicbrainz.ErrInvalidID
	}
	fps, err := r.fingerprints(albumID)
	if err != nil {
		return err
	}
	releases, err := r.Store.Releases()
	if err != nil {
		return err
	}
	if _, ok := releases[releaseID]; !ok {
		mb, err := r.MB.Release(ctx, releaseID)
		if err != nil {
			return err
		}
		if err := r.Store.SaveRelease(releaseFrom(mb)); err != nil {
			return err
		}
	}
	return r.correct(fps, store.Correction{Status: store.StatusManual, ReleaseID: releaseID})
}

// Ignore marks an album as not in MusicBrainz: it leaves Sin identificar.
func (r *Runner) Ignore(albumID int64) error {
	fps, err := r.fingerprints(albumID)
	if err != nil {
		return err
	}
	return r.correct(fps, store.Correction{Status: store.StatusIgnored})
}

// Reset removes an album's correction: the automatic identification decides
// again (Trigger a run if it was never matched).
func (r *Runner) Reset(albumID int64) error {
	fps, err := r.fingerprints(albumID)
	if err != nil {
		return err
	}
	return r.correct(fps, store.Correction{})
}

func (r *Runner) correct(fps []string, c store.Correction) error {
	if err := r.Store.SetCorrection(fps, c); err != nil {
		return err
	}
	return r.Apply()
}

func (r *Runner) fingerprints(albumID int64) ([]string, error) {
	fps, err := r.Store.AlbumFingerprints(albumID)
	if err == nil && len(fps) == 0 {
		err = ErrNoAlbum
	}
	return fps, err
}
