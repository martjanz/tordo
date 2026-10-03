<script>
  import Nav from './components/Nav.svelte'
  import Reproductor from './components/Reproductor.svelte'
  import { app, listenMode, stopStatus, watchStatus } from './lib/app.svelte.js'
  import { isHelpKey } from './lib/ayuda.js'
  import { auth, checkAuth } from './lib/auth.svelte.js'
  import { loadMine } from './lib/mine.svelte.js'
  import { navigate, route } from './lib/nav.svelte.js'
  import { appLink, resolve } from './lib/router.js'
  import { pageBlocked } from './lib/status.js'
  import Ajustes from './pages/Ajustes.svelte'
  import Album from './pages/Album.svelte'
  import Ayuda from './pages/Ayuda.svelte'
  import Artista from './pages/Artista.svelte'
  import Bienvenida from './pages/Bienvenida.svelte'
  import Buscar from './pages/Buscar.svelte'
  import Carpetas from './pages/Carpetas.svelte'
  import Entrar from './pages/Entrar.svelte'
  import Estadisticas from './pages/Estadisticas.svelte'
  import Explorar from './pages/Explorar.svelte'
  import Inicio from './pages/Inicio.svelte'
  import Lista from './pages/Lista.svelte'
  import Listas from './pages/Listas.svelte'
  import Revisar from './pages/Revisar.svelte'

  const current = $derived(resolve(route.path, route.search))

  // Until the first-use settings are saved, every page is the assistant.
  $effect(() => {
    if (app.status?.setupPending && current.page !== 'bienvenida') navigate('/bienvenida', { replace: true })
  })

  // Listen mode closes Revisar: a direct visit goes back to Inicio.
  $effect(() => {
    if (pageBlocked(current.page, app.status)) navigate('/', { replace: true })
  })

  // Who is asking: a device on the network without a session sees only the
  // PIN screen, and nothing is requested until it has one.
  checkAuth()
  const ready = $derived(auth.checked && !auth.pinRequired)

  let watching = false
  $effect(() => {
    if (ready && !watching) {
      watching = true
      watchStatus()
    } else if (!ready && watching) {
      watching = false
      stopStatus()
    }
  })

  // The ids of the marks can change after a scan.
  $effect(() => {
    if (!ready) return
    app.generation
    loadMine()
  })

  // Links inside the app change the route instead of reloading the page,
  // so the music keeps playing.
  function onclick(event) {
    const to = appLink(event, event.target.closest?.('a'), location.origin)
    if (to !== null) {
      event.preventDefault()
      navigate(to)
    }
  }

  // "?" opens the help from anywhere (not while typing).
  function onkeydown(event) {
    if (!ready || !isHelpKey(event)) return
    event.preventDefault()
    navigate('/ayuda')
  }
</script>

<svelte:window {onclick} {onkeydown} />

{#if !ready}
  <Nav page="" bare />
  {#if auth.checked}
    <main><Entrar /></main>
  {/if}
{:else}
<Nav page={current.page} bare={current.page === 'bienvenida'} />

<main>
  {#if current.page === 'inicio'}
    <Inicio />
  {:else if current.page === 'estadisticas'}
    <Estadisticas period={current.period} />
  {:else if current.page === 'explorar'}
    <Explorar />
  {:else if current.page === 'carpetas'}
    <Carpetas path={current.path} />
  {:else if current.page === 'album'}
    {#key current.id}
      <Album id={current.id} />
    {/key}
  {:else if current.page === 'artista'}
    {#key current.key}
      <Artista key={current.key} />
    {/key}
  {:else if current.page === 'buscar'}
    <Buscar q={current.q} />
  {:else if current.page === 'listas'}
    <Listas />
  {:else if current.page === 'lista'}
    {#key current.id}
      <Lista id={current.id} />
    {/key}
  {:else if current.page === 'revisar' && !listenMode()}
    <Revisar tab={current.tab} kind={current.kind} />
  {:else if current.page === 'ajustes'}
    <Ajustes />
  {:else if current.page === 'ayuda'}
    <Ayuda />
  {:else if current.page === 'bienvenida'}
    <Bienvenida />
  {:else}
    <p class="empty">No existe esta página. <a href="/">Ir al Inicio</a></p>
  {/if}
</main>

<Reproductor />
{/if}

<div class="notices" aria-live="polite">
  {#each app.notices as n (n.id)}
    <div class="notice">{n.text}</div>
  {/each}
</div>

<style>
  .notices {
    position: fixed;
    top: 64px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 8px;
    z-index: 40;
    width: min(560px, calc(100% - 32px));
  }
  .notice {
    background: var(--surface-2);
    border: 1px solid var(--warn);
    border-radius: var(--radius);
    padding: 8px 12px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
  }
</style>
