<script>
  // The ✦ toggle of an album: Preferidos, a shortlist of albums to come back
  // to (the ♥ is for tracks, artists and collections). In browse-only mode: a
  // static ✦ when marked, nothing otherwise. Same mark as the album's ♥ in the
  // API (kind "album").
  import { app } from '../lib/app.svelte.js'
  import { isLiked } from '../lib/marks.js'
  import { mine, toggleLike } from '../lib/mine.svelte.js'
  import Icono from './Icono.svelte'

  let { ref, size = 18 } = $props()

  const on = $derived(isLiked(mine.marks, 'album', ref))
</script>

{#if !app.status?.readOnly}
  <button class="icon preferred" class:on aria-pressed={on} onclick={() => toggleLike('album', ref)}
    title={on ? 'Quitar de Preferidos' : 'Agregar a Preferidos'} aria-label={on ? 'Quitar de Preferidos' : 'Agregar a Preferidos'}>
    <Icono name="sparkle" {size} />
  </button>
{:else if on}
  <span class="preferred on static" role="img" aria-label="Preferido"><Icono name="sparkle" {size} /></span>
{/if}

<style>
  .preferred.on {
    color: var(--accent);
  }
  .preferred.on :global(svg) {
    fill: currentColor;
  }
  .static {
    display: inline-flex;
    padding: 6px;
  }
</style>
