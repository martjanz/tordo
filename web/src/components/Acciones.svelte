<script>
  // The play buttons for an album, folder, artist or list: Reproducir and
  // Aleatorio, then the "Estación" icon (with a station prop; null when none
  // fits) and a "⋯" menu with the queue and list actions. One row, even on a
  // phone.
  import { play } from '../lib/playback.svelte.js'
  import BotonEstacion from './BotonEstacion.svelte'
  import Icono from './Icono.svelte'
  import MenuMas from './MenuMas.svelte'

  let { what, station = undefined } = $props()
</script>

<div class="actions">
  <button class="primary" onclick={() => play(what)}><Icono name="play" size={14} /> Reproducir</button>
  <button onclick={() => play(what, { shuffle: true })}><Icono name="shuffle" size={14} /> Aleatorio</button>
  {#if station}<BotonEstacion {station} compact />{/if}
  <MenuMas {what} />
</div>

<style>
  .actions {
    align-items: center;
    flex-wrap: nowrap;
  }
  button:not(.icon) {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 16px;
    white-space: nowrap;
  }
  .actions :global(.more) {
    padding: 8px;
  }
</style>
