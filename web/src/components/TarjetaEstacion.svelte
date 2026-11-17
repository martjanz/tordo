<script>
  // A station card: its name set on an accent-lit gradient with a radio badge,
  // then its name and its rules.
  import { t } from '../lib/i18n/index.svelte.js'
  import { DEFAULT_RULES, station as makeStation, summary } from '../lib/stations.js'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  let { station, onplay, ondelete = null } = $props()

  // Built-ins carry no rules: they get the defaults.
  const full = $derived(station.mode ? station : makeStation(station.name, station.source, DEFAULT_RULES))

  function remove() {
    if (confirm(t('station.confirmDelete', { name: station.name }))) ondelete(station)
  }
</script>

<div class="card">
  {#if onplay}
    <button class="pic" onclick={() => onplay(full)} aria-label={t('station.playNamed', { name: station.name })}>
      <Tapa name={station.name} badge="radio" tint="var(--accent)" />
    </button>
  {:else}
    <span class="pic"><Tapa name={station.name} badge="radio" tint="var(--accent)" /></span>
  {/if}
  <span class="title">{station.name}</span>
  <span class="sub" title={summary(full)}>{summary(full)}</span>
  {#if ondelete}
    <details class="menu">
      <summary aria-label={t('station.moreOf', { name: station.name })}><Icono name="more" size={16} /></summary>
      <div class="pop"><button onclick={remove}>{t('station.delete')}</button></div>
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
    display: block;
    width: 100%;
    padding: 0;
    margin-bottom: 8px;
    border: 0;
    background: none;
    text-align: left;
    border-radius: var(--radius-lg);
    cursor: pointer;
  }
  span.pic {
    cursor: default;
  }
  .title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* The rules wrap rather than get cut on one line. */
  .sub {
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    overflow: hidden;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .sub {
    font-size: var(--fs-sm);
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
