# M4a: follow-ups left after the reviews

Deferred by the task reviews and the final review (Opus) of branch `m4-escucha` as "can wait". None blocks merging; pick them up in M4b or after the collection check (plan Task 15).

## Worth doing in M4b

- **Album marks spill over shared fingerprints.** An album counts as liked/rated if any present track carries an album mark, so a compilation holding a bit-identical copy of one track of a liked album shows as liked (plan decision 1). Consider requiring most of the album's tracks.
- **Failed loudness refs are re-prioritized on every play** (and `missing()` keeps asking for albums that never get an album gain): each costs a runner pass with three full-table queries. Let `LoudnessFacts` say "failed", or skip refs that already have a `loudness` row.
- **`PlayTracks` runs one query per id**: `?liked=1` and `PlayPlaylist` on 5000 entries means 5000 queries. Batch with `IN (…)`.
- **`id:N` refs**: a like, play or playlist add made while a re-tagged file has no fingerprint yet is keyed by `id:N` and orphaned once the fingerprint is read.
- **Loudness errors**: a file that exists but cannot be opened, or an ffmpeg I/O failure that still prints output, is saved as a permanent error (only missing files and ffmpeg not starting are skipped).
- **`escuchado` sort** copies the plays map and does map lookups per comparison: measure at full scale in Task 15.

## Web polish

- `MenuPista` declares `role="menu"` without focus management or arrow keys (and holds a radiogroup); `Puntaje` arrow keys don't move focus; one Escape listener per row.
- `Lista.svelte`: a `load()` already in flight when a drag starts can still replace the entries; `saveRename`/delete bypass the deferred refresh; rename draft lost on blur; no autoscroll while dragging; drag index logic untested; real touch input untested.
- `Listas.svelte` effects lack stale guards; tabs lack `aria-controls`.
- `mine.svelte.js` rollback snapshot can undo an overlapping later change on error; `withLike` sets no `likedAt` until the next load.
- Changing `normalize` needs a restart (Ajustes is M5).

## Server and store polish

- `/api/play?album=<unknown>` is 404 but an unknown folder is 200 with no items; several sources in one `what` silently pick one; create-with-add is not atomic (compensating delete); the album existence check loads the whole album.
- Missing `liked`/`stars` in a body silently mean unlike/clear; `readBody` not reused by older endpoints.
- Playlist reads are not in one transaction; `RenamePlaylist` doesn't reuse `touch`.
- Tests not written: `Playlists()` order, 200-rune name cut, cover cap, mid-rune device cut, combined `megusta`+`puntaje` counts, rating 0 over HTTP, bad path ids, cmd/tordo loudness wiring.

## Merge note

The collection ♥ was added to the **current** Carpetas header (`web/src/pages/Carpetas.svelte`). The Carpetas explorer rewrite (branch `carpetas-explorador`) replaces that header: whichever branch merges second must keep `<MeGusta kind="collection" ref={path} />` in the new header, shown when the folder is a collection.
