<script>
  // Five stars for rating a track or album. Clicking the current rating
  // clears it; arrow keys move it. Browse-only mode: the stars as text.
  import { app } from '../lib/app.svelte.js'
  import { ratingOf } from '../lib/marks.js'
  import { mine, rate } from '../lib/mine.svelte.js'
  import Icono from './Icono.svelte'

  let { kind, id, size = 18 } = $props()

  const stars = $derived(ratingOf(mine.marks, kind, id))

  function set(n) {
    rate(kind, id, n === stars ? 0 : n)
  }

  function onkeydown(event) {
    const step = { ArrowRight: 1, ArrowUp: 1, ArrowLeft: -1, ArrowDown: -1 }[event.key]
    if (!step) return
    event.preventDefault()
    const n = Math.max(0, Math.min(5, stars + step))
    if (n !== stars) rate(kind, id, n)
  }
</script>

{#if !app.status?.readOnly}
  <div class="stars" role="radiogroup" aria-label="Puntaje" tabindex="-1" {onkeydown}>
    {#each [1, 2, 3, 4, 5] as n (n)}
      <button class="icon star" class:on={n <= stars} role="radio" aria-checked={n === stars}
        aria-label={n === 1 ? '1 estrella' : `${n} estrellas`} tabindex={n === (stars || 1) ? 0 : -1}
        onclick={() => set(n)}>
        <Icono name="star" {size} />
      </button>
    {/each}
  </div>
{:else if stars}
  <span class="text" role="img" aria-label="Puntaje: {stars} de 5">{'★'.repeat(stars)}{'☆'.repeat(5 - stars)}</span>
{/if}

<style>
  .stars {
    display: inline-flex;
  }
  .star {
    padding: 3px !important;
  }
  .star.on :global(svg) {
    fill: currentColor;
  }
  .text {
    color: var(--accent);
    letter-spacing: 2px;
  }
</style>
