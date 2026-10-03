<script>
  import { untrack } from 'svelte'
  // A playlist: its entries in order, which can be played, reordered by
  // dragging (pointer events: mouse and touch) or with Alt+arrows, and removed.
  import Acciones from '../components/Acciones.svelte'
  import Icono from '../components/Icono.svelte'
  import Tapa from '../components/Tapa.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { count, duration, length } from '../lib/format.js'
  import { RowDrag } from '../lib/drag.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { move } from '../lib/order.js'
  import { play } from '../lib/player.svelte.js'
  import { albumHref, artistHref } from '../lib/router.js'
  import { playlistStation } from '../lib/stations.js'

  let { id } = $props()

  let list = $state(null)
  let error = $state('')
  let renaming = $state(false)
  let draft = $state('')
  let ol = $state()
  const rows = new RowDrag({ rows: () => [...ol.children], onmove: reorder, onend: () => settle() })

  const writable = $derived(!app.status?.readOnly)

  // While a drag or a save is going on, entries must not be replaced: a
  // read that arrives then is deferred (stale) and done when things settle.
  let saving = false
  let dirty = false // a newer order waits to be sent
  let failed = false
  let stale = false

  async function load() {
    try {
      list = await api.playlist(id)
      error = ''
    } catch (e) {
      list = null
      error = e.status === 404 ? 'Esta lista ya no existe.' : e.message
    }
  }

  // refresh reads the list again, or later if it is being edited.
  function refresh() {
    if (rows.drag || saving) stale = true
    else load()
  }

  // settle runs the deferred read once the drag and the saves are over.
  function settle() {
    if (rows.drag || saving || !(stale || failed)) return
    stale = failed = false
    load()
  }

  $effect(() => {
    app.generation
    untrack(refresh) // not tracking drag and the like
  })

  const summary = $derived(list ? `${count(list.tracks, 'pista', 'pistas')} · ${length(list.durationMs)}` : '')

  function startRename() {
    draft = list.name
    renaming = true
  }

  async function saveRename() {
    const name = draft.trim()
    renaming = false
    if (!name || name === list.name) return
    try {
      await api.renamePlaylist(id, name)
      list.name = name
    } catch (e) {
      notify(e.message)
    }
  }

  function onrenamekey(event) {
    if (event.key === 'Enter') saveRename()
    else if (event.key === 'Escape') renaming = false
  }

  async function remove() {
    if (!confirm(`¿Eliminar la lista «${list.name}»? Las pistas no se tocan.`)) return
    try {
      await api.deletePlaylist(id)
      navigate('/listas')
    } catch (e) {
      notify(e.message)
    }
  }

  async function removeEntry(entry) {
    try {
      await api.removeFromPlaylist(id, entry.itemId)
      refresh()
    } catch (e) {
      notify(e.message)
    }
  }

  // reorder applies the new order at once; saves go one at a time, always
  // with the latest full order. If the server refuses, the list is read again
  // when the queue is empty.
  function reorder(from, to) {
    if (from === to) return
    list.entries = move(list.entries, from, to)
    dirty = true
    flush()
  }

  async function flush() {
    if (saving) return
    saving = true
    while (dirty) {
      dirty = false
      try {
        await api.reorderPlaylist(id, list.entries.map((e) => e.itemId))
      } catch (e) {
        notify(e.message)
        failed = true
        dirty = false
      }
    }
    saving = false
    settle()
  }

  function onkey(event, i) {
    const to = rows.key(event, i, list.entries.length)
    if (to < 0) return
    // The row moves: keep the focus on its handle.
    const itemId = list.entries[to].itemId
    queueMicrotask(() => ol?.querySelector(`[data-item="${itemId}"] .grip`)?.focus())
  }

  // Focuses an input that appears after a click (autofocus does not fire then).
  const focus = (el) => el.focus()
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if list}
  <div class="head">
    <div class="pic">
      <Tapa srcs={list.covers.map((c) => albumCoverURL(c, 320))} icon="lists" alt={list.name} />
    </div>
    <div class="info">
      <span class="label">Lista</span>
      {#if renaming}
        <input class="rename" bind:value={draft} onkeydown={onrenamekey} onblur={() => (renaming = false)} maxlength="200"
          aria-label="Nombre de la lista" use:focus />
      {:else}
        <h1>{list.name}</h1>
      {/if}
      <p class="sub">{summary}</p>
      <Acciones what={{ playlist: list.id }} station={playlistStation(list)} />
      {#if writable}
        <div class="manage">
          <button onclick={startRename}>Renombrar</button>
          <button onclick={remove}><Icono name="trash" size={14} /> Eliminar lista</button>
        </div>
      {/if}
    </div>
  </div>

  {#if list.entries.length === 0}
    <p class="empty">La lista está vacía. Agregale pistas con «+ Lista» en un álbum, una carpeta o una pista.</p>
  {:else}
    <ol class="entries" bind:this={ol}>
      {#each list.entries as e, i (e.itemId)}
        <li
          class="row"
          class:unplayable={!e.trackId || e.playable === false}
          class:dragging={rows.drag?.from === i}
          data-item={e.itemId}
          style:transform={rows.transform(i)}
          style:transition={rows.transition(i)}>
          {#if e.trackId && e.playable !== false}
            <button class="play" onclick={() => play({ playlist: list.id }, { startId: e.trackId })} aria-label="Reproducir {e.title}"></button>
          {/if}
          {#if writable}
            <button class="icon grip" aria-label="Mover {e.title} (Alt + flechas)" title="Arrastrar para reordenar"
              onpointerdown={(ev) => rows.down(ev, i)} onpointermove={rows.move} onpointerup={rows.up} onpointercancel={rows.cancel}
              onkeydown={(ev) => onkey(ev, i)}><Icono name="grip" size={16} /></button>
          {/if}
          <span class="n">{i + 1}</span>
          <span class="text">
            <span class="title">{e.title}{#if !e.trackId} <span class="na">no disponible</span>{/if}</span>
            <span class="artist">
              {#if e.artist}{#if e.artistKey}<a href={artistHref(e.artistKey)}>{e.artist}</a>{:else}{e.artist}{/if}{/if}
              {#if e.artist && e.album && e.albumId} · {/if}
              {#if e.album && e.albumId}<a href={albumHref(e.albumId)}>{e.album}</a>{:else if e.album}{e.album}{/if}
            </span>
          </span>
          <span class="time">{e.trackId ? duration(e.durationMs) : ''}</span>
          {#if writable}
            <button class="icon rm" onclick={() => removeEntry(e)} title="Quitar de la lista" aria-label="Quitar {e.title} de la lista">
              <Icono name="close" size={16} />
            </button>
          {/if}
        </li>
      {/each}
    </ol>
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
    width: 240px;
    flex: none;
  }
  .info {
    min-width: 0;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0 0 6px;
  }
  .rename {
    font-size: 24px;
    font-weight: 700;
    width: min(100%, 480px);
  }
  .manage {
    display: flex;
    gap: 8px;
    margin-top: 10px;
  }
  .manage button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
  }
  .entries {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .row {
    position: relative;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 0 0 8px;
    border-bottom: 1px solid var(--line);
    background: var(--bg);
  }
  .row:hover {
    background: var(--surface);
  }
  .row.dragging {
    z-index: 5;
    background: var(--surface-2);
    box-shadow: 0 4px 16px rgb(0 0 0 / 0.5);
  }
  .play {
    position: absolute;
    inset: 0;
    border: none;
    border-radius: 0;
    padding: 0;
  }
  .row > :not(.play) {
    position: relative;
    pointer-events: none;
  }
  .row a,
  .row .icon {
    pointer-events: auto;
  }
  .grip {
    cursor: grab;
    touch-action: none; /* a touch on the handle drags instead of scrolling */
    color: var(--faint);
  }
  .dragging .grip {
    cursor: grabbing;
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
    padding: 8px 0;
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
  .artist a:hover {
    color: var(--text);
    text-decoration: underline;
  }
  .unplayable {
    opacity: 0.55;
  }
  .na {
    font-size: 12px;
    color: var(--faint);
  }
  .time {
    font-size: 13px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    min-width: 40px;
    text-align: right;
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
