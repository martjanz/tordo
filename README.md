# Tordo

A local, portable catalog and player for a music collection spread over a few
folders of one drive. **Status: M5 (duplicate detection, settings, LAN access)** — scanning,
native tag reading, audio fingerprints, albums, folders and collections, automatic
MusicBrainz and Discogs identification, a localhost JSON API, and the web interface
with the player, likes, ratings, playlists, play counts, stations, Inicio, listening
statistics, loudness normalization, duplicate detection, and network access with PIN; see `docs/superpowers/specs/2026-10-01-tordo-design.md`.

- **Read-only** on your music: it never moves, renames, re-tags or deletes files.
- **Portable:** one executable per OS; the catalog (`tordo.db`) and
  `config.json` live next to it, with paths relative to it.
- **Local or LAN:** runs on `127.0.0.1` by default; optionally listens on the home network (`0.0.0.0`) with PIN authentication.

## Use

Put the `tordo/` folder on the drive, next to your music folders, and run the
binary for your system. The first run opens Bienvenida (the assistant) in the
browser, asking for the music folders (`-no-browser` to skip that; the address
is in the log). Choose the folders, an optional Discogs token and normalization,
then save to create `config.json` and scan the library. An existing `config.json`
never shows the assistant. Browse-only mode (read-only folders or `tordo`
binary without write access) shows a settings page instead.

```json
{
  "roots": ["../musica", "../musica-ordenar"],
  "ignore": ["Podcasts", "Soulseek/incomplete"],
  "discogsToken": "",
  "normalize": true,
  "lan": {
    "enabled": false,
    "port": 4747,
    "pinHash": ""
  }
}
```

`discogsToken` is optional; without it genres come from MusicBrainz, from its
links to Discogs and from the tags; get one at https://www.discogs.com/settings/developers.

`normalize` (true when absent) toggles loudness normalization; changes take effect
immediately in Ajustes without restarting.

`ignore` adds folders to skip: a name matches anywhere, a path with `/` matches
from the root. Other apps' folders (Ableton, djay, Audio Music Apps,
Soulseek's `downloading`, Apple Music libraries) are always skipped.
`ffprobe`, when on the `PATH`, reads the formats the built-in readers don't
(APE, WavPack, raw AAC…).

### Ajustes (Settings)

The settings page lets you manage the music roots, ignore patterns, Discogs token,
loudness normalization, and local network access. Each setting shows what it does
when changed: scanning the library (roots or ignore patterns changed), updating
genres (new Discogs token), or taking effect immediately (normalization).

The **Discogs token** field verifies the token with Discogs's `/oauth/identity`
endpoint once you click "Verificar"; the server accepts it anyway if verification
fails (no network). Changing the token triggers an identification update.

**Loudness normalization** can be toggled live without restarting Tordo; the
player and Explorar filters update immediately.

Settings can only be changed from the computer running Tordo. From other devices
on the local network, a message shows "Los ajustes solo se cambian en la computadora
que corre Tordo", and you can only close your session.

### Browsing and listening

Explorar lists albums, artists or tracks of the whole catalog, filtered by
decade and year, artist, country, genre and style, collection, folder,
quality, format and state (not identified, not playable); it also filters by
Me gusta (♥) and Puntaje (★ N or more), and orders by Más escuchado. The
filters live in the URL. Artist pages show their albums by year and their tracks on
compilations. The search (Ctrl+K / ⌘K) finds artists, albums, tracks and
folders while typing, ignoring case and accents. Carpetas is a file explorer:
a folder tree on the left (folders holding only one folder are merged into one
row, `Soulseek › complete`; filter it by name, or press `/`, and move with the
arrow keys and Enter) and, on the right, the folder's subfolders with their
relative size and the files directly in it, sortable by column. On a phone it
becomes one list per folder. A folder wrongly detected as a collection can be
told "no es una colección". Open an album
and play it, shuffle a folder, queue tracks. The queue
and the volume are kept in the browser, per device. MP3, FLAC, AAC, Opus,
Vorbis and WAV play as they are, with seeking; WMA, ALAC, AIFF, APE and
WavPack need `ffmpeg` on the `PATH` and are converted on the fly (lossless to
FLAC, lossy to MP3), without seeking. Covers come from the album folder's
image, the files' embedded pictures or the Cover Art Archive; thumbnails are
cached in `cache/thumbs/`.

#### Likes and ratings

Mark tracks, albums, artists and collections with ♥ (me gusta). Rate tracks and
albums with ★ (1–5 stars). These marks are shown on track rows (alongside a menu
with rating, "A continuación", "A la cola", and "Agregar a una lista…"), album
page headers, artist pages, the player bar, collection cards and folder pages.
They are saved per audio fingerprint, so they follow files that move, rename or
get re-tagged.

#### Play counts

A play is counted once per pass when you listen to at least 4 minutes or half the
track, whichever is less. Seeking does not add listening time. Play
counts are recorded with the device name; browse them in Explorar.

#### Playlists

Create a playlist from the Listas page or with "+ Lista" on any album, folder,
artist, collection, or from a track's ⋯ menu. Reorder entries by dragging the
handle (mouse or touch) or Alt+↑/↓. Rename, delete the list, or remove individual
entries; tracks whose files went missing stay listed as "no disponible" and are
skipped when playing. Max 5000 entries per playlist.

#### Listas page

View your playlists and curation: Listas de reproducción, your Me gusta by
type (Pistas · Álbumes · Artistas · Colecciones), and your Colecciones.

#### Stations

A station plays a sequence of albums or tracks drawn from a source, never repeating
what it just queued, with rules for variety and avoiding recently-heard music. Its
source is one of:
- An **Explorar query** with two views: `pistas` (the matching tracks), or `albumes`
  (every track of the matching albums; maps to Explorar's Artistas tab for artists).
- A **folder** (recursive; collections are folders too).
- A **playlist** (the entries that can still play).
- **Liked** (♥ me gusta): all liked tracks, all tracks of liked albums, all tracks of
  liked artists, and all tracks of liked collections.

**Modes** are `pistas` (track by track) or `albumes` (whole albums at a time; a single
track counts as an album of one).

**Rules** (with defaults):
- `artistGap` (0–20, default 3): no artist again within the last N tracks.
- `avoidDays` (0–365, default 7): skip tracks played in the last N days (an album:
  any of its tracks).
- The station never repeats a track it already queued (the browser sends up to the
  last 100 played).

When no track fits all rules, they **relax one at a time** in this order: artist gap,
avoid days, already queued (but never the one that just played, if there is another).
The batch is empty (and the station stops) only when the source has nothing playable;
otherwise, relaxed rules allow tracks to repeat.

The browser pulls up to 10 tracks at a time and asks for more when fewer than 3
remain. The queue keeps only the last 100 played tracks, so the station never runs
out. Saved stations are stored by name and live in Tus estaciones on Inicio.

**Weights** (for drawing tracks):
- Base: 1.
- ×3 when the track or its album is ♥.
- ×2 when its artist is ♥.
- × {1: 0.25, 2: 0.5, 3: 1, 4: 2, 5: 3} by the highest of the track's and album's rating (★).

#### Inicio

The first page, showing:
- Fixed rows: **Agregados recientemente** (newest albums), **Tus estaciones** (saved
  stations, plus two built-ins: "Toda la colección" and "Tus me gusta" when something
  is liked), **Seguir escuchando** (albums by their latest play), **Tus me gusta**
  (liked albums, most recent first).
- **Three rotating rows**, drawn with a seed from the catalog's facets (decade,
  country, genre, style, collections, never-played albums, unheard-in-a-year albums,
  random albums). Most rows need at least 4 albums to be drawn; the random albums row
  needs only 1. Rows with insufficient items are hidden.

The ↻ button draws a new seed and redraws the rotating rows; a new visit draws a new
seed. Only reloads after a scan or identification keep the same seed. Each row links
to Explorar with its filter and (except fixed non-curated rows) an ▶ button to start
its station.

#### Tu escucha (statistics)

Your listening history by period: **semana** (7 days), **mes** (30 days), **año**
(365 days), **siempre** (all time). Rankings of most-played tracks, albums, artists,
countries, and genres; totals (plays, time listened, distinct tracks); and
catalog-wide counts of never-played and unheard-in-a-year **albums** (linked to
Explorar).

A play is attributed to **one** track: the file it was played from when present
with the same audio, else the lowest-numbered present file with that audio. Each of
an album's genres counts as one play.

#### Escucha facet

Filter albums or tracks by `escucha`: **nunca** (never played) or **hace-un-anio**
(played, but the last play is more than 365 days old). Albums are never-played only
if none of their tracks have been played.

#### Artist pages

Show the artist's 5 most-played tracks (all time) above their albums.

| Method and path | What it returns |
|---|---|
| `GET /api/status` | scan progress, catalog counts, normalization status, `setupPending` flag |
| `POST /api/scan` | starts a scan (`Content-Type: application/json`) |
| `GET /api/albums?folder=` | albums, all or inside a folder |
| `GET /api/albums/{id}` | an album with its tracks (with `sizeBytes` per track), versions (other albums sharing duplicate groups), and technical data |
| `GET /api/folders?path=` | a folder: subfolders (with size), albums, loose tracks and the files directly in it (no `path`: the roots) |
| `GET /api/folders/tree?path=` | one level of the Carpetas tree: subfolders, pass-through chains merged |
| `GET /api/folders/tree/search?q=` | folders by name, with the tree rows leading to them |
| `GET /api/collections` | folders detected as collections, with their albums in order |
| `PUT /api/folders/collection` | `{"path", "collection": false}`: not a collection (`true` takes it back) |
| `GET /api/explore?ver=&<facets>&orden=&dir=&desde=&limite=` | a page of Explorar (albums, artists or tracks) with the facets' counts |
| `GET /api/artists/{key}` | an artist: albums by year, appearances on others' albums, and their 5 most-played tracks in `top` |
| `GET /api/search?q=&limite=` | artists, albums, tracks and folders matching `q` |
| `GET /api/genres` | the taxonomy of genres and styles |
| `GET /api/genres/unmapped` | raw tag genres no mapping knows (Géneros por mapear) |
| `PUT /api/genres/aliases`, `DELETE /api/genres/aliases?raw=` | map a raw genre (or ignore it), or forget the mapping |
| `GET /api/review/unidentified` | albums awaiting identification review |
| `PUT /api/albums/{id}/identification` | choose the identification for an album (`DELETE` goes back to automatic) |
| `GET /api/review/duplicates?tipo=` | duplicate groups; `tipo` is exacta, formatos, or sospecha |
| `GET /api/review/formats` | tracks needing conversion by format |
| `GET /api/marks` | likes and ratings for tracks and albums (by id), and liked artists and collections |
| `PUT /api/likes` | set or unset me gusta (♥) (`{kind, id|key, liked}`; `kind` is track/album/artist/collection) |
| `PUT /api/ratings` | set or clear a rating for a track or album (`{kind, id, stars}`; `stars` 0–5, 0 clears) |
| `POST /api/plays` | record a play of a track (`{id, listenedMs, device}`); ignored in browse-only mode |
| `GET /api/liked` | the liked tracks, albums, artists and collections, most recent first |
| `GET /api/gains?track=` | each track's gains in dB (`{id: {trackGain, albumGain}}`, from ReplayGain tags or measured loudness); unmeasured tracks go first in the analysis queue; tracks marked "unmeasurable" (no ReplayGain tag and measurement failed permanently) are not re-analyzed |
| `GET /api/playlists` | user's playlists |
| `POST /api/playlists` | create a new playlist |
| `GET /api/playlists/{id}` | entries of a playlist |
| `PUT /api/playlists/{id}` | rename a playlist |
| `DELETE /api/playlists/{id}` | delete a playlist |
| `POST /api/playlists/{id}/items` | add an entry to a playlist |
| `DELETE /api/playlists/{id}/items/{item}` | remove an entry |
| `PUT /api/playlists/{id}/order` | reorder entries |
| `GET /api/stations` | saved stations, most recently updated first |
| `POST /api/stations` | save a station (body: `{name, source, mode, artistGap, avoidDays}`) |
| `PUT /api/stations/{id}` | rename or update a station |
| `DELETE /api/stations/{id}` | delete a saved station |
| `GET /api/station/next?def=<JSON>&recientes=1,2,3` | the next batch of tracks from the station (at least 10 tracks, possibly more in albums mode to include complete albums), with mime and gains; `recientes` is a comma-separated list of up to 200 recently-played track ids; the response includes `station` (the station definition as the server applied it) |
| `GET /api/home?semilla=N` | Inicio: fixed rows, rotating rows, and saved stations; `semilla` (seed) draws the rotating rows, missing or invalid gives a random one |
| `GET /api/stats?periodo=&limite=` | Tu escucha: `periodo` is semana/mes/anio/siempre (default mes); `limite` is the length of each ranking (default 10, range 1–100) |
| `GET /api/play?album=&folder=&artist=&track=&shuffle=1&playlist=&liked=1` | the tracks to queue, in order (at most 5000) |
| `GET /stream/{trackId}` | a track's audio (`Range` for seeking; `?transcode=1` converts it) |
| `GET /api/covers/album/{id}?size=` | an album's picture, whole or as a 160/320/640 px thumbnail |
| `GET /api/covers/track/{id}?size=` | the picture embedded in a track's file |
| `GET /api/config` | current configuration (settings only readable from the local computer) |
| `PUT /api/config` | save configuration: roots, ignore patterns, Discogs token, normalization, LAN settings (local computer only) |
| `POST /api/config/root` | check whether a folder can be a root: returns its canonical path or an error |
| `POST /api/config/token` | check whether Discogs accepts a token: returns `{valid: true/false/null}` (null when offline) |
| `GET /api/lan` | LAN info: enabled, port, listening, and URLs for other devices to reach Tordo |
| `GET /api/lan/qr?url=` | SVG QR code of a URL |
| `GET /api/auth` | authentication status; returns session info when authenticated, 401 when PIN needed |
| `POST /api/auth/login` | authenticate with PIN from a LAN device: `{pin, device}` |
| `POST /api/auth/logout` | end this session |
| `GET /api/sessions` | list active sessions (local computer only) |
| `DELETE /api/sessions/{id}` | end a session by id |

## Identification

After every scan, tordo looks up each album on MusicBrainz (one request per
second, as MusicBrainz asks; a first pass over ~10,000 albums takes a few
hours and resumes where it stopped). Albums above the confidence threshold are
identified automatically: they show MusicBrainz's titles, the original year,
the artist's country, Discogs genres and styles, and a cover from the Cover Art
Archive when the folder has none (saved in `cache/covers/`). The others go to
Revisar → Sin identificar with their candidates: choose one, paste a
musicbrainz.org release, or say the album is not in MusicBrainz (also from the
album page, *Corregir identificación*). Raw tag genres no mapping knows go to
Revisar → Géneros por mapear. Corrections are stored per track
fingerprint: they survive moving, renaming and re-tagging the files, and the
automatic identification never overrides them. Without network, everything
else works and the identification retries on its own.

### Revisar (Review)

#### Duplicados (Duplicates)

Tordo detects three kinds of duplicate groups; it never deletes files, only lists them.

1. **Exacta (exact copies):** two or more files with identical audio (same audio fingerprint). Recoverable bytes of a group: all its files minus the largest one.
   Groups are sorted by recoverable bytes, largest first.

2. **Formatos (same recording in several formats):** tracks of the same album with the same MusicBrainz recording ID, or with the same normalized artist and title and duration within 2 seconds (chained by duration). Files are sorted by quality (lossless > lossy; lossless by bit depth then sample rate; lossy by bitrate), then by path. The best file gets a MEJOR badge when it is strictly better than every other file of its group.

3. **Sospecha (suspect, same artist and title across albums):** the same artist, title and duration (within 2 seconds) across different albums, holding at least two distinct audio fingerprints. Sorted by artist, then title. Informational only. The best file of each group gets a MEJOR badge when strictly better than all others.

An album is marked "con duplicados" (Estado filter in Explorar) when one or more of its tracks belong to an exact or formats duplicate group.

#### Formatos a convertir (Formats to convert)

Detects which tracks use formats that need conversion to play: WMA, WAV, AIFF, APE, WavPack, and ALAC.
Shows totals (files, bytes) and one item per album × format (or folder × format for loose tracks),
sorted by size. **Detection only:** nothing is converted automatically.

#### Album versions

The album page lists other albums that share one or more exact or formats duplicate groups with it:
their title, artist, year, formats of the shared tracks, how many of this album's tracks they share,
and the group kind (exacta when every shared track is an exact copy, else formatos).

## Loudness normalization

Tordo normalizes the volume across your collection to −18 LUFS (EBU R128).
It checks ReplayGain tags first; if they're missing, `ffmpeg` measures the
loudness in the background (newest tracks first), and the audio that's about
to play jumps the queue. Tracks not measured yet play at −6 dB. The
normalization only lowers volume, never raises it. Album gain is applied when
an album plays in order (shuffle off). Turn it on or off in Ajustes; the change
takes effect immediately without restarting.

## Uso desde la red (Network access)

By default, Tordo only listens on `127.0.0.1` (the computer running it). In Ajustes
you can turn on **Red local** to allow other devices on your home network to access it.

### Enabling LAN access

When you turn on "Usar Tordo desde otros dispositivos de la red", Tordo asks you to
set a 6–12 digit PIN. The PIN is chosen when you turn LAN on; a PIN typed while LAN
is off is not saved. Tordo then listens on `0.0.0.0` at port 4747 (default; change in
Ajustes if needed).

The connection is plain HTTP without encryption: this is safe on a home network. Use only
at home, not over the internet.

### PIN authentication

The PIN protects access from other devices on the LAN. After 5 wrong attempts, each
additional wrong attempt locks logins for 30 seconds (doubling each time, up to 1 hour);
a correct PIN resets the lock. While locked, the server answers 429 with `Retry-After`.

Changing the PIN closes every device's session (they must log in again).

### Sessions

When you log in from a device, the server creates a 90-day session (a cookie `tordo_sesion`,
`HttpOnly`, `SameSite=Lax`, not encrypted). If you don't use a device for a day, the
session is automatically extended another 90 days on your next request. Sessions older
than 90 days are deleted at the next login.

In Ajustes, you can see the list of active sessions (one row per device with its name
and last-used date) and close any one from that device.

### LAN addresses and QR

Ajustes shows the addresses where you can reach Tordo on your home network: `http://<ip>:<port>/`
for each local IPv4 address, then `http://<hostname>.local:<port>/`. Each address has a QR
code you can scan with your phone to open it.

The PIN screen appears when you load the page on a non-local device (phone, tablet, another
computer on the LAN). Once logged in, everything you can do from the main computer works:
play, browse, rate, like, make playlists, stations, and listen to your statistics. The only
things you cannot do from another device are change settings; those are only possible from
the computer running Tordo.

### Config: lan block

```json
"lan": {
  "enabled": false,
  "port": 4747,
  "pinHash": ""
}
```

- `enabled`: whether the LAN listener is running.
- `port`: the port to listen on (1024–65535); 0 or absent means the default port 4747.
- `pinHash`: the hashed PIN (PBKDF2-SHA256, 100,000 iterations). Empty when LAN is on but
  no PIN was set, or when LAN is off.

## Development

The web app (Svelte 5 + Vite, in `web/`) is embedded in the binary from
`internal/server/dist/`, which is not committed: build it before `go run` or
`go build`, or the browser shows a page asking for it.

```bash
go test ./...                       # full suite, no network
(cd web && npm ci && npm test && npm run build)   # web app tests and build
go run ./cmd/tordo -dir .run        # config and catalog in .run/
bash scripts/build.sh               # web app, then binaries in dist/tordo/
TORDO_CORPUS=/path/to/music go test ./internal/audiofile -run Corpus -v -timeout 0
```
