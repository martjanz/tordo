# Tordo M1 (Núcleo) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go binary that scans 1–5 music folders incrementally, reads tags and technical data natively, fingerprints the audio payload, groups files into tracks → albums → folders → collections in a SQLite catalog, and exposes it through a localhost JSON API.

**Architecture:** Same shape as Cinexplorer (`/Users/martin/x-projects/cinexplorer`), but a separate repo: code is copied and adapted, never imported. The scan runs in three phases: (1) walk the roots and sync the `files` table, (2) read new or changed audio files (tags + technical data + payload fingerprint) in batches, (3) rebuild the derived tables (`albums`, `tracks`, `folders`) from the catalog with a pure grouping function. No UI in M1 (that's M3); the API is checked with `curl`.

**Tech Stack:** Go 1.27, `modernc.org/sqlite` (pure Go, no cgo), standard library only otherwise. Optional `ffprobe` on `PATH`.

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish). M1 covers §2, §3, §4, §5 steps 1–5 and the parts of §9 that apply to localhost. Read §4.2 and §4.3 (album grouping, folders and collections) before Tasks 14–16.

## Global Constraints

- Module name `tordo`; Go `1.27.0` in `go.mod`; only dependency `modernc.org/sqlite v1.59.0` (+ its indirect deps).
- No cgo (`CGO_ENABLED=0` must build).
- **Never write to the music roots.** The only writes go to the app folder: `tordo.db`, `config.json`.
- Catalog paths are relative to the app folder, `/`-separated (`internal/appdir`).
- SQLite journal mode `DELETE` (not WAL); `foreign_keys(1)`; `SetMaxOpenConns(1)` — so never run a query while another `*sql.Rows` is still open.
- Server listens on `127.0.0.1` only and rejects non-localhost `Host` and cross-site requests (copied `localOnly`); write endpoints require `Content-Type: application/json`.
- User-facing strings (logs, errors, JSON error text) in Spanish; identifiers and comments in English.
- `go test ./...` passes without network; `go vet ./...` clean.
- Commit messages: conventional style (`feat:`, `test:`…), **no `Co-Authored-By:` trailer**.

## File Structure

```
go.mod, .gitignore, README.md, scripts/build.sh, .github/workflows/ci.yml
cmd/tordo/main.go                 flags, setup, listen on 127.0.0.1, start first scan
internal/appdir/                  app folder, Rel/Abs, Writable (copied from Cinexplorer)
internal/natsort/                 natural, case-insensitive ordering
internal/config/                  config.json: roots, ignore
internal/mediafile/               classify files by extension; ignored folders
internal/audiofile/               one-pass native readers + ffprobe fallback
    audiofile.go                  Info, Tags, Range, errors, source, binary helpers
    text.go                       Latin-1/UTF-8/UTF-16 decoding
    tags.go                       Tags.set (cross-format tag names), parsing helpers
    genres.go                     ID3v1 genre table and references
    id3.go                        ID3v2.2/2.3/2.4, ID3v1, APE footer size
    mp3.go                        MPEG frame headers, Xing/VBRI, readMP3
    flac.go, vorbis.go            readFLAC, Vorbis comments
    ogg.go                        readOgg (Opus, Vorbis)
    mp4.go                        readMP4 (M4A/AAC/ALAC)
    asf.go                        readASF (WMA)
    riff.go                       readWAV, readAIFF
    ffprobe.go, read.go           fallback and dispatcher (Read, ReadNative)
    build_test.go                 byte builders shared by the tests
internal/fingerprint/             hash of the audio payload (plain and Ogg)
internal/nameparse/               folder and file names; Norm, StripDisc, IsVarious
internal/grouping/                tracks → albums → folders → collections (pure)
internal/store/                   SQLite: files, audio reads, derived tables, views
internal/scan/                    the three-phase scanner
internal/server/                  JSON API
```

---
### Task 1: Module scaffold, appdir and natsort

**Files:**
- Create: `go.mod`, `.gitignore`, `.github/workflows/ci.yml`
- Create: `internal/appdir/appdir.go`, `internal/appdir/appdir_test.go` (copied from Cinexplorer)
- Create: `internal/natsort/natsort.go`, `internal/natsort/natsort_test.go`

**Interfaces:**
- Produces: `appdir.Resolve(override string) (string, error)`, `appdir.Rel(appDir, abs string) (string, error)`, `appdir.Abs(appDir, rel string) string`, `appdir.Writable(dir string) bool`, `natsort.Less(a, b string) bool`

- [ ] **Step 1: Create the module and repo files**

```bash
cd /Users/martin/x-projects/tordo
go mod init tordo
go mod edit -go=1.27.0
mkdir -p internal/appdir internal/natsort .github/workflows
cp /Users/martin/x-projects/cinexplorer/internal/appdir/appdir.go internal/appdir/
cp /Users/martin/x-projects/cinexplorer/internal/appdir/appdir_test.go internal/appdir/
sed -i '' 's/\.cx-write-test-/.tordo-write-test-/' internal/appdir/appdir.go
sed -i '' 's/"cinexplorer"/"tordo"/; s/"cine", "1970s", "Amarcord.mkv"/"musica", "Uruguay", "Zitarrosa.mp3"/; s#../cine/1970s/Amarcord.mkv#../musica/Uruguay/Zitarrosa.mp3#' internal/appdir/appdir_test.go
```

`.gitignore` (replace the existing one):

```
.DS_Store
.run/
/dist/
*.db
.worktrees/
.claude/
```

`.github/workflows/ci.yml`:

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
```

- [ ] **Step 2: Run the copied appdir tests**

Run: `go test ./internal/appdir/ -v`
Expected: PASS (3 tests). If the `sed` on the test file did not match, edit it by hand so the paths read `musica/Uruguay/Zitarrosa.mp3`.

- [ ] **Step 3: Write the failing natsort test**

`internal/natsort/natsort_test.go`:

```go
package natsort

import (
	"sort"
	"testing"
)

func TestLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"[LA-9] Soda Stereo", "[LA-10] Charly García", true},
		{"[LA-10] Charly García", "[LA-9] Soda Stereo", false},
		{"track 2", "Track 10", true},
		{"CD 01", "cd 1", false},
		{"a", "b", true},
		{"abc", "abcd", true},
		{"same", "same", false},
		{"Disc 2", "Disc 2b", true},
	}
	for _, c := range cases {
		if got := Less(c.a, c.b); got != c.want {
			t.Errorf("Less(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSortStable(t *testing.T) {
	s := []string{"10 - x", "2 - x", "1 - x", "01 - y"}
	sort.SliceStable(s, func(i, j int) bool { return Less(s[i], s[j]) })
	want := []string{"1 - x", "01 - y", "2 - x", "10 - x"}
	for i := range want {
		if s[i] != want[i] {
			t.Fatalf("got %q, want %q", s, want)
		}
	}
}
```

- [ ] **Step 4: Run it to see it fail**

Run: `go test ./internal/natsort/`
Expected: FAIL (`undefined: Less`).

- [ ] **Step 5: Implement**

`internal/natsort/natsort.go`:

```go
// Package natsort orders names the way people expect: case-insensitively,
// with runs of digits compared as numbers ("[LA-9]" before "[LA-10]").
package natsort

import "strings"

// Less reports whether a sorts before b in natural order.
func Less(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			na, ra := digits(a)
			nb, rb := digits(b)
			ta, tb := strings.TrimLeft(na, "0"), strings.TrimLeft(nb, "0")
			if len(ta) != len(tb) {
				return len(ta) < len(tb)
			}
			if ta != tb {
				return ta < tb
			}
			a, b = ra, rb
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// digits splits s into its leading run of digits and the rest.
func digits(s string) (string, string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/... && go vet ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod .gitignore .github internal/appdir internal/natsort
git commit -m "feat: module scaffold, appdir and natural sort"
```

---

### Task 2: config.json

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.FileName = "config.json"`, `type config.Config struct { Roots []string; Ignore []string }`, `config.Load(appDir string) (cfg Config, created bool, err error)`, `config.Save(appDir string, cfg Config) error`, `config.DefaultRoots(appDir string) ([]string, error)`

Later milestones add fields (`discogsToken`, `lan`, `normalize`); unknown JSON fields are ignored by `encoding/json`, so M1 only defines what it uses.

- [ ] **Step 1: Write the failing test**

`internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func appUnder(t *testing.T, siblings ...string) string {
	t.Helper()
	parent := t.TempDir()
	app := filepath.Join(parent, "tordo")
	for _, d := range append([]string{"tordo"}, siblings...) {
		if err := os.MkdirAll(filepath.Join(parent, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func TestLoadMissingGivesDefaults(t *testing.T) {
	app := appUnder(t, "musica", "musica-ordenar", ".Trashes", "$RECYCLE.BIN", "System Volume Information")
	cfg, created, err := Load(app)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("created = false for a missing config.json")
	}
	want := []string{"../musica", "../musica-ordenar"}
	if !reflect.DeepEqual(cfg.Roots, want) {
		t.Fatalf("Roots = %q, want %q", cfg.Roots, want)
	}
	if cfg.Ignore == nil {
		t.Fatal("Ignore must be an empty slice, not nil")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	app := appUnder(t)
	in := Config{Roots: []string{"../musica"}, Ignore: []string{"Soulseek/incomplete"}}
	if err := Save(app, in); err != nil {
		t.Fatal(err)
	}
	out, created, err := Load(app)
	if err != nil || created {
		t.Fatalf("Load = %v, created %v", err, created)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("got %+v, want %+v", out, in)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	app := appUnder(t)
	if err := os.WriteFile(filepath.Join(app, FileName), []byte("{roots"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(app); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestLoadNullListsBecomeEmpty(t *testing.T) {
	app := appUnder(t)
	if err := os.WriteFile(filepath.Join(app, FileName), []byte(`{"roots":["../m"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(app)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ignore == nil {
		t.Fatal("Ignore must default to an empty slice")
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/config/`
Expected: FAIL (`undefined: Load`).

- [ ] **Step 3: Implement**

`internal/config/config.go`:

```go
// Package config reads and writes config.json in the app folder.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "config.json"

type Config struct {
	Roots  []string `json:"roots"`  // catalog form: relative to the app folder, '/'-separated
	Ignore []string `json:"ignore"` // extra folder patterns to skip (see mediafile.IgnoredDir)
}

// Load reads config.json from appDir. When the file does not exist it returns
// defaults (the sibling folders of appDir as roots) and created=true.
func Load(appDir string) (Config, bool, error) {
	data, err := os.ReadFile(filepath.Join(appDir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		roots, err := DefaultRoots(appDir)
		if err != nil {
			return Config{}, false, err
		}
		return Config{Roots: roots, Ignore: []string{}}, true, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", FileName, err)
	}
	if cfg.Roots == nil {
		cfg.Roots = []string{}
	}
	if cfg.Ignore == nil {
		cfg.Ignore = []string{}
	}
	return cfg, false, nil
}

func Save(appDir string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(appDir, FileName), append(data, '\n'), 0o644)
}

// DefaultRoots lists the sibling folders of appDir as "../<name>", skipping
// hidden and system folders.
func DefaultRoots(appDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(appDir))
	if err != nil {
		return nil, err
	}
	self := filepath.Base(appDir)
	roots := []string{}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == self || skipRoot(name) {
			continue
		}
		roots = append(roots, "../"+name)
	}
	return roots, nil
}

func skipRoot(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
		return true
	}
	switch strings.ToLower(name) {
	case "system volume information", "recycler", "lost+found":
		return true
	}
	return false
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/config/ -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat: config.json with default sibling roots"
```

---

### Task 3: File classification and ignored folders

**Files:**
- Create: `internal/mediafile/mediafile.go`, `internal/mediafile/mediafile_test.go`

**Interfaces:**
- Produces: `type mediafile.Kind string` with constants `Audio = "audio"`, `Image = "image"`, `Lyrics = "lyrics"`, `Playlist = "playlist"`, `None = ""`; `mediafile.Classify(name string) Kind`; `mediafile.IgnoredDir(rel string, extra []string) bool` where `rel` is the folder's path relative to its root, `/`-separated (e.g. `"Soulseek/downloading"`).

Only `Audio`, `Image`, `Lyrics` and `Playlist` files are stored in the catalog; everything else (`.epub`, `.pdf`, `.nfo`, `.DS_Store`, `.db`…) is `None` and skipped. Images are needed for album covers; lyrics and playlists are kept for later milestones.

- [ ] **Step 1: Write the failing test**

`internal/mediafile/mediafile_test.go`:

```go
package mediafile

import "testing"

func TestClassify(t *testing.T) {
	cases := map[string]Kind{
		"05-Fue Amor.mp3":                   Audio,
		"01 - Camelô.OPUS":                  Audio,
		"track.flac":                        Audio,
		"x.m4a":                             Audio,
		"x.wma":                             Audio,
		"x.wav":                             Audio,
		"x.aiff":                            Audio,
		"x.ape":                             Audio,
		"x.wv":                              Audio,
		"x.ogg":                             Audio,
		"cover.JPG":                         Image,
		"folder.png":                        Image,
		"x.webp":                            Image,
		"letra.lrc":                         Lyrics,
		"lista.m3u8":                        Playlist,
		"libro.epub":                        None,
		"info.nfo":                          None,
		".DS_Store":                         None,
		"Thumbs.db":                         None,
		"._05-Fue Amor.mp3":                 None, // macOS resource fork on exFAT
		"Music Library.musiclibrary.tagset": None,
	}
	for name, want := range cases {
		if got := Classify(name); got != want {
			t.Errorf("Classify(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestIgnoredDir(t *testing.T) {
	cases := []struct {
		rel   string
		extra []string
		want  bool
	}{
		{"Ableton", nil, true},
		{"Audio Music Apps", nil, true},
		{"djay", nil, true},
		{"Soulseek/downloading", nil, true},
		{"Music/Music Library.musiclibrary", nil, true},
		{"Music/Previous Libraries.localized", nil, true},
		{"Music/Media.localized", nil, false}, // Apple Music keeps real files here
		{".hidden", nil, true},
		{"Brasil/.AppleDouble", nil, true},
		{"$RECYCLE.BIN", nil, true},
		{"Brasil/Elza Soares", nil, false},
		{"Soulseek/complete", nil, false},
		{"Podcasts", []string{"podcasts"}, true},
		{"Brasil/Podcasts", []string{"podcasts"}, true},
		{"Soulseek/incomplete", []string{"Soulseek/incomplete/"}, true},
		{"Otro/incomplete", []string{"Soulseek/incomplete"}, false},
		{"Brasil/Samba", []string{"brasil/s*"}, true},
	}
	for _, c := range cases {
		if got := IgnoredDir(c.rel, c.extra); got != c.want {
			t.Errorf("IgnoredDir(%q, %q) = %v, want %v", c.rel, c.extra, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/mediafile/`
Expected: FAIL (`undefined: Classify`).

- [ ] **Step 3: Implement**

`internal/mediafile/mediafile.go`:

```go
// Package mediafile tells which files and folders of a music library matter.
package mediafile

import (
	"path"
	"strings"
)

type Kind string

const (
	Audio    Kind = "audio"
	Image    Kind = "image"
	Lyrics   Kind = "lyrics"
	Playlist Kind = "playlist"
	None     Kind = ""
)

var byExt = map[string]Kind{
	".mp3": Audio, ".flac": Audio, ".m4a": Audio, ".aac": Audio, ".opus": Audio, ".ogg": Audio, ".oga": Audio,
	".wma": Audio, ".wav": Audio, ".aif": Audio, ".aiff": Audio, ".aifc": Audio, ".ape": Audio, ".wv": Audio,
	".jpg": Image, ".jpeg": Image, ".png": Image, ".webp": Image,
	".lrc":  Lyrics,
	".m3u":  Playlist, ".m3u8": Playlist,
}

// Classify tells a file's kind from its name. Files of kind None are not catalogued.
func Classify(name string) Kind {
	if strings.HasPrefix(name, "._") {
		return None // AppleDouble resource forks that macOS leaves on exFAT/FAT drives
	}
	return byExt[strings.ToLower(path.Ext(name))]
}

// builtinIgnore are folders of other apps and of the system, matched against
// a folder's lowercased name.
var builtinIgnore = []string{
	"ableton", "audio music apps", "djay", "downloading", "*.musiclibrary",
	"previous libraries.localized", "$recycle.bin", "system volume information", "@eadir", "lost+found",
}

// IgnoredDir reports whether the folder at rel (relative to its root,
// '/'-separated) must not be walked: hidden folders, the built-in list, and
// the user's patterns. A pattern without '/' matches a folder name anywhere;
// one with '/' matches the folder's path from its root. Patterns use
// path.Match syntax and are case-insensitive.
func IgnoredDir(rel string, extra []string) bool {
	base := path.Base(rel)
	if strings.HasPrefix(base, ".") {
		return true
	}
	lb, lr := strings.ToLower(base), strings.ToLower(rel)
	for _, p := range builtinIgnore {
		if ok, _ := path.Match(p, lb); ok {
			return true
		}
	}
	for _, p := range extra {
		p = strings.ToLower(strings.Trim(p, "/"))
		target := lb
		if strings.Contains(p, "/") {
			target = lr
		}
		if ok, _ := path.Match(p, target); ok {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/mediafile/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mediafile
git commit -m "feat: classify library files and skip other apps' folders"
```

---
### Task 4: audiofile core types, tag mapping and ID3

This is the base of the one-pass reader package. Every format reader fills an `Info` (technical data + `Tags` + the byte range of the audio payload). Tag names from every format go through one mapping function, `Tags.set`, so ID3 `TXXX`, Vorbis comments, MP4 freeform atoms, ASF descriptors and ffprobe keys all land in the same fields.

**Files:**
- Create: `internal/audiofile/audiofile.go`, `internal/audiofile/text.go`, `internal/audiofile/tags.go`, `internal/audiofile/genres.go`, `internal/audiofile/id3.go`
- Create: `internal/audiofile/build_test.go` (byte builders, extended by later tasks), `internal/audiofile/tags_test.go`, `internal/audiofile/id3_test.go`

**Interfaces:**
- Produces (exported): `audiofile.Version = 1`; errors `ErrUnsupported`, `ErrFormat`, `ErrIO`; types `Tags`, `Range{Offset, Length int64}`, `Info`.
- Produces (package-internal, used by Tasks 5–11): `type source struct{ r io.ReaderAt; size int64 }` with `read(off int64, n int) ([]byte, error)`; `be16/be32/be64/le16/le32/le64`; `syncsafe([]byte) int64`; `latin1`, `text8`, `utf16Text(b []byte, bigEndian bool)`, `clean`; `(*Tags).set(name, value string)`, `(*Tags).addGenre(string)`, `first(*string, string)`, `firstInt(*int, int)`, `atoi`, `parsePair`, `parseYear`; `genreNames []string`, `id3Genres(string) []string`; `readID3v2(s source, off int64, t *Tags) (int64, error)`, `readID3v1(s source, t *Tags) (int64, error)`, `apeTagSize(s source, end int64) (int64, error)`.
- Test builders (in `build_test.go`): `src`, `cat`, `u16be`, `u32be`, `u16le`, `u32le`, `u64le`, `ssafe`, `id3Frame`, `id3TextFrame`, `id3Tag`, `id3v1`.

- [ ] **Step 1: Write the core types**

`internal/audiofile/audiofile.go`:

```go
// Package audiofile reads the tags and technical data of audio files in one
// pass: natively for MP3, FLAC, Ogg (Opus, Vorbis), MP4/M4A, ASF/WMA, WAV and
// AIFF, and with ffprobe (when installed) for anything else. It only reads.
package audiofile

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Version identifies the readers' behavior: files read by an older version
// are read again on the next scan.
const Version = 1

var (
	ErrUnsupported = errors.New("formato sin lector propio")
	ErrFormat      = errors.New("formato inválido")
	ErrIO          = errors.New("error de lectura")
)

// Tags are the metadata the catalog uses, whatever the format called them.
type Tags struct {
	Title         string   `json:"title,omitempty"`
	Artist        string   `json:"artist,omitempty"` // several artists joined with "; "
	AlbumArtist   string   `json:"albumArtist,omitempty"`
	Album         string   `json:"album,omitempty"`
	Track         int      `json:"track,omitempty"`
	TrackTotal    int      `json:"trackTotal,omitempty"`
	Disc          int      `json:"disc,omitempty"`
	DiscTotal     int      `json:"discTotal,omitempty"`
	Year          int      `json:"year,omitempty"`
	OriginalYear  int      `json:"originalYear,omitempty"`
	Genres        []string `json:"genres,omitempty"` // raw values, normalized in M2
	Compilation   bool     `json:"compilation,omitempty"`
	MBRecordingID string   `json:"mbRecordingId,omitempty"`
	MBAlbumID     string   `json:"mbAlbumId,omitempty"`
	MBArtistID    string   `json:"mbArtistId,omitempty"`
	TrackGain     *float64 `json:"trackGain,omitempty"` // ReplayGain, dB
	AlbumGain     *float64 `json:"albumGain,omitempty"`
	HasCover      bool     `json:"hasCover,omitempty"` // an embedded picture
}

// Range is a byte range of a file.
type Range struct {
	Offset int64 `json:"offset"`
	Length int64 `json:"length"`
}

type Info struct {
	Format     string // container: mp3, flac, ogg, mp4, asf, wav, aiff, or ffprobe's format name
	Codec      string // mp3, flac, opus, vorbis, aac, alac, wma, wmapro, wmalossless, pcm…
	Lossless   bool
	DurationMs int64
	Bitrate    int // kbps
	SampleRate int
	BitDepth   int // lossless formats only
	Channels   int
	Tags       Tags
	Payload    Range  // the audio data without tags or pictures: what gets fingerprinted
	Reader     string // "native" or "ffprobe"
}

// maxRead caps a single read, so a corrupt length field cannot allocate gigabytes.
const maxRead = 32 << 20

type source struct {
	r    io.ReaderAt
	size int64
}

// read returns n bytes at off. Reading past the end of the file is a format
// error (some length field lies); a failing read is an I/O error.
func (s source) read(off int64, n int) ([]byte, error) {
	if off < 0 || n < 0 || n > maxRead || off+int64(n) > s.size {
		return nil, fmt.Errorf("%w: %d bytes en %d, fuera del archivo", ErrFormat, n, off)
	}
	b := make([]byte, n)
	k, err := s.r.ReadAt(b, off)
	if k == n {
		return b, nil
	}
	return nil, fmt.Errorf("%w: %v", ErrIO, err)
}

func be16(b []byte) uint16 { return binary.BigEndian.Uint16(b) }
func be32(b []byte) uint32 { return binary.BigEndian.Uint32(b) }
func be64(b []byte) uint64 { return binary.BigEndian.Uint64(b) }
func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func le64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }

// syncsafe decodes a 28-bit ID3v2 "synchsafe" integer (7 bits per byte).
func syncsafe(b []byte) int64 {
	return int64(b[0]&0x7F)<<21 | int64(b[1]&0x7F)<<14 | int64(b[2]&0x7F)<<7 | int64(b[3]&0x7F)
}
```

`internal/audiofile/text.go`:

```go
package audiofile

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// text8 decodes 8-bit text: UTF-8 when valid, Latin-1 otherwise. Many taggers
// write UTF-8 into fields declared Latin-1; real Latin-1 with accents is
// almost never valid UTF-8.
func text8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return latin1(b)
}

// utf16Text decodes UTF-16 with an optional byte order mark; without one,
// bigEndian decides.
func utf16Text(b []byte, bigEndian bool) string {
	if len(b) >= 2 {
		switch {
		case b[0] == 0xFF && b[1] == 0xFE:
			bigEndian, b = false, b[2:]
		case b[0] == 0xFE && b[1] == 0xFF:
			bigEndian, b = true, b[2:]
		}
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		if bigEndian {
			u[i] = be16(b[2*i:])
		} else {
			u[i] = le16(b[2*i:])
		}
	}
	return string(utf16.Decode(u))
}

// clean drops surrounding NULs and spaces.
func clean(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r == 0 || unicode.IsSpace(r) })
}
```

- [ ] **Step 2: Write the test builders**

`internal/audiofile/build_test.go`:

```go
package audiofile

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// Builders of tiny, hand-made audio files for the tests. Payload bytes are
// filler: the readers never decode audio.

func src(b []byte) source { return source{bytes.NewReader(b), int64(len(b))} }

func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func u16be(v int) []byte   { return binary.BigEndian.AppendUint16(nil, uint16(v)) }
func u32be(v int) []byte   { return binary.BigEndian.AppendUint32(nil, uint32(v)) }
func u16le(v int) []byte   { return binary.LittleEndian.AppendUint16(nil, uint16(v)) }
func u32le(v int) []byte   { return binary.LittleEndian.AppendUint32(nil, uint32(v)) }
func u64le(v int64) []byte { return binary.LittleEndian.AppendUint64(nil, uint64(v)) }

func ssafe(n int) []byte {
	return []byte{byte(n>>21) & 0x7F, byte(n>>14) & 0x7F, byte(n>>7) & 0x7F, byte(n) & 0x7F}
}

// id3Frame builds an ID3v2.3 or v2.4 frame (v2.4 sizes are synchsafe).
func id3Frame(major byte, id string, data []byte) []byte {
	size := u32be(len(data))
	if major == 4 {
		size = ssafe(len(data))
	}
	return cat([]byte(id), size, []byte{0, 0}, data)
}

// id3TextFrame encodes values as UTF-8 (v2.4) or UTF-16 with BOM (v2.3).
func id3TextFrame(major byte, id string, values ...string) []byte {
	if major == 4 {
		return id3Frame(4, id, cat([]byte{3}, []byte(strings.Join(values, "\x00"))))
	}
	b := []byte{1}
	for i, v := range values {
		if i > 0 {
			b = append(b, 0, 0)
		}
		b = append(b, 0xFF, 0xFE)
		for _, u := range utf16.Encode([]rune(v)) {
			b = binary.LittleEndian.AppendUint16(b, u)
		}
	}
	return id3Frame(3, id, b)
}

// id3Tag wraps frames in an ID3v2 header, with 16 bytes of padding.
func id3Tag(major byte, frames ...[]byte) []byte {
	body := cat(append(frames, make([]byte, 16))...)
	return cat([]byte{'I', 'D', '3', major, 0, 0}, ssafe(len(body)), body)
}

func id3v1(title, artist, album, year string, track, genre byte) []byte {
	b := make([]byte, 128)
	copy(b, "TAG")
	copy(b[3:33], title)
	copy(b[33:63], artist)
	copy(b[63:93], album)
	copy(b[93:97], year)
	b[126], b[127] = track, genre
	return b
}
```

- [ ] **Step 3: Write the failing tag-mapping test**

`internal/audiofile/tags_test.go`:

```go
package audiofile

import (
	"reflect"
	"testing"
)

func TestSetMapsNamesFromEveryFormat(t *testing.T) {
	var tg Tags
	tg.set("TITLE", "Fue amor")
	tg.set("title", "ignored: first value wins")
	tg.set("ARTIST", "Fito Páez")
	tg.set("artist", "fito páez") // same artist, different case: not repeated
	tg.set("ARTIST", "Charly García")
	tg.set("album_artist", "Fito Páez")
	tg.set("WM/AlbumTitle", "Tercer mundo")
	tg.set("TRACKNUMBER", "05/12")
	tg.set("DISCNUMBER", "1")
	tg.set("TOTALDISCS", "2")
	tg.set("DATE", "1990-10-01")
	tg.set("ORIGINALDATE", "1990")
	tg.set("GENRE", "Rock")
	tg.set("genre", "rock")
	tg.set("GENRE", "Latino")
	tg.set("COMPILATION", "1")
	tg.set("MusicBrainz Album Id", "album-mbid")
	tg.set("MUSICBRAINZ_TRACKID", "recording-mbid")
	tg.set("REPLAYGAIN_TRACK_GAIN", "-6.54 dB")
	tg.set("unknown", "x")
	tg.set("ALBUM", "  \x00 ")

	gain := -6.54
	want := Tags{
		Title: "Fue amor", Artist: "Fito Páez; Charly García", AlbumArtist: "Fito Páez", Album: "Tercer mundo",
		Track: 5, TrackTotal: 12, Disc: 1, DiscTotal: 2, Year: 1990, OriginalYear: 1990,
		Genres: []string{"Rock", "Latino"}, Compilation: true,
		MBAlbumID: "album-mbid", MBRecordingID: "recording-mbid", TrackGain: &gain,
	}
	if !reflect.DeepEqual(tg, want) {
		t.Fatalf("got  %+v\nwant %+v", tg, want)
	}
}

func TestParseHelpers(t *testing.T) {
	if n, total := parsePair("3/12"); n != 3 || total != 12 {
		t.Errorf("parsePair = %d, %d", n, total)
	}
	if n, total := parsePair("x"); n != 0 || total != 0 {
		t.Errorf("parsePair(x) = %d, %d", n, total)
	}
	for in, want := range map[string]int{"1990": 1990, "2008-05-01": 2008, "c. 1957": 1957, "20080501": 2008, "123456": 0, "": 0, "3000": 0} {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestID3Genres(t *testing.T) {
	cases := map[string][]string{
		"(17)":     {"Rock"},
		"(114)":    {"Samba"},
		"(17)Rock": {"Rock", "Rock"}, // addGenre removes the duplicate later
		"113":      {"Tango"},
		"(RX)":     {"Remix"},
		"Murga":    {"Murga"},
		"(999)":    nil,
	}
	for in, want := range cases {
		if got := id3Genres(in); !reflect.DeepEqual(got, want) {
			t.Errorf("id3Genres(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 4: Run it to see it fail**

Run: `go test ./internal/audiofile/`
Expected: FAIL (`undefined: Tags.set` / `parsePair` / `id3Genres`).

- [ ] **Step 5: Implement the tag mapping**

`internal/audiofile/tags.go`:

```go
package audiofile

import (
	"regexp"
	"strconv"
	"strings"
)

// fold reduces a tag name to lowercase letters and digits, so "ALBUM ARTIST",
// "album_artist" and "AlbumArtist" are the same, and "WM/AlbumTitle" becomes
// "wmalbumtitle".
func fold(name string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(name) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// set applies one tag by name. Names from every format (Vorbis comments,
// ID3 TXXX descriptions, MP4 freeform names, ASF attributes, ffprobe keys)
// are folded and mapped to fields; unknown names are ignored. The first value
// of a field wins, except artist and genre, which keep every distinct value.
func (t *Tags) set(name, value string) {
	value = clean(value)
	if value == "" {
		return
	}
	switch fold(name) {
	case "title":
		first(&t.Title, value)
	case "artist", "author":
		t.Artist = addValue(t.Artist, value)
	case "albumartist", "wmalbumartist":
		first(&t.AlbumArtist, value)
	case "album", "wmalbumtitle":
		first(&t.Album, value)
	case "tracknumber", "track", "wmtracknumber":
		n, total := parsePair(value)
		firstInt(&t.Track, n)
		firstInt(&t.TrackTotal, total)
	case "tracktotal", "totaltracks":
		firstInt(&t.TrackTotal, atoi(value))
	case "discnumber", "disc", "wmpartofset":
		n, total := parsePair(value)
		firstInt(&t.Disc, n)
		firstInt(&t.DiscTotal, total)
	case "disctotal", "totaldiscs":
		firstInt(&t.DiscTotal, atoi(value))
	case "date", "year", "wmyear":
		firstInt(&t.Year, parseYear(value))
	case "originaldate", "originalyear", "wmoriginalreleaseyear":
		firstInt(&t.OriginalYear, parseYear(value))
	case "genre", "wmgenre":
		t.addGenre(value)
	case "compilation", "wmiscompilation":
		t.Compilation = t.Compilation || value == "1" || strings.EqualFold(value, "true")
	case "musicbrainztrackid":
		first(&t.MBRecordingID, value)
	case "musicbrainzalbumid":
		first(&t.MBAlbumID, value)
	case "musicbrainzartistid":
		first(&t.MBArtistID, value)
	case "replaygaintrackgain":
		if t.TrackGain == nil {
			t.TrackGain = parseGain(value)
		}
	case "replaygainalbumgain":
		if t.AlbumGain == nil {
			t.AlbumGain = parseGain(value)
		}
	}
}

func first(dst *string, v string) {
	if *dst == "" {
		*dst = v
	}
}

func firstInt(dst *int, v int) {
	if *dst == 0 {
		*dst = v
	}
}

const valueSep = "; "

// addValue appends v to a "; "-joined list unless it is already there.
func addValue(cur, v string) string {
	if cur == "" {
		return v
	}
	for _, x := range strings.Split(cur, valueSep) {
		if strings.EqualFold(x, v) {
			return cur
		}
	}
	return cur + valueSep + v
}

func (t *Tags) addGenre(v string) {
	v = clean(v)
	if v == "" {
		return
	}
	for _, g := range t.Genres {
		if strings.EqualFold(g, v) {
			return
		}
	}
	t.Genres = append(t.Genres, v)
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// parsePair reads "3/12" (number and total) or "3".
func parsePair(s string) (int, int) {
	a, b, _ := strings.Cut(s, "/")
	return atoi(a), atoi(b)
}

var yearRe = regexp.MustCompile(`(?:^|\D)(1\d{3}|20\d{2})(?:\d{4})?(?:\D|$)`)

// parseYear finds a year (1000–2099) in a date: "1990", "2008-05-01",
// "20231105" (yt-dlp writes upload dates like that).
func parseYear(s string) int {
	m := yearRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// parseGain reads a ReplayGain value: "-6.54 dB".
func parseGain(s string) *float64 {
	s = strings.TrimSpace(s)
	if len(s) > 2 && strings.EqualFold(s[len(s)-2:], "db") {
		s = strings.TrimSpace(s[:len(s)-2])
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}
```

`internal/audiofile/genres.go`:

```go
package audiofile

import (
	"strconv"
	"strings"
)

// genreNames is the ID3v1 genre table (0–79 standard, 80–191 Winamp
// extensions). ID3v2 TCON, MP4 gnre and ID3v1 refer to it by number.
var genreNames = []string{
	"Blues", "Classic Rock", "Country", "Dance", "Disco", "Funk", "Grunge", "Hip-Hop", "Jazz", "Metal",
	"New Age", "Oldies", "Other", "Pop", "R&B", "Rap", "Reggae", "Rock", "Techno", "Industrial",
	"Alternative", "Ska", "Death Metal", "Pranks", "Soundtrack", "Euro-Techno", "Ambient", "Trip-Hop", "Vocal", "Jazz+Funk",
	"Fusion", "Trance", "Classical", "Instrumental", "Acid", "House", "Game", "Sound Clip", "Gospel", "Noise",
	"Alternative Rock", "Bass", "Soul", "Punk", "Space", "Meditative", "Instrumental Pop", "Instrumental Rock", "Ethnic", "Gothic",
	"Darkwave", "Techno-Industrial", "Electronic", "Pop-Folk", "Eurodance", "Dream", "Southern Rock", "Comedy", "Cult", "Gangsta",
	"Top 40", "Christian Rap", "Pop/Funk", "Jungle", "Native American", "Cabaret", "New Wave", "Psychedelic", "Rave", "Showtunes",
	"Trailer", "Lo-Fi", "Tribal", "Acid Punk", "Acid Jazz", "Polka", "Retro", "Musical", "Rock & Roll", "Hard Rock",
	"Folk", "Folk-Rock", "National Folk", "Swing", "Fast Fusion", "Bebop", "Latin", "Revival", "Celtic", "Bluegrass",
	"Avantgarde", "Gothic Rock", "Progressive Rock", "Psychedelic Rock", "Symphonic Rock", "Slow Rock", "Big Band", "Chorus", "Easy Listening", "Acoustic",
	"Humour", "Speech", "Chanson", "Opera", "Chamber Music", "Sonata", "Symphony", "Booty Bass", "Primus", "Porn Groove",
	"Satire", "Slow Jam", "Club", "Tango", "Samba", "Folklore", "Ballad", "Power Ballad", "Rhythmic Soul", "Freestyle",
	"Duet", "Punk Rock", "Drum Solo", "A Cappella", "Euro-House", "Dance Hall", "Goa", "Drum & Bass", "Club-House", "Hardcore",
	"Terror", "Indie", "Britpop", "Afro-Punk", "Polsk Punk", "Beat", "Christian Gangsta Rap", "Heavy Metal", "Black Metal", "Crossover",
	"Contemporary Christian", "Christian Rock", "Merengue", "Salsa", "Thrash Metal", "Anime", "JPop", "Synthpop", "Abstract", "Art Rock",
	"Baroque", "Bhangra", "Big Beat", "Breakbeat", "Chillout", "Downtempo", "Dub", "EBM", "Eclectic", "Electro",
	"Electroclash", "Emo", "Experimental", "Garage", "Global", "IDM", "Illbient", "Industro-Goth", "Jam Band", "Krautrock",
	"Leftfield", "Lounge", "Math Rock", "New Romantic", "Nu-Breakz", "Post-Punk", "Post-Rock", "Psytrance", "Shoegaze", "Space Rock",
	"Trop Rock", "World Music", "Neoclassical", "Audiobook", "Audio Theatre", "Neue Deutsche Welle", "Podcast", "Indie Rock", "G-Funk", "Dubstep",
	"Garage Rock", "Psybient",
}

// id3Genres expands ID3v1 genre references in a TCON value — "(17)",
// "(17)Rock", "17", "(RX)", "(CR)" — and returns any other text as is.
func id3Genres(s string) []string {
	var out []string
	for strings.HasPrefix(s, "(") {
		end := strings.IndexByte(s, ')')
		if end < 0 {
			break
		}
		ref := s[1:end]
		s = s[end+1:]
		switch ref {
		case "RX":
			out = append(out, "Remix")
		case "CR":
			out = append(out, "Cover")
		default:
			if n, err := strconv.Atoi(ref); err == nil && n >= 0 && n < len(genreNames) {
				out = append(out, genreNames[n])
			}
		}
	}
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		if n >= 0 && n < len(genreNames) {
			out = append(out, genreNames[n])
		}
		return out
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}
```

Check the table length: `len(genreNames)` must be 192 (indexes 0–191). Add this assertion to `tags_test.go`:

```go
func TestGenreTable(t *testing.T) {
	if len(genreNames) != 192 {
		t.Fatalf("len(genreNames) = %d, want 192", len(genreNames))
	}
	for i, want := range map[int]string{17: "Rock", 86: "Latin", 113: "Tango", 114: "Samba", 115: "Folklore", 191: "Psybient"} {
		if genreNames[i] != want {
			t.Errorf("genreNames[%d] = %q, want %q", i, genreNames[i], want)
		}
	}
}
```

- [ ] **Step 6: Run the tag tests**

Run: `go test ./internal/audiofile/ -run 'TestSet|TestParse|TestID3Genres|TestGenreTable' -v`
Expected: PASS.

- [ ] **Step 7: Write the failing ID3 test**

`internal/audiofile/id3_test.go`:

```go
package audiofile

import (
	"errors"
	"testing"
)

func TestID3v23(t *testing.T) {
	tag := id3Tag(3,
		id3TextFrame(3, "TIT2", "Só pra contrariar"),
		id3TextFrame(3, "TPE1", "Noel Rosa"),
		id3TextFrame(3, "TALB", "Noel pela primeira vez"),
		id3TextFrame(3, "TRCK", "16/20"),
		id3TextFrame(3, "TPOS", "2/3"),
		id3TextFrame(3, "TYER", "2000"),
		id3TextFrame(3, "TCON", "(114)"),
		id3TextFrame(3, "TXXX", "MusicBrainz Album Id", "album-mbid"),
		id3Frame(3, "UFID", cat([]byte("http://musicbrainz.org\x00"), []byte("rec-mbid"))),
		id3Frame(3, "APIC", []byte{0, 'i', 'm', 'a', 'g', 'e', '/', 'j', 'p', 'e', 'g', 0, 3, 0, 0xFF, 0xD8}),
	)
	file := cat(tag, []byte("audio"))
	var tg Tags
	n, err := readID3v2(src(file), 0, &tg)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(tag)) {
		t.Errorf("size = %d, want %d", n, len(tag))
	}
	if tg.Title != "Só pra contrariar" || tg.Artist != "Noel Rosa" || tg.Album != "Noel pela primeira vez" {
		t.Errorf("text = %+v", tg)
	}
	if tg.Track != 16 || tg.TrackTotal != 20 || tg.Disc != 2 || tg.DiscTotal != 3 || tg.Year != 2000 {
		t.Errorf("numbers = %+v", tg)
	}
	if len(tg.Genres) != 1 || tg.Genres[0] != "Samba" {
		t.Errorf("genres = %q", tg.Genres)
	}
	if tg.MBAlbumID != "album-mbid" || tg.MBRecordingID != "rec-mbid" || !tg.HasCover {
		t.Errorf("ids/cover = %+v", tg)
	}
}

func TestID3v24MultipleValuesAndUTF8InLatin1(t *testing.T) {
	tag := id3Tag(4,
		id3TextFrame(4, "TPE1", "Caetano Veloso", "Maria Bethânia"),
		id3TextFrame(4, "TDRC", "1978-01-01"),
		id3Frame(4, "TALB", cat([]byte{0}, []byte("Tercer Mundo (Páez)"))), // UTF-8 bytes declared Latin-1
	)
	var tg Tags
	if _, err := readID3v2(src(tag), 0, &tg); err != nil {
		t.Fatal(err)
	}
	if tg.Artist != "Caetano Veloso; Maria Bethânia" {
		t.Errorf("Artist = %q", tg.Artist)
	}
	if tg.Year != 1978 {
		t.Errorf("Year = %d", tg.Year)
	}
	if tg.Album != "Tercer Mundo (Páez)" {
		t.Errorf("Album = %q", tg.Album)
	}
}

func TestID3v22(t *testing.T) {
	frame := func(id, text string) []byte {
		data := cat([]byte{0}, []byte(text))
		return cat([]byte(id), []byte{0, 0, byte(len(data))}, data)
	}
	body := cat(frame("TT2", "Milonga triste"), frame("TP1", "Alfredo Zitarrosa"), make([]byte, 10))
	tag := cat([]byte{'I', 'D', '3', 2, 0, 0}, ssafe(len(body)), body)
	var tg Tags
	if _, err := readID3v2(src(tag), 0, &tg); err != nil {
		t.Fatal(err)
	}
	if tg.Title != "Milonga triste" || tg.Artist != "Alfredo Zitarrosa" {
		t.Errorf("tags = %+v", tg)
	}
}

func TestNoID3v2(t *testing.T) {
	var tg Tags
	n, err := readID3v2(src([]byte("fLaC and more bytes")), 0, &tg)
	if n != 0 || err != nil {
		t.Fatalf("n = %d, err = %v", n, err)
	}
}

func TestTruncatedID3v2(t *testing.T) {
	tag := id3Tag(3, id3TextFrame(3, "TIT2", "x"))
	var tg Tags
	if _, err := readID3v2(src(tag[:len(tag)-5]), 0, &tg); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestID3v1FillsOnlyEmptyFields(t *testing.T) {
	file := cat([]byte("audio bytes"), id3v1("Retirada", "La Nueva Milonga", "Murgas", "1985", 7, 113))
	tg := Tags{Title: "From ID3v2"}
	n, err := readID3v1(src(file), &tg)
	if err != nil || n != 128 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
	if tg.Title != "From ID3v2" || tg.Artist != "La Nueva Milonga" || tg.Album != "Murgas" || tg.Year != 1985 || tg.Track != 7 {
		t.Errorf("tags = %+v", tg)
	}
	if len(tg.Genres) != 1 || tg.Genres[0] != "Tango" {
		t.Errorf("genres = %q", tg.Genres)
	}
}

func TestAPETagSize(t *testing.T) {
	items := []byte("0123456789")
	footer := cat([]byte("APETAGEX"), u32le(2000), u32le(len(items)+32), u32le(1), u32le(1<<31), make([]byte, 8))
	header := cat([]byte("APETAGEX"), u32le(2000), u32le(len(items)+32), u32le(1), u32le(1<<31|1<<29), make([]byte, 8))
	file := cat([]byte("audio"), header, items, footer)
	n, err := apeTagSize(src(file), int64(len(file)))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(header)+len(items)+len(footer)) {
		t.Fatalf("n = %d", n)
	}
	if n, _ := apeTagSize(src([]byte("no tag here, just bytes, at least 32 long")), 41); n != 0 {
		t.Fatalf("n = %d without a tag", n)
	}
}
```

- [ ] **Step 8: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run 'ID3|APE'`
Expected: FAIL (`undefined: readID3v2`).

- [ ] **Step 9: Implement ID3**

`internal/audiofile/id3.go`:

```go
package audiofile

import (
	"bytes"
	"fmt"
)

// v22Names maps ID3v2.2 three-letter frame ids to their v2.3/2.4 names.
var v22Names = map[string]string{
	"TT2": "TIT2", "TP1": "TPE1", "TP2": "TPE2", "TAL": "TALB", "TRK": "TRCK", "TPA": "TPOS",
	"TYE": "TYER", "TOR": "TORY", "TCO": "TCON", "TCP": "TCMP", "TXX": "TXXX", "PIC": "APIC", "UFI": "UFID",
}

// id3Fields maps ID3 text frames to Tags.set names.
var id3Fields = map[string]string{
	"TIT2": "title", "TPE1": "artist", "TPE2": "albumartist", "TALB": "album", "TRCK": "tracknumber",
	"TPOS": "discnumber", "TYER": "date", "TDRC": "date", "TDOR": "originaldate", "TORY": "originaldate",
	"TCMP": "compilation",
}

// readID3v2 parses the ID3v2 tag at off into t and returns its total size
// (header, frames, padding and footer), or 0 when there is no tag at off.
// Compressed and encrypted frames are skipped.
func readID3v2(s source, off int64, t *Tags) (int64, error) {
	if off+10 > s.size {
		return 0, nil
	}
	h, err := s.read(off, 10)
	if err != nil {
		return 0, err
	}
	if string(h[:3]) != "ID3" {
		return 0, nil
	}
	major, flags := h[3], h[5]
	size := syncsafe(h[6:10])
	total := 10 + size
	if flags&0x10 != 0 {
		total += 10 // footer
	}
	if off+total > s.size {
		return 0, fmt.Errorf("%w: etiqueta ID3v2 truncada", ErrFormat)
	}
	if major < 2 || major > 4 {
		return total, nil // unknown version: skip it whole
	}
	body, err := s.read(off+10, int(size))
	if err != nil {
		return 0, err
	}
	if major < 4 && flags&0x80 != 0 {
		body = unsync(body)
	}
	pos := 0
	if flags&0x40 != 0 && major >= 3 && len(body) >= 4 { // extended header
		if major == 3 {
			pos = 4 + int(be32(body))
		} else {
			pos = int(syncsafe(body))
		}
	}
	idLen, hdrLen := 4, 10
	if major == 2 {
		idLen, hdrLen = 3, 6
	}
	for pos >= 0 && pos+hdrLen <= len(body) {
		fh := body[pos : pos+hdrLen]
		if !validFrameID(fh[:idLen]) {
			break // padding, or garbage
		}
		id := string(fh[:idLen])
		var n int
		var fflags uint16
		switch major {
		case 2:
			n = int(fh[3])<<16 | int(fh[4])<<8 | int(fh[5])
		case 3:
			n, fflags = int(be32(fh[4:])), be16(fh[8:])
		case 4:
			n, fflags = int(syncsafe(fh[4:])), be16(fh[8:])
		}
		pos += hdrLen
		if n < 0 || pos+n > len(body) {
			break
		}
		data := body[pos : pos+n]
		pos += n
		switch major {
		case 3:
			if fflags&0x00C0 != 0 {
				continue // compressed or encrypted
			}
			if fflags&0x0020 != 0 && len(data) > 0 {
				data = data[1:] // group id
			}
		case 4:
			if fflags&0x000C != 0 {
				continue // compressed or encrypted
			}
			if fflags&0x0040 != 0 && len(data) > 0 {
				data = data[1:] // group id
			}
			if fflags&0x0001 != 0 && len(data) >= 4 {
				data = data[4:] // data length indicator
			}
			if fflags&0x0002 != 0 {
				data = unsync(data)
			}
		}
		if v2, ok := v22Names[id]; ok {
			id = v2
		}
		t.id3Frame(id, data)
	}
	return total, nil
}

func validFrameID(id []byte) bool {
	for _, c := range id {
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func (t *Tags) id3Frame(id string, data []byte) {
	switch id {
	case "APIC":
		t.HasCover = true
	case "UFID":
		owner, rest, ok := bytes.Cut(data, []byte{0})
		if ok && string(owner) == "http://musicbrainz.org" {
			t.set("musicbrainztrackid", string(rest))
		}
	case "TXXX":
		if v := id3Text(data); len(v) >= 2 {
			t.set(v[0], v[1])
		}
	case "TCON":
		for _, v := range id3Text(data) {
			for _, g := range id3Genres(v) {
				t.addGenre(g)
			}
		}
	default:
		if name, ok := id3Fields[id]; ok {
			for _, v := range id3Text(data) {
				t.set(name, v)
			}
		}
	}
}

// id3Text decodes a text frame: an encoding byte, then one or more values
// separated by terminators (v2.4 allows several values).
func id3Text(data []byte) []string {
	if len(data) < 1 {
		return nil
	}
	enc, b := data[0], data[1:]
	var parts [][]byte
	if enc == 1 || enc == 2 {
		start := 0
		for i := 0; i+1 < len(b); i += 2 {
			if b[i] == 0 && b[i+1] == 0 {
				parts = append(parts, b[start:i])
				start = i + 2
			}
		}
		parts = append(parts, b[start:])
	} else {
		parts = bytes.Split(b, []byte{0})
	}
	var out []string
	for _, p := range parts {
		var s string
		switch enc {
		case 1:
			s = utf16Text(p, false)
		case 2:
			s = utf16Text(p, true)
		default: // 0 (Latin-1, often UTF-8 in practice) and 3 (UTF-8)
			s = text8(p)
		}
		out = append(out, clean(s))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// unsync undoes ID3 unsynchronisation: every 0xFF 0x00 becomes 0xFF.
func unsync(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		out = append(out, b[i])
		if b[i] == 0xFF && i+1 < len(b) && b[i+1] == 0x00 {
			i++
		}
	}
	return out
}

// readID3v1 fills the fields of t that are still empty from an ID3v1 tag at
// the end of the file, and returns the tag's size (128) or 0 when there is none.
func readID3v1(s source, t *Tags) (int64, error) {
	if s.size < 128 {
		return 0, nil
	}
	b, err := s.read(s.size-128, 128)
	if err != nil {
		return 0, err
	}
	if string(b[:3]) != "TAG" {
		return 0, nil
	}
	field := func(f []byte) string {
		if i := bytes.IndexByte(f, 0); i >= 0 {
			f = f[:i]
		}
		return clean(text8(f))
	}
	first(&t.Title, field(b[3:33]))
	first(&t.Artist, field(b[33:63]))
	first(&t.Album, field(b[63:93]))
	firstInt(&t.Year, parseYear(field(b[93:97])))
	if b[125] == 0 && b[126] != 0 {
		firstInt(&t.Track, int(b[126]))
	}
	if len(t.Genres) == 0 && int(b[127]) < len(genreNames) {
		t.Genres = []string{genreNames[b[127]]}
	}
	return 128, nil
}

// apeTagSize returns the size of an APE tag that ends at end (its 32-byte
// footer starts with "APETAGEX"), including its header when it has one, or 0.
func apeTagSize(s source, end int64) (int64, error) {
	if end < 32 {
		return 0, nil
	}
	f, err := s.read(end-32, 32)
	if err != nil {
		return 0, err
	}
	if string(f[:8]) != "APETAGEX" {
		return 0, nil
	}
	n := int64(le32(f[12:16])) // items and footer
	if le32(f[20:24])&(1<<31) != 0 {
		n += 32 // header
	}
	if n > end {
		return 0, fmt.Errorf("%w: etiqueta APE inválida", ErrFormat)
	}
	return n, nil
}
```

- [ ] **Step 10: Run all audiofile tests**

Run: `go test ./internal/audiofile/ -v && go vet ./internal/audiofile/`
Expected: PASS.

- [ ] **Step 11: Commit**

```bash
git add internal/audiofile
git commit -m "feat: audiofile core types, cross-format tag mapping and ID3 tags"
```

---
### Task 5: MP3 reader

**Files:**
- Create: `internal/audiofile/mp3.go`, `internal/audiofile/mp3_test.go`
- Modify: `internal/audiofile/build_test.go` (append MP3 builders)

**Interfaces:**
- Consumes: `source`, `readID3v2`, `readID3v1`, `apeTagSize` (Task 4).
- Produces: `readMP3(s source) (Info, error)`.

The audio payload of an MP3 runs from the first real MPEG frame to the end of the file, minus a trailing ID3v1 tag and APE tag. A frame header counts as real only when another frame of the same kind follows it (stray `0xFFE` bit patterns are common in leftover tag bytes). Duration comes from a Xing/Info or VBRI header when present and consistent: the header also records the byte count, and when that is more than 10% off the real payload (a file edited after encoding) the frame count is ignored. Otherwise duration is the payload size over the mean bitrate of frames sampled at four points of the file (a VBR file without a header often starts with low-bitrate silence, so the first frame alone misleads). Both cases come from the real collection.

- [ ] **Step 1: Append the MP3 builders to `build_test.go`**

```go
// mp3Frame is one MPEG-1 layer III frame: 128 kbps, 44.1 kHz, stereo, no
// padding (417 bytes), filled with fill (avoid 0xFF).
func mp3Frame(fill byte) []byte {
	f := bytes.Repeat([]byte{fill}, 417)
	f[0], f[1], f[2], f[3] = 0xFF, 0xFB, 0x90, 0x00
	return f
}

func mp3Frames(n int, fill byte) []byte {
	var b []byte
	for i := 0; i < n; i++ {
		b = append(b, mp3Frame(fill)...)
	}
	return b
}

// xingFrame is a first frame carrying a Xing (or "Info") header with the
// frame count and, when bytes > 0, the byte count.
func xingFrame(tag string, frames, bytes int) []byte {
	f := mp3Frame(0)
	copy(f[36:], tag) // 4-byte header + 32 bytes of side info (MPEG-1 stereo)
	flags := 1
	if bytes > 0 {
		flags |= 2
		copy(f[48:], u32be(bytes))
	}
	copy(f[40:], u32be(flags))
	copy(f[44:], u32be(frames))
	return f
}

// mp3FramesAt builds n MPEG-1 layer III 44.1 kHz stereo frames at a bitrate
// index (1 = 32 kbps, 9 = 128 kbps, 11 = 192 kbps).
func mp3FramesAt(n, brIdx int, fill byte) []byte {
	br := []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192}[brIdx]
	var b []byte
	for i := 0; i < n; i++ {
		f := bytes.Repeat([]byte{fill}, 144*br*1000/44100)
		f[0], f[1], f[2], f[3] = 0xFF, 0xFB, byte(brIdx<<4), 0x00
		b = append(b, f...)
	}
	return b
}
```

- [ ] **Step 2: Write the failing test**

`internal/audiofile/mp3_test.go`:

```go
package audiofile

import (
	"errors"
	"testing"
)

func TestMP3CBRWithID3v2(t *testing.T) {
	tag := id3Tag(3, id3TextFrame(3, "TIT2", "Fue amor"), id3TextFrame(3, "TPE1", "Fito Páez"))
	audio := mp3Frames(100, 0x11)
	info, err := readMP3(src(cat(tag, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "mp3" || info.Codec != "mp3" || info.Lossless {
		t.Errorf("format = %+v", info)
	}
	if info.SampleRate != 44100 || info.Channels != 2 || info.Bitrate != 128 {
		t.Errorf("tech = %d Hz, %d ch, %d kbps", info.SampleRate, info.Channels, info.Bitrate)
	}
	if info.DurationMs != 2606 { // 41700 bytes * 8 / 128 kbps
		t.Errorf("DurationMs = %d, want 2606", info.DurationMs)
	}
	if info.Payload != (Range{int64(len(tag)), int64(len(audio))}) {
		t.Errorf("Payload = %+v, want {%d %d}", info.Payload, len(tag), len(audio))
	}
	if info.Tags.Title != "Fue amor" || info.Tags.Artist != "Fito Páez" {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestMP3XingDuration(t *testing.T) {
	info, err := readMP3(src(cat(xingFrame("Xing", 100, 0), mp3Frames(100, 0x22))))
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationMs != 2612 { // 100 frames * 1152 samples / 44100 Hz
		t.Errorf("DurationMs = %d, want 2612", info.DurationMs)
	}
}

func TestMP3InconsistentInfoHeaderIsIgnored(t *testing.T) {
	// The header says 100 frames in 10000 bytes; the file has 101 frames
	// (42117 bytes): it was edited after encoding.
	info, err := readMP3(src(cat(xingFrame("Info", 100, 10000), mp3Frames(100, 0x22))))
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationMs != 2632 { // 42117 bytes * 8 / 128 kbps
		t.Errorf("DurationMs = %d, want 2632", info.DurationMs)
	}
}

func TestMP3VBRWithoutHeaderUsesMeanBitrate(t *testing.T) {
	// 20 frames of 32 kbps silence, then 200 frames at 192 kbps: 220 frames,
	// 5747 ms. The first frame alone would say 31.8 s.
	info, err := readMP3(src(cat(mp3FramesAt(20, 1, 0x11), mp3FramesAt(200, 11, 0x22))))
	if err != nil {
		t.Fatal(err)
	}
	if d := info.DurationMs - 5747; d > 170 || d < -170 {
		t.Errorf("DurationMs = %d, want about 5747", info.DurationMs)
	}
}

func TestMP3TrailingTagsAreNotPayload(t *testing.T) {
	audio := mp3Frames(10, 0x33)
	items := []byte("0123456789")
	ape := cat(items, []byte("APETAGEX"), u32le(2000), u32le(len(items)+32), u32le(1), u32le(0), make([]byte, 8))
	v1 := id3v1("Retirada", "La Nueva Milonga", "", "1985", 3, 113)
	info, err := readMP3(src(cat(audio, ape, v1)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Payload != (Range{0, int64(len(audio))}) {
		t.Errorf("Payload = %+v, want {0 %d}", info.Payload, len(audio))
	}
	if info.Tags.Title != "Retirada" || info.Tags.Track != 3 {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestMP3SkipsFalseSync(t *testing.T) {
	junk := []byte{0xFF, 0xFB, 0x90, 0x00, 1, 2, 3} // a header not followed by another frame
	audio := mp3Frames(5, 0x44)
	info, err := readMP3(src(cat(junk, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Payload.Offset != int64(len(junk)) {
		t.Errorf("Payload.Offset = %d, want %d", info.Payload.Offset, len(junk))
	}
}

func TestMP3NotMPEG(t *testing.T) {
	if _, err := readMP3(src([]byte("this is not an mp3 file at all"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestParseMPEGFrame(t *testing.T) {
	f, ok := parseMPEGFrame([]byte{0xFF, 0xFB, 0x90, 0x00})
	if !ok || f.version != 1 || f.layer != 3 || f.bitrate != 128 || f.sampleRate != 44100 || f.length != 417 || f.samples != 1152 {
		t.Errorf("MPEG1 L3 = %+v, %v", f, ok)
	}
	// MPEG-2 layer III, 64 kbps, 22.05 kHz, mono, padded: 72*64000/22050 + 1 = 209
	f, ok = parseMPEGFrame([]byte{0xFF, 0xF3, 0x82, 0xC0})
	if !ok || f.version != 2 || f.bitrate != 64 || f.sampleRate != 22050 || f.channels != 1 || f.length != 209 || f.samples != 576 {
		t.Errorf("MPEG2 L3 = %+v, %v", f, ok)
	}
	if _, ok := parseMPEGFrame([]byte{0xFF, 0xFB, 0xF0, 0x00}); ok {
		t.Error("bitrate index 15 must be rejected")
	}
}
```

Header bytes of the MPEG-2 case: `0xF3` = sync + version `10` (MPEG-2) + layer `01` (III) + no CRC; `0x82` = bitrate index 8 (64 kbps in the MPEG-2 table) + rate index 0 (22050) + padding bit 1; `0xC0` = channel mode 3 (mono).

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run MP3|MPEG`
Expected: FAIL (`undefined: readMP3`).

- [ ] **Step 4: Implement**

`internal/audiofile/mp3.go`:

```go
package audiofile

import "fmt"

type mpegFrame struct {
	version    int // 1, 2, or 25 (MPEG 2.5)
	layer      int // 1, 2 or 3
	bitrate    int // kbps
	sampleRate int
	channels   int
	samples    int // per frame
	length     int // bytes
}

var mpegBitrates = [5][14]int{
	{32, 64, 96, 128, 160, 192, 224, 256, 288, 320, 352, 384, 416, 448}, // MPEG-1 layer I
	{32, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384},    // MPEG-1 layer II
	{32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320},     // MPEG-1 layer III
	{32, 48, 56, 64, 80, 96, 112, 128, 144, 160, 176, 192, 224, 256},    // MPEG-2/2.5 layer I
	{8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160},         // MPEG-2/2.5 layers II and III
}

var mpegRates = [3][3]int{{44100, 48000, 32000}, {22050, 24000, 16000}, {11025, 12000, 8000}}

// parseMPEGFrame decodes a 4-byte MPEG audio frame header. Free-format and
// reserved values are rejected.
func parseMPEGFrame(b []byte) (mpegFrame, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return mpegFrame{}, false
	}
	verBits, layerBits := (b[1]>>3)&3, (b[1]>>1)&3
	brIdx, srIdx, pad := int(b[2]>>4), int((b[2]>>2)&3), int((b[2]>>1)&1)
	if verBits == 1 || layerBits == 0 || brIdx == 0 || brIdx == 15 || srIdx == 3 {
		return mpegFrame{}, false
	}
	f := mpegFrame{layer: 4 - int(layerBits), channels: 2}
	if b[3]>>6 == 3 {
		f.channels = 1
	}
	var vi int
	switch verBits {
	case 3:
		f.version, vi = 1, 0
	case 2:
		f.version, vi = 2, 1
	default:
		f.version, vi = 25, 2
	}
	f.sampleRate = mpegRates[vi][srIdx]
	table := f.layer - 1
	if f.version != 1 {
		table = 4
		if f.layer == 1 {
			table = 3
		}
	}
	f.bitrate = mpegBitrates[table][brIdx-1]
	br := f.bitrate * 1000
	switch {
	case f.layer == 1:
		f.samples, f.length = 384, (12*br/f.sampleRate+pad)*4
	case f.layer == 2 || f.version == 1:
		f.samples, f.length = 1152, 144*br/f.sampleRate+pad
	default: // layer III of MPEG-2 and 2.5
		f.samples, f.length = 576, 72*br/f.sampleRate+pad
	}
	return f, true
}

// findMPEGFrame returns the first frame header in [start, end) (searching up
// to 256 KiB) that is followed by another frame of the same kind, or that is
// the only frame before end.
func findMPEGFrame(s source, start, end int64) (int64, mpegFrame, error) {
	n := min(end-start, 256<<10)
	if n < 4 {
		return 0, mpegFrame{}, fmt.Errorf("%w: sin audio MPEG", ErrFormat)
	}
	b, err := s.read(start, int(n))
	if err != nil {
		return 0, mpegFrame{}, err
	}
	for i := 0; i+4 <= len(b); i++ {
		f, ok := parseMPEGFrame(b[i:])
		if !ok {
			continue
		}
		next := i + f.length
		switch {
		case start+int64(next) == end:
			return start + int64(i), f, nil // the last frame of the file
		case next+4 <= len(b):
			g, ok := parseMPEGFrame(b[next:])
			if ok && g.version == f.version && g.layer == f.layer && g.sampleRate == f.sampleRate {
				return start + int64(i), f, nil
			}
		case start+int64(next) < end:
			return start + int64(i), f, nil // the next frame lies beyond the search window
		}
	}
	return 0, mpegFrame{}, fmt.Errorf("%w: sin audio MPEG", ErrFormat)
}

// vbrHeader reads the frame and byte counts of a Xing/Info or VBRI header in
// the first frame; zeros when there is none (bytes is 0 when not recorded).
func vbrHeader(s source, off int64, f mpegFrame) (frames, bytes int64) {
	side := 32 // side information after the 4-byte header
	switch {
	case f.version == 1 && f.channels == 1:
		side = 17
	case f.version != 1 && f.channels == 2:
		side = 17
	case f.version != 1:
		side = 9
	}
	b, err := s.read(off, min(f.length, 64))
	if err != nil {
		return 0, 0
	}
	if x := 4 + side; x+12 <= len(b) {
		if tag := string(b[x : x+4]); tag == "Xing" || tag == "Info" {
			flags, p := be32(b[x+4:]), x+8
			if flags&1 != 0 {
				frames = int64(be32(b[p:]))
				p += 4
			}
			if flags&2 != 0 && p+4 <= len(b) {
				bytes = int64(be32(b[p:]))
			}
			return frames, bytes
		}
	}
	if v := 4 + 32; v+18 <= len(b) && string(b[v:v+4]) == "VBRI" {
		return int64(be32(b[v+14:])), int64(be32(b[v+10:]))
	}
	return 0, 0
}

// meanBitrate samples frames at four points of the payload and returns their
// mean bitrate. A frame counts only when the next one parses too.
func meanBitrate(s source, off, end int64, first mpegFrame) int {
	same := func(b []byte) (mpegFrame, bool) {
		g, ok := parseMPEGFrame(b)
		return g, ok && g.version == first.version && g.layer == first.layer && g.sampleRate == first.sampleRate
	}
	sum, n := 0, 0
	for _, at := range []int64{off, off + (end-off)/4, off + (end-off)/2, off + 3*(end-off)/4} {
		b, err := s.read(at, int(min(end-at, 32<<10)))
		if err != nil {
			continue
		}
		for i := 0; i+4 <= len(b); {
			g, ok := same(b[i:])
			if !ok {
				i++
				continue
			}
			if i+g.length+4 > len(b) {
				break
			}
			if _, ok := same(b[i+g.length:]); !ok {
				i++
				continue
			}
			sum += g.bitrate
			n++
			i += g.length
		}
	}
	if n == 0 {
		return first.bitrate
	}
	return sum / n
}

func readMP3(s source) (Info, error) {
	info := Info{Format: "mp3"}
	start := int64(0)
	for { // some files carry several ID3v2 tags in a row
		n, err := readID3v2(s, start, &info.Tags)
		if err != nil {
			return info, err
		}
		if n == 0 {
			break
		}
		start += n
	}
	end := s.size
	n, err := readID3v1(s, &info.Tags)
	if err != nil {
		return info, err
	}
	end -= n
	if n, err = apeTagSize(s, end); err != nil {
		return info, err
	}
	end -= n
	off, f, err := findMPEGFrame(s, start, end)
	if err != nil {
		return info, err
	}
	info.Codec = [...]string{"", "mp1", "mp2", "mp3"}[f.layer]
	info.SampleRate, info.Channels = f.sampleRate, f.channels
	info.Payload = Range{off, end - off}
	frames, bytes := vbrHeader(s, off, f)
	if diff := bytes - info.Payload.Length; frames > 0 && (bytes == 0 || 10*max(diff, -diff) <= info.Payload.Length) {
		info.DurationMs = frames * int64(f.samples) * 1000 / int64(f.sampleRate)
	} else {
		info.DurationMs = info.Payload.Length * 8 / int64(meanBitrate(s, off, end, f))
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(info.Payload.Length * 8 / info.DurationMs)
	}
	return info, nil
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/audiofile/ -v -run 'MP3|MPEG'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/audiofile
git commit -m "feat: native MP3 reader (frames, Xing/VBRI, trailing tags)"
```

---

### Task 6: FLAC reader and Vorbis comments

**Files:**
- Create: `internal/audiofile/vorbis.go`, `internal/audiofile/flac.go`, `internal/audiofile/flac_test.go`
- Modify: `internal/audiofile/build_test.go` (append FLAC builders)

**Interfaces:**
- Consumes: `source`, `readID3v2`, `Tags.set` (Task 4).
- Produces: `parseVorbisComment(b []byte, t *Tags) error` (also used by the Ogg reader, Task 7); `readFLAC(s source) (Info, error)`.

- [ ] **Step 1: Append the FLAC builders to `build_test.go`**

```go
func flacBlock(typ byte, last bool, data []byte) []byte {
	if last {
		typ |= 0x80
	}
	return cat([]byte{typ, byte(len(data) >> 16), byte(len(data) >> 8), byte(len(data))}, data)
}

func streamInfo(rate, channels, bits int, samples int64) []byte {
	d := make([]byte, 34)
	d[10] = byte(rate >> 12)
	d[11] = byte(rate >> 4)
	d[12] = byte(rate&0x0F)<<4 | byte(channels-1)<<1 | byte(bits-1)>>4
	d[13] = (byte(bits-1)&0x0F)<<4 | byte(samples>>32)&0x0F
	binary.BigEndian.PutUint32(d[14:], uint32(samples))
	return d
}

// vorbisComment builds a comment block: vendor, then "KEY=value" entries.
func vorbisComment(vendor string, entries ...string) []byte {
	b := cat(u32le(len(vendor)), []byte(vendor), u32le(len(entries)))
	for _, e := range entries {
		b = cat(b, u32le(len(e)), []byte(e))
	}
	return b
}
```

- [ ] **Step 2: Write the failing test**

`internal/audiofile/flac_test.go`:

```go
package audiofile

import (
	"errors"
	"reflect"
	"testing"
)

func flacFile(audio []byte) []byte {
	return cat([]byte("fLaC"),
		flacBlock(0, false, streamInfo(44100, 2, 16, 441000)),
		flacBlock(4, false, vorbisComment("reference libFLAC",
			"TITLE=Camelô", "ARTIST=Billy Blanco", "ARTIST=Miltinho", "ALBUMARTIST=Billy Blanco",
			"ALBUM=História da Música Popular Brasileira", "TRACKNUMBER=6", "TRACKTOTAL=12",
			"DATE=1971", "GENRE=Samba", "GENRE=MPB", "COMPILATION=1",
			"MUSICBRAINZ_TRACKID=rec-mbid", "REPLAYGAIN_TRACK_GAIN=-3.10 dB", "METADATA_BLOCK_PICTURE=AAAA")),
		flacBlock(6, false, []byte{0, 0, 0, 3}),
		flacBlock(1, true, make([]byte, 100)), // PADDING
		audio)
}

func TestFLAC(t *testing.T) {
	audio := make([]byte, 5000)
	file := flacFile(audio)
	info, err := readFLAC(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Codec != "flac" || !info.Lossless || info.SampleRate != 44100 || info.Channels != 2 || info.BitDepth != 16 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 10000 {
		t.Errorf("DurationMs = %d, want 10000", info.DurationMs)
	}
	if info.Payload != (Range{int64(len(file) - len(audio)), int64(len(audio))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	tg := info.Tags
	if tg.Title != "Camelô" || tg.Artist != "Billy Blanco; Miltinho" || tg.AlbumArtist != "Billy Blanco" || tg.Track != 6 || tg.TrackTotal != 12 || tg.Year != 1971 {
		t.Errorf("tags = %+v", tg)
	}
	if !reflect.DeepEqual(tg.Genres, []string{"Samba", "MPB"}) || !tg.Compilation || tg.MBRecordingID != "rec-mbid" || !tg.HasCover {
		t.Errorf("tags = %+v", tg)
	}
	if tg.TrackGain == nil || *tg.TrackGain != -3.10 {
		t.Errorf("TrackGain = %v", tg.TrackGain)
	}
}

func TestFLACWithLeadingID3(t *testing.T) {
	tag := id3Tag(3, id3TextFrame(3, "TIT2", "From ID3"))
	info, err := readFLAC(src(cat(tag, flacFile(make([]byte, 10)))))
	if err != nil {
		t.Fatal(err)
	}
	if info.Tags.Title != "From ID3" { // ID3 is read first: its value wins
		t.Errorf("Title = %q", info.Tags.Title)
	}
}

func TestFLACBadMagic(t *testing.T) {
	if _, err := readFLAC(src([]byte("OggS not a flac file"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}

func TestVorbisCommentTruncated(t *testing.T) {
	b := vorbisComment("v", "TITLE=x")
	var tg Tags
	if err := parseVorbisComment(b[:len(b)-3], &tg); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run 'FLAC|Vorbis'`
Expected: FAIL (`undefined: readFLAC`).

- [ ] **Step 4: Implement**

`internal/audiofile/vorbis.go`:

```go
package audiofile

import (
	"fmt"
	"strings"
)

// parseVorbisComment reads a Vorbis comment block (FLAC, Ogg Vorbis, Opus):
// little-endian lengths, a vendor string, then "KEY=value" entries.
func parseVorbisComment(b []byte, t *Tags) error {
	bad := fmt.Errorf("%w: comentarios Vorbis truncados", ErrFormat)
	if len(b) < 4 {
		return bad
	}
	p := 4 + int64(le32(b))
	if p+4 > int64(len(b)) {
		return bad
	}
	n := int64(le32(b[p:]))
	p += 4
	for i := int64(0); i < n; i++ {
		if p+4 > int64(len(b)) {
			return bad
		}
		l := int64(le32(b[p:]))
		p += 4
		if p+l > int64(len(b)) {
			return bad
		}
		k, v, ok := strings.Cut(string(b[p:p+l]), "=")
		p += l
		if !ok {
			continue
		}
		switch strings.ToUpper(k) {
		case "METADATA_BLOCK_PICTURE", "COVERART":
			t.HasCover = true
		default:
			t.set(k, v)
		}
	}
	return nil
}
```

`internal/audiofile/flac.go`:

```go
package audiofile

import "fmt"

func readFLAC(s source) (Info, error) {
	info := Info{Format: "flac", Codec: "flac", Lossless: true}
	off, err := readID3v2(s, 0, &info.Tags) // some taggers put ID3 before "fLaC"
	if err != nil {
		return info, err
	}
	magic, err := s.read(off, 4)
	if err != nil {
		return info, err
	}
	if string(magic) != "fLaC" {
		return info, fmt.Errorf("%w: falta la marca fLaC", ErrFormat)
	}
	pos := off + 4
	var samples int64
	for last := false; !last; {
		h, err := s.read(pos, 4)
		if err != nil {
			return info, err
		}
		last = h[0]&0x80 != 0
		typ, n := h[0]&0x7F, int(h[1])<<16|int(h[2])<<8|int(h[3])
		pos += 4
		switch typ {
		case 0: // STREAMINFO
			if n < 34 {
				return info, fmt.Errorf("%w: STREAMINFO corto", ErrFormat)
			}
			d, err := s.read(pos, 34)
			if err != nil {
				return info, err
			}
			info.SampleRate = int(d[10])<<12 | int(d[11])<<4 | int(d[12])>>4
			info.Channels = int((d[12]>>1)&7) + 1
			info.BitDepth = (int(d[12]&1)<<4 | int(d[13]>>4)) + 1
			samples = int64(d[13]&0x0F)<<32 | int64(be32(d[14:18]))
		case 4: // VORBIS_COMMENT
			d, err := s.read(pos, n)
			if err != nil {
				return info, err
			}
			if err := parseVorbisComment(d, &info.Tags); err != nil {
				return info, err
			}
		case 6: // PICTURE
			info.Tags.HasCover = true
		case 127:
			return info, fmt.Errorf("%w: bloque FLAC inválido", ErrFormat)
		}
		pos += int64(n)
	}
	if pos > s.size {
		return info, fmt.Errorf("%w: metadatos FLAC truncados", ErrFormat)
	}
	info.Payload = Range{pos, s.size - pos}
	if info.SampleRate > 0 {
		info.DurationMs = samples * 1000 / int64(info.SampleRate)
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(info.Payload.Length * 8 / info.DurationMs)
	}
	return info, nil
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/audiofile/ -v -run 'FLAC|Vorbis'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/audiofile
git commit -m "feat: native FLAC reader and Vorbis comments"
```

---

### Task 7: Ogg reader (Opus and Vorbis)

**Files:**
- Create: `internal/audiofile/ogg.go`, `internal/audiofile/ogg_test.go`
- Modify: `internal/audiofile/build_test.go` (append Ogg builders)

**Interfaces:**
- Consumes: `source`, `parseVorbisComment` (Task 6).
- Produces: `readOgg(s source) (Info, error)`. `Info.Format == "ogg"` and `Info.Payload` starts at the first page after the comment packet; the fingerprint (Task 11) hashes page contents from there.

Ogg packets can span pages (a big embedded cover makes the comment packet span many). A packet ends at a lacing value below 255.

- [ ] **Step 1: Append the Ogg builders to `build_test.go`**

```go
// mkOggRawPage builds a page from explicit lacing values (CRC left at zero:
// the reader does not check it).
func mkOggRawPage(headerType byte, granule int64, serial, seq int, lacing, data []byte) []byte {
	return cat([]byte("OggS"), []byte{0, headerType}, u64le(granule), u32le(serial), u32le(seq), u32le(0),
		[]byte{byte(len(lacing))}, lacing, data)
}

// mkOggPage builds a page holding whole packets.
func mkOggPage(headerType byte, granule int64, serial, seq int, packets ...[]byte) []byte {
	var lacing, data []byte
	for _, p := range packets {
		n := len(p)
		for ; n >= 255; n -= 255 {
			lacing = append(lacing, 255)
		}
		lacing = append(lacing, byte(n))
		data = append(data, p...)
	}
	return mkOggRawPage(headerType, granule, serial, seq, lacing, data)
}

func opusHead(channels, preSkip int) []byte {
	return cat([]byte("OpusHead"), []byte{1, byte(channels)}, u16le(preSkip), u32le(48000), u16le(0), []byte{0})
}

func vorbisID(channels, rate int) []byte {
	return cat([]byte{1}, []byte("vorbis"), u32le(0), []byte{byte(channels)}, u32le(rate),
		u32le(0), u32le(128000), u32le(0), []byte{0xB8, 1})
}
```

- [ ] **Step 2: Write the failing test**

`internal/audiofile/ogg_test.go`:

```go
package audiofile

import (
	"bytes"
	"errors"
	"testing"
)

func TestOpus(t *testing.T) {
	head := cat(
		mkOggPage(2, 0, 7, 0, opusHead(2, 312)),
		mkOggPage(0, 0, 7, 1, cat([]byte("OpusTags"), vorbisComment("yt-dlp", "TITLE=Criosho", "ARTIST=Barrios", "DATE=20231105"))),
	)
	audio := cat(
		mkOggPage(0, 48000, 7, 2, bytes.Repeat([]byte{1}, 200)),
		mkOggPage(0, 96000, 7, 3, bytes.Repeat([]byte{2}, 200)),
		mkOggPage(4, 144312, 7, 4, bytes.Repeat([]byte{3}, 200)),
	)
	info, err := readOgg(src(cat(head, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "ogg" || info.Codec != "opus" || info.Channels != 2 || info.SampleRate != 48000 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 3000 { // (144312 - 312) / 48000
		t.Errorf("DurationMs = %d, want 3000", info.DurationMs)
	}
	if info.Payload != (Range{int64(len(head)), int64(len(audio))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	if info.Tags.Title != "Criosho" || info.Tags.Artist != "Barrios" || info.Tags.Year != 2023 {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestVorbisCommentAcrossPages(t *testing.T) {
	comments := cat([]byte{3}, []byte("vorbis"), vorbisComment("Xiph", "TITLE=Tango del este", "ALBUM="+string(bytes.Repeat([]byte("x"), 400))), []byte{1})
	first, rest := comments[:255], comments[255:]
	lacingRest := []byte{}
	n := len(rest)
	for ; n >= 255; n -= 255 {
		lacingRest = append(lacingRest, 255)
	}
	lacingRest = append(lacingRest, byte(n))
	head := cat(
		mkOggPage(2, 0, 9, 0, vorbisID(2, 44100)),
		mkOggRawPage(0, 0, 9, 1, []byte{255}, first), // the packet continues on the next page
		mkOggRawPage(1, 0, 9, 2, lacingRest, rest),
	)
	audio := mkOggPage(4, 441000, 9, 3, bytes.Repeat([]byte{5}, 300))
	info, err := readOgg(src(cat(head, audio)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Codec != "vorbis" || info.SampleRate != 44100 || info.DurationMs != 10000 {
		t.Errorf("tech = %+v", info)
	}
	if info.Tags.Title != "Tango del este" || len(info.Tags.Album) != 400 {
		t.Errorf("tags = %+v", info.Tags)
	}
	if info.Payload.Offset != int64(len(head)) {
		t.Errorf("Payload.Offset = %d, want %d", info.Payload.Offset, len(head))
	}
}

func TestOggUnknownCodec(t *testing.T) {
	file := cat(mkOggPage(2, 0, 1, 0, []byte("\x7FFLAC....")), mkOggPage(0, 0, 1, 1, []byte("x")))
	if _, err := readOgg(src(file)); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestOggNotOgg(t *testing.T) {
	if _, err := readOgg(src([]byte("RIFF....WAVEfmt and some more bytes"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run 'Opus|Ogg|AcrossPages'`
Expected: FAIL (`undefined: readOgg`).

- [ ] **Step 4: Implement**

`internal/audiofile/ogg.go`:

```go
package audiofile

import "fmt"

type oggPage struct {
	granule int64
	serial  uint32
	lacing  []byte
	data    int64 // offset of the page's data
	length  int64 // data length
}

func (p oggPage) end() int64 { return p.data + p.length }

func readOggPage(s source, off int64) (oggPage, error) {
	h, err := s.read(off, 27)
	if err != nil {
		return oggPage{}, err
	}
	if string(h[:4]) != "OggS" {
		return oggPage{}, fmt.Errorf("%w: página Ogg inválida", ErrFormat)
	}
	lacing, err := s.read(off+27, int(h[26]))
	if err != nil {
		return oggPage{}, err
	}
	p := oggPage{granule: int64(le64(h[6:14])), serial: le32(h[14:18]), lacing: lacing, data: off + 27 + int64(len(lacing))}
	for _, l := range lacing {
		p.length += int64(l)
	}
	return p, nil
}

// readOgg reads an Ogg Opus or Ogg Vorbis file: codec and tags from the first
// two packets of the first logical stream, duration from its last granule
// position.
func readOgg(s source) (Info, error) {
	info := Info{Format: "ogg"}
	var packets [][]byte
	var cur []byte
	var serial uint32
	off := int64(0)
	for first := true; len(packets) < 2; first = false {
		p, err := readOggPage(s, off)
		if err != nil {
			return info, err
		}
		off = p.end()
		if first {
			serial = p.serial
		} else if p.serial != serial {
			continue // another logical stream
		}
		data, err := s.read(p.data, int(p.length))
		if err != nil {
			return info, err
		}
		pos := 0
		for _, l := range p.lacing {
			cur = append(cur, data[pos:pos+int(l)]...)
			pos += int(l)
			if l < 255 {
				packets = append(packets, cur)
				cur = nil
				if len(packets) == 2 {
					break
				}
			}
		}
		if len(cur) > maxRead {
			return info, fmt.Errorf("%w: paquete Ogg demasiado grande", ErrFormat)
		}
	}
	id, comments := packets[0], packets[1]
	var preSkip int64
	rate := 0
	switch {
	case len(id) >= 19 && string(id[:8]) == "OpusHead":
		info.Codec, info.Channels = "opus", int(id[9])
		preSkip, rate, info.SampleRate = int64(le16(id[10:12])), 48000, 48000
		if len(comments) < 8 || string(comments[:8]) != "OpusTags" {
			return info, fmt.Errorf("%w: faltan los comentarios Opus", ErrFormat)
		}
		if err := parseVorbisComment(comments[8:], &info.Tags); err != nil {
			return info, err
		}
	case len(id) >= 30 && id[0] == 1 && string(id[1:7]) == "vorbis":
		info.Codec, info.Channels = "vorbis", int(id[11])
		rate = int(le32(id[12:16]))
		info.SampleRate = rate
		if len(comments) < 7 || comments[0] != 3 || string(comments[1:7]) != "vorbis" {
			return info, fmt.Errorf("%w: faltan los comentarios Vorbis", ErrFormat)
		}
		if err := parseVorbisComment(comments[7:], &info.Tags); err != nil {
			return info, err
		}
	default:
		return info, fmt.Errorf("%w: códec Ogg sin lector propio", ErrUnsupported)
	}
	info.Payload = Range{off, s.size - off}
	last, err := lastOggGranule(s, serial)
	if err != nil {
		return info, err
	}
	if rate > 0 && last > preSkip {
		info.DurationMs = (last - preSkip) * 1000 / int64(rate)
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(info.Payload.Length * 8 / info.DurationMs)
	}
	return info, nil
}

// lastOggGranule returns the granule position of the stream's last page,
// looking at the final 64 KiB of the file.
func lastOggGranule(s source, serial uint32) (int64, error) {
	n := min(s.size, 64<<10)
	b, err := s.read(s.size-n, int(n))
	if err != nil {
		return 0, err
	}
	for i := len(b) - 27; i >= 0; i-- {
		if string(b[i:i+4]) == "OggS" && le32(b[i+14:i+18]) == serial {
			if g := int64(le64(b[i+6 : i+14])); g >= 0 {
				return g, nil
			}
		}
	}
	return 0, nil
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/audiofile/ -v -run 'Opus|Ogg|AcrossPages'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/audiofile
git commit -m "feat: native Ogg reader for Opus and Vorbis"
```

---
### Task 8: MP4/M4A reader

**Files:**
- Create: `internal/audiofile/mp4.go`, `internal/audiofile/mp4_test.go`
- Modify: `internal/audiofile/build_test.go` (append MP4 builders)

**Interfaces:**
- Consumes: `source`, `Tags.set`, `first`, `firstInt`, `genreNames`, `text8` (Task 4).
- Produces: `readMP4(s source) (Info, error)`.

Layout: `moov` holds `mvhd` (duration), `trak/mdia/minf/stbl/stsd` (codec, channels, rate) and `udta/meta/ilst` (tags). `meta` is normally a "full box" (4 bytes of version/flags before its children), but QuickTime writes it without them; tell them apart by peeking for `hdlr`. The payload is the largest `mdat`; the bitrate counts every `mdat`.

**Fragmented MP4** (`moof`/`mdat` pairs after `moov`; 33 FLAC-in-M4A files of the real collection) has `mvhd` duration 0: the duration is then in `moov/mvex/mehd`, in the `mvhd` timescale.

- [ ] **Step 1: Append the MP4 builders to `build_test.go`**

```go
func mkAtom(typ string, parts ...[]byte) []byte {
	body := cat(parts...)
	return cat(u32be(8+len(body)), []byte(typ), body)
}

func mp4Data(kind int, value []byte) []byte { return mkAtom("data", u32be(kind), u32be(0), value) }

func mp4Text(typ, v string) []byte { return mkAtom(typ, mp4Data(1, []byte(v))) }

// m4aFile builds an AAC M4A: 44.1 kHz stereo, 5 s, with the given ilst items.
func m4aFile(fullMeta bool, payload []byte, items ...[]byte) []byte {
	mvhd := mkAtom("mvhd", u32be(0), u32be(0), u32be(0), u32be(1000), u32be(5000), make([]byte, 80))
	entry := mkAtom("mp4a", make([]byte, 6), u16be(1), make([]byte, 8), u16be(2), u16be(16), u16be(0), u16be(0), u32be(44100<<16))
	stsd := mkAtom("stsd", u32be(0), u32be(1), entry)
	trak := mkAtom("trak", mkAtom("mdia", mkAtom("minf", mkAtom("stbl", stsd))))
	var meta []byte
	if fullMeta {
		meta = mkAtom("meta", u32be(0), mkAtom("hdlr", make([]byte, 25)), mkAtom("ilst", items...))
	} else {
		meta = mkAtom("meta", mkAtom("hdlr", make([]byte, 25)), mkAtom("ilst", items...))
	}
	moov := mkAtom("moov", mvhd, trak, mkAtom("udta", meta))
	return cat(mkAtom("ftyp", []byte("M4A "), u32be(0)), moov, mkAtom("mdat", payload))
}

// fragmentedM4A builds a fragmented FLAC-in-MP4: mvhd duration 0, the real
// duration (5 s at timescale 1000) in mvex/mehd, and two moof/mdat pairs.
func fragmentedM4A(first, second []byte) []byte {
	mvhd := mkAtom("mvhd", u32be(0), u32be(0), u32be(0), u32be(1000), u32be(0), make([]byte, 80))
	entry := mkAtom("fLaC", make([]byte, 6), u16be(1), make([]byte, 8), u16be(2), u16be(16), u16be(0), u16be(0), u32be(48000<<16))
	trak := mkAtom("trak", mkAtom("mdia", mkAtom("minf", mkAtom("stbl", mkAtom("stsd", u32be(0), u32be(1), entry)))))
	mvex := mkAtom("mvex", mkAtom("mehd", u32be(0), u32be(5000)), mkAtom("trex", make([]byte, 24)))
	moof := mkAtom("moof", mkAtom("mfhd", make([]byte, 8)))
	return cat(mkAtom("ftyp", []byte("iso5"), u32be(0)), mkAtom("moov", mvhd, trak, mvex),
		moof, mkAtom("mdat", first), moof, mkAtom("mdat", second))
}
```

- [ ] **Step 2: Write the failing test**

`internal/audiofile/mp4_test.go`:

```go
package audiofile

import (
	"errors"
	"reflect"
	"testing"
)

func TestM4A(t *testing.T) {
	payload := make([]byte, 16000)
	file := m4aFile(true, payload,
		mp4Text("\xa9nam", "Alcachofa"),
		mp4Text("\xa9ART", "Ricardo Villalobos"),
		mp4Text("aART", "Ricardo Villalobos"),
		mp4Text("\xa9alb", "Alcachofa"),
		mp4Text("\xa9day", "2003-05-12"),
		mkAtom("trkn", mp4Data(0, []byte{0, 0, 0, 3, 0, 12, 0, 0})),
		mkAtom("disk", mp4Data(0, []byte{0, 0, 0, 1, 0, 2})),
		mkAtom("gnre", mp4Data(0, u16be(53))), // ID3 index 52 + 1
		mkAtom("cpil", mp4Data(21, []byte{1})),
		mkAtom("covr", mp4Data(13, []byte{0xFF, 0xD8})),
		mkAtom("----", mkAtom("mean", u32be(0), []byte("com.apple.iTunes")),
			mkAtom("name", u32be(0), []byte("MusicBrainz Album Id")), mp4Data(1, []byte("album-mbid"))),
	)
	info, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "mp4" || info.Codec != "aac" || info.Lossless || info.Channels != 2 || info.SampleRate != 44100 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 5000 || info.Bitrate != 25 {
		t.Errorf("DurationMs = %d, Bitrate = %d", info.DurationMs, info.Bitrate)
	}
	if info.Payload != (Range{int64(len(file) - len(payload)), int64(len(payload))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	tg := info.Tags
	if tg.Title != "Alcachofa" || tg.Artist != "Ricardo Villalobos" || tg.AlbumArtist != "Ricardo Villalobos" || tg.Year != 2003 {
		t.Errorf("tags = %+v", tg)
	}
	if tg.Track != 3 || tg.TrackTotal != 12 || tg.Disc != 1 || tg.DiscTotal != 2 || !tg.Compilation || !tg.HasCover || tg.MBAlbumID != "album-mbid" {
		t.Errorf("tags = %+v", tg)
	}
	if !reflect.DeepEqual(tg.Genres, []string{"Electronic"}) {
		t.Errorf("Genres = %q", tg.Genres)
	}
}

func TestM4AQuickTimeMeta(t *testing.T) {
	info, err := readMP4(src(m4aFile(false, make([]byte, 10), mp4Text("\xa9nam", "Sin versión"))))
	if err != nil {
		t.Fatal(err)
	}
	if info.Tags.Title != "Sin versión" {
		t.Errorf("Title = %q", info.Tags.Title)
	}
}

func TestFragmentedM4A(t *testing.T) {
	first, second := make([]byte, 30000), make([]byte, 20000)
	file := fragmentedM4A(first, second)
	info, err := readMP4(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Codec != "flac" || !info.Lossless || info.BitDepth != 16 || info.SampleRate != 48000 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 5000 || info.Bitrate != 80 { // (30000 + 20000) bytes * 8 / 5000 ms
		t.Errorf("DurationMs = %d, Bitrate = %d", info.DurationMs, info.Bitrate)
	}
	if info.Payload.Length != 30000 {
		t.Errorf("Payload = %+v, want the largest mdat", info.Payload)
	}
}

func TestMP4WithoutMoov(t *testing.T) {
	file := cat(mkAtom("ftyp", []byte("M4A "), u32be(0)), mkAtom("mdat", make([]byte, 10)))
	if _, err := readMP4(src(file)); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run 'M4A|MP4'`
Expected: FAIL (`undefined: readMP4`).

- [ ] **Step 4: Implement**

`internal/audiofile/mp4.go`:

```go
package audiofile

import "fmt"

type atom struct {
	typ       string
	off, size int64 // the whole atom
	hdr       int64 // header length: 8, or 16 with a 64-bit size
}

func (a atom) body() int64 { return a.off + a.hdr }
func (a atom) end() int64  { return a.off + a.size }

func readAtom(s source, off, limit int64) (atom, error) {
	h, err := s.read(off, 8)
	if err != nil {
		return atom{}, err
	}
	a := atom{typ: string(h[4:8]), off: off, size: int64(be32(h)), hdr: 8}
	switch a.size {
	case 1:
		x, err := s.read(off+8, 8)
		if err != nil {
			return atom{}, err
		}
		a.size, a.hdr = int64(be64(x)), 16
	case 0:
		a.size = limit - off // up to the end of the parent
	}
	if a.size < a.hdr || off+a.size > limit {
		return atom{}, fmt.Errorf("%w: átomo MP4 %q inválido", ErrFormat, a.typ)
	}
	return a, nil
}

// eachAtom calls fn for every atom in [start, end).
func eachAtom(s source, start, end int64, fn func(atom) error) error {
	for off := start; off+8 <= end; {
		a, err := readAtom(s, off, end)
		if err != nil {
			return err
		}
		if err := fn(a); err != nil {
			return err
		}
		off = a.end()
	}
	return nil
}

// findAtom follows a path of atom types down from parent; ok is false when a
// step is missing.
func findAtom(s source, parent atom, path ...string) (atom, bool, error) {
	cur := parent
	for _, typ := range path {
		var found atom
		ok := false
		err := eachAtom(s, cur.body(), cur.end(), func(a atom) error {
			if !ok && a.typ == typ {
				found, ok = a, true
			}
			return nil
		})
		if err != nil || !ok {
			return atom{}, false, err
		}
		cur = found
	}
	return cur, true, nil
}

func readMP4(s source) (Info, error) {
	info := Info{Format: "mp4"}
	var moov, mdat atom
	var mdatTotal int64
	err := eachAtom(s, 0, s.size, func(a atom) error {
		switch a.typ {
		case "moov":
			moov = a
		case "mdat":
			mdatTotal += a.size - a.hdr
			if a.size > mdat.size {
				mdat = a
			}
		}
		return nil
	})
	if err != nil {
		return info, err
	}
	if moov.size == 0 {
		return info, fmt.Errorf("%w: falta el átomo moov", ErrFormat)
	}
	var scale int64
	if mvhd, ok, err := findAtom(s, moov, "mvhd"); err != nil {
		return info, err
	} else if ok {
		if scale, err = readMVHD(s, mvhd, &info); err != nil {
			return info, err
		}
	}
	if info.DurationMs == 0 && scale > 0 { // fragmented: the duration is in mvex/mehd
		if mehd, ok, err := findAtom(s, moov, "mvex", "mehd"); err != nil {
			return info, err
		} else if ok {
			d, err := s.read(mehd.body(), int(min(mehd.size-mehd.hdr, 12)))
			if err != nil {
				return info, err
			}
			switch {
			case len(d) >= 12 && d[0] == 1:
				info.DurationMs = int64(be64(d[4:12])) * 1000 / scale
			case len(d) >= 8:
				info.DurationMs = int64(be32(d[4:8])) * 1000 / scale
			}
		}
	}
	err = eachAtom(s, moov.body(), moov.end(), func(a atom) error {
		if a.typ != "trak" || info.Codec != "" {
			return nil
		}
		stsd, ok, err := findAtom(s, a, "mdia", "minf", "stbl", "stsd")
		if err != nil || !ok {
			return err
		}
		return readSTSD(s, stsd, &info)
	})
	if err != nil {
		return info, err
	}
	if meta, ok, err := findAtom(s, moov, "udta", "meta"); err != nil {
		return info, err
	} else if ok {
		if err := readMeta(s, meta, &info.Tags); err != nil {
			return info, err
		}
	}
	if info.Codec == "" {
		return info, fmt.Errorf("%w: sin pista de audio", ErrFormat)
	}
	if mdat.size > 0 {
		info.Payload = Range{mdat.body(), mdat.size - mdat.hdr}
	}
	if info.DurationMs > 0 {
		info.Bitrate = int(mdatTotal * 8 / info.DurationMs)
	}
	return info, nil
}

// readMVHD sets the duration and returns the movie timescale.
func readMVHD(s source, a atom, info *Info) (int64, error) {
	d, err := s.read(a.body(), int(min(a.size-a.hdr, 32)))
	if err != nil {
		return 0, err
	}
	var scale, dur int64
	switch {
	case len(d) >= 32 && d[0] == 1:
		scale, dur = int64(be32(d[20:24])), int64(be64(d[24:32]))
	case len(d) >= 20:
		scale, dur = int64(be32(d[12:16])), int64(be32(d[16:20]))
	}
	if scale > 0 {
		info.DurationMs = dur * 1000 / scale
	}
	return scale, nil
}

var mp4Codecs = map[string]struct {
	name     string
	lossless bool
}{
	"mp4a": {"aac", false}, "alac": {"alac", true}, "fLaC": {"flac", true}, "Opus": {"opus", false},
	"ac-3": {"ac3", false}, "ec-3": {"eac3", false}, ".mp3": {"mp3", false},
}

// readSTSD reads the first sample entry of a track; tracks that are not audio
// are left alone.
func readSTSD(s source, a atom, info *Info) error {
	if a.size-a.hdr < 8+36 {
		return nil
	}
	d, err := s.read(a.body()+8, 36)
	if err != nil {
		return err
	}
	c, ok := mp4Codecs[string(d[4:8])]
	if !ok {
		return nil
	}
	info.Codec, info.Lossless = c.name, c.lossless
	info.Channels = int(be16(d[24:26]))
	if c.lossless {
		info.BitDepth = int(be16(d[26:28]))
	}
	info.SampleRate = int(be32(d[32:36]) >> 16)
	return nil
}

var mp4Fields = map[string]string{
	"\xa9nam": "title", "\xa9ART": "artist", "aART": "albumartist", "\xa9alb": "album",
	"\xa9day": "date", "\xa9gen": "genre",
}

func readMeta(s source, meta atom, t *Tags) error {
	if meta.size-meta.hdr < 8 {
		return nil
	}
	start := meta.body()
	peek, err := s.read(start, 8)
	if err != nil {
		return err
	}
	if string(peek[4:8]) != "hdlr" {
		start += 4 // a full box: version and flags come first
	}
	return eachAtom(s, start, meta.end(), func(a atom) error {
		if a.typ != "ilst" {
			return nil
		}
		return eachAtom(s, a.body(), a.end(), func(item atom) error { return readItem(s, item, t) })
	})
}

func readItem(s source, item atom, t *Tags) error {
	if item.typ == "covr" {
		t.HasCover = true // not read: pictures can be large
		return nil
	}
	var name string
	var data []byte
	err := eachAtom(s, item.body(), item.end(), func(a atom) error {
		switch a.typ {
		case "name":
			b, err := s.read(a.body(), int(a.size-a.hdr))
			if err != nil {
				return err
			}
			if len(b) > 4 {
				name = string(b[4:])
			}
		case "data":
			b, err := s.read(a.body(), int(a.size-a.hdr))
			if err != nil {
				return err
			}
			if len(b) >= 8 {
				data = b[8:] // after the type indicator and the locale
			}
		}
		return nil
	})
	if err != nil || data == nil {
		return err
	}
	switch item.typ {
	case "----":
		t.set(name, text8(data))
	case "trkn", "disk":
		if len(data) >= 6 {
			n, total := int(be16(data[2:4])), int(be16(data[4:6]))
			if item.typ == "trkn" {
				firstInt(&t.Track, n)
				firstInt(&t.TrackTotal, total)
			} else {
				firstInt(&t.Disc, n)
				firstInt(&t.DiscTotal, total)
			}
		}
	case "gnre":
		if len(data) >= 2 {
			if g := int(be16(data)) - 1; g >= 0 && g < len(genreNames) {
				t.addGenre(genreNames[g])
			}
		}
	case "cpil":
		if len(data) >= 1 && data[0] != 0 {
			t.Compilation = true
		}
	default:
		if field, ok := mp4Fields[item.typ]; ok {
			t.set(field, text8(data))
		}
	}
	return nil
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/audiofile/ -v -run 'M4A|MP4'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/audiofile
git commit -m "feat: native MP4/M4A reader"
```

---

### Task 9: ASF/WMA reader

**Files:**
- Create: `internal/audiofile/asf.go`, `internal/audiofile/asf_test.go`
- Modify: `internal/audiofile/build_test.go` (append ASF builders)

**Interfaces:**
- Consumes: `source`, `Tags.set`, `utf16Text`, `clean` (Task 4).
- Produces: `readASF(s source) (Info, error)`; `guid(string) [16]byte` and the `asf*` GUID variables (the test builders use them).

ASF is a header object (with child objects) followed by a data object. GUIDs are stored with their first three fields little-endian. The data object has a 50-byte header before the packets.

- [ ] **Step 1: Append the ASF builders to `build_test.go`**

```go
func asfObject(id [16]byte, body ...[]byte) []byte {
	b := cat(body...)
	return cat(id[:], u64le(int64(24+len(b))), b)
}

func utf16z(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return append(b, 0, 0)
}

func asfDesc(name string, typ int, value []byte) []byte {
	n := utf16z(name)
	return cat(u16le(len(n)), n, u16le(typ), u16le(len(value)), value)
}

// wmaFile builds a WMA v2 file: 44.1 kHz stereo, 128 kbps, 5 s (plus a 3 s preroll).
func wmaFile(payload []byte) (file []byte, headerLen int) {
	fileProps := asfObject(asfFileProps, make([]byte, 40), u64le((5000+3000)*10000), u64le(0), u64le(3000), make([]byte, 16))
	wfx := cat(u16le(0x0161), u16le(2), u32le(44100), u32le(16000), u16le(4096), u16le(16))
	streamProps := asfObject(asfStreamProps, asfAudioMedia[:], make([]byte, 16), u64le(0), u32le(len(wfx)), u32le(0), u16le(1), u32le(0), wfx)
	title, author := utf16z("Guitarra negra"), utf16z("Alfredo Zitarrosa")
	content := asfObject(asfContentDesc, u16le(len(title)), u16le(len(author)), u16le(0), u16le(0), u16le(0), title, author)
	ext := asfObject(asfExtContent, u16le(4),
		asfDesc("WM/AlbumTitle", 0, utf16z("Guitarra negra")),
		asfDesc("WM/TrackNumber", 3, u32le(5)),
		asfDesc("WM/Genre", 0, utf16z("Folklore")),
		asfDesc("WM/Picture", 1, []byte{1, 2, 3}))
	objects := cat(fileProps, streamProps, content, ext)
	header := cat(asfHeader[:], u64le(int64(30+len(objects))), u32le(4), []byte{1, 2}, objects)
	data := cat(asfData[:], u64le(int64(50+len(payload))), make([]byte, 26), payload)
	return cat(header, data), len(header)
}
```

- [ ] **Step 2: Write the failing test**

`internal/audiofile/asf_test.go`:

```go
package audiofile

import (
	"errors"
	"reflect"
	"testing"
)

func TestWMA(t *testing.T) {
	payload := make([]byte, 3000)
	file, headerLen := wmaFile(payload)
	info, err := readASF(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "asf" || info.Codec != "wma" || info.Lossless || info.Channels != 2 || info.SampleRate != 44100 || info.Bitrate != 128 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 5000 {
		t.Errorf("DurationMs = %d, want 5000 (play duration minus preroll)", info.DurationMs)
	}
	if info.Payload != (Range{int64(headerLen + 50), int64(len(payload))}) {
		t.Errorf("Payload = %+v", info.Payload)
	}
	tg := info.Tags
	if tg.Title != "Guitarra negra" || tg.Artist != "Alfredo Zitarrosa" || tg.Album != "Guitarra negra" || tg.Track != 5 || !tg.HasCover {
		t.Errorf("tags = %+v", tg)
	}
	if !reflect.DeepEqual(tg.Genres, []string{"Folklore"}) {
		t.Errorf("Genres = %q", tg.Genres)
	}
}

func TestGUIDByteOrder(t *testing.T) {
	g := guid("75B22630-668E-11CF-A6D9-00AA0062CE6C")
	want := [16]byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C}
	if g != want {
		t.Fatalf("guid = % X", g)
	}
}

func TestASFNotASF(t *testing.T) {
	if _, err := readASF(src(make([]byte, 64))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run 'WMA|GUID|ASF'`
Expected: FAIL (`undefined: readASF`).

- [ ] **Step 4: Implement**

`internal/audiofile/asf.go`:

```go
package audiofile

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// guid turns the textual form of a GUID into its on-disk bytes (the first
// three fields are little-endian).
func guid(s string) [16]byte {
	h, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if err != nil || len(h) != 16 {
		panic("guid inválido: " + s)
	}
	var g [16]byte
	g[0], g[1], g[2], g[3] = h[3], h[2], h[1], h[0]
	g[4], g[5] = h[5], h[4]
	g[6], g[7] = h[7], h[6]
	copy(g[8:], h[8:])
	return g
}

var (
	asfHeader      = guid("75B22630-668E-11CF-A6D9-00AA0062CE6C")
	asfData        = guid("75B22636-668E-11CF-A6D9-00AA0062CE6C")
	asfFileProps   = guid("8CABDCA1-A947-11CF-8EE4-00C00C205365")
	asfStreamProps = guid("B7DC0791-A9B7-11CF-8EE6-00C00C205365")
	asfContentDesc = guid("75B22633-668E-11CF-A6D9-00AA0062CE6C")
	asfExtContent  = guid("D2D0A440-E307-11D2-97F0-00A0C95EA850")
	asfAudioMedia  = guid("F8699E40-5B4D-11CF-A8FD-00805F5C442B")
)

var asfCodecs = map[uint16]struct {
	name     string
	lossless bool
}{
	0x0160: {"wmav1", false}, 0x0161: {"wma", false}, 0x0162: {"wmapro", false}, 0x0163: {"wmalossless", true},
	0x000A: {"wmavoice", false}, 0x0055: {"mp3", false}, 0x0001: {"pcm", true},
}

func readASF(s source) (Info, error) {
	info := Info{Format: "asf"}
	h, err := s.read(0, 30)
	if err != nil {
		return info, err
	}
	if [16]byte(h[:16]) != asfHeader {
		return info, fmt.Errorf("%w: falta la cabecera ASF", ErrFormat)
	}
	hsize, count := int64(le64(h[16:24])), int(le32(h[24:28]))
	if hsize < 30 || hsize > s.size {
		return info, fmt.Errorf("%w: cabecera ASF truncada", ErrFormat)
	}
	off := int64(30)
	for i := 0; i < count && off+24 <= hsize; i++ {
		oh, err := s.read(off, 24)
		if err != nil {
			return info, err
		}
		size := int64(le64(oh[16:24]))
		if size < 24 || off+size > hsize {
			return info, fmt.Errorf("%w: objeto ASF inválido", ErrFormat)
		}
		body, n := off+24, size-24
		switch [16]byte(oh[:16]) {
		case asfFileProps:
			if n >= 80 {
				d, err := s.read(body, 80)
				if err != nil {
					return info, err
				}
				play, preroll := int64(le64(d[40:48])), int64(le64(d[56:64]))
				info.DurationMs = max(play/10000-preroll, 0)
			}
		case asfStreamProps:
			if n >= 70 && info.Codec == "" {
				d, err := s.read(body, 70)
				if err != nil {
					return info, err
				}
				if [16]byte(d[:16]) == asfAudioMedia {
					tag := le16(d[54:56])
					c, ok := asfCodecs[tag]
					if !ok {
						c.name = fmt.Sprintf("asf-0x%04x", tag)
					}
					info.Codec, info.Lossless = c.name, c.lossless
					info.Channels, info.SampleRate = int(le16(d[56:58])), int(le32(d[58:62]))
					info.Bitrate = int(le32(d[62:66])) * 8 / 1000
					if c.lossless {
						info.BitDepth = int(le16(d[68:70]))
					}
				}
			}
		case asfContentDesc:
			d, err := s.read(body, int(n))
			if err != nil {
				return info, err
			}
			readASFContent(d, &info.Tags)
		case asfExtContent:
			d, err := s.read(body, int(n))
			if err != nil {
				return info, err
			}
			readASFExtContent(d, &info.Tags)
		}
		off += size
	}
	if info.Codec == "" {
		return info, fmt.Errorf("%w: sin pista de audio", ErrFormat)
	}
	if d, err := s.read(hsize, 24); err == nil && [16]byte(d[:16]) == asfData {
		if size := int64(le64(d[16:24])); size >= 50 && hsize+size <= s.size {
			info.Payload = Range{hsize + 50, size - 50}
		}
	}
	if info.Payload.Length == 0 {
		info.Payload = Range{hsize, s.size - hsize}
	}
	return info, nil
}

// readASFContent reads the Content Description object: five lengths, then
// title, author, copyright, description and rating in UTF-16LE.
func readASFContent(d []byte, t *Tags) {
	if len(d) < 10 {
		return
	}
	var vals [5]string
	p := 10
	for i := range vals {
		l := int(le16(d[2*i:]))
		if p+l > len(d) {
			return
		}
		vals[i] = utf16Text(d[p:p+l], false)
		p += l
	}
	t.set("title", vals[0])
	t.set("artist", vals[1])
}

// readASFExtContent reads the Extended Content Description object: named
// attributes such as WM/AlbumTitle, WM/TrackNumber or MusicBrainz/Album Id.
func readASFExtContent(d []byte, t *Tags) {
	if len(d) < 2 {
		return
	}
	n, p := int(le16(d)), 2
	for i := 0; i < n; i++ {
		if p+2 > len(d) {
			return
		}
		nl := int(le16(d[p:]))
		p += 2
		if p+nl+4 > len(d) {
			return
		}
		name := clean(utf16Text(d[p:p+nl], false))
		p += nl
		typ, vl := le16(d[p:]), int(le16(d[p+2:]))
		p += 4
		if p+vl > len(d) {
			return
		}
		v := d[p : p+vl]
		p += vl
		switch typ {
		case 0: // string
			t.set(name, utf16Text(v, false))
		case 1: // bytes
			if name == "WM/Picture" {
				t.HasCover = true
			}
		case 2, 3: // bool, dword
			if len(v) >= 4 {
				t.set(name, strconv.Itoa(int(le32(v))))
			}
		case 4: // qword
			if len(v) >= 8 {
				t.set(name, strconv.FormatUint(le64(v), 10))
			}
		case 5: // word
			if len(v) >= 2 {
				t.set(name, strconv.Itoa(int(le16(v))))
			}
		}
	}
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/audiofile/ -v -run 'WMA|GUID|ASF'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/audiofile
git commit -m "feat: native ASF/WMA reader"
```

---

### Task 10: WAV and AIFF readers

**Files:**
- Create: `internal/audiofile/riff.go`, `internal/audiofile/riff_test.go`
- Modify: `internal/audiofile/build_test.go` (append RIFF/IFF builders)

**Interfaces:**
- Consumes: `source`, `readID3v2`, `Tags.set`, `text8` (Task 4).
- Produces: `readWAV(s source) (Info, error)`, `readAIFF(s source) (Info, error)`, `ext80([]byte) float64`.

Both are chunked: RIFF/WAV uses little-endian sizes, IFF/AIFF big-endian; chunks are padded to an even size. Files written while recording often claim a `data` size larger than the file: cut it at the end of the file instead of failing. AIFF's sample rate is an 80-bit extended float.

- [ ] **Step 1: Append the builders to `build_test.go`**

```go
func riffChunk(id string, body []byte) []byte {
	b := cat([]byte(id), u32le(len(body)), body)
	if len(body)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

func iffChunk(id string, body []byte) []byte {
	b := cat([]byte(id), u32be(len(body)), body)
	if len(body)%2 == 1 {
		b = append(b, 0)
	}
	return b
}

// wavFile builds a 16-bit stereo 44.1 kHz WAV of one second, with INFO tags.
func wavFile() []byte {
	chunks := cat(
		riffChunk("fmt ", cat(u16le(1), u16le(2), u32le(44100), u32le(176400), u16le(4), u16le(16))),
		riffChunk("LIST", cat([]byte("INFO"), riffChunk("INAM", []byte("Candombe\x00")), riffChunk("IART", []byte("Rubén Rada\x00")))),
		riffChunk("data", make([]byte, 176400)),
	)
	return cat([]byte("RIFF"), u32le(4+len(chunks)), []byte("WAVE"), chunks)
}

var ext44100 = []byte{0x40, 0x0E, 0xAC, 0x44, 0, 0, 0, 0, 0, 0}

func aiffFile() []byte {
	chunks := cat(
		iffChunk("COMM", cat(u16be(2), u32be(44100), u16be(16), ext44100)),
		iffChunk("SSND", cat(u32be(0), u32be(0), make([]byte, 176400))),
		iffChunk("ID3 ", id3Tag(3, id3TextFrame(3, "TIT2", "Tango del este"))),
	)
	return cat([]byte("FORM"), u32be(4+len(chunks)), []byte("AIFF"), chunks)
}
```

- [ ] **Step 2: Write the failing test**

`internal/audiofile/riff_test.go`:

```go
package audiofile

import (
	"bytes"
	"errors"
	"testing"
)

func TestWAV(t *testing.T) {
	file := wavFile()
	info, err := readWAV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "wav" || info.Codec != "pcm" || !info.Lossless || info.BitDepth != 16 || info.Channels != 2 || info.SampleRate != 44100 {
		t.Errorf("tech = %+v", info)
	}
	if info.DurationMs != 1000 || info.Bitrate != 1411 {
		t.Errorf("DurationMs = %d, Bitrate = %d", info.DurationMs, info.Bitrate)
	}
	dataAt := int64(bytes.Index(file, []byte("data")) + 8)
	if info.Payload != (Range{dataAt, 176400}) {
		t.Errorf("Payload = %+v, want {%d 176400}", info.Payload, dataAt)
	}
	if info.Tags.Title != "Candombe" || info.Tags.Artist != "Rubén Rada" {
		t.Errorf("tags = %+v", info.Tags)
	}
}

func TestWAVDataSizeLargerThanFile(t *testing.T) {
	file := wavFile()
	file = file[:len(file)-76400] // the data chunk still claims 176400 bytes
	info, err := readWAV(src(file))
	if err != nil {
		t.Fatal(err)
	}
	if info.Payload.Length != 100000 {
		t.Errorf("Payload.Length = %d, want 100000", info.Payload.Length)
	}
}

func TestAIFF(t *testing.T) {
	info, err := readAIFF(src(aiffFile()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "aiff" || info.Codec != "pcm" || !info.Lossless || info.SampleRate != 44100 || info.DurationMs != 1000 || info.BitDepth != 16 {
		t.Errorf("tech = %+v", info)
	}
	if info.Payload.Length != 176400 {
		t.Errorf("Payload = %+v", info.Payload)
	}
	if info.Tags.Title != "Tango del este" {
		t.Errorf("Title = %q", info.Tags.Title)
	}
}

func TestExt80(t *testing.T) {
	if got := ext80(ext44100); got != 44100 {
		t.Fatalf("ext80 = %v", got)
	}
}

func TestWAVNotRIFF(t *testing.T) {
	if _, err := readWAV(src([]byte("FORM....AIFFCOMM........"))); !errors.Is(err, ErrFormat) {
		t.Fatalf("err = %v, want ErrFormat", err)
	}
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run 'WAV|AIFF|Ext80'`
Expected: FAIL (`undefined: readWAV`).

- [ ] **Step 4: Implement**

`internal/audiofile/riff.go`:

```go
package audiofile

import (
	"bytes"
	"fmt"
	"math"
	"strings"
)

// eachChunk walks RIFF (little-endian sizes) or IFF/AIFF (big-endian) chunks
// from off. A chunk that claims more bytes than the file has is cut at the end
// of the file (common in files written while recording).
func eachChunk(s source, off int64, bigEndian bool, fn func(id string, body, n int64) error) error {
	for off+8 <= s.size {
		h, err := s.read(off, 8)
		if err != nil {
			return err
		}
		n := int64(le32(h[4:]))
		if bigEndian {
			n = int64(be32(h[4:]))
		}
		body := off + 8
		if body+n > s.size {
			n = s.size - body
		}
		if err := fn(string(h[:4]), body, n); err != nil {
			return err
		}
		off = body + n + n%2
	}
	return nil
}

var wavInfoFields = map[string]string{
	"INAM": "title", "IART": "artist", "IPRD": "album", "ICRD": "date", "IGNR": "genre", "ITRK": "tracknumber", "IPRT": "tracknumber",
}

func readWAV(s source) (Info, error) {
	info := Info{Format: "wav"}
	h, err := s.read(0, 12)
	if err != nil {
		return info, err
	}
	if string(h[:4]) != "RIFF" || string(h[8:12]) != "WAVE" {
		return info, fmt.Errorf("%w: no es un WAV", ErrFormat)
	}
	var byteRate int64
	err = eachChunk(s, 12, false, func(id string, body, n int64) error {
		switch id {
		case "fmt ":
			if n < 16 {
				return fmt.Errorf("%w: bloque fmt corto", ErrFormat)
			}
			d, err := s.read(body, 16)
			if err != nil {
				return err
			}
			tag := le16(d)
			info.Channels, info.SampleRate, info.BitDepth = int(le16(d[2:])), int(le32(d[4:])), int(le16(d[14:]))
			byteRate = int64(le32(d[8:]))
			switch tag {
			case 1, 0xFFFE:
				info.Codec, info.Lossless = "pcm", true
			case 3:
				info.Codec, info.Lossless = "pcm_float", true
			default:
				info.Codec, info.BitDepth = fmt.Sprintf("wav-0x%04x", tag), 0
			}
		case "data":
			info.Payload = Range{body, n}
		case "LIST":
			d, err := s.read(body, int(min(n, maxRead)))
			if err != nil {
				return err
			}
			if len(d) >= 4 && string(d[:4]) == "INFO" {
				readRIFFInfo(d[4:], &info.Tags)
			}
		case "id3 ", "ID3 ":
			_, err := readID3v2(s, body, &info.Tags)
			return err
		}
		return nil
	})
	if err != nil {
		return info, err
	}
	if info.Codec == "" || info.Payload.Length == 0 {
		return info, fmt.Errorf("%w: WAV sin formato o sin datos", ErrFormat)
	}
	if byteRate > 0 {
		info.DurationMs = info.Payload.Length * 1000 / byteRate
		info.Bitrate = int(byteRate * 8 / 1000)
	}
	return info, nil
}

func readRIFFInfo(d []byte, t *Tags) {
	for p := 0; p+8 <= len(d); {
		id, n := string(d[p:p+4]), int(le32(d[p+4:]))
		p += 8
		if n < 0 || p+n > len(d) {
			return
		}
		if field, ok := wavInfoFields[id]; ok {
			v := d[p : p+n]
			if i := bytes.IndexByte(v, 0); i >= 0 {
				v = v[:i]
			}
			t.set(field, text8(v))
		}
		p += n + n%2
	}
}

func readAIFF(s source) (Info, error) {
	info := Info{Format: "aiff"}
	h, err := s.read(0, 12)
	if err != nil {
		return info, err
	}
	kind := string(h[8:12])
	if string(h[:4]) != "FORM" || (kind != "AIFF" && kind != "AIFC") {
		return info, fmt.Errorf("%w: no es un AIFF", ErrFormat)
	}
	var frames int64
	err = eachChunk(s, 12, true, func(id string, body, n int64) error {
		switch id {
		case "COMM":
			if n < 18 {
				return fmt.Errorf("%w: bloque COMM corto", ErrFormat)
			}
			d, err := s.read(body, int(min(n, 22)))
			if err != nil {
				return err
			}
			info.Channels, frames, info.BitDepth = int(be16(d)), int64(be32(d[2:])), int(be16(d[6:]))
			info.SampleRate = int(ext80(d[8:18]))
			info.Codec, info.Lossless = "pcm", true
			if kind == "AIFC" && len(d) >= 22 {
				switch c := string(d[18:22]); c {
				case "NONE", "sowt", "twos":
				case "fl32", "FL32", "fl64", "FL64":
					info.Codec = "pcm_float"
				default:
					info.Codec, info.Lossless = strings.ToLower(strings.TrimSpace(c)), false
				}
			}
		case "SSND":
			if n < 8 {
				return fmt.Errorf("%w: bloque SSND corto", ErrFormat)
			}
			d, err := s.read(body, 8)
			if err != nil {
				return err
			}
			if skip := int64(be32(d)); 8+skip <= n {
				info.Payload = Range{body + 8 + skip, n - 8 - skip}
			}
		case "NAME", "AUTH":
			d, err := s.read(body, int(min(n, 4096)))
			if err != nil {
				return err
			}
			field := "title"
			if id == "AUTH" {
				field = "artist"
			}
			info.Tags.set(field, text8(d))
		case "ID3 ", "id3 ":
			_, err := readID3v2(s, body, &info.Tags)
			return err
		}
		return nil
	})
	if err != nil {
		return info, err
	}
	if info.Codec == "" || info.Payload.Length == 0 {
		return info, fmt.Errorf("%w: AIFF sin formato o sin datos", ErrFormat)
	}
	if info.SampleRate > 0 {
		info.DurationMs = frames * 1000 / int64(info.SampleRate)
	}
	info.Bitrate = info.SampleRate * info.Channels * info.BitDepth / 1000
	return info, nil
}

// ext80 decodes an 80-bit IEEE 754 extended float (AIFF sample rates).
func ext80(b []byte) float64 {
	exp := int(b[0]&0x7F)<<8 | int(b[1])
	mant := be64(b[2:10])
	if exp == 0 && mant == 0 {
		return 0
	}
	f := float64(mant) * math.Pow(2, float64(exp-16383-63))
	if b[0]&0x80 != 0 {
		f = -f
	}
	return f
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/audiofile/ -v -run 'WAV|AIFF|Ext80'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/audiofile
git commit -m "feat: native WAV and AIFF readers"
```

---
### Task 11: Dispatcher, ffprobe fallback and corpus check

**Files:**
- Create: `internal/audiofile/ffprobe.go`, `internal/audiofile/read.go`, `internal/audiofile/read_test.go`, `internal/audiofile/corpus_test.go`

**Interfaces:**
- Consumes: every `read*` function from Tasks 5–10.
- Produces: `audiofile.Read(ctx context.Context, path string) (Info, error)` (native first; ffprobe when the native reader cannot understand the file and ffprobe exists); `audiofile.ReadNative(path string) (Info, error)`; `audiofile.HasFFprobe() bool`.

Errors: `ErrIO` means "could not read now, retry on a later scan"; `ErrFormat` / `ErrUnsupported` mean "read, but not understood" (stored, not retried until the file changes or ffprobe appears).

- [ ] **Step 1: Write the failing test**

`internal/audiofile/read_test.go`:

```go
package audiofile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadNativeDispatch(t *testing.T) {
	wma, _ := wmaFile(make([]byte, 100))
	files := map[string][]byte{
		"a.mp3":  cat(id3Tag(3, id3TextFrame(3, "TIT2", "mp3")), mp3Frames(3, 0x11)),
		"b.FLAC": flacFile(make([]byte, 10)),
		"c.m4a":  m4aFile(true, make([]byte, 10)),
		"d.wma":  wma,
		"e.wav":  wavFile(),
		"f.aiff": aiffFile(),
		"g.opus": cat(mkOggPage(2, 0, 1, 0, opusHead(2, 0)), mkOggPage(0, 0, 1, 1, cat([]byte("OpusTags"), vorbisComment("v"))), mkOggPage(4, 48000, 1, 2, []byte{1})),
	}
	want := map[string]string{"a.mp3": "mp3", "b.FLAC": "flac", "c.m4a": "mp4", "d.wma": "asf", "e.wav": "wav", "f.aiff": "aiff", "g.opus": "ogg"}
	for name, data := range files {
		info, err := ReadNative(writeFile(t, name, data))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if info.Format != want[name] || info.Reader != "native" {
			t.Errorf("%s: Format = %q, Reader = %q", name, info.Format, info.Reader)
		}
	}
}

func TestReadNativeUnsupportedAndMissing(t *testing.T) {
	if _, err := ReadNative(writeFile(t, "x.ape", []byte("MAC "))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("ape: err = %v, want ErrUnsupported", err)
	}
	if _, err := ReadNative(filepath.Join(t.TempDir(), "missing.mp3")); !errors.Is(err, ErrIO) {
		t.Errorf("missing: err = %v, want ErrIO", err)
	}
}

func TestReadFallsBackToFFprobe(t *testing.T) {
	if !HasFFprobe() {
		t.Skip("ffprobe no está instalado")
	}
	// A WAV with a .ape extension: no native reader, ffprobe understands it.
	p := writeFile(t, "really-a-wav.ape", wavFile())
	info, err := Read(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Reader != "ffprobe" || info.Codec != "pcm_s16le" || !info.Lossless || info.DurationMs != 1000 {
		t.Errorf("info = %+v", info)
	}
	if info.Tags.Title != "Candombe" {
		t.Errorf("Title = %q", info.Tags.Title)
	}
	if info.Payload != (Range{0, int64(len(wavFile()))}) {
		t.Errorf("Payload = %+v (ffprobe: the whole file)", info.Payload)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/audiofile/ -run 'Dispatch|Unsupported|FFprobe'`
Expected: FAIL (`undefined: ReadNative`).

- [ ] **Step 3: Implement**

`internal/audiofile/read.go`:

```go
package audiofile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read reads the audio file at path: natively when there is a reader for its
// extension; with ffprobe when the native reader does not understand it (or
// there is none) and ffprobe is installed. An ErrIO is returned as is: the
// file could not be read now, which ffprobe would not fix.
func Read(ctx context.Context, path string) (Info, error) {
	info, err := ReadNative(path)
	if err == nil || errors.Is(err, ErrIO) || !HasFFprobe() {
		return info, err
	}
	if fi, ferr := readFFprobe(ctx, path); ferr == nil {
		return fi, nil
	}
	return info, err
}

// ReadNative reads the file with the built-in readers only.
func ReadNative(path string) (Info, error) {
	var read func(source) (Info, error)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		read = readMP3
	case ".flac":
		read = readFLAC
	case ".ogg", ".oga", ".opus":
		read = readOgg
	case ".m4a", ".mp4":
		read = readMP4
	case ".wma", ".asf":
		read = readASF
	case ".wav":
		read = readWAV
	case ".aif", ".aiff", ".aifc":
		read = readAIFF
	default:
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
	info, err := read(source{f, st.Size()})
	info.Reader = "native"
	return info, err
}
```

`internal/audiofile/ffprobe.go`:

```go
package audiofile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ffprobeOnce sync.Once
	ffprobePath string
)

// HasFFprobe reports whether ffprobe is on the PATH (checked once).
func HasFFprobe() bool {
	ffprobeOnce.Do(func() { ffprobePath, _ = exec.LookPath("ffprobe") })
	return ffprobePath != ""
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType   string `json:"codec_type"`
		CodecName   string `json:"codec_name"`
		SampleRate  string `json:"sample_rate"`
		Channels    int    `json:"channels"`
		BitsPerRaw  string `json:"bits_per_raw_sample"`
		BitsPerSamp int    `json:"bits_per_sample"`
		Disposition struct {
			AttachedPic int `json:"attached_pic"`
		} `json:"disposition"`
		Tags map[string]string `json:"tags"`
	} `json:"streams"`
	Format struct {
		FormatName string            `json:"format_name"`
		Duration   string            `json:"duration"`
		BitRate    string            `json:"bit_rate"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
}

var losslessCodecs = map[string]bool{"flac": true, "alac": true, "ape": true, "wavpack": true, "tta": true, "wmalossless": true, "mlp": true, "truehd": true}

// readFFprobe asks ffprobe. Its payload is the whole file: without knowing
// the container, tags cannot be told apart from audio.
func readFFprobe(ctx context.Context, path string) (Info, error) {
	st, err := os.Stat(path)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %v", ErrIO, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobePath, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path).Output()
	if err != nil {
		return Info{}, fmt.Errorf("%w: ffprobe: %v", ErrFormat, err)
	}
	var o ffprobeOutput
	if err := json.Unmarshal(out, &o); err != nil {
		return Info{}, fmt.Errorf("%w: ffprobe: %v", ErrFormat, err)
	}
	info := Info{Reader: "ffprobe", Format: strings.Split(o.Format.FormatName, ",")[0], Payload: Range{0, st.Size()}}
	for _, s := range o.Streams {
		if s.CodecType == "video" && s.Disposition.AttachedPic == 1 {
			info.Tags.HasCover = true
			continue
		}
		if s.CodecType != "audio" || info.Codec != "" {
			continue
		}
		info.Codec, info.Channels = s.CodecName, s.Channels
		info.SampleRate, _ = strconv.Atoi(s.SampleRate)
		info.Lossless = losslessCodecs[s.CodecName] || strings.HasPrefix(s.CodecName, "pcm_")
		if info.Lossless {
			info.BitDepth, _ = strconv.Atoi(s.BitsPerRaw)
			if info.BitDepth == 0 {
				info.BitDepth = s.BitsPerSamp
			}
		}
		for k, v := range s.Tags {
			info.Tags.set(k, v)
		}
	}
	if info.Codec == "" {
		return Info{}, fmt.Errorf("%w: ffprobe no encontró audio", ErrFormat)
	}
	for k, v := range o.Format.Tags {
		info.Tags.set(k, v)
	}
	if d, err := strconv.ParseFloat(o.Format.Duration, 64); err == nil {
		info.DurationMs = int64(d * 1000)
	}
	if br, err := strconv.Atoi(o.Format.BitRate); err == nil {
		info.Bitrate = br / 1000
	}
	return info, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/audiofile/ -v -run 'Dispatch|Unsupported|FFprobe'`
Expected: PASS (the ffprobe test runs here: `/opt/homebrew/bin/ffprobe` exists on this machine).

- [ ] **Step 5: Add the corpus check (optional, not in CI)**

`internal/audiofile/corpus_test.go`:

```go
package audiofile

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCorpus compares the native readers with ffprobe over a real collection:
//
//	TORDO_CORPUS=/Users/martin/Music go test ./internal/audiofile -run Corpus -v -timeout 0
//
// It fails when the native readers reject more than 1% of the files ffprobe
// reads; differences in tags or duration are logged for review.
func TestCorpus(t *testing.T) {
	root := os.Getenv("TORDO_CORPUS")
	if root == "" {
		t.Skip("TORDO_CORPUS no definido")
	}
	if !HasFFprobe() {
		t.Skip("hace falta ffprobe")
	}
	var files, failed, differ int
	firstArtist := func(s string) string {
		s, _, _ = strings.Cut(s, ";")
		return strings.TrimSpace(s)
	}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".mp3", ".flac", ".ogg", ".oga", ".opus", ".m4a", ".wma", ".wav", ".aif", ".aiff":
		default:
			return nil
		}
		want, ferr := readFFprobe(context.Background(), p)
		if ferr != nil {
			return nil // ffprobe cannot read it either
		}
		files++
		got, err := ReadNative(p)
		if err != nil {
			failed++
			t.Logf("FALLA %s: %v", p, err)
			return nil
		}
		var diffs []string
		if got.Tags.Title != want.Tags.Title {
			diffs = append(diffs, fmt.Sprintf("título %q ≠ %q", got.Tags.Title, want.Tags.Title))
		}
		if firstArtist(got.Tags.Artist) != firstArtist(want.Tags.Artist) {
			diffs = append(diffs, fmt.Sprintf("artista %q ≠ %q", got.Tags.Artist, want.Tags.Artist))
		}
		if got.Tags.Album != want.Tags.Album {
			diffs = append(diffs, fmt.Sprintf("álbum %q ≠ %q", got.Tags.Album, want.Tags.Album))
		}
		if dd := got.DurationMs - want.DurationMs; dd > max(1500, want.DurationMs/100) || -dd > max(1500, want.DurationMs/100) {
			diffs = append(diffs, fmt.Sprintf("duración %d ≠ %d ms", got.DurationMs, want.DurationMs))
		}
		if len(diffs) > 0 {
			differ++
			if differ <= 300 {
				t.Logf("DIFIERE %s: %s", p, strings.Join(diffs, "; "))
			}
		}
		return nil
	})
	t.Logf("%d archivos; %d rechazados por el lector propio; %d con diferencias", files, failed, differ)
	if files > 0 && failed*100 > files {
		t.Errorf("el lector propio rechaza más del 1%% de los archivos")
	}
}
```

Run: `go test ./internal/audiofile/ -run Corpus -v`
Expected: SKIP (no `TORDO_CORPUS`). The real run happens in Task 21.

- [ ] **Step 6: Run the package and commit**

Run: `go test ./internal/audiofile/ && go vet ./internal/audiofile/`
Expected: PASS.

```bash
git add internal/audiofile
git commit -m "feat: audiofile dispatcher, ffprobe fallback and corpus check"
```

---

### Task 12: Audio payload fingerprint

**Files:**
- Create: `internal/fingerprint/fingerprint.go`, `internal/fingerprint/fingerprint_test.go`

**Interfaces:**
- Produces: `fingerprint.Of(path string, offset, length int64, ogg bool) (string, error)` — 32 hex chars, or `""` for an empty payload.
- Callers (Task 18) pass `info.Payload.Offset`, `info.Payload.Length` and `ogg = info.Reader == "native" && info.Format == "ogg"`.

Spec §4.1: hash the first and last 64 KiB of the payload plus its length, so editing tags never changes a file's identity. For Ogg, rewriting comments renumbers (and re-CRCs) every later page, so the Ogg variant hashes page *contents* only: the first 64 KiB of page data, then the last page's data and granule position.

- [ ] **Step 1: Write the failing test**

`internal/fingerprint/fingerprint_test.go`:

```go
package fingerprint

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func pattern(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7) + seed
	}
	return b
}

func TestSameAudioDifferentTags(t *testing.T) {
	audio := pattern(300<<10, 1)
	a := append([]byte("ID3 short tag"), audio...)
	b := append([]byte("ID3 a much longer tag with a cover"), audio...)
	fa, err := Of(write(t, a), 13, int64(len(audio)), false)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := Of(write(t, b), 34, int64(len(audio)), false)
	if err != nil {
		t.Fatal(err)
	}
	if fa != fb || len(fa) != 32 {
		t.Fatalf("fingerprints differ: %s / %s", fa, fb)
	}
}

func TestDifferentAudio(t *testing.T) {
	a, b := pattern(300<<10, 1), pattern(300<<10, 2)
	fa, _ := Of(write(t, a), 0, int64(len(a)), false)
	fb, _ := Of(write(t, b), 0, int64(len(b)), false)
	if fa == fb {
		t.Fatal("different audio, same fingerprint")
	}
	short := pattern(1000, 1)
	fs, _ := Of(write(t, short), 0, 1000, false)
	fs2, _ := Of(write(t, short), 0, 999, false)
	if fs == fs2 {
		t.Fatal("the length must be part of the fingerprint")
	}
}

func TestEmptyPayload(t *testing.T) {
	f, err := Of(write(t, []byte("x")), 0, 0, false)
	if err != nil || f != "" {
		t.Fatalf("got %q, %v", f, err)
	}
}

// page builds an Ogg page holding one packet (< 255*255 bytes).
func page(seq uint32, granule int64, packet []byte) []byte {
	var lacing []byte
	n := len(packet)
	for ; n >= 255; n -= 255 {
		lacing = append(lacing, 255)
	}
	lacing = append(lacing, byte(n))
	h := make([]byte, 27)
	copy(h, "OggS")
	binary.LittleEndian.PutUint64(h[6:], uint64(granule))
	binary.LittleEndian.PutUint32(h[14:], 7)
	binary.LittleEndian.PutUint32(h[18:], seq)
	binary.LittleEndian.PutUint32(h[22:], seq*31) // a "CRC" that changes with the page
	h[26] = byte(len(lacing))
	return bytes.Join([][]byte{h, lacing, packet}, nil)
}

func TestOggIgnoresPageRenumbering(t *testing.T) {
	audio := [][]byte{pattern(4000, 1), pattern(4000, 2), pattern(4000, 3)}
	build := func(commentPages int) ([]byte, int64) {
		var head []byte
		for i := 0; i < commentPages; i++ {
			head = append(head, page(uint32(i), 0, []byte("comments"))...)
		}
		file := append([]byte{}, head...)
		for i, a := range audio {
			file = append(file, page(uint32(commentPages+i), int64(48000*(i+1)), a)...)
		}
		return file, int64(len(head))
	}
	a, offA := build(2)
	b, offB := build(5)
	fa, err := Of(write(t, a), offA, int64(len(a))-offA, true)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := Of(write(t, b), offB, int64(len(b))-offB, true)
	if err != nil {
		t.Fatal(err)
	}
	if fa != fb {
		t.Fatalf("renumbered pages changed the fingerprint: %s / %s", fa, fb)
	}
	audio[2] = pattern(4000, 9)
	c, offC := build(2)
	fc, _ := Of(write(t, c), offC, int64(len(c))-offC, true)
	if fc == fa {
		t.Fatal("different audio, same Ogg fingerprint")
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/fingerprint/`
Expected: FAIL (`undefined: Of`).

- [ ] **Step 3: Implement**

`internal/fingerprint/fingerprint.go`:

```go
// Package fingerprint identifies the audio content of a file, ignoring its
// tags and embedded pictures, so moved, renamed or re-tagged files keep their
// identity and identical rips are recognized.
package fingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

const chunk = 64 << 10

// Of fingerprints the payload [offset, offset+length) of the file at path:
// its first and last 64 KiB plus its length. With ogg, page headers are
// skipped (see hashOgg). An empty payload has no fingerprint ("").
func Of(path string, offset, length int64, ogg bool) (string, error) {
	if length <= 0 {
		return "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if ogg {
		err = hashOgg(h, f, offset, offset+length)
	} else {
		err = hashRange(h, f, offset, length)
	}
	if err != nil {
		return "", err
	}
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(length))
	h.Write(n[:])
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

func hashRange(w io.Writer, r io.ReaderAt, off, n int64) error {
	if n <= 2*chunk {
		_, err := io.Copy(w, io.NewSectionReader(r, off, n))
		return err
	}
	if _, err := io.Copy(w, io.NewSectionReader(r, off, chunk)); err != nil {
		return err
	}
	_, err := io.Copy(w, io.NewSectionReader(r, off+n-chunk, chunk))
	return err
}

// hashOgg hashes the data of the pages in [start, end) without their headers
// (rewriting comments renumbers and re-checksums every later page): the
// first 64 KiB of page data, then the last page's granule position and data.
func hashOgg(w io.Writer, r io.ReaderAt, start, end int64) error {
	buf := make([]byte, 27+255)
	left := int64(chunk)
	for off := start; off < end && left > 0; {
		data, n, err := oggPage(r, off, buf)
		if err != nil {
			return err
		}
		take := min(left, n)
		if _, err := io.Copy(w, io.NewSectionReader(r, data, take)); err != nil {
			return err
		}
		left -= take
		off = data + n
	}
	tail := min(end-start, 64<<10)
	b := make([]byte, tail)
	if _, err := r.ReadAt(b, end-tail); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	for i := len(b) - 27; i >= 0; i-- {
		if string(b[i:i+4]) != "OggS" {
			continue
		}
		data, n, err := oggPage(r, end-tail+int64(i), buf)
		if err != nil || data+n > end {
			continue // "OggS" inside page data
		}
		w.Write(b[i+6 : i+14]) // granule position
		_, err = io.Copy(w, io.NewSectionReader(r, data, n))
		return err
	}
	return nil
}

// oggPage reads the page header at off and returns where its data starts and
// how long it is.
func oggPage(r io.ReaderAt, off int64, buf []byte) (data, n int64, err error) {
	if _, err := r.ReadAt(buf[:27], off); err != nil {
		return 0, 0, err
	}
	if string(buf[:4]) != "OggS" {
		return 0, 0, errors.New("página Ogg inválida")
	}
	segs := int(buf[26])
	if _, err := r.ReadAt(buf[27:27+segs], off+27); err != nil {
		return 0, 0, err
	}
	for _, l := range buf[27 : 27+segs] {
		n += int64(l)
	}
	return off + 27 + int64(segs), n, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/fingerprint/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fingerprint
git commit -m "feat: fingerprint of the audio payload, Ogg-aware"
```

---
### Task 13: Folder name parser

**Files:**
- Create: `internal/nameparse/nameparse.go`, `internal/nameparse/folder.go`, `internal/nameparse/folder_test.go`

**Interfaces:**
- Produces: `nameparse.Norm(s string) string`; `nameparse.IsVarious(s string) bool`; `nameparse.StripDisc(s string) (rest string, disc int)`; `type nameparse.Folder struct { Position int; Artist, Album string; Year, Disc int; Various bool }`; `nameparse.ParseFolder(name string) Folder`. Package-internal: `tidy(string) string`, `reSpaces`.

Names only fill what tags lack (spec §5 step 4), so the parser aims for the common patterns of the real collection, not perfection. Every case in the test comes from `/Users/martin/Music`.

- [ ] **Step 1: Write the failing test**

`internal/nameparse/folder_test.go`:

```go
package nameparse

import "testing"

func TestParseFolder(t *testing.T) {
	cases := []struct {
		in   string
		want Folder
	}{
		{"El Cuarteto Imperial - (2008) Para Bailar y Bailar con El Cuarteto Imperial",
			Folder{Artist: "El Cuarteto Imperial", Album: "Para Bailar y Bailar con El Cuarteto Imperial", Year: 2008}},
		{"[LA-599] Rata Blanca - (1990) Magos, Espadas Y Rosas",
			Folder{Position: 599, Artist: "Rata Blanca", Album: "Magos, Espadas Y Rosas", Year: 1990}},
		{"[LA-600] Ricardo Villalobos - (2003) - Alcachofa",
			Folder{Position: 600, Artist: "Ricardo Villalobos", Album: "Alcachofa", Year: 2003}},
		{"(1990) Tercer Mundo", Folder{Album: "Tercer Mundo", Year: 1990}},
		{"Bill Evans Trio - 1961 -Waltz For Debby", Folder{Artist: "Bill Evans Trio", Album: "Waltz For Debby", Year: 1961}},
		{"Elizeth Cardoso - Live in Japan - 1977", Folder{Artist: "Elizeth Cardoso", Album: "Live in Japan", Year: 1977}},
		{"Herb Alpert - [1980] Beyond (2015) [24-88.2]", Folder{Artist: "Herb Alpert", Album: "Beyond", Year: 1980}},
		{"orquestra afro-brasileira - obaluaye! (1957) (digital) (aac vbr)",
			Folder{Artist: "orquestra afro-brasileira", Album: "obaluaye!", Year: 1957}},
		{"orquestra afro-brasileira - orquestra afro-brasileira 1968 (lp) (256kb)",
			Folder{Artist: "orquestra afro-brasileira", Album: "orquestra afro-brasileira", Year: 1968}},
		{"1971 - História da Música Popular Brasileira 32 BILLY BLANCO",
			Folder{Album: "História da Música Popular Brasileira 32 BILLY BLANCO", Year: 1971}},
		{"Noel Rosa - Noel pela primeira vez Cd 02 -x", Folder{Artist: "Noel Rosa", Album: "Noel pela primeira vez", Disc: 2}},
		{"Varios - Candombe de llamadas CD 1 (selección, falta tema 20)",
			Folder{Various: true, Album: "Candombe de llamadas (selección, falta tema 20)", Disc: 1}},
		{"AAVV - Vinicius de Moraes Songbook (1993)", Folder{Various: true, Album: "Vinicius de Moraes Songbook", Year: 1993}},
		{"VA - História da MPB", Folder{Various: true, Album: "História da MPB"}},
		{"Volume 2, CD 1 (Social Music)", Folder{Album: "Volume 2 (Social Music)", Disc: 1}},
		{"História das escolas de samba Disc 4", Folder{Album: "História das escolas de samba", Disc: 4}},
		{"CD1", Folder{Disc: 1}},
		{"Nelson_Cavaquinho", Folder{Album: "Nelson Cavaquinho"}},
		{"Silvia Barrios - Folclore y tango", Folder{Artist: "Silvia Barrios", Album: "Folclore y tango"}},
		{"15 discos de los 60", Folder{Album: "15 discos de los 60"}},
		{"Latinoamérica 600", Folder{Album: "Latinoamérica 600"}},
		{"07 - Soda Stereo - Signos", Folder{Position: 7, Artist: "Soda Stereo", Album: "Signos"}},
	}
	for _, c := range cases {
		if got := ParseFolder(c.in); got != c.want {
			t.Errorf("ParseFolder(%q)\n got  %+v\n want %+v", c.in, got, c.want)
		}
	}
}

func TestNorm(t *testing.T) {
	cases := map[string]string{
		"Fito Páez":           "fito paez",
		"  FITO   PÁEZ!! ":     "fito paez",
		"Nelson_Cavaquinho":   "nelson cavaquinho",
		"Ñandú & Çedilla":     "nandu cedilla",
		"Os Originais do Sam": "os originais do sam",
		"":                    "",
	}
	for in, want := range cases {
		if got := Norm(in); got != want {
			t.Errorf("Norm(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsVarious(t *testing.T) {
	for _, s := range []string{"VA", "V.A.", "AAVV", "Varios", "Various Artists", "Varios artistas", "VVAA"} {
		if !IsVarious(s) {
			t.Errorf("IsVarious(%q) = false", s)
		}
	}
	for _, s := range []string{"", "Vanessa", "Varios Rock"} {
		if IsVarious(s) {
			t.Errorf("IsVarious(%q) = true", s)
		}
	}
}

func TestStripDisc(t *testing.T) {
	cases := []struct {
		in   string
		rest string
		disc int
	}{
		{"Noel pela primeira vez Cd 02", "Noel pela primeira vez", 2},
		{"Greatest Hits (Disc 1)", "Greatest Hits", 1},
		{"Disco 3", "", 3},
		{"Discovery", "Discovery", 0},
		{"Disco 2000", "Disco 2000", 0},
	}
	for _, c := range cases {
		if rest, disc := StripDisc(c.in); rest != c.rest || disc != c.disc {
			t.Errorf("StripDisc(%q) = %q, %d; want %q, %d", c.in, rest, disc, c.rest, c.disc)
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/nameparse/`
Expected: FAIL (`undefined: ParseFolder`).

- [ ] **Step 3: Implement**

`internal/nameparse/nameparse.go`:

```go
// Package nameparse reads what folder and file names say about music:
// artist, album, year, disc, track and position in a collection. It only
// fills what the tags lack.
package nameparse

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// Norm folds a name for comparisons: lowercase, without accents or
// punctuation, single spaces. "Fito Páez" and "fito paez!" are equal.
func Norm(s string) string {
	s = accents.Replace(strings.ToLower(s))
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return b.String()
}

var various = map[string]bool{
	"va": true, "v a": true, "aavv": true, "vvaa": true, "varios": true, "various": true,
	"various artists": true, "varios artistas": true, "varios interpretes": true,
}

// IsVarious reports whether an artist name means "various artists".
func IsVarious(s string) bool { return various[Norm(s)] }

var (
	reDisc      = regexp.MustCompile(`(?i)[\s\-_,(\[]*\b(?:cd|disc|disco|disk|disque)\s*0*(\d{1,2})\b[)\]]?`)
	reShortTail = regexp.MustCompile(`^\s*-?\s*\w?\s*$`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// StripDisc removes a disc marker ("CD 2", "(Disc 1)", "Disco 3", "Cd 02 -x")
// and returns the rest and the disc number (0 when there is none). What
// follows the marker is kept, unless it is only a stray letter or dash.
func StripDisc(s string) (string, int) {
	m := reDisc.FindStringSubmatchIndex(s)
	if m == nil {
		return s, 0
	}
	disc, _ := strconv.Atoi(s[m[2]:m[3]])
	tail := s[m[1]:]
	if reShortTail.MatchString(tail) {
		tail = ""
	}
	return tidy(s[:m[0]] + " " + tail), disc
}

// tidy collapses spaces and trims separators from both ends.
func tidy(s string) string {
	return strings.Trim(reSpaces.ReplaceAllString(s, " "), " -–_,")
}
```

`internal/nameparse/folder.go`:

```go
package nameparse

import (
	"regexp"
	"strconv"
	"strings"
)

// Folder is what a folder's name says.
type Folder struct {
	Position int // order prefix inside a collection: "[LA-599] …" → 599, "07 - …" → 7
	Artist   string
	Album    string // the whole name when there is no "Artist - Album" split
	Year     int
	Disc     int
	Various  bool // "VA - …", "AAVV - …", "Varios - …": a compilation
}

var (
	rePosBracket = regexp.MustCompile(`^\[\s*(?:[A-Za-z]{1,5}\s*[-_ ]?\s*)?(\d{1,4})\s*\]\s*`)
	rePosNumber  = regexp.MustCompile(`^(\d{1,3})\s*[.)\-_]\s*`)
	reNoise      = regexp.MustCompile(`(?i)\s*[\[(]\s*(?:\d{2,3}\s*k(?:b|bps)?|v0|v2|320|aac[^)\]]*|mp3[^)\]]*|flac[^)\]]*|ogg[^)\]]*|digital|lp|vinyl|web|cd|cdrip|hi-?res|\d{2}\s*[-/ ]\s*\d{2,3}(?:[.,]\d)?(?:\s*khz)?|\d{2}\s*bits?[^)\]]*)\s*[\])]`)
	reYearParen  = regexp.MustCompile(`\s*[\[(]((?:19|20)\d{2})[\])]`)
	reYearLead   = regexp.MustCompile(`^((?:19|20)\d{2})\s*[-–.]\s*`)
	reYearMiddle = regexp.MustCompile(`\s+[-–]\s*((?:19|20)\d{2})\s*[-–]\s*`)
	reYearTail   = regexp.MustCompile(`(?:\s*[-–]\s*|\s+)((?:19|20)\d{2})$`)
	reSeparators = regexp.MustCompile(`\s*[-–](?:\s*[-–])+\s*`)
	reSplit      = regexp.MustCompile(`\s+[-–]\s*|\s*[-–]\s+`)
)

// ParseFolder reads a folder name such as "[LA-599] Rata Blanca - (1990)
// Magos, Espadas Y Rosas" or "Noel Rosa - Noel pela primeira vez Cd 02".
func ParseFolder(name string) Folder {
	var f Folder
	s := strings.ReplaceAll(name, "_", " ")
	if m := rePosBracket.FindStringSubmatchIndex(s); m != nil {
		f.Position, _ = strconv.Atoi(s[m[2]:m[3]])
		s = s[m[1]:]
	} else if m := rePosNumber.FindStringSubmatchIndex(s); m != nil {
		f.Position, _ = strconv.Atoi(s[m[2]:m[3]])
		s = s[m[1]:]
	}
	s = reNoise.ReplaceAllString(s, "")
	s, f.Disc = StripDisc(s)
	s, f.Year = stripYear(s)
	s = tidy(reSeparators.ReplaceAllString(s, " - "))
	if loc := reSplit.FindStringIndex(s); loc != nil {
		f.Artist, f.Album = tidy(s[:loc[0]]), tidy(s[loc[1]:])
	} else {
		f.Album = s
	}
	if IsVarious(f.Artist) {
		f.Various, f.Artist = true, ""
	}
	return f
}

// stripYear takes the release year out of a name: "(1990)", "[1980]",
// "1971 - …", "… - 1961 - …" or a trailing "… 1977". Other bracketed years
// (reissues: "Beyond (2015)") are dropped.
func stripYear(s string) (string, int) {
	year := 0
	take := func(m []int) { year, _ = strconv.Atoi(s[m[2]:m[3]]) }
	if m := reYearParen.FindStringSubmatchIndex(s); m != nil {
		take(m)
		s = reYearParen.ReplaceAllString(s[:m[0]]+" "+s[m[1]:], "")
	} else if m := reYearLead.FindStringSubmatchIndex(s); m != nil {
		take(m)
		s = s[m[1]:]
	} else if m := reYearMiddle.FindStringSubmatchIndex(s); m != nil {
		take(m)
		s = s[:m[0]] + " - " + s[m[1]:]
	} else if m := reYearTail.FindStringSubmatchIndex(s); m != nil && m[0] > 0 {
		take(m)
		s = s[:m[0]]
	}
	return s, year
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/nameparse/ -v`
Expected: PASS. If a case fails, fix the regular expression rather than the expectation: every expectation is what a person would read in that folder name. Two cases are deliberately imperfect and must stay as written: `"Varios - Candombe de llamadas CD 1 (selección, falta tema 20)"` keeps its parenthetical note in the album, and `"15 discos de los 60"` gets no position (a number followed only by a space is part of the name).

- [ ] **Step 5: Commit**

```bash
git add internal/nameparse
git commit -m "feat: folder name parser (artist, album, year, disc, position)"
```

---

### Task 14: File name parser

**Files:**
- Create: `internal/nameparse/file.go`, `internal/nameparse/file_test.go`

**Interfaces:**
- Consumes: `Norm`, `tidy` (Task 13).
- Produces: `type nameparse.File struct { Disc, Track, Year int; Artist, Title string }`; `nameparse.ParseFile(name, folderArtist string) File` — `name` includes the extension; `folderArtist` is the album artist from tags or the folder name ("" when unknown) and tells `"07 - Curare - Nilze Carvalho"` (title, then artist) from `"20 - Lito Barrientos - Cumbia en Do Menor"` (artist, then title).

- [ ] **Step 1: Write the failing test**

`internal/nameparse/file_test.go`:

```go
package nameparse

import "testing"

func TestParseFile(t *testing.T) {
	cases := []struct {
		name, folderArtist string
		want               File
	}{
		{"05-Fue Amor.mp3", "", File{Track: 5, Title: "Fue Amor"}},
		{"08- Inglaterra.mp3", "Fito Páez", File{Track: 8, Title: "Inglaterra"}},
		{"03. Detour Ahead (take 2).mp3", "", File{Track: 3, Title: "Detour Ahead (take 2)"}},
		{"06 Camelô.opus", "", File{Track: 6, Title: "Camelô"}},
		{"08 antonico(alcides gerardi).opus", "", File{Track: 8, Title: "antonico(alcides gerardi)"}},
		{"1-05 Garota de Ipanema.flac", "", File{Disc: 1, Track: 5, Title: "Garota de Ipanema"}},
		{"07 - Curare - Nilze Carvalho.mp3", "Nilze Carvalho", File{Track: 7, Title: "Curare", Artist: "Nilze Carvalho"}},
		{"16 - Só pra contrariar - c- Almirante.mp3", "Noel Rosa", File{Track: 16, Title: "Só pra contrariar", Artist: "Almirante"}},
		{"20 - Lito Barrientos y Su Orquesta  - Cumbia en Do Menor.mp3", "", File{Track: 20, Artist: "Lito Barrientos y Su Orquesta", Title: "Cumbia en Do Menor"}},
		{"3 - La añera - Silvia Barrios ⧸ Daniel Barrera.opus", "Silvia Barrios", File{Track: 3, Title: "La añera", Artist: "Silvia Barrios / Daniel Barrera"}},
		{"1985 - La Nueva Milonga - Retirada  A los Colores-doblada.mp3", "", File{Year: 1985, Artist: "La Nueva Milonga", Title: "Retirada A los Colores-doblada"}},
		{"retirada 1961 nueva milonga-T1b-D4-doblada.mp3", "", File{Title: "retirada 1961 nueva milonga-T1b-D4-doblada"}},
		{"Criosho [K-NtwAQ8Oqk].opus", "", File{Title: "Criosho"}},
		{"Gilberto Gil - Aquele Abraço.mp3", "", File{Artist: "Gilberto Gil", Title: "Aquele Abraço"}},
		{"05.mp3", "", File{Title: "05"}},
	}
	for _, c := range cases {
		if got := ParseFile(c.name, c.folderArtist); got != c.want {
			t.Errorf("ParseFile(%q, %q)\n got  %+v\n want %+v", c.name, c.folderArtist, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/nameparse/ -run ParseFile`
Expected: FAIL (`undefined: ParseFile`).

- [ ] **Step 3: Implement**

`internal/nameparse/file.go`:

```go
package nameparse

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// File is what a file's name says.
type File struct {
	Disc, Track, Year int
	Artist, Title     string
}

var (
	reYouTubeID = regexp.MustCompile(`\s*\[[A-Za-z0-9_-]{11}\]$`)
	reTrack     = regexp.MustCompile(`^(?:(\d)[-.](\d{1,3})|(\d{1,3}))(?:\s*[-.)_]\s*|\s+)`)
	reCredit    = regexp.MustCompile(`(?i)^(?:c-|c/|con |com |feat\.?|ft\.?)\s*`)
	reYearOnly  = regexp.MustCompile(`^(?:19|20)\d{2}$`)
	reFileSplit = regexp.MustCompile(`\s+[-–]\s+`)
)

// ParseFile reads a file name (with its extension): "05-Fue Amor.mp3",
// "1-05 Title.flac", "07 - Curare - Nilze Carvalho.mp3". folderArtist, when
// known, tells "Title - Artist" from "Artist - Title".
func ParseFile(name, folderArtist string) File {
	var f File
	s := strings.TrimSuffix(name, path.Ext(name))
	s = reYouTubeID.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "⧸", "/") // yt-dlp's stand-in for '/'
	s = strings.ReplaceAll(s, "_", " ")
	if m := reTrack.FindStringSubmatchIndex(s); m != nil && m[1] < len(s) {
		if m[2] >= 0 {
			f.Disc, _ = strconv.Atoi(s[m[2]:m[3]])
			f.Track, _ = strconv.Atoi(s[m[4]:m[5]])
		} else {
			f.Track, _ = strconv.Atoi(s[m[6]:m[7]])
		}
		s = s[m[1]:]
	}
	var parts []string
	for _, p := range reFileSplit.Split(s, -1) {
		if p = tidy(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) > 1 && reYearOnly.MatchString(parts[0]) {
		f.Year, _ = strconv.Atoi(parts[0])
		parts = parts[1:]
	}
	switch {
	case len(parts) == 0:
		f.Title = tidy(s)
	case len(parts) == 1:
		f.Title = parts[0]
	default:
		last := parts[len(parts)-1]
		head := strings.Join(parts[:len(parts)-1], " - ")
		switch {
		case reCredit.MatchString(last):
			f.Title, f.Artist = head, tidy(reCredit.ReplaceAllString(last, ""))
		case isArtist(last, folderArtist):
			f.Title, f.Artist = head, last
		default:
			f.Artist, f.Title = parts[0], strings.Join(parts[1:], " - ")
		}
	}
	return f
}

// isArtist reports whether s names the folder's artist (possibly with others:
// "Silvia Barrios / Daniel Barrera").
func isArtist(s, folderArtist string) bool {
	a := Norm(folderArtist)
	return a != "" && strings.HasPrefix(Norm(s), a)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/nameparse/ -v`
Expected: PASS (folder and file tests).

- [ ] **Step 5: Commit**

```bash
git add internal/nameparse
git commit -m "feat: file name parser (track, disc, artist, title)"
```

---
### Task 15: Grouping tracks into albums

**Files:**
- Create: `internal/grouping/grouping.go`, `internal/grouping/albums.go`, `internal/grouping/albums_test.go`

**Interfaces:**
- Consumes: `audiofile.Tags` (Task 4); `nameparse.ParseFolder`, `ParseFile`, `StripDisc`, `Norm`, `IsVarious` (Tasks 13–14); `natsort.Less` (Task 1).
- Produces:

```go
package grouping

const Various = "Varios artistas"

type Track struct { // input: one readable audio file
	FileID     int64
	Path       string // catalog form
	Tags       audiofile.Tags
	DurationMs int64
}

type Image struct { // input: an image file (cover candidates)
	Path string
	Size int64
}

type TrackOut struct {
	FileID     int64
	AlbumKey   string // "" for a loose track
	Dir        string
	Title      string
	Artist     string
	Disc       int
	Track      int
	Year       int
	DurationMs int64
}

type Album struct {
	Key         string // album folder + "\x00" + normalized title; stable across scans
	Dir         string // the album's folder (the parent of its disc folders)
	Title       string
	Artist      string // album artist, or Various
	Year        int
	Compilation bool
	Discs       int
	Tracks      int
	DurationMs  int64
	Position    int    // order inside its parent collection (0: none)
	CoverPath   string // image file next to the album ("" if none)
	CoverFileID int64  // a track with an embedded picture (0 if none)
	Genres      []string
}

type Folder struct { // filled by Task 16
	Path       string
	Parent     string // "" for a root
	Name       string
	Tracks     int // recursive
	Albums     int // recursive
	DurationMs int64
	Collection bool
}

type Result struct {
	Albums  []Album
	Tracks  []TrackOut // one per input track: album tracks in album order, then loose tracks
	Folders []Folder
}

func Build(tracks []Track, images []Image, roots []string) Result
```

Rules (spec §4.2): an album is keyed by its folder plus its normalized title (without disc marker). Disc folders (`CD1`, `Disc 2`, `… Cd 02`) belong to their parent folder. The title comes from the `album` tag, or from the folder name when the folder holds at least 2 tracks and is not a root. Tracks with neither are **loose** (`AlbumKey == ""`). Album artist: the most common `albumartist` tag; else the folder's artist; else the only track artist; else `Varios artistas` with `Compilation = true`. A "various" album artist tag or folder name (`VA - …`) also makes a compilation.

- [ ] **Step 1: Write the failing test**

`internal/grouping/albums_test.go`:

```go
package grouping

import (
	"reflect"
	"testing"

	"tordo/internal/audiofile"
)

var roots = []string{"../musica"}

func tr(id int64, p string, tags audiofile.Tags) Track {
	return Track{FileID: id, Path: p, Tags: tags, DurationMs: 1000}
}

func albumByTitle(t *testing.T, r Result, title string) Album {
	t.Helper()
	for _, a := range r.Albums {
		if a.Title == title {
			return a
		}
	}
	t.Fatalf("no album %q in %+v", title, r.Albums)
	return Album{}
}

func TestAlbumFromTags(t *testing.T) {
	d := "../musica/Soulseek/complete/radionauta/Fito Páez/(1990) Tercer Mundo"
	r := Build([]Track{
		tr(2, d+"/02 - B.mp3", audiofile.Tags{Title: "B", Artist: "Fito Páez", Album: "Tercer Mundo", Track: 2, Year: 1990, Genres: []string{"Rock"}}),
		tr(1, d+"/01 - A.mp3", audiofile.Tags{Title: "A", Artist: "Fito Páez", Album: "Tercer Mundo", Track: 1, Year: 1990, Genres: []string{"rock", "Latino"}, HasCover: true}),
	}, []Image{{Path: d + "/folder.jpg", Size: 10}, {Path: d + "/back.jpg", Size: 99}}, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := r.Albums[0]
	want := Album{Key: d + "\x00tercer mundo", Dir: d, Title: "Tercer Mundo", Artist: "Fito Páez", Year: 1990, Discs: 1, Tracks: 2,
		DurationMs: 2000, CoverPath: d + "/folder.jpg", CoverFileID: 1, Genres: []string{"rock", "Latino"}}
	if !reflect.DeepEqual(a, want) {
		t.Fatalf("got  %+v\nwant %+v", a, want)
	}
	if r.Tracks[0].FileID != 1 || r.Tracks[1].FileID != 2 || r.Tracks[0].AlbumKey != a.Key {
		t.Fatalf("tracks out of order: %+v", r.Tracks)
	}
}

func TestAlbumFromFolderNameWhenUntagged(t *testing.T) {
	d := "../musica/Soulseek/complete/morcal/Nilze_Carvalho/Nilze Carvalho - Choro de Menina - vol 2 - LP CID 1981"
	r := Build([]Track{
		tr(1, d+"/07 - Curare - Nilze Carvalho.mp3", audiofile.Tags{}),
		tr(2, d+"/08 - Doce de coco.mp3", audiofile.Tags{}),
	}, nil, roots)
	a := albumByTitle(t, r, "Choro de Menina - vol 2 - LP CID")
	if a.Artist != "Nilze Carvalho" || a.Year != 1981 || a.Tracks != 2 {
		t.Fatalf("album = %+v", a)
	}
	got := r.Tracks[0]
	if got.Title != "Curare" || got.Artist != "Nilze Carvalho" || got.Track != 7 || got.Year != 1981 {
		t.Fatalf("track = %+v", got)
	}
}

func TestDiscFoldersMergeIntoOneAlbum(t *testing.T) {
	base := "../musica/Soulseek/complete/morcal/Noel Rosa"
	r := Build([]Track{
		tr(1, base+"/Noel Rosa - Noel pela primeira vez Cd 01/01 - x.mp3", audiofile.Tags{Title: "x", Artist: "Noel Rosa", Album: "Noel pela primeira vez", Track: 1}),
		tr(2, base+"/Noel Rosa - Noel pela primeira vez Cd 02 -x/16 - y.mp3", audiofile.Tags{Title: "y", Artist: "Noel Rosa", Album: "Noel pela primeira vez CD 2", Track: 16}),
	}, nil, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := r.Albums[0]
	if a.Dir != base || a.Discs != 2 || a.Tracks != 2 || a.Title != "Noel pela primeira vez" {
		t.Fatalf("album = %+v", a)
	}
	if r.Tracks[0].Disc != 1 || r.Tracks[1].Disc != 2 {
		t.Fatalf("discs = %+v", r.Tracks)
	}
}

func TestCompilations(t *testing.T) {
	va := "../musica/Brasil/VA - História da MPB"
	mixed := "../musica/Uruguay/Selección de murgas"
	tagged := "../musica/Brasil/AAVV - Vinicius de Moraes Songbook (1993)"
	r := Build([]Track{
		tr(1, va+"/01.mp3", audiofile.Tags{Title: "a", Artist: "Billy Blanco", Album: "História da MPB"}),
		tr(2, va+"/02.mp3", audiofile.Tags{Title: "b", Artist: "Billy Blanco", Album: "História da MPB"}),
		tr(3, mixed+"/a.mp3", audiofile.Tags{Title: "a", Artist: "La Nueva Milonga", Album: "Murgas"}),
		tr(4, mixed+"/b.mp3", audiofile.Tags{Title: "b", Artist: "Falta y Resto", Album: "Murgas"}),
		tr(5, tagged+"/01.mp3", audiofile.Tags{Title: "a", Artist: "Tom Jobim", AlbumArtist: "Various Artists", Album: "Songbook"}),
	}, nil, roots)
	for _, title := range []string{"História da MPB", "Murgas", "Songbook"} {
		a := albumByTitle(t, r, title)
		if a.Artist != Various || !a.Compilation {
			t.Errorf("%s: artist %q, compilation %v", title, a.Artist, a.Compilation)
		}
	}
}

func TestLooseTracks(t *testing.T) {
	r := Build([]Track{
		tr(1, "../musica/Criosho [K-NtwAQ8Oqk].opus", audiofile.Tags{Title: "Criosho"}),
		tr(2, "../musica/youtube/Gilberto Gil - Aquele Abraço.opus", audiofile.Tags{}),
		tr(3, "../musica/Uruguay/suelto.mp3", audiofile.Tags{Title: "Suelto", Album: "Con álbum"}),
	}, nil, roots)
	if len(r.Albums) != 1 || r.Albums[0].Title != "Con álbum" {
		t.Fatalf("albums = %+v", r.Albums)
	}
	loose := map[int64]TrackOut{}
	for _, x := range r.Tracks {
		if x.AlbumKey == "" {
			loose[x.FileID] = x
		}
	}
	if len(loose) != 2 {
		t.Fatalf("loose = %+v", loose)
	}
	if g := loose[2]; g.Artist != "Gilberto Gil" || g.Title != "Aquele Abraço" {
		t.Fatalf("parsed loose track = %+v", g)
	}
}

func TestPositionFromFolderPrefix(t *testing.T) {
	d := "../musica/Latinoamérica 600/[LA-599] Rata Blanca - (1990) Magos, Espadas Y Rosas"
	r := Build([]Track{tr(1, d+"/01.mp3", audiofile.Tags{Title: "a", Artist: "Rata Blanca", Album: "Magos, Espadas y Rosas"})}, nil, roots)
	if r.Albums[0].Position != 599 {
		t.Fatalf("Position = %d", r.Albums[0].Position)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/grouping/`
Expected: FAIL (`undefined: Build`).

- [ ] **Step 3: Implement the types and the album pass**

`internal/grouping/grouping.go`:

```go
// Package grouping turns the catalog's audio files into tracks, albums,
// folders and collections. It is a pure function of its input: the scanner
// rebuilds the derived tables from it after every scan.
package grouping

import (
	"path"
	"strings"

	"tordo/internal/audiofile"
)

const Various = "Varios artistas"

type Track struct {
	FileID     int64
	Path       string // catalog form
	Tags       audiofile.Tags
	DurationMs int64
}

type Image struct {
	Path string
	Size int64
}

type TrackOut struct {
	FileID     int64
	AlbumKey   string // "" for a loose track
	Dir        string
	Title      string
	Artist     string
	Disc       int
	Track      int
	Year       int
	DurationMs int64
}

type Album struct {
	Key         string // album folder + "\x00" + normalized title
	Dir         string // the album's folder (the parent of its disc folders)
	Title       string
	Artist      string // album artist, or Various
	Year        int
	Compilation bool
	Discs       int
	Tracks      int
	DurationMs  int64
	Position    int    // order inside its parent collection (0: none)
	CoverPath   string // image file next to the album ("" if none)
	CoverFileID int64  // a track with an embedded picture (0 if none)
	Genres      []string
}

type Folder struct {
	Path       string
	Parent     string // "" for a root
	Name       string
	Tracks     int // recursive
	Albums     int // recursive
	DurationMs int64
	Collection bool
}

type Result struct {
	Albums  []Album
	Tracks  []TrackOut
	Folders []Folder
}

// Build groups tracks into albums, folders and collections. roots are the
// configured roots in catalog form; tracks outside them are ignored for
// folders.
func Build(tracks []Track, images []Image, roots []string) Result {
	isRoot := map[string]bool{}
	for _, r := range roots {
		isRoot[path.Clean(r)] = true
	}
	res := buildAlbums(tracks, images, isRoot)
	res.Folders = buildFolders(res, isRoot)
	return res
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func stem(p string) string { return strings.TrimSuffix(path.Base(p), path.Ext(p)) }
```

`internal/grouping/albums.go`:

```go
package grouping

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"tordo/internal/audiofile"
	"tordo/internal/nameparse"
	"tordo/internal/natsort"
)

type member struct {
	out      TrackOut
	path     string
	tags     audiofile.Tags
	album    string           // album title without disc marker
	albumDir string           // the album's folder
	folder   nameparse.Folder // what the album folder's name says
}

func buildAlbums(tracks []Track, images []Image, isRoot map[string]bool) Result {
	perDir := map[string]int{}
	for _, t := range tracks {
		perDir[path.Dir(t.Path)]++
	}
	parsed := map[string]nameparse.Folder{}
	parse := func(dir string) nameparse.Folder {
		if isRoot[dir] || !strings.Contains(dir, "/") {
			return nameparse.Folder{}
		}
		f, ok := parsed[dir]
		if !ok {
			f = nameparse.ParseFolder(path.Base(dir))
			parsed[dir] = f
		}
		return f
	}

	groups := map[string][]*member{}
	var order []string
	var loose []*member
	for _, t := range tracks {
		dir := path.Dir(t.Path)
		df := parse(dir)
		albumDir, af := dir, df
		if df.Disc > 0 && !isRoot[dir] {
			albumDir = path.Dir(dir)
			if df.Album == "" && df.Artist == "" { // a bare "CD1": the parent names the album
				af = parse(albumDir)
				af.Disc = df.Disc
			}
		}
		tg := t.Tags
		ff := nameparse.ParseFile(path.Base(t.Path), firstOf(tg.AlbumArtist, af.Artist))
		m := &member{
			path: t.Path, tags: tg, albumDir: albumDir, folder: af,
			out: TrackOut{
				FileID: t.FileID, Dir: dir, DurationMs: t.DurationMs,
				Title:  firstOf(tg.Title, ff.Title, stem(t.Path)),
				Artist: firstOf(tg.Artist, ff.Artist, tg.AlbumArtist, af.Artist),
				Disc:   firstNonZero(tg.Disc, af.Disc, ff.Disc),
				Track:  firstNonZero(tg.Track, ff.Track),
				Year:   firstNonZero(tg.Year, ff.Year, af.Year),
			},
		}
		title := tg.Album
		if title == "" && !isRoot[albumDir] && perDir[dir] >= 2 {
			title = af.Album
		}
		m.album, _ = nameparse.StripDisc(title)
		if m.album == "" {
			loose = append(loose, m)
			continue
		}
		key := albumDir + "\x00" + nameparse.Norm(m.album)
		m.out.AlbumKey = key
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], m)
	}

	covers := indexImages(images)
	var res Result
	for _, key := range order {
		ms := groups[key]
		sort.SliceStable(ms, func(i, j int) bool { return memberLess(ms[i], ms[j]) })
		a := makeAlbum(key, ms)
		a.CoverPath = covers.pick(a.Dir, ms)
		res.Albums = append(res.Albums, a)
		for _, m := range ms {
			res.Tracks = append(res.Tracks, m.out)
		}
	}
	sort.SliceStable(loose, func(i, j int) bool { return natsort.Less(loose[i].path, loose[j].path) })
	for _, m := range loose {
		res.Tracks = append(res.Tracks, m.out)
	}
	return res
}

func memberLess(a, b *member) bool {
	if a.out.Disc != b.out.Disc {
		return a.out.Disc < b.out.Disc
	}
	if a.out.Track != b.out.Track {
		return a.out.Track < b.out.Track
	}
	return natsort.Less(a.path, b.path)
}

func makeAlbum(key string, ms []*member) Album {
	a := Album{Key: key, Dir: ms[0].albumDir, Position: ms[0].folder.Position}
	var titles, albumArtists, artists, years counter
	compilations := 0
	for _, m := range ms {
		titles.add(m.album)
		albumArtists.add(m.tags.AlbumArtist)
		artists.add(m.out.Artist)
		if m.out.Year > 0 {
			years.add(strconv.Itoa(m.out.Year))
		}
		if m.tags.Compilation {
			compilations++
		}
		a.Tracks++
		a.DurationMs += m.out.DurationMs
		a.Discs = max(a.Discs, m.out.Disc, 1)
		for _, g := range m.tags.Genres {
			a.Genres = addGenre(a.Genres, g)
		}
		if a.CoverFileID == 0 && m.tags.HasCover {
			a.CoverFileID = m.out.FileID
		}
	}
	a.Title = titles.top()
	a.Year, _ = strconv.Atoi(years.top())
	folder := ms[0].folder
	switch aa := albumArtists.top(); {
	case aa != "" && !nameparse.IsVarious(aa):
		a.Artist = aa
	case aa != "" || folder.Various:
		a.Artist, a.Compilation = Various, true
	case folder.Artist != "":
		a.Artist = folder.Artist
	case artists.distinct() == 1:
		a.Artist = artists.top()
	case artists.distinct() > 1:
		a.Artist, a.Compilation = Various, true
	}
	if compilations*2 > len(ms) {
		a.Compilation = true
	}
	return a
}

func addGenre(gs []string, g string) []string {
	for _, x := range gs {
		if strings.EqualFold(x, g) {
			return gs
		}
	}
	return append(gs, g)
}

// counter counts values by their normalized form and remembers the first
// spelling seen of each.
type counter struct {
	keys []string
	n    map[string]int
	orig map[string]string
}

func (c *counter) add(v string) {
	if v == "" {
		return
	}
	k := nameparse.Norm(v)
	if c.n == nil {
		c.n, c.orig = map[string]int{}, map[string]string{}
	}
	if _, ok := c.n[k]; !ok {
		c.keys = append(c.keys, k)
		c.orig[k] = v
	}
	c.n[k]++
}

// top returns the most frequent value (the first seen on a tie), or "".
func (c *counter) top() string {
	best := ""
	for _, k := range c.keys {
		if best == "" || c.n[k] > c.n[best] {
			best = k
		}
	}
	return c.orig[best]
}

func (c *counter) distinct() int { return len(c.keys) }

// coverIndex finds cover images: by folder, and by parent folder for the
// image folders below an album ("Scans/", "Artwork/").
type coverIndex struct {
	byDir    map[string][]Image
	children map[string][]string // folder → its subfolders that hold images
}

func indexImages(images []Image) coverIndex {
	ix := coverIndex{byDir: map[string][]Image{}, children: map[string][]string{}}
	for _, img := range images {
		d := path.Dir(img.Path)
		if _, ok := ix.byDir[d]; !ok {
			ix.children[path.Dir(d)] = append(ix.children[path.Dir(d)], d)
		}
		ix.byDir[d] = append(ix.byDir[d], img)
	}
	return ix
}

// pick chooses the album's cover: a "cover", "folder" or "front" image is
// best; then one whose name mentions them; then the largest image in the
// album's own folders. Images named "back…" never count.
func (ix coverIndex) pick(albumDir string, ms []*member) string {
	dirs := map[string]bool{albumDir: true}
	for _, m := range ms {
		dirs[m.out.Dir] = true
	}
	best, bestScore, bestSize := "", 0, int64(-1)
	consider := func(img Image, own bool) {
		st := strings.ToLower(stem(img.Path))
		score := 0
		switch {
		case strings.Contains(st, "back") || strings.Contains(st, "contra"):
			score = 0
		case st == "cover" || st == "folder" || st == "front":
			score = 3
		case strings.Contains(st, "cover") || strings.Contains(st, "folder") || strings.Contains(st, "front"):
			score = 2
		case own:
			score = 1
		}
		if score > bestScore || (score == bestScore && score > 0 && img.Size > bestSize) {
			best, bestScore, bestSize = img.Path, score, img.Size
		}
	}
	for d := range dirs {
		for _, img := range ix.byDir[d] {
			consider(img, true)
		}
	}
	for _, sub := range ix.children[albumDir] {
		if !dirs[sub] {
			for _, img := range ix.byDir[sub] {
				consider(img, false)
			}
		}
	}
	return best
}
```

Add a stub so the package compiles before Task 16 (Task 16 replaces it), in `internal/grouping/folders.go`:

```go
package grouping

func buildFolders(res Result, isRoot map[string]bool) []Folder { return nil }
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/grouping/ -v`
Expected: PASS. Note in `TestAlbumFromFolderNameWhenUntagged` the album title keeps "- vol 2 - LP CID" because the folder name only splits at the first " - "; that is expected (names only fill gaps, M2 identification fixes titles).

- [ ] **Step 5: Commit**

```bash
git add internal/grouping
git commit -m "feat: group tracks into albums (disc folders, compilations, covers, loose tracks)"
```

---

### Task 16: Folders and collections

**Files:**
- Modify: `internal/grouping/folders.go` (replace the stub)
- Create: `internal/grouping/folders_test.go`

**Interfaces:**
- Consumes: `Result.Albums`, `Result.Tracks` (Task 15).
- Produces: `Result.Folders`, one `Folder` per folder that holds audio (recursively) under a root, sorted by path in natural order. Roots have `Parent == ""`.

Spec §4.3: a folder is a **collection** when it directly contains (as child folders) at least 3 albums by at least 2 different album artists, and its name is not an artist's name. Roots are never collections.

- [ ] **Step 1: Write the failing test**

`internal/grouping/folders_test.go`:

```go
package grouping

import (
	"testing"

	"tordo/internal/audiofile"
)

func album(id int64, dir, artist, title string) Track {
	return tr(id, dir+"/01.mp3", audiofile.Tags{Title: "t", Artist: artist, AlbumArtist: artist, Album: title})
}

func folderMap(r Result) map[string]Folder {
	m := map[string]Folder{}
	for _, f := range r.Folders {
		m[f.Path] = f
	}
	return m
}

func TestFolderTotals(t *testing.T) {
	r := Build([]Track{
		album(1, "../musica/Uruguay/Alfredo Zitarrosa/Guitarra negra", "Alfredo Zitarrosa", "Guitarra negra"),
		album(2, "../musica/Uruguay/Alfredo Zitarrosa/Adagio en mi país", "Alfredo Zitarrosa", "Adagio en mi país"),
		tr(3, "../musica/suelto.mp3", audiofile.Tags{Title: "x"}),
	}, nil, roots)
	f := folderMap(r)
	root := f["../musica"]
	if root.Parent != "" || root.Tracks != 3 || root.Albums != 2 || root.DurationMs != 3000 || root.Name != "musica" {
		t.Errorf("root = %+v", root)
	}
	z := f["../musica/Uruguay/Alfredo Zitarrosa"]
	if z.Parent != "../musica/Uruguay" || z.Tracks != 2 || z.Albums != 2 || z.Collection {
		t.Errorf("artist folder = %+v", z)
	}
	if len(r.Folders) != 5 {
		t.Errorf("folders = %d, want 5 (root, Uruguay, artist, 2 albums)", len(r.Folders))
	}
	if r.Folders[0].Path != "../musica" {
		t.Errorf("first folder = %q, want the root", r.Folders[0].Path)
	}
}

func TestCollections(t *testing.T) {
	la := "../musica/Latinoamérica 600"
	r := Build([]Track{
		album(1, la+"/[LA-598] Cypress Hill - (1999) Los Grandes Éxitos En Español", "Cypress Hill", "Los Grandes Éxitos En Español"),
		album(2, la+"/[LA-599] Rata Blanca - (1990) Magos, Espadas Y Rosas", "Rata Blanca", "Magos, Espadas Y Rosas"),
		album(3, la+"/[LA-600] Ricardo Villalobos - (2003) - Alcachofa", "Ricardo Villalobos", "Alcachofa"),
		// An artist's folder: same artist everywhere.
		album(4, "../musica/Noel Rosa/A", "Noel Rosa", "A"),
		album(5, "../musica/Noel Rosa/B", "Noel Rosa", "B"),
		album(6, "../musica/Noel Rosa/C", "Noel Rosa", "C"),
		// Named after an artist, even with a tribute album inside.
		album(7, "../musica/Fito Páez/Tercer Mundo", "Fito Páez", "Tercer Mundo"),
		album(8, "../musica/Fito Páez/Giros", "Fito Páez", "Giros"),
		album(9, "../musica/Fito Páez/Tributo", "Varios", "Tributo a Fito"),
		// Only two albums.
		album(10, "../musica/Dos/A", "X", "A"),
		album(11, "../musica/Dos/B", "Y", "B"),
	}, nil, roots)
	f := folderMap(r)
	if !f[la].Collection {
		t.Errorf("%s should be a collection: %+v", la, f[la])
	}
	for _, p := range []string{"../musica", "../musica/Noel Rosa", "../musica/Fito Páez", "../musica/Dos"} {
		if f[p].Collection {
			t.Errorf("%s should not be a collection", p)
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/grouping/ -run 'Folder|Collections'`
Expected: FAIL (the stub returns no folders).

- [ ] **Step 3: Implement**

Replace `internal/grouping/folders.go`:

```go
package grouping

import (
	"path"
	"sort"
	"strings"

	"tordo/internal/nameparse"
	"tordo/internal/natsort"
)

type folderTotals struct {
	tracks, albums int
	duration       int64
}

func buildFolders(res Result, isRoot map[string]bool) []Folder {
	totals := map[string]*folderTotals{}
	// up applies fn to dir and every folder above it, up to its root. Folders
	// outside every root are ignored.
	up := func(dir string, fn func(*folderTotals)) {
		if rootOf(dir, isRoot) == "" {
			return
		}
		for d := dir; ; d = path.Dir(d) {
			t := totals[d]
			if t == nil {
				t = &folderTotals{}
				totals[d] = t
			}
			fn(t)
			if isRoot[d] {
				return
			}
		}
	}
	for _, tr := range res.Tracks {
		up(tr.Dir, func(t *folderTotals) { t.tracks++; t.duration += tr.DurationMs })
	}
	for _, a := range res.Albums {
		up(a.Dir, func(t *folderTotals) { t.albums++ })
	}

	artists := map[string]bool{}
	for _, tr := range res.Tracks {
		artists[nameparse.Norm(tr.Artist)] = true
	}
	direct := map[string][]Album{} // folder → albums in its child folders
	for _, a := range res.Albums {
		artists[nameparse.Norm(a.Artist)] = true
		if !isRoot[a.Dir] {
			direct[path.Dir(a.Dir)] = append(direct[path.Dir(a.Dir)], a)
		}
	}
	delete(artists, nameparse.Norm(Various))
	delete(artists, "")

	out := make([]Folder, 0, len(totals))
	for p, t := range totals {
		f := Folder{Path: p, Name: path.Base(p), Tracks: t.tracks, Albums: t.albums, DurationMs: t.duration}
		if !isRoot[p] {
			f.Parent = path.Dir(p)
			f.Collection = isCollection(f.Name, direct[p], artists)
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return natsort.Less(out[i].Path, out[j].Path) })
	return out
}

// isCollection: at least 3 albums in child folders, by at least 2 different
// album artists, under a name that is not an artist's.
func isCollection(name string, albums []Album, artists map[string]bool) bool {
	if len(albums) < 3 {
		return false
	}
	distinct := map[string]bool{}
	for _, a := range albums {
		distinct[nameparse.Norm(a.Artist)] = true
	}
	return len(distinct) >= 2 && !artists[nameparse.Norm(name)]
}

// rootOf returns the root that contains dir (or is dir), or "".
func rootOf(dir string, isRoot map[string]bool) string {
	for r := range isRoot {
		if dir == r || strings.HasPrefix(dir, r+"/") {
			return r
		}
	}
	return ""
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/grouping/ -v`
Expected: PASS (all grouping tests). In `TestCollections`, `Fito Páez` is excluded because its name normalizes to an artist name, and `Noel Rosa` because all its albums share one artist.

- [ ] **Step 5: Commit**

```bash
git add internal/grouping
git commit -m "feat: folder tree totals and collection detection"
```

---
### Task 17: SQLite store — files, reads and derived tables

**Files:**
- Create: `internal/store/schema.sql`, `internal/store/store.go`, `internal/store/files.go`, `internal/store/reads.go`, `internal/store/catalog.go`
- Create: `internal/store/files_test.go`, `internal/store/catalog_test.go`

**Interfaces:**
- Consumes: `audiofile.Info`, `audiofile.Tags` (Task 4); `grouping.Track`, `grouping.Image`, `grouping.Result` (Tasks 15–16); `natsort.Less` (Task 1).
- Produces:

```go
func Open(file string, readOnly bool) (*Store, error)
func OpenMemory() (*Store, error)
func (s *Store) Close() error

type FileRow struct { Path string; Size, MTime int64; Kind string }
func (s *Store) SyncFiles(seen []FileRow, scannedRoots []string) error
func (s *Store) MarkOutsideRoots(roots []string) error

type ReadEnv struct { Version int; FFprobe bool }
type ReadTarget struct { FileID int64; Path string; Size, MTime int64 }
type ReadResult struct { Target ReadTarget; Info audiofile.Info; Fingerprint string; Err string; Env ReadEnv }
func (s *Store) PendingReads(env ReadEnv) ([]ReadTarget, error)
func (s *Store) SaveReads(rs []ReadResult) error
func (s *Store) GroupingInput() ([]grouping.Track, []grouping.Image, error)
func (s *Store) ReplaceGrouping(r grouping.Result) error

type Counts struct { Files, Audio, Unreadable, Tracks, Albums, Folders, Collections int }
type AlbumView struct { ID int64; Title, Artist string; Year int; Compilation bool; Discs, Tracks int; DurationMs int64; Dir string; Position int; CoverPath string; CoverFileID int64; Genres []string }
type TrackView struct { ID int64; Path, Title, Artist string; Disc, Track, Year int; DurationMs int64; Format, Codec string; Lossless bool; Bitrate, SampleRate, BitDepth int; Fingerprint string }
type AlbumDetail struct { AlbumView; Tracks []TrackView }
type FolderSummary struct { Path, Name string; TrackCount, AlbumCount int; DurationMs int64; Collection bool }
type FolderView struct { FolderSummary; Parent string; Children []FolderSummary; Albums []AlbumView; Loose []TrackView }
type CollectionView struct { FolderSummary; Albums []AlbumView }
func (s *Store) Counts() (Counts, error)
func (s *Store) Albums(folder string) ([]AlbumView, error)       // folder "" = all
func (s *Store) Album(id int64) (AlbumDetail, bool, error)
func (s *Store) Folder(p string) (FolderView, bool, error)        // p "" = the roots
func (s *Store) Collections() ([]CollectionView, error)
```

Design notes:
- `files` keeps every file ever seen (missing ones are flagged, never deleted), so later milestones can keep plays, likes and corrections.
- `audio` caches what reading a file produced, valid while `size`/`mtime` match. `PendingReads` returns new and changed files, files read by an older `audiofile.Version`, and failures that ffprobe might now fix.
- A file's `first_seen` is inherited from any other file with the same fingerprint (a moved file is not "new").
- `albums.id` stays stable across rebuilds because `ReplaceGrouping` upserts by `key` and deletes only albums that disappeared.
- `SetMaxOpenConns(1)`: collect rows and close them before running another query.

- [ ] **Step 1: Write the schema and the store skeleton**

`internal/store/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS files (
	id          INTEGER PRIMARY KEY,
	path        TEXT NOT NULL UNIQUE,
	size        INTEGER NOT NULL,
	mtime       INTEGER NOT NULL,
	kind        TEXT NOT NULL,
	fingerprint TEXT NOT NULL DEFAULT '',
	missing     INTEGER NOT NULL DEFAULT 0,
	first_seen  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS files_fingerprint ON files(fingerprint);

-- What reading an audio file produced; valid while size and mtime match.
CREATE TABLE IF NOT EXISTS audio (
	file_id      INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
	size         INTEGER NOT NULL,
	mtime        INTEGER NOT NULL,
	read_version INTEGER NOT NULL,
	with_ffprobe INTEGER NOT NULL,
	reader       TEXT NOT NULL DEFAULT '',
	error        TEXT NOT NULL DEFAULT '',
	format       TEXT NOT NULL DEFAULT '',
	codec        TEXT NOT NULL DEFAULT '',
	lossless     INTEGER NOT NULL DEFAULT 0,
	duration_ms  INTEGER NOT NULL DEFAULT 0,
	bitrate      INTEGER NOT NULL DEFAULT 0,
	sample_rate  INTEGER NOT NULL DEFAULT 0,
	bit_depth    INTEGER NOT NULL DEFAULT 0,
	channels     INTEGER NOT NULL DEFAULT 0,
	tags         TEXT NOT NULL DEFAULT '{}'
);

-- Derived tables, rebuilt from files + audio after every scan.
CREATE TABLE IF NOT EXISTS albums (
	id            INTEGER PRIMARY KEY,
	key           TEXT NOT NULL UNIQUE,
	dir           TEXT NOT NULL,
	title         TEXT NOT NULL,
	artist        TEXT NOT NULL,
	year          INTEGER NOT NULL,
	compilation   INTEGER NOT NULL,
	discs         INTEGER NOT NULL,
	tracks        INTEGER NOT NULL,
	duration_ms   INTEGER NOT NULL,
	position      INTEGER NOT NULL,
	cover_path    TEXT NOT NULL,
	cover_file_id INTEGER NOT NULL,
	genres        TEXT NOT NULL,
	seen          INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS albums_dir ON albums(dir);

CREATE TABLE IF NOT EXISTS tracks (
	file_id     INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
	album_id    INTEGER REFERENCES albums(id),
	dir         TEXT NOT NULL,
	title       TEXT NOT NULL,
	artist      TEXT NOT NULL,
	disc        INTEGER NOT NULL,
	track       INTEGER NOT NULL,
	year        INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS tracks_album ON tracks(album_id);
CREATE INDEX IF NOT EXISTS tracks_dir ON tracks(dir);

CREATE TABLE IF NOT EXISTS folders (
	path        TEXT PRIMARY KEY,
	parent      TEXT NOT NULL,
	name        TEXT NOT NULL,
	tracks      INTEGER NOT NULL,
	albums      INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL,
	collection  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS folders_parent ON folders(parent);
```

`internal/store/store.go`:

```go
// Package store persists the catalog in a SQLite file in the app folder.
package store

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
}

// Open opens (creating it if needed) the catalog. Journal mode DELETE instead
// of WAL: safer on removable drives. readOnly opens with query_only.
func Open(file string, readOnly bool) (*Store, error) {
	dsn := file + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if readOnly {
		dsn += "&_pragma=query_only(1)"
	} else {
		dsn += "&_pragma=journal_mode(DELETE)"
	}
	return openDB(dsn, !readOnly)
}

// OpenMemory returns an empty in-memory catalog (browse-only mode without a
// catalog file).
func OpenMemory() (*Store, error) {
	return openDB(":memory:?_pragma=foreign_keys(1)", true)
}

func openDB(dsn string, migrate bool) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if migrate {
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
```

Add the dependency:

```bash
go get modernc.org/sqlite@v1.59.0
```

- [ ] **Step 2: Write the failing test for files and reads**

`internal/store/files_test.go`:

```go
package store

import (
	"testing"

	"tordo/internal/audiofile"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var root = []string{"../musica"}

func audioRow(p string, size, mtime int64) FileRow {
	return FileRow{Path: p, Size: size, MTime: mtime, Kind: "audio"}
}

func save(t *testing.T, s *Store, env ReadEnv, fp string, info audiofile.Info) {
	t.Helper()
	pending, err := s.PendingReads(env)
	if err != nil {
		t.Fatal(err)
	}
	var rs []ReadResult
	for _, p := range pending {
		rs = append(rs, ReadResult{Target: p, Info: info, Fingerprint: fp + p.Path, Env: env})
	}
	if err := s.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
}

func TestPendingReadsFollowChanges(t *testing.T) {
	s := open(t)
	env := ReadEnv{Version: 1}
	if err := s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../musica/b.mp3", 20, 1), {Path: "../musica/cover.jpg", Size: 5, MTime: 1, Kind: "image"}}, root); err != nil {
		t.Fatal(err)
	}
	pending, _ := s.PendingReads(env)
	if len(pending) != 2 {
		t.Fatalf("pending = %+v, want the 2 audio files", pending)
	}
	save(t, s, env, "fp-", audiofile.Info{Format: "mp3", DurationMs: 1000})
	if pending, _ := s.PendingReads(env); len(pending) != 0 {
		t.Fatalf("pending after saving = %+v", pending)
	}
	// b.mp3 changes: read again; a newer reader version: read everything again.
	if err := s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../musica/b.mp3", 21, 2)}, root); err != nil {
		t.Fatal(err)
	}
	if pending, _ := s.PendingReads(env); len(pending) != 1 || pending[0].Path != "../musica/b.mp3" {
		t.Fatalf("pending = %+v, want b.mp3", pending)
	}
	if pending, _ := s.PendingReads(ReadEnv{Version: 2}); len(pending) != 2 {
		t.Fatalf("pending with a new version = %+v", pending)
	}
}

func TestFailedReadsRetryWhenFFprobeAppears(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{audioRow("../musica/x.ape", 10, 1)}, root)
	pending, _ := s.PendingReads(ReadEnv{Version: 1})
	if err := s.SaveReads([]ReadResult{{Target: pending[0], Err: "formato sin lector propio", Env: ReadEnv{Version: 1}}}); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.PendingReads(ReadEnv{Version: 1}); len(p) != 0 {
		t.Fatalf("a failure is not retried without ffprobe: %+v", p)
	}
	if p, _ := s.PendingReads(ReadEnv{Version: 1, FFprobe: true}); len(p) != 1 {
		t.Fatalf("a failure is retried once ffprobe exists: %+v", p)
	}
	c, _ := s.Counts()
	if c.Unreadable != 1 || c.Audio != 1 {
		t.Fatalf("counts = %+v", c)
	}
}

func TestMissingFiles(t *testing.T) {
	s := open(t)
	env := ReadEnv{Version: 1}
	s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), audioRow("../otra/b.mp3", 10, 1)}, []string{"../musica", "../otra"})
	save(t, s, env, "", audiofile.Info{})
	// ../otra was not scanned (unavailable): b stays. a disappeared from ../musica.
	if err := s.SyncFiles(nil, []string{"../musica"}); err != nil {
		t.Fatal(err)
	}
	tracks, _, _ := s.GroupingInput()
	if len(tracks) != 1 || tracks[0].Path != "../otra/b.mp3" {
		t.Fatalf("tracks = %+v", tracks)
	}
	// ../otra removed from the configuration: its files go missing too.
	if err := s.MarkOutsideRoots([]string{"../musica"}); err != nil {
		t.Fatal(err)
	}
	if tracks, _, _ := s.GroupingInput(); len(tracks) != 0 {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func TestGroupingInputCarriesTags(t *testing.T) {
	s := open(t)
	s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1), {Path: "../musica/folder.jpg", Size: 7, MTime: 1, Kind: "image"}}, root)
	save(t, s, ReadEnv{Version: 1}, "", audiofile.Info{DurationMs: 1234, Tags: audiofile.Tags{Title: "Fue amor", Genres: []string{"Rock"}}})
	tracks, images, err := s.GroupingInput()
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Tags.Title != "Fue amor" || tracks[0].DurationMs != 1234 || tracks[0].Tags.Genres[0] != "Rock" {
		t.Fatalf("tracks = %+v", tracks)
	}
	if len(images) != 1 || images[0].Path != "../musica/folder.jpg" || images[0].Size != 7 {
		t.Fatalf("images = %+v", images)
	}
}

func TestMovedFileKeepsFirstSeen(t *testing.T) {
	s := open(t)
	env := ReadEnv{Version: 1}
	s.SyncFiles([]FileRow{audioRow("../musica/a.mp3", 10, 1)}, root)
	p, _ := s.PendingReads(env)
	s.SaveReads([]ReadResult{{Target: p[0], Fingerprint: "same", Env: env}})
	var before int64
	s.db.QueryRow(`SELECT first_seen FROM files WHERE path = '../musica/a.mp3'`).Scan(&before)
	s.db.Exec(`UPDATE files SET first_seen = 1000 WHERE path = '../musica/a.mp3'`)

	s.SyncFiles([]FileRow{audioRow("../musica/moved/a.mp3", 10, 1)}, root)
	p, _ = s.PendingReads(env)
	s.SaveReads([]ReadResult{{Target: p[0], Fingerprint: "same", Env: env}})
	var after int64
	s.db.QueryRow(`SELECT first_seen FROM files WHERE path = '../musica/moved/a.mp3'`).Scan(&after)
	if after != 1000 {
		t.Fatalf("first_seen = %d, want 1000 (inherited)", after)
	}
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/store/`
Expected: FAIL (`undefined: FileRow` …).

- [ ] **Step 4: Implement files and reads**

`internal/store/files.go`:

```go
package store

import (
	"database/sql"
	"path"
	"strings"
	"time"
)

// FileRow is a catalogued file as the walk saw it.
type FileRow struct {
	Path  string // catalog form
	Size  int64
	MTime int64 // unix ms
	Kind  string
}

// SyncFiles upserts the files seen in a scan and marks as missing the known
// files under scannedRoots that were not seen. Files under other roots keep
// their state, so an unplugged drive does not wipe its entries. A file whose
// size or mtime changed loses its fingerprint until it is read again.
func (s *Store) SyncFiles(seen []FileRow, scannedRoots []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO files (path, size, mtime, kind, missing, first_seen) VALUES (?1, ?2, ?3, ?4, 0, ?5)
		ON CONFLICT(path) DO UPDATE SET
			fingerprint = CASE WHEN files.size != excluded.size OR files.mtime != excluded.mtime THEN '' ELSE files.fingerprint END,
			size = excluded.size, mtime = excluded.mtime, kind = excluded.kind, missing = 0`)
	if err != nil {
		return err
	}
	defer up.Close()
	now := time.Now().UnixMilli()
	present := make(map[string]bool, len(seen))
	for _, f := range seen {
		if _, err := up.Exec(f.Path, f.Size, f.MTime, f.Kind, now); err != nil {
			return err
		}
		present[f.Path] = true
	}
	gone, err := presentPaths(tx, func(p string) bool { return !present[p] && underAny(p, scannedRoots) })
	if err != nil {
		return err
	}
	if err := markMissing(tx, gone); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkOutsideRoots marks as missing the present files that are not under any
// of roots (the configured ones, available or not): what a removed root
// leaves behind. They come back if the root is added again.
func (s *Store) MarkOutsideRoots(roots []string) error {
	clean := make([]string, len(roots))
	for i, r := range roots {
		clean[i] = path.Clean(r)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	gone, err := presentPaths(tx, func(p string) bool { return !underAny(p, clean) })
	if err != nil {
		return err
	}
	if err := markMissing(tx, gone); err != nil {
		return err
	}
	return tx.Commit()
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

// presentPaths returns the present files whose path satisfies keep.
func presentPaths(q querier, keep func(string) bool) ([]string, error) {
	rows, err := q.Query(`SELECT path FROM files WHERE missing = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		if keep(p) {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

func markMissing(q querier, paths []string) error {
	for _, p := range paths {
		if _, err := q.Exec(`UPDATE files SET missing = 1 WHERE path = ?`, p); err != nil {
			return err
		}
	}
	return nil
}

func underAny(p string, roots []string) bool {
	for _, r := range roots {
		if strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/") {
			return true
		}
	}
	return false
}
```


`internal/store/reads.go`:

```go
package store

import (
	"encoding/json"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
)

// ReadEnv describes the reader a scan runs with.
type ReadEnv struct {
	Version int  // audiofile.Version
	FFprobe bool // audiofile.HasFFprobe()
}

// ReadTarget is an audio file to read.
type ReadTarget struct {
	FileID int64
	Path   string
	Size   int64
	MTime  int64
}

// ReadResult is what reading a target produced. Err is set when the file was
// read but not understood: it is kept so the file is not read again until it
// changes (or ffprobe appears).
type ReadResult struct {
	Target      ReadTarget
	Info        audiofile.Info
	Fingerprint string
	Err         string
	Env         ReadEnv
}

// PendingReads returns the present audio files never read, changed since,
// read by an older reader version, or that failed without ffprobe while
// ffprobe is available now.
func (s *Store) PendingReads(env ReadEnv) ([]ReadTarget, error) {
	rows, err := s.db.Query(`SELECT f.id, f.path, f.size, f.mtime FROM files f
		LEFT JOIN audio a ON a.file_id = f.id
		WHERE f.missing = 0 AND f.kind = 'audio'
		  AND (a.file_id IS NULL OR a.size != f.size OR a.mtime != f.mtime OR a.read_version < ?1
		       OR (?2 AND a.error != '' AND a.with_ffprobe = 0))
		ORDER BY f.path`, env.Version, env.FFprobe)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReadTarget
	for rows.Next() {
		var t ReadTarget
		if err := rows.Scan(&t.FileID, &t.Path, &t.Size, &t.MTime); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SaveReads stores a batch of results in one transaction. A file whose
// content was seen before under another path inherits that path's first_seen.
func (s *Store) SaveReads(rs []ReadResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	up, err := tx.Prepare(`INSERT INTO audio (file_id, size, mtime, read_version, with_ffprobe, reader, error, format, codec,
		lossless, duration_ms, bitrate, sample_rate, bit_depth, channels, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_id) DO UPDATE SET size = excluded.size, mtime = excluded.mtime,
		read_version = excluded.read_version, with_ffprobe = excluded.with_ffprobe, reader = excluded.reader,
		error = excluded.error, format = excluded.format, codec = excluded.codec, lossless = excluded.lossless,
		duration_ms = excluded.duration_ms, bitrate = excluded.bitrate, sample_rate = excluded.sample_rate,
		bit_depth = excluded.bit_depth, channels = excluded.channels, tags = excluded.tags`)
	if err != nil {
		return err
	}
	defer up.Close()
	fp, err := tx.Prepare(`UPDATE files SET fingerprint = ?1,
		first_seen = CASE WHEN ?1 = '' THEN first_seen ELSE
			MIN(first_seen, COALESCE((SELECT MIN(first_seen) FROM files WHERE fingerprint = ?1 AND id != ?2), first_seen)) END
		WHERE id = ?2`)
	if err != nil {
		return err
	}
	defer fp.Close()
	for _, r := range rs {
		tags, err := json.Marshal(r.Info.Tags)
		if err != nil {
			return err
		}
		i := r.Info
		if _, err := up.Exec(r.Target.FileID, r.Target.Size, r.Target.MTime, r.Env.Version, r.Env.FFprobe, i.Reader, r.Err,
			i.Format, i.Codec, i.Lossless, i.DurationMs, i.Bitrate, i.SampleRate, i.BitDepth, i.Channels, string(tags)); err != nil {
			return err
		}
		if _, err := fp.Exec(r.Fingerprint, r.Target.FileID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GroupingInput returns the present, readable audio files with their tags,
// and the present images.
func (s *Store) GroupingInput() ([]grouping.Track, []grouping.Image, error) {
	rows, err := s.db.Query(`SELECT f.id, f.path, a.duration_ms, a.tags FROM files f JOIN audio a ON a.file_id = f.id
		WHERE f.missing = 0 AND f.kind = 'audio' AND a.error = '' AND a.size = f.size AND a.mtime = f.mtime
		ORDER BY f.path`)
	if err != nil {
		return nil, nil, err
	}
	var tracks []grouping.Track
	for rows.Next() {
		var t grouping.Track
		var tags string
		if err := rows.Scan(&t.FileID, &t.Path, &t.DurationMs, &tags); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if err := json.Unmarshal([]byte(tags), &t.Tags); err != nil {
			rows.Close()
			return nil, nil, err
		}
		tracks = append(tracks, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	rows, err = s.db.Query(`SELECT path, size FROM files WHERE missing = 0 AND kind = 'image'`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var images []grouping.Image
	for rows.Next() {
		var img grouping.Image
		if err := rows.Scan(&img.Path, &img.Size); err != nil {
			return nil, nil, err
		}
		images = append(images, img)
	}
	return tracks, images, rows.Err()
}
```

`Counts` is used by the test above; add it now in `internal/store/catalog.go` (the rest of the file comes in Step 6):

```go
package store

// Counts summarizes the catalog for the status endpoint.
type Counts struct {
	Files       int `json:"files"`
	Audio       int `json:"audio"`
	Unreadable  int `json:"unreadable"`
	Tracks      int `json:"tracks"`
	Albums      int `json:"albums"`
	Folders     int `json:"folders"`
	Collections int `json:"collections"`
}

func (s *Store) Counts() (Counts, error) {
	var c Counts
	err := s.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM files WHERE missing = 0),
		(SELECT COUNT(*) FROM files WHERE missing = 0 AND kind = 'audio'),
		(SELECT COUNT(*) FROM files f JOIN audio a ON a.file_id = f.id WHERE f.missing = 0 AND a.error != ''),
		(SELECT COUNT(*) FROM tracks),
		(SELECT COUNT(*) FROM albums),
		(SELECT COUNT(*) FROM folders),
		(SELECT COUNT(*) FROM folders WHERE collection = 1)`).
		Scan(&c.Files, &c.Audio, &c.Unreadable, &c.Tracks, &c.Albums, &c.Folders, &c.Collections)
	return c, err
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/store/ -v`
Expected: PASS.

- [ ] **Step 6: Write the failing test for the derived tables and views**

`internal/store/catalog_test.go`:

```go
package store

import (
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
)

// catalog builds a store from tagged files through the real grouping.
func catalog(t *testing.T, files map[string]audiofile.Tags, images ...string) *Store {
	t.Helper()
	s := open(t)
	var rows []FileRow
	for p := range files {
		rows = append(rows, audioRow(p, 10, 1))
	}
	for _, p := range images {
		rows = append(rows, FileRow{Path: p, Size: 1, MTime: 1, Kind: "image"})
	}
	if err := s.SyncFiles(rows, root); err != nil {
		t.Fatal(err)
	}
	env := ReadEnv{Version: 1}
	pending, _ := s.PendingReads(env)
	var rs []ReadResult
	for _, p := range pending {
		rs = append(rs, ReadResult{Target: p, Env: env, Fingerprint: "fp" + p.Path,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", Bitrate: 320, SampleRate: 44100, DurationMs: 1000, Tags: files[p.Path]}})
	}
	if err := s.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	return s
}

func regroup(t *testing.T, s *Store) {
	t.Helper()
	tracks, images, err := s.GroupingInput()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGrouping(grouping.Build(tracks, images, root)); err != nil {
		t.Fatal(err)
	}
}

const la = "../musica/Latinoamérica 600"

func sample(t *testing.T) *Store {
	return catalog(t, map[string]audiofile.Tags{
		la + "/[LA-10] Soda Stereo - Signos/01.mp3":      {Title: "Signos", Artist: "Soda Stereo", Album: "Signos", Track: 1, Year: 1986},
		la + "/[LA-10] Soda Stereo - Signos/02.mp3":      {Title: "No existes", Artist: "Soda Stereo", Album: "Signos", Track: 2, Year: 1986},
		la + "/[LA-9] Charly García - Clics modernos/01.mp3": {Title: "Nos siguen pegando abajo", Artist: "Charly García", Album: "Clics modernos", Year: 1983},
		la + "/[LA-11] Virus - Locura/01.mp3":            {Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", Year: 1985},
		"../musica/suelto.mp3":                            {Title: "Suelto"},
	}, la+"/[LA-10] Soda Stereo - Signos/cover.jpg")
}

func TestAlbumsAndAlbum(t *testing.T) {
	s := sample(t)
	all, err := s.Albums("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Artist != "Charly García" || all[2].Artist != "Virus" {
		t.Fatalf("albums = %+v", all)
	}
	var signos AlbumView
	for _, a := range all {
		if a.Title == "Signos" {
			signos = a
		}
	}
	if signos.Tracks != 2 || signos.Year != 1986 || signos.Position != 10 || signos.CoverPath != la+"/[LA-10] Soda Stereo - Signos/cover.jpg" || signos.Genres == nil {
		t.Fatalf("signos = %+v", signos)
	}
	d, ok, err := s.Album(signos.ID)
	if err != nil || !ok {
		t.Fatalf("Album = %v, %v", ok, err)
	}
	if len(d.Tracks) != 2 || d.Tracks[0].Title != "Signos" || d.Tracks[1].Track != 2 || d.Tracks[0].Bitrate != 320 || d.Tracks[0].Fingerprint == "" {
		t.Fatalf("tracks = %+v", d.Tracks)
	}
	if _, ok, _ := s.Album(9999); ok {
		t.Fatal("Album(9999) found")
	}
	if under, _ := s.Albums(la + "/[LA-9] Charly García - Clics modernos"); len(under) != 1 {
		t.Fatalf("Albums(folder) = %+v", under)
	}
}

func TestAlbumIDsAreStable(t *testing.T) {
	s := sample(t)
	before, _ := s.Albums("")
	regroup(t, s)
	after, _ := s.Albums("")
	for i := range before {
		if before[i].ID != after[i].ID {
			t.Fatalf("album %q changed id: %d → %d", before[i].Title, before[i].ID, after[i].ID)
		}
	}
}

func TestFolderView(t *testing.T) {
	s := sample(t)
	top, ok, err := s.Folder("")
	if err != nil || !ok || len(top.Children) != 1 || top.Children[0].Path != "../musica" {
		t.Fatalf("roots = %+v, %v, %v", top, ok, err)
	}
	r, _, _ := s.Folder("../musica")
	if r.TrackCount != 5 || r.AlbumCount != 3 || len(r.Loose) != 1 || r.Loose[0].Title != "Suelto" || len(r.Children) != 1 {
		t.Fatalf("root = %+v", r)
	}
	l, _, _ := s.Folder(la)
	if !l.Collection || len(l.Children) != 3 || l.Children[0].Name != "[LA-9] Charly García - Clics modernos" || l.Parent != "../musica" {
		t.Fatalf("collection folder = %+v", l)
	}
	if _, ok, _ := s.Folder("../nope"); ok {
		t.Fatal("unknown folder found")
	}
}

func TestCollectionsInOrder(t *testing.T) {
	s := sample(t)
	cs, err := s.Collections()
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Path != la || len(cs[0].Albums) != 3 {
		t.Fatalf("collections = %+v", cs)
	}
	got := []int{cs[0].Albums[0].Position, cs[0].Albums[1].Position, cs[0].Albums[2].Position}
	if got[0] != 9 || got[1] != 10 || got[2] != 11 {
		t.Fatalf("positions = %v", got)
	}
	c, _ := s.Counts()
	if c.Albums != 3 || c.Tracks != 5 || c.Collections != 1 {
		t.Fatalf("counts = %+v", c)
	}
}
```

- [ ] **Step 7: Run it to see it fail**

Run: `go test ./internal/store/ -run 'Albums|Album|Folder|Collections'`
Expected: FAIL (`undefined: (*Store).ReplaceGrouping` …).

- [ ] **Step 8: Implement the derived tables and the views**

Append to `internal/store/catalog.go` (and turn its import line into the block below):

```go
import (
	"database/sql"
	"encoding/json"
	"errors"
	"path"
	"sort"

	"tordo/internal/grouping"
	"tordo/internal/natsort"
)

// ReplaceGrouping stores a grouping result. Albums are upserted by key so
// their ids survive rebuilds; albums that disappeared are deleted.
func (s *Store) ReplaceGrouping(r grouping.Result) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE albums SET seen = 0`); err != nil {
		return err
	}
	up, err := tx.Prepare(`INSERT INTO albums (key, dir, title, artist, year, compilation, discs, tracks, duration_ms,
		position, cover_path, cover_file_id, genres, seen) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(key) DO UPDATE SET dir = excluded.dir, title = excluded.title, artist = excluded.artist,
		year = excluded.year, compilation = excluded.compilation, discs = excluded.discs, tracks = excluded.tracks,
		duration_ms = excluded.duration_ms, position = excluded.position, cover_path = excluded.cover_path,
		cover_file_id = excluded.cover_file_id, genres = excluded.genres, seen = 1
		RETURNING id`)
	if err != nil {
		return err
	}
	defer up.Close()
	ids := make(map[string]int64, len(r.Albums))
	for _, a := range r.Albums {
		genres := a.Genres
		if genres == nil {
			genres = []string{}
		}
		g, err := json.Marshal(genres)
		if err != nil {
			return err
		}
		var id int64
		if err := up.QueryRow(a.Key, a.Dir, a.Title, a.Artist, a.Year, a.Compilation, a.Discs, a.Tracks, a.DurationMs,
			a.Position, a.CoverPath, a.CoverFileID, string(g)).Scan(&id); err != nil {
			return err
		}
		ids[a.Key] = id
	}
	if _, err := tx.Exec(`DELETE FROM tracks`); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO tracks (file_id, album_id, dir, title, artist, disc, track, year, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, t := range r.Tracks {
		var album any
		if t.AlbumKey != "" {
			album = ids[t.AlbumKey]
		}
		if _, err := ins.Exec(t.FileID, album, t.Dir, t.Title, t.Artist, t.Disc, t.Track, t.Year, t.DurationMs); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM albums WHERE seen = 0`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM folders`); err != nil {
		return err
	}
	fins, err := tx.Prepare(`INSERT INTO folders (path, parent, name, tracks, albums, duration_ms, collection) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer fins.Close()
	for _, f := range r.Folders {
		if _, err := fins.Exec(f.Path, f.Parent, f.Name, f.Tracks, f.Albums, f.DurationMs, f.Collection); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type AlbumView struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Year        int      `json:"year"`
	Compilation bool     `json:"compilation"`
	Discs       int      `json:"discs"`
	Tracks      int      `json:"tracks"`
	DurationMs  int64    `json:"durationMs"`
	Dir         string   `json:"dir"`
	Position    int      `json:"position"`
	CoverPath   string   `json:"coverPath"`
	CoverFileID int64    `json:"coverFileId"`
	Genres      []string `json:"genres"`
}

type TrackView struct {
	ID          int64  `json:"id"` // the file id
	Path        string `json:"path"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Disc        int    `json:"disc"`
	Track       int    `json:"track"`
	Year        int    `json:"year"`
	DurationMs  int64  `json:"durationMs"`
	Format      string `json:"format"`
	Codec       string `json:"codec"`
	Lossless    bool   `json:"lossless"`
	Bitrate     int    `json:"bitrate"`
	SampleRate  int    `json:"sampleRate"`
	BitDepth    int    `json:"bitDepth"`
	Fingerprint string `json:"fingerprint"`
}

type AlbumDetail struct {
	AlbumView
	Tracks []TrackView `json:"tracks"`
}

type FolderSummary struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	TrackCount int    `json:"trackCount"` // recursive
	AlbumCount int    `json:"albumCount"` // recursive
	DurationMs int64  `json:"durationMs"`
	Collection bool   `json:"collection"`
}

type FolderView struct {
	FolderSummary
	Parent   string          `json:"parent"`
	Children []FolderSummary `json:"children"`
	Albums   []AlbumView     `json:"albums"` // albums whose folder is this one
	Loose    []TrackView     `json:"loose"`  // tracks here that belong to no album
}

type CollectionView struct {
	FolderSummary
	Albums []AlbumView `json:"albums"` // in collection order
}

const albumCols = `id, title, artist, year, compilation, discs, tracks, duration_ms, dir, position, cover_path, cover_file_id, genres`

func (s *Store) queryAlbums(q string, args ...any) ([]AlbumView, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlbumView{}
	for rows.Next() {
		var a AlbumView
		var genres string
		if err := rows.Scan(&a.ID, &a.Title, &a.Artist, &a.Year, &a.Compilation, &a.Discs, &a.Tracks, &a.DurationMs,
			&a.Dir, &a.Position, &a.CoverPath, &a.CoverFileID, &genres); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(genres), &a.Genres); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const trackQuery = `SELECT t.file_id, f.path, t.title, t.artist, t.disc, t.track, t.year, t.duration_ms,
	a.format, a.codec, a.lossless, a.bitrate, a.sample_rate, a.bit_depth, f.fingerprint
	FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id `

func (s *Store) queryTracks(where string, args ...any) ([]TrackView, error) {
	rows, err := s.db.Query(trackQuery+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TrackView{}
	for rows.Next() {
		var t TrackView
		if err := rows.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Disc, &t.Track, &t.Year, &t.DurationMs,
			&t.Format, &t.Codec, &t.Lossless, &t.Bitrate, &t.SampleRate, &t.BitDepth, &t.Fingerprint); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Albums lists the albums, all of them or those inside folder (at any depth).
func (s *Store) Albums(folder string) ([]AlbumView, error) {
	q := `SELECT ` + albumCols + ` FROM albums`
	var args []any
	if folder != "" {
		q += ` WHERE dir = ?1 OR substr(dir, 1, length(?1) + 1) = ?1 || '/'`
		args = append(args, folder)
	}
	return s.queryAlbums(q+` ORDER BY artist COLLATE NOCASE, year, title COLLATE NOCASE`, args...)
}

func (s *Store) Album(id int64) (AlbumDetail, bool, error) {
	as, err := s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE id = ?`, id)
	if err != nil || len(as) == 0 {
		return AlbumDetail{}, false, err
	}
	tracks, err := s.queryTracks(`WHERE t.album_id = ? ORDER BY t.disc, t.track, f.path`, id)
	if err != nil {
		return AlbumDetail{}, false, err
	}
	return AlbumDetail{AlbumView: as[0], Tracks: tracks}, true, nil
}

// Folder returns a folder with its subfolders, the albums whose folder it is,
// and its loose tracks. p == "" is the virtual top level: the roots.
func (s *Store) Folder(p string) (FolderView, bool, error) {
	v := FolderView{Children: []FolderSummary{}, Albums: []AlbumView{}, Loose: []TrackView{}}
	if p != "" {
		err := s.db.QueryRow(`SELECT path, name, parent, tracks, albums, duration_ms, collection FROM folders WHERE path = ?`, p).
			Scan(&v.Path, &v.Name, &v.Parent, &v.TrackCount, &v.AlbumCount, &v.DurationMs, &v.Collection)
		if errors.Is(err, sql.ErrNoRows) {
			return FolderView{}, false, nil
		}
		if err != nil {
			return FolderView{}, false, err
		}
	}
	children, err := s.folderSummaries(`WHERE parent = ?`, p)
	if err != nil {
		return FolderView{}, false, err
	}
	sort.SliceStable(children, func(i, j int) bool { return natsort.Less(children[i].Name, children[j].Name) })
	v.Children = children
	if p == "" {
		return v, true, nil
	}
	if v.Albums, err = s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE dir = ?`, p); err != nil {
		return FolderView{}, false, err
	}
	sortByPosition(v.Albums)
	if v.Loose, err = s.queryTracks(`WHERE t.dir = ? AND t.album_id IS NULL ORDER BY f.path`, p); err != nil {
		return FolderView{}, false, err
	}
	return v, true, nil
}

func (s *Store) folderSummaries(where string, args ...any) ([]FolderSummary, error) {
	rows, err := s.db.Query(`SELECT path, name, tracks, albums, duration_ms, collection FROM folders `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FolderSummary{}
	for rows.Next() {
		var f FolderSummary
		if err := rows.Scan(&f.Path, &f.Name, &f.TrackCount, &f.AlbumCount, &f.DurationMs, &f.Collection); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Collections lists the detected collections with their albums in order.
func (s *Store) Collections() ([]CollectionView, error) {
	sums, err := s.folderSummaries(`WHERE collection = 1`)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(sums, func(i, j int) bool { return natsort.Less(sums[i].Name, sums[j].Name) })
	out := []CollectionView{}
	for _, f := range sums {
		all, err := s.Albums(f.Path)
		if err != nil {
			return nil, err
		}
		albums := []AlbumView{}
		for _, a := range all {
			if path.Dir(a.Dir) == f.Path {
				albums = append(albums, a)
			}
		}
		sortByPosition(albums)
		out = append(out, CollectionView{FolderSummary: f, Albums: albums})
	}
	return out, nil
}

// sortByPosition orders albums by their collection position (albums without
// one last), then by folder name in natural order.
func sortByPosition(as []AlbumView) {
	sort.SliceStable(as, func(i, j int) bool {
		pi, pj := as[i].Position, as[j].Position
		if (pi == 0) != (pj == 0) {
			return pi != 0
		}
		if pi != pj {
			return pi < pj
		}
		return natsort.Less(path.Base(as[i].Dir), path.Base(as[j].Dir))
	})
}
```

- [ ] **Step 9: Run the tests**

Run: `go test ./internal/store/ -v && go vet ./internal/store/`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum internal/store
git commit -m "feat: SQLite catalog: files, reads, albums, tracks, folders"
```

---
### Task 18: Scanner

**Files:**
- Create: `internal/scan/scan.go`, `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: `appdir.Abs/Rel` (Task 1); `mediafile.Classify/IgnoredDir` (Task 3); `audiofile.Read`, `Version`, `HasFFprobe`, `ErrIO` (Task 11); `fingerprint.Of` (Task 12); `grouping.Build` (Tasks 15–16); the `store` API (Task 17).
- Produces:

```go
var ErrBusy = errors.New("ya hay un escaneo en curso")
type Status struct {
	Running   bool      `json:"running"`
	Files     int64     `json:"files"`  // catalogued files seen by the walk
	ToRead    int       `json:"toRead"` // audio files this run reads
	Read      int       `json:"read"`
	Tracks    int       `json:"tracks"`
	Albums    int       `json:"albums"`
	LastError string    `json:"lastError"`
	Finished  time.Time `json:"finished"`
}
type Scanner struct {
	AppDir string
	Roots  []string // catalog form, cleaned
	Ignore []string
	Store  *store.Store
	Read   func(ctx context.Context, path string) (audiofile.Info, error) // nil: audiofile.Read
	Env    store.ReadEnv                                                 // used when Read is set
}
func (s *Scanner) Status() Status
func (s *Scanner) Run(ctx context.Context) error
```

Three phases (Cinexplorer's scanner, adapted): **walk** every available root (skipping ignored folders and uncatalogued files) and sync `files`; **read** the pending audio files in batches of 100 (an unreadable-now file is retried next scan; a file that vanished with its root stops the phase); **group** the whole catalog again. A root that is unavailable is skipped without touching its files; a root with read errors is synced but nothing under it is marked missing.

- [ ] **Step 1: Write the failing test**

`internal/scan/scan_test.go`:

```go
package scan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tordo/internal/audiofile"
	"tordo/internal/store"
)

// fakeRead treats a file's content as JSON tags ("BAD" is an unreadable format).
func fakeRead(ctx context.Context, p string) (audiofile.Info, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return audiofile.Info{}, fmt.Errorf("%w: %v", audiofile.ErrIO, err)
	}
	var tags audiofile.Tags
	if string(b) == "BAD" || json.Unmarshal(b, &tags) != nil {
		return audiofile.Info{}, fmt.Errorf("%w: prueba", audiofile.ErrFormat)
	}
	return audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: tags, Reader: "native",
		Payload: audiofile.Range{Offset: 0, Length: int64(len(b))}}, nil
}

type lib struct {
	t    *testing.T
	root string
	st   *store.Store
	sc   *Scanner
}

func newLib(t *testing.T) *lib {
	parent := t.TempDir()
	app, root := filepath.Join(parent, "tordo"), filepath.Join(parent, "musica")
	for _, d := range []string{app, root} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	sc := &Scanner{AppDir: app, Roots: []string{"../musica"}, Store: st, Read: fakeRead, Env: store.ReadEnv{Version: 1}}
	return &lib{t: t, root: root, st: st, sc: sc}
}

func (l *lib) write(rel, content string) {
	l.t.Helper()
	p := filepath.Join(l.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		l.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		l.t.Fatal(err)
	}
}

func tags(title, artist, album string, track int) string {
	b, _ := json.Marshal(audiofile.Tags{Title: title, Artist: artist, Album: album, Track: track})
	return string(b)
}

func (l *lib) run() Status {
	l.t.Helper()
	if err := l.sc.Run(context.Background()); err != nil {
		l.t.Fatal(err)
	}
	return l.sc.Status()
}

const magos = "Rata Blanca - (1990) Magos"

func (l *lib) sample() {
	l.write(magos+"/01 - Mujer amante.mp3", tags("Mujer amante", "Rata Blanca", "Magos, Espadas y Rosas", 1))
	l.write(magos+"/02 - La leyenda del hada y el mago.mp3", tags("La leyenda del hada y el mago", "Rata Blanca", "Magos, Espadas y Rosas", 2))
	l.write(magos+"/cover.jpg", "jpeg")
	l.write(magos+"/roto.mp3", "BAD")
	l.write("Ableton/sample.mp3", tags("x", "x", "x", 1))
	l.write("Soulseek/downloading/half.mp3", tags("x", "x", "x", 1))
	l.write(".hidden/y.mp3", tags("y", "y", "y", 1))
	l.write("libro.epub", "epub")
}

func TestScanBuildsCatalog(t *testing.T) {
	l := newLib(t)
	l.sample()
	st := l.run()
	if st.Running || st.Files != 4 || st.ToRead != 3 || st.Read != 3 || st.Tracks != 2 || st.Albums != 1 || st.LastError != "" {
		t.Fatalf("status = %+v", st)
	}
	c, _ := l.st.Counts()
	if c.Audio != 3 || c.Unreadable != 1 || c.Tracks != 2 || c.Albums != 1 {
		t.Fatalf("counts = %+v", c)
	}
	albums, _ := l.st.Albums("")
	if len(albums) != 1 || albums[0].Title != "Magos, Espadas y Rosas" || albums[0].CoverPath != "../musica/"+magos+"/cover.jpg" {
		t.Fatalf("albums = %+v", albums)
	}
}

func TestRescanIsIncremental(t *testing.T) {
	l := newLib(t)
	l.sample()
	l.run()
	if st := l.run(); st.ToRead != 0 {
		t.Fatalf("second scan reads %d files, want 0", st.ToRead)
	}
	p := magos + "/01 - Mujer amante.mp3"
	l.write(p, tags("Mujer amante (remaster)", "Rata Blanca", "Magos, Espadas y Rosas", 1))
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(l.root, filepath.FromSlash(p)), later, later); err != nil {
		t.Fatal(err)
	}
	if st := l.run(); st.ToRead != 1 {
		t.Fatalf("after a change, reads %d files, want 1", st.ToRead)
	}
	albums, _ := l.st.Albums("")
	d, _, _ := l.st.Album(albums[0].ID)
	if d.Tracks[0].Title != "Mujer amante (remaster)" {
		t.Fatalf("title = %q", d.Tracks[0].Title)
	}
}

func TestDeletedFileLeavesTheCatalog(t *testing.T) {
	l := newLib(t)
	l.sample()
	l.run()
	if err := os.Remove(filepath.Join(l.root, magos, "02 - La leyenda del hada y el mago.mp3")); err != nil {
		t.Fatal(err)
	}
	if st := l.run(); st.Tracks != 1 {
		t.Fatalf("tracks = %d, want 1", st.Tracks)
	}
}

func TestUnavailableRootKeepsTheCatalog(t *testing.T) {
	l := newLib(t)
	l.sample()
	l.run()
	if err := os.Rename(l.root, l.root+"-desenchufado"); err != nil {
		t.Fatal(err)
	}
	if st := l.run(); st.Tracks != 2 || st.Albums != 1 {
		t.Fatalf("status = %+v, want the previous catalog", st)
	}
}

func TestBusy(t *testing.T) {
	l := newLib(t)
	l.sample()
	started, release := make(chan struct{}), make(chan struct{})
	l.sc.Read = func(ctx context.Context, p string) (audiofile.Info, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		return fakeRead(ctx, p)
	}
	done := make(chan error)
	go func() { done <- l.sc.Run(context.Background()) }()
	<-started
	if err := l.sc.Run(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/scan/`
Expected: FAIL (`undefined: Scanner`).

- [ ] **Step 3: Implement**

`internal/scan/scan.go`:

```go
// Package scan walks the library roots and refreshes the catalog. It only
// reads the library; the only writes go to the catalog database.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tordo/internal/appdir"
	"tordo/internal/audiofile"
	"tordo/internal/fingerprint"
	"tordo/internal/grouping"
	"tordo/internal/mediafile"
	"tordo/internal/store"
)

var ErrBusy = errors.New("ya hay un escaneo en curso")

type Status struct {
	Running   bool      `json:"running"`
	Files     int64     `json:"files"`  // catalogued files seen by the walk
	ToRead    int       `json:"toRead"` // audio files this run reads
	Read      int       `json:"read"`
	Tracks    int       `json:"tracks"`
	Albums    int       `json:"albums"`
	LastError string    `json:"lastError"`
	Finished  time.Time `json:"finished"`
}

type Scanner struct {
	AppDir string
	Roots  []string // catalog form, cleaned
	Ignore []string // extra folder patterns from config.json
	Store  *store.Store
	// Read reads one audio file; nil means audiofile.Read (and Env from the
	// audiofile package).
	Read func(ctx context.Context, path string) (audiofile.Info, error)
	Env  store.ReadEnv

	mu     sync.Mutex
	status Status
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run performs one incremental scan. A call while another runs returns ErrBusy.
func (s *Scanner) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.status.Running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.status = Status{Running: true}
	s.mu.Unlock()

	err := s.run(ctx)

	s.mu.Lock()
	s.status.Running = false
	s.status.Finished = time.Now()
	if err != nil {
		s.status.LastError = err.Error()
	}
	s.mu.Unlock()
	return err
}

func (s *Scanner) run(ctx context.Context) error {
	attempted, clean, seen, err := s.walk(ctx)
	if err != nil {
		return err
	}
	if err := s.Store.SyncFiles(seen, clean); err != nil {
		return err
	}
	if err := s.Store.MarkOutsideRoots(s.Roots); err != nil {
		return err
	}
	readErr := s.readAll(ctx, attempted)
	if err := ctx.Err(); err != nil {
		return err
	}
	// The catalog is regrouped even when reading stopped early: what was read
	// is already saved.
	return errors.Join(readErr, s.group())
}

// walk lists the catalogued files of every available root. attempted are the
// roots walked at all; clean those walked without read errors (only under
// those may unseen files be marked missing).
func (s *Scanner) walk(ctx context.Context) (attempted, clean []string, seen []store.FileRow, err error) {
	for _, root := range s.Roots {
		abs := appdir.Abs(s.AppDir, root)
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			log.Printf("raíz no disponible, se omite: %s", root)
			continue
		}
		attempted = append(attempted, root)
		rootHadError := false
		werr := filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				rootHadError = true
				log.Printf("no se puede leer %s: %v", p, err)
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() {
				if p == abs {
					return nil
				}
				rel, err := filepath.Rel(abs, p)
				if err != nil {
					return err
				}
				if mediafile.IgnoredDir(filepath.ToSlash(rel), s.Ignore) {
					return fs.SkipDir
				}
				return nil
			}
			kind := mediafile.Classify(d.Name())
			if kind == mediafile.None {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				rootHadError = true
				log.Printf("no se puede leer %s: %v", p, err)
				return nil
			}
			rel, err := appdir.Rel(s.AppDir, p)
			if err != nil {
				return err
			}
			seen = append(seen, store.FileRow{Path: rel, Size: info.Size(), MTime: info.ModTime().UnixMilli(), Kind: string(kind)})
			s.mu.Lock()
			s.status.Files++
			s.mu.Unlock()
			return nil
		})
		if werr != nil {
			return nil, nil, nil, werr
		}
		// A root that vanished mid-walk means the drive was unplugged: keep
		// the previous catalog instead of marking everything missing.
		if _, err := os.Stat(abs); err != nil {
			return nil, nil, nil, fmt.Errorf("la raíz %s desapareció durante el escaneo", root)
		}
		if rootHadError {
			log.Printf("raíz leída en parte por errores de lectura; no se marcan ausentes bajo ella: %s", root)
			continue
		}
		clean = append(clean, root)
	}
	return attempted, clean, seen, nil
}

// readBatch is how many results are committed at once, so an interrupted
// scan keeps most of its work.
const readBatch = 100

// readAll reads the pending audio files under roots: tags, technical data and
// fingerprint. Files that cannot be read now are left for the next scan;
// files read but not understood are stored with their error.
func (s *Scanner) readAll(ctx context.Context, roots []string) error {
	read, env := s.Read, s.Env
	if read == nil {
		read, env = audiofile.Read, store.ReadEnv{Version: audiofile.Version, FFprobe: audiofile.HasFFprobe()}
	}
	pending, err := s.Store.PendingReads(env)
	if err != nil {
		return err
	}
	var targets []store.ReadTarget
	for _, t := range pending {
		if rootOf(t.Path, roots) != "" {
			targets = append(targets, t)
		}
	}
	s.mu.Lock()
	s.status.ToRead = len(targets)
	s.mu.Unlock()

	var batch []store.ReadResult
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := s.Store.SaveReads(batch)
		batch = batch[:0]
		return err
	}
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		abs := appdir.Abs(s.AppDir, t.Path)
		info, err := read(ctx, abs)
		r := store.ReadResult{Target: t, Info: info, Env: env}
		if err == nil {
			fp, ferr := fingerprint.Of(abs, info.Payload.Offset, info.Payload.Length, info.Reader == "native" && info.Format == "ogg")
			if ferr != nil {
				err = fmt.Errorf("%w: %v", audiofile.ErrIO, ferr)
			} else {
				r.Fingerprint = fp
			}
		}
		if err != nil && ctx.Err() == nil {
			root := rootOf(t.Path, roots)
			if _, statErr := os.Stat(appdir.Abs(s.AppDir, root)); statErr != nil {
				return errors.Join(fmt.Errorf("la raíz %s desapareció durante la lectura", root), flush())
			}
		}
		switch {
		case err == nil:
			batch = append(batch, r)
		case ctx.Err() != nil:
			return errors.Join(ctx.Err(), flush())
		case errors.Is(err, audiofile.ErrIO):
			log.Printf("no se puede leer %s, se reintentará: %v", t.Path, err)
		default:
			r.Info, r.Err = audiofile.Info{}, err.Error()
			batch = append(batch, r)
		}
		s.mu.Lock()
		s.status.Read++
		s.mu.Unlock()
		if len(batch) >= readBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}

// group rebuilds albums, tracks and folders from the whole catalog.
func (s *Scanner) group() error {
	tracks, images, err := s.Store.GroupingInput()
	if err != nil {
		return err
	}
	res := grouping.Build(tracks, images, s.Roots)
	if err := s.Store.ReplaceGrouping(res); err != nil {
		return err
	}
	s.mu.Lock()
	s.status.Tracks, s.status.Albums = len(res.Tracks), len(res.Albums)
	s.mu.Unlock()
	return nil
}

// rootOf returns the root (catalog form) that contains p, or "".
func rootOf(p string, roots []string) string {
	for _, r := range roots {
		if strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/") {
			return r
		}
	}
	return ""
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/scan/ -v -race`
Expected: PASS (no data races).

- [ ] **Step 5: Commit**

```bash
git add internal/scan
git commit -m "feat: three-phase incremental scanner"
```

---

### Task 19: JSON API

**Files:**
- Create: `internal/server/server.go`, `internal/server/server_test.go`

**Interfaces:**
- Consumes: the `store` views (Task 17); `scan.Scanner`, `scan.Status`, `scan.ErrBusy` (Task 18).
- Produces: `type server.Server struct { Store *store.Store; Scanner *scan.Scanner; ReadOnly bool }` and `(*Server).Handler() http.Handler` with:

| Method and path | Response |
|---|---|
| `GET /api/status` | `{"readOnly", "scan": scan.Status, "counts": store.Counts}` |
| `POST /api/scan` | 202 `{"started": true}`; 409 in browse-only mode or while a scan runs; 415 without `Content-Type: application/json` |
| `GET /api/albums?folder=` | `[]store.AlbumView` (all, or inside `folder`) |
| `GET /api/albums/{id}` | `store.AlbumDetail`; 400 bad id; 404 unknown |
| `GET /api/folders?path=` | `store.FolderView` (`path` empty: the roots); 404 unknown |
| `GET /api/collections` | `[]store.CollectionView` |

`localOnly` and `jsonOnly` are copied from `/Users/martin/x-projects/cinexplorer/internal/server/server.go` (same logic; comments adapted). LAN mode and the PIN come in M5.

- [ ] **Step 1: Write the failing test**

`internal/server/server_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
	"tordo/internal/scan"
	"tordo/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	roots := []string{"../musica"}
	if err := st.SyncFiles([]store.FileRow{{Path: "../musica/Virus - Locura/01.mp3", Size: 1, MTime: 1, Kind: "audio"}}, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	p, _ := st.PendingReads(env)
	st.SaveReads([]store.ReadResult{{Target: p[0], Env: env, Fingerprint: "fp",
		Info: audiofile.Info{Format: "mp3", DurationMs: 1000, Tags: audiofile.Tags{Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura"}}}})
	tracks, images, _ := st.GroupingInput()
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
	return st
}

func do(t *testing.T, h http.Handler, method, target string, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:4747"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatal(err)
	}
}

func TestReadEndpoints(t *testing.T) {
	h := (&Server{Store: newStore(t)}).Handler()

	var status struct {
		ReadOnly bool         `json:"readOnly"`
		Counts   store.Counts `json:"counts"`
	}
	decode(t, do(t, h, "GET", "/api/status", ""), &status)
	if status.Counts.Albums != 1 || status.Counts.Tracks != 1 {
		t.Fatalf("status = %+v", status)
	}

	var albums []store.AlbumView
	decode(t, do(t, h, "GET", "/api/albums", ""), &albums)
	if len(albums) != 1 || albums[0].Title != "Locura" {
		t.Fatalf("albums = %+v", albums)
	}
	var detail store.AlbumDetail
	decode(t, do(t, h, "GET", "/api/albums/"+itoa(albums[0].ID), ""), &detail)
	if len(detail.Tracks) != 1 || detail.Tracks[0].Title != "Una luna de miel en la mano" {
		t.Fatalf("album = %+v", detail)
	}
	if rec := do(t, h, "GET", "/api/albums/9999", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown album: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/albums/abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", rec.Code)
	}

	var top store.FolderView
	decode(t, do(t, h, "GET", "/api/folders", ""), &top)
	if len(top.Children) != 1 || top.Children[0].Path != "../musica" {
		t.Fatalf("roots = %+v", top)
	}
	if rec := do(t, h, "GET", "/api/folders?path=../nope", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown folder: %d", rec.Code)
	}
	rec := do(t, h, "GET", "/api/collections", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("collections = %d %s", rec.Code, rec.Body)
	}
}

func TestLocalOnly(t *testing.T) {
	h := (&Server{Store: newStore(t)}).Handler()
	req := httptest.NewRequest("GET", "/api/status", nil)
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign host: %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/status", "", "Sec-Fetch-Site", "cross-site", "Sec-Fetch-Mode", "no-cors"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site fetch: %d", rec.Code)
	}
}

func TestScanEndpoint(t *testing.T) {
	st := newStore(t)
	if rec := do(t, (&Server{Store: st, ReadOnly: true}).Handler(), "POST", "/api/scan", "{}", "Content-Type", "application/json"); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only: %d", rec.Code)
	}
	sc := &scan.Scanner{AppDir: t.TempDir(), Store: st, Env: store.ReadEnv{Version: 1},
		Read: func(_ context.Context, _ string) (audiofile.Info, error) { return audiofile.Info{}, nil }}
	h := (&Server{Store: st, Scanner: sc}).Handler()
	if rec := do(t, h, "POST", "/api/scan", "{}"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("without JSON: %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/scan", "{}", "Content-Type", "application/json"); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(5 * time.Second)
	for sc.Status().Finished.IsZero() {
		if time.Now().After(deadline) {
			t.Fatal("the scan did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
```


- [ ] **Step 2: Run it to see it fail**

Run: `go test ./internal/server/`
Expected: FAIL (`undefined: Server`).

- [ ] **Step 3: Implement**

`internal/server/server.go`:

```go
// Package server exposes the catalog as a JSON API. It only answers requests
// addressed to localhost (LAN access and its PIN come in M5).
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"tordo/internal/scan"
	"tordo/internal/store"
)

type Server struct {
	Store    *store.Store
	Scanner  *scan.Scanner // nil in browse-only mode
	ReadOnly bool
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("POST /api/scan", jsonOnly(s.scan))
	mux.HandleFunc("GET /api/albums", s.albums)
	mux.HandleFunc("GET /api/albums/{id}", s.album)
	mux.HandleFunc("GET /api/folders", s.folder)
	mux.HandleFunc("GET /api/collections", s.collections)
	return localOnly(mux)
}

// localOnly rejects requests whose Host is not localhost (DNS rebinding) and
// requests made by other sites' pages, which could otherwise read the catalog
// through the user's browser. Navigating to the app from a link still works.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "prohibido", http.StatusForbidden)
			return
		}
		site := r.Header.Get("Sec-Fetch-Site")
		navigation := r.Method == http.MethodGet && r.Header.Get("Sec-Fetch-Mode") == "navigate"
		if (site == "cross-site" || site == "same-site") && !navigation {
			http.Error(w, "prohibido", http.StatusForbidden)
			return
		}
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// jsonOnly requires a JSON content type, which forces a CORS preflight on
// cross-site requests and so blocks other pages from calling write endpoints.
func jsonOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "se requiere application/json", http.StatusUnsupportedMediaType)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("respuesta: %v", err)
	}
}

func fail(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "error interno", http.StatusInternalServerError)
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	var st scan.Status
	if s.Scanner != nil {
		st = s.Scanner.Status()
	}
	counts, err := s.Store.Counts()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st, "counts": counts})
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	if s.Scanner == nil {
		http.Error(w, "modo consulta: no se puede escanear", http.StatusConflict)
		return
	}
	if s.Scanner.Status().Running {
		http.Error(w, scan.ErrBusy.Error(), http.StatusConflict)
		return
	}
	go func() {
		if err := s.Scanner.Run(context.Background()); err != nil && !errors.Is(err, scan.ErrBusy) {
			log.Printf("escaneo: %v", err)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]bool{"started": true})
}

func (s *Server) albums(w http.ResponseWriter, r *http.Request) {
	as, err := s.Store.Albums(r.URL.Query().Get("folder"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, as)
}

func (s *Server) album(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	a, ok, err := s.Store.Album(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "álbum no encontrado", http.StatusNotFound)
		return
	}
	writeJSON(w, a)
}

func (s *Server) folder(w http.ResponseWriter, r *http.Request) {
	f, ok, err := s.Store.Folder(r.URL.Query().Get("path"))
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "carpeta no encontrada", http.StatusNotFound)
		return
	}
	writeJSON(w, f)
}

func (s *Server) collections(w http.ResponseWriter, r *http.Request) {
	cs, err := s.Store.Collections()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, cs)
}
```

`writeJSON` after `WriteHeader(202)` sets a header too late; that is harmless (the content type is just not sent for that one response).

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/server/ -v -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server
git commit -m "feat: localhost JSON API for status, albums, folders and collections"
```

---

### Task 20: Command, build script and README

**Files:**
- Create: `cmd/tordo/main.go`, `cmd/tordo/main_test.go`, `scripts/build.sh`, `README.md`

**Interfaces:**
- Consumes: `appdir`, `config`, `store`, `scan`, `server`.
- Produces: the `tordo` binary: flags `-dir` (or `TORDO_DIR`) and `-port` (0 = a free one). On start: resolve the app folder; load `config.json` (create it with the sibling folders as roots when missing and the folder is writable); open `tordo.db` (browse-only mode when the folder is not writable); start a scan in the background; serve the API on `127.0.0.1`.

- [ ] **Step 1: Write the failing test**

`cmd/tordo/main_test.go`:

```go
package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"tordo/internal/config"
)

func TestSetupCreatesConfigAndScans(t *testing.T) {
	parent := t.TempDir()
	app := filepath.Join(parent, "tordo")
	for _, d := range []string{app, filepath.Join(parent, "musica")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a, err := setup(app)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	cfg, created, err := config.Load(app)
	if err != nil || created {
		t.Fatalf("config.json not saved: %v, created %v", err, created)
	}
	if !reflect.DeepEqual(cfg.Roots, []string{"../musica"}) {
		t.Fatalf("roots = %q", cfg.Roots)
	}
	if a.scanner == nil || a.server.ReadOnly {
		t.Fatal("expected a writable setup with a scanner")
	}
	if err := a.scanner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(app, dbName)); err != nil {
		t.Fatalf("catalog file: %v", err)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./cmd/tordo/`
Expected: FAIL (`undefined: setup`).

- [ ] **Step 3: Implement**

`cmd/tordo/main.go`:

```go
// Command tordo catalogs a music library and serves the catalog on localhost.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"tordo/internal/appdir"
	"tordo/internal/config"
	"tordo/internal/scan"
	"tordo/internal/server"
	"tordo/internal/store"
)

const dbName = "tordo.db"

func main() {
	dir := flag.String("dir", os.Getenv("TORDO_DIR"), "carpeta de la app (por defecto, la del ejecutable)")
	port := flag.Int("port", 0, "puerto HTTP (0 = uno libre)")
	flag.Parse()
	if err := run(*dir, *port); err != nil {
		log.Fatal(err)
	}
}

type app struct {
	server  *server.Server
	scanner *scan.Scanner // nil in browse-only mode
	close   func() error
}

func run(dirOverride string, port int) error {
	a, err := setup(dirOverride)
	if err != nil {
		return err
	}
	defer a.close()
	if a.scanner != nil {
		go func() {
			if err := a.scanner.Run(context.Background()); err != nil {
				log.Printf("escaneo: %v", err)
			}
		}()
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	log.Printf("Tordo en http://%s/api/status (Ctrl+C para salir)", ln.Addr())
	return http.Serve(ln, a.server.Handler())
}

// setup resolves the app folder, loads (or creates) config.json and opens the
// catalog. A folder that is not writable gives browse-only mode: no scanner.
func setup(dirOverride string) (*app, error) {
	appDir, err := appdir.Resolve(dirOverride)
	if err != nil {
		return nil, err
	}
	readOnly := !appdir.Writable(appDir)
	cfg, created, err := config.Load(appDir)
	if err != nil {
		return nil, err
	}
	if created && !readOnly {
		if err := config.Save(appDir, cfg); err != nil {
			return nil, err
		}
		log.Printf("se creó %s con las carpetas %q; editalo para cambiarlas", config.FileName, cfg.Roots)
	}
	st, err := openStore(appDir, readOnly)
	if err != nil {
		return nil, err
	}
	a := &app{close: st.Close}
	if readOnly {
		log.Print("la carpeta de la app no es escribible: modo consulta")
	} else {
		roots := make([]string, len(cfg.Roots))
		for i, r := range cfg.Roots {
			roots[i] = path.Clean(r)
		}
		a.scanner = &scan.Scanner{AppDir: appDir, Roots: roots, Ignore: cfg.Ignore, Store: st}
	}
	a.server = &server.Server{Store: st, Scanner: a.scanner, ReadOnly: readOnly}
	return a, nil
}

// openStore opens the catalog. Browse-only mode with no catalog file yet
// falls back to an empty in-memory one.
func openStore(appDir string, readOnly bool) (*store.Store, error) {
	p := filepath.Join(appDir, dbName)
	if readOnly {
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			return store.OpenMemory()
		}
	}
	return store.Open(p, readOnly)
}
```

`scripts/build.sh`:

```bash
#!/usr/bin/env bash
# Cross-compiles Tordo for Windows, macOS (universal) and Linux (amd64,
# arm64) into dist/tordo/.
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist/tordo
mkdir -p "$out"
export CGO_ENABLED=0
build() { GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "-s -w" -o "$out/$3" ./cmd/tordo; }
build windows amd64 tordo-windows.exe
build linux   amd64 tordo-linux-amd64
build linux   arm64 tordo-linux-arm64
build darwin  amd64 tordo-macos-amd64
build darwin  arm64 tordo-macos-arm64
go run github.com/randall77/makefat@latest "$out/tordo-macos" "$out/tordo-macos-amd64" "$out/tordo-macos-arm64"
rm "$out/tordo-macos-amd64" "$out/tordo-macos-arm64"
ls -la "$out"
```

`README.md`:

````markdown
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
````

- [ ] **Step 4: Run everything**

Run: `go test ./... && go vet ./... && CGO_ENABLED=0 go build ./... && bash scripts/build.sh`
Expected: PASS; `dist/tordo/` holds `tordo-windows.exe`, `tordo-linux-amd64`, `tordo-linux-arm64`, `tordo-macos`.

- [ ] **Step 5: Commit**

```bash
chmod +x scripts/build.sh
git add cmd scripts README.md
git commit -m "feat: tordo command, cross-compile script and README"
```

---
### Task 21: Check against the real collection

The unit tests use hand-built files; this task runs M1 against `/Users/martin/Music` (52 GB, ~6,700 audio files, read-only) and fixes what the real data reveals. **Never write inside `/Users/martin/Music`.**

**Files:**
- Create: `docs/superpowers/notes/2026-10-01-m1-collection-check.md` (findings, for M2 planning)
- Modify: whichever package a finding points to, always test-first (add the real failing name or file to that package's tests).

- [ ] **Step 1: Native readers vs ffprobe**

Run: `TORDO_CORPUS=/Users/martin/Music go test ./internal/audiofile -run Corpus -v -timeout 0 2>&1 | tee /tmp/tordo-corpus.log | tail -40`
Expected: PASS (native readers reject under 1% of the files ffprobe reads). When this plan was written, the plan's code was compiled and run against this same collection: **6,697 files, 0 rejected**, ~310 differences, almost all accents that ffprobe garbles and tordo decodes correctly, plus ~6 MP3s whose duration both tools only estimate (VBR without header, a corrupt file, one edited after encoding). Then read the `FALLA` and `DIFIERE` lines:
- `FALLA` lines are reader bugs or truly odd files. For each distinct cause, reproduce it with a builder in the format's `_test.go`, watch it fail, fix the reader, and bump `audiofile.Version` to 2 if behavior changed for files already read.
- `DIFIERE` lines with mojibake on one side (e.g. `FÃ¡bio` vs `Fábio`) are expected: tordo decodes UTF-8 declared as Latin-1, ffprobe does not. Duration differences on MP3s without a Xing header are expected too (ffprobe estimates). Anything else is a bug: fix it test-first.

- [ ] **Step 2: Run tordo on the collection**

```bash
mkdir -p .run
printf '{\n  "roots": ["../../../Music"],\n  "ignore": []\n}\n' > .run/config.json
touch /tmp/tordo-before-scan
go run ./cmd/tordo -dir .run -port 4747
```

`.run` is `/Users/martin/x-projects/tordo/.run`, so `../../../Music` is `/Users/martin/Music`. In another terminal, poll until `running` is false, noting how long the first scan takes:

```bash
curl -s http://127.0.0.1:4747/api/status
```

- [ ] **Step 3: Check the results**

Expected values (from the collection survey in the spec, §1):
- `counts.audio` ≈ 6,700 (mp3 5,775, opus 411, flac 372, m4a 84, wma 54, minus anything inside ignored folders); `counts.unreadable` under 1% of that.
- `Latinoamérica 600` is a collection with its albums in `[LA-…]` order:
  `curl -s http://127.0.0.1:4747/api/collections | python3 -m json.tool | grep -E '"(name|title|position)"' | head -40`
- `Soulseek/complete/morcal/Noel Rosa`: the `Cd 01`/`Cd 02` folders of *Noel pela primeira vez* form **one** album with 2 discs:
  `curl -s 'http://127.0.0.1:4747/api/albums?folder=../../../Music/Soulseek/complete/morcal/Noel%20Rosa' | python3 -m json.tool | grep -E '"(title|discs|tracks)"'`
- `Brasil/VA - História da MPB` and `Uruguay/Selección de murgas` are compilations (`"artist": "Varios artistas"`).
- `Criosho [K-NtwAQ8Oqk].opus` (root) and `youtube/` are loose tracks: `curl -s 'http://127.0.0.1:4747/api/folders?path=../../../Music'`.
- No files from `Ableton/`, `djay/`, `Audio Music Apps/` or `Music/Music Library.musiclibrary` in the catalog: `sqlite3 .run/tordo.db "SELECT COUNT(*) FROM files WHERE path LIKE '%/Ableton/%' OR path LIKE '%/djay/%' OR path LIKE '%/Audio Music Apps/%' OR path LIKE '%.musiclibrary/%'"` → `0`.
- Spot-check 10 random albums for wrong titles, artists or split albums:
  `sqlite3 .run/tordo.db "SELECT title, artist, year, tracks, dir FROM albums ORDER BY random() LIMIT 10"`
  and list the folders that produced more than one album (a sign of a bad split):
  `sqlite3 .run/tordo.db "SELECT dir, COUNT(*) n FROM albums GROUP BY dir HAVING n > 1 ORDER BY n DESC LIMIT 20"`
- A second scan reads nothing: `curl -s -X POST -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:4747/api/scan`, then `/api/status` shows `"toRead":0` and finishes in seconds.
- The collection was not modified: `find /Users/martin/Music -newer /tmp/tordo-before-scan -print | head` prints nothing.

For each wrong result, find the cause in `nameparse` or `grouping`, add the real folder or file name to that package's tests, watch it fail, fix, and re-run Steps 2–3 (delete `.run/tordo.db` first if grouping rules changed; reads are cached per file, so only `audiofile.Version` bumps force re-reading).

- [ ] **Step 4: Write down the findings**

`docs/superpowers/notes/2026-10-01-m1-collection-check.md`, in Spanish like the spec: first-scan and re-scan times, the counts, the unreadable files by cause, what was fixed, and the problems left for M2 (e.g. albums whose title only comes from the folder name, genres to normalize, compilations detected or missed). Keep it short: numbers and bullet points.

- [ ] **Step 5: Final run and commit**

Run: `go test ./... && go vet ./...`
Expected: PASS.

```bash
git add -A
git status   # make sure .run/ and dist/ are not staged
git commit -m "test: check M1 against the real collection and record findings"
```
