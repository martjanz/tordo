<script>
  // Explorar: albums, artists or tracks of the whole catalog, filtered by
  // facets that live in the URL, loaded a page at a time as the list scrolls.
  import { tick } from 'svelte'
  import BarraFacetas from '../components/BarraFacetas.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import TarjetaArtista from '../components/TarjetaArtista.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { parse, toSearch } from '../lib/facets.js'
  import { navigate, patchState, route, savedScroll } from '../lib/nav.svelte.js'
  import { play } from '../lib/player.svelte.js'

  const PAGE = 120

  let data = $state(null) // the first page, with the facets and total
  let items = $state([]) // every item loaded so far
  let more = false // a next page is loading
  let sentinel = $state()
  let loadedSearch = null
  let loads = 0

  const query = $derived(parse(route.search))
  const key = (v) => (v === 'artistas' ? 'artists' : v === 'pistas' ? 'tracks' : 'albums')

  // Load on every change of the URL, and again when a scan or an
  // identification run ends (same URL: the list refreshes in place).
  $effect(() => {
    const search = route.search
    app.generation
    load(search)
  })

  async function load(search) {
    const id = ++loads
    const fresh = search !== loadedSearch
    // Coming back to a long list: load as much of it as was shown.
    const want = fresh ? Math.max(PAGE, history.state?.shown ?? PAGE) : Math.max(PAGE, items.length)
    let d
    try {
      d = await api.explore(search, 0, Math.min(500, want))
    } catch (e) {
      if (id === loads) notify(e.message)
      return
    }
    if (id !== loads || (route.path !== '/explorar' && route.path !== '/')) return // a newer request is on its way
    // The server drops malformed facets: show the URL of what it applied.
    const applied = toSearch(d.query)
    if (applied !== toSearch(parse(search))) {
      navigate('/explorar' + applied, { replace: true })
      return
    }
    data = d
    items = d[key(d.query.view)] ?? []
    loadedSearch = search
    if (fresh) {
      await tick()
      window.scrollTo(0, savedScroll())
    }
  }

  async function loadMore() {
    if (more || !data || items.length >= data.total) return
    more = true
    const id = loads
    try {
      const d = await api.explore(loadedSearch, items.length, PAGE)
      if (id === loads) {
        items = [...items, ...(d[key(d.query.view)] ?? [])]
        patchState({ shown: items.length })
      }
    } catch (e) {
      notify(e.message)
    } finally {
      more = false
    }
  }

  // playFrom plays the listed tracks from t on (a long list in parts: the
  // tracks go in the request's URL).
  function playFrom(t) {
    const i = items.indexOf(t)
    play({ tracks: items.slice(i, i + 500).map((x) => x.id) })
  }

  function change(q) {
    navigate('/explorar' + toSearch(q))
  }

  // Load the next page when the end of the list comes into view.
  $effect(() => {
    if (!sentinel) return
    const io = new IntersectionObserver((entries) => entries[0].isIntersecting && loadMore(), { rootMargin: '800px' })
    io.observe(sentinel)
    return () => io.disconnect()
  })

  const scanning = $derived(!!app.status?.scan?.running)
  const filtered = $derived(Object.keys(query.facets).length > 0)
</script>

{#if data}
  <BarraFacetas query={data.query} facets={data.facets} total={data.total} onchange={change} />
  {#if items.length}
    {#if data.query.view === 'pistas'}
      <div class="list">
        <ListaPistas
          tracks={items}
          numbered={false}
          showAlbum
          onplay={playFrom} />
      </div>
    {:else}
      <div class="grid list">
        {#each items as it (data.query.view === 'artistas' ? it.key : it.id)}
          {#if data.query.view === 'artistas'}
            <TarjetaArtista artist={it} />
          {:else}
            <TarjetaAlbum album={it} />
          {/if}
        {/each}
      </div>
    {/if}
    <div bind:this={sentinel}></div>
  {:else if filtered}
    <p class="empty">Nada con estos filtros. <a href="/explorar">Quitar filtros</a></p>
  {:else if scanning}
    <p class="empty">Escaneando… La música va a aparecer a medida que se encuentre.</p>
  {:else}
    <p class="empty">Todavía no hay música en el catálogo. Revisá las carpetas de config.json.</p>
  {/if}
{/if}

<style>
  .list {
    margin-top: 16px;
  }
</style>
