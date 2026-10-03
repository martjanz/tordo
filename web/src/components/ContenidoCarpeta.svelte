<script>
  // The right panel of Carpetas: the header of a folder and its detail list.
  // Also shows the roots when there are several (no trail, no totals).
  import { albumCoverURL, api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { DEFAULT_SORT, bar, entries, nextSort, playableIds, sortEntries } from '../lib/folders.js'
  import { count, duration, length, size } from '../lib/format.js'
  import { enqueue, play } from '../lib/player.svelte.js'
  import { albumHref, folderHref } from '../lib/router.js'
  import { folderStation } from '../lib/stations.js'
  import Acciones from './Acciones.svelte'
  import Icono from './Icono.svelte'
  import MeGusta from './MeGusta.svelte'
  import Tapa from './Tapa.svelte'

  let { folder } = $props()

  let sort = $state(DEFAULT_SORT)
  let sortedFor = null
  $effect(() => {
    if (sortedFor !== null && sortedFor !== folder.path) sort = DEFAULT_SORT
    sortedFor = folder.path
  })

  const rows = $derived(entries(folder))
  const folders = $derived(sortEntries(rows.folders, sort))
  const files = $derived(sortEntries(rows.files, sort))
  const ids = $derived(playableIds(files))
  const isRoot = $derived(!folder.path)
  const album = $derived(folder.albums?.length === 1 && folder.albums[0].dir === folder.path ? folder.albums[0] : null)

  const COLUMNS = [
    ['name', 'Nombre'],
    [null, 'Tamaño relativo'],
    ['tracks', 'Pistas'],
    ['duration', 'Duración'],
    ['format', 'Formato'],
    ['size', 'Tamaño'],
  ]

  function ariaSort(column) {
    if (sort.column !== column) return 'none'
    return sort.desc ? 'descending' : 'ascending'
  }

  // setCollection says a detected collection is not one (or takes it back).
  async function setCollection(collection) {
    try {
      const f = await api.setCollection(folder.path, collection)
      folder.collection = f.collection
    } catch (e) {
      notify(e.message)
    }
  }
</script>

<header>
  {#if isRoot}
    <h1>Carpetas</h1>
  {:else}
    <nav class="trail" aria-label="Ubicación">
      <a href="/carpetas">Carpetas</a>
      {#each folder.trail ?? [] as c, i (c.path)}
        <span>/</span>
        {#if i < folder.trail.length - 1}<a href={folderHref(c.path)}>{c.name}</a>{:else}<span class="here">{c.name}</span>{/if}
      {/each}
    </nav>
    <div class="title">
      {#if album}
        <div class="cover">
          <Tapa srcs={album.picture ? [albumCoverURL(album.id, 160)] : []} icon="disc" alt={album.title} />
        </div>
      {/if}
      <div>
        {#if folder.collection}<span class="label">Colección</span>{/if}
        <h1>{folder.name}</h1>
        <p class="sub">
          {count(folder.albumCount, 'álbum', 'álbumes')} · {count(folder.trackCount, 'pista', 'pistas')} · {length(folder.durationMs)} · {size(folder.sizeBytes)}
        </p>
        {#if album}<a class="album-link" href={albumHref(album.id)}>Ver álbum</a>{/if}
      </div>
    </div>
    <div class="tools">
      {#if folder.trackCount > 0}
        <Acciones what={{ folder: folder.path }} station={folderStation(folder)} />
      {/if}
      {#if folder.detected && !app.status?.readOnly}
        <button class="small" onclick={() => setCollection(!folder.collection)}>
          {folder.collection ? 'No es una colección' : 'Es una colección'}
        </button>
      {/if}
      {#if folder.collection}<MeGusta kind="collection" ref={folder.path} />{/if}
    </div>
  {/if}
</header>

{#if folders.length + files.length > 0}
  <table>
    <thead>
      <tr>
        {#each COLUMNS as [column, label] (label)}
          <th
            class:num={column === 'tracks' || column === 'duration' || column === 'size'}
            class:bar-col={column === null}
            aria-sort={column ? ariaSort(column) : undefined}>
            {#if column}
              <button class="head" onclick={() => (sort = nextSort(sort, column))}>
                {label}{#if sort.column === column}<span aria-hidden="true">{sort.desc ? ' ↓' : ' ↑'}</span>{/if}
              </button>
            {:else}
              {label}
            {/if}
          </th>
        {/each}
        <th class="act"><span class="sr">Acciones</span></th>
      </tr>
    </thead>
    <tbody>
      {#each folders as f (f.key)}
        <tr>
          <td class="name">
            <div class="cell">
              <Icono name={f.album ? 'disc' : 'folder'} size={16} />
              <a href={folderHref(f.path)}>{f.name}</a>
              {#if f.collection}<span class="label">Colección</span>{/if}
            </div>
          </td>
          <td class="bar-col"><div class="bar" style="width: {bar(f.sizeBytes, folder.sizeBytes)}%"></div></td>
          <td class="num">{f.tracks.toLocaleString('es')}</td>
          <td class="num">{f.durationMs ? length(f.durationMs) : ''}</td>
          <td>{f.format}</td>
          <td class="num">{size(f.sizeBytes)}</td>
          <td class="act">
            {#if f.tracks > 0}
              <button class="icon row-btn" onclick={() => play({ folder: f.path })} aria-label="Reproducir {f.name}">
                <Icono name="play" size={14} />
              </button>
            {/if}
          </td>
        </tr>
      {/each}
      {#each files as f (f.key)}
        {#if f.type === 'audio'}
          <tr class:dim={!f.playable}>
            <td class="name">
              <div class="cell">
              <Icono name="music" size={16} />
              <button
                class="link"
                disabled={!f.playable}
                title={f.playable ? undefined : f.track ? 'No se puede reproducir: falta ffmpeg' : 'No se pudo leer este archivo'}
                onclick={() => play({ tracks: ids }, { startId: f.id })}>{f.name}</button>
              </div>
            </td>
            <td class="bar-col"></td>
            <td class="num"></td>
            <td class="num">{duration(f.durationMs)}</td>
            <td>{f.format}</td>
            <td class="num">{size(f.sizeBytes)}</td>
            <td class="act">
              {#if f.playable}
                <button class="icon row-btn" onclick={() => enqueue({ tracks: [f.id] })} aria-label="Agregar {f.name} a la cola">
                  <Icono name="plus" size={14} />
                </button>
              {/if}
            </td>
          </tr>
        {:else}
          <tr class="dim">
            <td class="name"><div class="cell"><Icono name="file" size={16} /><span>{f.name}</span></div></td>
            <td class="bar-col"></td>
            <td class="num"></td>
            <td class="num"></td>
            <td></td>
            <td class="num">{size(f.sizeBytes)}</td>
            <td class="act"></td>
          </tr>
        {/if}
      {/each}
    </tbody>
  </table>
{/if}

<style>
  header {
    margin-bottom: 16px;
  }
  .trail {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    font-size: 13px;
    color: var(--muted);
    margin-bottom: 12px;
  }
  .trail span {
    color: var(--faint);
  }
  .trail .here {
    color: var(--muted);
  }
  .title {
    display: flex;
    gap: 16px;
    align-items: center;
  }
  .cover {
    width: 64px;
    flex: none;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0;
  }
  .album-link {
    font-size: 13px;
    color: var(--accent);
  }
  .tools {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: center;
    margin-top: 12px;
  }
  .small {
    font-size: 13px;
    color: var(--muted);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 14px;
  }
  th,
  td {
    padding: 6px 8px;
    text-align: left;
    white-space: nowrap;
    border-bottom: 1px solid var(--line);
  }
  th {
    font-size: 12px;
    font-weight: 600;
    color: var(--faint);
  }
  .num {
    text-align: right;
  }
  .head,
  .link {
    all: unset;
    cursor: pointer;
    color: inherit;
  }
  .head:hover,
  .link:hover:not(:disabled) {
    color: var(--strong);
  }
  .head:focus-visible,
  .link:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .link:disabled {
    cursor: default;
  }
  td.name {
    width: 100%;
    max-width: 0;
  }
  .cell {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .cell :global(svg) {
    flex: none;
    color: var(--faint);
  }
  .cell a,
  .cell .link,
  .cell > span:not(.label) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cell a {
    color: var(--strong);
  }
  .cell .label {
    flex: none;
  }
  .bar-col {
    width: 140px;
    min-width: 80px;
  }
  .bar {
    height: 8px;
    background: var(--accent);
    border-radius: 2px;
    opacity: 0.7;
  }
  .dim {
    opacity: 0.5;
  }
  .act {
    width: 36px;
    padding: 0 4px;
    text-align: center;
  }
  .row-btn {
    opacity: 0;
    transition: opacity 0.15s;
  }
  tr:hover .row-btn,
  .row-btn:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .row-btn {
      display: none;
    }
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
