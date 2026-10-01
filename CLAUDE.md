# Tordo

Local, portable music catalog and player: point it at 1–5 folders on a drive, it catalogs the audio files
(tags, technical data, albums, folders, collections, duplicates), identifies them online (MusicBrainz, Discogs)
and lets you browse and listen from the browser, also from other devices on the LAN.

Sibling project of Cinexplorer (`/Users/martin/x-projects/cinexplorer`), same stack and conventions.

## Where things are

- Design spec (source of truth): `docs/superpowers/specs/2026-10-01-tordo-design.md` (in Spanish).
- Implementation plans, one per milestone: `docs/superpowers/plans/`. Milestones M1–M5 are listed in the spec, §10.
- Work one milestone per session: read the spec and that milestone's plan, then execute it.

## Rules

- **Separate repo from Cinexplorer.** Code may be copied from Cinexplorer and adapted; never import or share modules.
- **Read-only on the music collection.** v1 never writes, moves, renames or deletes audio files. The only writes
  go to the app folder (`tordo.db`, `config.json`, `cache/`).
- **Portable:** catalog paths are relative to the app folder and `/`-separated (`internal/appdir`).
- **No cgo**, minimal dependencies (`modernc.org/sqlite` only, so far). External tools (ffprobe/ffmpeg) are optional.
- User-facing text (UI, log and error messages) in **Spanish**; code, identifiers and comments in English.
- TDD: tests first, `go test ./...` must pass with no network.
- Commit messages: never add `Co-Authored-By:` trailers.

## Commands

```bash
go test ./...                       # full suite (no network)
go run ./cmd/tordo -dir .run        # run with config and catalog in .run/
bash scripts/build.sh               # cross-compile into dist/tordo/
TORDO_CORPUS=/path/to/music go test ./internal/audiofile -run Corpus -v -timeout 0   # native reader vs ffprobe
```
