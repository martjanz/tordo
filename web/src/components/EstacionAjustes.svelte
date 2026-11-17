<script>
  // A small dialog to adjust the running station's name and rules, apply
  // them to the queue and save it. Closes on Escape and outside click.
  import { api } from '../lib/api.js'
  import { app, notify } from '../lib/app.svelte.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { setStation } from '../lib/playback.svelte.js'

  let { station, onclose } = $props()

  const GAPS = [0, 2, 3, 5, 10]
  const DAYS = [0, 1, 7, 30, 90]
  const gapLabel = (n) => (n === 0 ? t('station.edit.anyGap') : t('count.tracks', { n }))
  const dayLabel = (n) => (n === 0 ? t('station.edit.noAvoid') : t('count.days', { n }))

  // The form edits a copy: it deliberately captures the station's initial values.
  /* svelte-ignore state_referenced_locally */
  let name = $state(station.name)
  /* svelte-ignore state_referenced_locally */
  let mode = $state(station.mode)
  /* svelte-ignore state_referenced_locally */
  let artistGap = $state(station.artistGap)
  /* svelte-ignore state_referenced_locally */
  let avoidDays = $state(station.avoidDays)
  let busy = $state(false)
  let root = $state()

  // Rules outside the menus (saved by hand) stay selectable.
  const gaps = $derived((GAPS.includes(artistGap) ? GAPS : [...GAPS, artistGap]).map((v) => [v, gapLabel(v)]))
  const days = $derived((DAYS.includes(avoidDays) ? DAYS : [...DAYS, avoidDays]).map((v) => [v, dayLabel(v)]))

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
      notify(t('station.edit.saved', { name: st.name }))
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

<div class="dialog" bind:this={root} role="dialog" aria-label={t('station.edit.dialog')}>
  <label>{t('station.edit.name')} <input bind:value={name} maxlength="200" /></label>
  <label>
    {t('station.edit.mode')}
    <select bind:value={mode}>
      <option value="tracks">{t('facets.view.tracks')}</option>
      <option value="albums">{t('facets.view.albums')}</option>
    </select>
  </label>
  <label>
    {t('station.edit.gap')}
    <select bind:value={artistGap}>
      {#each gaps as [v, text] (v)}<option value={v}>{text}</option>{/each}
    </select>
  </label>
  <label>
    {t('station.edit.avoid')}
    <select bind:value={avoidDays}>
      {#each days as [v, text] (v)}<option value={v}>{text}</option>{/each}
    </select>
  </label>
  <div class="buttons">
    <button class="primary" onclick={apply}>{t('station.edit.apply')}</button>
    {#if !app.status?.readOnly}
      <button disabled={busy || !name.trim()} onclick={save}>{t('station.edit.save')}</button>
    {/if}
    <button onclick={onclose}>{t('station.edit.cancel')}</button>
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
