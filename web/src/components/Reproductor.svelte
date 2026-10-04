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
  } from '../lib/playback.svelte.js'
  import * as Q from '../lib/queue.js'
  import { app } from '../lib/app.svelte.js'
  import { albumHref } from '../lib/router.js'
  import AgregarALista from './AgregarALista.svelte'
  import Cola from './Cola.svelte'
  import Icono from './Icono.svelte'
  import MeGusta from './MeGusta.svelte'
  import Tapa from './Tapa.svelte'

  const entry = $derived(Q.current(player.queue))
  const pct = $derived(player.duration > 0 ? Math.min(100, (player.time / player.duration) * 100) : 0)
  const repeatTitle = { off: 'Repetir: no', all: 'Repetir: toda la cola', one: 'Repetir: esta pista' }

  let sheet = $state(false) // the full player, on phones
  let queueOpen = $state(false)
  let adding = $state(null) // where the list picker is open: 'bar' or 'sheet'

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

{#snippet addToList(where, size)}
  {#if !app.status?.readOnly}
    <span class="add">
      <button class="icon" onclick={(e) => (e.stopPropagation(), (adding = adding === where ? null : where))} aria-haspopup="dialog"
        aria-expanded={adding === where} title="Agregar a una lista" aria-label="Agregar a una lista"><Icono name="lists" {size} /></button>
      {#if adding === where}<AgregarALista up right={where === 'sheet'} what={{ tracks: [entry.id] }} onclose={() => (adding = null)} />{/if}
    </span>
  {/if}
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
      <span class="like"><MeGusta kind="track" ref={entry.id} size={18} /></span>
      <span class="like bar-add">{@render addToList('bar', 18)}</span>
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
        <div class="big-title">
          <b>{entry.title}</b>
          <span class="big-marks">{@render addToList('sheet', 24)}<MeGusta kind="track" ref={entry.id} size={24} /></span>
        </div>
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
  .big-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .like {
    position: relative;
    z-index: 1;
    flex: none;
  }
  .add {
    position: relative;
    display: inline-flex;
  }
  .big-marks {
    display: flex;
    align-items: center;
    gap: 4px;
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
    .bar-add {
      display: none; /* in the sheet instead */
    }
    .cover-all {
      display: block;
      position: absolute;
      inset: 0;
    }
  }
</style>
