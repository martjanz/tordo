<script>
  import { api, lanQrURL } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { pinProblem, portProblem } from '../lib/settings.js'

  // d.lan is the draft's {enabled, port, pin} ('' keeps the saved PIN);
  // config is what is saved: the addresses below follow it, not the draft.
  let { d = $bindable(), config, disabled = false } = $props()

  let info = $state(null) // GET /api/lan: {enabled, port, listening, urls}
  let loadError = $state('')
  let changingPin = $state(false)
  let loads = 0

  // The addresses are read again after every save that changes the switch or
  // the port. Only the latest read applies.
  $effect(() => {
    config.lan.enabled
    config.lan.port
    const id = ++loads
    if (!config.lan.enabled) {
      info = null
      return
    }
    api.lan().then(
      (i) => {
        if (id !== loads) return
        info = i
        loadError = ''
      },
      (e) => {
        if (id === loads) loadError = e.message
      },
    )
  })

  const hasPin = $derived(config.lan.hasPin)
  const portError = $derived(d.lan.enabled ? portProblem(d.lan.port) : '')
  const pinError = $derived(pinProblem(d.lan.pin, hasPin, d.lan.enabled))
  const askPin = $derived(!hasPin || changingPin)

  function cancelPin() {
    changingPin = false
    d.lan.pin = ''
  }

  async function copy(url) {
    try {
      await navigator.clipboard.writeText(url)
      notify(t('lan.copied'))
    } catch {
      notify(t('lan.copyFailed'))
    }
  }
</script>

<label class="check">
  <input type="checkbox" bind:checked={d.lan.enabled} disabled={disabled} />
  <span>{t('lan.use')}</span>
</label>

{#if d.lan.enabled}
  <div class="fields">
    <label class="field">
      <span class="label">{t('lan.port')}</span>
      <input
        class="port"
        type="number"
        min="1024"
        max="65535"
        bind:value={d.lan.port}
        aria-invalid={!!portError}
        {disabled}
      />
    </label>
    {#if portError}<p class="bad" role="alert">{portError}</p>{/if}

    <div class="field">
      {#if askPin}
        <label>
          <span class="label">{hasPin ? t('lan.changePin') : t('lan.choosePin')}</span>
          <input
            class="pin"
            type="password"
            inputmode="numeric"
            autocomplete="new-password"
            pattern="[0-9]*"
            maxlength="12"
            bind:value={d.lan.pin}
            aria-invalid={!!pinError}
            placeholder={t('lan.pinPlaceholder')}
            {disabled}
          />
        </label>
        {#if hasPin}<button onclick={cancelPin} {disabled}>{t('common.cancel')}</button>{/if}
      {:else}
        <span class="label">{t('lan.pin')}</span>
        <span>{t('lan.pinSaved')}</span>
        <button onclick={() => (changingPin = true)} {disabled}>{t('lan.changePin')}</button>
      {/if}
    </div>
    {#if pinError && (d.lan.pin !== '' || !hasPin)}<p class="bad" role="alert">{pinError}</p>{/if}
    {#if hasPin}<p class="note">{t('lan.changePinNote')}</p>{/if}
    <p class="note">{t('lan.httpNote')}</p>
  </div>
{/if}

{#if config.lan.enabled}
  {#if loadError}
    <p class="bad" role="alert">{loadError}</p>
  {:else if info && !info.listening}
    <p class="bad">{t('lan.notListening')}</p>
  {:else if info}
    <ul class="urls">
      {#each info.urls as url (url)}
        <li>
          <img src={lanQrURL(url)} width="160" height="160" alt={t('lan.qrAlt', { url })} />
          <div>
            <span class="path">{url}</span>
            <button onclick={() => copy(url)}>{t('lan.copy')}</button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
{/if}

<style>
  .fields {
    margin: 12px 0 0 26px;
  }
  .field {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
    margin-top: 10px;
  }
  .field label {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
  }
  .label {
    min-width: 90px;
  }
  .port {
    width: 100px;
  }
  .pin {
    width: 180px;
  }
  .note {
    margin: 8px 0 0;
    color: var(--muted);
  }
  .bad {
    color: var(--warn);
    margin: 6px 0 0;
  }
  .urls {
    list-style: none;
    padding: 0;
    margin: 16px 0 0;
    display: flex;
    flex-wrap: wrap;
    gap: 20px;
  }
  .urls li {
    display: flex;
    flex-direction: column;
    gap: 8px;
    align-items: flex-start;
  }
  .urls img {
    background: #fff;
    padding: 8px;
    border-radius: var(--radius);
    box-sizing: content-box;
  }
  .urls div {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
  }
</style>
