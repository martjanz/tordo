# Tordo

A local, portable catalog and player for a music collection spread over a few
folders of one drive. **Status: M3a (player)** — scanning, native tag reading,
audio fingerprints, albums, folders and collections, a localhost JSON API, and
the web interface with the player. Identification (MusicBrainz, Discogs) comes
in a later milestone (M2); see `docs/superpowers/specs/2026-10-01-tordo-design.md`.

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
  "ignore": ["Podcasts", "Soulseek/incomplete"]
}
```

`ignore` adds folders to skip: a name matches anywhere, a path with `/` matches
from the root. Other apps' folders (Ableton, djay, Audio Music Apps,
Soulseek's `downloading`, Apple Music libraries) are always skipped.
`ffprobe`, when on the `PATH`, reads the formats the built-in readers don't
(APE, WavPack, raw AAC…).

### Listening

Browse the folders as they are on disk (collections show their albums in
order), open an album and play it, shuffle a folder, queue tracks. The queue
and the volume are kept in the browser, per device. MP3, FLAC, AAC, Opus,
Vorbis and WAV play as they are, with seeking; WMA, ALAC, AIFF, APE and
WavPack need `ffmpeg` on the `PATH` and are converted on the fly (lossless to
FLAC, lossy to MP3), without seeking. Covers come from the album folder's
image or the files' embedded pictures; thumbnails are cached in
`cache/thumbs/`.

| Method and path | What it returns |
|---|---|
| `GET /api/status` | scan progress and catalog counts |
| `POST /api/scan` | starts a scan (`Content-Type: application/json`) |
| `GET /api/albums?folder=` | albums, all or inside a folder |
| `GET /api/albums/{id}` | an album with its tracks and technical data |
| `GET /api/folders?path=` | a folder: subfolders, albums and loose tracks (no `path`: the roots) |
| `GET /api/collections` | folders detected as collections, with their albums in order |
| `GET /api/play?album=&folder=&track=&shuffle=1` | the tracks to queue, in order (at most 5000) |
| `GET /stream/{trackId}` | a track's audio (`Range` for seeking; `?transcode=1` converts it) |
| `GET /api/covers/album/{id}?size=` | an album's picture, whole or as a 160/320/640 px thumbnail |
| `GET /api/covers/track/{id}?size=` | the picture embedded in a track's file |

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
