# Tordo M5 (Revisión y red) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Revisar's last two tabs (Duplicados with its three kinds and the MEJOR badge; Formatos a convertir, detection only), the album's versions, the first-run assistant and Ajustes (folders, Discogs token, normalization, LAN), configuration that applies without a restart, and listening from other devices on the LAN: fixed port, PIN, 90-day sessions, devices, QR code.

**Architecture:** Duplicates and formats to convert are pure catalog logic computed over the snapshot, which gains each track's file size and the releases' release groups. Configuration moves into a new `internal/engine` (adapted from Cinexplorer): it owns the parts that depend on `config.json` (scanner, identification runner) and replaces them when the settings change; the server reads them through `s.parts()`. The local listener (`127.0.0.1`) never changes; turning LAN on opens a **second** listener on `0.0.0.0:<port>` served by `LANHandler()`, which asks for a session cookie from every device but the computer itself. PINs are PBKDF2 hashes in `config.json`; sessions live in a new `sessions` table (schema v6). The QR code comes from a small encoder of our own (`internal/qr`), checked against reference matrices in `internal/qr/testdata/`.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`; `crypto/pbkdf2` is in the standard library), Svelte 5 + Vite 8 + Vitest 5 (as in M4).

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish): §6 Duplicados, §8 Revisar (Formatos a convertir), Álbum ("versiones/duplicados"), Explorar's `estado` facet ("con duplicados"), Ajustes / asistente inicial, §9 Red local y seguridad, §10 M5. Earlier plans (`2026-10-02-tordo-m4a-mis-datos.md`, `2026-10-02-tordo-m4b-estaciones.md`) set the conventions; their decisions still hold. Code to adapt from Cinexplorer (`/Users/martin/x-projects/cinexplorer`, copy and adapt, never import): `internal/engine/engine.go`, `internal/config/config.go` (`Root`, `CheckRoots`, `Available` and helpers), `internal/server/config.go`, `web/src/lib/settings.js`, `web/src/pages/Ajustes.svelte`, `web/src/pages/Bienvenida.svelte`, and their tests.

## Decisions this plan takes (not spelled out in the spec)

Agreed with the user before writing the plan: an own QR encoder (no new dependency); LAN switched live with two listeners; on first run nothing is scanned until the assistant saves; "the same identified album" means the same MusicBrainz release group.

1. **Exact copy** (`exacta`): two or more present tracks with the same non-empty audio fingerprint. Recoverable bytes of a group: its total minus its largest file (keeping one copy). Groups sorted by recoverable bytes, largest first.
2. **Same recording in several formats** (`formatos`): tracks of the **same album** grouped together when they share a non-empty MusicBrainz recording id, or when their normalized artist and title (`nameparse.Norm`; the track's artist, else its album's) are equal and their durations differ by 2 s or less (chained: sorted by duration, a gap over 2 s starts another group). "Same album" is the **album key**: `rg:<release group>` when the album is identified (it has a `ReleaseID`) and the release's group is known, `rel:<release id>` when the group is not known, else `al:<album id>`. Loose tracks (album 0) are never in this kind. A group needs at least two **distinct** fingerprints (an empty fingerprint counts as distinct): bit-identical files are kind 1.
3. **MEJOR**: quality order lossless > lossy; between lossless, bit depth, then sample rate; between lossy, bitrate. The best file gets `best: true` **only when it is strictly better than every other file of its group** (two equal FLACs: no badge). Exact groups have no badge. Files in a group: best first, then by quality, then by path.
4. **Suspect** (`sospecha`): the same artist + title + ±2 s rule over **all** present tracks, keeping only groups that span at least two album keys (loose tracks count as their own key, `track:<id>`) and hold at least two distinct fingerprints. Informational; sorted by artist, then title.
5. **Estado "con duplicados"** (`estado=con-duplicados`): a track in an exact or formats group, and the albums holding such a track. Suspects don't count.
6. **Album versions:** the album page lists the other albums that share at least one exact or formats group with it: title, artist, year, their formats, how many of this album's tracks they share, and the kind (`exacta` when every shared track is an exact copy, else `formatos`).
7. **Formats to convert:** present tracks whose `FormatKey` is `wma`, `wav`, `aiff`, `ape`, `wavpack` or `alac` (labels WMA, WAV, AIFF, APE, WavPack, ALAC). Totals (files, bytes), per format, and one item per album × format (loose tracks: per folder × format), largest first. Detection only: nothing is converted.
8. **First run:** no `config.json` → *setup pending*: nothing is written or scanned; the local browser is sent to `/bienvenida`, whose sibling folders of `tordo/` come checked. Saving the assistant writes `config.json` and scans everything. An existing `config.json` never shows the assistant. In browse-only mode there is no assistant (nothing can be saved). The assistant asks for folders, the optional Discogs token and normalization; it does **not** offer LAN (choosing a PIN before the music is even scanned is noise): it says LAN is turned on later in Ajustes, which has every setting of the spec's list.
9. **What a settings change does** (engine): only `normalize` or `lan` changed → the running parts stay; roots or ignore patterns changed → the scanner is rebuilt and the whole library is scanned again (incremental: unchanged files are not read); the Discogs token changed → the identification runner is rebuilt with the new client and triggered, no scan. Normalization reaches the player through `/api/status` (the page polls it, and Ajustes refreshes it after saving).
10. **LAN:** `config.json` holds `"lan": {"enabled": false, "port": 4747, "pinHash": ""}`. The local listener (`127.0.0.1`, `-port`) never changes. LAN on opens a second listener on `0.0.0.0:<port>`; off closes it; a new port reopens it. Opening fails (port taken) → the settings are not saved and the error is shown. Browse-only mode never opens it. Port: 1024–65535.
11. **Who is local:** every request on the local listener; on the LAN listener, a request from a loopback address whose `Host` is `localhost`, `127.0.0.1` or `[::1]`. The LAN listener accepts only `Host`s that are IP literals, `localhost`, the machine's host name or `<hostname>.local` (anything else is 403: DNS rebinding), and keeps the cross-site checks of `localOnly`.
12. **PIN:** 6 to 12 ASCII digits. Stored as `pbkdf2-sha256$<iterations>$<salt b64>$<key b64>` (100 000 iterations, 16-byte salt, 32-byte key, raw URL base64), compared in constant time. Enabling LAN needs a PIN (saved or given). A new PIN closes every session. **Attempts:** one limiter for the whole server (a single user): 5 wrong PINs are free, then each wrong PIN locks logins for 30 s, doubling with every further failure, at most 1 h; a right PIN resets it. While locked: 429 with `Retry-After`.
13. **Sessions:** a 32-byte random token in the cookie `tordo_sesion` (`HttpOnly`, `SameSite=Lax`, `Path=/`, `Max-Age` 90 days; no `Secure`: plain HTTP); the table keeps its SHA-256 (hex), the device name (from the page, `deviceName(navigator.userAgent)`, cut to 60 runes), creation and last use. 90 days **since last use**: a request whose session was last seen over a day ago updates `last_seen` and re-sends the cookie. Sessions older than 90 days are deleted at each login. Ajustes lists them, with "Cerrar sesión" per device; a device can close its own session.
14. **What needs a session** on the LAN listener (when not local): everything under `/api/` and `/stream/` except `GET /api/auth`, `POST /api/auth/login` and `POST /api/auth/logout` → 401 `ingresá el PIN`. The app shell (static files) is public: it holds no data. The page asks `GET /api/auth` at start and shows the PIN screen when needed; any later 401 shows it again (session closed from Ajustes).
15. **Settings endpoints are local only** (`/api/config…`, `/api/lan…`, `/api/sessions…`): 403 `Los ajustes solo se cambian en la computadora que corre Tordo.` From the phone, Ajustes shows that line and "Cerrar sesión en este dispositivo". Everything else (likes, ratings, playlists, corrections, stations, scan) works from any device with a session.
16. **LAN addresses** shown in Ajustes: `http://<ip>:<port>/` for each up, non-loopback, private IPv4 address, then `http://<hostname>.local:<port>/`. Each has a QR (`GET /api/lan/qr?url=…`, SVG), only for URLs in that list.
17. **M4b's collection check** (its Task 16) was never done; Task 18 here includes it.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new Go dependencies** (only `modernc.org/sqlite v1.59.0`). No cgo. Randomness: `crypto/rand` for salts and tokens.
- npm: no new packages (only `svelte`, `vite`, `vitest`, `@sveltejs/vite-plugin-svelte`, `@fontsource-variable/inter`).
- **Never write to the music roots.** The only writes go to the app folder: `tordo.db`, `config.json`, `cache/`. Browse-only mode writes nothing: write endpoints answer 409, `PUT /api/config` too, and the LAN listener is never opened.
- Catalog paths are relative to the app folder, `/`-separated. Audio and pictures are asked for **by id**, never by path.
- Write endpoints (POST/PUT) are wrapped in `jsonOnly`.
- SQLite `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open; inside a transaction read only through it.
- Schema: v6 only **adds** the `sessions` table (`schemaVersion = 6`, `derivedVersion` stays 2). Session writes are side writes that are **not** personal data: they move neither the catalog's change counter nor `PersonalVersion`.
- `internal/catalog` stays pure (no SQL, no network).
- User-facing strings (UI, logs, errors) in **Spanish** (rioplatense: *elegí*, *ingresá*, *probá*); identifiers and comments in English. URL names in Spanish (`/ajustes`, `/bienvenida`, `/revisar/duplicados`, `/revisar/formatos`, `tipo`, `exactas`, `formatos`, `sospechas`); JSON fields in English.
- UI files named in Spanish (`Ajustes.svelte`, `Bienvenida.svelte`, `Entrar.svelte`, `Duplicados.svelte`, `FormatosAConvertir.svelte`).
- The web build (`internal/server/dist/`) is **not committed**. `go test ./...` passes without Node and **without network**; `go vet ./...` and `gofmt -l .` clean; `cd web && npx vitest run` passes; `cd web && npm run build` succeeds.
- Commit messages: conventional style (`feat:`, `fix:`, `test:`, `docs:`), **no `Co-Authored-By:` trailer**.
- Git in the worktree: plain single commands (no pipes, no `&&`: the worktree guard refuses them).

## Model per task

Per the user's rule (cheapest model that can do the job): **Haiku** for transcribing fully specified code and docs, **Sonnet** for implementing from prose and multi-file work, **Opus** only for the final whole-branch review.

| Task | What | Model |
|---|---|---|
| 1 | Store: file sizes and release groups in the snapshot | Haiku |
| 2 | Catalog: duplicates, `con-duplicados`, album versions | Sonnet |
| 3 | Catalog: formats to convert | Haiku |
| 4 | Server: Revisar endpoints, versions on the album page | Sonnet |
| 5 | QR encoder | Sonnet |
| 6 | Auth: PIN, attempts, tokens | Sonnet |
| 7 | Store: schema v6, sessions | Sonnet |
| 8 | Config: LAN, roots checks, setup pending | Sonnet |
| 9 | Engine | Sonnet |
| 10 | Server: engine wiring and settings endpoints | Sonnet |
| 11 | Server: LAN listener, PIN login, sessions, QR | Sonnet |
| 12 | `cmd/tordo`: the engine, first run, LAN at start | Sonnet |
| 13 | Web libraries | Sonnet |
| 14 | Web: Duplicados, Formatos a convertir, album versions | Sonnet |
| 15 | Web: Ajustes, Bienvenida, ⚙ | Sonnet |
| 16 | Web: Red local, PIN screen, devices | Sonnet |
| 17 | Documentation | Haiku |
| — | Final review of the branch | Opus |
| 18 | Check against the collection and from the phone | manual (with the user) |

## File Structure

```
internal/store/catalog.go, snapshot.go    TrackView.SizeBytes; Snapshot.ReleaseGroups
internal/store/schema.sql, store.go       v6: sessions
internal/store/sessions.go                NEW: sessions CRUD
internal/catalog/duplicates.go            NEW: Duplicates, album keys, MEJOR, AlbumVersions
internal/catalog/convert.go               NEW: ToConvert
internal/catalog/catalog.go, facets.go    duplicates computed in Build; StateDuplicated
internal/qr/qr.go, rs.go, svg.go          NEW: QR encoder (byte mode, level M, versions 1–6), SVG
internal/qr/testdata/*.txt                reference matrices (already committed with the plan)
internal/auth/auth.go                     NEW: PIN format and hash, Limiter, tokens
internal/config/config.go, roots.go       LAN; Root, CheckRoots, Available (from Cinexplorer)
internal/engine/engine.go                 NEW: runtime built from config.json, replaced on change
internal/server/server.go                 parts(), routes shared by both handlers, local flag in the context
internal/server/review.go                 NEW: /api/review/duplicates, /api/review/formats
internal/server/settings.go               NEW: /api/config…
internal/server/lan.go                    NEW: LAN listener, LANHandler, /api/auth…, /api/sessions…, /api/lan…
internal/server/catalog.go                album page: versions
cmd/tordo/main.go                         engine, first run, LAN
web/src/lib/                              settings.js (NEW), dupes.js (NEW), api.js, router.js, facets.js, auth.svelte.js (NEW)
web/src/components/                       Duplicados, FormatosAConvertir, RedLocal, Dispositivos (NEW); Nav (⚙)
web/src/pages/                            Ajustes, Bienvenida, Entrar (NEW); Revisar, Album
web/src/App.svelte                        routes, setup redirect, PIN screen
README.md                                 Revisar, Ajustes, LAN, endpoints
```

## How to read the tasks

The plan gives the design, the exact interfaces, the test cases and the code where it is tricky; the rest is yours to write, matching the surrounding code (comment density, naming, idiom). Tests come first in each task: write them, see them fail, then write the code. Go tests use the existing helpers (`store`: `open`, `sample`, `library`, `regroup`, `trackNamed`; `server`: `newStore`, `mediaServer`, `do`, `decode`, `itoa`; `catalog`: `fixture`, `build`) — read them before writing new ones. Svelte components have no tests: their logic lives in tested `web/src/lib/` modules, and each web task ends with `npx vitest run` and `npm run build`.

After every task: `gofmt -l .` (empty), `go vet ./...`, `go test ./...`, and for web tasks `cd web && npx vitest run && npm run build`. Then commit.

---

### Task 1: Store: file sizes and release groups in the snapshot

**Files:**
- Modify: `internal/store/catalog.go` (`TrackView`, `trackQuery`, `trackDest`), `internal/store/snapshot.go`
- Test: `internal/store/catalog_test.go` (or the snapshot's existing test file)

**Interfaces (produces):**

```go
// in TrackView, after Fingerprint:
	SizeBytes   int64  `json:"sizeBytes"` // the file's size

// in Snapshot, after Artists:
	// ReleaseGroups maps the MusicBrainz releases the catalog holds to
	// their release groups (only releases whose group is known); never nil.
	ReleaseGroups map[string]string
```

`trackQuery` selects `f.size` and `trackDest` scans it into `t.SizeBytes` (every reader of `trackQuery` gets it: album pages, folders, Explorar). `Snapshot()` reads `SELECT id, json_extract(data, '$.groupId') FROM releases` inside its transaction, skipping NULL or empty groups.

- [ ] **Step 1: Write the failing tests:** on `sample(t)` (or the helper that saves files with known sizes), `Snapshot().Tracks[i].SizeBytes` equals the size given to `SyncFiles`, and `Album(id)`'s tracks carry it too. Save two releases with `SaveRelease` (`{ID: "r1", GroupID: "g1"}`, `{ID: "r2"}`): `Snapshot().ReleaseGroups` is `{"r1": "g1"}`; with no releases it is an empty, non-nil map.
- [ ] **Step 2: Run** `go test ./internal/store -run 'Snapshot|Size|Group'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...`: pass (JSON outputs gain `sizeBytes`; fix any test that compares whole track JSON).
- [ ] **Step 5: Commit** `feat: file sizes and release groups in the catalog snapshot`

---

### Task 2: Catalog: duplicates, `con-duplicados`, album versions

**Files:**
- Create: `internal/catalog/duplicates.go`
- Modify: `internal/catalog/catalog.go` (`Catalog` keeps the computed duplicates; `Build` computes them and adds the facet value), `internal/catalog/facets.go` (`StateDuplicated`)
- Test: `internal/catalog/duplicates_test.go`

**Consumes:** `store.TrackView.SizeBytes`, `store.Snapshot.ReleaseGroups` (Task 1); `store.AlbumView.ReleaseID`; `nameparse.Norm`; `FormatKey`.

**Interfaces (produces):**

```go
// Kinds of duplicate groups, as Revisar names them.
const (
	DupExact   = "exacta"   // the same audio (fingerprint) in several files
	DupFormats = "formatos" // the same recording in one album, in several files
	DupSuspect = "sospecha" // the same artist, title and length in different albums
)

// in facets.go, with the other FacetState values:
	StateDuplicated = "con-duplicados"

// DupFile is one file of a duplicate group.
type DupFile struct {
	ID         int64  `json:"id"` // the file id
	Path       string `json:"path"`
	Dir        string `json:"dir"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	AlbumID    int64  `json:"albumId"` // 0: a loose track
	Album      string `json:"album"`
	Format     string `json:"format"` // FormatKey
	Lossless   bool   `json:"lossless"`
	Bitrate    int    `json:"bitrate"`
	SampleRate int    `json:"sampleRate"`
	BitDepth   int    `json:"bitDepth"`
	DurationMs int64  `json:"durationMs"`
	SizeBytes  int64  `json:"sizeBytes"`
	Best       bool   `json:"best"` // MEJOR: strictly better than every other file of its group
}

// DupGroup is a set of files that hold, or may hold, the same music.
type DupGroup struct {
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`  // of its first file
	Artist      string    `json:"artist"` // of its first file
	Files       []DupFile `json:"files"`
	SizeBytes   int64     `json:"sizeBytes"`   // all its files
	Recoverable int64     `json:"recoverable"` // exact copies: the bytes freed keeping one; else 0
}

// Duplicates are the three kinds of groups of the catalog.
type Duplicates struct {
	Exact       []DupGroup
	Formats     []DupGroup
	Suspect     []DupGroup
	Recoverable int64 // the exact copies' recoverable bytes
}

// Version is another album that holds some of an album's tracks.
type Version struct {
	AlbumID int64    `json:"albumId"`
	Title   string   `json:"title"`
	Artist  string   `json:"artist"`
	Year    int      `json:"year"`
	Formats []string `json:"formats"` // FormatKey of its shared tracks, sorted, distinct
	Shared  int      `json:"shared"`  // this album's tracks with a counterpart there
	Kind    string   `json:"kind"`    // DupExact when every shared track is an exact copy, else DupFormats
}

func (c *Catalog) Duplicates() Duplicates          // computed once by Build; slices never nil; callers must not modify it
func (c *Catalog) AlbumVersions(id int64) []Version // never nil; by shared descending, then title
```

**Rules:** decisions 1–6 at the top of this plan, exactly. Present tracks only (the snapshot has only present tracks). Details:
- The album key needs the snapshot's albums and `ReleaseGroups`: compute duplicates inside `Build` (it has the snapshot) and keep the result on the `Catalog` (unexported field). Cost must stay near linear: bucket by fingerprint, by album key, by `artist\x00title`; sort each bucket by duration; union-find (or simple merging) for the recording id rule. The catalog has ~7 000 tracks today and is meant for ~150 000.
- Quality: write `better(a, b DupFile) int` (positive when a is better) used for sorting and for `Best`.
- After computing, add `StateDuplicated` to `values[FacetState]` of every track in an exact or formats group and of the albums of those tracks, so Explorar's `estado` facet, its counts and its filter work with no other change. Add its label where the other `estado` labels live on the server side, if any.
- `AlbumVersions` reads only exact and formats groups: for each group with a file in album `id`, every other album in that group is a version; `Shared` counts the distinct tracks of album `id` in groups shared with that album; files of album `id` itself and loose tracks are skipped.

- [ ] **Step 1: Write the failing tests** (`duplicates_test.go`, on snapshots of your own built with `tr(…)` and `store.AlbumView`s; set `Fingerprint`, `RecordingID`, `DurationMs`, `SizeBytes`, `Bitrate`, `BitDepth`, `SampleRate`, `AlbumID`):
  - `TestDuplicatesExact`: three files with fingerprint `fp1` (sizes 10, 12, 11) in three albums and a lone `fp2` → one exact group with the three files, `Recoverable` 21, `SizeBytes` 33, no `Best`; `Duplicates().Recoverable` 21; with a second exact group, groups are sorted by recoverable descending.
  - `TestDuplicatesFormats`: album A (identified: release `r1`, group `g1`) holds "Zamba de mi esperanza" as MP3 320 kbps (`fp-a`); album B (identified: release `r2`, same group `g1`) holds it as FLAC 16/44.1 (`fp-b`), 1.5 s longer → one formats group, FLAC first with `Best`. A WMA and a FLAC of the same title in one unidentified album → a formats group (key `al:`). Same title, durations 10 s apart, same album → no group. Same recording id, different titles, same album → grouped. Two FLAC 16/44.1 of the same recording (different fingerprints) → grouped, no `Best`. Two files with the same fingerprint in one album → exact group only, not formats. A release with no known group: albums with the same release id still share a key.
  - `TestDuplicatesSuspect`: the same artist/title/duration (±1 s) in two different unidentified albums with different fingerprints → one suspect group; the same in one album → not suspect (it is formats); a loose track + an album track → suspect; accents and case differ (`Canción`/`CANCION`) → still grouped; durations 3 s apart → not grouped; two exact copies in two albums and nothing else → not suspect.
  - `TestDuplicatedFacet`: tracks of exact and formats groups and their albums have `estado=con-duplicados` (`Explore` with that facet returns exactly them; the facet's count is right); a suspect-only track does not.
  - `TestAlbumVersions`: album A shares two tracks with B (one exact, one formats) and one track with C (exact) → `AlbumVersions(A)` is `[B (Shared 2, Kind formatos, Formats sorted), C (Shared 1, Kind exacta)]`; an album with no duplicates → empty, non-nil.
- [ ] **Step 2: Run** `go test ./internal/catalog -run 'Duplicat|Version'`: fails to compile.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/catalog`: pass.
- [ ] **Step 5: Commit** `feat: duplicates in the catalog: exact copies, formats with MEJOR, suspects; album versions`

---

### Task 3: Catalog: formats to convert

**Files:**
- Create: `internal/catalog/convert.go`
- Test: `internal/catalog/convert_test.go`

**Interfaces (produces):**

```go
// ConvertKeys are the formats Revisar suggests converting, in the order
// it lists them; ConvertLabels names them.
var ConvertKeys = []string{"wma", "wav", "aiff", "ape", "wavpack", "alac"}
var ConvertLabels = map[string]string{"wma": "WMA", "wav": "WAV", "aiff": "AIFF", "ape": "APE", "wavpack": "WavPack", "alac": "ALAC"}

type ConvertFormat struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

// ConvertItem is one album's (or, for loose tracks, one folder's) files
// in one of those formats.
type ConvertItem struct {
	AlbumID    int64  `json:"albumId"` // 0: loose tracks of Dir
	Title      string `json:"title"`   // the album's, or the folder's name
	Artist     string `json:"artist"`
	Dir        string `json:"dir"`
	Format     string `json:"format"` // a key of ConvertKeys
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
	DurationMs int64  `json:"durationMs"`
}

type ToConvert struct {
	Files   int             `json:"files"`
	Bytes   int64           `json:"bytes"`
	Formats []ConvertFormat `json:"formats"` // only those with files, in ConvertKeys order
	Items   []ConvertItem   `json:"items"`   // by bytes descending, then title
}

func (c *Catalog) ToConvert() ToConvert // Formats and Items never nil
```

A track's format is `FormatKey(t.Format, t.Codec)`. Loose tracks group by `Dir` (title: the folder's last path element).

- [ ] **Step 1: Write the failing test** `TestToConvert` on a snapshot of your own: two WMA tracks of one album (sizes 5 and 7), one ALAC (`Format: "mp4", Codec: "alac"`) of another album, one WAV (`Format: "wav", Codec: "pcm_s16le"`) loose in `../musica/sueltos`, an MP3 and a FLAC → `Files` 4, `Bytes` the sum of the four; `Formats` = WMA, WAV, ALAC in `ConvertKeys` order with their counts and bytes; `Items[0]` is the WMA album (2 files, 12 bytes); the WAV item has `AlbumID` 0 and `Title` `sueltos`. A catalog with none → zero totals, empty non-nil slices.
- [ ] **Step 2: Run** `go test ./internal/catalog -run ToConvert`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/catalog`: pass.
- [ ] **Step 5: Commit** `feat: formats to convert in the catalog`

---

### Task 4: Server: Revisar endpoints, versions on the album page

**Files:**
- Create: `internal/server/review.go`, `internal/server/review_test.go`
- Modify: `internal/server/server.go` (routes), `internal/server/catalog.go` (`albumPage` gains `Versions`)
- Test: `internal/server/catalog_test.go`

**Consumes:** `Catalog.Duplicates`, `Catalog.AlbumVersions`, `Catalog.ToConvert` (Tasks 2, 3).

**Endpoints:**
- `GET /api/review/duplicates?tipo=exactas|formatos|sospechas` (default and unknown values: `exactas`) →
  ```json
  {"counts": {"exact": 3, "formats": 1, "suspect": 12}, "recoverable": 123456, "groups": [DupGroup…]}
  ```
  `groups` is the list of the asked kind, never null.
- `GET /api/review/formats` → `catalog.ToConvert` as JSON.
- `GET /api/albums/{id}` gains `"versions": [Version…]` (never null; `[]` when the album is not in the catalog).

Both read `s.catalog()` (rebuilt when the store changes), as Explorar does.

- [ ] **Step 1: Write the failing tests:** build a store with two copies of one file (same fingerprint, two folders) and a WMA track (`mediaServerFiles` in `media_test.go` takes a map of `audiofile.Info`; read how the fingerprints get set there and in `newStore`). Assert: `/api/review/duplicates` → one exact group with two files, `counts.exact` 1, `recoverable` > 0; `?tipo=sospechas` → `groups` `[]`; `?tipo=raro` → the exact groups; `/api/review/formats` → the WMA counted; `/api/albums/{id}` of one copy's album → `versions` lists the other album; an album without duplicates → `versions: []`.
- [ ] **Step 2: Run** `go test ./internal/server -run 'Review|Album'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...`: pass.
- [ ] **Step 5: Commit** `feat: Revisar endpoints for duplicates and formats to convert; versions on the album page`

---

### Task 5: QR encoder

**Files:**
- Create: `internal/qr/qr.go` (encoding, placement, masks, penalty), `internal/qr/rs.go` (GF(256) and Reed-Solomon), `internal/qr/svg.go`
- Test: `internal/qr/qr_test.go`, `internal/qr/rs_test.go`; fixtures `internal/qr/testdata/v1.txt … v6.txt` (already in the repo)

Implement ISO/IEC 18004 (QR Code model 2) for **byte mode, error correction level M, versions 1 to 6** only (enough for any LAN URL: up to 106 bytes). Nothing else (no numeric/alphanumeric/Kanji modes, no other levels, no version information blocks — they start at version 7).

**Interfaces (produces):**

```go
package qr // QR codes for short texts (the LAN addresses): byte mode, level M, versions 1–6.

var ErrTooLong = errors.New("texto demasiado largo para el código QR")

// Code is a QR symbol without its quiet zone.
type Code struct {
	Size    int // modules per side: 17 + 4*Version
	Version int
	Mask    int
	dark    []bool // row-major
}

func (c *Code) Dark(x, y int) bool // x column, y row

// Encode makes the symbol of text: the smallest version that fits and the
// mask with the lowest penalty (ties: the lowest mask number).
func Encode(text string) (*Code, error)

// encode makes the symbol with a given mask (tests compare it with the
// reference matrices).
func encode(text string, mask int) (*Code, error)

// SVG draws the code with a quiet zone of quiet modules: a white square
// and one black path, viewBox in modules, shape-rendering="crispEdges".
func (c *Code) SVG(quiet int) string
```

**Tables (level M):**

| Version | Size | Data codewords | Blocks × (total, data) | EC per block | Byte capacity | Alignment centers | Remainder bits |
|---|---|---|---|---|---|---|---|
| 1 | 21 | 16 | 1 × (26, 16) | 10 | 14 | — | 0 |
| 2 | 25 | 28 | 1 × (44, 28) | 16 | 26 | 6, 18 | 7 |
| 3 | 29 | 44 | 1 × (70, 44) | 26 | 42 | 6, 22 | 7 |
| 4 | 33 | 64 | 2 × (50, 32) | 18 | 62 | 6, 26 | 7 |
| 5 | 37 | 86 | 2 × (67, 43) | 24 | 84 | 6, 30 | 7 |
| 6 | 41 | 108 | 4 × (43, 27) | 16 | 106 | 6, 34 | 7 |

**Steps of the encoding** (each a small function, tested where noted):
1. Bits: mode `0100`, the byte count in 8 bits, the bytes; terminator up to 4 zero bits; zero-pad to a byte; pad bytes `0xEC`, `0x11` alternating up to the data codewords.
2. Split the data codewords into the version's blocks; Reed-Solomon over GF(256) with the primitive polynomial `0x11D` and generator `∏(x − α^i), i = 0…n−1`, n = EC per block.
3. Interleave: data codewords column by column across blocks, then EC codewords the same way; then the remainder bits (zeros).
4. Function patterns: three finder patterns with separators, timing patterns on row and column 6, alignment patterns at every pair of centers of the table except where they overlap a finder, the dark module at (8, 4·V + 9) (column 8, row `Size − 8`), and reserved format areas.
5. Data placement: the standard zig-zag from the bottom-right corner, two columns at a time, upward and downward in turn, skipping column 6 and the function modules.
6. Masks (row `i`, column `j`): 0 `(i+j)%2==0`; 1 `i%2==0`; 2 `j%3==0`; 3 `(i+j)%3==0`; 4 `(i/2+j/3)%2==0`; 5 `(i*j)%2+(i*j)%3==0`; 6 `((i*j)%2+(i*j)%3)%2==0`; 7 `((i+j)%2+(i*j)%3)%2==0`. Applied to data modules only.
7. Format information: 5 bits = level M `00` + the mask's 3 bits; BCH(15,5) with generator `10100110111`; XOR `101010000010010`; written twice in the standard positions (bit 14 is the most significant).
8. Penalty (to choose the mask): the four rules of ISO 18004 §7.8.3 — runs of 5+ same-color modules in rows and columns (3 + run − 5), 2×2 blocks of one color (3 each), the patterns `1011101` with four light modules on either side in rows and columns (40 each), and the dark proportion (10 per 5 % away from 50 %).

- [ ] **Step 1: Write the failing tests:**
  - `rs_test.go`: the error correction codewords of the 1-M "HELLO WORLD" example (data `32 91 11 120 209 114 220 77 67 64 236 17 236 17 236 17`, 10 EC codewords) are `196 35 39 119 235 215 231 226 93 23`. GF(256): `exp[log[x]] == x` for 1–255, and α^8 == 0x1D.
  - `qr_test.go`: a helper reads a fixture (lines starting with `#` are the header: `text:`, `version:`, `mask:`; then one row per line, `#` dark and `.` light). For each fixture, `encode(text, mask)` has that version and size and matches every module (report the first differing module on failure). The format bits for M with mask 0 are `101010000010010`. `Encode` on each fixture's text picks the fixture's version and mask (the fixtures come from the `qrcode` Python package, which follows the spec's penalty rules; if a fixture's mask disagrees, recheck the penalty rules against the spec before changing the test, and say so in the report). `Encode` of 106 bytes works (version 6); 107 bytes → `ErrTooLong`. `SVG(4)` of the version 1 fixture starts with `<svg`, has `viewBox="0 0 29 29"` and one `<path`.
- [ ] **Step 2: Run** `go test ./internal/qr`: fails to compile.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/qr`: pass.
- [ ] **Step 5: Commit** `feat: QR encoder for the LAN addresses (byte mode, level M, versions 1-6)`

---

### Task 6: Auth: PIN, attempts, tokens

**Files:**
- Create: `internal/auth/auth.go`
- Test: `internal/auth/auth_test.go`

**Interfaces (produces):**

```go
// Package auth checks the PIN that devices on the LAN need, limits wrong
// attempts and makes session tokens. The sessions themselves live in the
// store; the PIN's hash lives in config.json.
package auth

var ErrPINFormat = errors.New("el PIN tiene que tener entre 6 y 12 dígitos, solo números")

func CheckFormat(pin string) error // 6–12 ASCII digits

// HashPIN derives a key from the PIN with PBKDF2-SHA256: the result is
// "pbkdf2-sha256$<iterations>$<salt>$<key>" (base64.RawURLEncoding).
func HashPIN(pin string) (string, error) // CheckFormat first; 16-byte salt from crypto/rand; 32-byte key

// VerifyPIN tells whether pin matches hash; false for a malformed hash.
func VerifyPIN(hash, pin string) bool // subtle.ConstantTimeCompare

const Iterations = 100_000

// Limiter slows down guessing: the first FreeAttempts wrong PINs are free,
// then each wrong PIN locks logins for a time that starts at FirstLock and
// doubles with every further failure, up to MaxLock. A right PIN resets it.
// One limiter serves every device: there is a single user.
type Limiter struct {
	Now func() time.Time // nil: time.Now
	mu       sync.Mutex
	failures int
	until    time.Time
}

const (
	FreeAttempts = 5
	FirstLock    = 30 * time.Second
	MaxLock      = time.Hour
)

func (l *Limiter) Wait() time.Duration // > 0 while locked
func (l *Limiter) Fail()
func (l *Limiter) Reset()

// NewToken makes a session token (32 random bytes, base64.RawURLEncoding)
// and the hash the store keeps of it.
func NewToken() (token, hash string, err error)
func TokenHash(token string) string // hex SHA-256
```

`crypto/pbkdf2.Key(sha256.New, pin, salt, Iterations, 32)` (Go 1.24+ signature: returns `([]byte, error)`). The iteration count travels in the hash, so `VerifyPIN` reads it from there (reject counts < 1 or > 10 000 000).

- [ ] **Step 1: Write the failing tests:**
  - `TestCheckFormat`: `123456`, `000000`, `123456789012` ok; `12345`, `1234567890123`, `12345a`, `１２３４５６` (full-width digits), `" 123456"` → `ErrPINFormat`.
  - `TestHashAndVerify`: `HashPIN("482915")` → 4 `$`-separated parts with `pbkdf2-sha256` and `100000`; two hashes of the same PIN differ (salt); `VerifyPIN` true for the PIN, false for `482916`; false for `""`, `"pbkdf2-sha256$x$y$z"`, a hash with 0 iterations; `HashPIN("12")` → `ErrPINFormat`.
  - `TestLimiter` with a fake clock: 5 `Fail` → `Wait` 0; the 6th → 30 s; advance 30 s → 0; the 7th → 60 s; then 120 s…; never over 1 h (fail 20 times); `Reset` → 0 and the next 5 failures are free again.
  - `TestTokens`: tokens are 43 characters, two tokens differ, `TokenHash(token) == hash`, the hash is 64 hex characters.
- [ ] **Step 2: Run** `go test ./internal/auth`: fails to compile.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/auth`: pass.
- [ ] **Step 5: Commit** `feat: PIN hashing, attempt limiter and session tokens`

---

### Task 7: Store: schema v6, sessions

**Files:**
- Modify: `internal/store/schema.sql`, `internal/store/store.go` (`schemaVersion = 6`; its comment gains "Version 6 (M5) added the LAN sessions.")
- Create: `internal/store/sessions.go`
- Test: `internal/store/sessions_test.go`, `internal/store/migrate_test.go`

Append to `schema.sql`:

```sql
-- Devices signed in from the LAN. token_hash is the SHA-256 (hex) of the
-- cookie's token; the token itself is never stored.
CREATE TABLE IF NOT EXISTS sessions (
	id         INTEGER PRIMARY KEY,
	token_hash TEXT NOT NULL UNIQUE,
	device     TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	last_seen  INTEGER NOT NULL
);
```

**Interfaces (produces):**

```go
// Session is a device signed in from the LAN.
type Session struct {
	ID        int64  `json:"id"`
	Device    string `json:"device"`
	CreatedAt int64  `json:"createdAt"` // unix ms
	LastSeen  int64  `json:"lastSeen"`  // unix ms
}

func (s *Store) CreateSession(tokenHash, device string) (int64, error) // device trimmed, cut to 60 runes, "" → "Dispositivo"
func (s *Store) SessionByToken(tokenHash string) (Session, bool, error)
func (s *Store) TouchSession(id int64) error                // last_seen = now()
func (s *Store) Sessions() ([]Session, error)               // last seen first, then id descending; never nil
func (s *Store) DeleteSession(id int64) (bool, error)
func (s *Store) DeleteSessions() error                      // every session (a new PIN)
func (s *Store) PruneSessions(before time.Time) (int, error) // deletes those last seen before; returns how many
```

Writes are `sideWrite(false, …)` (not catalog changes, not personal data) and use the store's `now()` (as stations and playlists do). In a read-only store they fail like other writes.

- [ ] **Step 1: Write the failing tests** (`open(t)`, overriding `now` where times matter):
  - `TestSessionLifecycle`: create two sessions → `SessionByToken` finds each with its device and `CreatedAt == LastSeen`; an unknown hash → `false, nil`; `TouchSession` moves `LastSeen` (and `Sessions()` order); `DeleteSession` → gone, a second delete → `false, nil`; `DeleteSessions` → empty list (non-nil); creating with an existing hash → error.
  - `TestSessionDevice`: `"  "` → `Dispositivo`; 80 runes → 60.
  - `TestPruneSessions`: sessions last seen at day 1 and day 100; `PruneSessions(day 10)` → 1, the day-100 one stays.
  - `TestSessionWritesAreSide`: on `sample(t)`, `Changes()` and `PersonalVersion()` unchanged across create/touch/delete.
  - `migrate_test.go`: the older catalog now migrates to `user_version` 6 with the `sessions` table.
- [ ] **Step 2: Run** `go test ./internal/store -run 'Session|Migrat'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/store`: pass.
- [ ] **Step 5: Commit** `feat: schema v6 with LAN sessions`

---

### Task 8: Config: LAN, root checks, setup pending

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/roots.go` (adapted from Cinexplorer's `config.go`: `Root`, `canonicalCase`, `canonicalName`, `onlyParents`, `CheckRoots`, `Available`)
- Test: `internal/config/config_test.go`, `internal/config/roots_test.go` (adapt Cinexplorer's tests of those functions)

**Interfaces (produces):**

```go
type LAN struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`    // 0 (absent): DefaultPort
	PinHash string `json:"pinHash"` // auth.HashPIN; "" when no PIN was chosen
}

const DefaultPort = 4747

// in Config, after Normalize:
	LAN LAN `json:"lan"`

func (l LAN) PortOrDefault() int

// Root turns a folder the user typed (absolute, or relative to the app
// folder) into a root in catalog form, or says why it cannot be one.
func Root(appDir, input string) (string, error)
// CheckRoots validates the roots of a settings change: each one must be in
// catalog form; new ones must exist (saved ones may be unavailable, as when
// a drive is unplugged); no duplicates; at least one.
func CheckRoots(appDir string, roots, saved []string) error
// Available tells whether a root's folder exists now.
func Available(appDir, root string) bool
```

`Load` keeps its signature `(Config, bool, error)`; `created` now reads as **setup pending**: the file does not exist, and the defaults are the sibling folders as roots, no ignore patterns, normalization on and `LAN{Port: DefaultPort}`. `Load` never writes (it never did; the caller saved). Copy Cinexplorer's root functions with their behavior and Spanish messages, adapting names; read Cinexplorer's tests and port the ones that apply.

- [ ] **Step 1: Write the failing tests:** a missing file → `created` true, `LAN.Port` 4747, `LAN.Enabled` false, `Normalizing()` true; a file without `lan` → `PortOrDefault()` 4747; a saved and reloaded config keeps `lan` (`enabled`, `port`, `pinHash`); the ported root tests (relative and absolute input, a file is not a folder, a missing folder, `CheckRoots` with an unavailable saved root, duplicates, none).
- [ ] **Step 2: Run** `go test ./internal/config`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...`: pass.
- [ ] **Step 5: Commit** `feat: LAN settings in config.json; root checks for the settings`

---

### Task 9: Engine

**Files:**
- Create: `internal/engine/engine.go` (adapted from Cinexplorer's `internal/engine/engine.go`)
- Test: `internal/engine/engine_test.go` (adapt Cinexplorer's where they apply)

**Consumes:** `config.Config`, `config.LAN`, `config.Save` (Task 8); `scan.Scanner`; `identify.Runner`, `identify.API`, `identify.Discogs`; `discogs.New`.

**Interfaces (produces):**

```go
// Package engine builds, from config.json, the parts of the app that depend
// on it (the scanner and the identification runner) and replaces them when
// the settings change. The LAN listener is opened and closed through a hook.
package engine

var ErrReadOnly = errors.New("modo consulta: la configuración no se puede modificar")

type ValidationError struct{ Err error } // Error, Unwrap as in Cinexplorer

type Engine struct {
	AppDir   string
	Store    *store.Store
	ReadOnly bool
	MB       identify.API                         // MusicBrainz, shared by every runtime
	Discogs  func(token string) identify.Discogs  // nil: discogs.New
	Loudness interface{ Trigger() }               // nil: none; triggered after every scan
	LAN      func(config.LAN) error               // opens, moves or closes the LAN listener; nil: none

	mu      sync.Mutex
	current atomic.Pointer[Runtime]
	pending atomic.Bool
}

type Runtime struct {
	Config     config.Config
	Scanner    *scan.Scanner    // nil in browse-only mode
	Identifier *identify.Runner // nil in browse-only mode
	*lifecycle
}

func (e *Engine) Start(cfg config.Config, setupPending bool) // builds; opens the LAN listener when enabled and not read-only (an error is logged, not returned); scans unless pending
func (e *Engine) Current() *Runtime
func (e *Engine) Use(rt *Runtime)          // tests: install a runtime as is
func (e *Engine) SetupPending() bool
func (e *Engine) Apply(cfg config.Config) error
func (e *Engine) Update(f func(cur config.Config) (config.Config, error)) error
func (e *Engine) Close()                   // stops the current runtime
func (rt *Runtime) Scan()                  // a scan of every root in the background; nothing in browse-only mode, after Stop or while one runs
func (rt *Runtime) Stop()                  // cancels its scan and identification, waits for them
```

**Behavior of `Update`** (decision 9; differs from Cinexplorer):
1. `ErrReadOnly` in browse-only mode. Under `e.mu`: `cfg, err := f(cur)`; a `*ValidationError` is returned as is.
2. When `cfg.LAN.Enabled` or `cfg.LAN.PortOrDefault()` differs from the current config's, and `e.LAN != nil`: call `e.LAN(cfg.LAN)` **before saving**; its error comes back as a `*ValidationError` (`no se pudo abrir el puerto 4747: …`) and nothing is saved.
3. `config.Save`. If it fails after step 2 changed the listener, call `e.LAN(old.LAN)` to undo it and return the save error.
4. Then: setup was pending → stop the old runtime, build, clear pending, scan. Only `Normalize` and `LAN` differ → keep the running parts (copy the runtime, swap its `Config`). `Roots` or `Ignore` differ → stop, build, scan (full incremental scan; tordo's scanner has no per-root runs and an incremental rescan is cheap). Otherwise (the Discogs token) → stop, build, `Identifier.Trigger()`.

`build(cfg)`: in browse-only mode only `Config`. Else an `identify.Runner{AppDir, Store, MB: e.MB, Discogs: e.Discogs(cfg.DiscogsToken)}` and a `scan.Scanner{AppDir, Roots: path.Clean of each root, Ignore: cfg.Ignore, Store, OnDone: <runner.Trigger, then Loudness.Trigger when set>}`. Log `sin token de Discogs: …` as `cmd/tordo` does today when the token is empty.

`Runtime.Scan` runs `Scanner.Run(ctx)` in a goroutine with the runtime's cancellable context; `scan.ErrBusy` and errors after cancellation are not logged.

- [ ] **Step 1: Write the failing tests** (temp app dir, an in-memory store, a fake `identify.API` that fails every call or finds nothing — no network; `Discogs` returning a fake that records its token):
  - `TestStartPending`: `Start(cfg, true)` → `SetupPending()` true, no scan ran (`Scanner.Status().Finished` zero after a short wait is fragile: instead give the scanner a root with one audio file and assert the store has no files), no `config.json`.
  - `TestApplyFirstUse`: after `Start(cfg, true)`, `Apply(cfg)` writes `config.json`, clears pending and scans (the store gets the file; wait with a deadline loop on `Scanner.Status()`).
  - `TestUpdateKeepsPartsForNormalize`: change only `Normalize` → `Current().Scanner` is the same pointer, config saved.
  - `TestUpdateRootsRescans`: add a second root with a file → new scanner, both roots' files catalogued.
  - `TestUpdateTokenRebuildsIdentifier`: change the token → new `Identifier`, the fake Discogs constructor got the new token; no scan started.
  - `TestUpdateLAN`: a `LAN` hook that records calls; enabling LAN calls it once with the new settings, saves, keeps the parts; a change of only `PinHash` does not call it; a hook error → `*ValidationError`, config file unchanged, current config unchanged.
  - `TestUpdateValidation` and `TestReadOnly`: `f` returning `*ValidationError` → returned, nothing saved; browse-only → `ErrReadOnly`.
- [ ] **Step 2: Run** `go test ./internal/engine`: fails to compile.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./internal/engine`: pass.
- [ ] **Step 5: Commit** `feat: engine: the parts that depend on config.json, replaced when it changes`

---

### Task 10: Server: engine wiring and settings endpoints

**Files:**
- Modify: `internal/server/server.go` (`Engine` field, `parts()`, `status`, `scan`, the `local` flag in the request context), `internal/server/identify.go` (read the identifier through `parts()`)
- Create: `internal/server/settings.go`
- Test: `internal/server/settings_test.go`, `internal/server/server_test.go`

**Consumes:** `engine.Engine`, `engine.Runtime`, `engine.ValidationError`, `engine.ErrReadOnly` (Task 9); `config.Root`, `config.CheckRoots`, `config.Available`, `config.DefaultRoots`, `config.LAN` (Task 8); `auth.CheckFormat`, `auth.HashPIN` (Task 6); `Store.DeleteSessions` (Task 7); `discogs.New`, `discogs.ErrUnauthorized`, `httpx.ErrOffline`.

**Interfaces (produces):**

```go
// in Server:
	// Engine holds what config.json decides; nil (tests): the fixed fields
	// Scanner, Identifier and Normalize, and no settings.
	Engine *engine.Engine
	// VerifyToken asks Discogs whether it takes a token; nil: asking for a
	// well-known release once.
	VerifyToken func(ctx context.Context, token string) error

// parts are the pieces config.json decides, read once per request.
type parts struct {
	scanner    *scan.Scanner // nil: browse-only mode
	identifier Identifier    // nil: browse-only mode
	normalize  bool
	lan        config.LAN
	scan       func() // starts a scan in the background
}

func (s *Server) parts() parts

// localRequest tells whether the request comes from the computer that runs
// Tordo (see decision 11); set by localOnly (always) and by the LAN gate.
func localRequest(r *http.Request) bool

// settings wraps the handlers that change or reveal the configuration: 403
// with "Los ajustes solo se cambian en la computadora que corre Tordo."
// when the request is not local.
func settings(h http.HandlerFunc) http.HandlerFunc
```

With an `Engine`, `parts()` reads `Engine.Current()` once (identifier set only when `rt.Identifier != nil`: never a nil pointer in the interface; `scan: rt.Scan`); without one, the old fields (`scan` runs `Scanner.Run(context.Background())` in a goroutine, as `scan` does today). Every handler that used `s.Scanner`, `s.Identifier` or `s.Normalize` reads `p := s.parts()` once. `GET /api/status` gains `"setupPending"` (false without an engine).

**Endpoints** (all wrapped in `settings`; writes also in `jsonOnly`; without an `Engine`: 404):
- `GET /api/config` →
  ```json
  {"setupPending": false, "readOnly": false,
   "roots": [{"path": "../musica", "available": true}], "suggested": ["../otra"], "ignore": ["Podcasts"],
   "hasToken": true, "tokenHint": "…a1b2", "normalize": true,
   "lan": {"enabled": false, "port": 4747, "hasPin": false}}
  ```
  `suggested`: sibling folders not already roots. `tokenHint` only for tokens of 8+ characters. Never the token nor the PIN hash.
- `PUT /api/config` with `{"roots": [...], "ignore": [...], "token": "…"|null, "normalize": true, "lan": {"enabled": true, "port": 4747, "pin": "…"|null}}`, resolved inside `Engine.Update`:
  - roots: `config.CheckRoots(appDir, roots, cur.Roots)`; ignore: trimmed, empty ones dropped, at most 100, each without `..`;
  - `token` null keeps the saved one, `""` removes it, else trimmed;
  - `lan.port` 1024–65535 else `el puerto tiene que estar entre 1024 y 65535`; `lan.pin` null keeps the saved hash, else `auth.CheckFormat` and `auth.HashPIN`; enabled with no hash saved nor given → `elegí un PIN para usar Tordo desde la red`;
  - a `*ValidationError` → 400 with its text; `ErrReadOnly` → 409; other errors → 500 `no se pudo guardar config.json` (logged).
  - After a successful save with a new PIN: `Store.DeleteSessions()` (a failure is logged). Answer: the new `GET /api/config` view.
- `POST /api/config/root` `{"path": "…"}` → `{"path": "../musica", "available": true}` or 400 with `config.Root`'s error.
- `POST /api/config/token` `{"token": "…"}` → `{"valid": true|false|null}`: `discogs.ErrUnauthorized` → false; `httpx.ErrOffline` or the 15 s deadline → null; anything else (a 404 included) → true. The default `VerifyToken` builds `discogs.New(token)`, sets `HTTP.Attempts = 1` and asks `Release(ctx, 249504)`. Empty token → 400 `falta el token`.

- [ ] **Step 1: Write the failing tests** (an `engine.Engine` on a temp dir with `MB` a fake and a sibling folder layout `<tmp>/tordo` + `<tmp>/musica`; `VerifyToken` replaced; requests through `do`, which is local):
  - `GET /api/config` on a pending engine → `setupPending` true, `suggested` has `../musica`, `roots` `[]`, no `pinHash` or token anywhere in the body.
  - `PUT` saving roots, ignore, a token and normalize off → 200, `config.json` has them, `hasToken` true, `tokenHint` ends with the token's last 4, `/api/status` says `normalize: false` and `setupPending: false`; a second `PUT` with `token: null` keeps it, `""` removes it.
  - LAN: enabling without a PIN → 400; with `pin: "12"` → 400 with the format message; with `pin: "482915"` → 200, `hasPin` true, the saved hash verifies with `auth.VerifyPIN`; then `pin: null` keeps it; a new PIN deletes existing sessions (create one first); port 80 → 400.
  - A root that does not exist → 400; `POST /api/config/root` with `../musica` → 200 `available` true; with a file → 400.
  - `POST /api/config/token`: the fake returning `discogs.ErrUnauthorized` → `valid` false; `httpx.ErrOffline` → null; nil → true.
  - Browse-only engine → `PUT` 409.
  - A request not marked local (build one with `httptest.NewRequest` and call `settings(h)` directly, without the flag) → 403 with the message.
  - The old tests keep passing (no engine: the fields).
- [ ] **Step 2: Run** `go test ./internal/server -run 'Config|Settings|Status'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...`: pass.
- [ ] **Step 5: Commit** `feat: settings endpoints on top of the engine; parts read per request`

---

### Task 11: Server: LAN listener, PIN login, sessions, QR

**Files:**
- Create: `internal/server/lan.go`
- Modify: `internal/server/server.go` (the mux built once and shared by `Handler()` and `LANHandler()`; routes)
- Test: `internal/server/lan_test.go`

**Consumes:** `parts().lan` (Task 10); `auth.VerifyPIN`, `auth.Limiter`, `auth.NewToken`, `auth.TokenHash` (Task 6); the store's sessions (Task 7); `qr.Encode`, `(*Code).SVG` (Task 5); `config.LAN` (Task 8).

**Interfaces (produces):**

```go
// LANHandler serves the LAN listener: the same routes as Handler, behind
// the LAN gate (decisions 11 and 14).
func (s *Server) LANHandler() http.Handler

// ApplyLAN opens, moves or closes the LAN listener (0.0.0.0:<port>) to
// match c. Browse-only mode: an error, nothing opened. Engine.LAN calls it.
func (s *Server) ApplyLAN(c config.LAN) error

// LANState is what Ajustes shows about the LAN.
type LANState struct {
	Enabled   bool     `json:"enabled"`
	Port      int      `json:"port"`
	Listening bool     `json:"listening"`
	URLs      []string `json:"urls"` // never nil; empty when not listening
}

// lanURLs lists the addresses to open Tordo from another device: one per
// private IPv4 address, then the host's .local name (decision 16).
func lanURLs(ips []net.IP, hostname string, port int) []string

// lanHost tells whether a Host header may reach the LAN listener (decision 11).
func lanHost(host, hostname string) bool

const sessionCookie = "tordo_sesion"
const sessionDays = 90
```

The server keeps (unexported) the listener (`*http.Server` with `ReadHeaderTimeout: 10 * time.Second`, `Serve` in a goroutine), its port, and an `auth.Limiter`. `ApplyLAN`: disabled → close if open; enabled on the open port → nothing; else close and `net.Listen("tcp", fmt.Sprintf(":%d", port))` (the error, wrapped: `no se pudo abrir el puerto %d: %w`). The interfaces come from `net.Interfaces()` (up, not loopback) and `os.Hostname()` (lowercase, a trailing `.local` dropped).

**The LAN gate** (wraps the shared mux in `LANHandler`):
1. `lanHost(r.Host)` false → 403 `prohibido`. Cross-site checks and `Cross-Origin-Resource-Policy` as in `localOnly` (extract them into a shared helper).
2. Local (`RemoteAddr` loopback **and** `Host` is `localhost`, `127.0.0.1` or `[::1]`, any port) → mark the context local and serve.
3. Else, the open paths (decision 14: not under `/api/` nor `/stream/`, or `GET /api/auth`, `POST /api/auth/login`, `POST /api/auth/logout`) → serve, not local.
4. Else the cookie: `SessionByToken(TokenHash(value))`; none → clear the cookie (`MaxAge: -1`) and 401 `ingresá el PIN`; found → when `LastSeen` is over 24 h old, `TouchSession` and re-send the cookie; put the session id in the context; serve.

**Endpoints** (on both handlers):
- `GET /api/auth` → `{"local": bool, "session": bool, "device": "…", "pinRequired": bool}` (`pinRequired` = not local and no valid session; reads the cookie itself, since this path is open).
- `POST /api/auth/login` (`jsonOnly`) `{"pin": "…", "device": "…"}`: local → 400 `desde esta computadora no hace falta el PIN`; LAN not enabled or no PIN hash → 403 `el acceso desde la red está apagado`; `Limiter.Wait() > 0` → 429, `Retry-After: <seconds, rounded up>`, `Demasiados intentos. Probá de nuevo en N s.`; wrong PIN → `Fail`, 401 `PIN incorrecto`; right → `Reset`, `PruneSessions(now − 90 days)`, `NewToken`, `CreateSession(hash, device)`, the cookie (`HttpOnly`, `SameSite=Lax`, `Path=/`, `MaxAge` 90 days), 200 `{"ok": true}`. Browse-only → 409.
- `POST /api/auth/logout` (`jsonOnly`): deletes the session of the cookie when there is one, clears the cookie, 200.
- `GET /api/sessions` (`settings`) → `[Session…]`.
- `DELETE /api/sessions/{id}` (`settings`) → 204, 404 unknown.
- `GET /api/lan` (`settings`) → `LANState`.
- `GET /api/lan/qr?url=…` (`settings`) → `image/svg+xml` (`qr.Encode(url).SVG(4)`), `Cache-Control: no-store`; a URL not in `LANState.URLs` → 400.

- [ ] **Step 1: Write the failing tests** (an engine with LAN enabled and a PIN hash, installed through `Engine.Use`; requests built with `httptest.NewRequest`, `RemoteAddr` `192.168.1.20:5000` for a phone, `127.0.0.1:5000` for the computer):
  - `TestLANHostCheck`: `lanHost` accepts `192.168.1.5:4747`, `[fe80::1]:4747`, `localhost:4747`, `mimac.local:4747`, `MIMAC:4747` (hostname `mimac`); rejects `evil.example:4747`, `mimac.evil.example`.
  - `TestLANGate`: a phone without cookie → `/api/status` 401, `/stream/1` 401, `/` 200 (the app shell; use a test `Static` FS), `GET /api/auth` 200 with `pinRequired` true; loopback with `Host: 127.0.0.1:4747` → `/api/status` 200 and `/api/config` 200; loopback with `Host: 192.168.1.5:4747` → treated as a phone (401); a foreign Host → 403.
  - `TestLogin`: wrong PIN → 401; right PIN → 200 and a `tordo_sesion` cookie with `HttpOnly`, `SameSite=Lax`, `Max-Age` 7776000; with it `/api/status` 200 and `/api/auth` says `session` true with the device name; `/api/config` with it → 403 (not local); with the limiter's clock faked: five wrong PINs → 401 each; the sixth wrong PIN → 401 and the lock starts; the next attempt, even with the right PIN → 429 with `Retry-After: 30`; 30 s later the right PIN → 200.
  - `TestSessionsEndpoints`: two logins → `GET /api/sessions` (local) lists both; `DELETE` one → its cookie now gets 401 and the cookie is cleared; logout with the other → 401 after.
  - `TestSessionTouch`: a session last seen 2 days ago (override the store's clock or create it with a fake `now`) → a request updates `LastSeen` and re-sends the cookie; a session seen 1 hour ago → no cookie in the answer.
  - `TestApplyLAN`: a free port (listen on `:0`, read the port, close) → `ApplyLAN` enabled → `GET http://127.0.0.1:<port>/api/auth` answers; disabled → the connection is refused; a taken port → an error naming the port.
  - `TestLANURLs`: IPs `192.168.1.5`, `10.0.0.2`, `8.8.8.8`, `127.0.0.1`, an IPv6 → `http://192.168.1.5:4747/`, `http://10.0.0.2:4747/`, then `http://mimac.local:4747/`; an empty hostname → no `.local` entry.
  - `TestLANQR`: `/api/lan/qr?url=<one of the URLs>` → 200 SVG; another URL → 400. (Inject the addresses: let the server hold an `interfaces func() ([]net.IP, string)`, nil meaning the real ones.)
- [ ] **Step 2: Run** `go test ./internal/server -run 'LAN|Login|Session'`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test -race ./...`: pass.
- [ ] **Step 5: Commit** `feat: LAN listener with PIN, sessions, devices and QR`

---

### Task 12: `cmd/tordo`: the engine, first run, LAN at start

**Files:**
- Modify: `cmd/tordo/main.go`, `cmd/tordo/main_test.go`

**Consumes:** `engine.Engine` (Task 9), `Server.Engine`, `Server.ApplyLAN` (Tasks 10, 11), `config.Load` (Task 8).

`setup` builds the store, the loudness runner (unchanged: it does not depend on the settings), the server, then the engine `{AppDir, Store, ReadOnly, MB: musicbrainz.New(), Loudness: <the runner, only when not nil>, LAN: srv.ApplyLAN (not in browse-only mode)}`, sets `srv.Engine`, and calls `Start(cfg, pending)` where `pending = created && !readOnly`. The "se creó config.json…" save and log go away; on first use log `Primera vez: elegí las carpetas en el navegador (asistente).` `run` keeps the local listener as it is (127.0.0.1, `-port`) and closes the engine at the end. Remove the code that moved into the engine (scanner, identification runner, `OnDone`). The local listener's address is printed as today; when LAN is on, also log `Desde la red: <first LAN URL>`.

- [ ] **Step 1: Write the failing test** (`main_test.go`, as the existing test does with a temp dir): `setup` on a folder without `config.json` → the engine's `SetupPending()` true, no `config.json` written, nothing scanned; with a `config.json` → not pending.
- [ ] **Step 2: Run** `go test ./cmd/tordo`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./...`, then by hand: `go run ./cmd/tordo -dir <scratch dir with a sibling folder> -no-browser` starts, logs the first-use line, `curl -s localhost:<port>/api/config` answers `setupPending: true`. Stop it.
- [ ] **Step 5: Commit** `feat: tordo runs on the engine: first-use assistant, settings without restart, LAN`

---

### Task 13: Web libraries

**Files:**
- Create: `web/src/lib/settings.js` + `settings.test.js` (adapted from Cinexplorer's), `web/src/lib/dupes.js` + `dupes.test.js`, `web/src/lib/auth.svelte.js`
- Modify: `web/src/lib/api.js` (+ tests), `web/src/lib/router.js` (+ tests), `web/src/lib/facets.js` (+ tests), `web/src/lib/app.svelte.js`

**Interfaces (produces):**

`api.js`, new methods on `api` (same style as the existing ones):

```js
config: () => get('/api/config'),
saveConfig: (body) => put('/api/config', body),
checkRoot: (path) => post('/api/config/root', { path }),
checkToken: (token) => post('/api/config/token', { token }),
lan: () => get('/api/lan'),
sessions: () => get('/api/sessions'),
closeSession: (id) => del(`/api/sessions/${id}`),
auth: () => get('/api/auth'),
login: (pin, device) => post('/api/auth/login', { pin, device }),
logout: () => post('/api/auth/logout'),
duplicates: (tipo) => get(`/api/review/duplicates?tipo=${encodeURIComponent(tipo)}`),
toConvert: () => get('/api/review/formats'),
```

plus `export function lanQrURL(url)` → `/api/lan/qr?url=<encoded>`, and `export function onUnauthorized(fn)`: `call` invokes the registered function on any 401 except from `/api/auth/login` (whose 401 means "PIN incorrecto"), then throws as usual. `ApiError` also carries `retryAfter` (seconds, from the `Retry-After` header, or 0).

`router.js`: `/ajustes` → `{ page: 'ajustes' }`; `/bienvenida` → `{ page: 'bienvenida' }`; `/revisar/duplicados` → `{ page: 'revisar', tab: 'duplicados', kind }` with `kind` from `?tipo=` (`exactas` default; `formatos`, `sospechas`); `/revisar/formatos` → `{ page: 'revisar', tab: 'formatos' }`; `export function duplicatesHref(kind)`.

`facets.js`: the `estado` label `'con-duplicados': 'Con duplicados'`.

`settings.js` (Cinexplorer's `draft`, `addRoot`, `body`, `changed`, `savedNotice`, minus language, image prefetch and tile size; plus):

```js
// draft(config) → { roots: [{path, available, checked}], ignore: 'one\nper\nline', token: null,
//                   normalize, lan: { enabled, port, pin: '' } }   ('' keeps the saved PIN)
// body(d) → { roots, ignore: [...trimmed non-empty lines], token, normalize,
//             lan: { enabled, port: Number(port), pin: pin === '' ? null : pin } }
// changed(config, d) → bool (roots, ignore, token, normalize, lan.enabled, lan.port, a new pin)
// savedNotice(config, d) → 'Ajustes guardados. Se escanean las carpetas.' on first use or when roots or
//   ignore changed; '… Se actualizan los géneros con el token nuevo.' for a token change;
//   'Ajustes guardados.' otherwise
export function pinProblem(pin, hasPin, enabled) // '' when fine, else the Spanish message:
//   enabled && !hasPin && pin === '' → 'Elegí un PIN para usar Tordo desde la red.'
//   pin !== '' && !/^[0-9]{6,12}$/.test(pin) → 'El PIN tiene que tener entre 6 y 12 dígitos, solo números.'
export function portProblem(port) // '' or 'El puerto tiene que estar entre 1024 y 65535.'
```

`dupes.js`:

```js
export const KINDS = [
  { kind: 'exactas', label: 'Copias idénticas', count: 'exact' },
  { kind: 'formatos', label: 'Mismo tema en varios formatos', count: 'formats' },
  { kind: 'sospechas', label: 'Sospechas', count: 'suspect' },
]
// fileQuality(f): 'FLAC 24 bits / 96 kHz', 'FLAC 16 bits / 44,1 kHz', 'MP3 320 kbps', 'WMA 128 kbps'
//   (formatName(f.format); lossless: bit depth and sample rate when known; lossy: bitrate when known)
export function fileQuality(f)
// groupLine(g): 'N archivos · <size(g.sizeBytes)>' and, for exact copies, ' · <size(g.recoverable)> recuperables'
export function groupLine(g)
```

`auth.svelte.js`: `export const auth = $state({ checked: false, local: true, session: false, pinRequired: false, device: '' })`; `export async function checkAuth()` (reads `api.auth()`; on a network error leaves `checked` false and retries in 5 s); registers `onUnauthorized(() => { auth.pinRequired = true })`.

`app.svelte.js`: `export function settingsSaved()` polls the status right away and bumps `generation` (as Cinexplorer's does), so normalization and the scan show at once.

- [ ] **Step 1: Write the failing tests:** `settings.test.js` (port Cinexplorer's for the kept functions; add ignore lines, normalize, the LAN fields, `pinProblem`, `portProblem`, `savedNotice` cases); `dupes.test.js` (`fileQuality` for FLAC 24/96, FLAC without bit depth, MP3 320, MP3 without bitrate, ALAC; `groupLine` for an exact and a formats group); `api.test.js` (`lanQrURL` encodes; a 401 calls the handler, a 401 from login does not; `retryAfter` read); `router.test.js` (the new routes, `tipo` default and unknown values, `duplicatesHref`); `facets.test.js` (the label).
- [ ] **Step 2: Run** `cd web && npx vitest run`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 5: Commit** `feat(web): settings, duplicates, auth and Revisar helpers`

---

### Task 14: Web: Duplicados, Formatos a convertir, album versions

**Files:**
- Create: `web/src/components/Duplicados.svelte`, `web/src/components/FormatosAConvertir.svelte`
- Modify: `web/src/pages/Revisar.svelte`, `web/src/pages/Album.svelte`, `web/src/App.svelte` (pass `kind` to Revisar)

**Consumes:** `api.duplicates`, `api.toConvert`, `KINDS`, `fileQuality`, `groupLine`, `duplicatesHref` (Task 13); `size`, `formatName`, `folderName` from `format.js`; `albumHref`, `folderHref`.

**Revisar** gains two tabs after the existing ones: *Duplicados* (`/revisar/duplicados`) and *Formatos a convertir* (`/revisar/formatos`), each with its count in the tab like the others (`counts.exact + counts.formats` for Duplicados; `files` for Formatos). Remove the "come with M5" comment.

**Duplicados:**
- A line on top: "Tordo nunca borra archivos: esto es solo para que decidas." Sub-tabs from `KINDS` (links with `duplicatesHref`, counts from the answer), and for exact copies "<size(recoverable)> recuperables en total".
- A short explanation per kind (one line): exact — "El mismo audio en varios archivos, aunque los tags difieran."; formats — "La misma grabación del mismo álbum en archivos distintos. MEJOR marca la de más calidad."; suspects — "Mismo artista, título y duración en álbumes distintos. Suele ser legítimo (un tema en su disco y en un compilado)."
- One card per group: title — artist, `groupLine(g)`; then one row per file: a **MEJOR** badge when `best`, `fileQuality(f)`, `size(f.sizeBytes)`, the album (link `albumHref`) or "Suelto", the folder (link `folderHref(f.dir)`, shown with `folderName`), a ▶ button that plays that one file (as other single-track plays do: `player.play` / the existing helper used by `ListaPistas`), and the file's name (ellipsized, full path in `title`).
- Empty: "No hay copias idénticas." / "No hay temas repetidos en varios formatos." / "No hay sospechas de duplicado." Reload on `app.generation`. Long lists: render the first 100 groups and a "Mostrar más" button (100 more each time).

**Formatos a convertir:** a line "Formatos que conviene convertir (WMA, WAV, AIFF, APE, WavPack, ALAC). Tordo solo los detecta: no convierte nada."; totals "N archivos · <size>"; one chip per format with its files and size; a list of items: title (album link, or the folder for loose tracks), artist, format label, "N archivos · size · duration". Empty: "No hay archivos en esos formatos."

**Album page:** when `versions` is not empty, a section *Versiones* under the tracks: one row per version — title (link), artist, year, its formats (`formatName`), "N temas en común", and "copia idéntica" when `kind === 'exacta'`; plus a link "Ver duplicados →" to `duplicatesHref(kind === 'exacta' ? 'exactas' : 'formatos')`.

- [ ] **Step 1:** Implement.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser** (`go run ./cmd/tordo -dir .run` on a copy, or `.run` itself if it already has a catalog): the two tabs with counts, sub-tabs, MEJOR badges, ▶ plays a file, links; Formatos a convertir with the WMA albums; an album with versions; Explorar `estado` offers "Con duplicados"; 375 px: cards stack, rows wrap without horizontal scroll.
- [ ] **Step 4: Commit** `feat(web): Duplicados and Formatos a convertir in Revisar; album versions`

---

### Task 15: Web: Ajustes, Bienvenida, ⚙

**Files:**
- Create: `web/src/pages/Ajustes.svelte`, `web/src/pages/Bienvenida.svelte`, `web/src/components/Raices.svelte`, `web/src/components/TokenDiscogs.svelte` (adapted from Cinexplorer's `Ajustes.svelte`, `Bienvenida.svelte`, `Raices.svelte`, `Token.svelte`)
- Modify: `web/src/App.svelte` (routes, first-use redirect), `web/src/components/Nav.svelte` (⚙), `web/src/components/Icono.svelte` (a `settings` icon if missing)

**Consumes:** `settings.js`, `api.config`, `api.saveConfig`, `api.checkRoot`, `api.checkToken`, `settingsSaved` (Task 13).

- **Nav:** a ⚙ link to `/ajustes` (with `aria-label="Ajustes"`) after the status indicator, on wide screens and in the phone's top bar (not a sixth bottom tab). Active on `ajustes`.
- **First use:** in `App.svelte`, when `app.status?.setupPending` and the page is not `bienvenida`, `navigate('/bienvenida', { replace: true })` (as Cinexplorer). Bienvenida shows no nav sections but the brand.
- **Bienvenida:** steps *Carpetas* (the roots: suggested siblings checked, add one by typing a path → `checkRoot`; the ignore list folded under "Carpetas a ignorar"), *Discogs* (optional token with "Verificar"; one line on what it adds: géneros y estilos más precisos; a link to `https://www.discogs.com/settings/developers` to create one), *Listo* (normalization on by default, with one line: "Empareja el volumen entre temas, solo bajando los más fuertes."; LAN is not offered here: "El acceso desde el teléfono se activa después, en Ajustes."). "Empezar" → `saveConfig(body(d))`, `settingsSaved()`, go to `/`. Needs at least one checked root.
- **Ajustes** (`/ajustes`), sections: *Carpetas* (`Raices` + ignore textarea), *Discogs* (`TokenDiscogs`: shows `tokenHint` when there is one, "Cambiar", "Quitar", "Verificar"), *Reproducción* (normalization checkbox), *Red local* (a placeholder `<section id="red">` that Task 16 fills). One "Guardar" button enabled when `changed(config, d)`, then `notify(savedNotice(config, d))`, reload the config into a new draft and `settingsSaved()`. Errors from the server shown inline above the button. Browse-only mode: everything disabled with "Modo consulta: la configuración no se puede cambiar." A 403 from `api.config()` (another device) shows "Los ajustes solo se cambian en la computadora que corre Tordo." and nothing else (Task 16 adds the logout button there).

- [ ] **Step 1:** Implement.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser:** with a scratch app dir without `config.json` (a sibling folder with a few audio files), the app opens `/bienvenida`, the folder is checked, "Empezar" scans and lands in Inicio with the music; Ajustes: add a root by path (a wrong path shows the error), change ignore patterns (rescan), verify a fake token (`valid: false`, shown as rejected; offline shows "no se pudo verificar"), turn normalization off (the status follows without a restart); ⚙ in the nav; 375 px.
- [ ] **Step 4: Commit** `feat(web): first-use assistant and Ajustes`

---

### Task 16: Web: Red local, PIN screen, devices

**Files:**
- Create: `web/src/components/RedLocal.svelte`, `web/src/components/Dispositivos.svelte`, `web/src/pages/Entrar.svelte`
- Modify: `web/src/pages/Ajustes.svelte` (the *Red local* section; the remote view), `web/src/App.svelte` (PIN screen)

**Consumes:** `auth`, `checkAuth` (Task 13); `api.lan`, `lanQrURL`, `api.sessions`, `api.closeSession`, `api.login`, `api.logout`; `pinProblem`, `portProblem`; `deviceName` from `device.js`.

- **App:** call `checkAuth()` at start. Until `auth.checked`, render nothing but the brand (no API calls: `watchStatus` and `loadMine` start once `auth.checked && !auth.pinRequired`). When `auth.pinRequired`, render only `Entrar` (no nav, no player). After a successful login, `checkAuth()` again and start the app (reload the page with `location.reload()` if starting the pollers twice is awkward).
- **Entrar:** the brand, "Ingresá el PIN de Tordo" with one `<input type="password" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]*">`, "Entrar" (Enter submits). Sends `login(pin, deviceName(navigator.userAgent))`. 401 → "PIN incorrecto."; 429 → "Demasiados intentos. Probá de nuevo en N s." with a countdown from `retryAfter` (button disabled until then); 403 → "El acceso desde la red está apagado."; network error → the usual message. Large touch targets (the phone is the user).
- **Red local** (in Ajustes, inside the same draft and "Guardar" as the rest): a switch "Usar Tordo desde otros dispositivos de la red"; when on: port (number input, `portProblem` inline), PIN ("Elegí un PIN" when `!hasPin`, else "Cambiar PIN", with a field shown on demand; `pinProblem` inline; a note "Cambiar el PIN cierra la sesión de todos los dispositivos."); a note "La conexión es HTTP sin cifrar: usalo solo en tu red de casa." After saving, when `GET /api/lan` says `listening`: the URLs, each with its QR (`<img src={lanQrURL(url)} width="160" height="160" alt="Código QR de …">` on a white background) and a copy button. When enabled but not listening: "No está escuchando." (the save error already said why).
- **Dispositivos** (under Red local, only when LAN is enabled): the sessions (device, "conectado desde <date>", "último uso <relative date>") each with "Cerrar sesión" (confirm, then `closeSession`, reload the list). Empty: "Ningún dispositivo entró todavía."
- **Ajustes from another device** (the 403): the message plus "Cerrar sesión en este dispositivo" → `api.logout()`, then show `Entrar`.

- [ ] **Step 1:** Implement.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`: pass.
- [ ] **Step 3: Check in the browser:** enable LAN with a PIN in Ajustes; open `http://<LAN URL>` in a second browser tab (another origin: it is the phone's view, `Host` is the IP) → Entrar; a wrong PIN, then the right one → the app works, plays, and Ajustes shows the remote message and logout; back on `127.0.0.1`, Dispositivos lists the session; closing it sends the other tab back to Entrar on its next request; the QR images render; turning LAN off makes the LAN URL refuse connections. 375 px for Entrar and Red local.
- [ ] **Step 4: Commit** `feat(web): LAN in Ajustes with QR and devices; PIN screen`

---

### Task 17: Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/notes/2026-10-02-m4a-followups.md`

- [ ] **Step 1:** Update `README.md` (Spanish, like the rest of it): the first-use assistant and Ajustes (what each setting does and what a change triggers: rescan, identification again, nothing); Revisar → Duplicados (the three kinds, how each is detected, MEJOR, that Tordo never deletes) and Formatos a convertir (detection only); "Con duplicados" in Explorar's Estado; album versions; **Uso desde la red** (turning it on, port 4747, the PIN and its rules, the lockout, 90-day sessions and the device list, the QR, that settings only change from the computer itself, that it is plain HTTP for a home network and so there is no installable app); `config.json`'s `lan` block; the new endpoints (`/api/review/duplicates`, `/api/review/formats`, `/api/config…`, `/api/auth…`, `/api/sessions…`, `/api/lan…`, `versions` in `/api/albums/{id}`, `setupPending` in `/api/status`, `sizeBytes` in tracks).
- [ ] **Step 2:** In the follow-ups note, mark "Changing `normalize` needs a restart" as done (M5: Ajustes applies it live).
- [ ] **Step 3: Commit** `docs: Revisar, Ajustes and LAN access in the README`

---

### Task 18: Check against the collection and from the phone (manual, with the user)

Not for a subagent: it needs the user's judgment, their collection and their phone.

- [ ] Copy `.run`'s catalog, `config.json` and `cache/` to a separate folder at the same depth as the repo, e.g. `/Users/martin/x-projects/tordo-m5-check/run`, so the relative roots still point at `~/Music`; never touch `~/Music` or `.run`. `cd web && npm run build`, then `go run ./cmd/tordo -dir <copy>`.
- [ ] The catalog opens and migrates to v6 (through v5) without a rescan; counts as before. `config.json` without `lan` loads; Ajustes shows it.
- [ ] **Duplicados:** time `/api/review/duplicates` for each kind and the first catalog build; read the exact copies (recoverable size plausible?), the formats groups (the 54 WMA albums: which have a FLAC/MP3 twin, and does MEJOR pick right?), and a sample of suspects (*História da MPB* vs. originals, compilations): false groups? missed obvious ones? Tune the rules (with tests first) if needed.
- [ ] **Formatos a convertir:** the WMA count matches (54 files in the sample), ALAC (33), total sizes plausible.
- [ ] **Ajustes:** change normalization and the ignore list and watch what happens; a Discogs token check (the user's real token only if they want; never paste it into chat).
- [ ] **LAN from the phone** (the user's iPhone on the home Wi-Fi): enable LAN with a PIN; scan the QR; the PIN screen; a wrong PIN; enter; then, on the phone:
  - the player: play an album, pause/seek, next track; the lock screen and headphone controls (Media Session: M3a's pending item); play continues with the screen locked;
  - drag to reorder a playlist with the finger (M4a's pending item; `Lista.svelte` uses pointer events: check that the page does not scroll instead, that the row follows the finger, and that the order is saved);
  - the iPod-style Carpetas: drill down, back, ▶ at each level, scroll restored;
  - ♥, ★, add to a playlist, a station; the Duplicados tab readable at phone width;
  - Ajustes on the phone says it is only from the computer; "Cerrar sesión" works; from the computer, Dispositivos lists the phone and closing it sends the phone to the PIN screen.
- [ ] **M4b's pending check** (its Task 16, never done): time `/api/station/next` (whole library and a country), `/api/home`, `/api/stats?periodo=siempre`; play a few stations by ear (variety, likes weigh more, a long session keeps refilling); Inicio's rotating rows make sense; statistics after the plays of this check.
- [ ] Write the findings to `docs/superpowers/notes/2026-10-xx-m5-collection-check.md`, fix what comes out (tests first) and then merge into `main`.
