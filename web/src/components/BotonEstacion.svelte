<script>
  // "Estación": starts an endless station. A station without rules gets the
  // defaults; null disables the button (nothing to base a station on).
  import { DEFAULT_RULES, station as makeStation } from '../lib/stations.js'
  import { playStation } from '../lib/player.svelte.js'
  import Icono from './Icono.svelte'

  let { station, label = 'Estación' } = $props()

  const start = () => playStation(station.mode ? station : makeStation(station.name, station.source, DEFAULT_RULES))
</script>

<button class="estacion" disabled={!station} onclick={start} title="Una cola sin fin con música parecida">
  <Icono name="radio" size={14} /> {label}
</button>

<style>
  button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
  }
</style>
