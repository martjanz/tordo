<script>
  import { untrack } from 'svelte'
  import Dispositivos from '../components/Dispositivos.svelte'
  import Raices from '../components/Raices.svelte'
  import RedLocal from '../components/RedLocal.svelte'
  import TokenDiscogs from '../components/TokenDiscogs.svelte'
  import { api } from '../lib/api.js'
  import { notify, refreshStatus, settingsSaved } from '../lib/app.svelte.js'
  import { auth, checkAuth } from '../lib/auth.svelte.js'
  import { body, changed, draft, pinProblem, portProblem, savedNotice } from '../lib/settings.js'

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

<svelte:head><title>Ajustes · Tordo</title></svelte:head>

<div class="ajustes">
  <h1>Ajustes</h1>
  <p class="ayuda"><a href="/ayuda">Ayuda: atajos, secciones y conceptos</a></p>
  {#if forbidden}
    <p class="warn">Los ajustes solo se cambian en la computadora que corre Tordo.</p>
    {#if !auth.local}
      <button onclick={logout}>Cerrar sesión en este dispositivo</button>
      {#if error}<p class="warn" role="alert">{error}</p>{/if}
    {/if}
  {:else if loadError}
    <p class="warn" role="alert">{loadError}</p>
  {:else if d}
    {#if config.readOnly}
      <p class="warn">Modo consulta: la configuración no se puede cambiar.</p>
    {/if}

    <section>
      <h2>Carpetas</h2>
      <p class="help">
        Quitar una carpeta la saca del catálogo (las correcciones se conservan). Una carpeta que no está disponible,
        como un disco desenchufado, se mantiene tal cual. Tordo solo lee la música: nunca mueve, renombra ni borra archivos.
      </p>
      <Raices bind:d disabled={config.readOnly} />
      <label class="ignore">
        <span class="label">Carpetas a ignorar</span>
        <textarea
          bind:value={d.ignore}
          rows="3"
          disabled={config.readOnly}
          placeholder={'Un patrón por línea, por ejemplo:\nSamples\n*.tmp'}
        ></textarea>
      </label>
    </section>

    <section>
      <h2>Discogs</h2>
      {#key version}
        <TokenDiscogs bind:token={d.token} {config} disabled={config.readOnly} />
      {/key}
    </section>

    <section>
      <h2>Reproducción</h2>
      <label class="check">
        <input type="checkbox" bind:checked={d.normalize} disabled={config.readOnly} />
        <span>
          Normalizar el volumen
          <small>Empareja el volumen entre temas, solo bajando los más fuertes.</small>
        </span>
      </label>
    </section>

    <section>
      <h2>Modo escucha</h2>
      <label class="check">
        <input
          type="checkbox"
          checked={!!config.listenMode}
          disabled={config.readOnly || toggling}
          onchange={(e) => setListenMode(e.currentTarget.checked)}
        />
        <span>
          Modo escucha
          <small>
            Oculta Revisar y la curaduría, y bloquea identificaciones, escaneos, colecciones y cambios de géneros. Los ♥,
            las ★, las listas y las estaciones siguen funcionando.
          </small>
        </span>
      </label>
    </section>

    <section id="red">
      <h2>Red local</h2>
      {#key version}
        <RedLocal bind:d {config} disabled={config.readOnly} />
        {#if config.lan.enabled}<Dispositivos disabled={config.readOnly} />{/if}
      {/key}
    </section>

    {#if !config.readOnly}
      {#if error}<p class="warn" role="alert">{error}</p>{/if}
      <div class="actions">
        <button class="primary" onclick={save} disabled={!dirty || saving || lanInvalid || !d.roots.some((r) => r.checked)}>
          {saving ? 'Guardando…' : 'Guardar'}
        </button>
        {#if dirty}<button onclick={() => ((d = draft(config)), version++)} disabled={saving}>Descartar</button>{/if}
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
  .ignore {
    display: block;
    margin-top: 16px;
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
