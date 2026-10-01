<script>
  // A list of tracks: a click plays from that track. Disc headers when the
  // tracks span several discs; the artist when it is not the album's.
  import { duration, quality } from '../lib/format.js'
  import { enqueue } from '../lib/player.svelte.js'
  import Icono from './Icono.svelte'

  let { tracks, onplay, albumArtist = '', numbered = true } = $props()

  const discs = $derived(new Set(tracks.map((t) => t.disc || 1)).size)
</script>

<ol class="pistas">
  {#each tracks as t, i (t.id)}
    {#if discs > 1 && (i === 0 || (tracks[i - 1].disc || 1) !== (t.disc || 1))}
      <li class="disc label">Disco {t.disc || 1}</li>
    {/if}
    <li class="row">
      <button class="play" onclick={() => onplay(t)}>
        <span class="n">{numbered ? t.track || '' : ''}</span>
        <span class="text">
          <span class="title">{t.title}</span>
          {#if t.artist && t.artist !== albumArtist}<span class="artist">{t.artist}</span>{/if}
        </span>
        <span class="quality">{quality(t)}</span>
        <span class="time">{duration(t.durationMs)}</span>
      </button>
      <button class="icon add" onclick={() => enqueue({ tracks: [t.id] })} title="Agregar a la cola" aria-label="Agregar a la cola">
        <Icono name="plus" size={16} />
      </button>
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
    display: flex;
    align-items: center;
    border-bottom: 1px solid var(--line);
  }
  .play {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 12px;
    border: none;
    border-radius: 0;
    padding: 8px;
    text-align: left;
  }
  .play:hover {
    background: var(--surface);
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
  .add {
    opacity: 0;
  }
  .row:hover .add,
  .add:focus-visible {
    opacity: 1;
  }
  @media (max-width: 640px) {
    .quality {
      display: none;
    }
    .add {
      opacity: 1;
    }
  }
</style>
