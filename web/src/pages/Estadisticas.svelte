<script>
  // Tu escucha: what was played in a period (week, month, year, always).
  import { untrack } from 'svelte'
  import Cargando from '../components/Cargando.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { country, duration } from '../lib/format.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { play } from '../lib/playback.svelte.js'
  import { artistHref, exploreHref, statsHref } from '../lib/router.js'

  let { period } = $props()

  const PERIODS = ['week', 'month', 'year', 'all']

  let data = $state(null)
  let loads = 0

  async function load(p) {
    const id = ++loads
    try {
      const d = await api.stats(p, 20)
      if (id === loads) data = d
    } catch (e) {
      if (id === loads) notify(e.message)
    }
  }

  // Reload on the period and when a scan or identification run ends.
  $effect(() => {
    app.generation
    const p = period
    untrack(() => load(p))
  })

  const tracks = $derived(data?.tracks ?? [])
  const albums = $derived(data?.albums ?? [])
  const artists = $derived(data?.artists ?? [])
  const countries = $derived(data?.countries ?? [])
  const genres = $derived(data?.genres ?? [])
  const plays = (n) => t('count.plays', { n })
</script>

<h1>{t('stats.title')}</h1>
<nav class="tabs">
  {#each PERIODS as p (p)}
    <a href={statsHref(p)} class:on={period === p}>{t('stats.' + p)}</a>
  {/each}
</nav>

{#if data}
  {#if !data.plays}
    <p class="empty">{t('stats.empty')}</p>
  {:else}
    <p class="totals">
      {plays(data.plays)} · {duration(data.listenedMs)} · {t('count.distinctTracks', { n: data.played })}
    </p>

    {#if tracks.length}
      <section>
        <h2>{t('facets.view.tracks')}</h2>
        <ListaPistas {tracks} numbered={false} showAlbum showPlays onplay={(track) => play({ tracks: tracks.map((x) => x.id) }, { startId: track.id })} />
      </section>
    {/if}

    {#if albums.length}
      <section>
        <h2>{t('facets.view.albums')}</h2>
        <div class="grid">
          {#each albums as a (a.id)}
            <div class="cell">
              <TarjetaAlbum album={a} />
              <span class="plays">{plays(a.plays)}</span>
            </div>
          {/each}
        </div>
      </section>
    {/if}

    {#if artists.length}
      <section>
        <h2>{t('facets.view.artists')}</h2>
        <ol class="rank">
          {#each artists as a (a.key ? 'k:' + a.key : 'n:' + a.name)}
            <li>
              {#if a.key}<a href={artistHref(a.key)}>{a.name}</a>{:else}<span>{a.name}</span>{/if}
              <span class="n">{plays(a.plays)}</span>
            </li>
          {/each}
        </ol>
      </section>
    {/if}

    {#if countries.length}
      <section>
        <h2>{t('stats.countries')}</h2>
        <ol class="rank">
          {#each countries as c (c.value)}
            <li>
              <a href={exploreHref({ country: c.value })}>{country(c.value)}</a>
              <span class="n">{plays(c.plays)}</span>
            </li>
          {/each}
        </ol>
      </section>
    {/if}

    {#if genres.length}
      <section>
        <h2>{t('stats.genres')}</h2>
        <ol class="rank">
          {#each genres as g (g.value)}
            <li>
              <a href={exploreHref({ genre: g.value })}>{g.value}</a>
              <span class="n">{plays(g.plays)}</span>
            </li>
          {/each}
        </ol>
      </section>
    {/if}
  {/if}

  {#if data.never || data.stale}
    <section>
      <h2>{t('stats.toListen')}</h2>
      <ul class="rank">
        {#if data.never}
          <li><a href={exploreHref({ listened: 'never' })}>{t('count.albumsNever', { n: data.never })} →</a></li>
        {/if}
        {#if data.stale}
          <li>
            <a href={exploreHref({ listened: 'year-ago' })}>{t('count.albumsStale', { n: data.stale })} →</a>
          </li>
        {/if}
      </ul>
    </section>
  {/if}
{:else}
  <Cargando />
{/if}

<style>
  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 20px;
    margin: 12px 0 16px;
    border-bottom: 1px solid var(--line);
  }
  .tabs a {
    padding: 6px 0;
    border-bottom: 2px solid transparent;
    color: var(--muted);
  }
  .tabs a.on {
    color: var(--strong);
    border-bottom-color: var(--accent);
  }
  .totals {
    color: var(--muted);
    margin: 0 0 8px;
  }
  section {
    margin-top: 28px;
  }
  .cell {
    min-width: 0;
  }
  .plays,
  .n {
    font-size: var(--fs-sm);
    color: var(--muted);
  }
  .rank {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .rank li {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .rank a:hover {
    text-decoration: underline;
  }
</style>
