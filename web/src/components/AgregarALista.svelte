<script>
  // A popover to add something ({album}, {folder}, {tracks}…) to a playlist:
  // pick one, or create a new one with it. The parent must be positioned;
  // up opens it above the parent (in the player bar).
  import { api } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'

  let { what, onclose, right = false, up = false } = $props()

  let lists = $state(null)
  let name = $state('')
  let busy = $state(false)
  let root = $state()

  $effect(() => {
    let stale = false
    api
      .playlists()
      .then((ls) => {
        if (!stale) lists = [...ls].sort((a, b) => (b.updatedAt > a.updatedAt ? 1 : b.updatedAt < a.updatedAt ? -1 : 0))
      })
      .catch((e) => {
        if (!stale) (lists = []), notify(e.message)
      })
    return () => (stale = true)
  })

  async function addTo(list) {
    if (busy) return
    busy = true
    try {
      const { added } = await api.addToPlaylist(list.id, what)
      notify(`${count(added, 'pista agregada', 'pistas agregadas')} a «${list.name}».`)
      onclose()
    } catch (e) {
      notify(e.message)
      busy = false
    }
  }

  async function create(event) {
    event.preventDefault()
    const n = name.trim()
    if (!n || busy) return
    busy = true
    try {
      const list = await api.createPlaylist(n, what)
      notify(`Lista «${list.name}» creada con ${count(list.tracks, 'pista', 'pistas')}.`)
      onclose()
    } catch (e) {
      notify(e.message)
      busy = false
    }
  }

  function onwindowclick(event) {
    if (root && !root.contains(event.target)) onclose()
  }

  function onkeydown(event) {
    if (event.key === 'Escape') onclose()
  }
</script>

<svelte:window onclick={onwindowclick} {onkeydown} />

<div class="pop" class:right class:up bind:this={root} role="dialog" aria-label="Agregar a una lista">
  {#if lists === null}
    <p class="sub">Cargando…</p>
  {:else if lists.length}
    <ul>
      {#each lists as l (l.id)}
        <li><button disabled={busy} onclick={() => addTo(l)}>{l.name} <span class="sub">{count(l.tracks, 'pista', 'pistas')}</span></button></li>
      {/each}
    </ul>
  {/if}
  <form onsubmit={create}>
    <input bind:value={name} placeholder="Nueva lista…" aria-label="Nombre de la nueva lista" maxlength="200" />
    <button class="primary" disabled={busy || !name.trim()}>Crear</button>
  </form>
</div>

<style>
  .pop {
    position: absolute;
    left: 0;
    top: 100%;
    margin-top: 4px;
    z-index: 30;
    width: 280px;
    max-width: calc(100vw - 2 * var(--gutter));
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: 8px;
    padding: 6px;
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.4);
    text-align: left;
  }
  .right {
    left: auto;
    right: 0;
  }
  .up {
    top: auto;
    bottom: 100%;
    margin: 0 0 4px;
  }
  ul {
    list-style: none;
    margin: 0 0 6px;
    padding: 0;
    max-height: 240px;
    overflow-y: auto;
  }
  li button {
    width: 100%;
    display: flex;
    justify-content: space-between;
    gap: 8px;
    border: none;
    background: none;
    text-align: left;
    padding: 8px 10px;
    border-radius: 6px;
  }
  li button:hover:not(:disabled) {
    background: var(--line);
  }
  .sub {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  p.sub {
    margin: 6px 10px;
  }
  form {
    display: flex;
    gap: 6px;
  }
  input {
    flex: 1;
    min-width: 0;
  }
</style>
