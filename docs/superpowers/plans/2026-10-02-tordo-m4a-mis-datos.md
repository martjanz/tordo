# Tordo M4a (Escucha I: me gusta, puntajes, historial, listas, normalización) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The listener's own data: ♥ me gusta on tracks, albums, artists and collections; ★ 1–5 on tracks and albums; every play recorded (≥ 50 % or ≥ 4 min); playlists (create, rename, reorder by dragging, add from any track/album/folder/artist, play); Explorar's facets "Me gusta" and "Puntaje" and the order "Más escuchado"; the Listas page with playlists, Me gusta and collections; and volume normalization (ReplayGain tags, else ffmpeg's EBU R128 analysis in the background), applied by the player.

**Architecture:** User data and measured loudness live in new tables of `tordo.db`, keyed by the track's **audio fingerprint** (as corrections are), so they survive moving, renaming and re-tagging. Their writes are **side writes**: they do not move the catalog's change counter, so liking a track or recording a play never rebuilds the in-memory catalog. The store resolves the user's data to current ids (`store.Personal`), cached by the server per (catalog changes, personal version), and `catalog` applies it as extra facet values at query time. A new package `internal/loudness` holds the gain math, the ffmpeg analysis and a background runner. The web app gets a small client store of marks, a play tracker and the gain choice as tested pure modules, the ♥/★ controls, the playlist page and the new Listas.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`), Svelte 5 + Vite 8 + Vitest 5 (as in M3).

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish). M4 in §10 is "Inicio, estaciones, historial y estadísticas, me gusta y puntajes, listas de reproducción, normalización de volumen". It is split like M3: **M4a (this plan)** covers §4 "Datos de usuario" (but saved stations), §7.1 "Normalización", §7.3's counting rule (stats are M4b), §7.4 whole, §8 Listas, the ♥/★ of Álbum, Artista and Colección, and Explorar's "me gusta", "puntaje" and "más escuchado" (§8). **M4b** (separate plan, after this one merges) covers §7.2 Estaciones, §7.3 statistics, §8 Inicio and the "▶ Estación" buttons.

## Decisions this plan takes (not spelled out in the spec)

1. **Identity of marked things.** Tracks: by `trackRef` = the audio fingerprint (`'id:<file id>'` if a file has none). Albums: an album's ♥/★ is written on **each of its tracks' refs** (kind `album`), like an album correction; an album is liked if any of its present tracks carries an album like, and its rating is the highest one found. Artists: by catalog artist key (the MusicBrainz id, else the folded name). Collections: by folder path (as "no es una colección"). Plays and playlist entries: by `trackRef`.
2. **Side writes.** `store.sideWrite` measures `total_changes()` before and after its transaction and subtracts the difference from what `Changes()`/`Snapshot()` report. A mutex orders side writes against those two reads (always mutex first, then the connection: no deadlock).
3. **Facets.** `megusta=si`: the item itself is liked (an album on Álbumes and Artistas, a track on Pistas). `puntaje=N` means "N stars or more" (an item rated 4 carries the values 1–4, so the menu counts are cumulative). Order `escuchado` ("Más escuchado"): plays of the track; of an album, the sum of its tracks; of an artist, the sum of their albums.
4. **A play** counts once per pass when the listened time (only forward progress in steps ≤ 2 s, so seeks and stalls do not count) reaches 4 min or half the duration. Repeat-one starts a new pass. Recorded by the browser with `POST /api/plays`; in browse-only mode the server answers 204 and records nothing. The device is a readable name from the user agent ("Mac · Safari").
5. **Normalization.** Target −18 LUFS (ReplayGain 2's reference). Gain sources, in order: the ReplayGain tags (already read into `audio.tags` as `trackGain`/`albumGain`), else ffmpeg's `ebur128` integrated loudness. An album's gain without tags is the duration-weighted energy mean of its tracks' loudness, when every track is known. The player uses the **album** gain when shuffle is off and the track's neighbor in play order (before or after) is from the same album, else the track gain; a track with no gain yet gets **−6 dB**; the gain only attenuates (`audio.volume = volume × min(1, 10^(dB/20))`) and is fixed when the track starts (never changes mid-track). The runner analyzes newest tracks first, but tracks the player asks gains for jump the queue; the player asks for the gains of the queued tracks that lack one. `config.json` gets `"normalize"` (default true), shown in `/api/status`; switching it from the UI is M5's Ajustes.
6. **Playlists** keep a snapshot of title, artist, album and duration per entry, so a track that disappeared still shows (dimmed, "no disponible") and is skipped when playing. Entries are capped at `MaxPlay` (5000) per list. Reordering sends the full new order of entry ids.
7. **Liked collections** show their ♥ on the Listas page cards and on the folder page header. The folder page is being rewritten in parallel (Carpetas as a file explorer): wiring its ♥ is the last code task, done **after syncing with `main`**.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new Go dependencies** (only `modernc.org/sqlite v1.59.0`). No cgo.
- npm: no new packages (only `svelte`, `vite`, `vitest`, `@sveltejs/vite-plugin-svelte`, `@fontsource-variable/inter`).
- **Never write to the music roots.** The only writes go to the app folder: `tordo.db`, `config.json`, `cache/`. Browse-only mode writes nothing: write endpoints answer 409 (but `POST /api/plays`: 204, nothing recorded), the pages hide their write actions (`app.status.readOnly`).
- Catalog paths are relative to the app folder, `/`-separated. Audio and pictures are asked for **by id**, never by path; folders by their catalog path.
- Server listens on `127.0.0.1` only (`localOnly`); write endpoints (POST/PUT) are wrapped in `jsonOnly`.
- SQLite `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open; inside a transaction read only through it.
- Schema: v4 only **adds** tables (`schemaVersion = 4`, `derivedVersion` stays 2). User tables are never dropped or rebuilt by a scan.
- `ffmpeg` is optional: without it there is no analysis (tags still give gains), never an error.
- User-facing strings (UI, logs, errors) in **Spanish** (rioplatense: *elegí*, *escribí*); identifiers and comments in English. URL names in Spanish (`megusta`, `puntaje`, `escuchado`, `/listas/{id}`). UI files named in Spanish (`MeGusta.svelte`, `Puntaje.svelte`, `Lista.svelte`).
- The web build (`internal/server/dist/`) is **not committed**. `go test ./...` passes without Node and **without network** (tests that run ffmpeg skip when it is missing); `go vet ./...` and `gofmt -l .` clean; `cd web && npx vitest run` passes; `cd web && npm run build` succeeds.
- Commit messages: conventional style (`feat:`, `fix:`, `test:`, `docs:`), **no `Co-Authored-By:` trailer**.

## Model per task

Per the user's rule (cheapest model that can do the job): **Haiku** for transcribing fully specified code and docs, **Sonnet** for implementing from prose and multi-file work, **Opus** only for the final whole-branch review.

| Task | What | Model |
|---|---|---|
| 1 | Schema v4 and side writes | Sonnet |
| 2 | Marks, plays and `Personal` in the store | Sonnet |
| 3 | Playlists in the store | Sonnet |
| 4 | Catalog: personal facets, "más escuchado", liked lists | Sonnet |
| 5 | Server: marks, likes, ratings, plays, liked, Explorar | Sonnet |
| 6 | Server: playlists, play sources | Sonnet |
| 7 | `internal/loudness`: math, ffmpeg analysis | Haiku |
| 8 | Loudness: store, runner, gains, config, status, wiring | Sonnet |
| 9 | Web libraries | Sonnet |
| 10 | Player: plays and gains | Sonnet |
| 11 | ♥ and ★ in the pages | Sonnet |
| 12 | Playlists and Listas in the web app | Sonnet |
| 13 | Sync with `main`, ♥ on the folder page | Sonnet |
| 14 | Documentation | Haiku |
| — | Final review of the branch | Opus |
| 15 | Check against the collection | manual (with the user) |

## File Structure

```
internal/store/schema.sql, store.go   v4: marks, plays, playlists, playlist_items, loudness; side-write bookkeeping
internal/store/side.go                NEW: sideWrite, Changes (moved from snapshot.go), PersonalVersion, trackRef
internal/store/snapshot.go            Snapshot reads the counter under the side-write lock
internal/store/personal.go            NEW: Like*/Rate*, RecordPlay, Personal, LikedTrackIDs
internal/store/playlists.go           NEW: playlists CRUD, entries, PlayPlaylist
internal/store/loudness.go            NEW: LoudnessPending, LoudnessTargets, SaveLoudness, LoudnessFacts
internal/catalog/personal.go          NEW: personal facet values, plays per album/artist, Liked
internal/catalog/facets.go            megusta, puntaje, escuchado; ExploreFor
internal/loudness/                    NEW: loudness.go (Target, Gain, AlbumLoudness, ParseEBUR128, Analyze, Gains),
                                      runner.go (background analysis, priorities, status)
internal/config/config.go             Normalize *bool, Normalizing()
internal/server/personal.go           NEW: personal cache; /api/marks, /api/likes, /api/ratings, /api/plays, /api/liked
internal/server/playlists.go          NEW: /api/playlists…; resolve() shared with /api/play
internal/server/media.go              play via resolve(); ?playlist=, ?liked=1; gains on items; /api/gains
internal/server/server.go             routes; Loudness and Normalize fields; status
internal/server/catalog.go            explore with the personal overlay
cmd/tordo/main.go                     loudness runner wiring; scanner OnDone triggers both runners
web/src/lib/                          api (+calls), marks.js, mine.svelte.js, plays.js, device.js, gain.js, order.js (NEW),
                                      facets (megusta, puntaje, escuchado), router (/listas/{id}), player (plays, gains)
web/src/components/                   MeGusta, Puntaje, MenuPista, AgregarALista (NEW); ListaPistas, Acciones, Reproductor, Icono
web/src/pages/                        Lista (NEW); Listas, Album, Artista, Carpetas (♥ collection, Task 13)
README.md                             listening data, playlists, normalization, endpoints
```

## How to read the tasks

The plan gives the design, the exact interfaces, the test cases and the code where it is tricky; the rest is yours to write, matching the surrounding code (comment density, naming, idiom). Tests come first in each task: write them, see them fail, then write the code. Go tests use the existing helpers (`store`: `open`, `catalog`, `sample`, `library`, `albumNamed`; `server`: `newStore`, `mediaServer`, `do`, `decode`, `itoa`; `catalog`: `fixture`, `build`). Svelte components have no tests: their logic lives in tested `web/src/lib/` modules, and each web task ends with `npx vitest run` and `npm run build`.

After every task: `gofmt -l .` (empty), `go vet ./...`, `go test ./...`, and for web tasks `cd web && npx vitest run && npm run build`. Then commit.

---

### Task 1: Schema v4 and side writes

The new tables, and the bookkeeping that keeps their writes from moving the catalog's change counter.

**Files:**
- Modify: `internal/store/schema.sql` (append the tables below)
- Modify: `internal/store/store.go` (`schemaVersion = 4`, comment; `Store` fields)
- Create: `internal/store/side.go`
- Modify: `internal/store/snapshot.go` (`Changes` moves to `side.go`; `Snapshot` takes the lock)
- Test: `internal/store/side_test.go`, `internal/store/migrate_test.go`

**Interfaces:**
- Produces:
  - `func (s *Store) sideWrite(personal bool, fn func(tx *sql.Tx) error) error`: runs fn in a transaction whose row changes are not catalog changes; `personal` also bumps the personal version.
  - `func (s *Store) Changes() (int64, error)`: unchanged meaning, now net of side writes.
  - `func (s *Store) PersonalVersion() int64`: grows with every personal side write.
  - `const trackRef` (SQL over `files f`): `CASE WHEN f.fingerprint != '' THEN f.fingerprint ELSE 'id:' || f.id END`.

Append to `schema.sql`:

```sql
-- The user's marks: likes (♥) and ratings (★ 1–5). kind is track, album,
-- artist or collection. Tracks and albums are marked by trackRef (the audio
-- fingerprint), so marks survive moving, renaming and re-tagging; an album's
-- mark is written on each of its tracks. Artists by catalog key,
-- collections by folder path. Never touched by scans.
CREATE TABLE IF NOT EXISTS marks (
	kind       TEXT NOT NULL,
	ref        TEXT NOT NULL,
	liked      INTEGER NOT NULL DEFAULT 0,
	liked_at   INTEGER NOT NULL DEFAULT 0,    -- unix ms, 0 when not liked
	rating     INTEGER NOT NULL DEFAULT 0,    -- 0: none, 1–5
	updated_at INTEGER NOT NULL,
	PRIMARY KEY (kind, ref)
);

-- Every play that counted (≥ 50 % or ≥ 4 min listened).
CREATE TABLE IF NOT EXISTS plays (
	id          INTEGER PRIMARY KEY,
	ref         TEXT NOT NULL,                -- trackRef
	file_id     INTEGER NOT NULL,             -- the file played, as it was then
	played_at   INTEGER NOT NULL,             -- unix ms
	listened_ms INTEGER NOT NULL,
	device      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS plays_ref ON plays(ref);
CREATE INDEX IF NOT EXISTS plays_played_at ON plays(played_at);

CREATE TABLE IF NOT EXISTS playlists (
	id         INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

-- A playlist's entries in order, by trackRef, with what the track was
-- called when added: an entry whose track disappeared still shows.
CREATE TABLE IF NOT EXISTS playlist_items (
	id          INTEGER PRIMARY KEY,
	playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
	pos         INTEGER NOT NULL,
	ref         TEXT NOT NULL,
	file_id     INTEGER NOT NULL,
	title       TEXT NOT NULL,
	artist      TEXT NOT NULL,
	album       TEXT NOT NULL,
	duration_ms INTEGER NOT NULL,
	added_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS playlist_items_list ON playlist_items(playlist_id, pos);

-- Integrated loudness measured by ffmpeg (ebur128), by trackRef. lufs is
-- NULL when the file could not be measured (error says why): not retried
-- until its audio changes (a new fingerprint).
CREATE TABLE IF NOT EXISTS loudness (
	ref         TEXT PRIMARY KEY,
	lufs        REAL,
	error       TEXT NOT NULL DEFAULT '',
	measured_at INTEGER NOT NULL
);
```

The store's comment on `schemaVersion` gains: "Version 4 (M4a) added the user's tables (marks, plays, playlists) and loudness."

`side.go` (the delicate part, given whole):

```go
package store

import (
	"database/sql"
	"sync"
)

// trackRef is, in SQL over files f, what the user's data is keyed by: the
// audio fingerprint, so that it survives moving, renaming and re-tagging.
const trackRef = `CASE WHEN f.fingerprint != '' THEN f.fingerprint ELSE 'id:' || f.id END`

// side keeps the user's writes (and measured loudness) out of the catalog's
// change counter: they do not change what the catalog shows, and counting
// them would rebuild it on every like or play.
type side struct {
	mu       sync.Mutex // held by side writes and by reads of the counter
	rows     int64      // rows changed by side writes since the store opened
	personal int64      // personal side writes since the store opened
}

// sideWrite runs fn in a transaction whose changes are not catalog
// changes. personal marks the user's data (marks, plays, playlists) as
// changed. The lock is always taken before the connection.
func (s *Store) sideWrite(personal bool, fn func(tx *sql.Tx) error) error {
	s.side.mu.Lock()
	defer s.side.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var before, after int64
	if err := tx.QueryRow(`SELECT total_changes()`).Scan(&before); err != nil {
		return err
	}
	ferr := fn(tx)
	// Count what fn changed even when it failed: total_changes keeps
	// counting rows a rollback undoes.
	if err := tx.QueryRow(`SELECT total_changes()`).Scan(&after); err != nil {
		return err
	}
	s.side.rows += after - before
	if ferr != nil {
		return ferr
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if personal {
		s.side.personal++
	}
	return nil
}

// Changes is the change counter of Snapshot, read on its own: a cheap way to
// know whether what was derived from the last snapshot is still current.
// Side writes do not count.
func (s *Store) Changes() (int64, error) {
	s.side.mu.Lock()
	defer s.side.mu.Unlock()
	var n int64
	err := s.db.QueryRow(`SELECT total_changes()`).Scan(&n)
	return n - s.side.rows, err
}

// PersonalVersion grows with every change to the user's data.
func (s *Store) PersonalVersion() int64 {
	s.side.mu.Lock()
	defer s.side.mu.Unlock()
	return s.side.personal
}
```

`Store` gets a field `side side`. In `Snapshot`, take `s.side.mu` **before** `s.db.Begin()` and hold it until the snapshot returns (`defer` unlock), and subtract `s.side.rows` from the `total_changes()` it reads; remove the old `Changes` from `snapshot.go` (keep its doc comment in `side.go` as above).

- [ ] **Step 1: Write the failing tests** in `internal/store/side_test.go`:
  - `TestSideWritesAreNotCatalogChanges`: `s := sample(t)`; `c0, _ := s.Changes()`; a `sideWrite(true, …)` that inserts a `marks` row and a `sideWrite(false, …)` that inserts a `loudness` row; `Changes()` still equals `c0`; `Snapshot().Changes` equals `c0`; `PersonalVersion()` is 1 (the loudness write is not personal); then `s.SetCollection(la, false)` (a catalog write) makes `Changes()` differ from `c0`.
  - `TestFailedSideWriteIsNotCounted`: fn inserts a `marks` row then returns an error: `sideWrite` returns that error, the row is not there, `Changes()` is unchanged, `PersonalVersion()` is 0.
  - In `migrate_test.go`, extend `TestOpenMigratesM1Catalog` (or add `TestOpenAddsUserTables`) so a catalog opened at v3 ends at `PRAGMA user_version` 4 with the five new tables present (`SELECT name FROM sqlite_master WHERE type='table'`) and its albums kept.
- [ ] **Step 2: Run** `go test ./internal/store -run 'Side|Migrat|UserTables'`: fails to compile (no `sideWrite`).
- [ ] **Step 3: Implement** as above.
- [ ] **Step 4: Run** `go test ./internal/store` and `go test ./...`: pass (server tests still use `Changes`).
- [ ] **Step 5: Commit** `feat: schema v4 with the user's tables; side writes don't move the catalog's change counter`

---

### Task 2: Marks, plays and `Personal` in the store

**Files:**
- Create: `internal/store/personal.go`
- Test: `internal/store/personal_test.go`

**Interfaces:**
- Consumes: `sideWrite`, `trackRef` (Task 1).
- Produces:

```go
// Kinds of marks.
const (
	KindTrack      = "track"
	KindAlbum      = "album"
	KindArtist     = "artist"
	KindCollection = "collection"
)

// Mark is a liked or rated track or album.
type Mark struct {
	Liked   bool  `json:"liked,omitempty"`
	LikedAt int64 `json:"likedAt,omitempty"` // unix ms
	Rating  int   `json:"rating,omitempty"`  // 1–5, 0 none
}

// Plays sums up a track's plays.
type Plays struct {
	Count int   `json:"count"`
	Last  int64 `json:"last"` // unix ms
}

// Personal is the user's data resolved to the catalog's current ids: only
// present tracks, their albums, and folders that exist.
type Personal struct {
	Tracks      map[int64]Mark   // marked tracks by file id
	Albums      map[int64]Mark   // marked albums by id
	Artists     map[string]int64 // liked artists: key → liked at
	Collections map[string]int64 // liked folders: path → liked at
	Plays       map[int64]Plays  // played tracks by file id
}

func (s *Store) LikeTrack(id int64, liked bool) (bool, error)  // false: no such present track
func (s *Store) RateTrack(id int64, stars int) (bool, error)   // stars 0 (clear) to 5
func (s *Store) LikeAlbum(id int64, liked bool) (bool, error)  // false: no such album
func (s *Store) RateAlbum(id int64, stars int) (bool, error)
func (s *Store) LikeArtist(key string, liked bool) error       // any non-empty key
func (s *Store) LikeCollection(path string, liked bool) (bool, error) // false: no such folder
func (s *Store) RecordPlay(id int64, at time.Time, listenedMs int64, device string) (bool, error)
func (s *Store) Personal() (Personal, error)
// LikedTrackIDs lists the present liked tracks, most recently liked first
// (one id per ref: the lowest file id).
func (s *Store) LikedTrackIDs() ([]int64, error)

var ErrBadRating = errors.New("el puntaje va de 0 a 5")
```

Rules:
- All writes go through `sideWrite(true, …)`.
- A like sets `liked = 1` and `liked_at = now` unless already liked (keeps the first time); an unlike sets both to 0. A rating writes `rating`. After either, delete the row if `liked = 0 AND rating = 0`.
- `LikeAlbum`/`RateAlbum` write one `album` row per **present** track of the album (`SELECT trackRef FROM tracks t JOIN files f ON f.id = t.file_id WHERE t.album_id = ? AND f.missing = 0`). No present track → `false`.
- `RecordPlay` stores the track's ref and file id with `played_at = at.UnixMilli()`; `device` trimmed and cut to 64 bytes (at a rune boundary).
- `Personal` reads in **one transaction** (single connection: read through the tx), each query fully consumed before the next:
  - tracks: `SELECT t.file_id, m.liked, m.liked_at, m.rating FROM tracks t JOIN files f ON f.id = t.file_id JOIN marks m ON m.kind = 'track' AND m.ref = (trackRef) WHERE f.missing = 0`
  - albums: same join with `m.kind = 'album'`, `WHERE t.album_id IS NOT NULL AND f.missing = 0 GROUP BY t.album_id`, taking `MAX(liked), MAX(liked_at), MAX(rating)`
  - artists: `SELECT ref, liked_at FROM marks WHERE kind = 'artist' AND liked = 1`
  - collections: `… kind = 'collection' AND liked = 1 AND ref IN (SELECT path FROM folders)`
  - plays: `SELECT t.file_id, COUNT(*), MAX(p.played_at) FROM tracks t JOIN files f ON f.id = t.file_id JOIN plays p ON p.ref = (trackRef) WHERE f.missing = 0 GROUP BY t.file_id`
  - Every map is non-nil (empty when there is nothing).

- [ ] **Step 1: Write the failing tests** (`personal_test.go`, on `library(t)`; get ids with `albumNamed` and a small helper `trackNamed(t, s, title) int64` you add to this file, reading `tracks` by title):
  - `TestLikeAndRateTrack`: like "Signos" → `Personal().Tracks[id]` liked with `LikedAt > 0`; liking again keeps the same `LikedAt`; rate 4 → rating 4 and still liked; unlike → still in the map with rating 4, not liked; rate 0 → gone from the map and no `marks` rows left; rating 6 → `ErrBadRating`; unknown id → `false, nil`.
  - `TestMarksFollowTheAudio`: like "Signos"; then simulate a move: `SyncFiles` with the same file under a new path and the same fingerprint (copy the `catalog` helper's steps: new `FileRow`, `PendingReads`, `SaveReads` with `Fingerprint` equal to the old one, `regroup`); the new file id is liked in `Personal()`.
  - `TestAlbumMarks`: like and rate 5 the album "Obras completas" (3 tracks) → `Personal().Albums[id]` liked, rating 5; `Tracks` has none of its tracks (album marks are not track marks); `marks` has 3 `album` rows; unlike → not liked, rating 5 kept; unknown album → `false`.
  - `TestArtistAndCollectionLikes`: `LikeArtist("soda-stereo", true)` → in `Artists`; `LikeCollection(la, true)` → in `Collections`; `LikeCollection("../musica/nope", true)` → `false`; unlike both → gone.
  - `TestRecordPlay`: two plays of "Signos" and one of "Sur" → `Plays` counts 2 and 1, `Last` is the later `at`; device `"  Mac · Safari  "` is stored trimmed; a 100-byte device is cut to ≤ 64 bytes and still valid UTF-8; unknown id → `false`.
  - `TestPersonalWritesKeepTheCatalog`: `Changes()` before and after a like, a rating, an album like and a play are equal; `PersonalVersion()` grew by 4.
  - `TestLikedTrackIDs`: like "Sur" then "Signos" (sleep 2 ms between, or pass times through a package-level `now = time.Now` variable you can override in tests) → `[Signos, Sur]`.
- [ ] **Step 2: Run** `go test ./internal/store -run 'Like|Rate|Mark|Play|Personal'`: fails to compile.
- [ ] **Step 3: Implement** `personal.go`. Use a package-level `var now = time.Now` for timestamps so tests can control them.
- [ ] **Step 4: Run** `go test ./internal/store`: pass.
- [ ] **Step 5: Commit** `feat: likes, ratings and plays in the store, keyed by audio fingerprint`

---

### Task 3: Playlists in the store

**Files:**
- Create: `internal/store/playlists.go`
- Test: `internal/store/playlists_test.go`

**Interfaces:**
- Consumes: `sideWrite`, `trackRef`, `PlayItem`/`playItems`/`playQuery` (`library.go`, unchanged).
- Produces:

```go
// PlaylistSummary is a playlist as the Listas page shows it.
type PlaylistSummary struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Tracks     int     `json:"tracks"`     // entries, available or not
	DurationMs int64   `json:"durationMs"` // of every entry, as added
	Covers     []int64 `json:"covers"`     // up to 4 distinct albums with a picture, in list order
	UpdatedAt  int64   `json:"updatedAt"`
}

// PlaylistEntry is one entry: TrackID 0 when its track is no longer in the
// catalog (it keeps how it was called when added).
type PlaylistEntry struct {
	ItemID     int64  `json:"itemId"`
	TrackID    int64  `json:"trackId"`
	AlbumID    int64  `json:"albumId"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMs int64  `json:"durationMs"`
	AddedAt    int64  `json:"addedAt"`
}

type Playlist struct {
	PlaylistSummary
	Entries []PlaylistEntry `json:"entries"`
}

var (
	ErrPlaylistName  = errors.New("escribí un nombre para la lista")
	ErrPlaylistFull  = errors.New("la lista llegó al máximo de pistas")
	ErrPlaylistOrder = errors.New("el orden no coincide con las pistas de la lista")
)

const MaxPlaylist = 5000

func (s *Store) Playlists() ([]PlaylistSummary, error)              // most recently updated first
func (s *Store) Playlist(id int64) (Playlist, bool, error)
func (s *Store) CreatePlaylist(name string) (int64, error)
func (s *Store) RenamePlaylist(id int64, name string) (bool, error)
func (s *Store) DeletePlaylist(id int64) (bool, error)
func (s *Store) AddToPlaylist(id int64, trackIDs []int64) (int, bool, error) // added, playlist exists
func (s *Store) RemoveFromPlaylist(id, itemID int64) (bool, error)
func (s *Store) ReorderPlaylist(id int64, itemIDs []int64) (bool, error)
func (s *Store) PlayPlaylist(id int64) ([]PlayItem, bool, error)    // available entries in order
```

Rules:
- Names are trimmed; empty → `ErrPlaylistName`; longer than 200 runes → cut. Duplicate names are allowed.
- Every write is a `sideWrite(true, …)` and sets the playlist's `updated_at`.
- `AddToPlaylist` appends present tracks (unknown or missing ids are skipped, not an error) with their current title (MusicBrainz's when paired, as `playQuery` reads it), artist, album and duration; `pos` continues after the last one. Going over `MaxPlaylist` → `ErrPlaylistFull` and nothing added.
- `RemoveFromPlaylist` deletes the entry and renumbers `pos` 0…n−1. `ReorderPlaylist` requires `itemIDs` to be exactly the playlist's entry ids in some order (`ErrPlaylistOrder` otherwise) and writes `pos` from that order.
- Resolving an entry to a track: the saved `file_id` when it is present and still has the entry's ref; else the lowest present file id with that ref; else 0 (unavailable). Do it in Go after reading the entries (one query for the entries, one for the candidate files by ref: `SELECT trackRef, f.id FROM files f JOIN tracks t ON t.file_id = f.id WHERE f.missing = 0 AND (trackRef) IN (…)`), not with nested queries while rows are open.
- `Covers`: walk the available entries in order, collect distinct album ids whose album has a picture (`hasPicture`), stop at 4.
- `PlayPlaylist` returns `PlayItem`s of the available entries in list order (use `playItems` with `WHERE t.file_id = ?` per id, as `PlayTracks` does; reuse `PlayTracks`).

- [ ] **Step 1: Write the failing tests** (`playlists_test.go`, on `library(t)`):
  - `TestPlaylistLifecycle`: create "  Para correr  " → listed as "Para correr" with 0 tracks; add [Sur, Signos, Sur] → 3 entries (duplicates allowed) in that order, `Tracks` 3, `DurationMs` 3000, `Covers` = [Signos' album] (only it has a picture in `library`); rename to "Mañanas" → listed with the new name; empty name → `ErrPlaylistName`; delete → gone, its entries gone (cascade), `Playlist(id)` → `false`.
  - `TestPlaylistReorderAndRemove`: entries a, b, c; reorder [c, a, b] → that order; reorder with a missing or foreign id → `ErrPlaylistOrder`, order unchanged; remove b → [c, a] with `pos` 0, 1.
  - `TestPlaylistEntryWhoseTrackLeft`: add "Sur"; mark its file missing (`SyncFiles` without it, then `regroup`) → the entry is there with `TrackID 0` and the title "Sur"; `PlayPlaylist` skips it; put the file back (same fingerprint, new path) → resolved again to the new file id.
  - `TestPlaylistLimit`: with `MaxPlaylist` lowered (make it a `var`), adding past it → `ErrPlaylistFull` and nothing added.
  - `TestPlaylistWritesKeepTheCatalog`: `Changes()` unchanged across create/add/reorder/delete.
- [ ] **Step 2: Run** `go test ./internal/store -run Playlist`: fails to compile.
- [ ] **Step 3: Implement** `playlists.go`.
- [ ] **Step 4: Run** `go test ./internal/store`: pass.
- [ ] **Step 5: Commit** `feat: playlists in the store, entries by audio fingerprint`

---

### Task 4: Catalog: personal facets, "más escuchado", liked lists

**Files:**
- Create: `internal/catalog/personal.go`
- Modify: `internal/catalog/facets.go`
- Test: `internal/catalog/personal_test.go`

**Interfaces:**
- Consumes: `store.Personal` (Task 2).
- Produces:

```go
const (
	FacetLiked  = "megusta" // LikedYes: the album or track itself is liked
	FacetRating = "puntaje" // "1"–"5": rated that many stars or more
	LikedYes    = "si"
	OrderPlays  = "escuchado"
)

// ExploreFor is Explore with the user's data: the Me gusta and Puntaje
// facets and the Más escuchado order. p nil: no user data.
func (c *Catalog) ExploreFor(q Query, p *store.Personal, offset, limit int) Result
// Explore stays, as ExploreFor(q, nil, offset, limit).

// LikedLists are the user's likes as the Listas page shows them, most
// recently liked first. Only what is in the catalog.
type LikedLists struct {
	Tracks      []Track               `json:"tracks"`
	Albums      []Album               `json:"albums"`
	Artists     []Artist              `json:"artists"`
	Collections []store.FolderSummary `json:"collections"`
}

func (c *Catalog) Liked(p *store.Personal) LikedLists
```

Design:
- `FacetNames` gains `FacetLiked` and `FacetRating` at the end. `valid`: `megusta` only `"si"`; `puntaje` `"1"`–`"5"`. `orders`: append `OrderPlays` to every tab (albums: año, título, artista, agregado, escuchado; artists: título, cantidad, agregado, escuchado; tracks: artista, año, título, agregado, escuchado). `naturalDir(OrderPlays)` is `Desc` (already the default branch).
- Personal values are **not** stored in the items (they change without rebuilding the catalog). `personal.go` builds them per call:

```go
// ratingValues[r] are the Puntaje values of an item rated r: "1" to r.
var ratingValues = [6][]string{nil, {"1"}, {"1", "2"}, {"1", "2", "3"}, {"1", "2", "3", "4"}, {"1", "2", "3", "4", "5"}}

// personalValues gives an item's Me gusta and Puntaje values, nil when it
// has none.
func personalValues(m store.Mark, ok bool) map[string][]string
```

- `matches(values, extra, facets, except)` and `counter.add(values, extra, labels, facets)` take the extra values: a facet matches when the wanted value is in `values[f]` **or** `extra[f]`; the counter counts both. Labels of the new values come from the web app (`facets.js`), so the server's label is the value.
- Plays for sorting, computed only when `q.Order == OrderPlays`: tracks from `p.Plays[id].Count`; albums by summing their tracks (one pass over `c.Tracks` by `AlbumID`); artists by summing their albums (`ArtistKey`). `sortAlbums`, `sortArtists`, `sortTracks` take a `plays map[…]int` argument (nil unless needed) and compare by it for `OrderPlays`, falling back to the existing tie-breakers.
- On the Artistas tab, as today, facets filter albums and the artists of the matching albums are listed: `megusta=si` there lists the artists of liked albums.
- `Liked`: tracks from `p.Tracks` (liked only) found in the catalog; albums from `p.Albums`; artists from `p.Artists` whose key is a catalog artist; collections from `p.Collections` whose path is a catalog folder (`c.folders[path].FolderSummary`). Each sorted by `LikedAt` descending, then id/key/path. Never nil slices.

- [ ] **Step 1: Write the failing tests** (`personal_test.go`, on `build()`; read ids from the fixture):
  - `TestParseQueryPersonal`: `megusta=si` and `puntaje=3` kept; `megusta=no`, `puntaje=0`, `puntaje=6`, `puntaje=x` dropped; `orden=escuchado` accepted on all three tabs with `Desc`.
  - `TestExploreLiked`: a `Personal` with one liked album and one album rated 3 → `megusta=si` returns just the liked one; `puntaje=2` returns the rated one, `puntaje=4` none; the `puntaje` menu counts `"1"`, `"2"`, `"3"` once each; the `megusta` menu counts 1 with no facet applied; `ExploreFor(q, nil, …)` equals `Explore(q, …)`.
  - `TestExploreLikedTracks`: on `ver=pistas`, a liked track (not its album) matches `megusta=si`; a liked album's tracks do not.
  - `TestOrderByPlays`: plays on two tracks of different albums (5 and 2) → albums sorted by `escuchado` put the 5 first, the 2 second, then the rest by the usual tie-breakers; tracks and artists likewise; `dir=asc` reverses.
  - `TestLiked`: liked track, album, artist key and collection path in a `Personal`, plus an artist key and a path not in the catalog → `Liked` lists the four known ones, most recent first, and drops the unknown ones; empty `Personal` → four empty (non-nil) slices.
- [ ] **Step 2: Run** `go test ./internal/catalog`: fails to compile.
- [ ] **Step 3: Implement.** Update the existing callers of `matches` and `counter.add`; existing tests must pass unchanged.
- [ ] **Step 4: Run** `go test ./internal/catalog`: pass.
- [ ] **Step 5: Commit** `feat: Me gusta, Puntaje and Más escuchado in Explorar's catalog; liked lists`

