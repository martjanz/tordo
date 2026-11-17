<script>
  // Géneros por mapear: raw genre values from the tags that no mapping
  // knows, most used first, to map onto the taxonomy (or ignore); and the
  // user's mappings, to undo one.
  import Cargando from './Cargando.svelte'
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { aliasLabel, genreChoice, genreOptions } from '../lib/review.js'

  let { onload } = $props()

  let unmapped = $state(null)
  let aliases = $state([])
  let options = $state([])
  let editing = $state(null) // the raw value being mapped: one picker at a time
  let chosen = $state('') // its option value
  let error = $state('')

  async function load() {
    try {
      const [u, a, t] = await Promise.all([api.unmappedGenres(), api.genreAliases(), api.genres()])
      unmapped = u
      aliases = a
      options = genreOptions(t)
      error = ''
      onload?.(u.length)
    } catch (e) {
      error = e.message
    }
  }

  $effect(() => {
    app.generation
    load()
  })

  function edit(raw) {
    editing = raw
    chosen = ''
  }

  async function save(raw) {
    try {
      await api.setGenreAlias({ raw, ...genreChoice(chosen) })
      editing = null
      await load()
    } catch (e) {
      notify(e.message)
    }
  }

  async function forget(raw) {
    try {
      await api.deleteGenreAlias(raw)
      await load()
    } catch (e) {
      notify(e.message)
    }
  }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if unmapped}
  {#if unmapped.length === 0}
    <p class="empty">{t('review.genres.allMapped')}</p>
  {:else}
    <ul class="rows">
      {#each unmapped as u (u.raw)}
        <li>
          <span class="raw">{u.raw}</span>
          <span class="n">{t('count.albums', { n: u.albums })}</span>
          {#if app.status?.readOnly}
            <!-- browse-only: nothing to map -->
          {:else if editing === u.raw}
            <select bind:value={chosen} aria-label={t('review.genres.genreFor', { raw: u.raw })}>
              <option value="">{t('review.genres.pick')}</option>
              {#each options as o (o.value)}
                <option value={o.value}>{o.label}</option>
              {/each}
            </select>
            <button class="primary" onclick={() => save(u.raw)} disabled={!chosen}>{t('common.save')}</button>
            <button onclick={() => (editing = null)}>{t('common.cancel')}</button>
          {:else}
            <button onclick={() => edit(u.raw)}>{t('review.genres.map')}</button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  {#if aliases.length}
    <h2>{t('review.genres.yours')}</h2>
    <ul class="rows">
      {#each aliases as a (a.raw)}
        <li>
          <span class="raw">{a.raw}</span>
          <span class="n">→ {aliasLabel(a)}</span>
          {#if !app.status?.readOnly}
            <button class="icon" onclick={() => forget(a.raw)} title={t('review.genres.forget')} aria-label={t('review.genres.forgetNamed', { raw: a.raw })}>✕</button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
{:else}
  <Cargando />
{/if}

<style>
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .rows li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .raw {
    color: var(--strong);
    min-width: 200px;
  }
  .n {
    color: var(--muted);
    font-size: var(--fs-sm);
    flex: 1;
  }
  select {
    max-width: 320px;
  }
</style>
