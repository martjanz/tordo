<script>
  // The persistent player bar. On phones it is a mini player above the
  // tabs that opens into a full-screen sheet.
  import { itemCoverURL } from '../lib/api.js'
  import { duration, quality } from '../lib/format.js'
  import {
    can,
    cycleRepeat,
    jumpTo,
    next,
    prev,
    remote,
    shown,
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

  const p = $derived(shown())
  const entry = $derived(Q.current(p.queue))
  const upcoming = $derived(sheet ? Q.upcoming(p.queue) : [])
  const pct = $derived(p.duration > 0 ? Math.min(100, (p.time / p.duration) * 100) : 0)
  // In Rockola mode a slider sends once, when released, not on every move.
  const live = (fn) => (e) => fn(Number(e.currentTarget.value))
  const repeatTitle = { off: 'Repetir: no', all: 'Repetir: toda la cola', one: 'Repetir: esta pista' }

  let sheet = $state(false) // the full player, on phones
  let queueOpen = $state(false)
  let adding = $state(null) // where the list picker is open: 'bar' or 'sheet'

  // Space plays and pauses, unless typing or on a button.
  function onkeydown(event) {
    if (event.key === 'Escape' && sheet && !adding) {
      sheet = false
      return
    }
    if (event.key !== ' '|| event.ctrlKey || event.metaKey || event.altKey) return
    const tag = event.target?.tagName
    if (['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON', 'A'].includes(tag) || event.target?.isContentEditable) return
    event.preventDefault()
    toggle()
  }
</script>

<svelte:window {onkeydown} />

{#snippet controls(big)}
  <div class="controls" class:big>
    {#if can('toggleShuffle')}
      <button class="icon" class:on={p.queue.shuffle} onclick={toggleShuffle} title="Aleatorio" aria-label="Aleatorio"
        aria-pressed={p.queue.shuffle}><Icono name="shuffle" size={big ? 22 : 18} /></button>
    {/if}
    {#if can('prev')}
      <button class="icon" onclick={prev} title="Anterior" aria-label="Anterior"><Icono name="prev" size={big ? 28 : 20} /></button>
    {/if}
    <button class="icon playpause" onclick={toggle} title={p.playing ? 'Pausa' : 'Reproducir'}
      aria-label={p.playing ? 'Pausa' : 'Reproducir'} disabled={!entry}>
      <Icono name={p.playing ? 'pause' : 'play'} size={big ? 30 : 20} />
    </button>
    {#if can('next')}
      <button class="icon" onclick={next} title="Siguiente" aria-label="Siguiente"><Icono name="next" size={big ? 28 : 20} /></button>
    {/if}
    {#if can('cycleRepeat')}
      <button class="icon" class:on={p.queue.repeat !== 'off'} onclick={cycleRepeat} title={repeatTitle[p.queue.repeat]}
        aria-label={repeatTitle[p.queue.repeat]}>
        <Icono name="repeat" size={big ? 22 : 18} />
        {#if p.queue.repeat === 'one'}<span class="one">1</span>{/if}
      </button>
    {/if}
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
    <span class="time">{duration(p.time * 1000)}</span>
    <input type="range" min="0" max={p.duration || 0} step="1" value={p.time} disabled={!p.seekable || !entry || !can('seek')}
      oninput={remote() ? undefined : live(seek)} onchange={remote() ? live(seek) : undefined} aria-label="Posición"
      title={p.seekable ? '' : 'Este formato se convierte al vuelo: no se puede adelantar'}
      style="--pct: {pct}%" />
    <span class="time">{duration(p.duration * 1000)}</span>
  </div>
{/snippet}

{#if !entry && remote()}
  <div class="bar idle">
    <div class="text">
      <span class="rockola-tag">Rockola · en la compu</span>
      {#if p.blocked}<span class="blocked">La compu necesita un clic para sonar</span>{/if}
      <span class="artist">La cola de la compu está vacía.</span>
    </div>
    <div class="right">
      <button class="icon" class:on={queueOpen} onclick={() => (queueOpen = !queueOpen)} title="Cola" aria-label="Cola">
        <Icono name="queue" />
      </button>
    </div>
  </div>
{:else if entry}
  <div class="bar">
    <div class="line" style="width: {pct}%"></div>
    <div class="now">
      <button class="open" onclick={() => (sheet = true)} aria-label="Abrir el reproductor">
        <span class="pic"><Tapa srcs={[itemCoverURL(entry, 160)]} /></span>
      </button>
      <span class="text">
        {#if remote()}<span class="rockola-tag">Rockola · en la compu</span>{/if}
        {#if remote() && p.blocked}<span class="blocked">La compu necesita un clic para sonar</span>{/if}
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
      {#if can('setVolume')}
        <button class="icon" onclick={toggleMute} title={p.muted ? 'Activar sonido' : 'Silenciar'}
          aria-label={p.muted ? 'Activar sonido' : 'Silenciar'}><Icono name={p.muted ? 'mute' : 'volume'} /></button>
        <input class="volume" type="range" min="0" max="1" step="0.01" value={p.muted ? 0 : p.volume}
          oninput={remote() ? undefined : live(setVolume)} onchange={remote() ? live(setVolume) : undefined} aria-label="Volumen"
          style="--pct: {(p.muted ? 0 : p.volume) * 100}%" />
      {/if}
      <button class="icon" class:on={queueOpen} onclick={() => (queueOpen = !queueOpen)} title="Cola" aria-label="Cola">
        <Icono name="queue" />
      </button>
    </div>
    <div class="mini">
      <button class="icon" onclick={toggle} aria-label={p.playing ? 'Pausa' : 'Reproducir'}>
        <Icono name={p.playing ? 'pause' : 'play'} size={22} />
      </button>
      {#if can('next')}
        <button class="icon" onclick={next} aria-label="Siguiente"><Icono name="next" size={22} /></button>
      {/if}
    </div>
  </div>

  {#if sheet}
    <div class="sheet" role="dialog" aria-label="Reproductor">
      <div class="backdrop" aria-hidden="true">
        {#key entry.key}<img src={itemCoverURL(entry, 160)} alt="" onerror={(e) => e.currentTarget.remove()} />{/key}
      </div>
      <header>
        <button class="icon" onclick={() => (sheet = false)} title="Cerrar (Esc)" aria-label="Cerrar"><Icono name="down" size={26} /></button>
        <span class="label">{remote() ? 'Rockola · en la compu' : 'Sonando'}</span>
        <button class="icon" onclick={() => ((queueOpen = true), (sheet = false))} title="Cola" aria-label="Cola"><Icono name="queue" size={22} /></button>
      </header>
      <div class="stage">
        {#key entry.key}
          <div class="big-pic"><Tapa srcs={[itemCoverURL(entry, 640)]} /></div>
        {/key}
        <div class="info">
          <div class="big-text">
            {#key entry.key}
              <h2 class="big-title" class:long={entry.title.length > 40}>{entry.title}</h2>
            {/key}
            <p class="big-artist">{entry.artist}</p>
            <p class="big-album">
              {#if entry.albumId}
                <a href={albumHref(entry.albumId)} onclick={() => (sheet = false)}>{entry.album}</a>
              {:else if entry.album}
                <span>{entry.album}</span>
              {/if}
              {#if entry.codec || entry.format}<span class="badge" class:lossless={entry.lossless}>{quality(entry)}</span>{/if}
            </p>
            <span class="big-marks">
              <MeGusta kind="track" ref={entry.id} size={24} />{@render addToList('sheet', 24)}
            </span>
          </div>
          <div class="deck">
            {@render progress()}
            {@render controls(true)}
          </div>
          {#if upcoming.length > 0}
            <div class="upnext">
              <p class="label">A continuación</p>
              <!-- A fourth, fading out, hints that the queue goes on (the
                   third, on short screens, where the fourth is hidden). -->
              {#each upcoming.slice(0, 4) as u, i (u.entry.key)}
                <button class="up" class:tail={i === 3} class:tail-short={i === 2 && upcoming.length > 3}
                  onclick={() => jumpTo(u.pos)} disabled={!can('jumpTo')}>
                  <span class="up-pic"><Tapa srcs={[itemCoverURL(u.entry, 160)]} /></span>
                  <span class="up-text"><b>{u.entry.title}</b><span>{u.entry.artist}</span></span>
                  <span class="time">{duration(u.entry.durationMs)}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>
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
  .rockola-tag {
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--accent);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .blocked {
    font-size: 11px;
    color: var(--warn);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .idle {
    grid-template-columns: minmax(0, 1fr) auto;
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
  /* The full player: a "now playing" screen to leave open. The cover,
     blurred, tints the whole screen; on wide screens the details sit
     beside the cover in big type. */
  .sheet {
    position: fixed;
    inset: 0;
    z-index: 45;
    background: var(--bg);
    display: flex;
    flex-direction: column;
    padding: 12px 24px calc(24px + env(safe-area-inset-bottom));
    overflow: hidden auto;
    isolation: isolate;
  }
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: -1;
    overflow: hidden;
  }
  .backdrop img {
    position: absolute;
    inset: -15%;
    width: 130%;
    height: 130%;
    object-fit: cover;
    filter: blur(70px) saturate(1.4);
    opacity: 0.55;
    animation: fade 1.2s ease;
  }
  .backdrop::after {
    content: '';
    position: absolute;
    inset: 0;
    background:
      radial-gradient(ellipse at 30% 40%, transparent 0%, rgb(0 0 0 / 0.35) 70%),
      linear-gradient(to bottom, rgb(21 23 27 / 0.2), rgb(21 23 27 / 0.85));
  }
  .sheet header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .sheet header .label {
    letter-spacing: 0.18em;
  }
  .stage {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 20px;
    padding-top: 12px;
  }
  .big-pic {
    width: min(100%, 52vh);
    align-self: center;
    animation: rise 0.5s ease;
  }
  .big-pic :global(.tapa) {
    border-radius: 8px;
    box-shadow: 0 24px 64px rgb(0 0 0 / 0.55), 0 2px 8px rgb(0 0 0 / 0.4);
  }
  .info {
    display: flex;
    flex-direction: column;
    gap: 20px;
    min-width: 0;
  }
  .big-text {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    padding-right: 80px; /* the marks */
  }
  .big-title {
    margin: 0;
    font-size: clamp(24px, 3.6vw + 8px, 64px);
    font-weight: 700;
    /* Tall enough to hold the descenders: the clamp's overflow: hidden cuts
       whatever hangs below the last line box (the tail of a g, y, p). */
    line-height: 1.2;
    letter-spacing: -0.025em;
    text-transform: none;
    color: var(--strong);
    text-wrap: balance;
    overflow-wrap: anywhere;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
    animation: rise 0.45s ease;
  }
  .big-title.long {
    font-size: clamp(22px, 2.6vw + 8px, 46px);
  }
  .big-artist {
    margin: 4px 0 0;
    font-size: clamp(17px, 1.2vw + 10px, 28px);
    font-weight: 500;
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .big-album {
    margin: 0;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 12px;
    font-size: clamp(14px, 0.5vw + 11px, 18px);
    color: var(--muted);
  }
  .big-album a:hover {
    text-decoration: underline;
  }
  .badge {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    padding: 2px 7px;
    border: 1px solid var(--line-strong);
    border-radius: 3px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
  }
  .badge.lossless {
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 45%, transparent);
  }
  .big-marks {
    position: absolute;
    top: 0;
    right: 0;
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .deck {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .upnext {
    display: none;
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
  }
  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .backdrop img,
    .big-pic,
    .big-title {
      animation: none;
    }
  }
  /* Wide screens: cover and details side by side. */
  @media (min-width: 900px) and (min-height: 520px) {
    .sheet {
      padding: 16px clamp(24px, 5vw, 80px) 32px;
    }
    .stage {
      flex-direction: row;
      align-items: center;
      justify-content: center;
      gap: clamp(32px, 5vw, 88px);
      padding: 0;
    }
    .big-pic {
      flex: none;
      width: min(44vw, 72vh, 640px);
      align-self: center;
    }
    .info {
      flex: 1;
      max-width: 680px;
      gap: clamp(24px, 4vh, 44px);
    }
    .sheet .deck .controls {
      justify-content: flex-start;
      gap: 20px;
    }
    .upnext {
      display: flex;
      flex-direction: column;
      gap: 2px;
      border-top: 1px solid rgb(255 255 255 / 0.08);
      padding-top: 12px;
    }
    .upnext .label {
      margin: 0 0 6px;
    }
    .up {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 6px 8px;
      margin: 0 -8px;
      border: none;
      border-radius: var(--radius);
      text-align: left;
      opacity: 0.8;
    }
    .up:hover:not(:disabled) {
      background: rgb(255 255 255 / 0.06);
      opacity: 1;
    }
    .up:disabled {
      opacity: 0.8;
    }
    .up-pic {
      width: 36px;
      flex: none;
    }
    .up-text {
      flex: 1;
      min-width: 0;
      display: flex;
      flex-direction: column;
    }
    .up-text b,
    .up-text span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .up-text b {
      font-weight: 500;
      color: var(--strong);
    }
    .up-text span {
      font-size: 13px;
      color: var(--muted);
    }
  }
  .up.tail,
  .up.tail-short {
    --fade: linear-gradient(to bottom, rgb(0 0 0 / 0.45), transparent 95%);
  }
  .up.tail {
    -webkit-mask-image: var(--fade);
    mask-image: var(--fade);
  }
  .up.tail:hover:not(:disabled) {
    -webkit-mask-image: none;
    mask-image: none;
  }
  @media (max-height: 760px) and (min-width: 900px) {
    .upnext .up.tail {
      display: none;
    }
    .up.tail-short {
      -webkit-mask-image: var(--fade);
      mask-image: var(--fade);
    }
    .up.tail-short:hover:not(:disabled) {
      -webkit-mask-image: none;
      mask-image: none;
    }
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
