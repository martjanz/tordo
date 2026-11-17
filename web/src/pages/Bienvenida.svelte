<script>
  import Idioma from '../components/Idioma.svelte'
  import Raices from '../components/Raices.svelte'
  import TokenDiscogs from '../components/TokenDiscogs.svelte'
  import { api } from '../lib/api.js'
  import { notify, settingsSaved } from '../lib/app.svelte.js'
  import { previewLanguage, t } from '../lib/i18n/index.svelte.js'
  import { navigate } from '../lib/nav.svelte.js'
  import { draft, firstBody } from '../lib/settings.js'

  const steps = $derived([t('settings.folders.title'), 'Discogs', t('setup.stepDone')])

  let config = $state(null)
  let d = $state(null)
  let step = $state(0)
  let saving = $state(false)
  let forbidden = $state(false) // another device: only the computer running Tordo sets it up
  let loadError = $state('')

  $effect(() => {
    api.config().then(
      (c) => {
        if (!c.setupPending) return navigate('/', { replace: true })
        config = c
        d = { ...draft(c), normalize: true } // on by default
      },
      (e) => {
        if (e.status === 403) forbidden = true
        else loadError = e.message
      },
    )
  })

  const rootsChosen = $derived(!!d && d.roots.some((r) => r.checked))

  async function start() {
    saving = true
    try {
      await api.saveConfig(firstBody(d))
      settingsSaved()
      navigate('/', { replace: true })
    } catch (e) {
      notify(e.message)
      saving = false
    }
  }
</script>

<svelte:head><title>{t('setup.title')} · Tordo</title></svelte:head>

{#if forbidden}
  <p class="bienvenida warn">{t('error.settings_local_only')}</p>
{:else if loadError}
  <p class="bienvenida warn" role="alert">{loadError}</p>
{:else if d}
  <div class="bienvenida">
    <div class="idioma"><Idioma value={d.language} onchange={(l) => ((d.language = l), previewLanguage(l))} /></div>

    <ol class="steps">
      {#each steps as name, i (i)}
        <li class:on={i === step} class:done={i < step}>{i + 1}. {name}</li>
      {/each}
    </ol>

    {#if step === 0}
      <h2>{t('setup.where')}</h2>
      <p class="help">{t('setup.whereHelp')}</p>
      <Raices bind:d />
      <details>
        <summary>{t('settings.folders.ignore')}</summary>
        <textarea
          bind:value={d.ignore}
          rows="3"
          aria-label={t('settings.folders.ignore')}
          placeholder={t('settings.folders.ignorePlaceholder')}
        ></textarea>
      </details>
    {:else if step === 1}
      <h2>{t('discogs.tokenLabel')}</h2>
      <TokenDiscogs bind:token={d.token} {config} />
    {:else}
      <h2>{t('setup.ready')}</h2>
      <label class="check">
        <input type="checkbox" bind:checked={d.normalize} />
        <span>
          {t('settings.normalize')}
          <small>{t('settings.normalizeHelp')}</small>
        </span>
      </label>
      <p class="help">{t('setup.later')}</p>
    {/if}

    <div class="actions">
      {#if step > 0}<button onclick={() => step--} disabled={saving}>{t('setup.back')}</button>{/if}
      <span class="spacer"></span>
      {#if step === 1 && d.token === null}
        <button onclick={() => step++}>{t('setup.skipToken')}</button>
      {:else if step < steps.length - 1}
        <button class="primary" onclick={() => step++} disabled={!rootsChosen}>{t('setup.next')}</button>
      {:else}
        <button class="primary" onclick={start} disabled={saving || !rootsChosen}>
          {saving ? t('common.saving') : t('setup.start')}
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
  .warn {
    color: var(--warn);
  }
  .idioma {
    margin-bottom: 16px;
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
