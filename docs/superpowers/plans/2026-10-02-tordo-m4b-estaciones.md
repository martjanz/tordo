# Tordo M4b (Escucha II: estaciones, estadísticas e Inicio) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stations that never end (a query plus a pace rule, fed 10 tracks at a time), saved stations, listening statistics by period, the Inicio page with fixed and rotating rows, "▶ Estación" wherever music is listed, and "Más escuchadas" on the artist page. Plus the M4a follow-ups worth doing now.

**Architecture:** Stations are pure catalog logic: `catalog.Station` (a `Source` plus rules) is cleaned and evaluated over the in-memory catalog with the user's data (`store.Personal`), and `Catalog.Next` picks the next tracks with weighted randomness, honoring the rules and relaxing them when the pool runs dry. The browser keeps the running station in its queue (`localStorage`) and asks `GET /api/station/next` for more when fewer than 3 tracks remain, sending what it already queued so the server never repeats them. Saved stations are a new user table (schema v5, only adding a table), written as side writes. Statistics need plays per period, read by a new store query that attributes each play to one current track; the catalog ranks them. Inicio's rows come from one catalog call with a seed (↻ draws another). A new Explorar facet, **Escucha** (`nunca`, `hace-un-anio`), makes "never played" and "not in a year" filters, Inicio rows and station sources at once.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`), Svelte 5 + Vite 8 + Vitest 5 (as in M4a).

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish): §7.2 Estaciones, §7.3 Historial y estadísticas (the counting rule is M4a's), §7.4 ("Me gusta" as a station source and likes/ratings as weights), §8 Inicio, Artista ("más escuchadas", "▶ estación"), Álbum/Colección/Carpeta ("estación"). The M4a plan (`2026-10-02-tordo-m4a-mis-datos.md`) set M4b's scope at its top; its decisions still hold.

## Decisions this plan takes (not spelled out in the spec)

1. **"/" becomes Inicio.** Inicio is the first section of the nav, as the spec lists it (Inicio · Explorar · Carpetas · Listas · Revisar). Explorar lives at `/explorar` (it already does). Old links of the form `/?pais=AR` still open Explorar: `/` with any Explorar parameter (`ver`, `orden`, `dir` or a facet name) resolves to Explorar.
2. **A station** is `{name, source, mode, artistGap, avoidDays}`:
   - `source` is exactly one of: an **Explorar query** (`view` + `facets`; no facets = the whole library), a **folder** (recursive; collections are folders), a **playlist** (its available entries), or **liked** (Me gusta).
   - Explorar sources: `view: "pistas"` takes the tracks that match (as Explorar's Pistas tab); `view: "albumes"` takes every track of the albums that match (as Álbumes and Artistas). An artist station is `{view: pistas, facets: {artista: key}}`: the artist's albums and their tracks on compilations (a track carries both its album's artist and its own).
   - **Liked** takes the union of: liked tracks, the tracks of liked albums, the tracks whose artist (the track's or its album's) is liked, and the tracks under liked collections.
   - **An album's station** is not "this album on loop": it is the album's **style** (its first genre link with a style), else its **genre**, else its **artist**, else its **decade**, as an Explorar source with `view: pistas`. Album ids change with rescans, so a saved station must not hold one; facets are stable.
   - Only stable references are stored: facet values, folder paths, playlist ids, artist keys. Every source leaves out tracks that cannot play (`estado=no-reproducible`).
3. **Rules.** `mode`: `pistas` (track by track) or `albumes` (a whole album, then another; a loose track counts as an album of one). `artistGap` N (0–20, default 3): no artist again within the last N tracks. `avoidDays` X (0–365, default 7): leave out tracks played in the last X days (an album: any of its tracks). Besides, a station never repeats a track it already queued (the browser sends up to its last 100). When no track satisfies everything, rules are relaxed **one at a time, in this order**: artist gap, then avoid days, then "not already queued" (but never the track — or album — that played last, when there is another). A pool with nothing at all gives an empty batch: the browser stops the station and says so.
4. **Weights** (§7.2 "mayor peso a me gusta y puntajes altos"): every track starts at 1; ×3 when the track or its album is liked; ×2 when its artist (the track's or the album's) is liked; × {1: 0.25, 2: 0.5, 3: 1, 4: 2, 5: 3} by the highest of the track's and the album's rating. An album's weight (albums mode) uses the album's like and rating and its artist. Picks are weighted random without replacement.
5. **Batches.** `GET /api/station/next?def=<JSON>&recientes=1,2,3` answers 10 tracks (albums mode: whole albums until there are at least 10, at least one album), shaped like `/api/play` (`items` with mime and gains, `ffmpeg`). The definition travels whole: the browser has it (also for saved stations), so the server needs no state per listener. The browser asks for more when fewer than 3 tracks remain after the current one; "Reproducir" anything else ends the station; the queue drops played entries beyond the last 100 while a station plays, so it doesn't grow forever.
6. **Saved stations**: table `stations (id, name, def, created_at, updated_at)`; `def` is the station's JSON without the name; most recently updated first. Saved from the queue panel ("Guardar"). Shown in Inicio's "Tus estaciones", which always starts with two built-ins: "Toda la colección" and "Tus me gusta" (the latter when something is liked).
7. **Escucha facet** (`escucha`): `nunca` (never played) and `hace-un-anio` (played, but the last play is more than 365 days old). A track by its own plays; an album by its tracks' (never = none of its tracks; stale = its latest play is over a year old). Like `megusta`, computed at query time from `store.Personal`.
8. **Statistics** by period: `semana` (7 days), `mes` (30), `anio` (365), `siempre`. Most played tracks, albums, artists (the track's artist; artists not in the catalog as album artists are listed by name, without a link), countries (the track's album's) and genres (each of the album's genres gets the play). Plus totals (plays, time listened, distinct tracks) and the catalog-wide counts of never played and not-in-a-year **albums**, linked to Explorar. Each play is attributed to **one** current track: the file it was played from when it is still present with the same audio, else the lowest present file with that audio (`Personal.Plays` instead counts a play on every copy, which is right for "never played" and "más escuchado" but would double rankings). Page `/estadisticas` ("Tu escucha"), linked from Inicio.
9. **Inicio** (`GET /api/home?semilla=N`): fixed rows *Agregados recientemente* (12 albums, newest first; "ver todas" → `/explorar?orden=agregado`), *Tus estaciones* (decision 6), *Seguir escuchando* (12 albums by their latest play; "ver todas" → `/explorar?orden=escuchado`), *Tus me gusta* (12 liked albums, most recent first; "ver todas" → `/explorar?megusta=si`; ▶ Estación "Tus me gusta"). Then **3 rotating rows**, drawn with the seed among: a decade, a country, a genre or style, a collection, *Nunca escuchados*, *Sin escuchar hace un año*, *Álbumes al azar*; a row needs at least 4 albums to be drawn (the random one, 1). Each rotating row shows 12 random albums of its facet, "ver todas" (Explorar with the facet) and "▶ Estación" (`view: albumes` with the facet). Rows with nothing are hidden. The page draws a new seed per visit; one ↻ button redraws the rotating rows. Fixed rows besides *Tus me gusta* have no station (their order is not a filter).
10. **Artist "Más escuchadas"**: the artist's 5 most played tracks (all time, `Personal.Plays`, the artist's albums and appearances), shown above the albums when there is at least one.
11. **M4a follow-ups taken now:** an album counts as liked/rated only when **more than half** of its present tracks carry the album mark (a compilation holding one shared copy of a liked album's track no longer shows as liked); `PlayTracks` reads in batches (`IN (…)`); refs `id:N` are re-keyed to the fingerprint when it is read; loudness: a track whose measurement failed is "unmeasurable" (not prioritized again, not asked for again by the browser), and a file that exists but cannot be opened, or an ffmpeg I/O error, is not saved as a permanent failure. Left for later (kept in the follow-ups note, Task 15): M4a's web polish and server/store polish lists.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new Go dependencies** (only `modernc.org/sqlite v1.59.0`). No cgo. Randomness: `math/rand/v2`.
- npm: no new packages (only `svelte`, `vite`, `vitest`, `@sveltejs/vite-plugin-svelte`, `@fontsource-variable/inter`).
- **Never write to the music roots.** The only writes go to the app folder: `tordo.db`, `config.json`, `cache/`. Browse-only mode writes nothing: write endpoints answer 409, the pages hide their write actions (`app.status.readOnly`). Stations still play in browse-only mode (`/api/station/next` is a GET).
- Catalog paths are relative to the app folder, `/`-separated. Audio and pictures are asked for **by id**, never by path; folders by their catalog path.
- Server listens on `127.0.0.1` only (`localOnly`); write endpoints (POST/PUT) are wrapped in `jsonOnly`.
- SQLite `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open; inside a transaction read only through it.
- Schema: v5 only **adds** the `stations` table (`schemaVersion = 5`, `derivedVersion` stays 2). User tables are never dropped or rebuilt by a scan. User writes go through `sideWrite(true, …)`.
- `internal/catalog` stays pure (no SQL, no network); its clock is a package variable `now = time.Now` that tests replace.
- User-facing strings (UI, logs, errors) in **Spanish** (rioplatense: *elegí*, *escribí*); identifiers and comments in English. URL names in Spanish (`/estadisticas`, `escucha`, `semilla`, `periodo`, `recientes`); the JSON fields of a station in English (`source`, `mode`, `artistGap`, `avoidDays`), its mode values in Spanish (`pistas`, `albumes`) like Explorar's views. UI files named in Spanish (`Inicio.svelte`, `Estadisticas.svelte`, `BotonEstacion.svelte`).
- The web build (`internal/server/dist/`) is **not committed**. `go test ./...` passes without Node and **without network**; `go vet ./...` and `gofmt -l .` clean; `cd web && npx vitest run` passes; `cd web && npm run build` succeeds.
- Commit messages: conventional style (`feat:`, `fix:`, `test:`, `docs:`), **no `Co-Authored-By:` trailer**.
- Git in the worktree: plain single commands (no pipes, no `&&`: the worktree guard refuses them).

## Model per task

Per the user's rule (cheapest model that can do the job): **Haiku** for transcribing fully specified code and docs, **Sonnet** for implementing from prose and multi-file work, **Opus** only for the final whole-branch review.

| Task | What | Model |
|---|---|---|
| 1 | Store: schema v5, saved stations | Sonnet |
| 2 | Store: plays by period, batched `PlayTracks` | Sonnet |
| 3 | Store: album marks by majority, re-keyed `id:N` refs | Sonnet |
| 4 | Loudness: unmeasurable tracks, temporary failures | Sonnet |
| 5 | Catalog: the Escucha facet | Sonnet |
| 6 | Catalog: stations (`Station`, pool, weights, `Next`) | Sonnet |
| 7 | Catalog: Inicio, statistics, artist's most played | Sonnet |
| 8 | Server: saved stations, `/api/station/next` | Sonnet |
| 9 | Server: `/api/home`, `/api/stats`, artist top | Sonnet |
| 10 | Web libraries | Sonnet |
| 11 | Player: stations | Sonnet |
| 12 | Web: Estación buttons, the station in the queue, Más escuchadas | Sonnet |
| 13 | Web: Inicio and the nav | Sonnet |
| 14 | Web: Estadísticas | Sonnet |
| 15 | Documentation | Haiku |
| — | Final review of the branch | Opus |
| 16 | Check against the collection | manual (with the user) |

## File Structure

```
internal/store/schema.sql, store.go   v5: stations
internal/store/stations.go            NEW: saved stations CRUD
internal/store/tally.go               NEW: PlayTally (plays of a period, one track per play)
internal/store/library.go             PlayTracks in batches
internal/store/personal.go            album marks by majority
internal/store/reads.go               re-key id:N refs when a fingerprint is read
internal/store/loudness.go            LoudnessFacts: Failed
internal/loudness/loudness.go         Track.Failed, TrackGain.Unmeasurable
internal/loudness/runner.go           unopenable files and I/O errors are not saved
internal/server/media.go              prioritizeUnmeasured skips unmeasurable; deliver() shared with stations
internal/catalog/listening.go         NEW: Escucha facet values, album listening aggregates, clock
internal/catalog/station.go           NEW: Station, Source, Clean, pool, weights, Next
internal/catalog/home.go              NEW: Home rows
internal/catalog/stats.go             NEW: Stats, ArtistTop
internal/catalog/facets.go            FacetListened in FacetNames, valid, ExploreFor extras
internal/server/stations.go           NEW: /api/stations…, /api/station/next
internal/server/home.go               NEW: /api/home, /api/stats
internal/server/catalog.go            artist page with "top"
internal/server/server.go             routes
web/src/lib/                          stations.js (NEW), router, api, facets (escucha), queue (station, trimPlayed),
                                      gain (unmeasurable), player (stations)
web/src/components/                   BotonEstacion, EstacionAjustes, TarjetaEstacion, FilaAlbumes (NEW);
                                      Acciones (station), Cola (station header), Icono, Nav
web/src/pages/                        Inicio, Estadisticas (NEW); Album, Artista, Explorar, Lista, ContenidoCarpeta (buttons)
web/src/App.svelte                    routes inicio, estadisticas
README.md                             stations, Inicio, statistics, endpoints
```

## How to read the tasks

The plan gives the design, the exact interfaces, the test cases and the code where it is tricky; the rest is yours to write, matching the surrounding code (comment density, naming, idiom). Tests come first in each task: write them, see them fail, then write the code. Go tests use the existing helpers (`store`: `open`, `catalog`, `sample`, `library`, `albumNamed`, `regroup`, `trackNamed` from `personal_test.go`; `server`: `newStore`, `mediaServer`, `do`, `decode`, `itoa`; `catalog`: `fixture`, `build`). Svelte components have no tests: their logic lives in tested `web/src/lib/` modules, and each web task ends with `npx vitest run` and `npm run build`.

After every task: `gofmt -l .` (empty), `go vet ./...`, `go test ./...`, and for web tasks `cd web && npx vitest run && npm run build`. Then commit.

---

### Task 1: Store: schema v5, saved stations

**Files:**
- Modify: `internal/store/schema.sql`, `internal/store/store.go` (`schemaVersion = 5`; its comment gains "Version 5 (M4b) added saved stations.")
- Create: `internal/store/stations.go`
- Test: `internal/store/stations_test.go`, `internal/store/migrate_test.go`

Append to `schema.sql`:

```sql
-- Saved stations. def is the station as JSON (source and rules, see
-- catalog.Station) without its name; the store does not read it.
CREATE TABLE IF NOT EXISTS stations (
	id         INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	def        TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
```

**Interfaces:**

```go
// SavedStation is a station the user saved. Def is its definition as JSON,
// as the server wrote it.
type SavedStation struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Def       json.RawMessage `json:"def"`
	CreatedAt int64           `json:"createdAt"`
	UpdatedAt int64           `json:"updatedAt"`
}

var ErrStationName = errors.New("escribí un nombre para la estación")

func (s *Store) Stations() ([]SavedStation, error)                      // most recently updated first, then id descending; never nil
func (s *Store) Station(id int64) (SavedStation, bool, error)
func (s *Store) CreateStation(name string, def []byte) (int64, error)
func (s *Store) UpdateStation(id int64, name string, def []byte) (bool, error)
func (s *Store) DeleteStation(id int64) (bool, error)
```

Rules: names trimmed and cut like playlists (reuse `playlistName`'s logic through a shared helper that takes the error to return; empty → `ErrStationName`). `def` must be valid JSON (`json.Valid`), else an error `definición inválida`. Writes are `sideWrite(true, …)` and use `now()` for the times.

- [ ] **Step 1: Write the failing tests** (`stations_test.go`, on `open(t)`):
  - `TestStationLifecycle`: create "  Brasil  " with `{"mode":"pistas"}` → listed as "Brasil" with that def (compare as JSON) and `CreatedAt == UpdatedAt > 0`; create a second → listed first; update the first (new name and def) → it moves first, `UpdatedAt` grew (override `now`); `Station(id)` returns it; delete → gone, `Station` → `false`; update/delete of an unknown id → `false, nil`.
  - `TestStationValidation`: empty name → `ErrStationName` (create and update); `def` `{` → error, nothing written; a 300-rune name is cut to 200.
  - `TestStationWritesKeepTheCatalog`: on `sample(t)`, `Changes()` unchanged across create/update/delete; `PersonalVersion()` grew by 3.
  - `migrate_test.go`: the test that opens an older catalog now expects `PRAGMA user_version` 5 and the `stations` table present.
- [ ] **Step 2: Run** `go test ./internal/store -run 'Station|Migrat'`: fails to compile.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/store`: pass.
- [ ] **Step 5: Commit** `feat: schema v5 with saved stations`

---

### Task 2: Store: plays by period, batched `PlayTracks`

**Files:**
- Create: `internal/store/tally.go`
- Modify: `internal/store/library.go` (`PlayTracks`)
- Test: `internal/store/tally_test.go`, `internal/store/library_test.go`

**Interfaces:**

```go
// PlayTally is the plays of a period resolved to the catalog's current
// tracks: each play counts once, for one track (see PlayTally).
type PlayTally struct {
	Counts     map[int64]int // plays by file id; never nil
	Plays      int           // every play of the period, resolved or not
	ListenedMs int64
}

// PlayTally sums the plays since a time (the zero time: all of them). A play
// goes to the file it was played from when that file is present with the
// same audio, else to the lowest present file with that audio; a play whose
// audio is no longer in the catalog only counts in the totals.
func (s *Store) PlayTally(since time.Time) (PlayTally, error)
```

How (no nested queries while rows are open):
1. `SELECT ref, file_id, COUNT(*), SUM(listened_ms) FROM plays WHERE played_at >= ? GROUP BY ref, file_id` (`since.UnixMilli()`, or 0 for the zero time), fully read.
2. The present files of those refs: `SELECT trackRef, f.id FROM files f JOIN tracks t ON t.file_id = f.id WHERE f.missing = 0 AND (trackRef) IN (…)`, in chunks of 500 (`chunks`, `placeholders` from `loudness.go`; they take `[]int64` — add string variants or generalize them). Keep per ref the set of present ids and the lowest.
3. Attribute in Go.

`PlayTracks(ids)` keeps its contract (the given order, repeats kept, unknown ids skipped) but reads with `WHERE t.file_id IN (…)` per chunk of 500 and then orders in Go.

- [ ] **Step 1: Write the failing tests:**
  - `tally_test.go` (on `library(t)`; `RecordPlay` with explicit times): plays of "Signos" at day 1 and day 9 and of "Sur" at day 9 (listened 1000 ms each) → `PlayTally(zero)` counts 2 and 1, `Plays` 3, `ListenedMs` 3000; `PlayTally(day 5)` counts only the day-9 plays. A copy: add a second file with the same fingerprint as "Signos" (a compilation folder, through `SyncFiles`/`SaveReads`/`regroup` as `TestMarksFollowTheAudio` does) → plays recorded on the original still go to the original id (not doubled); mark the original missing → they go to the copy; mark both missing → not in `Counts`, still in `Plays`.
  - `library_test.go` `TestPlayTracksKeepsOrder`: `PlayTracks([c, a, 999, a])` → titles of c, a, a; with `MaxPlay`-like volume: 1200 ids (the same few repeated) returns 1200 items in order (crosses a chunk).
- [ ] **Step 2: Run** `go test ./internal/store -run 'Tally|PlayTracks'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/store ./internal/server`: pass.
- [ ] **Step 5: Commit** `feat: plays by period, one track per play; PlayTracks in batches`

---

### Task 3: Store: album marks by majority, re-keyed `id:N` refs

**Files:**
- Modify: `internal/store/personal.go` (`Personal`'s album query)
- Modify: `internal/store/reads.go` (`SaveReads`)
- Test: `internal/store/personal_test.go`, `internal/store/reads_test.go` (or the file where `SaveReads` is tested; create `rekey_test.go` if none fits)

**Design:**
- **Majority.** In `Personal`, an album is liked when **more than half** of its present tracks carry an album like, and rated (the highest rating among them) when more than half carry an album rating; `LikedAt` is the latest among the liking ones. Count per album the present tracks and the marked ones in one query, e.g.:

```sql
SELECT t.album_id,
	COUNT(*),
	SUM(CASE WHEN m.liked = 1 THEN 1 ELSE 0 END), MAX(CASE WHEN m.liked = 1 THEN m.liked_at ELSE 0 END),
	SUM(CASE WHEN m.rating > 0 THEN 1 ELSE 0 END), MAX(COALESCE(m.rating, 0))
FROM tracks t JOIN files f ON f.id = t.file_id
LEFT JOIN marks m ON m.kind = 'album' AND m.ref = (trackRef)
WHERE t.album_id IS NOT NULL AND f.missing = 0
GROUP BY t.album_id
HAVING SUM(CASE WHEN m.ref IS NOT NULL THEN 1 ELSE 0 END) > 0
```

  then in Go: liked when `2*likedN > total`, rated when `2*ratedN > total`; add to `Albums` only when liked or rated.
- **Re-keying.** When `SaveReads` sets a fingerprint on a file, rows keyed `id:<that file id>` in `marks`, `plays`, `playlist_items` and `loudness` move to the fingerprint, in the same transaction: `UPDATE OR IGNORE marks SET ref = ?fp WHERE ref = ?old` then `DELETE FROM marks WHERE ref = ?old` (a mark already on the fingerprint wins); plays and playlist items with a plain `UPDATE`; loudness `DELETE` (a new measure will be taken on the fingerprint). These are catalog writes (inside the scan's transaction), which is fine: a scan moves the counter anyway.

- [ ] **Step 1: Write the failing tests:**
  - `TestAlbumMarkNeedsMostTracks`: on `library(t)` plus a compilation folder holding a bit-identical copy (same fingerprint) of "Signos" next to two other tracks (build it with the `catalog`-like steps; the compilation must group as one album of 3 tracks): like and rate 5 the album *Signos* → `Personal().Albums[Signos]` liked and rated, the compilation **not** in `Albums`. Like the compilation itself → liked.
  - `TestAlbumMarkWithAMissingTrack`: like *Obras completas* (3 tracks); mark one file missing → still liked (2 of 2 present carry it).
  - `TestIDRefsFollowTheFingerprint`: a file saved with an empty fingerprint (a `ReadResult` without one) is liked, played and added to a playlist (refs `id:N`); then `SaveReads` gives it a fingerprint → `marks`, `plays` and `playlist_items` hold the fingerprint, nothing with `id:N` remains, and `Personal()` still shows the like and the play.
- [ ] **Step 2: Run** `go test ./internal/store -run 'AlbumMark|IDRefs'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/store ./internal/catalog ./internal/server`: pass (the M4a album-like tests must still pass: their albums are wholly marked).
- [ ] **Step 5: Commit** `fix: an album counts as liked or rated only with most of its tracks; id refs follow the fingerprint`

---

### Task 4: Loudness: unmeasurable tracks, temporary failures

**Files:**
- Modify: `internal/loudness/loudness.go` (`Track.Failed`, `TrackGain.Unmeasurable`, `Gains`)
- Modify: `internal/store/loudness.go` (`LoudnessFacts` fills `Failed`)
- Modify: `internal/loudness/runner.go` (`measure`)
- Modify: `internal/server/media.go` (`prioritizeUnmeasured`)
- Test: `internal/loudness/loudness_test.go`, `internal/loudness/runner_test.go`, `internal/store/loudness_test.go`, `internal/server/media_test.go`

**Design:**
- `loudness.Track` gains `Failed bool` (a `loudness` row with `lufs` NULL); `LoudnessFacts` sets it (`l.ref IS NOT NULL AND l.lufs IS NULL`).
- `loudness.TrackGain` gains `Unmeasurable bool \`json:"unmeasurable,omitempty"\``: true when the track has no track gain (no tag) and `Failed`. `Gains` sets it.
- `prioritizeUnmeasured` skips unmeasurable tracks (the browser will skip them too, Task 10).
- `Runner.measure`: replace the `os.Stat` check with opening the file (`os.Open` + `Close`): a file that exists but cannot be opened is a temporary failure (log, `abort`, nothing saved). An `Analyze` error whose text contains `Input/output error` or `Permission denied` is also temporary.

- [ ] **Step 1: Write the failing tests:**
  - `loudness_test.go` `TestGains`: a track with `Failed: true` and no tag → `Unmeasurable` true, no track gain; with a `TagTrack` → not unmeasurable.
  - `store/loudness_test.go`: after `SaveLoudness(ref, nil, "roto")`, `LoudnessFacts` gives that track `Failed: true`; a measured one `false`.
  - `runner_test.go`: a fake `Analyze` returning an error mentioning `Input/output error` → nothing saved and the pass ends; a job whose file exists but is unreadable (create it, `chmod 000`; skip the test when running as root or on Windows) → nothing saved.
  - `media_test.go`: after saving an error row for the WMA track's ref, `/api/play?album=` gives it `"unmeasurable": true` and the fake `Loudness` is **not** asked to prioritize it.
- [ ] **Step 2: Run** the new tests: they fail.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...` and `go test -race ./internal/loudness`: pass.
- [ ] **Step 5: Commit** `fix: tracks that cannot be measured are not asked for again; unreadable files are not a permanent failure`

---

### Task 5: Catalog: the Escucha facet

**Files:**
- Create: `internal/catalog/listening.go`
- Modify: `internal/catalog/facets.go`, `internal/catalog/personal.go`
- Test: `internal/catalog/listening_test.go`

**Interfaces:**

```go
const (
	FacetListened = "escucha"
	ListenedNever = "nunca"        // never played
	ListenedStale = "hace-un-anio" // played, but not in over a year
)

// staleAfter is how old the last play of something "sin escuchar hace un año" is.
const staleAfter = 365 * 24 * time.Hour

// now is the catalog's clock; tests replace it.
var now = time.Now

// listening is the user's plays summed up per album: computed once per call.
type listening struct {
	tracks map[int64]store.Plays // p.Plays (nil p: nil map)
	albums map[int64]store.Plays // per album: plays summed, latest play
	cutoff int64                 // unix ms: a latest play before it is stale
}

func (c *Catalog) listening(p *store.Personal) *listening   // nil p → nil
func (l *listening) trackValues(id int64) []string           // nil when l is nil
func (l *listening) albumValues(id int64) []string
```

Design:
- `FacetNames` gains `FacetListened` at the end; `valid` accepts only `nunca` and `hace-un-anio`.
- `personalValues` becomes `personalValues(m store.Mark, ok bool, listened []string)`: the `escucha` values join the map (non-nil when there are any).
- `ExploreFor` builds `l := c.listening(p)` once and passes `l.trackValues(t.ID)` / `l.albumValues(a.ID)` for each item. With `p == nil` no item has `escucha` values (so `escucha=nunca` finds nothing: there is no user data to say so).
- Album plays: walk `p.Plays`, look the track up (`c.Track`), add to its album (count sum, max `Last`).

- [ ] **Step 1: Write the failing tests** (`listening_test.go`, on `build()`, `now` fixed at a date and restored with `t.Cleanup`):
  - `TestParseQueryListened`: `escucha=nunca` and `escucha=hace-un-anio` kept, `escucha=x` dropped.
  - `TestExploreListened`: `Personal.Plays` with track 101 (Signos) played yesterday and 201 (Doble vida) played two years ago → albums `escucha=nunca` lists every album but Signos and Doble vida; `escucha=hace-un-anio` lists Doble vida; the `escucha` menu counts `nunca` and `hace-un-anio` right; on `ver=pistas`, `escucha=nunca` lists every track but 101 and 201 (102 is in Signos but was never played); with `p == nil`, `escucha=nunca` finds nothing.
- [ ] **Step 2: Run** `go test ./internal/catalog`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/catalog ./internal/server`: pass.
- [ ] **Step 5: Commit** `feat: Explorar's Escucha facet: never played, not in over a year`

---

### Task 6: Catalog: stations

**Files:**
- Create: `internal/catalog/station.go`
- Test: `internal/catalog/station_test.go`

**Interfaces:**

```go
// Station modes.
const (
	ModeTracks = "pistas"  // track by track
	ModeAlbums = "albumes" // a whole album, then another
)

// StationBatch is how many tracks a station adds at a time.
const StationBatch = 10

// Defaults and limits of the rules.
const (
	DefaultArtistGap = 3
	DefaultAvoidDays = 7
	MaxArtistGap     = 20
	MaxAvoidDays     = 365
	MaxRecent        = 200 // the most recent tracks Next takes into account
)

// Source is what a station plays from: one of an Explorar query (View and
// Facets; no facets: the whole library), a folder and everything under it,
// a playlist, or the liked music.
type Source struct {
	View     string            `json:"view,omitempty"`   // ViewTracks or ViewAlbums
	Facets   map[string]string `json:"facets,omitempty"`
	Folder   string            `json:"folder,omitempty"`
	Playlist int64             `json:"playlist,omitempty"`
	Liked    bool              `json:"liked,omitempty"`
}

// Station is a source plus the rules that pace it.
type Station struct {
	Name      string `json:"name,omitempty"`
	Source    Source `json:"source"`
	Mode      string `json:"mode"`
	ArtistGap int    `json:"artistGap"` // no artist again within this many tracks; 0: no rule
	AvoidDays int    `json:"avoidDays"` // leave out what was played in the last days; 0: no rule
}

// Clean returns the station as Next reads it: one kind of source (folder,
// then playlist, then liked, then the Explorar query), well formed facets
// only, a known mode and view (pistas by default), rules within limits, the
// name trimmed and cut to 200 runes.
func (s Station) Clean() Station

// StationInput is what the next tracks depend on besides the station.
type StationInput struct {
	Personal *store.Personal // nil: no user data (no weights, nothing to avoid)
	Playlist []int64         // the playlist's available tracks, for a playlist source
	Recent   []int64         // the tracks the station queued so far, oldest first (the last MaxRecent count)
	Rand     *rand.Rand      // math/rand/v2
}

// Next picks a station's next tracks (see the plan's decisions 2–4): about
// count of them, by album in albums mode. Empty when the source has nothing
// playable.
func (c *Catalog) Next(st Station, in StationInput, count int) []int64
```

**Design:**
- `Clean`: `Folder` trimmed; if non-empty, the source is only the folder. Else `Playlist > 0` → only it. Else `Liked` → only it. Else an Explorar query: `View` is `ViewAlbums` when it is `ViewAlbums` or `ViewArtists`, else `ViewTracks`; `Facets` keeps the entries whose name is in `FacetNames` and whose value is `valid` (an empty map becomes nil). For the non-query kinds, `View` and `Facets` are cleared. `Mode` is `ModeAlbums` or else `ModeTracks`. `ArtistGap` and `AvoidDays` clamped to `[0, Max…]`. The rules' defaults are the web app's to send (a JSON without them means 0: no rule).
- **Pool** (`func (c *Catalog) pool(src Source, p *store.Personal, playlist []int64) []*Track`): the catalog's tracks of the source, never one carrying `estado=no-reproducible`, in catalog order (by file id):
  - Explorar, `ViewTracks`: tracks matching the facets with the track's personal values (as `ExploreFor` on Pistas, Escucha included).
  - Explorar, `ViewAlbums`: tracks whose album matches with the album's personal values (as on Álbumes); loose tracks are out.
  - Folder: `t.Dir == f` or `strings.HasPrefix(t.Dir, f+"/")`.
  - Playlist: the given ids found in the catalog, each once.
  - Liked: a track liked, or its album liked, or `p.Artists` has its `ArtistKey` or its album's `ArtistKey`, or its `Dir` is a liked collection or under one. Nil `p` → empty.
- **Weights** (decision 4): `func trackWeight(p *store.Personal, c *Catalog, t *Track) float64` and `albumWeight(p, a *Album)`; nil `p` → 1.
- **Artist identity** for the gap: `artistOf(t)` is `t.ArtistKey`, else `"~" + t.anorm` (a name not in the catalog's artists); an album's (albums mode) is its `ArtistKey`, else, for compilations and albums without an artist, `"album:" + id` (never collides).
- **Avoid days**: a track is recent when `p.Plays[id].Last >= now() - AvoidDays days`; an album when any of its pool tracks is.
- **Tracks mode** — the core, given whole:

```go
// nextTracks picks count tracks of pool one at a time, each from the first
// non-empty tier of candidates: every rule, then without the artist gap,
// then without avoiding recent plays, then allowing what the station
// already queued (but not the last track, when there is another).
func (c *Catalog) nextTracks(st Station, in StationInput, pool []*Track, count int) []int64 {
	queued := map[int64]bool{}
	var history []string // artists of the queued tracks, oldest first
	for _, id := range in.Recent {
		queued[id] = true
		if t, ok := c.Track(id); ok {
			history = append(history, artistOf(t))
		}
	}
	last := int64(0)
	if len(in.Recent) > 0 {
		last = in.Recent[len(in.Recent)-1]
	}
	weights := make([]float64, len(pool))
	for i, t := range pool {
		weights[i] = trackWeight(in.Personal, c, t)
	}
	avoid := c.playedSince(in.Personal, st.AvoidDays)
	var out []int64
	for len(out) < count {
		gap := map[string]bool{}
		for _, a := range history[max(0, len(history)-st.ArtistGap):] {
			gap[a] = true
		}
		tiers := []func(t *Track) bool{
			func(t *Track) bool { return !queued[t.ID] && !avoid[t.ID] && !gap[artistOf(t)] },
			func(t *Track) bool { return !queued[t.ID] && !avoid[t.ID] },
			func(t *Track) bool { return !queued[t.ID] },
			func(t *Track) bool { return t.ID != last || len(pool) == 1 },
		}
		i := -1
		for _, ok := range tiers {
			if i = pick(pool, weights, ok, in.Rand); i >= 0 {
				break
			}
		}
		if i < 0 {
			break
		}
		t := pool[i]
		out = append(out, t.ID)
		queued[t.ID], last = true, t.ID
		history = append(history, artistOf(t))
	}
	return out
}

// pick chooses an index of pool among those ok accepts, with probability
// proportional to its weight; -1 when ok accepts none.
func pick(pool []*Track, weights []float64, ok func(*Track) bool, r *rand.Rand) int {
	total := 0.0
	for i, t := range pool {
		if ok(t) {
			total += weights[i]
		}
	}
	if total <= 0 {
		return -1
	}
	x := r.Float64() * total
	for i, t := range pool {
		if ok(t) {
			if x -= weights[i]; x < 0 {
				return i
			}
		}
	}
	for i := len(pool) - 1; i >= 0; i-- { // rounding: the last accepted one
		if ok(pool[i]) {
			return i
		}
	}
	return -1
}
```

  (`playedSince(p, days)` gives the set of track ids played in the last `days` days; empty for 0 days or nil `p`.)
- **Albums mode**: group the pool by album (a loose track: a group of its own), each group's tracks in album order (`t.order`); the same tiers over groups — not containing a queued track, not avoided, its artist outside the last `ArtistGap` artists of the history — and the fourth tier "not the group of the last queued track, when there is another". Pick groups weighted by `albumWeight` (a loose track: `trackWeight`) until the batch holds at least `count` tracks; add each group's tracks to the history in order.
- `Next` = `Clean` is the caller's job (the server cleans); `Next` builds the pool, returns nil for an empty one, and dispatches on the mode. `Recent` longer than `MaxRecent`: only its last `MaxRecent` count.

- [ ] **Step 1: Write the failing tests** (`station_test.go`, on `build()`; `rand.New(rand.NewPCG(1, 2))`; `now` fixed):
  - `TestStationClean`: folder + playlist + facets → only the folder; playlist + liked → only the playlist; facets `{pais: "AR", decada: "1985", nope: "x"}` with view `artistas` → `{pais: "AR"}` and view `albumes`; mode `x` → `pistas`; gaps −1 and 99 → 0 and 20; avoid 9999 → 365; a 300-rune name cut.
  - `TestStationPool` (through `pool`): `{view: pistas, facets: {artista: soda}}` → 101, 102, 201 (501 is Soda's too, but WMA: unplayable in `build()`); `{view: albumes, facets: {pais: AR}}` → 101, 102; folder `la` → 101, 102, 301; playlist `[301, 999, 301]` → 301; liked with the album Signos liked and the artist `nameKey("Charly García")` liked → 101, 102, 301, 401; empty source → the 6 playable tracks (101, 102, 201, 301, 401 and the loose 901).
  - `TestNextTracksRules`: whole library, gap 0, avoid 0, `Recent` empty, count 10 → the 6 playable tracks once each first (no repeats while others remain), then repeats but never the same track twice in a row; with `Recent = [101]` the first pick is not 101.
  - `TestNextArtistGap`: source `{view: pistas, facets: {decada: "1980"}}` (101, 102, 201 by Soda; 301 by Charly), gap 1, count 2 with `Recent = [101]` → the first pick is 301 (the only other artist), then a Soda track.
  - `TestNextAvoidsRecentPlays`: plays of 101 and 102 yesterday, avoid 7, gap 0. Source `{estilo: "Pop Rock"}` (only Signos), count 2 → still 101 and 102 (the rule relaxes: they are the only tracks). Source `decada=1980` (101, 102, 201, 301), count 2 → 201 and 301, in some order.
  - `TestNextWeights`: source `decada=1980`, rules off, 101 liked and rated 5 (weight 9), others 1 → over 3000 single picks with `Recent` empty, 101 is chosen between 65 % and 85 % of the time (9/12 = 75 %).
  - `TestNextAlbums`: mode albums, whole library, count 3 → whole albums in album order (Signos' 101 then 102 together), at least 3 tracks; `Recent = [101, 102]` → the first album is not Signos.
  - `TestNextEmpty`: a source with nothing (`pais=BR`) → empty; a playlist source with no ids → empty.
- [ ] **Step 2: Run** `go test ./internal/catalog -run 'Station|Next'`: fails to compile.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/catalog`: pass.
- [ ] **Step 5: Commit** `feat: stations in the catalog: sources, pace rules, weights`

---

### Task 7: Catalog: Inicio, statistics, artist's most played

**Files:**
- Create: `internal/catalog/home.go`, `internal/catalog/stats.go`
- Test: `internal/catalog/home_test.go`, `internal/catalog/stats_test.go`

**Interfaces:**

```go
// home.go
const HomeRowSize = 12

// Kinds of Inicio's rotating rows.
const (
	RowDecade     = "decada"
	RowCountry    = "pais"
	RowGenre      = "genero"
	RowStyle      = "estilo"
	RowCollection = "coleccion"
	RowNever      = "nunca"
	RowStale      = "hace-un-anio"
	RowRandom     = "azar"
)

// HomeRow is a rotating row: a facet value and some of its albums.
type HomeRow struct {
	Kind   string            `json:"kind"`
	Value  string            `json:"value"`  // "1980", "BR", "Rock", a folder path; "" for nunca, hace-un-anio, azar
	Label  string            `json:"label"`  // the value's label when it is not the value (a collection's name)
	Facets map[string]string `json:"facets"` // Explorar's, for "ver todas" and the row's station (view albumes); {} for azar
	Albums []Album           `json:"albums"`
}

// Home is Inicio's rows (saved stations are the server's to add).
type Home struct {
	Recent   []Album   `json:"recent"`   // newest first
	Continue []Album   `json:"continue"` // by their latest play
	Liked    []Album   `json:"liked"`    // most recently liked first
	Rotating []HomeRow `json:"rotating"`
}

func (c *Catalog) Home(p *store.Personal, seed uint64) Home

// stats.go
type TrackPlays struct {
	Track
	Plays int `json:"plays"`
}
type AlbumPlays struct {
	Album
	Plays int `json:"plays"`
}
type ArtistPlays struct {
	Key   string `json:"key"` // "" when the artist has no page
	Name  string `json:"name"`
	Plays int    `json:"plays"`
}
type ValuePlays struct {
	Value string `json:"value"`
	Plays int    `json:"plays"`
}

// Stats ranks what was played in a period.
type Stats struct {
	Plays      int           `json:"plays"`      // of the period
	ListenedMs int64         `json:"listenedMs"`
	Played     int           `json:"played"`     // distinct tracks
	Tracks     []TrackPlays  `json:"tracks"`
	Albums     []AlbumPlays  `json:"albums"`
	Artists    []ArtistPlays `json:"artists"`
	Countries  []ValuePlays  `json:"countries"`
	Genres     []ValuePlays  `json:"genres"`
	Never      int           `json:"never"` // albums never played (all time)
	Stale      int           `json:"stale"` // albums not played in over a year
}

func (c *Catalog) Stats(t store.PlayTally, p *store.Personal, limit int) Stats

// ArtistTop lists an artist's most played tracks (their albums and their
// appearances), most played first; at most limit, only played ones.
func (c *Catalog) ArtistTop(key string, p *store.Personal, limit int) []TrackPlays
```

**Design:**
- `Home`: `r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))`.
  - `Recent`: albums by `Added` descending (ties: id descending), first 12.
  - `Continue`: albums with plays, by the latest play of their tracks (from `p.Plays`) descending, first 12. Nil `p` → empty.
  - `Liked`: `c.Liked(p).Albums`, first 12.
  - Rotating: candidates, each with its albums (album-level matching with personal values, as on Álbumes). "≥ 4" below is a package variable `minRowAlbums = 4` (tests lower it):
    - decade: the decades with ≥ 4 albums, one at random → facets `{decada: v}`;
    - country: the countries with ≥ 4 albums → `{pais: v}`;
    - genre or style: the genres and styles with ≥ 4 albums, one at random among both → kind `genero` `{genero: g}` or `estilo` `{estilo: s}`;
    - collection: the collections (`c.folders` with `Collection`) with ≥ 4 albums → `{coleccion: path}`, label the folder's name;
    - `nunca`, `hace-un-anio`: ≥ 4 albums → `{escucha: …}`;
    - `azar`: ≥ 1 album → `{}`.
    Shuffle the seven kinds (genre/style is one kind) with `r`, keep the first 3 that qualify; each row's albums: 12 of its albums at random (`r.Perm`), never nil.
  - Never nil slices.
- `Stats`: totals from the tally (`Played` = number of ids in `Counts` that are catalog tracks); tracks by count (ties: `anorm`, `norm`, id); albums by the sum of their tracks; artists by the track's `ArtistKey`, else by its normalized name (key "", name as the track spells it); countries by the track's `pais` values; genres by its `genero` values; each list sorted by plays descending then value/name, cut to `limit`. `Never`/`Stale`: albums with `escucha` values `nunca`/`hace-un-anio` (Task 5's `listening`).
- `ArtistTop`: tracks whose `ArtistKey` is key or whose album's `ArtistKey` is key, with `p.Plays[id].Count > 0`, sorted by count descending (ties: album order), cut to `limit`; never nil.

- [ ] **Step 1: Write the failing tests** (on `build()`, `now` fixed):
  - `home_test.go` `TestHome`: with plays on 301 (latest) and 101, and Signos liked → `Recent` starts with Rock nacional (the highest `Added`); `Continue` = [Clics modernos, Signos]; `Liked` = [Signos]. The fixture's 5 albums are too few for the threshold of 4, so make it a package variable (`minRowAlbums = 4`): at 4, every seed gives only the `azar` row. Lowered to 2 (restored with `t.Cleanup`): decade 1980 (3 albums), decade 1990 (2), Rock (2), the collection `la` (2) and `nunca` (Doble vida, Bocanada, Rock nacional) qualify, the country AR (1 album) does not; over seeds 1–50 every call gives 3 rows of distinct kinds (genre and style count as one), each row's albums all carry its facet value, a country row never appears, and every qualifying kind appears for some seed; the same seed twice gives the same rows. With `p == nil`: `Continue` and `Liked` empty, not nil, and no `nunca` row.
  - `stats_test.go` `TestStats`: tally `{101: 3, 102: 1, 201: 2, 502: 1}` with `Plays` 8 (one unresolved) → tracks [101, 201, 102, 502]; albums [Signos 4, Doble vida 2, Rock nacional 1]; artists [Soda Stereo (soda key) 6, Sui Generis (key "") 1]; countries [AR 4]; genres [Rock 6, Latin 4]; `Plays` 8, `Played` 4; `limit` 2 cuts every list to 2.
  - `TestArtistTop`: plays on 101 (3), 501 (1, the Soda track on the compilation), 102 (0) → `ArtistTop(soda)` = [101, 501]; an unknown key → empty.
- [ ] **Step 2: Run** `go test ./internal/catalog -run 'Home|Stats|ArtistTop'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/catalog`: pass.
- [ ] **Step 5: Commit** `feat: Inicio's rows, listening statistics and an artist's most played tracks in the catalog`

---

### Task 8: Server: saved stations, `/api/station/next`

**Files:**
- Create: `internal/server/stations.go`
- Modify: `internal/server/media.go` (extract `deliver`), `internal/server/server.go` (routes)
- Test: `internal/server/stations_test.go`

**Interfaces:**
- `func (s *Server) deliver(items []store.PlayItem) (map[string]any, error)`: what `play` writes today after resolving and shuffling (playable filter, `MaxPlay` cut, gains, prioritizing); `play` and `stationNext` both use it.
- Routes:

| Route | Body / query | Answer |
|---|---|---|
| `GET /api/stations` | | `[{id, name, source, mode, artistGap, avoidDays, createdAt, updatedAt}]` |
| `POST /api/stations` (jsonOnly) | a `catalog.Station` | 201 + the saved station as listed; 400 `ErrStationName`'s text; 409 read-only |
| `PUT /api/stations/{id}` (jsonOnly) | a `catalog.Station` | 204; 400; 404 `estación no encontrada`; 409 |
| `DELETE /api/stations/{id}` | | 204; 404; 409 |
| `GET /api/station/next` | `def` (a `catalog.Station` as JSON), `recientes` (comma-separated ids, at most `MaxRecent`; bad ones ignored) | `deliver`'s answer for `Next(…, StationBatch)`; 400 `estación inválida` for a missing or malformed `def`; 404 `lista no encontrada` for a playlist source that does not exist |

- The saved station's def is `station.Clean()` with `Name` emptied, marshaled; the name is stored apart. Listing decodes each def back into a `catalog.Station` (a def that fails to decode is listed with its name and an empty source: the whole library).
- `stationNext`: decode, `Clean`, for a playlist source `Store.PlayPlaylist(id)` → ids; `catalog()` and `personal()`; `Next` with `rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))`; `Store.PlayTracks(ids)`; `deliver`. The answer also carries `"station"`: the cleaned station (the browser keeps what the server applied).

- [ ] **Step 1: Write the failing tests** (`stations_test.go`, on `mediaServer(t, "")`: two tracks of *Locura*, the WMA one unplayable):
  - `TestStationsEndpoints`: POST `{"name": "Locura", "source": {"folder": "<the album's folder>"}, "mode": "albumes", "artistGap": 3}` → 201 with an id and the cleaned fields; GET lists it; PUT a new name → 204, listed renamed; `{"name": " "}` → 400; DELETE → 204, then 404; read-only → 409 for POST/PUT/DELETE and GET still 200.
  - `TestStationNext`: `def={"source": {}}` → 1 item (the MP3; the WMA is unplayable), with `mime`; `recientes=<mp3 id>` → still the MP3 (the only track: tier 4); `def` missing or `{` → 400; a playlist source with an unknown id → 404; a playlist with both tracks → the MP3.
  - Existing `TestPlay*` keep passing (the `deliver` extraction).
- [ ] **Step 2: Run** `go test ./internal/server -run 'Station|Play'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/server` and `go test -race ./internal/server`: pass.
- [ ] **Step 5: Commit** `feat: saved stations and /api/station/next`

---

### Task 9: Server: `/api/home`, `/api/stats`, artist top

**Files:**
- Create: `internal/server/home.go`
- Modify: `internal/server/catalog.go` (`artist`), `internal/server/server.go` (routes)
- Test: `internal/server/home_test.go`

**Routes:**

| Route | Query | Answer |
|---|---|---|
| `GET /api/home` | `semilla` (uint64; missing or bad: random) | `catalog.Home` plus `"stations"`: the saved stations as `GET /api/stations` lists them |
| `GET /api/stats` | `periodo`: `semana`, `mes`, `anio`, `siempre` (default `mes`; anything else 400 `período inválido`); `limite` 1–100 (default 10) | `catalog.Stats` plus `"period"` (the one applied) |
| `GET /api/artists/{key}` | | as before plus `"top"`: `ArtistTop(key, p, 5)` |

`stats` computes `since` from `time.Now()` (7, 30, 365 days; `siempre`: the zero time), reads `Store.PlayTally(since)`, and passes `personal()` for `never`/`stale`.

- [ ] **Step 1: Write the failing tests** (`home_test.go`, on `newStore`/`mediaServer` as fits):
  - `TestHomeEndpoint`: `/api/home?semilla=7` twice → the same body; it has `recent` (the album), `continue` `[]`, `liked` `[]`, `stations` `[]`; after recording a play, `continue` has the album; after saving a station, `stations` has it.
  - `TestStatsEndpoint`: a play recorded now → `?periodo=semana` has `plays` 1 and the track first in `tracks`; `?periodo=x` → 400; no `periodo` → `"period": "mes"`.
  - `TestArtistTop`: a play of the artist's track → `/api/artists/{key}` has `top` with it and `"plays": 1`; no plays → `top` `[]`.
- [ ] **Step 2: Run** `go test ./internal/server -run 'Home|Stats|ArtistTop'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...`: pass.
- [ ] **Step 5: Commit** `feat: endpoints for Inicio, statistics and an artist's most played tracks`

---

### Task 10: Web libraries

**Files:**
- Create: `web/src/lib/stations.js` + `stations.test.js`
- Modify: `web/src/lib/router.js` + test, `web/src/lib/api.js` + test, `web/src/lib/facets.js` + test, `web/src/lib/queue.js` + test, `web/src/lib/gain.js` + test

**Interfaces:**

```js
// router.js
resolve('/') → { page: 'inicio' }
resolve('/', '?pais=AR') → { page: 'explorar' }   // any of ver, orden, dir or a facet name
resolve('/estadisticas', '?periodo=anio') → { page: 'estadisticas', period: 'anio' } // default 'mes'
export function statsHref(period)                   // '/estadisticas' for 'mes', else '?periodo=…'

// api.js
api.stationNext(station, recent)   // GET /api/station/next?def=<JSON of station without id>&recientes=1,2,3
api.stations()                     // GET /api/stations
api.createStation(station)         // POST /api/stations (station without id)
api.updateStation(id, station)     // PUT /api/stations/{id}
api.deleteStation(id)              // DELETE /api/stations/{id}
api.home(seed)                     // GET /api/home?semilla=…
api.stats(period, limit)           // GET /api/stats?periodo=…&limite=…
export function stationQuery(station, recent) // the query string of stationNext (tested)

// facets.js: FACETS gains { name: 'escucha', label: 'Escucha' } after 'puntaje'; NAMES gains 'escucha' at the end;
// LABELS.escucha = { nunca: 'Nunca escuchado', 'hace-un-anio': 'Sin escuchar hace un año' }.

// queue.js
empty() gains station: null; restore keeps a station object (else null).
export function trimPlayed(q, keep) // drops entries played before the last `keep` ones (in play order), when there are any; list, order and pos stay consistent

// gain.js: missing() also skips entries with unmeasurable: true.

// stations.js (pure)
export const DEFAULT_RULES = { mode: 'pistas', artistGap: 3, avoidDays: 7 }
export const LOW_WATER = 3          // ask for more when fewer than this remain after the current track
export const RECENT = 100           // ids sent as recientes
export const KEEP_PLAYED = 100      // played entries kept while a station plays
export function station(name, source, rules = DEFAULT_RULES) // { name, source, ...rules }
export function exploreStation(query, facets) // Explorar's query → { name, source: { view, facets } }; name from the applied facets' labels (valueLabel / the facets menus), 'Toda la colección' when none; artistas → view 'albumes'
export function albumStation(album)  // decision 2: style, genre, artist key, decade → { name, source } or null
export function artistStation(artist) // { name: artist.name, source: { view: 'pistas', facets: { artista: artist.key } } }
export function folderStation(folder) // { name: folder.name, source: { folder: folder.path } }
export function playlistStation(list) // { name: list.name, source: { playlist: list.id } }
export const LIKED = { name: 'Tus me gusta', source: { liked: true } }
export const LIBRARY = { name: 'Toda la colección', source: {} }
export function needsMore(q)         // q.station and fewer than LOW_WATER entries after pos in play order
export function recentIds(q)         // the ids of the last RECENT entries up to the end of the play order, oldest first
export function summary(st)          // 'Pistas · sin repetir artista en 3 · evita lo escuchado en 7 días'; 'Álbumes'; rules at 0 are left out
export function rowTitle(row)        // Inicio's rotating rows: decada '1980' → 'Los 80' ('1990' → 'Los 90', '2000' → 'Los 2000'), pais → country name, genero/estilo → the value, coleccion → label, nunca → 'Nunca escuchados', hace-un-anio → 'Sin escuchar hace un año', azar → 'Álbumes al azar'
```

- [ ] **Step 1: Write the failing tests:** each function above, including: `resolve('/', '?orden=titulo')` → explorar and `resolve('/', '?foo=1')` → inicio; `stationQuery` encodes the JSON without `id` and joins `recientes`, and omits `recientes` when empty; `trimPlayed` with pos 150 and keep 100 drops the first 50 of the play order (from `list` too), pos becomes 100, with shuffle on and off; `restore` of a queue with a station keeps it and of `station: 'x'` gives null; `needsMore` at the end of a 10-entry station queue (pos 7 → true, pos 5 → false, no station → false); `recentIds` caps at 100; `exploreStation` for `{view: 'artistas', facets: {pais: 'AR', genero: 'Rock'}}` → view `albumes`, name `'Argentina · Rock'`; `albumStation` for an album with genres `[{genre: 'Rock', style: 'Pop Rock'}]` → `{estilo: 'Pop Rock'}` named `'Pop Rock'`, without genres but with an artist key → the artist, with nothing but year 1986 → `{decada: '1980'}` named `'Los 80'`, with nothing → null; `summary` of the defaults; `missing` skips unmeasurable.
- [ ] **Step 2: Run** `cd web && npx vitest run`: the new tests fail.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 5: Commit** `feat(web): station, Inicio and statistics helpers; Escucha facet; "/" is Inicio`

---

### Task 11: Player: stations

**Files:**
- Modify: `web/src/lib/player.svelte.js`

**Behavior** (exports: `playStation`, `stopStation`, `setStation`):
- `playStation(st)`: `api.stationNext(st, [])`; errors → `notify(e.message)`; no items → `notify('Esta estación no tiene nada para sonar.')`; else `setQueue(Q.replace({ ...player.queue, shuffle: false, station: res.station ? { ...res.station, id: st.id } : st }, res.items))`. (Keep `id` for saved stations: the server's cleaned station has none.)
- `play(…)` sets `station: null` in the queue it builds (anything else played ends the station). `enqueue` keeps it.
- `topUp()`: when `S.needsMore(player.queue)` and no top-up is in flight: `api.stationNext(st, S.recentIds(player.queue))`; when the answer arrives, if `player.queue.station` is no longer the same object, drop it; else `Q.append` the items, `trimPlayed(…, KEEP_PLAYED)`, `persist()`, `refreshGains()`. No items → `notify('La estación se quedó sin pistas.')` and `station: null`. An error → `notify` once per station (a flag) and retry on the next call. Call it at the end of `start`, after the swap in `onEnded`, after `removeAt` and `clearUpcoming`, and at load when the restored queue has a station.
- `stopStation()`: `station: null`, persist (the queue stays). `setStation(st)`: replaces the station (rules or name changed, or it was saved and has an id), persist; the next top-up uses it.

- [ ] **Step 1:** Implement (the logic is tested in Task 10; this is wiring).
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser** later, with Task 12's buttons (start a station, let it play past the 7th track: one `GET /api/station/next` with `recientes`, the queue grows by 10).
- [ ] **Step 4: Commit** `feat(web): the player plays stations without end`

---

### Task 12: Web: Estación buttons, the station in the queue, Más escuchadas

**Files:**
- Modify: `web/src/components/Icono.svelte` (icons `radio`, `home`, `refresh`, `chart`)
- Create: `web/src/components/BotonEstacion.svelte`, `web/src/components/EstacionAjustes.svelte`
- Modify: `web/src/components/Acciones.svelte`, `web/src/components/Cola.svelte`, `web/src/components/ContenidoCarpeta.svelte`
- Modify: `web/src/pages/Album.svelte`, `web/src/pages/Artista.svelte`, `web/src/pages/Lista.svelte`, `web/src/pages/Explorar.svelte`

Feather paths:

```js
radio: '<circle cx="12" cy="12" r="2"/><path d="M16.24 7.76a6 6 0 0 1 0 8.49m-8.48-.01a6 6 0 0 1 0-8.49m11.31-2.82a10 10 0 0 1 0 14.14m-14.14 0a10 10 0 0 1 0-14.14"/>',
home: '<path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>',
refresh: '<polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>',
chart: '<line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/>',
```

**Interfaces and placement:**
- `<BotonEstacion station label? />`: a button with the `radio` icon and "Estación" that calls `playStation(station(…))` with `DEFAULT_RULES` unless the station carries rules. Disabled when `station` is null.
- `Acciones` gains an optional `station` prop: when given, a `BotonEstacion` after "Aleatorio".
- Album: `station={albumStation(album)}` (no button when null). Artista: `artistStation(artist)`. ContenidoCarpeta (folder header, collection or not): `folderStation(folder)`. Lista: `playlistStation(list)`. Explorar: a `BotonEstacion` in a line above the results with `exploreStation(data.query, data.facets)` (also with no facets: "Toda la colección").
- **Cola**: when `player.queue.station` is set, a header block above "Sonando": the `radio` icon, "Estación · {name}", `summary(station)`, and buttons "Ajustar" (opens `EstacionAjustes`) and "Detener" (`stopStation`).
- `<EstacionAjustes station onclose />`: a small dialog: name (text), "Modo" (Pistas / Álbumes), "No repetir artista en" (select: no importa = 0, 2, 3, 5, 10 pistas), "Evitar lo escuchado en los últimos" (select: no evitar = 0, 1, 7, 30, 90 días); "Aplicar" → `setStation`; "Guardar" (hidden in browse-only mode; disabled with an empty name) → `api.updateStation(id, …)` when the station has an `id`, else `api.createStation(…)` and `setStation` with the new id; notifies `Estación «Nombre» guardada.`; errors notified. Closes on Escape and outside click.
- **Artista "Más escuchadas"**: when `artist.top.length`, an `h2` "Más escuchadas" and a `ListaPistas` with `showAlbum` (playing from a row plays those tracks from it), above "Álbumes".

- [ ] **Step 1:** Implement. Follow the existing components' styles.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser** (`go run ./cmd/tordo -dir .run` after the build): start a station from an artist, an album, a folder, a list and Explorar filtered by country; the queue shows the station; let it pass the 7th track (or remove upcoming tracks) → it refills; "Ajustar" to albums mode → the next batch is whole albums; "Guardar" → `GET /api/stations` lists it; "Detener" → the queue ends with what it has; play an album → the station is gone. An artist with plays shows "Más escuchadas". 375 px: the buttons wrap.
- [ ] **Step 4: Commit** `feat(web): Estación buttons, the station in the queue, an artist's most played`

---

### Task 13: Web: Inicio and the nav

**Files:**
- Create: `web/src/pages/Inicio.svelte`, `web/src/components/FilaAlbumes.svelte`, `web/src/components/TarjetaEstacion.svelte`
- Modify: `web/src/App.svelte` (routes `inicio`; the not-found link goes to "/" "Ir al Inicio"), `web/src/components/Nav.svelte` (Inicio first: `{ page: 'inicio', href: '/', label: 'Inicio', icon: 'home' }`)

**Interfaces:**
- `<FilaAlbumes title href? station? onrefresh? albums />`: a row: the title, "ver todas →" when `href`, a `BotonEstacion` when `station`, a ↻ (`refresh`) button when `onrefresh`; the albums as `TarjetaAlbum`s in one horizontally scrolling line (scroll snapping; on phones the cards are ~140 px wide).
- `<TarjetaEstacion station onplay ondelete? />`: a square card (the `radio` icon on an accent-tinted background), the name, `summary(station)`; a click plays; a ⋯ with "Eliminar" when `ondelete` (`confirm('¿Eliminar la estación «Nombre»?')`).
- **Inicio** (`/`): `seed` = a random integer per visit; `api.home(seed)`; reload on `app.generation` (same seed). Header: "Inicio" and a link "Tu escucha →" (`statsHref('mes')`, `chart` icon). Then:
  1. *Agregados recientemente* — `recent`, href `/explorar?orden=agregado`.
  2. *Tus estaciones* — a row of `TarjetaEstacion`: `LIBRARY`, `LIKED` (when `mine.marks` has any like), then the saved ones (delete → `api.deleteStation`, reload).
  3. *Seguir escuchando* — `continue`, href `/explorar?orden=escuchado`.
  4. *Tus me gusta* — `liked`, href `exploreHref({ megusta: 'si' })`, station `LIKED`.
  5. The rotating rows: `rowTitle(row)`, href `exploreHref(row.facets)`, station `station(rowTitle(row), { view: 'albumes', facets: row.facets })`; the first rotating row has the ↻ (a new seed, rotating rows only, keeping the scroll).
  Rows without albums are hidden. Empty catalog: the same messages as Explorar ("Escaneando…" / "Todavía no hay música…").

- [ ] **Step 1:** Implement.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser:** `/` shows Inicio with its rows; ↻ changes the rotating rows; "ver todas" opens Explorar filtered; ▶ on a row and on a station card plays a station; deleting a saved station; `/?pais=AR` opens Explorar; the nav marks Inicio; at 375 px the bottom tabs show five sections and the rows scroll sideways.
- [ ] **Step 4: Commit** `feat(web): Inicio with fixed and rotating rows; "/" is Inicio`

---

### Task 14: Web: Estadísticas

**Files:**
- Create: `web/src/pages/Estadisticas.svelte`
- Modify: `web/src/App.svelte` (route `estadisticas`), `web/src/components/Nav.svelte` (Inicio active on `estadisticas`)

**Page** (`/estadisticas?periodo=…`): title "Tu escucha"; tabs Última semana · Último mes · Último año · Siempre (`statsHref`); a line "N reproducciones · {duration(listenedMs)} · N pistas distintas"; sections, each hidden when empty:
- *Pistas* — `ListaPistas` with `showAlbum` (plays from the clicked one), each row's play count shown (add an optional `plays` display to `ListaPistas` only if it fits cleanly; else a simple ranked list).
- *Álbumes* — a grid of `TarjetaAlbum` with "N reproducciones" under each (or a compact ranked list with covers).
- *Artistas* — a ranked list: name (linked when `key`), plays.
- *Países*, *Géneros* — ranked lists linked to `exploreHref({ pais })` / `exploreHref({ genero })`, with `country()` for country names.
- *Por escuchar* — "N álbumes nunca escuchados →" (`exploreHref({ escucha: 'nunca' })`) and "N sin escuchar hace más de un año →" (`exploreHref({ escucha: 'hace-un-anio' })`).
No plays in the period: "Todavía no hay reproducciones en este período." Reload on `app.generation` and on the period.

- [ ] **Step 1:** Implement.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser** with a few recorded plays: every period, the links, 375 px.
- [ ] **Step 4: Commit** `feat(web): listening statistics`

---

### Task 15: Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/notes/2026-10-02-m4a-followups.md`

- [ ] **Step 1:** Update `README.md` (Spanish, like the rest of it): Inicio (its rows, ↻, "/" is Inicio and old `/?…` links open Explorar); stations (sources, modes, the rules and their defaults, how they relax, weights of likes and ratings, saving them, that they never end and the queue keeps only the last 100 played); the Escucha facet; statistics (periods, how plays are counted per track); the artist's "Más escuchadas"; the new endpoints: `/api/stations…`, `/api/station/next`, `/api/home`, `/api/stats`, `top` in `/api/artists/{key}`; `unmeasurable` in gains.
- [ ] **Step 2:** In the follow-ups note, mark the items done in M4b (Task 3, Task 4, `PlayTracks`) as done, and keep the rest.
- [ ] **Step 3: Commit** `docs: stations, Inicio and statistics in the README`

---

### Task 16: Check against the collection (manual, with the user)

Not for a subagent: it needs the user's judgment and their collection.

- [ ] Copy `.run`'s catalog (and `config.json`, `cache/`) to a separate folder at the same depth as the repo, e.g. `/Users/martin/x-projects/tordo-m4b-check/run`, so the relative roots still point at `~/Music`; never touch `~/Music` or `.run`. Run `go run ./cmd/tordo -dir <copy>` after `npm run build`.
- [ ] The v4 catalog opens and migrates to v5 without a rescan; counts as before.
- [ ] Time `/api/station/next` on the whole library and on a country (target well under 100 ms), `/api/home` and `/api/stats?periodo=siempre`.
- [ ] Play stations by ear: a country, a genre, an artist (artist gap relaxes), a collection in albums mode, Me gusta; check the variety (no artist back-to-back with gap 3), that liked and well-rated tracks come up more, and that a long session keeps going (let it refill a few times; the queue stays bounded).
- [ ] Inicio: the rotating rows make sense on the real collection (decades, countries, genres, collections; `nunca` with most of the albums); ↻.
- [ ] Statistics with the plays recorded during the check; "Más escuchadas" on an artist.
- [ ] Phone layout at 375 px: Inicio rows, the queue's station header, the settings dialog.
- [ ] Write the findings to `docs/superpowers/notes/2026-10-xx-m4b-collection-check.md` and fix or plan what comes out. Then merge into `main`.
