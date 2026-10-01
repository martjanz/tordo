<script>
  import Acciones from '../components/Acciones.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import Tapa from '../components/Tapa.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count, folderName, length, quality } from '../lib/format.js'
  import { play } from '../lib/player.svelte.js'
  import { folderHref } from '../lib/router.js'

  let { id } = $props()

  let album = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    let stale = false
    api
      .album(id)
      .then((a) => {
        if (!stale) (album = a), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.status === 404 ? 'Este álbum ya no está en el catálogo.' : e.message
      })
    return () => (stale = true)
  })

  const summary = $derived(
    album
      ? [album.year || '', count(album.tracks.length, 'pista', 'pistas'), length(album.durationMs)].filter(Boolean).join(' · ')
      : '',
  )

  // formats sums up the album's formats: "FLAC 16/44,1", or several.
  const formats = $derived(album ? [...new Set(album.tracks.map(quality))].join(' · ') : '')
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if album}
  <div class="head">
    <div class="pic"><Tapa srcs={[albumCoverURL(album.id, 640)]} alt={album.title} /></div>
    <div class="info">
      <span class="label">{album.compilation ? 'Compilado' : 'Álbum'}</span>
      <h1>{album.title}</h1>
      <p class="artist">{album.artist}</p>
      <p class="sub">{summary}</p>
      <p class="sub">{formats}</p>
      <a class="path" href={folderHref(album.dir)}>{folderName(album.dir)}</a>
      <Acciones what={{ album: album.id }} />
    </div>
  </div>
  <ListaPistas tracks={album.tracks} albumArtist={album.artist} onplay={(t) => play({ album: album.id }, { startId: t.id })} />
{/if}

<style>
  .head {
    display: flex;
    gap: 32px;
    align-items: flex-end;
    margin: 8px 0 24px;
  }
  .pic {
    width: 240px;
    flex: none;
  }
  .info {
    min-width: 0;
  }
  .artist {
    font-size: 17px;
    color: var(--text);
    margin: 0 0 6px;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0 0 2px;
  }
  .path:hover {
    color: var(--text);
  }
  @media (max-width: 640px) {
    .head {
      flex-direction: column;
      align-items: stretch;
      gap: 16px;
    }
    .pic {
      width: min(100%, 320px);
      align-self: center;
    }
  }
</style>
