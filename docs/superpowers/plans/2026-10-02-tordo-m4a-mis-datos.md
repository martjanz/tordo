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

---

### Task 5: Server: marks, likes, ratings, plays, liked, Explorar

**Files:**
- Create: `internal/server/personal.go`
- Modify: `internal/server/server.go` (routes, `Server.pers` field)
- Modify: `internal/server/catalog.go` (`explore` uses the personal data)
- Test: `internal/server/personal_test.go`

**Interfaces:**
- Consumes: Tasks 2 and 4.
- Produces:
  - `func (s *Server) personal() (*store.Personal, error)`: cached in `personalState{mu sync.Mutex; changes, version int64; p *store.Personal}`, reloaded when `Store.Changes()` or `Store.PersonalVersion()` moved (ids move with scans).
  - Routes:

| Route | Body / query | Answer |
|---|---|---|
| `GET /api/marks` | | `{"tracks": {id: Mark}, "albums": {id: Mark}, "artists": [keys], "collections": [paths]}` (sorted lists; maps with ids as JSON keys) |
| `PUT /api/likes` (jsonOnly) | `{"kind": "track"\|"album"\|"artist"\|"collection", "id": n, "key": "…", "liked": bool}` (`id` for track/album, `key` for artist and collection) | 204; 400 bad kind or missing id/key; 404 unknown track/album/folder; 409 read-only |
| `PUT /api/ratings` (jsonOnly) | `{"kind": "track"\|"album", "id": n, "stars": 0–5}` | 204; 400 bad kind or stars (`ErrBadRating`'s text); 404; 409 |
| `POST /api/plays` (jsonOnly) | `{"id": n, "listenedMs": n, "device": "…"}` | 204; 400 negative `listenedMs`; 404 unknown track; read-only: 204 and nothing recorded |
| `GET /api/liked` | | `catalog.LikedLists` |
| `GET /api/explore` | as before | now `ExploreFor` with the personal data |

  - Errors in Spanish: `tipo inválido`, `falta el id`, `falta la clave`, `pista no encontrada`, `álbum no encontrado`, `carpeta no encontrada`, `cuerpo inválido`; read-only uses the existing `modo consulta: no se pueden guardar cambios`. Bodies are read with `io.LimitReader(r.Body, maxBody)` as the other endpoints do.

- [ ] **Step 1: Write the failing tests** (`personal_test.go`; `newStore` has one track in the album *Locura* under `../musica/Virus - Locura`; find ids through `/api/explore?ver=pistas` or the store):
  - `TestLikesEndpoint`: like the track, the album, the artist key `"virus"` and the folder `../musica/Virus - Locura` → each 204, and `/api/marks` shows them; `kind: "x"` → 400; track id 999 → 404; folder `../nope` → 404; no `Content-Type` → 415; `ReadOnly: true` → 409.
  - `TestRatingsEndpoint`: stars 4 on the track → `/api/marks` rating 4; stars 6 → 400; `kind: "artist"` → 400.
  - `TestPlaysEndpoint`: POST a play → 204 and `Store.Personal().Plays[id].Count == 1`; read-only server → 204 and still 1; id 999 → 404; `listenedMs: -1` → 400.
  - `TestExploreFollowsLikes`: `/api/explore?megusta=si` total 0; like the album → 1; unlike → 0. And the catalog is **not rebuilt** by the like: in package `server`, compare `s.cat.cat` (the pointer) before and after the like, with a call to `/api/explore` in between to build it first.
  - `TestLikedEndpoint`: like the track and the artist → `/api/liked` lists the track under `tracks` and the artist under `artists`; `albums` and `collections` are `[]`, not `null`.
- [ ] **Step 2: Run** `go test ./internal/server -run 'Likes|Ratings|Plays|Explore|Liked'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/server` and `go test -race ./internal/server ./internal/store`: pass.
- [ ] **Step 5: Commit** `feat: endpoints for likes, ratings, plays and liked lists; Explorar with the user's data`

---

### Task 6: Server: playlists, play sources

`/api/play`'s parsing becomes `resolve`, shared by playing and by adding to a playlist, and learns `?playlist=` and `?liked=1`.

**Files:**
- Create: `internal/server/playlists.go`
- Modify: `internal/server/media.go` (`play` through `resolve`)
- Modify: `internal/server/server.go` (routes)
- Test: `internal/server/playlists_test.go`, `internal/server/media_test.go`

**Interfaces:**
- Consumes: Task 3; `LikedTrackIDs` (Task 2).
- Produces:

```go
// httpError is a request the server refuses, with the status and text to answer.
type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string { return e.msg }

// resolve lists the tracks a play query names: album, folder, artist,
// track (repeatable), playlist (its available entries in order) or liked=1
// (the liked tracks, most recent first). Bad or unknown references are
// *httpError (400, 404).
func (s *Server) resolve(q url.Values) ([]store.PlayItem, error)

// what is a play query as a JSON body: {"album": 3}, {"folder": "…"},
// {"artist": "…"}, {"tracks": [1, 2]}, {"playlist": 4}.
type what struct {
	Album    *int64  `json:"album"`
	Folder   *string `json:"folder"`
	Artist   *string `json:"artist"`
	Tracks   []int64 `json:"tracks"`
	Playlist *int64  `json:"playlist"`
}

func (w what) values() url.Values
```

  - The missing-source error text becomes `falta qué reproducir: album, folder, artist, track, playlist o liked`.
  - Routes (writes 409 in read-only mode; POST/PUT wrapped in `jsonOnly`):

| Route | Body | Answer |
|---|---|---|
| `GET /api/playlists` | | `[]store.PlaylistSummary` |
| `POST /api/playlists` | `{"name": "…", "add": what?}` | 201 + the new summary; `add` is resolved **before** creating, so a bad `add` creates nothing |
| `GET /api/playlists/{id}` | | the playlist; each entry also has `"playable"` (false when unavailable or needing ffmpeg) and `"artistKey"` (from the catalog's track) |
| `PUT /api/playlists/{id}` | `{"name": "…"}` | 204; 400 `ErrPlaylistName`; 404 |
| `DELETE /api/playlists/{id}` | | 204; 404 |
| `POST /api/playlists/{id}/items` | `what` | 200 `{"added": n}`; 400 `ErrPlaylistFull`'s text; 404 |
| `DELETE /api/playlists/{id}/items/{item}` | | 204; 404 |
| `PUT /api/playlists/{id}/order` | `{"items": [itemIds]}` | 204; 400 `ErrPlaylistOrder`'s text; 404 |
| `GET /api/play?playlist={id}` / `?liked=1` | | as other play sources |

  - `play` and the playlist handlers turn an `*httpError` into `http.Error(w, e.msg, e.code)` and anything else into `fail(w, err)`.

- [ ] **Step 1: Write the failing tests** (`playlists_test.go`, on `mediaServer(t, "")`: two tracks of *Locura*, the WMA one unplayable without ffmpeg):
  - `TestPlaylistEndpoints`: `POST /api/playlists {"name": "Viaje", "add": {"album": A}}` → 201, 2 tracks; `GET /api/playlists` lists it; `GET /api/playlists/{id}` → 2 entries, the WMA one `playable: false`; add `{"tracks": [mp3]}` → `{"added": 1}`; reorder to reversed → 204 and that order; remove one entry → 204; rename → 204; `{"name": " "}` → 400; delete → 204 then 404.
  - `TestPlaylistErrors`: unknown playlist → 404 on every route; `add: {"album": 999}` → 404 and no playlist created; bad JSON → 400; read-only → 409 on POST/PUT/DELETE, GET still 200.
  - `TestPlayPlaylistAndLiked` (in `media_test.go`): a playlist with both tracks → `/api/play?playlist=` gives 1 item and `skipped: 1`; liking the MP3 track through the store → `/api/play?liked=1` gives it; `/api/play?playlist=999` → 404; `/api/play` with nothing → 400 with the new text.
  - Existing `TestPlayEndpoint` keeps passing.
- [ ] **Step 2: Run** `go test ./internal/server -run 'Playlist|Play'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/server`: pass.
- [ ] **Step 5: Commit** `feat: playlist endpoints; play a playlist or the liked tracks`

---

### Task 7: `internal/loudness`: math and ffmpeg analysis

Pure functions and the ffmpeg call, given whole. The store will import this package for its types (`Job`, `Track`), so it must **not** import `store`.

**Files:**
- Create: `internal/loudness/loudness.go`
- Test: `internal/loudness/loudness_test.go`

**Interfaces:**
- Produces: `Reference`, `Floor`, `ErrNoMeasure`, `Gain`, `FromGain`, `Part`, `AlbumLoudness`, `ParseEBUR128`, `Analyze`, `Job`, `Track`, `TrackGain`, `Gains` (below).

- [ ] **Step 1: Write the failing test** `internal/loudness/loudness_test.go`:

```go
package loudness

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// summary is ffmpeg 9's ebur128 output for a 440 Hz tone, trimmed.
const summary = `  Stream #0:0: Audio: pcm_s16le, 44100 Hz, mono, s16, 705 kb/s
[Parsed_ebur128_0 @ 0x126005490] Summary:

  Integrated loudness:
    I:         -22.2 LUFS
    Threshold: -32.2 LUFS

  Loudness range:
    LRA:        20.0 LU
    Threshold: -42.2 LUFS
    LRA low:   -42.2 LUFS
    LRA high:  -22.2 LUFS
[out#0/null @ 0x12600b930] video:0KiB audio:258KiB subtitle:0KiB other streams:0KiB global headers:0KiB muxing overhead: unknown
`

func TestParseEBUR128(t *testing.T) {
	if v, err := ParseEBUR128(summary); err != nil || v != -22.2 {
		t.Fatalf("got %v, %v", v, err)
	}
	for name, out := range map[string]string{
		"no summary": "    I:         -22.2 LUFS\n",
		"silence":    "Summary:\n\n  Integrated loudness:\n    I:         -70.0 LUFS\n",
		"inf":        "Summary:\n    I:         -inf LUFS\n",
		"empty":      "",
	} {
		if _, err := ParseEBUR128(out); err != ErrNoMeasure {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestGainMath(t *testing.T) {
	if g := Gain(-10); g != -8 {
		t.Errorf("Gain(-10) = %v", g)
	}
	if l := FromGain(-8); l != -10 {
		t.Errorf("FromGain(-8) = %v", l)
	}
	if _, ok := AlbumLoudness(nil); ok {
		t.Error("no parts: no loudness")
	}
	// Two equal halves keep their loudness; a loud track dominates a quiet
	// one of the same length (energy, not the mean of the LUFS values).
	if l, _ := AlbumLoudness([]Part{{-12, 1000}, {-12, 1000}}); math.Abs(l+12) > 1e-9 {
		t.Errorf("equal = %v", l)
	}
	l, _ := AlbumLoudness([]Part{{-10, 1000}, {-30, 1000}})
	if math.Abs(l-(-12.97)) > 0.01 {
		t.Errorf("mixed = %v, want ≈ -12.97", l)
	}
	// Longer tracks weigh more.
	if l, _ := AlbumLoudness([]Part{{-10, 3000}, {-30, 1000}}); l < -11.3 || l > -11.2 {
		t.Errorf("weighted = %v", l)
	}
}

func f(v float64) *float64 { return &v }

func TestGains(t *testing.T) {
	facts := map[int64]Track{
		1: {AlbumID: 7, DurationMs: 1000, TagTrack: f(-6), TagAlbum: f(-5)}, // tags win
		2: {AlbumID: 8, DurationMs: 1000, LUFS: f(-10)},
		3: {AlbumID: 8, DurationMs: 1000, LUFS: f(-10)},
		4: {AlbumID: 9, DurationMs: 1000, LUFS: f(-10)},
		5: {AlbumID: 9, DurationMs: 1000}, // not measured yet: album 9 has no album gain
		6: {DurationMs: 1000, LUFS: f(-14)}, // a loose track
		7: {AlbumID: 10, DurationMs: 1000, TagTrack: f(-4)},
		8: {AlbumID: 10, DurationMs: 1000, LUFS: f(-14)}, // a tag track gain counts as a measure
	}
	g := Gains(facts)
	check := func(id int64, track, album *float64) {
		t.Helper()
		got := g[id]
		eq := func(a, b *float64) bool { return (a == nil) == (b == nil) && (a == nil || math.Abs(*a-*b) < 0.006) }
		if !eq(got.Track, track) || !eq(got.Album, album) {
			t.Errorf("%d: got %+v", id, got)
		}
	}
	check(1, f(-6), f(-5))
	check(2, f(-8), f(-8))
	check(4, f(-8), nil)
	check(5, nil, nil)
	check(6, f(-4), nil)
	check(7, f(-4), f(-4))
}

// TestAnalyze runs the real ffmpeg on a generated tone, when there is one.
func TestAnalyze(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("sin ffmpeg")
	}
	file := filepath.Join(t.TempDir(), "tone.wav")
	if out, err := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi",
		"-i", "sine=frequency=440:duration=3", "-y", file).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	v, err := Analyze(context.Background(), ffmpeg, file)
	if err != nil || v > -15 || v < -30 {
		t.Fatalf("got %v, %v", v, err)
	}
	if err := os.WriteFile(file, []byte("not audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Analyze(context.Background(), ffmpeg, file); err == nil {
		t.Fatal("a broken file has no loudness")
	}
}
```

- [ ] **Step 2: Run** `go test ./internal/loudness`: fails to compile.
- [ ] **Step 3: Write** `internal/loudness/loudness.go`:

```go
// Package loudness gives tracks a gain toward a common loudness, from their
// ReplayGain tags or measured with ffmpeg's EBU R128 filter. It only reads
// the audio files.
package loudness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Reference is the loudness tracks are brought to, in LUFS: ReplayGain 2's.
const Reference = -18.0

// Floor is the quietest loudness taken as a measure: at or below it
// (silence, a broken decode) there is nothing to normalize.
const Floor = -70.0

// ErrNoMeasure means ffmpeg gave no usable integrated loudness.
var ErrNoMeasure = errors.New("ffmpeg no midió el volumen")

// Gain is the gain in dB that brings a track of this loudness to Reference.
func Gain(lufs float64) float64 { return Reference - lufs }

// FromGain is the loudness a ReplayGain gain implies.
func FromGain(gain float64) float64 { return Reference - gain }

// Part is a track of an album: its loudness and length.
type Part struct {
	LUFS       float64
	DurationMs int64
}

// AlbumLoudness is the loudness of tracks played in a row: the mean of
// their energy, weighted by duration (EBU R128 integrates energy, not
// LUFS values). false when there are no parts.
func AlbumLoudness(parts []Part) (float64, bool) {
	var energy float64
	var total int64
	for _, p := range parts {
		d := max(p.DurationMs, 1)
		energy += float64(d) * math.Pow(10, p.LUFS/10)
		total += d
	}
	if total == 0 {
		return 0, false
	}
	return 10 * math.Log10(energy/float64(total)), true
}

var integratedRe = regexp.MustCompile(`(?m)^\s*I:\s*(-inf|-?[0-9]+(?:\.[0-9]+)?)\s*LUFS`)

// ParseEBUR128 reads the integrated loudness from the summary ffmpeg's
// ebur128 filter writes at the end.
func ParseEBUR128(out string) (float64, error) {
	i := strings.LastIndex(out, "Summary:")
	if i < 0 {
		return 0, ErrNoMeasure
	}
	m := integratedRe.FindStringSubmatch(out[i:])
	if m == nil || m[1] == "-inf" {
		return 0, ErrNoMeasure
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || v <= Floor {
		return 0, ErrNoMeasure
	}
	return v, nil
}

// Analyze measures a file's integrated loudness with ffmpeg: its first audio
// stream, one thread, nothing written.
func Analyze(ctx context.Context, ffmpeg, file string) (float64, error) {
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostats", "-hide_banner", "-nostdin", "-threads", "1",
		"-i", file, "-map", "0:a:0", "-af", "ebur128=framelog=quiet", "-f", "null", "-")
	var out bytes.Buffer
	cmd.Stderr = &out
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	v, err := ParseEBUR128(out.String())
	if err != nil && runErr != nil {
		return 0, fmt.Errorf("%w: %s", ErrNoMeasure, lastLine(out.String()))
	}
	return v, err
}

// lastLine is the last non-empty line of ffmpeg's output: usually the error.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Job is a track to measure: its file id, its fingerprint ref and its path
// relative to the app folder.
type Job struct {
	ID   int64
	Ref  string
	Path string
}

// Track is what a track's gains come from (see the store's LoudnessFacts).
type Track struct {
	AlbumID    int64
	DurationMs int64
	TagTrack   *float64 // ReplayGain tags, dB
	TagAlbum   *float64
	LUFS       *float64 // measured by Analyze
}

// TrackGain is a track's gains in dB, nil when unknown.
type TrackGain struct {
	Track *float64 `json:"trackGain,omitempty"`
	Album *float64 `json:"albumGain,omitempty"`
}

// loudnessOf is a track's known loudness: from its track gain tag, else
// measured.
func loudnessOf(t Track) (float64, bool) {
	if t.TagTrack != nil {
		return FromGain(*t.TagTrack), true
	}
	if t.LUFS != nil {
		return *t.LUFS, true
	}
	return 0, false
}

func round(v float64) *float64 {
	r := math.Round(v*100) / 100
	return &r
}

// Gains computes every track's gains. facts must hold every track of the
// albums involved: an album gain without tags needs all of them measured.
func Gains(facts map[int64]Track) map[int64]TrackGain {
	albums := map[int64][]Part{}
	unknown := map[int64]bool{}
	for _, t := range facts {
		if t.AlbumID == 0 {
			continue
		}
		if l, ok := loudnessOf(t); ok {
			albums[t.AlbumID] = append(albums[t.AlbumID], Part{l, t.DurationMs})
		} else {
			unknown[t.AlbumID] = true
		}
	}
	out := make(map[int64]TrackGain, len(facts))
	for id, t := range facts {
		var g TrackGain
		if l, ok := loudnessOf(t); ok {
			g.Track = round(Gain(l))
		}
		switch {
		case t.TagAlbum != nil:
			g.Album = round(*t.TagAlbum)
		case t.AlbumID != 0 && !unknown[t.AlbumID]:
			if l, ok := AlbumLoudness(albums[t.AlbumID]); ok {
				g.Album = round(Gain(l))
			}
		}
		out[id] = g
	}
	return out
}
```

- [ ] **Step 4: Run** `go test ./internal/loudness -v`: pass (`TestAnalyze` runs where ffmpeg is installed, skips elsewhere).
- [ ] **Step 5: Commit** `feat: loudness package: gains toward -18 LUFS, album loudness, ffmpeg ebur128 analysis`

---

### Task 8: Loudness: store, runner, gains, config, status, wiring

**Files:**
- Create: `internal/store/loudness.go`
- Create: `internal/loudness/runner.go`
- Modify: `internal/config/config.go`
- Modify: `internal/server/server.go` (fields, status, route `GET /api/gains`)
- Modify: `internal/server/media.go` (gains on play items)
- Modify: `cmd/tordo/main.go`
- Test: `internal/store/loudness_test.go`, `internal/loudness/runner_test.go`, `internal/config/config_test.go`, `internal/server/media_test.go`, `internal/server/server_test.go`

**Interfaces:**
- Consumes: Task 7; `sideWrite` (Task 1).
- Produces:

```go
// store
func (s *Store) LoudnessPending(limit int) ([]loudness.Job, error)   // newest first
func (s *Store) LoudnessJobs(ids []int64) ([]loudness.Job, error)     // those of ids still to measure
func (s *Store) LoudnessPendingCount() (int, error)
func (s *Store) SaveLoudness(ref string, lufs *float64, errText string) error // side write, not personal
func (s *Store) LoudnessFacts(ids []int64) (map[int64]loudness.Track, error)

// loudness
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

type Runner struct {
	Catalog Catalog
	AppDir  string
	FFmpeg  string
	Analyze func(ctx context.Context, ffmpeg, file string) (float64, error) // nil: the package's Analyze
	// unexported: mu, running, queue of prioritized ids, done, pending, ctx/cancel, wg
}

func (r *Runner) Trigger()               // start a pass when none is running
func (r *Runner) Prioritize(ids []int64) // measure these first; starts a pass when idle
func (r *Runner) Status() Status
func (r *Runner) Close()                 // cancels the pass and waits for it

// config
type Config struct { …; Normalize *bool `json:"normalize,omitempty"` }
func (c Config) Normalizing() bool // nil: true

// server
type Loudness interface {
	Prioritize(ids []int64)
	Status() loudness.Status
}
// Server gains: Loudness Loudness (nil: no analysis), Normalize bool.
```

Rules:
- **Store.** A track needs measuring when it is present, its `audio.tags` has no `trackGain` (`json_extract(a.tags, '$.trackGain') IS NULL`), and its ref has no `loudness` row. One job per ref (the lowest file id). `LoudnessPending` orders by the newest `first_seen` of the ref. `LoudnessFacts(ids)` returns the facts of the given present tracks **and of every present track of their albums** (`t.album_id IN (SELECT album_id FROM tracks WHERE file_id IN (…))`), with `TagTrack`/`TagAlbum` from `json_extract(a.tags, '$.trackGain'|'$.albumGain')` and `LUFS` from `loudness` by ref (NULL stays nil). Query ids in chunks of 500. `SaveLoudness` upserts with `measured_at = now`, through `sideWrite(false, …)`.
- **Runner.** One goroutine at most. A pass loops: if prioritized ids are queued, take `Catalog.LoudnessJobs(those ids)` (clearing the queue); else `Catalog.LoudnessPending(20)`; none → the pass ends (`idle`). For each job, check for newly prioritized ids before it (they go first), then `Analyze(ctx, FFmpeg, appdir.Abs(AppDir, job.Path))`: a value → `SaveLoudness(ref, &v, "")`; an error other than the context's → `SaveLoudness(ref, nil, err.Error())` (so it is not retried); a store error → log `volumen: …` and end the pass. `Pending` is read with `LoudnessPendingCount` when a pass starts and decremented per job; `Done` counts measured jobs. `Close` cancels and waits. `Trigger`/`Prioritize` after `Close` do nothing.
- **Config.** `Load` of a missing file returns `Normalize: true` (a pointer), so a new `config.json` has `"normalize": true`. An existing file without the key reads as `nil` → `Normalizing()` true.
- **Server.** `/api/status` gains `"normalize": s.Normalize` and `"loudness"`: `s.Loudness.Status()`, or `{"state": "off"}` when nil. `/api/play` items gain `trackGain`/`albumGain` (embed `loudness.TrackGain` in `playItem`): one `LoudnessFacts` call for the returned ids, `loudness.Gains` on it; then `Prioritize` the first 20 returned ids without a track gain (when `s.Loudness != nil`). `GET /api/gains?track=1&track=2` (at most 100 ids, else 400) answers `{id: TrackGain}` for those ids and prioritizes the ones without a track gain.
- **main.** Look up ffmpeg before building the scanner. When not read-only **and** ffmpeg exists, build `loudness.Runner{Catalog: st, AppDir: appDir, FFmpeg: ffmpeg}`, set `server.Loudness` (only when non-nil, as with `Identifier`), and make the scanner's `OnDone` call both `runner.Trigger()` and `loud.Trigger()`; `a.close` closes it before the store. Without ffmpeg, log nothing new (the existing "sin ffmpeg" line covers it). `server.Normalize = cfg.Normalizing()`.

- [ ] **Step 1: Write the failing tests:**
  - `internal/store/loudness_test.go` (on `library(t)`; give one track a `TrackGain` tag through a custom `catalog` call): `LoudnessPending` lists every present track but the tagged one, newest first, each ref once; `SaveLoudness` of one removes it from the list and from `LoudnessPendingCount`; an error row (`nil`, "roto") removes it too; `LoudnessJobs([tagged, measured, other])` returns only `other`; `LoudnessFacts([Signos 01])` returns both *Signos* tracks with their album id, duration, the saved `LUFS` on one and nil on the other; `Changes()` unchanged by `SaveLoudness` and `PersonalVersion()` too.
  - `internal/loudness/runner_test.go`: a fake `Catalog` in memory (jobs by id, saved results) and a fake `Analyze` (returns −10 for most paths, an error for one, and blocks on a channel for a "slow" one): `Trigger` measures all and saves them (the failing one with its error text and nil LUFS); `Status` goes `running` → `idle` with `Done` counted; with a pass running on the slow job, `Prioritize([x])` makes `x` the next one measured (check the order of saves); `Close` during the slow job returns (the fake `Analyze` honors `ctx`) and nothing more is saved. Use channels, not sleeps, to sequence.
  - `internal/config/config_test.go`: a new config has `Normalizing()` true and its JSON contains `"normalize": true`; a file with `"normalize": false` → false; a file without the key → true.
  - `internal/server/media_test.go` `TestPlayGains`: in `mediaServer`, save loudness −10 for the MP3's ref (its fingerprint is its path, see the helper) → `/api/play?album=` gives it `trackGain: -8` and, since the WMA track is unmeasured, no `albumGain`; a fake `Loudness` records `Prioritize` calls → the WMA id was prioritized; `/api/gains?track=<mp3>` → `{"<mp3>": {"trackGain": -8}}`; 101 ids → 400.
  - `internal/server/server_test.go`: `/api/status` has `"normalize"` and `"loudness": {"state": "off"}` with no runner.
- [ ] **Step 2: Run** the new tests: they fail.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...` and `go test -race ./internal/loudness ./internal/server ./internal/store ./cmd/tordo`: pass.
- [ ] **Step 5: Commit** `feat: measure loudness in the background with ffmpeg; gains on play items; normalize in config`

