<script>
  // A spinner and "Cargando…" for a page whose data is on its way. It shows
  // only after a moment, so a quick answer does not flash it.
  let { delay = 400 } = $props()

  let shown = $state(false)

  $effect(() => {
    const t = setTimeout(() => (shown = true), delay)
    return () => clearTimeout(t)
  })
</script>

{#if shown}
  <p class="loading" role="status"><span class="spinner" aria-hidden="true"></span> Cargando…</p>
{/if}

<style>
  .loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    color: var(--muted);
    padding: 48px 0;
    margin: 0;
  }
  .spinner {
    width: 18px;
    height: 18px;
    border: 2px solid var(--line-strong);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation-duration: 2.4s;
    }
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
