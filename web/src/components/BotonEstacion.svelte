<script>
  // "Estación": starts an endless station. A station without rules gets the
  // defaults; null disables the button (nothing to base a station on). With
  // compact, only the icon, its label as the tooltip.
  import { DEFAULT_RULES, station as makeStation } from '../lib/stations.js'
  import { can, playStation } from '../lib/playback.svelte.js'
  import Icono from './Icono.svelte'

  let { station, label = 'Estación', compact = false } = $props()

  const hint = 'Una cola sin fin con música parecida'

  const start = () => playStation(station.mode ? station : makeStation(station.name, station.source, DEFAULT_RULES))
</script>

{#if can('playStation')}
  {#if compact}
    <button class="icon estacion" disabled={!station} onclick={start} title="{label}: {hint.toLowerCase()}" aria-label={label}>
      <Icono name="radio" size={18} />
    </button>
  {:else}
    <button class="estacion" disabled={!station} onclick={start} title={hint}>
      <Icono name="radio" size={14} /> {label}
    </button>
  {/if}
{/if}

<style>
  button:not(.icon) {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
  }
</style>
