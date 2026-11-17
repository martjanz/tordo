<script>
  // A list of tracks: a click plays from that track. Disc headers with
  // discHeaders (one album's track list) when it spans several discs; the
  // artist when it is not the album's (with a link to their page when there
  // is one); the album with showAlbum. The format only on the tracks that
  // differ from the one most of the list shares (the album header says it).
  // The track playing shows an equaliser instead of its number.
  // Tracks the server cannot deliver (playable false) are dimmed.
  import { artistLabel, dominantQuality, duration, quality } from '../lib/format.js'
  import { t as translate } from '../lib/i18n/index.svelte.js'
  import { shown } from '../lib/playback.svelte.js'
  import * as Q from '../lib/queue.js'
  import { albumHref, artistHref } from '../lib/router.js'
  import { isLiked } from '../lib/marks.js'
  import { mine } from '../lib/mine.svelte.js'
  import { discHeaders } from '../lib/discs.js'
  import Icono from './Icono.svelte'
  import MeGusta from './MeGusta.svelte'
  import MenuMas from './MenuMas.svelte'

  let { tracks, onplay, albumArtist = '', numbered = true, showAlbum = false, showPlays = false, discHeaders: withDiscs = false } = $props()

  const headers = $derived(discHeaders(tracks, withDiscs))
  const usual = $derived(dominantQuality(tracks))
  const p = $derived(shown())
  const nowId = $derived(Q.current(p.queue)?.id)
  const otherArtist = (t) => t.artist && t.artist !== albumArtist
</script>

<ol class="pistas">
  {#each tracks as t, i (t.id)}
    {#if headers[i]}
      <li class="disc label">{translate('album.disc', { n: headers[i] })}</li>
    {/if}
    {@const now = t.id === nowId}
    {@const q = quality(t)}
    <li class="row" class:now class:unplayable={t.playable === false} aria-current={now ? 'true' : undefined}>
      <!-- The whole row plays the track; the links on top of it stay links. -->
      <button
        class="play"
        onclick={() => onplay(t)}
        aria-label={translate('player.playNamed', { title: t.title })}
        title={t.playable === false ? translate('album.unplayable') : undefined}></button>
      <span class="n">
        {#if now}
          <span class="eq" class:paused={!p.playing} role="img" aria-label={p.playing ? translate('player.nowPlaying') : translate('player.paused')}><i></i><i></i><i></i></span>
        {:else}
          <span class="num">{numbered ? t.track || '' : ''}</span>
          {#if t.playable !== false}<span class="hover-play"><Icono name="play" size={14} /></span>{/if}
        {/if}
      </span>
      <span class="text">
        <span class="title">{t.title}</span>
        {#if otherArtist(t) || (showAlbum && t.album)}
          <span class="artist">
            {#if otherArtist(t)}
              {#if t.artistKey}<a href={artistHref(t.artistKey)}>{artistLabel(t.artist)}</a>{:else}{artistLabel(t.artist)}{/if}
            {/if}
            {#if showAlbum && t.album}
              {otherArtist(t) ? ' · ' : ''}<a href={albumHref(t.albumId)}>{t.album}</a>
            {/if}
          </span>
        {/if}
      </span>
      {#if showPlays}<span class="plays">{translate('count.times', { n: t.plays ?? 0 })}</span>{/if}
      {#if q && q !== usual}<span class="quality">{q}</span>{/if}
      <span class="like" class:liked={isLiked(mine.marks, 'track', t.id)}><MeGusta kind="track" ref={t.id} size={16} /></span>
      <span class="time">{duration(t.durationMs)}</span>
      <MenuMas what={{ tracks: [t.id] }} rating={{ kind: 'track', id: t.id }} />
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
    padding: var(--space-4) var(--space-2) 6px;
  }
  .row {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    min-height: 44px;
    padding: 0 var(--space-1) 0 var(--space-2);
    border-radius: var(--radius);
  }
  .row:hover,
  .row:has(:focus-visible) {
    background: var(--surface-2);
  }
  .play {
    position: absolute;
    inset: 0;
    border: none;
    border-radius: inherit;
    padding: 0;
    background: none;
  }
  /* Everything sits on the play button and lets clicks through, but links,
     the heart and the menu. */
  .row > :not(.play) {
    position: relative;
    pointer-events: none;
  }
  .row a,
  .row .like,
  .row :global(.menu) {
    pointer-events: auto;
  }
  .n {
    width: 24px;
    flex: none;
    display: inline-flex;
    justify-content: flex-end;
    align-items: center;
    color: var(--faint);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }
  .hover-play {
    display: none;
    color: var(--strong);
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    padding: 6px 0;
    line-height: 1.35;
  }
  .title,
  .artist {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    font-size: var(--fs-md);
    color: var(--text);
  }
  .artist {
    font-size: var(--fs-sm);
    color: var(--muted);
  }
  .artist a:hover {
    color: var(--text);
    text-decoration: underline;
  }
  .unplayable .title {
    color: var(--faint);
  }
  .now .title {
    color: var(--accent);
  }
  .quality,
  .plays {
    font-size: var(--fs-xs);
    color: var(--faint);
    white-space: nowrap;
  }
  .plays {
    color: var(--muted);
  }
  .time {
    font-size: var(--fs-sm);
    color: var(--muted);
    font-variant-numeric: tabular-nums;
    min-width: 40px;
    text-align: right;
  }
  .like {
    display: inline-flex;
  }

  /* The track playing: three bars that move while it sounds. */
  .eq {
    display: inline-flex;
    align-items: flex-end;
    gap: 2px;
    height: 12px;
    margin-right: 2px;
  }
  .eq i {
    width: 3px;
    height: 100%;
    background: var(--accent);
    border-radius: 1px;
    transform-origin: bottom;
    animation: eq 0.9s ease-in-out infinite alternate;
  }
  .eq i:nth-child(2) {
    animation-delay: -0.3s;
    animation-duration: 0.7s;
  }
  .eq i:nth-child(3) {
    animation-delay: -0.6s;
    animation-duration: 1.1s;
  }
  .eq.paused i {
    animation-play-state: paused;
  }
  @keyframes eq {
    from {
      transform: scaleY(0.25);
    }
    to {
      transform: scaleY(1);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .eq i {
      animation: none;
    }
    .eq i:nth-child(2) {
      transform: scaleY(0.6);
    }
    .eq i:nth-child(3) {
      transform: scaleY(0.4);
    }
  }

  /* With a mouse, the heart (unless liked) and the menu show on hover, and a
     play icon takes the number's place. On touch screens they always show. */
  @media (hover: hover) {
    .row:hover .num {
      display: none;
    }
    .row:hover .hover-play {
      display: inline-flex;
    }
    .like:not(.liked),
    .row :global(.menu) {
      opacity: 0;
    }
    .row:hover .like,
    .row:has(:focus-visible) .like,
    .row:hover :global(.menu),
    .row:has(:focus-visible) :global(.menu),
    .row :global(.menu:has([aria-expanded='true'])) {
      opacity: 1;
    }
  }
  @media (max-width: 640px) {
    .quality {
      display: none;
    }
  }
</style>
