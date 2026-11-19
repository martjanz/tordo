# Tordo

English · [Español](README.es.md) · [Português](README.pt-BR.md)

**Your music collection, organized and playable from any browser in your home.**
Point Tordo at the folders where your music lives: it catalogs every album, recognizes them
online, and lets you browse and listen from your computer, phone or tablet.

- **Your files stay as they are.** Tordo only reads your music: it never moves, renames,
  re-tags or deletes anything.
- **One file, nothing to install.** Tordo keeps its catalog in its own folder, so you can
  carry it on a drive to another computer.
- **Private.** It runs on your computer and your home network, with no account and no cloud.

![Explore: the album grid with filters by decade, artist, country, genre and more](docs/screenshots/explorar.webp)

## Getting started

1. **Download** the file for your computer from the
   [latest release](https://github.com/martjanz/tordo/releases/latest):
   **Windows** `tordo-windows.exe` · **Mac** `tordo-macos.zip` · **Linux**
   `tordo-linux-amd64.zip` (`tordo-linux-arm64.zip` on a Raspberry Pi).
2. **Put Tordo next to your music folders** (on the same drive, if your music is on an
   external drive):
   - **Mac and Linux:** double-click the zip to get a `tordo` folder, and move that folder.
   - **Windows:** create a folder named `tordo` and move the downloaded file into it.
3. **Double-click the `tordo` file in that folder.** Tordo opens in your browser.
   The first time, your computer may ask before running it: see
   [If your computer blocks it](#if-your-computer-blocks-it).
4. **Choose your music folders** and click **Start**. Albums appear while Tordo reads them.

That's all. Next time, just double-click the file again. A small text window stays open
while Tordo runs; close it to stop Tordo.

To listen from your phone, open **Settings → Local network** on the computer. For help on
anything else, press **?** in Tordo.

### If your computer blocks it

Tordo is not distributed through an app store, so the first run needs one extra step:

- **Windows:** when *Windows protected your PC* appears, click **More info**, then **Run anyway**.
- **Mac:** when the Mac says it cannot verify Tordo, click **Done**, open **System Settings →
  Privacy & Security**, scroll down, click **Open Anyway** and double-click the file again.
- **Linux:** if double-clicking opens the file in a text editor, right-click it and choose
  **Run as a Program** (or **Run**).

## What you can do

- **Browse** by album, artist, track or folder, filtering by decade, country, genre, style,
  format, quality and more. Search finds anything as you type (Ctrl+K or ⌘K).
- **Get the details right:** albums are recognized on [MusicBrainz](https://musicbrainz.org)
  (and optionally [Discogs](https://www.discogs.com)) for correct titles, years, countries,
  genres and covers, without touching your files.
- **Play** albums, folders and playlists, shuffle, or start an endless **station** from an
  artist, a genre, a folder or your favorites.
- **Mark** songs with ♥, bookmark albums to come back to and rate with ★.
- **Listen at an even volume** across albums (loudness normalization).
- **See your listening history:** most-played songs, albums, artists and genres.
- **Tidy up:** find duplicates and files in formats browsers can't play. Tordo only lists
  them, it never deletes.
- **Listen from any device** on your home network, protected by a PIN.
- **Jukebox:** play through the computer's speakers and let everyone add songs from their
  phones.
- **In your language:** Spanish, English or Brazilian Portuguese.

<table>
  <tr>
    <td width="50%"><img src="docs/screenshots/inicio.png" alt="Home: recently added albums and your stations"><br><sub><b>Home</b>: recently added albums and your stations</sub></td>
    <td width="50%"><img src="docs/screenshots/album.png" alt="Album page with tracks, format, rating and identification"><br><sub><b>Album</b>: tracks, format, rating, play, shuffle, station, queue and playlist</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/screenshots/now-playing.png" alt="Now playing: the full-screen player"><br><sub><b>Now playing</b>: the full-screen player</sub></td>
    <td width="50%"><img src="docs/screenshots/carpetas.png" alt="Folders: browse the music folders with track counts, duration and size"><br><sub><b>Folders</b>: your folders with track counts, duration, format and size</sub></td>
  </tr>
</table>

<sub>The screenshots show the Spanish interface.</sub>

## Good to know

- **The first scan takes a while.** Reading a large collection takes minutes, and
  recognizing it online takes longer: about a few hours for 10,000 albums the first time.
  You can listen meanwhile, and if you close Tordo it picks up where it left off.
- **It works offline.** Only the online recognition needs the internet; it waits and retries
  on its own.
- **Optional extras:**
  - [ffmpeg](https://ffmpeg.org/download.html) lets Tordo play WMA, ALAC, AIFF, APE and WavPack
    files and even out the volume of songs without ReplayGain tags.
  - A free [Discogs token](https://www.discogs.com/settings/developers), pasted in Settings,
    gives more precise genres and styles.
- **Home network only.** Access from other devices is unencrypted HTTP: fine at home, not for
  the internet.
- **Settings are changed on the computer running Tordo**, not from other devices.

## For advanced users

### What Tordo writes

Only its own folder (the executable's folder): `tordo.db` (the catalog), `config.json` (the
settings) and `cache/` (covers and thumbnails). Catalog paths are relative to that folder, so
the whole drive can move between computers and operating systems. If the folder is read-only,
Tordo runs in browse-only mode.

### Command line

```text
tordo [-dir PATH] [-port N] [-no-browser]
```

| Option | Meaning |
|---|---|
| `-dir PATH` | the app folder (default: the executable's; also `TORDO_DIR`) |
| `-port N` | the port on `127.0.0.1` (default: a free one; the address is printed at startup) |
| `-no-browser` | don't open the browser |

`ffprobe` and `ffmpeg` are found on the `PATH`.

### Configuration

The first-run assistant and Settings write `config.json`; editing it by hand works too (while
Tordo is stopped).

```json
{
  "roots": ["../music", "../to-sort"],
  "ignore": ["Podcasts", "Soulseek/incomplete"],
  "discogsToken": "",
  "normalize": true,
  "listenMode": false,
  "language": "",
  "rescanMinutes": 60,
  "lan": { "enabled": false, "port": 4747, "pinHash": "" }
}
```

| Key | Meaning |
|---|---|
| `roots` | the music folders, relative to the app folder (or absolute) |
| `ignore` | folders to skip: a name matches anywhere, a path with `/` from the root. Other apps' folders (Ableton, djay, Apple Music, Soulseek's `downloading`…) are always skipped |
| `discogsToken` | optional; without it genres come from MusicBrainz, its links to Discogs and the tags |
| `normalize` | loudness normalization (default `true`) |
| `listenMode` | hides curation (Review, identification, scans, genre mapping) for letting others use Tordo; the API answers 403 on those routes |
| `language` | `""` follows the browser (English when it's not a supported one), or `"es"`, `"en"`, `"pt-BR"` |
| `rescanMinutes` | how often the folders are checked for changes (default `60`, `0` turns it off, maximum `10080`) |
| `lan.enabled` | listen on the home network (`0.0.0.0`) besides `127.0.0.1` |
| `lan.port` | the network port, 1024–65535 (default `4747`) |
| `lan.pinHash` | the PIN, hashed with PBKDF2-SHA256 (set it in Settings; required to turn the network on) |

### More documentation

- [How Tordo works](docs/how-it-works.md): every feature in detail, how scanning and
  identification work, stations, duplicates, loudness, and network security.
- [HTTP API](docs/api.md): the JSON API the web app uses.
- [Design spec](docs/superpowers/specs/2026-10-01-tordo-design.md) (in Spanish).

### Development

Go (no cgo) with a Svelte 5 + Vite web app in `web/`, embedded in the binary from
`internal/server/dist/`. That folder is not committed: build the web app before `go run` or
`go build`, or the browser shows a page asking for it.

```bash
go test ./...                                    # full suite, no network
(cd web && npm ci && npm test && npm run build)  # web app tests and build
go run ./cmd/tordo -dir .run                     # config and catalog in .run/
bash scripts/build.sh                            # web app, then binaries in dist/tordo/
TORDO_CORPUS=/path/to/music go test ./internal/audiofile -run Corpus -v -timeout 0
```

To publish a release, push a version tag (`git tag v1.0.0 && git push origin v1.0.0`): the
`release` workflow runs the tests and `scripts/build.sh`, zips the macOS and Linux builds with
`scripts/package.sh` (a zip keeps the executable bit), then attaches them, the Windows `.exe`,
the bare Linux binaries and `SHA256SUMS.txt` to the tag's GitHub release.
