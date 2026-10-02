# M4a: follow-ups left after the reviews

Deferred by the task reviews and the final review (Opus) of branch `m4-escucha` as "can wait". None blocks merging; pick them up in M4b or after the collection check (plan Task 15).

## Worth doing in M4b

- **Album marks spill over shared fingerprints.** DONE: An album counts as liked/rated only when more than half its present tracks carry the album mark (a compilation holding one shared copy shows as not liked).
- **Failed loudness refs are re-prioritized on every play.** DONE: "`unmeasurable`" tracks (files that cannot be opened or ffmpeg I/O errors) are not re-analyzed.
- **`PlayTracks` runs one query per id.** DONE: Batched with `IN (…)`.
- **`id:N` refs.** DONE: Re-keyed to the fingerprint when it is read.
- **Loudness errors.** DONE: A file that exists but cannot be opened, or an ffmpeg I/O error, is marked "unmeasurable" (not prioritized again); only missing files are skipped without a note.
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
