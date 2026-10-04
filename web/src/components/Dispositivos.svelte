<script>
  import { api } from '../lib/api.js'
  import { notify } from '../lib/app.svelte.js'
  import { since } from '../lib/format.js'

  // The devices that entered with the PIN (the list is read when it opens
  // and after closing one).
  let { disabled = false } = $props()

  let sessions = $state(null)
  let error = $state('')
  let loads = 0

  async function load() {
    const id = ++loads
    try {
      const list = await api.sessions()
      if (id !== loads) return
      sessions = list
      error = ''
    } catch (e) {
      if (id === loads) error = e.message
    }
  }

  $effect(() => {
    load()
  })

  async function close(s) {
    if (!confirm(`¿Cerrar la sesión de ${s.device}?`)) return
    try {
      await api.closeSession(s.id)
    } catch (e) {
      notify(e.message)
    }
    await load()
  }

  async function setHost(s, host) {
    try {
      await api.setSessionHost(s.id, host)
    } catch (e) {
      notify(e.message)
    }
    await load()
  }

  const date = (ms) => new Date(ms).toLocaleString('es', { dateStyle: 'medium', timeStyle: 'short' })
</script>

<h3>Dispositivos</h3>
<p class="note help">Los anfitriones controlan la rockola por completo; el resto solo agrega temas, pausa y quita lo que agregó.</p>
{#if error}
  <p class="bad" role="alert">{error}</p>
{:else if sessions === null}
  <p class="note">Cargando…</p>
{:else if sessions.length === 0}
  <p class="note">Ningún dispositivo entró todavía.</p>
{:else}
  <ul>
    {#each sessions as s (s.id)}
      <li>
        <div class="who">
          <strong>{s.device}</strong>
          <small>conectado desde {date(s.createdAt)} · último uso {since(s.lastSeen)}</small>
        </div>
        <label class="host">
          <input type="checkbox" checked={s.host} {disabled} onchange={(e) => setHost(s, e.currentTarget.checked)} />
          Anfitrión de la rockola
        </label>
        <button onclick={() => close(s)} {disabled}>Cerrar sesión</button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  h3 {
    margin: 24px 0 8px;
    font-size: 15px;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 8px 0;
    border-bottom: 1px solid var(--line);
  }
  .who {
    min-width: 0;
  }
  small {
    display: block;
    color: var(--faint);
  }
  .note {
    color: var(--muted);
    margin: 0;
  }
  .help {
    margin-bottom: 8px;
    font-size: 13px;
  }
  .host {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: auto;
    font-size: 13px;
    color: var(--muted);
    white-space: nowrap;
  }
  .bad {
    color: var(--warn);
  }
</style>
