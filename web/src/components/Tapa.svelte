<script>
  // A square picture: one cover, or a mosaic of four. Pictures fade in as they
  // load. With a name, the picture sits on a muted gradient picked from it,
  // and when there is no picture the name is set on it: the title, like a
  // typographic sleeve, on a big square, the initials on a round or small one. Without a name, an icon.
  // A small one (a thumbnail in a row) has tighter corners; badge is an icon
  // in a corner and tint a colour that glows over the gradient.
  import { initials, placeholderBackground, titleSize } from '../lib/placeholder.js'
  import Icono from './Icono.svelte'

  let { srcs = [], alt = '', name = '', icon = 'disc', badge = '', tint = '', round = false, small = false } = $props()

  // A picture that fails to load is dropped: a mosaic missing one shows
  // the first picture left, and one cover failing tries the next.
  let failed = $state({})
  let loaded = $state({})
  const ok = $derived((srcs.length >= 4 ? srcs.slice(0, 4) : srcs).filter((s) => !failed[s]))
  const shown = $derived(ok.length >= 4 ? ok : ok.slice(0, 1))
  const label = $derived(name || alt)
  const bg = $derived.by(() => {
    if (!label) return ''
    const glow = tint ? `radial-gradient(circle at 75% 20%, color-mix(in srgb, ${tint} 55%, transparent), transparent 65%), ` : ''
    return glow + placeholderBackground(label)
  })
  const short = $derived(round || small)

  // A cached picture may be complete before the load handler is attached.
  function seen(img) {
    if (img.complete && img.naturalWidth > 0) loaded = { ...loaded, [img.getAttribute('src')]: true }
  }
</script>

<div
  class="tapa"
  class:round
  class:small
  class:mosaic={shown.length === 4}
  style:background-image={bg || null}
  role="img"
  aria-label={label}>
  {#if shown.length === 0}
    {#if bg && short}
      <span class="initials" aria-hidden="true">{initials(label)}</span>
    {:else if bg}
      <span class="title" style:font-size="max(var(--fs-xs), {titleSize(label)}cqi)" aria-hidden="true">{label}</span>
    {:else}
      <span class="none"><Icono name={icon} size={32} /></span>
    {/if}
  {:else}
    {#each shown as src (src)}
      <img
        {src}
        alt=""
        loading="lazy"
        decoding="async"
        class:loaded={loaded[src]}
        use:seen
        onload={() => (loaded = { ...loaded, [src]: true })}
        onerror={() => (failed = { ...failed, [src]: true })} />
    {/each}
  {/if}
  {#if badge}
    <span class="badge" aria-hidden="true"><Icono name={badge} size={16} /></span>
  {/if}
</div>

<style>
  .tapa {
    position: relative;
    aspect-ratio: 1;
    width: 100%;
    background-color: var(--surface-2);
    border-radius: var(--radius-lg);
    overflow: hidden;
    display: grid;
    container-type: inline-size;
  }
  .mosaic {
    grid-template-columns: 1fr 1fr;
    grid-template-rows: 1fr 1fr;
  }
  .small {
    border-radius: var(--radius);
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
    opacity: 0;
    transition: opacity 0.3s ease-out;
  }
  img.loaded {
    opacity: 1;
  }
  @media (prefers-reduced-motion: reduce) {
    img {
      transition: none;
    }
  }
  .none {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--faint);
  }
  .initials,
  .title {
    color: rgba(255, 255, 255, 0.9);
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.25);
    user-select: none;
  }
  .initials {
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 600;
    letter-spacing: 0.02em;
    /* Grows with the picture: a 40px thumbnail to the artist page's circle. */
    font-size: max(var(--fs-xs), 28cqi);
  }
  /* The title set like a typographic sleeve: heavy capitals, tight lines
     balanced, bottom left. Its size (titleSize) shrinks as the title grows. */
  .title {
    align-self: end;
    margin: 0 4.5cqi 5cqi;
    padding-top: 0.2em; /* keeps accents on the first line from being clipped */
    color: #fff;
    font-weight: 800;
    text-transform: uppercase;
    letter-spacing: -0.025em;
    line-height: 0.84; /* TITLE_LEADING */
    text-wrap: balance;
    overflow-wrap: anywhere;
    overflow: hidden;
  }
  .badge {
    position: absolute;
    left: 8px;
    top: 8px;
    display: flex;
    padding: 5px;
    border-radius: 50%;
    background: var(--accent);
    color: var(--on-accent);
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.35);
  }
</style>
