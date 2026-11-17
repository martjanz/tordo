<script>
  // The Sin identificar queue: albums MusicBrainz matched below the
  // threshold, with their candidates. An album leaves the queue once
  // corrected.
  import Cargando from './Cargando.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { count, folderName } from '../lib/format.js'
  import { play } from '../lib/playback.svelte.js'
  import { candidateChip, confidenceLevel, folderDiffers, percent, searchURL, worthAccepting } from '../lib/review.js'
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
    if (!confirm(`¿Aceptar el primer candidato de ${count(list.length, 'álbum', 'álbumes')}? Revisalos antes con «Ver solo esos».`)) return
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
  <p class="empty">No hay álbumes sin identificar.</p>
{:else if albums}
  {#if likely.length && !app.status?.readOnly}
    <div class="bulk">
      {#if accepting}
        <p>Aceptando <strong>{accepting.done}</strong> de {accepting.of}…</p>
      {:else}
        <p>
          <strong>{likely.length}</strong>
          {likely.length === 1 ? 'álbum tiene' : 'álbumes tienen'} un primer candidato de 70 % o más.
        </p>
        <div class="bulk-actions">
          <label><input type="checkbox" bind:checked={onlyLikely} /> Ver solo esos</label>
          <button class="primary" onclick={acceptAll}>Aceptar el primer candidato de cada uno</button>
        </div>
      {/if}
    </div>
  {/if}
  <ul class="queue">
    {#each shown as a (a.id)}
      <li>
        <div class="row" class:open={open === a.id}>
          <a class="thumb" href={albumHref(a.id)}><Tapa srcs={a.picture ? [albumCoverURL(a.id, 160)] : []} alt={a.title} /></a>
          <div class="text">
            <a class="title" href={albumHref(a.id)}>{a.tagTitle}</a>
            <span class="meta">{[a.tagArtist, a.year || '', count(a.tracks, 'pista', 'pistas')].filter(Boolean).join(' · ')}</span>
            {#if folderDiffers(a)}
              <a class="meta folder" href={folderHref(a.dir)} title="Carpeta">{folderName(a.dir)}</a>
            {/if}
            {#if a.candidates.length}
              {@const c = a.candidates[0]}
              <span class="best" title="Mejor candidato de MusicBrainz">
                <span class="score {confidenceLevel(c.score)}">{percent(c.score)}</span>
                <span class="chip">{candidateChip(c)}</span>
              </span>
            {:else}
              <span class="best none">Sin candidatos</span>
            {/if}
          </div>
          <div class="buttons">
            <button class="icon" onclick={() => play({ album: a.id })} title="Escuchar" aria-label="Escuchar {a.tagTitle}">
              <Icono name="play" size={16} />
            </button>
            <a class="icon" href={searchURL(a)} target="_blank" rel="noreferrer" title="Buscar en MusicBrainz" aria-label="Buscar {a.tagTitle} en MusicBrainz">
              <Icono name="search" size={16} />
            </a>
            {#if !app.status?.readOnly}
              <button class="fix" onclick={() => (open = open === a.id ? null : a.id)} aria-expanded={open === a.id}>{open === a.id ? 'Cerrar' : 'Corregir'}</button>
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
    justify-content: space-between;
    gap: var(--space-2) var(--space-4);
    margin: 0 0 var(--space-3);
    padding: var(--space-3) var(--space-4);
    background: var(--surface);
    border: 1px solid var(--line);
    border-left: 3px solid var(--accent);
    border-radius: var(--radius-lg);
    color: var(--muted);
    font-size: var(--fs-sm);
  }
  .bulk p {
    margin: 0;
  }
  .bulk strong {
    color: var(--strong);
    font-size: var(--fs-lg);
    font-variant-numeric: tabular-nums;
  }
  .bulk-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-3);
  }
  .bulk-actions label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
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
    gap: 2px;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    font-size: var(--fs-sm);
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .folder {
    color: var(--faint);
    font-size: var(--fs-xs);
  }
  .best {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
    margin-top: 2px;
    font-size: var(--fs-sm);
  }
  .best.none {
    color: var(--faint);
    font-size: var(--fs-xs);
  }
  .chip {
    color: var(--text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* The confidence badge: green from the bulk accept's 70 %, amber from 50 %,
     grey below. */
  .score {
    flex: none;
    padding: 1px 6px;
    border-radius: var(--radius);
    font-size: var(--fs-xs);
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--muted);
    background: var(--surface-2);
  }
  .score.high {
    color: var(--ok);
    background: color-mix(in srgb, var(--ok) 16%, transparent);
  }
  .score.mid {
    color: var(--warn);
    background: color-mix(in srgb, var(--warn) 16%, transparent);
  }
  .buttons {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: none;
  }
  a.icon {
    display: inline-flex;
    padding: 6px;
    border-radius: 50%;
    color: var(--muted);
  }
  a.icon:hover {
    color: var(--strong);
  }
  .fix {
    margin-left: 4px;
  }
  .panel {
    margin: 10px 0 4px 78px;
    max-width: 720px;
  }
  /* With a mouse, the row's actions show on hover (and while correcting). On
     touch screens they always show. */
  @media (hover: hover) {
    .buttons {
      opacity: 0;
      transition: opacity 0.12s;
    }
    .row:hover .buttons,
    .row:focus-within .buttons,
    .row.open .buttons {
      opacity: 1;
    }
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
