<script>
  // A station card: a square with the radio icon, its name and its rules.
  import { DEFAULT_RULES, station as makeStation, summary } from '../lib/stations.js'
  import Icono from './Icono.svelte'

  let { station, onplay, ondelete = null } = $props()

  // Built-ins carry no rules: they get the defaults.
  const full = $derived(station.mode ? station : makeStation(station.name, station.source, DEFAULT_RULES))

  function remove() {
    if (confirm(`¿Eliminar la estación «${station.name}»?`)) ondelete(station)
  }
</script>

<div class="card">
  <button class="pic" onclick={() => onplay(full)} aria-label="Reproducir la estación {station.name}">
    <Icono name="radio" size={40} />
  </button>
  <span class="title">{station.name}</span>
  <span class="sub">{summary(full)}</span>
  {#if ondelete}
    <details class="menu">
      <summary aria-label="Más opciones de {station.name}"><Icono name="more" size={16} /></summary>
      <div class="pop"><button onclick={remove}>Eliminar</button></div>
    </details>
  {/if}
</div>

<style>
  .card {
    position: relative;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .pic {
    aspect-ratio: 1;
    width: 100%;
    margin-bottom: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: color-mix(in srgb, var(--accent) 18%, var(--surface-2));
    color: var(--accent);
    border-radius: var(--radius);
  }
  .title,
  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
  }
  .menu {
    position: absolute;
    top: 6px;
    right: 6px;
  }
  .menu summary {
    list-style: none;
    cursor: pointer;
    display: flex;
    padding: 4px;
    border-radius: 50%;
    background: rgba(0, 0, 0, 0.45);
    color: var(--strong);
  }
  .menu summary::-webkit-details-marker {
    display: none;
  }
  .pop {
    position: absolute;
    right: 0;
    margin-top: 4px;
    z-index: 5;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 4px;
  }
  .pop button {
    white-space: nowrap;
  }
</style>
