<script>
  // An artist: picture, country, years, their albums by year and their
  // tracks on others' albums (compilations).
  import Acciones from '../components/Acciones.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import MeGusta from '../components/MeGusta.svelte'
  import Tapa from '../components/Tapa.svelte'
  import TarjetaAlbum from '../components/TarjetaAlbum.svelte'
  import { albumCoverURL, api, artistMBURL } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count, country, years } from '../lib/format.js'
  import { play } from '../lib/player.svelte.js'
  import { albumHref, exploreHref } from '../lib/router.js'
  import { artistStation } from '../lib/stations.js'

  let { key } = $props()

  let artist = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    let stale = false
    api
      .artist(key)
      .then((a) => {
        if (!stale) (artist = a), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.status === 404 ? 'Este artista ya no está en el catálogo.' : e.message
      })
    return () => (stale = true)
  })

  // span is the years of the artist's albums (MusicBrainz's begin is a
  // person's birth, not when they started).
  const span = $derived(artist ? years(artist.firstYear, artist.lastYear) : '')

  const TYPES = { Person: 'Solista', Group: 'Grupo', Orchestra: 'Orquesta', Choir: 'Coro' }
</script>

{#if error}
  <p class="empty">{error}</p>
{:else if artist}
  <div class="head">
    <div class="pic">
      <Tapa srcs={artist.cover ? [albumCoverURL(artist.cover, 640)] : []} icon="user" round alt={artist.name} />
    </div>
    <div class="info">
      <span class="label">{TYPES[artist.type] ?? 'Artista'}</span>
      <h1>{artist.name}</h1>
      <p class="sub">
        {#if artist.country}<a href={exploreHref({ pais: artist.country })}>{country(artist.country)}</a> · {/if}
        {#if span}{span} · {/if}
        {count(artist.albums.length, 'álbum', 'álbumes')}
      </p>
      <p class="sub links">
        <a href={exploreHref({ artista: artist.key }, 'pistas')}>Ver sus pistas</a>
        {#if artist.mbid}· <a href={artistMBURL(artist.mbid)} target="_blank" rel="noreferrer">MusicBrainz ↗</a>{/if}
      </p>
      <div class="acts">
        <Acciones what={{ artist: artist.key }} station={artistStation(artist)} />
        <MeGusta kind="artist" ref={artist.key} size={20} />
      </div>
    </div>
  </div>

  {#if artist.top?.length}
    <h2>Más escuchadas</h2>
    <ListaPistas
      tracks={artist.top}
      numbered={false}
      showAlbum
      albumArtist={artist.name}
      onplay={(t) => play({ tracks: artist.top.map((x) => x.id) }, { startId: t.id })} />
  {/if}

  {#if artist.albums.length}
    <h2>Álbumes</h2>
    <div class="grid">
      {#each artist.albums as a (a.id)}
        <TarjetaAlbum album={a} />
      {/each}
    </div>
  {/if}

  {#if artist.appearances.length}
    <h2>En compilados y otros álbumes</h2>
    {#each artist.appearances as ap (ap.album.id)}
      <p class="appearance">
        <a href={albumHref(ap.album.id)}>{ap.album.title}</a>
        <span>{[ap.album.artist, ap.album.year || ''].filter(Boolean).join(' · ')}</span>
      </p>
      <ListaPistas
        tracks={ap.tracks}
        numbered={false}
        albumArtist={artist.name}
        onplay={(t) => play({ tracks: ap.tracks.map((x) => x.id) }, { startId: t.id })} />
    {/each}
  {/if}
{/if}

<style>
  .head {
    display: flex;
    gap: 32px;
    align-items: flex-end;
    margin: 8px 0 24px;
  }
  .pic {
    width: 200px;
    flex: none;
  }
  .info {
    min-width: 0;
  }
  .acts {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0 0 2px;
  }
  .sub a:hover {
    color: var(--text);
    text-decoration: underline;
  }
  .appearance {
    display: flex;
    gap: 10px;
    align-items: baseline;
    margin: 20px 0 4px;
  }
  .appearance a {
    color: var(--strong);
    font-weight: 500;
  }
  .appearance span {
    font-size: 13px;
    color: var(--faint);
  }
  @media (max-width: 640px) {
    .head {
      flex-direction: column;
      align-items: center;
      text-align: center;
      gap: 16px;
    }
    .pic {
      width: min(60%, 240px);
    }
  }
</style>
