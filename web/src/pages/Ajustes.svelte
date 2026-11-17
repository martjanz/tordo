<script>
  import { untrack } from 'svelte'
  import AudioDeEstaCompu from '../components/AudioDeEstaCompu.svelte'
  import Dispositivos from '../components/Dispositivos.svelte'
  import Idioma from '../components/Idioma.svelte'
  import Raices from '../components/Raices.svelte'
  import RedLocal from '../components/RedLocal.svelte'
  import TokenDiscogs from '../components/TokenDiscogs.svelte'
  import { api } from '../lib/api.js'
  import { notify, refreshStatus, settingsSaved } from '../lib/app.svelte.js'
  import { auth, checkAuth } from '../lib/auth.svelte.js'
  import { applyLanguage, t } from '../lib/i18n/index.svelte.js'
  import { body, changed, draft, pinProblem, portProblem, rescanChoices, savedNotice } from '../lib/settings.js'

  let config = $state(null)
  let d = $state(null)
  let saving = $state(false)
  let forbidden = $state(false) // another device: the settings are not its to change
  let error = $state('')
  let loadError = $state('')
  let version = $state(0) // remounts the token field after saving

  $effect(() => {
    api.config().then(
      (c) => {
        config = c
        d = draft(c)
      },
      (e) => {
        if (e.status === 403) forbidden = true
        else loadError = e.message
      },
    )
  })

  // Editing the draft clears the error of the last save.
  $effect(() => {
    JSON.stringify(d)
    untrack(() => (error = ''))
  })

  const dirty = $derived(!!config && !!d && changed(config, d))
  const lanInvalid = $derived(
    !!d &&
      !!config &&
      (portProblem(d.lan.port) !== '' || (d.lan.enabled && pinProblem(d.lan.pin, config.lan.hasPin, true) !== '')),
  )

  // Another device ends its own session and gets the PIN screen.
  async function logout() {
    try {
      await api.logout()
    } catch (e) {
      error = e.message
      return
    }
    auth.pinRequired = true
    checkAuth()
  }

  // Listen mode saves at once and on its own: the body omits the other
  // fields, so the pending edits of the form are neither saved nor lost, and
  // saving them later keeps the mode (the server keeps what the body omits).
  let toggling = $state(false)
  async function setListenMode(on) {
    toggling = true
    error = ''
    try {
      const saved = await api.saveConfig({ listenMode: on })
      config = { ...config, listenMode: saved.listenMode }
      refreshStatus()
    } catch (e) {
      error = e.message
    }
    toggling = false
  }

  // The language saves at once, like listen mode; the page changes language
  // when the server has it. A failure keeps the previous selection.
  let langKey = $state(0) // remounts the select to put the old value back
  async function setLanguage(language) {
    error = ''
    try {
      const saved = await api.saveConfig({ language })
      config = { ...config, language: saved.language }
      d = { ...d, language: saved.language }
      applyLanguage(saved.language)
      refreshStatus()
    } catch (e) {
      notify(e.message)
      langKey++
    }
  }

  async function save() {
    saving = true
    error = ''
    const notice = savedNotice(config, d)
    try {
      config = await api.saveConfig(body(d))
      d = draft(config)
      version++
      settingsSaved()
      notify(notice)
    } catch (e) {
      error = e.message
    }
    saving = false
  }
</script>

<svelte:head><title>{t('nav.settings')} · Tordo</title></svelte:head>

<div class="ajustes">
  <h1>{t('nav.settings')}</h1>
  <p class="ayuda"><a href="/help">{t('settings.helpLink')}</a></p>
  {#if forbidden}
    <p class="warn">{t('error.settings_local_only')}</p>
    {#if !auth.local}
      <button onclick={logout}>{t('settings.logout')}</button>
      {#if error}<p class="warn" role="alert">{error}</p>{/if}
    {/if}
  {:else if loadError}
    <p class="warn" role="alert">{loadError}</p>
  {:else if d}
    {#if config.readOnly}
      <p class="warn">{t('settings.readOnly')}</p>
    {/if}

    <section>
      <h2>{t('settings.language.title')}</h2>
      {#key langKey}
        <Idioma value={config.language ?? ''} onchange={setLanguage} disabled={config.readOnly} />
      {/key}
    </section>

    <section>
      <h2>{t('settings.folders.title')}</h2>
      <p class="help">{t('settings.folders.help')}</p>
      <Raices bind:d disabled={config.readOnly} />
      <label class="ignore">
        <span class="label">{t('settings.folders.ignore')}</span>
        <textarea
          bind:value={d.ignore}
          rows="3"
          disabled={config.readOnly}
          placeholder={t('settings.folders.ignorePlaceholder')}
        ></textarea>
      </label>
      <label class="rescan">
        <span class="label">{t('settings.folders.rescan')}</span>
        <select bind:value={d.rescanMinutes} disabled={config.readOnly}>
          {#each rescanChoices(config.rescanMinutes) as choice (choice.minutes)}
            <option value={choice.minutes}>{choice.label}</option>
          {/each}
        </select>
        <small>{t('settings.folders.rescanHelp')}</small>
      </label>
    </section>

    <section>
      <h2>Discogs</h2>
      {#key version}
        <TokenDiscogs bind:token={d.token} {config} disabled={config.readOnly} />
      {/key}
    </section>

    <section>
      <h2>{t('settings.playback')}</h2>
      <label class="check">
        <input type="checkbox" bind:checked={d.normalize} disabled={config.readOnly} />
        <span>
          {t('settings.normalize')}
          <small>{t('settings.normalizeHelp')}</small>
        </span>
      </label>
      {#if auth.local}<AudioDeEstaCompu />{/if}
    </section>

    <section>
      <h2>{t('nav.listenMode')}</h2>
      <label class="check">
        <input
          type="checkbox"
          checked={!!config.listenMode}
          disabled={config.readOnly || toggling}
          onchange={(e) => setListenMode(e.currentTarget.checked)}
        />
        <span>
          {t('nav.listenMode')}
          <small>{t('settings.listenHelp')}</small>
        </span>
      </label>
    </section>

    <section id="network">
      <h2>{t('settings.network')}</h2>
      {#key version}
        <RedLocal bind:d {config} disabled={config.readOnly} />
        {#if config.lan.enabled}<Dispositivos disabled={config.readOnly} />{/if}
      {/key}
    </section>

    {#if !config.readOnly}
      {#if error}<p class="warn" role="alert">{error}</p>{/if}
      <div class="actions">
        <button class="primary" onclick={save} disabled={!dirty || saving || lanInvalid || !d.roots.some((r) => r.checked)}>
          {saving ? t('common.saving') : t('common.save')}
        </button>
        {#if dirty}<button onclick={() => ((d = draft(config)), version++)} disabled={saving}>{t('settings.discard')}</button>{/if}
      </div>
    {/if}
  {/if}
</div>

<style>
  .ajustes {
    max-width: 720px;
  }
  section {
    padding: 4px 0 20px;
    border-bottom: 1px solid var(--line);
  }
  .help {
    color: var(--muted);
    margin-top: 0;
  }
  .ayuda {
    margin-top: 0;
    color: var(--muted);
  }
  .warn {
    color: var(--warn);
  }
  .ignore,
  .rescan {
    display: block;
    margin-top: 16px;
  }
  .rescan select {
    display: block;
    margin-top: 6px;
  }
  .rescan small {
    display: block;
    margin-top: 6px;
    color: var(--muted);
  }
  .ignore textarea {
    display: block;
    width: 100%;
    margin-top: 6px;
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
    margin-top: 24px;
  }
</style>
