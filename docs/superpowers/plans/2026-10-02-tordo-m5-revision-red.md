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
8. **First run:** no `config.json` → *setup pending*: nothing is written or scanned; the local browser is sent to `/bienvenida`, whose sibling folders of `tordo/` come checked. Saving the assistant writes `config.json` and scans everything. An existing `config.json` never shows the assistant. In browse-only mode there is no assistant (nothing can be saved).
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
