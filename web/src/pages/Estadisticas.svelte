<script>
  // Tu escucha: what was played in a period (week, month, year, always).
  import { untrack } from 'svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { count, country, duration } from '../lib/format.js'
  import { play } from '../lib/playback.svelte.js'
  import { artistHref, exploreHref, statsHref } from '../lib/router.js'

  let { period } = $props()

  const PERIODS = [
    { value: 'semana', label: 'Última semana' },
    { value: 'mes', label: 'Último mes' },
    { value: 'anio', label: 'Último año' },
    { value: 'siempre', label: 'Siempre' },
  ]

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
  const plays = (n) => count(n, 'reproducción', 'reproducciones')
</script>

<h1>Tu escucha</h1>
<nav class="tabs">
  {#each PERIODS as p (p.value)}
    <a href={statsHref(p.value)} class:on={period === p.value}>{p.label}</a>
  {/each}
</nav>

{#if data}
  {#if !data.plays}
    <p class="empty">Todavía no hay reproducciones en este período.</p>
  {:else}
    <p class="totals">
      {plays(data.plays)} · {duration(data.listenedMs)} · {count(data.played, 'pista distinta', 'pistas distintas')}
    </p>

    {#if tracks.length}
      <section>
        <h2>Pistas</h2>
        <ListaPistas {tracks} numbered={false} showAlbum showPlays onplay={(t) => play({ tracks: tracks.map((x) => x.id) }, { startId: t.id })} />
      </section>
    {/if}

    {#if albums.length}
      <section>
        <h2>Álbumes</h2>
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
        <h2>Artistas</h2>
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
        <h2>Países</h2>
        <ol class="rank">
          {#each countries as c (c.value)}
            <li>
              <a href={exploreHref({ pais: c.value })}>{country(c.value)}</a>
              <span class="n">{plays(c.plays)}</span>
            </li>
          {/each}
        </ol>
      </section>
    {/if}

    {#if genres.length}
      <section>
        <h2>Géneros</h2>
        <ol class="rank">
          {#each genres as g (g.value)}
            <li>
              <a href={exploreHref({ genero: g.value })}>{g.value}</a>
              <span class="n">{plays(g.plays)}</span>
            </li>
          {/each}
        </ol>
      </section>
    {/if}
  {/if}

  {#if data.never || data.stale}
    <section>
      <h2>Por escuchar</h2>
      <ul class="rank">
        {#if data.never}
          <li><a href={exploreHref({ escucha: 'nunca' })}>{count(data.never, 'álbum nunca escuchado', 'álbumes nunca escuchados')} →</a></li>
        {/if}
        {#if data.stale}
          <li>
            <a href={exploreHref({ escucha: 'hace-un-anio' })}>{count(data.stale, 'álbum sin escuchar', 'álbumes sin escuchar')} hace más de un año →</a>
          </li>
        {/if}
      </ul>
    </section>
  {/if}
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
    font-size: 13px;
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
