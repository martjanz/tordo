<script>
  // The folders as they are on disk: a tree on the left, the open folder on the right.
  import ContenidoCarpeta from '../components/ContenidoCarpeta.svelte'
  import { api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { syncGeneration } from '../lib/carpetas.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { folderHref } from '../lib/router.js'

  let { path } = $props()

  let folder = $state(null)
  let error = $state('')
  let rootPath = $state('') // the only root, when there is just one (the narrow layout needs it)

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
          navigate(folderHref(rootPath), { replace: true })
          return
        }
        if (!p && f.children.length === 0) {
          folder = null
          error = 'Todavía no hay música en el catálogo. Revisá las carpetas de config.json.'
          return
        }
        folder = f
        error = ''
      })
      .catch((e) => {
        if (stale) return
        folder = null
        error = e.status === 404 ? 'Esta carpeta ya no está en el catálogo.' : e.message
      })
    return () => (stale = true)
  })
</script>

<div class="explorer">
  <aside class="tree" aria-label="Carpetas">
    <!-- the folder tree goes here -->
  </aside>
  <section class="content">
    {#if error}
      <p class="empty">{error}</p>
    {:else if folder}
      <ContenidoCarpeta {folder} />
    {/if}
  </section>
</div>

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
</style>
