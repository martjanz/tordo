# Tordo

A local, portable catalog and player for a music collection spread over a few
folders of one drive. **Status: M3a (player)** — scanning, native tag reading,
audio fingerprints, albums, folders and collections, automatic MusicBrainz and
Discogs identification, a localhost JSON API, and the web interface with the
player. Explorar, search and the review pages come next (M3b); see
`docs/superpowers/specs/2026-10-01-tordo-design.md`.

- **Read-only** on your music: it never moves, renames, re-tags or deletes files.
- **Portable:** one executable per OS; the catalog (`tordo.db`) and
  `config.json` live next to it, with paths relative to it.
- **Local:** the server only listens on `127.0.0.1`.

## Use

Put the `tordo/` folder on the drive, next to your music folders, and run the
binary for your system. The first run creates `config.json` with the sibling
folders as roots, scans them and opens Tordo in the browser (`-no-browser`
to skip that; the address is in the log).

```json
{
  "roots": ["../musica", "../musica-ordenar"],
  "ignore": ["Podcasts", "Soulseek/incomplete"],
  "discogsToken": ""
}
```

`discogsToken` is optional; without it genres come from MusicBrainz, from its links to Discogs and from the tags; get one at https://www.discogs.com/settings/developers.

`ignore` adds folders to skip: a name matches anywhere, a path with `/` matches
from the root. Other apps' folders (Ableton, djay, Audio Music Apps,
Soulseek's `downloading`, Apple Music libraries) are always skipped.
`ffprobe`, when on the `PATH`, reads the formats the built-in readers don't
(APE, WavPack, raw AAC…).

### Browsing and listening

Explorar lists albums, artists or tracks of the whole catalog, filtered by
decade and year, artist, country, genre and style, collection, folder,
quality, format and state (not identified, not playable); the filters live in
the URL. Artist pages show their albums by year and their tracks on
compilations. The search (Ctrl+K / ⌘K) finds artists, albums, tracks and
folders while typing, ignoring case and accents. Carpetas shows the folders
as they are on disk (collections show their albums in order; a folder wrongly
detected as a collection can be told "no es una colección"). Open an album
and play it, shuffle a folder, queue tracks. The queue
and the volume are kept in the browser, per device. MP3, FLAC, AAC, Opus,
Vorbis and WAV play as they are, with seeking; WMA, ALAC, AIFF, APE and
WavPack need `ffmpeg` on the `PATH` and are converted on the fly (lossless to
FLAC, lossy to MP3), without seeking. Covers come from the album folder's
image, the files' embedded pictures or the Cover Art Archive; thumbnails are
cached in `cache/thumbs/`.

| Method and path | What it returns |
|---|---|
| `GET /api/status` | scan progress and catalog counts |
| `POST /api/scan` | starts a scan (`Content-Type: application/json`) |
| `GET /api/albums?folder=` | albums, all or inside a folder |
| `GET /api/albums/{id}` | an album with its tracks and technical data |
| `GET /api/folders?path=` | a folder: subfolders, albums and loose tracks (no `path`: the roots) |
| `GET /api/collections` | folders detected as collections, with their albums in order |
| `PUT /api/folders/collection` | `{"path", "collection": false}`: not a collection (`true` takes it back) |
| `GET /api/explore?ver=&<facets>&orden=&dir=&desde=&limite=` | a page of Explorar (albums, artists or tracks) with the facets' counts |
| `GET /api/artists/{key}` | an artist: albums by year and appearances on others' albums |
| `GET /api/search?q=&limite=` | artists, albums, tracks and folders matching `q` |
| `GET /api/genres` | the taxonomy of genres and styles |
| `GET /api/genres/unmapped` | raw tag genres no mapping knows (Géneros por mapear) |
| `PUT /api/genres/aliases`, `DELETE /api/genres/aliases?raw=` | map a raw genre (or ignore it), or forget the mapping |
| `GET /api/review/unidentified` | albums awaiting identification review |
| `PUT /api/albums/{id}/identification` | choose the identification for an album (`DELETE` goes back to automatic) |
| `GET /api/play?album=&folder=&artist=&track=&shuffle=1` | the tracks to queue, in order (at most 5000) |
| `GET /stream/{trackId}` | a track's audio (`Range` for seeking; `?transcode=1` converts it) |
| `GET /api/covers/album/{id}?size=` | an album's picture, whole or as a 160/320/640 px thumbnail |
| `GET /api/covers/track/{id}?size=` | the picture embedded in a track's file |

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
