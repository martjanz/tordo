<script>
  // The instant search in the header: Ctrl+K / ⌘K opens it, results come
  // while typing, arrows and Enter pick one.
  import { tick } from 'svelte'
  import { albumCoverURL, api, itemCoverURL } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { navigate, route } from '../lib/nav.svelte.js'
  import { searchHref } from '../lib/router.js'
  import { enterHref, isShortcut, move, options, searchable, total } from '../lib/search.js'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  const GROUPS = { artist: 'Artistas', album: 'Álbumes', track: 'Pistas', folder: 'Carpetas' }

  let open = $state(false)
  let q = $state('')
  let result = $state(null)
  let selected = $state(-1)
  let input = $state()
  let box = $state()
  let timer
  let asks = 0

  const opts = $derived(options(result))
  const found = $derived(total(result))

  async function show() {
    open = true
    await tick()
    input?.focus()
    input?.select()
  }

  function close() {
    open = false
    selected = -1
  }

  // Ask after a pause in the typing; answers to older queries are dropped.
  function oninput() {
    clearTimeout(timer)
    selected = -1
    if (!searchable(q)) {
      asks++
      result = null
      return
    }
    timer = setTimeout(ask, 150)
  }

  async function ask() {
    const id = ++asks
    try {
      const r = await api.find(q, 4)
      if (id === asks) result = r
    } catch (e) {
      if (id === asks) notify(e.message)
    }
  }

  function go(href) {
    if (!href) return
    close()
    navigate(href)
  }

  function onkeydown(event) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      selected = move(selected, event.key, opts.length)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      go(enterHref(selected, opts, q))
    } else if (event.key === 'Escape') {
      close()
    }
  }

  function onwindowkeydown(event) {
    if (isShortcut(event)) {
      event.preventDefault()
      show()
    }
  }

  // The path is fixed when the click starts: the button that opened the
  // search is gone from the page by the time the click reaches the window.
  function onwindowclick(event) {
    if (open && box && !event.composedPath().includes(box)) close()
  }

  // Following a result (or any link) closes the search.
  $effect(() => {
    route.path
    route.search
    close()
  })

  function picture(o) {
    switch (o.kind) {
      case 'artist':
        return o.item.cover ? [albumCoverURL(o.item.cover, 160)] : []
      case 'album':
        return [albumCoverURL(o.item.id, 160)]
      case 'track':
        return [itemCoverURL(o.item, 160)]
    }
    return []
  }

  function meta(o) {
    const it = o.item
    switch (o.kind) {
      case 'artist':
        return count(it.albums, 'álbum', 'álbumes')
      case 'album':
        return [it.artist, it.year || ''].filter(Boolean).join(' · ')
      case 'track':
        return [it.artist, it.album].filter(Boolean).join(' · ')
    }
    return it.collection ? 'Colección' : count(it.tracks, 'pista', 'pistas')
  }

  const ICONS = { artist: 'user', album: 'disc', track: 'disc', folder: 'folder' }
</script>

<svelte:window onkeydown={onwindowkeydown} onclick={onwindowclick} />

<div class="busqueda" bind:this={box}>
  {#if open}
    <input
      bind:this={input}
      bind:value={q}
      {oninput}
      {onkeydown}
      type="search"
      placeholder="Artista, álbum, tema, carpeta…"
      aria-label="Buscar en el catálogo"
      autocomplete="off"
      spellcheck="false"
    />
    {#if result && searchable(q)}
      <div class="panel" role="listbox">
        {#if found === 0}
          <p class="none">Nada con «{q}».</p>
        {/if}
        {#each opts as o, i (o.kind + (o.item.key ?? o.item.id ?? o.item.path))}
          {#if i === 0 || opts[i - 1].kind !== o.kind}
            <div class="label group">{GROUPS[o.kind]}</div>
          {/if}
          <a class="option" class:selected={selected === i} href={o.href} onclick={close} role="option" aria-selected={selected === i}>
            <span class="thumb"><Tapa srcs={picture(o)} icon={ICONS[o.kind]} round={o.kind === 'artist'} /></span>
            <span class="text">
              <span class="title">{o.item.name ?? o.item.title}</span>
              <span class="meta">{meta(o)}</span>
            </span>
          </a>
        {/each}
        {#if found > opts.length}
          <a class="all" href={searchHref(q)} onclick={close}>Ver los {found.toLocaleString('es')} resultados →</a>
        {/if}
      </div>
    {/if}
  {:else}
    <button class="icon" onclick={show} title="Buscar (Ctrl+K)" aria-label="Buscar">
      <Icono name="search" size={18} />
    </button>
  {/if}
</div>

<style>
  .busqueda {
    position: relative;
  }
  input {
    width: 280px;
  }
  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 8px);
    width: 380px;
    max-height: calc(100vh - 80px);
    overflow-y: auto;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.6);
    padding: 6px 0;
    z-index: 40;
  }
  .option {
    display: flex;
    gap: 10px;
    align-items: center;
    padding: 5px 12px;
  }
  .option.selected,
  .option:hover {
    background: var(--line);
  }
  .thumb {
    flex: none;
    width: 36px;
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .title,
  .meta {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .title {
    color: var(--strong);
  }
  .meta {
    font-size: 12px;
    color: var(--faint);
  }
  .group {
    padding: 10px 12px 4px;
  }
  .none {
    margin: 0;
    padding: 8px 12px;
    color: var(--muted);
  }
  .all {
    display: block;
    padding: 8px 12px 4px;
    font-size: 13px;
    color: var(--accent);
    border-top: 1px solid var(--line);
    margin-top: 6px;
  }
  @media (max-width: 640px) {
    .busqueda:has(input) {
      position: static;
    }
    input {
      position: absolute;
      left: var(--gutter);
      right: var(--gutter);
      top: 8px;
      width: auto;
      z-index: 41;
    }
    .panel {
      left: var(--gutter);
      right: var(--gutter);
      width: auto;
      top: 52px;
    }
  }
</style>
