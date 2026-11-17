<script>
  // The Sin identificar queue: albums MusicBrainz matched below the
  // threshold, with their candidates. An album leaves the queue once
  // corrected.
  import Cargando from './Cargando.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { folderName } from '../lib/format.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { play } from '../lib/playback.svelte.js'
  import { candidateLine, percent, searchURL, worthAccepting } from '../lib/review.js'
  import { albumHref, folderHref } from '../lib/router.js'
  import Icono from './Icono.svelte'
  import Identificar from './Identificar.svelte'
  import Tapa from './Tapa.svelte'

  let { onload } = $props()

  let albums = $state(null)
  let error = $state('')
  let open = $state(null) // the album being corrected
  let accepting = $state(null) // {done, of} while accepting the best candidates
  let onlyLikely = $state(false) // show only the albums the bulk accept would take

  $effect(() => {
    app.generation
    api
      .unidentified()
      .then((as) => {
        albums = as
        error = ''
        onload?.(as.length)
      })
      .catch((e) => (error = e.message))
  })

  function corrected(a) {
    if (a.status !== 'unmatched') {
      albums = albums.filter((x) => x.id !== a.id)
      onload?.(albums.length)
    }
    open = null
  }

  const likely = $derived(albums ? worthAccepting(albums) : [])

  const shown = $derived(onlyLikely ? likely : (albums ?? []))

  // acceptAll takes the best candidate of every likely album, one at a time.
  async function acceptAll() {
    const list = likely
    if (!confirm(t('review.unidentified.confirmAccept', { albums: t('count.albums', { n: list.length }), only: t('review.unidentified.onlyLikely') }))) return
    accepting = { done: 0, of: list.length }
    for (const a of list) {
      try {
        corrected(await api.setIdentification(a.id, { releaseId: a.candidates[0].releaseId }))
      } catch (e) {
        notify(`${a.title}: ${e.message}`)
      }
      accepting.done++
    }
    accepting = null
  }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if albums?.length === 0}
  <p class="empty">{t('review.unidentified.empty')}</p>
{:else if albums}
  {#if likely.length && !app.status?.readOnly}
    <p class="bulk">
      {#if accepting}
        {t('review.unidentified.accepting', { done: accepting.done, total: accepting.of })}
      {:else}
        {t('count.albumsLikely', { n: likely.length })}
        <label><input type="checkbox" bind:checked={onlyLikely} /> {t('review.unidentified.onlyLikely')}</label>
        <button onclick={acceptAll}>{t('review.unidentified.acceptAll')}</button>
      {/if}
    </p>
  {/if}
  <ul class="queue">
    {#each shown as a (a.id)}
      <li>
        <div class="row">
          <a class="thumb" href={albumHref(a.id)}><Tapa srcs={a.picture ? [albumCoverURL(a.id, 160)] : []} alt={a.title} /></a>
          <div class="text">
            <a class="title" href={albumHref(a.id)}>{a.tagTitle}</a>
            <span class="meta">{[a.tagArtist, a.year || '', t('count.tracks', { n: a.tracks })].filter(Boolean).join(' · ')}</span>
            <a class="meta folder" href={folderHref(a.dir)}>{folderName(a.dir)}</a>
            {#if a.candidates.length}
              <span class="meta best">{t('review.unidentified.best', { title: a.candidates[0].title, line: candidateLine(a.candidates[0]), percent: percent(a.candidates[0].score) })}</span>
            {:else}
              <span class="meta best">{t('review.unidentified.noCandidates')}</span>
            {/if}
          </div>
          <div class="buttons">
            <button class="icon" onclick={() => play({ album: a.id })} title={t('review.unidentified.listen')} aria-label={t('review.unidentified.listenNamed', { title: a.tagTitle })}>
              <Icono name="play" size={16} />
            </button>
            <a class="search" href={searchURL(a)} target="_blank" rel="noreferrer">{t('review.unidentified.searchMB')}</a>
            {#if !app.status?.readOnly}
              <button onclick={() => (open = open === a.id ? null : a.id)}>{open === a.id ? t('common.close') : t('review.unidentified.fix')}</button>
            {/if}
          </div>
        </div>
        {#if open === a.id}
          <div class="panel"><Identificar album={a} onchange={corrected} /></div>
        {/if}
      </li>
    {/each}
  </ul>
{:else}
  <Cargando />
{/if}

<style>
  .bulk {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 12px;
    color: var(--muted);
  }
  .queue {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .queue li {
    border-bottom: 1px solid var(--line);
    padding: 10px 0;
  }
  .row {
    display: flex;
    gap: 14px;
    align-items: center;
  }
  .thumb {
    width: 64px;
    flex: none;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .meta {
    font-size: var(--fs-sm);
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .folder,
  .best {
    color: var(--faint);
    font-size: var(--fs-xs);
  }
  .buttons {
    display: flex;
    align-items: center;
    gap: 10px;
    flex: none;
  }
  .search {
    font-size: var(--fs-sm);
    color: var(--muted);
  }
  .panel {
    margin: 10px 0 4px 78px;
    max-width: 720px;
  }
  @media (max-width: 640px) {
    .row {
      flex-wrap: wrap;
    }
    .buttons {
      width: 100%;
      justify-content: flex-end;
    }
    .panel {
      margin-left: 0;
    }
  }
</style>
