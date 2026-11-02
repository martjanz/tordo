<script>
  // The status indicator of the top bar. Hovering it (with a mouse) or
  // clicking it opens a panel with the steps, the progress and the catalog's
  // figures; a click keeps it open until a click elsewhere or Esc.
  import { app } from '../lib/app.svelte.js'
  import { panel, summary } from '../lib/status.js'

  const s = $derived(summary(app.status, app.lost))
  const p = $derived(panel(app.status, app.lost))

  let open = $state(false)
  let pinned = false
  let root = $state()
  let leaving

  const canHover = () => window.matchMedia('(hover: hover)').matches

  function enter() {
    clearTimeout(leaving)
    if (canHover()) open = true
  }

  function leave() {
    if (pinned) return
    leaving = setTimeout(() => (open = false), 200)
  }

  function toggle() {
    pinned = !(open && pinned)
    open = pinned
  }

  function close() {
    open = pinned = false
  }

  function onwindowclick(event) {
    if (open && root && !root.contains(event.target)) close()
  }

  function onkeydown(event) {
    if (open && event.key === 'Escape') {
      close()
      root?.querySelector('.estado')?.focus()
    }
  }
</script>

<svelte:window onclick={onwindowclick} {onkeydown} />

{#if s.text}
  <span class="wrap" bind:this={root} onmouseenter={enter} onmouseleave={leave} role="presentation">
    <button class="estado {s.tone}" type="button" onclick={toggle} aria-expanded={open} aria-haspopup="dialog"
      aria-label="{s.text}. Ver detalles">
      <span class="dot"></span>
      <span class="text" role="status">{s.text}</span>
    </button>
    {#if open}
      <div class="panel" role="dialog" aria-label="Estado de Tordo">
        <div class="head {p.tone}">
          <span class="dot"></span>
          <strong>{p.title}</strong>
        </div>
        {#if p.error}<p class="error">{p.error}</p>{/if}

        {#if p.steps}
          <ol class="steps">
            {#each p.steps as step, i (step.label)}
              <li class={step.state}>
                <span class="mark">{step.state === 'done' ? '✓' : i + 1}</span>
                <span>{step.label}</span>
              </li>
            {/each}
          </ol>
        {/if}

        {#if p.progress}
          <div class="progress">
            <div class="row">
              <span>{p.progress.label}</span>
              <span class="num">{p.progress.value}{p.progress.percent != null ? ` · ${p.progress.percent} %` : ''}</span>
            </div>
            <div class="track" class:indeterminate={p.progress.percent == null}>
              <div class="fill" style:width={p.progress.percent != null ? `${Math.max(p.progress.percent, 1)}%` : null}></div>
            </div>
            {#if p.progress.eta}<div class="eta">{p.progress.eta}</div>{/if}
          </div>
        {/if}

        {#if p.facts.length}
          <dl class="facts">
            {#each p.facts as f (f.label)}
              <div>
                <dd>{#if f.href}<a href={f.href} onclick={close}>{f.value}</a>{:else}{f.value}{/if}</dd>
                <dt>{f.label}</dt>
              </div>
            {/each}
          </dl>
        {/if}

        {#each p.notes as note (note)}<p class="note">{note}</p>{/each}

        <a class="more" href="/ayuda#como-funciona" onclick={close}>Cómo funciona →</a>
      </div>
    {/if}
  </span>
{/if}

<style>
  .wrap {
    position: relative;
    display: inline-flex;
  }
  .estado {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--faint);
    white-space: nowrap;
    background: none;
    border: 1px solid transparent;
    border-radius: 999px;
    padding: 4px 8px;
    cursor: pointer;
  }
  .estado:hover,
  .estado[aria-expanded='true'] {
    border-color: var(--line-strong);
    background: var(--surface);
  }
  .dot {
    width: 7px;
    height: 7px;
    flex: none;
    border-radius: 50%;
    background: var(--faint);
  }
  .busy .dot {
    background: var(--accent);
    animation: pulse 1.2s ease-in-out infinite;
  }
  .ok {
    color: var(--ok);
  }
  .ok .dot {
    background: var(--ok);
  }
  .warn {
    color: var(--warn);
  }
  .warn .dot {
    background: var(--warn);
  }
  .down {
    color: var(--down);
  }
  .down .dot {
    background: var(--down);
  }

  .panel {
    position: absolute;
    right: 0;
    top: calc(100% + 6px);
    z-index: 30;
    width: 320px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: 10px;
    padding: 14px 16px;
    box-shadow: 0 10px 30px rgb(0 0 0 / 0.45);
    font-size: 13px;
    color: var(--text);
    white-space: normal;
    cursor: default;
  }
  /* The gap between the button and the panel still counts as hovering. */
  .panel::before {
    content: '';
    position: absolute;
    inset: -8px 0 auto 0;
    height: 8px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 15px;
  }
  .head strong {
    color: var(--strong);
    font-weight: 600;
  }
  .error {
    margin: 8px 0 0;
    padding: 8px 10px;
    border-radius: 6px;
    background: rgb(224 80 80 / 0.12);
    color: var(--text);
    overflow-wrap: anywhere;
  }

  .steps {
    list-style: none;
    margin: 14px 0 0;
    padding: 0;
    display: grid;
    gap: 2px;
  }
  .steps li {
    position: relative;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 3px 0;
    color: var(--faint);
  }
  /* The line that joins the marks. */
  .steps li:not(:last-child)::after {
    content: '';
    position: absolute;
    left: 9px;
    top: 24px;
    height: 8px;
    border-left: 1px solid var(--line-strong);
  }
  .mark {
    width: 19px;
    height: 19px;
    flex: none;
    display: grid;
    place-items: center;
    border-radius: 50%;
    border: 1px solid var(--line-strong);
    font-size: 10px;
    font-variant-numeric: tabular-nums;
  }
  .steps .done {
    color: var(--muted);
  }
  .steps .done .mark {
    border-color: transparent;
    background: var(--line);
    color: var(--ok);
    font-size: 11px;
  }
  .steps .busy,
  .steps .warn {
    color: var(--strong);
    font-weight: 600;
  }
  .steps .busy .mark {
    border-color: var(--accent);
    background: var(--accent);
    color: var(--on-accent);
  }
  .steps .warn .mark {
    border-color: var(--warn);
    color: var(--warn);
  }

  .progress {
    margin-top: 14px;
  }
  .row {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    color: var(--muted);
  }
  .num {
    color: var(--text);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .track {
    position: relative;
    height: 6px;
    margin-top: 6px;
    border-radius: 3px;
    background: var(--line);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    border-radius: 3px;
    background: var(--accent);
    transition: width 0.6s ease;
  }
  .indeterminate .fill {
    width: 35%;
    animation: slide 1.4s ease-in-out infinite;
  }
  .eta {
    margin-top: 6px;
    color: var(--muted);
    font-size: 12px;
  }

  .facts {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(84px, 1fr));
    gap: 6px;
    margin: 14px 0 0;
  }
  .facts div {
    padding: 8px 10px;
    border-radius: 6px;
    background: var(--surface);
  }
  .facts dd {
    margin: 0;
    color: var(--strong);
    font-size: 15px;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }
  .facts dd a {
    color: var(--accent);
  }
  .facts dt {
    color: var(--faint);
    font-size: 11px;
  }

  .note {
    margin: 12px 0 0;
    color: var(--muted);
    line-height: 1.4;
  }
  .more {
    display: inline-block;
    margin-top: 12px;
    color: var(--accent);
    font-size: 12px;
  }

  @keyframes pulse {
    50% {
      opacity: 0.3;
    }
  }
  @keyframes slide {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(290%);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .busy .dot,
    .indeterminate .fill {
      animation: none;
    }
  }
  @media (max-width: 640px) {
    .text {
      display: none; /* the dot only; a tap opens the panel */
    }
    .estado {
      padding: 8px;
    }
    .panel {
      position: fixed;
      top: calc(var(--nav-h) + 4px);
      left: var(--gutter);
      right: var(--gutter);
      width: auto;
      max-height: calc(100dvh - var(--nav-h) - var(--player-h) - var(--tabs-h) - 16px);
      overflow-y: auto;
    }
  }
</style>
