<script>
  // Play, shuffle and queue buttons for an album, folder or collection; with
  // a station prop (null when none fits) also its "Estación" button.
  import { app } from '../lib/app.svelte.js'
  import { enqueue, play } from '../lib/player.svelte.js'
  import BotonEstacion from './BotonEstacion.svelte'
  import AgregarALista from './AgregarALista.svelte'
  import Icono from './Icono.svelte'

  let { what, station = undefined } = $props()

  let adding = $state(false)
</script>

<div class="actions">
  <button class="primary" onclick={() => play(what)}><Icono name="play" size={14} /> Reproducir</button>
  <button onclick={() => play(what, { shuffle: true })}><Icono name="shuffle" size={14} /> Aleatorio</button>
  {#if station}<BotonEstacion {station} />{/if}
  <button onclick={() => enqueue(what, { next: true })}><Icono name="next_up" size={14} /> A continuación</button>
  <button onclick={() => enqueue(what)}><Icono name="plus" size={14} /> A la cola</button>
  {#if !app.status?.readOnly}
    <span class="add">
      <button onclick={(e) => (e.stopPropagation(), (adding = !adding))} aria-haspopup="dialog" aria-expanded={adding}><Icono name="lists" size={14} /> + Lista</button>
      {#if adding}<AgregarALista {what} onclose={() => (adding = false)} />{/if}
    </span>
  {/if}
</div>

<style>
  .add {
    position: relative;
    display: inline-block;
  }
  button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
  }
</style>
