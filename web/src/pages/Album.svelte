<script>
  import Acciones from '../components/Acciones.svelte'
  import Preferido from '../components/Preferido.svelte'
  import Puntaje from '../components/Puntaje.svelte'
  import Identificar from '../components/Identificar.svelte'
  import ListaPistas from '../components/ListaPistas.svelte'
  import Tapa from '../components/Tapa.svelte'
  import { albumCoverURL, api, releaseURL } from '../lib/api.js'
  import { app, listenMode } from '../lib/app.svelte.js'
  import { artistLabel, country, folderName, formatName, length, quality } from '../lib/format.js'
  import { msg, t, text } from '../lib/i18n/index.svelte.js'
  import { play } from '../lib/playback.svelte.js'
  import { albumStation } from '../lib/stations.js'
  import { genreLinks, identification } from '../lib/review.js'
  import { albumHref, artistHref, duplicatesHref, exploreHref, folderHref } from '../lib/router.js'

  let { id } = $props()

  let album = $state(null)
  let error = $state('')
  let correcting = $state(false)

  $effect(() => {
    app.generation
    let stale = false
    api
      .album(id)
      .then((a) => {
        if (!stale) (album = a), (error = '')
      })
      .catch((e) => {
        if (!stale) error = e.status === 404 ? msg('album.gone') : e.message
      })
    return () => (stale = true)
  })

  // year is the original year, with the edition's when it differs.
  const year = $derived.by(() => {
    if (!album) return ''
    const y = album.originalYear || album.year
    return album.originalYear && album.year && album.year !== album.originalYear ? t('album.edition', { year: y, edition: album.year }) : y || ''
  })

  const summary = $derived(
    album ? [year, t('count.tracks', { n: album.tracks.length }), length(album.durationMs)].filter(Boolean).join(' · ') : '',
  )

  // formats sums up the album's formats: "FLAC 16/44,1", or several.
  const formats = $derived(album ? [...new Set(album.tracks.map(quality))].join(' · ') : '')
  const unplayable = $derived(album ? album.tracks.filter((t) => !t.playable).length : 0)
  const identified = $derived(album && (album.status === 'auto' || album.status === 'manual') && album.releaseId)

  function corrected(a) {
    album = a
    correcting = false
  }
</script>

{#if error}
  <p class="empty">{text(error)}</p>
{:else if album}
  <div class="head">
    <div class="pic"><Tapa srcs={album.picture ? [albumCoverURL(album.id, 640)] : []} alt={album.title} /></div>
    <div class="info">
      <span class="label">{album.compilation ? t('album.kindCompilation') : t('album.kindAlbum')}</span>
      <h1>{album.title}</h1>
      <p class="artist">
        {#if album.artistKey}<a href={artistHref(album.artistKey)}>{artistLabel(album.artist)}</a>{:else}{artistLabel(album.artist)}{/if}
      </p>
      <p class="sub">
        {summary}{#if album.country}{' · '}<a href={exploreHref({ country: album.country })}>{country(album.country)}</a>{/if}
      </p>
      {#if album.genres.length}
        <p class="genres">
          {#each genreLinks(album.genres) as g (g.label)}
            <a class="chip" href={exploreHref(g.facets)}>{g.label}</a>
          {/each}
        </p>
      {/if}
      <p class="sub">{formats}{unplayable ? ` · ${t('count.tracksNeedFfmpeg', { n: unplayable })}` : ''}</p>
      <a class="path" href={folderHref(album.dir)}>{folderName(album.dir)}</a>
      <div class="marks">
        <Preferido ref={album.id} size={20} />
        <Puntaje kind="album" id={album.id} />
      </div>
      <Acciones what={{ album: album.id }} station={albumStation(album)} />
    </div>
  </div>
  <ListaPistas tracks={album.tracks} albumArtist={album.artist} onplay={(t) => play({ album: album.id }, { startId: t.id })} />

  {#if album.versions?.length && !listenMode()}
    <section class="versions">
      <h2>{t('album.versions')}</h2>
      <ul>
        {#each album.versions as v (v.albumId)}
          <li>
            <a class="vtitle" href={albumHref(v.albumId)}>{v.title}</a>
            <span>{artistLabel(v.artist)}</span>
            {#if v.year}<span>{v.year}</span>{/if}
            <span>{v.formats.map(formatName).join(' · ')}</span>
            <span>{t('count.sharedSongs', { n: v.shared })}</span>
            {#if v.kind === 'exact'}<span class="exact">{t('album.exactCopy')}</span>{/if}
          </li>
        {/each}
      </ul>
      <a class="sub" href={duplicatesHref(album.versions.some((v) => v.kind === 'exact') ? 'exact' : 'formats')}>{t('album.seeDuplicates')}</a>
    </section>
  {/if}

  <section class="ident">
    <p class="sub">
      {identification(album)}
      {#if identified}· <a href={releaseURL(album.releaseId)} target="_blank" rel="noreferrer">{t('album.viewMB')}</a>{/if}
      {#if album.title !== album.tagTitle}· {t('album.inTags', { title: album.tagTitle })}{/if}
    </p>
    {#if !app.status?.readOnly && !listenMode()}
      {#if correcting}
        <Identificar {album} onchange={corrected} />
        <button onclick={() => (correcting = false)}>{t('common.close')}</button>
      {:else}
        <button onclick={() => (correcting = true)}>{t('album.correct')}</button>
      {/if}
    {/if}
  </section>
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
  .artist a:hover,
  .sub a:hover {
    text-decoration: underline;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
    margin: 0 0 2px;
  }
  .genres {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 6px 0;
  }
  .chip {
    font-size: 12px;
    padding: 1px 9px;
    border: 1px solid var(--line-strong);
    border-radius: 12px;
    color: var(--muted);
  }
  .chip:hover {
    border-color: var(--accent);
    color: var(--accent);
  }
  .path:hover {
    color: var(--text);
  }
  .marks {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 6px 0 4px;
  }
  .versions {
    margin-top: 32px;
  }
  .versions ul {
    list-style: none;
    margin: 0 0 8px;
    padding: 0;
  }
  .versions li {
    display: flex;
    flex-wrap: wrap;
    gap: 2px 14px;
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
    font-size: 13px;
    color: var(--muted);
  }
  .vtitle {
    color: var(--strong);
    font-weight: 500;
  }
  .vtitle:hover {
    text-decoration: underline;
  }
  .exact {
    color: var(--accent);
  }
  .ident {
    margin-top: 32px;
    padding-top: 16px;
    border-top: 1px solid var(--line);
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .ident :global(.identificar) {
    width: min(100%, 720px);
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
