<script>
  import { itemCoverURL } from '../lib/api.js'
  import { count, duration } from '../lib/format.js'
  import { RowDrag } from '../lib/drag.svelte.js'
  import { clearUpcoming, jumpTo, moveTo, player, removeAt, stopStation } from '../lib/player.svelte.js'
  import * as Q from '../lib/queue.js'
  import { summary } from '../lib/stations.js'
  import EstacionAjustes from './EstacionAjustes.svelte'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  let { onclose } = $props()

  let adjusting = $state(false)
  const station = $derived(player.queue.station)
  const SHOWN = 200 // rows drawn; the rest are summed up
  const now = $derived(Q.current(player.queue))
  const next = $derived(Q.upcoming(player.queue))

  // Upcoming rows reorder by dragging their handle or with Alt+arrows.
  let list = $state()
  const rows = new RowDrag({ rows: () => [...list.children], onmove: (from, to) => moveTo(next[from].pos, next[to].pos) })

  // When the track changes the rows shift under a drag: drop it.
  $effect(() => {
    now?.key
    rows.cancel()
  })

  function onkey(event, i) {
    const to = rows.key(event, i, Math.min(next.length, SHOWN))
    if (to < 0) return
    // The row moves: keep the focus on its handle.
    const key = next[to].entry.key
    queueMicrotask(() => list?.querySelector(`[data-key="${key}"] .grip`)?.focus())
  }
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
    {#if station}
      <section class="station">
        <p class="name"><Icono name="radio" size={16} /> <b>Estación · {station.name}</b></p>
        <p class="rules">{summary(station)}</p>
        <div class="stbuttons">
          <button onclick={(e) => (e.stopPropagation(), (adjusting = !adjusting))} aria-expanded={adjusting}>Ajustar</button>
          <button onclick={stopStation}>Detener</button>
        </div>
        {#if adjusting}<EstacionAjustes {station} onclose={() => (adjusting = false)} />{/if}
      </section>
    {/if}
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
      <div bind:this={list}>
      {#each next.slice(0, SHOWN) as u, i (u.entry.key)}
        <div class="row" class:dragging={rows.drag?.from === i} data-key={u.entry.key}
          style:transform={rows.transform(i)} style:transition={rows.transition(i)}>
          <button class="icon grip" aria-label="Mover {u.entry.title} (Alt + flechas)" title="Arrastrar para reordenar"
            onpointerdown={(ev) => rows.down(ev, i)} onpointermove={rows.move} onpointerup={rows.up} onpointercancel={rows.cancel}
            onkeydown={(ev) => onkey(ev, i)}><Icono name="grip" size={16} /></button>
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
      </div>
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
    --grip-w: 32px;
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
  .station {
    margin: 4px 0 8px;
    padding: 10px 8px;
    border-bottom: 1px solid var(--line);
  }
  .station p {
    margin: 0;
  }
  .name {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--accent);
  }
  .name b {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--strong);
  }
  .rules {
    font-size: 13px;
    color: var(--muted);
    margin: 2px 0 8px;
  }
  .stbuttons {
    display: flex;
    gap: 8px;
  }
  .row {
    position: relative;
    display: flex;
    align-items: center;
    border-radius: var(--radius);
  }
  .row.dragging {
    z-index: 5;
    background: var(--surface-2);
    box-shadow: 0 4px 16px rgb(0 0 0 / 0.5);
  }
  .grip {
    flex: none;
    width: var(--grip-w);
    cursor: grab;
    touch-action: none; /* a touch on the handle drags instead of scrolling */
    color: var(--faint);
  }
  .dragging .grip {
    cursor: grabbing;
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
    margin-left: var(--grip-w);
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
