# Tordo — Carpetas as a file explorer: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Carpetas page's grid of cards with a file explorer: a folder tree on the left (chains of pass-through folders merged into one row, loaded level by level, filterable, driven by the keyboard) and a detail list on the right (subfolders with a relative size bar, then the files directly in the folder); on narrow screens, one iPod-style list per folder.

**Architecture:** The store gains two read-only pieces: the content of a folder grows sizes and its direct files (`internal/store/folders.go`), and a new pure in-memory tree (`internal/store/tree.go`) built from the `folders` table answers one tree level at a time and the folder-name filter. The server exposes them as `GET /api/folders/tree` and `GET /api/folders/tree/search`, and adds `playable` to the files of `GET /api/folders`. In the web app, all the logic (tree flattening, revealing a path, keys, sorting, sizes) lives in tested pure modules (`web/src/lib/tree.js`, `web/src/lib/folders.js`); the components only wire it.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`), Svelte 5 + Vite 8 + Vitest 5.

**Spec:** `docs/superpowers/specs/2026-10-02-tordo-carpetas-explorador-design.md` (Spanish, approved). It replaces the Carpetas page of `docs/superpowers/specs/2026-10-01-tordo-design.md` §8. Read both, plus `CLAUDE.md`.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new Go dependencies**; no cgo. npm: **no new packages**.
- **Read-only on the music collection**: nothing in this plan opens an audio file; the explorer is built from the catalog only. **No schema change** (no `schemaVersion` bump).
- Catalog paths are relative to the app folder, `/`-separated (e.g. `../musica/Soulseek/complete`). Folders are addressed by catalog path; audio by file id.
- SQLite `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open (read all rows, close, then query again).
- **Byte vs character lengths:** SQLite `length()`/`substr()` count characters, Go `len()` counts bytes. Never mix them; filter path prefixes in Go or with `>=`/`<` range bounds.
- Subtree range: every path strictly under folder `p` satisfies `path >= p + "/" AND path < p + "0"` (`'0'` is the byte after `'/'`). Use it for anything "under a folder" on `files.path` (it is `UNIQUE`, so indexed).
- User-facing strings (UI, errors) in **Spanish** (rioplatense); identifiers and comments in English. UI file names in Spanish (`ArbolCarpetas.svelte`).
- `go test ./...` passes with no network and no Node; `go vet ./...` and `gofmt -l .` clean; `cd web && npx vitest run` passes; `cd web && npm run build` succeeds. The build output (`internal/server/dist/`) is **not committed**.
- Svelte components have no tests (as in M3a/M3b); their logic lives in `web/src/lib/*.js` with Vitest tests.
- Commit messages: conventional style (`feat:`, `fix:`, `test:`, `docs:`), **no `Co-Authored-By:` trailer**.

## File Structure

```
internal/store/folders.go       NEW  folder stats (size, main codec) over a subtree; files directly in a folder
internal/store/folders_test.go  NEW
internal/store/catalog.go       MOD  FolderView: SizeBytes, Children []FolderChild, Files []FolderFile
internal/store/tree.go          NEW  folderTree (pure): levels with merged chains, name filter; Store.FolderTree, Store.SearchFolderTree
internal/store/tree_test.go     NEW
internal/server/folders.go      NEW  GET /api/folders/tree, GET /api/folders/tree/search; folder handler moves here, files get playable
internal/server/folders_test.go NEW
internal/server/server.go       MOD  routes; the folder handler moves out
web/src/lib/format.js           MOD  size(bytes)
web/src/lib/folders.js          NEW  detail list: entries, sorting, relative bar
web/src/lib/tree.js             NEW  tree: visible rows, reveal a path, keys, open branches in localStorage, filter rows, narrow helpers
web/src/lib/api.js              MOD  folderTree, folderTreeSearch
web/src/lib/carpetas.svelte.js  NEW  the tree state shared by both layouts (levels, open branches), loaders
web/src/App.svelte              MOD  Carpetas is no longer re-mounted on every path change
web/src/pages/Carpetas.svelte   REWRITE  wide: two panels; narrow: ListaCarpeta
web/src/components/ContenidoCarpeta.svelte  NEW  right panel: header + detail list
web/src/components/ArbolCarpetas.svelte     NEW  left panel: filter + tree (keyboard, ARIA tree)
web/src/components/ListaCarpeta.svelte      NEW  narrow screens: one list per folder, ⋯ menu
web/src/components/Icono.svelte             MOD  icons: chevron right/left, file, music
README.md                       MOD  endpoints, Carpetas
```

## Shared definitions (used by several tasks)

**Pass-through folder:** a folder with no tracks of its own (`tracks.dir = path` count is 0) and exactly one subfolder. A tree row starts at a folder and, while the current folder is pass-through, continues into its only subfolder. The row's `path` is the **deepest** folder of the chain; its `label` joins all the names with `" › "` (U+203A with a space on each side).

**Tree levels:** level `""` is the top: the roots (`folders.parent = ""`), or, when there is exactly one root, that root's subfolders. Level `p` (any folder path) is the subfolders of `p`. Rows inside a level are in natural order of their **first** folder's name (`natsort.Less`). A level key is always a folder path (or `""`); the tree's open branches and loaded levels are keyed by row paths (deepest folders).

**JSON of a tree row** (`store.TreeNode`):
```json
{"path": "../musica/Soulseek/complete", "label": "Soulseek › complete", "name": "complete",
 "tracks": 5, "sizeBytes": 51, "album": 0, "collection": false, "hasChildren": true}
```
`tracks`: recursive (the `folders.tracks` of the deepest folder). `album`: id of an album whose `dir` is the deepest folder (the smallest id if several), else 0. `collection`: the deepest folder is a collection (detected and not dismissed). `hasChildren`: the deepest folder has subfolders.

**Test fixture used by Tasks 1–3** (`explorer(t)` in `internal/store/tree_test.go`; Task 1 creates it in `folders_test.go`, Task 2 reuses it). Built with the existing helper `catalog(t, files, images...)` from `internal/store/catalog_test.go` (audio rows have size 10, images size 1, every track `mp3`/`mp3`, 320 kbps, 1000 ms):

```go
const sk = "../musica/Soulseek/complete"

// explorer is a library shaped like the real one's Soulseek downloads:
// pass-through chains, an album folder with a loose file next to albums,
// and an accented (NFD) folder name.
func explorer(t *testing.T) *Store {
	return catalog(t, map[string]audiofile.Tags{
		sk + "/Low_ID/Lhasa de Sela/01.mp3":         {Title: "El desierto", Artist: "Lhasa de Sela", Album: "La Llorona", Track: 1},
		sk + "/Low_ID/Lhasa de Sela/02.mp3":         {Title: "De cara a la pared", Artist: "Lhasa de Sela", Album: "La Llorona", Track: 2},
		sk + "/Low_ID/music of the crusades/01.mp3": {Title: "Chanterai por mon corage", Artist: "Early Music Consort", Album: "Music of the Crusades", Track: 1},
		sk + "/Low_ID/suelto.mp3":                   {Title: "Suelto"},
		sk + "/benteveo/Zitarrosa/Canta/01.mp3":     {Title: "Guitarra negra", Artist: "Alfredo Zitarrosa", Album: "Canta", Track: 1},
		sk + "/benteveo/Zitarrosa/Canta/02.mp3":     {Title: "Doña Soledad", Artist: "Alfredo Zitarrosa", Album: "Canta", Track: 2},
		"../musica/Uruguay/Jaime Roos/01.mp3":       {Title: "Brindis por Pierrot", Artist: "Jaime Roos", Album: "Brindis por Pierrot", Track: 1},
		"../musica/Brasil/Mu\u0301sica Popular/01.mp3": {Title: "Aquarela", Artist: "Toquinho", Album: "Aquarela", Track: 1},
	}, sk+"/Low_ID/Lhasa de Sela/folder.jpg", sk+"/Low_ID/scan.jpg")
}
```

Its folders: `../musica` (root, no own tracks, 3 subfolders) → `Brasil`, `Soulseek`, `Uruguay`. `Soulseek` (0 own, 1 sub) → `complete` (0 own, 2 subs: `Low_ID`, `benteveo`). `Low_ID` (1 own: `suelto.mp3`; subs `Lhasa de Sela`, `music of the crusades`). `benteveo` (0 own, 1 sub) → `Zitarrosa` (0 own, 1 sub) → `Canta` (album folder, 2 tracks). `catalog()` gives every extra path `Kind: "image"`, size 1. Sizes: `Lhasa de Sela` = 2 × 10 + 1 = 21; `Low_ID` = 21 + 10 (crusades) + 10 (`suelto.mp3`) + 1 (`scan.jpg`) = 42; `complete` = 42 + 20 (`Canta`) = 62; the root `../musica` = 62 + 10 + 10 = 82.

---

### Task 1: Store — folder sizes, main codec and direct files

**Model:** Sonnet.

**Files:**
- Create: `internal/store/folders.go`
- Create: `internal/store/folders_test.go`
- Modify: `internal/store/catalog.go` (`FolderView`, `Folder`)

**Interfaces:**
- Consumes: `Store.Folder(p)`, `FolderSummary`, `folderSummaries`, `natsort.Less`, the `catalog()` test helper.
- Produces:
  ```go
  // FolderChild is a subfolder as the folder page lists it.
  type FolderChild struct {
      FolderSummary
      SizeBytes int64  `json:"sizeBytes"` // every present catalogued file under it
      Album     int64  `json:"album"`     // an album whose folder it is (smallest id), else 0
      Codec     string `json:"codec"`     // the codec most of its audio files use ("" without audio)
  }
  // FolderFile is a present catalogued file directly in a folder.
  type FolderFile struct {
      ID         int64  `json:"id"`   // the file id; for a track, also the track id
      Name       string `json:"name"` // file name
      Kind       string `json:"kind"` // audio | image | lyrics | playlist
      SizeBytes  int64  `json:"sizeBytes"`
      Track      bool   `json:"track"` // an audio file the catalog has as a track
      DurationMs int64  `json:"durationMs"`
      Format     string `json:"format"`
      Codec      string `json:"codec"`
      Lossless   bool   `json:"lossless"`
      Bitrate    int    `json:"bitrate"`
      SampleRate int    `json:"sampleRate"`
      BitDepth   int    `json:"bitDepth"`
  }
  // FolderView gains:
  //   SizeBytes int64         `json:"sizeBytes"`
  //   Children  []FolderChild `json:"children"`  (was []FolderSummary)
  //   Files     []FolderFile  `json:"files"`     (never nil; empty for p == "")
  func (s *Store) folderStats(p string) (size int64, codec string, err error)   // unexported
  func (s *Store) folderFiles(p string) ([]FolderFile, error)                    // unexported
  ```

**Design:**
- `folderStats(p)`: one query over the subtree range, grouped by codec:
  ```sql
  SELECT COALESCE(a.codec, ''), COUNT(a.file_id), SUM(f.size)
  FROM files f LEFT JOIN audio a ON a.file_id = f.id AND f.kind = 'audio'
  WHERE f.path >= ?1 AND f.path < ?2 AND f.missing = 0
  GROUP BY 1
  ```
  with `?1 = p + "/"`, `?2 = p + "0"`. Size = sum of all groups; codec = the non-empty codec with the highest count (ties: the alphabetically first, for stable output). Read all rows, close, then return.
- `folderFiles(p)`: select `f.id, f.path, f.kind, f.size, t.file_id IS NOT NULL, COALESCE(a.duration_ms,0), COALESCE(a.format,''), COALESCE(a.codec,''), COALESCE(a.lossless,0), COALESCE(a.bitrate,0), COALESCE(a.sample_rate,0), COALESCE(a.bit_depth,0)` from `files f LEFT JOIN audio a ON a.file_id = f.id LEFT JOIN tracks t ON t.file_id = f.id` over the same range with `f.missing = 0`; in Go keep only rows whose path, after the `p + "/"` prefix, has no `/` (direct children). `Name = path.Base`. Sort with `natsort.Less` on `Name`. Never `nil`.
- `Folder(p)`: for `p != ""` set `v.SizeBytes, _ = folderStats(p)` (ignore the codec) and `v.Files = folderFiles(p)`. For every child (including the roots when `p == ""`): `FolderChild{FolderSummary: c, SizeBytes, Codec}` from `folderStats(c.Path)`, and `Album` from `SELECT MIN(id) FROM albums WHERE dir = ?` (0 when NULL). Keep the existing natural sort of children. Do all child queries **after** the children rows are closed.
- Existing callers keep compiling: `FolderChild` embeds `FolderSummary`, so `r.Children[0].Collection` still works. `setCollection` in the server writes `f.FolderSummary` — unchanged.

- [ ] **Step 1: Write the failing tests** in `internal/store/folders_test.go` (add the `explorer` fixture and `sk` constant from "Shared definitions" here):
  - `TestFolderSizes`: `Folder(sk + "/Low_ID")` → `SizeBytes == 42`; `Folder(sk)` → `SizeBytes == 62`; the child `Lhasa de Sela` of `Low_ID` has `SizeBytes == 21`, `Codec == "mp3"`, `Album != 0`; the child `benteveo` of `complete` has `Album == 0`, `SizeBytes == 20`. `Folder("")` → its one child (the root `../musica`) has `SizeBytes == 62 + 10 + 10 == 82`.
  - `TestFolderSizesSkipMissing`: rescan without one file — `s.SyncFiles(rowsWithout(sk+"/Low_ID/suelto.mp3"), root)` where the rows are rebuilt the same way `catalog()` builds them (`audioRow(p, 10, 1)` for audio, `FileRow{Kind: "image", Size: 1}` for the extras) — then `Folder(sk + "/Low_ID").SizeBytes == 32` and `suelto.mp3` is not in its `Files`.
  - `TestFolderFiles`: `Folder(sk + "/Low_ID").Files` lists exactly `scan.jpg`, `suelto.mp3`, in that (natural) order — not the files of its subfolders; `suelto.mp3` has `Kind "audio"`, `Track true`, `DurationMs 1000`, `Codec "mp3"`, `Bitrate 320`, `SizeBytes 10`; `scan.jpg` has `Kind "image"`, `Track false`, `SizeBytes 1`. `Folder(sk + "/Low_ID/Lhasa de Sela").Files` has 3 entries (`01.mp3`, `02.mp3`, `folder.jpg`). `Folder("").Files` is non-nil and empty.
  - `TestFolderFilesPrefixSibling`: a folder named like a prefix of a sibling must not leak: build `catalog(t, {"../musica/A/01.mp3", "../musica/A B/01.mp3", "../musica/A0/01.mp3"})` (any tags with distinct albums) and check `Folder("../musica/A")` has `SizeBytes == 10` and one file.
- [ ] **Step 2: Run** `go test ./internal/store -run 'TestFolder' -v` — expect compile errors / failures.
- [ ] **Step 3: Implement** `folders.go` and the `catalog.go` changes as designed.
- [ ] **Step 4: Run** `go test ./internal/store` (all existing store tests must still pass, notably `TestFolderView`, `TestNotACollection`), `go vet ./...`, `gofmt -l .`, then `go test ./...` (the server package uses `FolderView`).
- [ ] **Step 5: Commit** `feat: folder sizes, main codec and direct files in the folder view`

---

### Task 2: Store — the folder tree (levels with merged chains, name filter)

**Model:** Sonnet.

**Files:**
- Create: `internal/store/tree.go`
- Create: `internal/store/tree_test.go`

**Interfaces:**
- Consumes: `folders`, `tracks`, `albums` tables; `isCollection` SQL constant (`catalog.go`); `folderStats` (Task 1); `nameparse.Norm`; `natsort.Less`; the `explorer` fixture (Task 1, `folders_test.go`).
- Produces:
  ```go
  type TreeNode struct {
      Path        string `json:"path"`
      Label       string `json:"label"`
      Name        string `json:"name"`
      Tracks      int    `json:"tracks"`
      SizeBytes   int64  `json:"sizeBytes"`
      Album       int64  `json:"album"`
      Collection  bool   `json:"collection"`
      HasChildren bool   `json:"hasChildren"`
  }
  // TreeSearch is what filtering the tree by folder name shows: the rows on
  // the way to each match, by level, with every branch open.
  type TreeSearch struct {
      Levels  map[string][]TreeNode `json:"levels"`  // level key → rows (only those leading to a match), natural order
      Matches []string              `json:"matches"` // paths of the rows that match, natural order of path
      More    bool                  `json:"more"`    // more folders matched than the limit
  }
  const TreeSearchLimit = 200
  func (s *Store) FolderTree(p string) ([]TreeNode, bool, error)   // false: p is not a folder of the catalog
  func (s *Store) SearchFolderTree(q string) (TreeSearch, error)
  ```
  Internal (pure, tested directly):
  ```go
  type treeFolder struct {
      Path, Parent, Name string
      Tracks, Own        int   // recursive tracks; tracks directly in it
      Album              int64 // smallest album id whose dir it is, else 0
      Collection         bool
  }
  func newFolderTree(fs []treeFolder) *folderTree
  func (t *folderTree) level(key string) ([]TreeNode, bool) // SizeBytes left 0
  func (t *folderTree) search(q string, limit int) TreeSearch // SizeBytes left 0
  ```

**Design:**
- `loadFolderTree()`: three queries, each fully read and closed before the next: `SELECT path, parent, name, tracks, ` + `isCollection` + ` FROM folders`; `SELECT dir, COUNT(*) FROM tracks GROUP BY dir`; `SELECT dir, MIN(id) FROM albums GROUP BY dir`. Build `[]treeFolder`, then `newFolderTree`. No caching (measured in Task 9; if a level takes > 50 ms on the real collection, cache by `Store.Changes()` then).
- `folderTree` keeps `byPath map[string]*treeFolder`, `children map[string][]string` (sorted with `natsort.Less` on the child's name), `roots []string` (sorted the same way).
- `node(start)`: `f := start; names := [f.Name]; for f.Own == 0 && len(children[f.Path]) == 1 { f = child; names = append(names, f.Name) }`; `TreeNode{Path: f.Path, Label: strings.Join(names, " › "), Name: f.Name, Tracks: f.Tracks, Album: f.Album, Collection: f.Collection, HasChildren: len(children[f.Path]) > 0}`.
- `level("")`: if `len(roots) == 1` the nodes of the root's children, else the nodes of the roots. `level(p)`: `false` if `p` is unknown, else the nodes of `children[p]`. Never `nil` (empty slice).
- `search(q, limit)`: `words := strings.Fields(nameparse.Norm(q))`; fewer than 2 letters/digits in total (`len([]rune(strings.Join(words, "")))  < 2`) → empty result (`Levels` empty map, `Matches` empty slice). A folder matches when, with `nw := strings.Fields(nameparse.Norm(f.Name))`, every word but the last equals some word of `nw` and the last word is a prefix of some word of `nw`. Skip the single root (it is not a row). Sort the matching folder paths with `natsort.Less`; `More = len > limit`; keep the first `limit`. For each match `m`, walk from the top: `key := ""; loop { nodes := level(key); find n with n.Path == m || strings.HasPrefix(n.Path, m+"/")` (m is inside n's chain) → record n in `Levels[key]`, add `n.Path` to `Matches`, stop; else find n with `strings.HasPrefix(m, n.Path+"/")` → record n in `Levels[key]`, `key = n.Path`, continue; none → stop (cannot happen for a catalog folder) `}`. Dedupe rows per level and matches (two matching folders of one chain are one row); keep each level's rows in the level's own order (filter `level(key)` by the recorded set), and `Matches` sorted with `natsort.Less`.
- `Store.FolderTree(p)`: load, `level(p)`, then fill `SizeBytes` of every node with `folderStats(node.Path)`. `Store.SearchFolderTree(q)`: load, `search(q, TreeSearchLimit)`, fill sizes of every node in every level.

- [ ] **Step 1: Write the failing tests** in `internal/store/tree_test.go`:
  - `TestTreeChains` (pure, hand-built `[]treeFolder`): root `r` (Own 0) with children `r/a` (Own 0, one child `r/a/b`, Own 0, one child `r/a/b/c`, Own 2, no children) and `r/z` (Own 1, one child `r/z/y`). `level("")` → 2 rows: `{Path: "r/a/b/c", Label: "a › b › c", Name: "c", HasChildren: false}` and `{Path: "r/z", Label: "z", HasChildren: true}` (z has own tracks: not merged although it has one child). A folder with 0 own tracks and **two** children is not merged. `level("r/a")` → one row `r/a/b/c` labelled `b › c`. `level("nope")` → false.
  - `TestTreeSeveralRoots` (pure): two roots `r1`, `r2` → `level("")` lists both roots (each merged if pass-through), not their children.
  - `TestFolderTree` (store, `explorer(t)`): `FolderTree("")` → three rows (`Brasil` and `Uruguay` have no own tracks and one subfolder each, so they are merged too), labels in order: `Brasil › Música Popular` (NFD name as stored; compare with the `Mu\u0301sica` literal), `Soulseek › complete`, `Uruguay › Jaime Roos`. The Soulseek row: `Path == sk`, `Tracks == 6`, `SizeBytes == 62`, `HasChildren`, `Album == 0`. `FolderTree(sk)` → `Low_ID` (`Label "Low_ID"`, `HasChildren`, `Tracks 4`) and `benteveo › Zitarrosa › Canta` (`Path == sk+"/benteveo/Zitarrosa/Canta"`, `Album != 0`, `HasChildren == false`, `SizeBytes == 20`), in that order (natural, case-insensitive: `benteveo` < `Low_ID`, so **benteveo first** — assert that order). `FolderTree("../nope")` → `ok == false`.
  - `TestSearchFolderTree` (store, `explorer(t)`):
    - `"zitar"` → `Matches == [sk+"/benteveo/Zitarrosa/Canta"]`; `Levels[""]` is just the Soulseek row; `Levels[sk]` is just the Canta row.
    - `"musica"` (no accent) matches the NFD `Música Popular`: `Matches == ["../musica/Brasil/Mu\u0301sica Popular"]`, `Levels[""]` has only that row.
    - `"music of"` → matches `music of the crusades` (words `music`, `of` prefix); `"of music"` → no match? `of` must equal a word (yes, "of") and `music` must prefix a word (yes) → matches too; assert it matches (word order does not matter).
    - `"lhasa sel"` matches `Lhasa de Sela`; `"lhas de"` does **not** (only the last word is a prefix).
    - `"m"` (1 letter) → empty `Matches`, empty `Levels`.
    - Two matches under one level share it: `"l"`-prefixed query `"lo"` matches `Low_ID` only (prefix of `low`): `Levels[sk]` has one row; and query `"complete"` matches the pass-through `complete`, reported as the row `sk` (`Matches == [sk]`).
    - `More`: pure test with 205 matching folders under one root and `search(q, 200)` → `len(Matches) == 200`, `More == true`.
- [ ] **Step 2: Run** `go test ./internal/store -run 'Tree' -v` — expect failures.
- [ ] **Step 3: Implement** `tree.go`.
- [ ] **Step 4: Run** `go test ./internal/store`, `go vet ./...`, `gofmt -l .`.
- [ ] **Step 5: Commit** `feat: folder tree with merged pass-through chains and a name filter`

---

### Task 3: Server — tree endpoints, playable files

**Model:** Sonnet.

**Files:**
- Create: `internal/server/folders.go`
- Create: `internal/server/folders_test.go`
- Modify: `internal/server/server.go` (routes; move `folder` and `setCollection` handlers into `folders.go` unchanged except as below)
- Modify: `internal/server/media_test.go` only if `TestFolderArt`'s decode struct no longer fits (it embeds `store.FolderView`, which now has `Children []FolderChild` — it should still decode).
- Modify: `README.md` (endpoint table)

**Interfaces:**
- Consumes: `Store.Folder`, `Store.FolderTree`, `Store.SearchFolderTree`, `store.FolderFile`, `s.Streamer.Plan`, `stream.Unplayable`, the test helpers `mediaServer`, `do`, `decode` (`internal/server/*_test.go`).
- Produces (HTTP):
  - `GET /api/folders/tree` and `GET /api/folders/tree?path=P` → `[]store.TreeNode` (JSON array, `[]` when empty). Unknown `P` → 404 `carpeta no encontrada`.
  - `GET /api/folders/tree/search?q=Q` → `store.TreeSearch`. Missing/short `q` → 200 with the empty result.
  - `GET /api/folders?path=` → as before plus `sizeBytes`, children's `sizeBytes`/`album`/`codec`, and `files: [{…store.FolderFile, "playable": bool}]`.
  ```go
  // folderFile is a file of a folder as the page gets it.
  type folderFile struct {
      store.FolderFile
      Playable bool `json:"playable"` // a track the server can deliver (false: not a track, or it needs ffmpeg)
  }
  ```

**Design:**
- Routes in `Handler()`: `mux.HandleFunc("GET /api/folders/tree", s.folderTree)`, `mux.HandleFunc("GET /api/folders/tree/search", s.searchFolderTree)` next to `GET /api/folders`.
- `folder`: build the existing anonymous struct but replace `Files` by `[]folderFile` (shadow the embedded field with a field `Files []folderFile \`json:"files"\`` on the outer struct, so the JSON key is `files` once). `Playable = f.Track && mode != stream.Unplayable` with `mode, _ := s.Streamer.Plan(stream.Format{Container: f.Format, Codec: f.Codec, Lossless: f.Lossless})`.

- [ ] **Step 1: Write the failing tests** in `internal/server/folders_test.go`, using `mediaServer(t, "")` (root `../musica`, one folder `Virus - Locura` with `01.mp3` (mp3) and `02.wma` (asf/wma), both size 10):
  - `TestFolderTreeEndpoint`: `GET /api/folders/tree` → one row, `label == "Virus - Locura"`, `tracks == 2`, `sizeBytes == 20`, `album != 0`, `hasChildren == false`. `GET /api/folders/tree?path=../musica/Virus%20-%20Locura` → `[]` (decode into a slice, check `len == 0` and that the body starts with `[`). `GET /api/folders/tree?path=../nope` → 404.
  - `TestFolderTreeSearchEndpoint`: `?q=locu` → `matches == ["../musica/Virus - Locura"]`, `levels[""]` has that row; `?q=x` → `matches` empty, status 200.
  - `TestFolderFilesPlayable`: `GET /api/folders?path=../musica/Virus%20-%20Locura` → `sizeBytes == 20`, `files` has `01.mp3` (`playable: true`, `track: true`) and `02.wma` (`playable: false`); after `s.Streamer.FFmpeg = "ffmpeg"` (as `TestArtistEndpoint` does: `Plan` only checks it is set) both are playable.
- [ ] **Step 2: Run** `go test ./internal/server -run 'Folder' -v` — expect failures.
- [ ] **Step 3: Implement** `folders.go`, the routes, and update `README.md`'s endpoint table: change the `GET /api/folders?path=` row to "a folder: subfolders (with size), albums, loose tracks and the files directly in it (no `path`: the roots)" and add rows for `GET /api/folders/tree?path=` ("one level of the Carpetas tree: subfolders, pass-through chains merged") and `GET /api/folders/tree/search?q=` ("folders by name, with the tree rows leading to them").
- [ ] **Step 4: Run** `go test ./...`, `go vet ./...`, `gofmt -l .`.
- [ ] **Step 5: Commit** `feat: Carpetas tree endpoints; playable files in the folder view`

---

### Task 4: Web — sizes and the detail list logic

**Model:** Sonnet.

**Files:**
- Modify: `web/src/lib/format.js`, `web/src/lib/format.test.js`
- Create: `web/src/lib/folders.js`, `web/src/lib/folders.test.js`

**Interfaces:**
- Consumes: `codecName`, `quality` (`format.js`); the `GET /api/folders` JSON (Task 3).
- Produces:
  ```js
  // format.js
  export function size(bytes) // "84 KB", "7,2 MB", "1,4 GB", "3,0 GB", "410 MB", "512 B", "" for null/undefined
  // folders.js
  export function entries(folder) // { folders: Entry[], files: Entry[] }
  // Entry: { key, type: 'folder'|'audio'|'other', name, path?, id?, tracks, durationMs, format, sizeBytes,
  //          album, collection, playable }
  export function sortEntries(list, sort) // new array; sort = { column: null|'name'|'size'|'tracks'|'duration'|'format', desc }
  export function nextSort(sort, column)  // { column, desc }
  export function bar(sizeBytes, totalBytes) // percent 0–100 for the relative size bar
  export function playableIds(files)      // ids of the playable audio entries, in the given order
  export const DEFAULT_SORT = { column: null, desc: false }
  ```

**Design:**
- `size`: decimal units (1000), as the Finder: `< 1000` → `"N B"`; else divide by 1000 through `KB, MB, GB, TB`; values `< 10` with one decimal, else rounded to an integer; Spanish decimal comma via `toLocaleString('es', { minimumFractionDigits, maximumFractionDigits })`. Round **before** choosing the unit's decimals so `9.96 MB` reads `"10 MB"`, not `"10,0 MB"`; and `999.6 KB` reads `"1,0 MB"`.
- `entries(folder)`:
  - folders from `folder.children`: `{ key: 'd:' + path, type: 'folder', name, path, tracks: trackCount, durationMs, format: codec ? codecName({ codec }) : '', sizeBytes, album, collection, playable: trackCount > 0 }`.
  - files from `folder.files`: audio → `{ key: 'f:' + id, type: 'audio', name, id, tracks: 0, durationMs, format: quality(file), sizeBytes, album: 0, collection: false, playable }`; anything else → `type: 'other'`, `durationMs: 0`, `format: ''`, `playable: false`.
  - Missing `children`/`files` → empty arrays.
- `sortEntries(list, { column: null })` returns a copy in the given order. Otherwise a **stable** sort by the column: `name`, `format` with `Intl.Collator('es', { numeric: true, sensitivity: 'base' })`; `size` → `sizeBytes`, `tracks`, `duration` → `durationMs` numerically; `desc` reverses the comparison (not the array, so ties keep their order). Sorting is applied separately to `folders` and `files` by the caller.
- `nextSort(sort, column)`: same column → `{ column, desc: !sort.desc }`; another column → `{ column, desc: column !== 'name' && column !== 'format' }` (numbers start biggest first).
- `bar(size, total)`: `total <= 0 || !size` → 0; else `Math.max(1, Math.round(size / total * 100))`, capped at 100.

- [ ] **Step 1: Write the failing tests.**
  `format.test.js` — add:
  ```js
  describe('size', () => {
    it.each([
      [512, '512 B'], [84_000, '84 KB'], [7_200_000, '7,2 MB'], [1_400_000_000, '1,4 GB'],
      [3_000_000_000, '3,0 GB'], [410_000_000, '410 MB'], [9_960_000, '10 MB'], [999_600, '1,0 MB'],
      [30_100_000_000, '30 GB'], [0, '0 B'],
    ])('%d', (n, want) => expect(size(n)).toBe(want))
    it('is empty without a value', () => expect(size(undefined)).toBe(''))
  })
  ```
  `folders.test.js`:
  ```js
  import { describe, expect, it } from 'vitest'
  import { bar, DEFAULT_SORT, entries, nextSort, playableIds, sortEntries } from './folders.js'

  const folder = {
    sizeBytes: 3_000_000_000,
    children: [
      { path: 'm/RARAS', name: 'RARAS PARTITURAS', trackCount: 183, durationMs: 39_600_000, sizeBytes: 1_400_000_000, codec: 'mp3', album: 0, collection: false },
      { path: 'm/Lhasa', name: 'Lhasa de Sela', trackCount: 35, durationMs: 7_200_000, sizeBytes: 410_000_000, codec: 'flac', album: 4, collection: false },
      { path: 'm/vacia', name: 'vacía', trackCount: 0, durationMs: 0, sizeBytes: 0, codec: '', album: 0, collection: false },
    ],
    files: [
      { id: 9, name: 'suelto.mp3', kind: 'audio', sizeBytes: 7_200_000, durationMs: 187_000, format: 'mp3', codec: 'mp3', bitrate: 320, playable: true, track: true },
      { id: 10, name: 'raro.wma', kind: 'audio', sizeBytes: 5_000_000, durationMs: 100_000, format: 'asf', codec: 'wma', bitrate: 128, playable: false, track: true },
      { id: 11, name: 'folder.jpg', kind: 'image', sizeBytes: 84_000, playable: false, track: false },
    ],
  }

  describe('entries', () => {
    it('lists subfolders, then files', () => {
      const { folders, files } = entries(folder)
      expect(folders.map((e) => [e.type, e.name, e.format, e.playable])).toEqual([
        ['folder', 'RARAS PARTITURAS', 'MP3', true], ['folder', 'Lhasa de Sela', 'FLAC', true], ['folder', 'vacía', '', false],
      ])
      expect(files.map((e) => [e.type, e.name, e.format, e.playable])).toEqual([
        ['audio', 'suelto.mp3', 'MP3 320', true], ['audio', 'raro.wma', 'WMA 128', false], ['other', 'folder.jpg', '', false],
      ])
      expect(folders[1]).toMatchObject({ key: 'd:m/Lhasa', path: 'm/Lhasa', tracks: 35, album: 4 })
      expect(files[0]).toMatchObject({ key: 'f:9', id: 9, durationMs: 187_000, sizeBytes: 7_200_000 })
    })
    it('copes with a folder without files', () => {
      expect(entries({ children: [] })).toEqual({ folders: [], files: [] })
    })
  })

  describe('sorting', () => {
    const { folders, files } = entries(folder)
    it('keeps the server order by default', () => {
      expect(sortEntries(folders, DEFAULT_SORT).map((e) => e.name)).toEqual(['RARAS PARTITURAS', 'Lhasa de Sela', 'vacía'])
    })
    it('sorts by name in natural order, and back', () => {
      const named = [{ name: '[LA-10] b' }, { name: '[LA-9] a' }, { name: 'Árbol' }]
      expect(sortEntries(named, { column: 'name', desc: false }).map((e) => e.name)).toEqual(['[LA-9] a', '[LA-10] b', 'Árbol'])
      expect(sortEntries(named, { column: 'name', desc: true }).map((e) => e.name)).toEqual(['Árbol', '[LA-10] b', '[LA-9] a'])
    })
    it('sorts by size, tracks, duration and format', () => {
      expect(sortEntries(folders, { column: 'size', desc: true }).map((e) => e.name)).toEqual(['RARAS PARTITURAS', 'Lhasa de Sela', 'vacía'])
      expect(sortEntries(folders, { column: 'tracks', desc: false }).map((e) => e.tracks)).toEqual([0, 35, 183])
      expect(sortEntries(files, { column: 'duration', desc: false }).map((e) => e.name)).toEqual(['folder.jpg', 'raro.wma', 'suelto.mp3'])
      expect(sortEntries(folders, { column: 'format', desc: false }).map((e) => e.format)).toEqual(['', 'FLAC', 'MP3'])
    })
    it('is stable on ties, both ways', () => {
      const tied = [{ name: 'a', sizeBytes: 1 }, { name: 'b', sizeBytes: 1 }]
      expect(sortEntries(tied, { column: 'size', desc: true }).map((e) => e.name)).toEqual(['a', 'b'])
    })
    it('flips the same column, starts numbers biggest first', () => {
      expect(nextSort(DEFAULT_SORT, 'name')).toEqual({ column: 'name', desc: false })
      expect(nextSort({ column: 'name', desc: false }, 'name')).toEqual({ column: 'name', desc: true })
      expect(nextSort({ column: 'name', desc: false }, 'size')).toEqual({ column: 'size', desc: true })
      expect(nextSort(DEFAULT_SORT, 'format')).toEqual({ column: 'format', desc: false })
    })
  })

  describe('bar', () => {
    it('is relative to the folder', () => {
      expect(bar(1_400_000_000, 3_000_000_000)).toBe(47)
      expect(bar(1, 3_000_000_000)).toBe(1)
      expect(bar(0, 3_000_000_000)).toBe(0)
      expect(bar(5, 0)).toBe(0)
      expect(bar(10, 5)).toBe(100)
    })
  })

  describe('playableIds', () => {
    it('keeps the playable audio, in order', () => {
      expect(playableIds(entries(folder).files)).toEqual([9])
    })
  })
  ```
- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/format.test.js src/lib/folders.test.js` — expect failures.
- [ ] **Step 3: Implement** `size` and `folders.js`.
- [ ] **Step 4: Run** `cd web && npx vitest run` (all pass).
- [ ] **Step 5: Commit** `feat: web logic for the folder detail list (sizes, sorting, relative bar)`

---

### Task 5: Web — the tree logic

**Model:** Sonnet.

**Files:**
- Create: `web/src/lib/tree.js`, `web/src/lib/tree.test.js`

**Interfaces:**
- Consumes: tree rows as in "Shared definitions" (`{path, label, name, tracks, sizeBytes, album, collection, hasChildren}`), `store.TreeSearch` JSON (`{levels, matches, more}`).
- Produces:
  ```js
  // levels: { [levelKey]: Node[] } ('' is the top); open: { [path]: true }
  export function visibleRows(levels, open, key = '')  // [{ node, depth, parent }] depth 1 at the top; parent = level key
  export function missingLevels(levels, open, key = '') // paths of visible open rows whose level is not loaded
  export function locate(levels, target)               // { chain: string[], node: string|null, missing: string|null }
  export async function reveal(levels, target, load)   // loads what locate() misses; returns { chain, node }
  export function keyAction(rows, cursor, key, open)   // { type: 'cursor'|'open'|'close'|'show', path } | null
  export function isFilterKey(event)                   // '/' without modifiers, outside text fields
  export const OPEN_KEY = 'tordo.carpetas.abiertas'
  export function loadOpen(storage)                    // {} on any error or bad data
  export function saveOpen(storage, open)              // swallows errors
  export function filterOpen(levels)                   // every level key of a search result, open
  export function matchRows(result)                    // [{ node, parent }] one per match, in match order
  export function direction(from, to)                  // 'forward' | 'back' | 'none'
  export function shortPath(path, root)                // path below root, middle segments collapsed
  ```

**Design (rules the tests pin down):**
- `visibleRows`: depth-first; a row's children are listed only if `open[row.path]` and `levels[row.path]` is loaded. Unknown `key` → `[]`.
- `missingLevels`: walk like `visibleRows`; collect `row.path` where `open[row.path] && row.node.hasChildren && !levels[row.path]`.
- `locate(levels, target)`: `chain = []`, `key = ''`; loop: `list = levels[key]`; not loaded → `{ chain, node: null, missing: key }`. In `list`, a row `n` that contains target in its chain → `{ chain, node: n.path, missing: null }`. A row's chain starts at `top`: `n.path` with its last `label.split(' › ').length - 1` segments removed; target is in the chain when `n.path === target`, or when `n.path.startsWith(target + '/')` **and** (`target === top` or `target.startsWith(top + '/')`). Without the `top` check, a folder above the level (the single root `m`) would wrongly match every row. Else a row with `target.startsWith(n.path + '/')` → `chain.push(n.path)`, `key = n.path`, continue. Else → `{ chain, node: null, missing: null }` (not in the tree: e.g. the single root itself). `target === ''` → `{ chain: [], node: null, missing: null }` once level `''` is loaded.
- `reveal(levels, target, load)`: loop `locate`; when `missing !== null`, `levels[missing] = await load(missing)` and retry; at most 64 rounds (guard); returns `{ chain, node }`. Mutates `levels` (the caller passes its `$state` object).
- `keyAction(rows, cursor, key, open)` (`rows` from `visibleRows`; `cursor` a row path or null):
  - no rows → null. Cursor not among rows → `ArrowDown`/`ArrowUp`/`Home`/`End` set the cursor to the first row; others null.
  - `ArrowDown` / `ArrowUp`: next / previous row (stay at the ends: return `{type:'cursor', path: same}`).
  - `Home` / `End`: first / last row.
  - `ArrowRight`: no children → null; closed → `{type:'open'}`; open → the next row if it is a child (its `parent === cursor`) → `{type:'cursor', path: child}`, else null (children still loading).
  - `ArrowLeft`: open → `{type:'close'}`; else its `parent` if it is a row (`parent !== ''`) → `{type:'cursor', path: parent}`; else null.
  - `Enter` → `{type:'show', path: cursor}`. Anything else → null.
- `isFilterKey(event)`: `event.key === '/'` and no `ctrlKey/metaKey/altKey`, and `event.target` is not an `INPUT`, `TEXTAREA`, `SELECT` or `isContentEditable`.
- `loadOpen(storage)`: parse `storage.getItem(OPEN_KEY)` as a JSON array of strings → object; anything else (throws, null, not an array, non-strings) → `{}`. `saveOpen` writes `JSON.stringify(Object.keys(open).filter(k => open[k]))` inside try/catch.
- `filterOpen(levels)`: `Object.fromEntries(Object.keys(levels).filter(k => k !== '').map(k => [k, true]))`.
- `matchRows(result)`: for each `match` find the row with that path in any `result.levels[k]` → `{ node, parent: k }`; skip matches not found.
- `direction(from, to)`: equal → `'none'`; `from === ''` or `to.startsWith(from + '/')` → `'forward'`; else `'back'`.
- `shortPath(path, root)`: strip `root + '/'` when it is a prefix, else strip leading `../` segments; split on `/`; more than 3 segments → `first/…/second-to-last/last` (`…` is U+2026); return joined with `/`.

- [ ] **Step 1: Write the failing tests** in `web/src/lib/tree.test.js`. Use this fixture:
  ```js
  const n = (path, label, hasChildren = true) => ({ path, label, name: label.split(' › ').pop(), tracks: 1, sizeBytes: 1, album: 0, collection: false, hasChildren })
  const S = 'm/Soulseek/complete'
  const levels = () => ({
    '': [n('m/Brasil', 'Brasil'), n(S, 'Soulseek › complete'), n('m/Uruguay', 'Uruguay', false)],
    [S]: [n(S + '/benteveo/Zitarrosa/Canta', 'benteveo › Zitarrosa › Canta', false), n(S + '/Low_ID', 'Low_ID')],
    [S + '/Low_ID']: [n(S + '/Low_ID/Lhasa', 'Lhasa', false)],
  })
  ```
  Cases (write each as an `it`):
  - `visibleRows(levels(), {})` → paths `['m/Brasil', S, 'm/Uruguay']`, all depth 1, parent `''`. With `open = {[S]: true, [S+'/Low_ID']: true}` → `['m/Brasil', S, S+'/benteveo/Zitarrosa/Canta', S+'/Low_ID', S+'/Low_ID/Lhasa', 'm/Uruguay']` with depths `[1,1,2,2,3,1]` and the Lhasa row's parent `S+'/Low_ID'`. Open but unloaded `m/Brasil` shows no children.
  - `missingLevels(levels(), { 'm/Brasil': true, [S]: true, 'm/Uruguay': true })` → `['m/Brasil']` (Uruguay has no children; S is loaded).
  - `locate`: target `S+'/Low_ID/Lhasa'` → `{ chain: [S, S+'/Low_ID'], node: S+'/Low_ID/Lhasa', missing: null }`; target `'m/Soulseek'` (pass-through inside the row S) → `{ chain: [], node: S }`; target `S+'/benteveo/Zitarrosa'` → `{ chain: [S], node: S+'/benteveo/Zitarrosa/Canta' }`; target `'m/Brasil/x'` → `missing: 'm/Brasil'`, `chain: ['m/Brasil']`; target `'m'` (the root) → `node: null, missing: null`; target `'m/Soul'` (a prefix of a name, not a folder) → `node: null` (no false match on `startsWith` without `/`); with `{}` levels → `missing: ''`.
  - `reveal` with a fake `load` that returns `levels()[key]` (or, for `'m/Brasil'`, `[n('m/Brasil/x', 'x', false)]`) and records the keys: starting from `{}`, `reveal(lv, S+'/Low_ID/Lhasa', load)` loads `['', S, S+'/Low_ID']` in that order and returns `{ chain: [S, S+'/Low_ID'], node: S+'/Low_ID/Lhasa' }`; a second `reveal` of the same target loads nothing.
  - `keyAction` over `rows = visibleRows(levels(), open)` with `open = {[S]: true}` (rows: Brasil, S, Canta, Low_ID, Uruguay):
    - `ArrowDown` from Brasil → cursor S; `ArrowUp` from Brasil → cursor Brasil; `End` → Uruguay; `Home` → Brasil.
    - `ArrowRight` on Brasil (closed, has children) → `{type:'open', path:'m/Brasil'}`; on S (open) → cursor Canta; on Uruguay (no children) → null; on Low_ID while open but not loaded (`open[S+'/Low_ID'] = true`, level removed) → null.
    - `ArrowLeft` on S (open) → `{type:'close', path:S}`; on Canta → cursor S; on Brasil (closed, top) → null.
    - `Enter` on Low_ID → `{type:'show', path:S+'/Low_ID'}`; `'x'` → null; cursor `null` + `ArrowDown` → cursor Brasil; empty rows → null.
  - `isFilterKey`: `{ key: '/', target: { tagName: 'BODY' } }` → true; with `ctrlKey: true` → false; target `{ tagName: 'INPUT' }` → false; target `{ tagName: 'DIV', isContentEditable: true }` → false; `{ key: 'a', target: { tagName: 'BODY' } }` → false.
  - `loadOpen`/`saveOpen` with a fake storage (`{ getItem, setItem }` over a Map): round trip of `{a: true, b: true, c: false}` → `{a: true, b: true}`; `getItem` returning `'nope'`, `'{"a":1}'`, `'[1]'`, or throwing → `{}`; `setItem` throwing does not throw.
  - `filterOpen({ '': [], a: [], b: [] })` → `{ a: true, b: true }`.
  - `matchRows({ levels: levels(), matches: [S+'/Low_ID/Lhasa', 'm/nope', 'm/Uruguay'] })` → `[{ node: (Lhasa row), parent: S+'/Low_ID' }, { node: (Uruguay row), parent: '' }]`.
  - `direction('', 'm/a')` → forward; `('m/a', 'm/a/b')` → forward; `('m/a/b', 'm/a')` → back; `('m/a', 'm/ab')` → back; `('m/a', 'm/a')` → none.
  - `shortPath('../Music/Soulseek/complete/Low_ID/Lhasa', '../Music')` → `'Soulseek/…/Low_ID/Lhasa'`; `shortPath('../Music/Uruguay/Zitarrosa', '../Music')` → `'Uruguay/Zitarrosa'`; `shortPath('../../otro/a/b', '')` → `'otro/a/b'`.
- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/tree.test.js` — expect failures.
- [ ] **Step 3: Implement** `tree.js`.
- [ ] **Step 4: Run** `cd web && npx vitest run`.
- [ ] **Step 5: Commit** `feat: web logic for the folder tree (rows, reveal, keys, filter)`

---

### Task 6: Web — the wide layout: page, shared tree state, content panel

**Model:** Sonnet.

**Files:**
- Modify: `web/src/lib/api.js`, `web/src/lib/api.test.js`
- Create: `web/src/lib/carpetas.svelte.js`
- Modify: `web/src/App.svelte`
- Rewrite: `web/src/pages/Carpetas.svelte`
- Create: `web/src/components/ContenidoCarpeta.svelte`
- Modify: `web/src/components/Icono.svelte` (add `chevron_right`: `<polyline points="9 18 15 12 9 6"/>`, `chevron_left`: `<polyline points="15 18 9 12 15 6"/>`, `file`: `<path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/><polyline points="13 2 13 9 20 9"/>`, `music`: `<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>` — Feather, MIT)

**Interfaces:**
- Consumes: Tasks 3–5; `Acciones.svelte` (`what={{ folder: path }}`), `Tapa.svelte`, `albumCoverURL`, `play`/`enqueue` (`player.svelte.js`: `play({ tracks }, { startId })`, `enqueue({ tracks: [id] })`, `play({ folder })`), `app.generation`, `app.status?.readOnly`, `notify`, `navigate`, `savedScroll`, `folderHref`, `albumHref`, `count`, `length`, `duration`, `size`.
- Produces:
  ```js
  // api.js
  folderTree: (path) => get(path ? `/api/folders/tree?${new URLSearchParams({ path })}` : '/api/folders/tree'),
  folderTreeSearch: (q) => get(`/api/folders/tree/search?${new URLSearchParams({ q })}`),
  // carpetas.svelte.js — the tree state both layouts share, kept across visits to the page
  export const tree = $state({ levels: {}, open: {}, generation: -1 })
  export function syncGeneration(generation) // a scan ended: forget the loaded levels (keep open branches)
  export async function loadLevel(key)       // tree.levels[key] = await api.folderTree(key)
  export function revealPath(target)         // reveal(tree.levels, target, api.folderTree) — marks the chain open and saves
  export function setOpen(path, on)          // tree.open[path] = on (delete when false); saveOpen(localStorage, tree.open)
  // tree.open starts from loadOpen(localStorage) (guarded: localStorage itself may throw)
  ```
  `ContenidoCarpeta.svelte` props: `folder` (the `/api/folders` response). `Carpetas.svelte` props: `path` (from the route). Task 7 adds `<ArbolCarpetas selected={path} />` in the left slot; Task 8 adds the narrow branch.

**Design:**
- `App.svelte`: render `<Carpetas path={current.path} />` **without** `{#key current.path}` so the tree survives navigation; the page reloads its content in an `$effect` on `path` and `app.generation`.
- `Carpetas.svelte` (wide only in this task):
  - Content load effect (keep the stale-guard pattern of the current page): `api.folder(path)`; on `''` with exactly one child → `navigate(folderHref(child.path), { replace: true })` (remember `rootPath` in the page for Task 8); 404 (`e.status === 404`) → message `Esta carpeta ya no está en el catálogo.`; empty catalog at `''` → the existing message `Todavía no hay música en el catálogo. Revisá las carpetas de config.json.` Also call `syncGeneration(app.generation)` in the effect.
  - Several roots: at `''` the content panel shows `<h1>Carpetas</h1>` and the roots as folder rows (the `/api/folders` answer for `''` has `children` and no `trail`/`files`); `ContenidoCarpeta` must cope with that (no crumbs, no totals line, no actions).
  - Layout: `<div class="explorer"><aside class="tree" aria-label="Carpetas"><!-- Task 7 --></aside><section class="content">…</section></div>`. CSS: `.explorer { display: grid; grid-template-columns: minmax(220px, 300px) minmax(0, 1fr); gap: 24px; }`; `.tree { position: sticky; top: <the Nav bar height: read it from Nav.svelte, or use a CSS variable if it defines one>; max-height: calc(100dvh - <nav> - var(--player-h) - var(--tabs-h)); overflow: auto; }` — the tree scrolls on its own, the content scrolls with the window.
- `ContenidoCarpeta.svelte`:
  - Header: crumbs from `folder.trail` (every level a link with `folderHref`; the last one plain text), `Colección` label when `folder.collection`, `<h1>{folder.name}</h1>`, totals line `{count(albumCount,'álbum','álbumes')} · {count(trackCount,'pista','pistas')} · {length(durationMs)} · {size(sizeBytes)}`, `<Acciones what={{ folder: folder.path }} />` when `trackCount > 0`, the `No es una colección / Es una colección` button when `folder.detected && !app.status?.readOnly` (same `api.setCollection` call and update as the current page). Album folder (`folder.albums.length === 1 && folder.albums[0].dir === folder.path`): a 64 px `Tapa` with `albumCoverURL(id, 160)` (only if `album.picture`; otherwise the `disc` icon) and a link `Ver álbum` to `albumHref(id)`.
  - Detail list: a `<table>` with header buttons `Nombre · Tamaño relativo · Pistas · Duración · Formato · Tamaño`; each sortable header is a `<button>` calling `sort = nextSort(sort, column)`; the `<th>` has `aria-sort` (`ascending`/`descending`/`none`). The bar column is not sortable (it mirrors Tamaño). `sort` resets to `DEFAULT_SORT` when `folder.path` changes. Rows: `sortEntries(folders, sort)` then `sortEntries(files, sort)`.
  - Folder row: icon `disc` if `album` else `folder` (and the `Colección` label after the name when `collection`); the name is a link `folderHref(path)` (navigating selects it; Task 7's tree follows the URL); a `<div class="bar" style="width: {bar(sizeBytes, folder.sizeBytes)}%">`; tracks (`toLocaleString('es')`), `length(durationMs)` (empty when 0), format, `size(sizeBytes)`; a ▶ button (visible on row hover or focus, hidden with `@media (hover: none)`), `aria-label="Reproducir {name}"`, calling `play({ folder: path })`, only when `tracks > 0`.
  - Audio row: icon `music`; the name is a `<button>` that plays from it: `play({ tracks: playableIds(sortedFiles) }, { startId: id })` (the files in the order shown); `duration(durationMs)`, format, size; a `+` button (`aria-label="Agregar {name} a la cola"`) → `enqueue({ tracks: [id] })`. Not playable: dimmed (`opacity: .5`), name button disabled, `title="No se puede reproducir: falta ffmpeg"` when `track` is true.
  - Other file row: icon `file`, dimmed, size only, no action.
  - Empty folder (no rows): nothing extra (the header is enough).
- [ ] **Step 1: Add to `api.test.js`'s `asks the server` test** the calls `await api.folderTree('')`, `await api.folderTree('../musica/a b')`, `await api.folderTreeSearch('zitar')` and expect `['GET', '/api/folders/tree', null]`, `['GET', '/api/folders/tree?path=..%2Fmusica%2Fa+b', null]`, `['GET', '/api/folders/tree/search?q=zitar', null]`. Run `cd web && npx vitest run src/lib/api.test.js` — fails.
- [ ] **Step 2: Implement** the api functions; run the test — passes.
- [ ] **Step 3: Implement** `carpetas.svelte.js`, `Icono.svelte` icons, `ContenidoCarpeta.svelte`, `Carpetas.svelte`, `App.svelte`.
- [ ] **Step 4: Run** `cd web && npx vitest run && npm run build` (no warnings from Svelte about a11y or unused CSS other than what was already there; fix new ones).
- [ ] **Step 5: Commit** `feat: Carpetas content panel as a detail list`

---

### Task 7: Web — the tree panel (filter, keyboard, ARIA)

**Model:** Sonnet.

**Files:**
- Create: `web/src/components/ArbolCarpetas.svelte`
- Modify: `web/src/pages/Carpetas.svelte` (mount it in the `aside`)

**Interfaces:**
- Consumes: `tree`, `loadLevel`, `revealPath`, `setOpen` (`carpetas.svelte.js`); `visibleRows`, `missingLevels`, `keyAction`, `isFilterKey`, `filterOpen` (`tree.js`); `api.folderTreeSearch`; `search.MIN` (`web/src/lib/search.js`, the minimum query length); `navigate`, `folderHref`.
- Produces: `<ArbolCarpetas selected={path} />` — `selected` is the folder shown on the right (the URL's `ruta`).

**Design:**
- State: `cursor` (row path), `query` (filter text), `result` (search result or null).
- Effects:
  1. On `selected` (and after `tree.levels` is reset by a scan): `revealPath(selected)` → `cursor = node ?? cursor`; then scroll the cursor row into view (`block: 'nearest'`). Errors → `notify(e.message)`.
  2. Lazy levels: whenever `missingLevels(tree.levels, tree.open)` is non-empty, `loadLevel` each (once per key in flight).
  3. Filter: debounce 200 ms; `query.trim().length < MIN` → `result = null`; else `api.folderTreeSearch(query)` (ignore stale responses).
- Rows: `result ? visibleRows(result.levels, filterOpen(result.levels)) : visibleRows(tree.levels, tree.open)`. In filter mode ▸/▾ is shown open and clicking it does nothing; a footer line `Hay más coincidencias: escribí más para acotar.` when `result.more`; `Ninguna carpeta se llama así.` when `result.matches.length === 0`. Matching rows' labels are bold.
- Markup (WAI-ARIA tree, flat): `<ul role="tree" aria-label="Carpetas" onkeydown>`; each row `<li role="treeitem" aria-level={depth} aria-expanded={hasChildren ? isOpen : undefined} aria-selected={row.node.path === selected} tabindex={row.node.path === cursor ? 0 : -1} data-path>`; when no row has the cursor, the first row gets `tabindex=0`. Row content: indentation `padding-left: calc((depth - 1) * 16px + 4px)`; a toggle `<span>` ▸/▾ (Icono `chevron_right`, rotated 90° when open) that toggles `setOpen` on click (`stopPropagation`); icon (`disc` when `album`, else `folder`); the `label` (ellipsis, `title={label}`); a `Colección` mark when `collection`; the track count right-aligned in muted digits (`toLocaleString('es')`).
- Click on a row: `cursor = path; navigate(folderHref(path))`.
- Keys (`onkeydown` on the `ul`): `action = keyAction(rows, cursor, event.key, openOf)` where `openOf` is `tree.open` (or `filterOpen(...)` in filter mode, where `open`/`close` are ignored); if `action` → `event.preventDefault()`; `cursor` → set and focus that `li`; `open`/`close` → `setOpen(path, true/false)`; `show` → `navigate(folderHref(path))`.
- `/`: `<svelte:window onkeydown>` → `if (isFilterKey(event)) { event.preventDefault(); input.focus() }`. `Escape` in the input clears it and returns focus to the tree. `ArrowDown` in the input moves focus to the cursor row (or the first row).
- The input: `<input type="search" placeholder="Filtrar carpetas…" aria-label="Filtrar carpetas" bind:value={query}>` at the top of the panel, sticky inside it.
- Styling: row height ~30 px, 14 px text; selected row (`aria-selected`) with `var(--surface-2)` background and `var(--strong)` text; the cursor's focus ring is the global `:focus-visible`.
- [ ] **Step 1: Implement** the component and mount it.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`.
- [ ] **Step 3: Smoke-check in the browser** against a dev catalog: `go run ./cmd/tordo -dir .run` (after the build), open `/carpetas`; expand and collapse with mouse and keys, reload (branches stay open), `/` then type a name with and without accent, `Escape`. Use the Browser pane (`preview_start` with the URL) and `read_page`; report what was checked.
- [ ] **Step 4: Commit** `feat: Carpetas folder tree with filter and keyboard navigation`

---

### Task 8: Web — narrow screens: iPod-style lists

**Model:** Sonnet.

**Files:**
- Create: `web/src/components/ListaCarpeta.svelte`
- Modify: `web/src/pages/Carpetas.svelte`

**Interfaces:**
- Consumes: `tree`, `loadLevel`, `revealPath` (`carpetas.svelte.js`); `direction`, `matchRows`, `shortPath` (`tree.js`); `entries`, `playableIds` (`folders.js`); `size`, `duration`, `folderName`; `api.folder`, `api.folderTreeSearch`, `api.setCollection`; `play`, `enqueue`; `navigate`, `savedScroll`.
- Produces: `<ListaCarpeta path rootPath folder />` — `folder` is the page's `/api/folders` response for `path` (the page already loads it).

**Design:**
- `Carpetas.svelte`: `narrow` from `matchMedia('(max-width: 720px)')` (a `$state` updated on `change`; cleaned up on destroy). Narrow → render the lists instead of the two panels. Transition: keep the previous path in a variable; `dir = direction(previous, path)`; wrap the list in `{#key path}` inside a `.stack { display: grid }` container whose children share `grid-area: 1 / 1`; `in:fly={{ x: dir === 'back' ? '-30%' : '100%', duration }}` and `out:fly={{ x: dir === 'back' ? '100%' : '-30%', duration }}`, `duration = reducedMotion ? 0 : 220` (`matchMedia('(prefers-reduced-motion: reduce)')`); `overflow-x: hidden` on the stack. After the folder loads on a history back/forward (`popstate`), restore `window.scrollTo(0, savedScroll())` as the current page does.
- `ListaCarpeta.svelte`:
  - Loads `loadLevel(path)` (the merged subfolders of this folder) unless `tree.levels[path]` is loaded, and `revealPath(path)` to find the parent screen: `back = chain.at(-1) ?? (path === rootPath ? null : rootPath)`; the back label is that folder's name (`folderName(back)`).
  - Header (sticky): `‹ {backName}` link to `folderHref(back)` (absent at the root), the folder name centered (ellipsis), and a `⋯` button (Icono `more`, `aria-label="Acciones"`, `aria-haspopup="menu"`) opening a small menu (`role="menu"`, items `role="menuitem"`): Reproducir (`play({ folder: path })`), Aleatorio (`play({ folder: path }, { shuffle: true })`), A la cola (`enqueue({ folder: path })`), and No es una colección / Es una colección when `folder.detected && !app.status?.readOnly`. Close on choice, on `Escape` and on a click outside.
  - Rows (one line each, ≥ 44 px tall, a single number on the right, no columns or bars): first `Ver álbum` (link to `albumHref`) for an album folder; then every row of `tree.levels[path]`: icon (`disc`/`folder`) + `label` + tracks + `›` (Icono `chevron_right`), a link to `folderHref(node.path)`; then `entries(folder).files`: audio → name + `duration(durationMs)`, tapping plays `play({ tracks: playableIds(files) }, { startId: id })`, dimmed when not playable; others dimmed with `size(sizeBytes)`.
  - At the root screen (`path === rootPath` or `''`): the `Filtrar carpetas…` input above the list (debounced 200 ms, `MIN` letters); with a result, the list shows `matchRows(result)` instead: icon + label + on a second muted line `shortPath(parent || node.path, rootPath)`, linking to `folderHref(node.path)`; the same "más coincidencias" / "ninguna" lines as the tree.
- [ ] **Step 1: Implement** both files.
- [ ] **Step 2: Run** `cd web && npx vitest run && npm run build`.
- [ ] **Step 3: Smoke-check** with the Browser pane at phone width (`resize_window` preset `mobile`, reload): enter two levels, go back with `navigate('back')`, check the list returns to its scroll position, open ⋯; then reset with preset `desktop`. Report what was checked.
- [ ] **Step 4: Commit** `feat: Carpetas as iPod-style lists on narrow screens`

---

### Task 9: Check against the real collection, README

**Model:** the controller (Opus) runs it, with the user; not a subagent.

**Files:**
- Modify: `README.md` (the paragraph on Carpetas, line ~45: describe the explorer: tree with merged chains, filter, `/` and arrow keys, detail list, phone lists)
- Create: `docs/superpowers/notes/2026-10-02-carpetas-collection-check.md` (findings)

- [ ] **Step 1:** Build (`cd web && npm run build`), run `go run ./cmd/tordo -dir .run` against the real catalog, and check in the Browser pane every point of spec §5 "En el navegador": the root does not mix `Soulseek` with one-track folders as cards (it is a tree row with its count); `Soulseek › complete` is one row; *História da MPB* (49) and *Zitarrosa* (46) are navigable and sortable; the filter finds a name typed without its accent; the tree works with the keyboard alone (`/`, ↑↓←→, Enter); phone width: lists, back, scroll position. Time `GET /api/folders/tree`, `/api/folders/tree/search?q=zitarrosa` and `/api/folders?path=<Soulseek>` (the Network panel or `curl -w '%{time_total}'`); if a tree request is over 50 ms, note it (and cache the loaded tree by `Store.Changes()` in a follow-up).
- [ ] **Step 2:** Write the notes file (what was checked, timings, issues found) and fix small issues in place; larger ones become follow-up tasks.
- [ ] **Step 3:** `go test ./... && go vet ./... && (cd web && npx vitest run && npm run build)`; commit `docs: Carpetas explorer checked against the collection`.
