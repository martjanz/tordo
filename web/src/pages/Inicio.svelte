<script>
  // Inicio: rows of albums and stations. The stations and the personal rows
  // always, plus three rotating ones, all in an order drawn with a seed (a
  // new one per visit, or on demand).
  import { untrack } from 'svelte'
  import Cargando from '../components/Cargando.svelte'
  import FilaAlbumes from '../components/FilaAlbumes.svelte'
  import Icono from '../components/Icono.svelte'
  import TarjetaEstacion from '../components/TarjetaEstacion.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { mine } from '../lib/mine.svelte.js'
  import { can, playStation } from '../lib/playback.svelte.js'
  import { exploreHref, statsHref } from '../lib/router.js'
  import { FAVORITE_TRACKS, LIBRARY, LIKED, PREFERRED_ALBUMS, rowTitle, station } from '../lib/stations.js'
  import { homeSections } from '../lib/home.js'
  import { scanMessage } from '../lib/status.js'

  const newSeed = () => Math.floor(Math.random() * 2 ** 31)

  let seed = newSeed()
  let data = $state(null)
  let loads = 0

  async function load() {
    const id = ++loads
    try {
      const d = await api.home(seed)
      if (id === loads) data = d
    } catch (e) {
      if (id === loads) notify(e.message)
    }
  }

  // Load on a scan or identification run ending too (same seed).
  $effect(() => {
    app.generation
    untrack(load)
  })

  // A new seed redraws the rotating rows and the order, as a new visit
  // would; the page stays where it is.
  function refresh() {
    seed = newSeed()
    load()
  }

  async function remove(st) {
    try {
      await api.deleteStation(st.id)
    } catch (e) {
      notify(e.message)
    }
    load()
  }

  const anyLike = $derived(
    Object.values(mine.marks.tracks).some((m) => m.liked) ||
      Object.values(mine.marks.albums).some((m) => m.liked) ||
      Object.keys(mine.marks.artists).length > 0 ||
      Object.keys(mine.marks.collections).length > 0,
  )
  const stations = $derived([LIBRARY, ...(anyLike ? [LIKED] : []), ...(data?.stations ?? [])])
  // Ordered when the rows arrive, so a refresh reshuffles with them, not
  // the old rows ahead of time.
  const sections = $derived(data ? homeSections(data, seed) : [])
  const empty = $derived(data && sections.length === 1)
  const scanning = $derived(!!app.status?.scan?.running)
</script>

<div class="head">
  <h1>Inicio</h1>
  <span class="tools">
    <a class="stats" href={statsHref('mes')}><Icono name="chart" size={16} /> Historial de escucha →</a>
    {#if data && !empty}
      <button class="icon" onclick={refresh} title="Otras filas" aria-label="Cambiar las filas">
        <Icono name="refresh" size={16} />
      </button>
    {/if}
  </span>
</div>

{#if data}
  {#if empty}
    {#if scanning}
      {@const msg = scanMessage(app.status.scan)}
      <p class="empty">{msg.title}<br /><small>{msg.detail}</small></p>
    {:else}
      <p class="empty">Todavía no hay música en el catálogo. Revisá las carpetas de config.json.</p>
    {/if}
  {:else}
    {#each sections as s (s.key)}
      {#if s.key === 'stations'}
        <section class="stations">
          <h2>Tus estaciones</h2>
          <div class="strip">
            {#each stations as st (st.id ?? st.name)}
              <div class="cell">
                <TarjetaEstacion station={st} onplay={can('playStation') ? playStation : null} ondelete={st.id != null && !app.status?.readOnly ? remove : null} />
              </div>
            {/each}
          </div>
        </section>
      {:else if s.key === 'continue'}
        <FilaAlbumes title="Seguir escuchando" href="/explorar?orden=escuchado" albums={data.continue} />
      {:else if s.key === 'likedTracks'}
        <FilaAlbumes title="Tus canciones favoritas" href={exploreHref({ megusta: 'si' }, 'pistas')} station={FAVORITE_TRACKS} albums={data.likedTracks} />
      {:else if s.key === 'liked'}
        <FilaAlbumes title="Álbumes preferidos" href={exploreHref({ megusta: 'si' })} station={PREFERRED_ALBUMS} albums={data.liked} />
      {:else if s.row.kind === 'recientes'}
        <FilaAlbumes title={rowTitle(s.row)} href="/explorar?orden=agregado" albums={s.row.albums} />
      {:else}
        <FilaAlbumes
          title={rowTitle(s.row)}
          href={exploreHref(s.row.facets)}
          station={station(rowTitle(s.row), { view: 'albumes', facets: s.row.facets })}
          albums={s.row.albums} />
      {/if}
    {/each}
  {/if}
{:else}
  <Cargando />
{/if}

<style>
  .head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 16px;
  }
  .head h1 {
    margin: 0;
  }
  .tools {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }
  .stats {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: var(--fs-md);
    color: var(--muted);
  }
  .stations {
    margin-bottom: 28px;
  }
  .strip {
    display: flex;
    gap: 16px;
    overflow-x: auto;
    scroll-snap-type: x proximity;
    padding-bottom: 8px;
  }
  .cell {
    flex: 0 0 168px;
    min-width: 0;
    scroll-snap-align: start;
  }
  @media (max-width: 640px) {
    .strip {
      gap: 12px;
    }
    .cell {
      flex-basis: 140px;
    }
  }
</style>
