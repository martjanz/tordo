<script>
  import { listenMode } from '../lib/app.svelte.js'
  import { SECTIONS } from '../lib/ayuda.js'
</script>

<svelte:head><title>Ayuda · Tordo</title></svelte:head>

<div class="ayuda">
  <h1>Ayuda</h1>
  <p class="intro">Qué hace cada parte de Tordo, los atajos y algunas pautas de uso.</p>
  <nav class="toc" aria-label="Contenido de la ayuda">
    {#each SECTIONS as s (s.id)}<a href="#{s.id}">{s.title}</a>{/each}
  </nav>

  {#each SECTIONS as s (s.id)}
    <section id={s.id}>
      <h2>{s.title}</h2>
      <dl>
        {#each s.items as item}
          <dt>
            {#if item.keys}
              {#each item.keys as k, i}{#if i > 0}<span class="or"> o </span>{/if}<kbd>{k}</kbd>{/each}
            {:else if item.href}
              <a href={item.href}>{item.term}</a>
            {:else}
              {item.term}
            {/if}
          </dt>
          <dd>
            {item.text}
            {#if item.blockedInListen && listenMode()}<span class="blocked">No disponible en modo escucha.</span>{/if}
          </dd>
        {/each}
      </dl>
    </section>
  {/each}
</div>

<style>
  .ayuda {
    max-width: 720px;
  }
  .intro {
    color: var(--muted);
    margin-top: 0;
  }
  .toc {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 16px;
    margin-bottom: 8px;
    font-size: 13px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  section {
    padding: 4px 0 20px;
    border-bottom: 1px solid var(--line);
  }
  dl {
    display: grid;
    grid-template-columns: minmax(130px, max-content) 1fr;
    gap: 10px 20px;
    margin: 0;
  }
  dt {
    color: var(--strong);
    font-weight: 600;
  }
  dd {
    margin: 0;
    color: var(--text);
  }
  kbd {
    font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
    font-size: 12px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 1px 6px;
    white-space: nowrap;
  }
  .or {
    color: var(--faint);
    font-weight: 400;
  }
  .blocked {
    display: block;
    color: var(--warn);
  }
  @media (max-width: 640px) {
    dl {
      grid-template-columns: 1fr;
      gap: 2px;
    }
    dd {
      margin-bottom: 10px;
    }
  }
</style>
