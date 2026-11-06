<script>
  // A spinner and what the server is doing, for a page whose data is on its
  // way. It shows only after a moment, so a quick answer does not flash it.
  import { app } from '../lib/app.svelte.js'
  import { loadingMessage } from '../lib/status.js'

  let { delay = 400 } = $props()

  const msg = $derived(loadingMessage(app.status, app.lost))

  let shown = $state(false)

  $effect(() => {
    const t = setTimeout(() => (shown = true), delay)
    return () => clearTimeout(t)
  })
</script>

{#if shown}
  <div class="loading" role="status">
    <p class="title"><span class="spinner" aria-hidden="true"></span> {msg.title}</p>
    {#if msg.detail}<p class="detail">{msg.detail}</p>{/if}
  </div>
{/if}

<style>
  .loading {
    color: var(--muted);
    padding: 48px 0;
    text-align: center;
  }
  .title {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    margin: 0;
  }
  .detail {
    margin: 8px 0 0;
    font-size: 13px;
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
