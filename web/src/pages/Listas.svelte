<script>
  // Lists: the playlists, favourite tracks (♥), Preferidos albums (✦), what
  // else the user likes (artists, collections) and the collections found in
  // the folders.
  import ListaPistas from '../components/ListaPistas.svelte'
  import Icono from '../components/Icono.svelte'
  import MeGusta from '../components/MeGusta.svelte'
  import Tapa from '../components/Tapa.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import TarjetaArtista from '../components/TarjetaArtista.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { mine } from '../lib/mine.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { play } from '../lib/playback.svelte.js'
  import { folderHref, listHref } from '../lib/router.js'

  // The albums with a picture: a folder image, an embedded or a cached one.
  const withPicture = (albums) => albums.filter((a) => a.picture).slice(0, 4)

  let playlists = $state(null)
  let collections = $state(null)
  let likedLists = $state(null)
  let error = $state('')
  let tab = $state('artists')
  let creating = $state(false)
  let name = $state('')
  let busy = false

  const writable = $derived(!app.status?.readOnly)

  $effect(() => {
    app.generation
    api
      .playlists()
      .then((ls) => ((playlists = ls), (error = '')))
      .catch((e) => (error = e.message))
    api
      .collections()
      .then((cs) => (collections = cs))
      .catch((e) => (error = e.message))
  })

  // The likes load again when one changes elsewhere.
  $effect(() => {
    app.generation
    mine.marks
    api
      .liked()
      .then((l) => (likedLists = l))
      .catch((e) => (error = e.message))
  })

  const tabs = $derived([
    { id: 'artists', label: 'Artistas', n: likedLists?.artists.length ?? 0 },
    { id: 'collections', label: 'Colecciones', n: likedLists?.collections.length ?? 0 },
  ])

  // A liked collection comes without its albums: take them from the full list.
  const albumsOf = (c) => collections?.find((x) => x.path === c.path)?.albums ?? []

  async function create(event) {
    event.preventDefault()
    const n = name.trim()
    if (!n || busy) return
    busy = true
    try {
      const list = await api.createPlaylist(n)
      navigate(listHref(list.id))
    } catch (e) {
      notify(e.message)
      busy = false
    }
  }

  // Focuses an input that appears after a click (autofocus does not fire then).
  const focus = (el) => el.focus()
</script>

<h1>Listas</h1>
{#if error}
  <p class="empty">{error}</p>
{/if}

<h2>Listas de reproducción</h2>
{#if playlists}
  {#if playlists.length === 0 && !writable}
    <p class="empty">Todavía no hay listas.</p>
  {:else}
    {#if playlists.length === 0}
      <p class="empty">Todavía no hay listas. Creá una acá o con «+ Lista» en un álbum, una carpeta o una pista.</p>
    {/if}
    <div class="grid">
      {#each playlists as p (p.id)}
        <a class="card" href={listHref(p.id)}>
          <Tapa srcs={p.covers.map((c) => albumCoverURL(c, 320))} icon="lists" alt={p.name} />
          <span class="name">{p.name}</span>
          <span class="sub">{count(p.tracks, 'pista', 'pistas')}</span>
        </a>
      {/each}
      {#if writable}
        <div class="card">
          {#if creating}
            <form class="new" onsubmit={create}>
              <input bind:value={name} placeholder="Nombre de la lista" aria-label="Nombre de la lista" maxlength="200" use:focus
                onkeydown={(e) => e.key === 'Escape' && (creating = false)} />
              <button class="primary" disabled={!name.trim()}>Crear</button>
            </form>
          {:else}
            <button class="newcard" onclick={() => ((name = ''), (busy = false), (creating = true))}>
              <Icono name="plus" size={28} />
              <span>Nueva lista</span>
            </button>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
{/if}

<h2>Canciones favoritas</h2>
{#if likedLists}
  {#if likedLists.tracks.length === 0}
    <p class="empty">Todavía no hay canciones favoritas: tocá ♥ en las que te gusten.</p>
  {:else}
    <div class="bar">
      <button class="primary" onclick={() => play({ liked: true })}><Icono name="play" size={14} /> Reproducir</button>
      <button onclick={() => play({ liked: true }, { shuffle: true })}><Icono name="shuffle" size={14} /> Aleatorio</button>
    </div>
    <ListaPistas tracks={likedLists.tracks} showAlbum numbered={false} onplay={(t) => play({ liked: true }, { startId: t.id })} />
  {/if}
{/if}

<h2>Álbumes preferidos</h2>
{#if likedLists}
  {#if likedLists.albums.length === 0}
    <p class="empty">Todavía no hay álbumes preferidos: tocá ✦ en los que quieras tener a mano.</p>
  {:else}
    <div class="grid">
      {#each likedLists.albums as a (a.id)}<TarjetaAlbum album={a} />{/each}
    </div>
  {/if}
{/if}

<h2>Me gusta</h2>
{#if likedLists}
  <div class="tabs" role="tablist">
    {#each tabs as t (t.id)}
      <button role="tab" aria-selected={tab === t.id} class:on={tab === t.id} onclick={() => (tab = t.id)}>{t.label} · {t.n}</button>
    {/each}
  </div>
  {#if tabs.find((t) => t.id === tab).n === 0}
    <p class="empty">Nada por acá todavía: tocá ♥ en lo que te guste.</p>
  {:else if tab === 'artists'}
    <div class="grid">
      {#each likedLists.artists as a (a.key)}<TarjetaArtista artist={a} />{/each}
    </div>
  {:else}
    <div class="grid">
      {#each likedLists.collections as c (c.path)}
        <div class="card">
          <a href={folderHref(c.path)}>
            <Tapa srcs={withPicture(albumsOf(c)).map((a) => albumCoverURL(a.id, 320))} icon="lists" alt={c.name} />
          </a>
          <span class="name">{c.name}</span>
          <span class="sub">{count(c.albumCount, 'álbum', 'álbumes')} <MeGusta kind="collection" ref={c.path} size={16} /></span>
        </div>
      {/each}
    </div>
  {/if}
{/if}

<h2>Colecciones</h2>
{#if collections?.length === 0}
  <p class="empty">
    No se encontraron colecciones: carpetas con álbumes de varios artistas, como «Latinoamérica 600».
  </p>
{:else if collections}
  <div class="grid">
    {#each collections as c (c.path)}
      <div class="card">
        <a href={folderHref(c.path)}>
          <Tapa srcs={withPicture(c.albums).map((a) => albumCoverURL(a.id, 320))} icon="lists" alt={c.name} />
        </a>
        <a class="name" href={folderHref(c.path)}>{c.name}</a>
        <span class="sub">{count(c.albums.length, 'álbum', 'álbumes')} <MeGusta kind="collection" ref={c.path} size={16} /></span>
      </div>
    {/each}
  </div>
{/if}

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .name {
    margin-top: 4px;
    color: var(--strong);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 13px;
    color: var(--muted);
  }
  .new {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .newcard {
    aspect-ratio: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    border: 1px dashed var(--line-strong);
    color: var(--muted);
  }
  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 12px;
  }
  .tabs button.on {
    border-color: var(--accent);
    color: var(--accent);
  }
  .bar {
    display: flex;
    gap: 8px;
    margin-bottom: 8px;
  }
  .bar button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
  }
</style>
