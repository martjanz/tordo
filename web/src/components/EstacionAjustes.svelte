<script>
  // A small dialog to adjust the running station's name and rules, apply
  // them to the queue and save it. Closes on Escape and outside click.
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { setStation } from '../lib/player.svelte.js'

  let { station, onclose } = $props()

  const GAPS = [
    [0, 'No importa'],
    [2, '2 pistas'],
    [3, '3 pistas'],
    [5, '5 pistas'],
    [10, '10 pistas'],
  ]
  const DAYS = [
    [0, 'No evitar'],
    [1, '1 día'],
    [7, '7 días'],
    [30, '30 días'],
    [90, '90 días'],
  ]

  let name = $state(station.name)
  let mode = $state(station.mode)
  let artistGap = $state(station.artistGap)
  let avoidDays = $state(station.avoidDays)
  let busy = $state(false)
  let root = $state()

  // Rules outside the menus (saved by hand) stay selectable.
  const gaps = $derived(GAPS.some(([v]) => v === artistGap) ? GAPS : [...GAPS, [artistGap, `${artistGap} pistas`]])
  const days = $derived(DAYS.some(([v]) => v === avoidDays) ? DAYS : [...DAYS, [avoidDays, `${avoidDays} días`]])

  const edited = () => ({ ...station, name: name.trim(), mode, artistGap: Number(artistGap), avoidDays: Number(avoidDays) })

  function apply() {
    setStation(edited())
    onclose()
  }

  async function save() {
    const st = edited()
    if (!st.name || busy) return
    busy = true
    try {
      if (st.id) {
        await api.updateStation(st.id, st)
        setStation(st)
      } else {
        const saved = await api.createStation(st)
        setStation({ ...st, id: saved.id })
      }
      notify(`Estación «${st.name}» guardada.`)
      onclose()
    } catch (e) {
      notify(e.message)
      busy = false
    }
  }

  function onwindowclick(event) {
    if (root && !root.contains(event.target)) onclose()
  }

  function onkeydown(event) {
    if (event.key === 'Escape') onclose()
  }
</script>

<svelte:window onclick={onwindowclick} {onkeydown} />

<div class="dialog" bind:this={root} role="dialog" aria-label="Ajustar la estación">
  <label>Nombre <input bind:value={name} maxlength="200" /></label>
  <label>
    Modo
    <select bind:value={mode}>
      <option value="pistas">Pistas</option>
      <option value="albumes">Álbumes</option>
    </select>
  </label>
  <label>
    No repetir artista en
    <select bind:value={artistGap}>
      {#each gaps as [v, t] (v)}<option value={v}>{t}</option>{/each}
    </select>
  </label>
  <label>
    Evitar lo escuchado en los últimos
    <select bind:value={avoidDays}>
      {#each days as [v, t] (v)}<option value={v}>{t}</option>{/each}
    </select>
  </label>
  <div class="buttons">
    <button class="primary" onclick={apply}>Aplicar</button>
    {#if !app.status?.readOnly}
      <button disabled={busy || !name.trim()} onclick={save}>Guardar</button>
    {/if}
    <button onclick={onclose}>Cancelar</button>
  </div>
</div>

<style>
  .dialog {
    margin: 4px 8px 8px;
    padding: 12px;
    background: var(--surface-2);
    border: 1px solid var(--line-strong);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 13px;
    color: var(--muted);
  }
  .buttons {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
</style>
