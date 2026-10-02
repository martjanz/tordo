<script>
  // Correcting an album's identification: one of the candidates MusicBrainz
  // offered, a release pasted from musicbrainz.org, "not in MusicBrainz",
  // or back to the automatic identification. onchange gets the album as the
  // server answers it.
  import { api, releaseURL } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { candidateLine, percent } from '../lib/review.js'

  let { album, onchange } = $props()

  let pasted = $state('')
  let busy = $state(false)

  async function run(f) {
    if (busy) return
    busy = true
    try {
      onchange(await f())
    } catch (e) {
      notify(e.message)
    } finally {
      busy = false
    }
  }

  const choose = (releaseId) => run(() => api.setIdentification(album.id, { releaseId }))
  const ignore = () => run(() => api.setIdentification(album.id, { ignore: true }))
  const reset = () => run(() => api.resetIdentification(album.id))

  function onsubmit(event) {
    event.preventDefault()
    if (pasted.trim()) choose(pasted.trim())
  }
</script>

<div class="identificar" class:busy>
  {#if album.candidates?.length}
    <ul class="candidates">
      {#each album.candidates as c (c.releaseId)}
        <li class:current={c.releaseId === album.releaseId}>
          <button onclick={() => choose(c.releaseId)} disabled={busy}>Elegir</button>
          <span class="score">{percent(c.score)}</span>
          <span class="text">
            <span class="title">{c.title}</span>
            <span class="meta">{candidateLine(c)}</span>
          </span>
          <a href={releaseURL(c.releaseId)} target="_blank" rel="noreferrer" title="Ver en MusicBrainz">↗</a>
        </li>
      {/each}
    </ul>
  {:else}
    <p class="none">MusicBrainz no ofreció candidatos.</p>
  {/if}
  <form {onsubmit}>
    <input bind:value={pasted} placeholder="Pegá un enlace de musicbrainz.org/release/…" aria-label="Lanzamiento de MusicBrainz" />
    <button type="submit" disabled={busy || !pasted.trim()}>Usar</button>
  </form>
  <div class="more">
    {#if album.status !== 'ignored'}
      <button onclick={ignore} disabled={busy}>No está en MusicBrainz</button>
    {/if}
    {#if album.status === 'manual' || album.status === 'ignored'}
      <button onclick={reset} disabled={busy}>Volver a la identificación automática</button>
    {/if}
  </div>
</div>

<style>
  .identificar.busy {
    opacity: 0.6;
  }
  .candidates {
    list-style: none;
    margin: 0 0 10px;
    padding: 0;
  }
  .candidates li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
  }
  .candidates li.current .title {
    color: var(--accent);
  }
  .score {
    width: 44px;
    white-space: nowrap;
    text-align: right;
    font-variant-numeric: tabular-nums;
    color: var(--muted);
    font-size: 13px;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .title,
  .meta {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
  }
  .meta {
    font-size: 12px;
    color: var(--faint);
  }
  .none {
    color: var(--muted);
    margin: 0 0 10px;
  }
  form {
    display: flex;
    gap: 8px;
  }
  form input {
    flex: 1;
    min-width: 0;
  }
  .more {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 10px;
  }
</style>
