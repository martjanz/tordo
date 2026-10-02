# Carpetas explorer — check against the collection (2026-10-02)

Catalog: `/Users/martin/Music` (460 albums, 6.685 tracks, 55 GB), branch `carpetas-explorador`, built web app, browser pane.

## Timings (local, warm)

| Request | Time |
|---|---|
| `GET /api/folders/tree` (top level) | 19 ms |
| `GET /api/folders/tree?path=…/Soulseek/complete` (33 rows) | 7 ms |
| `GET /api/folders?path=…/Soulseek/complete` (sizes of 33 subfolders) | 18 ms |
| `GET /api/folders/tree/search?q=zitarrosa` (22 matches) | 11 ms |

No caching needed (the plan's threshold was 50 ms).

## Checked

- The top level is a tree of 13 rows with track counts; `Soulseek › complete` (3.884) is one row, `USA › Anthology of American Folk Music, Volumes 1-3` too. The detail list shows `Soulseek` (30 GB) with a long bar next to one-track folders.
- *VA - História da MPB* (49 subfolders) and *Alfredo Zitarrosa* are navigable; the content follows the disk's natural order.
- Filter: `historia da mpb` (no accent, NFD folder name) finds it; `/` focuses the filter; ↓ moves into the tree, Enter shows the folder, the URL follows.
- Phone width (375 px): one list per folder, filter on the root screen, `Ver álbum` row on album folders, back with history restores the scroll.

## Issues

- Long audio file names in the wide detail list render as "…" only (the name button with `all: unset` inside an ellipsized cell). Sent to the final review fix.
