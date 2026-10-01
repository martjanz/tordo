<script>
  // A square picture: one cover, or a mosaic of four, with a placeholder
  // when there is none.
  import Icono from './Icono.svelte'

  let { srcs = [], alt = '', icon = 'disc', round = false } = $props()

  // A picture that fails to load is dropped: a mosaic missing one shows
  // the first picture left, and one cover failing tries the next.
  let failed = $state({})
  const ok = $derived((srcs.length >= 4 ? srcs.slice(0, 4) : srcs).filter((s) => !failed[s]))
  const shown = $derived(ok.length >= 4 ? ok : ok.slice(0, 1))
</script>

<div class="tapa" class:round class:mosaic={shown.length === 4} role="img" aria-label={alt}>
  {#if shown.length === 0}
    <span class="none"><Icono name={icon} size={32} /></span>
  {:else}
    {#each shown as src (src)}
      <img {src} alt="" loading="lazy" decoding="async" onerror={() => (failed = { ...failed, [src]: true })} />
    {/each}
  {/if}
</div>

<style>
  .tapa {
    aspect-ratio: 1;
    width: 100%;
    background: var(--surface-2);
    border-radius: var(--radius);
    overflow: hidden;
    display: grid;
  }
  .mosaic {
    grid-template-columns: 1fr 1fr;
    grid-template-rows: 1fr 1fr;
  }
  .round {
    border-radius: 50%;
  }
  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    min-width: 0;
    min-height: 0;
  }
  .none {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--faint);
  }
</style>
