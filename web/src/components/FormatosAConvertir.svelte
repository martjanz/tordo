<script>
  // Revisar → Formatos a convertir: the files in formats worth converting.
  // Detection only: nothing is converted.
  import { api } from '../lib/api.js'
  import { app } from '../lib/app.svelte.js'
  import { count, duration, folderName, size } from '../lib/format.js'
  import { albumHref, folderHref } from '../lib/router.js'

  let { onload } = $props()

  let data = $state(null)
  let error = $state('')

  $effect(() => {
    app.generation
    let stale = false
    api
      .toConvert()
      .then((d) => {
        if (stale) return
        data = d
        error = ''
        onload?.(d.files)
      })
      .catch((e) => {
        if (!stale) error = e.message
      })
    return () => (stale = true)
  })

  const label = (key) => data?.formats.find((f) => f.key === key)?.label ?? key.toUpperCase()
</script>

<p class="note">Formatos que conviene convertir (WMA, WAV, AIFF, APE, WavPack, ALAC). Tordo solo los detecta: no convierte nada.</p>

{#if error}
  <p class="empty">{error}</p>
{:else if data}
  {#if data.files === 0}
    <p class="empty">No hay archivos en esos formatos.</p>
  {:else}
    <p class="total">{count(data.files, 'archivo', 'archivos')} · {size(data.bytes)}</p>
    <p class="chips">
      {#each data.formats as f (f.key)}
        <span class="chip">{f.label}: {count(f.files, 'archivo', 'archivos')} · {size(f.bytes)}</span>
      {/each}
    </p>
    <ul class="items">
      {#each data.items as it (`${it.albumId}/${it.dir}/${it.format}`)}
        <li>
          {#if it.albumId}
            <a class="title" href={albumHref(it.albumId)}>{it.title}</a>
          {:else}
            <a class="title" href={folderHref(it.dir)}>{folderName(it.dir) || it.title}</a>
          {/if}
          <span class="artist">{it.artist}</span>
          <span class="fmt">{label(it.format)}</span>
          <span class="meta">{count(it.files, 'archivo', 'archivos')} · {size(it.bytes)} · {duration(it.durationMs)}</span>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<style>
  .note {
    margin: 0 0 12px;
    color: var(--muted);
  }
  .total {
    margin: 0 0 8px;
    color: var(--strong);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0 0 14px;
  }
  .chip {
    font-size: 12px;
    padding: 1px 9px;
    border: 1px solid var(--line-strong);
    border-radius: 12px;
    color: var(--muted);
  }
  .items {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .items li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 2px 14px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .title {
    color: var(--strong);
    font-weight: 500;
  }
  .title:hover {
    text-decoration: underline;
  }
  .artist {
    color: var(--muted);
    flex: 1 1 120px;
    min-width: 0;
  }
  .fmt {
    font-size: 12px;
    padding: 0 7px;
    border: 1px solid var(--line-strong);
    border-radius: 3px;
    color: var(--text);
  }
  .meta {
    color: var(--muted);
    font-size: 13px;
  }
</style>
