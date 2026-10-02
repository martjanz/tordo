# Tordo M4a (Escucha I: me gusta, puntajes, historial, listas, normalización) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The listener's own data: ♥ me gusta on tracks, albums, artists and collections; ★ 1–5 on tracks and albums; every play recorded (≥ 50 % or ≥ 4 min); playlists (create, rename, reorder by dragging, add from any track/album/folder/artist, play); Explorar's facets "Me gusta" and "Puntaje" and the order "Más escuchado"; the Listas page with playlists, Me gusta and collections; and volume normalization (ReplayGain tags, else ffmpeg's EBU R128 analysis in the background), applied by the player.

**Architecture:** User data and measured loudness live in new tables of `tordo.db`, keyed by the track's **audio fingerprint** (as corrections are), so they survive moving, renaming and re-tagging. Their writes are **side writes**: they do not move the catalog's change counter, so liking a track or recording a play never rebuilds the in-memory catalog. The store resolves the user's data to current ids (`store.Personal`), cached by the server per (catalog changes, personal version), and `catalog` applies it as extra facet values at query time. A new package `internal/loudness` holds the gain math, the ffmpeg analysis and a background runner. The web app gets a small client store of marks, a play tracker and the gain choice as tested pure modules, the ♥/★ controls, the playlist page and the new Listas.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`), Svelte 5 + Vite 8 + Vitest 5 (as in M3).

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish). M4 in §10 is "Inicio, estaciones, historial y estadísticas, me gusta y puntajes, listas de reproducción, normalización de volumen". It is split like M3: **M4a (this plan)** covers §4 "Datos de usuario" (but saved stations), §7.1 "Normalización", §7.3's counting rule (stats are M4b), §7.4 whole, §8 Listas, the ♥/★ of Álbum, Artista and Colección, and Explorar's "me gusta", "puntaje" and "más escuchado" (§8). **M4b** (separate plan, after this one merges) covers §7.2 Estaciones (with saved stations, "Me gusta" as a source and likes/ratings as weights), §7.3 statistics, §8 Inicio, the artist page's "más escuchadas", playing a playlist as a station, and the "▶ Estación" buttons.

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

---

### Task 9: Web libraries

Everything the pages and the player need, as tested pure modules, plus the API calls and the client store of marks.

**Files:**
- Modify: `web/src/lib/api.js`, `web/src/lib/api.test.js`
- Modify: `web/src/lib/facets.js`, `web/src/lib/facets.test.js`
- Modify: `web/src/lib/router.js`, `web/src/lib/router.test.js`
- Modify: `web/src/lib/queue.js`, `web/src/lib/queue.test.js` (`peekPos`)
- Create: `web/src/lib/marks.js` + `marks.test.js`, `web/src/lib/mine.svelte.js`
- Create: `web/src/lib/plays.js` + `plays.test.js`
- Create: `web/src/lib/gain.js` + `gain.test.js`
- Create: `web/src/lib/device.js` + `device.test.js`
- Create: `web/src/lib/order.js` + `order.test.js`

**Interfaces:**
- Consumes: the endpoints of Tasks 5, 6 and 8.
- Produces:

```js
// api.js: playQuery also writes {playlist: id} and {liked: true} ("liked=1").
api.marks()                                  // GET /api/marks
api.like({ kind, id, key }, liked)           // PUT /api/likes: id for track/album, key for artist/collection
api.rate({ kind, id }, stars)                // PUT /api/ratings
api.recordPlay({ id, listenedMs, device })   // POST /api/plays
api.liked()                                  // GET /api/liked
api.gains(ids)                               // GET /api/gains?track=…
api.playlists()                              // GET /api/playlists
api.playlist(id)                             // GET /api/playlists/{id}
api.createPlaylist(name, add)                // POST /api/playlists {name, add?}
api.renamePlaylist(id, name)                 // PUT /api/playlists/{id}
api.deletePlaylist(id)                       // DELETE /api/playlists/{id}
api.addToPlaylist(id, what)                  // POST /api/playlists/{id}/items (what: {album}|{folder}|{artist}|{tracks}|{playlist})
api.removeFromPlaylist(id, itemId)           // DELETE /api/playlists/{id}/items/{itemId}
api.reorderPlaylist(id, itemIds)             // PUT /api/playlists/{id}/order {items}

// marks.js (pure; marks never mutated, every change returns a new object)
// marks: { tracks: {id: {liked, likedAt, rating}}, albums: {id: {…}}, artists: {key: true}, collections: {path: true} }
export function empty()
export function fromServer(json)               // arrays of keys/paths → objects
export function isLiked(m, kind, ref)          // kind: 'track' | 'album' | 'artist' | 'collection'
export function ratingOf(m, kind, id)          // 0 when none
export function withLike(m, kind, ref, liked)
export function withRating(m, kind, id, stars) // 0 clears

// mine.svelte.js
export const mine = $state({ marks: M.empty(), loaded: false })
export async function loadMine()               // GET /api/marks; errors leave the last marks
export async function toggleLike(kind, ref)    // optimistic; on error: roll back, notify(e.message)
export async function rate(kind, id, stars)    // same

// plays.js: see below.
// gain.js: see below.
// device.js
export function deviceName(ua)                 // "iPhone · Safari", "Mac · Firefox", "Android · Chrome", "Windows · Edge"…; "Navegador" when unknown
// order.js
export function move(list, from, to)           // a copy with list[from] moved to index to (clamped)
// queue.js
export function peekPos(q)                     // the play-order position peek() returns, -1 when none
// router.js
resolve('/listas/12') → { page: 'lista', id: 12 }; export function listHref(id)
// facets.js
FACETS gains { name: 'megusta', label: 'Me gusta' } and { name: 'puntaje', label: 'Puntaje' } (after 'coleccion', not in "Más");
NAMES gains 'megusta', 'puntaje' at the end (the server's order); every tab's ORDERS gains { value: 'escuchado', label: 'Más escuchado' } at the end;
LABELS.megusta = { si: 'Me gusta' }; LABELS.puntaje = { 5: '★★★★★', 4: '★★★★ o más', 3: '★★★ o más', 2: '★★ o más', 1: '★ o más' }; valueLabel uses them.
```

`plays.js` (given whole: the counting rule of the spec):

```js
// When a play counts (spec §7.3): 4 minutes, or half the track, of actual
// listening. Only forward progress in small steps counts, so seeking ahead
// or a stall does not. Pure: the player keeps the state.

export const MIN_MS = 4 * 60 * 1000
const MAX_STEP = 2 // seconds between two time updates that still count as listening

export function counts(listenedMs, durationMs) {
  return listenedMs >= MIN_MS || (durationMs > 0 && listenedMs >= durationMs / 2)
}

// begin starts a pass over entry: a new play to count.
export function begin(entry) {
  return { id: entry?.id ?? null, durationMs: entry?.durationMs ?? 0, listenedMs: 0, last: null, sent: false }
}

// tick takes the element's current time, in seconds.
export function tick(t, seconds) {
  if (t.id == null) return t
  const step = t.last == null ? 0 : seconds - t.last
  const listenedMs = step > 0 && step <= MAX_STEP ? t.listenedMs + step * 1000 : t.listenedMs
  return { ...t, listenedMs, last: seconds }
}

// due tells when the pass should be recorded: once.
export function due(t) {
  return t.id != null && !t.sent && counts(t.listenedMs, t.durationMs)
}

export function sent(t) {
  return { ...t, sent: true }
}
```

`gain.js` (given whole: decision 5):

```js
// The normalization gain of a queued track. Entries carry trackGain and
// albumGain (dB, from the server) when known. Only attenuates.
import * as Q from './queue.js'

export const UNKNOWN_DB = -6 // a track not measured yet: typical for modern masters

// gainDb is the gain of the entry at position pos of the play order: the
// album's when the queue plays that album in order (shuffle off and a
// neighbor in play order from the same album), else the track's.
export function gainDb(q, pos) {
  const key = q.order[pos]
  const entry = q.list.find((e) => e.key === key)
  if (!entry) return 0
  const albumOf = (p) => (p >= 0 && p < q.order.length ? q.list.find((e) => e.key === q.order[p])?.albumId : undefined)
  const inAlbum = !q.shuffle && entry.albumId && (albumOf(pos - 1) === entry.albumId || albumOf(pos + 1) === entry.albumId)
  if (inAlbum && entry.albumGain != null) return entry.albumGain
  return entry.trackGain ?? UNKNOWN_DB
}

// factor turns a gain into a volume multiplier, never above 1.
export function factor(db) {
  return Math.min(1, Math.pow(10, db / 20))
}

// missing lists the ids of up to count entries from position from of the
// play order whose track gain is not known yet (each id once).
export function missing(q, from, count) {
  const ids = []
  for (let p = Math.max(0, from); p < q.order.length && ids.length < count; p++) {
    const e = q.list.find((x) => x.key === q.order[p])
    if (e && e.trackGain == null && !ids.includes(e.id)) ids.push(e.id)
  }
  return ids
}

// withGains sets the gains the server answered ({id: {trackGain, albumGain}})
// on every entry of those tracks.
export function withGains(q, gains) {
  return { ...q, list: q.list.map((e) => (gains[e.id] ? { ...e, ...gains[e.id] } : e)) }
}
```

- [ ] **Step 1: Write the failing tests:**
  - `plays.test.js`: a 6-min track counts at 3 min of ticks (1 s steps) and not at 2:59; a 10-min track counts at 4 min; a jump of 120 s adds nothing; going back (step < 0) adds nothing; `due` is true once, false after `sent`; `begin(null)` never becomes due.
  - `gain.js`: album in order (shuffle off, neighbor same album) → `albumGain`; a lone track of that album between others → `trackGain`; shuffle on → `trackGain`; no gains → `UNKNOWN_DB`; `albumGain` missing → `trackGain`; `factor(0) === 1`, `factor(6) === 1`, `factor(-6) ≈ 0.501`; `missing` skips known ones and duplicates and respects `count`; `withGains` updates every entry with that id and leaves the rest.
  - `marks.test.js`: `fromServer` of a server answer; `withLike`/`withRating` return new objects (the input is unchanged), unlike keeps a rating, rating 0 on an unliked track removes the entry; `isLiked` and `ratingOf` for every kind, including unknown refs.
  - `device.test.js`: real user agents for iPhone Safari, iPad Safari, Android Chrome, Mac Safari, Mac Chrome, Mac Firefox, Windows Edge, Linux Firefox; `''` → `'Navegador'`.
  - `order.test.js`: forward, backward, same place, out-of-range `to` clamped; the input is not changed.
  - `queue.test.js`: `peekPos` with repeat off at the end (−1), repeat all at the end (0), repeat one (the same pos).
  - `facets.test.js`: `parse('?megusta=si&puntaje=4&orden=escuchado')` keeps all three; `toSearch` round-trips them; `valueLabel('puntaje', '4')` is `'★★★★ o más'`.
  - `router.test.js`: `/listas/12` → `{page: 'lista', id: 12}`, `/listas/0` and `/listas/x` → notfound, `listHref(12)`.
  - `api.test.js`: `playQuery({playlist: 3})` → `playlist=3`; `playQuery({liked: true})` → `liked=1`.
- [ ] **Step 2: Run** `cd web && npx vitest run`: the new tests fail.
- [ ] **Step 3: Implement.** `mine.svelte.js` imports `api` and `notify`; it has no tests (as `player.svelte.js`).
- [ ] **Step 4: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 5: Commit** `feat(web): marks, play counting, normalization gain, device names, playlist calls`

---

### Task 10: Player: plays and gains

**Files:**
- Modify: `web/src/lib/player.svelte.js`
- Modify: `web/src/App.svelte` (load the marks)

**Interfaces:**
- Consumes: `plays.js`, `gain.js`, `device.js`, `api.gains`, `api.recordPlay`, `Q.peekPos`, `loadMine` (Task 9); `app.status.normalize` (Task 8).

Behavior (no new exports from the player):
- **Gains.** Each `<audio>` element keeps the factor of the entry it holds in `a.dataset.gain`. `applyVolume` sets `a.volume = player.volume * (normalizing() ? Number(a.dataset.gain || 1) : 1)`, where `normalizing()` is `app.status?.normalize !== false`. `start(entry, …)` sets `active.dataset.gain = factor(gainDb(player.queue, player.queue.pos))` before playing; `preload()` sets `standby.dataset.gain` from `gainDb(player.queue, Q.peekPos(player.queue))`; both then call `applyVolume()`. The gain of the element that is playing never changes mid-track.
- **Fetching gains.** `refreshGains()`: `ids = missing(player.queue, player.queue.pos, 10)`; none → return; `api.gains(ids)` → `player.queue = withGains(player.queue, res)`, `persist()`; then, if `standby` holds an entry (its `dataset.key`) that is not playing, recompute its `dataset.gain` and `applyVolume()`. Errors are ignored (the next call tries again). Call it after `setQueue`, after `enqueue`, at the end of `start`, and at load (the restored queue).
- **Plays.** A module variable `pass = P.begin(null)`. `show(entry, …)` (used by both `start` and the swap in `onEnded`) sets `pass = P.begin(entry)`. `onTime` does `pass = P.tick(pass, active.currentTime)` and, when `P.due(pass)`, `api.recordPlay({ id: pass.id, listenedMs: Math.round(pass.listenedMs), device: deviceName(navigator.userAgent) }).catch(() => {})` and `pass = P.sent(pass)`. Repeat-one replays go through `start` → a new pass.
- **App.svelte:** an `$effect` that reads `app.generation` and calls `loadMine()` (ids can change after a scan).

- [ ] **Step 1:** Implement as above (the logic is tested in Task 9; this is wiring).
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser** (`go run ./cmd/tordo -dir .run` after the build, with a few albums in `.run`'s roots, ffmpeg installed): play an album; in the network panel, `GET /api/gains` follows the play; after half of a short track, `POST /api/plays` once (204); seek ahead past the half on another track → no play recorded until enough is actually heard. The `<audio>` elements are not in the DOM, so check the gain by logging `active.volume` temporarily (remove the log before committing), and by ear against the same track with `"normalize": false` in `config.json` (restart).
- [ ] **Step 4: Commit** `feat(web): the player records plays and normalizes volume`

---

### Task 11: ♥ and ★ in the pages

**Files:**
- Modify: `web/src/components/Icono.svelte` (icons `heart`, `star`, `trash`, `grip`)
- Create: `web/src/components/MeGusta.svelte`, `web/src/components/Puntaje.svelte`, `web/src/components/MenuPista.svelte`
- Modify: `web/src/components/ListaPistas.svelte`, `web/src/components/Reproductor.svelte`
- Modify: `web/src/pages/Album.svelte`, `web/src/pages/Artista.svelte`

**Interfaces:**
- Consumes: `mine`, `toggleLike`, `rate`, `isLiked`, `ratingOf` (Task 9); `enqueue` (player).
- Produces:
  - `<MeGusta kind ref size? />`: a ♥ toggle button (`aria-pressed`, `aria-label` "Me gusta" / "Quitar de Me gusta"), filled in the accent color when liked. In browse-only mode: no button; a static filled ♥ when liked, nothing otherwise.
  - `<Puntaje kind id size? />`: five star buttons in a `role="radiogroup"` labelled "Puntaje"; clicking the current rating clears it (0); arrow keys move the rating. Browse-only: the stars as text, nothing when unrated.
  - `<MenuPista track />`: a ⋯ button opening a small menu: "Reproducir a continuación", "Agregar a la cola", a `Puntaje` row (not in browse-only mode). Closes on choosing, outside click and Escape. Task 12 adds "Agregar a una lista…".

Feather paths for `Icono`:

```js
heart: '<path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/>',
star: '<polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/>',
trash: '<polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>',
grip: '<circle cx="9" cy="6" r="1"/><circle cx="15" cy="6" r="1"/><circle cx="9" cy="12" r="1"/><circle cx="15" cy="12" r="1"/><circle cx="9" cy="18" r="1"/><circle cx="15" cy="18" r="1"/>',
```

(Filled ♥/★: the component's CSS sets `fill: currentColor` on its `svg` when on.)

Placement:
- **ListaPistas** rows: a `MeGusta kind="track"` before the time (shown when liked; on row hover or focus otherwise, always on phones, like the `+` button) and a `MenuPista` after the `+`. Both need `pointer-events: auto` like `.add`.
- **Album** header: `MeGusta kind="album" ref={album.id}` and `Puntaje kind="album" id={album.id}` on a line above `Acciones`.
- **Artista** header: `MeGusta kind="artist" ref={artist.key}` next to the name's actions.
- **Reproductor**: `MeGusta kind="track" ref={entry.id}` next to the title of the current track (desktop bar and the phone sheet).
- Explorar needs nothing: its bar takes the new facets and orders from `facets.js`.

- [ ] **Step 1:** Implement. Follow the existing components' styles (`.icon` buttons, `var(--accent)`, `var(--muted)`).
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser:** like a track in an album, the album, its artist; rate the album 4 and a track 5; reload: all kept; `/explorar?megusta=si` lists the album and `?ver=pistas&megusta=si` the track; `?puntaje=4` the album; "Más escuchado" orders by the plays recorded in Task 10; phone width (375 px): rows still fit (title ellipsized).
- [ ] **Step 4: Commit** `feat(web): like and rate tracks, albums and artists`

---

### Task 12: Playlists and Listas in the web app

**Files:**
- Create: `web/src/components/AgregarALista.svelte`
- Create: `web/src/pages/Lista.svelte`
- Modify: `web/src/pages/Listas.svelte`, `web/src/App.svelte`, `web/src/components/Nav.svelte` (Listas active on `lista`)
- Modify: `web/src/components/Acciones.svelte`, `web/src/components/MenuPista.svelte`

**Interfaces:**
- Consumes: the playlist calls, `move`, `listHref`, `mine` (Task 9); `MeGusta` (Task 11); `play`, `enqueue`.
- Produces:
  - `<AgregarALista what onclose />`: a popover listing the playlists (most recently updated first) and a "Nueva lista…" field with "Crear". Choosing one calls `api.addToPlaylist(id, what)` and notifies `«N pistas agregadas a «Nombre».»` (use `count`); creating calls `api.createPlaylist(name, what)` and notifies `Lista «Nombre» creada con N pistas.`. Errors are notified. Closes on success, outside click and Escape.
  - `Acciones` gains a "+ Lista" button (hidden in browse-only mode) opening `AgregarALista` with its `what`; `MenuPista` gains "Agregar a una lista…" with `{ tracks: [track.id] }`.
  - Page **`/listas/{id}`** (`Lista.svelte`): header with the covers mosaic (`Tapa` with `covers.map((id) => albumCoverURL(id, 320))`, icon `lists`), label "Lista", the name (a "Renombrar" button turns it into an input: Enter saves, Escape cancels), "N pistas · duración", `Acciones what={{ playlist: id }}` and "Eliminar lista" (`confirm('¿Eliminar la lista «Nombre»? Las pistas no se tocan.')`, then go to `/listas`). Entries: a row each with a drag handle (`grip`), position, title, artist and album (links when known), duration, a ✕ (remove). A click on the row plays the list from that entry (`play({ playlist: id }, { startId: entry.trackId })`). Unavailable entries are dimmed with "no disponible" and do not play. Not found → "Esta lista ya no existe." Browse-only mode: no handle, ✕, rename or delete.
  - **Reordering:** pointer events on the handle (works with touch and mouse): `pointerdown` captures the pointer and remembers the row; `pointermove` computes the target index from the rows' vertical midpoints and shows the dragged row in its would-be place; `pointerup` applies `move(entries, from, to)` at once (optimistic), then `api.reorderPlaylist(id, entries.map((e) => e.itemId))`; on error reload the list and notify. The handle is a `<button>`: Alt+↑ / Alt+↓ move the entry one place (same call).
  - **Listas** page, three sections:
    1. "Listas de reproducción": cards (mosaic, name, "N pistas") linking to `/listas/{id}`, and a "Nueva lista" card that turns into a name field; creating goes to the new list. Empty: "Todavía no hay listas. Creá una acá o con «+ Lista» en un álbum, una carpeta o una pista."
    2. "Me gusta": tabs Pistas · Álbumes · Artistas · Colecciones with their counts (from `api.liked()`); Pistas: ▶ Reproducir and ⤮ Aleatorio (`play({ liked: true }, …)`) and `ListaPistas` with `showAlbum` (`onplay` plays the liked tracks from that one); Álbumes: `TarjetaAlbum` grid; Artistas: `TarjetaArtista` grid; Colecciones: cards as in section 3. Each empty tab: "Nada por acá todavía: tocá ♥ en lo que te guste."
    3. "Colecciones": as today, each card with a `MeGusta kind="collection" ref={c.path}`.
    Everything reloads on `app.generation`; the Me gusta lists also when `mine.marks` changes (a like elsewhere).

- [ ] **Step 1:** Implement.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser:** create a list from an album's "+ Lista"; add a track from its ⋯ menu and a folder from Carpetas; reorder by dragging (mouse) and with Alt+↓; at 375 px drag with touch emulation; play it, shuffled too; rename; remove an entry; delete the list. Listas shows the lists, Me gusta with its four tabs, collections with ♥.
- [ ] **Step 4: Commit** `feat(web): playlists, and Listas with Me gusta`

---

### Task 13: Sync with `main`, ♥ on the folder page

Another session rewrites Carpetas (`docs/superpowers/specs/2026-10-02-tordo-carpetas-explorador-design.md`). This task brings its work in and puts the collection's ♥ in whatever the folder page header is then.

**Files:**
- Modify: whatever the merge touches; `web/src/pages/Carpetas.svelte` (or the component that now draws the folder header)

- [ ] **Step 1:** `git fetch` is not needed (same repo): `git merge main`. Resolve conflicts keeping both sides' intent. Likely spots: `internal/server/server.go` (route list), `internal/store/library.go`, `internal/store/schema.sql`/`store.go` (if Carpetas also bumped `schemaVersion`, take the higher number + 1 for this branch's tables and update the migration test), `web/src/lib/api.js`, `web/src/App.svelte`, `web/src/components/ListaPistas.svelte`, `README.md`.
- [ ] **Step 2:** In the folder page header, next to "No es una colección / Es una colección", add `<MeGusta kind="collection" ref={path} />` when the folder is a collection.
- [ ] **Step 3: Run** `gofmt -l .`, `go vet ./...`, `go test ./...`, `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 4: Commit** the merge (its own message) and then `feat(web): like a collection from its folder page`.

---

### Task 14: Documentation

**Files:**
- Modify: `README.md`

- [ ] **Step 1:** Update `README.md` (Spanish, like the rest of it) with: ♥ and ★ (on what, where they show, that they follow the audio through moves and re-tagging); what counts as a play; playlists (adding from anywhere, reordering, entries whose file disappeared); Explorar's "Me gusta", "Puntaje" and "Más escuchado"; normalization (ReplayGain tags, else ffmpeg analysis in the background, newest first; −18 LUFS; album gain when an album plays in order; `"normalize": false` in `config.json` to turn it off); and the new endpoints in the endpoints list: `/api/marks`, `/api/likes`, `/api/ratings`, `/api/plays`, `/api/liked`, `/api/gains`, `/api/playlists…`, and `/api/play`'s `playlist` and `liked`.
- [ ] **Step 2: Commit** `docs: listening data, playlists and normalization in the README`

---

### Task 15: Check against the collection (manual, with the user)

Not for a subagent: it needs the user's judgment and their collection.

- [ ] Run against a copy of the real catalog (`go run ./cmd/tordo -dir <copy>` after `npm run build`), ffmpeg installed.
- [ ] `/api/status` → `loudness` running; note the measuring speed (tracks per minute) and the CPU use while browsing and playing; it must not make the page or the playback stutter.
- [ ] Explorar answers as fast as before with marks and plays present (the catalog is not rebuilt per like: time `/api/explore` before and after a like), and `?orden=escuchado` on the Pistas tab of the whole collection stays under ~100 ms.
- [ ] Play a loud modern album and a quiet old one back to back: similar level; the same album in order keeps its internal dynamics (album gain).
- [ ] Phone on the LAN is M5: check the phone layout at 375 px in the desktop browser.
- [ ] Move or rename an album folder on a test copy and rescan: its ♥, ★, plays and playlist entries are still there.
- [ ] Write the findings to `docs/superpowers/notes/2026-10-xx-m4a-collection-check.md` (as M2 and M3a did) and fix or plan what comes out.

