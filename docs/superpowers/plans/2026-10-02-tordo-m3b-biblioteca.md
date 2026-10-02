# Tordo M3b (Biblioteca: Explorar, artistas, búsqueda, Revisar) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Browse the whole collection from the browser: Explorar with combinable facets (decade/year, artist, country, genre → style, collection, folder, quality, format, state) over albums, artists and tracks; artist pages; an instant search (Ctrl+K) over artists, albums, tracks and folders; the album page with MusicBrainz's data and corrections; Revisar (Sin identificar, Géneros por mapear); one cover endpoint including the Cover Art Archive; and "no es una colección", persisted.

**Architecture:** As in Cinexplorer, the pages that span the catalog work on an in-memory model: `store.Snapshot` reads everything in one transaction, the new pure package `internal/catalog` arranges it (albums, artists, tracks, facet values, artist keys, the artist page, search documents), and the server rebuilds it only when the store's change counter (`total_changes()`) moves. `internal/search` is an in-memory FTS5 index over the catalog's documents. Per-item pages that already exist (album, folder) keep reading the store. The web app gets Explorar (the new `/`), Artista, Buscar and Revisar, and the album page grows the identification. Schema v3 only adds `not_collections`.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`, whose FTS5 the search uses), Svelte 5 + Vite 8 + Vitest 5 (as in M3a).

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish). M3b covers what §10 lists for M3 and M3a left out: from §8 **Explorar** (tabs, facets, orders, facets in the URL; "me gusta", "puntaje", "con duplicados" and "más escuchado" belong to M4/M5), **Artista** (country, years, albums by year, appearances in compilations; photo, most played and ♥ are later), **Álbum** with M2's data and `⋯ Corregir identificación`, **Revisar** (Sin identificar, Géneros por mapear), **Búsqueda** (§8), "no es una colección" (§4.3) and the M3a plan's list "M3b: what comes after M2". Also the findings of `docs/superpowers/notes/2026-10-01-m2-collection-check.md` (bulk accept of ≥ 0,70 candidates, deleting a genre mapping) and `…-m3a-collection-check.md` (NFD names in the search).

**Verified:** when this plan was written, every task was applied in order to a copy of the repo at commit `fdca367` (end of M3a, M2 merged), each followed by `gofmt -l`, `go vet ./...`, `go test ./...` and, from Task 8 on, `npx vitest run` and `npm run build`: all passed after every task (`go test -race` on `server`, `catalog`, `search`, `store`, `covers`, `stream` and `cmd/tordo` at the end; 77 Vitest tests). The result was run against a copy of the M2/M3a catalog of `/Users/martin/Music` (460 albums, 6685 tracks) in a browser, desktop and phone sizes: Explorar answered in ~13 ms once built (70 ms the first time); facets, tabs and paging; the search found the NFD folder *Latinoamérica 600* typing `latinoamerica`; artists merged across spellings and MusicBrainz credits (192 artists; "Caetano Veloso & Maria Bethânia" and "Caetano Veloso" are one); a correction round trip (not in MusicBrainz, then automatic); mapping and forgetting a genre; "no es una colección" on *Brasil* and back. See "Checks against the collection" at the end.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new Go dependencies** (only `modernc.org/sqlite v1.59.0`; FTS5 is built in). No cgo.
- npm: no new packages (only `svelte`, `vite`, `vitest`, `@sveltejs/vite-plugin-svelte`, `@fontsource-variable/inter`).
- **Never write to the music roots.** The only writes go to the app folder: `tordo.db`, `config.json`, `cache/`. Browse-only mode writes nothing: write endpoints answer 409, the pages hide their write actions (`app.status.readOnly`).
- Catalog paths are relative to the app folder, `/`-separated. Audio and pictures are asked for **by id**, never by path; folders by their catalog path.
- Server listens on `127.0.0.1` only (`localOnly`); write endpoints require `Content-Type: application/json` (DELETE endpoints need a CORS preflight anyway).
- SQLite `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open, and inside a transaction read only through it.
- Schema changes bump `schemaVersion`; derived tables are only dropped below `derivedVersion` (2).
- `ffmpeg` is optional: without it, tracks that need it are "no reproducible" (a facet value, dimmed rows), never an error.
- User-facing strings (UI, logs, errors) in **Spanish** (rioplatense: *revisá*, *pegá*, *elegí*); identifiers and comments in English. Facet and order names in URLs are Spanish (`decada`, `anio`, `genero`, `orden`…), as in Cinexplorer. UI files are named in Spanish (`Explorar.svelte`, `BarraFacetas.svelte`).
- The web build (`internal/server/dist/`) is **not committed**. `go test ./...` passes without Node and **without network**; `go vet ./...` clean; `cd web && npx vitest run` passes.
- Commit messages: conventional style (`feat:`, `fix:`, `test:`, `docs:`), **no `Co-Authored-By:` trailer**.

## File Structure

```
internal/store/snapshot.go         Snapshot (albums, tracks, folders, MB artists in one transaction), Changes
internal/store/catalog.go          + Picture/hasPicture, Detected/isCollection/folderCols, SetCollection; readers take a querier
internal/store/library.go          CoverFiles (folder image + Cover Art Archive cache); MB titles in play items
internal/store/apply.go            + AlbumCandidates, DeleteGenreAlias
internal/store/schema.sql, store.go not_collections; schema v3 (derivedVersion 2)
internal/catalog/                  NEW, pure: catalog.go (Build: albums, artists, tracks, artist keys), facets.go
                                   (Query, ParseQuery, Explore: filter, count, sort, page), artist.go (ArtistPage),
                                   search.go (SearchDocs, Found)
internal/search/                   NEW: FTS5 index in memory (adapted from Cinexplorer), folding with nameparse.Norm
internal/covers/covers.go          + the Cover Art Archive cache as the last source
internal/server/catalog.go         NEW: catalogState, /api/explore, /api/artists/{key}, /api/search, the album page
internal/server/server.go          routes; PUT /api/folders/collection; album → writeAlbum; M2's cover route removed
internal/server/identify.go        corrections answer the album page; DELETE /api/genres/aliases
internal/server/media.go           /api/play?artist=
web/src/lib/                       router, api, facets (NEW), search (NEW), review (NEW), format (+country, formatName, years)
web/src/components/                BarraFacetas, TarjetaArtista, Busqueda, Identificar, SinIdentificar, GenerosPorMapear (NEW);
                                   ListaPistas (links over a play button), TarjetaAlbum, Nav, Icono
web/src/pages/                     Explorar, Artista, Buscar, Revisar (NEW); Album, Carpetas, Listas (changed)
README.md                          browsing, search, Revisar, endpoints
```

Deferred beyond M3: artist photos, album-level manual genre edits, "más escuchadas" and ♥/★ (M4), stations from Explorar (M4), duplicates and formats to convert in Revisar (M5), playing a whole Explorar result as a station (M4).

## How to read the tasks

New files are given whole. Changed files are given as unified diff hunks (`@@` headers, three lines of context) against the state the previous task left: apply them by hand, matching the context lines. Tests come first in each task: write them, see them fail, then write the code. Svelte components have no tests (as in M3a): their logic lives in the tested `web/src/lib/` modules, and each web task ends with the build.

---
### Task 1: Covers from the Cover Art Archive, MusicBrainz titles in play items

M2 kept two cover endpoints: its own `GET /api/albums/{id}/cover` (folder image or Cover Art Archive cache) and M3a's `GET /api/covers/album/{id}` (folder image or embedded picture, with thumbnails). They become one: `covers` tries the folder image, then up to three embedded pictures, then the Cover Art Archive image M2 downloaded (`albums.cover_cache`), and M2's route goes away. Albums gain `picture` (any of the three), so pages can skip asking for covers that do not exist; `FolderArt` counts cached covers too. Play items (`/api/play`) use MusicBrainz's track and album titles when the album is identified, as the album page does.

**Files:**
- Modify: `internal/covers/covers.go`
- Modify: `internal/server/identify.go`
- Modify: `internal/server/identify_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/store/catalog.go`
- Modify: `internal/store/library.go`
- Modify: `internal/store/library_test.go`
- Modify: `web/src/pages/Listas.svelte`

**Interfaces:**
- Consumes: `store.AlbumView`, `albums.cover_cache` (M2), `covers.Covers` (M3a).
- Produces:
  - `store.CoverFiles{Folder, Cache string}`; `(*Store).AlbumCover(albumID int64) (CoverFiles, bool, error)` (was `(string, bool, error)`).
  - `store.AlbumView.Picture bool` (`json:"picture"`), and the SQL constant `hasPicture` in `internal/store/catalog.go`.
  - `PlayItem.Title`/`.Album` are MusicBrainz's when paired.
  - Route removed: `GET /api/albums/{id}/cover` (and `(*Server).cover`).

- [ ] **Step 1: Write the failing tests**

Modify `internal/server/identify_test.go` (unified diff against the previous task):

```diff
@@ -11,6 +11,7 @@ import (
 	"strconv"
 	"testing"
 
+	"tordo/internal/covers"
 	"tordo/internal/httpx"
 	"tordo/internal/identify"
 	"tordo/internal/musicbrainz"
@@ -140,9 +141,12 @@ func TestUnidentifiedEndpoint(t *testing.T) {
 	}
 }
 
-func TestCoverEndpoint(t *testing.T) {
+// TestCachedCover: an album without a picture of its own gets the Cover Art
+// Archive image the identification downloaded, through the covers endpoint.
+func TestCachedCover(t *testing.T) {
 	s, _, id := identifyServer(t)
-	url := "/api/albums/" + strconv.FormatInt(id, 10) + "/cover"
+	s.Covers = covers.New(s.Store, s.AppDir, "")
+	url := "/api/covers/album/" + strconv.FormatInt(id, 10)
 	if rec := do(t, s.Handler(), "GET", url, ""); rec.Code != http.StatusNotFound {
 		t.Fatalf("no cover: %d", rec.Code)
 	}
@@ -150,15 +154,20 @@ func TestCoverEndpoint(t *testing.T) {
 	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
 		t.Fatal(err)
 	}
-	if err := os.WriteFile(name, []byte("jpeg"), 0o644); err != nil {
+	jpeg := []byte("\xff\xd8\xff\xe0 jpeg")
+	if err := os.WriteFile(name, jpeg, 0o644); err != nil {
 		t.Fatal(err)
 	}
 	if err := s.Store.ApplyIdentities([]store.AlbumIdentity{{AlbumID: id, Status: store.StatusAuto, ReleaseID: "r1",
 		CoverCache: "cache/covers/r1.jpg"}}, nil); err != nil {
 		t.Fatal(err)
 	}
-	if rec := do(t, s.Handler(), "GET", url, ""); rec.Code != http.StatusOK || rec.Body.String() != "jpeg" {
-		t.Fatalf("cached cover: %d %q", rec.Code, rec.Body)
+	rec := do(t, s.Handler(), "GET", url, "")
+	if rec.Code != http.StatusOK || rec.Body.String() != string(jpeg) || rec.Header().Get("Content-Type") != "image/jpeg" {
+		t.Fatalf("cached cover: %d %q %v", rec.Code, rec.Body, rec.Header())
+	}
+	if rec := do(t, s.Handler(), "GET", "/api/albums/"+strconv.FormatInt(id, 10)+"/cover", ""); rec.Code != http.StatusNotFound {
+		t.Fatal("the M2 cover route is gone: /api/covers/album serves every picture")
 	}
 }
```

Modify `internal/store/library_test.go` (unified diff against the previous task):

```diff
@@ -120,17 +120,45 @@ func TestAlbumPictures(t *testing.T) {
 		t.Fatalf("files = %q", files)
 	}
 	cover, ok, err := s.AlbumCover(signos)
-	if err != nil || !ok || cover != la+"/[LA-10] Soda Stereo - Signos/cover.jpg" {
-		t.Fatalf("cover %q, ok %v, err %v", cover, ok, err)
+	if err != nil || !ok || cover != (CoverFiles{Folder: la + "/[LA-10] Soda Stereo - Signos/cover.jpg"}) {
+		t.Fatalf("cover %+v, ok %v, err %v", cover, ok, err)
 	}
-	if files, _ := s.AlbumPictureFiles(albumNamed(t, s, "Locura")); len(files) != 0 {
+	locura := albumNamed(t, s, "Locura")
+	if files, _ := s.AlbumPictureFiles(locura); len(files) != 0 {
 		t.Fatalf("Locura has no pictures: %q", files)
 	}
+	if a, _, _ := s.Album(locura); a.Picture {
+		t.Fatal("Locura has a picture")
+	}
+	if err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: locura, Status: StatusAuto, ReleaseID: "r1",
+		CoverCache: "cache/covers/r1.jpg"}}, nil); err != nil {
+		t.Fatal(err)
+	}
+	if cover, _, _ := s.AlbumCover(locura); cover != (CoverFiles{Cache: "cache/covers/r1.jpg"}) {
+		t.Fatalf("cached cover: %+v", cover)
+	}
+	if a, _, _ := s.Album(locura); !a.Picture {
+		t.Fatal("a cached cover is a picture")
+	}
 	if _, ok, _ := s.AlbumCover(9999); ok {
 		t.Fatal("unknown album found")
 	}
 }
 
+func TestPlayItemsUseMusicBrainzTitles(t *testing.T) {
+	s := library(t)
+	id := albumNamed(t, s, "Locura")
+	before, _ := s.PlayAlbum(id)
+	if err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Status: StatusAuto, ReleaseID: "r1", Title: "Locura (MB)",
+		Artist: "Virus", Tracks: []TrackIdentity{{FileID: before[0].ID, Title: "Una luna de miel en la mano (MB)"}}}}, nil); err != nil {
+		t.Fatal(err)
+	}
+	items, err := s.PlayAlbum(id)
+	if err != nil || len(items) != 1 || items[0].Title != "Una luna de miel en la mano (MB)" || items[0].Album != "Locura (MB)" {
+		t.Fatalf("items = %+v, err %v", items, err)
+	}
+}
+
 func TestFolderArt(t *testing.T) {
 	s := library(t)
 	art, err := s.FolderArt("../musica")
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/store ./internal/covers ./internal/server`
Expected: FAIL — build failure: `cover != (CoverFiles{...})` mismatched types / `a.Picture undefined`.

- [ ] **Step 3: Write the code**

Modify `internal/covers/covers.go` (unified diff against the previous task):

```diff
@@ -1,7 +1,8 @@
-// Package covers serves album and track pictures: the album folder's image
-// or a picture embedded in its files, whole or shrunk to a thumbnail. It
-// never writes to the music folders: thumbnails are kept in the app's
-// cache folder.
+// Package covers serves album and track pictures: the album folder's image,
+// a picture embedded in its files or the Cover Art Archive image the
+// identification downloaded, whole or shrunk to a thumbnail. It never
+// writes to the music folders: thumbnails are kept in the app's cache
+// folder.
 package covers
 
 import (
@@ -59,21 +60,24 @@ type source struct {
 }
 
 func (c *Covers) albumSources(id int64) ([]source, bool, error) {
-	folderImage, ok, err := c.Store.AlbumCover(id)
+	files, ok, err := c.Store.AlbumCover(id)
 	if err != nil || !ok {
 		return nil, ok, err
 	}
 	var out []source
-	if folderImage != "" {
-		out = append(out, source{appdir.Abs(c.AppDir, folderImage), false})
+	if files.Folder != "" {
+		out = append(out, source{appdir.Abs(c.AppDir, files.Folder), false})
 	}
-	files, err := c.Store.AlbumPictureFiles(id)
+	embedded, err := c.Store.AlbumPictureFiles(id)
 	if err != nil {
 		return nil, true, err
 	}
-	for _, f := range files[:min(len(files), maxCandidates)] {
+	for _, f := range embedded[:min(len(embedded), maxCandidates)] {
 		out = append(out, source{appdir.Abs(c.AppDir, f), true})
 	}
+	if files.Cache != "" {
+		out = append(out, source{appdir.Abs(c.AppDir, files.Cache), false})
+	}
 	return out, true, nil
 }
```

Modify `internal/server/identify.go` (unified diff against the previous task):

```diff
@@ -9,7 +9,6 @@ import (
 	"strconv"
 	"strings"
 
-	"tordo/internal/appdir"
 	"tordo/internal/genres"
 	"tordo/internal/httpx"
 	"tordo/internal/identify"
@@ -142,29 +141,6 @@ func (s *Server) afterCorrection(w http.ResponseWriter, id int64, err error) {
 	writeJSON(w, a)
 }
 
-// cover serves an album's folder image or its cached Cover Art Archive
-// image. Paths come from the catalog, never from the request.
-func (s *Server) cover(w http.ResponseWriter, r *http.Request) {
-	id, ok := pathID(w, r)
-	if !ok {
-		return
-	}
-	a, ok, err := s.Store.Album(id)
-	if err != nil {
-		fail(w, err)
-		return
-	}
-	p := a.CoverPath
-	if p == "" {
-		p = a.CoverCache
-	}
-	if !ok || p == "" {
-		http.Error(w, "sin carátula", http.StatusNotFound)
-		return
-	}
-	http.ServeFile(w, r, appdir.Abs(s.AppDir, p))
-}
-
 func (s *Server) genreTaxonomy(w http.ResponseWriter, r *http.Request) { writeJSON(w, genres.Taxonomy) }
 
 func (s *Server) unmappedGenres(w http.ResponseWriter, r *http.Request) {
```

Modify `internal/server/server.go` (unified diff against the previous task):

```diff
@@ -43,7 +43,6 @@ func (s *Server) Handler() http.Handler {
 	mux.HandleFunc("GET /api/review/unidentified", s.unidentified)
 	mux.HandleFunc("PUT /api/albums/{id}/identification", jsonOnly(s.setIdentification))
 	mux.HandleFunc("DELETE /api/albums/{id}/identification", s.resetIdentification)
-	mux.HandleFunc("GET /api/albums/{id}/cover", s.cover)
 	mux.HandleFunc("GET /api/genres", s.genreTaxonomy)
 	mux.HandleFunc("GET /api/genres/unmapped", s.unmappedGenres)
 	mux.HandleFunc("GET /api/genres/aliases", s.genreAliases)
```

Modify `internal/store/catalog.go` (unified diff against the previous task):

```diff
@@ -132,6 +132,7 @@ type AlbumView struct {
 	CoverPath    string      `json:"coverPath"`
 	CoverFileID  int64       `json:"coverFileId"`
 	CoverCache   string      `json:"coverCache"`
+	Picture      bool        `json:"picture"` // a folder image, an embedded picture or a cached one
 	Genres       []GenreLink `json:"genres"`
 	RawGenres    []string    `json:"rawGenres"`
 	Status       string      `json:"status"`
@@ -194,7 +195,10 @@ type CollectionView struct {
 const albumCols = `id, CASE WHEN mb_title != '' THEN mb_title ELSE title END,
 	CASE WHEN mb_artist != '' THEN mb_artist ELSE artist END, year, original_year, compilation OR mb_compilation,
 	selection, discs, tracks, duration_ms, dir, position, cover_path, cover_file_id, cover_cache, genres, status,
-	release_id, confidence, country, artist_id, title, artist, match_query`
+	release_id, confidence, country, artist_id, title, artist, match_query, ` + hasPicture
+
+// hasPicture tells, in SQL over albums, whether an album has a picture.
+const hasPicture = `(cover_path != '' OR cover_file_id != 0 OR cover_cache != '')`
 
 func (s *Store) queryAlbums(q string, args ...any) ([]AlbumView, error) {
 	out, err := s.scanAlbums(q, args...)
@@ -216,7 +220,7 @@ func (s *Store) scanAlbums(q string, args ...any) ([]AlbumView, error) {
 		var raw string
 		if err := rows.Scan(&a.ID, &a.Title, &a.Artist, &a.Year, &a.OriginalYear, &a.Compilation, &a.Selection, &a.Discs,
 			&a.Tracks, &a.DurationMs, &a.Dir, &a.Position, &a.CoverPath, &a.CoverFileID, &a.CoverCache, &raw, &a.Status,
-			&a.ReleaseID, &a.Confidence, &a.Country, &a.ArtistID, &a.TagTitle, &a.TagArtist, &a.MatchQuery); err != nil {
+			&a.ReleaseID, &a.Confidence, &a.Country, &a.ArtistID, &a.TagTitle, &a.TagArtist, &a.MatchQuery, &a.Picture); err != nil {
 			return nil, err
 		}
 		if err := json.Unmarshal([]byte(raw), &a.RawGenres); err != nil {
```

Modify `internal/store/library.go` (unified diff against the previous task):

```diff
@@ -49,7 +49,8 @@ type PlayItem struct {
 	track      int
 }
 
-const playQuery = `SELECT t.file_id, t.title, t.artist, COALESCE(al.title, ''), COALESCE(t.album_id, 0), t.duration_ms,
+const playQuery = `SELECT t.file_id, CASE WHEN t.mb_title != '' THEN t.mb_title ELSE t.title END, t.artist,
+	COALESCE(CASE WHEN al.mb_title != '' THEN al.mb_title ELSE al.title END, ''), COALESCE(t.album_id, 0), t.duration_ms,
 	a.format, a.codec, a.lossless, t.dir, f.path, t.disc, t.track
 	FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id
 	LEFT JOIN albums al ON al.id = t.album_id `
@@ -209,16 +210,21 @@ func (s *Store) AlbumPictureFiles(albumID int64) ([]string, error) {
 	return out, rows.Err()
 }
 
-// AlbumCover returns where an album's picture can come from: the folder
-// image chosen by the grouping (a path relative to the app folder, or "")
-// and whether the album exists.
-func (s *Store) AlbumCover(albumID int64) (string, bool, error) {
-	var p string
-	err := s.db.QueryRow(`SELECT cover_path FROM albums WHERE id = ?`, albumID).Scan(&p)
+// CoverFiles are the image files an album's picture can come from, as
+// paths relative to the app folder ("" when there is none).
+type CoverFiles struct {
+	Folder string // the folder image the grouping chose
+	Cache  string // the Cover Art Archive image the identification downloaded
+}
+
+// AlbumCover returns an album's image files and whether the album exists.
+func (s *Store) AlbumCover(albumID int64) (CoverFiles, bool, error) {
+	var c CoverFiles
+	err := s.db.QueryRow(`SELECT cover_path, cover_cache FROM albums WHERE id = ?`, albumID).Scan(&c.Folder, &c.Cache)
 	if errors.Is(err, sql.ErrNoRows) {
-		return "", false, nil
+		return CoverFiles{}, false, nil
 	}
-	return p, err == nil, err
+	return c, err == nil, err
 }
 
 // FolderArt returns, for each subfolder of p, up to four of the albums with
@@ -231,7 +237,7 @@ func (s *Store) FolderArt(p string) (map[string][]int64, error) {
 		return out, nil // the top level lists the roots: they get no art
 	}
 	prefix := p + "/"
-	rows, err := s.db.Query(`SELECT id, dir, cover_path != '' OR cover_file_id != 0 FROM albums
+	rows, err := s.db.Query(`SELECT id, dir, `+hasPicture+` FROM albums
 		WHERE substr(dir, 1, length(?1)) = ?1`, prefix)
 	if err != nil {
 		return nil, err
```

Modify `web/src/pages/Listas.svelte` (unified diff against the previous task):

```diff
@@ -7,8 +7,8 @@
   import { count } from '../lib/format.js'
   import { folderHref } from '../lib/router.js'
 
-  // The albums with a picture: the folder's image or an embedded one.
-  const withPicture = (albums) => albums.filter((a) => a.coverPath || a.coverFileId).slice(0, 4)
+  // The albums with a picture: a folder image, an embedded or a cached one.
+  const withPicture = (albums) => albums.filter((a) => a.picture).slice(0, 4)
 
   let collections = $state(null)
   let error = $state('')
```

- [ ] **Step 4: Run the checks**

```bash
gofmt -l internal cmd   # prints nothing
go vet ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/covers/covers.go internal/server/identify.go internal/server/identify_test.go internal/server/server.go internal/store/catalog.go internal/store/library.go internal/store/library_test.go web/src/pages/Listas.svelte
git commit -m "feat: one cover endpoint for folder, embedded and Cover Art Archive pictures; MusicBrainz titles in play items"
```

### Task 2: "No es una colección", kept across scans (schema v3)

Collections are detected by the grouping (M1); the spec gives each one a persisted "no es una colección". Schema v3 adds `not_collections` (path, updated_at); `migrate` keeps dropping derived tables only below version 2 (`derivedVersion`), so a v2 catalog upgrades in place. Folders read as collections only when detected and not dismissed (`isCollection`); `FolderSummary.Detected` tells the page a detected folder can be dismissed or taken back. `PUT /api/folders/collection {"path","collection"}` stores the decision (409 in browse-only mode).

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `internal/store/catalog.go`
- Modify: `internal/store/catalog_test.go`
- Modify: `internal/store/migrate_test.go`
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/store.go`

**Interfaces:**
- Consumes: `folders.collection` (M1).
- Produces:
  - `store.FolderSummary.Detected bool` (`json:"detected"`); `Collection` now means "detected and not dismissed".
  - SQL constants `isCollection`, `folderCols` in `internal/store/catalog.go`.
  - `(*Store).SetCollection(path string, collection bool) error`.
  - `schemaVersion = 3`, `derivedVersion = 2`.
  - `PUT /api/folders/collection` → the folder's `store.FolderSummary`.

- [ ] **Step 1: Write the failing tests**

Modify `internal/server/server_test.go` (unified diff against the previous task):

```diff
@@ -146,3 +146,29 @@ func TestScanEndpoint(t *testing.T) {
 }
 
 func itoa(n int64) string { return strconv.FormatInt(n, 10) }
+
+func TestSetCollectionEndpoint(t *testing.T) {
+	s := &Server{Store: newStore(t)}
+	h := s.Handler()
+	put := func(body string) *httptest.ResponseRecorder {
+		return do(t, h, "PUT", "/api/folders/collection", body, "Content-Type", "application/json")
+	}
+	var f store.FolderSummary
+	decode(t, put(`{"path":"../musica","collection":false}`), &f)
+	if f.Path != "../musica" || f.Collection {
+		t.Fatalf("folder = %+v", f)
+	}
+	if rec := put(`{"path":"../nope","collection":false}`); rec.Code != http.StatusNotFound {
+		t.Fatalf("unknown folder: %d", rec.Code)
+	}
+	if rec := put(`{"collection":false}`); rec.Code != http.StatusBadRequest {
+		t.Fatalf("no path: %d", rec.Code)
+	}
+	if rec := do(t, h, "PUT", "/api/folders/collection", `{"path":"../musica"}`); rec.Code != http.StatusUnsupportedMediaType {
+		t.Fatalf("not JSON: %d", rec.Code)
+	}
+	s.ReadOnly = true
+	if rec := do(t, s.Handler(), "PUT", "/api/folders/collection", `{"path":"../musica"}`, "Content-Type", "application/json"); rec.Code != http.StatusConflict {
+		t.Fatalf("browse-only: %d", rec.Code)
+	}
+}
```

Modify `internal/store/catalog_test.go` (unified diff against the previous task):

```diff
@@ -140,3 +140,31 @@ func TestCollectionsInOrder(t *testing.T) {
 		t.Fatalf("counts = %+v", c)
 	}
 }
+
+func TestNotACollection(t *testing.T) {
+	s := sample(t)
+	if err := s.SetCollection(la, false); err != nil {
+		t.Fatal(err)
+	}
+	regroup(t, s) // the decision survives scans
+	if cs, _ := s.Collections(); len(cs) != 0 {
+		t.Fatalf("collections = %+v", cs)
+	}
+	f, _, _ := s.Folder(la)
+	if f.Collection || !f.Detected {
+		t.Fatalf("dismissed folder = %+v", f.FolderSummary)
+	}
+	r, _, _ := s.Folder("../musica")
+	if r.Children[0].Collection || !r.Children[0].Detected {
+		t.Fatalf("as a subfolder = %+v", r.Children[0])
+	}
+	if c, _ := s.Counts(); c.Collections != 0 {
+		t.Fatalf("counts = %+v", c)
+	}
+	if err := s.SetCollection(la, true); err != nil {
+		t.Fatal(err)
+	}
+	if cs, _ := s.Collections(); len(cs) != 1 {
+		t.Fatalf("taken back: %+v", cs)
+	}
+}
```

Modify `internal/store/migrate_test.go` (unified diff against the previous task):

```diff
@@ -48,8 +48,12 @@ func TestOpenMigratesM1Catalog(t *testing.T) {
 		VALUES ('../musica/x', '../musica', 'x', 0, 0, 0, 0)`); err != nil {
 		t.Fatal(err)
 	}
+	// Make it a v2 catalog: no not_collections yet.
+	if _, err := s.db.Exec(`DROP TABLE not_collections; PRAGMA user_version = 2`); err != nil {
+		t.Fatal(err)
+	}
 	s.Close()
-	s, err = Open(file, false) // already v2: nothing is dropped
+	s, err = Open(file, false) // v2 to v3 only adds a table: nothing is dropped
 	if err != nil {
 		t.Fatal(err)
 	}
@@ -59,6 +63,9 @@ func TestOpenMigratesM1Catalog(t *testing.T) {
 	if folders != 1 {
 		t.Fatalf("a v2 catalog lost its folders on open")
 	}
+	if err := s.SetCollection("../musica/x", false); err != nil {
+		t.Fatalf("no not_collections table: %v", err)
+	}
 }
 
 func TestReadOnlyOpenRejectsOldCatalog(t *testing.T) {
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/store ./internal/server`
Expected: FAIL — build failure: `s.SetCollection undefined`, `f.Detected undefined`.

- [ ] **Step 3: Write the code**

Modify `internal/server/server.go` (unified diff against the previous task):

```diff
@@ -7,6 +7,7 @@ import (
 	"context"
 	"encoding/json"
 	"errors"
+	"io"
 	"io/fs"
 	"log"
 	"net"
@@ -39,6 +40,7 @@ func (s *Server) Handler() http.Handler {
 	mux.HandleFunc("GET /api/albums/{id}", s.album)
 	mux.HandleFunc("GET /api/folders", s.folder)
 	mux.HandleFunc("GET /api/collections", s.collections)
+	mux.HandleFunc("PUT /api/folders/collection", jsonOnly(s.setCollection))
 	mux.HandleFunc("POST /api/identify", jsonOnly(s.identify))
 	mux.HandleFunc("GET /api/review/unidentified", s.unidentified)
 	mux.HandleFunc("PUT /api/albums/{id}/identification", jsonOnly(s.setIdentification))
@@ -197,6 +199,41 @@ func (s *Server) folder(w http.ResponseWriter, r *http.Request) {
 	}{f, art, trail})
 }
 
+// setCollection answers "no es una colección" (collection false) on a
+// detected collection, or takes it back (true), with the folder's summary.
+func (s *Server) setCollection(w http.ResponseWriter, r *http.Request) {
+	if s.ReadOnly {
+		http.Error(w, "modo consulta: no se pueden guardar cambios", http.StatusConflict)
+		return
+	}
+	var body struct {
+		Path       string `json:"path"`
+		Collection bool   `json:"collection"`
+	}
+	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&body); err != nil || body.Path == "" {
+		http.Error(w, "cuerpo inválido", http.StatusBadRequest)
+		return
+	}
+	if _, ok, err := s.Store.Folder(body.Path); err != nil || !ok {
+		if err != nil {
+			fail(w, err)
+		} else {
+			http.Error(w, "carpeta no encontrada", http.StatusNotFound)
+		}
+		return
+	}
+	if err := s.Store.SetCollection(body.Path, body.Collection); err != nil {
+		fail(w, err)
+		return
+	}
+	f, _, err := s.Store.Folder(body.Path)
+	if err != nil {
+		fail(w, err)
+		return
+	}
+	writeJSON(w, f.FolderSummary)
+}
+
 func (s *Server) collections(w http.ResponseWriter, r *http.Request) {
 	cs, err := s.Store.Collections()
 	if err != nil {
```

Modify `internal/store/catalog.go` (unified diff against the previous task):

```diff
@@ -6,6 +6,7 @@ import (
 	"errors"
 	"path"
 	"sort"
+	"time"
 
 	"tordo/internal/grouping"
 	"tordo/internal/natsort"
@@ -33,7 +34,7 @@ func (s *Store) Counts() (Counts, error) {
 		(SELECT COUNT(*) FROM tracks),
 		(SELECT COUNT(*) FROM albums),
 		(SELECT COUNT(*) FROM folders),
-		(SELECT COUNT(*) FROM folders WHERE collection = 1),
+		(SELECT COUNT(*) FROM folders WHERE `+isCollection+`),
 		(SELECT COUNT(*) FROM albums WHERE status IN ('auto', 'manual')),
 		(SELECT COUNT(*) FROM albums WHERE status = 'unmatched')`).
 		Scan(&c.Files, &c.Audio, &c.Unreadable, &c.Tracks, &c.Albums, &c.Folders, &c.Collections, &c.Identified, &c.Unidentified)
@@ -176,9 +177,16 @@ type FolderSummary struct {
 	TrackCount int    `json:"trackCount"` // recursive
 	AlbumCount int    `json:"albumCount"` // recursive
 	DurationMs int64  `json:"durationMs"`
-	Collection bool   `json:"collection"`
+	Collection bool   `json:"collection"` // detected, and the user did not say otherwise
+	Detected   bool   `json:"detected"`   // detected as a collection
 }
 
+// isCollection tells, in SQL over folders, whether a folder is a
+// collection: detected, and not dismissed by the user.
+const isCollection = `(collection = 1 AND path NOT IN (SELECT path FROM not_collections))`
+
+const folderCols = `path, name, tracks, albums, duration_ms, ` + isCollection + `, collection`
+
 type FolderView struct {
 	FolderSummary
 	Parent   string          `json:"parent"`
@@ -311,8 +319,8 @@ func (s *Store) Album(id int64) (AlbumDetail, bool, error) {
 func (s *Store) Folder(p string) (FolderView, bool, error) {
 	v := FolderView{Children: []FolderSummary{}, Albums: []AlbumView{}, Loose: []TrackView{}}
 	if p != "" {
-		err := s.db.QueryRow(`SELECT path, name, parent, tracks, albums, duration_ms, collection FROM folders WHERE path = ?`, p).
-			Scan(&v.Path, &v.Name, &v.Parent, &v.TrackCount, &v.AlbumCount, &v.DurationMs, &v.Collection)
+		err := s.db.QueryRow(`SELECT `+folderCols+`, parent FROM folders WHERE path = ?`, p).
+			Scan(&v.Path, &v.Name, &v.TrackCount, &v.AlbumCount, &v.DurationMs, &v.Collection, &v.Detected, &v.Parent)
 		if errors.Is(err, sql.ErrNoRows) {
 			return FolderView{}, false, nil
 		}
@@ -340,7 +348,7 @@ func (s *Store) Folder(p string) (FolderView, bool, error) {
 }
 
 func (s *Store) folderSummaries(where string, args ...any) ([]FolderSummary, error) {
-	rows, err := s.db.Query(`SELECT path, name, tracks, albums, duration_ms, collection FROM folders `+where, args...)
+	rows, err := s.db.Query(`SELECT `+folderCols+` FROM folders `+where, args...)
 	if err != nil {
 		return nil, err
 	}
@@ -348,7 +356,7 @@ func (s *Store) folderSummaries(where string, args ...any) ([]FolderSummary, err
 	out := []FolderSummary{}
 	for rows.Next() {
 		var f FolderSummary
-		if err := rows.Scan(&f.Path, &f.Name, &f.TrackCount, &f.AlbumCount, &f.DurationMs, &f.Collection); err != nil {
+		if err := rows.Scan(&f.Path, &f.Name, &f.TrackCount, &f.AlbumCount, &f.DurationMs, &f.Collection, &f.Detected); err != nil {
 			return nil, err
 		}
 		out = append(out, f)
@@ -358,7 +366,7 @@ func (s *Store) folderSummaries(where string, args ...any) ([]FolderSummary, err
 
 // Collections lists the detected collections with their albums in order.
 func (s *Store) Collections() ([]CollectionView, error) {
-	sums, err := s.folderSummaries(`WHERE collection = 1`)
+	sums, err := s.folderSummaries(`WHERE ` + isCollection)
 	if err != nil {
 		return nil, err
 	}
@@ -381,6 +389,20 @@ func (s *Store) Collections() ([]CollectionView, error) {
 	return out, nil
 }
 
+// SetCollection records whether a detected collection is one: false
+// dismisses it ("no es una colección"), true takes that back. The decision
+// is kept by path, across scans.
+func (s *Store) SetCollection(path string, collection bool) error {
+	var err error
+	if collection {
+		_, err = s.db.Exec(`DELETE FROM not_collections WHERE path = ?`, path)
+	} else {
+		_, err = s.db.Exec(`INSERT INTO not_collections (path, updated_at) VALUES (?, ?)
+			ON CONFLICT(path) DO UPDATE SET updated_at = excluded.updated_at`, path, time.Now().UnixMilli())
+	}
+	return err
+}
+
 // sortByPosition orders albums by their collection position (albums without
 // one last), then by folder name in natural order.
 func sortByPosition(as []AlbumView) {
```

Modify `internal/store/schema.sql` (unified diff against the previous task):

```diff
@@ -139,6 +139,12 @@ CREATE TABLE IF NOT EXISTS artists (
 	fetched_at INTEGER NOT NULL
 );
 
+-- Folders the user says are not collections, though detected as such.
+CREATE TABLE IF NOT EXISTS not_collections (
+	path       TEXT PRIMARY KEY,
+	updated_at INTEGER NOT NULL
+);
+
 -- The user's genre mappings: raw value (genres.Key) → genre/style.
 CREATE TABLE IF NOT EXISTS genre_alias (
 	raw   TEXT PRIMARY KEY,
```

Modify `internal/store/store.go` (unified diff against the previous task):

```diff
@@ -36,9 +36,13 @@ func OpenMemory() (*Store, error) {
 }
 
 // schemaVersion is kept in PRAGMA user_version. Version 2 (M2) added columns
-// to the derived tables: they are dropped and the next scan rebuilds them.
-// The other tables are only ever added to.
-const schemaVersion = 2
+// to the derived tables: below it they are dropped and the next scan
+// rebuilds them. Version 3 (M3b) only added not_collections. The other
+// tables are only ever added to.
+const schemaVersion = 3
+
+// derivedVersion is the last version that changed the derived tables.
+const derivedVersion = 2
 
 // ErrOldCatalog means a catalog from an older version was opened read-only,
 // so it could not be updated.
@@ -67,7 +71,7 @@ func migrate(db *sql.DB) error {
 	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
 		return err
 	}
-	if v < schemaVersion {
+	if v < derivedVersion {
 		if _, err := db.Exec(`DROP TABLE IF EXISTS tracks; DROP TABLE IF EXISTS album_genres;
 			DROP TABLE IF EXISTS albums; DROP TABLE IF EXISTS folders`); err != nil {
 			return err
```

- [ ] **Step 4: Run the checks**

```bash
gofmt -l internal cmd   # prints nothing
go vet ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go internal/store/catalog.go internal/store/catalog_test.go internal/store/migrate_test.go internal/store/schema.sql internal/store/store.go
git commit -m "feat: no es una colección, kept across scans (schema v3)"
```

### Task 3: Catalog snapshot and change counter

Explorar, the artist pages and the search work on the whole catalog in memory (as Cinexplorer does), rebuilt only when the store changed. `Snapshot` reads albums (with genres), tracks, folders and MusicBrainz artists in one transaction; `Changes` is SQLite's `total_changes()` on the catalog's single connection, a cheap "did anything change" check. The album, genre and document readers take a `querier` so they run inside the transaction (with `SetMaxOpenConns(1)`, a query through `s.db` while the transaction holds the connection would block). `trackQuery` gains album id, folder and first-seen time, scanned with `trackDest`.

**Files:**
- Modify: `internal/store/catalog.go`
- Modify: `internal/store/releases.go`
- Create: `internal/store/snapshot.go`
- Create: `internal/store/snapshot_test.go`

**Interfaces:**
- Consumes: `store.AlbumView`, `store.TrackView`, `store.FolderSummary`, `store.Artist`, `querier` (`internal/store/files.go`).
- Produces:
  - `store.SnapTrack{TrackView; AlbumID int64; Dir string; Added int64}`, `store.SnapFolder{FolderSummary; Parent string}`.
  - `store.Snapshot{Albums []AlbumView; Tracks []SnapTrack; Folders []SnapFolder; Artists map[string]Artist; Changes int64}`.
  - `(*Store).Snapshot() (Snapshot, error)`, `(*Store).Changes() (int64, error)`.
  - Package-level `readAlbums(q querier, query string, args ...any)`, `scanAlbums`, `fillGenres(q, as)`, `loadDocs[T](q querier, table)`, `trackDest(t *TrackView, albumID *int64, dir *string, added *int64) []any`.

- [ ] **Step 1: Write the failing tests**

Create `internal/store/snapshot_test.go`:

```go
package store

import "testing"

func TestSnapshot(t *testing.T) {
	s := sample(t)
	id := albumID(t, s, "Signos")
	if err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Status: StatusAuto, ReleaseID: "r1", ArtistID: "a1",
		Genres: []GenreLink{{Genre: "Rock", Source: "mb"}}}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveArtist(Artist{ID: "a1", Name: "Soda Stereo", Country: "AR"}); err != nil {
		t.Fatal(err)
	}
	before, err := s.Changes()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Changes != before {
		t.Fatalf("changes %d, counter %d", snap.Changes, before)
	}
	if len(snap.Albums) != 3 || len(snap.Tracks) != 5 || len(snap.Folders) != 5 || snap.Artists["a1"].Country != "AR" {
		t.Fatalf("snapshot: %d albums, %d tracks, %d folders, artists %v", len(snap.Albums), len(snap.Tracks), len(snap.Folders), snap.Artists)
	}
	var signos AlbumView
	for _, a := range snap.Albums {
		if a.ID == id {
			signos = a
		}
	}
	if len(signos.Genres) != 1 || signos.Genres[0].Genre != "Rock" {
		t.Fatalf("genres = %+v", signos.Genres)
	}
	loose := 0
	for _, tr := range snap.Tracks {
		if tr.Added == 0 || tr.Dir == "" || tr.Format != "mp3" {
			t.Fatalf("track = %+v", tr)
		}
		if tr.AlbumID == 0 {
			loose++
		}
	}
	if loose != 1 {
		t.Fatalf("loose tracks = %d", loose)
	}
	roots := 0
	for _, f := range snap.Folders {
		if f.Parent == "" {
			roots++
		}
	}
	if roots != 1 {
		t.Fatalf("roots = %d", roots)
	}
	if err := s.SetCollection(la, false); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.Changes(); after == before {
		t.Fatal("a write did not move the counter")
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/store`
Expected: FAIL — build failure: `s.Snapshot undefined`, `s.Changes undefined`.

- [ ] **Step 3: Write the code**

Modify `internal/store/catalog.go` (unified diff against the previous task):

```diff
@@ -209,15 +209,20 @@ const albumCols = `id, CASE WHEN mb_title != '' THEN mb_title ELSE title END,
 const hasPicture = `(cover_path != '' OR cover_file_id != 0 OR cover_cache != '')`
 
 func (s *Store) queryAlbums(q string, args ...any) ([]AlbumView, error) {
-	out, err := s.scanAlbums(q, args...)
+	return readAlbums(s.db, q, args...)
+}
+
+// readAlbums reads albums (albumCols) with their genres through q.
+func readAlbums(q querier, query string, args ...any) ([]AlbumView, error) {
+	out, err := scanAlbums(q, query, args...)
 	if err != nil {
 		return nil, err
 	}
-	return out, s.fillGenres(out)
+	return out, fillGenres(q, out)
 }
 
-func (s *Store) scanAlbums(q string, args ...any) ([]AlbumView, error) {
-	rows, err := s.db.Query(q, args...)
+func scanAlbums(q querier, query string, args ...any) ([]AlbumView, error) {
+	rows, err := q.Query(query, args...)
 	if err != nil {
 		return nil, err
 	}
@@ -241,7 +246,7 @@ func (s *Store) scanAlbums(q string, args ...any) ([]AlbumView, error) {
 
 // fillGenres sets the albums' genre links. It reads the whole (small)
 // album_genres table once instead of once per album.
-func (s *Store) fillGenres(as []AlbumView) error {
+func fillGenres(q querier, as []AlbumView) error {
 	if len(as) == 0 {
 		return nil
 	}
@@ -250,7 +255,7 @@ func (s *Store) fillGenres(as []AlbumView) error {
 		at[as[i].ID] = i
 		as[i].Genres = []GenreLink{}
 	}
-	rows, err := s.db.Query(`SELECT album_id, genre, style, source FROM album_genres ORDER BY album_id, genre, style`)
+	rows, err := q.Query(`SELECT album_id, genre, style, source FROM album_genres ORDER BY album_id, genre, style`)
 	if err != nil {
 		return err
 	}
@@ -270,7 +275,8 @@ func (s *Store) fillGenres(as []AlbumView) error {
 
 const trackQuery = `SELECT t.file_id, f.path, CASE WHEN t.mb_title != '' THEN t.mb_title ELSE t.title END, t.title,
 	t.recording_id, t.artist, t.disc, t.track, t.year, t.duration_ms,
-	a.format, a.codec, a.lossless, a.bitrate, a.sample_rate, a.bit_depth, f.fingerprint
+	a.format, a.codec, a.lossless, a.bitrate, a.sample_rate, a.bit_depth, f.fingerprint,
+	COALESCE(t.album_id, 0), t.dir, f.first_seen
 	FROM tracks t JOIN files f ON f.id = t.file_id JOIN audio a ON a.file_id = t.file_id `
 
 func (s *Store) queryTracks(where string, args ...any) ([]TrackView, error) {
@@ -282,8 +288,9 @@ func (s *Store) queryTracks(where string, args ...any) ([]TrackView, error) {
 	out := []TrackView{}
 	for rows.Next() {
 		var t TrackView
-		if err := rows.Scan(&t.ID, &t.Path, &t.Title, &t.TagTitle, &t.RecordingID, &t.Artist, &t.Disc, &t.Track, &t.Year,
-			&t.DurationMs, &t.Format, &t.Codec, &t.Lossless, &t.Bitrate, &t.SampleRate, &t.BitDepth, &t.Fingerprint); err != nil {
+		var albumID, added int64
+		var dir string
+		if err := rows.Scan(trackDest(&t, &albumID, &dir, &added)...); err != nil {
 			return nil, err
 		}
 		out = append(out, t)
@@ -291,6 +298,14 @@ func (s *Store) queryTracks(where string, args ...any) ([]TrackView, error) {
 	return out, rows.Err()
 }
 
+// trackDest lists where a row of trackQuery goes: the view, then its album
+// id, folder and first-seen time.
+func trackDest(t *TrackView, albumID *int64, dir *string, added *int64) []any {
+	return []any{&t.ID, &t.Path, &t.Title, &t.TagTitle, &t.RecordingID, &t.Artist, &t.Disc, &t.Track, &t.Year,
+		&t.DurationMs, &t.Format, &t.Codec, &t.Lossless, &t.Bitrate, &t.SampleRate, &t.BitDepth, &t.Fingerprint,
+		albumID, dir, added}
+}
+
 // Albums lists the albums, all of them or those inside folder (at any depth).
 func (s *Store) Albums(folder string) ([]AlbumView, error) {
 	q := `SELECT ` + albumCols + ` FROM albums`
```

Modify `internal/store/releases.go` (unified diff against the previous task):

```diff
@@ -60,11 +60,11 @@ type Artist struct {
 
 func (s *Store) SaveRelease(r Release) error { return s.saveDoc("releases", r.ID, r) }
 
-func (s *Store) Releases() (map[string]Release, error) { return loadDocs[Release](s, "releases") }
+func (s *Store) Releases() (map[string]Release, error) { return loadDocs[Release](s.db, "releases") }
 
 func (s *Store) SaveArtist(a Artist) error { return s.saveDoc("artists", a.ID, a) }
 
-func (s *Store) Artists() (map[string]Artist, error) { return loadDocs[Artist](s, "artists") }
+func (s *Store) Artists() (map[string]Artist, error) { return loadDocs[Artist](s.db, "artists") }
 
 // saveDoc and loadDocs keep JSON documents in a table (id, data,
 // fetched_at). table is always one of the constants above, never input.
@@ -78,8 +78,8 @@ func (s *Store) saveDoc(table, id string, v any) error {
 	return err
 }
 
-func loadDocs[T any](s *Store, table string) (map[string]T, error) {
-	rows, err := s.db.Query(`SELECT id, data FROM ` + table)
+func loadDocs[T any](q querier, table string) (map[string]T, error) {
+	rows, err := q.Query(`SELECT id, data FROM ` + table)
 	if err != nil {
 		return nil, err
 	}
```

Create `internal/store/snapshot.go`:

```go
package store

// SnapTrack is a track as the catalog's pages see it.
type SnapTrack struct {
	TrackView
	AlbumID int64  `json:"albumId"` // 0: a loose track
	Dir     string `json:"dir"`
	Added   int64  `json:"added"` // when its file was first seen, unix ms
}

// SnapFolder is a folder with its parent ("" for a root).
type SnapFolder struct {
	FolderSummary
	Parent string `json:"parent"`
}

// Snapshot is the whole catalog as Explorar, the artist pages and the
// search see it, read at one point in time.
type Snapshot struct {
	Albums  []AlbumView       // by id, with their genres
	Tracks  []SnapTrack       // by file id
	Folders []SnapFolder      // by path
	Artists map[string]Artist // MusicBrainz artists by id
	// Changes counts the rows written through the catalog's connection since
	// it was opened: it differs between two snapshots when the catalog
	// changed in between (it restarts if the connection does).
	Changes int64
}

// Changes is the change counter of Snapshot, read on its own: a cheap way to
// know whether what was derived from the last snapshot is still current.
func (s *Store) Changes() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT total_changes()`).Scan(&n)
	return n, err
}

// Snapshot reads the catalog in one transaction, so that it comes from one
// state even while the scanner or the identification write. The catalog has
// a single connection, which the transaction holds: everything here reads
// through tx.
func (s *Store) Snapshot() (Snapshot, error) {
	var snap Snapshot
	tx, err := s.db.Begin()
	if err != nil {
		return snap, err
	}
	defer tx.Rollback()
	if err := tx.QueryRow(`SELECT total_changes()`).Scan(&snap.Changes); err != nil {
		return snap, err
	}
	if snap.Albums, err = readAlbums(tx, `SELECT `+albumCols+` FROM albums ORDER BY id`); err != nil {
		return snap, err
	}
	if snap.Tracks, err = snapTracks(tx); err != nil {
		return snap, err
	}
	if snap.Folders, err = snapFolders(tx); err != nil {
		return snap, err
	}
	if snap.Artists, err = loadDocs[Artist](tx, "artists"); err != nil {
		return snap, err
	}
	return snap, nil
}

func snapTracks(q querier) ([]SnapTrack, error) {
	rows, err := q.Query(trackQuery + ` ORDER BY t.file_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SnapTrack{}
	for rows.Next() {
		var t SnapTrack
		if err := rows.Scan(trackDest(&t.TrackView, &t.AlbumID, &t.Dir, &t.Added)...); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func snapFolders(q querier) ([]SnapFolder, error) {
	rows, err := q.Query(`SELECT ` + folderCols + `, parent FROM folders ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SnapFolder{}
	for rows.Next() {
		var f SnapFolder
		if err := rows.Scan(&f.Path, &f.Name, &f.TrackCount, &f.AlbumCount, &f.DurationMs, &f.Collection, &f.Detected, &f.Parent); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run the checks**

```bash
gofmt -l internal cmd   # prints nothing
go vet ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/store/catalog.go internal/store/releases.go internal/store/snapshot.go internal/store/snapshot_test.go
git commit -m "feat: catalog snapshot and change counter"
```

### Task 4: The catalog: albums, artists and tracks with facets

A new pure package `internal/catalog` (adapted from Cinexplorer's) arranges a snapshot for Explorar.

- **Albums** carry their facet values: decade and year (the original year when MusicBrainz knows it), artist key, country, genres and styles, the collections they are in, Carpeta (the root and first-level folder, like Cinexplorer's Ubicación), quality (lossless/lossy), format (`FormatKey`: the codec, the container for PCM, `wma` for every WMA) and state (`sin-identificar`, `no-reproducible` when a track needs ffmpeg and there is none).
- **Artists** are album artists: the MusicBrainz id when the album is identified, else the folded name (`nameKey`: "fito-paez"). A name-only album joins the MusicBrainz artist whose name (MusicBrainz's own, or as their albums credit or tag it) is that name, when exactly one has it: "Charly Garcia" and "Charly García" are one artist, and "soda stereo" joins "Soda Stereo & Andrea Echeverri"'s id. Compilations have no artist.
- **Tracks** inherit their album's facets; their own artist joins an artist when the name matches (compilation tracks count for their artist); loose tracks get their own year and folders.
- `Explore` filters a tab, counts every facet over the other facets (the decade menu also ignores the year, the genre menu the style; the style menu lists only the chosen genre's styles), sorts with stable tie-breaks and pages. The Artistas tab lists the album artists of the matching albums.

**Files:**
- Create: `internal/catalog/catalog.go`
- Create: `internal/catalog/catalog_test.go`
- Create: `internal/catalog/facets.go`

**Interfaces:**
- Consumes: `store.Snapshot` (Task 3), `nameparse.Norm`, `nameparse.IsVarious`, `grouping.Various`, `genres.Valid`.
- Produces (`internal/catalog`):
  - `Album`, `Artist`, `Track` (JSON for the pages; `Track` embeds `store.TrackView` without path and fingerprint, plus `albumId`, `album`, `dir`, `artistKey`, `added`).
  - `Options{Playable func(store.TrackView) bool}`; `Build(snap store.Snapshot, opt Options) *Catalog`.
  - `(*Catalog).Album(id int64) (*Album, bool)`, `.Artist(key string) (*Artist, bool)`, `.Track(id int64) (*Track, bool)`.
  - Facet constants `FacetDecade` (`decada`), `FacetYear` (`anio`), `FacetArtist` (`artista`), `FacetCountry` (`pais`), `FacetGenre` (`genero`), `FacetStyle` (`estilo`), `FacetCollection` (`coleccion`), `FacetFolder` (`carpeta`), `FacetQuality` (`calidad`), `FacetFormat` (`formato`), `FacetState` (`estado`); values `QualityLossless`/`QualityLossy` (`sin-perdida`/`con-perdida`), `StateUnidentified`/`StateUnplayable` (`sin-identificar`/`no-reproducible`).
  - Views `ViewAlbums`/`ViewArtists`/`ViewTracks` (`albumes`/`artistas`/`pistas`); orders `OrderYear` (`anio`), `OrderTitle` (`titulo`), `OrderArtist` (`artista`), `OrderAdded` (`agregado`), `OrderCount` (`cantidad`); `Asc`/`Desc`.
  - `Query{View, Facets, Order, Dir}`; `ParseQuery(url.Values) Query` (params `ver`, the facets, `orden`, `dir`).
  - `FacetValue{Value, Label, Count}`; `Result{Query, Total, Offset, Albums, Artists, Tracks, Facets}`; `(*Catalog).Explore(q Query, offset, limit int) Result`.
  - `FormatKey(format, codec string) string`; `sortAlbums(as []Album, order, dir string)` (used by Task 5).

- [ ] **Step 1: Write the failing tests**

Create `internal/catalog/catalog_test.go`:

```go
package catalog

import (
	"net/url"
	"reflect"
	"testing"

	"tordo/internal/store"
)

const (
	root = "../musica"
	la   = root + "/Latinoamérica 600"
	soda = "0ab49580-c84f-44d4-875f-d83760ea2cfe" // a MusicBrainz artist id
)

type fixtureAlbum struct {
	album  store.AlbumView
	tracks []store.SnapTrack
}

func tr(id int64, title, artist, format, codec string, lossless bool) store.SnapTrack {
	return store.SnapTrack{TrackView: store.TrackView{ID: id, Title: title, Artist: artist, Format: format, Codec: codec,
		Lossless: lossless, Track: int(id % 100), Path: "x", Fingerprint: "fp"}, Added: 1000 + id}
}

// fixture is a small catalog:
//   - Signos: identified (Soda Stereo's MusicBrainz id, credited with a guest), 1986, Latin / Nueva Cancion
//     and Rock / Pop Rock, in the collection, FLAC;
//   - Doble vida: by "soda stereo", not identified, 1988 (joins Signos' artist by MusicBrainz's name), MP3;
//   - Clics modernos and Bocanada: by "Charly Garcia" and "Charly García", no id, 1983 and 1999;
//     Bocanada in the queue (unmatched);
//   - Rock nacional: a compilation with a Soda Stereo track, 1995, WMA;
//   - a loose track in youtube/, 2020, Opus.
func fixture() store.Snapshot {
	albums := []fixtureAlbum{
		{store.AlbumView{ID: 1, Title: "Signos", Artist: "Soda Stereo & Andrea Echeverri", TagArtist: "Soda", Year: 1990, OriginalYear: 1986,
			Status: store.StatusAuto, ArtistID: soda, Country: "AR", Dir: la + "/[LA-10] Soda Stereo - Signos", Picture: true,
			Genres: []store.GenreLink{{Genre: "Latin", Style: "Nueva Cancion", Source: "discogs"},
				{Genre: "Rock", Style: "Pop Rock", Source: "discogs"}}},
			[]store.SnapTrack{tr(101, "Signos", "Soda Stereo", "flac", "flac", true), tr(102, "No existes", "Soda Stereo", "flac", "flac", true)}},
		{store.AlbumView{ID: 2, Title: "Doble vida", Artist: "soda stereo", TagArtist: "soda stereo", Year: 1988,
			Status: store.StatusAuto, Dir: root + "/Soda Stereo/Doble vida",
			Genres: []store.GenreLink{{Genre: "Rock", Source: "tag"}}},
			[]store.SnapTrack{tr(201, "En la ciudad de la furia", "Soda Stereo", "mp3", "mp3", false)}},
		{store.AlbumView{ID: 3, Title: "Clics modernos", Artist: "Charly Garcia", TagArtist: "Charly Garcia", Year: 1983,
			Dir: la + "/[LA-9] Charly García - Clics modernos"},
			[]store.SnapTrack{tr(301, "Nos siguen pegando abajo", "Charly García", "mp3", "mp3", false)}},
		{store.AlbumView{ID: 4, Title: "Bocanada", Artist: "Charly García", TagArtist: "Charly García", Year: 1999,
			Status: store.StatusUnmatched, Dir: root + "/Charly García/Bocanada"},
			[]store.SnapTrack{tr(401, "Bocanada", "Charly García", "mp3", "mp3", false)}},
		{store.AlbumView{ID: 5, Title: "Rock nacional", Artist: "Varios artistas", Compilation: true, Year: 1995,
			Dir: root + "/Compilados/Rock nacional", Picture: true},
			[]store.SnapTrack{tr(501, "Persiana americana", "Soda Stereo", "asf", "wmav2", false),
				tr(502, "Rasguña las piedras", "Sui Generis", "asf", "wmav2", false)}},
	}
	snap := store.Snapshot{Changes: 7, Artists: map[string]store.Artist{
		soda: {ID: soda, Name: "Soda Stereo", Country: "AR", Type: "Group", Begin: "1982", End: "2007"}}}
	for _, f := range []struct{ path, parent, name string }{
		{root, "", "musica"}, {la, root, "Latinoamérica 600"},
		{la + "/[LA-10] Soda Stereo - Signos", la, "[LA-10] Soda Stereo - Signos"},
		{la + "/[LA-9] Charly García - Clics modernos", la, "[LA-9] Charly García - Clics modernos"},
		{root + "/Soda Stereo", root, "Soda Stereo"}, {root + "/Soda Stereo/Doble vida", root + "/Soda Stereo", "Doble vida"},
		{root + "/Charly García", root, "Charly García"}, {root + "/Charly García/Bocanada", root + "/Charly García", "Bocanada"},
		{root + "/Compilados", root, "Compilados"}, {root + "/Compilados/Rock nacional", root + "/Compilados", "Rock nacional"},
		{root + "/youtube", root, "youtube"},
	} {
		snap.Folders = append(snap.Folders, store.SnapFolder{FolderSummary: store.FolderSummary{Path: f.path, Name: f.name,
			TrackCount: 1, Collection: f.path == la}, Parent: f.parent})
	}
	for _, a := range albums {
		a.album.Tracks = len(a.tracks)
		snap.Albums = append(snap.Albums, a.album)
		for _, t := range a.tracks {
			t.AlbumID, t.Dir = a.album.ID, a.album.Dir
			t.Year = a.album.Year
			snap.Tracks = append(snap.Tracks, t)
		}
	}
	loose := tr(901, "Juízo Final", "Nelson Cavaquinho", "ogg", "opus", false)
	loose.Dir, loose.Year = root+"/youtube", 2020
	snap.Tracks = append(snap.Tracks, loose)
	return snap
}

// wmaNeedsFFmpeg is the Playable of a server without ffmpeg.
func wmaNeedsFFmpeg(t store.TrackView) bool { return t.Format != "asf" }

func build() *Catalog { return Build(fixture(), Options{Playable: wmaNeedsFFmpeg}) }

func albumTitles(as []Album) []string {
	out := []string{}
	for _, a := range as {
		out = append(out, a.Title)
	}
	return out
}

func TestArtistKeys(t *testing.T) {
	c := build()
	key := func(id int64) string { a, _ := c.Album(id); return a.ArtistKey }
	if key(1) != soda || key(2) != soda {
		t.Fatalf("Soda Stereo: %q and %q", key(1), key(2))
	}
	if key(3) != "charly-garcia" || key(4) != "charly-garcia" {
		t.Fatalf("Charly García: %q and %q", key(3), key(4))
	}
	if al, _ := c.Album(5); al.ArtistKey != "" || al.Artist != "Varios artistas" || !al.Compilation {
		t.Fatalf("compilation = %+v", al)
	}
	if len(c.Artists) != 2 {
		t.Fatalf("artists = %+v", c.Artists)
	}
	ch, _ := c.Artist("charly-garcia")
	if ch.Albums != 2 || ch.Name != "Charly Garcia" || ch.Cover != 0 {
		t.Fatalf("Charly = %+v", ch) // as many albums with each spelling: the first in byte order
	}
	s, _ := c.Artist(soda)
	if s.Name != "Soda Stereo" || s.Country != "AR" || s.Albums != 2 || s.Cover != 1 {
		t.Fatalf("Soda = %+v", s)
	}
}

func TestAlbumFacetValues(t *testing.T) {
	c := build()
	signos, _ := c.Album(1)
	want := map[string][]string{
		FacetDecade: {"1980"}, FacetYear: {"1986"}, FacetArtist: {soda}, FacetCountry: {"AR"},
		FacetGenre: {"Latin", "Rock"}, FacetStyle: {"Nueva Cancion", "Pop Rock"}, FacetCollection: {la}, FacetFolder: {root, la},
		FacetQuality: {QualityLossless}, FacetFormat: {"flac"},
	}
	if !reflect.DeepEqual(signos.values, want) {
		t.Fatalf("Signos:\n got %v\nwant %v", signos.values, want)
	}
	if signos.Year != 1986 {
		t.Fatalf("year = %d, want the original", signos.Year)
	}
	comp, _ := c.Album(5)
	if !reflect.DeepEqual(comp.values[FacetFormat], []string{"wma"}) || !reflect.DeepEqual(comp.values[FacetState], []string{StateUnplayable}) {
		t.Fatalf("compilation: %v", comp.values)
	}
	boca, _ := c.Album(4)
	if !reflect.DeepEqual(boca.values[FacetState], []string{StateUnidentified}) {
		t.Fatalf("Bocanada: %v", boca.values)
	}
	if c.Albums[0].Added != 1101 {
		t.Fatalf("added = %d", c.Albums[0].Added)
	}
}

func TestExploreAlbums(t *testing.T) {
	c := build()
	r := c.Explore(ParseQuery(url.Values{"decada": {"1980"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Doble vida", "Signos", "Clics modernos"}) || r.Total != 3 {
		t.Fatalf("1980s, newest first: %q (total %d)", got, r.Total)
	}
	// A menu counts over the other facets: the decade menu shows every
	// decade, the others only what the 1980s have.
	if got := r.Facets[FacetDecade]; len(got) != 2 || got[0] != (FacetValue{"1990", "1990", 2}) || got[1] != (FacetValue{"1980", "1980", 3}) {
		t.Fatalf("decades = %+v", got)
	}
	if got := r.Facets[FacetArtist]; len(got) != 2 || got[0] != (FacetValue{soda, "Soda Stereo", 2}) || got[1].Label != "Charly Garcia" {
		t.Fatalf("artists = %+v", got)
	}
	if got := r.Facets[FacetCollection]; len(got) != 1 || got[0] != (FacetValue{la, "Latinoamérica 600", 2}) {
		t.Fatalf("collections = %+v", got)
	}
	if got := r.Facets[FacetFolder]; got[0] != (FacetValue{root, "musica", 3}) || got[1].Label != "musica/Latinoamérica 600" {
		t.Fatalf("folders = %+v", got)
	}
	// Choosing a decade keeps its menu whole even with a year chosen.
	r = c.Explore(ParseQuery(url.Values{"decada": {"1980"}, "anio": {"1983"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Clics modernos"}) || len(r.Facets[FacetDecade]) != 2 {
		t.Fatalf("1983: %q, decades %+v", got, r.Facets[FacetDecade])
	}
	r = c.Explore(ParseQuery(url.Values{"genero": {"Rock"}, "estilo": {"Pop Rock"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Signos"}) || len(r.Facets[FacetGenre]) != 2 || r.Facets[FacetGenre][0] != (FacetValue{"Rock", "Rock", 2}) {
		t.Fatalf("Pop Rock: %q, genres %+v", got, r.Facets[FacetGenre])
	}
	// Signos is Latin too: the style menu lists only Rock's styles.
	if got := r.Facets[FacetStyle]; len(got) != 1 || got[0] != (FacetValue{"Pop Rock", "Pop Rock", 1}) {
		t.Fatalf("styles = %+v", got)
	}
	r = c.Explore(ParseQuery(url.Values{"estado": {StateUnplayable}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Rock nacional"}) {
		t.Fatalf("unplayable: %q", got)
	}
	r = c.Explore(ParseQuery(url.Values{"orden": {"titulo"}}), 1, 2)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Clics modernos", "Doble vida"}) || r.Total != 5 || r.Offset != 1 {
		t.Fatalf("page 2 by title: %q, total %d", got, r.Total)
	}
	r = c.Explore(ParseQuery(url.Values{"orden": {"artista"}, "dir": {"desc"}}), 0, 100)
	if got := albumTitles(r.Albums); !reflect.DeepEqual(got, []string{"Rock nacional", "Signos", "Doble vida", "Bocanada", "Clics modernos"}) {
		t.Fatalf("by artist, Z to A: %q", got)
	}
	if r := c.Explore(ParseQuery(nil), 10, 100); r.Albums == nil || len(r.Albums) != 0 || r.Total != 5 {
		t.Fatalf("past the end: %+v", r)
	}
}

func TestExploreArtistsAndTracks(t *testing.T) {
	c := build()
	r := c.Explore(ParseQuery(url.Values{"ver": {"artistas"}, "decada": {"1980"}}), 0, 100)
	if len(r.Artists) != 2 || r.Artists[0].Name != "Charly Garcia" || r.Artists[0].Albums != 1 || r.Artists[1].Albums != 2 {
		t.Fatalf("artists of the 1980s: %+v", r.Artists)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"artistas"}, "orden": {"cantidad"}}), 0, 100)
	if r.Artists[0].Key != "charly-garcia" || r.Artists[1].Key != soda { // as many albums: by name
		t.Fatalf("by number of albums: %+v", r.Artists)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"pistas"}, "artista": {soda}}), 0, 100)
	got := []string{}
	for _, tr := range r.Tracks {
		got = append(got, tr.Title)
	}
	// Soda Stereo's tracks, in the compilation too; by artist, then year.
	if want := []string{"Signos", "No existes", "En la ciudad de la furia", "Persiana americana"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tracks = %q, want %q", got, want)
	}
	if tr := r.Tracks[3]; tr.Album != "Rock nacional" || tr.ArtistKey != soda || tr.Path != "" || tr.Year != 1995 {
		t.Fatalf("compilation track = %+v", tr)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"pistas"}, "carpeta": {root + "/youtube"}}), 0, 100)
	if len(r.Tracks) != 1 || r.Tracks[0].Title != "Juízo Final" || r.Tracks[0].AlbumID != 0 || r.Tracks[0].Dir != root+"/youtube" {
		t.Fatalf("loose track: %+v", r.Tracks)
	}
	if got := r.Facets[FacetDecade]; len(got) != 1 || got[0].Value != "2020" {
		t.Fatalf("loose track decade: %+v", got)
	}
	r = c.Explore(ParseQuery(url.Values{"ver": {"pistas"}, "formato": {"wma"}}), 0, 100)
	if len(r.Tracks) != 2 || r.Facets[FacetState][0] != (FacetValue{StateUnplayable, StateUnplayable, 2}) {
		t.Fatalf("WMA tracks: %d, states %+v", len(r.Tracks), r.Facets[FacetState])
	}
}

func TestParseQuery(t *testing.T) {
	q := ParseQuery(url.Values{"decada": {"1985"}, "anio": {"1986"}, "pais": {"ar"}, "calidad": {"mala"}, "x": {"y"},
		"orden": {"cantidad"}, "dir": {"asc"}})
	want := Query{View: ViewAlbums, Facets: map[string]string{FacetYear: "1986"}, Order: OrderYear, Dir: Asc}
	if !reflect.DeepEqual(q, want) {
		t.Fatalf("q = %+v", q)
	}
	q = ParseQuery(url.Values{"ver": {"artistas"}, "orden": {"cantidad"}})
	if q.View != ViewArtists || q.Order != OrderCount || q.Dir != Desc {
		t.Fatalf("artists = %+v", q)
	}
	if q := ParseQuery(url.Values{"ver": {"pistas"}}); q.Order != OrderArtist || q.Dir != Asc {
		t.Fatalf("tracks = %+v", q)
	}
}

func TestFormatKey(t *testing.T) {
	for _, c := range []struct{ format, codec, want string }{
		{"mp3", "mp3", "mp3"}, {"asf", "wmapro", "wma"}, {"wav", "pcm_s16le", "wav"}, {"aiff", "pcm_float", "aiff"},
		{"mp4", "alac", "alac"}, {"ogg", "", "ogg"},
	} {
		if got := FormatKey(c.format, c.codec); got != c.want {
			t.Errorf("FormatKey(%q, %q) = %q", c.format, c.codec, got)
		}
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/catalog`
Expected: FAIL — build failure: package `tordo/internal/catalog` has no non-test Go files / `undefined: Build`.

- [ ] **Step 3: Write the code**

Create `internal/catalog/catalog.go`:

```go
// Package catalog turns a snapshot of the store into what Explorar, the
// artist pages and the search show: albums, artists and tracks with the
// values of their facets. Everything here is pure: no SQL, no network.
package catalog

import (
	"cmp"
	"path"
	"slices"
	"strconv"
	"strings"

	"tordo/internal/grouping"
	"tordo/internal/nameparse"
	"tordo/internal/store"
)

// Album is one album card of Explorar.
type Album struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	ArtistKey   string `json:"artistKey"` // "" for compilations and albums without an artist
	Year        int    `json:"year"`      // the original year when known, else the tags'
	Compilation bool   `json:"compilation"`
	Tracks      int    `json:"tracks"`
	DurationMs  int64  `json:"durationMs"`
	Picture     bool   `json:"picture"`
	Country     string `json:"country"`
	Status      string `json:"status"`
	Dir         string `json:"dir"`
	Added       int64  `json:"added"` // the first time one of its files was seen, unix ms

	values map[string][]string // by facet
	labels map[string]string   // facet + "\x00" + value → label, for values whose label is not the value
	norm   string              // title as compared when sorting
	anorm  string              // artist as compared when sorting
	tagged string              // the title and artist in its tags, when they differ (for the search)
}

// Artist is one artist card of Explorar: the album artist of one or more
// albums (compilations have none).
type Artist struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Albums  int    `json:"albums"`
	Cover   int64  `json:"cover"` // an album with a picture, for the card; 0 without one
	Added   int64  `json:"added"`

	mbid  string
	norm  string
	years []int
	names []string // every spelling of its name in the albums
}

// Track is one row of Explorar's Pistas.
type Track struct {
	store.TrackView
	AlbumID   int64  `json:"albumId"` // 0: a loose track
	Album     string `json:"album"`
	Dir       string `json:"dir"`
	ArtistKey string `json:"artistKey"` // the track artist's page, when there is one
	Added     int64  `json:"added"`

	values map[string][]string
	labels map[string]string
	norm   string
	anorm  string
	order  string // album, disc and track, for the album order of equal years
}

// Options are what building the catalog needs besides the snapshot.
type Options struct {
	// Playable tells whether the server can deliver a track (formats that
	// need ffmpeg are not playable without it). Nil: everything is.
	Playable func(store.TrackView) bool
}

// Catalog is a snapshot arranged for the pages.
type Catalog struct {
	Albums  []Album  // by id
	Artists []Artist // by name
	Tracks  []Track  // by file id
	Changes int64    // the snapshot's change counter

	albumIndex  map[int64]int
	artistIndex map[string]int
	trackIndex  map[int64]int
	artistNames map[string]string // normalized name → artist key, for track artists
	folders     map[string]store.SnapFolder
	snapArtists map[string]store.Artist
}

// Build arranges a snapshot.
func Build(snap store.Snapshot, opt Options) *Catalog {
	c := &Catalog{Changes: snap.Changes, albumIndex: map[int64]int{}, artistIndex: map[string]int{}, trackIndex: map[int64]int{},
		folders: map[string]store.SnapFolder{}, snapArtists: snap.Artists}
	for _, f := range snap.Folders {
		c.folders[f.Path] = f
	}
	tracksOf := map[int64][]store.SnapTrack{}
	for _, t := range snap.Tracks {
		if t.AlbumID != 0 {
			tracksOf[t.AlbumID] = append(tracksOf[t.AlbumID], t)
		}
	}
	keys := artistKeys(snap.Albums, snap.Artists)
	c.Albums = make([]Album, 0, len(snap.Albums))
	for _, a := range snap.Albums {
		c.albumIndex[a.ID] = len(c.Albums)
		c.Albums = append(c.Albums, c.album(a, keys[a.ID], tracksOf[a.ID], opt))
	}
	c.buildArtists(snap.Artists)
	c.Tracks = make([]Track, 0, len(snap.Tracks))
	for _, t := range snap.Tracks {
		c.trackIndex[t.ID] = len(c.Tracks)
		c.Tracks = append(c.Tracks, c.track(t, opt))
	}
	return c
}

// Track returns a track by id.
func (c *Catalog) Track(id int64) (*Track, bool) {
	i, ok := c.trackIndex[id]
	if !ok {
		return nil, false
	}
	return &c.Tracks[i], true
}

// Album returns an album by id.
func (c *Catalog) Album(id int64) (*Album, bool) {
	i, ok := c.albumIndex[id]
	if !ok {
		return nil, false
	}
	return &c.Albums[i], true
}

// Artist returns an artist by key.
func (c *Catalog) Artist(key string) (*Artist, bool) {
	i, ok := c.artistIndex[key]
	if !ok {
		return nil, false
	}
	return &c.Artists[i], true
}

// isCompilation tells albums credited to no one artist.
func isCompilation(a store.AlbumView) bool {
	return a.Compilation || a.Artist == grouping.Various || nameparse.IsVarious(a.Artist)
}

// nameKey is the key of an artist known only by name: its folded name with
// dashes ("fito-paez"). "" when the name has no letters or digits.
func nameKey(name string) string {
	return strings.ReplaceAll(nameparse.Norm(name), " ", "-")
}

// artistKeys gives each album's artist a key: the MusicBrainz id when the
// album is identified, else the folded name. An album known only by name
// joins the MusicBrainz artist whose name (MusicBrainz's, or as their albums
// credit or tag it) is that name, when there is exactly one.
func artistKeys(albums []store.AlbumView, mb map[string]store.Artist) map[int64]string {
	byName := map[string]map[string]bool{} // folded name → MusicBrainz ids
	note := func(name, id string) {
		n := nameKey(name)
		if n == "" {
			return
		}
		if byName[n] == nil {
			byName[n] = map[string]bool{}
		}
		byName[n][id] = true
	}
	for _, a := range albums {
		if a.ArtistID != "" && !isCompilation(a) {
			note(a.Artist, a.ArtistID)
			note(a.TagArtist, a.ArtistID)
			note(mb[a.ArtistID].Name, a.ArtistID)
		}
	}
	out := map[int64]string{}
	for _, a := range albums {
		switch {
		case isCompilation(a):
		case a.ArtistID != "":
			out[a.ID] = a.ArtistID
		default:
			n := nameKey(a.Artist)
			if ids := byName[n]; len(ids) == 1 {
				for id := range ids {
					out[a.ID] = id
				}
			} else if n != "" {
				out[a.ID] = n
			}
		}
	}
	return out
}

func (c *Catalog) album(a store.AlbumView, key string, tracks []store.SnapTrack, opt Options) Album {
	year := a.OriginalYear
	if year == 0 {
		year = a.Year
	}
	artist := a.Artist
	if isCompilation(a) {
		artist = grouping.Various
	}
	out := Album{ID: a.ID, Title: a.Title, Artist: artist, ArtistKey: key, Year: year, Compilation: isCompilation(a),
		Tracks: a.Tracks, DurationMs: a.DurationMs, Picture: a.Picture, Country: a.Country, Status: a.Status, Dir: a.Dir,
		values: map[string][]string{}, labels: map[string]string{},
		norm: nameparse.Norm(a.Title), anorm: nameparse.Norm(artist)}
	if a.TagTitle != a.Title || a.TagArtist != a.Artist {
		out.tagged = a.TagTitle + " " + a.TagArtist
	}
	for _, t := range tracks {
		if out.Added == 0 || (t.Added > 0 && t.Added < out.Added) {
			out.Added = t.Added
		}
	}
	add := out.add
	if year > 0 {
		add(FacetDecade, strconv.Itoa(year/10*10), "")
		add(FacetYear, strconv.Itoa(year), "")
	}
	if key != "" {
		add(FacetArtist, key, "") // labelled with the artist's name once artists are built
	}
	if a.Country != "" {
		add(FacetCountry, a.Country, "")
	}
	for _, g := range a.Genres {
		add(FacetGenre, g.Genre, "")
		if g.Style != "" {
			add(FacetStyle, g.Style, "")
		}
	}
	for _, p := range c.collectionsOf(path.Dir(a.Dir)) {
		add(FacetCollection, p, c.folders[p].Name)
	}
	for _, p := range c.places(a.Dir) {
		add(FacetFolder, p, c.placeLabel(p))
	}
	unplayable := false
	for _, t := range tracks {
		c.formatValues(add, t.TrackView)
		if opt.Playable != nil && !opt.Playable(t.TrackView) {
			unplayable = true
		}
	}
	if a.Status == store.StatusUnmatched {
		add(FacetState, StateUnidentified, "")
	}
	if unplayable {
		add(FacetState, StateUnplayable, "")
	}
	return out
}

func (c *Catalog) formatValues(add func(facet, value, label string), t store.TrackView) {
	if t.Lossless {
		add(FacetQuality, QualityLossless, "")
	} else {
		add(FacetQuality, QualityLossy, "")
	}
	if k := FormatKey(t.Format, t.Codec); k != "" {
		add(FacetFormat, k, "")
	}
}

// add records a facet value once, with its label when it is not the value.
func (a *Album) add(facet, value, label string) { addValue(a.values, a.labels, facet, value, label) }
func (t *Track) add(facet, value, label string) { addValue(t.values, t.labels, facet, value, label) }

func addValue(values map[string][]string, labels map[string]string, facet, value, label string) {
	if slices.Contains(values[facet], value) {
		return
	}
	values[facet] = append(values[facet], value)
	if label != "" && label != value {
		labels[facet+"\x00"+value] = label
	}
}

// collectionsOf lists the collections among dir and the folders above it.
func (c *Catalog) collectionsOf(dir string) []string {
	var out []string
	for f, ok := c.folders[dir]; ok; f, ok = c.folders[f.Parent] {
		if f.Collection {
			out = append(out, f.Path)
		}
	}
	return out
}

// places gives the Carpeta values of a folder: the root it is under and,
// below the root, its first-level folder.
func (c *Catalog) places(dir string) []string {
	var chain []string // from dir up to its root
	for f, ok := c.folders[dir]; ok; f, ok = c.folders[f.Parent] {
		chain = append(chain, f.Path)
	}
	slices.Reverse(chain)
	return chain[:min(2, len(chain))]
}

// placeLabel names a Carpeta value: "musica", "musica/Brasil".
func (c *Catalog) placeLabel(p string) string {
	f := c.folders[p]
	if f.Parent == "" {
		return f.Name
	}
	return c.folders[f.Parent].Name + "/" + f.Name
}

// buildArtists makes one artist per album artist key, named as MusicBrainz
// names it or, without an id, as most of its albums spell it.
func (c *Catalog) buildArtists(mb map[string]store.Artist) {
	type acc struct {
		Artist
		spellings map[string]int
	}
	byKey := map[string]*acc{}
	var order []string
	for i := range c.Albums {
		al := &c.Albums[i]
		if al.ArtistKey == "" {
			continue
		}
		a := byKey[al.ArtistKey]
		if a == nil {
			a = &acc{Artist: Artist{Key: al.ArtistKey}, spellings: map[string]int{}}
			if m, ok := mb[al.ArtistKey]; ok {
				a.mbid, a.Name, a.Country = m.ID, m.Name, m.Country
			}
			byKey[al.ArtistKey] = a
			order = append(order, al.ArtistKey)
		}
		a.Albums++
		a.spellings[al.Artist]++
		if a.Cover == 0 && al.Picture {
			a.Cover = al.ID
		}
		if a.Country == "" {
			a.Country = al.Country
		}
		if al.Added > a.Added {
			a.Added = al.Added
		}
		if al.Year > 0 {
			a.years = append(a.years, al.Year)
		}
	}
	c.artistNames = map[string]string{}
	for _, k := range order {
		a := byKey[k]
		if a.Name == "" {
			best := 0
			for name, n := range a.spellings {
				if n > best || (n == best && name < a.Name) {
					a.Name, best = name, n
				}
			}
		}
		a.norm = nameparse.Norm(a.Name)
		for name := range a.spellings {
			c.artistNames[nameparse.Norm(name)] = k
			if name != a.Name {
				a.names = append(a.names, name)
			}
		}
		slices.Sort(a.names)
		c.Artists = append(c.Artists, a.Artist)
		c.artistNames[a.norm] = k
	}
	slices.SortFunc(c.Artists, func(x, y Artist) int { return cmp.Or(cmp.Compare(x.norm, y.norm), cmp.Compare(x.Key, y.Key)) })
	for i := range c.Artists {
		c.artistIndex[c.Artists[i].Key] = i
	}
	for i := range c.Albums {
		if k := c.Albums[i].ArtistKey; k != "" {
			c.Albums[i].labels[FacetArtist+"\x00"+k] = byKey[k].Name
		}
	}
}

func (c *Catalog) track(t store.SnapTrack, opt Options) Track {
	out := Track{TrackView: t.TrackView, AlbumID: t.AlbumID, Dir: t.Dir, Added: t.Added, values: map[string][]string{},
		labels: map[string]string{}, norm: nameparse.Norm(t.Title), anorm: nameparse.Norm(t.Artist)}
	out.TrackView.Path = "" // not needed by the pages
	out.TrackView.Fingerprint = ""
	out.ArtistKey = c.artistNames[out.anorm]
	al, ok := c.Album(t.AlbumID)
	if ok {
		out.Album = al.Title
		out.order = al.norm + "\x00" + strconv.Itoa(al.Year)
		for f, vs := range al.values {
			if f == FacetQuality || f == FacetFormat || f == FacetState {
				continue // the track's own
			}
			for _, v := range vs {
				out.add(f, v, al.labels[f+"\x00"+v])
			}
		}
	} else {
		if t.Year > 0 {
			out.add(FacetDecade, strconv.Itoa(t.Year/10*10), "")
			out.add(FacetYear, strconv.Itoa(t.Year), "")
		}
		for _, p := range c.collectionsOf(t.Dir) {
			out.add(FacetCollection, p, c.folders[p].Name)
		}
		for _, p := range c.places(t.Dir) {
			out.add(FacetFolder, p, c.placeLabel(p))
		}
	}
	if out.ArtistKey != "" {
		if a, ok := c.Artist(out.ArtistKey); ok {
			out.add(FacetArtist, out.ArtistKey, a.Name)
		}
	}
	if ok && al.Year > 0 {
		out.Year = al.Year // the album's (original) year, as the album shows it
	}
	out.order += "\x00" + strconv.Itoa(1000+t.Disc) + strconv.Itoa(10000+t.Track)
	c.formatValues(out.add, t.TrackView)
	if ok && al.Status == store.StatusUnmatched {
		out.add(FacetState, StateUnidentified, "")
	}
	if opt.Playable != nil && !opt.Playable(t.TrackView) {
		out.add(FacetState, StateUnplayable, "")
	}
	return out
}

// FormatKey names a track's format for the Formato facet: its codec, its
// container for PCM ("wav", "aiff"), "wma" for every WMA flavor.
func FormatKey(format, codec string) string {
	c := strings.ToLower(codec)
	switch {
	case strings.HasPrefix(c, "pcm"):
		return strings.ToLower(format)
	case strings.HasPrefix(c, "wma"):
		return "wma"
	case c == "":
		return strings.ToLower(format)
	}
	return c
}
```

Create `internal/catalog/facets.go`:

```go
package catalog

import (
	"cmp"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"tordo/internal/genres"
)

// Facet names, as they appear in Explorar's URL.
const (
	FacetDecade     = "decada"
	FacetYear       = "anio"
	FacetArtist     = "artista"   // an artist key
	FacetCountry    = "pais"      // ISO 3166-1 alpha-2
	FacetGenre      = "genero"    // a genre of the taxonomy
	FacetStyle      = "estilo"    // a style of the taxonomy
	FacetCollection = "coleccion" // a collection's folder path
	FacetFolder     = "carpeta"   // a root, or a first-level folder under it
	FacetQuality    = "calidad"   // QualityLossless or QualityLossy
	FacetFormat     = "formato"   // FormatKey
	FacetState      = "estado"    // StateUnidentified or StateUnplayable
)

// FacetNames lists the facets in the order of Explorar's bar.
var FacetNames = []string{FacetDecade, FacetYear, FacetArtist, FacetCountry, FacetGenre, FacetStyle, FacetCollection,
	FacetFolder, FacetQuality, FacetFormat, FacetState}

// Values of FacetQuality and FacetState.
const (
	QualityLossless   = "sin-perdida"
	QualityLossy      = "con-perdida"
	StateUnidentified = "sin-identificar"
	StateUnplayable   = "no-reproducible"
)

// Explorar's tabs.
const (
	ViewAlbums  = "albumes"
	ViewArtists = "artistas"
	ViewTracks  = "pistas"
)

// Sort orders and directions. Artists sort by name (OrderTitle) or by
// their number of albums (OrderCount).
const (
	OrderYear   = "anio"
	OrderTitle  = "titulo"
	OrderArtist = "artista"
	OrderAdded  = "agregado"
	OrderCount  = "cantidad"
	Asc         = "asc"
	Desc        = "desc"
)

// Query is what Explorar asks for: a tab, one value per facet, and an order.
type Query struct {
	View   string            `json:"view"`
	Facets map[string]string `json:"facets"`
	Order  string            `json:"order"`
	Dir    string            `json:"dir"`
}

var (
	numberRe  = regexp.MustCompile(`^[1-9][0-9]{0,3}$`)
	countryRe = regexp.MustCompile(`^[A-Z]{2}$`)
)

// valid reports whether value is well formed for a facet. A well formed value
// that matches nothing is still applied (and finds nothing).
func valid(facet, value string) bool {
	switch facet {
	case FacetDecade:
		n, err := strconv.Atoi(value)
		return err == nil && numberRe.MatchString(value) && n%10 == 0
	case FacetYear:
		return numberRe.MatchString(value)
	case FacetCountry:
		return countryRe.MatchString(value)
	case FacetQuality:
		return value == QualityLossless || value == QualityLossy
	case FacetState:
		return value == StateUnidentified || value == StateUnplayable
	}
	return strings.TrimSpace(value) != "" && len(value) <= 1024
}

// orders lists the orders of a tab, its default first.
func orders(view string) []string {
	switch view {
	case ViewArtists:
		return []string{OrderTitle, OrderCount, OrderAdded}
	case ViewTracks:
		return []string{OrderArtist, OrderYear, OrderTitle, OrderAdded}
	}
	return []string{OrderYear, OrderTitle, OrderArtist, OrderAdded}
}

// naturalDir is an order's usual direction: A to Z for names, newest or
// largest first otherwise.
func naturalDir(order string) string {
	if order == OrderTitle || order == OrderArtist {
		return Asc
	}
	return Desc
}

// ParseQuery reads Explorar's parameters: ver, the facets, orden and dir.
// Unknown parameters and malformed values are dropped; each tab has its
// default order, and each order its natural direction.
func ParseQuery(v url.Values) Query {
	q := Query{View: ViewAlbums, Facets: map[string]string{}}
	if w := v.Get("ver"); w == ViewArtists || w == ViewTracks {
		q.View = w
	}
	for _, f := range FacetNames {
		if val := v.Get(f); valid(f, val) {
			q.Facets[f] = val
		}
	}
	os := orders(q.View)
	q.Order = os[0]
	if o := v.Get("orden"); slices.Contains(os, o) {
		q.Order = o
	}
	q.Dir = naturalDir(q.Order)
	if d := v.Get("dir"); d == Asc || d == Desc {
		q.Dir = d
	}
	return q
}

// matches reports whether values has every facet but except.
func matches(values map[string][]string, facets map[string]string, except string) bool {
	for f, want := range facets {
		// The decade's menu also ignores the year, and the genre's the
		// style: choosing another decade or genre drops them.
		if f == except || (except == FacetDecade && f == FacetYear) || (except == FacetGenre && f == FacetStyle) {
			continue
		}
		if !slices.Contains(values[f], want) {
			return false
		}
	}
	return true
}

// FacetValue is one option of a facet's menu.
type FacetValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// counter counts facet values over the items of one tab.
type counter struct {
	counts map[string]map[string]int
	labels map[string]string
}

func newCounter() *counter {
	return &counter{counts: map[string]map[string]int{}, labels: map[string]string{}}
}

// add counts an item's values for every facet whose menu it belongs to: the
// item matches every other facet.
func (k *counter) add(values map[string][]string, labels map[string]string, facets map[string]string) {
	for _, f := range FacetNames {
		if !matches(values, facets, f) {
			continue
		}
		if k.counts[f] == nil {
			k.counts[f] = map[string]int{}
		}
		for _, v := range values[f] {
			k.counts[f][v]++
			if l, ok := labels[f+"\x00"+v]; ok {
				k.labels[f+"\x00"+v] = l
			}
		}
	}
}

// result lists each facet's values: years and decades newest first, the
// rest most frequent first.
func (k *counter) result() map[string][]FacetValue {
	out := map[string][]FacetValue{}
	for _, f := range FacetNames {
		vs := []FacetValue{}
		for v, n := range k.counts[f] {
			l, ok := k.labels[f+"\x00"+v]
			if !ok {
				l = v
			}
			vs = append(vs, FacetValue{Value: v, Label: l, Count: n})
		}
		slices.SortFunc(vs, func(a, b FacetValue) int {
			if f == FacetDecade || f == FacetYear {
				return cmp.Compare(b.Value, a.Value)
			}
			return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label)), cmp.Compare(a.Value, b.Value))
		})
		out[f] = vs
	}
	return out
}

// Result is a page of Explorar: the items of the tab that match the query
// (Albums, Artists or Tracks), how many there are in all, and every facet's
// values counted over the other facets.
type Result struct {
	Query   Query                   `json:"query"`
	Total   int                     `json:"total"`
	Offset  int                     `json:"offset"`
	Albums  []Album                 `json:"albums,omitempty"`
	Artists []Artist                `json:"artists,omitempty"`
	Tracks  []Track                 `json:"tracks,omitempty"`
	Facets  map[string][]FacetValue `json:"facets"`
}

// Explore answers Explorar: the items of q's tab from offset, at most limit.
// The Artistas tab lists the album artists of the matching albums, with how
// many of them each has; its menus count albums.
func (c *Catalog) Explore(q Query, offset, limit int) Result {
	r := Result{Query: q, Offset: offset}
	k := newCounter()
	switch q.View {
	case ViewTracks:
		var found []Track
		for i := range c.Tracks {
			t := &c.Tracks[i]
			k.add(t.values, t.labels, q.Facets)
			if matches(t.values, q.Facets, "") {
				found = append(found, *t)
			}
		}
		sortTracks(found, q.Order, q.Dir)
		r.Total, r.Tracks = len(found), page(found, offset, limit)
	default:
		var found []Album
		for i := range c.Albums {
			a := &c.Albums[i]
			k.add(a.values, a.labels, q.Facets)
			if matches(a.values, q.Facets, "") {
				found = append(found, *a)
			}
		}
		if q.View == ViewArtists {
			artists := c.artistsOf(found)
			sortArtists(artists, q.Order, q.Dir)
			r.Total, r.Artists = len(artists), page(artists, offset, limit)
		} else {
			sortAlbums(found, q.Order, q.Dir)
			r.Total, r.Albums = len(found), page(found, offset, limit)
		}
	}
	r.Facets = k.result()
	if g := q.Facets[FacetGenre]; g != "" {
		// An album may have several genres: list only the chosen one's styles.
		r.Facets[FacetStyle] = slices.DeleteFunc(r.Facets[FacetStyle], func(v FacetValue) bool { return !genres.Valid(g, v.Value) })
	}
	return r
}

// artistsOf lists the album artists of albums, counting their albums.
func (c *Catalog) artistsOf(albums []Album) []Artist {
	n := map[string]int{}
	var keys []string
	for _, a := range albums {
		if a.ArtistKey == "" {
			continue
		}
		if n[a.ArtistKey] == 0 {
			keys = append(keys, a.ArtistKey)
		}
		n[a.ArtistKey]++
	}
	out := make([]Artist, 0, len(keys))
	for _, key := range keys {
		if a, ok := c.Artist(key); ok {
			cp := *a
			cp.Albums = n[key]
			out = append(out, cp)
		}
	}
	return out
}

// page returns items[offset:offset+limit], never nil.
func page[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return []T{}
	}
	return items[offset:min(len(items), offset+limit)]
}

// direction applies dir to a comparison.
func direction(c int, dir string) int {
	if dir == Desc {
		return -c
	}
	return c
}

// sortAlbums orders albums; albums without a year go last when sorting by
// year. Ties fall back to artist, year, title and id, so pages are stable.
func sortAlbums(as []Album, order, dir string) {
	slices.SortStableFunc(as, func(a, b Album) int {
		if order == OrderYear && (a.Year == 0) != (b.Year == 0) {
			return cmp.Compare(b.Year, a.Year) // the one with a year first
		}
		var c int
		switch order {
		case OrderTitle:
			c = cmp.Compare(a.norm, b.norm)
		case OrderArtist:
			c = cmp.Or(cmp.Compare(a.anorm, b.anorm), cmp.Compare(a.Year, b.Year))
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		default:
			c = cmp.Compare(a.Year, b.Year)
		}
		return cmp.Or(direction(c, dir), cmp.Compare(a.anorm, b.anorm), cmp.Compare(a.Year, b.Year),
			cmp.Compare(a.norm, b.norm), cmp.Compare(a.ID, b.ID))
	})
}

func sortArtists(as []Artist, order, dir string) {
	slices.SortStableFunc(as, func(a, b Artist) int {
		var c int
		switch order {
		case OrderCount:
			c = cmp.Compare(a.Albums, b.Albums)
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		default:
			c = cmp.Compare(a.norm, b.norm)
		}
		return cmp.Or(direction(c, dir), cmp.Compare(a.norm, b.norm), cmp.Compare(a.Key, b.Key))
	})
}

// sortTracks orders tracks; by artist, an artist's tracks come by year and
// in album order.
func sortTracks(ts []Track, order, dir string) {
	slices.SortStableFunc(ts, func(a, b Track) int {
		if order == OrderYear && (a.Year == 0) != (b.Year == 0) {
			return cmp.Compare(b.Year, a.Year)
		}
		var c int
		switch order {
		case OrderTitle:
			c = cmp.Compare(a.norm, b.norm)
		case OrderAdded:
			c = cmp.Compare(a.Added, b.Added)
		case OrderYear:
			c = cmp.Compare(a.Year, b.Year)
		default:
			c = cmp.Compare(a.anorm, b.anorm)
		}
		return cmp.Or(direction(c, dir), cmp.Compare(a.anorm, b.anorm), cmp.Compare(a.Year, b.Year),
			cmp.Compare(a.order, b.order), cmp.Compare(a.norm, b.norm), cmp.Compare(a.ID, b.ID))
	})
}
```

- [ ] **Step 4: Run the checks**

```bash
gofmt -l internal cmd   # prints nothing
go vet ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/catalog.go internal/catalog/catalog_test.go internal/catalog/facets.go
git commit -m "feat: catalog of albums, artists and tracks with facets"
```

### Task 5: Artist page

The Artista page of the spec: country, years (of their albums: MusicBrainz's "begin" is a person's birth), their albums oldest first, and their tracks on others' albums (compilations), grouped by album in album order. Most played (M4) and artist photos (deferred) are not here.

**Files:**
- Create: `internal/catalog/artist.go`
- Create: `internal/catalog/artist_test.go`

**Interfaces:**
- Consumes: `Catalog`, `sortAlbums` (Task 4), `store.Artist` (MusicBrainz type, area, begin, end).
- Produces: `catalog.ArtistPage{Artist; MBID, Type, Area, Begin, End string; FirstYear, LastYear int; Albums []Album; Appearances []Appearance}`, `catalog.Appearance{Album Album; Tracks []Track}`, `(*Catalog).ArtistPage(key string) (ArtistPage, bool)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/catalog/artist_test.go`:

```go
package catalog

import (
	"reflect"
	"testing"
)

func TestArtistPage(t *testing.T) {
	c := build()
	p, ok := c.ArtistPage(soda)
	if !ok {
		t.Fatal("no page")
	}
	if p.MBID != soda || p.Type != "Group" || p.Begin != "1982" || p.FirstYear != 1986 || p.LastYear != 1988 || p.Country != "AR" {
		t.Fatalf("page = %+v", p)
	}
	if got := albumTitles(p.Albums); !reflect.DeepEqual(got, []string{"Signos", "Doble vida"}) {
		t.Fatalf("albums = %q", got)
	}
	if len(p.Appearances) != 1 || p.Appearances[0].Album.Title != "Rock nacional" || len(p.Appearances[0].Tracks) != 1 ||
		p.Appearances[0].Tracks[0].Title != "Persiana americana" {
		t.Fatalf("appearances = %+v", p.Appearances)
	}
	ch, _ := c.ArtistPage("charly-garcia")
	if ch.MBID != "" || len(ch.Albums) != 2 || len(ch.Appearances) != 0 {
		t.Fatalf("Charly = %+v", ch)
	}
	if _, ok := c.ArtistPage("nadie"); ok {
		t.Fatal("unknown artist found")
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/catalog`
Expected: FAIL — build failure: `c.ArtistPage undefined`.

- [ ] **Step 3: Write the code**

Create `internal/catalog/artist.go`:

```go
package catalog

import (
	"cmp"
	"slices"
)

// ArtistPage is what the Artista page shows.
type ArtistPage struct {
	Artist
	MBID        string       `json:"mbid"` // "" when known only by name
	Type        string       `json:"type"` // MusicBrainz's: Person, Group…
	Area        string       `json:"area"`
	Begin       string       `json:"begin"` // MusicBrainz dates: "1982", "1951-10-23"
	End         string       `json:"end"`
	FirstYear   int          `json:"firstYear"` // of its albums, 0 when unknown
	LastYear    int          `json:"lastYear"`
	Albums      []Album      `json:"albums"`      // its own, oldest first
	Appearances []Appearance `json:"appearances"` // its tracks on others' albums (compilations)
}

// Appearance is an album of others with tracks by the artist.
type Appearance struct {
	Album  Album   `json:"album"`
	Tracks []Track `json:"tracks"`
}

// ArtistPage returns an artist's page.
func (c *Catalog) ArtistPage(key string) (ArtistPage, bool) {
	a, ok := c.Artist(key)
	if !ok {
		return ArtistPage{}, false
	}
	p := ArtistPage{Artist: *a, Albums: []Album{}, Appearances: []Appearance{}}
	if m, ok := c.snapArtists[a.mbid]; ok && a.mbid != "" {
		p.MBID, p.Type, p.Area, p.Begin, p.End = m.ID, m.Type, m.Area, m.Begin, m.End
	}
	for _, al := range c.Albums {
		if al.ArtistKey == key {
			p.Albums = append(p.Albums, al)
		}
	}
	sortAlbums(p.Albums, OrderYear, Asc)
	for _, y := range a.years {
		if p.FirstYear == 0 || y < p.FirstYear {
			p.FirstYear = y
		}
		p.LastYear = max(p.LastYear, y)
	}
	byAlbum := map[int64]int{}
	for _, t := range c.Tracks {
		if t.ArtistKey != key || t.AlbumID == 0 {
			continue
		}
		al, ok := c.Album(t.AlbumID)
		if !ok || al.ArtistKey == key {
			continue
		}
		i, seen := byAlbum[al.ID]
		if !seen {
			i = len(p.Appearances)
			byAlbum[al.ID] = i
			p.Appearances = append(p.Appearances, Appearance{Album: *al})
		}
		p.Appearances[i].Tracks = append(p.Appearances[i].Tracks, t)
	}
	slices.SortStableFunc(p.Appearances, func(x, y Appearance) int {
		return cmp.Or(cmp.Compare(x.Album.Year, y.Album.Year), cmp.Compare(x.Album.norm, y.Album.norm), cmp.Compare(x.Album.ID, y.Album.ID))
	})
	for _, ap := range p.Appearances {
		slices.SortStableFunc(ap.Tracks, func(x, y Track) int { return cmp.Compare(x.order, y.order) })
	}
	return p, true
}
```

- [ ] **Step 4: Run the checks**

```bash
gofmt -l internal cmd   # prints nothing
go vet ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/artist.go internal/catalog/artist_test.go
git commit -m "feat: artist page"
```

### Task 6: Instant search

`internal/search` is Cinexplorer's FTS5 index adapted: documents with a name and a context, ranked `bm25(docs, 0, 10, 2)`, all words required and the last one a prefix. Both the indexed text and the query go through `nameparse.Norm`, so case, accents and decomposed letters (macOS's NFD folder names, the M3a check's pending item) do not matter. The catalog gives one document per artist (all their spellings), album (with its tag title and artist), track (with artist and album) and folder (other than albums' own folders). `Found` turns the ranked keys back into items, by kind, with totals; artists come by number of albums.

**Files:**
- Create: `internal/catalog/search.go`
- Create: `internal/catalog/search_test.go`
- Create: `internal/search/search.go`
- Create: `internal/search/search_test.go`

**Interfaces:**
- Consumes: `Catalog` (Task 4), `nameparse.Norm`.
- Produces:
  - `search.Doc{Key, Name, Context string}`, `search.New() (*Index, error)`, `(*Index).Refresh(changes int64, docs func() []Doc) error`, `(*Index).Query(q string) ([]string, error)`, `search.Terms(q string) []string`, `search.MinLength = 2`.
  - `(*Catalog).SearchDocs() []search.Doc`; `catalog.Folder{Path, Name, Collection, Albums, Tracks}`; `catalog.Found{Artists, Albums, Tracks, Folders, Totals map[string]int}`; `(*Catalog).Found(keys []string, limit int) Found`.

- [ ] **Step 1: Write the failing tests**

Create `internal/catalog/search_test.go`:

```go
package catalog

import (
	"testing"

	"tordo/internal/search"
	"tordo/internal/store"
)

func TestSearch(t *testing.T) {
	c := build()
	x, err := search.New()
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	if err := x.Refresh(c.Changes, c.SearchDocs); err != nil {
		t.Fatal(err)
	}
	find := func(q string, limit int) Found {
		keys, err := x.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		return c.Found(keys, limit)
	}
	f := find("soda", 10)
	if len(f.Artists) != 1 || f.Artists[0].Key != soda || len(f.Albums) != 2 || len(f.Tracks) != 4 || f.Totals["tracks"] != 4 {
		t.Fatalf("soda: %+v", f)
	}
	f = find("garcia", 1) // both spellings, one key
	if len(f.Artists) != 1 || f.Totals["albums"] != 2 || len(f.Albums) != 1 || f.Totals["artists"] != 1 {
		t.Fatalf("garcia: %+v", f)
	}
	f = find("latinoamérica", 10)
	if len(f.Folders) != 1 || f.Folders[0].Path != la || !f.Folders[0].Collection || len(f.Albums) != 0 {
		t.Fatalf("collection: %+v", f)
	}
	if f := find("signos", 10); len(f.Folders) != 0 {
		t.Fatalf("an album's own folder is the album: %+v", f.Folders)
	}
	if f := find("juizo", 10); len(f.Tracks) != 1 || f.Tracks[0].AlbumID != 0 {
		t.Fatalf("loose track: %+v", f)
	}
}

func TestFoundArtistsByAlbums(t *testing.T) {
	snap := fixture()
	snap.Albums = append(snap.Albums, store.AlbumView{ID: 6, Title: "Demo", Artist: "Charly Sosa", Dir: root + "/Charly Sosa", Tracks: 1})
	c := Build(snap, Options{})
	f := c.Found([]string{"artist:charly-sosa", "artist:charly-garcia"}, 1)
	if len(f.Artists) != 1 || f.Artists[0].Key != "charly-garcia" || f.Totals["artists"] != 2 {
		t.Fatalf("found = %+v", f)
	}
}
```

Create `internal/search/search_test.go`:

```go
package search

import (
	"reflect"
	"testing"
)

func index(t *testing.T, docs ...Doc) *Index {
	t.Helper()
	x, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { x.Close() })
	if err := x.Refresh(1, func() []Doc { return docs }); err != nil {
		t.Fatal(err)
	}
	return x
}

func TestQuery(t *testing.T) {
	x := index(t,
		Doc{Key: "track:1", Name: "Fito", Context: "Fito Páez · Circo Beat"},
		Doc{Key: "artist:fito", Name: "Fito Páez"},
		Doc{Key: "album:2", Name: "Circo Beat", Context: "Fito Páez"},
		Doc{Key: "folder:x", Name: "Latinoamérica 600"}, // decomposed, as macOS writes names
	)
	for _, c := range []struct {
		q    string
		want []string
	}{
		{"fito paez", []string{"artist:fito", "track:1", "album:2"}}, // the name weighs more than the context
		{"PÁEZ circ", []string{"album:2", "track:1"}},                // the last word is a prefix
		{"latinoamérica", []string{"folder:x"}},
		{"latinoamér", []string{"folder:x"}},
		{"f", nil}, // too short
		{"!!", nil},
		{"zzz", nil},
	} {
		got, err := x.Query(c.q)
		if err != nil {
			t.Fatalf("%q: %v", c.q, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q = %q, want %q", c.q, got, c.want)
		}
	}
}

func TestRefreshOnlyWhenChanged(t *testing.T) {
	x := index(t, Doc{Key: "a", Name: "Signos"})
	calls := 0
	x.Refresh(1, func() []Doc { calls++; return nil })
	if calls != 0 {
		t.Fatal("rebuilt without changes")
	}
	x.Refresh(2, func() []Doc { calls++; return []Doc{{Key: "b", Name: "Doble vida"}} })
	if got, _ := x.Query("signos"); calls != 1 || got != nil {
		t.Fatalf("after a change: calls %d, signos %q", calls, got)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/search ./internal/catalog`
Expected: FAIL — build failure: package `tordo/internal/search` not found / `c.SearchDocs undefined`.

- [ ] **Step 3: Write the code**

Create `internal/catalog/search.go`:

```go
package catalog

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"tordo/internal/search"
	"tordo/internal/store"
)

// Search document keys: a kind and the item's id.
const (
	keyArtist = "artist:"
	keyAlbum  = "album:"
	keyTrack  = "track:"
	keyFolder = "folder:"
)

// Folder is a folder found by the search: one that is not an album's own.
type Folder struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Collection bool   `json:"collection"`
	Albums     int    `json:"albums"`
	Tracks     int    `json:"tracks"`
}

// SearchDocs gives the search index one document per artist, album, track
// and folder (other than albums' own folders, which their albums stand for).
func (c *Catalog) SearchDocs() []search.Doc {
	docs := make([]search.Doc, 0, len(c.Artists)+len(c.Albums)+len(c.Tracks))
	for _, a := range c.Artists {
		docs = append(docs, search.Doc{Key: keyArtist + a.Key, Name: a.Name, Context: strings.Join(a.names, " ")})
	}
	for _, a := range c.Albums {
		docs = append(docs, search.Doc{Key: keyAlbum + strconv.FormatInt(a.ID, 10), Name: a.Title,
			Context: a.Artist + " " + a.tagged})
	}
	for _, t := range c.Tracks {
		ctx := t.Artist + " " + t.Album
		if t.TagTitle != t.Title {
			ctx += " " + t.TagTitle
		}
		docs = append(docs, search.Doc{Key: keyTrack + strconv.FormatInt(t.ID, 10), Name: t.Title, Context: ctx})
	}
	albumDirs := map[string]bool{}
	for _, a := range c.Albums {
		albumDirs[a.Dir] = true
	}
	for _, f := range c.sortedFolders() {
		if !albumDirs[f.Path] && f.TrackCount > 0 {
			docs = append(docs, search.Doc{Key: keyFolder + f.Path, Name: f.Name, Context: c.folders[f.Parent].Name})
		}
	}
	return docs
}

func (c *Catalog) sortedFolders() []store.SnapFolder {
	out := make([]store.SnapFolder, 0, len(c.folders))
	for _, f := range c.folders {
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b store.SnapFolder) int { return cmp.Compare(a.Path, b.Path) })
	return out
}

// Found is what a search finds, by kind, best first: at most limit of each,
// with how many there are in all.
type Found struct {
	Artists []Artist       `json:"artists"`
	Albums  []Album        `json:"albums"`
	Tracks  []Track        `json:"tracks"`
	Folders []Folder       `json:"folders"`
	Totals  map[string]int `json:"totals"` // by kind: artists, albums, tracks, folders
}

// Found turns the index's keys into items. Artists come by their number of
// albums: their names all match, and the one with most albums is likely
// the one sought.
func (c *Catalog) Found(keys []string, limit int) Found {
	f := Found{Artists: []Artist{}, Albums: []Album{}, Tracks: []Track{}, Folders: []Folder{}, Totals: map[string]int{}}
	// room counts one more item of a kind and tells whether it is shown.
	room := func(kind string, shown int) bool {
		f.Totals[kind]++
		return shown < limit
	}
	for _, k := range keys {
		kind, id, _ := strings.Cut(k, ":")
		n, _ := strconv.ParseInt(id, 10, 64)
		switch kind + ":" {
		case keyArtist:
			if a, ok := c.Artist(id); ok {
				f.Totals["artists"]++
				f.Artists = append(f.Artists, *a)
			}
		case keyAlbum:
			if a, ok := c.Album(n); ok && room("albums", len(f.Albums)) {
				f.Albums = append(f.Albums, *a)
			}
		case keyTrack:
			if t, ok := c.Track(n); ok && room("tracks", len(f.Tracks)) {
				f.Tracks = append(f.Tracks, *t)
			}
		case keyFolder:
			if d, ok := c.folders[id]; ok && room("folders", len(f.Folders)) {
				f.Folders = append(f.Folders, Folder{Path: d.Path, Name: d.Name, Collection: d.Collection,
					Albums: d.AlbumCount, Tracks: d.TrackCount})
			}
		}
	}
	slices.SortStableFunc(f.Artists, func(a, b Artist) int { return cmp.Compare(b.Albums, a.Albums) })
	f.Artists = f.Artists[:min(limit, len(f.Artists))]
	return f
}
```

Create `internal/search/search.go`:

```go
// Package search is the instant search of the catalog: a full-text index
// (SQLite FTS5) kept in memory and rebuilt from the catalog when it changes.
// It ignores case and accents (also decomposed ones, as macOS writes file
// names) and matches the last word as a prefix, so results come while
// typing.
package search

import (
	"database/sql"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"tordo/internal/nameparse"
)

// Doc is what the index keeps of one artist, album, track or folder.
type Doc struct {
	Key     string // what the catalog finds it by ("album:12")
	Name    string // its name or title
	Context string // what else finds it: an album's artist, a track's album…
}

// MinLength is how many letters or digits a query needs.
const MinLength = 2

// rank weighs the columns: name, context (the key is not indexed).
const rank = `bm25(docs, 0, 10, 2)`

type Index struct {
	mu      sync.Mutex
	db      *sql.DB
	changes int64
	built   bool
}

// New returns an empty index.
func New() (*Index, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	// An in-memory database lives in its connection: keep exactly one.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	if _, err := db.Exec(`CREATE VIRTUAL TABLE docs USING fts5(key UNINDEXED, name, context,
		tokenize = "unicode61 remove_diacritics 2")`); err != nil {
		db.Close()
		return nil, err
	}
	return &Index{db: db}, nil
}

func (x *Index) Close() error { return x.db.Close() }

// Refresh rebuilds the index from docs() when the catalog changed since the
// last build: changes is the catalog's change counter. docs are indexed in
// order, which breaks ties between equal ranks.
func (x *Index) Refresh(changes int64, docs func() []Doc) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.built && x.changes == changes {
		return nil
	}
	tx, err := x.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM docs`); err != nil {
		return err
	}
	ins, err := tx.Prepare(`INSERT INTO docs (key, name, context) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	for _, d := range docs() {
		if _, err := ins.Exec(d.Key, nameparse.Norm(d.Name), nameparse.Norm(d.Context)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	x.changes, x.built = changes, true
	return nil
}

// Terms splits a query into folded words (no case, no accents).
func Terms(q string) []string { return strings.Fields(nameparse.Norm(q)) }

// expression is the FTS5 query for terms: all of them, the last as a prefix.
// Terms are letters and digits only, so quoting them is enough.
func expression(terms []string) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = `"` + t + `"`
	}
	parts[len(parts)-1] += "*"
	return strings.Join(parts, " ")
}

// Query returns the keys of the documents that match q, best first. A query
// with fewer than MinLength letters or digits matches nothing.
func (x *Index) Query(q string) ([]string, error) {
	terms := Terms(q)
	if len([]rune(strings.Join(terms, ""))) < MinLength {
		return nil, nil
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	rows, err := x.db.Query(`SELECT key FROM docs WHERE docs MATCH ? ORDER BY `+rank+`, rowid`, expression(terms))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
```

- [ ] **Step 4: Run the checks**

```bash
gofmt -l internal cmd   # prints nothing
go vet ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/search.go internal/catalog/search_test.go internal/search/search.go internal/search/search_test.go
git commit -m "feat: instant search of artists, albums, tracks and folders"
```

### Task 7: Server: Explorar, artists, search, the album page, playing an artist, forgetting a genre mapping

The server keeps the built catalog and the search index in `catalogState`, current while `Store.Changes` stays (typing makes a request per pause). New endpoints: `GET /api/explore` (`ver`, the facets, `orden`, `dir`, `desde`, `limite` ≤ 500, default 120), `GET /api/artists/{key}`, `GET /api/search?q=&limite=` (default 6, at most 200). `GET /api/albums/{id}` (and the answer to a correction) becomes the album page: the album with `artistKey`, its tracks with `artistKey`, `playable` and `transcoded`, and the MusicBrainz `candidates` stored for it (`Store.AlbumCandidates`). `/api/play?artist=key` plays an artist: their albums by year, then their appearances. `DELETE /api/genres/aliases?raw=` forgets a mapping (the M2 check had to delete one by hand) and re-applies identities.

**Files:**
- Create: `internal/server/catalog.go`
- Create: `internal/server/catalog_test.go`
- Modify: `internal/server/identify.go`
- Modify: `internal/server/identify_test.go`
- Modify: `internal/server/media.go`
- Modify: `internal/server/server.go`
- Modify: `internal/store/apply.go`
- Modify: `internal/store/apply_test.go`

**Interfaces:**
- Consumes: Tasks 1–6; `stream.Streamer.Plan`; `Identifier.Apply`.
- Produces:
  - `(*Store).AlbumCandidates(albumID int64) ([]Candidate, error)` (never nil), `(*Store).DeleteGenreAlias(raw string) error`.
  - `Server.cat catalogState`; `(*Server).catalog() (*catalog.Catalog, error)`, `(*Server).searchIndex()`, `(*Server).playable(store.TrackView) bool`, `(*Server).writeAlbum(w, id)`, `(*Server).playArtist(key) ([]store.PlayItem, bool, error)`, `intParam(r, name, def, lo, hi int) int`.
  - Routes: `GET /api/explore`, `GET /api/artists/{key}`, `GET /api/search`, `DELETE /api/genres/aliases`; `/api/play` accepts `artist`.
  - JSON: album page = `store.AlbumDetail` + `artistKey` + `candidates`, tracks with `artistKey`, `playable`, `transcoded`; search = `{q, artists, albums, tracks, folders, totals}`.

- [ ] **Step 1: Write the failing tests**

Create `internal/server/catalog_test.go`:

```go
package server

import (
	"net/http"
	"net/url"
	"testing"

	"tordo/internal/catalog"
	"tordo/internal/store"
)

func TestExploreEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var r catalog.Result
	decode(t, do(t, h, "GET", "/api/explore", ""), &r)
	if r.Total != 1 || len(r.Albums) != 1 || r.Albums[0].Title != "Locura" || r.Albums[0].ArtistKey != "virus" || r.Query.View != "albumes" {
		t.Fatalf("albums = %+v", r)
	}
	if got := r.Facets["estado"]; len(got) != 1 || got[0].Value != catalog.StateUnplayable {
		t.Fatalf("without ffmpeg the WMA track is not playable: %+v", got)
	}
	r = catalog.Result{}
	decode(t, do(t, h, "GET", "/api/explore?ver=pistas&formato=wma&limite=1", ""), &r)
	if r.Total != 1 || len(r.Tracks) != 1 || r.Tracks[0].Title != "Pecados para dos" {
		t.Fatalf("tracks = %+v", r)
	}
	r = catalog.Result{}
	decode(t, do(t, h, "GET", "/api/explore?ver=artistas&desde=5", ""), &r)
	if r.Total != 1 || len(r.Artists) != 0 {
		t.Fatalf("artists past the end = %+v", r)
	}
}

func TestCatalogFollowsChanges(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var r catalog.Result
	decode(t, do(t, h, "GET", "/api/explore?pais=AR", ""), &r)
	if r.Total != 0 {
		t.Fatalf("before: %+v", r)
	}
	as, _ := s.Store.Albums("")
	if err := s.Store.ApplyIdentities([]store.AlbumIdentity{{AlbumID: as[0].ID, Status: store.StatusAuto, ReleaseID: "r1",
		Country: "AR"}}, nil); err != nil {
		t.Fatal(err)
	}
	decode(t, do(t, h, "GET", "/api/explore?pais=AR", ""), &r)
	if r.Total != 1 {
		t.Fatalf("the catalog was not rebuilt: %+v", r)
	}
}

func TestArtistEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var p catalog.ArtistPage
	decode(t, do(t, h, "GET", "/api/artists/virus", ""), &p)
	if p.Name != "Virus" || len(p.Albums) != 1 {
		t.Fatalf("artist = %+v", p)
	}
	if rec := do(t, h, "GET", "/api/artists/"+url.PathEscape("nadie/x"), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown artist: %d", rec.Code)
	}
	s.Streamer.FFmpeg = "ffmpeg"
	var pl playResponse
	decode(t, do(t, h, "GET", "/api/play?artist=virus", ""), &pl)
	if len(pl.Items) != 2 || pl.Items[0].Title != "Una luna de miel en la mano" {
		t.Fatalf("play the artist: %+v", pl)
	}
	if rec := do(t, h, "GET", "/api/play?artist=nadie", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("play an unknown artist: %d", rec.Code)
	}
}

func TestSearchEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var f struct {
		Q string `json:"q"`
		catalog.Found
	}
	decode(t, do(t, h, "GET", "/api/search?q=pecad", ""), &f)
	if f.Q != "pecad" || len(f.Tracks) != 1 || f.Tracks[0].Title != "Pecados para dos" || len(f.Albums) != 0 {
		t.Fatalf("found = %+v", f)
	}
	f.Found = catalog.Found{}
	decode(t, do(t, h, "GET", "/api/search?q=virus&limite=1", ""), &f)
	if len(f.Artists) != 1 || len(f.Albums) != 1 || len(f.Tracks) != 1 || f.Totals["tracks"] != 2 {
		t.Fatalf("virus = %+v", f)
	}
	f.Found = catalog.Found{}
	decode(t, do(t, h, "GET", "/api/search?q=v", ""), &f)
	if len(f.Tracks) != 0 || f.Tracks == nil {
		t.Fatalf("too short = %+v", f)
	}
}

func TestAlbumPage(t *testing.T) {
	s, _ := mediaServer(t, "")
	id := onlyAlbum(t, s)
	var p struct {
		Title     string `json:"title"`
		ArtistKey string `json:"artistKey"`
		Tracks    []struct {
			Title      string `json:"title"`
			ArtistKey  string `json:"artistKey"`
			Playable   bool   `json:"playable"`
			Transcoded bool   `json:"transcoded"`
		} `json:"tracks"`
		Candidates []store.Candidate `json:"candidates"`
	}
	decode(t, do(t, s.Handler(), "GET", "/api/albums/"+itoa(id), ""), &p)
	if p.Title != "Locura" || p.ArtistKey != "virus" || len(p.Tracks) != 2 || p.Candidates == nil {
		t.Fatalf("album = %+v", p)
	}
	if !p.Tracks[0].Playable || p.Tracks[1].Playable || p.Tracks[0].ArtistKey != "virus" {
		t.Fatalf("without ffmpeg: %+v", p.Tracks)
	}
	s.Streamer.FFmpeg = "ffmpeg"
	decode(t, do(t, s.Handler(), "GET", "/api/albums/"+itoa(id), ""), &p)
	if !p.Tracks[1].Playable || !p.Tracks[1].Transcoded {
		t.Fatalf("with ffmpeg: %+v", p.Tracks)
	}
}
```

Modify `internal/server/identify_test.go` (unified diff against the previous task):

```diff
@@ -199,6 +199,15 @@ func TestGenreEndpoints(t *testing.T) {
 	if rec := do(t, s.Handler(), "GET", "/api/genres/aliases", ""); rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
 		t.Fatalf("list aliases: %d", rec.Code)
 	}
+	if rec := do(t, s.Handler(), "DELETE", "/api/genres/aliases?raw=Chamam%C3%A9", ""); rec.Code != http.StatusNoContent || f.applied != 2 {
+		t.Fatalf("delete alias: %d, applied %d", rec.Code, f.applied)
+	}
+	if as, _ := s.Store.GenreAliases(); len(as) != 0 {
+		t.Fatalf("aliases after delete = %+v", as)
+	}
+	if rec := do(t, s.Handler(), "DELETE", "/api/genres/aliases", ""); rec.Code != http.StatusBadRequest {
+		t.Fatalf("no raw: %d", rec.Code)
+	}
 }
 
 func TestReleaseIDFrom(t *testing.T) {
```

Modify `internal/store/apply_test.go` (unified diff against the previous task):

```diff
@@ -118,6 +118,12 @@ func TestGenreAliases(t *testing.T) {
 	if err != nil || !reflect.DeepEqual(got, want) {
 		t.Fatalf("aliases = %+v, err %v", got, err)
 	}
+	if err := s.DeleteGenreAlias("chamame"); err != nil {
+		t.Fatal(err)
+	}
+	if got, _ := s.GenreAliases(); !reflect.DeepEqual(got, want[1:]) {
+		t.Fatalf("after deleting: %+v", got)
+	}
 }
 
 func TestApplyIdentitiesSkipsStaleRows(t *testing.T) {
@@ -144,3 +150,21 @@ func TestApplyIdentitiesSkipsStaleRows(t *testing.T) {
 		t.Fatalf("the valid identity was not applied: %+v", c)
 	}
 }
+
+func TestAlbumCandidates(t *testing.T) {
+	s := sample(t)
+	id := albumID(t, s, "Signos")
+	if cs, err := s.AlbumCandidates(id); err != nil || cs == nil || len(cs) != 0 {
+		t.Fatalf("not looked up: %v, %v", cs, err)
+	}
+	want := []Candidate{{ReleaseID: "r1", Title: "Signos", Score: 0.7}}
+	if err := s.SaveMatches([]Match{{Query: "q", Status: StatusUnmatched, Confidence: 0.7, Candidates: want}}); err != nil {
+		t.Fatal(err)
+	}
+	if err := s.ApplyIdentities([]AlbumIdentity{{AlbumID: id, Query: "q", Status: StatusUnmatched}}, nil); err != nil {
+		t.Fatal(err)
+	}
+	if cs, err := s.AlbumCandidates(id); err != nil || !reflect.DeepEqual(cs, want) {
+		t.Fatalf("candidates = %+v, %v", cs, err)
+	}
+}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./internal/server ./internal/store`
Expected: FAIL — build failure: `s.Store.AlbumCandidates undefined`, routes answering 404 / 405.

- [ ] **Step 3: Write the code**

Create `internal/server/catalog.go`:

```go
package server

import (
	"net/http"
	"strconv"
	"sync"

	"tordo/internal/catalog"
	"tordo/internal/search"
	"tordo/internal/store"
	"tordo/internal/stream"
)

// catalogState is the catalog built from the store and its search index,
// kept between requests while the store's change counter stays: Explorar
// and typing in the search ask often, and building takes a whole snapshot.
type catalogState struct {
	mu      sync.Mutex
	cat     *catalog.Catalog
	index   *search.Index
	indexed int64 // the change counter the index was built at
}

// playable tells whether the server can deliver a track's audio.
func (s *Server) playable(t store.TrackView) bool {
	mode, _ := s.Streamer.Plan(stream.Format{Container: t.Format, Codec: t.Codec, Lossless: t.Lossless})
	return mode != stream.Unplayable
}

// catalog returns the catalog, rebuilt when the store changed.
func (s *Server) catalog() (*catalog.Catalog, error) {
	changes, err := s.Store.Changes()
	if err != nil {
		return nil, err
	}
	st := &s.cat
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.cat != nil && st.cat.Changes == changes {
		return st.cat, nil
	}
	snap, err := s.Store.Snapshot()
	if err != nil {
		return nil, err
	}
	st.cat = catalog.Build(snap, catalog.Options{Playable: s.playable})
	return st.cat, nil
}

// searchIndex returns the catalog and its search index, both current.
func (s *Server) searchIndex() (*catalog.Catalog, *search.Index, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, nil, err
	}
	st := &s.cat
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.index == nil {
		if st.index, err = search.New(); err != nil {
			return nil, nil, err
		}
	}
	if err := st.index.Refresh(c.Changes, c.SearchDocs); err != nil {
		return nil, nil, err
	}
	return c, st.index, nil
}

// intParam reads a query parameter between lo and hi, def when missing or
// out of range.
func intParam(r *http.Request, name string, def, lo, hi int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || n < lo || n > hi {
		return def
	}
	return n
}

// explore answers Explorar: ?ver= (albumes, artistas, pistas), the facets,
// ?orden= and ?dir=, a page from ?desde= of ?limite= items.
func (s *Server) explore(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	q := catalog.ParseQuery(r.URL.Query())
	writeJSON(w, c.Explore(q, intParam(r, "desde", 0, 0, 1<<30), intParam(r, "limite", 120, 1, 500)))
}

func (s *Server) artist(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	p, ok := c.ArtistPage(r.PathValue("key"))
	if !ok {
		http.Error(w, "artista no encontrado", http.StatusNotFound)
		return
	}
	writeJSON(w, p)
}

// find is the instant search: ?q=, at most ?limite= of each kind.
func (s *Server) find(w http.ResponseWriter, r *http.Request) {
	c, idx, err := s.searchIndex()
	if err != nil {
		fail(w, err)
		return
	}
	q := r.URL.Query().Get("q")
	keys, err := idx.Query(q)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, struct {
		Q string `json:"q"`
		catalog.Found
	}{q, c.Found(keys, intParam(r, "limite", 6, 1, 200))})
}

// albumTrack is a track of the album page: whether the server can deliver it.
type albumTrack struct {
	store.TrackView
	ArtistKey  string `json:"artistKey"`  // the track artist's page, when there is one
	Playable   bool   `json:"playable"`   // false: it needs ffmpeg, and there is none
	Transcoded bool   `json:"transcoded"` // converted by ffmpeg: no seeking
}

// albumPage is the album page: the album, its artist's page, its tracks and
// the MusicBrainz releases offered for it.
type albumPage struct {
	store.AlbumDetail
	ArtistKey  string            `json:"artistKey"`
	Tracks     []albumTrack      `json:"tracks"`
	Candidates []store.Candidate `json:"candidates"`
}

// writeAlbum answers an album's page.
func (s *Server) writeAlbum(w http.ResponseWriter, id int64) {
	a, ok, err := s.Store.Album(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !ok {
		http.Error(w, "álbum no encontrado", http.StatusNotFound)
		return
	}
	cands, err := s.Store.AlbumCandidates(id)
	if err != nil {
		fail(w, err)
		return
	}
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	p := albumPage{AlbumDetail: a, Tracks: make([]albumTrack, 0, len(a.Tracks)), Candidates: cands}
	if al, ok := c.Album(id); ok {
		p.ArtistKey = al.ArtistKey
	}
	for _, t := range a.Tracks {
		mode, _ := s.Streamer.Plan(stream.Format{Container: t.Format, Codec: t.Codec, Lossless: t.Lossless})
		at := albumTrack{TrackView: t, Playable: mode != stream.Unplayable, Transcoded: mode == stream.Transcode}
		if ct, ok := c.Track(t.ID); ok {
			at.ArtistKey = ct.ArtistKey
		}
		p.Tracks = append(p.Tracks, at)
	}
	writeJSON(w, p)
}
```

Modify `internal/server/identify.go` (unified diff against the previous task):

```diff
@@ -129,16 +129,7 @@ func (s *Server) afterCorrection(w http.ResponseWriter, id int64, err error) {
 		return
 	}
 	s.Identifier.Trigger()
-	a, ok, err := s.Store.Album(id)
-	if err != nil {
-		fail(w, err)
-		return
-	}
-	if !ok {
-		http.Error(w, "álbum no encontrado", http.StatusNotFound)
-		return
-	}
-	writeJSON(w, a)
+	s.writeAlbum(w, id)
 }
 
 func (s *Server) genreTaxonomy(w http.ResponseWriter, r *http.Request) { writeJSON(w, genres.Taxonomy) }
@@ -189,3 +180,24 @@ func (s *Server) setGenreAlias(w http.ResponseWriter, r *http.Request) {
 	}
 	writeJSON(w, a)
 }
+
+// deleteGenreAlias forgets the user's mapping of ?raw=.
+func (s *Server) deleteGenreAlias(w http.ResponseWriter, r *http.Request) {
+	if s.noIdentifier(w) {
+		return
+	}
+	raw := genres.Key(r.URL.Query().Get("raw"))
+	if raw == "" {
+		http.Error(w, "falta el valor (raw)", http.StatusBadRequest)
+		return
+	}
+	if err := s.Store.DeleteGenreAlias(raw); err != nil {
+		fail(w, err)
+		return
+	}
+	if err := s.Identifier.Apply(); err != nil {
+		fail(w, err)
+		return
+	}
+	w.WriteHeader(http.StatusNoContent)
+}
```

Modify `internal/server/media.go` (unified diff against the previous task):

```diff
@@ -47,9 +47,11 @@ func (s *Server) streamTrack(w http.ResponseWriter, r *http.Request) {
 }
 
 // play answers GET /api/play: the tracks to queue for an album
-// (?album=id), a folder or collection (?folder=path, recursive, in order)
-// or some tracks (?track=id, repeatable), shuffled with ?shuffle=1. Tracks
-// that cannot play (they need ffmpeg) are left out and counted.
+// (?album=id), a folder or collection (?folder=path, recursive, in order),
+// an artist (?artist=key: their albums by year, then their tracks on
+// others' albums) or some tracks (?track=id, repeatable), shuffled with
+// ?shuffle=1. Tracks that cannot play (they need ffmpeg) are left out and
+// counted.
 func (s *Server) play(w http.ResponseWriter, r *http.Request) {
 	q := r.URL.Query()
 	var items []store.PlayItem
@@ -64,6 +66,13 @@ func (s *Server) play(w http.ResponseWriter, r *http.Request) {
 		items, err = s.Store.PlayAlbum(id)
 	case q.Has("folder"):
 		items, err = s.Store.PlayFolder(q.Get("folder"))
+	case q.Has("artist"):
+		var ok bool
+		items, ok, err = s.playArtist(q.Get("artist"))
+		if err == nil && !ok {
+			http.Error(w, "artista no encontrado", http.StatusNotFound)
+			return
+		}
 	case q.Has("track"):
 		var ids []int64
 		for _, v := range q["track"] {
@@ -76,7 +85,7 @@ func (s *Server) play(w http.ResponseWriter, r *http.Request) {
 		}
 		items, err = s.Store.PlayTracks(ids)
 	default:
-		http.Error(w, "falta qué reproducir: album, folder o track", http.StatusBadRequest)
+		http.Error(w, "falta qué reproducir: album, folder, artist o track", http.StatusBadRequest)
 		return
 	}
 	if err != nil {
@@ -103,6 +112,35 @@ func (s *Server) play(w http.ResponseWriter, r *http.Request) {
 	writeJSON(w, map[string]any{"items": out, "skipped": skipped, "truncated": truncated, "ffmpeg": s.Streamer.FFmpeg != ""})
 }
 
+// playArtist lists an artist's tracks: their albums oldest first, then their
+// appearances on others' albums.
+func (s *Server) playArtist(key string) ([]store.PlayItem, bool, error) {
+	c, err := s.catalog()
+	if err != nil {
+		return nil, false, err
+	}
+	p, ok := c.ArtistPage(key)
+	if !ok {
+		return nil, false, nil
+	}
+	var out []store.PlayItem
+	for _, a := range p.Albums {
+		items, err := s.Store.PlayAlbum(a.ID)
+		if err != nil {
+			return nil, true, err
+		}
+		out = append(out, items...)
+	}
+	var ids []int64
+	for _, ap := range p.Appearances {
+		for _, t := range ap.Tracks {
+			ids = append(ids, t.ID)
+		}
+	}
+	items, err := s.Store.PlayTracks(ids)
+	return append(out, items...), true, err
+}
+
 func (s *Server) albumCover(w http.ResponseWriter, r *http.Request) {
 	s.coverByID(w, r, s.Covers.ServeAlbum)
 }
```

Modify `internal/server/server.go` (unified diff against the previous task):

```diff
@@ -12,7 +12,6 @@ import (
 	"log"
 	"net"
 	"net/http"
-	"strconv"
 	"strings"
 
 	"tordo/internal/covers"
@@ -30,6 +29,8 @@ type Server struct {
 	Covers     *covers.Covers  // nil: no pictures
 	Static     fs.FS           // the web app; nil: the embedded build
 	ReadOnly   bool
+
+	cat catalogState
 }
 
 func (s *Server) Handler() http.Handler {
@@ -40,6 +41,9 @@ func (s *Server) Handler() http.Handler {
 	mux.HandleFunc("GET /api/albums/{id}", s.album)
 	mux.HandleFunc("GET /api/folders", s.folder)
 	mux.HandleFunc("GET /api/collections", s.collections)
+	mux.HandleFunc("GET /api/explore", s.explore)
+	mux.HandleFunc("GET /api/artists/{key}", s.artist)
+	mux.HandleFunc("GET /api/search", s.find)
 	mux.HandleFunc("PUT /api/folders/collection", jsonOnly(s.setCollection))
 	mux.HandleFunc("POST /api/identify", jsonOnly(s.identify))
 	mux.HandleFunc("GET /api/review/unidentified", s.unidentified)
@@ -49,6 +53,7 @@ func (s *Server) Handler() http.Handler {
 	mux.HandleFunc("GET /api/genres/unmapped", s.unmappedGenres)
 	mux.HandleFunc("GET /api/genres/aliases", s.genreAliases)
 	mux.HandleFunc("PUT /api/genres/aliases", jsonOnly(s.setGenreAlias))
+	mux.HandleFunc("DELETE /api/genres/aliases", s.deleteGenreAlias)
 	mux.HandleFunc("GET /api/play", s.play)
 	mux.HandleFunc("GET /api/covers/album/{id}", s.albumCover)
 	mux.HandleFunc("GET /api/covers/track/{id}", s.trackCover)
@@ -153,21 +158,9 @@ func (s *Server) albums(w http.ResponseWriter, r *http.Request) {
 }
 
 func (s *Server) album(w http.ResponseWriter, r *http.Request) {
-	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
-	if err != nil {
-		http.Error(w, "id inválido", http.StatusBadRequest)
-		return
-	}
-	a, ok, err := s.Store.Album(id)
-	if err != nil {
-		fail(w, err)
-		return
-	}
-	if !ok {
-		http.Error(w, "álbum no encontrado", http.StatusNotFound)
-		return
+	if id, ok := pathID(w, r); ok {
+		s.writeAlbum(w, id)
 	}
-	writeJSON(w, a)
 }
 
 func (s *Server) folder(w http.ResponseWriter, r *http.Request) {
```

Modify `internal/store/apply.go` (unified diff against the previous task):

```diff
@@ -1,5 +1,11 @@
 package store
 
+import (
+	"database/sql"
+	"encoding/json"
+	"errors"
+)
+
 // GenreLink is one of an album's genres and where it came from.
 type GenreLink struct {
 	Genre  string `json:"genre"`
@@ -149,6 +155,21 @@ func (s *Store) Unidentified() ([]UnidentifiedView, error) {
 	return out, nil
 }
 
+// AlbumCandidates returns the MusicBrainz releases offered for an album when
+// it was matched, best first (none when it was not looked up).
+func (s *Store) AlbumCandidates(albumID int64) ([]Candidate, error) {
+	var raw string
+	err := s.db.QueryRow(`SELECT m.candidates FROM albums a JOIN matches m ON m.query = a.match_query WHERE a.id = ?`, albumID).Scan(&raw)
+	if errors.Is(err, sql.ErrNoRows) {
+		return []Candidate{}, nil
+	}
+	if err != nil {
+		return nil, err
+	}
+	out := []Candidate{}
+	return out, json.Unmarshal([]byte(raw), &out)
+}
+
 // GenreAlias is the user's mapping of a raw genre value.
 type GenreAlias struct {
 	Raw   string `json:"raw"`   // genres.Key form
@@ -162,6 +183,13 @@ func (s *Store) SetGenreAlias(a GenreAlias) error {
 	return err
 }
 
+// DeleteGenreAlias forgets a mapping: the raw value maps as built in, or
+// goes back to the Géneros por mapear queue.
+func (s *Store) DeleteGenreAlias(raw string) error {
+	_, err := s.db.Exec(`DELETE FROM genre_alias WHERE raw = ?`, raw)
+	return err
+}
+
 func (s *Store) GenreAliases() ([]GenreAlias, error) {
 	rows, err := s.db.Query(`SELECT raw, genre, style FROM genre_alias ORDER BY raw`)
 	if err != nil {
```

- [ ] **Step 4: Run the checks**

```bash
gofmt -l internal cmd   # prints nothing
go vet ./...
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/server/catalog.go internal/server/catalog_test.go internal/server/identify.go internal/server/identify_test.go internal/server/media.go internal/server/server.go internal/store/apply.go internal/store/apply_test.go
git commit -m "feat: explore, artist and search endpoints; album page with candidates and playable tracks; play an artist; forget a genre mapping"
```

### Task 8: Web libraries: routes, API calls, facets, search, review

Pure modules first, with their tests. Routes: `/` and `/explorar` → Explorar (Inicio is M4), `/artista/{key}`, `/buscar?q=`, `/revisar` and `/revisar/generos`. `facets.js` mirrors the server's query (tabs, facets with years inside decades and styles inside genres, each tab's orders) and writes stable URLs. `search.js` is Cinexplorer's keyboard logic over four kinds of results. `review.js` reads identifications, candidates and genre mappings. `format.js` gains country names (`Intl.DisplayNames`, Spanish), format names and year spans. Until Task 9 the app has no Explorar page: `/` shows "No existe esta página" for a moment of the plan; nothing ships in between.

**Files:**
- Modify: `web/src/lib/api.js`
- Modify: `web/src/lib/api.test.js`
- Create: `web/src/lib/facets.js`
- Create: `web/src/lib/facets.test.js`
- Modify: `web/src/lib/format.js`
- Modify: `web/src/lib/format.test.js`
- Create: `web/src/lib/review.js`
- Create: `web/src/lib/review.test.js`
- Modify: `web/src/lib/router.js`
- Modify: `web/src/lib/router.test.js`
- Create: `web/src/lib/search.js`
- Create: `web/src/lib/search.test.js`

**Interfaces:**
- Consumes: the endpoints of Tasks 1, 2 and 7.
- Produces (JS):
  - `router.js`: `resolve` (pages `explorar`, `carpetas`, `listas`, `album`, `artista {key}`, `buscar {q}`, `revisar {tab: 'sin-identificar'|'generos'}`, `notfound`), `artistHref(key)`, `exploreHref(facets, view)`, `searchHref(q)` (plus M3a's `folderHref`, `albumHref`, `appLink`).
  - `api.js`: `api.explore(search, from, limit)`, `api.artist(key)`, `api.find(q, limit)`, `api.setCollection(path, collection)`, `api.identify()`, `api.unidentified()`, `api.setIdentification(id, {releaseId}|{ignore: true})`, `api.resetIdentification(id)`, `api.genres()`, `api.unmappedGenres()`, `api.genreAliases()`, `api.setGenreAlias(alias)`, `api.deleteGenreAlias(raw)`; `playQuery` accepts `artist`; `releaseURL(id)`, `artistMBURL(id)`.
  - `facets.js`: `VIEWS`, `FACETS`, `ORDERS`, `LABELS`, `defaultDir`, `parse`, `toSearch`, `withFacet`, `withView`, `withOrder`, `valueLabel`, `chips`, `inside`.
  - `search.js`: `MIN`, `searchable`, `trackHref`, `options`, `total`, `move`, `enterHref`, `isShortcut`.
  - `review.js`: `percent`, `candidateLine`, `identification`, `genreLinks`, `worthAccepting`, `searchURL`, `genreOptions`, `genreChoice`, `aliasLabel`.
  - `format.js`: `country(code)`, `formatName(key)`, `years(first, last)`.

- [ ] **Step 1: Write the failing tests**

Modify `web/src/lib/api.test.js` (unified diff against the previous task):

```diff
@@ -1,11 +1,12 @@
-import { describe, expect, it } from 'vitest'
-import { albumCoverURL, itemCoverURL, playQuery, streamURL } from './api.js'
+import { afterEach, describe, expect, it, vi } from 'vitest'
+import { albumCoverURL, api, itemCoverURL, playQuery, releaseURL, streamURL } from './api.js'
 
 describe('playQuery', () => {
   it('writes what to play', () => {
     expect(playQuery({ album: 7 })).toBe('album=7')
     expect(playQuery({ folder: '../musica/Brasil', shuffle: true })).toBe('folder=..%2Fmusica%2FBrasil&shuffle=1')
     expect(playQuery({ tracks: [3, 1] })).toBe('track=3&track=1')
+    expect(playQuery({ artist: 'charly-garcia' })).toBe('artist=charly-garcia')
   })
 })
 
@@ -19,3 +20,37 @@ describe('urls', () => {
     expect(streamURL(9, true)).toBe('/stream/9?transcode=1')
   })
 })
+
+describe('calls', () => {
+  afterEach(() => vi.unstubAllGlobals())
+
+  function recorder() {
+    const calls = []
+    vi.stubGlobal('fetch', async (url, init) => {
+      calls.push([init.method, url, init.body ?? null])
+      return { ok: true, status: init.method === 'DELETE' ? 204 : 200, json: async () => ({}) }
+    })
+    return calls
+  }
+
+  it('asks the server', async () => {
+    const calls = recorder()
+    await api.explore('?decada=1980&ver=pistas', 120, 60)
+    await api.explore('')
+    await api.artist('sui generis/x')
+    await api.find('soda st')
+    await api.setCollection('../musica/x', false)
+    await api.setIdentification(4, { releaseId: 'abc' })
+    await api.deleteGenreAlias('Música uruguaya')
+    expect(calls).toEqual([
+      ['GET', '/api/explore?decada=1980&ver=pistas&desde=120&limite=60', null],
+      ['GET', '/api/explore?limite=120', null],
+      ['GET', '/api/artists/sui%20generis%2Fx', null],
+      ['GET', '/api/search?q=soda+st&limite=6', null],
+      ['PUT', '/api/folders/collection', '{"path":"../musica/x","collection":false}'],
+      ['PUT', '/api/albums/4/identification', '{"releaseId":"abc"}'],
+      ['DELETE', '/api/genres/aliases?raw=M%C3%BAsica+uruguaya', null],
+    ])
+    expect(releaseURL('abc')).toBe('https://musicbrainz.org/release/abc')
+  })
+})
```

Create `web/src/lib/facets.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { chips, inside, parse, toSearch, valueLabel, withFacet, withOrder, withView } from './facets.js'

describe('parse and toSearch', () => {
  it('round-trips queries, leaving defaults out', () => {
    const q = parse('?decada=1980&anio=1986&pais=AR&orden=titulo&x=1')
    expect(q).toEqual({ view: 'albumes', facets: { decada: '1980', anio: '1986', pais: 'AR' }, order: 'titulo', dir: 'asc' })
    expect(toSearch(q)).toBe('?decada=1980&anio=1986&pais=AR&orden=titulo')
    expect(toSearch(parse(''))).toBe('')
    expect(toSearch(parse('?orden=anio&dir=desc'))).toBe('')
    expect(toSearch(parse('?dir=asc'))).toBe('?dir=asc')
  })
  it('knows each tab and its orders', () => {
    expect(parse('?ver=artistas')).toEqual({ view: 'artistas', facets: {}, order: 'titulo', dir: 'asc' })
    expect(parse('?ver=pistas&orden=cantidad').order).toBe('artista')
    expect(parse('?ver=otra').view).toBe('albumes')
    expect(toSearch(parse('?ver=artistas&orden=cantidad'))).toBe('?ver=artistas&orden=cantidad')
  })
})

describe('changing the query', () => {
  const base = parse('?decada=1980&anio=1986&genero=Rock&estilo=Pop+Rock')
  it('keeps years inside the decade and styles with their genre', () => {
    expect(withFacet(base, 'decada', '1990').facets).toEqual({ decada: '1990', genero: 'Rock', estilo: 'Pop Rock' })
    expect(withFacet(base, 'decada', null).facets).toEqual({ genero: 'Rock', estilo: 'Pop Rock' })
    expect(withFacet(parse(''), 'anio', '1975').facets).toEqual({ anio: '1975', decada: '1970' })
    expect(withFacet(base, 'genero', 'Latin').facets).toEqual({ decada: '1980', anio: '1986', genero: 'Latin' })
    expect(withFacet(base, 'estilo', null).facets).toEqual({ decada: '1980', anio: '1986', genero: 'Rock' })
  })
  it('switches tabs and orders', () => {
    const q = withView(withOrder(base, 'titulo'), 'pistas')
    expect([q.view, q.order, q.dir, q.facets.genero]).toEqual(['pistas', 'artista', 'asc', 'Rock'])
    expect(withOrder(base, 'agregado').dir).toBe('desc')
  })
})

describe('labels', () => {
  it('names values', () => {
    expect(valueLabel('decada', '1980')).toBe('1980s')
    expect(valueLabel('pais', 'UY')).toBe('Uruguay')
    expect(valueLabel('formato', 'flac')).toBe('FLAC')
    expect(valueLabel('calidad', 'sin-perdida')).toBe('Sin pérdida')
    expect(valueLabel('estado', 'no-reproducible')).toBe('No reproducible')
    expect(valueLabel('artista', 'charly-garcia', { label: 'Charly García' })).toBe('Charly García')
    expect(valueLabel('artista', 'x')).toBe('x')
  })
  it('lists chips and nested values', () => {
    const counts = { artista: [{ value: 'a1', label: 'Soda Stereo', count: 2 }], anio: [{ value: '1986' }, { value: '1995' }],
      estilo: [{ value: 'Pop Rock' }] }
    const q = parse('?decada=1980&artista=a1')
    expect(chips(q, counts)).toEqual([
      { name: 'decada', value: '1980', label: '1980s' },
      { name: 'artista', value: 'a1', label: 'Soda Stereo' },
    ])
    expect(inside('decada', q, counts)).toEqual([{ value: '1986' }])
    expect(inside('genero', q, counts)).toEqual([])
    expect(inside('genero', parse('?genero=Rock'), counts)).toEqual([{ value: 'Pop Rock' }])
  })
})
```

Modify `web/src/lib/format.test.js` (unified diff against the previous task):

```diff
@@ -1,5 +1,5 @@
 import { describe, expect, it } from 'vitest'
-import { codecName, count, duration, folderName, length, quality } from './format.js'
+import { codecName, count, country, duration, folderName, formatName, length, quality, years } from './format.js'
 
 describe('durations', () => {
   it('reads track lengths and totals', () => {
@@ -45,3 +45,21 @@ describe('folderName', () => {
     expect(folderName('')).toBe('')
   })
 })
+
+describe('names', () => {
+  it('names countries and formats', () => {
+    expect(country('AR')).toBe('Argentina')
+    expect(country('BR')).toBe('Brasil')
+    expect(country('')).toBe('')
+    expect(country('ZZZ')).toBe('ZZZ')
+    expect(formatName('flac')).toBe('FLAC')
+    expect(formatName('wma')).toBe('WMA')
+    expect(formatName('aiff')).toBe('AIFF')
+    expect(formatName('xyz')).toBe('XYZ')
+  })
+  it('reads year spans', () => {
+    expect(years(1982, 2007)).toBe('1982–2007')
+    expect(years(1990, 1990)).toBe('1990')
+    expect(years(0, 0)).toBe('')
+  })
+})
```

Create `web/src/lib/review.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { aliasLabel, candidateLine, genreChoice, genreLinks, genreOptions, identification, percent, searchURL, worthAccepting } from './review.js'

describe('review', () => {
  it('reads scores and candidates', () => {
    expect(percent(0.873)).toBe('87 %')
    expect(percent(undefined)).toBe('0 %')
    expect(candidateLine({ artist: 'Soda Stereo', year: 1986, country: 'AR', tracks: 9 })).toBe('Soda Stereo · 1986 · Argentina · 9 pistas')
    expect(candidateLine({ artist: 'X', year: 0, country: '' })).toBe('X')
  })
  it('says where the data comes from', () => {
    expect(identification({ status: 'auto', confidence: 0.92 })).toBe('Identificado en MusicBrainz (92 %)')
    expect(identification({ status: 'ignored' })).toBe('No está en MusicBrainz')
    expect(identification({ status: '' })).toBe('Todavía sin buscar en MusicBrainz')
  })
  it('links genres and styles', () => {
    const gs = [
      { genre: 'Rock', style: 'Pop Rock' },
      { genre: 'Rock', style: 'New Wave' },
      { genre: 'Latin', source: 'tag' },
    ]
    expect(genreLinks(gs)).toEqual([
      { label: 'Rock', facets: { genero: 'Rock' } },
      { label: 'Latin', facets: { genero: 'Latin' } },
      { label: 'Pop Rock', facets: { genero: 'Rock', estilo: 'Pop Rock' } },
      { label: 'New Wave', facets: { genero: 'Rock', estilo: 'New Wave' } },
    ])
    expect(genreLinks(undefined)).toEqual([])
  })
  it('picks the albums worth accepting', () => {
    const q = [{ id: 1, candidates: [{ score: 0.78 }] }, { id: 2, candidates: [{ score: 0.5 }] }, { id: 3, candidates: [] }]
    expect(worthAccepting(q).map((a) => a.id)).toEqual([1])
  })
  it('maps raw genres', () => {
    const opts = genreOptions([{ name: 'Folk, World, & Country', styles: ['Folk'] }, { name: 'Latin', styles: ['Samba'] }])
    expect(opts.map((o) => o.label)).toEqual([
      'No es un género (ignorarlo)',
      'Folk, World, & Country',
      'Folk, World, & Country › Folk',
      'Latin',
      'Latin › Samba',
    ])
    expect(genreChoice(opts[0].value)).toEqual({ genre: '', style: '' })
    expect(genreChoice(opts[2].value)).toEqual({ genre: 'Folk, World, & Country', style: 'Folk' })
    expect(genreChoice(opts[3].value)).toEqual({ genre: 'Latin', style: '' })
    expect(aliasLabel({ raw: 'cueca', genre: '' })).toBe('ignorado')
    expect(aliasLabel({ raw: 'cueca', genre: 'Latin', style: 'Samba' })).toBe('Latin › Samba')
  })
  it('looks albums up on MusicBrainz', () => {
    expect(searchURL({ title: 'Signos (MB)', tagTitle: 'Signos', artist: 'Soda Stereo' })).toBe(
      'https://musicbrainz.org/search?type=release&query=Signos%20Soda%20Stereo',
    )
  })
})
```

Modify `web/src/lib/router.test.js` (unified diff against the previous task):

```diff
@@ -1,15 +1,21 @@
 import { describe, expect, it } from 'vitest'
-import { albumHref, appLink, folderHref, resolve } from './router.js'
+import { albumHref, appLink, artistHref, exploreHref, folderHref, resolve, searchHref } from './router.js'
 
 describe('resolve', () => {
   it.each([
-    ['/', '', { page: 'carpetas', path: '' }],
+    ['/', '', { page: 'explorar' }],
+    ['/explorar', '?decada=1980', { page: 'explorar' }],
     ['/carpetas', '', { page: 'carpetas', path: '' }],
     ['/carpetas', '?ruta=..%2Fmusica%2FLatinoam%C3%A9rica%20600', { page: 'carpetas', path: '../musica/Latinoamérica 600' }],
     ['/listas', '', { page: 'listas' }],
     ['/album/12', '', { page: 'album', id: 12 }],
     ['/album/0', '', { page: 'notfound' }],
     ['/album/abc', '', { page: 'notfound' }],
+    ['/artista/charly-garcia', '', { page: 'artista', key: 'charly-garcia' }],
+    ['/artista/%E0%A4%A', '', { page: 'notfound' }],
+    ['/buscar', '?q=soda', { page: 'buscar', q: 'soda' }],
+    ['/revisar', '', { page: 'revisar', tab: 'sin-identificar' }],
+    ['/revisar/generos', '', { page: 'revisar', tab: 'generos' }],
     ['/otra', '', { page: 'notfound' }],
   ])('%s%s', (path, search, want) => {
     expect(resolve(path, search)).toEqual(want)
@@ -24,6 +30,17 @@ describe('hrefs', () => {
     expect(folderHref('')).toBe('/carpetas')
     expect(albumHref(7)).toBe('/album/7')
   })
+  it('round-trips artist keys', () => {
+    const key = 'sui-generis/x?'
+    const url = new URL(artistHref(key), 'http://127.0.0.1')
+    expect(resolve(url.pathname, url.search)).toEqual({ page: 'artista', key })
+  })
+  it('links Explorar and the search', () => {
+    expect(exploreHref()).toBe('/explorar')
+    expect(exploreHref({ pais: 'AR', genero: 'Folk, World, & Country' })).toBe('/explorar?pais=AR&genero=Folk%2C+World%2C+%26+Country')
+    expect(exploreHref({ artista: 'x' }, 'pistas')).toBe('/explorar?ver=pistas&artista=x')
+    expect(searchHref(' soda stereo ')).toBe('/buscar?q=soda%20stereo')
+  })
 })
 
 function anchor(href, attrs = {}) {
```

Create `web/src/lib/search.test.js`:

```js
import { describe, expect, it } from 'vitest'
import { enterHref, isShortcut, move, options, searchable, total, trackHref } from './search.js'

const result = {
  artists: [{ key: 'charly-garcia', name: 'Charly García' }],
  albums: [{ id: 3, title: 'Clics modernos' }],
  tracks: [{ id: 9, albumId: 3 }, { id: 10, albumId: 0, dir: '../musica/youtube' }],
  folders: [{ path: '../musica/Latinoamérica 600' }],
  totals: { artists: 1, albums: 4, tracks: 2, folders: 1 },
}

describe('search', () => {
  it('asks only with enough letters or digits', () => {
    expect(searchable('so')).toBe(true)
    expect(searchable('s!')).toBe(false)
    expect(searchable('  ')).toBe(false)
    expect(searchable(undefined)).toBe(false)
  })
  it('lists options in keyboard order', () => {
    expect(options(result).map((o) => o.href)).toEqual([
      '/artista/charly-garcia',
      '/album/3',
      '/album/3',
      '/carpetas?ruta=..%2Fmusica%2Fyoutube',
      '/carpetas?ruta=..%2Fmusica%2FLatinoam%C3%A9rica%20600',
    ])
    expect(options(null)).toEqual([])
    expect(total(result)).toBe(8)
    expect(trackHref(result.tracks[1])).toBe('/carpetas?ruta=..%2Fmusica%2Fyoutube')
  })
  it('moves the selection and follows Enter', () => {
    expect(move(-1, 'ArrowDown', 3)).toBe(0)
    expect(move(2, 'ArrowDown', 3)).toBe(-1)
    expect(move(-1, 'ArrowUp', 3)).toBe(2)
    expect(move(0, 'ArrowDown', 0)).toBe(-1)
    const opts = options(result)
    expect(enterHref(1, opts, 'cl')).toBe('/album/3')
    expect(enterHref(-1, opts, 'clics')).toBe('/buscar?q=clics')
    expect(enterHref(-1, opts, 'c')).toBeNull()
  })
  it('opens with Ctrl+K or ⌘K', () => {
    expect(isShortcut({ ctrlKey: true, key: 'k' })).toBe(true)
    expect(isShortcut({ metaKey: true, key: 'K' })).toBe(true)
    expect(isShortcut({ ctrlKey: true, shiftKey: true, key: 'k' })).toBe(false)
    expect(isShortcut({ key: 'k' })).toBe(false)
  })
})
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `(cd web && npx vitest run)`
Expected: FAIL — the new test files fail to import (`does not provide an export named ...`).

- [ ] **Step 3: Write the code**

Modify `web/src/lib/api.js` (unified diff against the previous task):

```diff
@@ -29,13 +29,16 @@ async function call(method, path, body) {
 
 const get = (path) => call('GET', path)
 const post = (path, body) => call('POST', path, body ?? {})
+const put = (path, body) => call('PUT', path, body)
+const del = (path) => call('DELETE', path)
 
 // playQuery writes what to play as /api/play's query: {album}, {folder},
-// {tracks: [ids]}, plus shuffle.
-export function playQuery({ album, folder, tracks, shuffle }) {
+// {artist}, {tracks: [ids]}, plus shuffle.
+export function playQuery({ album, folder, artist, tracks, shuffle }) {
   const params = new URLSearchParams()
   if (album != null) params.set('album', album)
   if (folder != null) params.set('folder', folder)
+  if (artist != null) params.set('artist', artist)
   for (const id of tracks ?? []) params.append('track', id)
   if (shuffle) params.set('shuffle', '1')
   return params.toString()
@@ -48,6 +51,36 @@ export const api = {
   folder: (path) => get(path ? `/api/folders?${new URLSearchParams({ path })}` : '/api/folders'),
   collections: () => get('/api/collections'),
   play: (what) => get(`/api/play?${playQuery(what)}`),
+  // search is Explorar's query string ("?decada=1980"); from and limit page it.
+  explore: (search, from = 0, limit = 120) => {
+    const params = new URLSearchParams(search)
+    if (from) params.set('desde', from)
+    params.set('limite', limit)
+    return get(`/api/explore?${params}`)
+  },
+  artist: (key) => get(`/api/artists/${encodeURIComponent(key)}`),
+  find: (q, limit = 6) => get(`/api/search?${new URLSearchParams({ q, limite: limit })}`),
+  setCollection: (path, collection) => put('/api/folders/collection', { path, collection }),
+  identify: () => post('/api/identify'),
+  unidentified: () => get('/api/review/unidentified'),
+  // identification is {releaseId} (an MBID or a musicbrainz.org URL) or {ignore: true}.
+  setIdentification: (id, identification) => put(`/api/albums/${id}/identification`, identification),
+  resetIdentification: (id) => del(`/api/albums/${id}/identification`),
+  genres: () => get('/api/genres'),
+  unmappedGenres: () => get('/api/genres/unmapped'),
+  genreAliases: () => get('/api/genres/aliases'),
+  setGenreAlias: (alias) => put('/api/genres/aliases', alias),
+  deleteGenreAlias: (raw) => del(`/api/genres/aliases?${new URLSearchParams({ raw })}`),
+}
+
+// releaseURL links a MusicBrainz release.
+export function releaseURL(id) {
+  return `https://musicbrainz.org/release/${id}`
+}
+
+// artistMBURL links a MusicBrainz artist.
+export function artistMBURL(id) {
+  return `https://musicbrainz.org/artist/${id}`
 }
 
 // Picture URLs. size is one of the server's thumbnail sizes (160, 320,
```

Create `web/src/lib/facets.js`:

```js
// Explorar's query: tab, facets and order, as they appear in the URL.
import { country, formatName } from './format.js'

export const VIEWS = [
  { value: 'albumes', label: 'Álbumes' },
  { value: 'artistas', label: 'Artistas' },
  { value: 'pistas', label: 'Pistas' },
]

// FACETS lists the facets of the bar, in order. The year lives inside the
// decade menu and the style inside the genre menu; `more` facets go in the
// "Más" menu.
export const FACETS = [
  { name: 'decada', label: 'Década' },
  { name: 'artista', label: 'Artista' },
  { name: 'pais', label: 'País' },
  { name: 'genero', label: 'Género' },
  { name: 'coleccion', label: 'Colección' },
  { name: 'carpeta', label: 'Carpeta', more: true },
  { name: 'calidad', label: 'Calidad', more: true },
  { name: 'formato', label: 'Formato', more: true },
  { name: 'estado', label: 'Estado', more: true },
]

// NAMES are the facets in URL order, the year and style included.
const NAMES = ['decada', 'anio', 'artista', 'pais', 'genero', 'estilo', 'coleccion', 'carpeta', 'calidad', 'formato', 'estado']

// ORDERS lists each tab's orders, its default first (as the server).
export const ORDERS = {
  albumes: [
    { value: 'anio', label: 'Año' },
    { value: 'titulo', label: 'Título' },
    { value: 'artista', label: 'Artista' },
    { value: 'agregado', label: 'Agregado' },
  ],
  artistas: [
    { value: 'titulo', label: 'Nombre' },
    { value: 'cantidad', label: 'Cantidad de álbumes' },
    { value: 'agregado', label: 'Agregado' },
  ],
  pistas: [
    { value: 'artista', label: 'Artista' },
    { value: 'anio', label: 'Año' },
    { value: 'titulo', label: 'Título' },
    { value: 'agregado', label: 'Agregado' },
  ],
}

export const LABELS = {
  calidad: { 'sin-perdida': 'Sin pérdida', 'con-perdida': 'Con pérdida' },
  estado: { 'sin-identificar': 'Sin identificar', 'no-reproducible': 'No reproducible' },
}

// defaultDir is the natural direction of an order: A to Z for names,
// newest or largest first otherwise.
export function defaultDir(order) {
  return order === 'titulo' || order === 'artista' ? 'asc' : 'desc'
}

function viewOf(v) {
  return ORDERS[v] ? v : 'albumes'
}

// parse reads a query string ("?decada=1970&orden=titulo"). Unknown
// parameters are dropped; the server validates the values.
export function parse(search) {
  const params = new URLSearchParams(search)
  const view = viewOf(params.get('ver'))
  const facets = {}
  for (const name of NAMES) {
    const v = params.get(name)
    if (v) facets[name] = v
  }
  const orders = ORDERS[view]
  const order = orders.some((o) => o.value === params.get('orden')) ? params.get('orden') : orders[0].value
  const dir = ['asc', 'desc'].includes(params.get('dir')) ? params.get('dir') : defaultDir(order)
  return { view, facets, order, dir }
}

// toSearch writes a query back, in a stable order and leaving defaults out,
// so that equal queries give equal URLs.
export function toSearch({ view, facets, order, dir }) {
  const params = new URLSearchParams()
  view = viewOf(view)
  if (view !== 'albumes') params.set('ver', view)
  for (const name of NAMES) {
    if (facets[name]) params.set(name, facets[name])
  }
  const natural = ORDERS[view][0].value
  if (order && order !== natural) params.set('orden', order)
  if (dir && dir !== defaultDir(order || natural)) params.set('dir', dir)
  const s = params.toString()
  return s ? `?${s}` : ''
}

// withFacet sets (or, with a null value, clears) one facet. A year outside
// the chosen decade is dropped, and clearing the decade clears the year; a
// style goes with its genre.
export function withFacet(query, name, value) {
  const facets = { ...query.facets }
  if (value == null || value === '') delete facets[name]
  else facets[name] = String(value)
  if (name === 'decada' && facets.anio && (!facets.decada || !facets.anio.startsWith(facets.decada.slice(0, 3)))) {
    delete facets.anio
  }
  if (name === 'anio' && value && !facets.decada) {
    facets.decada = String(Math.floor(Number(value) / 10) * 10)
  }
  if (name === 'genero') delete facets.estilo
  return { ...query, facets }
}

// withView switches tabs, keeping the facets, with the tab's default order.
export function withView(query, view) {
  view = viewOf(view)
  const order = ORDERS[view][0].value
  return { ...query, view, order, dir: defaultDir(order) }
}

// withOrder changes the order, resetting the direction to its natural one.
export function withOrder(query, order) {
  return { ...query, order, dir: defaultDir(order) }
}

// valueLabel is how a facet value reads in menus and chips. counted is the
// server's {value, label, count} when known.
export function valueLabel(name, value, counted) {
  switch (name) {
    case 'decada':
      return `${value}s`
    case 'pais':
      return country(value)
    case 'formato':
      return formatName(value)
    case 'calidad':
    case 'estado':
      return LABELS[name][value] ?? value
  }
  return counted?.label || value
}

// chips lists the applied facets, labelled with the server's counts.
export function chips(query, counts = {}) {
  const out = []
  for (const name of NAMES) {
    const value = query.facets[name]
    if (!value) continue
    const counted = (counts[name] ?? []).find((v) => v.value === value)
    out.push({ name, value, label: valueLabel(name, value, counted) })
  }
  return out
}

// inside lists the values of a facet nested in a chosen one: the years of
// the chosen decade, the styles of the chosen genre.
export function inside(name, query, counts = {}) {
  if (name === 'decada' && query.facets.decada) {
    return (counts.anio ?? []).filter((v) => v.value.startsWith(query.facets.decada.slice(0, 3)))
  }
  if (name === 'genero' && query.facets.genero) return counts.estilo ?? []
  return []
}
```

Modify `web/src/lib/format.js` (unified diff against the previous task):

```diff
@@ -60,3 +60,34 @@ export function quality(t) {
 export function folderName(path) {
   return (path ?? '').split('/').filter((p) => p && p !== '..').pop() ?? ''
 }
+
+const regions = (() => {
+  try {
+    return new Intl.DisplayNames(['es'], { type: 'region' })
+  } catch {
+    return null
+  }
+})()
+
+// country names an ISO 3166-1 code in Spanish: "AR" → "Argentina".
+export function country(code) {
+  if (!code) return ''
+  try {
+    return regions?.of(code) ?? code
+  } catch {
+    return code
+  }
+}
+
+const FORMATS = { ...CODECS, wav: 'WAV', aiff: 'AIFF', ogg: 'Ogg', mp4: 'MP4' }
+
+// formatName names a value of Explorar's Formato facet ("flac" → "FLAC").
+export function formatName(key) {
+  return FORMATS[key] ?? (key ?? '').toUpperCase()
+}
+
+// years reads a span of years: "1982–2007", or "1975" for one.
+export function years(first, last) {
+  if (!first) return ''
+  return last && last !== first ? `${first}–${last}` : String(first)
+}
```

Create `web/src/lib/review.js`:

```js
// How identifications read: statuses, candidates, confidence.
import { country } from './format.js'

// percent reads a score: 0.873 → "87 %".
export function percent(score) {
  return `${Math.round((score ?? 0) * 100)} %`
}

// candidateLine sums up a MusicBrainz candidate: "Soda Stereo · 1986 ·
// Argentina · 9 pistas".
export function candidateLine(c) {
  return [c.artist, c.year || '', country(c.country), c.tracks ? `${c.tracks} pistas` : ''].filter(Boolean).join(' · ')
}

// identification tells where an album's data comes from.
export function identification(album) {
  switch (album.status) {
    case 'auto':
      return `Identificado en MusicBrainz (${percent(album.confidence)})`
    case 'manual':
      return 'Identificado a mano en MusicBrainz'
    case 'unmatched':
      return 'Sin identificar: revisá los candidatos'
    case 'ignored':
      return 'No está en MusicBrainz'
    case 'selection':
      return 'Selección de la carpeta: MusicBrainz no la conoce'
  }
  return 'Todavía sin buscar en MusicBrainz'
}

// genreLinks lists an album's genres for links to Explorar: each genre once,
// then each style with its genre.
export function genreLinks(genres) {
  const out = []
  const seen = new Set()
  for (const g of genres ?? []) {
    if (!seen.has(g.genre)) {
      seen.add(g.genre)
      out.push({ label: g.genre, facets: { genero: g.genre } })
    }
  }
  for (const g of genres ?? []) {
    if (g.style) out.push({ label: g.style, facets: { genero: g.genre, estilo: g.style } })
  }
  return out
}

// worthAccepting lists the albums of the queue whose best candidate scores
// at least min: likely right, missed for a reissue's year or another
// edition's durations.
export function worthAccepting(albums, min = 0.7) {
  return albums.filter((a) => (a.candidates?.[0]?.score ?? 0) >= min)
}

// searchURL looks an album up on musicbrainz.org, to paste the right release.
export function searchURL(album) {
  const q = [album.tagTitle || album.title, album.tagArtist || album.artist].filter(Boolean).join(' ')
  return `https://musicbrainz.org/search?type=release&query=${encodeURIComponent(q)}`
}

// genreOptions lists what a raw genre can map to: each genre, then its
// styles ("Latin › Samba"), and "not a genre" first. value encodes the
// choice: genreChoice reads it back.
export function genreOptions(taxonomy) {
  const out = [{ value: '|', label: 'No es un género (ignorarlo)' }]
  for (const g of taxonomy ?? []) {
    out.push({ value: `${g.name}|`, label: g.name })
    for (const s of g.styles) out.push({ value: `${g.name}|${s}`, label: `${g.name} › ${s}` })
  }
  return out
}

export function genreChoice(value) {
  const i = value.lastIndexOf('|')
  return { genre: value.slice(0, i), style: value.slice(i + 1) }
}

// aliasLabel reads a user's mapping: "Folk, World, & Country › Folk", or
// "ignorado".
export function aliasLabel(a) {
  if (!a.genre) return 'ignorado'
  return a.style ? `${a.genre} › ${a.style}` : a.genre
}
```

Modify `web/src/lib/router.js` (unified diff against the previous task):

```diff
@@ -3,10 +3,22 @@
 // resolve maps a pathname and query to the page to show and its parameters.
 export function resolve(pathname, search = '') {
   const params = new URLSearchParams(search)
-  if (pathname === '/' || pathname === '/carpetas') return { page: 'carpetas', path: params.get('ruta') ?? '' }
+  if (pathname === '/' || pathname === '/explorar') return { page: 'explorar' }
+  if (pathname === '/carpetas') return { page: 'carpetas', path: params.get('ruta') ?? '' }
   if (pathname === '/listas') return { page: 'listas' }
-  const m = pathname.match(/^\/album\/([1-9][0-9]*)$/)
+  if (pathname === '/buscar') return { page: 'buscar', q: params.get('q') ?? '' }
+  if (pathname === '/revisar') return { page: 'revisar', tab: 'sin-identificar' }
+  if (pathname === '/revisar/generos') return { page: 'revisar', tab: 'generos' }
+  let m = pathname.match(/^\/album\/([1-9][0-9]*)$/)
   if (m) return { page: 'album', id: Number(m[1]) }
+  m = pathname.match(/^\/artista\/([^/]+)$/)
+  if (m) {
+    try {
+      return { page: 'artista', key: decodeURIComponent(m[1]) }
+    } catch {
+      return { page: 'notfound' } // a malformed escape
+    }
+  }
   return { page: 'notfound' }
 }
 
@@ -30,3 +42,20 @@ export function folderHref(path) {
 export function albumHref(id) {
   return `/album/${id}`
 }
+
+export function artistHref(key) {
+  return `/artista/${encodeURIComponent(key)}`
+}
+
+// exploreHref links Explorar with some facets: exploreHref({ pais: 'AR' }).
+export function exploreHref(facets = {}, view = '') {
+  const params = new URLSearchParams()
+  if (view && view !== 'albumes') params.set('ver', view)
+  for (const [k, v] of Object.entries(facets)) if (v) params.set(k, v)
+  const s = params.toString()
+  return s ? `/explorar?${s}` : '/explorar'
+}
+
+export function searchHref(q) {
+  return `/buscar?q=${encodeURIComponent(q.trim())}`
+}
```

Create `web/src/lib/search.js`:

```js
// The instant search: when to ask, where results lead, keyboard selection.
import { albumHref, artistHref, folderHref, searchHref } from './router.js'

// MIN is how many letters or digits a query needs (as in the server).
export const MIN = 2

// searchable reports whether q is worth asking for.
export function searchable(q) {
  return (q ?? '').replace(/[^\p{L}\p{N}]/gu, '').length >= MIN
}

// trackHref is where a found track leads: its album, or its folder when it
// is a loose track.
export function trackHref(t) {
  return t.albumId ? albumHref(t.albumId) : folderHref(t.dir)
}

// options are the entries of the dropdown, in keyboard order: artists,
// albums, tracks, folders.
export function options(result) {
  if (!result) return []
  return [
    ...result.artists.map((a) => ({ kind: 'artist', item: a, href: artistHref(a.key) })),
    ...result.albums.map((a) => ({ kind: 'album', item: a, href: albumHref(a.id) })),
    ...result.tracks.map((t) => ({ kind: 'track', item: t, href: trackHref(t) })),
    ...result.folders.map((f) => ({ kind: 'folder', item: f, href: folderHref(f.path) })),
  ]
}

// total counts everything found, shown or not.
export function total(result) {
  return Object.values(result?.totals ?? {}).reduce((a, b) => a + b, 0)
}

// move returns the selected option after an arrow key: -1 is none (the
// text field), and the selection wraps around.
export function move(selected, key, count) {
  if (count === 0) return -1
  if (key === 'ArrowDown') return selected + 1 >= count ? -1 : selected + 1
  if (key === 'ArrowUp') return selected <= -1 ? count - 1 : selected - 1
  return selected
}

// enterHref is where Enter goes: the selected option, or the results page.
export function enterHref(selected, opts, q) {
  if (selected >= 0 && selected < opts.length) return opts[selected].href
  return searchable(q) ? searchHref(q) : null
}

// isShortcut tells the key that opens the search from anywhere: Ctrl+K, or
// ⌘K on a Mac.
export function isShortcut(event) {
  return !!(event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k'
}
```

- [ ] **Step 4: Run the checks**

```bash
(cd web && npx vitest run)
(cd web && npm run build)
go test ./...
```
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.js web/src/lib/api.test.js web/src/lib/facets.js web/src/lib/facets.test.js web/src/lib/format.js web/src/lib/format.test.js web/src/lib/review.js web/src/lib/review.test.js web/src/lib/router.js web/src/lib/router.test.js web/src/lib/search.js web/src/lib/search.test.js
git commit -m "feat: web routes, API calls, facets, search and review helpers"
```

### Task 9: Explorar

The Explorar page: tabs Álbumes / Artistas / Pistas, the facet bar (Cinexplorer's `BarraFacetas` adapted: nested years and styles, "Más" for Carpeta, Calidad, Formato and Estado), order and direction, chips; albums and artists in a grid, tracks as a list; 120 items per request, the next page when the end comes into view, and as many as were shown when coming back. A click on a track plays the list from there (at most 500 tracks: they go in the request's URL). `ListaPistas` is restructured so a row can hold links (artist, album) on top of its play button (a link inside a `<button>` is invalid HTML), dims tracks the server cannot play, and shows the album with `showAlbum`. Album cards skip the cover request for albums without a picture. The nav gets Explorar first.

**Files:**
- Modify: `web/src/App.svelte`
- Create: `web/src/components/BarraFacetas.svelte`
- Modify: `web/src/components/Icono.svelte`
- Modify: `web/src/components/ListaPistas.svelte`
- Modify: `web/src/components/Nav.svelte`
- Modify: `web/src/components/TarjetaAlbum.svelte`
- Create: `web/src/components/TarjetaArtista.svelte`
- Create: `web/src/pages/Explorar.svelte`

**Interfaces:**
- Consumes: `facets.js`, `api.explore`, `api.js` cover URLs, `player.svelte.js` `play`, `nav.svelte.js` (M3a).
- Produces: components `BarraFacetas` (props `query`, `facets`, `total`, `onchange`), `TarjetaArtista` (`artist`), `ListaPistas` (new prop `showAlbum`; rows read `t.artistKey`, `t.album`, `t.albumId`, `t.playable`); page `Explorar`; icons `explore`, `review`, `search`, `user`, `more`.

- [ ] **Step 1: Write the code**

Modify `web/src/App.svelte` (unified diff against the previous task):

```diff
@@ -6,6 +6,7 @@
   import { appLink, resolve } from './lib/router.js'
   import Album from './pages/Album.svelte'
   import Carpetas from './pages/Carpetas.svelte'
+  import Explorar from './pages/Explorar.svelte'
   import Listas from './pages/Listas.svelte'
 
   const current = $derived(resolve(route.path, route.search))
@@ -28,7 +29,9 @@
 <Nav page={current.page} />
 
 <main>
-  {#if current.page === 'carpetas'}
+  {#if current.page === 'explorar'}
+    <Explorar />
+  {:else if current.page === 'carpetas'}
     {#key current.path}
       <Carpetas path={current.path} />
     {/key}
@@ -39,7 +42,7 @@
   {:else if current.page === 'listas'}
     <Listas />
   {:else}
-    <p class="empty">No existe esta página. <a href="/carpetas">Ir a Carpetas</a></p>
+    <p class="empty">No existe esta página. <a href="/explorar">Ir a Explorar</a></p>
   {/if}
 </main>
```

Create `web/src/components/BarraFacetas.svelte`:

```svelte
<script>
  // Explorar's bar: tabs, facet menus, order, and the chips of what is
  // applied.
  import { count } from '../lib/format.js'
  import { chips, FACETS, inside, ORDERS, valueLabel, VIEWS, withFacet, withOrder, withView } from '../lib/facets.js'

  // query: the applied query; facets: the server's counts per facet;
  // onchange(query): a new query was chosen.
  let { query, facets, total, onchange } = $props()

  let open = $state(null) // name of the open menu, "more", or null
  let filters = $state({}) // text filter of each facet's menu: "Más" shows several at once
  let bar = $state()

  const main = FACETS.filter((f) => !f.more)
  const more = FACETS.filter((f) => f.more)
  const applied = $derived(chips(query, facets))
  const NESTED = { decada: { name: 'anio', label: 'Años' }, genero: { name: 'estilo', label: 'Estilos' } }

  function toggle(name) {
    open = open === name ? null : name
    filters = {}
  }

  function choose(name, value) {
    open = null
    onchange(withFacet(query, name, query.facets[name] === value ? null : value))
  }

  function values(name) {
    const all = facets?.[name] ?? []
    const f = (filters[name] ?? '').trim().toLowerCase()
    if (!f) return all
    return all.filter((v) => valueLabel(name, v.value, v).toLowerCase().includes(f))
  }

  function onwindowclick(event) {
    if (open && bar && !bar.contains(event.target)) open = null
  }

  // What the total counts: artists, tracks, or albums.
  const totalLabel = $derived(
    query.view === 'artistas'
      ? count(total, 'artista', 'artistas')
      : query.view === 'pistas'
        ? count(total, 'pista', 'pistas')
        : count(total, 'álbum', 'álbumes'),
  )
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = null)} />

{#snippet options(name, list)}
  <ul>
    {#each list as v (v.value)}
      <li>
        <button class="option" class:chosen={query.facets[name] === v.value} onclick={() => choose(name, v.value)}>
          <span>{valueLabel(name, v.value, v)}</span><span class="count">{v.count}</span>
        </button>
      </li>
    {:else}
      <li class="none">Sin valores</li>
    {/each}
  </ul>
{/snippet}

{#snippet menu(name)}
  {@const nested = NESTED[name]}
  {#if (facets?.[name] ?? []).length > 12}
    <!-- svelte-ignore a11y_autofocus -->
    <input type="search" placeholder="Filtrar…" bind:value={filters[name]} autofocus />
  {/if}
  {@render options(name, values(name))}
  {#if nested && inside(name, query, facets).length}
    <div class="label sub">{nested.label}</div>
    {@render options(nested.name, inside(name, query, facets))}
  {/if}
{/snippet}

<div class="tabs" role="tablist">
  {#each VIEWS as v (v.value)}
    <button role="tab" class:on={query.view === v.value} aria-selected={query.view === v.value} onclick={() => onchange(withView(query, v.value))}>
      {v.label}
    </button>
  {/each}
</div>

<div class="bar" bind:this={bar}>
  {#each main as f (f.name)}
    <div class="facet">
      <button class="dd" class:on={query.facets[f.name]} onclick={() => toggle(f.name)} aria-expanded={open === f.name}>
        {f.label} ▾
      </button>
      {#if open === f.name}
        <div class="menu">{@render menu(f.name)}</div>
      {/if}
    </div>
  {/each}
  <div class="facet">
    <button class="dd" class:on={more.some((f) => query.facets[f.name])} onclick={() => toggle('more')} aria-expanded={open === 'more'}>
      Más ▾
    </button>
    {#if open === 'more'}
      <div class="menu wide">
        {#each more as f (f.name)}
          <div class="label sub">{f.label}</div>
          {@render menu(f.name)}
        {/each}
      </div>
    {/if}
  </div>

  <div class="right">
    <span class="total">{totalLabel}</span>
    <select value={query.order} onchange={(e) => onchange(withOrder(query, e.currentTarget.value))} aria-label="Orden">
      {#each ORDERS[query.view] as o (o.value)}
        <option value={o.value}>{o.label}</option>
      {/each}
    </select>
    <button
      class="dir"
      onclick={() => onchange({ ...query, dir: query.dir === 'asc' ? 'desc' : 'asc' })}
      aria-label={query.dir === 'asc' ? 'Ascendente' : 'Descendente'}
      title={query.dir === 'asc' ? 'Ascendente' : 'Descendente'}>{query.dir === 'asc' ? '↑' : '↓'}</button
    >
  </div>
</div>

{#if applied.length}
  <div class="chips">
    {#each applied as c (c.name)}
      <button class="chip" onclick={() => onchange(withFacet(query, c.name, null))} aria-label={`Quitar ${c.label}`}>
        {c.label} ✕
      </button>
    {/each}
    {#if applied.length > 1}
      <button class="chip clear" onclick={() => onchange({ ...query, facets: {} })}>Quitar todo</button>
    {/if}
  </div>
{/if}

<style>
  .tabs {
    display: flex;
    gap: 20px;
    margin-bottom: 12px;
  }
  .tabs button {
    border: none;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    padding: 4px 0;
    font-size: 15px;
    color: var(--muted);
  }
  .tabs button.on {
    color: var(--strong);
    border-bottom-color: var(--accent);
  }
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--line);
    font-size: 13px;
  }
  .facet {
    position: relative;
  }
  .dd {
    color: var(--muted);
    border-color: var(--line);
    padding: 3px 10px;
  }
  .dd.on {
    color: var(--accent);
    border-color: var(--accent);
  }
  .menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 15;
    min-width: 220px;
    max-height: 60vh;
    overflow-y: auto;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 6px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.55);
  }
  .menu.wide {
    min-width: 260px;
  }
  .menu input {
    width: 100%;
    margin-bottom: 6px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .option {
    width: 100%;
    display: flex;
    justify-content: space-between;
    gap: 12px;
    border: none;
    padding: 4px 8px;
    text-align: left;
  }
  .option:hover {
    background: var(--surface-2);
  }
  .option.chosen {
    color: var(--accent);
  }
  .count {
    color: var(--faint);
  }
  .none {
    color: var(--faint);
    padding: 4px 8px;
  }
  .sub {
    padding: 8px 8px 2px;
  }
  .right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .total {
    color: var(--faint);
  }
  .dir {
    padding: 3px 9px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 10px;
  }
  .chip {
    border-color: var(--accent);
    color: var(--accent);
    font-size: 13px;
    padding: 2px 10px;
    border-radius: 12px;
  }
  .chip.clear {
    border-color: var(--line-strong);
    color: var(--muted);
  }
  @media (max-width: 640px) {
    .right {
      margin-left: 0;
      width: 100%;
    }
    .menu {
      position: fixed;
      left: 16px;
      right: 16px;
      top: 120px;
    }
  }
</style>
```

Modify `web/src/components/Icono.svelte` (unified diff against the previous task):

```diff
@@ -21,6 +21,12 @@
     down: '<polyline points="6 9 12 15 18 9"/>',
     plus: '<line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>',
     next_up: '<polyline points="13 17 18 12 13 7"/><polyline points="6 17 11 12 6 7"/>',
+    explore: '<circle cx="12" cy="12" r="10"/><polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76"/>',
+    review:
+      '<polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>',
+    search: '<circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>',
+    user: '<path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
+    more: '<circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/>',
   }
 
   let { name, size = 18 } = $props()
```

Modify `web/src/components/ListaPistas.svelte` (unified diff against the previous task):

```diff
@@ -1,13 +1,17 @@
 <script>
   // A list of tracks: a click plays from that track. Disc headers when the
-  // tracks span several discs; the artist when it is not the album's.
+  // tracks span several discs; the artist when it is not the album's (with
+  // a link to their page when there is one); the album with showAlbum.
+  // Tracks the server cannot deliver (playable false) are dimmed.
   import { duration, quality } from '../lib/format.js'
   import { enqueue } from '../lib/player.svelte.js'
+  import { albumHref, artistHref } from '../lib/router.js'
   import Icono from './Icono.svelte'
 
-  let { tracks, onplay, albumArtist = '', numbered = true } = $props()
+  let { tracks, onplay, albumArtist = '', numbered = true, showAlbum = false } = $props()
 
   const discs = $derived(new Set(tracks.map((t) => t.disc || 1)).size)
+  const otherArtist = (t) => t.artist && t.artist !== albumArtist
 </script>
 
 <ol class="pistas">
@@ -15,16 +19,29 @@
     {#if discs > 1 && (i === 0 || (tracks[i - 1].disc || 1) !== (t.disc || 1))}
       <li class="disc label">Disco {t.disc || 1}</li>
     {/if}
-    <li class="row">
-      <button class="play" onclick={() => onplay(t)}>
-        <span class="n">{numbered ? t.track || '' : ''}</span>
-        <span class="text">
-          <span class="title">{t.title}</span>
-          {#if t.artist && t.artist !== albumArtist}<span class="artist">{t.artist}</span>{/if}
-        </span>
-        <span class="quality">{quality(t)}</span>
-        <span class="time">{duration(t.durationMs)}</span>
-      </button>
+    <li class="row" class:unplayable={t.playable === false}>
+      <!-- The whole row plays the track; the links on top of it stay links. -->
+      <button
+        class="play"
+        onclick={() => onplay(t)}
+        aria-label="Reproducir {t.title}"
+        title={t.playable === false ? 'No se puede reproducir: falta ffmpeg' : undefined}></button>
+      <span class="n">{numbered ? t.track || '' : ''}</span>
+      <span class="text">
+        <span class="title">{t.title}</span>
+        {#if otherArtist(t) || (showAlbum && t.album)}
+          <span class="artist">
+            {#if otherArtist(t)}
+              {#if t.artistKey}<a href={artistHref(t.artistKey)}>{t.artist}</a>{:else}{t.artist}{/if}
+            {/if}
+            {#if showAlbum && t.album}
+              {otherArtist(t) ? ' · ' : ''}<a href={albumHref(t.albumId)}>{t.album}</a>
+            {/if}
+          </span>
+        {/if}
+      </span>
+      <span class="quality">{quality(t)}</span>
+      <span class="time">{duration(t.durationMs)}</span>
       <button class="icon add" onclick={() => enqueue({ tracks: [t.id] })} title="Agregar a la cola" aria-label="Agregar a la cola">
         <Icono name="plus" size={16} />
       </button>
@@ -42,23 +59,32 @@
     padding: 16px 8px 6px;
   }
   .row {
+    position: relative;
     display: flex;
     align-items: center;
+    gap: 12px;
+    padding: 0 0 0 8px;
     border-bottom: 1px solid var(--line);
   }
+  .row:hover {
+    background: var(--surface);
+  }
   .play {
-    flex: 1;
-    min-width: 0;
-    display: flex;
-    align-items: center;
-    gap: 12px;
+    position: absolute;
+    inset: 0;
     border: none;
     border-radius: 0;
-    padding: 8px;
-    text-align: left;
+    padding: 0;
   }
-  .play:hover {
-    background: var(--surface);
+  /* Everything sits on the play button and lets clicks through, but links
+     and the queue button. */
+  .row > :not(.play) {
+    position: relative;
+    pointer-events: none;
+  }
+  .row a,
+  .row .add {
+    pointer-events: auto;
   }
   .n {
     width: 24px;
@@ -72,6 +98,7 @@
     min-width: 0;
     display: flex;
     flex-direction: column;
+    padding: 8px 0;
   }
   .title,
   .artist {
@@ -86,6 +113,13 @@
     font-size: 13px;
     color: var(--muted);
   }
+  .artist a:hover {
+    color: var(--text);
+    text-decoration: underline;
+  }
+  .unplayable .title {
+    color: var(--faint);
+  }
   .quality {
     font-size: 12px;
     color: var(--faint);
```

Modify `web/src/components/Nav.svelte` (unified diff against the previous task):

```diff
@@ -4,9 +4,9 @@
 
   let { page } = $props()
 
-  // The sections of the app; Inicio, Explorar and Revisar come with later
-  // milestones.
+  // The sections of the app; Inicio comes with M4.
   const SECTIONS = [
+    { page: 'explorar', href: '/explorar', label: 'Explorar', icon: 'explore' },
     { page: 'carpetas', href: '/carpetas', label: 'Carpetas', icon: 'folder' },
     { page: 'listas', href: '/listas', label: 'Listas', icon: 'lists' },
   ]
```

Modify `web/src/components/TarjetaAlbum.svelte` (unified diff against the previous task):

```diff
@@ -12,7 +12,7 @@
 <div class="card">
   <div class="pic">
     <a href={albumHref(album.id)}>
-      <Tapa srcs={[albumCoverURL(album.id, 320)]} alt={album.title} />
+      <Tapa srcs={album.picture === false ? [] : [albumCoverURL(album.id, 320)]} alt={album.title} />
     </a>
     <button class="icon play" onclick={() => play({ album: album.id })} title="Reproducir" aria-label="Reproducir {album.title}">
       <Icono name="play" size={18} />
```

Create `web/src/components/TarjetaArtista.svelte`:

```svelte
<script>
  // An artist card: a round picture (one of their album covers), name and
  // how many albums.
  import { albumCoverURL } from '../lib/api.js'
  import { count } from '../lib/format.js'
  import { artistHref } from '../lib/router.js'
  import Tapa from './Tapa.svelte'

  let { artist } = $props()
</script>

<a class="card" href={artistHref(artist.key)}>
  <Tapa srcs={artist.cover ? [albumCoverURL(artist.cover, 320)] : []} icon="user" round alt={artist.name} />
  <span class="name">{artist.name}</span>
  <span class="sub">{count(artist.albums, 'álbum', 'álbumes')}</span>
</a>

<style>
  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    min-width: 0;
    text-align: center;
  }
  .name {
    margin-top: 6px;
    max-width: 100%;
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

Create `web/src/pages/Explorar.svelte`:

```svelte
<script>
  // Explorar: albums, artists or tracks of the whole catalog, filtered by
  // facets that live in the URL, loaded a page at a time as the list scrolls.
  import { tick } from 'svelte'
  import BarraFacetas from '../components/BarraFacetas.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import TarjetaArtista from '../components/TarjetaArtista.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { parse, toSearch } from '../lib/facets.js'
  import { navigate, patchState, route, savedScroll } from '../lib/nav.svelte.js'
  import { play } from '../lib/player.svelte.js'

  const PAGE = 120

  let data = $state(null) // the first page, with the facets and total
  let items = $state([]) // every item loaded so far
  let more = false // a next page is loading
  let sentinel = $state()
  let loadedSearch = null
  let loads = 0

  const query = $derived(parse(route.search))
  const key = (v) => (v === 'artistas' ? 'artists' : v === 'pistas' ? 'tracks' : 'albums')

  // Load on every change of the URL, and again when a scan or an
  // identification run ends (same URL: the list refreshes in place).
  $effect(() => {
    const search = route.search
    app.generation
    load(search)
  })

  async function load(search) {
    const id = ++loads
    const fresh = search !== loadedSearch
    // Coming back to a long list: load as much of it as was shown.
    const want = fresh ? Math.max(PAGE, history.state?.shown ?? PAGE) : Math.max(PAGE, items.length)
    let d
    try {
      d = await api.explore(search, 0, Math.min(500, want))
    } catch (e) {
      if (id === loads) notify(e.message)
      return
    }
    if (id !== loads || (route.path !== '/explorar' && route.path !== '/')) return // a newer request is on its way
    // The server drops malformed facets: show the URL of what it applied.
    const applied = toSearch(d.query)
    if (applied !== toSearch(parse(search))) {
      navigate('/explorar' + applied, { replace: true })
      return
    }
    data = d
    items = d[key(d.query.view)] ?? []
    loadedSearch = search
    if (fresh) {
      await tick()
      window.scrollTo(0, savedScroll())
    }
  }

  async function loadMore() {
    if (more || !data || items.length >= data.total) return
    more = true
    const id = loads
    try {
      const d = await api.explore(loadedSearch, items.length, PAGE)
      if (id === loads) {
        items = [...items, ...(d[key(d.query.view)] ?? [])]
        patchState({ shown: items.length })
      }
    } catch (e) {
      notify(e.message)
    } finally {
      more = false
    }
  }

  // playFrom plays the listed tracks from t on (a long list in parts: the
  // tracks go in the request's URL).
  function playFrom(t) {
    const i = items.indexOf(t)
    play({ tracks: items.slice(i, i + 500).map((x) => x.id) })
  }

  function change(q) {
    navigate('/explorar' + toSearch(q))
  }

  // Load the next page when the end of the list comes into view.
  $effect(() => {
    if (!sentinel) return
    const io = new IntersectionObserver((entries) => entries[0].isIntersecting && loadMore(), { rootMargin: '800px' })
    io.observe(sentinel)
    return () => io.disconnect()
  })

  const scanning = $derived(!!app.status?.scan?.running)
  const filtered = $derived(Object.keys(query.facets).length > 0)
</script>

{#if data}
  <BarraFacetas query={data.query} facets={data.facets} total={data.total} onchange={change} />
  {#if items.length}
    {#if data.query.view === 'pistas'}
      <div class="list">
        <ListaPistas
          tracks={items}
          numbered={false}
          showAlbum
          onplay={playFrom} />
      </div>
    {:else}
      <div class="grid list">
        {#each items as it (data.query.view === 'artistas' ? it.key : it.id)}
          {#if data.query.view === 'artistas'}
            <TarjetaArtista artist={it} />
          {:else}
            <TarjetaAlbum album={it} />
          {/if}
        {/each}
      </div>
    {/if}
    <div bind:this={sentinel}></div>
  {:else if filtered}
    <p class="empty">Nada con estos filtros. <a href="/explorar">Quitar filtros</a></p>
  {:else if scanning}
    <p class="empty">Escaneando… La música va a aparecer a medida que se encuentre.</p>
  {:else}
    <p class="empty">Todavía no hay música en el catálogo. Revisá las carpetas de config.json.</p>
  {/if}
{/if}

<style>
  .list {
    margin-top: 16px;
  }
</style>
```

- [ ] **Step 2: Run the checks**

```bash
(cd web && npx vitest run)
(cd web && npm run build)
go test ./...
```
Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/App.svelte web/src/components/BarraFacetas.svelte web/src/components/Icono.svelte web/src/components/ListaPistas.svelte web/src/components/Nav.svelte web/src/components/TarjetaAlbum.svelte web/src/components/TarjetaArtista.svelte web/src/pages/Explorar.svelte
git commit -m "feat: Explorar with facets, artist cards and track lists with links"
```

### Task 10: Artist page and search

`Artista`: round picture (one of their covers), type, country (a link to Explorar), years, "Ver sus pistas" (Explorar's Pistas for the artist), MusicBrainz link, play/shuffle/queue the artist, their albums and their appearances. `Busqueda` in the header (Ctrl+K / ⌘K): results while typing (4 of each kind), arrows and Enter, "Ver los N resultados" → `Buscar`, the results page (200 of each kind).

**Files:**
- Modify: `web/src/App.svelte`
- Create: `web/src/components/Busqueda.svelte`
- Modify: `web/src/components/Nav.svelte`
- Create: `web/src/pages/Artista.svelte`
- Create: `web/src/pages/Buscar.svelte`

**Interfaces:**
- Consumes: `api.artist`, `api.find`, `search.js`, `router.js`, `TarjetaAlbum`, `TarjetaArtista`, `ListaPistas`, `Acciones` (with `{artist}`).
- Produces: pages `Artista` (prop `key`), `Buscar` (prop `q`); component `Busqueda`.

- [ ] **Step 1: Write the code**

Modify `web/src/App.svelte` (unified diff against the previous task):

```diff
@@ -5,6 +5,8 @@
   import { navigate, route } from './lib/nav.svelte.js'
   import { appLink, resolve } from './lib/router.js'
   import Album from './pages/Album.svelte'
+  import Artista from './pages/Artista.svelte'
+  import Buscar from './pages/Buscar.svelte'
   import Carpetas from './pages/Carpetas.svelte'
   import Explorar from './pages/Explorar.svelte'
   import Listas from './pages/Listas.svelte'
@@ -39,6 +41,12 @@
     {#key current.id}
       <Album id={current.id} />
     {/key}
+  {:else if current.page === 'artista'}
+    {#key current.key}
+      <Artista key={current.key} />
+    {/key}
+  {:else if current.page === 'buscar'}
+    <Buscar q={current.q} />
   {:else if current.page === 'listas'}
     <Listas />
   {:else}
```

Create `web/src/components/Busqueda.svelte`:

```svelte
<script>
  // The instant search in the header: Ctrl+K / ⌘K opens it, results come
  // while typing, arrows and Enter pick one.
  import { tick } from 'svelte'
  import { albumCoverURL, api, itemCoverURL } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { navigate, route } from '../lib/nav.svelte.js'
  import { searchHref } from '../lib/router.js'
  import { enterHref, isShortcut, move, options, searchable, total } from '../lib/search.js'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  const GROUPS = { artist: 'Artistas', album: 'Álbumes', track: 'Pistas', folder: 'Carpetas' }

  let open = $state(false)
  let q = $state('')
  let result = $state(null)
  let selected = $state(-1)
  let input = $state()
  let box = $state()
  let timer
  let asks = 0

  const opts = $derived(options(result))
  const found = $derived(total(result))

  async function show() {
    open = true
    await tick()
    input?.focus()
    input?.select()
  }

  function close() {
    open = false
    selected = -1
  }

  // Ask after a pause in the typing; answers to older queries are dropped.
  function oninput() {
    clearTimeout(timer)
    selected = -1
    if (!searchable(q)) {
      asks++
      result = null
      return
    }
    timer = setTimeout(ask, 150)
  }

  async function ask() {
    const id = ++asks
    try {
      const r = await api.find(q, 4)
      if (id === asks) result = r
    } catch (e) {
      if (id === asks) notify(e.message)
    }
  }

  function go(href) {
    if (!href) return
    close()
    navigate(href)
  }

  function onkeydown(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      selected = move(selected, event.key, opts.length)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      go(enterHref(selected, opts, q))
    } else if (event.key === 'Escape') {
      close()
    }
  }

  function onwindowkeydown(event) {
    if (isShortcut(event)) {
      event.preventDefault()
      show()
    }
  }

  // The path is fixed when the click starts: the button that opened the
  // search is gone from the page by the time the click reaches the window.
  function onwindowclick(event) {
    if (open && box && !event.composedPath().includes(box)) close()
  }

  // Following a result (or any link) closes the search.
  $effect(() => {
    route.path
    route.search
    close()
  })

  function picture(o) {
    switch (o.kind) {
      case 'artist':
        return o.item.cover ? [albumCoverURL(o.item.cover, 160)] : []
      case 'album':
        return [albumCoverURL(o.item.id, 160)]
      case 'track':
        return [itemCoverURL(o.item, 160)]
    }
    return []
  }

  function meta(o) {
    const it = o.item
    switch (o.kind) {
      case 'artist':
        return count(it.albums, 'álbum', 'álbumes')
      case 'album':
        return [it.artist, it.year || ''].filter(Boolean).join(' · ')
      case 'track':
        return [it.artist, it.album].filter(Boolean).join(' · ')
    }
    return it.collection ? 'Colección' : count(it.tracks, 'pista', 'pistas')
  }

  const ICONS = { artist: 'user', album: 'disc', track: 'disc', folder: 'folder' }
</script>

<svelte:window onkeydown={onwindowkeydown} onclick={onwindowclick} />

<div class="busqueda" bind:this={box}>
  {#if open}
    <input
      bind:this={input}
      bind:value={q}
      {oninput}
      {onkeydown}
      type="search"
      placeholder="Artista, álbum, tema, carpeta…"
      aria-label="Buscar en el catálogo"
      autocomplete="off"
      spellcheck="false"
    />
    {#if result && searchable(q)}
      <div class="panel" role="listbox">
        {#if found === 0}
          <p class="none">Nada con «{q}».</p>
        {/if}
        {#each opts as o, i (o.kind + (o.item.key ?? o.item.id ?? o.item.path))}
          {#if i === 0 || opts[i - 1].kind !== o.kind}
            <div class="label group">{GROUPS[o.kind]}</div>
          {/if}
          <a class="option" class:selected={selected === i} href={o.href} onclick={close} role="option" aria-selected={selected === i}>
            <span class="thumb"><Tapa srcs={picture(o)} icon={ICONS[o.kind]} round={o.kind === 'artist'} /></span>
            <span class="text">
              <span class="title">{o.item.name ?? o.item.title}</span>
              <span class="meta">{meta(o)}</span>
            </span>
          </a>
        {/each}
        {#if found > opts.length}
          <a class="all" href={searchHref(q)} onclick={close}>Ver los {found.toLocaleString('es')} resultados →</a>
        {/if}
      </div>
    {/if}
  {:else}
    <button class="icon" onclick={show} title="Buscar (Ctrl+K)" aria-label="Buscar">
      <Icono name="search" size={18} />
    </button>
  {/if}
</div>

<style>
  .busqueda {
    position: relative;
  }
  input {
    width: 280px;
  }
  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 8px);
    width: 380px;
    max-height: calc(100vh - 80px);
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.6);
    padding: 6px 0;
    z-index: 40;
  }
  .option {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 5px 12px;
  }
  .option.selected,
  .option:hover {
    background: var(--line);
  }
  .thumb {
    flex: none;
    width: 36px;
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .title,
  .meta {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .title {
    color: var(--strong);
  }
  .meta {
    font-size: 12px;
    color: var(--faint);
  }
  .group {
    padding: 10px 12px 4px;
  }
  .none {
    margin: 0;
    padding: 8px 12px;
    color: var(--muted);
  }
  .all {
    display: block;
    padding: 8px 12px 4px;
    font-size: 13px;
    color: var(--accent);
    border-top: 1px solid var(--line);
    margin-top: 6px;
  }
  @media (max-width: 640px) {
    .busqueda:has(input) {
      position: static;
    }
    input {
      position: absolute;
      left: var(--gutter);
      right: var(--gutter);
      top: 8px;
      width: auto;
      z-index: 41;
    }
    .panel {
      left: var(--gutter);
      right: var(--gutter);
      width: auto;
      top: 52px;
    }
  }
</style>
```

Modify `web/src/components/Nav.svelte` (unified diff against the previous task):

```diff
@@ -1,4 +1,5 @@
 <script>
+  import Busqueda from './Busqueda.svelte'
   import Estado from './Estado.svelte'
   import Icono from './Icono.svelte'
 
@@ -27,6 +28,7 @@
       {/each}
     </span>
     <span class="spacer"></span>
+    <Busqueda />
     <Estado />
   </nav>
 </header>
```

Create `web/src/pages/Artista.svelte`:

```svelte
<script>
  // An artist: picture, country, years, their albums by year and their
  // tracks on others' albums (compilations).
  import Acciones from '../components/Acciones.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import Tapa from '../components/Tapa.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import { albumCoverURL, api, artistMBURL } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count, country, years } from '../lib/format.js'
  import { play } from '../lib/player.svelte.js'
  import { albumHref, exploreHref } from '../lib/router.js'

  let { key } = $props()

  let artist = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    let stale = false
    api
      .artist(key)
      .then((a) => {
        if (!stale) (artist = a), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.status === 404 ? 'Este artista ya no está en el catálogo.' : e.message
      })
    return () => (stale = true)
  })

  // span is the years of the artist's albums (MusicBrainz's begin is a
  // person's birth, not when they started).
  const span = $derived(artist ? years(artist.firstYear, artist.lastYear) : '')

  const TYPES = { Person: 'Solista', Group: 'Grupo', Orchestra: 'Orquesta', Choir: 'Coro' }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if artist}
  <div class="head">
    <div class="pic">
      <Tapa srcs={artist.cover ? [albumCoverURL(artist.cover, 640)] : []} icon="user" round alt={artist.name} />
    </div>
    <div class="info">
      <span class="label">{TYPES[artist.type] ?? 'Artista'}</span>
      <h1>{artist.name}</h1>
      <p class="sub">
        {#if artist.country}<a href={exploreHref({ pais: artist.country })}>{country(artist.country)}</a> · {/if}
        {#if span}{span} · {/if}
        {count(artist.albums.length, 'álbum', 'álbumes')}
      </p>
      <p class="sub links">
        <a href={exploreHref({ artista: artist.key }, 'pistas')}>Ver sus pistas</a>
        {#if artist.mbid}· <a href={artistMBURL(artist.mbid)} target="_blank" rel="noreferrer">MusicBrainz ↗</a>{/if}
      </p>
      <Acciones what={{ artist: artist.key }} />
    </div>
  </div>

  {#if artist.albums.length}
    <h2>Álbumes</h2>
    <div class="grid">
      {#each artist.albums as a (a.id)}
        <TarjetaAlbum album={a} />
      {/each}
    </div>
  {/if}

  {#if artist.appearances.length}
    <h2>En compilados y otros álbumes</h2>
    {#each artist.appearances as ap (ap.album.id)}
      <p class="appearance">
        <a href={albumHref(ap.album.id)}>{ap.album.title}</a>
        <span>{[ap.album.artist, ap.album.year || ''].filter(Boolean).join(' · ')}</span>
      </p>
      <ListaPistas
        tracks={ap.tracks}
        numbered={false}
        albumArtist={artist.name}
        onplay={(t) => play({ tracks: ap.tracks.map((x) => x.id) }, { startId: t.id })} />
    {/each}
  {/if}
{/if}

<style>
  .head {
    display: flex;
    gap: 32px;
    align-items: flex-end;
    margin: 8px 0 24px;
  }
  .pic {
    width: 200px;
    flex: none;
  }
  .info {
    min-width: 0;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0 0 2px;
  }
  .sub a:hover {
    color: var(--text);
    text-decoration: underline;
  }
  .appearance {
    display: flex;
    gap: 10px;
    align-items: baseline;
    margin: 20px 0 4px;
  }
  .appearance a {
    color: var(--strong);
    font-weight: 500;
  }
  .appearance span {
    font-size: 13px;
    color: var(--faint);
  }
  @media (max-width: 640px) {
    .head {
      flex-direction: column;
      align-items: center;
      text-align: center;
      gap: 16px;
    }
    .pic {
      width: min(60%, 240px);
    }
  }
</style>
```

Create `web/src/pages/Buscar.svelte`:

```svelte
<script>
  // Every result of a search, by kind (the header's search shows a few).
  import ListaPistas from '../components/ListaPistas.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import TarjetaArtista from '../components/TarjetaArtista.svelte'
  import { api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { play } from '../lib/player.svelte.js'
  import { folderHref } from '../lib/router.js'
  import { searchable, total } from '../lib/search.js'

  const LIMIT = 200

  let { q } = $props()

  let result = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    result = null
    if (!searchable(q)) return
    let stale = false
    api
      .find(q, LIMIT)
      .then((r) => {
        if (!stale) (result = r), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.message
      })
    return () => (stale = true)
  })

  // more tells how many of a kind are left out.
  const more = (kind, shown) => (result.totals[kind] ?? 0) - shown
</script>

<h1>Buscar</h1>
{#if !searchable(q)}
  <p class="empty">Escribí al menos dos letras en la búsqueda (Ctrl+K).</p>
{:else if error}
  <p class="empty">{error}</p>
{:else if result}
  <p class="sub">{count(total(result), 'resultado', 'resultados')} para «{q}»</p>
  {#if total(result) === 0}
    <p class="empty">Nada con «{q}».</p>
  {/if}
  {#if result.artists.length}
    <h2>Artistas</h2>
    <div class="grid">
      {#each result.artists as a (a.key)}
        <TarjetaArtista artist={a} />
      {/each}
    </div>
  {/if}
  {#if result.albums.length}
    <h2>Álbumes</h2>
    <div class="grid">
      {#each result.albums as a (a.id)}
        <TarjetaAlbum album={a} />
      {/each}
    </div>
    {#if more('albums', result.albums.length) > 0}<p class="sub">Y {more('albums', result.albums.length)} más.</p>{/if}
  {/if}
  {#if result.tracks.length}
    <h2>Pistas</h2>
    <ListaPistas
      tracks={result.tracks}
      numbered={false}
      showAlbum
      onplay={(t) => play({ tracks: result.tracks.map((x) => x.id) }, { startId: t.id })} />
    {#if more('tracks', result.tracks.length) > 0}<p class="sub">Y {more('tracks', result.tracks.length)} más.</p>{/if}
  {/if}
  {#if result.folders.length}
    <h2>Carpetas</h2>
    <ul class="folders">
      {#each result.folders as f (f.path)}
        <li>
          <a href={folderHref(f.path)}>{f.name}</a>
          <span>{f.collection ? 'Colección · ' : ''}{count(f.tracks, 'pista', 'pistas')}</span>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<style>
  .sub {
    font-size: 13px;
    color: var(--muted);
  }
  .folders {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .folders li {
    display: flex;
    gap: 12px;
    align-items: baseline;
    padding: 8px;
    border-bottom: 1px solid var(--line);
  }
  .folders a {
    color: var(--strong);
  }
  .folders span {
    font-size: 13px;
    color: var(--faint);
  }
</style>
```

- [ ] **Step 2: Run the checks**

```bash
(cd web && npx vitest run)
(cd web && npm run build)
go test ./...
```
Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/App.svelte web/src/components/Busqueda.svelte web/src/components/Nav.svelte web/src/pages/Artista.svelte web/src/pages/Buscar.svelte
git commit -m "feat: artist page and search (Ctrl+K)"
```

### Task 11: Album page with MusicBrainz data and Corregir identificación

The album page shows what M2 found: the original year (with the edition's when it differs), the artist as a link, the country and the genres/styles as links to Explorar, and how many tracks need ffmpeg. Under the tracks: where the data comes from (auto with its confidence, manual, not in MusicBrainz, selection, not looked up), a link to the release, the tag title when MusicBrainz's differs, and `⋯ Corregir identificación`: `Identificar` lists the candidates (pick one), takes a pasted musicbrainz.org release, marks the album as not in MusicBrainz, or goes back to the automatic identification. Hidden in browse-only mode.

**Files:**
- Create: `web/src/components/Identificar.svelte`
- Modify: `web/src/pages/Album.svelte`

**Interfaces:**
- Consumes: the album page JSON (Task 7), `api.setIdentification`, `api.resetIdentification`, `review.js`.
- Produces: component `Identificar` (props `album` with `id`, `status`, `releaseId`, `candidates`; `onchange(albumPage)`), reused by Revisar.

- [ ] **Step 1: Write the code**

Create `web/src/components/Identificar.svelte`:

```svelte
<script>
  // Correcting an album's identification: one of the candidates MusicBrainz
  // offered, a release pasted from musicbrainz.org, "not in MusicBrainz",
  // or back to the automatic identification. onchange gets the album as the
  // server answers it.
  import { api, releaseURL } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { candidateLine, percent } from '../lib/review.js'

  let { album, onchange } = $props()

  let pasted = $state('')
  let busy = $state(false)

  async function run(f) {
    if (busy) return
    busy = true
    try {
      onchange(await f())
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  const choose = (releaseId) => run(() => api.setIdentification(album.id, { releaseId }))
  const ignore = () => run(() => api.setIdentification(album.id, { ignore: true }))
  const reset = () => run(() => api.resetIdentification(album.id))

  function onsubmit(event) {
    event.preventDefault()
    if (pasted.trim()) choose(pasted.trim())
  }
</script>

<div class="identificar" class:busy>
  {#if album.candidates?.length}
    <ul class="candidates">
      {#each album.candidates as c (c.releaseId)}
        <li class:current={c.releaseId === album.releaseId}>
          <button onclick={() => choose(c.releaseId)} disabled={busy}>Elegir</button>
          <span class="score">{percent(c.score)}</span>
          <span class="text">
            <span class="title">{c.title}</span>
            <span class="meta">{candidateLine(c)}</span>
          </span>
          <a href={releaseURL(c.releaseId)} target="_blank" rel="noreferrer" title="Ver en MusicBrainz">↗</a>
        </li>
      {/each}
    </ul>
  {:else}
    <p class="none">MusicBrainz no ofreció candidatos.</p>
  {/if}
  <form {onsubmit}>
    <input bind:value={pasted} placeholder="Pegá un enlace de musicbrainz.org/release/…" aria-label="Lanzamiento de MusicBrainz" />
    <button type="submit" disabled={busy || !pasted.trim()}>Usar</button>
  </form>
  <div class="more">
    {#if album.status !== 'ignored'}
      <button onclick={ignore} disabled={busy}>No está en MusicBrainz</button>
    {/if}
    {#if album.status === 'manual' || album.status === 'ignored'}
      <button onclick={reset} disabled={busy}>Volver a la identificación automática</button>
    {/if}
  </div>
</div>

<style>
  .identificar.busy {
    opacity: 0.6;
  }
  .candidates {
    list-style: none;
    margin: 0 0 10px;
    padding: 0;
  }
  .candidates li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
  }
  .candidates li.current .title {
    color: var(--accent);
  }
  .score {
    width: 44px;
    white-space: nowrap;
    text-align: right;
    font-variant-numeric: tabular-nums;
    color: var(--muted);
    font-size: 13px;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .title,
  .meta {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
  }
  .meta {
    font-size: 12px;
    color: var(--faint);
  }
  .none {
    color: var(--muted);
    margin: 0 0 10px;
  }
  form {
    display: flex;
    gap: 8px;
  }
  form input {
    flex: 1;
    min-width: 0;
  }
  .more {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 10px;
  }
</style>
```

Modify `web/src/pages/Album.svelte` (unified diff against the previous task):

```diff
@@ -1,17 +1,20 @@
 <script>
   import Acciones from '../components/Acciones.svelte'
+  import Identificar from '../components/Identificar.svelte'
   import ListaPistas from '../components/ListaPistas.svelte'
   import Tapa from '../components/Tapa.svelte'
-  import { albumCoverURL, api } from '../lib/api.js'
+  import { albumCoverURL, api, releaseURL } from '../lib/api.js'
   import { app } from '../lib/app.svelte.js'
-  import { count, folderName, length, quality } from '../lib/format.js'
+  import { count, country, folderName, length, quality } from '../lib/format.js'
   import { play } from '../lib/player.svelte.js'
-  import { folderHref } from '../lib/router.js'
+  import { genreLinks, identification } from '../lib/review.js'
+  import { artistHref, exploreHref, folderHref } from '../lib/router.js'
 
   let { id } = $props()
 
   let album = $state(null)
   let error = $state('')
+  let correcting = $state(false)
 
   $effect(() => {
     app.generation
@@ -27,32 +30,71 @@
     return () => (stale = true)
   })
 
+  // year is the original year, with the edition's when it differs.
+  const year = $derived.by(() => {
+    if (!album) return ''
+    const y = album.originalYear || album.year
+    return album.originalYear && album.year && album.year !== album.originalYear ? `${y} (edición ${album.year})` : y || ''
+  })
+
   const summary = $derived(
-    album
-      ? [album.year || '', count(album.tracks.length, 'pista', 'pistas'), length(album.durationMs)].filter(Boolean).join(' · ')
-      : '',
+    album ? [year, count(album.tracks.length, 'pista', 'pistas'), length(album.durationMs)].filter(Boolean).join(' · ') : '',
   )
 
   // formats sums up the album's formats: "FLAC 16/44,1", or several.
   const formats = $derived(album ? [...new Set(album.tracks.map(quality))].join(' · ') : '')
+  const unplayable = $derived(album ? album.tracks.filter((t) => !t.playable).length : 0)
+  const identified = $derived(album && (album.status === 'auto' || album.status === 'manual') && album.releaseId)
+
+  function corrected(a) {
+    album = a
+    correcting = false
+  }
 </script>
 
 {#if error}
   <p class="empty">{error}</p>
 {:else if album}
   <div class="head">
-    <div class="pic"><Tapa srcs={[albumCoverURL(album.id, 640)]} alt={album.title} /></div>
+    <div class="pic"><Tapa srcs={album.picture ? [albumCoverURL(album.id, 640)] : []} alt={album.title} /></div>
     <div class="info">
       <span class="label">{album.compilation ? 'Compilado' : 'Álbum'}</span>
       <h1>{album.title}</h1>
-      <p class="artist">{album.artist}</p>
-      <p class="sub">{summary}</p>
-      <p class="sub">{formats}</p>
+      <p class="artist">
+        {#if album.artistKey}<a href={artistHref(album.artistKey)}>{album.artist}</a>{:else}{album.artist}{/if}
+      </p>
+      <p class="sub">
+        {summary}{#if album.country}{' · '}<a href={exploreHref({ pais: album.country })}>{country(album.country)}</a>{/if}
+      </p>
+      {#if album.genres.length}
+        <p class="genres">
+          {#each genreLinks(album.genres) as g (g.label)}
+            <a class="chip" href={exploreHref(g.facets)}>{g.label}</a>
+          {/each}
+        </p>
+      {/if}
+      <p class="sub">{formats}{unplayable ? ` · ${count(unplayable, 'pista necesita', 'pistas necesitan')} ffmpeg` : ''}</p>
       <a class="path" href={folderHref(album.dir)}>{folderName(album.dir)}</a>
       <Acciones what={{ album: album.id }} />
     </div>
   </div>
   <ListaPistas tracks={album.tracks} albumArtist={album.artist} onplay={(t) => play({ album: album.id }, { startId: t.id })} />
+
+  <section class="ident">
+    <p class="sub">
+      {identification(album)}
+      {#if identified}· <a href={releaseURL(album.releaseId)} target="_blank" rel="noreferrer">ver en MusicBrainz ↗</a>{/if}
+      {#if album.title !== album.tagTitle}· en los tags: «{album.tagTitle}»{/if}
+    </p>
+    {#if !app.status?.readOnly}
+      {#if correcting}
+        <Identificar {album} onchange={corrected} />
+        <button onclick={() => (correcting = false)}>Cerrar</button>
+      {:else}
+        <button onclick={() => (correcting = true)}>⋯ Corregir identificación</button>
+      {/if}
+    {/if}
+  </section>
 {/if}
 
 <style>
@@ -74,14 +116,47 @@
     color: var(--text);
     margin: 0 0 6px;
   }
+  .artist a:hover,
+  .sub a:hover {
+    text-decoration: underline;
+  }
   .sub {
     font-size: 13px;
     color: var(--muted);
     margin: 0 0 2px;
   }
+  .genres {
+    display: flex;
+    flex-wrap: wrap;
+    gap: 6px;
+    margin: 6px 0;
+  }
+  .chip {
+    font-size: 12px;
+    padding: 1px 9px;
+    border: 1px solid var(--line-strong);
+    border-radius: 12px;
+    color: var(--muted);
+  }
+  .chip:hover {
+    border-color: var(--accent);
+    color: var(--accent);
+  }
   .path:hover {
     color: var(--text);
   }
+  .ident {
+    margin-top: 32px;
+    padding-top: 16px;
+    border-top: 1px solid var(--line);
+    display: flex;
+    flex-direction: column;
+    align-items: flex-start;
+    gap: 10px;
+  }
+  .ident :global(.identificar) {
+    width: min(100%, 720px);
+  }
   @media (max-width: 640px) {
     .head {
       flex-direction: column;
```

- [ ] **Step 2: Run the checks**

```bash
(cd web && npx vitest run)
(cd web && npm run build)
go test ./...
```
Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Identificar.svelte web/src/pages/Album.svelte
git commit -m "feat: album page with MusicBrainz data and Corregir identificación"
```

### Task 12: Revisar and "no es una colección"

Revisar gets two tabs (Duplicados and Formatos a convertir are M5). **Sin identificar**: each album of the queue with its folder, its best candidate and score, play, "Buscar en MusicBrainz" (a search on musicbrainz.org to copy the right release) and Corregir (`Identificar`); a corrected album leaves the list. The M2 check found 26–34 albums whose first candidate is right at 0.65–0.79: "Ver solo esos" lists the ones at 70 % or more and "Aceptar el primer candidato de cada uno" takes them one at a time, after a confirmation. **Géneros por mapear**: the raw values most used first, mapped with one picker at a time (a `<select>` of the ~200 genres and styles in each of ~80 rows would put 16,000 options in the page), and "Tus mapeos" to forget one. In Carpetas, a detected collection has "No es una colección" (and "Es una colección" to take it back). The nav gets Revisar.

**Files:**
- Modify: `web/src/App.svelte`
- Create: `web/src/components/GenerosPorMapear.svelte`
- Modify: `web/src/components/Nav.svelte`
- Create: `web/src/components/SinIdentificar.svelte`
- Modify: `web/src/pages/Carpetas.svelte`
- Create: `web/src/pages/Revisar.svelte`

**Interfaces:**
- Consumes: `api.unidentified`, `api.setIdentification`, genre calls, `api.setCollection`, `Identificar`, `review.js`, `FolderSummary.detected`.
- Produces: page `Revisar` (prop `tab`); components `SinIdentificar`, `GenerosPorMapear` (prop `onload(count)`).

- [ ] **Step 1: Write the code**

Modify `web/src/App.svelte` (unified diff against the previous task):

```diff
@@ -10,6 +10,7 @@
   import Carpetas from './pages/Carpetas.svelte'
   import Explorar from './pages/Explorar.svelte'
   import Listas from './pages/Listas.svelte'
+  import Revisar from './pages/Revisar.svelte'
 
   const current = $derived(resolve(route.path, route.search))
 
@@ -49,6 +50,8 @@
     <Buscar q={current.q} />
   {:else if current.page === 'listas'}
     <Listas />
+  {:else if current.page === 'revisar'}
+    <Revisar tab={current.tab} />
   {:else}
     <p class="empty">No existe esta página. <a href="/explorar">Ir a Explorar</a></p>
   {/if}
```

Create `web/src/components/GenerosPorMapear.svelte`:

```svelte
<script>
  // Géneros por mapear: raw genre values from the tags that no mapping
  // knows, most used first, to map onto the taxonomy (or ignore); and the
  // user's mappings, to undo one.
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { aliasLabel, genreChoice, genreOptions } from '../lib/review.js'

  let { onload } = $props()

  let unmapped = $state(null)
  let aliases = $state([])
  let options = $state([])
  let editing = $state(null) // the raw value being mapped: one picker at a time
  let chosen = $state('') // its option value
  let error = $state('')

  async function load() {
    try {
      const [u, a, t] = await Promise.all([api.unmappedGenres(), api.genreAliases(), api.genres()])
      unmapped = u
      aliases = a
      options = genreOptions(t)
      error = ''
      onload?.(u.length)
    } catch (e) {
      error = e.message
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  function edit(raw) {
    editing = raw
    chosen = ''
  }

  async function save(raw) {
    try {
      await api.setGenreAlias({ raw, ...genreChoice(chosen) })
      editing = null
      await load()
    } catch (e) {
      notify(e.message)
    }
  }

  async function forget(raw) {
    try {
      await api.deleteGenreAlias(raw)
      await load()
    } catch (e) {
      notify(e.message)
    }
  }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if unmapped}
  {#if unmapped.length === 0}
    <p class="empty">Todos los géneros de los tags están mapeados.</p>
  {:else}
    <ul class="rows">
      {#each unmapped as u (u.raw)}
        <li>
          <span class="raw">{u.raw}</span>
          <span class="n">{count(u.albums, 'álbum', 'álbumes')}</span>
          {#if app.status?.readOnly}
            <!-- browse-only: nothing to map -->
          {:else if editing === u.raw}
            <select bind:value={chosen} aria-label="Género para {u.raw}">
              <option value="">Elegí un género…</option>
              {#each options as o (o.value)}
                <option value={o.value}>{o.label}</option>
              {/each}
            </select>
            <button class="primary" onclick={() => save(u.raw)} disabled={!chosen}>Guardar</button>
            <button onclick={() => (editing = null)}>Cancelar</button>
          {:else}
            <button onclick={() => edit(u.raw)}>Mapear…</button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  {#if aliases.length}
    <h2>Tus mapeos</h2>
    <ul class="rows">
      {#each aliases as a (a.raw)}
        <li>
          <span class="raw">{a.raw}</span>
          <span class="n">→ {aliasLabel(a)}</span>
          {#if !app.status?.readOnly}
            <button class="icon" onclick={() => forget(a.raw)} title="Olvidar este mapeo" aria-label="Olvidar el mapeo de {a.raw}">✕</button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<style>
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .rows li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .raw {
    color: var(--strong);
    min-width: 200px;
  }
  .n {
    color: var(--muted);
    font-size: 13px;
    flex: 1;
  }
  select {
    max-width: 320px;
  }
</style>
```

Modify `web/src/components/Nav.svelte` (unified diff against the previous task):

```diff
@@ -10,6 +10,7 @@
     { page: 'explorar', href: '/explorar', label: 'Explorar', icon: 'explore' },
     { page: 'carpetas', href: '/carpetas', label: 'Carpetas', icon: 'folder' },
     { page: 'listas', href: '/listas', label: 'Listas', icon: 'lists' },
+    { page: 'revisar', href: '/revisar', label: 'Revisar', icon: 'review' },
   ]
 </script>
```

Create `web/src/components/SinIdentificar.svelte`:

```svelte
<script>
  // The Sin identificar queue: albums MusicBrainz matched below the
  // threshold, with their candidates. An album leaves the queue once
  // corrected.
  import { albumCoverURL, api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { count, folderName } from '../lib/format.js'
  import { play } from '../lib/player.svelte.js'
  import { candidateLine, percent, searchURL, worthAccepting } from '../lib/review.js'
  import { albumHref, folderHref } from '../lib/router.js'
  import Icono from './Icono.svelte'
  import Identificar from './Identificar.svelte'
  import Tapa from './Tapa.svelte'

  let { onload } = $props()

  let albums = $state(null)
  let error = $state('')
  let open = $state(null) // the album being corrected
  let accepting = $state(null) // {done, of} while accepting the best candidates
  let onlyLikely = $state(false) // show only the albums the bulk accept would take

  $effect(() => {
    app.generation
    api
      .unidentified()
      .then((as) => {
        albums = as
        error = ''
        onload?.(as.length)
      })
      .catch((e) => (error = e.message))
  })

  function corrected(a) {
    if (a.status !== 'unmatched') {
      albums = albums.filter((x) => x.id !== a.id)
      onload?.(albums.length)
    }
    open = null
  }

  const likely = $derived(albums ? worthAccepting(albums) : [])

  const shown = $derived(onlyLikely ? likely : (albums ?? []))

  // acceptAll takes the best candidate of every likely album, one at a time.
  async function acceptAll() {
    const list = likely
    if (!confirm(`¿Aceptar el primer candidato de ${count(list.length, 'álbum', 'álbumes')}? Revisalos antes con «Ver solo esos».`)) return
    accepting = { done: 0, of: list.length }
    for (const a of list) {
      try {
        corrected(await api.setIdentification(a.id, { releaseId: a.candidates[0].releaseId }))
      } catch (e) {
        notify(`${a.title}: ${e.message}`)
      }
      accepting.done++
    }
    accepting = null
  }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if albums?.length === 0}
  <p class="empty">No hay álbumes sin identificar.</p>
{:else if albums}
  {#if likely.length && !app.status?.readOnly}
    <p class="bulk">
      {#if accepting}
        Aceptando {accepting.done} de {accepting.of}…
      {:else}
        {count(likely.length, 'álbum tiene', 'álbumes tienen')} un primer candidato de 70 % o más.
        <label><input type="checkbox" bind:checked={onlyLikely} /> Ver solo esos</label>
        <button onclick={acceptAll}>Aceptar el primer candidato de cada uno</button>
      {/if}
    </p>
  {/if}
  <ul class="queue">
    {#each shown as a (a.id)}
      <li>
        <div class="row">
          <a class="thumb" href={albumHref(a.id)}><Tapa srcs={a.picture ? [albumCoverURL(a.id, 160)] : []} alt={a.title} /></a>
          <div class="text">
            <a class="title" href={albumHref(a.id)}>{a.tagTitle}</a>
            <span class="meta">{[a.tagArtist, a.year || '', count(a.tracks, 'pista', 'pistas')].filter(Boolean).join(' · ')}</span>
            <a class="meta folder" href={folderHref(a.dir)}>{folderName(a.dir)}</a>
            {#if a.candidates.length}
              <span class="meta best">Mejor candidato: {a.candidates[0].title} — {candidateLine(a.candidates[0])} ({percent(a.candidates[0].score)})</span>
            {:else}
              <span class="meta best">Sin candidatos</span>
            {/if}
          </div>
          <div class="buttons">
            <button class="icon" onclick={() => play({ album: a.id })} title="Escuchar" aria-label="Escuchar {a.tagTitle}">
              <Icono name="play" size={16} />
            </button>
            <a class="search" href={searchURL(a)} target="_blank" rel="noreferrer">Buscar en MusicBrainz ↗</a>
            {#if !app.status?.readOnly}
              <button onclick={() => (open = open === a.id ? null : a.id)}>{open === a.id ? 'Cerrar' : 'Corregir'}</button>
            {/if}
          </div>
        </div>
        {#if open === a.id}
          <div class="panel"><Identificar album={a} onchange={corrected} /></div>
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .bulk {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
    color: var(--muted);
  }
  .queue {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .queue li {
    border-bottom: 1px solid var(--line);
    padding: 10px 0;
  }
  .row {
    display: flex;
    gap: 14px;
    align-items: center;
  }
  .thumb {
    width: 64px;
    flex: none;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .meta {
    font-size: 13px;
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .folder,
  .best {
    color: var(--faint);
    font-size: 12px;
  }
  .buttons {
    display: flex;
    align-items: center;
    gap: 10px;
    flex: none;
  }
  .search {
    font-size: 13px;
    color: var(--muted);
  }
  .panel {
    margin: 10px 0 4px 78px;
    max-width: 720px;
  }
  @media (max-width: 640px) {
    .row {
      flex-wrap: wrap;
    }
    .buttons {
      width: 100%;
      justify-content: flex-end;
    }
    .panel {
      margin-left: 0;
    }
  }
</style>
```

Modify `web/src/pages/Carpetas.svelte` (unified diff against the previous task):

```diff
@@ -7,7 +7,7 @@
   import Tapa from '../components/Tapa.svelte'
   import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
   import { albumCoverURL, api } from '../lib/api.js'
-  import { app } from '../lib/app.svelte.js'
+  import { app, notify } from '../lib/app.svelte.js'
   import { count, length } from '../lib/format.js'
   import { navigate, savedScroll } from '../lib/nav.svelte.js'
   import { play } from '../lib/player.svelte.js'
@@ -50,6 +50,16 @@
     return c.collection ? `Colección · ${n}` : n
   }
 
+  // setCollection says a detected collection is not one (or takes it back).
+  async function setCollection(collection) {
+    try {
+      const f = await api.setCollection(folder.path, collection)
+      folder.collection = f.collection
+    } catch (e) {
+      notify(e.message)
+    }
+  }
+
   // A subfolder holding a single album (its own folder) opens the album.
   function childHref(child) {
     const art = folder.art?.[child.path] ?? []
@@ -75,6 +85,11 @@
     {#if folder.trackCount > 0}
       <Acciones what={{ folder: path }} />
     {/if}
+    {#if folder.detected && !app.status?.readOnly}
+      <button class="small" onclick={() => setCollection(!folder.collection)}>
+        {folder.collection ? 'No es una colección' : 'Es una colección'}
+      </button>
+    {/if}
   {:else}
     <h1>Carpetas</h1>
     {#if folder.children.length === 0}
@@ -123,6 +138,11 @@
 {/if}
 
 <style>
+  .small {
+    font-size: 13px;
+    color: var(--muted);
+    margin-bottom: 8px;
+  }
   .trail {
     display: flex;
     flex-wrap: wrap;
```

Create `web/src/pages/Revisar.svelte`:

```svelte
<script>
  // Revisar: what needs the user's judgment. Duplicados and Formatos a
  // convertir come with M5.
  import GenerosPorMapear from '../components/GenerosPorMapear.svelte'
  import SinIdentificar from '../components/SinIdentificar.svelte'

  let { tab } = $props()

  let counts = $state({})
</script>

<h1>Revisar</h1>
<nav class="tabs">
  <a href="/revisar" class:on={tab === 'sin-identificar'}>Sin identificar{counts.unidentified != null ? ` (${counts.unidentified})` : ''}</a>
  <a href="/revisar/generos" class:on={tab === 'generos'}>Géneros por mapear{counts.genres != null ? ` (${counts.genres})` : ''}</a>
</nav>

{#if tab === 'generos'}
  <GenerosPorMapear onload={(n) => (counts.genres = n)} />
{:else}
  <SinIdentificar onload={(n) => (counts.unidentified = n)} />
{/if}

<style>
  .tabs {
    display: flex;
    gap: 20px;
    margin: 12px 0 16px;
    border-bottom: 1px solid var(--line);
  }
  .tabs a {
    padding: 6px 0;
    border-bottom: 2px solid transparent;
    color: var(--muted);
  }
  .tabs a.on {
    color: var(--strong);
    border-bottom-color: var(--accent);
  }
</style>
```

- [ ] **Step 2: Run the checks**

```bash
(cd web && npx vitest run)
(cd web && npm run build)
go test ./...
```
Expected: all pass.

- [ ] **Step 3: Commit**

```bash
git add web/src/App.svelte web/src/components/GenerosPorMapear.svelte web/src/components/Nav.svelte web/src/components/SinIdentificar.svelte web/src/pages/Carpetas.svelte web/src/pages/Revisar.svelte
git commit -m "feat: Revisar (Sin identificar, Géneros por mapear) and no es una colección"
```

### Task 13: Documentation

README: browsing and listening (Explorar, artists, search, "no es una colección"), covers from the Cover Art Archive, the new endpoints, and Revisar in place of the bare API for corrections.

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the documentation**

Modify `README.md` (unified diff against the previous task):

```diff
@@ -35,16 +35,23 @@ Soulseek's `downloading`, Apple Music libraries) are always skipped.
 `ffprobe`, when on the `PATH`, reads the formats the built-in readers don't
 (APE, WavPack, raw AAC…).
 
-### Listening
+### Browsing and listening
 
-Browse the folders as they are on disk (collections show their albums in
-order), open an album and play it, shuffle a folder, queue tracks. The queue
+Explorar lists albums, artists or tracks of the whole catalog, filtered by
+decade and year, artist, country, genre and style, collection, folder,
+quality, format and state (not identified, not playable); the filters live in
+the URL. Artist pages show their albums by year and their tracks on
+compilations. The search (Ctrl+K / ⌘K) finds artists, albums, tracks and
+folders while typing, ignoring case and accents. Carpetas shows the folders
+as they are on disk (collections show their albums in order; a folder wrongly
+detected as a collection can be told "no es una colección"). Open an album
+and play it, shuffle a folder, queue tracks. The queue
 and the volume are kept in the browser, per device. MP3, FLAC, AAC, Opus,
 Vorbis and WAV play as they are, with seeking; WMA, ALAC, AIFF, APE and
 WavPack need `ffmpeg` on the `PATH` and are converted on the fly (lossless to
 FLAC, lossy to MP3), without seeking. Covers come from the album folder's
-image or the files' embedded pictures; thumbnails are cached in
-`cache/thumbs/`.
+image, the files' embedded pictures or the Cover Art Archive; thumbnails are
+cached in `cache/thumbs/`.
 
 | Method and path | What it returns |
 |---|---|
@@ -54,10 +61,16 @@ image or the files' embedded pictures; thumbnails are cached in
 | `GET /api/albums/{id}` | an album with its tracks and technical data |
 | `GET /api/folders?path=` | a folder: subfolders, albums and loose tracks (no `path`: the roots) |
 | `GET /api/collections` | folders detected as collections, with their albums in order |
+| `PUT /api/folders/collection` | `{"path", "collection": false}`: not a collection (`true` takes it back) |
+| `GET /api/explore?ver=&<facets>&orden=&dir=&desde=&limite=` | a page of Explorar (albums, artists or tracks) with the facets' counts |
+| `GET /api/artists/{key}` | an artist: albums by year and appearances on others' albums |
+| `GET /api/search?q=&limite=` | artists, albums, tracks and folders matching `q` |
 | `GET /api/genres` | the taxonomy of genres and styles |
+| `GET /api/genres/unmapped` | raw tag genres no mapping knows (Géneros por mapear) |
+| `PUT /api/genres/aliases`, `DELETE /api/genres/aliases?raw=` | map a raw genre (or ignore it), or forget the mapping |
 | `GET /api/review/unidentified` | albums awaiting identification review |
-| `PUT /api/albums/{id}/identification` | choose the identification for an album |
-| `GET /api/play?album=&folder=&track=&shuffle=1` | the tracks to queue, in order (at most 5000) |
+| `PUT /api/albums/{id}/identification` | choose the identification for an album (`DELETE` goes back to automatic) |
+| `GET /api/play?album=&folder=&artist=&track=&shuffle=1` | the tracks to queue, in order (at most 5000) |
 | `GET /stream/{trackId}` | a track's audio (`Range` for seeking; `?transcode=1` converts it) |
 | `GET /api/covers/album/{id}?size=` | an album's picture, whole or as a 160/320/640 px thumbnail |
 | `GET /api/covers/track/{id}?size=` | the picture embedded in a track's file |
@@ -70,8 +83,10 @@ hours and resumes where it stopped). Albums above the confidence threshold are
 identified automatically: they show MusicBrainz's titles, the original year,
 the artist's country, Discogs genres and styles, and a cover from the Cover Art
 Archive when the folder has none (saved in `cache/covers/`). The others go to
-`GET /api/review/unidentified` with their candidates; choose one with
-`PUT /api/albums/{id}/identification`. Corrections are stored per track
+Revisar → Sin identificar with their candidates: choose one, paste a
+musicbrainz.org release, or say the album is not in MusicBrainz (also from the
+album page, *Corregir identificación*). Raw tag genres no mapping knows go to
+Revisar → Géneros por mapear. Corrections are stored per track
 fingerprint: they survive moving, renaming and re-tagging the files, and the
 automatic identification never overrides them. Without network, everything
 else works and the identification retries on its own.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: Explorar, artists, search, Revisar and the new endpoints"
```


---

## Checks against the collection (manual, after Task 13)

Run with the user's collection, as in the M2 and M3a checks, and record the findings in `docs/superpowers/notes/2026-10-02-m3b-collection-check.md` (Spanish, like the earlier notes). Work on a **copy** of the catalog when trying corrections, or undo them after.

1. `(cd web && npm run build) && go run ./cmd/tordo -dir .run`: `/` opens Explorar with every album; time the first `GET /api/explore` (builds the catalog) and the next ones.
2. Facets: Década → a year, País, Género → Estilo (only that genre's styles), Colección, Más → Carpeta, Calidad, Formato, Estado (Sin identificar should match Revisar's count; No reproducible is empty with ffmpeg on the `PATH`). Back and forward keep the facets and the scroll.
3. Artistas tab: look for one artist split in two (name-only vs MusicBrainz). Note any left and why (MusicBrainz name vs tags).
4. Pistas tab: a click plays from that track; artist and album links work.
5. Search (Ctrl+K): an accent-less query finds accented and NFD names (`latinoamerica`, `paez`); Enter on a result; "Ver los N resultados".
6. Artist page of one with compilation appearances; ▶ plays albums by year, then the appearances.
7. Album page of an identified album (year, country, genres link to Explorar; candidates in Corregir) and of an unidentified one; a correction and back.
8. Revisar → Sin identificar: "Ver solo esos" (≥ 70 %): look through them before accepting; count how many are right. Géneros por mapear: map two or three values the M2 note lists (MPU, Vals, Cueca…).
9. "No es una colección" on a wrongly detected collection (the M3a check had *Brasil*: a country folder), and back.
10. Phone width (375 px): tabs, facet menus, search box, Revisar.

Things that need a decision go to the note's *Pendiente* section, for M4 or later.
