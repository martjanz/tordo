<script>
  // A small menu for a track: queue actions and rating.
  import { app } from '../lib/app.svelte.js'
  import { enqueue } from '../lib/player.svelte.js'
  import AgregarALista from './AgregarALista.svelte'
  import Icono from './Icono.svelte'
  import Puntaje from './Puntaje.svelte'

  let { track } = $props()

  let open = $state(false)
  let adding = $state(false) // the list picker replaces the menu
  let root = $state()

  function choose(next) {
    enqueue({ tracks: [track.id] }, { next })
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
    <AgregarALista right what={{ tracks: [track.id] }} onclose={() => ((adding = false), (open = false))} />
  {:else if open}
    <div class="pop" role="menu">
      <button role="menuitem" onclick={() => choose(true)}>Reproducir a continuación</button>
      <button role="menuitem" onclick={() => choose(false)}>Agregar a la cola</button>
      {#if !app.status?.readOnly}
        <button role="menuitem" onclick={(e) => (e.stopPropagation(), (adding = true))}>Agregar a una lista…</button>
        <div class="rate"><Puntaje kind="track" id={track.id} size={16} /></div>
      {/if}
    </div>
  {/if}
</span>

<style>
  .menu {
    position: relative;
  }
  .pop {
    position: absolute;
    right: 0;
    top: 100%;
    z-index: 20;
    min-width: 220px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: 8px;
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
    border-radius: 6px;
  }
  .pop > button:hover {
    background: var(--line);
  }
  .rate {
    padding: 4px 6px;
  }
</style>
