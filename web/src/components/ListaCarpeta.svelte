<script>
  // One screen of the narrow Carpetas layout: a folder as an iPod-style list.
  import { untrack } from 'svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { loadLevel, revealPath, tree } from '../lib/carpetas.svelte.js'
  import { entries, playableIds } from '../lib/folders.js'
  import { duration, folderName, size } from '../lib/format.js'
  import { matchRows, shortPath } from '../lib/tree.js'
  import { enqueue, play } from '../lib/player.svelte.js'
  import { albumHref, folderHref } from '../lib/router.js'
  import { MIN } from '../lib/search.js'
  import Icono from './Icono.svelte'

  let { path, rootPath, folder } = $props()

  let back = $state(null) // the folder this screen came from; null at the top
  let menuOpen = $state(false)
  let menu = $state(null)
  let query = $state('')
  let result = $state(null)

  const isTop = $derived(path === rootPath || path === '')
  const level = $derived(tree.levels[path] ?? [])
  const files = $derived(entries(folder).files)
  const ids = $derived(playableIds(files))
  const album = $derived(folder.albums?.length === 1 && folder.albums[0].dir === folder.path ? folder.albums[0] : null)
  const title = $derived(path === '' ? 'Carpetas' : folder.name || folderName(path))
  const backName = $derived(back === null ? '' : back === '' ? 'Carpetas' : folderName(back))
  const found = $derived(result ? matchRows(result) : null)

  // The subfolders of this folder (levels in between merged).
  $effect(() => {
    tree.generation // a scan resets the levels: load again
    if (tree.levels[path]) return
    untrack(() => loadLevel(path)).catch((e) => notify(e.message))
  })

  // The screen to go back to.
  $effect(() => {
    const p = path
    tree.generation
    let stale = false
    untrack(() => revealPath(p))
      .then((r) => {
        if (stale) return
        back = r.chain.at(-1) ?? (p === rootPath || p === '' ? null : rootPath)
      })
      .catch((e) => notify(e.message))
    return () => (stale = true)
  })

  // Filter by name, only on the top screen; debounced, stale responses ignored.
  $effect(() => {
    const q = isTop ? query : ''
    if (q.trim().length < MIN) {
      result = null
      return
    }
    let stale = false
    const timer = setTimeout(() => {
      api
        .folderTreeSearch(q)
        .then((r) => {
          if (!stale) result = r
        })
        .catch((e) => {
          if (!stale) notify(e.message)
        })
    }, 200)
    return () => {
      stale = true
      clearTimeout(timer)
    }
  })

  async function setCollection(collection) {
    menuOpen = false
    try {
      const f = await api.setCollection(folder.path, collection)
      folder.collection = f.collection
    } catch (e) {
      notify(e.message)
    }
  }

  function act(fn) {
    menuOpen = false
    fn()
  }

  function windowClick(event) {
    if (menuOpen && menu && !menu.contains(event.target)) menuOpen = false
  }

  function windowKeydown(event) {
    if (menuOpen && event.key === 'Escape') menuOpen = false
  }
</script>

<svelte:window onclick={windowClick} onkeydown={windowKeydown} />

<div class="lista">
  <header>
    {#if back !== null}
      <a class="back" href={folderHref(back)}><Icono name="chevron_right" size={16} /><span>{backName}</span></a>
    {:else}
      <span class="back"></span>
    {/if}
    <h1>{title}</h1>
    <div class="more" bind:this={menu}>
      {#if path !== ''}
        <button class="icon" aria-label="Acciones" aria-haspopup="menu" aria-expanded={menuOpen} onclick={() => (menuOpen = !menuOpen)}>
          <Icono name="more" size={20} />
        </button>
      {/if}
      {#if menuOpen}
        <div class="menu" role="menu">
          {#if folder.trackCount > 0}
            <button role="menuitem" onclick={() => act(() => play({ folder: path }))}>Reproducir</button>
            <button role="menuitem" onclick={() => act(() => play({ folder: path }, { shuffle: true }))}>Aleatorio</button>
            <button role="menuitem" onclick={() => act(() => enqueue({ folder: path }))}>A la cola</button>
          {/if}
          {#if folder.detected && !app.status?.readOnly}
            <button role="menuitem" onclick={() => setCollection(!folder.collection)}>
              {folder.collection ? 'No es una colección' : 'Es una colección'}
            </button>
          {/if}
        </div>
      {/if}
    </div>
  </header>

  {#if isTop}
    <div class="filter">
      <input type="search" placeholder="Filtrar carpetas…" aria-label="Filtrar carpetas" bind:value={query} />
    </div>
  {/if}

  <ul>
    {#if found}
      {#each found as m (m.node.path)}
        <li>
          <a class="row two" href={folderHref(m.node.path)}>
            <Icono name={m.node.album ? 'disc' : 'folder'} size={18} />
            <span class="text">
              <span class="name">{m.node.label}</span>
              <span class="sub">{shortPath(m.parent || m.node.path, rootPath)}</span>
            </span>
            <span class="num">{m.node.tracks.toLocaleString('es')}</span>
            <Icono name="chevron_right" size={16} />
          </a>
        </li>
      {/each}
    {:else}
      {#if album}
        <li>
          <a class="row" href={albumHref(album.id)}>
            <Icono name="disc" size={18} />
            <span class="name">Ver álbum</span>
            <Icono name="chevron_right" size={16} />
          </a>
        </li>
      {/if}
      {#each level as node (node.path)}
        <li>
          <a class="row" href={folderHref(node.path)}>
            <Icono name={node.album ? 'disc' : 'folder'} size={18} />
            <span class="name">{node.label}</span>
            <span class="num">{node.tracks.toLocaleString('es')}</span>
            <Icono name="chevron_right" size={16} />
          </a>
        </li>
      {/each}
      {#each files as f (f.key)}
        <li>
          {#if f.type === 'audio'}
            <button
              class="row"
              class:dim={!f.playable}
              disabled={!f.playable}
              title={f.playable ? undefined : f.track ? 'No se puede reproducir: falta ffmpeg' : 'No se pudo leer este archivo'}
              onclick={() => play({ tracks: ids }, { startId: f.id })}>
              <Icono name="music" size={18} />
              <span class="name">{f.name}</span>
              <span class="num">{duration(f.durationMs)}</span>
            </button>
          {:else}
            <div class="row dim">
              <Icono name="file" size={18} />
              <span class="name">{f.name}</span>
              <span class="num">{size(f.sizeBytes)}</span>
            </div>
          {/if}
        </li>
      {/each}
    {/if}
  </ul>
  {#if result?.matches.length === 0}
    <p class="note">Ninguna carpeta se llama así.</p>
  {:else if result?.more}
    <p class="note">Hay más coincidencias: escribí más para acotar.</p>
  {/if}
</div>

<style>
  header {
    position: sticky;
    top: var(--nav-h);
    z-index: 2;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 2fr) minmax(0, 1fr);
    align-items: center;
    gap: 8px;
    min-height: 44px;
    background: var(--bg);
    border-bottom: 1px solid var(--line);
  }
  .back {
    display: flex;
    align-items: center;
    min-width: 0;
    min-height: 44px;
    color: var(--accent);
    font-size: 15px;
  }
  .back :global(svg) {
    transform: rotate(180deg);
    flex: none;
  }
  .back span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  h1 {
    margin: 0;
    font-size: 16px;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .more {
    position: relative;
    justify-self: end;
  }
  .more .icon {
    min-width: 44px;
    min-height: 44px;
  }
  .menu {
    position: absolute;
    right: 0;
    top: 100%;
    z-index: 3;
    min-width: 190px;
    display: flex;
    flex-direction: column;
    background: var(--surface-2);
    border: 1px solid var(--line);
    border-radius: 8px;
    padding: 4px;
    box-shadow: 0 6px 24px rgb(0 0 0 / 0.35);
  }
  .menu button {
    all: unset;
    box-sizing: border-box;
    min-height: 44px;
    display: flex;
    align-items: center;
    padding: 0 12px;
    border-radius: 6px;
    font-size: 15px;
    color: var(--strong);
    cursor: pointer;
  }
  .menu button:hover,
  .menu button:focus-visible {
    background: var(--surface);
  }
  .filter {
    padding: 8px 0;
  }
  .filter input {
    width: 100%;
    box-sizing: border-box;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    border-bottom: 1px solid var(--line);
  }
  .row {
    all: unset;
    box-sizing: border-box;
    width: 100%;
    min-height: 44px;
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 15px;
    color: var(--strong);
    cursor: pointer;
  }
  .row :global(svg) {
    flex: none;
    color: var(--faint);
  }
  .row.two {
    padding: 4px 0;
  }
  .row:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .text .name {
    flex: none;
  }
  .sub {
    color: var(--muted);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .num {
    color: var(--muted);
    font-size: 13px;
    font-variant-numeric: tabular-nums;
  }
  .dim {
    opacity: 0.5;
    cursor: default;
  }
  .note {
    color: var(--muted);
    font-size: 13px;
    margin: 8px 0;
  }
</style>
