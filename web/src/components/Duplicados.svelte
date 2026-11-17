<script>
  // Revisar → Duplicados: groups of files that hold, or may hold, the same
  // music, by kind. Tordo never deletes: this is only to decide.
  import { api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { KINDS, fileQuality, groupLine } from '../lib/dupes.js'
  import { folderName, size } from '../lib/format.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { play } from '../lib/playback.svelte.js'
  import { albumHref, duplicatesHref, folderHref } from '../lib/router.js'
  import Icono from './Icono.svelte'

  let { kind = 'exact', onload } = $props()

  const PAGE = 100
  const current = $derived(KINDS.find((k) => k.kind === kind))

  let answer = $state(null)
  let error = $state('')
  let shown = $state(PAGE)

  $effect(() => {
    app.generation
    const k = kind
    let stale = false
    shown = PAGE
    api
      .duplicates(k)
      .then((a) => {
        if (stale) return
        answer = a
        error = ''
        onload?.(a.counts.exact + a.counts.formats)
      })
      .catch((e) => {
        if (!stale) error = e.message
      })
    return () => (stale = true)
  })

  const groups = $derived(answer?.groups ?? [])
</script>

<p class="note">{t('review.duplicates.note')}</p>

{#if error}
  <p class="empty">{error}</p>
{:else if answer}
  <nav class="kinds">
    {#each KINDS as k (k.kind)}
      <a href={duplicatesHref(k.kind)} class:on={kind === k.kind}>{k.label} ({answer.counts[k.count]})</a>
    {/each}
  </nav>
  {#if kind === 'exact'}
    <p class="note">{t('review.duplicates.recoverableTotal', { size: size(answer.recoverable) })}</p>
  {/if}
  <p class="help">{current?.help}</p>

  {#if groups.length === 0}
    <p class="empty">{current?.empty}</p>
  {:else}
    {#each groups.slice(0, shown) as g, i (i)}
      <section class="group">
        <header>
          <span class="name">{g.title} — {g.artist}</span>
          <span class="line">{groupLine(g)}</span>
        </header>
        <ul>
          {#each g.files as f (f.id)}
            <li>
              {#if f.best}<span class="best">{t('review.duplicates.best')}</span>{/if}
              <span class="quality">{fileQuality(f)}</span>
              <span class="size">{size(f.sizeBytes)}</span>
              <span class="where">
                {#if f.albumId}<a href={albumHref(f.albumId)}>{f.album}</a>{:else}<span class="loose">{t('review.duplicates.loose')}</span>{/if}
                <a class="folder" href={folderHref(f.dir)}>{folderName(f.dir)}</a>
              </span>
              <button class="icon" onclick={() => play({ tracks: [f.id] })} title={t('review.duplicates.listenFile')} aria-label={t('review.duplicates.listenPath', { path: f.path })}>
                <Icono name="play" size={16} />
              </button>
              <span class="file" title={f.path}>{f.path.split('/').pop()}</span>
            </li>
          {/each}
        </ul>
      </section>
    {/each}
    {#if groups.length > shown}
      <button onclick={() => (shown += PAGE)}>{t('review.duplicates.showMore')}</button>
    {/if}
  {/if}
{/if}

<style>
  .note {
    margin: 0 0 8px;
    color: var(--muted);
    font-size: 13px;
  }
  .help {
    margin: 8px 0 14px;
    color: var(--muted);
  }
  .kinds {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 8px;
  }
  .kinds a {
    padding: 2px 12px;
    border: 1px solid var(--line-strong);
    border-radius: 14px;
    color: var(--muted);
    font-size: 13px;
  }
  .kinds a.on {
    border-color: var(--accent);
    color: var(--accent);
  }
  .group {
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 10px 14px;
    margin-bottom: 12px;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 14px;
    margin-bottom: 6px;
  }
  .name {
    color: var(--strong);
    font-weight: 500;
  }
  .line {
    color: var(--muted);
    font-size: 13px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 12px;
    padding: 6px 0;
    border-top: 1px solid var(--line);
    font-size: 13px;
  }
  .best {
    font-size: 11px;
    font-weight: 600;
    padding: 0 7px;
    border-radius: 3px;
    background: var(--accent);
    color: var(--on-accent);
  }
  .quality {
    color: var(--text);
  }
  .size {
    color: var(--muted);
  }
  .where {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 12px;
    min-width: 0;
  }
  .where a:hover {
    text-decoration: underline;
  }
  .folder,
  .loose {
    color: var(--faint);
  }
  .file {
    flex: 1 1 160px;
    min-width: 0;
    color: var(--faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
