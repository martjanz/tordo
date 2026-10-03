<script>
  // Inicio: rows of albums and stations. Fixed rows first, then three
  // rotating ones drawn with a seed (a new one per visit, or on demand).
  import { untrack } from 'svelte'
  import FilaAlbumes from '../components/FilaAlbumes.svelte'
  import Icono from '../components/Icono.svelte'
  import TarjetaEstacion from '../components/TarjetaEstacion.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { mine } from '../lib/mine.svelte.js'
  import { playStation } from '../lib/player.svelte.js'
  import { exploreHref, statsHref } from '../lib/router.js'
  import { FAVORITE_TRACKS, LIBRARY, LIKED, PREFERRED_ALBUMS, rowTitle, station } from '../lib/stations.js'

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

  // A new seed redraws the rotating rows; the page stays where it is.
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
  const rotating = $derived((data?.rotating ?? []).filter((r) => r.albums?.length))
  const empty = $derived(
    data && !data.recent?.length && !data.continue?.length && !data.likedTracks?.length && !data.liked?.length && !rotating.length,
  )
  const scanning = $derived(!!app.status?.scan?.running)
</script>

<div class="head">
  <h1>Inicio</h1>
  <a class="stats" href={statsHref('mes')}><Icono name="chart" size={16} /> Tu escucha →</a>
</div>

{#if data}
  {#if empty}
    {#if scanning}
      <p class="empty">Escaneando… La música va a aparecer a medida que se encuentre.</p>
    {:else}
      <p class="empty">Todavía no hay música en el catálogo. Revisá las carpetas de config.json.</p>
    {/if}
  {:else}
    {#if data.recent?.length}
      <FilaAlbumes title="Agregados recientemente" href="/explorar?orden=agregado" albums={data.recent} />
    {/if}

    <section class="stations">
      <h2>Tus estaciones</h2>
      <div class="strip">
        {#each stations as st (st.id ?? st.name)}
          <div class="cell">
            <TarjetaEstacion station={st} onplay={playStation} ondelete={st.id != null && !app.status?.readOnly ? remove : null} />
          </div>
        {/each}
      </div>
    </section>

    {#if data.continue?.length}
      <FilaAlbumes title="Seguir escuchando" href="/explorar?orden=escuchado" albums={data.continue} />
    {/if}
    {#if data.likedTracks?.length}
      <FilaAlbumes title="Tus canciones favoritas" href={exploreHref({ megusta: 'si' }, 'pistas')} station={FAVORITE_TRACKS} albums={data.likedTracks} />
    {/if}
    {#if data.liked?.length}
      <FilaAlbumes title="Álbumes preferidos" href={exploreHref({ megusta: 'si' })} station={PREFERRED_ALBUMS} albums={data.liked} />
    {/if}
    {#each rotating as row, i (row.kind + ':' + row.value)}
      <FilaAlbumes
        title={rowTitle(row)}
        href={exploreHref(row.facets)}
        station={station(rowTitle(row), { view: 'albumes', facets: row.facets })}
        onrefresh={i === 0 ? refresh : null}
        albums={row.albums} />
    {/each}
  {/if}
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
  .stats {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 14px;
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
