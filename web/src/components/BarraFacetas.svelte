<script>
  // Explorar's bar: tabs, facet menus, order, the station, and the chips of
  // what is applied. On phones the facets are one row that scrolls sideways.
  import { listenMode } from '../lib/app.svelte.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import BotonEstacion from './BotonEstacion.svelte'
  import Icono from './Icono.svelte'
  import { chips, FACETS, facetLabel, inside, ORDERS, valueLabel, VIEWS, withFacet, withOrder, withView } from '../lib/facets.js'

  // query: the applied query; facets: the server's counts per facet;
  // station: what the "Estación" button plays (null disables it);
  // onchange(query): a new query was chosen.
  let { query, facets, total, station, onchange } = $props()

  let open = $state(null) // name of the open menu, "more", or null
  let filters = $state({}) // text filter of each facet's menu: "Más" shows several at once
  let bar = $state()
  let menuTop = $state(0) // on phones the menu is fixed: right under its button

  const main = FACETS.filter((f) => !f.more)
  // "Estado" only holds maintenance states (unidentified, duplicates).
  const more = $derived(FACETS.filter((f) => f.more && !(listenMode() && f.name === 'state')))
  const applied = $derived(chips(query, facets))
  const NESTED = { decade: { name: 'year', label: 'explore.bar.years' }, genre: { name: 'style', label: 'explore.bar.styles' } }

  function toggle(name, event) {
    menuTop = event.currentTarget.getBoundingClientRect().bottom + 6
    open = open === name ? null : name
    filters = {}
  }

  function choose(name, value) {
    open = null
    onchange(withFacet(query, name, query.facets[name] === value ? null : value))
  }

  function values(name) {
    const all = facets?.[name] ?? []
    const f = (filters[name] ?? '').trim().toLowerCase()
    if (!f) return all
    return all.filter((v) => valueLabel(name, v.value, v, query.view).toLowerCase().includes(f))
  }

  function onwindowclick(event) {
    if (open && bar && !bar.contains(event.target)) open = null
  }

  // What the total counts: artists, tracks, or albums.
  const totalLabel = $derived(
    query.view === 'artists'
      ? t('count.artists', { n: total })
      : query.view === 'tracks'
        ? t('count.tracks', { n: total })
        : t('count.albums', { n: total }),
  )
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = null)} />

{#snippet options(name, list)}
  <ul>
    {#each list as v (v.value)}
      <li>
        <button class="option" class:chosen={query.facets[name] === v.value} onclick={() => choose(name, v.value)}>
          <span>{valueLabel(name, v.value, v, query.view)}</span><span class="count">{v.count}</span>
        </button>
      </li>
    {:else}
      <li class="none">{t('explore.bar.noValues')}</li>
    {/each}
  </ul>
{/snippet}

{#snippet menu(name)}
  {@const nested = NESTED[name]}
  {#if (facets?.[name] ?? []).length > 12}
    <!-- svelte-ignore a11y_autofocus -->
    <input type="search" placeholder={t('explore.bar.filter')} bind:value={filters[name]} autofocus />
  {/if}
  {@render options(name, values(name))}
  {#if nested && inside(name, query, facets).length}
    <div class="label sub">{t(nested.label)}</div>
    {@render options(nested.name, inside(name, query, facets))}
  {/if}
{/snippet}

<div class="tabs" role="tablist">
  {#each VIEWS as v (v.value)}
    <button role="tab" class:on={query.view === v.value} aria-selected={query.view === v.value} onclick={() => onchange(withView(query, v.value))}>
      {v.label}
    </button>
  {/each}
</div>

<div class="bar" bind:this={bar} style:--menu-top="{menuTop}px">
  <div class="facets">
  {#each main as f (f.name)}
    <div class="facet">
      <button class="dd" class:on={query.facets[f.name]} onclick={(e) => toggle(f.name, e)} aria-expanded={open === f.name}>
        {facetLabel(f, query.view)} ▾
      </button>
      {#if open === f.name}
        <div class="menu">{@render menu(f.name)}</div>
      {/if}
    </div>
  {/each}
  <div class="facet">
    <button class="dd" class:on={more.some((f) => query.facets[f.name])} onclick={(e) => toggle('more', e)} aria-expanded={open === 'more'}>
      {t('explore.bar.more')} ▾
    </button>
    {#if open === 'more'}
      <div class="menu wide">
        {#each more as f (f.name)}
          <div class="label sub">{facetLabel(f, query.view)}</div>
          {@render menu(f.name)}
        {/each}
      </div>
    {/if}
  </div>
  </div>

  <div class="right">
    <span class="total">{totalLabel}</span>
    <select value={query.order} onchange={(e) => onchange(withOrder(query, e.currentTarget.value))} aria-label={t('explore.bar.order')}>
      {#each ORDERS[query.view] as o (o.value)}
        <option value={o.value}>{o.label}</option>
      {/each}
    </select>
    {#if query.order === 'random'}
      <!-- No direction to a shuffle: draw another one instead. -->
      <button class="dir reshuffle" onclick={() => onchange(withOrder(query, 'random'))} aria-label={t('explore.bar.reshuffle')}
        title={t('explore.bar.reshuffle')}><Icono name="refresh" size={14} /></button>
    {:else}
      <button
        class="dir"
        onclick={() => onchange({ ...query, dir: query.dir === 'asc' ? 'desc' : 'asc' })}
        aria-label={query.dir === 'asc' ? t('explore.bar.asc') : t('explore.bar.desc')}
        title={query.dir === 'asc' ? t('explore.bar.asc') : t('explore.bar.desc')}>{query.dir === 'asc' ? '↑' : '↓'}</button
      >
    {/if}
    <BotonEstacion {station} compact />
  </div>
</div>

{#if applied.length}
  <div class="chips">
    {#each applied as c (c.name)}
      <button class="chip" onclick={() => onchange(withFacet(query, c.name, null))} aria-label={t('explore.bar.remove', { label: c.label })}>
        {c.label} ✕
      </button>
    {/each}
    {#if applied.length > 1}
      <button class="chip clear" onclick={() => onchange({ ...query, facets: {} })}>{t('explore.bar.removeAll')}</button>
    {/if}
  </div>
{/if}

<style>
  .tabs {
    display: flex;
    gap: 20px;
    margin-bottom: 12px;
  }
  .tabs button {
    border: none;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    padding: 4px 0;
    font-size: var(--fs-md);
    color: var(--muted);
  }
  .tabs button.on {
    color: var(--strong);
    border-bottom-color: var(--accent);
  }
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--line);
    font-size: var(--fs-sm);
  }
  .facets {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  .facet {
    position: relative;
  }
  .dd {
    white-space: nowrap;
    color: var(--muted);
    border-color: var(--line);
    padding: 3px 10px;
  }
  .dd.on {
    color: var(--accent);
    border-color: var(--accent);
  }
  .menu {
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    z-index: 15;
    min-width: 220px;
    max-height: 60vh;
    overflow-y: auto;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 6px;
    box-shadow: 0 10px 32px rgba(0, 0, 0, 0.55);
  }
  .menu.wide {
    min-width: 260px;
  }
  .menu input {
    width: 100%;
    margin-bottom: 6px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .option {
    width: 100%;
    display: flex;
    justify-content: space-between;
    gap: 12px;
    border: none;
    padding: 4px 8px;
    text-align: left;
  }
  .option:hover {
    background: var(--surface-2);
  }
  .option.chosen {
    color: var(--accent);
  }
  .count {
    color: var(--faint);
  }
  .none {
    color: var(--faint);
    padding: 4px 8px;
  }
  .sub {
    padding: 8px 8px 2px;
  }
  .right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .total {
    color: var(--faint);
    white-space: nowrap;
  }
  .right select {
    padding-block: 3px; /* as tall as the facet buttons */
  }
  .dir {
    padding: 3px 9px;
  }
  .reshuffle {
    display: inline-flex;
    align-items: center;
    align-self: stretch;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 10px;
  }
  .chip {
    border-color: var(--accent);
    color: var(--accent);
    font-size: var(--fs-sm);
    padding: 2px 10px;
    border-radius: 12px;
  }
  .chip.clear {
    border-color: var(--line-strong);
    color: var(--muted);
  }
  @media (max-width: 640px) {
    .bar {
      gap: 6px;
      padding-bottom: 6px;
    }
    /* One row of chips that swipes, out to the screen's edges. */
    .facets {
      flex-wrap: nowrap;
      gap: 6px;
      overflow-x: auto;
      overscroll-behavior-x: contain;
      margin-inline: calc(-1 * var(--gutter));
      padding-inline: var(--gutter);
      scroll-padding-inline: var(--gutter);
      scrollbar-width: none;
    }
    .facets::-webkit-scrollbar {
      display: none;
    }
    .dd {
      border-radius: 14px;
    }
    .right {
      margin-left: 0;
      width: 100%;
    }
    .right select {
      min-width: 0;
      flex: 0 1 auto;
    }
    /* The station sits at the row's end. */
    .right > :global(.estacion) {
      margin-left: auto;
    }
    .menu {
      position: fixed;
      left: var(--gutter);
      right: var(--gutter);
      top: var(--menu-top);
      max-height: calc(100dvh - var(--menu-top) - 96px);
    }
  }
</style>
