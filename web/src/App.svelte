<script>
  import Nav from './components/Nav.svelte'
  import Reproductor from './components/Reproductor.svelte'
  import { app, watchStatus } from './lib/app.svelte.js'
  import { navigate, route } from './lib/nav.svelte.js'
  import { appLink, resolve } from './lib/router.js'
  import Album from './pages/Album.svelte'
  import Carpetas from './pages/Carpetas.svelte'
  import Listas from './pages/Listas.svelte'

  const current = $derived(resolve(route.path, route.search))

  watchStatus()

  // Links inside the app change the route instead of reloading the page,
  // so the music keeps playing.
  function onclick(event) {
    const to = appLink(event, event.target.closest?.('a'), location.origin)
    if (to !== null) {
      event.preventDefault()
      navigate(to)
    }
  }
</script>

<svelte:window {onclick} />

<Nav page={current.page} />

<main>
  {#if current.page === 'carpetas'}
    {#key current.path}
      <Carpetas path={current.path} />
    {/key}
  {:else if current.page === 'album'}
    {#key current.id}
      <Album id={current.id} />
    {/key}
  {:else if current.page === 'listas'}
    <Listas />
  {:else}
    <p class="empty">No existe esta página. <a href="/carpetas">Ir a Carpetas</a></p>
  {/if}
</main>

<Reproductor />

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
