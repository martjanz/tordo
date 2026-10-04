<script>
  // An album card: cover, title, artist and year, with a play button.
  import { albumCoverURL } from '../lib/api.js'
  import { play } from '../lib/playback.svelte.js'
  import { albumHref } from '../lib/router.js'
  import Icono from './Icono.svelte'
  import Tapa from './Tapa.svelte'

  let { album } = $props()
</script>

<div class="card">
  <div class="pic">
    <a href={albumHref(album.id)}>
      <Tapa srcs={album.picture === false ? [] : [albumCoverURL(album.id, 320)]} alt={album.title} />
    </a>
    <button class="icon play" onclick={() => play({ album: album.id })} title="Reproducir" aria-label="Reproducir {album.title}">
      <Icono name="play" size={18} />
    </button>
  </div>
  <a href={albumHref(album.id)} class="title">{album.title}</a>
  <span class="sub">{album.artist}{album.year ? ` · ${album.year}` : ''}</span>
</div>

<style>
  .card {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .pic {
    position: relative;
    margin-bottom: 8px;
  }
  .pic a {
    display: block;
  }
  .title,
  .sub {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
  }
  .play {
    position: absolute;
    right: 8px;
    bottom: 8px;
    width: 40px;
    height: 40px;
    background: var(--accent);
    color: var(--on-accent) !important;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .card:hover .play,
  .play:focus-visible {
    opacity: 1;
  }
  @media (hover: none) {
    .play {
      display: none; /* phones open the album */
    }
  }
</style>
