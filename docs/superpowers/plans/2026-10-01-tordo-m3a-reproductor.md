# Tordo M3a (Biblioteca y reproductor, sin dependencias de M2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Listen to the collection from the browser: a Svelte web app embedded in the binary, with Carpetas (the folders as on disk, collections included), album pages, Listas (detected collections), a persistent player bar with a queue in the browser (shuffle, repeat, play next), near-gapless playback, Media Session, audio streamed with Range or converted by ffmpeg, and covers from folder images or embedded pictures with cached thumbnails.

**Architecture:** M3 is split in two plans. **M3a (this one)** only needs what M1 already built, so it runs **in parallel with M2**: it adds new packages (`stream`, `covers`, `platform`), new store queries in a new file (`store/library.go`, no schema change), new endpoints in a new file (`server/media.go`), and the whole `web/` app. **M3b** (written after M2 merges) adds what depends on M2's schema and API: Explorar with facets (genre/style, country), the artist page, search (FTS5), identification details and corrections on the album page, Revisar (Sin identificar, Géneros por mapear) and the Cover Art Archive covers. The player keeps its queue in `localStorage` (one per device); the server only resolves *what* to play (`GET /api/play`) and delivers audio by track id (`GET /stream/{id}`), never by path.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`), Svelte 5 + Vite 8 + Vitest 5, `@fontsource-variable/inter` (the same front-end stack and versions as Cinexplorer). Optional `ffmpeg` on the `PATH`.

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish). M3a covers §7.1 (player: bar, two `<audio>` elements with preload, queue in `localStorage`, Media Session, streaming with Range, ffmpeg conversion) except normalization (M4), and from §8 the shell (navigation, status indicator, responsive layout with bottom tabs on phones), **Carpetas**, the **Álbum** page with M1 data, and **Listas** (collections). Also the M2 plan's "deferred to M3": serving *embedded* pictures.

**Verified:** when this plan was written, all of its code was applied to a copy of the repo at commit `b097589` (end of M1 + the M2 plan, no M2 code): `go vet ./...` clean, `go test ./...` and `go test -race` on `server`, `covers`, `stream` and `cmd/tordo` pass with no network, `npx vitest run` passes (50 tests), `npm run build` builds, and the app was run against `/Users/martin/Music` in a browser (desktop and phone sizes): browsing, covers and thumbnails, MP3/FLAC playback with seeking, the preloaded hand-off to the next track, WMA converted by ffmpeg, the queue panel, the phone sheet, and the queue restored after a reload. `TestCorpusPictures` on that collection: 3733 files with a picture, 40 empty (taggers that write an empty APIC), 0 failures.

## Running in parallel with M2

M2 is being executed in `.worktrees/m2-identificacion`. Execute M3a in **its own worktree from `main`** (`superpowers:using-git-worktrees`, e.g. `.worktrees/m3a-reproductor`), never in M2's.

- M3a changes **no schema** (`internal/store/schema.sql` and `store.go` are untouched) and reads only M1 columns, which M2 keeps.
- Files both milestones touch, and how to merge whichever lands second:
  - `internal/server/server.go`: both add `AppDir string` to `Server` (keep one) and register routes (keep both sets). M2's `GET /api/albums/{id}/cover` and M3a's `GET /api/covers/album/{id}` coexist; M3b unifies them.
  - `cmd/tordo/main.go`: both add fields to the `server.Server` literal in `setup` (keep all).
  - `README.md`, `CLAUDE.md`: text; keep both.
  - `internal/store/catalog.go` is **not** modified by M3a (M2 rewrites its views).
- After both merge, `go test ./...` must pass; M3b starts from there.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new Go dependencies** (only `modernc.org/sqlite v1.59.0` and its indirect deps). No cgo. The Windows browser opener uses `rundll32`, not `golang.org/x/sys`.
- npm: only `svelte`, `vite`, `vitest`, `@sveltejs/vite-plugin-svelte` (dev) and `@fontsource-variable/inter`, with Cinexplorer's version ranges.
- **Never write to the music roots.** The only writes go to the app folder: `tordo.db`, `config.json`, `cache/` (thumbnails in `cache/thumbs/`). Browse-only mode (app folder not writable) writes nothing, thumbnails included.
- Catalog paths are relative to the app folder, `/`-separated; the server turns them into OS paths with `appdir.Abs`. Audio and pictures are always asked for **by id**, never by path.
- Server listens on `127.0.0.1` only (M1's `localOnly`); write endpoints require `Content-Type: application/json`.
- SQLite `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open.
- `ffmpeg` is optional: without it, formats browsers do not play are reported and skipped, never a crash.
- User-facing strings (UI, logs, errors) in **Spanish** (rioplatense: *reproducí*, *revisá*); identifiers and comments in English. UI component and page files are named in Spanish, as in Cinexplorer (`Reproductor.svelte`, `Carpetas.svelte`).
- The web build (`internal/server/dist/`) is **not committed**: only `internal/server/dist/.gitkeep` (restored by every build from `web/public/.gitkeep`). `go test ./...` passes without Node; a binary built without the web app answers a page asking to build it.
- `go test ./...` passes **without network**; `go vet ./...` clean; `cd web && npx vitest run` passes.
- Commit messages: conventional style (`feat:`, `test:`…), **no `Co-Authored-By:` trailer**.

## File Structure

```
internal/audiofile/picture.go     Picture: the embedded picture of a file (ID3 APIC, FLAC PICTURE, Vorbis
                                  METADATA_BLOCK_PICTURE, MP4 covr, ASF WM/Picture); readers feed it via source.pic
internal/store/library.go         TrackFile, PlayItem, PlayAlbum, PlayFolder, PlayTracks, AlbumCover,
                                  AlbumPictureFiles, FolderArt, FolderTrail (no schema change)
internal/stream/                  Streamer: Plan (direct / transcode / unplayable + MIME), Serve (Range or ffmpeg)
internal/covers/                  Covers: album/track pictures, ETag, thumbnails (box filter) cached in cache/thumbs
internal/platform/                Open: the default browser
internal/server/media.go          GET /api/play, /stream/{id}, /api/covers/album/{id}, /api/covers/track/{id}
internal/server/static.go         the embedded web app (all:dist), with a page when it was not built
internal/server/server.go         Server gains AppDir, Streamer, Covers, Static; folder answer gains art and trail
cmd/tordo/                        wiring; -no-browser; opens the browser
web/                              Svelte 5 + Vite app
    src/lib/                      api, router, nav, app (status), status, format, queue (pure), player
    src/components/               Icono, Estado, Nav, Tapa, Cola, Reproductor, TarjetaAlbum, ListaPistas, Acciones
    src/pages/                    Carpetas, Album, Listas
scripts/build.sh                  builds the web app first
.github/workflows/ci.yml          + web job (npm ci, test, build)
```

## M3b: what comes after M2 (not in this plan)

Written once M2 is merged, against its final code:

- **Explorar** (`/explorar`, the new `/`): tabs Álbumes / Artistas / Pistas; facets década/año, artista, país, género → estilo, colección, carpeta, formato (lossless/lossy/codec), estado (sin identificar, no reproducible); orders año, título, artista, agregado; facets in the URL; an in-memory `catalog` package (adapted from Cinexplorer's) cached by a store change counter; pagination.
- **Artista** page: country, active years, albums by year, appearances in compilations (artist photos stay deferred).
- **Búsqueda**: FTS5 index (adapted from Cinexplorer's `search`) over artists, albums, tracks and folders, accent- and case-insensitive, Ctrl+K / ⌘K.
- **Álbum** with M2 data: original year, country, genres/styles, MusicBrainz titles, `⋯ Corregir identificación` (M2's `PUT/DELETE /api/albums/{id}/identification`), and `playable` on tracks.
- **Revisar**: Sin identificar (candidates, MusicBrainz URL, ignore) and Géneros por mapear (raw value → genre/style).
- Covers: one endpoint for folder image → embedded picture → Cover Art Archive cache (M2's `cover_cache`); play items use MusicBrainz titles.
- "No es una colección" (persisted; schema v3), "/" → Explorar.

Deferred beyond M3: artist photos, album-level manual genre edits (`source = manual`), and normalization/♥/★/playlists (M4).

---

### Task 1: Embedded pictures

**Files:**
- Create: `internal/audiofile/picture.go`
- Modify: `internal/audiofile/audiofile.go` (`source`), `read.go` (`nativeReader`), `id3.go`, `flac.go`, `vorbis.go`, `ogg.go`, `mp4.go`, `asf.go`, `build_test.go` (`src`, `wmaFileWith`), `flac_test.go`, `corpus_test.go`
- Test: `internal/audiofile/picture_test.go`

**Interfaces:**
- Produces: `audiofile.Picture(path string) (data []byte, mime string, err error)`, `audiofile.ErrNoPicture`. `source` gains `pic *picture`; `parseVorbisComment(b, t, pic)` and `readASFExtContent(d, t, pic)` take it; `nativeReader(path) func(source) (Info, error)` (nil for extensions without a native reader).

While scanning, readers keep doing what they do (they only set `Tags.HasCover`): `source.pic` is nil, and every `picture` method is a no-op on a nil receiver. `Picture` reads the file with the same native readers and a non-nil `pic`: ID3 frames, Vorbis comments and ASF attributes are in memory already; FLAC PICTURE blocks and MP4 `covr` atoms are kept as a byte range and read at the end. Rules: the first picture wins, unless a later one is the front cover (picture type 3); empty pictures do not count (some taggers write an `APIC` with no data: the real collection has 40); the MIME type is lower-cased and sniffed when missing or wrong (`image/jpg`, `jpg`); something that is not an image (an ID3 link, `-->`) is `ErrNoPicture`. No `Version` bump: nothing stored changes.

- [ ] **Step 1: Write the failing tests**

`internal/audiofile/picture_test.go`:

```go
package audiofile

import (
	"bytes"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
)

// Smallest headers http.DetectContentType recognizes, plus filler.
var (
	jpeg = []byte("\xFF\xD8\xFFfront-cover")
	png  = []byte("\x89PNG\x0D\x0A\x1A\x0Aback-cover")
)

// flacPictureBlock builds a FLAC PICTURE block body.
func flacPictureBlock(typ int, mime string, data []byte) []byte {
	return cat(u32be(typ), u32be(len(mime)), []byte(mime), u32be(0), make([]byte, 16), u32be(len(data)), data)
}

func apic(typ byte, mime string, data []byte) []byte {
	return cat([]byte{0}, []byte(mime), []byte{0, typ}, []byte("desc\x00"), data)
}

func TestPicture(t *testing.T) {
	wmPicture := func(typ byte, data []byte) []byte {
		return cat([]byte{typ}, u32le(len(data)), utf16z("image/jpeg"), utf16z(""), data)
	}
	wma, _ := wmaFileWith(make([]byte, 100), wmPicture(3, jpeg))
	cases := []struct {
		name string
		file []byte
		want []byte
		mime string
	}{
		{"front cover after another picture.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", apic(4, "image/png", png)),
			id3Frame(3, "APIC", apic(3, "image/jpeg", jpeg))), mp3Frames(20, 0x11)), jpeg, "image/jpeg"},
		{"utf-16 description.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", cat([]byte{1}, []byte("image/png\x00"), []byte{3}, []byte{0xFF, 0xFE, 'd', 0, 0, 0}, png))),
			mp3Frames(20, 0x11)), png, "image/png"},
		{"first picture without a front cover.mp3", cat(id3Tag(4,
			id3Frame(4, "APIC", apic(0, "image/png", png)),
			id3Frame(4, "APIC", apic(4, "image/jpeg", jpeg))), mp3Frames(20, 0x11)), png, "image/png"},
		{"block.flac", cat([]byte("fLaC"),
			flacBlock(0, false, streamInfo(44100, 2, 16, 441000)),
			flacBlock(6, true, flacPictureBlock(3, "image/jpeg", jpeg)), make([]byte, 100)), jpeg, "image/jpeg"},
		{"vorbis comment.opus", cat(
			mkOggPage(2, 0, 7, 0, opusHead(2, 312)),
			mkOggPage(0, 0, 7, 1, cat([]byte("OpusTags"), vorbisComment("yt-dlp",
				"METADATA_BLOCK_PICTURE="+base64.StdEncoding.EncodeToString(flacPictureBlock(3, "image/png", png))))),
			mkOggPage(4, 48312, 7, 2, make([]byte, 200))), png, "image/png"},
		{"covr.m4a", m4aFile(true, make([]byte, 100), mkAtom("covr", mp4Data(13, jpeg))), jpeg, "image/jpeg"},
		{"wm-picture.wma", wma, jpeg, "image/jpeg"},
		{"mime sniffed when the tag lies.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", apic(3, "jpg", png))), mp3Frames(20, 0x11)), png, "image/png"},
		{"upper-case mime.mp3", cat(id3Tag(3,
			id3Frame(3, "APIC", apic(3, "image/PNG", png))), mp3Frames(20, 0x11)), png, "image/png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, mime, err := Picture(writeFile(t, c.name, c.file))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, c.want) || mime != c.mime {
				t.Fatalf("got %q (%s), want %q (%s)", data, mime, c.want, c.mime)
			}
		})
	}
}

func TestPictureMissing(t *testing.T) {
	cases := map[string][]byte{
		"no picture.mp3":  cat(id3Tag(3, id3TextFrame(3, "TIT2", "Sin tapa")), mp3Frames(20, 0x11)),
		"a link.mp3":      cat(id3Tag(3, id3Frame(3, "APIC", apic(3, "-->", []byte("http://example.com/x.jpg")))), mp3Frames(20, 0x11)),
		"unsupported.ape": []byte("MAC "),
		"empty.mp3":       cat(id3Tag(3, id3Frame(3, "APIC", []byte("\x00image/jpg\x00\x00\x00"))), mp3Frames(20, 0x11)),
	}
	for name, b := range cases {
		if _, _, err := Picture(writeFile(t, name, b)); !errors.Is(err, ErrNoPicture) {
			t.Errorf("%s: err = %v, want ErrNoPicture", name, err)
		}
	}
	if _, _, err := Picture(filepath.Join(t.TempDir(), "gone.mp3")); !errors.Is(err, ErrIO) {
		t.Errorf("missing file: err = %v, want ErrIO", err)
	}
}
```

In `internal/audiofile/build_test.go`, `src` must name the fields (the struct gains one), and the WMA builder takes the picture as a parameter. Replace `src`:

```go
func src(b []byte) source { return source{r: bytes.NewReader(b), size: int64(len(b))} }
```

and replace the line `func wmaFile(payload []byte) (file []byte, headerLen int) {` (keep the body that follows it) with:

```go
func wmaFile(payload []byte) (file []byte, headerLen int) {
	return wmaFileWith(payload, []byte{1, 2, 3})
}

// wmaFileWith is wmaFile with the given WM/Picture value.
func wmaFileWith(payload, picture []byte) (file []byte, headerLen int) {
```

and inside it, `asfDesc("WM/Picture", 1, []byte{1, 2, 3}))` becomes `asfDesc("WM/Picture", 1, picture))`.

In `internal/audiofile/flac_test.go`, `TestVorbisCommentTruncated` calls `parseVorbisComment(b[:len(b)-3], &tg, nil)`.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/audiofile/`
Expected: FAIL to build (`undefined: Picture`, `undefined: ErrNoPicture`, `unknown field r in struct literal`, `too many arguments in call to parseVorbisComment`).

- [ ] **Step 3: Implement `internal/audiofile/picture.go`**

```go
package audiofile

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// ErrNoPicture means the file has no embedded picture the readers understand.
var ErrNoPicture = errors.New("el archivo no tiene imagen")

// picture collects the embedded picture while a file is read for Picture. It
// keeps the first one, or a front cover (picture type 3) over any other.
// Pictures the readers have not read yet (FLAC blocks, MP4 atoms) are kept as
// a range of the file and read at the end; FLAC PICTURE blocks (also inside
// base64 Vorbis comments) are decoded at the end too.
type picture struct {
	found bool
	front bool
	data  []byte
	mime  string
	at    Range // the picture's bytes in the file, when data is nil
	flac  bool  // data (or at) is a whole FLAC PICTURE block
}

// offer considers a picture: the first one wins unless a front cover comes
// later. Empty pictures (some taggers write them) are not pictures.
func (p *picture) offer(front bool, data []byte, mime string, at Range, flac bool) {
	if p == nil || (p.found && (p.front || !front)) || (data != nil && len(data) == 0) || (data == nil && at.Length <= 0) {
		return
	}
	*p = picture{found: true, front: front, data: data, mime: mime, at: at, flac: flac}
}

// offerID3 reads an APIC frame: encoding, MIME type (v2.2: a three-letter
// format), picture type, description, data.
func (p *picture) offerID3(data []byte, v22 bool) {
	if p == nil || len(data) < 2 {
		return
	}
	enc, rest := data[0], data[1:]
	var mime string
	if v22 {
		if len(rest) < 3 {
			return
		}
		mime = "image/" + strings.ToLower(string(rest[:3]))
		rest = rest[3:]
	} else {
		m, after, ok := bytes.Cut(rest, []byte{0})
		if !ok {
			return
		}
		mime, rest = string(m), after
	}
	if len(rest) < 1 {
		return
	}
	front := rest[0] == 3
	rest = rest[1:]
	// The description ends with a terminator of its encoding's width.
	if enc == 1 || enc == 2 {
		for i := 0; ; i += 2 {
			if i+1 >= len(rest) {
				return
			}
			if rest[i] == 0 && rest[i+1] == 0 {
				rest = rest[i+2:]
				break
			}
		}
	} else {
		_, after, ok := bytes.Cut(rest, []byte{0})
		if !ok {
			return
		}
		rest = after
	}
	p.offer(front, rest, mime, Range{}, false)
}

// offerVorbis reads a picture from a Vorbis comment: METADATA_BLOCK_PICTURE
// (a base64 FLAC PICTURE block) or the older COVERART (a base64 image).
func (p *picture) offerVorbis(key, value string) {
	if p == nil {
		return
	}
	b, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return
	}
	if key == "COVERART" {
		p.offer(false, b, "", Range{}, false)
		return
	}
	p.offer(len(b) >= 4 && be32(b) == 3, b, "", Range{}, true)
}

// offerASF reads a WM/Picture value: picture type, data length, MIME type
// and description in UTF-16LE, then the data.
func (p *picture) offerASF(v []byte) {
	if p == nil || len(v) < 5 {
		return
	}
	front, n := v[0] == 3, int(le32(v[1:5]))
	rest := v[5:]
	var mime string
	for field := 0; field < 2; field++ {
		end := -1
		for i := 0; i+1 < len(rest); i += 2 {
			if rest[i] == 0 && rest[i+1] == 0 {
				end = i
				break
			}
		}
		if end < 0 {
			return
		}
		if field == 0 {
			mime = utf16Text(rest[:end], false)
		}
		rest = rest[end+2:]
	}
	if n > len(rest) {
		return
	}
	p.offer(front, rest[:n], mime, Range{}, false)
}

// finish reads what was left as a range and decodes FLAC PICTURE blocks.
func (p *picture) finish(s source) error {
	if !p.found {
		return ErrNoPicture
	}
	if p.data == nil {
		d, err := s.read(p.at.Offset, int(p.at.Length))
		if err != nil {
			return err
		}
		p.data = d
	}
	if p.flac {
		d, mime, ok := flacPicture(p.data)
		if !ok {
			return fmt.Errorf("%w: bloque PICTURE inválido", ErrFormat)
		}
		p.data, p.mime = d, mime
	}
	sniffed := http.DetectContentType(p.data)
	p.mime = strings.ToLower(p.mime)
	if !strings.HasPrefix(p.mime, "image/") || p.mime == "image/jpg" {
		p.mime = sniffed
	}
	if !strings.HasPrefix(sniffed, "image/") {
		return ErrNoPicture // a link ("-->") or something that is not an image
	}
	return nil
}

// flacPicture decodes a FLAC PICTURE block: type, MIME type, description,
// width, height, depth, colors, then the data, all lengths big-endian.
func flacPicture(b []byte) ([]byte, string, bool) {
	p := 4
	field := func() ([]byte, bool) {
		if p+4 > len(b) {
			return nil, false
		}
		n := int(be32(b[p:]))
		p += 4
		if n < 0 || p+n > len(b) {
			return nil, false
		}
		f := b[p : p+n]
		p += n
		return f, true
	}
	mime, ok := field()
	if !ok {
		return nil, "", false
	}
	if _, ok := field(); !ok { // description
		return nil, "", false
	}
	p += 16 // width, height, depth, colors
	data, ok := field()
	return data, string(mime), ok
}

// Picture returns the picture embedded in the audio file at path, a front
// cover when there are several, and its MIME type. It reads MP3 (ID3v2),
// FLAC, Ogg (Opus, Vorbis), MP4/M4A, ASF/WMA and AIFF/WAV with an ID3 chunk.
func Picture(path string) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrIO, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrIO, err)
	}
	read := nativeReader(path)
	if read == nil {
		return nil, "", ErrNoPicture
	}
	pic := &picture{}
	s := source{r: f, size: st.Size(), pic: pic}
	if _, err := read(s); err != nil && !pic.found {
		return nil, "", err
	}
	if err := pic.finish(s); err != nil {
		return nil, "", err
	}
	return pic.data, pic.mime, nil
}
```

- [ ] **Step 4: Feed it from the readers**

`internal/audiofile/audiofile.go`, the `source` struct:

```go
type source struct {
	r    io.ReaderAt
	size int64
	pic  *picture // set by Picture: the readers keep the embedded picture
}
```

`internal/audiofile/read.go`: move the extension switch out of `ReadNative` so `Picture` can use it. `ReadNative` becomes:

```go
// ReadNative reads the file with the built-in readers only.
func ReadNative(path string) (Info, error) {
	read := nativeReader(path)
	if read == nil {
		return Info{}, ErrUnsupported
	}
	f, err := os.Open(path)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	info, err := read(source{r: f, size: st.Size()})
	info.Reader = "native"
	return info, err
}

// nativeReader returns the built-in reader for path's extension, or nil.
func nativeReader(path string) func(source) (Info, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return readMP3
	case ".flac":
		return readFLAC
	case ".ogg", ".oga", ".opus":
		return readOgg
	case ".m4a", ".mp4":
		return readMP4
	case ".wma", ".asf":
		return readASF
	case ".wav":
		return readWAV
	case ".aif", ".aiff", ".aifc":
		return readAIFF
	}
	return nil
}
```

`internal/audiofile/id3.go`, in `readID3v2`'s frame loop, right before `t.id3Frame(id, data)` (after the `v22Names` mapping, so v2.2 `PIC` arrives as `APIC`):

```go
		if id == "APIC" {
			s.pic.offerID3(data, major == 2)
		}
```

`internal/audiofile/flac.go`, in `readFLAC`: the `VORBIS_COMMENT` case calls `parseVorbisComment(d, &info.Tags, s.pic)`, and the `PICTURE` case becomes:

```go
		case 6: // PICTURE
			info.Tags.HasCover = true
			if s.pic != nil && n >= 4 {
				typ, err := s.read(pos, 4)
				if err != nil {
					return info, err
				}
				s.pic.offer(be32(typ) == 3, nil, "", Range{pos, int64(n)}, true)
			}
```

`internal/audiofile/vorbis.go`: the signature and comment become

```go
// parseVorbisComment reads a Vorbis comment block (FLAC, Ogg Vorbis, Opus):
// little-endian lengths, a vendor string, then "KEY=value" entries. Pictures
// go to pic (nil while scanning).
func parseVorbisComment(b []byte, t *Tags, pic *picture) error {
```

and the switch at the end of the loop:

```go
		switch key := strings.ToUpper(k); key {
		case "METADATA_BLOCK_PICTURE", "COVERART":
			t.HasCover = true
			pic.offerVorbis(key, v)
		default:
			t.set(k, v)
		}
```

`internal/audiofile/ogg.go`: both calls become `parseVorbisComment(comments[8:], &info.Tags, s.pic)` and `parseVorbisComment(comments[7:], &info.Tags, s.pic)`.

`internal/audiofile/mp4.go`, the start of `readItem`:

```go
func readItem(s source, item atom, t *Tags) error {
	if item.typ == "covr" {
		t.HasCover = true // read only by Picture: pictures can be large
		if s.pic == nil {
			return nil
		}
		return eachAtom(s, item.body(), item.end(), func(a atom) error {
			if a.typ == "data" && a.size-a.hdr > 8 {
				s.pic.offer(false, nil, "", Range{a.body() + 8, a.size - a.hdr - 8}, false)
			}
			return nil
		})
	}
```

`internal/audiofile/asf.go`: `readASF` calls `readASFExtContent(d, &info.Tags, s.pic)`; the function becomes `func readASFExtContent(d []byte, t *Tags, pic *picture) {`, and its `WM/Picture` case:

```go
			if name == "WM/Picture" {
				t.HasCover = true
				pic.offerASF(v)
			}
```

- [ ] **Step 5: Run the tests**

Run: `go vet ./internal/audiofile/ && go test ./internal/audiofile/`
Expected: PASS.

- [ ] **Step 6: Add the corpus check**

Append to `internal/audiofile/corpus_test.go` (and add `"errors"` to its imports):

```go
// TestCorpusPictures checks that Picture extracts the embedded picture of
// every file the readers saw one in:
//
//	TORDO_CORPUS=/Users/martin/Music go test ./internal/audiofile -run CorpusPictures -v -timeout 0
func TestCorpusPictures(t *testing.T) {
	root := os.Getenv("TORDO_CORPUS")
	if root == "" {
		t.Skip("TORDO_CORPUS no definido")
	}
	var withCover, empty, failed int
	mimes := map[string]int{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || nativeReader(p) == nil {
			return nil
		}
		info, err := ReadNative(p)
		if err != nil || !info.Tags.HasCover {
			return nil
		}
		withCover++
		_, mime, err := Picture(p)
		if errors.Is(err, ErrNoPicture) {
			empty++ // an empty or linked picture
			return nil
		}
		if err != nil {
			failed++
			t.Logf("SIN IMAGEN %s: %v", p, err)
			return nil
		}
		mimes[mime]++
		return nil
	})
	t.Logf("%d archivos con imagen; %d vacías o enlazadas; %d sin extraer; tipos %v", withCover, empty, failed, mimes)
	if withCover > 0 && failed*100 > withCover {
		t.Errorf("no se extrae la imagen de más del 1%% de los archivos que la tienen")
	}
}
```

Run: `go test ./internal/audiofile/` (skips without `TORDO_CORPUS`). If the collection is at hand: `TORDO_CORPUS=/Users/martin/Music go test ./internal/audiofile -run CorpusPictures -v -timeout 0` — expected `0 sin extraer`.

- [ ] **Step 7: Commit**

```bash
git add internal/audiofile
git commit -m "feat: extract embedded pictures from audio files"
```

---

### Task 2: Store — what to play, and where pictures come from

**Files:**
- Create: `internal/store/library.go`
- Test: `internal/store/library_test.go`

**Interfaces:**
- Consumes: M1's `albums`, `tracks`, `files`, `audio`, `folders` tables (no schema change); `queryAlbums`, `albumCols`, `sortByPosition` from `catalog.go`; `natsort.Less`.
- Produces:
  - `store.TrackFile{ID int64; Path, Format, Codec string; Lossless bool}`, `(*Store).TrackFile(id int64) (TrackFile, bool, error)` — present tracks only.
  - `store.PlayItem{ID int64 "id"; Title "title"; Artist "artist"; Album "album"; AlbumID int64 "albumId"; DurationMs int64 "durationMs"; Format "format"; Codec "codec"; Lossless bool "lossless"}` (plus unexported sort fields).
  - `(*Store).PlayAlbum(id int64) ([]PlayItem, error)`, `PlayFolder(path string) ([]PlayItem, error)`, `PlayTracks(ids []int64) ([]PlayItem, error)`.
  - `(*Store).AlbumCover(albumID int64) (folderImage string, ok bool, err error)`, `AlbumPictureFiles(albumID int64) ([]string, error)`.
  - `(*Store).FolderArt(path string) (map[string][]int64, error)`; `store.Crumb{Path "path"; Name "name"}`, `(*Store).FolderTrail(path string) ([]Crumb, error)`.

`PlayFolder` plays a folder (or a collection, which is a folder) **in the order its page shows it**: subfolders first in natural order, recursively; then the folder's own albums in collection order (`sortByPosition`), each in disc/track order; then its loose tracks. An album whose discs are in `CD1`/`CD2` subfolders belongs to the folder above them (M1's grouping), so it plays once, whole. `AlbumPictureFiles` lists the album's tracks whose tags say they hold a picture (`json_extract(tags, '$.hasCover')`), the grouping's cover track first: when that one turns out empty, the next stands in. `FolderArt` gives each subfolder's card its picture: the album when the subfolder is that album's own folder (also how the page knows to link the card to the album), else up to four albums **with a picture** for a mosaic.

- [ ] **Step 1: Write the failing tests**

`internal/store/library_test.go`:

```go
package store

import (
	"reflect"
	"testing"

	"tordo/internal/audiofile"
)

func titles(items []PlayItem) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func albumID(t *testing.T, s *Store, title string) int64 {
	t.Helper()
	as, err := s.Albums("")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.Title == title {
			return a.ID
		}
	}
	t.Fatalf("no album %q", title)
	return 0
}

const tango = "../musica/Tangos/Troilo - Obras completas"

// library is sample plus a two-disc album and a loose track in a subfolder.
func library(t *testing.T) *Store {
	return catalog(t, map[string]audiofile.Tags{
		la + "/[LA-10] Soda Stereo - Signos/01.mp3":          {Title: "Signos", Artist: "Soda Stereo", Album: "Signos", Track: 1, Year: 1986, HasCover: true},
		la + "/[LA-10] Soda Stereo - Signos/02.mp3":          {Title: "No existes", Artist: "Soda Stereo", Album: "Signos", Track: 2, Year: 1986, HasCover: true},
		la + "/[LA-9] Charly García - Clics modernos/01.mp3": {Title: "Nos siguen pegando abajo", Artist: "Charly García", Album: "Clics modernos", Year: 1983},
		la + "/[LA-11] Virus - Locura/01.mp3":                {Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", Year: 1985},
		"../musica/suelto.mp3":                               {Title: "Suelto"},
		tango + "/CD2/01.mp3":                                {Title: "Sur", Artist: "Troilo", Album: "Obras completas", Disc: 2, Track: 1},
		tango + "/CD1/02.mp3":                                {Title: "Barrio de tango", Artist: "Troilo", Album: "Obras completas", Disc: 1, Track: 2},
		tango + "/CD1/01.mp3":                                {Title: "Che bandoneón", Artist: "Troilo", Album: "Obras completas", Disc: 1, Track: 1},
		"../musica/Tangos/youtube/a.opus":                    {Title: "A"}, // alone in its folder: loose
	}, la+"/[LA-10] Soda Stereo - Signos/cover.jpg")
}

func TestPlayAlbum(t *testing.T) {
	s := library(t)
	items, err := s.PlayAlbum(albumID(t, s, "Obras completas"))
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !reflect.DeepEqual(got, []string{"Che bandoneón", "Barrio de tango", "Sur"}) {
		t.Fatalf("tracks = %q", got)
	}
	it := items[0]
	if it.Artist != "Troilo" || it.Album != "Obras completas" || it.AlbumID == 0 || it.DurationMs != 1000 || it.Format != "mp3" || it.Codec != "mp3" {
		t.Fatalf("item = %+v", it)
	}
}

func TestPlayFolder(t *testing.T) {
	s := library(t)
	items, err := s.PlayFolder("../musica")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Nos siguen pegando abajo", "Signos", "No existes", "Una luna de miel en la mano", // the collection, in order
		"Che bandoneón", "Barrio de tango", "Sur", // the two-disc album, once
		"A",      // the loose track of Tangos/youtube
		"Suelto", // loose track of the folder itself, last
	}
	if got := titles(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("tracks = %q", got)
	}
	if items[len(items)-1].AlbumID != 0 {
		t.Fatalf("a loose track has album %d", items[len(items)-1].AlbumID)
	}
	items, err = s.PlayFolder(tango + "/CD1")
	if err != nil || len(items) != 0 {
		t.Fatalf("a disc folder plays its album from the folder above: %q, err %v", titles(items), err)
	}
	items, _ = s.PlayFolder("../mus") // a prefix of a name is not a folder
	if len(items) != 0 {
		t.Fatalf("prefix: %q", titles(items))
	}
}

func TestPlayTracksAndTrackFile(t *testing.T) {
	s := library(t)
	album, _ := s.PlayAlbum(albumID(t, s, "Signos"))
	items, err := s.PlayTracks([]int64{album[1].ID, 9999, album[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(items); !reflect.DeepEqual(got, []string{"No existes", "Signos"}) {
		t.Fatalf("tracks = %q", got)
	}
	f, ok, err := s.TrackFile(album[0].ID)
	if err != nil || !ok || f.Path != la+"/[LA-10] Soda Stereo - Signos/01.mp3" || f.Format != "mp3" || f.Codec != "mp3" {
		t.Fatalf("file %+v, ok %v, err %v", f, ok, err)
	}
	if _, ok, err := s.TrackFile(9999); ok || err != nil {
		t.Fatalf("unknown track: ok %v, err %v", ok, err)
	}
}

func TestAlbumPictures(t *testing.T) {
	s := library(t)
	signos := albumID(t, s, "Signos")
	files, err := s.AlbumPictureFiles(signos)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %q", files)
	}
	cover, ok, err := s.AlbumCover(signos)
	if err != nil || !ok || cover != la+"/[LA-10] Soda Stereo - Signos/cover.jpg" {
		t.Fatalf("cover %q, ok %v, err %v", cover, ok, err)
	}
	if files, _ := s.AlbumPictureFiles(albumID(t, s, "Locura")); len(files) != 0 {
		t.Fatalf("Locura has no pictures: %q", files)
	}
	if _, ok, _ := s.AlbumCover(9999); ok {
		t.Fatal("unknown album found")
	}
}

func TestFolderArt(t *testing.T) {
	s := library(t)
	art, err := s.FolderArt("../musica")
	if err != nil {
		t.Fatal(err)
	}
	// Only Signos has a picture; Tangos holds an album without one, deeper.
	want := map[string][]int64{"../musica/Latinoamérica 600": {albumID(t, s, "Signos")}}
	if !reflect.DeepEqual(art, want) {
		t.Fatalf("art = %v, want %v", art, want)
	}
	art, _ = s.FolderArt(la)
	if got := art[la+"/[LA-11] Virus - Locura"]; !reflect.DeepEqual(got, []int64{albumID(t, s, "Locura")}) {
		t.Fatalf("an album's own folder, even without a picture: %v", got)
	}
	if art, _ := s.FolderArt(""); len(art) != 0 {
		t.Fatalf("roots: %v", art)
	}
}

func TestFolderTrail(t *testing.T) {
	s := library(t)
	trail, err := s.FolderTrail(la + "/[LA-11] Virus - Locura")
	if err != nil {
		t.Fatal(err)
	}
	want := []Crumb{{"../musica", "musica"}, {la, "Latinoamérica 600"}, {la + "/[LA-11] Virus - Locura", "[LA-11] Virus - Locura"}}
	if !reflect.DeepEqual(trail, want) {
		t.Fatalf("trail = %+v", trail)
	}
	if trail, _ := s.FolderTrail(""); len(trail) != 0 {
		t.Fatalf("top level: %+v", trail)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/store/ -run 'Play|Pictures|FolderArt|FolderTrail'`
Expected: FAIL to build (`undefined: PlayItem`, `s.PlayAlbum undefined`…).

- [ ] **Step 3: Implement `internal/store/library.go`**

```go
package store

import (
	"cmp"
	"database/sql"
	"errors"
	"path"
	"slices"
	"strings"

	"tordo/internal/natsort"
)

// TrackFile is what serving a track's audio needs.
type TrackFile struct {
	ID       int64
	Path     string // relative to the app folder
	Format   string
	Codec    string
	Lossless bool
}

// TrackFile returns a present, readable track's file.
func (s *Store) TrackFile(id int64) (TrackFile, bool, error) {
	f := TrackFile{ID: id}
	err := s.db.QueryRow(`SELECT f.path, a.format, a.codec, a.lossless FROM tracks t
		JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
		WHERE t.file_id = ? AND f.missing = 0`, id).Scan(&f.Path, &f.Format, &f.Codec, &f.Lossless)
	if errors.Is(err, sql.ErrNoRows) {
		return TrackFile{}, false, nil
	}
	return f, err == nil, err
}

// PlayItem is a track as the player queues it.
type PlayItem struct {
	ID         int64  `json:"id"` // the file id
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	AlbumID    int64  `json:"albumId"` // 0: a loose track
	DurationMs int64  `json:"durationMs"`
	Format     string `json:"format"`
	Codec      string `json:"codec"`
	Lossless   bool   `json:"lossless"`
	dir        string
	path       string
	disc       int
	track      int
}

const playQuery = `SELECT t.file_id, t.title, t.artist, COALESCE(al.title, ''), COALESCE(t.album_id, 0), t.duration_ms,
	a.format, a.codec, a.lossless, t.dir, f.path, t.disc, t.track
	FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
	LEFT JOIN albums al ON al.id = t.album_id `

func (s *Store) playItems(where string, args ...any) ([]PlayItem, error) {
	rows, err := s.db.Query(playQuery+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlayItem{}
	for rows.Next() {
		var p PlayItem
		if err := rows.Scan(&p.ID, &p.Title, &p.Artist, &p.Album, &p.AlbumID, &p.DurationMs, &p.Format, &p.Codec,
			&p.Lossless, &p.dir, &p.path, &p.disc, &p.track); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// albumOrder sorts an album's tracks: disc, track number, then file name.
func albumOrder(a, b PlayItem) int {
	return cmp.Or(cmp.Compare(a.disc, b.disc), cmp.Compare(a.track, b.track), natCompare(a.path, b.path))
}

func natCompare(a, b string) int {
	switch {
	case natsort.Less(a, b):
		return -1
	case natsort.Less(b, a):
		return 1
	}
	return 0
}

// PlayAlbum returns an album's tracks in order.
func (s *Store) PlayAlbum(id int64) ([]PlayItem, error) {
	items, err := s.playItems(`WHERE t.album_id = ?`, id)
	slices.SortStableFunc(items, albumOrder)
	return items, err
}

// PlayTracks returns the given tracks in the given order, skipping unknown ids.
func (s *Store) PlayTracks(ids []int64) ([]PlayItem, error) {
	out := []PlayItem{}
	for _, id := range ids {
		items, err := s.playItems(`WHERE t.file_id = ?`, id)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

// PlayFolder returns every track inside a folder, in the order its page
// shows them: subfolders first (natural order, recursively), then the
// folder's albums (collection order) with their tracks in order, then its
// loose tracks. An album whose discs are in subfolders (CD1, CD2) belongs to
// the folder above them, so it is played once, whole.
func (s *Store) PlayFolder(p string) ([]PlayItem, error) {
	items, err := s.playItems(`WHERE t.dir = ?1 OR substr(t.dir, 1, length(?1) + 1) = ?1 || '/'`, p)
	if err != nil {
		return nil, err
	}
	albums, err := s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE dir = ?1 OR substr(dir, 1, length(?1) + 1) = ?1 || '/'`, p)
	if err != nil {
		return nil, err
	}
	byAlbum := map[int64][]PlayItem{}
	looseIn := map[string][]PlayItem{}
	for _, it := range items {
		if it.AlbumID != 0 {
			byAlbum[it.AlbumID] = append(byAlbum[it.AlbumID], it)
		} else {
			looseIn[it.dir] = append(looseIn[it.dir], it)
		}
	}
	albumsIn := map[string][]AlbumView{}
	children := map[string][]string{}
	addDir := func(d string) {
		for d != p && strings.HasPrefix(d, p+"/") {
			parent := path.Dir(d)
			if !slices.Contains(children[parent], d) {
				children[parent] = append(children[parent], d)
			}
			d = parent
		}
	}
	for _, a := range albums {
		albumsIn[a.Dir] = append(albumsIn[a.Dir], a)
		addDir(a.Dir)
	}
	for d := range looseIn {
		addDir(d)
	}
	out := []PlayItem{}
	var walk func(d string)
	walk = func(d string) {
		kids := children[d]
		slices.SortFunc(kids, func(a, b string) int { return natCompare(path.Base(a), path.Base(b)) })
		for _, k := range kids {
			walk(k)
		}
		as := albumsIn[d]
		sortByPosition(as)
		for _, a := range as {
			tracks := byAlbum[a.ID]
			slices.SortStableFunc(tracks, albumOrder)
			out = append(out, tracks...)
		}
		loose := looseIn[d]
		slices.SortFunc(loose, func(a, b PlayItem) int { return natCompare(a.path, b.path) })
		out = append(out, loose...)
	}
	walk(p)
	return out, nil
}

// AlbumPictureFiles returns the paths of an album's tracks with an embedded
// picture, the album's cover track first: some files claim a picture but
// hold an empty one, so the next one can stand in.
func (s *Store) AlbumPictureFiles(albumID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT f.path FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
		JOIN albums al ON al.id = t.album_id
		WHERE t.album_id = ? AND json_extract(a.tags, '$.hasCover')
		ORDER BY t.file_id != al.cover_file_id, t.disc, t.track, f.path`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AlbumCover returns where an album's picture can come from: the folder
// image chosen by the grouping (a path relative to the app folder, or "")
// and whether the album exists.
func (s *Store) AlbumCover(albumID int64) (string, bool, error) {
	var p string
	err := s.db.QueryRow(`SELECT cover_path FROM albums WHERE id = ?`, albumID).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return p, err == nil, err
}

// FolderArt returns, for each subfolder of p, up to four of the albums with
// a picture inside it (at any depth, in natural order of their folders):
// what its card shows. A subfolder that is one album's own folder gets just
// that album, picture or not.
func (s *Store) FolderArt(p string) (map[string][]int64, error) {
	out := map[string][]int64{}
	if p == "" {
		return out, nil // the top level lists the roots: they get no art
	}
	prefix := p + "/"
	rows, err := s.db.Query(`SELECT id, dir, cover_path != '' OR cover_file_id != 0 FROM albums
		WHERE substr(dir, 1, length(?1)) = ?1`, prefix)
	if err != nil {
		return nil, err
	}
	type entry struct {
		id      int64
		dir     string
		picture bool
	}
	byChild := map[string][]entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.dir, &e.picture); err != nil {
			rows.Close()
			return nil, err
		}
		child := prefix + strings.SplitN(strings.TrimPrefix(e.dir, prefix), "/", 2)[0]
		byChild[child] = append(byChild[child], e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for child, es := range byChild {
		slices.SortFunc(es, func(a, b entry) int { return natCompare(a.dir, b.dir) })
		if es[0].dir == child {
			out[child] = []int64{es[0].id}
			continue
		}
		for _, e := range es {
			if e.picture && len(out[child]) < 4 {
				out[child] = append(out[child], e.id)
			}
		}
	}
	return out, nil
}

// Crumb is one folder on the way from a root to a folder.
type Crumb struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// FolderTrail returns the folders from p's root down to p itself.
func (s *Store) FolderTrail(p string) ([]Crumb, error) {
	out := []Crumb{}
	for p != "" && len(out) < 256 {
		var c Crumb
		var parent string
		err := s.db.QueryRow(`SELECT path, name, parent FROM folders WHERE path = ?`, p).Scan(&c.Path, &c.Name, &parent)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append([]Crumb{c}, out...)
		p = parent
	}
	return out, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go vet ./internal/store/ && go test ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/library.go internal/store/library_test.go
git commit -m "feat: store queries for playing albums, folders and tracks, and album pictures"
```

---
### Task 3: Streaming

**Files:**
- Create: `internal/stream/stream.go`
- Test: `internal/stream/stream_test.go`

**Interfaces:**
- Produces: `stream.Format{Container, Codec string; Lossless bool}`; `stream.Mode` (`Direct`, `Transcode`, `Unplayable`); `stream.ErrUnplayable`; `stream.Streamer{FFmpeg string}` with `Plan(Format) (Mode, mime string)` and `Serve(w, r, path string, f Format, transcode bool)`; `stream.LookFFmpeg() string`.

What browsers play goes as it is through `http.ServeContent` (Range, so seeking works): MP3, FLAC, Ogg Opus/Vorbis, MP4 with AAC/FLAC/Opus/MP3, WAV PCM. The rest (WMA, ALAC, AIFF, APE, WavPack…) is converted by `ffmpeg` as it plays: **lossless sources to FLAC** (no quality lost; FLAC's encoder is always built into ffmpeg; a small deviation from the spec's "Opus o MP3"), lossy ones to MP3 (`libmp3lame -q:a 2`). A conversion has no length, so it cannot seek. `?transcode=1` asks for a conversion of a "direct" format, for a browser that cannot play it (Safari and Ogg): the player decides with `canPlayType`. The request's context runs ffmpeg, so stopping playback kills it. Tests never need ffmpeg: the test binary doubles as a fake one (`TestMain` + `TORDO_FAKE_FFMPEG=1`), which works on Windows too.

- [ ] **Step 1: Write the failing tests**

`internal/stream/stream_test.go`:

```go
package stream

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain doubles as a fake ffmpeg: run with TORDO_FAKE_FFMPEG=1, the test
// binary prints its arguments instead of converting anything.
func TestMain(m *testing.M) {
	if os.Getenv("TORDO_FAKE_FFMPEG") == "1" {
		fmt.Print("ffmpeg " + strings.Join(os.Args[1:], " "))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeFFmpeg(t *testing.T) Streamer {
	t.Setenv("TORDO_FAKE_FFMPEG", "1")
	return Streamer{FFmpeg: os.Args[0]}
}

func TestPlan(t *testing.T) {
	with, without := Streamer{FFmpeg: "ffmpeg"}, Streamer{}
	cases := []struct {
		f    Format
		mode Mode
		mime string
	}{
		{Format{"mp3", "mp3", false}, Direct, "audio/mpeg"},
		{Format{"flac", "flac", true}, Direct, "audio/flac"},
		{Format{"ogg", "opus", false}, Direct, "audio/ogg; codecs=opus"},
		{Format{"ogg", "vorbis", false}, Direct, "audio/ogg; codecs=vorbis"},
		{Format{"mp4", "aac", false}, Direct, "audio/mp4"},
		{Format{"mp4", "flac", true}, Direct, "audio/mp4; codecs=flac"},
		{Format{"wav", "pcm", true}, Direct, "audio/wav"},
		{Format{"wav", "pcm_s24le", true}, Direct, "audio/wav"}, // ffprobe's name
		{Format{"mp4", "alac", true}, Transcode, "audio/flac"},
		{Format{"asf", "wma", false}, Transcode, "audio/mpeg"},
		{Format{"asf", "wmalossless", true}, Transcode, "audio/flac"},
		{Format{"aiff", "pcm", true}, Transcode, "audio/flac"},
		{Format{"ape", "ape", true}, Transcode, "audio/flac"},
	}
	for _, c := range cases {
		if mode, mime := with.Plan(c.f); mode != c.mode || mime != c.mime {
			t.Errorf("%+v: %v %q, want %v %q", c.f, mode, mime, c.mode, c.mime)
		}
		if c.mode == Transcode {
			if mode, _ := without.Plan(c.f); mode != Unplayable {
				t.Errorf("%+v without ffmpeg: %v", c.f, mode)
			}
		}
	}
}

func audioFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.mp3")
	if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func serve(s Streamer, path string, f Format, transcode bool, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/stream/1", nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	s.Serve(rec, req, path, f, transcode)
	return rec
}

func TestServeDirectWithRange(t *testing.T) {
	p := audioFile(t)
	mp3 := Format{"mp3", "mp3", false}
	rec := serve(Streamer{}, p, mp3, false)
	if rec.Code != http.StatusOK || rec.Body.String() != "0123456789" || rec.Header().Get("Content-Type") != "audio/mpeg" ||
		rec.Header().Get("Accept-Ranges") != "bytes" {
		t.Fatalf("whole file: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	rec = serve(Streamer{}, p, mp3, false, "Range", "bytes=2-5")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "2345" || rec.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if rec := serve(Streamer{}, filepath.Join(t.TempDir(), "gone.mp3"), mp3, false); rec.Code != http.StatusNotFound {
		t.Fatalf("missing file: %d", rec.Code)
	}
}

func TestServeTranscodes(t *testing.T) {
	p := audioFile(t)
	s := fakeFFmpeg(t)
	rec := serve(s, p, Format{"asf", "wma", false}, false)
	want := "ffmpeg -nostdin -v error -i " + p + " -map 0:a:0 -vn -c:a libmp3lame -q:a 2 -f mp3 pipe:1"
	if rec.Code != http.StatusOK || rec.Body.String() != want || rec.Header().Get("Content-Type") != "audio/mpeg" ||
		rec.Header().Get("Accept-Ranges") != "" {
		t.Fatalf("wma: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	rec = serve(s, p, Format{"mp4", "alac", true}, false)
	if !strings.Contains(rec.Body.String(), "-c:a flac -compression_level 0 -f flac pipe:1") || rec.Header().Get("Content-Type") != "audio/flac" {
		t.Fatalf("alac: %q %v", rec.Body, rec.Header())
	}
	rec = serve(s, p, Format{"ogg", "opus", false}, true) // the browser cannot play Ogg
	if !strings.Contains(rec.Body.String(), "libmp3lame") {
		t.Fatalf("asked conversion: %q", rec.Body)
	}
}

func TestServeWithoutFFmpeg(t *testing.T) {
	p := audioFile(t)
	if rec := serve(Streamer{}, p, Format{"asf", "wma", false}, false); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wma without ffmpeg: %d", rec.Code)
	}
	// Asking for a conversion without ffmpeg sends the file as it is.
	if rec := serve(Streamer{}, p, Format{"ogg", "opus", false}, true); rec.Code != http.StatusOK || rec.Body.String() != "0123456789" {
		t.Fatalf("opus: %d %q", rec.Code, rec.Body)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/stream/`
Expected: FAIL to build (`undefined: Streamer`…).

- [ ] **Step 3: Implement `internal/stream/stream.go`**

```go
// Package stream delivers a track's audio to the browser: the file as it is,
// with Range requests (seeking), when browsers play its format; converted on
// the fly by ffmpeg otherwise (no seeking). It only reads the file.
package stream

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Format is what deciding how to deliver a track needs.
type Format struct {
	Container string // audiofile.Info.Format: mp3, flac, ogg, mp4, asf, wav, aiff, or ffprobe's name
	Codec     string
	Lossless  bool
}

// Mode is how a track reaches the browser.
type Mode int

const (
	Direct     Mode = iota // the file as it is
	Transcode              // converted by ffmpeg
	Unplayable             // needs ffmpeg, which is not installed
)

// ErrUnplayable means the track needs ffmpeg and there is none.
var ErrUnplayable = errors.New("este formato necesita ffmpeg para reproducirse")

// Streamer serves tracks. FFmpeg is the path to ffmpeg ("" when there is
// none); see LookFFmpeg.
type Streamer struct {
	FFmpeg string
}

var (
	ffmpegOnce sync.Once
	ffmpegPath string
)

// LookFFmpeg returns the path to ffmpeg on the PATH, or "" (looked up once).
func LookFFmpeg() string {
	ffmpegOnce.Do(func() { ffmpegPath, _ = exec.LookPath("ffmpeg") })
	return ffmpegPath
}

// directMIME is the MIME type of the formats browsers play, by container and
// codec ("" for those they don't). Browsers that lack one of them (Safari
// and Ogg, for instance) ask for a conversion instead: see Serve.
func directMIME(f Format) string {
	switch f.Container {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		if f.Codec == "flac" {
			return "audio/flac"
		}
	case "ogg":
		switch f.Codec {
		case "opus", "vorbis":
			return "audio/ogg; codecs=" + f.Codec
		}
	case "mp4", "mov":
		switch f.Codec {
		case "aac":
			return "audio/mp4"
		case "flac", "opus", "mp3":
			return "audio/mp4; codecs=" + f.Codec
		}
	case "wav":
		if strings.HasPrefix(f.Codec, "pcm") {
			return "audio/wav"
		}
	}
	return ""
}

// transcodeMIME is what a conversion produces: FLAC for lossless sources (no
// quality lost), MP3 for the rest.
func transcodeMIME(f Format) string {
	if f.Lossless {
		return "audio/flac"
	}
	return "audio/mpeg"
}

// Plan says how f is delivered, and with which MIME type.
func (s Streamer) Plan(f Format) (Mode, string) {
	if m := directMIME(f); m != "" {
		return Direct, m
	}
	if s.FFmpeg == "" {
		return Unplayable, ""
	}
	return Transcode, transcodeMIME(f)
}

// Serve writes the audio of the file at path. transcode asks for a
// conversion even of a format that is usually played as it is (the browser
// said it cannot play it). Without ffmpeg, a format that needs it gets 415.
func (s Streamer) Serve(w http.ResponseWriter, r *http.Request, path string, f Format, transcode bool) {
	mode, mime := s.Plan(f)
	if transcode && mode == Direct && s.FFmpeg != "" {
		mode, mime = Transcode, transcodeMIME(f)
	}
	switch mode {
	case Unplayable:
		http.Error(w, ErrUnplayable.Error(), http.StatusUnsupportedMediaType)
	case Direct:
		serveFile(w, r, path, mime)
	case Transcode:
		s.transcode(w, r, path, f, mime)
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, path, mime string) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("audio: %v", err)
		http.Error(w, "no se pudo abrir el archivo", http.StatusNotFound)
		return
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		log.Printf("audio: %v", err)
		http.Error(w, "no se pudo abrir el archivo", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", mime)
	http.ServeContent(w, r, "", st.ModTime(), file)
}

// transcode runs ffmpeg and streams its output. Without a length the
// browser cannot seek; stopping playback cancels the request, which kills
// ffmpeg.
func (s Streamer) transcode(w http.ResponseWriter, r *http.Request, path string, f Format, mime string) {
	if _, err := os.Stat(path); err != nil {
		log.Printf("audio: %v", err)
		http.Error(w, "no se pudo abrir el archivo", http.StatusNotFound)
		return
	}
	args := []string{"-nostdin", "-v", "error", "-i", path, "-map", "0:a:0", "-vn"}
	if mime == "audio/flac" {
		args = append(args, "-c:a", "flac", "-compression_level", "0", "-f", "flac", "pipe:1")
	} else {
		args = append(args, "-c:a", "libmp3lame", "-q:a", "2", "-f", "mp3", "pipe:1")
	}
	cmd := exec.CommandContext(r.Context(), s.FFmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = w, &stderr
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "no-store")
	if err := cmd.Run(); err != nil && r.Context().Err() == nil {
		log.Printf("ffmpeg (%s, %s): %v: %s", path, f.Codec, err, strings.TrimSpace(stderr.String()))
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go vet ./internal/stream/ && go test -race ./internal/stream/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/stream
git commit -m "feat: stream audio with Range, or converted by ffmpeg"
```

---
### Task 4: Covers and thumbnails

**Files:**
- Create: `internal/covers/covers.go`, `internal/covers/shrink.go`
- Test: `internal/covers/covers_test.go`

**Interfaces:**
- Consumes: `store.AlbumCover`, `store.AlbumPictureFiles`, `store.TrackFile` (Task 2); `audiofile.Picture`, `audiofile.ErrNoPicture` (Task 1); `appdir.Abs`.
- Produces: `covers.New(st *store.Store, appDir, cacheDir string) *Covers` (`cacheDir` "" keeps no thumbnails), `(*Covers).ServeAlbum(w, r, albumID int64)`, `(*Covers).ServeTrack(w, r, trackID int64)`, `covers.Sizes = []int{160, 320, 640}`; field `Covers.CacheDir`.

An album's picture: the folder image the grouping chose (`cover.jpg`, `folder.jpg`…), else the picture embedded in its tracks (up to three candidates, for empty pictures). A track's picture: the one embedded in its own file (loose tracks: YouTube downloads carry thumbnails). `?size=160|320|640` asks for a thumbnail: decoded with the standard library (JPEG, PNG, GIF), scaled with a box filter, encoded as JPEG 85, and cached in `cache/thumbs/<etag>.jpg` (written through a temporary file and renamed); a format the standard library cannot decode (WebP) is sent whole. The ETag hashes the candidate files' paths, sizes and mtimes plus the size asked for, so it is computed with a few `stat`s: the browser revalidates (`Cache-Control: no-cache`) and gets `304` without reading any picture. At most two pictures are decoded at once (a 3000 px cover takes ~70 MB while decoding). M3b adds the Cover Art Archive cache that M2 downloads.

- [ ] **Step 1: Write the failing tests**

`internal/covers/covers_test.go`:

```go
package covers

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
	"tordo/internal/store"
)

// pngImage is a w×h picture, red on the left half and blue on the right.
func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 0, 0, 255}
			if x >= w/2 {
				c = color.RGBA{0, 0, 255, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// mp3With is an ID3v2.3 tag with one APIC frame (data may be empty), then
// twenty silent MPEG-1 Layer III frames (128 kbps, 44.1 kHz: 417 bytes each).
func mp3With(data []byte) []byte {
	frame := append([]byte{0}, []byte("image/png\x00")...)
	frame = append(frame, 3, 0)
	frame = append(frame, data...)
	var body []byte
	body = append(body, "APIC"...)
	body = binary.BigEndian.AppendUint32(body, uint32(len(frame)))
	body = append(body, 0, 0)
	body = append(body, frame...)
	n := len(body)
	tag := []byte{'I', 'D', '3', 3, 0, 0, byte(n>>21) & 0x7F, byte(n>>14) & 0x7F, byte(n>>7) & 0x7F, byte(n) & 0x7F}
	out := append(tag, body...)
	for i := 0; i < 20; i++ {
		out = append(out, 0xFF, 0xFB, 0x90, 0x00)
		out = append(out, make([]byte, 413)...)
	}
	return out
}

type fixture struct {
	covers *Covers
	store  *store.Store
	music  string // the music folder, absolute
}

// setup writes files under <tmp>/musica and catalogs them with the given
// tags, from an app folder at <tmp>/tordo.
func setup(t *testing.T, files map[string][]byte, tags map[string]audiofile.Tags) fixture {
	t.Helper()
	base := t.TempDir()
	app, music := filepath.Join(base, "tordo"), filepath.Join(base, "musica")
	var rows []store.FileRow
	for name, data := range files {
		p := filepath.Join(music, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		kind := "audio"
		if filepath.Ext(name) == ".jpg" || filepath.Ext(name) == ".png" {
			kind = "image"
		}
		rows = append(rows, store.FileRow{Path: "../musica/" + name, Size: int64(len(data)), MTime: 1, Kind: kind})
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	roots := []string{"../musica"}
	if err := st.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	pending, _ := st.PendingReads(env)
	var rs []store.ReadResult
	for _, p := range pending {
		rs = append(rs, store.ReadResult{Target: p, Env: env, Fingerprint: p.Path,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: tags[p.Path[len("../musica/"):]]}})
	}
	if err := st.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	tracks, images, _ := st.GroupingInput()
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
	return fixture{New(st, app, filepath.Join(app, "cache", "thumbs")), st, music}
}

func (f fixture) album(t *testing.T, title string) int64 {
	t.Helper()
	as, _ := f.store.Albums("")
	for _, a := range as {
		if a.Title == title {
			return a.ID
		}
	}
	t.Fatalf("no album %q", title)
	return 0
}

func get(serve func(http.ResponseWriter, *http.Request, int64), id int64, query string, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/covers/x/"+strconv.FormatInt(id, 10)+query, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	serve(rec, req, id)
	return rec
}

func TestAlbumCover(t *testing.T) {
	folderImage, embedded := pngImage(t, 800, 400), pngImage(t, 10, 10)
	tag := func(album string) audiofile.Tags {
		return audiofile.Tags{Title: "t", Artist: "a", Album: album, HasCover: true}
	}
	f := setup(t, map[string][]byte{
		"Con tapa/01.mp3":    mp3With(embedded),
		"Con tapa/02.mp3":    mp3With(embedded),
		"Con tapa/cover.png": folderImage,
		"Embebida/01.mp3":    mp3With(nil), // claims a picture, holds an empty one
		"Embebida/02.mp3":    mp3With(embedded),
		"Sin tapa/01.mp3":    mp3With(nil),
		"Sin tapa/02.mp3":    mp3With(nil),
		"Suelto/youtube.mp3": mp3With(embedded),
	}, map[string]audiofile.Tags{
		"Con tapa/01.mp3": tag("Con tapa"), "Con tapa/02.mp3": tag("Con tapa"),
		"Embebida/01.mp3": tag("Embebida"), "Embebida/02.mp3": tag("Embebida"),
		"Sin tapa/01.mp3": tag("Sin tapa"), "Sin tapa/02.mp3": {Title: "u", Album: "Sin tapa"},
		"Suelto/youtube.mp3": {Title: "y", HasCover: true},
	})

	rec := get(f.covers.ServeAlbum, f.album(t, "Con tapa"), "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), folderImage) || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("folder image first: %d %s", rec.Code, rec.Header())
	}
	tag1 := rec.Header().Get("ETag")
	if rec := get(f.covers.ServeAlbum, f.album(t, "Con tapa"), "", "If-None-Match", tag1); rec.Code != http.StatusNotModified {
		t.Fatalf("revalidation: %d", rec.Code)
	}

	rec = get(f.covers.ServeAlbum, f.album(t, "Con tapa"), "?size=320")
	img, err := jpeg.Decode(rec.Body)
	if err != nil || img.Bounds().Dx() != 320 || img.Bounds().Dy() != 160 || rec.Header().Get("ETag") == tag1 {
		t.Fatalf("thumbnail: %v %v, etag %q", img.Bounds(), err, rec.Header().Get("ETag"))
	}
	if r, _, b, _ := img.At(10, 80).RGBA(); r < 0xE000 || b > 0x2000 {
		t.Fatalf("left half is not red: %v", img.At(10, 80))
	}
	cached, _ := filepath.Glob(filepath.Join(f.covers.CacheDir, "*.jpg"))
	if len(cached) != 1 {
		t.Fatalf("cache = %q", cached)
	}

	rec = get(f.covers.ServeAlbum, f.album(t, "Embebida"), "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), embedded) {
		t.Fatalf("second file stands in for an empty picture: %d", rec.Code)
	}
	if rec := get(f.covers.ServeAlbum, f.album(t, "Sin tapa"), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no picture: %d", rec.Code)
	}
	if rec := get(f.covers.ServeAlbum, 9999, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown album: %d", rec.Code)
	}

	items, _ := f.store.PlayFolder("../musica/Suelto")
	rec = get(f.covers.ServeTrack, items[0].ID, "?size=999") // not a thumbnail size: the original
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), embedded) {
		t.Fatalf("track picture: %d", rec.Code)
	}
}

func TestBrowseOnlyKeepsNoThumbnails(t *testing.T) {
	f := setup(t, map[string][]byte{"A/cover.png": pngImage(t, 400, 400), "A/01.mp3": []byte("x"), "A/02.mp3": []byte("x")},
		map[string]audiofile.Tags{"A/01.mp3": {Album: "A"}, "A/02.mp3": {Album: "A"}})
	f.covers.CacheDir = ""
	rec := get(f.covers.ServeAlbum, f.album(t, "A"), "?size=160")
	if img, err := jpeg.Decode(rec.Body); err != nil || img.Bounds().Dx() != 160 {
		t.Fatalf("thumbnail: %v", err)
	}
}

func TestShrinkKeepsSmallImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	if shrink(img, 160) != image.Image(img) {
		t.Fatal("a small image was scaled")
	}
	if b := shrink(image.NewRGBA(image.Rect(0, 0, 50, 1000)), 160).Bounds(); b.Dx() != 8 || b.Dy() != 160 {
		t.Fatalf("tall image: %v", b)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/covers/`
Expected: FAIL to build (`undefined: New`, `undefined: shrink`).

- [ ] **Step 3: Implement `internal/covers/shrink.go`**

```go
package covers

import (
	"image"
	"image/draw"
)

// shrink scales img down so its longer side is size, averaging the source
// pixels under each destination pixel (a box filter: good enough for
// covers, and in the standard library only). Smaller images are returned as
// they are.
func shrink(img image.Image, size int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= size && h <= size {
		return img
	}
	nw, nh := size, size
	if w > h {
		nh = max(1, h*size/w)
	} else {
		nw = max(1, w*size/h)
	}
	src, ok := img.(*image.RGBA)
	if !ok || src.Bounds().Min != (image.Point{}) {
		src = image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := y*h/nh, max((y+1)*h/nh, y*h/nh+1)
		for x := 0; x < nw; x++ {
			x0, x1 := x*w/nw, max((x+1)*w/nw, x*w/nw+1)
			var sum [4]uint32
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					sum[0] += uint32(p[0])
					sum[1] += uint32(p[1])
					sum[2] += uint32(p[2])
					sum[3] += uint32(p[3])
				}
			}
			n := uint32((y1 - y0) * (x1 - x0))
			d := dst.Pix[y*dst.Stride+x*4:]
			d[0], d[1], d[2], d[3] = uint8(sum[0]/n), uint8(sum[1]/n), uint8(sum[2]/n), uint8(sum[3]/n)
		}
	}
	return dst
}
```

- [ ] **Step 4: Implement `internal/covers/covers.go`**

```go
// Package covers serves album and track pictures: the album folder's image
// or a picture embedded in its files, whole or shrunk to a thumbnail. It
// never writes to the music folders: thumbnails are kept in the app's
// cache folder.
package covers

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tordo/internal/appdir"
	"tordo/internal/audiofile"
	"tordo/internal/store"
)

// Sizes are the thumbnail sizes asked for with ?size= (pixels of the longer
// side); any other size gets the original picture.
var Sizes = []int{160, 320, 640}

// maxCandidates caps the files tried for an album: some files claim a
// picture but hold an empty one.
const maxCandidates = 3

// maxImage caps a folder image: anything larger is not a cover.
const maxImage = 32 << 20

var errNone = errors.New("sin imagen")

type Covers struct {
	Store    *store.Store
	AppDir   string
	CacheDir string // where thumbnails go; "" keeps none (browse-only mode)
	sem      chan struct{}
}

// New returns the cover server. Decoding large pictures takes memory, so at
// most a few thumbnails are made at once.
func New(st *store.Store, appDir, cacheDir string) *Covers {
	return &Covers{Store: st, AppDir: appDir, CacheDir: cacheDir, sem: make(chan struct{}, 2)}
}

// source is a place a picture can come from.
type source struct {
	path     string // absolute
	embedded bool   // a picture inside an audio file; else an image file
}

func (c *Covers) albumSources(id int64) ([]source, bool, error) {
	folderImage, ok, err := c.Store.AlbumCover(id)
	if err != nil || !ok {
		return nil, ok, err
	}
	var out []source
	if folderImage != "" {
		out = append(out, source{appdir.Abs(c.AppDir, folderImage), false})
	}
	files, err := c.Store.AlbumPictureFiles(id)
	if err != nil {
		return nil, true, err
	}
	for _, f := range files[:min(len(files), maxCandidates)] {
		out = append(out, source{appdir.Abs(c.AppDir, f), true})
	}
	return out, true, nil
}

func (c *Covers) trackSources(id int64) ([]source, bool, error) {
	f, ok, err := c.Store.TrackFile(id)
	if err != nil || !ok {
		return nil, ok, err
	}
	return []source{{appdir.Abs(c.AppDir, f.Path), true}}, true, nil
}

// ServeAlbum answers GET /api/covers/album/{id}[?size=N].
func (c *Covers) ServeAlbum(w http.ResponseWriter, r *http.Request, id int64) {
	srcs, ok, err := c.albumSources(id)
	c.serve(w, r, srcs, ok, err)
}

// ServeTrack answers GET /api/covers/track/{id}[?size=N]: the picture
// embedded in the track's own file.
func (c *Covers) ServeTrack(w http.ResponseWriter, r *http.Request, id int64) {
	srcs, ok, err := c.trackSources(id)
	c.serve(w, r, srcs, ok, err)
}

func (c *Covers) serve(w http.ResponseWriter, r *http.Request, srcs []source, ok bool, err error) {
	if err != nil {
		log.Printf("carátula: %v", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	if !ok || len(srcs) == 0 {
		http.Error(w, "sin carátula", http.StatusNotFound)
		return
	}
	size := thumbSize(r.URL.Query().Get("size"))
	tag := etag(srcs, size)
	// The URL never changes for an album: the browser revalidates it, and a
	// matching tag saves reading the picture again.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", tag)
	if r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	data, mime, err := c.picture(srcs, size, tag)
	if errors.Is(err, errNone) {
		http.Error(w, "sin carátula", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("carátula: %v", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func thumbSize(s string) int {
	n, _ := strconv.Atoi(s)
	for _, z := range Sizes {
		if n == z {
			return n
		}
	}
	return 0
}

// etag identifies what a request would get: the candidate files as they
// are now, and the size.
func etag(srcs []source, size int) string {
	h := sha1.New()
	for _, s := range srcs {
		fmt.Fprintf(h, "%s|%v", s.path, s.embedded)
		if st, err := os.Stat(s.path); err == nil {
			fmt.Fprintf(h, "|%d|%d", st.Size(), st.ModTime().UnixNano())
		}
		h.Write([]byte{0})
	}
	fmt.Fprintf(h, "size=%d", size)
	return `"` + hex.EncodeToString(h.Sum(nil))[:20] + `"`
}

// picture returns the first picture the sources give, shrunk to size (when
// it is larger), from the thumbnail cache when it is there.
func (c *Covers) picture(srcs []source, size int, tag string) ([]byte, string, error) {
	var cached string
	if size > 0 && c.CacheDir != "" {
		cached = filepath.Join(c.CacheDir, strings.Trim(tag, `"`)+".jpg")
		if b, err := os.ReadFile(cached); err == nil {
			return b, "image/jpeg", nil
		}
	}
	data, mime, err := original(srcs)
	if err != nil || size == 0 {
		return data, mime, err
	}
	c.sem <- struct{}{}
	thumb, err := shrinkJPEG(data, size)
	<-c.sem
	if err != nil {
		return data, mime, nil // a format the standard library does not decode (WebP…): send it whole
	}
	if cached != "" {
		if err := writeAtomic(cached, thumb); err != nil {
			log.Printf("miniatura: %v", err)
		}
	}
	return thumb, "image/jpeg", nil
}

func original(srcs []source) ([]byte, string, error) {
	for _, s := range srcs {
		if s.embedded {
			data, mime, err := audiofile.Picture(s.path)
			if err == nil {
				return data, mime, nil
			}
			if !errors.Is(err, audiofile.ErrNoPicture) {
				log.Printf("carátula de %s: %v", s.path, err)
			}
			continue
		}
		st, err := os.Stat(s.path)
		if err != nil || st.Size() > maxImage {
			continue
		}
		data, err := os.ReadFile(s.path)
		if err != nil {
			continue
		}
		mime := http.DetectContentType(data)
		if strings.HasPrefix(mime, "image/") {
			return data, mime, nil
		}
	}
	return nil, "", errNone
}

// shrinkJPEG decodes a picture and, when its longer side exceeds size,
// scales it down and encodes it as JPEG.
func shrinkJPEG(data []byte, size int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, shrink(img, size), &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAtomic(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".thumb-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), name)
}
```

- [ ] **Step 5: Run the tests**

Run: `go vet ./internal/covers/ && go test -race ./internal/covers/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/covers
git commit -m "feat: album and track covers with cached thumbnails"
```

---

### Task 5: API — play, stream and covers

**Files:**
- Create: `internal/server/media.go`
- Modify: `internal/server/server.go` (`Server`, `Handler`, `folder`), `cmd/tordo/main.go` (`setup`), `cmd/tordo/main_test.go`
- Test: `internal/server/media_test.go`

**Interfaces:**
- Consumes: Tasks 2–4.
- Produces: `server.MaxPlay = 5000`; `Server` gains `AppDir string`, `Streamer stream.Streamer`, `Covers *covers.Covers`; the endpoints below. The JSON of a play item: `{id, title, artist, album, albumId, durationMs, format, codec, lossless, mime, transcoded}`.

| Method and path | What it does |
|---|---|
| `GET /api/play?album=ID` | an album's tracks, in order |
| `GET /api/play?folder=PATH` | a folder's or collection's tracks, recursive, in page order |
| `GET /api/play?track=ID&track=ID…` | those tracks, in that order |
| `…&shuffle=1` | shuffled (on the server, so a 100k-track folder samples all of it before the cap) |
| `GET /stream/{id}` | the track's audio; `?transcode=1` converts it |
| `GET /api/covers/album/{id}?size=` | the album's picture or thumbnail |
| `GET /api/covers/track/{id}?size=` | the picture embedded in the track's file |
| `GET /api/folders?path=` | now also `art` (subfolder → album ids) and `trail` (`[{path, name}]` from the root) |

`/api/play` answers `{"items": [...], "skipped": n, "ffmpeg": bool}`: at most `MaxPlay` items; tracks that need ffmpeg when there is none are left out and counted, so the page can say why.

- [ ] **Step 1: Write the failing tests**

`internal/server/media_test.go`:

```go
package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/covers"
	"tordo/internal/grouping"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// mediaServer catalogs an album with an MP3 and a WMA track whose files
// exist under <tmp>/musica, served from the app folder <tmp>/tordo.
func mediaServer(t *testing.T, ffmpeg string) (*Server, []int64) {
	t.Helper()
	base := t.TempDir()
	app := filepath.Join(base, "tordo")
	files := map[string]audiofile.Info{
		"Virus - Locura/01.mp3": {Format: "mp3", Codec: "mp3", DurationMs: 1000,
			Tags: audiofile.Tags{Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", Track: 1}},
		"Virus - Locura/02.wma": {Format: "asf", Codec: "wma", DurationMs: 1000,
			Tags: audiofile.Tags{Title: "Pecados para dos", Artist: "Virus", Album: "Locura", Track: 2}},
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	var rows []store.FileRow
	for name := range files {
		p := filepath.Join(base, "musica", filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, store.FileRow{Path: "../musica/" + name, Size: 10, MTime: 1, Kind: "audio"})
	}
	roots := []string{"../musica"}
	if err := st.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	pending, _ := st.PendingReads(env)
	var rs []store.ReadResult
	for _, p := range pending {
		rs = append(rs, store.ReadResult{Target: p, Env: env, Fingerprint: p.Path, Info: files[p.Path[len("../musica/"):]]})
	}
	if err := st.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	tracks, images, _ := st.GroupingInput()
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
	items, _ := st.PlayFolder("../musica")
	ids := []int64{}
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	return &Server{Store: st, AppDir: app, Streamer: stream.Streamer{FFmpeg: ffmpeg}, Covers: covers.New(st, app, "")}, ids
}

type playResponse struct {
	Items []struct {
		store.PlayItem
		MIME       string `json:"mime"`
		Transcoded bool   `json:"transcoded"`
	} `json:"items"`
	Skipped int  `json:"skipped"`
	FFmpeg  bool `json:"ffmpeg"`
}

func TestPlayEndpoint(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	var p playResponse
	decode(t, do(t, h, "GET", "/api/play?folder=../musica", ""), &p)
	if len(p.Items) != 1 || p.Skipped != 1 || p.FFmpeg || p.Items[0].Title != "Una luna de miel en la mano" ||
		p.Items[0].MIME != "audio/mpeg" || p.Items[0].Transcoded {
		t.Fatalf("without ffmpeg: %+v", p)
	}
	s.Streamer.FFmpeg = "ffmpeg"
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?album="+itoa(onlyAlbum(t, s)), ""), &p)
	if len(p.Items) != 2 || p.Skipped != 0 || !p.FFmpeg || !p.Items[1].Transcoded || p.Items[1].MIME != "audio/mpeg" {
		t.Fatalf("with ffmpeg: %+v", p)
	}
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?track="+itoa(ids[1])+"&track="+itoa(ids[0])+"&shuffle=0", ""), &p)
	if len(p.Items) != 2 || p.Items[0].ID != ids[1] {
		t.Fatalf("tracks: %+v", p)
	}
	p = playResponse{}
	decode(t, do(t, h, "GET", "/api/play?folder=../musica&shuffle=1", ""), &p)
	if len(p.Items) != 2 {
		t.Fatalf("shuffled: %+v", p)
	}
	for _, bad := range []string{"/api/play", "/api/play?album=x", "/api/play?track=1&track=y"} {
		if rec := do(t, h, "GET", bad, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, rec.Code)
		}
	}
}

// onlyAlbum returns the id of the catalog's only album.
func onlyAlbum(t *testing.T, s *Server) int64 {
	t.Helper()
	as, err := s.Store.Albums("")
	if err != nil || len(as) != 1 {
		t.Fatalf("albums %+v, err %v", as, err)
	}
	return as[0].ID
}

func TestStreamEndpoint(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	rec := do(t, h, "GET", "/stream/"+itoa(ids[0]), "", "Range", "bytes=0-3")
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "0123" || rec.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("mp3: %d %q %v", rec.Code, rec.Body, rec.Header())
	}
	if rec := do(t, h, "GET", "/stream/"+itoa(ids[1]), ""); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("wma without ffmpeg: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/stream/9999", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown track: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/stream/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}
}

func TestCoverEndpoints(t *testing.T) {
	s, ids := mediaServer(t, "")
	h := s.Handler()
	if rec := do(t, h, "GET", "/api/covers/track/"+itoa(ids[0]), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a track without a picture: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/covers/album/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}
	s.Covers = nil
	if rec := do(t, s.Handler(), "GET", "/api/covers/album/1", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("without covers: %d", rec.Code)
	}
}

func TestFolderArt(t *testing.T) {
	s, _ := mediaServer(t, "")
	var f struct {
		store.FolderView
		Art   map[string][]int64 `json:"art"`
		Trail []store.Crumb      `json:"trail"`
	}
	decode(t, do(t, s.Handler(), "GET", "/api/folders?path=../musica", ""), &f)
	if len(f.Children) != 1 || len(f.Art["../musica/Virus - Locura"]) != 1 || len(f.Trail) != 1 || f.Trail[0].Name != "musica" {
		t.Fatalf("folder = %+v", f)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/server/`
Expected: FAIL to build (`unknown field AppDir in struct literal`…).

- [ ] **Step 3: Implement `internal/server/media.go`**

```go
package server

import (
	"math/rand/v2"
	"net/http"
	"strconv"

	"tordo/internal/appdir"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// MaxPlay caps the tracks one play request returns: the player keeps its
// queue in the browser's storage.
const MaxPlay = 5000

// playItem is a track as the player gets it: how the server will deliver it.
type playItem struct {
	store.PlayItem
	MIME       string `json:"mime"`       // what /stream/{id} sends
	Transcoded bool   `json:"transcoded"` // converted by ffmpeg: no seeking
}

func trackFormat(f store.TrackFile) stream.Format {
	return stream.Format{Container: f.Format, Codec: f.Codec, Lossless: f.Lossless}
}

// streamTrack answers GET /stream/{id}: the track's audio, by id (never by
// path). ?transcode=1 asks for a conversion of a format the browser cannot
// play.
func (s *Server) streamTrack(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	f, ok, err := s.Store.TrackFile(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "pista no encontrada", http.StatusNotFound)
		return
	}
	s.Streamer.Serve(w, r, appdir.Abs(s.AppDir, f.Path), trackFormat(f), r.URL.Query().Get("transcode") == "1")
}

// play answers GET /api/play: the tracks to queue for an album
// (?album=id), a folder or collection (?folder=path, recursive, in order)
// or some tracks (?track=id, repeatable), shuffled with ?shuffle=1. Tracks
// that cannot play (they need ffmpeg) are left out and counted.
func (s *Server) play(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var items []store.PlayItem
	var err error
	switch {
	case q.Has("album"):
		id, perr := strconv.ParseInt(q.Get("album"), 10, 64)
		if perr != nil {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}
		items, err = s.Store.PlayAlbum(id)
	case q.Has("folder"):
		items, err = s.Store.PlayFolder(q.Get("folder"))
	case q.Has("track"):
		var ids []int64
		for _, v := range q["track"] {
			id, perr := strconv.ParseInt(v, 10, 64)
			if perr != nil {
				http.Error(w, "id inválido", http.StatusBadRequest)
				return
			}
			ids = append(ids, id)
		}
		items, err = s.Store.PlayTracks(ids)
	default:
		http.Error(w, "falta qué reproducir: album, folder o track", http.StatusBadRequest)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	if q.Get("shuffle") == "1" {
		rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	}
	out := []playItem{}
	skipped := 0
	for _, it := range items {
		mode, mime := s.Streamer.Plan(stream.Format{Container: it.Format, Codec: it.Codec, Lossless: it.Lossless})
		if mode == stream.Unplayable {
			skipped++
			continue
		}
		if len(out) < MaxPlay {
			out = append(out, playItem{PlayItem: it, MIME: mime, Transcoded: mode == stream.Transcode})
		}
	}
	writeJSON(w, map[string]any{"items": out, "skipped": skipped, "ffmpeg": s.Streamer.FFmpeg != ""})
}

func (s *Server) albumCover(w http.ResponseWriter, r *http.Request) {
	s.cover(w, r, s.Covers.ServeAlbum)
}

func (s *Server) trackCover(w http.ResponseWriter, r *http.Request) {
	s.cover(w, r, s.Covers.ServeTrack)
}

func (s *Server) cover(w http.ResponseWriter, r *http.Request, serve func(http.ResponseWriter, *http.Request, int64)) {
	if s.Covers == nil {
		http.Error(w, "sin carátula", http.StatusNotFound)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	serve(w, r, id)
}
```

- [ ] **Step 4: Wire it in `internal/server/server.go`**

Replace the package comment:

```go
// Package server exposes the catalog as a JSON API, the tracks' audio and
// pictures, and the embedded web app. It only answers requests addressed to
// localhost (LAN access and its PIN come in M5).
```

Add `"tordo/internal/covers"` and `"tordo/internal/stream"` to the imports and replace the `Server` struct:

```go
type Server struct {
	Store    *store.Store
	Scanner  *scan.Scanner   // nil in browse-only mode
	AppDir   string          // where catalog paths are relative to
	Streamer stream.Streamer // FFmpeg "" when there is none
	Covers   *covers.Covers  // nil: no pictures
	ReadOnly bool
}
```

In `Handler`, after `mux.HandleFunc("GET /api/collections", s.collections)`:

```go
	mux.HandleFunc("GET /api/play", s.play)
	mux.HandleFunc("GET /api/covers/album/{id}", s.albumCover)
	mux.HandleFunc("GET /api/covers/track/{id}", s.trackCover)
	mux.HandleFunc("GET /stream/{id}", s.streamTrack)
```

In `folder`, replace the final `writeJSON(w, f)` with:

```go
	art, err := s.Store.FolderArt(f.Path)
	if err != nil {
		fail(w, err)
		return
	}
	trail, err := s.Store.FolderTrail(f.Path)
	if err != nil {
		fail(w, err)
		return
	}
	// art: up to four album ids per subfolder, for its card's picture;
	// trail: the folders from the root down to this one.
	writeJSON(w, struct {
		store.FolderView
		Art   map[string][]int64 `json:"art"`
		Trail []store.Crumb      `json:"trail"`
	}{f, art, trail})
```

- [ ] **Step 5: Wire it in `cmd/tordo/main.go`**

Add `"tordo/internal/covers"` and `"tordo/internal/stream"` to the imports. In `setup`, replace `a.server = &server.Server{Store: st, Scanner: a.scanner, ReadOnly: readOnly}` with:

```go
	thumbs := "" // browse-only mode writes nothing, thumbnails included
	if !readOnly {
		thumbs = filepath.Join(appDir, "cache", "thumbs")
	}
	ffmpeg := stream.LookFFmpeg()
	if ffmpeg == "" {
		log.Print("sin ffmpeg: WMA, APE, WavPack, AIFF y ALAC no se pueden reproducir")
	}
	a.server = &server.Server{Store: st, Scanner: a.scanner, AppDir: appDir, Streamer: stream.Streamer{FFmpeg: ffmpeg},
		Covers: covers.New(st, appDir, thumbs), ReadOnly: readOnly}
```

In `cmd/tordo/main_test.go`, after the check `if a.scanner == nil || a.server.ReadOnly { … }`:

```go
	if a.server.AppDir != app || a.server.Covers == nil || a.server.Covers.CacheDir != filepath.Join(app, "cache", "thumbs") {
		t.Fatalf("server = %+v", a.server)
	}
```

- [ ] **Step 6: Run the tests**

Run: `go vet ./... && go test ./... && go test -race ./internal/server/ ./cmd/...`
Expected: PASS. (A log line `carátula de …: formato inválido` from `TestCoverEndpoints` is expected: its MP3 is not a real one.)

- [ ] **Step 7: Commit**

```bash
git add internal/server cmd/tordo
git commit -m "feat: endpoints to play, stream and show covers"
```

---

### Task 6: Web app shell, embedded in the server

**Files:**
- Create: `internal/server/static.go`, `internal/server/dist/.gitkeep` (empty), `internal/platform/platform.go`
- Create: `web/package.json`, `web/vite.config.js`, `web/svelte.config.js`, `web/index.html`, `web/public/.gitkeep` (empty), `web/public/favicon.svg`, `web/src/main.js`, `web/src/app.css`, `web/src/App.svelte`, `web/src/lib/{api,router,nav.svelte,app.svelte,status,format}.js`, `web/src/components/{Icono,Estado,Nav}.svelte`
- Modify: `internal/server/server.go` (`Static`, route), `cmd/tordo/main.go` (`-no-browser`, open the browser), `.gitignore`, `scripts/build.sh`, `.github/workflows/ci.yml`
- Test: `internal/server/static_test.go`, `web/src/lib/{api,router,status,format}.test.js`

**Interfaces:**
- Consumes: the API of M1 and Task 5.
- Produces (Go): `Server.Static fs.FS` (nil: the embedded build); `GET /` and every path without an extension → `index.html`; `platform.Open(target string) error`.
- Produces (JS), used by every later task:
  - `api.js`: `api.status()`, `api.scan()`, `api.album(id)`, `api.folder(path)`, `api.collections()`, `api.play(what)`; `playQuery({album, folder, tracks, shuffle})`; `albumCoverURL(id, size)`, `trackCoverURL(id, size)`, `itemCoverURL(item, size)`, `streamURL(id, transcode)`; `ApiError{status, message}`.
  - `router.js`: `resolve(pathname, search)` → `{page: 'carpetas', path}` (also for `/`), `{page: 'listas'}`, `{page: 'album', id}`, `{page: 'notfound'}`; `appLink(event, anchor, origin)`; `folderHref(path)` (`/carpetas?ruta=…`), `albumHref(id)`.
  - `nav.svelte.js`: `route` (`$state {path, search}`), `navigate(url, {replace})`, `patchState(data)`, `savedScroll()`.
  - `app.svelte.js`: `app` (`$state {status, generation, notices}`), `watchStatus()`, `notify(text)`.
  - `status.js`: `busy(st)`, `progress(st)`, `summary(st)` → `{text, tone}`. It already reads M2's `identify` status when present (`running`, `offline`), so nothing changes when M2 merges.
  - `format.js`: `duration(ms)`, `length(ms)`, `count(n, one, many)`, `codecName(t)`, `quality(t)`, `folderName(path)`.
  - Components `Icono {name, size}` (Feather icons: play, pause, prev, next, shuffle, repeat, queue, volume, mute, folder, disc, lists, close, down, plus, next_up), `Estado`, `Nav {page}`.
  - CSS tokens in `app.css` (`--bg`, `--surface`, `--accent`…, `--player-h`, `--tabs-h`) and classes `.grid`, `.actions`, `.label`, `.path`, `.empty`, `button.icon`, `button.primary`.

The look is Cinexplorer's (dark, Inter, uppercase section labels) with square covers and an amber accent. On phones (≤ 640 px) the sections move to tabs at the bottom; `--tabs-h` and `--player-h` reserve room for them and for the player bar (Task 8). The app is served from `internal/server/dist/`, embedded with `//go:embed all:dist` so the `.gitkeep` that keeps the folder in git is embedded too (and never served). Vite empties that folder on every build and copies `web/public/` into it, `.gitkeep` included.

- [ ] **Step 1: Write the Go tests**

`internal/server/static_test.go`:

```go
package server

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticApp(t *testing.T) {
	s := &Server{Store: newStore(t), Static: fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>Tordo</title>")},
		"assets/index-abc1.js": {Data: []byte("console.log(1)")},
		"favicon.svg":          {Data: []byte("<svg/>")},
		".gitkeep":             {Data: []byte("")},
	}}
	h := s.Handler()
	cases := []struct {
		path, body, cache string
		code              int
	}{
		{"/", "<title>Tordo</title>", "no-cache", 200},
		{"/carpetas?ruta=../musica", "<title>Tordo</title>", "no-cache", 200},
		{"/album/12", "<title>Tordo</title>", "no-cache", 200},
		{"/assets/index-abc1.js", "console.log(1)", "public, max-age=31536000, immutable", 200},
		{"/favicon.svg", "<svg/>", "", 200},
		{"/assets/missing.js", "", "", http.StatusNotFound},
		{"/.gitkeep", "", "", http.StatusNotFound},
		{"/api/nada", "", "", http.StatusNotFound},
		{"/stream/x", "", "", http.StatusBadRequest}, // the stream route answers, not the app
	}
	for _, c := range cases {
		rec := do(t, h, "GET", c.path, "")
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.body) || rec.Header().Get("Cache-Control") != c.cache {
			t.Errorf("%s: %d %q (Cache-Control %q), want %d %q (%q)", c.path, rec.Code, rec.Body, rec.Header().Get("Cache-Control"),
				c.code, c.body, c.cache)
		}
	}
}

func TestStaticWithoutBuild(t *testing.T) {
	s := &Server{Store: newStore(t), Static: fstest.MapFS{".gitkeep": {Data: []byte("")}}}
	rec := do(t, s.Handler(), "GET", "/carpetas", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Falta la interfaz") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestEmbeddedApp(t *testing.T) {
	// Whether or not the web app was built, the embedded files answer a page.
	rec := do(t, (&Server{Store: newStore(t)}).Handler(), "GET", "/", "")
	if rec.Code != 200 || !strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html>") {
		t.Fatalf("status %d: %.200s", rec.Code, rec.Body)
	}
}
```

Run: `go test ./internal/server/ -run Static`
Expected: FAIL to build (`unknown field Static in struct literal`).

- [ ] **Step 2: Serve the web app**

Create the empty file `internal/server/dist/.gitkeep`, then `internal/server/static.go`:

```go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist is the web app's build (web/ compiled by Vite, `npm run build`). It
// is not committed: the folder only holds a .gitkeep until it is built.
//
//go:embed all:dist
var dist embed.FS

// notBuilt is what the app shows when the binary was compiled without the
// web app.
const notBuilt = `<!doctype html><html lang="es"><meta charset="utf-8"><title>Tordo</title>
<body style="font-family:system-ui;background:#15171b;color:#dde;padding:48px">
<h1>Falta la interfaz</h1><p>Este ejecutable se compiló sin la interfaz web. Compilala con
<code>cd web &amp;&amp; npm ci &amp;&amp; npm run build</code> y volvé a compilar tordo
(<code>scripts/build.sh</code> hace las dos cosas).</p></body></html>`

// webApp returns the files of the web app: s.Static, or the embedded build.
func (s *Server) webApp() fs.FS {
	if s.Static != nil {
		return s.Static
	}
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// static serves the web app. Files of the build are served as they are
// (those under assets/ carry a content hash in their name, so browsers may
// keep them for good); any other path without an extension is one of the
// app's routes (/carpetas, /album/12…) and gets index.html, which the app's
// router resolves.
func (s *Server) static() http.Handler {
	app := s.webApp()
	files := http.FileServerFS(app)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(p, "api/") || strings.HasPrefix(p, "stream/") || strings.Contains("/"+p, "/.") {
			http.NotFound(w, r)
			return
		}
		if p != "" && p != "index.html" {
			if st, err := fs.Stat(app, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
			if path.Ext(p) != "" {
				http.NotFound(w, r) // a missing file, not a route
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		if _, err := fs.Stat(app, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(notBuilt))
			return
		}
		http.ServeFileFS(w, r, app, "index.html")
	})
}
```

In `internal/server/server.go`: add `"io/fs"` to the imports, the field `Static   fs.FS           // the web app; nil: the embedded build` to `Server` (after `Covers`), and as the **last** route in `Handler`:

```go
	mux.Handle("GET /", s.static())
```

Run: `go vet ./internal/server/ && go test ./internal/server/`
Expected: PASS.

- [ ] **Step 3: Open the browser at start**

`internal/platform/platform.go`:

```go
// Package platform opens URLs with the system's default application.
package platform

import (
	"os/exec"
	"runtime"
)

// Open opens target (a URL, file or folder) with its default application,
// without waiting for it.
func Open(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
```

In `cmd/tordo/main.go`: the package comment becomes

```go
// Command tordo catalogs a music library and serves it, to browse and
// listen, on localhost: it opens the browser at start.
```

add `"tordo/internal/platform"` to the imports; in `main`, add the flag and pass it on:

```go
	noBrowser := flag.Bool("no-browser", false, "no abrir el navegador")
	flag.Parse()
	if err := run(*dir, *port, !*noBrowser); err != nil {
```

`run` becomes `func run(dirOverride string, port int, browser bool) error`, and its last lines:

```go
	url := "http://" + ln.Addr().String() + "/"
	log.Printf("Tordo en %s (Ctrl+C para salir)", url)
	if browser {
		if err := platform.Open(url); err != nil {
			log.Printf("no se pudo abrir el navegador: %v", err)
		}
	}
	return http.Serve(ln, a.server.Handler())
```

Run: `go vet ./... && GOOS=windows go vet ./internal/platform ./cmd/... && go test ./cmd/...`
Expected: PASS.

- [ ] **Step 4: Scaffold `web/`**

`web/package.json`:

```json
{
  "name": "tordo-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^7.3.1",
    "svelte": "^5.57.1",
    "vite": "^8.3.1",
    "vitest": "^5.0.2"
  },
  "dependencies": {
    "@fontsource-variable/inter": "^5.3.0"
  }
}
```

`web/vite.config.js`:

```js
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// The build goes where the Go server embeds it (public/.gitkeep keeps the
// folder in git). In development, `npm run dev` serves the app and sends
// /api and /stream to the Go server, started with:
// go run ./cmd/tordo -dir .run -port 4747 -no-browser
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/server/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:4747',
      '/stream': 'http://127.0.0.1:4747',
    },
  },
  test: {
    include: ['src/**/*.test.js'],
  },
})
```

`web/svelte.config.js`:

```js
// Svelte 5 with the defaults; the file only exists so that the Vite plugin
// does not warn about its absence.
export default {}
```

`web/index.html`:

```html
<!doctype html>
<html lang="es">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
    <meta name="theme-color" content="#15171b" />
    <title>Tordo</title>
    <link rel="icon" href="/favicon.svg" />
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.js"></script>
  </body>
</html>
```

`web/public/favicon.svg` (and an empty `web/public/.gitkeep`):

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="6" fill="#15171b"/><circle cx="16" cy="16" r="10.5" fill="none" stroke="#f0a830" stroke-width="2.5"/><circle cx="16" cy="16" r="3" fill="#f0a830"/></svg>
```

`web/src/main.js`:

```js
import '@fontsource-variable/inter'
import './app.css'
import { mount } from 'svelte'
import App from './App.svelte'

mount(App, { target: document.getElementById('app') })
```

`web/src/app.css`:

```css
:root {
  --bg: #15171b;
  --surface: #1c1f24;
  --surface-2: #24282e;
  --line: #2b3037;
  --line-strong: #3b424b;
  --text: #d9dde2;
  --strong: #ffffff;
  --muted: #9ba3ac;
  --faint: #6d7680;
  --accent: #f0a830;
  --on-accent: #1a1204;
  --warn: #e0a040;
  --radius: 4px;
  --gutter: 16px;
  --player-h: 72px;
  --tabs-h: 0px; /* the bottom tabs, on phones */
  color-scheme: dark;
  font-family: 'Inter Variable', system-ui, sans-serif;
  font-size: 15px;
  line-height: 1.45;
}

@media (max-width: 640px) {
  :root {
    --player-h: 60px;
    --tabs-h: 56px;
  }
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  padding-bottom: calc(var(--player-h) + var(--tabs-h) + env(safe-area-inset-bottom));
}

a {
  color: inherit;
  text-decoration: none;
}

a:hover {
  color: var(--strong);
}

button,
input,
select {
  font: inherit;
  color: inherit;
}

button {
  cursor: pointer;
  background: transparent;
  border: 1px solid var(--line-strong);
  border-radius: var(--radius);
  padding: 4px 12px;
}

button:hover:not(:disabled) {
  border-color: var(--muted);
}

button:disabled {
  opacity: 0.5;
  cursor: default;
}

button.primary {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--on-accent);
  font-weight: 600;
}

/* A button that is only an icon. */
button.icon {
  border: none;
  padding: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: var(--muted);
}

button.icon:hover:not(:disabled) {
  color: var(--strong);
}

button.icon.on {
  color: var(--accent);
}

:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

main {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 var(--gutter) 48px;
}

h1 {
  font-size: 26px;
  line-height: 1.2;
  color: var(--strong);
  margin: 0 0 4px;
}

h2 {
  font-size: 13px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--faint);
  font-weight: 600;
  margin: 32px 0 12px;
}

.label {
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--faint);
}

.path {
  font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
  font-size: 12px;
  color: var(--faint);
  word-break: break-all;
}

.empty {
  color: var(--muted);
  padding: 48px 0;
  text-align: center;
}

/* A grid of square cards: covers first. */
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 20px 16px;
}

@media (max-width: 640px) {
  .grid {
    grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
    gap: 16px 12px;
  }
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0;
}
```

Run: `cd web && npm install` (creates `package-lock.json`; commit it).

- [ ] **Step 5: Write the JS tests**

`web/src/lib/router.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { albumHref, appLink, folderHref, resolve } from './router.js'

describe('resolve', () => {
  it.each([
    ['/', '', { page: 'carpetas', path: '' }],
    ['/carpetas', '', { page: 'carpetas', path: '' }],
    ['/carpetas', '?ruta=..%2Fmusica%2FLatinoam%C3%A9rica%20600', { page: 'carpetas', path: '../musica/Latinoamérica 600' }],
    ['/listas', '', { page: 'listas' }],
    ['/album/12', '', { page: 'album', id: 12 }],
    ['/album/0', '', { page: 'notfound' }],
    ['/album/abc', '', { page: 'notfound' }],
    ['/otra', '', { page: 'notfound' }],
  ])('%s%s', (path, search, want) => {
    expect(resolve(path, search)).toEqual(want)
  })
})

describe('hrefs', () => {
  it('round-trips folder paths', () => {
    const p = '../musica/[LA-10] Soda Stereo - Signos & más'
    const url = new URL(folderHref(p), 'http://127.0.0.1')
    expect(resolve(url.pathname, url.search)).toEqual({ page: 'carpetas', path: p })
    expect(folderHref('')).toBe('/carpetas')
    expect(albumHref(7)).toBe('/album/7')
  })
})

function anchor(href, attrs = {}) {
  return { href, target: attrs.target ?? '', hasAttribute: (name) => name in attrs }
}

const click = { button: 0, defaultPrevented: false }
const origin = 'http://127.0.0.1:4747'

describe('appLink', () => {
  it('keeps links to the app inside the app', () => {
    expect(appLink(click, anchor('http://127.0.0.1:4747/carpetas?ruta=x'), origin)).toBe('/carpetas?ruta=x')
    expect(appLink(click, anchor('/album/1'), origin)).toBe('/album/1')
  })
  it('leaves the rest to the browser', () => {
    expect(appLink(click, null, origin)).toBeNull()
    expect(appLink(click, anchor('https://musicbrainz.org/'), origin)).toBeNull()
    expect(appLink(click, anchor('/api/covers/album/1'), origin)).toBeNull()
    expect(appLink(click, anchor('/stream/1'), origin)).toBeNull()
    expect(appLink(click, anchor('/carpetas', { target: '_blank' }), origin)).toBeNull()
    expect(appLink(click, anchor('/carpetas', { download: '' }), origin)).toBeNull()
    expect(appLink({ ...click, ctrlKey: true }, anchor('/carpetas'), origin)).toBeNull()
    expect(appLink({ ...click, button: 1 }, anchor('/carpetas'), origin)).toBeNull()
    expect(appLink({ ...click, defaultPrevented: true }, anchor('/carpetas'), origin)).toBeNull()
  })
})
```

`web/src/lib/api.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { albumCoverURL, itemCoverURL, playQuery, streamURL } from './api.js'

describe('playQuery', () => {
  it('writes what to play', () => {
    expect(playQuery({ album: 7 })).toBe('album=7')
    expect(playQuery({ folder: '../musica/Brasil', shuffle: true })).toBe('folder=..%2Fmusica%2FBrasil&shuffle=1')
    expect(playQuery({ tracks: [3, 1] })).toBe('track=3&track=1')
  })
})

describe('urls', () => {
  it('points at covers and audio', () => {
    expect(albumCoverURL(5, 320)).toBe('/api/covers/album/5?size=320')
    expect(albumCoverURL(5)).toBe('/api/covers/album/5')
    expect(itemCoverURL({ id: 9, albumId: 5 }, 160)).toBe('/api/covers/album/5?size=160')
    expect(itemCoverURL({ id: 9, albumId: 0 }, 160)).toBe('/api/covers/track/9?size=160')
    expect(streamURL(9)).toBe('/stream/9')
    expect(streamURL(9, true)).toBe('/stream/9?transcode=1')
  })
})
```

`web/src/lib/status.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { busy, progress, summary } from './status.js'

describe('summary', () => {
  it.each([
    [null, { text: '', tone: '' }],
    [{ scan: { running: true, files: 120 } }, { text: 'Escaneando · 120 archivos', tone: 'busy' }],
    [{ scan: { running: true, files: 120, read: 30, toRead: 90 } }, { text: 'Escaneando · 30/90 leídos', tone: 'busy' }],
    [{ scan: {}, identify: { state: 'running', identified: 3, toIdentify: 10 } }, { text: 'Identificando · 3/10', tone: 'busy' }],
    [
      { scan: {}, identify: { state: 'running', identified: 10, toIdentify: 10, enriched: 2, toEnrich: 5 } },
      { text: 'Completando datos · 2/5', tone: 'busy' },
    ],
    [{ readOnly: true, scan: {} }, { text: 'Modo consulta', tone: 'warn' }],
    [{ scan: { lastError: 'disco desconectado' } }, { text: 'El último escaneo falló', tone: 'warn' }],
    [{ scan: {}, identify: { state: 'offline' } }, { text: 'Sin conexión con MusicBrainz', tone: 'warn' }],
    [{ scan: {} }, { text: 'Al día', tone: '' }],
  ])('%j', (st, want) => {
    expect(summary(st)).toEqual(want)
  })
})

describe('busy and progress', () => {
  it('follows scans and identification runs', () => {
    expect(busy({ scan: { running: true } })).toBe(true)
    expect(busy({ scan: {}, identify: { state: 'running' } })).toBe(true)
    expect(busy({ scan: {}, identify: { state: 'idle' } })).toBe(false)
    expect(progress({ scan: { read: 3, albums: 1 } })).toBe('3|1|0|0')
  })
})
```

`web/src/lib/format.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { codecName, count, duration, folderName, length, quality } from './format.js'

describe('durations', () => {
  it('reads track lengths and totals', () => {
    expect(duration(187000)).toBe('3:07')
    expect(duration(3723000)).toBe('1:02:03')
    expect(duration(0)).toBe('0:00')
    expect(duration(undefined)).toBe('0:00')
    expect(length(48 * 60000)).toBe('48 min')
    expect(length(72 * 60000)).toBe('1 h 12 min')
    expect(length(120 * 60000)).toBe('2 h')
  })
})

describe('count', () => {
  it('agrees in number', () => {
    expect(count(1, 'pista', 'pistas')).toBe('1 pista')
    expect(count(12, 'pista', 'pistas')).toBe('12 pistas')
    expect(count(0, 'álbum', 'álbumes')).toBe('0 álbumes')
  })
})

describe('quality', () => {
  it.each([
    [{ format: 'flac', codec: 'flac', lossless: true, bitDepth: 24, sampleRate: 96000 }, 'FLAC 24/96'],
    [{ format: 'wav', codec: 'pcm', lossless: true, bitDepth: 16, sampleRate: 44100 }, 'WAV 16/44,1'],
    [{ format: 'mp3', codec: 'mp3', bitrate: 320 }, 'MP3 320'],
    [{ format: 'asf', codec: 'wma', bitrate: 128 }, 'WMA 128'],
    [{ format: 'mp4', codec: 'alac', lossless: true }, 'ALAC'],
    [{ format: 'ogg', codec: 'opus', bitrate: 0 }, 'Opus'],
    [{ format: 'ape', codec: 'ape', lossless: true }, 'APE'],
  ])('%j', (t, want) => {
    expect(quality(t)).toBe(want)
  })
  it('names unknown codecs as they come', () => {
    expect(codecName({ format: 'wv', codec: 'wv2' })).toBe('WV2')
  })
})

describe('folderName', () => {
  it('takes the last part', () => {
    expect(folderName('../musica/Latinoamérica 600')).toBe('Latinoamérica 600')
    expect(folderName('../../../Music')).toBe('Music')
    expect(folderName('')).toBe('')
  })
})
```

Run: `cd web && npx vitest run`
Expected: FAIL (`Failed to resolve import "./router.js"`…).

- [ ] **Step 6: Implement the libraries**

`web/src/lib/router.js`:

```js
// Routes of the app. Pure: nav.svelte.js keeps the current route.

// resolve maps a pathname and query to the page to show and its parameters.
export function resolve(pathname, search = '') {
  const params = new URLSearchParams(search)
  if (pathname === '/' || pathname === '/carpetas') return { page: 'carpetas', path: params.get('ruta') ?? '' }
  if (pathname === '/listas') return { page: 'listas' }
  const m = pathname.match(/^\/album\/([1-9][0-9]*)$/)
  if (m) return { page: 'album', id: Number(m[1]) }
  return { page: 'notfound' }
}

// appLink returns the path + query a click on a link should navigate to
// inside the app, or null when the browser should handle it (another site,
// the API or audio, a new tab, a download, modifier keys).
export function appLink(event, anchor, origin) {
  if (!anchor || event.defaultPrevented || event.button !== 0) return null
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return null
  if (anchor.target || anchor.hasAttribute('download')) return null
  const url = new URL(anchor.href, origin)
  if (url.origin !== origin) return null
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/stream/')) return null
  return url.pathname + url.search
}

export function folderHref(path) {
  return path ? `/carpetas?ruta=${encodeURIComponent(path)}` : '/carpetas'
}

export function albumHref(id) {
  return `/album/${id}`
}
```

`web/src/lib/api.js`:

```js
// Calls to the Go server. Errors carry the HTTP status and the server's text.

export class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
}

async function call(method, path, body) {
  const init = { method, headers: {} }
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  let res
  try {
    res = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'No se pudo contactar a Tordo. ¿Sigue abierto?')
  }
  if (!res.ok) {
    const text = (await res.text()).trim()
    throw new ApiError(res.status, text || `Error ${res.status}`)
  }
  if (res.status === 204 || res.status === 202) return null
  return res.json()
}

const get = (path) => call('GET', path)
const post = (path, body) => call('POST', path, body ?? {})

// playQuery writes what to play as /api/play's query: {album}, {folder},
// {tracks: [ids]}, plus shuffle.
export function playQuery({ album, folder, tracks, shuffle }) {
  const params = new URLSearchParams()
  if (album != null) params.set('album', album)
  if (folder != null) params.set('folder', folder)
  for (const id of tracks ?? []) params.append('track', id)
  if (shuffle) params.set('shuffle', '1')
  return params.toString()
}

export const api = {
  status: () => get('/api/status'),
  scan: () => post('/api/scan'),
  album: (id) => get(`/api/albums/${id}`),
  folder: (path) => get(path ? `/api/folders?${new URLSearchParams({ path })}` : '/api/folders'),
  collections: () => get('/api/collections'),
  play: (what) => get(`/api/play?${playQuery(what)}`),
}

// Picture URLs. size is one of the server's thumbnail sizes (160, 320,
// 640); without it, the original picture.
export function albumCoverURL(id, size) {
  return `/api/covers/album/${id}` + (size ? `?size=${size}` : '')
}

export function trackCoverURL(id, size) {
  return `/api/covers/track/${id}` + (size ? `?size=${size}` : '')
}

// itemCoverURL is the picture of a queued track: its album's, or its own
// when it is a loose track.
export function itemCoverURL(item, size) {
  return item.albumId ? albumCoverURL(item.albumId, size) : trackCoverURL(item.id, size)
}

export function streamURL(id, transcode = false) {
  return `/stream/${id}` + (transcode ? '?transcode=1' : '')
}
```

`web/src/lib/status.js`:

```js
// Reading /api/status: {readOnly, scan, identify (from M2), counts}.

// busy reports whether a scan or an identification run is going on.
export function busy(st) {
  return !!st && (!!st.scan?.running || st.identify?.state === 'running')
}

// progress sums up what a run has changed so far: when it moves, the pages
// have new data to show.
export function progress(st) {
  if (!st) return ''
  return [st.scan?.read ?? 0, st.scan?.albums ?? 0, st.identify?.identified ?? 0, st.identify?.enriched ?? 0].join('|')
}

// summary is the short text of the status indicator, and its tone:
// "busy", "warn" or "" (all quiet).
export function summary(st) {
  if (!st) return { text: '', tone: '' }
  if (st.scan?.running) {
    const { files = 0, read = 0, toRead = 0 } = st.scan
    const detail = toRead > 0 ? `${read}/${toRead} leídos` : `${files} archivos`
    return { text: `Escaneando · ${detail}`, tone: 'busy' }
  }
  const id = st.identify
  if (id?.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      return { text: `Completando datos · ${id.enriched}/${id.toEnrich}`, tone: 'busy' }
    }
    return { text: `Identificando · ${id.identified}/${id.toIdentify}`, tone: 'busy' }
  }
  if (st.readOnly) return { text: 'Modo consulta', tone: 'warn' }
  if (st.scan?.lastError) return { text: 'El último escaneo falló', tone: 'warn' }
  if (id?.state === 'offline') return { text: 'Sin conexión con MusicBrainz', tone: 'warn' }
  return { text: 'Al día', tone: '' }
}
```

`web/src/lib/format.js`:

```js
// How numbers, durations and formats read in the app (Spanish).

// duration is a track's length: "3:07", "1:02:03".
export function duration(ms) {
  const s = Math.max(0, Math.round((ms ?? 0) / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = String(s % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

// length is a long total: "48 min", "1 h 12 min".
export function length(ms) {
  const min = Math.round((ms ?? 0) / 60000)
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  return min % 60 ? `${h} h ${min % 60} min` : `${h} h`
}

// count reads "1 pista", "12 pistas" (thousands with a dot: "1.234 pistas").
export function count(n, one, many) {
  return `${n.toLocaleString('es')} ${n === 1 ? one : many}`
}

const CODECS = {
  mp3: 'MP3',
  mp2: 'MP2',
  aac: 'AAC',
  alac: 'ALAC',
  flac: 'FLAC',
  opus: 'Opus',
  vorbis: 'Vorbis',
  wma: 'WMA',
  wmav1: 'WMA',
  wmapro: 'WMA Pro',
  wmalossless: 'WMA Lossless',
  ape: 'APE',
  wavpack: 'WavPack',
}

// codecName names a track's codec ("pcm" by its container: WAV, AIFF).
export function codecName(t) {
  if (t.codec?.startsWith('pcm')) return (t.format || 'PCM').toUpperCase()
  return CODECS[t.codec] ?? (t.codec || t.format || '').toUpperCase()
}

// quality is a track's format as it reads in lists: "FLAC 24/96",
// "MP3 320", "WAV 16/44,1".
export function quality(t) {
  const name = codecName(t)
  if (t.lossless) {
    if (!t.bitDepth || !t.sampleRate) return name
    const khz = (t.sampleRate / 1000).toLocaleString('es', { maximumFractionDigits: 1 })
    return `${name} ${t.bitDepth}/${khz}`
  }
  return t.bitrate ? `${name} ${t.bitrate}` : name
}

// folderName is the last part of a catalog path.
export function folderName(path) {
  return (path ?? '').split('/').filter((p) => p && p !== '..').pop() ?? ''
}
```

`web/src/lib/nav.svelte.js`:

```js
// The current route, kept in sync with the address bar.

export const route = $state({ path: location.pathname, search: location.search })

// patchState merges data into the current history entry (the scroll
// position…), so going back restores it.
export function patchState(data) {
  history.replaceState({ ...history.state, ...data }, '')
}

// navigate goes to a path inside the app. replace rewrites the current
// entry instead of adding one.
export function navigate(url, { replace = false } = {}) {
  if (replace) {
    history.replaceState(history.state, '', url)
  } else {
    patchState({ scroll: window.scrollY })
    history.pushState({}, '', url)
    window.scrollTo(0, 0)
  }
  route.path = location.pathname
  route.search = location.search
}

// savedScroll is the scroll position to restore on this entry (0 for a new
// one).
export function savedScroll() {
  return history.state?.scroll ?? 0
}

history.scrollRestoration = 'manual'
window.addEventListener('popstate', () => {
  route.path = location.pathname
  route.search = location.search
})
```

`web/src/lib/app.svelte.js`:

```js
// State shared by the whole app: the server's status and the notices.
import { api } from './api.js'
import { busy, progress } from './status.js'

export const app = $state({
  status: null,
  // generation grows each time a scan or an identification run ends, and
  // every 20 s while a long one makes progress: pages read it to load their
  // data again.
  generation: 0,
  notices: [],
})

let timer
let wasBusy = false
let lastProgress = ''
let lastRefresh = Date.now() // pages load their data when they open

async function poll() {
  clearTimeout(timer)
  try {
    const st = await api.status()
    const now = busy(st)
    const moved = progress(st) !== lastProgress
    if ((wasBusy && !now) || (now && moved && Date.now() - lastRefresh > 20000)) {
      app.generation++
      lastRefresh = Date.now()
    }
    wasBusy = now
    lastProgress = progress(st)
    app.status = st
  } catch {
    // The next poll tries again.
  }
  timer = setTimeout(poll, wasBusy ? 3000 : 30000)
}

// watchStatus starts polling /api/status: every 3 s while something runs,
// every 30 s otherwise.
export function watchStatus() {
  poll()
}

let seq = 0

// notify shows a notice for 6 seconds.
export function notify(text) {
  const id = ++seq
  app.notices.push({ id, text })
  setTimeout(() => {
    app.notices = app.notices.filter((n) => n.id !== id)
  }, 6000)
}
```

Run: `cd web && npx vitest run`
Expected: PASS (34 tests).

- [ ] **Step 7: The shell components**

`web/src/components/Icono.svelte`:

```svelte
<script>
  // Icons from Feather (MIT), drawn with the current text color.
  const PATHS = {
    play: '<polygon points="6 4 20 12 6 20 6 4" fill="currentColor"/>',
    pause: '<rect x="6" y="4" width="4" height="16" fill="currentColor"/><rect x="14" y="4" width="4" height="16" fill="currentColor"/>',
    prev: '<polygon points="19 20 9 12 19 4 19 20" fill="currentColor"/><line x1="5" y1="19" x2="5" y2="5"/>',
    next: '<polygon points="5 4 15 12 5 20 5 4" fill="currentColor"/><line x1="19" y1="5" x2="19" y2="19"/>',
    shuffle:
      '<polyline points="16 3 21 3 21 8"/><line x1="4" y1="20" x2="21" y2="3"/><polyline points="21 16 21 21 16 21"/><line x1="15" y1="15" x2="21" y2="21"/><line x1="4" y1="4" x2="9" y2="9"/>',
    repeat:
      '<polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/>',
    queue:
      '<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>',
    volume: '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"/>',
    mute: '<polygon points="11 5 6 9 2 9 2 15 6 15 11 19 11 5"/><line x1="23" y1="9" x2="17" y2="15"/><line x1="17" y1="9" x2="23" y2="15"/>',
    folder: '<path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>',
    disc: '<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/>',
    lists:
      '<rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/>',
    close: '<line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>',
    down: '<polyline points="6 9 12 15 18 9"/>',
    plus: '<line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>',
    next_up: '<polyline points="13 17 18 12 13 7"/><polyline points="6 17 11 12 6 7"/>',
  }

  let { name, size = 18 } = $props()
</script>

<svg
  viewBox="0 0 24 24"
  width={size}
  height={size}
  aria-hidden="true"
  fill="none"
  stroke="currentColor"
  stroke-width="2"
  stroke-linecap="round"
  stroke-linejoin="round">{@html PATHS[name] ?? ''}</svg
>
```

`web/src/components/Estado.svelte`:

```svelte
<script>
  import { app } from '../lib/app.svelte.js'
  import { summary } from '../lib/status.js'

  const s = $derived(summary(app.status))
</script>

{#if s.text}
  <span class="estado {s.tone}" title={app.status?.scan?.lastError || s.text} role="status">
    <span class="dot"></span>
    <span class="text">{s.text}</span>
  </span>
{/if}

<style>
  .estado {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--faint);
    white-space: nowrap;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--faint);
  }
  .busy .dot {
    background: var(--accent);
    animation: pulse 1.2s ease-in-out infinite;
  }
  .warn {
    color: var(--warn);
  }
  .warn .dot {
    background: var(--warn);
  }
  @keyframes pulse {
    50% {
      opacity: 0.3;
    }
  }
  @media (max-width: 640px) {
    .text {
      display: none; /* the dot only; the title has the text */
    }
  }
</style>
```

`web/src/components/Nav.svelte`:

```svelte
<script>
  import Estado from './Estado.svelte'
  import Icono from './Icono.svelte'

  let { page } = $props()

  // The sections of the app; Inicio, Explorar and Revisar come with later
  // milestones.
  const SECTIONS = [
    { page: 'carpetas', href: '/carpetas', label: 'Carpetas', icon: 'folder' },
    { page: 'listas', href: '/listas', label: 'Listas', icon: 'lists' },
  ]
</script>

<header>
  <nav>
    <a class="brand" href="/">
      <svg viewBox="0 0 32 32" width="20" height="20" aria-hidden="true">
        <circle cx="16" cy="16" r="10.5" fill="none" stroke="currentColor" stroke-width="2.5" />
        <circle cx="16" cy="16" r="3" fill="currentColor" />
      </svg>
      Tordo
    </a>
    <span class="sections">
      {#each SECTIONS as s (s.page)}
        <a href={s.href} class:active={page === s.page}>{s.label}</a>
      {/each}
    </span>
    <span class="spacer"></span>
    <Estado />
  </nav>
</header>

<!-- On phones the sections go to tabs at the bottom, under the player. -->
<nav class="tabs" aria-label="Secciones">
  {#each SECTIONS as s (s.page)}
    <a href={s.href} class:active={page === s.page}>
      <Icono name={s.icon} size={20} />
      <span>{s.label}</span>
    </a>
  {/each}
</nav>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 20;
    background: rgba(21, 23, 27, 0.94);
    backdrop-filter: blur(6px);
    border-bottom: 1px solid var(--line);
    margin-bottom: 16px;
  }
  header nav {
    max-width: 1400px;
    margin: 0 auto;
    padding: 12px var(--gutter);
    display: flex;
    align-items: center;
    gap: 20px;
  }
  .sections {
    display: flex;
    gap: 20px;
    font-size: 13px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-right: 20px;
    border-right: 1px solid var(--line-strong);
    color: var(--strong);
    font-size: 18px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }
  .brand svg {
    color: var(--accent);
  }
  .active {
    color: var(--strong);
  }
  .spacer {
    flex: 1;
  }
  .tabs {
    display: none;
  }
  @media (max-width: 640px) {
    .sections {
      display: none;
    }
    .brand {
      border-right: none;
    }
    .tabs {
      display: flex;
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 25;
      height: calc(var(--tabs-h) + env(safe-area-inset-bottom));
      padding-bottom: env(safe-area-inset-bottom);
      background: var(--surface);
      border-top: 1px solid var(--line);
    }
    .tabs a {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 2px;
      font-size: 11px;
      color: var(--faint);
    }
    .tabs a.active {
      color: var(--accent);
    }
  }
</style>
```

`web/src/App.svelte` (the pages arrive in Tasks 9–11; until then every section shows a placeholder):

```svelte
<script>
  import Nav from './components/Nav.svelte'
  import { app, watchStatus } from './lib/app.svelte.js'
  import { navigate, route } from './lib/nav.svelte.js'
  import { appLink, resolve } from './lib/router.js'

  const current = $derived(resolve(route.path, route.search))

  watchStatus()

  // Links inside the app change the route instead of reloading the page,
  // so the music keeps playing.
  function onclick(event) {
    const to = appLink(event, event.target.closest?.('a'), location.origin)
    if (to !== null) {
      event.preventDefault()
      navigate(to)
    }
  }
</script>

<svelte:window {onclick} />

<Nav page={current.page} />

<main>
  {#if current.page === 'notfound'}
    <p class="empty">No existe esta página. <a href="/carpetas">Ir a Carpetas</a></p>
  {:else}
    <p class="empty">Esta sección llega en las próximas tareas.</p>
  {/if}
</main>

<div class="notices" aria-live="polite">
  {#each app.notices as n (n.id)}
    <div class="notice">{n.text}</div>
  {/each}
</div>

<style>
  .notices {
    position: fixed;
    top: 64px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 40;
    width: min(560px, calc(100% - 32px));
  }
  .notice {
    background: var(--surface-2);
    border: 1px solid var(--warn);
    border-radius: var(--radius);
    padding: 8px 12px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
</style>
```

- [ ] **Step 8: Build, ignore the build, and build it in CI and releases**

Append to `.gitignore`:

```
node_modules/
# The web app's build is embedded by the Go server but not committed.
/internal/server/dist/*
!/internal/server/dist/.gitkeep
```

In `scripts/build.sh`, the comment and the first lines become:

```bash
# Builds the web app and cross-compiles Tordo for Windows, macOS (universal)
# and Linux (amd64, arm64) into dist/tordo/.
set -euo pipefail
cd "$(dirname "$0")/.."
(cd web && npm ci && npm test && npm run build)
```

Replace `.github/workflows/ci.yml`:

```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test ./...
  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 24
          cache: npm
          cache-dependency-path: web/package-lock.json
      - run: npm ci
      - run: npm test
      - run: npm run build
```

Run: `cd web && npm run build && cd .. && ls -a internal/server/dist && git status --short internal/server/dist`
Expected: the build lists `.gitkeep`, `assets/`, `favicon.svg`, `index.html`; git shows nothing to commit under `dist/` (only `.gitkeep` is tracked, and it is unchanged).

Run: `go test ./... && go run ./cmd/tordo -dir .run` and look at the browser it opens: the header with *Tordo · Carpetas · Listas* and the status on the right («Escaneando · …», then «Al día»); at 375 px wide, the tabs at the bottom. Stop it with Ctrl+C.

- [ ] **Step 9: Commit**

```bash
git add .gitignore .github scripts internal/server internal/platform cmd/tordo web
git commit -m "feat: web app shell, embedded in the server, and the browser opens at start"
```

---

### Task 7: The play queue

**Files:**
- Create: `web/src/lib/queue.js`
- Test: `web/src/lib/queue.test.js`

**Interfaces:**
- Produces: `queue.js` — pure functions over a plain, JSON-serializable state `{list, order, pos, shuffle, repeat, nextKey}`: `MAX` (5000, as the server's `MaxPlay`), `REPEAT` (`['off', 'all', 'one']`), `empty()`, `replace(q, items, start, rand)`, `append(q, items)`, `playNext(q, items)`, `current(q)`, `next(q, auto)` (null at the end), `peek(q)` (what plays when the current entry ends), `prev(q)`, `jump(q, pos)`, `remove(q, pos)`, `clearUpcoming(q)`, `setShuffle(q, on, rand)`, `cycleRepeat(q)`, `upcoming(q)` (`[{entry, pos}]`), `restore(json)`.

Every queued item becomes an entry with its own `key`, so a track can be queued twice. `list` keeps the entries as added (the order without shuffle), `order` lists their keys in play order, `pos` is the current entry's place in `order`. Shuffle reorders only what is still to play, and turning it off goes back to the list's order from the current entry. "Play next" goes right after the current entry, in the list too, so it stays next after turning shuffle off. `next(q, true)` (the track ended by itself) honors repeat-one; a user's "next" does not. `restore` survives anything in `localStorage` (missing, broken, from an older version).

- [ ] **Step 1: Write the failing tests**

`web/src/lib/queue.test.js`:

```js
import { describe, expect, it } from 'vitest'
import * as Q from './queue.js'

const items = (...ids) => ids.map((id) => ({ id, title: `t${id}` }))
const ids = (q) => q.order.map((k) => q.list.find((e) => e.key === k).id)
const cur = (q) => Q.current(q)?.id ?? null
// A rand that always returns 0: a fixed, repeatable shuffle.
const first = () => 0

describe('replace', () => {
  it('starts a new queue at a track', () => {
    const q = Q.replace(Q.empty(), items(1, 2, 3), 1)
    expect(ids(q)).toEqual([1, 2, 3])
    expect(cur(q)).toBe(2)
  })
  it('with shuffle, plays the chosen track first and shuffles the rest', () => {
    const q = Q.replace({ ...Q.empty(), shuffle: true }, items(1, 2, 3, 4), 2, first)
    expect(cur(q)).toBe(3)
    expect(q.pos).toBe(0)
    expect(ids(q).slice(1).sort()).toEqual([1, 2, 4])
    expect(q.list.map((e) => e.id)).toEqual([1, 2, 3, 4])
  })
  it('empties on no items and caps at MAX', () => {
    expect(Q.replace(Q.replace(Q.empty(), items(1)), []).pos).toBe(-1)
    const many = Array.from({ length: Q.MAX + 10 }, (_, i) => ({ id: i }))
    expect(Q.replace(Q.empty(), many).list.length).toBe(Q.MAX)
  })
})

describe('adding', () => {
  it('appends at the end and starts an empty queue', () => {
    let q = Q.append(Q.empty(), items(1))
    expect(cur(q)).toBe(1)
    q = Q.append(q, items(2, 3))
    expect(ids(q)).toEqual([1, 2, 3])
    expect(cur(q)).toBe(1)
  })
  it('plays next right after the current track, also without shuffle later', () => {
    let q = Q.replace(Q.empty(), items(1, 2, 3), 0)
    q = Q.playNext(q, items(9))
    expect(ids(q)).toEqual([1, 9, 2, 3])
    expect(q.list.map((e) => e.id)).toEqual([1, 9, 2, 3])
    q = Q.setShuffle(Q.setShuffle(q, true, first), false)
    expect(ids(q)).toEqual([1, 9, 2, 3])
  })
  it('keeps duplicates apart', () => {
    const q = Q.append(Q.replace(Q.empty(), items(1)), items(1))
    expect(new Set(q.order).size).toBe(2)
  })
})

describe('moving', () => {
  const q = Q.replace(Q.empty(), items(1, 2, 3), 0)
  it('goes forward and stops at the end', () => {
    expect(cur(Q.next(q))).toBe(2)
    expect(Q.next(Q.jump(q, 2))).toBeNull()
    expect(Q.peek(Q.jump(q, 2))).toBeNull()
  })
  it('repeats all and one', () => {
    const all = { ...Q.jump(q, 2), repeat: 'all' }
    expect(cur(Q.next(all))).toBe(1)
    expect(Q.peek(all).id).toBe(1)
    const one = { ...q, repeat: 'one' }
    expect(cur(Q.next(one, true))).toBe(1) // the track ended: again
    expect(cur(Q.next(one))).toBe(2) // the user asked for the next one
    expect(Q.peek(one).id).toBe(1)
  })
  it('goes back', () => {
    expect(cur(Q.prev(Q.jump(q, 2)))).toBe(2)
    expect(cur(Q.prev(q))).toBe(1)
    expect(cur(Q.prev({ ...q, repeat: 'all' }))).toBe(3)
  })
  it('cycles repeat modes', () => {
    expect(Q.cycleRepeat(q).repeat).toBe('all')
    expect(Q.cycleRepeat(Q.cycleRepeat(Q.cycleRepeat(q))).repeat).toBe('off')
  })
})

describe('removing', () => {
  const q = Q.jump(Q.replace(Q.empty(), items(1, 2, 3, 4), 0), 1)
  it('keeps the current track when another goes', () => {
    expect(cur(Q.remove(q, 0))).toBe(2)
    expect(cur(Q.remove(q, 3))).toBe(2)
  })
  it('moves on when the current one goes', () => {
    expect(cur(Q.remove(q, 1))).toBe(3)
    const last = Q.jump(q, 3)
    expect(cur(Q.remove(last, 3))).toBe(3)
    expect(Q.remove(Q.replace(Q.empty(), items(1)), 0).pos).toBe(-1)
  })
  it('clears what is still to play', () => {
    const c = Q.clearUpcoming(q)
    expect(ids(c)).toEqual([1, 2])
    expect(cur(c)).toBe(2)
    expect(Q.upcoming(c)).toEqual([])
  })
  it('lists what comes next', () => {
    expect(Q.upcoming(q).map((u) => [u.entry.id, u.pos])).toEqual([
      [3, 2],
      [4, 3],
    ])
  })
})

describe('shuffle', () => {
  it('shuffles only what is still to play, and comes back to the list order', () => {
    const q = Q.jump(Q.replace(Q.empty(), items(1, 2, 3, 4, 5), 0), 1)
    const s = Q.setShuffle(q, true, () => 0.99)
    expect(ids(s).slice(0, 2)).toEqual([1, 2])
    expect(ids(s).slice(2).sort()).toEqual([3, 4, 5])
    expect(cur(s)).toBe(2)
    const back = Q.setShuffle(Q.jump(s, 4), false)
    expect(ids(back)).toEqual([1, 2, 3, 4, 5])
    expect(cur(back)).toBe(Q.current(Q.jump(s, 4)).id)
  })
})

describe('restore', () => {
  it('round-trips and survives garbage', () => {
    const q = Q.cycleRepeat(Q.replace(Q.empty(), items(1, 2), 1))
    expect(Q.restore(JSON.stringify(q))).toEqual(q)
    expect(Q.restore('no es json')).toEqual(Q.empty())
    expect(Q.restore('{"list":[],"order":[3]}')).toEqual(Q.empty())
    expect(Q.restore(null)).toEqual(Q.empty())
    const r = Q.restore(JSON.stringify({ ...q, repeat: 'x', pos: 9, nextKey: 0 }))
    expect([r.repeat, r.pos, r.nextKey]).toEqual(['off', 0, 3])
  })
})
```

- [ ] **Step 2: Run them to see them fail**

Run: `cd web && npx vitest run src/lib/queue.test.js`
Expected: FAIL (`Failed to resolve import "./queue.js"`).

- [ ] **Step 3: Implement `web/src/lib/queue.js`**

```js
// The play queue: pure functions over a plain, serializable state. Every
// queued track is an entry with its own key, so the same track can be
// queued twice. list keeps the entries as they were added (the order
// without shuffle); order lists their keys in play order; pos is the
// current entry's place in order (-1: nothing).

export const MAX = 5000 // as the server's MaxPlay: what fits in localStorage

export const REPEAT = ['off', 'all', 'one']

export function empty() {
  return { list: [], order: [], pos: -1, shuffle: false, repeat: 'off', nextKey: 1 }
}

function entries(q, items) {
  let key = q.nextKey
  const out = items.map((item) => ({ ...item, key: key++ }))
  return { out, nextKey: key }
}

// shuffled returns a copy of keys in random order (Fisher–Yates; rand
// returns [0, 1)).
function shuffled(keys, rand) {
  const a = [...keys]
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(rand() * (i + 1))
    ;[a[i], a[j]] = [a[j], a[i]]
  }
  return a
}

// replace starts a new queue with items, playing items[start] first. With
// shuffle on, the rest come in random order.
export function replace(q, items, start = 0, rand = Math.random) {
  items = items.slice(0, MAX)
  const { out, nextKey } = entries(q, items)
  const keys = out.map((e) => e.key)
  if (keys.length === 0) return { ...q, list: [], order: [], pos: -1, nextKey }
  start = Math.min(Math.max(start, 0), keys.length - 1)
  let order = keys
  let pos = start
  if (q.shuffle) {
    order = [keys[start], ...shuffled(keys.filter((_, i) => i !== start), rand)]
    pos = 0
  }
  return { ...q, list: out, order, pos, nextKey }
}

// append adds items at the end of the queue ("Agregar a la cola").
export function append(q, items) {
  items = items.slice(0, Math.max(0, MAX - q.list.length))
  const { out, nextKey } = entries(q, items)
  const order = [...q.order, ...out.map((e) => e.key)]
  return { ...q, list: [...q.list, ...out], order, pos: q.pos < 0 && order.length ? 0 : q.pos, nextKey }
}

// playNext puts items right after the current track ("Reproducir a
// continuación"), in the list and in the play order.
export function playNext(q, items) {
  items = items.slice(0, Math.max(0, MAX - q.list.length))
  if (q.pos < 0) return append(q, items)
  const { out, nextKey } = entries(q, items)
  const keys = out.map((e) => e.key)
  const cur = q.order[q.pos]
  const at = q.list.findIndex((e) => e.key === cur) + 1
  const list = [...q.list.slice(0, at), ...out, ...q.list.slice(at)]
  const order = [...q.order.slice(0, q.pos + 1), ...keys, ...q.order.slice(q.pos + 1)]
  return { ...q, list, order, nextKey }
}

function byKey(q, key) {
  return q.list.find((e) => e.key === key) ?? null
}

// current is the entry playing (or paused), or null.
export function current(q) {
  return q.pos >= 0 && q.pos < q.order.length ? byKey(q, q.order[q.pos]) : null
}

// nextPos is where the queue goes after the current entry: auto is true
// when the track ended by itself (repeat one plays it again). -1: nothing
// more to play.
function nextPos(q, auto) {
  if (q.pos < 0 || q.order.length === 0) return -1
  if (auto && q.repeat === 'one') return q.pos
  if (q.pos + 1 < q.order.length) return q.pos + 1
  return q.repeat === 'off' ? -1 : 0
}

// next moves to the following entry, or returns null at the end.
export function next(q, auto = false) {
  const pos = nextPos(q, auto)
  return pos < 0 ? null : { ...q, pos }
}

// peek is the entry that will play when the current one ends (to preload
// it), or null.
export function peek(q) {
  const pos = nextPos(q, true)
  return pos < 0 ? null : byKey(q, q.order[pos])
}

// prev moves to the previous entry (the last one from the first with
// repeat on; the first one stays).
export function prev(q) {
  if (q.pos < 0) return q
  if (q.pos > 0) return { ...q, pos: q.pos - 1 }
  return q.repeat === 'off' ? q : { ...q, pos: q.order.length - 1 }
}

// jump plays the entry at a place of the play order.
export function jump(q, pos) {
  return pos >= 0 && pos < q.order.length ? { ...q, pos } : q
}

// remove takes the entry at a place of the play order out of the queue.
// Removing the current one makes the next one current.
export function remove(q, pos) {
  if (pos < 0 || pos >= q.order.length) return q
  const key = q.order[pos]
  const order = q.order.filter((_, i) => i !== pos)
  const list = q.list.filter((e) => e.key !== key)
  let p = q.pos
  if (pos < p) p--
  if (p >= order.length) p = order.length - 1
  return { ...q, list, order, pos: order.length ? p : -1 }
}

// clearUpcoming keeps what was played and the current entry, and drops the
// rest.
export function clearUpcoming(q) {
  const keep = new Set(q.order.slice(0, q.pos + 1))
  return { ...q, order: q.order.slice(0, q.pos + 1), list: q.list.filter((e) => keep.has(e.key)) }
}

// setShuffle turns shuffle on (what is still to play comes in random
// order) or off (back to the list's order, from the current entry).
export function setShuffle(q, on, rand = Math.random) {
  if (on === q.shuffle) return q
  if (on) {
    const done = q.order.slice(0, q.pos + 1)
    return { ...q, shuffle: true, order: [...done, ...shuffled(q.order.slice(q.pos + 1), rand)] }
  }
  const cur = q.order[q.pos]
  const order = q.list.map((e) => e.key)
  return { ...q, shuffle: false, order, pos: q.pos < 0 ? -1 : order.indexOf(cur) }
}

// cycleRepeat goes off → all → one → off.
export function cycleRepeat(q) {
  return { ...q, repeat: REPEAT[(REPEAT.indexOf(q.repeat) + 1) % REPEAT.length] }
}

// upcoming lists the entries after the current one, with their place in
// the play order.
export function upcoming(q) {
  return q.order.slice(q.pos + 1).map((key, i) => ({ entry: byKey(q, key), pos: q.pos + 1 + i }))
}

// restore reads a saved queue back, or an empty one when it is missing or
// broken.
export function restore(json) {
  try {
    const q = JSON.parse(json)
    if (!Array.isArray(q?.list) || !Array.isArray(q?.order)) return empty()
    const keys = new Set(q.list.map((e) => e.key))
    if (q.order.some((k) => !keys.has(k))) return empty()
    if (!REPEAT.includes(q.repeat)) q.repeat = 'off'
    q.shuffle = !!q.shuffle
    q.pos = Number.isInteger(q.pos) && q.pos < q.order.length ? q.pos : q.order.length ? 0 : -1
    q.nextKey = Math.max(q.nextKey ?? 1, ...q.list.map((e) => e.key + 1))
    return q
  } catch {
    return empty()
  }
}
```

- [ ] **Step 4: Run the tests**

Run: `cd web && npx vitest run`
Expected: PASS (50 tests).

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/queue.js web/src/lib/queue.test.js
git commit -m "feat: play queue with shuffle, repeat and play next"
```

---
### Task 8: The player

**Files:**
- Create: `web/src/lib/player.svelte.js`, `web/src/components/Tapa.svelte`, `web/src/components/Cola.svelte`, `web/src/components/Reproductor.svelte`
- Modify: `web/src/App.svelte`

**Interfaces:**
- Consumes: `queue.js` (Task 7); `api`, `itemCoverURL`, `streamURL`, `notify`, `count` (Task 6).
- Produces:
  - `player.svelte.js`: `player` (`$state {queue, playing, loading, time, duration, seekable, volume, muted, ffmpeg}`); `play(what, {shuffle, startId})` (asks `/api/play` and replaces the queue), `enqueue(what, {next})` (adds to the end, or right after the current track); `toggle()`, `next()`, `prev()`, `jumpTo(pos)`, `removeAt(pos)`, `clearUpcoming()`, `seek(seconds)`, `setVolume(v)`, `toggleMute()`, `toggleShuffle()`, `cycleRepeat()`. `what` is `{album: id}`, `{folder: path}` or `{tracks: [ids]}`.
  - `Tapa {srcs, alt, icon, round}`: a square picture; one cover, or a 2×2 mosaic when four load; a failing picture is dropped (the next one stands in); a placeholder icon when none loads.

Two `<audio>` elements (not in the DOM): `active` plays; 30 s before the end, `standby` loads the next entry (`peek`), and when `active` ends they swap and the next track starts at once (almost gapless). Every listener checks it belongs to `active`, so the preloading element never moves the bar. An entry's URL is `/stream/{id}` unless the server already converts it (`transcoded`) or the browser says it cannot play its `mime` (`canPlayType`), in which case it asks for `?transcode=1` (with ffmpeg). Converted tracks cannot seek: the bar disables the slider and says why. Three failures in a row stop playback instead of skipping through a whole broken queue. The queue (`tordo.cola`), the position of the current track (`tordo.tiempo`, every 5 s and on pause) and the volume (`tordo.volumen`) are kept in `localStorage`; after a reload the bar shows the queue where it was and *Reproducir* resumes from there without loading anything before. Media Session shows title, artist, album and cover on the lock screen and wires play/pause, previous, next and seeking (headphones, media keys). Space plays and pauses outside fields and buttons.

The bar: cover, title (a link to the album) and artist; shuffle, previous, play/pause, next, repeat (off → all → one); the slider with times; volume and the queue button. On phones it is a mini player (cover, title, play/pause, next, a thin progress line) above the tabs; tapping it opens a full-screen sheet with the big cover, the slider and big controls. The queue panel (`Cola`) lists what is playing and what comes next (the first 200, then "y N pistas más"), with remove buttons and *Vaciar*.

- [ ] **Step 1: Implement `web/src/lib/player.svelte.js`**

```js
// The player: two <audio> elements, one playing and one loading the next
// track so it takes over as soon as the current one ends (almost gapless).
// The queue and the volume live in this browser's storage: each device has
// its own, and they survive reloading the page.
import { api, itemCoverURL, streamURL } from './api.js'
import { notify } from './app.svelte.js'
import { count } from './format.js'
import * as Q from './queue.js'

const QUEUE_KEY = 'tordo.cola'
const TIME_KEY = 'tordo.tiempo'
const VOLUME_KEY = 'tordo.volumen'
const PRELOAD_SECONDS = 30 // start loading the next track this long before the end
const RESTART_SECONDS = 3 // "previous" after this long restarts the track

function load(key, fallback) {
  try {
    return localStorage.getItem(key) ?? fallback
  } catch {
    return fallback
  }
}

function save(key, value) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Full or blocked storage: the queue just won't survive a reload.
  }
}

const saved = (() => {
  try {
    return JSON.parse(load(TIME_KEY, 'null'))
  } catch {
    return null
  }
})()

export const player = $state({
  queue: Q.restore(load(QUEUE_KEY, null)),
  playing: false,
  loading: false,
  time: 0, // seconds into the current track
  duration: 0,
  seekable: true,
  volume: Math.min(1, Math.max(0, Number(load(VOLUME_KEY, '1')) || 0)),
  muted: false,
  ffmpeg: true, // as the server last said
})

{
  // Show the restored track as it was left, without loading it yet.
  const entry = Q.current(player.queue)
  player.duration = (entry?.durationMs ?? 0) / 1000
  if (entry && saved?.key === entry.key) player.time = saved.time
}

let active = null // the <audio> playing
let standby = null // the <audio> loading the next track
let errors = 0 // tracks that failed in a row
let startAt = 0 // where to start the track being loaded

const canPlayCache = {}

function canPlay(mime) {
  if (!(mime in canPlayCache)) canPlayCache[mime] = !!mime && active.canPlayType(mime) !== ''
  return canPlayCache[mime]
}

// source is how an entry is asked for: as it is, or converted when the
// browser cannot play its format (Safari and Ogg, for instance).
function source(entry) {
  const convert = !entry.transcoded && !canPlay(entry.mime) && player.ffmpeg
  return { url: streamURL(entry.id, convert), seekable: !entry.transcoded && !convert }
}

function makeAudio() {
  const a = new Audio()
  a.preload = 'auto'
  const mine = (fn) => () => a === active && fn()
  a.addEventListener('timeupdate', mine(onTime))
  a.addEventListener('loadedmetadata', mine(onMetadata))
  a.addEventListener('ended', mine(onEnded))
  a.addEventListener('play', mine(() => (player.playing = true)))
  a.addEventListener('pause', mine(onPause))
  a.addEventListener('waiting', mine(() => (player.loading = true)))
  a.addEventListener('playing', mine(() => ((player.loading = false), (errors = 0))))
  a.addEventListener('error', mine(onError))
  return a
}

function audios() {
  if (!active) {
    active = makeAudio()
    standby = makeAudio()
    applyVolume()
  }
}

function applyVolume() {
  for (const a of [active, standby]) {
    if (a) {
      a.volume = player.volume
      a.muted = player.muted
    }
  }
}

function persist() {
  save(QUEUE_KEY, JSON.stringify(player.queue))
}

let lastTimeSave = 0

function onTime() {
  player.time = active.currentTime
  const entry = Q.current(player.queue)
  if (Date.now() - lastTimeSave > 5000 && entry) {
    lastTimeSave = Date.now()
    save(TIME_KEY, JSON.stringify({ key: entry.key, time: player.time }))
  }
  if (player.duration - player.time < PRELOAD_SECONDS) preload()
  positionState()
}

function onMetadata() {
  const d = active.duration
  if (Number.isFinite(d) && d > 0) player.duration = d
  if (startAt > 0 && player.seekable) active.currentTime = startAt
  startAt = 0
}

function onPause() {
  player.playing = false
  const entry = Q.current(player.queue)
  if (entry) save(TIME_KEY, JSON.stringify({ key: entry.key, time: active.currentTime }))
}

function onEnded() {
  const q = Q.next(player.queue, true)
  if (!q) {
    player.playing = false
    player.time = 0
    return
  }
  player.queue = q
  persist()
  const entry = Q.current(q)
  if (standby.dataset.key === String(entry.key)) {
    // The next track is loaded: swap the elements.
    ;[active, standby] = [standby, active]
    standby.removeAttribute('src')
    standby.dataset.key = ''
    show(entry, active.dataset.seekable === '1')
    active.play().catch(() => {})
    return
  }
  start(entry, 0, true)
}

function onError() {
  const entry = Q.current(player.queue)
  player.loading = false
  if (!entry) return
  notify(`No se pudo reproducir «${entry.title}».`)
  if (++errors >= 3) {
    player.playing = false
    return
  }
  const q = Q.next(player.queue)
  if (q) {
    player.queue = q
    persist()
    start(Q.current(q), 0, true)
  }
}

// show updates what the page shows for entry: duration, seekability, the
// lock screen.
function show(entry, seekable) {
  player.duration = (entry.durationMs ?? 0) / 1000
  player.seekable = seekable
  player.time = 0
  mediaSession(entry)
}

// start loads entry in the active element, from at seconds, and plays it
// when play is true.
function start(entry, at, play) {
  audios()
  const src = source(entry)
  standby.removeAttribute('src')
  standby.dataset.key = ''
  active.src = src.url
  active.dataset.key = String(entry.key)
  active.dataset.seekable = src.seekable ? '1' : ''
  show(entry, src.seekable)
  player.time = at
  startAt = at
  player.loading = play
  if (play) active.play().catch(() => (player.loading = false))
}

// preload starts loading the track that comes next, once.
function preload() {
  const entry = Q.peek(player.queue)
  if (!entry || entry.key === Q.current(player.queue)?.key || standby.dataset.key === String(entry.key)) return
  const src = source(entry)
  standby.src = src.url
  standby.dataset.key = String(entry.key)
  standby.dataset.seekable = src.seekable ? '1' : ''
  standby.load()
}

function setQueue(q, play = true) {
  player.queue = q
  persist()
  const entry = Q.current(q)
  if (entry) start(entry, 0, play)
}

// play asks the server for what to play ({album}, {folder}, {tracks}) and
// plays it: from the track with id startId, or shuffled.
export async function play(what, { shuffle = false, startId = null } = {}) {
  let res
  try {
    res = await api.play({ ...what, shuffle })
  } catch (e) {
    notify(e.message)
    return
  }
  player.ffmpeg = res.ffmpeg
  if (res.skipped > 0) notify(`${count(res.skipped, 'pista no se puede', 'pistas no se pueden')} reproducir: falta ffmpeg.`)
  if (res.items.length === 0) return
  const at = startId == null ? 0 : Math.max(0, res.items.findIndex((it) => it.id === startId))
  setQueue(Q.replace({ ...player.queue, shuffle }, res.items, shuffle ? Math.floor(Math.random() * res.items.length) : at))
}

// enqueue adds what to the end of the queue, or right after the current
// track with next: true.
export async function enqueue(what, { next = false } = {}) {
  let res
  try {
    res = await api.play(what)
  } catch (e) {
    notify(e.message)
    return
  }
  player.ffmpeg = res.ffmpeg
  if (res.items.length === 0) return
  const wasEmpty = !Q.current(player.queue)
  player.queue = next ? Q.playNext(player.queue, res.items) : Q.append(player.queue, res.items)
  persist()
  notify(next ? 'Suena a continuación.' : `${count(res.items.length, 'pista agregada', 'pistas agregadas')} a la cola.`)
  if (wasEmpty) start(Q.current(player.queue), 0, false)
}

export function toggle() {
  const entry = Q.current(player.queue)
  if (!entry) return
  audios()
  if (active.dataset.key !== String(entry.key)) {
    start(entry, player.time, true) // after a reload: start where it was
    return
  }
  if (active.paused) active.play().catch(() => {})
  else active.pause()
}

export function next() {
  const q = Q.next(player.queue)
  if (q) setQueue(q, true)
}

export function prev() {
  if (active && active.currentTime > RESTART_SECONDS && player.seekable) {
    active.currentTime = 0
    return
  }
  setQueue(Q.prev(player.queue), true)
}

export function jumpTo(pos) {
  setQueue(Q.jump(player.queue, pos), true)
}

export function removeAt(pos) {
  const wasCurrent = pos === player.queue.pos
  player.queue = Q.remove(player.queue, pos)
  persist()
  if (!wasCurrent) return
  const entry = Q.current(player.queue)
  if (entry) start(entry, 0, player.playing)
  else stop()
}

export function clearUpcoming() {
  player.queue = Q.clearUpcoming(player.queue)
  persist()
  if (standby) {
    standby.removeAttribute('src')
    standby.dataset.key = ''
  }
}

function stop() {
  if (active) {
    active.pause()
    active.removeAttribute('src')
    active.dataset.key = ''
  }
  player.playing = false
  player.time = 0
  player.duration = 0
}

export function seek(seconds) {
  if (!active || !player.seekable) return
  active.currentTime = Math.max(0, Math.min(seconds, player.duration || seconds))
  player.time = active.currentTime
}

export function setVolume(v) {
  player.volume = Math.min(1, Math.max(0, v))
  player.muted = false
  save(VOLUME_KEY, String(player.volume))
  applyVolume()
}

export function toggleMute() {
  player.muted = !player.muted
  applyVolume()
}

export function toggleShuffle() {
  player.queue = Q.setShuffle(player.queue, !player.queue.shuffle)
  persist()
}

export function cycleRepeat() {
  player.queue = Q.cycleRepeat(player.queue)
  persist()
}

// mediaSession shows the track on the lock screen and in the system's
// media controls, which also work with headphones and media keys.
function mediaSession(entry) {
  if (!('mediaSession' in navigator)) return
  navigator.mediaSession.metadata = new MediaMetadata({
    title: entry.title,
    artist: entry.artist,
    album: entry.album,
    artwork: [
      { src: location.origin + itemCoverURL(entry, 320), sizes: '320x320', type: 'image/jpeg' },
      { src: location.origin + itemCoverURL(entry, 640), sizes: '640x640', type: 'image/jpeg' },
    ],
  })
}

let lastPosition = 0

function positionState() {
  if (!('mediaSession' in navigator) || !navigator.mediaSession.setPositionState) return
  if (Date.now() - lastPosition < 1000 || !(player.duration > 0)) return
  lastPosition = Date.now()
  try {
    navigator.mediaSession.setPositionState({
      duration: player.duration,
      position: Math.min(player.time, player.duration),
      playbackRate: 1,
    })
  } catch {
    // Some browsers reject a position past a duration they computed.
  }
}

if ('mediaSession' in navigator) {
  const handlers = {
    play: toggle,
    pause: toggle,
    previoustrack: prev,
    nexttrack: next,
    seekto: (d) => seek(d.seekTime),
  }
  for (const [action, fn] of Object.entries(handlers)) {
    try {
      navigator.mediaSession.setActionHandler(action, fn)
    } catch {
      // An action this browser does not support.
    }
  }
}
```

- [ ] **Step 2: Implement `web/src/components/Tapa.svelte`**

```svelte
<script>
  // A square picture: one cover, or a mosaic of four, with a placeholder
  // when there is none.
  import Icono from './Icono.svelte'

  let { srcs = [], alt = '', icon = 'disc', round = false } = $props()

  // A picture that fails to load is dropped: a mosaic missing one shows
  // the first picture left, and one cover failing tries the next.
  let failed = $state({})
  const ok = $derived((srcs.length >= 4 ? srcs.slice(0, 4) : srcs).filter((s) => !failed[s]))
  const shown = $derived(ok.length >= 4 ? ok : ok.slice(0, 1))
</script>

<div class="tapa" class:round class:mosaic={shown.length === 4} role="img" aria-label={alt}>
  {#if shown.length === 0}
    <span class="none"><Icono name={icon} size={32} /></span>
  {:else}
    {#each shown as src (src)}
      <img {src} alt="" loading="lazy" decoding="async" onerror={() => (failed = { ...failed, [src]: true })} />
    {/each}
  {/if}
</div>

<style>
  .tapa {
    aspect-ratio: 1;
    width: 100%;
    background: var(--surface-2);
    border-radius: var(--radius);
    overflow: hidden;
    display: grid;
  }
  .mosaic {
    grid-template-columns: 1fr 1fr;
    grid-template-rows: 1fr 1fr;
  }
  .round {
    border-radius: 50%;
  }
  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    min-width: 0;
    min-height: 0;
  }
  .none {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--faint);
  }
</style>
```

- [ ] **Step 3: Implement `web/src/components/Cola.svelte`**

```svelte
<script>
  import { itemCoverURL } from '../lib/api.js'
  import { count, duration } from '../lib/format.js'
  import { clearUpcoming, jumpTo, player, removeAt } from '../lib/player.svelte.js'
  import * as Q from '../lib/queue.js'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  let { onclose } = $props()

  const SHOWN = 200 // rows drawn; the rest are summed up
  const now = $derived(Q.current(player.queue))
  const next = $derived(Q.upcoming(player.queue))
</script>

<aside class="cola" aria-label="Cola">
  <header>
    <h3>Cola</h3>
    {#if next.length > 0}
      <button onclick={clearUpcoming}>Vaciar</button>
    {/if}
    <button class="icon" onclick={onclose} title="Cerrar" aria-label="Cerrar"><Icono name="close" /></button>
  </header>
  <div class="scroll">
    {#if now}
      <p class="label">Sonando</p>
      <div class="row current">
        <span class="pic"><Tapa srcs={[itemCoverURL(now, 160)]} /></span>
        <span class="text"><b>{now.title}</b><span>{now.artist}</span></span>
        <span class="time">{duration(now.durationMs)}</span>
      </div>
    {/if}
    {#if next.length > 0}
      <p class="label">A continuación</p>
      {#each next.slice(0, SHOWN) as u (u.entry.key)}
        <div class="row">
          <button class="play" onclick={() => jumpTo(u.pos)}>
            <span class="pic"><Tapa srcs={[itemCoverURL(u.entry, 160)]} /></span>
            <span class="text"><b>{u.entry.title}</b><span>{u.entry.artist}</span></span>
            <span class="time">{duration(u.entry.durationMs)}</span>
          </button>
          <button class="icon" onclick={() => removeAt(u.pos)} title="Quitar de la cola" aria-label="Quitar de la cola">
            <Icono name="close" size={16} />
          </button>
        </div>
      {/each}
      {#if next.length > SHOWN}
        <p class="more">y {count(next.length - SHOWN, 'pista más', 'pistas más')}</p>
      {/if}
    {:else}
      <p class="empty">No hay nada más en la cola.</p>
    {/if}
  </div>
</aside>

<style>
  .cola {
    position: fixed;
    right: 0;
    top: 0;
    bottom: calc(var(--player-h) + var(--tabs-h) + env(safe-area-inset-bottom));
    width: min(400px, 100%);
    z-index: 35;
    background: var(--surface);
    border-left: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    box-shadow: -8px 0 32px rgba(0, 0, 0, 0.4);
  }
  header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 12px 12px 16px;
    border-bottom: 1px solid var(--line);
  }
  h3 {
    flex: 1;
    margin: 0;
    font-size: 16px;
    color: var(--strong);
  }
  .scroll {
    overflow-y: auto;
    padding: 8px 8px 24px;
  }
  .label {
    margin: 12px 8px 6px;
  }
  .row {
    display: flex;
    align-items: center;
  }
  .row .play,
  .current {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 8px;
    border: none;
    border-radius: var(--radius);
    text-align: left;
  }
  .current {
    padding-right: 48px;
  }
  .row .play:hover {
    background: var(--surface-2);
  }
  .pic {
    width: 40px;
    flex: none;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .text b,
  .text span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .text b {
    font-weight: 500;
    color: var(--strong);
  }
  .current .text b {
    color: var(--accent);
  }
  .text span {
    font-size: 13px;
    color: var(--muted);
  }
  .time {
    font-size: 12px;
    color: var(--faint);
    font-variant-numeric: tabular-nums;
  }
  .more {
    color: var(--faint);
    font-size: 13px;
    padding: 8px;
  }
  .empty {
    padding: 24px 0;
  }
</style>
```

- [ ] **Step 4: Implement `web/src/components/Reproductor.svelte`**

```svelte
<script>
  // The persistent player bar. On phones it is a mini player above the
  // tabs that opens into a full-screen sheet.
  import { itemCoverURL } from '../lib/api.js'
  import { duration } from '../lib/format.js'
  import {
    cycleRepeat,
    next,
    player,
    prev,
    seek,
    setVolume,
    toggle,
    toggleMute,
    toggleShuffle,
  } from '../lib/player.svelte.js'
  import * as Q from '../lib/queue.js'
  import { albumHref } from '../lib/router.js'
  import Cola from './Cola.svelte'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  const entry = $derived(Q.current(player.queue))
  const pct = $derived(player.duration > 0 ? Math.min(100, (player.time / player.duration) * 100) : 0)
  const repeatTitle = { off: 'Repetir: no', all: 'Repetir: toda la cola', one: 'Repetir: esta pista' }

  let sheet = $state(false) // the full player, on phones
  let queueOpen = $state(false)

  // Space plays and pauses, unless typing or on a button.
  function onkeydown(event) {
    if (event.key !== ' ' || event.ctrlKey || event.metaKey || event.altKey) return
    const tag = event.target?.tagName
    if (['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON', 'A'].includes(tag) || event.target?.isContentEditable) return
    event.preventDefault()
    toggle()
  }
</script>

<svelte:window {onkeydown} />

{#snippet controls(big)}
  <div class="controls" class:big>
    <button class="icon" class:on={player.queue.shuffle} onclick={toggleShuffle} title="Aleatorio" aria-label="Aleatorio"
      aria-pressed={player.queue.shuffle}><Icono name="shuffle" size={big ? 22 : 18} /></button>
    <button class="icon" onclick={prev} title="Anterior" aria-label="Anterior"><Icono name="prev" size={big ? 28 : 20} /></button>
    <button class="icon playpause" onclick={toggle} title={player.playing ? 'Pausa' : 'Reproducir'}
      aria-label={player.playing ? 'Pausa' : 'Reproducir'} disabled={!entry}>
      <Icono name={player.playing ? 'pause' : 'play'} size={big ? 30 : 20} />
    </button>
    <button class="icon" onclick={next} title="Siguiente" aria-label="Siguiente"><Icono name="next" size={big ? 28 : 20} /></button>
    <button class="icon" class:on={player.queue.repeat !== 'off'} onclick={cycleRepeat} title={repeatTitle[player.queue.repeat]}
      aria-label={repeatTitle[player.queue.repeat]}>
      <Icono name="repeat" size={big ? 22 : 18} />
      {#if player.queue.repeat === 'one'}<span class="one">1</span>{/if}
    </button>
  </div>
{/snippet}

{#snippet progress()}
  <div class="progress">
    <span class="time">{duration(player.time * 1000)}</span>
    <input type="range" min="0" max={player.duration || 0} step="1" value={player.time} disabled={!player.seekable || !entry}
      oninput={(e) => seek(Number(e.currentTarget.value))} aria-label="Posición"
      title={player.seekable ? '' : 'Este formato se convierte al vuelo: no se puede adelantar'}
      style="--pct: {pct}%" />
    <span class="time">{duration(player.duration * 1000)}</span>
  </div>
{/snippet}

{#if entry}
  <div class="bar">
    <div class="line" style="width: {pct}%"></div>
    <div class="now">
      <button class="open" onclick={() => (sheet = true)} aria-label="Abrir el reproductor">
        <span class="pic"><Tapa srcs={[itemCoverURL(entry, 160)]} /></span>
      </button>
      <span class="text">
        {#if entry.albumId}
          <a class="title" href={albumHref(entry.albumId)}>{entry.title}</a>
        {:else}
          <span class="title">{entry.title}</span>
        {/if}
        <span class="artist">{entry.artist}</span>
      </span>
      <button class="open cover-all" onclick={() => (sheet = true)} aria-label="Abrir el reproductor"></button>
    </div>
    <div class="center">
      {@render controls(false)}
      {@render progress()}
    </div>
    <div class="right">
      <button class="icon" onclick={toggleMute} title={player.muted ? 'Activar sonido' : 'Silenciar'}
        aria-label={player.muted ? 'Activar sonido' : 'Silenciar'}><Icono name={player.muted ? 'mute' : 'volume'} /></button>
      <input class="volume" type="range" min="0" max="1" step="0.01" value={player.muted ? 0 : player.volume}
        oninput={(e) => setVolume(Number(e.currentTarget.value))} aria-label="Volumen"
        style="--pct: {(player.muted ? 0 : player.volume) * 100}%" />
      <button class="icon" class:on={queueOpen} onclick={() => (queueOpen = !queueOpen)} title="Cola" aria-label="Cola">
        <Icono name="queue" />
      </button>
    </div>
    <div class="mini">
      <button class="icon" onclick={toggle} aria-label={player.playing ? 'Pausa' : 'Reproducir'}>
        <Icono name={player.playing ? 'pause' : 'play'} size={22} />
      </button>
      <button class="icon" onclick={next} aria-label="Siguiente"><Icono name="next" size={22} /></button>
    </div>
  </div>

  {#if sheet}
    <div class="sheet" role="dialog" aria-label="Reproductor">
      <header>
        <button class="icon" onclick={() => (sheet = false)} aria-label="Cerrar"><Icono name="down" size={26} /></button>
        <span class="label">Sonando</span>
        <button class="icon" onclick={() => ((queueOpen = true), (sheet = false))} aria-label="Cola"><Icono name="queue" size={22} /></button>
      </header>
      <div class="big-pic"><Tapa srcs={[itemCoverURL(entry, 640)]} /></div>
      <div class="big-text">
        <b>{entry.title}</b>
        <span>{entry.artist}</span>
        {#if entry.albumId}
          <a href={albumHref(entry.albumId)} onclick={() => (sheet = false)}>{entry.album}</a>
        {/if}
      </div>
      {@render progress()}
      {@render controls(true)}
    </div>
  {/if}
{/if}

{#if queueOpen}
  <Cola onclose={() => (queueOpen = false)} />
{/if}

<style>
  .bar {
    position: fixed;
    left: 0;
    right: 0;
    bottom: calc(var(--tabs-h) + env(safe-area-inset-bottom));
    height: var(--player-h);
    z-index: 30;
    background: var(--surface);
    border-top: 1px solid var(--line);
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 2fr) minmax(0, 1fr);
    align-items: center;
    gap: 16px;
    padding: 0 var(--gutter);
  }
  .line {
    display: none;
  }
  .now {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
    position: relative;
  }
  .open {
    border: none;
    padding: 0;
  }
  .cover-all {
    display: none;
  }
  .pic {
    display: block;
    width: 48px;
  }
  .text {
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .title,
  .artist {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .artist {
    font-size: 13px;
    color: var(--muted);
  }
  .center {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }
  .controls {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .controls .icon {
    position: relative;
  }
  .playpause {
    background: var(--strong);
    color: var(--bg) !important;
    width: 34px;
    height: 34px;
  }
  .big .playpause {
    width: 64px;
    height: 64px;
  }
  .one {
    position: absolute;
    right: 0;
    bottom: 0;
    font-size: 9px;
    font-weight: 700;
  }
  .progress {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    max-width: 560px;
  }
  .time {
    font-size: 11px;
    color: var(--faint);
    font-variant-numeric: tabular-nums;
    min-width: 36px;
    text-align: center;
  }
  input[type='range'] {
    flex: 1;
    appearance: none;
    height: 4px;
    border-radius: 2px;
    background: linear-gradient(to right, var(--muted) var(--pct), var(--line-strong) var(--pct));
    cursor: pointer;
  }
  input[type='range']:disabled {
    cursor: default;
  }
  input[type='range']::-webkit-slider-thumb {
    appearance: none;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--strong);
  }
  input[type='range']::-moz-range-thumb {
    width: 12px;
    height: 12px;
    border: none;
    border-radius: 50%;
    background: var(--strong);
  }
  .right {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
  }
  .volume {
    max-width: 110px;
  }
  .mini {
    display: none;
  }
  .sheet {
    position: fixed;
    inset: 0;
    z-index: 45;
    background: var(--bg);
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding: 12px 24px calc(32px + env(safe-area-inset-bottom));
  }
  .sheet header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .big-pic {
    width: min(100%, 52vh);
    align-self: center;
  }
  .big-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .big-text b {
    font-size: 20px;
    color: var(--strong);
  }
  .big-text span,
  .big-text a {
    color: var(--muted);
  }
  .sheet .progress {
    max-width: none;
  }
  .sheet .controls {
    justify-content: space-between;
  }
  @media (max-width: 640px) {
    .bar {
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 8px;
      padding: 0 8px;
    }
    .line {
      display: block;
      position: absolute;
      top: -1px;
      left: 0;
      height: 2px;
      background: var(--accent);
    }
    .center,
    .right {
      display: none;
    }
    .mini {
      display: flex;
    }
    .pic {
      width: 44px;
    }
    .cover-all {
      display: block;
      position: absolute;
      inset: 0;
    }
  }
</style>
```

- [ ] **Step 5: Mount the bar**

In `web/src/App.svelte`, import it (`import Reproductor from './components/Reproductor.svelte'`) and add `<Reproductor />` between `</main>` and the notices `<div>`.

- [ ] **Step 6: Build and try it**

Run: `cd web && npx vitest run && npm run build`
Expected: PASS and a clean build (no Svelte warnings).

No page plays anything yet (Tasks 9–11), so seed a queue from the browser's console. Run `go run ./cmd/tordo -dir .run`, then in the console of the page it opens:

```js
const r = await (await fetch('/api/play?album=1')).json()
let k = 1
const list = r.items.map((it) => ({ ...it, key: k++ }))
localStorage.setItem('tordo.cola', JSON.stringify({ list, order: list.map((e) => e.key), pos: 0, shuffle: false, repeat: 'off', nextKey: k }))
location.reload()
```

Check: the bar shows the first track and its duration; ▶ plays it; the slider seeks (for an MP3/FLAC album); ⏭ moves on; dragging the slider to 3 s before the end hands over to the next track without a pause; the queue button opens the panel and a click on an entry plays it; a reload keeps the queue and the position; at 375 px the mini player opens the sheet. With ffmpeg, an album of WMA files plays (`/api/play?album=` with one of them) and its slider is disabled.

- [ ] **Step 7: Commit**

```bash
git add web/src
git commit -m "feat: player bar, queue panel and audio engine with preload and Media Session"
```

---

### Task 9: The album page

**Files:**
- Create: `web/src/pages/Album.svelte`, `web/src/components/ListaPistas.svelte`, `web/src/components/Acciones.svelte`
- Modify: `web/src/App.svelte`

**Interfaces:**
- Consumes: `api.album`, `albumCoverURL`, `app.generation`, `count`, `length`, `quality`, `folderName`, `folderHref` (Task 6); `play`, `enqueue` (Task 8); `Tapa` (Task 8).
- Produces: `ListaPistas {tracks, onplay(track), albumArtist, numbered}` (rows: number, title, the artist when it is not the album's, format, duration, a ➕ to queue the track; disc headers when the tracks span discs); `Acciones {what}` (*Reproducir*, *Aleatorio*, *A continuación*, *A la cola*).

`/album/{id}` with M1's data: the cover (640 px), *Álbum* or *Compilado*, title, artist, "year · N pistas · duration", the formats ("FLAC 16/44,1"), the folder (a link to Carpetas) and the actions; a click on a track plays the album from that track. M3b adds the identification data (original year, country, genres) and *Corregir identificación*.

- [ ] **Step 1: Implement `web/src/components/ListaPistas.svelte`**

```svelte
<script>
  // A list of tracks: a click plays from that track. Disc headers when the
  // tracks span several discs; the artist when it is not the album's.
  import { duration, quality } from '../lib/format.js'
  import { enqueue } from '../lib/player.svelte.js'
  import Icono from './Icono.svelte'

  let { tracks, onplay, albumArtist = '', numbered = true } = $props()

  const discs = $derived(new Set(tracks.map((t) => t.disc || 1)).size)
</script>

<ol class="pistas">
  {#each tracks as t, i (t.id)}
    {#if discs > 1 && (i === 0 || (tracks[i - 1].disc || 1) !== (t.disc || 1))}
      <li class="disc label">Disco {t.disc || 1}</li>
    {/if}
    <li class="row">
      <button class="play" onclick={() => onplay(t)}>
        <span class="n">{numbered ? t.track || '' : ''}</span>
        <span class="text">
          <span class="title">{t.title}</span>
          {#if t.artist && t.artist !== albumArtist}<span class="artist">{t.artist}</span>{/if}
        </span>
        <span class="quality">{quality(t)}</span>
        <span class="time">{duration(t.durationMs)}</span>
      </button>
      <button class="icon add" onclick={() => enqueue({ tracks: [t.id] })} title="Agregar a la cola" aria-label="Agregar a la cola">
        <Icono name="plus" size={16} />
      </button>
    </li>
  {/each}
</ol>

<style>
  .pistas {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .disc {
    padding: 16px 8px 6px;
  }
  .row {
    display: flex;
    align-items: center;
    border-bottom: 1px solid var(--line);
  }
  .play {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 12px;
    border: none;
    border-radius: 0;
    padding: 8px;
    text-align: left;
  }
  .play:hover {
    background: var(--surface);
  }
  .n {
    width: 24px;
    text-align: right;
    color: var(--faint);
    font-variant-numeric: tabular-nums;
    flex: none;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .title,
  .artist {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
  }
  .artist {
    font-size: 13px;
    color: var(--muted);
  }
  .quality {
    font-size: 12px;
    color: var(--faint);
    white-space: nowrap;
  }
  .time {
    font-size: 13px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    min-width: 40px;
    text-align: right;
  }
  .add {
    opacity: 0;
  }
  .row:hover .add,
  .add:focus-visible {
    opacity: 1;
  }
  @media (max-width: 640px) {
    .quality {
      display: none;
    }
    .add {
      opacity: 1;
    }
  }
</style>
```

- [ ] **Step 2: Implement `web/src/components/Acciones.svelte`**

```svelte
<script>
  // Play, shuffle and queue buttons for an album, folder or collection.
  import { enqueue, play } from '../lib/player.svelte.js'
  import Icono from './Icono.svelte'

  let { what } = $props()
</script>

<div class="actions">
  <button class="primary" onclick={() => play(what)}><Icono name="play" size={14} /> Reproducir</button>
  <button onclick={() => play(what, { shuffle: true })}><Icono name="shuffle" size={14} /> Aleatorio</button>
  <button onclick={() => enqueue(what, { next: true })}><Icono name="next_up" size={14} /> A continuación</button>
  <button onclick={() => enqueue(what)}><Icono name="plus" size={14} /> A la cola</button>
</div>

<style>
  button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
  }
</style>
```

- [ ] **Step 3: Implement `web/src/pages/Album.svelte`**

```svelte
<script>
  import Acciones from '../components/Acciones.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import Tapa from '../components/Tapa.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count, folderName, length, quality } from '../lib/format.js'
  import { play } from '../lib/player.svelte.js'
  import { folderHref } from '../lib/router.js'

  let { id } = $props()

  let album = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    let stale = false
    api
      .album(id)
      .then((a) => {
        if (!stale) (album = a), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.status === 404 ? 'Este álbum ya no está en el catálogo.' : e.message
      })
    return () => (stale = true)
  })

  const summary = $derived(
    album
      ? [album.year || '', count(album.tracks.length, 'pista', 'pistas'), length(album.durationMs)].filter(Boolean).join(' · ')
      : '',
  )

  // formats sums up the album's formats: "FLAC 16/44,1", or several.
  const formats = $derived(album ? [...new Set(album.tracks.map(quality))].join(' · ') : '')
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if album}
  <div class="head">
    <div class="pic"><Tapa srcs={[albumCoverURL(album.id, 640)]} alt={album.title} /></div>
    <div class="info">
      <span class="label">{album.compilation ? 'Compilado' : 'Álbum'}</span>
      <h1>{album.title}</h1>
      <p class="artist">{album.artist}</p>
      <p class="sub">{summary}</p>
      <p class="sub">{formats}</p>
      <a class="path" href={folderHref(album.dir)}>{folderName(album.dir)}</a>
      <Acciones what={{ album: album.id }} />
    </div>
  </div>
  <ListaPistas tracks={album.tracks} albumArtist={album.artist} onplay={(t) => play({ album: album.id }, { startId: t.id })} />
{/if}

<style>
  .head {
    display: flex;
    gap: 32px;
    align-items: flex-end;
    margin: 8px 0 24px;
  }
  .pic {
    width: 240px;
    flex: none;
  }
  .info {
    min-width: 0;
  }
  .artist {
    font-size: 17px;
    color: var(--text);
    margin: 0 0 6px;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0 0 2px;
  }
  .path:hover {
    color: var(--text);
  }
  @media (max-width: 640px) {
    .head {
      flex-direction: column;
      align-items: stretch;
      gap: 16px;
    }
    .pic {
      width: min(100%, 320px);
      align-self: center;
    }
  }
</style>
```

- [ ] **Step 4: Route to it**

In `web/src/App.svelte`, import it (`import Album from './pages/Album.svelte'`) and make `<main>`:

```svelte
<main>
  {#if current.page === 'album'}
    {#key current.id}
      <Album id={current.id} />
    {/key}
  {:else if current.page === 'notfound'}
    <p class="empty">No existe esta página. <a href="/carpetas">Ir a Carpetas</a></p>
  {:else}
    <p class="empty">Esta sección llega en las próximas tareas.</p>
  {/if}
</main>
```

- [ ] **Step 5: Build and try it**

Run: `cd web && npx vitest run && npm run build`, then `go run ./cmd/tordo -dir .run` and open `/album/1`: the header and the tracks show; *Reproducir* plays from the first track, a click on track 3 plays from it, *Aleatorio* turns shuffle on (the bar's ⤮ lights up), *A continuación* and ➕ add to the queue (a notice says so); `/album/999999` says the album is not in the catalog.

- [ ] **Step 6: Commit**

```bash
git add web/src
git commit -m "feat: album page with its tracks and play actions"
```

---
### Task 10: Carpetas

**Files:**
- Create: `web/src/pages/Carpetas.svelte`, `web/src/components/TarjetaAlbum.svelte`
- Modify: `web/src/App.svelte`

**Interfaces:**
- Consumes: `api.folder` (with `art` and `trail`, Task 5); `navigate`, `savedScroll`, `folderHref`, `albumHref` (Task 6); `play` (Task 8); `Acciones`, `ListaPistas` (Task 9); `Tapa`.
- Produces: `TarjetaAlbum {album}` (an album card: cover, title, "artist · year", a ▶ on hover).

`/carpetas?ruta=…` (and `/`): the folders as they are on disk, in natural order. The top level lists the roots; with **a single root** it goes straight into it (replacing the history entry). A folder shows its trail (*Carpetas / Music / Brasil*), *Colección* when it is one, its name, "N álbumes · N pistas · duration", the actions (recursive, in order), then its subfolders as cards (a mosaic of their albums' covers, a ▶ on hover; a subfolder that is one album's own folder opens the album), its own albums, and its loose tracks (a click plays them from that track). Going back restores the scroll position.

- [ ] **Step 1: Implement `web/src/components/TarjetaAlbum.svelte`**

```svelte
<script>
  // An album card: cover, title, artist and year, with a play button.
  import { albumCoverURL } from '../lib/api.js'
  import { play } from '../lib/player.svelte.js'
  import { albumHref } from '../lib/router.js'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  let { album } = $props()
</script>

<div class="card">
  <div class="pic">
    <a href={albumHref(album.id)}>
      <Tapa srcs={[albumCoverURL(album.id, 320)]} alt={album.title} />
    </a>
    <button class="icon play" onclick={() => play({ album: album.id })} title="Reproducir" aria-label="Reproducir {album.title}">
      <Icono name="play" size={18} />
    </button>
  </div>
  <a href={albumHref(album.id)} class="title">{album.title}</a>
  <span class="sub">{album.artist}{album.year ? ` · ${album.year}` : ''}</span>
</div>

<style>
  .card {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .pic {
    position: relative;
    margin-bottom: 8px;
  }
  .pic a {
    display: block;
  }
  .title,
  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
  }
  .play {
    position: absolute;
    right: 8px;
    bottom: 8px;
    width: 40px;
    height: 40px;
    background: var(--accent);
    color: var(--on-accent) !important;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .card:hover .play,
  .play:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .play {
      display: none; /* phones open the album */
    }
  }
</style>
```

- [ ] **Step 2: Implement `web/src/pages/Carpetas.svelte`**

```svelte
<script>
  // The folders as they are on disk: the roots, then each folder with its
  // subfolders, albums and loose tracks. Collections are folders too.
  import Acciones from '../components/Acciones.svelte'
  import Icono from '../components/Icono.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import Tapa from '../components/Tapa.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count, length } from '../lib/format.js'
  import { navigate, savedScroll } from '../lib/nav.svelte.js'
  import { play } from '../lib/player.svelte.js'
  import { albumHref, folderHref } from '../lib/router.js'

  let { path } = $props()

  let folder = $state(null)
  let error = $state('')

  $effect(() => {
    const p = path
    app.generation // reload when a scan ends
    let stale = false
    api
      .folder(p)
      .then((f) => {
        if (stale) return
        // With a single root, the top level is that root.
        if (!p && f.children.length === 1) {
          navigate(folderHref(f.children[0].path), { replace: true })
          return
        }
        folder = f
        error = ''
        requestAnimationFrame(() => window.scrollTo(0, savedScroll()))
      })
      .catch((e) => {
        if (!stale) error = e.message
      })
    return () => (stale = true)
  })

  function childSummary(c) {
    const n = c.albumCount > 0 ? count(c.albumCount, 'álbum', 'álbumes') : count(c.trackCount, 'pista', 'pistas')
    return c.collection ? `Colección · ${n}` : n
  }

  // A subfolder holding a single album (its own folder) opens the album.
  function childHref(child) {
    const art = folder.art?.[child.path] ?? []
    return child.albumCount === 1 && art.length === 1 ? albumHref(art[0]) : folderHref(child.path)
  }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if folder}
  {#if path}
    <nav class="trail" aria-label="Ubicación">
      <a href="/carpetas">Carpetas</a>
      {#each folder.trail.slice(0, -1) as c (c.path)}
        <span>/</span><a href={folderHref(c.path)}>{c.name}</a>
      {/each}
    </nav>
    {#if folder.collection}<span class="label">Colección</span>{/if}
    <h1>{folder.name}</h1>
    <p class="sub">
      {count(folder.albumCount, 'álbum', 'álbumes')} · {count(folder.trackCount, 'pista', 'pistas')} · {length(folder.durationMs)}
    </p>
    {#if folder.trackCount > 0}
      <Acciones what={{ folder: path }} />
    {/if}
  {:else}
    <h1>Carpetas</h1>
    {#if folder.children.length === 0}
      <p class="empty">Todavía no hay música en el catálogo. Revisá las carpetas de config.json.</p>
    {/if}
  {/if}

  {#if folder.children.length > 0}
    {#if path}<h2>Carpetas</h2>{/if}
    <div class="grid">
      {#each folder.children as c (c.path)}
        <div class="card">
          <div class="pic">
            <a href={childHref(c)}>
              <Tapa srcs={(folder.art?.[c.path] ?? []).map((id) => albumCoverURL(id, 320))} icon="folder" alt={c.name} />
            </a>
            {#if c.trackCount > 0}
              <button class="icon play" onclick={() => play({ folder: c.path })} title="Reproducir" aria-label="Reproducir {c.name}">
                <Icono name="play" size={18} />
              </button>
            {/if}
          </div>
          <a class="name" href={childHref(c)}>{c.name}</a>
          <span class="sub">{childSummary(c)}</span>
        </div>
      {/each}
    </div>
  {/if}

  {#if folder.albums?.length > 0}
    <h2>Álbumes</h2>
    <div class="grid">
      {#each folder.albums as a (a.id)}
        <TarjetaAlbum album={a} />
      {/each}
    </div>
  {/if}

  {#if folder.loose?.length > 0}
    <h2>Pistas sueltas</h2>
    <ListaPistas
      tracks={folder.loose}
      numbered={false}
      onplay={(t) => play({ tracks: folder.loose.map((x) => x.id) }, { startId: t.id })} />
  {/if}
{/if}

<style>
  .trail {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    font-size: 13px;
    color: var(--muted);
    margin-bottom: 12px;
  }
  .trail span {
    color: var(--faint);
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0;
  }
  .card {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .pic {
    position: relative;
    margin-bottom: 8px;
  }
  .pic a {
    display: block;
  }
  .name {
    color: var(--strong);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .card .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .play {
    position: absolute;
    right: 8px;
    bottom: 8px;
    width: 40px;
    height: 40px;
    background: var(--accent);
    color: var(--on-accent) !important;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .card:hover .play,
  .play:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .play {
      display: none;
    }
  }
</style>
```

- [ ] **Step 3: Route to it**

In `web/src/App.svelte`, import it (`import Carpetas from './pages/Carpetas.svelte'`) and add the first branch of `<main>`:

```svelte
  {#if current.page === 'carpetas'}
    {#key current.path}
      <Carpetas path={current.path} />
    {/key}
  {:else if current.page === 'album'}
```

(the rest of `<main>` as in Task 9).

- [ ] **Step 4: Build and try it**

Run: `cd web && npm run build`, then `go run ./cmd/tordo -dir .run`: the browser opens on the root's folders with covers (thumbnails appear in `.run/cache/thumbs/`); a collection folder says *Colección* and lists its albums in order; an album folder's card opens the album; ▶ on a card and *Aleatorio* on a folder play it; the trail goes back up; *Atrás* restores the scroll. At 375 px, two cards per row and the tabs at the bottom.

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat: Carpetas, the folders as on disk, with covers and play actions"
```

---
### Task 11: Listas (collections)

**Files:**
- Create: `web/src/pages/Listas.svelte`
- Modify: `web/src/App.svelte`

**Interfaces:**
- Consumes: `api.collections` (M1), `albumCoverURL`, `folderHref`, `count`, `Tapa`.

`/listas`: for now, the collections M1 detects (folders with albums of several artists), each with a mosaic of four of its albums that have a picture (`coverPath` or `coverFileId`), its name and how many albums; a click opens it in Carpetas. Playlists and likes arrive in M4.

- [ ] **Step 1: Implement `web/src/pages/Listas.svelte`**

```svelte
<script>
  // Lists: for now, the collections found in the folders. Playlists and
  // likes come in M4.
  import Tapa from '../components/Tapa.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { folderHref } from '../lib/router.js'

  // The albums with a picture: the folder's image or an embedded one.
  const withPicture = (albums) => albums.filter((a) => a.coverPath || a.coverFileId).slice(0, 4)

  let collections = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    api
      .collections()
      .then((cs) => ((collections = cs), (error = '')))
      .catch((e) => (error = e.message))
  })
</script>

<h1>Listas</h1>
<h2>Colecciones</h2>
{#if error}
  <p class="empty">{error}</p>
{:else if collections?.length === 0}
  <p class="empty">
    No se encontraron colecciones: carpetas con álbumes de varios artistas, como «Latinoamérica 600».
  </p>
{:else if collections}
  <div class="grid">
    {#each collections as c (c.path)}
      <a class="card" href={folderHref(c.path)}>
        <Tapa srcs={withPicture(c.albums).map((a) => albumCoverURL(a.id, 320))} icon="lists" alt={c.name} />
        <span class="name">{c.name}</span>
        <span class="sub">{count(c.albums.length, 'álbum', 'álbumes')}</span>
      </a>
    {/each}
  </div>
{/if}

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .name {
    margin-top: 4px;
    color: var(--strong);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
  }
</style>
```

- [ ] **Step 2: Route to it**

`web/src/App.svelte`, final version:

```svelte
<script>
  import Nav from './components/Nav.svelte'
  import Reproductor from './components/Reproductor.svelte'
  import { app, watchStatus } from './lib/app.svelte.js'
  import { navigate, route } from './lib/nav.svelte.js'
  import { appLink, resolve } from './lib/router.js'
  import Album from './pages/Album.svelte'
  import Carpetas from './pages/Carpetas.svelte'
  import Listas from './pages/Listas.svelte'

  const current = $derived(resolve(route.path, route.search))

  watchStatus()

  // Links inside the app change the route instead of reloading the page,
  // so the music keeps playing.
  function onclick(event) {
    const to = appLink(event, event.target.closest?.('a'), location.origin)
    if (to !== null) {
      event.preventDefault()
      navigate(to)
    }
  }
</script>

<svelte:window {onclick} />

<Nav page={current.page} />

<main>
  {#if current.page === 'carpetas'}
    {#key current.path}
      <Carpetas path={current.path} />
    {/key}
  {:else if current.page === 'album'}
    {#key current.id}
      <Album id={current.id} />
    {/key}
  {:else if current.page === 'listas'}
    <Listas />
  {:else}
    <p class="empty">No existe esta página. <a href="/carpetas">Ir a Carpetas</a></p>
  {/if}
</main>

<Reproductor />

<div class="notices" aria-live="polite">
  {#each app.notices as n (n.id)}
    <div class="notice">{n.text}</div>
  {/each}
</div>

<style>
  .notices {
    position: fixed;
    top: 64px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 40;
    width: min(560px, calc(100% - 32px));
  }
  .notice {
    background: var(--surface-2);
    border: 1px solid var(--warn);
    border-radius: var(--radius);
    padding: 8px 12px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
</style>
```

- [ ] **Step 3: Build and try it**

Run: `cd web && npx vitest run && npm run build && cd .. && go vet ./... && go test ./...`
Expected: PASS. Then `go run ./cmd/tordo -dir .run` → *Listas*: the collections with their mosaics; a click opens the collection.

- [ ] **Step 4: Commit**

```bash
git add web/src
git commit -m "feat: Listas with the detected collections"
```

---
### Task 12: Documentation and check against the real collection

**Files:**
- Modify: `README.md`, `CLAUDE.md`
- Create: `docs/superpowers/notes/2026-10-01-m3a-collection-check.md`

- [ ] **Step 1: README**

In `README.md`, in *Use*, the end of the first paragraph becomes:

```markdown
binary for your system. The first run creates `config.json` with the sibling
folders as roots, scans them and opens Tordo in the browser (`-no-browser`
to skip that; the address is in the log).
```

After the paragraph about `ffprobe`, add:

```markdown
### Listening

Browse the folders as they are on disk (collections show their albums in
order), open an album and play it, shuffle a folder, queue tracks. The queue
and the volume are kept in the browser, per device. MP3, FLAC, AAC, Opus,
Vorbis and WAV play as they are, with seeking; WMA, ALAC, AIFF, APE and
WavPack need `ffmpeg` on the `PATH` and are converted on the fly (lossless to
FLAC, lossy to MP3), without seeking. Covers come from the album folder's
image or the files' embedded pictures; thumbnails are cached in
`cache/thumbs/`.
```

Add to the endpoints table:

```markdown
| `GET /api/play?album=&folder=&track=&shuffle=1` | the tracks to queue, in order (at most 5000) |
| `GET /stream/{trackId}` | a track's audio (`Range` for seeking; `?transcode=1` converts it) |
| `GET /api/covers/album/{id}?size=` | an album's picture, whole or as a 160/320/640 px thumbnail |
| `GET /api/covers/track/{id}?size=` | the picture embedded in a track's file |
```

and replace the whole *Development* section with:

````markdown
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
````

If M2 already changed the status line at the top of the README, mention the player there too.

- [ ] **Step 2: CLAUDE.md**

In *Commands*, after `go test ./...`:

```bash
(cd web && npm test)                # web app tests (Vitest)
(cd web && npm run build)           # web app into internal/server/dist/ (embedded; not committed)
```

and the `go run` line's comment becomes `# run with config and catalog in .run/ (build the web app first)`.

- [ ] **Step 3: Check against the real collection**

With the collection at `/Users/martin/Music` (or `TORDO_CORPUS`'s), `ffmpeg` installed:

```bash
TORDO_CORPUS=/Users/martin/Music go test ./internal/audiofile -run CorpusPictures -v -timeout 0
bash scripts/build.sh
go run ./cmd/tordo -dir .run
```

In the browser, desktop and phone width (375 px), check and note in `docs/superpowers/notes/2026-10-01-m3a-collection-check.md` (in Spanish, like the M1 note), with what failed or looked wrong:

1. Carpetas: the single root opens directly; how many folder cards have no picture, and whether those albums really have none (no `cover.*`, no embedded picture).
2. A collection (*Latinoamérica 600*): *Colección*, albums in their `[LA-n]` order; ▶ plays them in that order (look at the queue panel).
3. An MP3 album and a FLAC album: play, seek, next, previous; the hand-off between two tracks (no audible gap beyond the files' own silence).
4. A WMA album (Zitarrosa): plays converted, the slider disabled; without ffmpeg on the `PATH` (`PATH=/usr/bin:/bin go run ./cmd/tordo -dir .run`), the page says the tracks need ffmpeg.
5. Opus files from YouTube (loose tracks): play from the folder's loose tracks list, with their embedded thumbnail in the bar.
6. Reload in the middle of a track: the bar keeps the queue and resumes from the position.
7. Media keys or the lock screen of the phone (from another device this waits for M5's LAN mode; on the Mac, the media keys and the Now Playing widget).
8. Time to open the root folder the first time (thumbnails being made) and the second time (cached).

Things that need a decision go to the note's *Pendiente* section, for M3b or later.

- [ ] **Step 4: Commit**

```bash
git add README.md CLAUDE.md docs/superpowers/notes/2026-10-01-m3a-collection-check.md
git commit -m "docs: listening, web app build, new endpoints and the M3a collection check"
```
