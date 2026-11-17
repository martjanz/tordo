<script>
  // "Estación": starts an endless station. A station without rules gets the
  // defaults; null disables the button (nothing to base a station on).
  import { t } from '../lib/i18n/index.svelte.js'
  import { DEFAULT_RULES, station as makeStation } from '../lib/stations.js'
  import { can, playStation } from '../lib/playback.svelte.js'
  import Icono from './Icono.svelte'

  let { station, label = undefined } = $props()

  const start = () => playStation(station.mode ? station : makeStation(station.name, station.source, DEFAULT_RULES))
</script>

{#if can('playStation')}
  <button class="estacion" disabled={!station} onclick={start} title={t('station.hint')}>
    <Icono name="radio" size={14} /> {label ?? t('station.label')}
  </button>
{/if}

<style>
  button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 14px;
  }
</style>
