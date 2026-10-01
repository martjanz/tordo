<script>
  // Lists: for now, the collections found in the folders. Playlists and
  // likes come in M4.
  import Tapa from '../components/Tapa.svelte'
  import { albumCoverURL, api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count } from '../lib/format.js'
  import { folderHref } from '../lib/router.js'

  // The albums with a picture: the folder's image or an embedded one.
  const withPicture = (albums) => albums.filter((a) => a.coverPath || a.coverFileId).slice(0, 4)

  let collections = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    api
      .collections()
      .then((cs) => ((collections = cs), (error = '')))
      .catch((e) => (error = e.message))
  })
</script>

<h1>Listas</h1>
<h2>Colecciones</h2>
{#if error}
  <p class="empty">{error}</p>
{:else if collections?.length === 0}
  <p class="empty">
    No se encontraron colecciones: carpetas con álbumes de varios artistas, como «Latinoamérica 600».
  </p>
{:else if collections}
  <div class="grid">
    {#each collections as c (c.path)}
      <a class="card" href={folderHref(c.path)}>
        <Tapa srcs={withPicture(c.albums).map((a) => albumCoverURL(a.id, 320))} icon="lists" alt={c.name} />
        <span class="name">{c.name}</span>
        <span class="sub">{count(c.albums.length, 'álbum', 'álbumes')}</span>
      </a>
    {/each}
  </div>
{/if}

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .name {
    margin-top: 4px;
    color: var(--strong);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    font-size: 13px;
    color: var(--muted);
  }
</style>
