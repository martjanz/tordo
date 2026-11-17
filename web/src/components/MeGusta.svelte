<script>
  // A heart toggle for a track, artist or collection (albums have Preferido). In browse-only
  // mode: a static heart when liked, nothing otherwise.
  import { app } from '../lib/app.svelte.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { isLiked } from '../lib/marks.js'
  import { mine, toggleLike } from '../lib/mine.svelte.js'
  import Icono from './Icono.svelte'

  let { kind, ref, size = 18 } = $props()

  const liked = $derived(isLiked(mine.marks, kind, ref))
</script>

{#if !app.status?.readOnly}
  <button class="icon heart" class:on={liked} aria-pressed={liked} onclick={() => toggleLike(kind, ref)}
    title={liked ? t('marks.unlike') : t('facets.liked.yes')} aria-label={liked ? t('marks.unlike') : t('facets.liked.yes')}>
    <Icono name="heart" {size} />
  </button>
{:else if liked}
  <span class="heart on static" role="img" aria-label={t('facets.liked.yes')}><Icono name="heart" {size} /></span>
{/if}

<style>
  .heart.on {
    color: var(--accent);
  }
  .heart.on :global(svg) {
    fill: currentColor;
  }
  .static {
    display: inline-flex;
    padding: 6px;
  }
</style>
