<script>
  // A shelf of Inicio: a title with its actions and a "Ver todo" link, over
  // a row of cards that scrolls sideways. The row runs to the page's edges
  // (its first card still lined up with the title), hides its scrollbar,
  // fades on the sides that have more, and pages with arrows on hover.
  import { edges, pageStep } from '../lib/shelf.js'
  import Icono from './Icono.svelte'

  // actions: icon buttons beside the title; children: the cards, one
  // element each.
  let { title, href = '', actions = null, children } = $props()

  let strip = $state()
  let more = $state({ start: false, end: false })

  function update() {
    if (!strip) return
    const e = edges(strip)
    if (e.start !== more.start || e.end !== more.end) more = e
  }

  // The row changes size with the window and its length with the cards.
  $effect(() => {
    if (!strip) return
    update()
    const ro = new ResizeObserver(update)
    ro.observe(strip)
    const mo = new MutationObserver(update)
    mo.observe(strip, { childList: true })
    return () => {
      ro.disconnect()
      mo.disconnect()
    }
  })

  function move(dir) {
    const css = getComputedStyle(strip)
    const first = strip.firstElementChild
    const cell = first ? first.getBoundingClientRect().width + (parseFloat(css.columnGap) || 0) : 0
    const inView = strip.clientWidth - (parseFloat(css.paddingLeft) || 0) - (parseFloat(css.paddingRight) || 0)
    const smooth = !matchMedia('(prefers-reduced-motion: reduce)').matches
    strip.scrollBy({ left: dir * pageStep(inView, cell), behavior: smooth ? 'smooth' : 'auto' })
  }
</script>

<section class="shelf">
  <div class="head">
    <h2>{title}</h2>
    {#if actions}<span class="actions">{@render actions()}</span>{/if}
    {#if href}<a class="all" {href}>Ver todo</a>{/if}
  </div>
  <div class="rail">
    <div class="strip" class:fade-start={more.start} class:fade-end={more.end} bind:this={strip} onscroll={update}>
      {@render children()}
    </div>
    {#if more.start}
      <button class="icon arrow prev" tabindex="-1" onclick={() => move(-1)} aria-label="Anteriores" title="Anteriores">
        <Icono name="left" size={20} />
      </button>
    {/if}
    {#if more.end}
      <button class="icon arrow next" tabindex="-1" onclick={() => move(1)} aria-label="Siguientes" title="Siguientes">
        <Icono name="right" size={20} />
      </button>
    {/if}
  </div>
</section>

<style>
  .shelf {
    --cell: 168px;
    --gap: var(--space-4);
    /* How far the fade reaches into the row: past the gutter, into the card. */
    --fade: calc(var(--gutter) + var(--space-7));
    margin-bottom: var(--space-6);
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
    min-width: 0;
  }
  .head h2 {
    margin: 0;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .actions {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }
  .all {
    margin-left: auto;
    flex: none;
    font-size: var(--fs-sm);
    font-weight: 500;
    color: var(--muted);
  }
  .all:hover {
    color: var(--strong);
  }
  /* Full width: out to the page's edges, past its gutter. */
  .rail {
    position: relative;
    margin-inline: calc(-1 * var(--gutter));
  }
  .strip {
    display: flex;
    gap: var(--gap);
    overflow-x: auto;
    overscroll-behavior-x: contain;
    scroll-snap-type: x proximity;
    padding: var(--space-1) var(--gutter);
    scroll-padding-inline: var(--gutter);
    scrollbar-width: none;
  }
  .strip::-webkit-scrollbar {
    display: none;
  }
  .strip > :global(*) {
    flex: 0 0 var(--cell);
    min-width: 0;
    scroll-snap-align: start;
  }
  .strip.fade-start,
  .strip.fade-end {
    --fade-l: 0px;
    --fade-r: 0px;
    mask-image: linear-gradient(to right, transparent, #000 var(--fade-l), #000 calc(100% - var(--fade-r)), transparent);
  }
  .strip.fade-start {
    --fade-l: var(--fade);
  }
  .strip.fade-end {
    --fade-r: var(--fade);
  }
  /* On the covers' middle, over the fade. */
  .arrow {
    position: absolute;
    top: calc(var(--space-1) + var(--cell) / 2);
    translate: 0 -50%;
    width: 40px;
    height: 40px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    color: var(--strong);
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.5);
    opacity: 0;
    transition: opacity 0.15s;
  }
  .prev {
    left: max(var(--space-2), calc(var(--gutter) - 20px));
  }
  .next {
    right: max(var(--space-2), calc(var(--gutter) - 20px));
  }
  .rail:hover .arrow {
    opacity: 1;
  }
  .arrow:hover:not(:disabled) {
    background: var(--line-strong);
    color: var(--strong);
  }
  /* Phones swipe. */
  @media (hover: none) {
    .arrow {
      display: none;
    }
  }
  @media (max-width: 640px) {
    .shelf {
      --cell: 140px;
      --gap: var(--space-3);
      --fade: calc(var(--gutter) + var(--space-5));
    }
  }
</style>
