# Tordo

A local, portable catalog and player for a music collection spread over a few
folders of one drive. **Status: M1 (core)** — scanning, native tag reading,
audio fingerprints, albums, folders and collections, and a localhost JSON API.
Identification (MusicBrainz, Discogs), the web interface and the player come in
later milestones; see `docs/superpowers/specs/2026-10-01-tordo-design.md`.

- **Read-only** on your music: it never moves, renames, re-tags or deletes files.
- **Portable:** one executable per OS; the catalog (`tordo.db`) and
  `config.json` live next to it, with paths relative to it.
- **Local:** the server only listens on `127.0.0.1`.

## Use

Put the `tordo/` folder on the drive, next to your music folders, and run the
binary for your system. The first run creates `config.json` with the sibling
folders as roots, scans them and prints the API address.

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

| Method and path | What it returns |
|---|---|
| `GET /api/status` | scan progress and catalog counts |
| `POST /api/scan` | starts a scan (`Content-Type: application/json`) |
| `GET /api/albums?folder=` | albums, all or inside a folder |
| `GET /api/albums/{id}` | an album with its tracks and technical data |
| `GET /api/folders?path=` | a folder: subfolders, albums and loose tracks (no `path`: the roots) |
| `GET /api/collections` | folders detected as collections, with their albums in order |

## Development

```bash
go test ./...                       # full suite, no network
go run ./cmd/tordo -dir .run        # config and catalog in .run/
bash scripts/build.sh               # binaries in dist/tordo/
TORDO_CORPUS=/path/to/music go test ./internal/audiofile -run Corpus -v -timeout 0
```
