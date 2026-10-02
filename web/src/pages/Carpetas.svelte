<script>
  // The folders as they are on disk: a tree on the left, the open folder on the right.
  import ArbolCarpetas from '../components/ArbolCarpetas.svelte'
  import ContenidoCarpeta from '../components/ContenidoCarpeta.svelte'
  import ListaCarpeta from '../components/ListaCarpeta.svelte'
  import { api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { syncGeneration } from '../lib/carpetas.svelte.js'
  import { navigate, savedScroll } from '../lib/nav.svelte.js'
  import { folderHref } from '../lib/router.js'
  import { direction } from '../lib/tree.js'
  import { onMount, tick } from 'svelte'
  import { fly } from 'svelte/transition'

  let { path } = $props()

  let folder = $state(null)
  let error = $state('')
  let rootPath = $state('') // the only root, when there is just one (the narrow layout needs it)
  let rootKnown = $state(false)

  // Narrow screens show one list per folder instead of the two panels.
  let narrow = $state(matchMedia('(max-width: 720px)').matches)
  let reduced = $state(matchMedia('(prefers-reduced-motion: reduce)').matches)
  let dir = 'none' // how the shown folder changed: 'forward' | 'back' | 'none'
  let shown = null // the folder path currently shown
  // The first load after mount restores the scroll too: arriving by going back from another page
  // fires popstate before this page exists. Later, only popstate sets it.
  let popped = true
  let rescroll = null
  const duration = $derived(reduced ? 0 : 220)

  onMount(() => {
    const wide = matchMedia('(max-width: 720px)')
    const motion = matchMedia('(prefers-reduced-motion: reduce)')
    const onWide = (e) => (narrow = e.matches)
    const onMotion = (e) => (reduced = e.matches)
    const onPop = () => (popped = true)
    wide.addEventListener('change', onWide)
    motion.addEventListener('change', onMotion)
    window.addEventListener('popstate', onPop)
    return () => {
      wide.removeEventListener('change', onWide)
      motion.removeEventListener('change', onMotion)
      window.removeEventListener('popstate', onPop)
      clearTimeout(rescroll)
    }
  })

  // After a back/forward the page returns to where it was (a push scrolls to 0 in navigate).
  async function restoreScroll() {
    if (!popped) return
    popped = false
    clearTimeout(rescroll)
    await tick()
    window.scrollTo(0, savedScroll())
    // The leaving screen still takes room while it slides out.
    if (narrow && duration) rescroll = setTimeout(() => window.scrollTo(0, savedScroll()), duration + 30)
  }

  // The root does not depend on the entry route (a deep link or reload lands below it).
  $effect(() => {
    if (rootKnown) return
    api
      .folder('')
      .then((f) => {
        rootPath = f.children.length === 1 ? f.children[0].path : ''
        rootKnown = true
      })
      .catch(() => {})
  })

  $effect(() => {
    const p = path
    const generation = app.generation // reload when a scan ends
    syncGeneration(generation)
    let stale = false
    api
      .folder(p)
      .then((f) => {
        if (stale) return
        // With a single root, the top level is that root.
        if (!p && f.children.length === 1) {
          rootPath = f.children[0].path
          rootKnown = true
          navigate(folderHref(rootPath), { replace: true })
          return
        }
        if (!p && f.children.length === 0) {
          folder = null
          error = 'Todavía no hay música en el catálogo. Revisá las carpetas de config.json.'
          return
        }
        dir = shown === null ? 'none' : direction(shown, f.path)
        shown = f.path
        folder = f
        error = ''
        restoreScroll()
      })
      .catch((e) => {
        if (stale) return
        popped = false
        folder = null
        error = e.status === 404 ? 'Esta carpeta ya no está en el catálogo.' : e.message
      })
    return () => (stale = true)
  })
</script>

{#if narrow}
  <div class="stack">
    {#if error}
      <p class="empty">{error}</p>
    {:else if folder && rootKnown}
      {#key folder.path}
        <div
          class="screen"
          in:fly={{ x: dir === 'back' ? '-30%' : '100%', duration }}
          out:fly={{ x: dir === 'back' ? '100%' : '-30%', duration }}>
          <ListaCarpeta path={folder.path} {rootPath} {folder} />
        </div>
      {/key}
    {/if}
  </div>
{:else}
  <div class="explorer">
    <aside class="tree" aria-label="Carpetas">
      <ArbolCarpetas selected={path} />
    </aside>
    <section class="content">
      {#if error}
        <p class="empty">{error}</p>
      {:else if folder}
        <ContenidoCarpeta {folder} />
      {/if}
    </section>
  </div>
{/if}

<style>
  .explorer {
    display: grid;
    grid-template-columns: minmax(220px, 300px) minmax(0, 1fr);
    gap: 24px;
  }
  .tree {
    position: sticky;
    top: var(--nav-h);
    align-self: start;
    max-height: calc(100dvh - var(--nav-h) - var(--player-h) - var(--tabs-h));
    overflow: auto;
  }
  .content {
    min-width: 0;
  }
  .stack {
    display: grid;
    overflow-x: clip; /* clip, not hidden: hidden would break the sticky header */
  }
  .stack > :global(*) {
    grid-area: 1 / 1;
    min-width: 0;
  }
</style>
