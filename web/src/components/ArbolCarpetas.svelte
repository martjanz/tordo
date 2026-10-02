<script>
  // The folder tree: lazy levels, filter by name, keyboard navigation (WAI-ARIA tree).
  import { tick, untrack } from 'svelte'
  import { api } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { loadLevel, revealPath, setOpen, tree } from '../lib/carpetas.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { folderHref } from '../lib/router.js'
  import { MIN } from '../lib/search.js'
  import { filterOpen, isFilterKey, keyAction, missingLevels, visibleRows } from '../lib/tree.js'
  import Icono from './Icono.svelte'

  let { selected } = $props()

  let cursor = $state('')
  let query = $state('')
  let result = $state(null)
  let input = $state(null)
  let list = $state(null)

  const matching = $derived(new Set(result?.matches ?? []))
  const openMap = $derived(result ? filterOpen(result.levels) : tree.open)
  const rows = $derived(result ? visibleRows(result.levels, openMap) : visibleRows(tree.levels, tree.open))
  const hasCursor = $derived(rows.some((r) => r.node.path === cursor))

  function rowEl(path) {
    return [...(list?.querySelectorAll('li[role="treeitem"]') ?? [])].find((el) => el.dataset.path === path)
  }

  async function show(path, focus) {
    await tick()
    const el = rowEl(path)
    if (!el) return
    if (focus) el.focus()
    el.scrollIntoView({ block: 'nearest' })
  }

  // 1. Show the selected folder: load and open what leads to it, put the cursor on it.
  $effect(() => {
    const target = selected
    tree.generation // a scan resets the levels: reveal again
    let stale = false
    untrack(() => revealPath(target))
      .then((r) => {
        if (stale || !r.node) return
        cursor = r.node
        show(r.node, false)
      })
      .catch((e) => notify(e.message))
    return () => (stale = true)
  })

  // 2. Lazy levels: load the open branches that are not loaded yet.
  const loading = new Set()
  $effect(() => {
    for (const key of missingLevels(tree.levels, tree.open)) {
      if (loading.has(key)) continue
      loading.add(key)
      loadLevel(key)
        .catch((e) => notify(e.message))
        .finally(() => loading.delete(key))
    }
  })

  // 3. Filter, debounced; stale responses are ignored.
  $effect(() => {
    const q = query
    if (q.trim().length < MIN) {
      result = null
      return
    }
    let stale = false
    const timer = setTimeout(() => {
      api
        .folderTreeSearch(q)
        .then((r) => {
          if (!stale) result = r
        })
        .catch((e) => {
          if (!stale) notify(e.message)
        })
    }, 200)
    return () => {
      stale = true
      clearTimeout(timer)
    }
  })

  function toggle(event, row) {
    event.stopPropagation()
    if (result) return
    setOpen(row.node.path, !tree.open[row.node.path])
  }

  function click(path) {
    cursor = path
    navigate(folderHref(path))
  }

  function onkeydown(event) {
    const action = keyAction(rows, cursor, event.key, openMap)
    if (!action) return
    event.preventDefault()
    if (action.type === 'cursor') {
      cursor = action.path
      show(action.path, true)
    } else if (action.type === 'open' || action.type === 'close') {
      if (!result) setOpen(action.path, action.type === 'open')
    } else if (action.type === 'show') {
      navigate(folderHref(action.path))
    }
  }

  function windowKeydown(event) {
    if (isFilterKey(event)) {
      event.preventDefault()
      input?.focus()
    }
  }

  function inputKeydown(event) {
    if (event.key === 'Escape') {
      event.preventDefault()
      query = ''
      result = null
      const path = hasCursor ? cursor : rows[0]?.node.path
      if (path) show(path, true)
    } else if (event.key === 'ArrowDown' && rows.length) {
      event.preventDefault()
      const path = hasCursor ? cursor : rows[0].node.path
      cursor = path
      show(path, true)
    }
  }
</script>

<svelte:window onkeydown={windowKeydown} />

<div class="panel">
  <div class="filter">
    <input
      type="search"
      placeholder="Filtrar carpetas…"
      aria-label="Filtrar carpetas"
      bind:value={query}
      bind:this={input}
      onkeydown={inputKeydown}
    />
  </div>
  <ul role="tree" aria-label="Carpetas" {onkeydown} bind:this={list}>
    {#each rows as row, i (row.node.path)}
      {@const node = row.node}
      {@const isOpen = !!openMap[node.path]}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <li
        role="treeitem"
        aria-level={row.depth}
        aria-expanded={node.hasChildren ? isOpen : undefined}
        aria-selected={node.path === selected}
        tabindex={node.path === cursor || (!hasCursor && i === 0) ? 0 : -1}
        data-path={node.path}
        style:padding-left="calc(({row.depth} - 1) * 16px + 4px)"
        onclick={() => click(node.path)}
      >
        <span class="toggle" class:open={isOpen} class:none={!node.hasChildren} onclick={(e) => toggle(e, row)} aria-hidden="true">
          {#if node.hasChildren}<Icono name="chevron_right" size={14} />{/if}
        </span>
        <Icono name={node.album ? 'disc' : 'folder'} size={15} />
        <span class="name" class:hit={matching.has(node.path)} title={node.label}>{node.label}</span>
        {#if node.collection}<span class="label">Colección</span>{/if}
        <span class="tracks">{node.tracks.toLocaleString('es')}</span>
      </li>
    {/each}
  </ul>
  {#if result?.matches.length === 0}
    <p class="note">Ninguna carpeta se llama así.</p>
  {:else if result?.more}
    <p class="note">Hay más coincidencias: escribí más para acotar.</p>
  {/if}
</div>

<style>
  .filter {
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--bg);
    padding-bottom: 8px;
  }
  .filter input {
    width: 100%;
    box-sizing: border-box;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 30px;
    box-sizing: border-box;
    padding-right: 6px;
    font-size: 14px;
    cursor: pointer;
    border-radius: 4px;
    white-space: nowrap;
  }
  li:hover {
    background: var(--surface);
  }
  li[aria-selected='true'] {
    background: var(--surface-2);
    color: var(--strong);
  }
  .toggle {
    display: inline-flex;
    width: 16px;
    flex: none;
    justify-content: center;
    color: var(--muted);
  }
  .toggle :global(svg) {
    transition: transform 0.12s;
  }
  .toggle.open :global(svg) {
    transform: rotate(90deg);
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .name.hit {
    font-weight: 700;
  }
  .tracks {
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    font-size: 12px;
  }
  .note {
    color: var(--muted);
    font-size: 13px;
    margin: 8px 4px;
  }
</style>
