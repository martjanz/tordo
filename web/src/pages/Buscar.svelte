<script>
  // Every result of a search, by kind (the header's search shows a few).
  import ListaPistas from '../components/ListaPistas.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import TarjetaArtista from '../components/TarjetaArtista.svelte'
  import { api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { play } from '../lib/player.svelte.js'
  import { folderHref } from '../lib/router.js'
  import { searchable, total } from '../lib/search.js'

  const LIMIT = 200

  let { q } = $props()

  let result = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    result = null
    if (!searchable(q)) return
    let stale = false
    api
      .find(q, LIMIT)
      .then((r) => {
        if (!stale) (result = r), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.message
      })
    return () => (stale = true)
  })

  // more tells how many of a kind are left out.
  const more = (kind, shown) => (result.totals[kind] ?? 0) - shown
</script>

<h1>Buscar</h1>
{#if !searchable(q)}
  <p class="empty">Escribí al menos dos letras en la búsqueda (Ctrl+K).</p>
{:else if error}
  <p class="empty">{error}</p>
{:else if result}
  <p class="sub">{count(total(result), 'resultado', 'resultados')} para «{q}»</p>
  {#if total(result) === 0}
    <p class="empty">Nada con «{q}».</p>
  {/if}
  {#if result.artists.length}
    <h2>Artistas</h2>
    <div class="grid">
      {#each result.artists as a (a.key)}
        <TarjetaArtista artist={a} />
      {/each}
    </div>
  {/if}
  {#if result.albums.length}
    <h2>Álbumes</h2>
    <div class="grid">
      {#each result.albums as a (a.id)}
        <TarjetaAlbum album={a} />
      {/each}
    </div>
    {#if more('albums', result.albums.length) > 0}<p class="sub">Y {more('albums', result.albums.length)} más.</p>{/if}
  {/if}
  {#if result.tracks.length}
    <h2>Pistas</h2>
    <ListaPistas
      tracks={result.tracks}
      numbered={false}
      showAlbum
      onplay={(t) => play({ tracks: result.tracks.map((x) => x.id) }, { startId: t.id })} />
    {#if more('tracks', result.tracks.length) > 0}<p class="sub">Y {more('tracks', result.tracks.length)} más.</p>{/if}
  {/if}
  {#if result.folders.length}
    <h2>Carpetas</h2>
    <ul class="folders">
      {#each result.folders as f (f.path)}
        <li>
          <a href={folderHref(f.path)}>{f.name}</a>
          <span>{f.collection ? 'Colección · ' : ''}{count(f.tracks, 'pista', 'pistas')}</span>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<style>
  .sub {
    font-size: 13px;
    color: var(--muted);
  }
  .folders {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .folders li {
    display: flex;
    gap: 12px;
    align-items: baseline;
    padding: 8px;
    border-bottom: 1px solid var(--line);
  }
  .folders a {
    color: var(--strong);
  }
  .folders span {
    font-size: 13px;
    color: var(--faint);
  }
</style>
