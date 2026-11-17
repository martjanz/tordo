<script>
  import { untrack } from 'svelte'
  import { api } from '../lib/api.js'
  import { t } from '../lib/i18n/index.svelte.js'

  // token is the draft's: null keeps the saved one, "" removes it, text
  // replaces it. config tells whether one is saved.
  let { token = $bindable(null), config, disabled = false } = $props()
  let mode = $state(untrack(() => config.hasToken) ? 'keep' : 'edit') // keep | edit | remove
  let text = $state('')
  let check = $state(null) // null, 'checking', true, false, 'unknown'
  let checks = 0

  $effect(() => {
    if (mode === 'edit') token = text.trim() ? text : null
    else token = mode === 'remove' ? '' : null
  })

  // Only the latest verify() fills check: an earlier, slower one would
  // label text the person has since changed.
  async function verify() {
    const id = ++checks
    check = 'checking'
    try {
      const r = await api.checkToken(text)
      if (id === checks) check = r.valid === null ? 'unknown' : r.valid
    } catch {
      if (id === checks) check = 'unknown'
    }
  }
</script>

<p class="help">
  {#each t('discogs.intro').split(/(\{discogs\}|\{url\})/) as part, i (i)}
    {#if part === '{discogs}'}
      <a href="https://www.discogs.com/" target="_blank" rel="noreferrer">Discogs</a>
    {:else if part === '{url}'}
      <a href="https://www.discogs.com/settings/developers" target="_blank" rel="noreferrer">discogs.com/settings/developers</a>
    {:else}
      {part}
    {/if}
  {/each}
</p>

{#if mode === 'keep'}
  <p class="saved">
    {t('discogs.loaded')} <span class="path">{config.tokenHint}</span>
    <button onclick={() => (mode = 'edit')} {disabled}>{t('discogs.change')}</button>
    <button onclick={() => (mode = 'remove')} {disabled}>{t('discogs.remove')}</button>
  </p>
{:else if mode === 'remove'}
  <p class="saved">
    {t('discogs.removing')}
    <button onclick={() => (mode = 'keep')} {disabled}>{t('discogs.undo')}</button>
  </p>
{:else}
  <div class="row">
    <input
      type="text"
      bind:value={text}
      oninput={() => ((checks += 1), (check = null))}
      placeholder={t('discogs.placeholder')}
      aria-label={t('discogs.tokenLabel')}
      autocomplete="off"
      spellcheck="false"
      {disabled}
    />
    <button onclick={verify} disabled={disabled || !text.trim() || check === 'checking'}>{t('discogs.verify')}</button>
    {#if config.hasToken}<button onclick={() => ((mode = 'keep'), (text = ''))} {disabled}>{t('common.cancel')}</button>{/if}
  </div>
  {#if check === 'checking'}
    <p class="note">{t('discogs.checking')}</p>
  {:else if check === true}
    <p class="note ok">{t('discogs.accepted')}</p>
  {:else if check === false}
    <p class="note bad">{t('discogs.rejected')}</p>
  {:else if check === 'unknown'}
    <p class="note">{t('discogs.unknown')}</p>
  {/if}
{/if}

<style>
  .help {
    color: var(--muted);
    margin-top: 0;
  }
  .help a {
    color: var(--accent);
  }
  .saved {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
  }
  .row {
    display: flex;
    gap: 8px;
  }
  .row input {
    flex: 1;
    min-width: 0;
    font-family: ui-monospace, 'Cascadia Mono', Consolas, monospace;
    font-size: var(--fs-sm);
  }
  .note {
    margin: 6px 0 0;
    color: var(--muted);
  }
  .ok {
    color: #6fbf73;
  }
  .bad {
    color: var(--warn);
  }
</style>
