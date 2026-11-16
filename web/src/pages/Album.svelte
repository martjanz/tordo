<script>
  import Acciones from '../components/Acciones.svelte'
  import Preferido from '../components/Preferido.svelte'
  import Puntaje from '../components/Puntaje.svelte'
  import Identificar from '../components/Identificar.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import Tapa from '../components/Tapa.svelte'
  import { albumCoverURL, api, releaseURL } from '../lib/api.js'
  import { app, listenMode } from '../lib/app.svelte.js'
  import { albumMeta } from '../lib/album.js'
  import { count, folderName, formatName } from '../lib/format.js'
  import { play } from '../lib/playback.svelte.js'
  import { albumStation } from '../lib/stations.js'
  import { identification } from '../lib/review.js'
  import { albumHref, artistHref, duplicatesHref, folderHref } from '../lib/router.js'

  let { id } = $props()

  let album = $state(null)
  let error = $state('')
  let correcting = $state(false)

  $effect(() => {
    app.generation
    let stale = false
    api
      .album(id)
      .then((a) => {
        if (!stale) (album = a), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.status === 404 ? 'Este álbum ya no está en el catálogo.' : e.message
      })
    return () => (stale = true)
  })

  const meta = $derived(album ? albumMeta(album) : [])
  const identified = $derived(album && (album.status === 'auto' || album.status === 'manual') && album.releaseId)

  function corrected(a) {
    album = a
    correcting = false
  }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if album}
  <div class="head">
    {#if album.picture}
      <div class="glow" aria-hidden="true"><img src={albumCoverURL(album.id, 640)} alt="" /></div>
    {/if}
    <div class="pic"><Tapa srcs={album.picture ? [albumCoverURL(album.id, 640)] : []} alt={album.title} /></div>
    <div class="info">
      <span class="label">{album.compilation ? 'Compilado' : 'Álbum'}</span>
      <h1>{album.title}</h1>
      <p class="artist">
        {#if album.artistKey}<a href={artistHref(album.artistKey)}>{album.artist}</a>{:else}{album.artist}{/if}
      </p>
      <p class="meta">
        {#each meta as group, i (i)}{#if i}{' '}<span class="dot">·</span>{' '}{/if}{#each group as item, j (j)}{#if j}, {/if}{#if item.href}<a href={item.href}>{item.label}</a>{:else}{item.label}{/if}{/each}{/each}
      </p>
      <div class="bar">
        <Acciones what={{ album: album.id }} station={albumStation(album)} />
        <div class="marks">
          <Preferido ref={album.id} size={20} />
          <Puntaje kind="album" id={album.id} />
        </div>
      </div>
    </div>
  </div>
  <ListaPistas tracks={album.tracks} albumArtist={album.artist} discHeaders onplay={(t) => play({ album: album.id }, { startId: t.id })} />

  {#if album.versions?.length && !listenMode()}
    <section class="versions">
      <h2>Versiones</h2>
      <ul>
        {#each album.versions as v (v.albumId)}
          <li>
            <a class="vtitle" href={albumHref(v.albumId)}>{v.title}</a>
            <span>{v.artist}</span>
            {#if v.year}<span>{v.year}</span>{/if}
            <span>{v.formats.map(formatName).join(' · ')}</span>
            <span>{count(v.shared, 'tema', 'temas')} en común</span>
            {#if v.kind === 'exacta'}<span class="exact">copia idéntica</span>{/if}
          </li>
        {/each}
      </ul>
      <a class="sub" href={duplicatesHref(album.versions.some((v) => v.kind === 'exacta') ? 'exactas' : 'formatos')}>Ver duplicados →</a>
    </section>
  {/if}

  <section class="details">
    <h2>Detalles</h2>
    <dl>
      <dt>Carpeta</dt>
      <dd><a class="path" href={folderHref(album.dir)} title={album.dir}>{folderName(album.dir)}</a></dd>
      <dt>Identificación</dt>
      <dd>
        {identification(album)}
        {#if identified}· <a href={releaseURL(album.releaseId)} target="_blank" rel="noreferrer">ver en MusicBrainz ↗</a>{/if}
      </dd>
      {#if album.title !== album.tagTitle}
        <dt>En los tags</dt>
        <dd>«{album.tagTitle}»</dd>
      {/if}
    </dl>
    {#if !app.status?.readOnly && !listenMode()}
      {#if correcting}
        <Identificar {album} onchange={corrected} />
        <button onclick={() => (correcting = false)}>Cerrar</button>
      {:else}
        <button class="fix" onclick={() => (correcting = true)}>Corregir identificación</button>
      {/if}
    {/if}
  </section>
{/if}

<style>
  /* Its own stacking context, for the glow behind it, above the track list,
     for the "⋯" menu that opens over it. */
  .head {
    position: relative;
    z-index: 1;
    display: flex;
    gap: var(--space-6);
    align-items: flex-end;
    margin: var(--space-4) 0 var(--space-6);
  }
  /* The cover, blurred, tints the header: the Sonando view's glow, softer. */
  .glow {
    position: absolute;
    z-index: -1;
    inset: calc(-1 * var(--space-6)) calc(-1 * var(--gutter)) calc(-1 * var(--space-7));
    overflow: hidden;
    pointer-events: none;
    mask-image: radial-gradient(ellipse 70% 90% at 20% 40%, #000 30%, transparent 100%);
  }
  .glow img {
    position: absolute;
    left: 0;
    top: 0;
    width: 60%;
    height: 100%;
    object-fit: cover;
    filter: blur(80px) saturate(1.4);
    opacity: 0.35;
    animation: fade 1.2s ease;
  }
  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  .pic {
    width: 300px;
    flex: none;
    border-radius: var(--radius-lg);
    box-shadow: 0 16px 48px rgb(0 0 0 / 0.5);
  }
  .info {
    min-width: 0;
    flex: 1;
  }
  .info h1 {
    font-size: var(--fs-display);
    margin: var(--space-1) 0 var(--space-2);
    text-wrap: balance;
    overflow-wrap: anywhere;
  }
  .artist {
    font-size: var(--fs-xl);
    font-weight: 500;
    line-height: 1.3;
    color: var(--text);
    margin: 0 0 var(--space-3);
  }
  .meta {
    font-size: var(--fs-sm);
    color: var(--muted);
    margin: 0;
  }
  .artist a:hover,
  .meta a:hover,
  .details a:hover {
    text-decoration: underline;
  }
  .meta a {
    color: var(--text);
  }
  .dot {
    color: var(--faint);
  }
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2) var(--space-5);
    margin-top: var(--space-5);
  }
  .bar :global(.actions) {
    margin: 0;
  }
  .marks {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .sub {
    font-size: var(--fs-sm);
    color: var(--muted);
    margin: 0 0 2px;
  }
  .versions {
    margin-top: var(--space-6);
  }
  .versions ul {
    list-style: none;
    margin: 0 0 8px;
    padding: 0;
  }
  .versions li {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 14px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
    font-size: var(--fs-sm);
    color: var(--muted);
  }
  .vtitle {
    color: var(--strong);
    font-weight: 500;
  }
  .vtitle:hover {
    text-decoration: underline;
  }
  .exact {
    color: var(--accent);
  }
  /* Where the album comes from, quieter: its folder and its identification. */
  .details {
    margin-top: var(--space-7);
    padding-top: var(--space-2);
    border-top: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-3);
    font-size: var(--fs-sm);
    color: var(--muted);
  }
  .details h2 {
    font-size: var(--fs-sm);
    color: var(--muted);
    margin: var(--space-3) 0 0;
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-4);
    margin: 0;
  }
  dt {
    color: var(--faint);
  }
  dd {
    margin: 0;
    min-width: 0;
  }
  .details .path {
    font-size: var(--fs-sm);
  }
  .path:hover {
    color: var(--text);
  }
  .fix {
    font-size: var(--fs-sm);
    color: var(--muted);
  }
  .details :global(.identificar) {
    width: min(100%, 720px);
  }
  @media (max-width: 640px) {
    .head {
      flex-direction: column;
      align-items: stretch;
      gap: var(--space-5);
      margin-top: var(--space-2);
    }
    .pic {
      width: min(100%, 280px);
      align-self: center;
    }
    .glow img {
      width: 100%;
      height: 70%;
    }
    .glow {
      mask-image: radial-gradient(ellipse 90% 60% at 50% 30%, #000 30%, transparent 100%);
    }
    .artist {
      font-size: var(--fs-lg);
      margin-bottom: var(--space-2);
    }
    .bar {
      margin-top: var(--space-4);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .glow img {
      animation: none;
    }
  }
</style>
