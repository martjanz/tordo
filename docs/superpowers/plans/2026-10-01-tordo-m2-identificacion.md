# Tordo M2 (Identificación) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Identify every album of the M1 catalog on MusicBrainz (confidence score, automatic above a threshold, a *Sin identificar* queue with candidates below it), enrich identified albums (MusicBrainz title and track titles, original year, artist country, Discogs genres and styles, Cover Art Archive covers), normalize raw tag genres to a genre/style taxonomy with a *Géneros por mapear* queue, and let the user correct identifications with corrections stored per track fingerprint.

**Architecture:** A background `identify.Runner` (adapted from Cinexplorer's) runs after every scan and on demand. It works in phases that each save as they go: **match** (one album query → a stored `matches` row, keyed by the query itself, so moving a folder never re-queries), **enrich** (release, artist, Discogs genres), **covers** (Cover Art Archive into `cache/covers/`), and **apply**: a pure, local function that recomputes every album's identity from what is stored (the user's corrections first, then the match) and writes it onto the derived `albums`/`tracks` rows and a derived `album_genres` table. The scan rebuilds the derived tables; apply never needs the network, so it can always run. HTTP clients share Cinexplorer's `httpx` (rate limiter + retries); tests use fake APIs and `httptest` servers with recorded-shape fixtures, never the network.

**Tech Stack:** Go 1.27, `modernc.org/sqlite` (pure Go), standard library only otherwise. MusicBrainz web service v2 (JSON), Cover Art Archive, Discogs API.

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish). M2 covers §5 steps 6–7, §5.1 (network errors), the *Corrección* and *Género / Estilo* entities of §4, and the *Sin identificar* and *Géneros por mapear* queues of §8 (as API; the UI is M3). Also read `docs/superpowers/notes/2026-10-01-m1-collection-check.md`: its "Pendiente para M2" list is mapped to tasks below.

**Verified:** when this plan was written, all of its code was applied to a copy of the repo at commit `7deb7b0` (end of M1): `go vet ./...` clean, `go test ./...` and `go test -race` on `identify`, `server`, `scan` and `cmd/tordo` pass, with no network. Snippets that edit existing files assume that state.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new dependencies** (only `modernc.org/sqlite v1.59.0` and its indirect deps). No cgo.
- **Never write to the music roots.** The only writes go to the app folder: `tordo.db`, `config.json`, `cache/` (covers go to `cache/covers/<release-mbid>.jpg`).
- Catalog paths are relative to the app folder, `/`-separated; cache paths stored in the catalog are relative to the app folder too (`cache/covers/…`).
- SQLite journal `DELETE`, `foreign_keys(1)`, `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open.
- MusicBrainz: **at most 1 request per second** (`httpx.Limiter{Rate: 1, Burst: 1}`) and the User-Agent `Tordo/0.2 ( https://github.com/martjanz/tordo )` on every request (MusicBrainz, Cover Art Archive and Discogs).
- Discogs: optional token from `config.json` (`"discogsToken"`). Without a token, release/master lookups still work (25 req/min); search needs the token.
- Network errors never stop the app: offline → retry in 5 min, doubling up to 1 h, and after every scan (spec §5.1).
- **Automatic identification never overrides a correction.** Corrections are stored per track fingerprint.
- Server listens on `127.0.0.1` only (M1's `localOnly`); write endpoints require `Content-Type: application/json`.
- User-facing strings (logs, errors, JSON error text) in Spanish; identifiers and comments in English.
- `go test ./...` passes **without network**; `go vet ./...` clean. Tests use fakes, `httptest` servers and fixtures in `testdata/`.
- Commit messages: conventional style (`feat:`, `test:`…), **no `Co-Authored-By:` trailer**.

## M1 findings ("Pendiente para M2") → where they are handled

| Finding | Handled in |
|---|---|
| Junk tags: `WWW.TODOMUSICACD.8K.COM`, `(LoMusical.Com)`, artists `M031CAS_e_01_01` | Task 2 (`nameparse.CleanTag`), Task 3 (grouping ignores them, falls back to the folder; unknown artist from the parent artist folder) |
| Crossed tags in *Raras Partituras* (album `Epsa Music`) | Task 10: every album is also searched by its folder name |
| Ugly titles (`FITO PAEZ - 2004 - mi vida con ellas 1`), truncated ID3v1 titles, wrong year | Tasks 10–12: folder-name query variant; identified albums show MusicBrainz's album and track titles and the original year |
| *História da MPB* not a compilation | Task 12: an identified album credited to Various Artists becomes a compilation (if MusicBrainz credits each volume to the composer, the check in Task 16 records it) |
| Catch-all folders (`baulera`, `voces`, `infantiles`) | Task 3 marks folder selections; Task 12: a selection MusicBrainz identifies is a real album, one it does not is status `selection` and stays out of *Sin identificar*. **Deferred:** splitting a selection into the real albums inside it (needs per-track identification; after M5) |
| Unicode NFD from macOS | Task 2 (`nameparse.Compose`): names from folders and files are composed to NFC for display and queries; paths stay untouched. Accent-insensitive search is M3 |
| 207 raw genres | Task 7 (taxonomy + aliases), Task 6/13 (*Géneros por mapear* queue, user aliases) |
| 61 albums without a cover | Task 13: Cover Art Archive for identified albums with no folder image and no embedded picture |

**Deferred on purpose** (not in the M2 row of spec §10, or needing the UI): artist photos (M3 artist page), serving *embedded* pictures (M3; the cover endpoint here serves folder images and cached covers), album-level manual genre edits (`source = manual`, M3), changing the Discogs token without a restart (the hot-swappable `engine` arrives with Ajustes in M5), loudness (M4).

## File Structure

```
internal/httpx/                   rate limiter + GET with retries (copied from Cinexplorer), UserAgent
internal/nameparse/junk.go        CleanTag: download-site and file-code tags
internal/nameparse/compose.go     Compose: NFD → NFC for Latin letters
internal/grouping/                ignores junk tags; Album.Selection; artist from the parent artist folder
internal/store/
    schema.sql                    v2: new columns and tables
    store.go                      PRAGMA user_version migration
    identify.go                   identification targets, matches, corrections
    releases.go                   releases and artists (JSON documents)
    apply.go                      ApplyIdentities, review queue, genre aliases, unmapped genres
    catalog.go                    views gain identification fields and genre links
internal/genres/                  taxonomy (Discogs-based), Key, Normalizer, FromDiscogs
internal/musicbrainz/             search, release, release group, artist, Cover Art Archive
internal/discogs/                 release, master, search, ParseURL
internal/identify/
    query.go                      Query, Variant, BuildQuery, Key
    score.go                      similarity and the confidence score
    match.go                      API, Identify
    resolve.go                    resolve: stored data → an album's identity (pure)
    runner.go                     Runner: match, enrich, covers, apply; corrections; offline retry
internal/config/                  + discogsToken
internal/scan/                    + OnDone hook
internal/server/                  identification, review, genres and cover endpoints
cmd/tordo/                        wiring
```

---
### Task 1: httpx (copied from Cinexplorer)

**Files:**
- Create: `internal/httpx/httpx.go`, `internal/httpx/httpx_test.go` (copied), `internal/httpx/useragent_test.go`

**Interfaces:**
- Produces: `httpx.Client{HTTP *http.Client; Limiter *Limiter; Attempts int; Backoff time.Duration; Header http.Header}`, `(*Client).Get(ctx, url string) ([]byte, error)`, `httpx.Limiter{Rate, Burst float64}`, `httpx.ErrOffline`, `httpx.ErrTooLarge`, `*httpx.StatusError{Code int; URL string}`, `httpx.UserAgent`.

`Get` retries 429, 5xx and network errors with exponential backoff (honouring `Retry-After`) and wraps the final failure in `ErrOffline`; other non-2xx statuses return a `*StatusError` at once. The default `http.Client` (30 s timeout) follows redirects, which the Cover Art Archive needs.

- [ ] **Step 1: Copy the package**

```bash
mkdir -p internal/httpx
cp /Users/martin/x-projects/cinexplorer/internal/httpx/httpx.go internal/httpx/
cp /Users/martin/x-projects/cinexplorer/internal/httpx/httpx_test.go internal/httpx/
grep -n cinexplorer internal/httpx/*.go   # expected: nothing (the package has no internal imports)
```

- [ ] **Step 2: Write the failing User-Agent test**

`internal/httpx/useragent_test.go`:

```go
package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserAgentNamesTheProject(t *testing.T) {
	if !strings.HasPrefix(UserAgent, "Tordo/") || !strings.Contains(UserAgent, "https://github.com/martjanz/tordo") {
		t.Fatalf("UserAgent = %q", UserAgent)
	}
}

func TestGetSendsUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer srv.Close()
	c := fastClient()
	c.Header = http.Header{"User-Agent": {UserAgent}}
	if _, err := c.Get(context.Background(), srv.URL); err != nil || got != UserAgent {
		t.Fatalf("err %v, User-Agent %q", err, got)
	}
}
```

- [ ] **Step 3: Run it to see it fail**

Run: `go test ./internal/httpx/`
Expected: FAIL (`undefined: UserAgent`).

- [ ] **Step 4: Add the constant**

In `internal/httpx/httpx.go`, after the `ErrTooLarge` declaration:

```go
// UserAgent identifies tordo to the services it calls. MusicBrainz asks
// every client for a name, a version and a contact URL.
const UserAgent = "Tordo/0.2 ( https://github.com/martjanz/tordo )"
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/httpx/ -v`
Expected: PASS (the copied tests plus the two new ones).

- [ ] **Step 6: Commit**

```bash
git add internal/httpx
git commit -m "feat: httpx client with rate limiting and retries, copied from Cinexplorer"
```

---
### Task 2: Junk tags and NFC names in nameparse

**Files:**
- Create: `internal/nameparse/junk.go`, `internal/nameparse/junk_test.go`, `internal/nameparse/compose.go`, `internal/nameparse/compose_test.go`
- Modify: `internal/nameparse/folder.go` (`ParseFolder`), `internal/nameparse/file.go` (`ParseFile`)

**Interfaces:**
- Produces: `nameparse.CleanTag(s string) string`, `nameparse.Compose(s string) string`. `ParseFolder` and `ParseFile` now return composed (NFC) text.

macOS gives file names in NFD (`e` + U+0301). `Norm` already ignores combining marks, so album keys do not change; `Compose` is for what is shown and sent to MusicBrainz. There is no `golang.org/x/text` (no new dependencies): a table of the Latin letters this collection uses is enough, anything else is left as is.

- [ ] **Step 1: Write the failing tests**

`internal/nameparse/junk_test.go`:

```go
package nameparse

import "testing"

func TestCleanTag(t *testing.T) {
	cases := map[string]string{
		"WWW.TODOMUSICACD.8K.COM":                     "",
		"www.lomusical.com":                           "",
		"http://musica.example.net":                   "",
		"Tercer Mundo (LoMusical.Com)":                "Tercer Mundo",
		"Naturaleza Sangre [www.todomusicacd.8k.com]": "Naturaleza Sangre",
		"M031CAS_e_01_01":                             "",
		"Tercer Mundo":                                "Tercer Mundo",
		"P.1":                                         "P.1",
		"U2":                                          "U2",
		"Blink-182":                                   "Blink-182",
		"Mr. Bungle":                                  "Mr. Bungle",
		"La Ley es Tela de Araña y Melo":              "La Ley es Tela de Araña y Melo",
		"  Signos  ":                                  "Signos",
		"":                                            "",
	}
	for in, want := range cases {
		if got := CleanTag(in); got != want {
			t.Errorf("CleanTag(%q) = %q, want %q", in, got, want)
		}
	}
}
```

`internal/nameparse/compose_test.go`:

```go
package nameparse

import "testing"

func TestCompose(t *testing.T) {
	cases := map[string]string{
		"Império":    "Império",
		"Fito Páez":  "Fito Páez",
		"Coração":   "Coração",
		"Müller":     "Müller",
		"Niño":       "Niño",
		"Páez":        "Páez",
		"plain ascii": "plain ascii",
		"x́":          "x́", // no precomposed letter: left alone
	}
	for in, want := range cases {
		if got := Compose(in); got != want {
			t.Errorf("Compose(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsersCompose(t *testing.T) {
	if f := ParseFolder("Fito Páez - (1990) Tercer Mundo"); f.Artist != "Fito Páez" {
		t.Errorf("ParseFolder artist = %q", f.Artist)
	}
	if f := ParseFile("05 - Fue amor de verano de Páez.mp3", ""); f.Title != "Fue amor de verano de Páez" {
		t.Errorf("ParseFile title = %q", f.Title)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/nameparse/`
Expected: FAIL (`undefined: CleanTag`, `undefined: Compose`).

- [ ] **Step 3: Implement**

`internal/nameparse/junk.go`:

```go
package nameparse

import (
	"regexp"
	"strings"
)

var (
	reSiteNote = regexp.MustCompile(`(?i)\s*[\[(][^)\]]*(?:www\.|https?://|\.(?:com|net|org|info|biz)\b)[^)\]]*[\])]`)
	reSite     = regexp.MustCompile(`(?i)(?:\bwww\.|https?://|\.(?:com|net|org|info|biz)\b)`)
	reFileCode = regexp.MustCompile(`^[A-Za-z]{0,4}\d{2,}[A-Za-z0-9]*(?:_[A-Za-z0-9]+)+$`)
)

// CleanTag drops what download sites add to album and artist tags: a
// bracketed site name ("Tercer Mundo (LoMusical.Com)" → "Tercer Mundo"). A
// value that is only a site or a file code ("WWW.TODOMUSICACD.8K.COM",
// "M031CAS_e_01_01") comes back empty, so the folder name is used instead.
// Not meant for track titles.
func CleanTag(s string) string {
	s = strings.TrimSpace(reSiteNote.ReplaceAllString(s, ""))
	if reSite.MatchString(s) || reFileCode.MatchString(s) {
		return ""
	}
	return s
}
```

`internal/nameparse/compose.go`:

```go
package nameparse

import "strings"

// Compose turns decomposed Latin letters ("e" + U+0301, as macOS writes file
// names) into precomposed ones ("é"), so names from folders read like names
// from tags. Other text is left as is.
func Compose(s string) string {
	if !strings.ContainsFunc(s, isMark) {
		return s
	}
	rs := []rune(s)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) {
			if c, ok := composed[[2]rune{rs[i], rs[i+1]}]; ok {
				b.WriteRune(c)
				i++
				continue
			}
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

func isMark(r rune) bool { return r >= 0x300 && r <= 0x36F }

// composed maps a base letter and a combining mark to the precomposed letter.
var composed = func() map[[2]rune]rune {
	m := map[[2]rune]rune{}
	add := func(mark rune, pairs string) { // base, composed, base, composed…
		rs := []rune(pairs)
		for i := 0; i+1 < len(rs); i += 2 {
			m[[2]rune{rs[i], mark}] = rs[i+1]
		}
	}
	add('̀', "aàeèiìoòuùAÀEÈIÌOÒUÙ")
	add('́', "aáeéiíoóuúyýAÁEÉIÍOÓUÚYÝcćCĆnńNŃsśSŚzźZŹ")
	add('̂', "aâeêiîoôuûAÂEÊIÎOÔUÛ")
	add('̃', "aãoõnñAÃOÕNÑ")
	add('̈', "aäeëiïoöuüyÿAÄEËIÏOÖUÜ")
	add('̧', "cçCÇ")
	add('̊', "aåAÅ")
	return m
}()
```

In `internal/nameparse/folder.go`, make the first statement of `ParseFolder`:

```go
	name = Compose(name)
```

In `internal/nameparse/file.go`, make the first statement of `ParseFile`:

```go
	name = Compose(name)
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/nameparse/ ./internal/grouping/`
Expected: PASS (grouping's tests still pass: keys use `Norm`, which already ignored the marks).

- [ ] **Step 5: Commit**

```bash
git add internal/nameparse
git commit -m "feat: drop junk album and artist tags; compose NFD names to NFC"
```

---
### Task 3: Grouping — junk tags, selections, artist from the parent folder

**Files:**
- Modify: `internal/grouping/grouping.go` (`Album`), `internal/grouping/albums.go`
- Test: `internal/grouping/albums_test.go`

**Interfaces:**
- Consumes: `nameparse.CleanTag`, `nameparse.Compose` (Task 2).
- Produces: `grouping.Album.Selection bool` — a folder of loose picks made into one album by `settleFolders`.

Three M1 findings: junk album/artist tags must count as missing (the folder name then names the album); the selection made by `settleFolders` must be marked so identification can treat it apart; an album with no artist at all (every artist tag was junk) takes the name of its parent folder **when that name is an artist the catalog knows** (`Uruguay/Alfredo Zitarrosa/Casi en privado`). The "known artist" guard keeps catch-all parents (`infantiles`, `Uruguay`) from becoming artists.

- [ ] **Step 1: Write the failing tests**

Append to `internal/grouping/albums_test.go`:

```go
func TestJunkTagsFallBackToFolder(t *testing.T) {
	d := "../musica/Fito Páez/Fito Páez - (2003) Naturaleza Sangre"
	r := Build([]Track{
		tr(1, d+"/01.mp3", audiofile.Tags{Title: "Naturaleza sangre", Artist: "Fito Páez", Album: "WWW.TODOMUSICACD.8K.COM", Track: 1}),
		tr(2, d+"/02.mp3", audiofile.Tags{Title: "Yo sé que no es fácil", Artist: "Fito Páez", Album: "WWW.TODOMUSICACD.8K.COM", Track: 2}),
	}, nil, roots)
	if len(r.Albums) != 1 {
		t.Fatalf("albums = %+v", r.Albums)
	}
	a := albumByTitle(t, r, "Naturaleza Sangre")
	if a.Artist != "Fito Páez" || a.Year != 2003 || a.Tracks != 2 || a.Selection {
		t.Fatalf("album = %+v", a)
	}
}

func TestUnknownArtistComesFromParentArtistFolder(t *testing.T) {
	z := "../musica/Uruguay/Alfredo Zitarrosa"
	casi := z + "/Casi en privado"
	canta := z + "/Alfredo Zitarrosa - (1966) Canta Zitarrosa"
	r := Build([]Track{
		tr(1, casi+"/01.mp3", audiofile.Tags{Title: "Uno", Artist: "M031CAS_e_01_01", Album: "P.1"}),
		tr(2, casi+"/02.mp3", audiofile.Tags{Title: "Dos", Artist: "M031CAS_e_01_02", Album: "P.2"}),
		tr(3, casi+"/03.mp3", audiofile.Tags{Title: "Tres", Artist: "M031CAS_e_01_03", Album: "P.3"}),
		tr(4, canta+"/01.mp3", audiofile.Tags{Title: "Guitarra negra", Artist: "Alfredo Zitarrosa", Album: "Canta Zitarrosa", Track: 1}),
		tr(5, canta+"/02.mp3", audiofile.Tags{Title: "Recordándote", Artist: "Alfredo Zitarrosa", Album: "Canta Zitarrosa", Track: 2}),
	}, nil, roots)
	a := albumByTitle(t, r, "Casi en privado")
	if a.Artist != "Alfredo Zitarrosa" || a.Compilation || !a.Selection || a.Tracks != 3 {
		t.Fatalf("album = %+v", a)
	}
	if b := albumByTitle(t, r, "Canta Zitarrosa"); b.Selection {
		t.Fatalf("a tagged album is not a selection: %+v", b)
	}
}

func TestParentFolderMustBeAKnownArtist(t *testing.T) {
	d := "../musica/infantiles/Canciones para mirar"
	r := Build([]Track{
		tr(1, d+"/01.mp3", audiofile.Tags{Title: "Uno", Album: "Canciones para mirar"}),
		tr(2, d+"/02.mp3", audiofile.Tags{Title: "Dos", Album: "Canciones para mirar"}),
	}, nil, roots)
	if a := albumByTitle(t, r, "Canciones para mirar"); a.Artist != "" {
		t.Fatalf("artist = %q, want none", a.Artist)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/grouping/`
Expected: FAIL (`a.Selection undefined` — compile error).

- [ ] **Step 3: Implement**

In `internal/grouping/grouping.go`, add to `Album` after `Genres`:

```go
	Selection   bool // a folder of loose picks grouped as one album (settleFolders)
```

In `internal/grouping/albums.go`:

1. Add a field to `member`:

```go
	selection bool             // part of a folder selection (settleFolders)
```

2. In `buildAlbums`, right after `tg := t.Tags`:

```go
		// Download sites' names and file codes are not album or artist names.
		tg.Album, tg.AlbumArtist, tg.Artist = nameparse.CleanTag(tg.Album), nameparse.CleanTag(tg.AlbumArtist), nameparse.CleanTag(tg.Artist)
```

3. In `settleFolders`, in the selection case, replace the line `m.album, m.tags.AlbumArtist, m.out.AlbumKey = title, "", key` with:

```go
				m.album, m.tags.AlbumArtist, m.out.AlbumKey, m.selection = title, "", key, true
```

4. In `makeAlbum`, set the flag where the album is created:

```go
	a := Album{Key: key, Dir: ms[0].albumDir, Position: ms[0].folder.Position, Selection: ms[0].selection}
```

5. At the end of `buildAlbums`, just before `return res`:

```go
	artists := map[string]bool{}
	for _, t := range res.Tracks {
		artists[nameparse.Norm(t.Artist)] = true
	}
	for _, a := range res.Albums {
		artists[nameparse.Norm(a.Artist)] = true
	}
	delete(artists, "")
	for i := range res.Albums {
		if res.Albums[i].Artist == "" {
			res.Albums[i].Artist = parentArtist(res.Albums[i].Dir, isRoot, artists)
		}
	}
```

6. Add the helper at the end of the file:

```go
// parentArtist is the name of the album folder's parent when it is an
// artist's folder: not a root, and named like an artist the catalog knows
// ("Uruguay/Alfredo Zitarrosa/Casi en privado").
func parentArtist(dir string, isRoot map[string]bool, artists map[string]bool) string {
	p := path.Dir(dir)
	if isRoot[dir] || isRoot[p] || !strings.Contains(p, "/") {
		return ""
	}
	name := nameparse.Compose(path.Base(p))
	if !artists[nameparse.Norm(name)] {
		return ""
	}
	return name
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/grouping/ ./internal/store/ ./internal/scan/`
Expected: PASS. (`TestAlbumFromTags` compares a whole `Album` with `reflect.DeepEqual`; `Selection` is false there, so it still passes.)

- [ ] **Step 5: Commit**

```bash
git add internal/grouping
git commit -m "feat: grouping ignores junk tags, marks selections and finds artists in parent folders"
```

---
### Task 4: Catalog schema v2 and its migration

**Files:**
- Modify: `internal/store/schema.sql` (whole file below), `internal/store/store.go`, `internal/store/catalog.go` (`ReplaceGrouping`)
- Test: `internal/store/migrate_test.go`

**Interfaces:**
- Produces: tables `matches`, `corrections`, `releases`, `artists`, `genre_alias`, `album_genres`, `unmapped_genres`; new columns on `albums` and `tracks`; `store.ErrOldCatalog`. `ReplaceGrouping` stores `Album.Selection`.

The derived tables (`albums`, `tracks`, `folders`) gain columns. They are rebuilt by every scan anyway, so the migration drops them when `PRAGMA user_version` is below 2 and the next scan (which `main` starts at once) fills them again; `files` and `audio` keep everything, so nothing is read again. The new tables only ever get added to. Identification columns on `albums` survive rebuilds because `ReplaceGrouping` upserts by album key and only sets the grouping columns; `tracks` are re-inserted, so their two identification columns are empty until the identification runner applies again (it runs after every scan).

- [ ] **Step 1: Write the failing tests**

`internal/store/migrate_test.go`:

```go
package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"tordo/internal/grouping"
)

// m1Catalog writes a catalog file shaped like M1's (user_version 0).
func m1Catalog(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "tordo.db")
	db, err := sql.Open("sqlite", file)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT NOT NULL UNIQUE, size INTEGER NOT NULL,
			mtime INTEGER NOT NULL, kind TEXT NOT NULL, fingerprint TEXT NOT NULL DEFAULT '', missing INTEGER NOT NULL DEFAULT 0,
			first_seen INTEGER NOT NULL);
		INSERT INTO files (path, size, mtime, kind, first_seen) VALUES ('../musica/a.mp3', 1, 1, 'audio', 1);
		CREATE TABLE albums (id INTEGER PRIMARY KEY, key TEXT NOT NULL UNIQUE, title TEXT NOT NULL);
		INSERT INTO albums (key, title) VALUES ('k', 'Viejo');`); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestOpenMigratesM1Catalog(t *testing.T) {
	file := m1Catalog(t)
	s, err := Open(file, false)
	if err != nil {
		t.Fatal(err)
	}
	var files, albums, version int
	s.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&files)
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM albums WHERE status = ''`).Scan(&albums); err != nil {
		t.Fatalf("albums has no status column: %v", err)
	}
	s.db.QueryRow(`PRAGMA user_version`).Scan(&version)
	if files != 1 || albums != 0 || version != schemaVersion {
		t.Fatalf("files %d albums %d version %d", files, albums, version)
	}
	if _, err := s.db.Exec(`INSERT INTO folders (path, parent, name, tracks, albums, duration_ms, collection)
		VALUES ('../musica/x', '../musica', 'x', 0, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(file, false) // already v2: nothing is dropped
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var folders int
	s.db.QueryRow(`SELECT COUNT(*) FROM folders`).Scan(&folders)
	if folders != 1 {
		t.Fatalf("a v2 catalog lost its folders on open")
	}
}

func TestReadOnlyOpenRejectsOldCatalog(t *testing.T) {
	if _, err := Open(m1Catalog(t), true); !errors.Is(err, ErrOldCatalog) {
		t.Fatalf("err = %v, want ErrOldCatalog", err)
	}
}

func TestSelectionIsStored(t *testing.T) {
	s := open(t)
	if err := s.ReplaceGrouping(grouping.Result{Albums: []grouping.Album{
		{Key: "k", Dir: "../musica/baulera", Title: "baulera", Tracks: 1, Discs: 1, Selection: true},
	}}); err != nil {
		t.Fatal(err)
	}
	var sel bool
	if err := s.db.QueryRow(`SELECT selection FROM albums WHERE key = 'k'`).Scan(&sel); err != nil || !sel {
		t.Fatalf("selection = %v, err %v", sel, err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/store/ -run 'Migrat|ReadOnlyOpen|Selection'`
Expected: FAIL (`undefined: schemaVersion`, `undefined: ErrOldCatalog`).

- [ ] **Step 3: Replace `internal/store/schema.sql`**

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

-- Derived tables, rebuilt from files + audio after every scan. The
-- identification columns of albums are written by ApplyIdentities and kept
-- across rebuilds (albums are upserted by key).
CREATE TABLE IF NOT EXISTS albums (
	id             INTEGER PRIMARY KEY,
	key            TEXT NOT NULL UNIQUE,
	dir            TEXT NOT NULL,
	title          TEXT NOT NULL,
	artist         TEXT NOT NULL,
	year           INTEGER NOT NULL,
	compilation    INTEGER NOT NULL,
	discs          INTEGER NOT NULL,
	tracks         INTEGER NOT NULL,
	duration_ms    INTEGER NOT NULL,
	position       INTEGER NOT NULL,
	cover_path     TEXT NOT NULL,
	cover_file_id  INTEGER NOT NULL,
	genres         TEXT NOT NULL,              -- raw tag values, JSON
	selection      INTEGER NOT NULL DEFAULT 0,
	seen           INTEGER NOT NULL DEFAULT 1,
	match_query    TEXT NOT NULL DEFAULT '',   -- identify.Query.Key()
	status         TEXT NOT NULL DEFAULT '',   -- '' | auto | manual | unmatched | ignored | selection
	release_id     TEXT NOT NULL DEFAULT '',   -- MusicBrainz release
	confidence     REAL NOT NULL DEFAULT 0,
	mb_title       TEXT NOT NULL DEFAULT '',
	mb_artist      TEXT NOT NULL DEFAULT '',
	mb_compilation INTEGER NOT NULL DEFAULT 0,
	original_year  INTEGER NOT NULL DEFAULT 0,
	country        TEXT NOT NULL DEFAULT '',   -- ISO 3166-1 alpha-2
	artist_id      TEXT NOT NULL DEFAULT '',   -- MusicBrainz artist
	cover_cache    TEXT NOT NULL DEFAULT ''    -- relative to the app folder
);
CREATE INDEX IF NOT EXISTS albums_dir ON albums(dir);

CREATE TABLE IF NOT EXISTS tracks (
	file_id      INTEGER PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
	album_id     INTEGER REFERENCES albums(id),
	dir          TEXT NOT NULL,
	title        TEXT NOT NULL,
	artist       TEXT NOT NULL,
	disc         INTEGER NOT NULL,
	track        INTEGER NOT NULL,
	year         INTEGER NOT NULL,
	duration_ms  INTEGER NOT NULL,
	mb_title     TEXT NOT NULL DEFAULT '',
	recording_id TEXT NOT NULL DEFAULT ''
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

-- Derived: an album's genres, from its best source, written by ApplyIdentities.
CREATE TABLE IF NOT EXISTS album_genres (
	album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	genre    TEXT NOT NULL,
	style    TEXT NOT NULL DEFAULT '',
	source   TEXT NOT NULL,                    -- tag | mb | discogs | manual
	PRIMARY KEY (album_id, genre, style)
);

-- Derived: raw tag genres nothing maps (the Géneros por mapear queue).
CREATE TABLE IF NOT EXISTS unmapped_genres (
	raw    TEXT PRIMARY KEY,
	albums INTEGER NOT NULL
);

-- The result of matching one album query against MusicBrainz, keyed by the
-- query: the same album found again (moved, rescanned) is not looked up again.
CREATE TABLE IF NOT EXISTS matches (
	query           TEXT PRIMARY KEY,
	status          TEXT NOT NULL,             -- auto | unmatched
	release_id      TEXT NOT NULL DEFAULT '',
	confidence      REAL NOT NULL DEFAULT 0,
	candidates      TEXT NOT NULL DEFAULT '[]',
	matcher_version INTEGER NOT NULL,
	updated_at      INTEGER NOT NULL
);

-- The user's identifications, per track fingerprint: they survive moving,
-- renaming and re-tagging. Never touched by the automatic identification.
CREATE TABLE IF NOT EXISTS corrections (
	fingerprint TEXT PRIMARY KEY,
	status      TEXT NOT NULL,                 -- manual | ignored
	release_id  TEXT NOT NULL DEFAULT '',
	updated_at  INTEGER NOT NULL
);

-- MusicBrainz releases and artists as fetched and enriched (JSON store.Release, store.Artist).
CREATE TABLE IF NOT EXISTS releases (
	id         TEXT PRIMARY KEY,
	data       TEXT NOT NULL,
	fetched_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS artists (
	id         TEXT PRIMARY KEY,
	data       TEXT NOT NULL,
	fetched_at INTEGER NOT NULL
);

-- The user's genre mappings: raw value (genres.Key) → genre/style.
CREATE TABLE IF NOT EXISTS genre_alias (
	raw   TEXT PRIMARY KEY,
	genre TEXT NOT NULL,                       -- '' = not a genre: ignore it
	style TEXT NOT NULL DEFAULT ''
);
```

- [ ] **Step 4: Migrate on open**

In `internal/store/store.go`, add `"errors"` and `"fmt"` to the imports and replace `openDB` with:

```go
// schemaVersion is kept in PRAGMA user_version. Version 2 (M2) added columns
// to the derived tables: they are dropped and the next scan rebuilds them.
// The other tables are only ever added to.
const schemaVersion = 2

// ErrOldCatalog means a catalog from an older version was opened read-only,
// so it could not be updated.
var ErrOldCatalog = errors.New("el catálogo es de una versión anterior: abrí tordo una vez con la carpeta escribible para actualizarlo")

func openDB(dsn string, writable bool) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if writable {
		err = migrate(db)
	} else {
		err = checkVersion(db)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v < 2 {
		if _, err := db.Exec(`DROP TABLE IF EXISTS tracks; DROP TABLE IF EXISTS album_genres;
			DROP TABLE IF EXISTS albums; DROP TABLE IF EXISTS folders`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion))
	return err
}

func checkVersion(db *sql.DB) error {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v < schemaVersion {
		return ErrOldCatalog
	}
	return nil
}
```

(`Open` and `OpenMemory` keep calling `openDB(dsn, !readOnly)` and `openDB(…, true)`.)

- [ ] **Step 5: Store the selection flag**

In `ReplaceGrouping` (`internal/store/catalog.go`), replace the upsert statement and its call:

```go
	up, err := tx.Prepare(`INSERT INTO albums (key, dir, title, artist, year, compilation, discs, tracks, duration_ms,
		position, cover_path, cover_file_id, genres, selection, seen) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(key) DO UPDATE SET dir = excluded.dir, title = excluded.title, artist = excluded.artist,
		year = excluded.year, compilation = excluded.compilation, discs = excluded.discs, tracks = excluded.tracks,
		duration_ms = excluded.duration_ms, position = excluded.position, cover_path = excluded.cover_path,
		cover_file_id = excluded.cover_file_id, genres = excluded.genres, selection = excluded.selection, seen = 1
		RETURNING id`)
```

```go
		if err := up.QueryRow(a.Key, a.Dir, a.Title, a.Artist, a.Year, a.Compilation, a.Discs, a.Tracks, a.DurationMs,
			a.Position, a.CoverPath, a.CoverFileID, string(g), a.Selection).Scan(&id); err != nil {
			return err
		}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/store/ ./internal/scan/ ./internal/server/ ./cmd/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/store
git commit -m "feat: catalog schema v2 for identification, with migration of M1 catalogs"
```

---
### Task 5: Store — identification targets, matches, corrections, releases, artists

**Files:**
- Create: `internal/store/identify.go`, `internal/store/releases.go`
- Test: `internal/store/identify_test.go`

**Interfaces:**
- Produces:
  - Statuses `store.StatusPending ""`, `StatusAuto "auto"`, `StatusManual "manual"`, `StatusUnmatched "unmatched"`, `StatusIgnored "ignored"`, `StatusSelection "selection"`; `store.CoverNone = "none"`.
  - `store.TargetTrack{FileID int64; Fingerprint, Title string; Disc, Track int; DurationMs int64; MBAlbumID string}`
  - `store.IdentifyTarget{AlbumID int64; Dir, Title, Artist string; Year int; Compilation, Selection, HasCover bool; RawGenres []string; Tracks []TargetTrack}`
  - `(*Store).IdentifyTargets() ([]IdentifyTarget, error)`, `(*Store).AlbumFingerprints(albumID int64) ([]string, error)`
  - `store.Candidate{ReleaseID, Title, Artist string; Year int; Country string; Tracks int; Score float64}` (JSON camelCase)
  - `store.Match{Query, Status, ReleaseID string; Confidence float64; Candidates []Candidate; MatcherVersion int}`, `(*Store).SaveMatches([]Match) error`, `(*Store).Matches() (map[string]Match, error)`
  - `store.Correction{Status, ReleaseID string}` (comparable), `(*Store).Corrections() (map[string]Correction, error)` (by fingerprint), `(*Store).SetCorrection(fps []string, c Correction) error` (empty `Status` removes)
  - `store.ReleaseTrack{Disc, Position int; Title string; LengthMs int64; RecordingID string}`, `store.DiscogsInfo{Done, Searched bool; ID, MasterID int; Genres, Styles []string}`, `store.Release{ID, GroupID, Title, Artist string; ArtistIDs, ArtistNames []string; Various bool; Year, OriginalYear int; Country string; Tracks []ReleaseTrack; MBGenres, DiscogsURLs []string; GroupChecked bool; Discogs DiscogsInfo; Cover string}`, `(*Store).SaveRelease(Release) error`, `(*Store).Releases() (map[string]Release, error)`
  - `store.Artist{ID, Name, SortName, Type, Country, Area, Begin, End string}`, `(*Store).SaveArtist(Artist) error`, `(*Store).Artists() (map[string]Artist, error)`

Releases and artists are stored as JSON documents: tordo never queries inside them, and at the expected scale (~10k albums) loading them all is cheap. Everything here is plain persistence; the decisions live in `identify`.

- [ ] **Step 1: Write the failing tests**

`internal/store/identify_test.go`:

```go
package store

import (
	"reflect"
	"testing"

	"tordo/internal/audiofile"
)

func targetByTitle(t *testing.T, ts []IdentifyTarget, title string) IdentifyTarget {
	t.Helper()
	for _, x := range ts {
		if x.Title == title {
			return x
		}
	}
	t.Fatalf("no target %q in %+v", title, ts)
	return IdentifyTarget{}
}

func TestIdentifyTargets(t *testing.T) {
	s := sample(t)
	ts, err := s.IdentifyTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 3 {
		t.Fatalf("targets = %+v", ts)
	}
	signos := targetByTitle(t, ts, "Signos")
	dir := la + "/[LA-10] Soda Stereo - Signos"
	if signos.Artist != "Soda Stereo" || signos.Year != 1986 || !signos.HasCover || signos.Dir != dir || len(signos.Tracks) != 2 {
		t.Fatalf("signos = %+v", signos)
	}
	if tr := signos.Tracks[0]; tr.Fingerprint != "fp"+dir+"/01.mp3" || tr.Title != "Signos" || tr.Track != 1 || tr.DurationMs != 1000 {
		t.Fatalf("first track = %+v", tr)
	}
	if locura := targetByTitle(t, ts, "Locura"); locura.HasCover {
		t.Fatalf("locura has no cover: %+v", locura)
	}
}

func TestIdentifyTargetsCarryTagReleaseID(t *testing.T) {
	s := catalog(t, map[string]audiofile.Tags{
		"../musica/Virus - Locura/01.mp3": {Title: "Una luna de miel en la mano", Artist: "Virus", Album: "Locura", MBAlbumID: "mbid-locura"},
	})
	ts, _ := s.IdentifyTargets()
	if len(ts) != 1 || ts[0].Tracks[0].MBAlbumID != "mbid-locura" {
		t.Fatalf("targets = %+v", ts)
	}
	fps, err := s.AlbumFingerprints(ts[0].AlbumID)
	if err != nil || !reflect.DeepEqual(fps, []string{"fp../musica/Virus - Locura/01.mp3"}) {
		t.Fatalf("fingerprints = %q, err %v", fps, err)
	}
	if fps, _ := s.AlbumFingerprints(9999); len(fps) != 0 {
		t.Fatalf("unknown album has fingerprints %q", fps)
	}
}

func TestMatchesRoundTrip(t *testing.T) {
	s := open(t)
	m := Match{Query: "signos|soda stereo", Status: StatusAuto, ReleaseID: "r1", Confidence: 0.93, MatcherVersion: 1,
		Candidates: []Candidate{{ReleaseID: "r1", Title: "Signos", Artist: "Soda Stereo", Year: 1986, Country: "AR", Tracks: 8, Score: 0.93}}}
	if err := s.SaveMatches([]Match{m, {Query: "nada", Status: StatusUnmatched, MatcherVersion: 1}}); err != nil {
		t.Fatal(err)
	}
	m.Confidence = 0.95 // saving again updates
	if err := s.SaveMatches([]Match{m}); err != nil {
		t.Fatal(err)
	}
	ms, err := s.Matches()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ms[m.Query], m) || len(ms) != 2 || ms["nada"].Candidates == nil {
		t.Fatalf("matches = %+v", ms)
	}
}

func TestCorrectionsSetAndClear(t *testing.T) {
	s := open(t)
	manual := Correction{Status: StatusManual, ReleaseID: "r1"}
	if err := s.SetCorrection([]string{"fp1", "fp2", ""}, manual); err != nil {
		t.Fatal(err)
	}
	cs, _ := s.Corrections()
	if len(cs) != 2 || cs["fp1"] != manual {
		t.Fatalf("corrections = %+v", cs)
	}
	if err := s.SetCorrection([]string{"fp1"}, Correction{}); err != nil {
		t.Fatal(err)
	}
	cs, _ = s.Corrections()
	if _, ok := cs["fp1"]; ok || len(cs) != 1 {
		t.Fatalf("after clearing fp1: %+v", cs)
	}
}

func TestReleasesAndArtistsRoundTrip(t *testing.T) {
	s := open(t)
	r := Release{ID: "r1", GroupID: "g1", Title: "Signos", Artist: "Soda Stereo", ArtistIDs: []string{"a1"},
		ArtistNames: []string{"Soda Stereo"}, Year: 1986, OriginalYear: 1986, Country: "AR",
		Tracks:   []ReleaseTrack{{Disc: 1, Position: 1, Title: "Táctica y estrategia", LengthMs: 231000, RecordingID: "rec1"}},
		MBGenres: []string{"rock"}, DiscogsURLs: []string{"https://www.discogs.com/release/1234"},
		Discogs: DiscogsInfo{Done: true, ID: 1234, Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}, Cover: CoverNone}
	if err := s.SaveRelease(r); err != nil {
		t.Fatal(err)
	}
	r.Cover = "cache/covers/r1.jpg"
	if err := s.SaveRelease(r); err != nil {
		t.Fatal(err)
	}
	rs, err := s.Releases()
	if err != nil || !reflect.DeepEqual(rs["r1"], r) {
		t.Fatalf("releases = %+v, err %v", rs, err)
	}
	a := Artist{ID: "a1", Name: "Soda Stereo", SortName: "Soda Stereo", Type: "Group", Country: "AR", Area: "Argentina", Begin: "1982", End: "2007"}
	if err := s.SaveArtist(a); err != nil {
		t.Fatal(err)
	}
	as, err := s.Artists()
	if err != nil || as["a1"] != a {
		t.Fatalf("artists = %+v, err %v", as, err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/store/`
Expected: FAIL (`undefined: IdentifyTarget`, …).

- [ ] **Step 3: Implement `internal/store/identify.go`**

```go
package store

import (
	"encoding/json"
	"time"

	"tordo/internal/audiofile"
)

// Album identification statuses.
const (
	StatusPending   = ""          // not looked up yet
	StatusAuto      = "auto"      // matched automatically, above the threshold
	StatusManual    = "manual"    // chosen by the user
	StatusUnmatched = "unmatched" // below the threshold: the Sin identificar queue
	StatusIgnored   = "ignored"   // the user says it is not in MusicBrainz
	StatusSelection = "selection" // a folder selection MusicBrainz does not know
)

// TargetTrack is a track of an album to identify.
type TargetTrack struct {
	FileID      int64
	Fingerprint string
	Title       string
	Disc, Track int
	DurationMs  int64
	MBAlbumID   string // from the tags
}

// IdentifyTarget is an album as the identification sees it.
type IdentifyTarget struct {
	AlbumID     int64
	Dir         string
	Title       string // as grouped: from tags or the folder name
	Artist      string
	Year        int
	Compilation bool
	Selection   bool
	HasCover    bool // a folder image or an embedded picture
	RawGenres   []string
	Tracks      []TargetTrack // in disc/track order
}

// IdentifyTargets returns every album with its tracks.
func (s *Store) IdentifyTargets() ([]IdentifyTarget, error) {
	rows, err := s.db.Query(`SELECT id, dir, title, artist, year, compilation, selection,
		cover_path != '' OR cover_file_id != 0, genres FROM albums ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var out []IdentifyTarget
	index := map[int64]int{}
	for rows.Next() {
		var t IdentifyTarget
		var genres string
		if err := rows.Scan(&t.AlbumID, &t.Dir, &t.Title, &t.Artist, &t.Year, &t.Compilation, &t.Selection, &t.HasCover, &genres); err != nil {
			rows.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(genres), &t.RawGenres); err != nil {
			rows.Close()
			return nil, err
		}
		index[t.AlbumID] = len(out)
		out = append(out, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.db.Query(`SELECT t.album_id, t.file_id, f.fingerprint, t.title, t.disc, t.track, t.duration_ms, a.tags
		FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
		WHERE t.album_id IS NOT NULL ORDER BY t.album_id, t.disc, t.track, f.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var albumID int64
		var tr TargetTrack
		var tags string
		if err := rows.Scan(&albumID, &tr.FileID, &tr.Fingerprint, &tr.Title, &tr.Disc, &tr.Track, &tr.DurationMs, &tags); err != nil {
			return nil, err
		}
		var tg audiofile.Tags
		if err := json.Unmarshal([]byte(tags), &tg); err != nil {
			return nil, err
		}
		tr.MBAlbumID = tg.MBAlbumID
		if i, ok := index[albumID]; ok {
			out[i].Tracks = append(out[i].Tracks, tr)
		}
	}
	return out, rows.Err()
}

// AlbumFingerprints returns the fingerprints of an album's tracks (none for
// an unknown album).
func (s *Store) AlbumFingerprints(albumID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT f.fingerprint FROM tracks t JOIN files f ON f.id = t.file_id
		WHERE t.album_id = ? AND f.fingerprint != '' ORDER BY f.path`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, err
		}
		out = append(out, fp)
	}
	return out, rows.Err()
}

// Candidate is a MusicBrainz release offered for an album.
type Candidate struct {
	ReleaseID string  `json:"releaseId"`
	Title     string  `json:"title"`
	Artist    string  `json:"artist"`
	Year      int     `json:"year"`
	Country   string  `json:"country"`
	Tracks    int     `json:"tracks"`
	Score     float64 `json:"score"`
}

// Match is the result of matching one album query.
type Match struct {
	Query          string
	Status         string // StatusAuto or StatusUnmatched
	ReleaseID      string // set when StatusAuto
	Confidence     float64
	Candidates     []Candidate // best first
	MatcherVersion int
}

func (s *Store) SaveMatches(ms []Match) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(`INSERT INTO matches (query, status, release_id, confidence, candidates, matcher_version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(query) DO UPDATE SET status = excluded.status,
		release_id = excluded.release_id, confidence = excluded.confidence, candidates = excluded.candidates,
		matcher_version = excluded.matcher_version, updated_at = excluded.updated_at`)
	if err != nil {
		return err
	}
	defer st.Close()
	now := time.Now().UnixMilli()
	for _, m := range ms {
		cands := m.Candidates
		if cands == nil {
			cands = []Candidate{}
		}
		b, err := json.Marshal(cands)
		if err != nil {
			return err
		}
		if _, err := st.Exec(m.Query, m.Status, m.ReleaseID, m.Confidence, string(b), m.MatcherVersion, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Matches returns every stored match by query.
func (s *Store) Matches() (map[string]Match, error) {
	rows, err := s.db.Query(`SELECT query, status, release_id, confidence, candidates, matcher_version FROM matches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Match{}
	for rows.Next() {
		var m Match
		var cands string
		if err := rows.Scan(&m.Query, &m.Status, &m.ReleaseID, &m.Confidence, &cands, &m.MatcherVersion); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(cands), &m.Candidates); err != nil {
			return nil, err
		}
		out[m.Query] = m
	}
	return out, rows.Err()
}

// Correction is the user's identification of a track's album.
type Correction struct {
	Status    string // StatusManual or StatusIgnored
	ReleaseID string // set when StatusManual
}

// Corrections returns every correction by fingerprint.
func (s *Store) Corrections() (map[string]Correction, error) {
	rows, err := s.db.Query(`SELECT fingerprint, status, release_id FROM corrections`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Correction{}
	for rows.Next() {
		var fp string
		var c Correction
		if err := rows.Scan(&fp, &c.Status, &c.ReleaseID); err != nil {
			return nil, err
		}
		out[fp] = c
	}
	return out, rows.Err()
}

// SetCorrection stores c for every fingerprint; an empty c.Status removes
// their corrections (back to the automatic identification).
func (s *Store) SetCorrection(fps []string, c Correction) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	for _, fp := range fps {
		if fp == "" {
			continue
		}
		if c.Status == "" {
			_, err = tx.Exec(`DELETE FROM corrections WHERE fingerprint = ?`, fp)
		} else {
			_, err = tx.Exec(`INSERT INTO corrections (fingerprint, status, release_id, updated_at) VALUES (?, ?, ?, ?)
				ON CONFLICT(fingerprint) DO UPDATE SET status = excluded.status, release_id = excluded.release_id,
				updated_at = excluded.updated_at`, fp, c.Status, c.ReleaseID, now)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
```

- [ ] **Step 4: Implement `internal/store/releases.go`**

```go
package store

import (
	"encoding/json"
	"time"
)

// CoverNone marks a release the Cover Art Archive has no front cover for.
const CoverNone = "none"

type ReleaseTrack struct {
	Disc        int    `json:"disc"`
	Position    int    `json:"position"`
	Title       string `json:"title"`
	LengthMs    int64  `json:"lengthMs"`
	RecordingID string `json:"recordingId"`
}

// DiscogsInfo is what Discogs said about a release.
type DiscogsInfo struct {
	Done     bool     `json:"done"`     // MusicBrainz's links to Discogs were followed
	Searched bool     `json:"searched"` // searched on Discogs (needs a token)
	ID       int      `json:"id"`
	MasterID int      `json:"masterId"`
	Genres   []string `json:"genres"`
	Styles   []string `json:"styles"`
}

// Release is a MusicBrainz release as tordo keeps it, with what enrichment
// added.
type Release struct {
	ID           string         `json:"id"`
	GroupID      string         `json:"groupId"`
	Title        string         `json:"title"`
	Artist       string         `json:"artist"` // the artist credit as MusicBrainz writes it
	ArtistIDs    []string       `json:"artistIds"`
	ArtistNames  []string       `json:"artistNames"`
	Various      bool           `json:"various"` // credited to Various Artists
	Year         int            `json:"year"`
	OriginalYear int            `json:"originalYear"` // the release group's first release
	Country      string         `json:"country"`
	Tracks       []ReleaseTrack `json:"tracks"`
	MBGenres     []string       `json:"mbGenres"`
	DiscogsURLs  []string       `json:"discogsUrls"`
	GroupChecked bool           `json:"groupChecked"` // the release group's links were looked up
	Discogs      DiscogsInfo    `json:"discogs"`
	Cover        string         `json:"cover"` // path relative to the app folder, CoverNone, or "" (not looked up)
}

type Artist struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SortName string `json:"sortName"`
	Type     string `json:"type"`
	Country  string `json:"country"` // ISO 3166-1 alpha-2, "" when unknown
	Area     string `json:"area"`
	Begin    string `json:"begin"`
	End      string `json:"end"`
}

func (s *Store) SaveRelease(r Release) error { return s.saveDoc("releases", r.ID, r) }

func (s *Store) Releases() (map[string]Release, error) { return loadDocs[Release](s, "releases") }

func (s *Store) SaveArtist(a Artist) error { return s.saveDoc("artists", a.ID, a) }

func (s *Store) Artists() (map[string]Artist, error) { return loadDocs[Artist](s, "artists") }

// saveDoc and loadDocs keep JSON documents in a table (id, data,
// fetched_at). table is always one of the constants above, never input.
func (s *Store) saveDoc(table, id string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO `+table+` (id, data, fetched_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data, fetched_at = excluded.fetched_at`, id, string(b), time.Now().UnixMilli())
	return err
}

func loadDocs[T any](s *Store, table string) (map[string]T, error) {
	rows, err := s.db.Query(`SELECT id, data FROM ` + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]T{}
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/store/ -v -run 'Identify|Matches|Corrections|Releases'`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store
git commit -m "feat: store identification targets, matches, corrections, releases and artists"
```

---
### Task 6: Store — applying identities, the review queue and genre aliases

**Files:**
- Create: `internal/store/apply.go`
- Modify: `internal/store/catalog.go` (`Counts`, `AlbumView`, `TrackView`, `albumCols`, `queryAlbums`, `trackQuery`, `queryTracks`)
- Test: `internal/store/apply_test.go`; modify `internal/store/catalog_test.go:76`

**Interfaces:**
- Consumes: Task 5 types.
- Produces:
  - `store.GenreLink{Genre, Style, Source string}` (JSON `genre`, `style` omitempty, `source`)
  - `store.TrackIdentity{FileID int64; Title, RecordingID string}`
  - `store.AlbumIdentity{AlbumID int64; Query, Status, ReleaseID string; Confidence float64; Title, Artist string; Compilation bool; OriginalYear int; Country, ArtistID, CoverCache string; Genres []GenreLink; Tracks []TrackIdentity}`
  - `(*Store).ApplyIdentities(ids []AlbumIdentity, unmapped map[string]int) error`
  - `(*Store).ReleaseIDsInUse() ([]string, error)`, `(*Store).CoverlessReleases() ([]string, error)`
  - `store.UnidentifiedView{AlbumView; Candidates []Candidate}`, `(*Store).Unidentified() ([]UnidentifiedView, error)`
  - `store.GenreAlias{Raw, Genre, Style string}`, `(*Store).SetGenreAlias(GenreAlias) error`, `(*Store).GenreAliases() ([]GenreAlias, error)`
  - `store.UnmappedGenre{Raw string; Albums int}`, `(*Store).UnmappedGenres() ([]UnmappedGenre, error)`
  - `AlbumView` gains `OriginalYear, Selection, CoverCache, RawGenres, Status, ReleaseID, Confidence, Country, ArtistID, TagTitle, TagArtist` and `Genres` becomes `[]GenreLink`; `Title`/`Artist` are MusicBrainz's when identified. `TrackView` gains `TagTitle`, `RecordingID`; `Title` is MusicBrainz's when aligned. `Counts` gains `Identified`, `Unidentified`.

`ApplyIdentities` always receives every album (the runner recomputes them all, locally, in well under a second for 10k albums), so it resets the track columns and the derived genre tables before writing.

- [ ] **Step 1: Write the failing tests**

`internal/store/apply_test.go`:

```go
package store

import (
	"reflect"
	"testing"
)

func albumID(t *testing.T, s *Store, title string) int64 {
	t.Helper()
	as, err := s.Albums("")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if a.TagTitle == title {
			return a.ID
		}
	}
	t.Fatalf("no album %q", title)
	return 0
}

func TestApplyIdentities(t *testing.T) {
	s := sample(t)
	id := albumID(t, s, "Signos")
	before, _, _ := s.Album(id)
	first := before.Tracks[0].ID
	genres := []GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}
	err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Query: "q", Status: StatusAuto, ReleaseID: "r1", Confidence: 0.9,
		Title: "Signos (remasterizado)", Artist: "Soda Stereo", OriginalYear: 1986, Country: "AR", ArtistID: "a1",
		CoverCache: "cache/covers/r1.jpg", Genres: genres,
		Tracks: []TrackIdentity{{FileID: first, Title: "Signos (MB)", RecordingID: "rec1"}}}},
		map[string]int{"Chamamé": 2})
	if err != nil {
		t.Fatal(err)
	}
	a, ok, err := s.Album(id)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if a.Title != "Signos (remasterizado)" || a.TagTitle != "Signos" || a.Status != StatusAuto || a.ReleaseID != "r1" ||
		a.Country != "AR" || a.OriginalYear != 1986 || a.CoverCache != "cache/covers/r1.jpg" || !reflect.DeepEqual(a.Genres, genres) {
		t.Fatalf("album = %+v", a.AlbumView)
	}
	if tr := a.Tracks[0]; tr.Title != "Signos (MB)" || tr.TagTitle != "Signos" || tr.RecordingID != "rec1" {
		t.Fatalf("track = %+v", tr)
	}
	if a.Tracks[1].Title != "No existes" {
		t.Fatalf("unaligned track lost its title: %+v", a.Tracks[1])
	}
	c, _ := s.Counts()
	if c.Identified != 1 || c.Unidentified != 0 {
		t.Fatalf("counts = %+v", c)
	}
	un, err := s.UnmappedGenres()
	if err != nil || !reflect.DeepEqual(un, []UnmappedGenre{{Raw: "Chamamé", Albums: 2}}) {
		t.Fatalf("unmapped = %+v, err %v", un, err)
	}
}

func TestIdentityKeptAcrossRegroup(t *testing.T) {
	s := sample(t)
	id := albumID(t, s, "Signos")
	a, _, _ := s.Album(id)
	if err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Status: StatusAuto, ReleaseID: "r1", Title: "Signos MB",
		Genres: []GenreLink{{Genre: "Rock", Source: "mb"}}, Tracks: []TrackIdentity{{FileID: a.Tracks[0].ID, Title: "MB"}}}}, nil); err != nil {
		t.Fatal(err)
	}
	regroup(t, s)
	a, _, _ = s.Album(id)
	// The album row keeps its identity; tracks are rebuilt until the next apply.
	if a.Status != StatusAuto || a.Title != "Signos MB" || len(a.Genres) != 1 || a.Tracks[0].Title != "Signos" {
		t.Fatalf("after regroup: %+v", a)
	}
}

func TestUnidentifiedAndReleasesInUse(t *testing.T) {
	s := sample(t)
	signos, clics, locura := albumID(t, s, "Signos"), albumID(t, s, "Clics modernos"), albumID(t, s, "Locura")
	cands := []Candidate{{ReleaseID: "r9", Title: "Clics Modernos", Artist: "Charly García", Score: 0.7}}
	if err := s.SaveMatches([]Match{{Query: "q-clics", Status: StatusUnmatched, Confidence: 0.7, Candidates: cands, MatcherVersion: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyIdentities([]AlbumIdentity{
		{AlbumID: signos, Status: StatusAuto, ReleaseID: "r1"},
		{AlbumID: clics, Query: "q-clics", Status: StatusUnmatched, Confidence: 0.7},
		{AlbumID: locura, Status: StatusManual, ReleaseID: "r2"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	un, err := s.Unidentified()
	if err != nil || len(un) != 1 || un[0].ID != clics || !reflect.DeepEqual(un[0].Candidates, cands) {
		t.Fatalf("unidentified = %+v, err %v", un, err)
	}
	inUse, _ := s.ReleaseIDsInUse()
	if !reflect.DeepEqual(inUse, []string{"r1", "r2"}) {
		t.Fatalf("in use = %q", inUse)
	}
	coverless, _ := s.CoverlessReleases() // Signos has cover.jpg
	if !reflect.DeepEqual(coverless, []string{"r2"}) {
		t.Fatalf("coverless = %q", coverless)
	}
}

func TestGenreAliases(t *testing.T) {
	s := open(t)
	if err := s.SetGenreAlias(GenreAlias{Raw: "chamame", Genre: "Latin"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGenreAlias(GenreAlias{Raw: "chamame", Genre: "Folk, World, & Country", Style: "Folk"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGenreAlias(GenreAlias{Raw: "uruguay"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GenreAliases()
	want := []GenreAlias{{Raw: "chamame", Genre: "Folk, World, & Country", Style: "Folk"}, {Raw: "uruguay"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("aliases = %+v, err %v", got, err)
	}
}
```

In `internal/store/catalog_test.go` line 76, `signos.Genres == nil` becomes `signos.RawGenres == nil` (the raw tag values moved to `RawGenres`).

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/store/`
Expected: FAIL (`undefined: GenreLink`, `a.TagTitle undefined`, …).

- [ ] **Step 3: Implement `internal/store/apply.go`**

```go
package store

// GenreLink is one of an album's genres and where it came from.
type GenreLink struct {
	Genre  string `json:"genre"`
	Style  string `json:"style,omitempty"`
	Source string `json:"source"` // tag | mb | discogs | manual
}

// TrackIdentity is the MusicBrainz track a catalog track was paired with.
type TrackIdentity struct {
	FileID      int64
	Title       string
	RecordingID string
}

// AlbumIdentity is what the identification decided for an album.
type AlbumIdentity struct {
	AlbumID      int64
	Query        string // the album's identify query key
	Status       string
	ReleaseID    string
	Confidence   float64
	Title        string // MusicBrainz's; "" keeps the grouped one
	Artist       string
	Compilation  bool // MusicBrainz credits Various Artists
	OriginalYear int
	Country      string
	ArtistID     string
	CoverCache   string
	Genres       []GenreLink
	Tracks       []TrackIdentity
}

// ApplyIdentities writes the identity of every album, and the raw genre
// values nothing maps with how many albums use each. ids must cover all
// albums: track titles and genres not in ids are cleared.
func (s *Store) ApplyIdentities(ids []AlbumIdentity, unmapped map[string]int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{`UPDATE tracks SET mb_title = '', recording_id = ''`, `DELETE FROM album_genres`, `DELETE FROM unmapped_genres`} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	al, err := tx.Prepare(`UPDATE albums SET match_query = ?, status = ?, release_id = ?, confidence = ?, mb_title = ?,
		mb_artist = ?, mb_compilation = ?, original_year = ?, country = ?, artist_id = ?, cover_cache = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer al.Close()
	gi, err := tx.Prepare(`INSERT OR IGNORE INTO album_genres (album_id, genre, style, source) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer gi.Close()
	ti, err := tx.Prepare(`UPDATE tracks SET mb_title = ?, recording_id = ? WHERE file_id = ?`)
	if err != nil {
		return err
	}
	defer ti.Close()
	for _, id := range ids {
		if _, err := al.Exec(id.Query, id.Status, id.ReleaseID, id.Confidence, id.Title, id.Artist, id.Compilation,
			id.OriginalYear, id.Country, id.ArtistID, id.CoverCache, id.AlbumID); err != nil {
			return err
		}
		for _, g := range id.Genres {
			if _, err := gi.Exec(id.AlbumID, g.Genre, g.Style, g.Source); err != nil {
				return err
			}
		}
		for _, t := range id.Tracks {
			if _, err := ti.Exec(t.Title, t.RecordingID, t.FileID); err != nil {
				return err
			}
		}
	}
	for raw, n := range unmapped {
		if _, err := tx.Exec(`INSERT INTO unmapped_genres (raw, albums) VALUES (?, ?)`, raw, n); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const identified = `status IN ('auto', 'manual') AND release_id != ''`

// ReleaseIDsInUse lists the releases of identified albums.
func (s *Store) ReleaseIDsInUse() ([]string, error) {
	return s.column(`SELECT DISTINCT release_id FROM albums WHERE ` + identified + ` ORDER BY release_id`)
}

// CoverlessReleases lists the releases of identified albums that have
// neither a folder image nor an embedded picture.
func (s *Store) CoverlessReleases() ([]string, error) {
	return s.column(`SELECT DISTINCT release_id FROM albums WHERE ` + identified +
		` AND cover_path = '' AND cover_file_id = 0 ORDER BY release_id`)
}

func (s *Store) column(q string, args ...any) ([]string, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// UnidentifiedView is an album of the Sin identificar queue.
type UnidentifiedView struct {
	AlbumView
	Candidates []Candidate `json:"candidates"`
}

// Unidentified lists the albums below the threshold with their candidates.
func (s *Store) Unidentified() ([]UnidentifiedView, error) {
	as, err := s.queryAlbums(`SELECT `+albumCols+` FROM albums WHERE status = ?
		ORDER BY artist COLLATE NOCASE, title COLLATE NOCASE`, StatusUnmatched)
	if err != nil {
		return nil, err
	}
	ms, err := s.Matches()
	if err != nil {
		return nil, err
	}
	out := make([]UnidentifiedView, 0, len(as))
	for _, a := range as {
		c := ms[a.MatchQuery].Candidates
		if c == nil {
			c = []Candidate{}
		}
		out = append(out, UnidentifiedView{AlbumView: a, Candidates: c})
	}
	return out, nil
}

// GenreAlias is the user's mapping of a raw genre value.
type GenreAlias struct {
	Raw   string `json:"raw"`   // genres.Key form
	Genre string `json:"genre"` // "" = not a genre: ignore it
	Style string `json:"style"`
}

func (s *Store) SetGenreAlias(a GenreAlias) error {
	_, err := s.db.Exec(`INSERT INTO genre_alias (raw, genre, style) VALUES (?, ?, ?)
		ON CONFLICT(raw) DO UPDATE SET genre = excluded.genre, style = excluded.style`, a.Raw, a.Genre, a.Style)
	return err
}

func (s *Store) GenreAliases() ([]GenreAlias, error) {
	rows, err := s.db.Query(`SELECT raw, genre, style FROM genre_alias ORDER BY raw`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GenreAlias{}
	for rows.Next() {
		var a GenreAlias
		if err := rows.Scan(&a.Raw, &a.Genre, &a.Style); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UnmappedGenre is an entry of the Géneros por mapear queue.
type UnmappedGenre struct {
	Raw    string `json:"raw"`
	Albums int    `json:"albums"`
}

// UnmappedGenres lists the queue, most used first.
func (s *Store) UnmappedGenres() ([]UnmappedGenre, error) {
	rows, err := s.db.Query(`SELECT raw, albums FROM unmapped_genres ORDER BY albums DESC, raw`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UnmappedGenre{}
	for rows.Next() {
		var u UnmappedGenre
		if err := rows.Scan(&u.Raw, &u.Albums); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Update the views in `internal/store/catalog.go`**

`Counts` gains two fields and two subqueries:

```go
type Counts struct {
	Files        int `json:"files"`
	Audio        int `json:"audio"`
	Unreadable   int `json:"unreadable"`
	Tracks       int `json:"tracks"`
	Albums       int `json:"albums"`
	Folders      int `json:"folders"`
	Collections  int `json:"collections"`
	Identified   int `json:"identified"`
	Unidentified int `json:"unidentified"`
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
		(SELECT COUNT(*) FROM folders WHERE collection = 1),
		(SELECT COUNT(*) FROM albums WHERE status IN ('auto', 'manual')),
		(SELECT COUNT(*) FROM albums WHERE status = 'unmatched')`).
		Scan(&c.Files, &c.Audio, &c.Unreadable, &c.Tracks, &c.Albums, &c.Folders, &c.Collections, &c.Identified, &c.Unidentified)
	return c, err
}
```

Replace `AlbumView`, `TrackView`, `albumCols`, `queryAlbums`, `trackQuery` and `queryTracks` with:

```go
type AlbumView struct {
	ID           int64       `json:"id"`
	Title        string      `json:"title"` // MusicBrainz's when identified, else from tags or folder
	Artist       string      `json:"artist"`
	Year         int         `json:"year"`
	OriginalYear int         `json:"originalYear"`
	Compilation  bool        `json:"compilation"`
	Selection    bool        `json:"selection"`
	Discs        int         `json:"discs"`
	Tracks       int         `json:"tracks"`
	DurationMs   int64       `json:"durationMs"`
	Dir          string      `json:"dir"`
	Position     int         `json:"position"`
	CoverPath    string      `json:"coverPath"`
	CoverFileID  int64       `json:"coverFileId"`
	CoverCache   string      `json:"coverCache"`
	Genres       []GenreLink `json:"genres"`
	RawGenres    []string    `json:"rawGenres"`
	Status       string      `json:"status"`
	ReleaseID    string      `json:"releaseId"`
	Confidence   float64     `json:"confidence"`
	Country      string      `json:"country"`
	ArtistID     string      `json:"artistId"`
	TagTitle     string      `json:"tagTitle"`
	TagArtist    string      `json:"tagArtist"`
	MatchQuery   string      `json:"-"`
}

type TrackView struct {
	ID          int64  `json:"id"` // the file id
	Path        string `json:"path"`
	Title       string `json:"title"` // MusicBrainz's when paired, else from tags or file name
	TagTitle    string `json:"tagTitle"`
	RecordingID string `json:"recordingId"`
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

const albumCols = `id, CASE WHEN mb_title != '' THEN mb_title ELSE title END,
	CASE WHEN mb_artist != '' THEN mb_artist ELSE artist END, year, original_year, compilation OR mb_compilation,
	selection, discs, tracks, duration_ms, dir, position, cover_path, cover_file_id, cover_cache, genres, status,
	release_id, confidence, country, artist_id, title, artist, match_query`

func (s *Store) queryAlbums(q string, args ...any) ([]AlbumView, error) {
	out, err := s.scanAlbums(q, args...)
	if err != nil {
		return nil, err
	}
	return out, s.fillGenres(out)
}

func (s *Store) scanAlbums(q string, args ...any) ([]AlbumView, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AlbumView{}
	for rows.Next() {
		var a AlbumView
		var raw string
		if err := rows.Scan(&a.ID, &a.Title, &a.Artist, &a.Year, &a.OriginalYear, &a.Compilation, &a.Selection, &a.Discs,
			&a.Tracks, &a.DurationMs, &a.Dir, &a.Position, &a.CoverPath, &a.CoverFileID, &a.CoverCache, &raw, &a.Status,
			&a.ReleaseID, &a.Confidence, &a.Country, &a.ArtistID, &a.TagTitle, &a.TagArtist, &a.MatchQuery); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &a.RawGenres); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// fillGenres sets the albums' genre links. It reads the whole (small)
// album_genres table once instead of once per album.
func (s *Store) fillGenres(as []AlbumView) error {
	if len(as) == 0 {
		return nil
	}
	at := make(map[int64]int, len(as))
	for i := range as {
		at[as[i].ID] = i
		as[i].Genres = []GenreLink{}
	}
	rows, err := s.db.Query(`SELECT album_id, genre, style, source FROM album_genres ORDER BY album_id, genre, style`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var g GenreLink
		if err := rows.Scan(&id, &g.Genre, &g.Style, &g.Source); err != nil {
			return err
		}
		if i, ok := at[id]; ok {
			as[i].Genres = append(as[i].Genres, g)
		}
	}
	return rows.Err()
}

const trackQuery = `SELECT t.file_id, f.path, CASE WHEN t.mb_title != '' THEN t.mb_title ELSE t.title END, t.title,
	t.recording_id, t.artist, t.disc, t.track, t.year, t.duration_ms,
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
		if err := rows.Scan(&t.ID, &t.Path, &t.Title, &t.TagTitle, &t.RecordingID, &t.Artist, &t.Disc, &t.Track, &t.Year,
			&t.DurationMs, &t.Format, &t.Codec, &t.Lossless, &t.Bitrate, &t.SampleRate, &t.BitDepth, &t.Fingerprint); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
```

(`Albums` keeps ordering by the `artist` column, which in `ORDER BY` is the table's tag artist: identification does not reshuffle lists.)

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/store/ ./internal/server/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store
git commit -m "feat: apply album identities, review queue and genre aliases in the store"
```

---
### Task 7: Genre taxonomy and normalization

**Files:**
- Create: `internal/genres/genres.go`, `internal/genres/taxonomy.go`, `internal/genres/genres_test.go`

**Interfaces:**
- Consumes: `nameparse.Norm`.
- Produces: `genres.Mapping{Genre, Style string}`, `genres.Genre{Name string; Styles []string}`, `genres.Taxonomy []Genre`, `genres.Key(raw string) string`, `genres.New(user map[string]Mapping) *Normalizer` (user aliases by `Key`; `Genre == ""` means "not a genre"), `(*Normalizer).Map(raw string) ([]Mapping, bool)`, `genres.FromDiscogs(gs, styles []string) []Mapping`, `genres.Valid(genre, style string) bool`.

The taxonomy is Discogs' 15 genres with the styles that matter for this collection (Discogs names; `Murga` is tordo's own addition). Raw values from tags and MusicBrainz go through `Map`: the user's aliases, then ignored non-genres (`Other`, country names), then exact genre/style names, then built-in aliases, then compound values split on `, / ; |` (the longest known run first, so `Latin, Folk, World, & Country` is two genres, not four). `ok == false` means some part is unknown: the raw value belongs in *Géneros por mapear*. Discogs genres and styles are already taxonomy values and are paired by `FromDiscogs` without going through `Map`.

- [ ] **Step 1: Write the failing tests**

`internal/genres/genres_test.go`:

```go
package genres

import (
	"reflect"
	"testing"
)

const fwc = "Folk, World, & Country"

func TestMap(t *testing.T) {
	n := New(nil)
	cases := []struct {
		raw  string
		want []Mapping
		ok   bool
	}{
		{"Rock", []Mapping{{Genre: "Rock"}}, true},
		{"rock nacional", []Mapping{{Genre: "Rock"}}, true},
		{"Samba", []Mapping{{Genre: "Latin", Style: "Samba"}}, true},
		{"Bossa Nova", []Mapping{{Genre: "Latin", Style: "Bossanova"}}, true},
		{"Folclore", []Mapping{{Genre: fwc, Style: "Folk"}}, true},
		{"Latino", []Mapping{{Genre: "Latin"}}, true},
		{"Latin, Folk, World, & Country", []Mapping{{Genre: "Latin"}, {Genre: fwc}}, true},
		{"Pop Pop Internacional", []Mapping{{Genre: "Pop"}}, true},
		{"Rock/Pop", []Mapping{{Genre: "Rock"}, {Genre: "Pop"}}, true},
		{"Funk / Soul", []Mapping{{Genre: "Funk / Soul"}}, true},
		{"murga", []Mapping{{Genre: "Latin", Style: "Murga"}}, true},
		{"Pop Rock", []Mapping{{Genre: "Rock", Style: "Pop Rock"}}, true},
		{"Uruguay", nil, true},
		{"Other", nil, true},
		{"(12)", nil, true},
		{"", nil, true},
		{"Chamamé", nil, false},
		{"Rock, Chamamé", []Mapping{{Genre: "Rock"}}, false},
	}
	for _, c := range cases {
		got, ok := n.Map(c.raw)
		if ok != c.ok || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Map(%q) = %+v, %v; want %+v, %v", c.raw, got, ok, c.want, c.ok)
		}
	}
}

func TestUserAliasesWin(t *testing.T) {
	n := New(map[string]Mapping{
		Key("Chamamé"): {Genre: fwc, Style: "Folk"},
		Key("Rock"):    {Genre: "Latin"},
		Key("Basura"):  {},
	})
	if got, ok := n.Map("chamame"); !ok || !reflect.DeepEqual(got, []Mapping{{Genre: fwc, Style: "Folk"}}) {
		t.Errorf("chamame = %+v, %v", got, ok)
	}
	if got, _ := n.Map("Rock"); !reflect.DeepEqual(got, []Mapping{{Genre: "Latin"}}) {
		t.Errorf("rock = %+v", got)
	}
	if got, ok := n.Map("basura"); !ok || got != nil {
		t.Errorf("basura = %+v, %v", got, ok)
	}
}

func TestKey(t *testing.T) {
	if Key("Pop  Pop Internacional!") != "pop internacional" || Key("Folclóre") != "folclore" {
		t.Fatalf("Key: %q %q", Key("Pop  Pop Internacional!"), Key("Folclóre"))
	}
}

func TestFromDiscogs(t *testing.T) {
	got := FromDiscogs([]string{"Rock", "Latin"}, []string{"Pop Rock", "Samba", "Rarísimo"})
	want := []Mapping{{Genre: "Rock", Style: "Pop Rock"}, {Genre: "Latin", Style: "Samba"}, {Genre: "Rock", Style: "Rarísimo"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	got = FromDiscogs([]string{"Jazz", "Latin"}, []string{"Bossanova"})
	want = []Mapping{{Genre: "Latin", Style: "Bossanova"}, {Genre: "Jazz"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if FromDiscogs(nil, []string{"Samba"}) != nil {
		t.Fatal("styles without genres")
	}
}

func TestValid(t *testing.T) {
	if !Valid("Latin", "") || !Valid("Latin", "Samba") || !Valid(fwc, "Folk") {
		t.Fatal("valid values rejected")
	}
	if Valid("Latin", "Pop Rock") || Valid("Cumbia", "") || Valid("", "") {
		t.Fatal("invalid values accepted")
	}
}

func TestTaxonomyKeysAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, g := range Taxonomy {
		for _, name := range append([]string{g.Name}, g.Styles...) {
			if prev, ok := seen[Key(name)]; ok {
				t.Errorf("%q and %q share the key %q", prev, name, Key(name))
			}
			seen[Key(name)] = name
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/genres/`
Expected: FAIL (package does not compile: `undefined: New`, …).

- [ ] **Step 3: Write the taxonomy and built-in aliases**

`internal/genres/taxonomy.go`:

```go
package genres

// Genre is a top-level genre with its styles.
type Genre struct {
	Name   string   `json:"name"`
	Styles []string `json:"styles"`
}

// Taxonomy is Discogs' genres with the styles tordo knows under each
// (Discogs' names; "Murga" is tordo's own). A style belongs to one genre.
var Taxonomy = []Genre{
	{"Blues", []string{"Chicago Blues", "Country Blues", "Delta Blues", "Electric Blues", "Piano Blues"}},
	{"Brass & Military", []string{"Brass Band", "Marches", "Military"}},
	{"Children's", []string{"Educational", "Nursery Rhymes", "Story"}},
	{"Classical", []string{"Baroque", "Choral", "Contemporary", "Modern", "Opera", "Romantic"}},
	{"Electronic", []string{"Ambient", "Downtempo", "Drum n Bass", "Electro", "Experimental", "House", "IDM", "Minimal",
		"Synth-pop", "Techno", "Trance"}},
	{"Folk, World, & Country", []string{"African", "Andean", "Bluegrass", "Celtic", "Chacarera", "Country", "Fado",
		"Flamenco", "Folk", "Zamba"}},
	{"Funk / Soul", []string{"Disco", "Funk", "Gospel", "Neo Soul", "Rhythm & Blues", "Soul"}},
	{"Hip Hop", []string{"Boom Bap", "Conscious", "Gangsta", "Trip Hop"}},
	{"Jazz", []string{"Bebop", "Big Band", "Contemporary Jazz", "Cool Jazz", "Free Jazz", "Fusion", "Hard Bop",
		"Latin Jazz", "Modal", "Post Bop", "Soul-Jazz", "Swing"}},
	{"Latin", []string{"Bachata", "Baião", "Bolero", "Bossanova", "Candombe", "Cha-Cha", "Choro", "Cumbia", "Forró",
		"Mambo", "Merengue", "Milonga", "MPB", "Murga", "Música Criolla", "Nueva Cancion", "Nueva Trova", "Ranchera",
		"Reggaeton", "Rumba", "Salsa", "Samba", "Son", "Tango", "Vallenato"}},
	{"Non-Music", []string{"Comedy", "Field Recording", "Interview", "Poetry", "Spoken Word"}},
	{"Pop", []string{"Ballad", "Chanson", "Easy Listening", "Europop", "Indie Pop", "Light Music", "Vocal"}},
	{"Reggae", []string{"Dancehall", "Dub", "Rocksteady", "Roots Reggae", "Ska"}},
	{"Rock", []string{"Alternative Rock", "Blues Rock", "Classic Rock", "Folk Rock", "Garage Rock", "Hard Rock",
		"Heavy Metal", "Indie Rock", "New Wave", "Pop Rock", "Post-Punk", "Prog Rock", "Psychedelic Rock", "Punk",
		"Rock & Roll", "Soft Rock"}},
	{"Stage & Screen", []string{"Musical", "Score", "Soundtrack", "Theme"}},
}

// ignored are raw values that are not genres. Country names are common in
// this collection ("Uruguay"); the country comes from MusicBrainz instead.
var ignored = keys("other", "others", "otro", "otros", "outros", "unknown", "desconocido", "genre", "género",
	"misc", "miscellaneous", "general", "default", "none", "various", "varios", "music", "música",
	"uruguay", "uruguayo", "argentina", "argentino", "brasil", "brazil", "brasileiro", "chile", "cuba", "méxico",
	"colombia", "perú", "españa", "spain", "usa")

// aliases are the built-in spellings of taxonomy values.
var aliases = func() map[string]Mapping {
	m := map[string]Mapping{}
	add := func(genre, style string, raws ...string) {
		for _, r := range raws {
			m[Key(r)] = Mapping{Genre: genre, Style: style}
		}
	}
	add("Latin", "", "latino", "latina", "latin music", "música latina", "latinoamericana", "música latinoamericana")
	add("Folk, World, & Country", "Folk", "folclore", "folklore", "folklore argentino", "folclore argentino",
		"folklore uruguayo", "folclore uruguayo", "folk music")
	add("Folk, World, & Country", "", "world", "world music", "música del mundo", "música do mundo")
	add("Rock", "", "rock nacional", "rock argentino", "rock en español", "rock latino", "rock uruguayo", "rock brasileiro")
	add("Rock", "Rock & Roll", "rock and roll", "rock n roll", "rock'n'roll")
	add("Rock", "Alternative Rock", "alternative", "alternativo", "alternativa")
	add("Rock", "Indie Rock", "indie")
	add("Rock", "Heavy Metal", "metal")
	add("Rock", "Punk", "punk rock")
	add("Latin", "MPB", "música popular brasileira")
	add("Latin", "Bossanova", "bossa nova", "bossa")
	add("Latin", "Samba", "samba enredo", "samba de enredo", "sambas de enredo", "pagode", "samba-canção")
	add("Latin", "Tango", "tango nuevo", "tango argentino")
	add("Latin", "Cumbia", "cumbia colombiana", "cumbia villera", "cumbia argentina")
	add("Latin", "Nueva Trova", "trova", "nueva trova cubana")
	add("Latin", "Nueva Cancion", "nueva canción", "canto popular", "canto popular uruguayo")
	add("Latin", "Murga", "murgas")
	add("Jazz", "Latin Jazz", "jazz latino")
	add("Hip Hop", "", "hiphop", "rap", "hip hop rap")
	add("Funk / Soul", "Rhythm & Blues", "r&b", "rnb", "rhythm and blues")
	add("Electronic", "", "electrónica", "dance", "edm")
	add("Classical", "", "clásica", "música clásica")
	add("Stage & Screen", "Soundtrack", "banda sonora", "bso", "ost", "soundtracks", "trilha sonora")
	add("Children's", "", "infantil", "infantiles", "niños", "kids", "children", "children's music", "música infantil")
	add("Pop", "", "pop internacional", "pop latino", "pop en español")
	add("Non-Music", "Spoken Word", "audiolibro", "audiobook", "spoken")
	return m
}()

func keys(raws ...string) map[string]bool {
	m := map[string]bool{}
	for _, r := range raws {
		m[Key(r)] = true
	}
	return m
}
```

- [ ] **Step 4: Implement the normalizer**

`internal/genres/genres.go`:

```go
// Package genres maps raw genre values (tags, MusicBrainz) to a two-level
// taxonomy based on Discogs': genre and style.
package genres

import (
	"slices"
	"strings"

	"tordo/internal/nameparse"
)

// Mapping is a genre, or a style under its genre.
type Mapping struct {
	Genre string `json:"genre"`
	Style string `json:"style,omitempty"`
}

// Key folds a raw value for lookups: normalized like names, with a word
// repeated right after itself dropped ("Pop Pop Internacional").
func Key(raw string) string {
	var out []string
	for _, w := range strings.Fields(nameparse.Norm(raw)) {
		if len(out) == 0 || out[len(out)-1] != w {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

var (
	known      = map[string]Mapping{} // genre and style names by Key
	styleOwner = map[string]string{}  // style Key → genre
)

func init() {
	for _, g := range Taxonomy {
		known[Key(g.Name)] = Mapping{Genre: g.Name}
		for _, s := range g.Styles {
			known[Key(s)] = Mapping{Genre: g.Name, Style: s}
			styleOwner[Key(s)] = g.Name
		}
	}
}

// Normalizer maps raw values with the built-in aliases and the user's.
type Normalizer struct {
	user map[string]Mapping // by Key; an empty Genre means "not a genre"
}

func New(user map[string]Mapping) *Normalizer { return &Normalizer{user: user} }

// Map returns what a raw value means. ok is false when (part of) it is
// unknown: the value then belongs in the Géneros por mapear queue. A value
// that is not a genre ("Other", "Uruguay") maps to nothing with ok true.
func (n *Normalizer) Map(raw string) ([]Mapping, bool) {
	if ms, ok := n.one(Key(raw)); ok {
		return ms, true
	}
	var parts []string
	for _, p := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '/' || r == ';' || r == '|' }) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) < 2 {
		return nil, false
	}
	var out []Mapping
	all := true
	for i := 0; i < len(parts); {
		j := len(parts)
		for ; j > i; j-- { // the longest known run first: "Folk, World, & Country"
			if ms, ok := n.one(Key(strings.Join(parts[i:j], ", "))); ok {
				out = appendNew(out, ms...)
				break
			}
		}
		if j == i {
			all = false
			i++
		} else {
			i = j
		}
	}
	return out, all
}

func (n *Normalizer) one(k string) ([]Mapping, bool) {
	if m, ok := n.user[k]; ok {
		if m.Genre == "" {
			return nil, true
		}
		return []Mapping{m}, true
	}
	if k == "" || ignored[k] || isNumber(k) {
		return nil, true
	}
	if m, ok := known[k]; ok {
		return []Mapping{m}, true
	}
	if m, ok := aliases[k]; ok {
		return []Mapping{m}, true
	}
	return nil, false
}

func isNumber(k string) bool {
	return strings.Trim(k, "0123456789 ") == ""
}

func appendNew(ms []Mapping, add ...Mapping) []Mapping {
	for _, m := range add {
		if !slices.Contains(ms, m) {
			ms = append(ms, m)
		}
	}
	return ms
}

// FromDiscogs pairs a Discogs release's genres and styles: a style goes
// under its taxonomy genre when the release lists that genre, else under the
// release's first genre; genres left without a style stand alone.
func FromDiscogs(gs, styles []string) []Mapping {
	if len(gs) == 0 {
		return nil
	}
	var out []Mapping
	used := map[string]bool{}
	for _, s := range styles {
		g := gs[0]
		if o := styleOwner[Key(s)]; o != "" && slices.Contains(gs, o) {
			g = o
		}
		out = appendNew(out, Mapping{Genre: g, Style: s})
		used[g] = true
	}
	for _, g := range gs {
		if !used[g] {
			out = appendNew(out, Mapping{Genre: g})
		}
	}
	return out
}

// Valid reports whether genre is in the taxonomy and style ("" for none) is
// one of its styles.
func Valid(genre, style string) bool {
	for _, g := range Taxonomy {
		if g.Name == genre {
			return style == "" || slices.Contains(g.Styles, style)
		}
	}
	return false
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/genres/ -v`
Expected: PASS. If a `TestMap` case fails, fix the alias table, not the test: the cases are values from the collection.

- [ ] **Step 6: Commit**

```bash
git add internal/genres
git commit -m "feat: genre taxonomy based on Discogs and normalization of raw genres"
```

---
### Task 8: MusicBrainz client and Cover Art Archive

**Files:**
- Create: `internal/musicbrainz/musicbrainz.go`, `internal/musicbrainz/musicbrainz_test.go`
- Create: `internal/musicbrainz/testdata/search-signos.json`, `release-signos.json`, `release-group-signos.json`, `artist-soda.json`, `artist-subdivision.json`

**Interfaces:**
- Consumes: `httpx` (Task 1).
- Produces:
  - `musicbrainz.BaseURL`, `CoverBaseURL`, `VariousArtistsID = "89ad4ac3-39f7-470e-963a-56509c546377"`, `ErrNotFound`, `ErrInvalidID`, `ValidID(id string) bool`
  - `musicbrainz.Client{Base, CoverBase string; HTTP, Covers *httpx.Client}`, `musicbrainz.New() *Client`
  - `SearchQuery{Title, Artist string; Various bool}`; `ReleaseHit{ID, Title, Artist string; ArtistNames []string; Various bool; Year int; Country string; Tracks, Score int}`; `(*Client).SearchReleases(ctx, SearchQuery) ([]ReleaseHit, error)`
  - `Track{Disc, Position int; Title string; LengthMs int64; RecordingID string}`; `Release{ID, Title, Artist string; ArtistIDs, ArtistNames []string; Various bool; Year int; Country, GroupID string; OriginalYear int; Tracks []Track; Genres, DiscogsURLs []string}`; `(*Client).Release(ctx, id) (Release, error)`
  - `GroupLinks{DiscogsURLs, Genres []string}`; `(*Client).ReleaseGroupLinks(ctx, id) (GroupLinks, error)`
  - `Artist{ID, Name, SortName, Type, Country, Area, Begin, End string}`; `(*Client).Artist(ctx, id) (Artist, error)`
  - `(*Client).Cover(ctx, kind, id string) ([]byte, error)` — `kind` is `"release"` or `"release-group"`.

The fixtures are trimmed to the fields tordo reads but keep the exact shape of MusicBrainz's JSON (`artist-credit`, `release-group`, `media[].tracks[]`, `relations[].url.resource`, `genres[]`). The ids are made up (valid MBID format). Real responses are in https://musicbrainz.org/doc/MusicBrainz_API; Step 6 compares the parser against one live response once, by hand — the tests themselves never touch the network.

Search uses Lucene terms, not phrases: messy tag titles still match. Terms are lowercased (so `and`/`or`/`not` in a title are words, not operators) and Lucene's syntax characters become spaces.

- [ ] **Step 1: Write the fixtures**

`internal/musicbrainz/testdata/search-signos.json`:

```json
{
  "created": "2026-10-01T12:00:00.000Z",
  "count": 2,
  "offset": 0,
  "releases": [
    {
      "id": "a1b2c3d4-0000-4000-8000-000000000001",
      "score": 100,
      "title": "Signos",
      "status": "Official",
      "artist-credit": [
        {"name": "Soda Stereo", "artist": {"id": "a1b2c3d4-0000-4000-8000-000000000003", "name": "Soda Stereo", "sort-name": "Soda Stereo"}}
      ],
      "release-group": {"id": "a1b2c3d4-0000-4000-8000-000000000002", "primary-type": "Album"},
      "date": "1986-11-10",
      "country": "AR",
      "track-count": 8,
      "media": [{"format": "Vinyl", "track-count": 8}]
    },
    {
      "id": "a1b2c3d4-0000-4000-8000-000000000004",
      "score": 91,
      "title": "Signos",
      "artist-credit": [
        {"name": "Various Artists", "artist": {"id": "89ad4ac3-39f7-470e-963a-56509c546377", "name": "Various Artists", "sort-name": "Various Artists"}}
      ],
      "release-group": {"id": "a1b2c3d4-0000-4000-8000-000000000005"},
      "date": "2001",
      "track-count": 14
    }
  ]
}
```

`internal/musicbrainz/testdata/release-signos.json`:

```json
{
  "id": "a1b2c3d4-0000-4000-8000-000000000001",
  "title": "Signos",
  "status": "Official",
  "date": "1986-11-10",
  "country": "AR",
  "artist-credit": [
    {"name": "Soda Stereo", "joinphrase": "", "artist": {"id": "a1b2c3d4-0000-4000-8000-000000000003", "name": "Soda Stereo", "sort-name": "Soda Stereo", "type": "Group"}}
  ],
  "release-group": {"id": "a1b2c3d4-0000-4000-8000-000000000002", "title": "Signos", "first-release-date": "1986-11-10", "primary-type": "Album"},
  "media": [
    {
      "position": 1,
      "format": "Vinyl",
      "track-count": 3,
      "tracks": [
        {"id": "c1000000-0000-4000-8000-000000000001", "number": "A1", "position": 1, "title": "Táctica y estrategia", "length": 231000,
         "recording": {"id": "b1000000-0000-4000-8000-000000000001", "title": "Táctica y estrategia", "length": 231000}},
        {"id": "c1000000-0000-4000-8000-000000000002", "number": "A2", "position": 2, "title": "Sin sobresaltos", "length": 210000,
         "recording": {"id": "b1000000-0000-4000-8000-000000000002", "title": "Sin sobresaltos", "length": 210000}},
        {"id": "c1000000-0000-4000-8000-000000000003", "number": "A3", "position": 3, "title": "Prófugos", "length": null,
         "recording": {"id": "b1000000-0000-4000-8000-000000000003", "title": "Prófugos", "length": 268000}}
      ]
    }
  ],
  "relations": [
    {"type": "discogs", "target-type": "url", "url": {"id": "d1000000-0000-4000-8000-000000000001", "resource": "https://www.discogs.com/release/1234"}},
    {"type": "amazon asin", "target-type": "url", "url": {"id": "d1000000-0000-4000-8000-000000000002", "resource": "https://www.amazon.com/gp/product/B000000000"}}
  ],
  "genres": [
    {"id": "e1000000-0000-4000-8000-000000000001", "name": "rock", "count": 3},
    {"id": "e1000000-0000-4000-8000-000000000002", "name": "new wave", "count": 1}
  ]
}
```

`internal/musicbrainz/testdata/release-group-signos.json`:

```json
{
  "id": "a1b2c3d4-0000-4000-8000-000000000002",
  "title": "Signos",
  "first-release-date": "1986-11-10",
  "primary-type": "Album",
  "relations": [
    {"type": "discogs", "target-type": "url", "url": {"resource": "https://www.discogs.com/master/5678"}},
    {"type": "wikidata", "target-type": "url", "url": {"resource": "https://www.wikidata.org/wiki/Q0"}}
  ],
  "genres": [{"name": "rock", "count": 2}]
}
```

`internal/musicbrainz/testdata/artist-soda.json`:

```json
{
  "id": "a1b2c3d4-0000-4000-8000-000000000003",
  "name": "Soda Stereo",
  "sort-name": "Soda Stereo",
  "type": "Group",
  "country": "AR",
  "area": {"id": "f1000000-0000-4000-8000-000000000001", "name": "Argentina", "iso-3166-1-codes": ["AR"]},
  "life-span": {"begin": "1982", "end": "2007", "ended": true}
}
```

`internal/musicbrainz/testdata/artist-subdivision.json`:

```json
{
  "id": "a1b2c3d4-0000-4000-8000-000000000006",
  "name": "Noel Rosa",
  "sort-name": "Rosa, Noel",
  "type": "Person",
  "area": {"id": "f1000000-0000-4000-8000-000000000002", "name": "Rio de Janeiro", "iso-3166-2-codes": ["BR-RJ"]},
  "life-span": {"begin": "1910-12-11", "end": "1937-05-04", "ended": true}
}
```

- [ ] **Step 2: Write the failing tests**

`internal/musicbrainz/musicbrainz_test.go`:

```go
package musicbrainz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"tordo/internal/httpx"
)

const (
	signosID   = "a1b2c3d4-0000-4000-8000-000000000001"
	signosRG   = "a1b2c3d4-0000-4000-8000-000000000002"
	sodaID     = "a1b2c3d4-0000-4000-8000-000000000003"
	noelRosaID = "a1b2c3d4-0000-4000-8000-000000000006"
)

// fakeMB serves the fixtures and records the requests.
type fakeMB struct {
	mu   sync.Mutex
	reqs []*http.Request
}

func (f *fakeMB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	f.mu.Unlock()
	files := map[string]string{
		"/release":                   "search-signos.json",
		"/release/" + signosID:       "release-signos.json",
		"/release-group/" + signosRG: "release-group-signos.json",
		"/artist/" + sodaID:          "artist-soda.json",
		"/artist/" + noelRosaID:      "artist-subdivision.json",
	}
	if strings.HasPrefix(r.URL.Path, "/caa/") {
		if r.URL.Path == "/caa/release/"+signosID+"/front-500" {
			w.Write([]byte("JPEG"))
			return
		}
		http.NotFound(w, r)
		return
	}
	name, ok := files[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write(b)
}

func (f *fakeMB) last() *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[len(f.reqs)-1]
}

func newTest(t *testing.T) (*Client, *fakeMB) {
	t.Helper()
	f := &fakeMB{}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	h := http.Header{"User-Agent": {httpx.UserAgent}}
	return &Client{Base: srv.URL, CoverBase: srv.URL + "/caa",
		HTTP:   &httpx.Client{Backoff: time.Millisecond, Header: h},
		Covers: &httpx.Client{Backoff: time.Millisecond, Header: h}}, f
}

func TestSearchReleases(t *testing.T) {
	c, f := newTest(t)
	hits, err := c.SearchReleases(context.Background(), SearchQuery{Title: "Signos (Remastered)", Artist: "Soda Stereo"})
	if err != nil {
		t.Fatal(err)
	}
	r := f.last()
	q := r.URL.Query()
	if q.Get("query") != "release:(signos remastered) AND artist:(soda stereo)" || q.Get("fmt") != "json" ||
		q.Get("limit") != "10" || r.Header.Get("User-Agent") != httpx.UserAgent {
		t.Fatalf("request %s, User-Agent %q", r.URL, r.Header.Get("User-Agent"))
	}
	want := []ReleaseHit{
		{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR", Tracks: 8, Score: 100},
		{ID: "a1b2c3d4-0000-4000-8000-000000000004", Title: "Signos", Artist: "Various Artists", ArtistNames: []string{"Various Artists"},
			Various: true, Year: 2001, Tracks: 14, Score: 91},
	}
	if !reflect.DeepEqual(hits, want) {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchVariousAndEmpty(t *testing.T) {
	c, f := newTest(t)
	if _, err := c.SearchReleases(context.Background(), SearchQuery{Title: "História da MPB", Artist: "x", Various: true}); err != nil {
		t.Fatal(err)
	}
	if got := f.last().URL.Query().Get("query"); got != "release:(história da mpb) AND arid:"+VariousArtistsID {
		t.Fatalf("query = %q", got)
	}
	n := len(f.reqs)
	if hits, err := c.SearchReleases(context.Background(), SearchQuery{Title: " ()"}); err != nil || hits != nil || len(f.reqs) != n {
		t.Fatalf("an empty title must not be searched: %v %v", hits, err)
	}
}

func TestTerms(t *testing.T) {
	cases := map[string]string{
		"Rock AND Roll":          "rock and roll",
		`AC/DC: "Live" [1992]!`:  "ac dc live 1992",
		"Magos, Espadas Y Rosas": "magos, espadas y rosas",
	}
	for in, want := range cases {
		if got := terms(in); got != want {
			t.Errorf("terms(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRelease(t *testing.T) {
	c, f := newTest(t)
	rel, err := c.Release(context.Background(), signosID)
	if err != nil {
		t.Fatal(err)
	}
	if inc := f.last().URL.Query().Get("inc"); inc != "recordings artist-credits release-groups url-rels genres" {
		t.Fatalf("inc = %q", inc)
	}
	want := Release{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistIDs: []string{sodaID}, ArtistNames: []string{"Soda Stereo"},
		Year: 1986, Country: "AR", GroupID: signosRG, OriginalYear: 1986,
		Tracks: []Track{
			{Disc: 1, Position: 1, Title: "Táctica y estrategia", LengthMs: 231000, RecordingID: "b1000000-0000-4000-8000-000000000001"},
			{Disc: 1, Position: 2, Title: "Sin sobresaltos", LengthMs: 210000, RecordingID: "b1000000-0000-4000-8000-000000000002"},
			{Disc: 1, Position: 3, Title: "Prófugos", LengthMs: 268000, RecordingID: "b1000000-0000-4000-8000-000000000003"},
		},
		Genres: []string{"rock", "new wave"}, DiscogsURLs: []string{"https://www.discogs.com/release/1234"}}
	if !reflect.DeepEqual(rel, want) {
		t.Fatalf("release = %+v", rel)
	}
}

func TestReleaseErrors(t *testing.T) {
	c, f := newTest(t)
	if _, err := c.Release(context.Background(), "a1b2c3d4-0000-4000-8000-00000000ffff"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	n := len(f.reqs)
	for _, bad := range []string{"", "../artist/x", "A1B2C3D4-0000-4000-8000-000000000001"} {
		if _, err := c.Release(context.Background(), bad); !errors.Is(err, ErrInvalidID) {
			t.Fatalf("Release(%q) err = %v", bad, err)
		}
	}
	if len(f.reqs) != n {
		t.Fatal("an invalid id was requested")
	}
}

func TestReleaseGroupLinks(t *testing.T) {
	c, _ := newTest(t)
	g, err := c.ReleaseGroupLinks(context.Background(), signosRG)
	if err != nil || !reflect.DeepEqual(g, GroupLinks{DiscogsURLs: []string{"https://www.discogs.com/master/5678"}, Genres: []string{"rock"}}) {
		t.Fatalf("links = %+v, err %v", g, err)
	}
}

func TestArtist(t *testing.T) {
	c, _ := newTest(t)
	a, err := c.Artist(context.Background(), sodaID)
	want := Artist{ID: sodaID, Name: "Soda Stereo", SortName: "Soda Stereo", Type: "Group", Country: "AR", Area: "Argentina", Begin: "1982", End: "2007"}
	if err != nil || a != want {
		t.Fatalf("artist = %+v, err %v", a, err)
	}
	a, err = c.Artist(context.Background(), noelRosaID)
	if err != nil || a.Country != "BR" || a.Area != "Rio de Janeiro" {
		t.Fatalf("subdivision area: %+v, err %v", a, err)
	}
}

func TestCover(t *testing.T) {
	c, _ := newTest(t)
	b, err := c.Cover(context.Background(), "release", signosID)
	if err != nil || string(b) != "JPEG" {
		t.Fatalf("cover %q, err %v", b, err)
	}
	if _, err := c.Cover(context.Background(), "release-group", signosRG); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := c.Cover(context.Background(), "artist", signosID); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestNewIsPolite(t *testing.T) {
	c := New()
	if c.Base != BaseURL || c.HTTP.Limiter == nil || c.HTTP.Limiter.Rate != 1 || c.HTTP.Limiter.Burst != 1 ||
		c.HTTP.Header.Get("User-Agent") != httpx.UserAgent || c.Covers.Header.Get("User-Agent") != httpx.UserAgent {
		t.Fatalf("client = %+v", c)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/musicbrainz/`
Expected: FAIL (package does not compile).

- [ ] **Step 4: Implement**

`internal/musicbrainz/musicbrainz.go`:

```go
// Package musicbrainz is a small client for the MusicBrainz web service (at
// most one request per second, as its rules ask) and the Cover Art Archive.
package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"tordo/internal/httpx"
)

const (
	BaseURL          = "https://musicbrainz.org/ws/2"
	CoverBaseURL     = "https://coverartarchive.org"
	VariousArtistsID = "89ad4ac3-39f7-470e-963a-56509c546377"
)

var (
	ErrNotFound  = errors.New("no existe en MusicBrainz")
	ErrInvalidID = errors.New("id de MusicBrainz inválido")
)

var validID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ValidID reports whether id has the form of a MusicBrainz identifier.
func ValidID(id string) bool { return validID.MatchString(id) }

type Client struct {
	Base      string        // BaseURL in production
	CoverBase string        // CoverBaseURL in production
	HTTP      *httpx.Client // MusicBrainz
	Covers    *httpx.Client // Cover Art Archive
}

// New returns a client for the real services: one request per second to
// MusicBrainz, two per second to the Cover Art Archive.
func New() *Client {
	ua := http.Header{"User-Agent": {httpx.UserAgent}}
	return &Client{
		Base: BaseURL, CoverBase: CoverBaseURL,
		HTTP:   &httpx.Client{Limiter: &httpx.Limiter{Rate: 1, Burst: 1}, Header: ua},
		Covers: &httpx.Client{Limiter: &httpx.Limiter{Rate: 2, Burst: 2}, Header: ua},
	}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, v any) error {
	q.Set("fmt", "json")
	body, err := c.HTTP.Get(ctx, c.Base+path+"?"+q.Encode())
	if err != nil {
		return notFound(err)
	}
	return json.Unmarshal(body, v)
}

func notFound(err error) error {
	var se *httpx.StatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return ErrNotFound
	}
	return err
}

type artistCredit []struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

func (ac artistCredit) text() string {
	var b strings.Builder
	for _, c := range ac {
		b.WriteString(c.Name + c.JoinPhrase)
	}
	return b.String()
}

func (ac artistCredit) ids() []string {
	var out []string
	for _, c := range ac {
		out = append(out, c.Artist.ID)
	}
	return out
}

func (ac artistCredit) names() []string {
	var out []string
	for _, c := range ac {
		out = append(out, c.Name)
	}
	return out
}

func (ac artistCredit) various() bool {
	for _, c := range ac {
		if c.Artist.ID == VariousArtistsID {
			return true
		}
	}
	return false
}

type relations []struct {
	Type string `json:"type"`
	URL  struct {
		Resource string `json:"resource"`
	} `json:"url"`
}

func (rs relations) discogs() []string {
	var out []string
	for _, r := range rs {
		if r.Type == "discogs" && r.URL.Resource != "" {
			out = append(out, r.URL.Resource)
		}
	}
	return out
}

type genreList []struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (gs genreList) names() []string {
	var out []string
	for _, g := range gs {
		out = append(out, g.Name)
	}
	return out
}

// year reads the year of a MusicBrainz date ("1986-11-10", "1986", "").
func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

// terms turns free text into Lucene terms: lowercase (so "and", "or" and
// "not" are words, not operators) and without Lucene's syntax characters.
func terms(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`+-&|!(){}[]^"~*?:\/`, r) {
			return ' '
		}
		return r
	}, strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

type SearchQuery struct {
	Title   string
	Artist  string // ignored when Various
	Various bool   // search releases credited to Various Artists
}

// ReleaseHit is a release found by a search.
type ReleaseHit struct {
	ID          string
	Title       string
	Artist      string
	ArtistNames []string
	Various     bool
	Year        int
	Country     string
	Tracks      int
	Score       int // MusicBrainz's own relevance, 0–100
}

type searchResponse struct {
	Releases []struct {
		ID           string       `json:"id"`
		Score        int          `json:"score"`
		Title        string       `json:"title"`
		ArtistCredit artistCredit `json:"artist-credit"`
		Date         string       `json:"date"`
		Country      string       `json:"country"`
		TrackCount   int          `json:"track-count"`
	} `json:"releases"`
}

// SearchReleases finds up to 10 releases by title and artist. An empty
// title searches nothing.
func (c *Client) SearchReleases(ctx context.Context, q SearchQuery) ([]ReleaseHit, error) {
	title := terms(q.Title)
	if title == "" {
		return nil, nil
	}
	lq := "release:(" + title + ")"
	switch {
	case q.Various:
		lq += " AND arid:" + VariousArtistsID
	case terms(q.Artist) != "":
		lq += " AND artist:(" + terms(q.Artist) + ")"
	}
	var resp searchResponse
	if err := c.get(ctx, "/release", url.Values{"query": {lq}, "limit": {"10"}}, &resp); err != nil {
		return nil, err
	}
	var out []ReleaseHit
	for _, r := range resp.Releases {
		out = append(out, ReleaseHit{ID: r.ID, Title: r.Title, Artist: r.ArtistCredit.text(), ArtistNames: r.ArtistCredit.names(),
			Various: r.ArtistCredit.various(), Year: year(r.Date), Country: r.Country, Tracks: r.TrackCount, Score: r.Score})
	}
	return out, nil
}

type Track struct {
	Disc        int // the medium's position
	Position    int
	Title       string
	LengthMs    int64
	RecordingID string
}

type Release struct {
	ID           string
	Title        string
	Artist       string // the artist credit as written
	ArtistIDs    []string
	ArtistNames  []string
	Various      bool
	Year         int
	Country      string
	GroupID      string
	OriginalYear int // the release group's first release
	Tracks       []Track
	Genres       []string
	DiscogsURLs  []string
}

type releaseResponse struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Date         string       `json:"date"`
	Country      string       `json:"country"`
	ArtistCredit artistCredit `json:"artist-credit"`
	ReleaseGroup struct {
		ID               string `json:"id"`
		FirstReleaseDate string `json:"first-release-date"`
	} `json:"release-group"`
	Media []struct {
		Position int `json:"position"`
		Tracks   []struct {
			Position  int    `json:"position"`
			Title     string `json:"title"`
			Length    int64  `json:"length"`
			Recording struct {
				ID     string `json:"id"`
				Length int64  `json:"length"`
			} `json:"recording"`
		} `json:"tracks"`
	} `json:"media"`
	Relations relations `json:"relations"`
	Genres    genreList `json:"genres"`
}

// Release looks up a release with its tracks, release group, Discogs links
// and genres.
func (c *Client) Release(ctx context.Context, id string) (Release, error) {
	if !ValidID(id) {
		return Release{}, ErrInvalidID
	}
	var r releaseResponse
	inc := url.Values{"inc": {"recordings artist-credits release-groups url-rels genres"}}
	if err := c.get(ctx, "/release/"+id, inc, &r); err != nil {
		return Release{}, err
	}
	rel := Release{ID: r.ID, Title: r.Title, Artist: r.ArtistCredit.text(), ArtistIDs: r.ArtistCredit.ids(),
		ArtistNames: r.ArtistCredit.names(), Various: r.ArtistCredit.various(), Year: year(r.Date), Country: r.Country,
		GroupID: r.ReleaseGroup.ID, OriginalYear: year(r.ReleaseGroup.FirstReleaseDate), Genres: r.Genres.names(),
		DiscogsURLs: r.Relations.discogs()}
	for _, m := range r.Media {
		for _, t := range m.Tracks {
			length := t.Length
			if length == 0 {
				length = t.Recording.Length
			}
			rel.Tracks = append(rel.Tracks, Track{Disc: m.Position, Position: t.Position, Title: t.Title, LengthMs: length,
				RecordingID: t.Recording.ID})
		}
	}
	return rel, nil
}

// GroupLinks is what a release group adds when its release lacks it.
type GroupLinks struct {
	DiscogsURLs []string // Discogs masters, usually
	Genres      []string
}

func (c *Client) ReleaseGroupLinks(ctx context.Context, id string) (GroupLinks, error) {
	if !ValidID(id) {
		return GroupLinks{}, ErrInvalidID
	}
	var r struct {
		Relations relations `json:"relations"`
		Genres    genreList `json:"genres"`
	}
	if err := c.get(ctx, "/release-group/"+id, url.Values{"inc": {"url-rels genres"}}, &r); err != nil {
		return GroupLinks{}, err
	}
	return GroupLinks{DiscogsURLs: r.Relations.discogs(), Genres: r.Genres.names()}, nil
}

type Artist struct {
	ID       string
	Name     string
	SortName string
	Type     string // Person, Group…
	Country  string // ISO 3166-1 alpha-2, "" when unknown
	Area     string
	Begin    string
	End      string
}

// Artist looks up an artist. Its country is MusicBrainz's country field,
// else its area's country code, else the country of its area's subdivision
// code ("BR-RJ" → "BR").
func (c *Client) Artist(ctx context.Context, id string) (Artist, error) {
	if !ValidID(id) {
		return Artist{}, ErrInvalidID
	}
	var r struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		SortName string `json:"sort-name"`
		Type     string `json:"type"`
		Country  string `json:"country"`
		Area     *struct {
			Name string   `json:"name"`
			ISO1 []string `json:"iso-3166-1-codes"`
			ISO2 []string `json:"iso-3166-2-codes"`
		} `json:"area"`
		LifeSpan struct {
			Begin string `json:"begin"`
			End   string `json:"end"`
		} `json:"life-span"`
	}
	if err := c.get(ctx, "/artist/"+id, url.Values{}, &r); err != nil {
		return Artist{}, err
	}
	a := Artist{ID: r.ID, Name: r.Name, SortName: r.SortName, Type: r.Type, Country: r.Country,
		Begin: r.LifeSpan.Begin, End: r.LifeSpan.End}
	if r.Area != nil {
		a.Area = r.Area.Name
		switch {
		case a.Country != "":
		case len(r.Area.ISO1) > 0:
			a.Country = r.Area.ISO1[0]
		case len(r.Area.ISO2) > 0 && len(r.Area.ISO2[0]) >= 2:
			a.Country = r.Area.ISO2[0][:2]
		}
	}
	return a, nil
}

// Cover returns the front cover (500 px) of a release or release group
// (kind "release" or "release-group") from the Cover Art Archive;
// ErrNotFound when it has none.
func (c *Client) Cover(ctx context.Context, kind, id string) ([]byte, error) {
	if kind != "release" && kind != "release-group" {
		return nil, fmt.Errorf("musicbrainz: carátula de %q", kind)
	}
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	b, err := c.Covers.Get(ctx, c.CoverBase+"/"+kind+"/"+id+"/front-500")
	return b, notFound(err)
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/musicbrainz/ -v`
Expected: PASS.

- [ ] **Step 6: Compare with one live response (manual, needs network, not part of the suite)**

```bash
curl -s -A 'Tordo/0.2 ( https://github.com/martjanz/tordo )' \
  'https://musicbrainz.org/ws/2/release/?query=release:(signos)%20AND%20artist:(soda%20stereo)&limit=2&fmt=json' | python3 -m json.tool | head -60
```

Check that the field names used by `searchResponse` (`releases`, `id`, `score`, `title`, `artist-credit`, `date`, `country`, `track-count`) are there. Then look up one id from that answer with `…/ws/2/release/<id>?inc=recordings+artist-credits+release-groups+url-rels+genres&fmt=json` and check `media[].tracks[].length`, `release-group.first-release-date`, `relations[].url.resource`. If a name differs, fix the struct tag **and** the fixture, and say so in the commit message.

- [ ] **Step 7: Commit**

```bash
git add internal/musicbrainz
git commit -m "feat: MusicBrainz and Cover Art Archive client"
```

---
### Task 9: Discogs client

**Files:**
- Create: `internal/discogs/discogs.go`, `internal/discogs/discogs_test.go`
- Create: `internal/discogs/testdata/release-1234.json`, `master-5678.json`, `search-signos.json`

**Interfaces:**
- Consumes: `httpx`.
- Produces: `discogs.BaseURL`, `ErrNotFound`, `ErrUnauthorized`, `ErrNoToken`; `discogs.Client{Base, Token string; HTTP *httpx.Client}`, `discogs.New(token string) *Client`, `(*Client).HasToken() bool`; `discogs.Info{ID, MasterID int; Genres, Styles []string}`; `(*Client).Release(ctx, id int) (Info, error)`, `(*Client).Master(ctx, id int) (Info, error)` (its `ID` is 0, `MasterID` the master); `discogs.Result{ID, MasterID int; Artist, Title string; Year int; Genres, Styles []string}`, `(*Client).Search(ctx, artist, title string) ([]Result, error)`; `discogs.ParseURL(u string) (kind string, id int, ok bool)`.

Discogs allows 25 requests/min without a token and 60 with one (tordo uses 55). Release and master lookups work without a token; `/database/search` needs one (`Authorization: Discogs token=…`), so `Search` returns `ErrNoToken` without making a request. Search results already carry `genre` and `style`, so a search costs one request.

- [ ] **Step 1: Write the fixtures**

`internal/discogs/testdata/release-1234.json`:

```json
{"id": 1234, "title": "Signos", "master_id": 5678, "year": 1986, "country": "Argentina",
 "genres": ["Rock"], "styles": ["Pop Rock", "New Wave"]}
```

`internal/discogs/testdata/master-5678.json`:

```json
{"id": 5678, "main_release": 1234, "title": "Signos", "year": 1986, "genres": ["Rock"], "styles": ["Pop Rock"]}
```

`internal/discogs/testdata/search-signos.json`:

```json
{"pagination": {"page": 1, "pages": 1, "per_page": 5, "items": 1},
 "results": [{"id": 1234, "type": "release", "master_id": 5678, "title": "Soda Stereo - Signos", "year": "1986",
              "genre": ["Rock"], "style": ["Pop Rock", "New Wave"]}]}
```

- [ ] **Step 2: Write the failing tests**

`internal/discogs/discogs_test.go`:

```go
package discogs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"tordo/internal/httpx"
)

func newTest(t *testing.T, token string) (*Client, *[]*http.Request) {
	t.Helper()
	var reqs []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqs = append(reqs, r)
		if r.Header.Get("Authorization") == "Discogs token=malo" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		files := map[string]string{"/releases/1234": "release-1234.json", "/masters/5678": "master-5678.json",
			"/database/search": "search-signos.json"}
		name, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		b, _ := os.ReadFile("testdata/" + name)
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	c := New(token)
	c.Base = srv.URL
	c.HTTP.Limiter, c.HTTP.Backoff = nil, time.Millisecond
	return c, &reqs
}

func TestReleaseAndMaster(t *testing.T) {
	c, reqs := newTest(t, "")
	info, err := c.Release(context.Background(), 1234)
	if err != nil || !reflect.DeepEqual(info, Info{ID: 1234, MasterID: 5678, Genres: []string{"Rock"}, Styles: []string{"Pop Rock", "New Wave"}}) {
		t.Fatalf("release = %+v, err %v", info, err)
	}
	if r := (*reqs)[0]; r.Header.Get("User-Agent") != httpx.UserAgent || r.Header.Get("Authorization") != "" {
		t.Fatalf("headers = %v", r.Header)
	}
	m, err := c.Master(context.Background(), 5678)
	if err != nil || !reflect.DeepEqual(m, Info{MasterID: 5678, Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}) {
		t.Fatalf("master = %+v, err %v", m, err)
	}
	if _, err := c.Release(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSearch(t *testing.T) {
	c, reqs := newTest(t, "secreto")
	rs, err := c.Search(context.Background(), "Soda Stereo", "Signos")
	want := []Result{{ID: 1234, MasterID: 5678, Artist: "Soda Stereo", Title: "Signos", Year: 1986,
		Genres: []string{"Rock"}, Styles: []string{"Pop Rock", "New Wave"}}}
	if err != nil || !reflect.DeepEqual(rs, want) {
		t.Fatalf("results = %+v, err %v", rs, err)
	}
	r := (*reqs)[0]
	q := r.URL.Query()
	if r.Header.Get("Authorization") != "Discogs token=secreto" || q.Get("type") != "release" ||
		q.Get("artist") != "Soda Stereo" || q.Get("release_title") != "Signos" {
		t.Fatalf("request %s, headers %v", r.URL, r.Header)
	}
}

func TestSearchNeedsToken(t *testing.T) {
	c, reqs := newTest(t, "")
	if c.HasToken() {
		t.Fatal("HasToken without a token")
	}
	if _, err := c.Search(context.Background(), "Soda Stereo", "Signos"); !errors.Is(err, ErrNoToken) || len(*reqs) != 0 {
		t.Fatalf("err = %v, requests %d", err, len(*reqs))
	}
}

func TestBadToken(t *testing.T) {
	c, _ := newTest(t, "malo")
	if _, err := c.Search(context.Background(), "Soda Stereo", "Signos"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestNewRates(t *testing.T) {
	if r := New("").HTTP.Limiter.Rate; r != 25.0/60 {
		t.Fatalf("anonymous rate = %v", r)
	}
	if r := New("x").HTTP.Limiter.Rate; r != 55.0/60 {
		t.Fatalf("token rate = %v", r)
	}
}

func TestParseURL(t *testing.T) {
	cases := []struct {
		in   string
		kind string
		id   int
		ok   bool
	}{
		{"https://www.discogs.com/release/1234", "release", 1234, true},
		{"https://www.discogs.com/release/1234-Soda-Stereo-Signos", "release", 1234, true},
		{"https://www.discogs.com/Soda-Stereo-Signos/master/5678", "master", 5678, true},
		{"https://www.discogs.com/es/master/5678-Signos", "master", 5678, true},
		{"https://www.discogs.com/artist/99-Soda-Stereo", "", 0, false},
		{"https://example.com/release/1", "", 0, false},
	}
	for _, c := range cases {
		kind, id, ok := ParseURL(c.in)
		if kind != c.kind || id != c.id || ok != c.ok {
			t.Errorf("ParseURL(%q) = %q %d %v", c.in, kind, id, ok)
		}
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/discogs/`
Expected: FAIL (package does not compile).

- [ ] **Step 4: Implement**

`internal/discogs/discogs.go`:

```go
// Package discogs is a small client for the Discogs API: genres and styles
// of releases and masters, and release search (which needs a token).
package discogs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"tordo/internal/httpx"
)

const BaseURL = "https://api.discogs.com"

var (
	ErrNotFound     = errors.New("no existe en Discogs")
	ErrUnauthorized = errors.New("Discogs rechazó el token")
	ErrNoToken      = errors.New("sin token de Discogs")
)

type Client struct {
	Base  string
	Token string
	HTTP  *httpx.Client
}

// New returns a client for the real API: 25 requests per minute without a
// token, 55 with one (Discogs allows 60).
func New(token string) *Client {
	h := http.Header{"User-Agent": {httpx.UserAgent}}
	rate := 25.0 / 60
	if token != "" {
		h.Set("Authorization", "Discogs token="+token)
		rate = 55.0 / 60
	}
	return &Client{Base: BaseURL, Token: token, HTTP: &httpx.Client{Limiter: &httpx.Limiter{Rate: rate, Burst: 1}, Header: h}}
}

func (c *Client) HasToken() bool { return c.Token != "" }

// Info is the genres and styles of a release or a master.
type Info struct {
	ID       int // the release (0 for a master)
	MasterID int
	Genres   []string
	Styles   []string
}

type entity struct {
	ID       int      `json:"id"`
	MasterID int      `json:"master_id"`
	Genres   []string `json:"genres"`
	Styles   []string `json:"styles"`
}

func (c *Client) Release(ctx context.Context, id int) (Info, error) {
	var e entity
	if err := c.get(ctx, fmt.Sprintf("/releases/%d", id), &e); err != nil {
		return Info{}, err
	}
	return Info{ID: e.ID, MasterID: e.MasterID, Genres: e.Genres, Styles: e.Styles}, nil
}

func (c *Client) Master(ctx context.Context, id int) (Info, error) {
	var e entity
	if err := c.get(ctx, fmt.Sprintf("/masters/%d", id), &e); err != nil {
		return Info{}, err
	}
	return Info{MasterID: e.ID, Genres: e.Genres, Styles: e.Styles}, nil
}

// Result is a release found by a search.
type Result struct {
	ID       int
	MasterID int
	Artist   string
	Title    string
	Year     int
	Genres   []string
	Styles   []string
}

// Search finds up to 5 releases by artist and title. It needs a token.
func (c *Client) Search(ctx context.Context, artist, title string) ([]Result, error) {
	if c.Token == "" {
		return nil, ErrNoToken
	}
	q := url.Values{"type": {"release"}, "artist": {artist}, "release_title": {title}, "per_page": {"5"}}
	var resp struct {
		Results []struct {
			ID       int      `json:"id"`
			MasterID int      `json:"master_id"`
			Title    string   `json:"title"` // "Artist - Title"
			Year     string   `json:"year"`
			Genre    []string `json:"genre"`
			Style    []string `json:"style"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/database/search?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	out := []Result{}
	for _, r := range resp.Results {
		a, t, ok := strings.Cut(r.Title, " - ")
		if !ok {
			a, t = "", r.Title
		}
		y, _ := strconv.Atoi(r.Year)
		out = append(out, Result{ID: r.ID, MasterID: r.MasterID, Artist: a, Title: t, Year: y, Genres: r.Genre, Styles: r.Style})
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, v any) error {
	body, err := c.HTTP.Get(ctx, c.Base+path)
	var se *httpx.StatusError
	if errors.As(err, &se) {
		switch se.Code {
		case http.StatusNotFound:
			return ErrNotFound
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrUnauthorized
		}
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

var reURL = regexp.MustCompile(`discogs\.com/(?:[^/]+/)*(release|master)/(\d+)`)

// ParseURL reads a discogs.com link to a release or a master.
func ParseURL(u string) (kind string, id int, ok bool) {
	m := reURL.FindStringSubmatch(u)
	if m == nil {
		return "", 0, false
	}
	id, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return m[1], id, true
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/discogs/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/discogs
git commit -m "feat: Discogs client for genres, styles and release search"
```

---
### Task 10: Identification queries and the confidence score

**Files:**
- Create: `internal/identify/query.go`, `internal/identify/score.go`
- Test: `internal/identify/query_test.go`, `internal/identify/score_test.go`

**Interfaces:**
- Consumes: `store.IdentifyTarget`, `store.TargetTrack` (Task 5); `nameparse.ParseFolder`, `Norm`, `StripDisc`, `IsVarious`, `CleanTag`, `Compose`; `musicbrainz.ValidID`, `ReleaseHit`, `Release` (Task 8).
- Produces: `identify.MatcherVersion = 1`; `identify.Variant{Title, Artist string; Various bool}`; `identify.Query{Variants []Variant; Year int; Durations []int64; ReleaseID string}`; `identify.BuildQuery(store.IdentifyTarget) Query`; `(Query).Key() string`; `identify.Threshold = 0.80`; unexported `similarity`, `titleScore`, `artistScore`, `yearScore`, `tracksScore`, `durationsScore`, `hitScore`, `releaseScore`, `stripTitle`, `absDiff`, `durationSlackMs`, `durationsWeight`.

The score (spec §5 step 6) weighs title 0.30, artist 0.20, year 0.10 (exact or ±1), track count 0.15 and track durations 0.25 (a track matches within 3 s). A search hit has no durations, so `hitScore` is the score without them and `hitScore + durationsWeight` is the best the hit could reach: only hits whose bound reaches the threshold are looked up. Variants, in order: the album as grouped, the album folder's name (it fixes junk or crossed tags: *Raras Partituras*, `FITO PAEZ - 2004 - …`), and both without disc markers or bracketed notes. Compilations search Various Artists. `Key` identifies the query: same key, same stored match, so an album that moves keeps its match; any change of title, artist, year or track durations makes a new key and a new lookup.

- [ ] **Step 1: Write the failing tests**

`internal/identify/query_test.go`:

```go
package identify

import (
	"reflect"
	"testing"

	"tordo/internal/store"
)

const validMBID = "a1b2c3d4-0000-4000-8000-000000000001"

func TestBuildQueryPlainAlbum(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Soda Stereo/Soda Stereo - (1986) Signos", Title: "Signos",
		Artist: "Soda Stereo", Year: 1986, Tracks: []store.TargetTrack{{DurationMs: 231500}, {DurationMs: 209000}}})
	want := Query{Variants: []Variant{{Title: "Signos", Artist: "Soda Stereo"}}, Year: 1986, Durations: []int64{231500, 209000}}
	if !reflect.DeepEqual(q, want) {
		t.Fatalf("query = %+v", q)
	}
}

func TestBuildQueryFolderVariant(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Fito Páez/Fito Páez - (2004) Mi vida con ellas",
		Title: "FITO PAEZ - 2004 - mi vida con ellas 1", Artist: "Fito Páez"})
	want := []Variant{{Title: "FITO PAEZ - 2004 - mi vida con ellas 1", Artist: "Fito Páez"}, {Title: "Mi vida con ellas", Artist: "Fito Páez"}}
	if !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryStripsNotes(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Soda Stereo/Signos remaster", Title: "Signos (Remastered 2007)", Artist: "Soda Stereo"})
	want := []Variant{{Title: "Signos (Remastered 2007)", Artist: "Soda Stereo"}, {Title: "Signos remaster", Artist: "Soda Stereo"},
		{Title: "Signos", Artist: "Soda Stereo"}}
	if !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryCompilation(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/Brasil/VA - História da MPB", Title: "História da MPB",
		Artist: "Varios artistas", Compilation: true})
	if want := []Variant{{Title: "História da MPB", Various: true}}; !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryComposesAndDropsJunk(t *testing.T) {
	q := BuildQuery(store.IdentifyTarget{Dir: "../musica/x/Império", Title: "Império", Artist: "www.lomusical.com"})
	if want := []Variant{{Title: "Império"}}; !reflect.DeepEqual(q.Variants, want) {
		t.Fatalf("variants = %+v", q.Variants)
	}
}

func TestBuildQueryTagReleaseID(t *testing.T) {
	tracks := []store.TargetTrack{{MBAlbumID: validMBID}, {MBAlbumID: validMBID}, {MBAlbumID: "basura"}}
	if q := BuildQuery(store.IdentifyTarget{Dir: "../musica/a/b", Title: "b", Tracks: tracks}); q.ReleaseID != validMBID {
		t.Fatalf("release id = %q", q.ReleaseID)
	}
	tracks[1].MBAlbumID = ""
	if q := BuildQuery(store.IdentifyTarget{Dir: "../musica/a/b", Title: "b", Tracks: tracks}); q.ReleaseID != "" {
		t.Fatalf("a minority id was used: %q", q.ReleaseID)
	}
}

func TestQueryKey(t *testing.T) {
	base := store.IdentifyTarget{Dir: "../musica/Soda Stereo/Signos", Title: "Signos", Artist: "Soda Stereo", Year: 1986,
		Tracks: []store.TargetTrack{{DurationMs: 231500}, {DurationMs: 209000}}}
	moved := base
	moved.Dir = "../musica/Ordenar/Signos"
	if BuildQuery(base).Key() != BuildQuery(moved).Key() {
		t.Fatal("moving the folder changed the key")
	}
	changed := base
	changed.Tracks = []store.TargetTrack{{DurationMs: 231500}, {DurationMs: 250000}}
	if BuildQuery(base).Key() == BuildQuery(changed).Key() {
		t.Fatal("other durations kept the key")
	}
}
```

`internal/identify/score_test.go`:

```go
package identify

import (
	"math"
	"testing"

	"tordo/internal/musicbrainz"
)

func TestSimilarity(t *testing.T) {
	if similarity("Fito Páez", "fito paez!") != 1 || similarity("", "x") != 0 {
		t.Fatal("similarity of equal or empty names")
	}
	if s := similarity("Sin sobresalt", "Sin sobresaltos"); s < 0.8 || s >= 1 {
		t.Fatalf("truncated title similarity = %v", s)
	}
}

func TestPartialScores(t *testing.T) {
	cases := []struct {
		name      string
		got, want float64
	}{
		{"year unknown", yearScore(0, 1986), 0.5},
		{"year exact", yearScore(1986, 1986), 1},
		{"year off by one", yearScore(1986, 1987), 0.5},
		{"year from original", yearScore(1986, 2001, 1986), 1},
		{"year wrong", yearScore(1986, 2001), 0},
		{"tracks equal", tracksScore(8, 8), 1},
		{"tracks partial", tracksScore(6, 12), 0.25},
		{"tracks more", tracksScore(12, 8), 0},
		{"durations positional", durationsScore([]int64{231500, 209000}, []int64{231000, 210000}), 1},
		{"durations half", durationsScore([]int64{231500, 100000}, []int64{231000, 210000}), 0.5},
		{"durations subset", durationsScore([]int64{210000}, []int64{231000, 210000}), 1},
		{"durations unknown", durationsScore([]int64{0, 0}, []int64{231000, 210000}), 0.5},
		{"artist various", artistScore(Variant{Various: true}, true, "Various Artists", nil), 1},
		{"artist various mismatch", artistScore(Variant{Artist: "Billy Blanco"}, true, "Various Artists", nil), 0.3},
		{"artist unknown", artistScore(Variant{}, false, "Soda Stereo", nil), 0.5},
		{"artist in credit", artistScore(Variant{Artist: "Noel Rosa"}, false, "Noel Rosa & Aracy de Almeida",
			[]string{"Noel Rosa", "Aracy de Almeida"}), 1},
		{"title without notes", titleScore(Variant{Title: "Signos (Remastered)"}, "Signos"), 1},
	}
	for _, c := range cases {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestReleaseScore(t *testing.T) {
	q := Query{Year: 1986, Durations: []int64{231500, 209000}}
	v := Variant{Title: "Signos", Artist: "Soda Stereo"}
	rel := musicbrainz.Release{Title: "Signos", Artist: "Soda Stereo", ArtistNames: []string{"Soda Stereo"}, Year: 1986,
		Tracks: []musicbrainz.Track{{LengthMs: 231000}, {LengthMs: 210000}}}
	if s := releaseScore(v, q, rel); math.Abs(s-1) > 1e-9 {
		t.Fatalf("perfect release scores %v", s)
	}
	hit := musicbrainz.ReleaseHit{Title: "Signos", Artist: "Soda Stereo", Year: 1986, Tracks: 2}
	if s := hitScore(v, q, hit); math.Abs(s+durationsWeight-1) > 1e-9 {
		t.Fatalf("perfect hit scores %v", s)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/identify/`
Expected: FAIL (package does not compile).

- [ ] **Step 3: Implement `internal/identify/query.go`**

```go
// Package identify identifies albums on MusicBrainz, enriches them
// (MusicBrainz, Discogs, Cover Art Archive) and applies the result, the
// user's corrections first, to the catalog.
package identify

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"tordo/internal/musicbrainz"
	"tordo/internal/nameparse"
	"tordo/internal/store"
)

// MatcherVersion changes when matching changes: queries matched by an older
// version are matched again.
const MatcherVersion = 1

// Variant is one way of searching an album.
type Variant struct {
	Title   string
	Artist  string // "" when Various or unknown
	Various bool
}

// Query describes an album for the matcher.
type Query struct {
	Variants  []Variant // searched in order
	Year      int
	Durations []int64 // the album's tracks in disc/track order, ms
	ReleaseID string  // the tags' MusicBrainz album id, when most tracks agree on one
}

// BuildQuery describes an album: as grouped, as its folder names it, and
// both without disc markers or bracketed notes. Compilations search Various
// Artists.
func BuildQuery(t store.IdentifyTarget) Query {
	q := Query{Year: t.Year, ReleaseID: commonReleaseID(t.Tracks)}
	for _, tr := range t.Tracks {
		q.Durations = append(q.Durations, tr.DurationMs)
	}
	various := t.Compilation || nameparse.IsVarious(t.Artist)
	artist := clean(t.Artist)
	if various {
		artist = ""
	}
	f := nameparse.ParseFolder(path.Base(t.Dir))
	fVarious := various || f.Various
	fArtist := clean(f.Artist)
	if fArtist == "" {
		fArtist = artist
	}
	if fVarious {
		fArtist = ""
	}
	seen := map[string]bool{}
	add := func(title, artist string, various bool) {
		title = clean(title)
		k := fmt.Sprintf("%s\x00%s\x00%t", nameparse.Norm(title), nameparse.Norm(artist), various)
		if nameparse.Norm(title) == "" || seen[k] {
			return
		}
		seen[k] = true
		q.Variants = append(q.Variants, Variant{Title: title, Artist: artist, Various: various})
	}
	add(t.Title, artist, various)
	add(f.Album, fArtist, fVarious)
	add(stripTitle(clean(t.Title)), artist, various)
	add(stripTitle(clean(f.Album)), fArtist, fVarious)
	return q
}

// Key identifies the query: a stored match is reused while the key stays.
func (q Query) Key() string {
	var b strings.Builder
	for _, v := range q.Variants {
		fmt.Fprintf(&b, "%s/%s/%t;", nameparse.Norm(v.Title), nameparse.Norm(v.Artist), v.Various)
	}
	fmt.Fprintf(&b, "|%d|%s|", q.Year, q.ReleaseID)
	for i, d := range q.Durations {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%d", (d+500)/1000)
	}
	return b.String()
}

// clean composes a name and drops it when it is a download site or a code.
func clean(s string) string { return nameparse.CleanTag(nameparse.Compose(s)) }

var reNotes = regexp.MustCompile(`\s*[(\[][^)\]]*[)\]]`)

// stripTitle drops disc markers and bracketed notes: "Signos (Remastered)".
func stripTitle(s string) string {
	s, _ = nameparse.StripDisc(s)
	return strings.TrimSpace(reNotes.ReplaceAllString(s, ""))
}

// commonReleaseID is the MusicBrainz album id most of the tracks carry.
func commonReleaseID(ts []store.TargetTrack) string {
	n := map[string]int{}
	for _, t := range ts {
		if musicbrainz.ValidID(t.MBAlbumID) {
			n[t.MBAlbumID]++
		}
	}
	for id, c := range n {
		if c*2 > len(ts) {
			return id
		}
	}
	return ""
}
```

- [ ] **Step 4: Implement `internal/identify/score.go`**

```go
package identify

import (
	"tordo/internal/musicbrainz"
	"tordo/internal/nameparse"
)

// Threshold is the confidence above which an album is identified
// automatically; below it, the album goes to Sin identificar.
const Threshold = 0.80

// Weights of the confidence score (they add up to 1).
const (
	titleWeight     = 0.30
	artistWeight    = 0.20
	yearWeight      = 0.10
	tracksWeight    = 0.15
	durationsWeight = 0.25
)

// durationSlackMs is how far apart two durations of the same track may be.
const durationSlackMs = 3000

// similarity compares two names after Norm: 1 − edit distance / longer
// length. Empty names are not similar to anything.
func similarity(a, b string) float64 {
	ra, rb := []rune(nameparse.Norm(a)), []rune(nameparse.Norm(b))
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	return 1 - float64(levenshtein(ra, rb))/float64(max(len(ra), len(rb)))
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func titleScore(v Variant, title string) float64 {
	return max(similarity(v.Title, title), similarity(stripTitle(v.Title), stripTitle(title)))
}

// artistScore compares the album's artist with the release's credit and
// each credited name. A compilation against a single-artist release (or the
// reverse) is unlikely but possible: a composer's songbook credited to him.
func artistScore(v Variant, various bool, credit string, names []string) float64 {
	switch {
	case v.Various && various:
		return 1
	case v.Various != various:
		return 0.3
	case v.Artist == "":
		return 0.5
	}
	best := similarity(v.Artist, credit)
	for _, n := range names {
		best = max(best, similarity(v.Artist, n))
	}
	return best
}

// yearScore rewards the exact year (of the release or its first release)
// and half of it for ±1. Without a year it gives a neutral half.
func yearScore(year int, candidates ...int) float64 {
	if year == 0 {
		return 0.5
	}
	best := 0.0
	for _, y := range candidates {
		switch {
		case y == year:
			return 1
		case y != 0 && (y == year-1 || y == year+1):
			best = 0.5
		}
	}
	return best
}

// tracksScore: the same count is best; an album missing tracks gets a
// share; more tracks than the release has, nothing.
func tracksScore(n, m int) float64 {
	switch {
	case n == m:
		return 1
	case n < m:
		return 0.5 * float64(n) / float64(m)
	}
	return 0
}

// durationsScore is the share of the album's tracks (with a known
// duration) that match a release track: position by position when both
// have as many tracks, else any unused track. Unknown durations give a
// neutral half.
func durationsScore(album, release []int64) float64 {
	known := 0
	for _, d := range album {
		if d > 0 {
			known++
		}
	}
	if known == 0 || len(release) == 0 {
		return 0.5
	}
	matched := 0
	if len(album) == len(release) {
		for i, d := range album {
			if near(d, release[i]) {
				matched++
			}
		}
	} else {
		used := make([]bool, len(release))
		for _, d := range album {
			for j, r := range release {
				if !used[j] && near(d, r) {
					used[j] = true
					matched++
					break
				}
			}
		}
	}
	return float64(matched) / float64(known)
}

func near(a, b int64) bool { return a > 0 && b > 0 && absDiff(a, b) <= durationSlackMs }

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

// hitScore is a search hit's score without durations (a hit has none):
// hitScore + durationsWeight is the most it can reach once looked up.
func hitScore(v Variant, q Query, h musicbrainz.ReleaseHit) float64 {
	return titleWeight*titleScore(v, h.Title) + artistWeight*artistScore(v, h.Various, h.Artist, h.ArtistNames) +
		yearWeight*yearScore(q.Year, h.Year) + tracksWeight*tracksScore(len(q.Durations), h.Tracks)
}

// releaseScore is a looked-up release's confidence score.
func releaseScore(v Variant, q Query, rel musicbrainz.Release) float64 {
	lengths := make([]int64, len(rel.Tracks))
	for i, t := range rel.Tracks {
		lengths[i] = t.LengthMs
	}
	return titleWeight*titleScore(v, rel.Title) + artistWeight*artistScore(v, rel.Various, rel.Artist, rel.ArtistNames) +
		yearWeight*yearScore(q.Year, rel.Year, rel.OriginalYear) + tracksWeight*tracksScore(len(q.Durations), len(rel.Tracks)) +
		durationsWeight*durationsScore(q.Durations, lengths)
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/identify/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/identify
git commit -m "feat: identification queries and confidence score"
```

---
### Task 11: Matching an album against MusicBrainz

**Files:**
- Create: `internal/identify/match.go`, `internal/identify/fake_test.go`
- Test: `internal/identify/match_test.go`

**Interfaces:**
- Consumes: Task 10; `musicbrainz` types; `httpx.ErrOffline`; `store.Match`, `store.Candidate`.
- Produces: `identify.API` (interface below); `identify.Identify(ctx, api API, q Query, fetched map[string]musicbrainz.Release) (store.Match, error)`; unexported `lookup`, `fatal`. Test helpers in `fake_test.go` (`fakeMB`, `fakeDiscogs`, `signosRelease`, `signosHit`, ids) are shared by Tasks 12–13.

```go
type API interface {
	SearchReleases(ctx context.Context, q musicbrainz.SearchQuery) ([]musicbrainz.ReleaseHit, error)
	Release(ctx context.Context, id string) (musicbrainz.Release, error)
	ReleaseGroupLinks(ctx context.Context, id string) (musicbrainz.GroupLinks, error)
	Artist(ctx context.Context, id string) (musicbrainz.Artist, error)
	Cover(ctx context.Context, kind, id string) ([]byte, error)
}
```

`Identify`: a release id in the tags is trusted (confidence 1) if MusicBrainz has it. Otherwise each variant is searched in turn; hits are ranked by `hitScore`, and those that could still reach the threshold are looked up (at most `maxLookups` = 4 lookups per album, reused from `fetched` when already looked up in this run) until one scores ≥ `Threshold`. Every hit becomes a candidate (looked-up ones with their full score, the others with the score without durations); the best 5 are kept. Only fatal errors (offline, cancelled) are returned: a rejected query just leaves the album unmatched.

Requests per identified album: usually 1 search + 1 lookup; the lookup is reused by enrichment. At 1 req/s, the ~460 albums of the sample collection take ~15 min; 10k albums, a few hours (spec §5).

- [ ] **Step 1: Write the fakes**

`internal/identify/fake_test.go`:

```go
package identify

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"tordo/internal/discogs"
	"tordo/internal/musicbrainz"
)

const (
	signosID    = "a1b2c3d4-0000-4000-8000-000000000001"
	signosGroup = "a1b2c3d4-0000-4000-8000-000000000002"
	sodaID      = "a1b2c3d4-0000-4000-8000-000000000003"
	otherID     = "a1b2c3d4-0000-4000-8000-000000000004"
)

func signosRelease() musicbrainz.Release {
	return musicbrainz.Release{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistIDs: []string{sodaID},
		ArtistNames: []string{"Soda Stereo"}, Year: 1986, Country: "AR", GroupID: signosGroup, OriginalYear: 1986,
		Tracks: []musicbrainz.Track{
			{Disc: 1, Position: 1, Title: "Táctica y estrategia", LengthMs: 231000, RecordingID: "rec-1"},
			{Disc: 1, Position: 2, Title: "Sin sobresaltos", LengthMs: 210000, RecordingID: "rec-2"},
		},
		Genres: []string{"rock"}, DiscogsURLs: []string{"https://www.discogs.com/release/1234-Soda-Stereo-Signos"}}
}

func signosHit() musicbrainz.ReleaseHit {
	return musicbrainz.ReleaseHit{ID: signosID, Title: "Signos", Artist: "Soda Stereo", ArtistNames: []string{"Soda Stereo"},
		Year: 1986, Country: "AR", Tracks: 2, Score: 100}
}

// otherHit looks like Signos by title only.
func otherHit() musicbrainz.ReleaseHit {
	return musicbrainz.ReleaseHit{ID: otherID, Title: "Signos", Artist: "Los Signos", ArtistNames: []string{"Los Signos"},
		Year: 2001, Tracks: 12, Score: 80}
}

// fakeMB answers from maps and records every call. Search keys are
// "title|artist|various".
type fakeMB struct {
	mu       sync.Mutex
	search   map[string][]musicbrainz.ReleaseHit
	releases map[string]musicbrainz.Release
	groups   map[string]musicbrainz.GroupLinks
	artists  map[string]musicbrainz.Artist
	covers   map[string][]byte // "kind/id"
	err      error             // returned by every call when set
	calls    []string
}

func (f *fakeMB) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeMB) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeMB) SearchReleases(ctx context.Context, q musicbrainz.SearchQuery) ([]musicbrainz.ReleaseHit, error) {
	key := fmt.Sprintf("%s|%s|%t", q.Title, q.Artist, q.Various)
	if err := f.record("search " + key); err != nil {
		return nil, err
	}
	return f.search[key], nil
}

func (f *fakeMB) Release(ctx context.Context, id string) (musicbrainz.Release, error) {
	if err := f.record("release " + id); err != nil {
		return musicbrainz.Release{}, err
	}
	r, ok := f.releases[id]
	if !ok {
		return r, musicbrainz.ErrNotFound
	}
	return r, nil
}

func (f *fakeMB) ReleaseGroupLinks(ctx context.Context, id string) (musicbrainz.GroupLinks, error) {
	if err := f.record("group " + id); err != nil {
		return musicbrainz.GroupLinks{}, err
	}
	return f.groups[id], nil
}

func (f *fakeMB) Artist(ctx context.Context, id string) (musicbrainz.Artist, error) {
	if err := f.record("artist " + id); err != nil {
		return musicbrainz.Artist{}, err
	}
	a, ok := f.artists[id]
	if !ok {
		return a, musicbrainz.ErrNotFound
	}
	return a, nil
}

func (f *fakeMB) Cover(ctx context.Context, kind, id string) ([]byte, error) {
	if err := f.record("cover " + kind + "/" + id); err != nil {
		return nil, err
	}
	b, ok := f.covers[kind+"/"+id]
	if !ok {
		return nil, musicbrainz.ErrNotFound
	}
	return b, nil
}

// fakeDiscogs answers from maps. Search keys are "artist|title".
type fakeDiscogs struct {
	mu       sync.Mutex
	token    bool
	releases map[int]discogs.Info
	masters  map[int]discogs.Info
	search   map[string][]discogs.Result
	err      error
	calls    []string
}

func (f *fakeDiscogs) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	return f.err
}

func (f *fakeDiscogs) HasToken() bool { return f.token }

func (f *fakeDiscogs) Release(ctx context.Context, id int) (discogs.Info, error) {
	if err := f.record(fmt.Sprintf("release %d", id)); err != nil {
		return discogs.Info{}, err
	}
	i, ok := f.releases[id]
	if !ok {
		return i, discogs.ErrNotFound
	}
	return i, nil
}

func (f *fakeDiscogs) Master(ctx context.Context, id int) (discogs.Info, error) {
	if err := f.record(fmt.Sprintf("master %d", id)); err != nil {
		return discogs.Info{}, err
	}
	i, ok := f.masters[id]
	if !ok {
		return i, discogs.ErrNotFound
	}
	return i, nil
}

func (f *fakeDiscogs) Search(ctx context.Context, artist, title string) ([]discogs.Result, error) {
	if !f.token {
		return nil, discogs.ErrNoToken
	}
	if err := f.record("search " + artist + "|" + title); err != nil {
		return nil, err
	}
	return f.search[artist+"|"+title], nil
}
```

- [ ] **Step 2: Write the failing tests**

`internal/identify/match_test.go`:

```go
package identify

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

func signosQuery() Query {
	return Query{Variants: []Variant{{Title: "Signos", Artist: "Soda Stereo"}}, Year: 1986, Durations: []int64{231500, 209000}}
}

func TestIdentifyPicksMatchingRelease(t *testing.T) {
	mb := &fakeMB{
		search:   map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {otherHit(), signosHit()}},
		releases: map[string]musicbrainz.Release{signosID: signosRelease()},
	}
	fetched := map[string]musicbrainz.Release{}
	m, err := Identify(context.Background(), mb, signosQuery(), fetched)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != store.StatusAuto || m.ReleaseID != signosID || m.Confidence < Threshold || m.MatcherVersion != MatcherVersion ||
		m.Query != signosQuery().Key() {
		t.Fatalf("match = %+v", m)
	}
	if want := []string{"search Signos|Soda Stereo|false", "release " + signosID}; !reflect.DeepEqual(mb.calls, want) {
		t.Fatalf("calls = %q", mb.calls)
	}
	if _, ok := fetched[signosID]; !ok || len(m.Candidates) != 2 || m.Candidates[0].ReleaseID != signosID {
		t.Fatalf("fetched %v, candidates %+v", fetched, m.Candidates)
	}
}

func TestIdentifyBelowThresholdKeepsCandidates(t *testing.T) {
	mb := &fakeMB{search: map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {otherHit()}}}
	m, err := Identify(context.Background(), mb, signosQuery(), map[string]musicbrainz.Release{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != store.StatusUnmatched || m.ReleaseID != "" || len(m.Candidates) != 1 || m.Candidates[0].ReleaseID != otherID ||
		m.Confidence != m.Candidates[0].Score || mb.count("release ") != 0 {
		t.Fatalf("match = %+v, calls %q", m, mb.calls)
	}
}

func TestIdentifyTriesFolderVariant(t *testing.T) {
	const miVida = "a1b2c3d4-0000-4000-8000-000000000010"
	rel := musicbrainz.Release{ID: miVida, Title: "Mi vida con ellas", Artist: "Fito Páez", ArtistNames: []string{"Fito Páez"},
		Year: 2004, Tracks: []musicbrainz.Track{{Title: "Uno", LengthMs: 200000}}}
	mb := &fakeMB{
		search: map[string][]musicbrainz.ReleaseHit{"Mi vida con ellas|Fito Páez|false": {{ID: miVida, Title: "Mi vida con ellas",
			Artist: "Fito Páez", ArtistNames: []string{"Fito Páez"}, Year: 2004, Tracks: 1}}},
		releases: map[string]musicbrainz.Release{miVida: rel},
	}
	q := Query{Variants: []Variant{{Title: "FITO PAEZ - 2004 - mi vida con ellas 1", Artist: "Fito Páez"},
		{Title: "Mi vida con ellas", Artist: "Fito Páez"}}, Year: 2004, Durations: []int64{201000}}
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusAuto || m.ReleaseID != miVida {
		t.Fatalf("match = %+v, err %v, calls %q", m, err, mb.calls)
	}
}

func TestIdentifyTrustsTagReleaseID(t *testing.T) {
	mb := &fakeMB{releases: map[string]musicbrainz.Release{signosID: signosRelease()}}
	q := signosQuery()
	q.ReleaseID = signosID
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusAuto || m.Confidence != 1 || mb.count("search ") != 0 {
		t.Fatalf("match = %+v, err %v, calls %q", m, err, mb.calls)
	}
}

func TestIdentifyUnknownTagReleaseIDSearches(t *testing.T) {
	mb := &fakeMB{
		search:   map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {signosHit()}},
		releases: map[string]musicbrainz.Release{signosID: signosRelease()},
	}
	q := signosQuery()
	q.ReleaseID = otherID
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.ReleaseID != signosID {
		t.Fatalf("match = %+v, err %v", m, err)
	}
}

func TestIdentifyVariousArtists(t *testing.T) {
	const mpb = "a1b2c3d4-0000-4000-8000-000000000011"
	rel := musicbrainz.Release{ID: mpb, Title: "História da MPB", Artist: "Various Artists", Various: true,
		ArtistIDs: []string{musicbrainz.VariousArtistsID}, Tracks: []musicbrainz.Track{{LengthMs: 180000}}}
	mb := &fakeMB{
		search: map[string][]musicbrainz.ReleaseHit{"História da MPB||true": {{ID: mpb, Title: "História da MPB",
			Artist: "Various Artists", Various: true, Tracks: 1}}},
		releases: map[string]musicbrainz.Release{mpb: rel},
	}
	q := Query{Variants: []Variant{{Title: "História da MPB", Various: true}}, Durations: []int64{181000}}
	m, err := Identify(context.Background(), mb, q, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusAuto || m.ReleaseID != mpb {
		t.Fatalf("match = %+v, err %v", m, err)
	}
}

func TestIdentifyOfflineIsFatal(t *testing.T) {
	mb := &fakeMB{err: fmt.Errorf("%w: prueba", httpx.ErrOffline)}
	if _, err := Identify(context.Background(), mb, signosQuery(), map[string]musicbrainz.Release{}); !errors.Is(err, httpx.ErrOffline) {
		t.Fatalf("err = %v", err)
	}
}

func TestIdentifyRejectedQueryIsNotFatal(t *testing.T) {
	mb := &fakeMB{err: &httpx.StatusError{Code: 400, URL: "x"}}
	m, err := Identify(context.Background(), mb, signosQuery(), map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusUnmatched {
		t.Fatalf("match = %+v, err %v", m, err)
	}
}

func TestIdentifyWithoutVariants(t *testing.T) {
	mb := &fakeMB{}
	m, err := Identify(context.Background(), mb, Query{}, map[string]musicbrainz.Release{})
	if err != nil || m.Status != store.StatusUnmatched || len(mb.calls) != 0 {
		t.Fatalf("match = %+v, err %v, calls %q", m, err, mb.calls)
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/identify/`
Expected: FAIL (`undefined: Identify`).

- [ ] **Step 4: Implement `internal/identify/match.go`**

```go
package identify

import (
	"context"
	"errors"
	"log"
	"math"
	"sort"

	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// API is the part of the MusicBrainz client the identification uses.
type API interface {
	SearchReleases(ctx context.Context, q musicbrainz.SearchQuery) ([]musicbrainz.ReleaseHit, error)
	Release(ctx context.Context, id string) (musicbrainz.Release, error)
	ReleaseGroupLinks(ctx context.Context, id string) (musicbrainz.GroupLinks, error)
	Artist(ctx context.Context, id string) (musicbrainz.Artist, error)
	Cover(ctx context.Context, kind, id string) ([]byte, error)
}

// maxLookups is how many releases one album may look up (a request each).
const maxLookups = 4

// maxCandidates is how many candidates an unidentified album keeps.
const maxCandidates = 5

// Identify matches an album against MusicBrainz. A release id from the
// tags is trusted when MusicBrainz has it; otherwise each variant is
// searched in turn and the hits that could still reach the threshold are
// looked up (at most maxLookups per album) until one does. Releases it looks
// up are added to fetched. Only fatal errors (offline, cancelled) are
// returned: a rejected query leaves the album unmatched.
func Identify(ctx context.Context, api API, q Query, fetched map[string]musicbrainz.Release) (store.Match, error) {
	m := store.Match{Query: q.Key(), Status: store.StatusUnmatched, MatcherVersion: MatcherVersion}
	if q.ReleaseID != "" {
		rel, err := lookup(ctx, api, q.ReleaseID, fetched)
		if fatal(err) {
			return m, err
		}
		if err == nil {
			m.Status, m.ReleaseID, m.Confidence = store.StatusAuto, rel.ID, 1
			m.Candidates = []store.Candidate{candidateOf(rel, 1)}
			return m, nil
		}
	}
	best := map[string]store.Candidate{}
	keep := func(c store.Candidate) {
		if old, ok := best[c.ReleaseID]; !ok || c.Score > old.Score {
			best[c.ReleaseID] = c
		}
	}
	lookups := 0
variants:
	for _, v := range q.Variants {
		hits, err := api.SearchReleases(ctx, musicbrainz.SearchQuery{Title: v.Title, Artist: v.Artist, Various: v.Various})
		if fatal(err) {
			return m, err
		}
		if err != nil {
			log.Printf("búsqueda en MusicBrainz de %q: %v", v.Title, err)
			continue
		}
		scores := make(map[string]float64, len(hits))
		for _, h := range hits {
			scores[h.ID] = hitScore(v, q, h)
			keep(candidateOfHit(h, scores[h.ID]))
		}
		sort.SliceStable(hits, func(i, j int) bool { return scores[hits[i].ID] > scores[hits[j].ID] })
		for _, h := range hits {
			if scores[h.ID]+durationsWeight < Threshold {
				break
			}
			if _, ok := fetched[h.ID]; !ok {
				if lookups >= maxLookups {
					break
				}
				lookups++
			}
			rel, err := lookup(ctx, api, h.ID, fetched)
			if fatal(err) {
				return m, err
			}
			if err != nil {
				continue
			}
			s := releaseScore(v, q, rel)
			keep(candidateOf(rel, s))
			if s >= Threshold {
				m.Status, m.ReleaseID, m.Confidence = store.StatusAuto, rel.ID, round(s)
				break variants
			}
		}
	}
	m.Candidates = top(best)
	if m.Status != store.StatusAuto && len(m.Candidates) > 0 {
		m.Confidence = m.Candidates[0].Score
	}
	return m, nil
}

// lookup gets a release, from fetched when this run already looked it up.
func lookup(ctx context.Context, api API, id string, fetched map[string]musicbrainz.Release) (musicbrainz.Release, error) {
	if r, ok := fetched[id]; ok {
		return r, nil
	}
	r, err := api.Release(ctx, id)
	if err == nil {
		fetched[id] = r
	}
	return r, err
}

// fatal errors stop a run: no network, or cancellation. Other errors only
// skip the item at hand.
func fatal(err error) bool {
	return errors.Is(err, httpx.ErrOffline) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func candidateOfHit(h musicbrainz.ReleaseHit, score float64) store.Candidate {
	return store.Candidate{ReleaseID: h.ID, Title: h.Title, Artist: h.Artist, Year: h.Year, Country: h.Country,
		Tracks: h.Tracks, Score: round(score)}
}

func candidateOf(r musicbrainz.Release, score float64) store.Candidate {
	return store.Candidate{ReleaseID: r.ID, Title: r.Title, Artist: r.Artist, Year: r.Year, Country: r.Country,
		Tracks: len(r.Tracks), Score: round(score)}
}

func round(s float64) float64 { return math.Round(s*100) / 100 }

func top(best map[string]store.Candidate) []store.Candidate {
	out := make([]store.Candidate, 0, len(best))
	for _, c := range best {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ReleaseID < out[j].ReleaseID
	})
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/identify/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/identify
git commit -m "feat: match albums against MusicBrainz with query variants and a lookup budget"
```

---
### Task 12: Resolving an album's identity

**Files:**
- Create: `internal/identify/resolve.go`
- Test: `internal/identify/resolve_test.go`

**Interfaces:**
- Consumes: Tasks 5–7, 10, 11; `grouping.Various`.
- Produces (unexported, used by the runner): `world{matches; corrections; releases; artists; genres *genres.Normalizer}`, `resolve(t store.IdentifyTarget, q Query, w world, unmapped map[string]int) store.AlbumIdentity`, `correctionOf`, `releaseFrom(musicbrainz.Release) store.Release`, `mainArtist(store.Release) string`, `countryOf`, `alignTracks`, `albumGenres`.

`resolve` is pure and local: no requests, so it can run any time (after a scan, after a correction, offline). The rules:

1. **Corrections first.** If at least half of the album's tracks carry a correction, the most common one decides (`manual` → that release, confidence 1; `ignored`). Corrections are per fingerprint, so they follow the tracks through moves, renames and re-tagging, and an album rebuilt differently by the scan still finds them.
2. Otherwise the stored match for the album's query key: `auto` (its release) or `unmatched`. No match yet: status `""` (pending).
3. An unmatched **selection** is status `selection`: it stays out of *Sin identificar*.
4. With a stored release: MusicBrainz's title and artist credit (`Varios artistas` when credited to Various Artists, which also makes it a compilation), original year (else the release year), main artist id, country (the main artist's; for Various Artists the release country unless it is `XW`/`XE`/`XU`), the cached cover only when the album has no folder image or embedded picture, and track titles paired with the release's tracks (`alignTracks`).
5. Genres from the best source: Discogs (identified) → MusicBrainz genres (identified, through the normalizer, unknown ones dropped) → raw tags (through the normalizer; unknown raw values counted in `unmapped`: the *Géneros por mapear* queue lists only values that actually decide some album's genres).

- [ ] **Step 1: Write the failing tests**

`internal/identify/resolve_test.go`:

```go
package identify

import (
	"reflect"
	"testing"

	"tordo/internal/genres"
	"tordo/internal/grouping"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

const signosDir = "../musica/Soda Stereo/Soda Stereo - (1986) Signos"

func target() store.IdentifyTarget {
	return store.IdentifyTarget{AlbumID: 7, Dir: signosDir, Title: "Signos", Artist: "Soda Stereo", Year: 1986,
		RawGenres: []string{"Rock nacional", "Chamamé"},
		Tracks: []store.TargetTrack{
			{FileID: 1, Fingerprint: "fp-1", Title: "Tactica y estrategia", Disc: 1, Track: 1, DurationMs: 231500},
			{FileID: 2, Fingerprint: "fp-2", Title: "Sin sobresalt", Disc: 1, Track: 2, DurationMs: 209000},
		}}
}

func newWorld() world {
	return world{matches: map[string]store.Match{}, corrections: map[string]store.Correction{},
		releases: map[string]store.Release{}, artists: map[string]store.Artist{}, genres: genres.New(nil)}
}

func autoMatch(w world, tg store.IdentifyTarget) {
	k := BuildQuery(tg).Key()
	w.matches[k] = store.Match{Query: k, Status: store.StatusAuto, ReleaseID: signosID, Confidence: 0.97}
}

func TestResolveAutoMatch(t *testing.T) {
	tg := target()
	q := BuildQuery(tg)
	rel := releaseFrom(signosRelease())
	rel.Discogs = store.DiscogsInfo{Done: true, Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}
	rel.Cover = "cache/covers/" + signosID + ".jpg"
	w := newWorld()
	autoMatch(w, tg)
	w.releases[signosID] = rel
	w.artists[sodaID] = store.Artist{ID: sodaID, Country: "AR"}
	unmapped := map[string]int{}
	got := resolve(tg, q, w, unmapped)
	want := store.AlbumIdentity{AlbumID: 7, Query: q.Key(), Status: store.StatusAuto, ReleaseID: signosID, Confidence: 0.97,
		Title: "Signos", Artist: "Soda Stereo", OriginalYear: 1986, Country: "AR", ArtistID: sodaID, CoverCache: rel.Cover,
		Genres: []store.GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}},
		Tracks: []store.TrackIdentity{{FileID: 1, Title: "Táctica y estrategia", RecordingID: "rec-1"},
			{FileID: 2, Title: "Sin sobresaltos", RecordingID: "rec-2"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if len(unmapped) != 0 {
		t.Fatalf("unmapped = %v", unmapped)
	}
}

func TestResolveCorrectionWins(t *testing.T) {
	tg := target()
	w := newWorld()
	autoMatch(w, tg)
	manual := store.Correction{Status: store.StatusManual, ReleaseID: otherID}
	w.corrections["fp-1"], w.corrections["fp-2"] = manual, manual
	unmapped := map[string]int{}
	got := resolve(tg, BuildQuery(tg), w, unmapped)
	if got.Status != store.StatusManual || got.ReleaseID != otherID || got.Confidence != 1 || got.Title != "" {
		t.Fatalf("got %+v", got)
	}
	// The release is not stored yet: genres still come from the tags.
	if want := []store.GenreLink{{Genre: "Rock", Source: "tag"}}; !reflect.DeepEqual(got.Genres, want) {
		t.Fatalf("genres = %+v", got.Genres)
	}
	if !reflect.DeepEqual(unmapped, map[string]int{"Chamamé": 1}) {
		t.Fatalf("unmapped = %v", unmapped)
	}
}

func TestResolveMinorityCorrectionIgnored(t *testing.T) {
	tg := target()
	tg.Tracks = append(tg.Tracks, store.TargetTrack{FileID: 3, Fingerprint: "fp-3", Title: "Prófugos", Disc: 1, Track: 3})
	w := newWorld()
	k := BuildQuery(tg).Key()
	w.matches[k] = store.Match{Query: k, Status: store.StatusUnmatched, Confidence: 0.4}
	w.corrections["fp-1"] = store.Correction{Status: store.StatusIgnored}
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Status != store.StatusUnmatched {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestResolvePendingAndSelection(t *testing.T) {
	tg := target()
	w := newWorld()
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Status != store.StatusPending {
		t.Fatalf("status = %q", got.Status)
	}
	k := BuildQuery(tg).Key()
	w.matches[k] = store.Match{Query: k, Status: store.StatusUnmatched}
	tg.Selection = true
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Status != store.StatusSelection {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestResolveVariousArtists(t *testing.T) {
	tg := target()
	w := newWorld()
	autoMatch(w, tg)
	rel := releaseFrom(signosRelease())
	rel.Various, rel.ArtistIDs, rel.Artist, rel.Country = true, []string{musicbrainz.VariousArtistsID}, "Various Artists", "BR"
	w.releases[signosID] = rel
	got := resolve(tg, BuildQuery(tg), w, map[string]int{})
	if got.Artist != grouping.Various || !got.Compilation || got.Country != "BR" || got.ArtistID != "" {
		t.Fatalf("got %+v", got)
	}
	rel.Country = "XW"
	w.releases[signosID] = rel
	if got := resolve(tg, BuildQuery(tg), w, map[string]int{}); got.Country != "" {
		t.Fatalf("worldwide release gave country %q", got.Country)
	}
}

func TestResolveKeepsLocalCoverAndUsesMBGenres(t *testing.T) {
	tg := target()
	tg.HasCover = true
	w := newWorld()
	autoMatch(w, tg)
	rel := releaseFrom(signosRelease())
	rel.Cover = "cache/covers/x.jpg"
	rel.MBGenres = []string{"rock", "new wave", "singer-songwriter"}
	w.releases[signosID] = rel
	unmapped := map[string]int{}
	got := resolve(tg, BuildQuery(tg), w, unmapped)
	want := []store.GenreLink{{Genre: "Rock", Source: "mb"}, {Genre: "Rock", Style: "New Wave", Source: "mb"}}
	if got.CoverCache != "" || !reflect.DeepEqual(got.Genres, want) || len(unmapped) != 0 {
		t.Fatalf("cover %q, genres %+v, unmapped %v", got.CoverCache, got.Genres, unmapped)
	}
}

func TestAlignTracks(t *testing.T) {
	rts := releaseFrom(signosRelease()).Tracks
	// Fewer tracks than the release: paired by duration.
	got := alignTracks([]store.TargetTrack{{FileID: 9, Title: "Sin sobresalt", DurationMs: 209000}}, rts)
	if want := []store.TrackIdentity{{FileID: 9, Title: "Sin sobresaltos", RecordingID: "rec-2"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("by duration: %+v", got)
	}
	// Unknown duration: paired by title.
	got = alignTracks([]store.TargetTrack{{FileID: 9, Title: "Tactica y estrategia"}}, rts)
	if len(got) != 1 || got[0].RecordingID != "rec-1" {
		t.Fatalf("by title: %+v", got)
	}
	// Same count but a duration far off: that track is left alone.
	got = alignTracks([]store.TargetTrack{{FileID: 1, DurationMs: 231000}, {FileID: 2, DurationMs: 400000}}, rts)
	if len(got) != 1 || got[0].FileID != 1 {
		t.Fatalf("positional: %+v", got)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/identify/`
Expected: FAIL (`undefined: world`, `undefined: resolve`, …).

- [ ] **Step 3: Implement `internal/identify/resolve.go`**

```go
package identify

import (
	"tordo/internal/genres"
	"tordo/internal/grouping"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// world is everything resolve reads: all local, no requests.
type world struct {
	matches     map[string]store.Match      // by query key
	corrections map[string]store.Correction // by fingerprint
	releases    map[string]store.Release
	artists     map[string]store.Artist
	genres      *genres.Normalizer
}

// resolve decides an album's identity from what is stored. Raw tag genres
// nothing maps are counted in unmapped when they decide the album's genres.
func resolve(t store.IdentifyTarget, q Query, w world, unmapped map[string]int) store.AlbumIdentity {
	id := store.AlbumIdentity{AlbumID: t.AlbumID, Query: q.Key()}
	if c, ok := correctionOf(t, w.corrections); ok {
		id.Status, id.ReleaseID = c.Status, c.ReleaseID
		if c.Status == store.StatusManual {
			id.Confidence = 1
		}
	} else if m, ok := w.matches[id.Query]; ok {
		id.Status, id.Confidence = m.Status, m.Confidence
		if m.Status == store.StatusAuto {
			id.ReleaseID = m.ReleaseID
		}
	}
	if id.Status == store.StatusUnmatched && t.Selection {
		id.Status = store.StatusSelection
	}
	rel, ok := w.releases[id.ReleaseID]
	identified := ok && id.ReleaseID != ""
	if identified {
		id.Title, id.Artist, id.Compilation = rel.Title, rel.Artist, rel.Various
		if rel.Various {
			id.Artist = grouping.Various
		}
		id.OriginalYear = rel.OriginalYear
		if id.OriginalYear == 0 {
			id.OriginalYear = rel.Year
		}
		id.ArtistID = mainArtist(rel)
		id.Country = countryOf(rel, w.artists)
		if !t.HasCover && rel.Cover != "" && rel.Cover != store.CoverNone {
			id.CoverCache = rel.Cover
		}
		id.Tracks = alignTracks(t.Tracks, rel.Tracks)
	}
	id.Genres = albumGenres(t, rel, identified, w.genres, unmapped)
	return id
}

// correctionOf returns the user's correction for an album: the most common
// one when at least half of its tracks carry one.
func correctionOf(t store.IdentifyTarget, cs map[string]store.Correction) (store.Correction, bool) {
	counts := map[store.Correction]int{}
	n := 0
	for _, tr := range t.Tracks {
		if c, ok := cs[tr.Fingerprint]; ok && tr.Fingerprint != "" {
			counts[c]++
			n++
		}
	}
	if n == 0 || n*2 < len(t.Tracks) {
		return store.Correction{}, false
	}
	var best store.Correction
	bn := 0
	for c, k := range counts {
		if k > bn || (k == bn && c.ReleaseID < best.ReleaseID) {
			best, bn = c, k
		}
	}
	return best, true
}

// releaseFrom keeps what tordo uses of a MusicBrainz release.
func releaseFrom(m musicbrainz.Release) store.Release {
	rel := store.Release{ID: m.ID, GroupID: m.GroupID, Title: m.Title, Artist: m.Artist, ArtistIDs: m.ArtistIDs,
		ArtistNames: m.ArtistNames, Various: m.Various, Year: m.Year, OriginalYear: m.OriginalYear, Country: m.Country,
		MBGenres: m.Genres, DiscogsURLs: m.DiscogsURLs}
	for _, t := range m.Tracks {
		rel.Tracks = append(rel.Tracks, store.ReleaseTrack{Disc: t.Disc, Position: t.Position, Title: t.Title,
			LengthMs: t.LengthMs, RecordingID: t.RecordingID})
	}
	return rel
}

// mainArtist is the first credited artist that is not Various Artists.
func mainArtist(rel store.Release) string {
	for _, id := range rel.ArtistIDs {
		if id != musicbrainz.VariousArtistsID {
			return id
		}
	}
	return ""
}

// unspecific release countries: worldwide, Europe, unknown.
var unspecific = map[string]bool{"": true, "XW": true, "XE": true, "XU": true}

// countryOf is the album's country: its main artist's, or for Various
// Artists the release's own country when it names one.
func countryOf(rel store.Release, artists map[string]store.Artist) string {
	if rel.Various {
		if unspecific[rel.Country] {
			return ""
		}
		return rel.Country
	}
	return artists[mainArtist(rel)].Country
}

// alignTracks pairs the album's tracks with the release's: by position when
// both have as many tracks, else each with the unused release track with
// the closest title. A pair needs durations within durationSlackMs, or a
// similar title when a duration is unknown.
func alignTracks(ts []store.TargetTrack, rts []store.ReleaseTrack) []store.TrackIdentity {
	pairs := func(t store.TargetTrack, r store.ReleaseTrack) bool {
		if t.DurationMs > 0 && r.LengthMs > 0 {
			return absDiff(t.DurationMs, r.LengthMs) <= durationSlackMs
		}
		return similarity(t.Title, r.Title) >= 0.6
	}
	identity := func(t store.TargetTrack, r store.ReleaseTrack) store.TrackIdentity {
		return store.TrackIdentity{FileID: t.FileID, Title: r.Title, RecordingID: r.RecordingID}
	}
	var out []store.TrackIdentity
	if len(ts) == len(rts) {
		for i, t := range ts {
			if pairs(t, rts[i]) {
				out = append(out, identity(t, rts[i]))
			}
		}
		return out
	}
	used := make([]bool, len(rts))
	for _, t := range ts {
		best := -1
		for j, r := range rts {
			if used[j] || !pairs(t, r) {
				continue
			}
			if best < 0 || similarity(t.Title, r.Title) > similarity(t.Title, rts[best].Title) {
				best = j
			}
		}
		if best >= 0 {
			used[best] = true
			out = append(out, identity(t, rts[best]))
		}
	}
	return out
}

// albumGenres takes the genres from the best source: Discogs, MusicBrainz,
// the tags.
func albumGenres(t store.IdentifyTarget, rel store.Release, identified bool, n *genres.Normalizer, unmapped map[string]int) []store.GenreLink {
	if identified {
		if ms := genres.FromDiscogs(rel.Discogs.Genres, rel.Discogs.Styles); len(ms) > 0 {
			return links(ms, "discogs")
		}
		var ms []genres.Mapping
		for _, g := range rel.MBGenres {
			m, _ := n.Map(g)
			ms = append(ms, m...)
		}
		if len(ms) > 0 {
			return links(ms, "mb")
		}
	}
	var ms []genres.Mapping
	for _, raw := range t.RawGenres {
		m, ok := n.Map(raw)
		ms = append(ms, m...)
		if !ok {
			unmapped[raw]++
		}
	}
	return links(ms, "tag")
}

func links(ms []genres.Mapping, source string) []store.GenreLink {
	var out []store.GenreLink
	seen := map[genres.Mapping]bool{}
	for _, m := range ms {
		if !seen[m] {
			seen[m] = true
			out = append(out, store.GenreLink{Genre: m.Genre, Style: m.Style, Source: source})
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/identify/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/identify
git commit -m "feat: resolve album identities from corrections, matches and enrichment"
```

---
### Task 13: The identification runner

**Files:**
- Create: `internal/identify/runner.go`
- Test: `internal/identify/runner_test.go`

**Interfaces:**
- Consumes: everything above; `discogs.Info`, `discogs.Result`, `discogs.ParseURL`, `discogs.ErrUnauthorized`, `discogs.ErrNotFound`.
- Produces:
  - States `identify.StateIdle "idle"`, `StateRunning "running"`, `StateOffline "offline"`; Discogs states `DiscogsOK "ok"`, `DiscogsNoToken "noToken"`, `DiscogsBadToken "badToken"`; `identify.ErrBusy`, `identify.ErrNoAlbum`.
  - `identify.Status{State string; ToIdentify, Identified, ToEnrich, Enriched int; Discogs string}` (JSON camelCase)
  - `identify.Discogs` interface: `HasToken() bool; Release(ctx, int) (discogs.Info, error); Master(ctx, int) (discogs.Info, error); Search(ctx, artist, title string) ([]discogs.Result, error)`
  - `identify.Runner{AppDir string; Store *store.Store; MB API; Discogs Discogs; After func(time.Duration, func()) func()}` with `Status() Status`, `Trigger()`, `Close()`, `Run(ctx) error`, `Apply() error`, `SetRelease(ctx, albumID int64, releaseID string) error`, `Ignore(albumID int64) error`, `Reset(albumID int64) error`.

Adapted from Cinexplorer's runner (`/Users/martin/x-projects/cinexplorer/internal/identify/runner.go`: `Trigger`/`Close`/`Run`, the `again` flag, the offline backoff with `After`). One run:

1. **match** every album whose query has no stored match (or one from an older `MatcherVersion`), skipping albums the user corrected; matches are saved in batches of 20 together with the releases they chose; every 100 albums the result is applied so it shows up during a long first run.
2. **apply**.
3. **enrich** each release in use: look it up if a correction named it; its main artist (country, type, years); Discogs genres and styles from MusicBrainz's links (the release's, else its release group's; a release without genres falls back to its master), else from a Discogs search when there is a token. Work already done is skipped: a second run makes no requests.
4. **covers**: identified albums with no folder image and no embedded picture get the Cover Art Archive front (release, else release group) in `cache/covers/<release>.jpg`, written through a temporary file and renamed; a release with none is marked `none` and not asked again.
5. **apply**, always, even after an error.

Offline (MusicBrainz or Discogs unreachable after retries): the run stops, the state is `offline` and a retry is scheduled in 5 min, doubling up to 1 h; a scan triggers a run too. Discogs rejecting the token (401/403) only disables Discogs for the rest of the runner's life (state `badToken`); identification goes on.

Corrections (`SetRelease`, `Ignore`, `Reset`) store the decision on every track fingerprint of the album and apply at once; the caller triggers a run afterwards so enrichment (artist, Discogs, cover) follows. `SetRelease` looks the release up first (unless stored), which validates the id.

- [ ] **Step 1: Write the failing tests**

`internal/identify/runner_test.go`:

```go
package identify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"tordo/internal/audiofile"
	"tordo/internal/discogs"
	"tordo/internal/genres"
	"tordo/internal/grouping"
	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

var roots = []string{"../musica"}

type file struct {
	path string
	fp   string
	ms   int64
	tags audiofile.Tags
}

func newCatalog(t *testing.T, files ...file) *store.Store {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	load(t, st, files...)
	return st
}

// load makes files the catalog's only present files and regroups, as a scan would.
func load(t *testing.T, st *store.Store, files ...file) {
	t.Helper()
	var rows []store.FileRow
	byPath := map[string]file{}
	for _, f := range files {
		rows = append(rows, store.FileRow{Path: f.path, Size: 1, MTime: 1, Kind: "audio"})
		byPath[f.path] = f
	}
	if err := st.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	pending, err := st.PendingReads(env)
	if err != nil {
		t.Fatal(err)
	}
	var rs []store.ReadResult
	for _, p := range pending {
		f := byPath[p.Path]
		rs = append(rs, store.ReadResult{Target: p, Env: env, Fingerprint: f.fp,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: f.ms, Tags: f.tags}})
	}
	if err := st.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	tracks, images, err := st.GroupingInput()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
}

func signosFiles() []file {
	tags := func(title string, n int) audiofile.Tags {
		return audiofile.Tags{Title: title, Artist: "Soda Stereo", Album: "Signos", Track: n, Year: 1986, Genres: []string{"Rock nacional"}}
	}
	return []file{
		{signosDir + "/01.mp3", "fp-1", 231500, tags("Tactica y estrategia", 1)},
		{signosDir + "/02.mp3", "fp-2", 209000, tags("Sin sobresalt", 2)},
	}
}

func signosMB() *fakeMB {
	return &fakeMB{
		search:   map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {signosHit()}},
		releases: map[string]musicbrainz.Release{signosID: signosRelease()},
		artists:  map[string]musicbrainz.Artist{sodaID: {ID: sodaID, Name: "Soda Stereo", Type: "Group", Country: "AR", Area: "Argentina"}},
		covers:   map[string][]byte{"release/" + signosID: []byte("jpeg")},
	}
}

func signosDiscogs() *fakeDiscogs {
	return &fakeDiscogs{releases: map[int]discogs.Info{1234: {ID: 1234, MasterID: 99, Genres: []string{"Rock"},
		Styles: []string{"Pop Rock", "New Wave"}}}}
}

type retries struct {
	mu     sync.Mutex
	delays []time.Duration
}

func newRunner(t *testing.T, st *store.Store, mb *fakeMB, dg *fakeDiscogs) (*Runner, *retries) {
	rt := &retries{}
	r := &Runner{AppDir: t.TempDir(), Store: st, MB: mb,
		After: func(d time.Duration, f func()) func() {
			rt.mu.Lock()
			rt.delays = append(rt.delays, d)
			rt.mu.Unlock()
			return func() {}
		}}
	if dg != nil { // a nil *fakeDiscogs in the interface would not read as "no Discogs"
		r.Discogs = dg
	}
	t.Cleanup(r.Close)
	return r, rt
}

func onlyAlbum(t *testing.T, st *store.Store) store.AlbumDetail {
	t.Helper()
	as, err := st.Albums("")
	if err != nil || len(as) != 1 {
		t.Fatalf("albums = %+v, err %v", as, err)
	}
	a, _, err := st.Album(as[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRunIdentifiesAndEnriches(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb, dg := signosMB(), signosDiscogs()
	r, _ := newRunner(t, st, mb, dg)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if a.Status != store.StatusAuto || a.ReleaseID != signosID || a.Title != "Signos" || a.Country != "AR" || a.OriginalYear != 1986 {
		t.Fatalf("album = %+v", a.AlbumView)
	}
	wantGenres := []store.GenreLink{{Genre: "Rock", Style: "New Wave", Source: "discogs"}, {Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}
	if !reflect.DeepEqual(a.Genres, wantGenres) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if a.CoverCache != "cache/covers/"+signosID+".jpg" {
		t.Fatalf("cover = %q", a.CoverCache)
	}
	if b, err := os.ReadFile(filepath.Join(r.AppDir, "cache", "covers", signosID+".jpg")); err != nil || string(b) != "jpeg" {
		t.Fatalf("cover file %q, err %v", b, err)
	}
	if tr := a.Tracks[1]; tr.Title != "Sin sobresaltos" || tr.TagTitle != "Sin sobresalt" || tr.RecordingID != "rec-2" {
		t.Fatalf("track = %+v", tr)
	}
	if s := r.Status(); s.State != StateIdle || s.Identified != 1 || s.Discogs != DiscogsNoToken {
		t.Fatalf("status = %+v", s)
	}
	if un, _ := st.UnmappedGenres(); len(un) != 0 {
		t.Fatalf("unmapped = %+v", un)
	}
}

func TestSecondRunMakesNoRequests(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb, dg := signosMB(), signosDiscogs()
	r, _ := newRunner(t, st, mb, dg)
	for range 2 {
		if err := r.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(mb.calls) != 4 || len(dg.calls) != 1 { // search, release, artist, cover; Discogs release
		t.Fatalf("MusicBrainz calls %q, Discogs calls %q", mb.calls, dg.calls)
	}
}

func TestUnmatchedGoesToReview(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := &fakeMB{search: map[string][]musicbrainz.ReleaseHit{"Signos|Soda Stereo|false": {otherHit()}}}
	r, _ := newRunner(t, st, mb, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	un, err := st.Unidentified()
	if err != nil || len(un) != 1 || len(un[0].Candidates) != 1 || un[0].Candidates[0].ReleaseID != otherID {
		t.Fatalf("unidentified = %+v, err %v", un, err)
	}
	if c, _ := st.Counts(); c.Unidentified != 1 || c.Identified != 0 {
		t.Fatalf("counts = %+v", c)
	}
}

func TestOfflineSchedulesRetry(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	mb.err = fmt.Errorf("%w: prueba", httpx.ErrOffline)
	r, rt := newRunner(t, st, mb, nil)
	for range 2 {
		if err := r.Run(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if r.Status().State != StateOffline || !reflect.DeepEqual(rt.delays, []time.Duration{5 * time.Minute, 10 * time.Minute}) {
		t.Fatalf("state %q, retries %v", r.Status().State, rt.delays)
	}
	if a := onlyAlbum(t, st); a.Status != store.StatusPending {
		t.Fatalf("status = %q", a.Status)
	}
}

func TestCorrectionSurvivesMoveAndRetag(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := &fakeMB{releases: map[string]musicbrainz.Release{signosID: signosRelease()}}
	r, _ := newRunner(t, st, mb, nil)
	ctx := context.Background()
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if a.Status != store.StatusUnmatched {
		t.Fatalf("status = %q", a.Status)
	}
	if err := r.SetRelease(ctx, a.ID, signosID); err != nil {
		t.Fatal(err)
	}
	if a = onlyAlbum(t, st); a.Status != store.StatusManual || a.Title != "Signos" || a.Confidence != 1 {
		t.Fatalf("after correction: %+v", a.AlbumView)
	}
	moved := signosFiles()
	for i := range moved {
		moved[i].path = "../musica/Ordenar/cosas de soda/" + path.Base(moved[i].path)
		moved[i].tags.Album, moved[i].tags.Artist = "Disco de Soda", "Soda"
	}
	load(t, st, moved...)
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	a = onlyAlbum(t, st)
	if a.Dir != "../musica/Ordenar/cosas de soda" || a.Status != store.StatusManual || a.ReleaseID != signosID || a.Title != "Signos" {
		t.Fatalf("after moving and re-tagging: %+v", a.AlbumView)
	}
	searches := mb.count("search ")
	if err := r.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if mb.count("search ") != searches {
		t.Fatal("a corrected album was searched again")
	}
}

func TestSetReleaseErrors(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	id := onlyAlbum(t, st).ID
	if err := r.SetRelease(context.Background(), id, "nada"); !errors.Is(err, musicbrainz.ErrInvalidID) {
		t.Fatalf("err = %v", err)
	}
	if err := r.SetRelease(context.Background(), id, otherID); !errors.Is(err, musicbrainz.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := r.SetRelease(context.Background(), 9999, signosID); !errors.Is(err, ErrNoAlbum) {
		t.Fatalf("err = %v", err)
	}
}

func TestIgnoreAndReset(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	id := onlyAlbum(t, st).ID
	if err := r.Ignore(id); err != nil {
		t.Fatal(err)
	}
	if un, _ := st.Unidentified(); onlyAlbum(t, st).Status != store.StatusIgnored || len(un) != 0 {
		t.Fatalf("ignored album still in review: %+v", un)
	}
	if err := r.Reset(id); err != nil {
		t.Fatal(err)
	}
	if un, _ := st.Unidentified(); onlyAlbum(t, st).Status != store.StatusUnmatched || len(un) != 1 {
		t.Fatalf("reset album not back in review: %+v", un)
	}
	if err := r.Ignore(9999); !errors.Is(err, ErrNoAlbum) {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectionStaysOutOfReview(t *testing.T) {
	d := "../musica/baulera"
	st := newCatalog(t,
		file{d + "/a.mp3", "fp-a", 1000, audiofile.Tags{Title: "A", Artist: "Uno", Album: "Uno"}},
		file{d + "/b.mp3", "fp-b", 1000, audiofile.Tags{Title: "B", Artist: "Dos", Album: "Dos"}},
		file{d + "/c.mp3", "fp-c", 1000, audiofile.Tags{Title: "C", Artist: "Tres", Album: "Tres"}})
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if un, _ := st.Unidentified(); a.Status != store.StatusSelection || !a.Selection || len(un) != 0 {
		t.Fatalf("album %+v, review %+v", a.AlbumView, un)
	}
}

func TestTagGenresAndAliases(t *testing.T) {
	files := signosFiles()
	for i := range files {
		files[i].tags.Genres = []string{"Rock nacional", "Chamamé"}
	}
	st := newCatalog(t, files...)
	r, _ := newRunner(t, st, &fakeMB{}, nil)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a := onlyAlbum(t, st); !reflect.DeepEqual(a.Genres, []store.GenreLink{{Genre: "Rock", Source: "tag"}}) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if un, _ := st.UnmappedGenres(); !reflect.DeepEqual(un, []store.UnmappedGenre{{Raw: "Chamamé", Albums: 1}}) {
		t.Fatalf("unmapped = %+v", un)
	}
	if err := st.SetGenreAlias(store.GenreAlias{Raw: genres.Key("Chamamé"), Genre: "Folk, World, & Country", Style: "Folk"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Apply(); err != nil {
		t.Fatal(err)
	}
	want := []store.GenreLink{{Genre: "Folk, World, & Country", Style: "Folk", Source: "tag"}, {Genre: "Rock", Source: "tag"}}
	if a := onlyAlbum(t, st); !reflect.DeepEqual(a.Genres, want) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if un, _ := st.UnmappedGenres(); len(un) != 0 {
		t.Fatalf("unmapped = %+v", un)
	}
}

func TestDiscogsSearchWithToken(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	rel := signosRelease()
	rel.DiscogsURLs = nil
	mb.releases[signosID] = rel
	dg := &fakeDiscogs{token: true, search: map[string][]discogs.Result{"Soda Stereo|Signos": {{ID: 77, Artist: "Soda Stereo",
		Title: "Signos", Genres: []string{"Rock"}, Styles: []string{"Pop Rock"}}}}}
	r, _ := newRunner(t, st, mb, dg)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a := onlyAlbum(t, st); !reflect.DeepEqual(a.Genres, []store.GenreLink{{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}) {
		t.Fatalf("genres = %+v", a.Genres)
	}
	if mb.count("group "+signosGroup) != 1 || r.Status().Discogs != DiscogsOK {
		t.Fatalf("calls %q, status %+v", mb.calls, r.Status())
	}
}

func TestDiscogsBadTokenOnlyDisablesDiscogs(t *testing.T) {
	st := newCatalog(t, signosFiles()...)
	mb := signosMB()
	rel := signosRelease()
	rel.DiscogsURLs = nil
	mb.releases[signosID] = rel
	dg := &fakeDiscogs{token: true, err: discogs.ErrUnauthorized}
	r, _ := newRunner(t, st, mb, dg)
	if err := r.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := onlyAlbum(t, st)
	if a.Status != store.StatusAuto || r.Status().Discogs != DiscogsBadToken ||
		!reflect.DeepEqual(a.Genres, []store.GenreLink{{Genre: "Rock", Source: "mb"}}) {
		t.Fatalf("album %+v, status %+v", a.AlbumView, r.Status())
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/identify/`
Expected: FAIL (`undefined: Runner`, …).

- [ ] **Step 3: Implement `internal/identify/runner.go`**

```go
package identify

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tordo/internal/discogs"
	"tordo/internal/genres"
	"tordo/internal/httpx"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// Runner states, as the API shows them.
const (
	StateIdle    = "idle"
	StateRunning = "running"
	StateOffline = "offline" // waiting to retry
)

// Discogs states.
const (
	DiscogsOK       = "ok"
	DiscogsNoToken  = "noToken"  // only MusicBrainz's links to Discogs are followed
	DiscogsBadToken = "badToken" // Discogs rejected the token
)

var (
	ErrBusy    = errors.New("ya hay una identificación en curso")
	ErrNoAlbum = errors.New("álbum no encontrado")
)

// Retry delays while offline: the first, doubled up to the last.
const (
	firstRetry = 5 * time.Minute
	maxRetry   = time.Hour
)

// saveBatch is how many matches are committed at once; applyEvery is how
// many matched albums trigger an intermediate apply.
const (
	saveBatch  = 20
	applyEvery = 100
)

type Status struct {
	State      string `json:"state"`
	ToIdentify int    `json:"toIdentify"`
	Identified int    `json:"identified"`
	ToEnrich   int    `json:"toEnrich"`
	Enriched   int    `json:"enriched"`
	Discogs    string `json:"discogs"`
}

// Discogs is the part of the Discogs client the runner uses.
type Discogs interface {
	HasToken() bool
	Release(ctx context.Context, id int) (discogs.Info, error)
	Master(ctx context.Context, id int) (discogs.Info, error)
	Search(ctx context.Context, artist, title string) ([]discogs.Result, error)
}

type Runner struct {
	AppDir  string
	Store   *store.Store
	MB      API
	Discogs Discogs // nil: no Discogs at all
	// After schedules f after d and returns a function that cancels it;
	// nil means time.AfterFunc. Tests replace it.
	After func(d time.Duration, f func()) (cancel func())

	applyMu sync.Mutex // serializes Apply (runs and corrections)

	mu       sync.Mutex
	status   Status
	running  bool
	again    bool // a Trigger arrived during a run
	backoff  time.Duration
	cancel   func() // pending retry
	badToken bool   // Discogs rejected the token
	base     context.Context
	stop     context.CancelFunc
	closed   bool
	wg       sync.WaitGroup
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.status
	if s.State == "" {
		s.State = StateIdle
	}
	switch {
	case r.Discogs == nil || !r.Discogs.HasToken():
		s.Discogs = DiscogsNoToken
	case r.badToken:
		s.Discogs = DiscogsBadToken
	default:
		s.Discogs = DiscogsOK
	}
	return s
}

// Trigger starts a run in the background. A trigger during a run makes one
// more run follow it. After Close it does nothing.
func (r *Runner) Trigger() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	if r.running {
		r.again = true
		r.mu.Unlock()
		return
	}
	r.running = true
	if r.base == nil {
		r.base, r.stop = context.WithCancel(context.Background())
	}
	ctx := r.base
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		for {
			if err := r.run(ctx); err != nil && ctx.Err() == nil {
				log.Printf("identificación: %v", err)
			}
			r.mu.Lock()
			if !r.again || r.closed {
				r.running = false
				r.mu.Unlock()
				return
			}
			r.again = false
			r.mu.Unlock()
		}
	}()
}

// Close stops the background runs for good: it cancels the one in progress
// and the pending retry, and waits for it to end. What it saved stays.
func (r *Runner) Close() {
	r.mu.Lock()
	r.closed = true
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.stop != nil {
		r.stop()
	}
	r.mu.Unlock()
	r.wg.Wait()
}

// Run performs one run synchronously; ErrBusy if one is in progress.
func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrBusy
	}
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
	return r.run(ctx)
}

func (r *Runner) run(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.status = Status{State: StateRunning}
	r.mu.Unlock()

	// Releases looked up while matching, reused by enrichment in this run.
	fetched := map[string]musicbrainz.Release{}
	err := r.identifyAll(ctx, fetched)
	if err == nil {
		err = r.Apply()
	}
	if err == nil {
		err = r.enrichAll(ctx, fetched)
	}
	if err == nil {
		err = r.coversAll(ctx)
	}
	if aerr := r.Apply(); err == nil {
		err = aerr
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.closed:
		r.status.State = StateIdle
		return nil
	case errors.Is(err, httpx.ErrOffline):
		r.status.State = StateOffline
		if r.backoff == 0 {
			r.backoff = firstRetry
		} else {
			r.backoff = min(2*r.backoff, maxRetry)
		}
		after := r.After
		if after == nil {
			after = func(d time.Duration, f func()) func() { t := time.AfterFunc(d, f); return func() { t.Stop() } }
		}
		r.cancel = after(r.backoff, r.Trigger)
		log.Printf("identificación: sin conexión, se reintenta en %v", r.backoff)
		return nil
	}
	r.status.State = StateIdle
	r.backoff = 0
	return err
}

func (r *Runner) identifyAll(ctx context.Context, fetched map[string]musicbrainz.Release) error {
	targets, err := r.Store.IdentifyTargets()
	if err != nil {
		return err
	}
	matches, err := r.Store.Matches()
	if err != nil {
		return err
	}
	corrections, err := r.Store.Corrections()
	if err != nil {
		return err
	}
	known, err := r.Store.Releases()
	if err != nil {
		return err
	}
	var jobs []Query
	queued := map[string]bool{}
	for _, t := range targets {
		if _, ok := correctionOf(t, corrections); ok {
			continue // the automatic identification never overrides a correction
		}
		q := BuildQuery(t)
		k := q.Key()
		if m, ok := matches[k]; (ok && m.MatcherVersion >= MatcherVersion) || queued[k] {
			continue
		}
		queued[k] = true
		jobs = append(jobs, q)
	}
	r.mu.Lock()
	r.status.ToIdentify = len(jobs)
	r.mu.Unlock()

	var batch []store.Match
	flush := func() error {
		for _, m := range batch {
			if _, ok := known[m.ReleaseID]; m.Status != store.StatusAuto || ok {
				continue
			}
			rel := releaseFrom(fetched[m.ReleaseID])
			if err := r.Store.SaveRelease(rel); err != nil {
				return err
			}
			known[m.ReleaseID] = rel
		}
		err := r.Store.SaveMatches(batch)
		batch = batch[:0]
		return err
	}
	for i, q := range jobs {
		if err := ctx.Err(); err != nil {
			return errors.Join(err, flush())
		}
		m, err := Identify(ctx, r.MB, q, fetched)
		if err != nil {
			return errors.Join(err, flush())
		}
		batch = append(batch, m)
		r.mu.Lock()
		r.status.Identified++
		r.mu.Unlock()
		if len(batch) >= saveBatch {
			if err := flush(); err != nil {
				return err
			}
		}
		if (i+1)%applyEvery == 0 {
			if err := r.Apply(); err != nil {
				return err
			}
		}
	}
	return flush()
}

func (r *Runner) enrichAll(ctx context.Context, fetched map[string]musicbrainz.Release) error {
	ids, err := r.Store.ReleaseIDsInUse()
	if err != nil {
		return err
	}
	releases, err := r.Store.Releases()
	if err != nil {
		return err
	}
	artists, err := r.Store.Artists()
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.status.ToEnrich = len(ids)
	r.mu.Unlock()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, ok := releases[id]
		if !ok { // named by a correction made while offline
			mb, err := lookup(ctx, r.MB, id, fetched)
			switch {
			case fatal(err):
				return err
			case err != nil:
				log.Printf("no se pudo obtener el lanzamiento %s de MusicBrainz: %v", id, err)
				continue
			}
			rel = releaseFrom(mb)
			if err := r.Store.SaveRelease(rel); err != nil {
				return err
			}
		}
		if aid := mainArtist(rel); aid != "" {
			if _, ok := artists[aid]; !ok {
				a, err := r.fetchArtist(ctx, aid)
				if err != nil {
					return err
				}
				artists[aid] = a
			}
		}
		if r.needsDiscogs(rel) {
			if err := r.discogsFor(ctx, &rel); err != nil {
				return err
			}
			if err := r.Store.SaveRelease(rel); err != nil {
				return err
			}
		}
		r.mu.Lock()
		r.status.Enriched++
		r.mu.Unlock()
	}
	return nil
}

// fetchArtist looks an artist up and stores it; one MusicBrainz does not
// have is stored empty so it is not asked for again. Only fatal errors are
// returned.
func (r *Runner) fetchArtist(ctx context.Context, id string) (store.Artist, error) {
	a, err := r.MB.Artist(ctx, id)
	switch {
	case fatal(err):
		return store.Artist{}, err
	case errors.Is(err, musicbrainz.ErrNotFound):
		a = musicbrainz.Artist{ID: id}
	case err != nil:
		log.Printf("no se pudo obtener el artista %s de MusicBrainz: %v", id, err)
		return store.Artist{ID: id}, nil
	}
	sa := store.Artist{ID: a.ID, Name: a.Name, SortName: a.SortName, Type: a.Type, Country: a.Country, Area: a.Area,
		Begin: a.Begin, End: a.End}
	return sa, r.Store.SaveArtist(sa)
}

func (r *Runner) needsDiscogs(rel store.Release) bool {
	if r.Discogs == nil {
		return false
	}
	if !rel.Discogs.Done {
		return true
	}
	return len(rel.Discogs.Genres) == 0 && !rel.Discogs.Searched && r.Discogs.HasToken() && !r.tokenRejected()
}

// discogsFor fills a release's Discogs genres and styles: from MusicBrainz's
// links to Discogs (the release's, else its group's), else from a Discogs
// search when there is a token. Only fatal errors are returned.
func (r *Runner) discogsFor(ctx context.Context, rel *store.Release) error {
	if !rel.Discogs.Done {
		if len(rel.DiscogsURLs) == 0 && !rel.GroupChecked && rel.GroupID != "" {
			g, err := r.MB.ReleaseGroupLinks(ctx, rel.GroupID)
			if fatal(err) {
				return err
			}
			if err != nil {
				log.Printf("no se pudo obtener el grupo %s de MusicBrainz: %v", rel.GroupID, err)
			} else {
				rel.DiscogsURLs = g.DiscogsURLs
				if len(rel.MBGenres) == 0 {
					rel.MBGenres = g.Genres
				}
			}
			rel.GroupChecked = true
		}
		info, err := r.followDiscogs(ctx, rel.DiscogsURLs)
		switch {
		case fatal(err):
			return err
		case errors.Is(err, discogs.ErrUnauthorized):
			r.rejectToken()
			return nil // Done stays false: retried once the token is fixed
		case err != nil:
			log.Printf("Discogs, %s: %v", rel.Title, err)
		}
		rel.Discogs.ID, rel.Discogs.MasterID, rel.Discogs.Genres, rel.Discogs.Styles = info.ID, info.MasterID, info.Genres, info.Styles
		rel.Discogs.Done = true
	}
	if len(rel.Discogs.Genres) > 0 || rel.Discogs.Searched || !r.Discogs.HasToken() || r.tokenRejected() {
		return nil
	}
	results, err := r.Discogs.Search(ctx, searchArtist(*rel), rel.Title)
	switch {
	case fatal(err):
		return err
	case errors.Is(err, discogs.ErrUnauthorized):
		r.rejectToken()
		return nil
	case err != nil:
		log.Printf("búsqueda en Discogs de %q: %v", rel.Title, err)
		return nil
	}
	if res, ok := bestResult(*rel, results); ok {
		rel.Discogs.ID, rel.Discogs.MasterID, rel.Discogs.Genres, rel.Discogs.Styles = res.ID, res.MasterID, res.Genres, res.Styles
	}
	rel.Discogs.Searched = true
	return nil
}

// followDiscogs reads genres and styles from the first Discogs link that
// has them; a release without genres falls back to its master.
func (r *Runner) followDiscogs(ctx context.Context, urls []string) (discogs.Info, error) {
	for _, u := range urls {
		kind, id, ok := discogs.ParseURL(u)
		if !ok {
			continue
		}
		var info discogs.Info
		var err error
		if kind == "release" {
			info, err = r.Discogs.Release(ctx, id)
			if err == nil && len(info.Genres) == 0 && info.MasterID != 0 {
				if m, merr := r.Discogs.Master(ctx, info.MasterID); merr == nil {
					info.Genres, info.Styles = m.Genres, m.Styles
				}
			}
		} else {
			info, err = r.Discogs.Master(ctx, id)
		}
		if errors.Is(err, discogs.ErrNotFound) {
			continue
		}
		if err != nil {
			return discogs.Info{}, err
		}
		if len(info.Genres) > 0 {
			return info, nil
		}
	}
	return discogs.Info{}, nil
}

func (r *Runner) rejectToken() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.badToken {
		log.Print("Discogs rechazó el token: se sigue sin Discogs; revisá discogsToken en config.json")
	}
	r.badToken = true
}

func (r *Runner) tokenRejected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.badToken
}

func searchArtist(rel store.Release) string {
	if rel.Various {
		return "Various"
	}
	if len(rel.ArtistNames) > 0 {
		return rel.ArtistNames[0]
	}
	return rel.Artist
}

// bestResult is the first search result with the release's title (and
// artist, unless a compilation). Discogs adds " (2)" to homonyms; the
// similarity tolerates it.
func bestResult(rel store.Release, rs []discogs.Result) (discogs.Result, bool) {
	for _, res := range rs {
		if similarity(res.Title, rel.Title) >= 0.8 && (rel.Various || similarity(res.Artist, searchArtist(rel)) >= 0.6) {
			return res, true
		}
	}
	return discogs.Result{}, false
}

func (r *Runner) coversAll(ctx context.Context) error {
	ids, err := r.Store.CoverlessReleases()
	if err != nil {
		return err
	}
	releases, err := r.Store.Releases()
	if err != nil {
		return err
	}
	for _, id := range ids {
		rel, ok := releases[id]
		if !ok || rel.Cover != "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := r.MB.Cover(ctx, "release", id)
		if errors.Is(err, musicbrainz.ErrNotFound) && rel.GroupID != "" {
			b, err = r.MB.Cover(ctx, "release-group", rel.GroupID)
		}
		switch {
		case fatal(err):
			return err
		case errors.Is(err, musicbrainz.ErrNotFound):
			rel.Cover = store.CoverNone
		case err != nil:
			log.Printf("carátula de %s: %v", id, err)
			continue
		default:
			if rel.Cover, err = r.saveCover(id, b); err != nil {
				return err
			}
		}
		if err := r.Store.SaveRelease(rel); err != nil {
			return err
		}
	}
	return nil
}

// saveCover writes a cover into cache/covers/ through a temporary file and
// returns its path relative to the app folder.
func (r *Runner) saveCover(id string, b []byte) (string, error) {
	rel := "cache/covers/" + id + ".jpg"
	name := filepath.Join(r.AppDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(name), id+".*.tmp")
	if err != nil {
		return "", err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), name)
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return rel, nil
}

// Apply recomputes every album's identity from what is stored (no requests)
// and writes it.
func (r *Runner) Apply() error {
	r.applyMu.Lock()
	defer r.applyMu.Unlock()
	targets, err := r.Store.IdentifyTargets()
	if err != nil {
		return err
	}
	var w world
	if w.matches, err = r.Store.Matches(); err != nil {
		return err
	}
	if w.corrections, err = r.Store.Corrections(); err != nil {
		return err
	}
	if w.releases, err = r.Store.Releases(); err != nil {
		return err
	}
	if w.artists, err = r.Store.Artists(); err != nil {
		return err
	}
	aliases, err := r.Store.GenreAliases()
	if err != nil {
		return err
	}
	user := map[string]genres.Mapping{}
	for _, a := range aliases {
		user[a.Raw] = genres.Mapping{Genre: a.Genre, Style: a.Style}
	}
	w.genres = genres.New(user)
	unmapped := map[string]int{}
	ids := make([]store.AlbumIdentity, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, resolve(t, BuildQuery(t), w, unmapped))
	}
	return r.Store.ApplyIdentities(ids, unmapped)
}

// SetRelease identifies an album by hand. The release is looked up first
// (which validates the id) unless already stored; the correction goes on
// every track's fingerprint. The caller should Trigger a run afterwards so
// the artist, Discogs genres and cover follow.
func (r *Runner) SetRelease(ctx context.Context, albumID int64, releaseID string) error {
	if !musicbrainz.ValidID(releaseID) {
		return musicbrainz.ErrInvalidID
	}
	fps, err := r.fingerprints(albumID)
	if err != nil {
		return err
	}
	releases, err := r.Store.Releases()
	if err != nil {
		return err
	}
	if _, ok := releases[releaseID]; !ok {
		mb, err := r.MB.Release(ctx, releaseID)
		if err != nil {
			return err
		}
		if err := r.Store.SaveRelease(releaseFrom(mb)); err != nil {
			return err
		}
	}
	return r.correct(fps, store.Correction{Status: store.StatusManual, ReleaseID: releaseID})
}

// Ignore marks an album as not in MusicBrainz: it leaves Sin identificar.
func (r *Runner) Ignore(albumID int64) error {
	fps, err := r.fingerprints(albumID)
	if err != nil {
		return err
	}
	return r.correct(fps, store.Correction{Status: store.StatusIgnored})
}

// Reset removes an album's correction: the automatic identification decides
// again (Trigger a run if it was never matched).
func (r *Runner) Reset(albumID int64) error {
	fps, err := r.fingerprints(albumID)
	if err != nil {
		return err
	}
	return r.correct(fps, store.Correction{})
}

func (r *Runner) correct(fps []string, c store.Correction) error {
	if err := r.Store.SetCorrection(fps, c); err != nil {
		return err
	}
	return r.Apply()
}

func (r *Runner) fingerprints(albumID int64) ([]string, error) {
	fps, err := r.Store.AlbumFingerprints(albumID)
	if err == nil && len(fps) == 0 {
		err = ErrNoAlbum
	}
	return fps, err
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/identify/ -v && go vet ./internal/identify/`
Expected: PASS, no vet warnings.

- [ ] **Step 5: Commit**

```bash
git add internal/identify
git commit -m "feat: identification runner with enrichment, covers, corrections and offline retry"
```

---
### Task 14: API — identification, review, genres and covers

**Files:**
- Modify: `internal/server/server.go`
- Create: `internal/server/identify.go`
- Test: `internal/server/identify_test.go`

**Interfaces:**
- Consumes: `identify.Status`, `identify.ErrNoAlbum`, `identify.StateIdle`; `musicbrainz.ErrInvalidID`, `ErrNotFound`; `httpx.ErrOffline`; `genres.Taxonomy`, `Valid`, `Key`; store methods from Tasks 5–6; `appdir.Abs`.
- Produces: `server.Identifier` interface (implemented by `*identify.Runner`); `Server.Identifier`, `Server.AppDir`; the endpoints below.

```go
type Identifier interface {
	Status() identify.Status
	Trigger()
	Apply() error
	SetRelease(ctx context.Context, albumID int64, releaseID string) error
	Ignore(albumID int64) error
	Reset(albumID int64) error
}
```

| Method and path | What it does |
|---|---|
| `GET /api/status` | adds `"identify"`: the runner's status (`{"state":"off"}` in browse-only mode) |
| `POST /api/identify` | starts an identification run (202) |
| `GET /api/review/unidentified` | the *Sin identificar* queue, with candidates |
| `PUT /api/albums/{id}/identification` | `{"releaseId": "<mbid or musicbrainz.org/release/… URL>"}` or `{"ignore": true}`; answers the updated album |
| `DELETE /api/albums/{id}/identification` | removes the correction; answers the updated album |
| `GET /api/albums/{id}/cover` | the folder image, else the cached Cover Art Archive image; 404 otherwise (embedded pictures: M3) |
| `GET /api/genres` | the taxonomy |
| `GET /api/genres/unmapped` | the *Géneros por mapear* queue |
| `GET /api/genres/aliases` | the user's mappings |
| `PUT /api/genres/aliases` | `{"raw": "Chamamé", "genre": "Folk, World, & Country", "style": "Folk"}` (`genre: ""` = not a genre) |

Every write needs `Content-Type: application/json` (M1's `jsonOnly`), except `DELETE`, which browsers always preflight. After a correction the handler triggers a run, so the artist, Discogs genres and cover of a manually chosen release follow.

- [ ] **Step 1: Write the failing tests**

`internal/server/identify_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"tordo/internal/httpx"
	"tordo/internal/identify"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

type fakeIdentifier struct {
	triggered, applied int
	set                map[int64]string
	ignored, reset     []int64
	err                error
}

func (f *fakeIdentifier) Status() identify.Status {
	return identify.Status{State: identify.StateIdle, Discogs: identify.DiscogsNoToken}
}
func (f *fakeIdentifier) Trigger()     { f.triggered++ }
func (f *fakeIdentifier) Apply() error { f.applied++; return nil }
func (f *fakeIdentifier) SetRelease(ctx context.Context, id int64, rel string) error {
	if f.set == nil {
		f.set = map[int64]string{}
	}
	f.set[id] = rel
	return f.err
}
func (f *fakeIdentifier) Ignore(id int64) error { f.ignored = append(f.ignored, id); return f.err }
func (f *fakeIdentifier) Reset(id int64) error  { f.reset = append(f.reset, id); return f.err }

func identifyServer(t *testing.T) (*Server, *fakeIdentifier, int64) {
	t.Helper()
	st := newStore(t)
	as, err := st.Albums("")
	if err != nil || len(as) != 1 {
		t.Fatalf("albums = %+v, err %v", as, err)
	}
	f := &fakeIdentifier{}
	return &Server{Store: st, Identifier: f, AppDir: t.TempDir()}, f, as[0].ID
}

const jsonCT = "application/json"

func TestStatusIncludesIdentify(t *testing.T) {
	s, _, _ := identifyServer(t)
	var got struct {
		Identify identify.Status `json:"identify"`
	}
	rec := do(t, s.Handler(), "GET", "/api/status", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Identify.State != identify.StateIdle {
		t.Fatalf("status %s, err %v", rec.Body, err)
	}
	s.Identifier = nil
	rec = do(t, s.Handler(), "GET", "/api/status", "")
	var off struct {
		Identify map[string]string `json:"identify"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &off); off.Identify["state"] != "off" {
		t.Fatalf("browse-only status %s", rec.Body)
	}
}

func TestIdentifyEndpoint(t *testing.T) {
	s, f, _ := identifyServer(t)
	if rec := do(t, s.Handler(), "POST", "/api/identify", "{}"); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("without JSON content type: %d", rec.Code)
	}
	if rec := do(t, s.Handler(), "POST", "/api/identify", "{}", "Content-Type", jsonCT); rec.Code != http.StatusAccepted || f.triggered != 1 {
		t.Fatalf("code %d, triggered %d", rec.Code, f.triggered)
	}
	s.Identifier = nil
	if rec := do(t, s.Handler(), "POST", "/api/identify", "{}", "Content-Type", jsonCT); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only: %d", rec.Code)
	}
}

func TestSetIdentification(t *testing.T) {
	s, f, id := identifyServer(t)
	url := "/api/albums/" + strconv.FormatInt(id, 10) + "/identification"
	const mbid = "a1b2c3d4-0000-4000-8000-000000000001"
	rec := do(t, s.Handler(), "PUT", url, `{"releaseId": "https://musicbrainz.org/release/`+mbid+`?x=1"}`, "Content-Type", jsonCT)
	var a store.AlbumDetail
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &a) != nil || a.ID != id || f.set[id] != mbid || f.triggered != 1 {
		t.Fatalf("code %d body %s set %v triggered %d", rec.Code, rec.Body, f.set, f.triggered)
	}
	if rec := do(t, s.Handler(), "PUT", url, `{"ignore": true}`, "Content-Type", jsonCT); rec.Code != http.StatusOK || len(f.ignored) != 1 {
		t.Fatalf("ignore: %d %v", rec.Code, f.ignored)
	}
	if rec := do(t, s.Handler(), "DELETE", url, ""); rec.Code != http.StatusOK || len(f.reset) != 1 {
		t.Fatalf("reset: %d %v", rec.Code, f.reset)
	}
	if rec := do(t, s.Handler(), "PUT", url, `{}`, "Content-Type", jsonCT); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body: %d", rec.Code)
	}
	errs := []struct {
		err  error
		code int
	}{
		{musicbrainz.ErrInvalidID, http.StatusBadRequest},
		{musicbrainz.ErrNotFound, http.StatusNotFound},
		{identify.ErrNoAlbum, http.StatusNotFound},
		{fmt.Errorf("%w: prueba", httpx.ErrOffline), http.StatusServiceUnavailable},
		{errors.New("otro"), http.StatusInternalServerError},
	}
	for _, e := range errs {
		f.err = e.err
		if rec := do(t, s.Handler(), "PUT", url, `{"releaseId": "x"}`, "Content-Type", jsonCT); rec.Code != e.code {
			t.Errorf("%v: code %d, want %d", e.err, rec.Code, e.code)
		}
	}
}

func TestUnidentifiedEndpoint(t *testing.T) {
	s, _, id := identifyServer(t)
	cands := []store.Candidate{{ReleaseID: "r1", Title: "Locura", Artist: "Virus", Score: 0.7}}
	if err := s.Store.SaveMatches([]store.Match{{Query: "q", Status: store.StatusUnmatched, Candidates: cands, MatcherVersion: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ApplyIdentities([]store.AlbumIdentity{{AlbumID: id, Query: "q", Status: store.StatusUnmatched}}, nil); err != nil {
		t.Fatal(err)
	}
	rec := do(t, s.Handler(), "GET", "/api/review/unidentified", "")
	var got []store.UnidentifiedView
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got) != 1 || got[0].Candidates[0].ReleaseID != "r1" {
		t.Fatalf("code %d body %s", rec.Code, rec.Body)
	}
}

func TestCoverEndpoint(t *testing.T) {
	s, _, id := identifyServer(t)
	url := "/api/albums/" + strconv.FormatInt(id, 10) + "/cover"
	if rec := do(t, s.Handler(), "GET", url, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no cover: %d", rec.Code)
	}
	name := filepath.Join(s.AppDir, "cache", "covers", "r1.jpg")
	os.MkdirAll(filepath.Dir(name), 0o755)
	if err := os.WriteFile(name, []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.ApplyIdentities([]store.AlbumIdentity{{AlbumID: id, Status: store.StatusAuto, ReleaseID: "r1",
		CoverCache: "cache/covers/r1.jpg"}}, nil); err != nil {
		t.Fatal(err)
	}
	if rec := do(t, s.Handler(), "GET", url, ""); rec.Code != http.StatusOK || rec.Body.String() != "jpeg" {
		t.Fatalf("cached cover: %d %q", rec.Code, rec.Body)
	}
}

func TestGenreEndpoints(t *testing.T) {
	s, f, _ := identifyServer(t)
	rec := do(t, s.Handler(), "GET", "/api/genres", "")
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) || len(rec.Body.String()) < 100 {
		t.Fatalf("taxonomy: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, s.Handler(), "GET", "/api/genres/unmapped", ""); rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Fatalf("unmapped: %d %q", rec.Code, rec.Body)
	}
	bad := []string{`{"raw": "Chamamé", "genre": "Cumbia"}`, `{"raw": "Chamamé", "genre": "Latin", "style": "Pop Rock"}`,
		`{"raw": "", "genre": "Latin"}`, `{"raw": "x", "style": "Samba"}`}
	for _, b := range bad {
		if rec := do(t, s.Handler(), "PUT", "/api/genres/aliases", b, "Content-Type", jsonCT); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", b, rec.Code)
		}
	}
	rec = do(t, s.Handler(), "PUT", "/api/genres/aliases", `{"raw": "Chamamé", "genre": "Folk, World, & Country", "style": "Folk"}`,
		"Content-Type", jsonCT)
	if rec.Code != http.StatusOK || f.applied != 1 {
		t.Fatalf("alias: %d %s, applied %d", rec.Code, rec.Body, f.applied)
	}
	as, _ := s.Store.GenreAliases()
	if len(as) != 1 || as[0].Raw != "chamame" {
		t.Fatalf("aliases = %+v", as)
	}
	if rec := do(t, s.Handler(), "GET", "/api/genres/aliases", ""); rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("list aliases: %d", rec.Code)
	}
}

func TestReleaseIDFrom(t *testing.T) {
	cases := map[string]string{
		" a1b2c3d4-0000-4000-8000-000000000001 ":                                            "a1b2c3d4-0000-4000-8000-000000000001",
		"https://musicbrainz.org/release/a1b2c3d4-0000-4000-8000-000000000001":              "a1b2c3d4-0000-4000-8000-000000000001",
		"https://musicbrainz.org/release/a1b2c3d4-0000-4000-8000-000000000001/recordings?x": "a1b2c3d4-0000-4000-8000-000000000001",
		"cualquier cosa": "cualquier cosa",
	}
	for in, want := range cases {
		if got := releaseIDFrom(in); got != want {
			t.Errorf("releaseIDFrom(%q) = %q", in, got)
		}
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/server/`
Expected: FAIL (`unknown field Identifier`, `undefined: releaseIDFrom`).

- [ ] **Step 3: Implement**

In `internal/server/server.go`:

1. Replace the `Server` struct:

```go
type Server struct {
	Store      *store.Store
	Scanner    *scan.Scanner // nil in browse-only mode
	Identifier Identifier    // nil in browse-only mode
	AppDir     string        // where cache paths are relative to
	ReadOnly   bool
}
```

2. Register the routes in `Handler`, after the M1 ones:

```go
	mux.HandleFunc("POST /api/identify", jsonOnly(s.identify))
	mux.HandleFunc("GET /api/review/unidentified", s.unidentified)
	mux.HandleFunc("PUT /api/albums/{id}/identification", jsonOnly(s.setIdentification))
	mux.HandleFunc("DELETE /api/albums/{id}/identification", s.resetIdentification)
	mux.HandleFunc("GET /api/albums/{id}/cover", s.cover)
	mux.HandleFunc("GET /api/genres", s.genreTaxonomy)
	mux.HandleFunc("GET /api/genres/unmapped", s.unmappedGenres)
	mux.HandleFunc("GET /api/genres/aliases", s.genreAliases)
	mux.HandleFunc("PUT /api/genres/aliases", jsonOnly(s.setGenreAlias))
```

3. In `status`, add the runner's status to the answer:

```go
	var ident any = map[string]string{"state": "off"}
	if s.Identifier != nil {
		ident = s.Identifier.Status()
	}
	writeJSON(w, map[string]any{"readOnly": s.ReadOnly, "scan": st, "identify": ident, "counts": counts})
```

`internal/server/identify.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"tordo/internal/appdir"
	"tordo/internal/genres"
	"tordo/internal/httpx"
	"tordo/internal/identify"
	"tordo/internal/musicbrainz"
	"tordo/internal/store"
)

// Identifier is what the API needs from the identification runner.
type Identifier interface {
	Status() identify.Status
	Trigger()
	Apply() error
	SetRelease(ctx context.Context, albumID int64, releaseID string) error
	Ignore(albumID int64) error
	Reset(albumID int64) error
}

const maxBody = 1 << 16

func (s *Server) noIdentifier(w http.ResponseWriter) bool {
	if s.Identifier == nil {
		http.Error(w, "modo consulta: no se puede identificar ni corregir", http.StatusConflict)
		return true
	}
	return false
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func (s *Server) identify(w http.ResponseWriter, r *http.Request) {
	if s.noIdentifier(w) {
		return
	}
	s.Identifier.Trigger()
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]bool{"started": true})
}

func (s *Server) unidentified(w http.ResponseWriter, r *http.Request) {
	us, err := s.Store.Unidentified()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, us)
}

// releaseIDFrom accepts an MBID or a musicbrainz.org release URL.
func releaseIDFrom(s string) string {
	s = strings.TrimSpace(s)
	if _, rest, ok := strings.Cut(s, "musicbrainz.org/release/"); ok {
		if i := strings.IndexAny(rest, "/?#"); i >= 0 {
			rest = rest[:i]
		}
		return rest
	}
	return s
}

func (s *Server) setIdentification(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok || s.noIdentifier(w) {
		return
	}
	var body struct {
		ReleaseID string `json:"releaseId"`
		Ignore    bool   `json:"ignore"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&body); err != nil {
		http.Error(w, "cuerpo inválido", http.StatusBadRequest)
		return
	}
	var err error
	switch {
	case body.Ignore:
		err = s.Identifier.Ignore(id)
	case strings.TrimSpace(body.ReleaseID) != "":
		err = s.Identifier.SetRelease(r.Context(), id, releaseIDFrom(body.ReleaseID))
	default:
		http.Error(w, "falta releaseId o ignore", http.StatusBadRequest)
		return
	}
	s.afterCorrection(w, id, err)
}

func (s *Server) resetIdentification(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok || s.noIdentifier(w) {
		return
	}
	s.afterCorrection(w, id, s.Identifier.Reset(id))
}

// afterCorrection answers a correction with the updated album and starts a
// run, so the artist, genres and cover of a chosen release follow.
func (s *Server) afterCorrection(w http.ResponseWriter, id int64, err error) {
	switch {
	case errors.Is(err, musicbrainz.ErrInvalidID):
		http.Error(w, "id de MusicBrainz inválido", http.StatusBadRequest)
		return
	case errors.Is(err, musicbrainz.ErrNotFound):
		http.Error(w, "ese lanzamiento no existe en MusicBrainz", http.StatusNotFound)
		return
	case errors.Is(err, identify.ErrNoAlbum):
		http.Error(w, "álbum no encontrado", http.StatusNotFound)
		return
	case errors.Is(err, httpx.ErrOffline):
		http.Error(w, "sin conexión con MusicBrainz; probá más tarde", http.StatusServiceUnavailable)
		return
	case err != nil:
		fail(w, err)
		return
	}
	s.Identifier.Trigger()
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

// cover serves an album's folder image or its cached Cover Art Archive
// image. Paths come from the catalog, never from the request.
func (s *Server) cover(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	a, ok, err := s.Store.Album(id)
	if err != nil {
		fail(w, err)
		return
	}
	p := a.CoverPath
	if p == "" {
		p = a.CoverCache
	}
	if !ok || p == "" {
		http.Error(w, "sin carátula", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, appdir.Abs(s.AppDir, p))
}

func (s *Server) genreTaxonomy(w http.ResponseWriter, r *http.Request) { writeJSON(w, genres.Taxonomy) }

func (s *Server) unmappedGenres(w http.ResponseWriter, r *http.Request) {
	us, err := s.Store.UnmappedGenres()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, us)
}

func (s *Server) genreAliases(w http.ResponseWriter, r *http.Request) {
	as, err := s.Store.GenreAliases()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, as)
}

func (s *Server) setGenreAlias(w http.ResponseWriter, r *http.Request) {
	if s.noIdentifier(w) {
		return
	}
	var a store.GenreAlias
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&a); err != nil {
		http.Error(w, "cuerpo inválido", http.StatusBadRequest)
		return
	}
	if genres.Key(a.Raw) == "" {
		http.Error(w, "falta el valor a mapear (raw)", http.StatusBadRequest)
		return
	}
	if (a.Genre == "" && a.Style != "") || (a.Genre != "" && !genres.Valid(a.Genre, a.Style)) {
		http.Error(w, "género o estilo desconocido", http.StatusBadRequest)
		return
	}
	a.Raw = genres.Key(a.Raw)
	if err := s.Store.SetGenreAlias(a); err != nil {
		fail(w, err)
		return
	}
	if err := s.Identifier.Apply(); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, a)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/server/ -v`
Expected: PASS (M1's server tests too: `newStore` and `do` are reused).

- [ ] **Step 5: Commit**

```bash
git add internal/server
git commit -m "feat: API for identification status, corrections, review queue, genres and covers"
```

---
### Task 15: Configuration, scan hook and wiring

**Files:**
- Modify: `internal/config/config.go`, `internal/scan/scan.go`, `cmd/tordo/main.go`, `README.md`
- Test: `internal/config/token_test.go`, `internal/scan/ondone_test.go`, `cmd/tordo/main_test.go`

**Interfaces:**
- Produces: `config.Config.DiscogsToken string` (`"discogsToken"`); `scan.Scanner.OnDone func()` — called after every scan that was not cancelled (with or without errors: grouping ran either way). The command builds one `identify.Runner` with `musicbrainz.New()` and `discogs.New(cfg.DiscogsToken)`, starts it after each scan and closes it on exit.

A changed token needs a restart in M2; hot-swapping configuration (Cinexplorer's `engine`) comes with Ajustes in M5.

- [ ] **Step 1: Write the failing tests**

`internal/config/token_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscogsToken(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"roots": ["../musica"], "discogsToken": "abc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(dir)
	if err != nil || cfg.DiscogsToken != "abc" {
		t.Fatalf("token %q, err %v", cfg.DiscogsToken, err)
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if again, _, _ := Load(dir); again.DiscogsToken != "abc" {
		t.Fatal("token lost on save")
	}
}
```

`internal/scan/ondone_test.go`:

```go
package scan

import (
	"context"
	"testing"
)

func TestOnDoneAfterEachScan(t *testing.T) {
	l := newLib(t)
	n := 0
	l.sc.OnDone = func() { n++ }
	l.write(magos+"/01.mp3", tags("La leyenda del hada y el mago", "Rata Blanca", "Magos", 1))
	l.run()
	if n != 1 {
		t.Fatalf("OnDone called %d times", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l.sc.Run(ctx)
	if n != 1 {
		t.Fatal("OnDone called after a cancelled scan")
	}
}
```

In `cmd/tordo/main_test.go`, in `TestSetupCreatesConfigAndScans`, after the check `if a.scanner == nil || a.server.ReadOnly {…}`, add:

```go
	if a.runner == nil || a.server.Identifier == nil || a.scanner.OnDone == nil {
		t.Fatal("expected an identification runner started after scans")
	}
```

(The test library is empty, so the run that the scan triggers makes no requests; `a.close` waits for it.)

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/config/ ./internal/scan/ ./cmd/...`
Expected: FAIL (`cfg.DiscogsToken undefined`, `l.sc.OnDone undefined`, `a.runner undefined`).

- [ ] **Step 3: Implement**

`internal/config/config.go`, the struct:

```go
type Config struct {
	Roots        []string `json:"roots"`        // catalog form: relative to the app folder, '/'-separated
	Ignore       []string `json:"ignore"`       // extra folder patterns to skip (see mediafile.IgnoredDir)
	DiscogsToken string   `json:"discogsToken"` // optional: enables Discogs search
}
```

`internal/scan/scan.go`: add to `Scanner`, after `Env`:

```go
	// OnDone is called after every scan that was not cancelled (the
	// identification runner's Trigger); nil for none.
	OnDone func()
```

and at the end of `Run`, replacing `return err`:

```go
	if ctx.Err() == nil && s.OnDone != nil {
		s.OnDone()
	}
	return err
```

`cmd/tordo/main.go`: add the imports `tordo/internal/discogs`, `tordo/internal/identify`, `tordo/internal/musicbrainz`; add a field to `app`:

```go
	runner  *identify.Runner // nil in browse-only mode
```

and replace, in `setup`, everything from `a := &app{close: st.Close}` to the end of the function with:

```go
	a := &app{close: st.Close}
	if readOnly {
		log.Print("la carpeta de la app no es escribible: modo consulta")
	} else {
		roots := make([]string, len(cfg.Roots))
		for i, r := range cfg.Roots {
			roots[i] = path.Clean(r)
		}
		if cfg.DiscogsToken == "" {
			log.Print("sin token de Discogs: los géneros salen de MusicBrainz, de sus enlaces a Discogs y de los tags")
		}
		runner := &identify.Runner{AppDir: appDir, Store: st, MB: musicbrainz.New(), Discogs: discogs.New(cfg.DiscogsToken)}
		a.runner = runner
		a.scanner = &scan.Scanner{AppDir: appDir, Roots: roots, Ignore: cfg.Ignore, Store: st, OnDone: runner.Trigger}
		a.close = func() error {
			runner.Close()
			return st.Close()
		}
	}
	a.server = &server.Server{Store: st, Scanner: a.scanner, ReadOnly: readOnly, AppDir: appDir}
	if a.runner != nil { // only when set: a nil *Runner in the interface would not read as "none"
		a.server.Identifier = a.runner
	}
	return a, nil
```

`README.md`: change the status line to **Status: M2 (identification)** and describe it; add `"discogsToken": ""` to the `config.json` example with one sentence ("optional; without it genres come from MusicBrainz, from its links to Discogs and from the tags; get one at https://www.discogs.com/settings/developers"); add the Task 14 endpoints to the table; add an "Identification" section:

```markdown
## Identification

After every scan, tordo looks up each album on MusicBrainz (one request per
second, as MusicBrainz asks; a first pass over ~10,000 albums takes a few
hours and resumes where it stopped). Albums above the confidence threshold are
identified automatically: they show MusicBrainz's titles, the original year,
the artist's country, Discogs genres and styles, and a cover from the Cover Art
Archive when the folder has none (saved in `cache/covers/`). The others go to
`GET /api/review/unidentified` with their candidates; choose one with
`PUT /api/albums/{id}/identification`. Corrections are stored per track
fingerprint: they survive moving, renaming and re-tagging the files, and the
automatic identification never overrides them. Without network, everything
else works and the identification retries on its own.
```

- [ ] **Step 4: Run the whole suite**

Run: `go test ./... && go vet ./... && gofmt -l .`
Expected: PASS, no vet warnings, `gofmt -l` prints nothing.

- [ ] **Step 5: Try it by hand (no music needed)**

```bash
mkdir -p /tmp/tordo-m2/musica /tmp/tordo-m2/tordo
go run ./cmd/tordo -dir /tmp/tordo-m2/tordo -port 4748 &
sleep 3
curl -s http://127.0.0.1:4748/api/status | python3 -m json.tool | grep -A7 '"identify"'
curl -s http://127.0.0.1:4748/api/genres | python3 -m json.tool | head -12
kill %1
```

Expected: `"state": "idle"`, `"discogs": "noToken"`; the taxonomy starts with `Blues`.

- [ ] **Step 6: Commit**

```bash
git add internal/config internal/scan cmd/tordo README.md
git commit -m "feat: run the identification after every scan; Discogs token in config.json"
```

---
### Task 16: Check against the real collection

The unit tests use fakes; this task runs M2 against `/Users/martin/Music` (~460 albums, read-only) and the real MusicBrainz, Discogs and Cover Art Archive, and fixes what the real data reveals. **Never write inside `/Users/martin/Music`.** A false identification is worse than a missing one: when in doubt, prefer raising the threshold to lowering it.

**Files:**
- Create: `docs/superpowers/notes/2026-10-01-m2-collection-check.md` (findings, for M3 planning)
- Modify: whichever package a finding points to, always test-first (add the real case to that package's tests). A change to `BuildQuery` or the score bumps `identify.MatcherVersion`.

- [ ] **Step 1: Run tordo on the collection**

`.run/` still holds M1's catalog: opening it migrates it to v2 (derived tables dropped, files and reads kept), and the first scan rebuilds the albums without reading any audio again.

```bash
cat .run/config.json   # roots: ["../../../Music"]; add "discogsToken" if you have one
touch /tmp/tordo-before-m2
go run ./cmd/tordo -dir .run -port 4747
```

In another terminal, poll until `scan.running` is false and `identify.state` is `idle`, noting how long the first identification takes (expected: ~460 albums × ~2 requests at 1 req/s, plus artists → 20–40 min):

```bash
curl -s http://127.0.0.1:4747/api/status | python3 -m json.tool
```

- [ ] **Step 2: Check the results**

- Coverage: `counts.identified` and `counts.unidentified` out of `counts.albums`. Write them down; most tagged albums should be identified.
- False positives — the important check. Look at 20 random automatic identifications and their confidence:
  `sqlite3 .run/tordo.db "SELECT confidence, title, mb_title, artist, mb_artist FROM albums WHERE status = 'auto' ORDER BY random() LIMIT 20"`
  and at the lowest-confidence ones: `… WHERE status = 'auto' ORDER BY confidence LIMIT 20`. Any wrong one: find which score part misled (title, artist, durations), add the case to `score_test.go` or `match_test.go`, fix, bump `MatcherVersion`.
- The review queue: `curl -s http://127.0.0.1:4747/api/review/unidentified | python3 -m json.tool | grep -E '"(tagTitle|tagArtist|title|artist|score)"' | head -80`. Albums whose first candidate is plainly right show a threshold or score problem; albums with no candidate at all show a query problem (check `BuildQuery` on that album's folder name).
- The M1 findings:
  - *Naturaleza Sangre* (album tag `WWW.TODOMUSICACD.8K.COM`): titled from its folder, identified.
  - *Casi en privado*: artist Alfredo Zitarrosa; identified, or status `selection`.
  - `FITO PAEZ - 2004 - mi vida con ellas 1`: identified through the folder variant, title from MusicBrainz.
  - The ID3v1-truncated titles (`La Ley es Tela de Araña y Melo`): full track titles from MusicBrainz (`GET /api/albums/{id}`, `title` vs `tagTitle`).
  - *História da MPB* volumes: are they compilations now? If MusicBrainz credits each volume to the composer, write that down: it is MusicBrainz's data, not a bug.
  - *Raras Partituras*: found through the folder name?
  - `baulera`, `voces`, `infantiles`: status (`selection`, or identified albums).
  - `sqlite3 .run/tordo.db "SELECT status, COUNT(*) FROM albums GROUP BY status"`.
- Genres: `curl -s http://127.0.0.1:4747/api/genres/unmapped | python3 -m json.tool | head -60` — how many raw values remain of M1's 207, and which. Values that are plainly a taxonomy genre or style belong in the built-in aliases (`internal/genres/taxonomy.go`, with a `TestMap` case); the rest are for the user to map. Map one by hand to check the endpoint:
  `curl -s -X PUT -H 'Content-Type: application/json' -d '{"raw":"<value>","genre":"Latin","style":"Tango"}' http://127.0.0.1:4747/api/genres/aliases`
  and check that the albums with that value now list the genre. Genre sources: `sqlite3 .run/tordo.db "SELECT source, COUNT(DISTINCT album_id) FROM album_genres GROUP BY source"`.
- Countries: `sqlite3 .run/tordo.db "SELECT country, COUNT(*) FROM albums WHERE country != '' GROUP BY country ORDER BY 2 DESC"` — AR, UY, BR, US should lead.
- Covers: of M1's 61 albums with no cover, how many have one now:
  `sqlite3 .run/tordo.db "SELECT COUNT(*), SUM(cover_cache != '') FROM albums WHERE cover_path = '' AND cover_file_id = 0"`; `ls .run/cache/covers | wc -l`; open one through `http://127.0.0.1:4747/api/albums/<id>/cover`.
- A correction survives a rescan: pick an album from the review queue whose candidate is right, `curl -s -X PUT -H 'Content-Type: application/json' -d '{"releaseId":"<id>"}' http://127.0.0.1:4747/api/albums/<album>/identification`, check `"status":"manual"`, then `curl -s -X POST -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:4747/api/scan`; once both are idle, the album (look it up again by folder: ids survive, but check) is still `manual`. Then `DELETE` it and see it go back to the queue.
- A second run makes no requests: `curl -s -X POST -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:4747/api/identify`; `/api/status` shows `toIdentify: 0` and it ends in seconds.
- The collection was not modified: `find /Users/martin/Music -newer /tmp/tordo-before-m2 -print | head` prints nothing.

For each wrong result, find the cause, add the real name or case to the package's tests, watch it fail, fix, and re-run (a `MatcherVersion` bump re-matches everything; to redo only enrichment, `sqlite3 .run/tordo.db "DELETE FROM releases; DELETE FROM artists"`).

- [ ] **Step 3: Write down the findings**

`docs/superpowers/notes/2026-10-01-m2-collection-check.md`, in Spanish like the spec and the M1 notes: time of the first identification, requests if counted, identified / unidentified / selection / ignored counts, false positives found and what was fixed, the state of each M1 "Pendiente para M2" item, genres left unmapped, covers found, and what is left for M3 (e.g. display of NFC names in search, embedded covers, artist photos). Numbers and bullet points.

- [ ] **Step 4: Final run and commit**

Run: `go test ./... && go vet ./...`
Expected: PASS.

```bash
git add -A
git status   # make sure .run/ and dist/ are not staged
git commit -m "test: check M2 against the real collection and record findings"
```
