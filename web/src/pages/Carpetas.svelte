<script>
  // The folders as they are on disk: the roots, then each folder with its
  // subfolders, albums and loose tracks. Collections are folders too.
  import Acciones from '../components/Acciones.svelte'
  import Icono from '../components/Icono.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import Tapa from '../components/Tapa.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count, length } from '../lib/format.js'
  import { navigate, savedScroll } from '../lib/nav.svelte.js'
  import { play } from '../lib/player.svelte.js'
  import { albumHref, folderHref } from '../lib/router.js'

  let { path } = $props()

  let folder = $state(null)
  let error = $state('')
  let restoredFor = null // the folder whose scroll position was restored: reloads after a scan keep the page where it is

  $effect(() => {
    const p = path
    app.generation // reload when a scan ends
    let stale = false
    api
      .folder(p)
      .then((f) => {
        if (stale) return
        // With a single root, the top level is that root.
        if (!p && f.children.length === 1) {
          navigate(folderHref(f.children[0].path), { replace: true })
          return
        }
        folder = f
        error = ''
        if (restoredFor !== p) {
          restoredFor = p
          requestAnimationFrame(() => window.scrollTo(0, savedScroll()))
        }
      })
      .catch((e) => {
        if (!stale) error = e.message
      })
    return () => (stale = true)
  })

  function childSummary(c) {
    const n = c.albumCount > 0 ? count(c.albumCount, 'álbum', 'álbumes') : count(c.trackCount, 'pista', 'pistas')
    return c.collection ? `Colección · ${n}` : n
  }

  // A subfolder holding a single album (its own folder) opens the album.
  function childHref(child) {
    const art = folder.art?.[child.path] ?? []
    return child.albumCount === 1 && art.length === 1 ? albumHref(art[0]) : folderHref(child.path)
  }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if folder}
  {#if path}
    <nav class="trail" aria-label="Ubicación">
      <a href="/carpetas">Carpetas</a>
      {#each folder.trail.slice(0, -1) as c (c.path)}
        <span>/</span><a href={folderHref(c.path)}>{c.name}</a>
      {/each}
    </nav>
    {#if folder.collection}<span class="label">Colección</span>{/if}
    <h1>{folder.name}</h1>
    <p class="sub">
      {count(folder.albumCount, 'álbum', 'álbumes')} · {count(folder.trackCount, 'pista', 'pistas')} · {length(folder.durationMs)}
    </p>
    {#if folder.trackCount > 0}
      <Acciones what={{ folder: path }} />
    {/if}
  {:else}
    <h1>Carpetas</h1>
    {#if folder.children.length === 0}
      <p class="empty">Todavía no hay música en el catálogo. Revisá las carpetas de config.json.</p>
    {/if}
  {/if}

  {#if folder.children.length > 0}
    {#if path}<h2>Carpetas</h2>{/if}
    <div class="grid">
      {#each folder.children as c (c.path)}
        <div class="card">
          <div class="pic">
            <a href={childHref(c)}>
              <Tapa srcs={(folder.art?.[c.path] ?? []).map((id) => albumCoverURL(id, 320))} icon="folder" alt={c.name} />
            </a>
            {#if c.trackCount > 0}
              <button class="icon play" onclick={() => play({ folder: c.path })} title="Reproducir" aria-label="Reproducir {c.name}">
                <Icono name="play" size={18} />
              </button>
            {/if}
          </div>
          <a class="name" href={childHref(c)}>{c.name}</a>
          <span class="sub">{childSummary(c)}</span>
        </div>
      {/each}
    </div>
  {/if}

  {#if folder.albums?.length > 0}
    <h2>Álbumes</h2>
    <div class="grid">
      {#each folder.albums as a (a.id)}
        <TarjetaAlbum album={a} />
      {/each}
    </div>
  {/if}

  {#if folder.loose?.length > 0}
    <h2>Pistas sueltas</h2>
    <ListaPistas
      tracks={folder.loose}
      numbered={false}
      onplay={(t) => play({ tracks: folder.loose.map((x) => x.id) }, { startId: t.id })} />
  {/if}
{/if}

<style>
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
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0;
  }
  .card {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .pic {
    position: relative;
    margin-bottom: 8px;
  }
  .pic a {
    display: block;
  }
  .name {
    color: var(--strong);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .card .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .play {
    position: absolute;
    right: 8px;
    bottom: 8px;
    width: 40px;
    height: 40px;
    background: var(--accent);
    color: var(--on-accent) !important;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .card:hover .play,
  .play:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .play {
      display: none;
    }
  }
</style>
