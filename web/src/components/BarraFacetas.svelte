<script>
  // Explorar's bar: tabs, facet menus, order, and the chips of what is
  // applied.
  import { listenMode } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { chips, FACETS, inside, ORDERS, valueLabel, VIEWS, withFacet, withOrder, withView } from '../lib/facets.js'

  // query: the applied query; facets: the server's counts per facet;
  // onchange(query): a new query was chosen.
  let { query, facets, total, onchange } = $props()

  let open = $state(null) // name of the open menu, "more", or null
  let filters = $state({}) // text filter of each facet's menu: "Más" shows several at once
  let bar = $state()

  const main = FACETS.filter((f) => !f.more)
  // "Estado" only holds maintenance states (unidentified, duplicates).
  const more = $derived(FACETS.filter((f) => f.more && !(listenMode() && f.name === 'estado')))
  const applied = $derived(chips(query, facets))
  const NESTED = { decada: { name: 'anio', label: 'Años' }, genero: { name: 'estilo', label: 'Estilos' } }

  function toggle(name) {
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
    return all.filter((v) => valueLabel(name, v.value, v).toLowerCase().includes(f))
  }

  function onwindowclick(event) {
    if (open && bar && !bar.contains(event.target)) open = null
  }

  // What the total counts: artists, tracks, or albums.
  const totalLabel = $derived(
    query.view === 'artistas'
      ? count(total, 'artista', 'artistas')
      : query.view === 'pistas'
        ? count(total, 'pista', 'pistas')
        : count(total, 'álbum', 'álbumes'),
  )
</script>

<svelte:window onclick={onwindowclick} onkeydown={(e) => e.key === 'Escape' && (open = null)} />

{#snippet options(name, list)}
  <ul>
    {#each list as v (v.value)}
      <li>
        <button class="option" class:chosen={query.facets[name] === v.value} onclick={() => choose(name, v.value)}>
          <span>{valueLabel(name, v.value, v)}</span><span class="count">{v.count}</span>
        </button>
      </li>
    {:else}
      <li class="none">Sin valores</li>
    {/each}
  </ul>
{/snippet}

{#snippet menu(name)}
  {@const nested = NESTED[name]}
  {#if (facets?.[name] ?? []).length > 12}
    <!-- svelte-ignore a11y_autofocus -->
    <input type="search" placeholder="Filtrar…" bind:value={filters[name]} autofocus />
  {/if}
  {@render options(name, values(name))}
  {#if nested && inside(name, query, facets).length}
    <div class="label sub">{nested.label}</div>
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

<div class="bar" bind:this={bar}>
  {#each main as f (f.name)}
    <div class="facet">
      <button class="dd" class:on={query.facets[f.name]} onclick={() => toggle(f.name)} aria-expanded={open === f.name}>
        {f.label} ▾
      </button>
      {#if open === f.name}
        <div class="menu">{@render menu(f.name)}</div>
      {/if}
    </div>
  {/each}
  <div class="facet">
    <button class="dd" class:on={more.some((f) => query.facets[f.name])} onclick={() => toggle('more')} aria-expanded={open === 'more'}>
      Más ▾
    </button>
    {#if open === 'more'}
      <div class="menu wide">
        {#each more as f (f.name)}
          <div class="label sub">{f.label}</div>
          {@render menu(f.name)}
        {/each}
      </div>
    {/if}
  </div>

  <div class="right">
    <span class="total">{totalLabel}</span>
    <select value={query.order} onchange={(e) => onchange(withOrder(query, e.currentTarget.value))} aria-label="Orden">
      {#each ORDERS[query.view] as o (o.value)}
        <option value={o.value}>{o.label}</option>
      {/each}
    </select>
    <button
      class="dir"
      onclick={() => onchange({ ...query, dir: query.dir === 'asc' ? 'desc' : 'asc' })}
      aria-label={query.dir === 'asc' ? 'Ascendente' : 'Descendente'}
      title={query.dir === 'asc' ? 'Ascendente' : 'Descendente'}>{query.dir === 'asc' ? '↑' : '↓'}</button
    >
  </div>
</div>

{#if applied.length}
  <div class="chips">
    {#each applied as c (c.name)}
      <button class="chip" onclick={() => onchange(withFacet(query, c.name, null))} aria-label={`Quitar ${c.label}`}>
        {c.label} ✕
      </button>
    {/each}
    {#if applied.length > 1}
      <button class="chip clear" onclick={() => onchange({ ...query, facets: {} })}>Quitar todo</button>
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
    font-size: 15px;
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
    font-size: 13px;
  }
  .facet {
    position: relative;
  }
  .dd {
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
  }
  .dir {
    padding: 3px 9px;
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
    font-size: 13px;
    padding: 2px 10px;
    border-radius: 12px;
  }
  .chip.clear {
    border-color: var(--line-strong);
    color: var(--muted);
  }
  @media (max-width: 640px) {
    .right {
      margin-left: 0;
      width: 100%;
    }
    .menu {
      position: fixed;
      left: 16px;
      right: 16px;
      top: 120px;
    }
  }
</style>
