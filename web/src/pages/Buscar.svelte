<script>
  // Every result of a search, by kind (the header's search shows a few).
  import ListaPistas from '../components/ListaPistas.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import TarjetaArtista from '../components/TarjetaArtista.svelte'
  import { api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { play } from '../lib/playback.svelte.js'
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

<h1>{t('search.open')}</h1>
{#if !searchable(q)}
  <p class="empty">{t('search.needTwo')}</p>
{:else if error}
  <p class="empty">{error}</p>
{:else if result}
  <p class="sub">{t('search.resultsFor', { results: t('count.results', { n: total(result) }), q })}</p>
  {#if total(result) === 0}
    <p class="empty">{t('search.none', { q })}</p>
  {/if}
  {#if result.artists.length}
    <h2>{t('facets.view.artists')}</h2>
    <div class="grid">
      {#each result.artists as a (a.key)}
        <TarjetaArtista artist={a} />
      {/each}
    </div>
  {/if}
  {#if result.albums.length}
    <h2>{t('facets.view.albums')}</h2>
    <div class="grid">
      {#each result.albums as a (a.id)}
        <TarjetaAlbum album={a} />
      {/each}
    </div>
    {#if more('albums', result.albums.length) > 0}<p class="sub">{t('search.more', { n: more('albums', result.albums.length) })}</p>{/if}
  {/if}
  {#if result.tracks.length}
    <h2>{t('facets.view.tracks')}</h2>
    <ListaPistas
      tracks={result.tracks}
      numbered={false}
      showAlbum
      onplay={(track) => play({ tracks: result.tracks.map((x) => x.id) }, { startId: track.id })} />
    {#if more('tracks', result.tracks.length) > 0}<p class="sub">{t('search.more', { n: more('tracks', result.tracks.length) })}</p>{/if}
  {/if}
  {#if result.folders.length}
    <h2>{t('nav.folders')}</h2>
    <ul class="folders">
      {#each result.folders as f (f.path)}
        <li>
          <a href={folderHref(f.path)}>{f.name}</a>
          <span>{f.collection ? t('facets.facet.collection') + ' · ' : ''}{t('count.tracks', { n: f.tracks })}</span>
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
