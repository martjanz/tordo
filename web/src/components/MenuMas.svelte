<script>
  // The "⋯" menu for a track, album, folder… ({tracks}, {album}, …): queue
  // actions, adding to a list and, with a rating ({kind, id}), the stars.
  import { app } from '../lib/app.svelte.js'
  import { enqueue } from '../lib/playback.svelte.js'
  import AgregarALista from './AgregarALista.svelte'
  import Icono from './Icono.svelte'
  import Puntaje from './Puntaje.svelte'

  let { what, rating = null } = $props()

  let open = $state(false)
  let adding = $state(false) // the list picker replaces the menu
  let root = $state()

  function choose(next) {
    enqueue(what, { next })
    open = false
  }

  function onwindowclick(event) {
    if (open && !adding && root && !root.contains(event.target)) open = false
  }

  function onkeydown(event) {
    if (open && !adding && event.key === 'Escape') {
      open = false
      root?.querySelector('.more')?.focus()
    }
  }
</script>

<svelte:window onclick={onwindowclick} {onkeydown} />

<span class="menu" bind:this={root}>
  <button class="icon more" onclick={() => (open = !open)} aria-haspopup="menu" aria-expanded={open}
    title="Más opciones" aria-label="Más opciones"><Icono name="more" size={16} /></button>
  {#if open && adding}
    <AgregarALista right {what} onclose={() => ((adding = false), (open = false))} />
  {:else if open}
    <div class="pop" role="menu">
      <button role="menuitem" onclick={() => choose(true)}><Icono name="next_up" size={14} /> Reproducir a continuación</button>
      <button role="menuitem" onclick={() => choose(false)}><Icono name="queue" size={14} /> Agregar a la cola</button>
      {#if !app.status?.readOnly}
        <button role="menuitem" onclick={(e) => (e.stopPropagation(), (adding = true))}><Icono name="lists" size={14} /> Agregar a una lista…</button>
        {#if rating}<div class="rate"><Puntaje kind={rating.kind} id={rating.id} size={16} /></div>{/if}
      {/if}
    </div>
  {/if}
</span>

<style>
  .menu {
    position: relative;
    display: inline-flex;
  }
  .pop {
    position: absolute;
    right: 0;
    top: 100%;
    z-index: 20;
    min-width: 220px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius-lg);
    padding: 4px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.4);
  }
  .pop > button {
    border: none;
    background: none;
    text-align: left;
    padding: 8px 10px;
    border-radius: var(--radius);
    display: flex;
    align-items: center;
    gap: 10px;
    white-space: nowrap;
  }
  .pop > button :global(svg) {
    color: var(--muted);
  }
  .pop > button:hover {
    background: var(--line);
  }
  .rate {
    padding: 4px 6px;
  }
</style>
