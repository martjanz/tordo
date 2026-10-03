<script>
  import { onDestroy, tick } from 'svelte'
  import { api } from '../lib/api.js'
  import { deviceName } from '../lib/device.js'

  let pin = $state('')
  let busy = $state(false)
  let error = $state('')
  let wait = $state(0) // seconds until another try is allowed
  let timer
  let field = $state()

  const locked = $derived(wait > 0)

  function countdown(seconds) {
    clearInterval(timer)
    wait = seconds
    timer = setInterval(() => {
      wait -= 1
      if (wait <= 0) clearInterval(timer)
    }, 1000)
  }

  onDestroy(() => clearInterval(timer))

  async function submit(event) {
    event.preventDefault()
    if (busy || locked || pin === '') return
    busy = true
    error = ''
    try {
      await api.login(pin, deviceName(navigator.userAgent))
      // The session cookie is set: the app starts from scratch.
      location.reload()
      return
    } catch (e) {
      if (e.status === 401) error = 'PIN incorrecto.'
      else if (e.status === 429) countdown(e.retryAfter || 30)
      else if (e.status === 403) error = 'El acceso desde la red está apagado.'
      else error = e.message
      pin = ''
    }
    busy = false
    await tick()
    field?.focus() // typing the next PIN needs no extra tap
  }
</script>

<svelte:head><title>Entrar · Tordo</title></svelte:head>

<form class="entrar" onsubmit={submit}>
  <h1>Ingresá el PIN de Tordo</h1>
  <input
    type="password"
    inputmode="numeric"
    autocomplete="one-time-code"
    pattern="[0-9]*"
    bind:value={pin}
    bind:this={field}
    aria-label="PIN"
    maxlength="12"
    disabled={busy}
  />
  <button class="primary" type="submit" disabled={busy || locked || pin === ''}>
    {busy ? 'Entrando…' : 'Entrar'}
  </button>
  {#if locked}
    <p class="warn" role="alert">Demasiados intentos. Probá de nuevo en {wait} s.</p>
  {:else if error}
    <p class="warn" role="alert">{error}</p>
  {/if}
</form>

<style>
  .entrar {
    max-width: 360px;
    margin: 24px auto 0;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  h1 {
    font-size: 20px;
    margin: 0;
  }
  input {
    font-size: 24px;
    letter-spacing: 0.3em;
    text-align: center;
    padding: 14px 12px;
    min-height: 56px;
  }
  button {
    min-height: 52px;
    font-size: 17px;
  }
  .warn {
    color: var(--warn);
    margin: 0;
  }
</style>
