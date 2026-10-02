<script>
  // A list of tracks: a click plays from that track. Disc headers when the
  // tracks span several discs; the artist when it is not the album's (with
  // a link to their page when there is one); the album with showAlbum.
  // Tracks the server cannot deliver (playable false) are dimmed.
  import { duration, quality } from '../lib/format.js'
  import { enqueue } from '../lib/player.svelte.js'
  import { albumHref, artistHref } from '../lib/router.js'
  import { isLiked } from '../lib/marks.js'
  import { mine } from '../lib/mine.svelte.js'
  import Icono from './Icono.svelte'
  import MeGusta from './MeGusta.svelte'
  import MenuPista from './MenuPista.svelte'

  let { tracks, onplay, albumArtist = '', numbered = true, showAlbum = false } = $props()

  const discs = $derived(new Set(tracks.map((t) => t.disc || 1)).size)
  const otherArtist = (t) => t.artist && t.artist !== albumArtist
</script>

<ol class="pistas">
  {#each tracks as t, i (t.id)}
    {#if discs > 1 && (i === 0 || (tracks[i - 1].disc || 1) !== (t.disc || 1))}
      <li class="disc label">Disco {t.disc || 1}</li>
    {/if}
    <li class="row" class:unplayable={t.playable === false}>
      <!-- The whole row plays the track; the links on top of it stay links. -->
      <button
        class="play"
        onclick={() => onplay(t)}
        aria-label="Reproducir {t.title}"
        title={t.playable === false ? 'No se puede reproducir: falta ffmpeg' : undefined}></button>
      <span class="n">{numbered ? t.track || '' : ''}</span>
      <span class="text">
        <span class="title">{t.title}</span>
        {#if otherArtist(t) || (showAlbum && t.album)}
          <span class="artist">
            {#if otherArtist(t)}
              {#if t.artistKey}<a href={artistHref(t.artistKey)}>{t.artist}</a>{:else}{t.artist}{/if}
            {/if}
            {#if showAlbum && t.album}
              {otherArtist(t) ? ' · ' : ''}<a href={albumHref(t.albumId)}>{t.album}</a>
            {/if}
          </span>
        {/if}
      </span>
      <span class="quality">{quality(t)}</span>
      <span class="like" class:liked={isLiked(mine.marks, 'track', t.id)}><MeGusta kind="track" ref={t.id} size={16} /></span>
      <span class="time">{duration(t.durationMs)}</span>
      <button class="icon add" onclick={() => enqueue({ tracks: [t.id] })} title="Agregar a la cola" aria-label="Agregar a la cola">
        <Icono name="plus" size={16} />
      </button>
      <MenuPista track={t} />
    </li>
  {/each}
</ol>

<style>
  .pistas {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .disc {
    padding: 16px 8px 6px;
  }
  .row {
    position: relative;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 0 0 0 8px;
    border-bottom: 1px solid var(--line);
  }
  .row:hover {
    background: var(--surface);
  }
  .play {
    position: absolute;
    inset: 0;
    border: none;
    border-radius: 0;
    padding: 0;
  }
  /* Everything sits on the play button and lets clicks through, but links
     and the queue button. */
  .row > :not(.play) {
    position: relative;
    pointer-events: none;
  }
  .row a,
  .row .add,
  .row .like,
  .row :global(.menu) {
    pointer-events: auto;
  }
  .n {
    width: 24px;
    text-align: right;
    color: var(--faint);
    font-variant-numeric: tabular-nums;
    flex: none;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    padding: 8px 0;
  }
  .title,
  .artist {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
  }
  .artist {
    font-size: 13px;
    color: var(--muted);
  }
  .artist a:hover {
    color: var(--text);
    text-decoration: underline;
  }
  .unplayable .title {
    color: var(--faint);
  }
  .quality {
    font-size: 12px;
    color: var(--faint);
    white-space: nowrap;
  }
  .time {
    font-size: 13px;
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    min-width: 40px;
    text-align: right;
  }
  .add,
  .like:not(.liked) {
    opacity: 0;
  }
  .row:hover .add,
  .add:focus-visible,
  .row:hover .like,
  .like:focus-within {
    opacity: 1;
  }
  .like {
    display: inline-flex;
  }
  @media (max-width: 640px) {
    .quality {
      display: none;
    }
    .add,
    .like:not(.liked) {
      opacity: 1;
    }
  }
</style>
