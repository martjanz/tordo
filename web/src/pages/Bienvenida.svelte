<script>
  import Raices from '../components/Raices.svelte'
  import TokenDiscogs from '../components/TokenDiscogs.svelte'
  import { api } from '../lib/api.js'
  import { notify, settingsSaved } from '../lib/app.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { body, draft } from '../lib/settings.js'

  const steps = ['Carpetas', 'Discogs', 'Listo']

  let config = $state(null)
  let d = $state(null)
  let step = $state(0)
  let saving = $state(false)

  $effect(() => {
    api.config().then(
      (c) => {
        if (!c.setupPending) return navigate('/', { replace: true })
        config = c
        d = { ...draft(c), normalize: true } // on by default
      },
      (e) => notify(e.message),
    )
  })

  const rootsChosen = $derived(!!d && d.roots.some((r) => r.checked))

  async function start() {
    saving = true
    try {
      await api.saveConfig(body(d))
      settingsSaved()
      navigate('/', { replace: true })
    } catch (e) {
      notify(e.message)
      saving = false
    }
  }
</script>

<svelte:head><title>Bienvenida · Tordo</title></svelte:head>

{#if d}
  <div class="bienvenida">
    <ol class="steps">
      {#each steps as name, i (name)}
        <li class:on={i === step} class:done={i < step}>{i + 1}. {name}</li>
      {/each}
    </ol>

    {#if step === 0}
      <h2>¿Dónde está tu música?</h2>
      <p class="help">
        Estas son las carpetas que están al lado de la de Tordo. Tordo solo las lee: nunca mueve, renombra ni borra
        archivos.
      </p>
      <Raices bind:d />
      <details>
        <summary>Carpetas a ignorar</summary>
        <textarea
          bind:value={d.ignore}
          rows="3"
          aria-label="Carpetas a ignorar"
          placeholder={'Un patrón por línea, por ejemplo:\nSamples\n*.tmp'}
        ></textarea>
      </details>
    {:else if step === 1}
      <h2>Token de Discogs</h2>
      <TokenDiscogs bind:token={d.token} {config} />
    {:else}
      <h2>Listo para empezar</h2>
      <label class="check">
        <input type="checkbox" bind:checked={d.normalize} />
        <span>
          Normalizar el volumen
          <small>Empareja el volumen entre temas, solo bajando los más fuertes.</small>
        </span>
      </label>
      <p class="help">El acceso desde el teléfono se activa después, en Ajustes.</p>
    {/if}

    <div class="actions">
      {#if step > 0}<button onclick={() => step--} disabled={saving}>Atrás</button>{/if}
      <span class="spacer"></span>
      {#if step === 1 && d.token === null}
        <button onclick={() => step++}>Seguir sin token</button>
      {:else if step < steps.length - 1}
        <button class="primary" onclick={() => step++} disabled={!rootsChosen}>Siguiente</button>
      {:else}
        <button class="primary" onclick={start} disabled={saving || !rootsChosen}>
          {saving ? 'Guardando…' : 'Empezar'}
        </button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .bienvenida {
    max-width: 640px;
    margin: 32px auto 0;
  }
  .steps {
    display: flex;
    gap: 20px;
    list-style: none;
    padding: 0 0 12px;
    margin: 0 0 24px;
    border-bottom: 1px solid var(--line);
    font-size: 12px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--faint);
  }
  .steps .on {
    color: var(--strong);
  }
  .steps .done {
    color: var(--muted);
  }
  h2 {
    font-size: 18px;
    letter-spacing: 0;
    text-transform: none;
    color: var(--strong);
    margin: 0 0 8px;
  }
  .help {
    color: var(--muted);
  }
  details {
    margin-top: 16px;
  }
  summary {
    cursor: pointer;
    color: var(--muted);
  }
  textarea {
    display: block;
    width: 100%;
    margin-top: 8px;
    resize: vertical;
    font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
    font-size: 13px;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: var(--radius);
    padding: 6px 8px;
  }
  .check {
    display: flex;
    gap: 10px;
    align-items: baseline;
  }
  small {
    display: block;
    color: var(--faint);
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-top: 28px;
  }
  .spacer {
    flex: 1;
  }
  @media (max-width: 640px) {
    .steps {
      gap: 12px;
      flex-wrap: wrap;
    }
  }
</style>
