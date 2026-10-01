<script>
  import { app } from '../lib/app.svelte.js'
  import { summary } from '../lib/status.js'

  const s = $derived(summary(app.status))
</script>

{#if s.text}
  <span class="estado {s.tone}" title={app.status?.scan?.lastError || s.text} role="status">
    <span class="dot"></span>
    <span class="text">{s.text}</span>
  </span>
{/if}

<style>
  .estado {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--faint);
    white-space: nowrap;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--faint);
  }
  .busy .dot {
    background: var(--accent);
    animation: pulse 1.2s ease-in-out infinite;
  }
  .warn {
    color: var(--warn);
  }
  .warn .dot {
    background: var(--warn);
  }
  @keyframes pulse {
    50% {
      opacity: 0.3;
    }
  }
  @media (max-width: 640px) {
    .text {
      display: none; /* the dot only; the title has the text */
    }
  }
</style>
