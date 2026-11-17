<script>
  import { api } from '../lib/api.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { addRoot, checkAll } from '../lib/settings.js'

  // d is the settings draft; the folders are d.roots.
  let { d = $bindable(), disabled = false } = $props()
  let path = $state('')
  let error = $state('')
  let checking = $state(false)

  // Folders read as they sit next to the app: "../musica" is "musica".
  function label(p) {
    return p.replace(/^(\.\.\/)+/, '')
  }

  async function add(event) {
    event.preventDefault()
    if (!path.trim()) return
    checking = true
    error = ''
    try {
      d = addRoot(d, await api.checkRoot(path))
      path = ''
    } catch (e) {
      error = e.message
    }
    checking = false
  }
</script>

{#if d.roots.length > 1}
  <div class="all">
    <button type="button" class="link" onclick={() => (d = checkAll(d, true))} {disabled}>{t('roots.checkAll')}</button>
    <button type="button" class="link" onclick={() => (d = checkAll(d, false))} {disabled}>{t('roots.uncheckAll')}</button>
  </div>
{/if}
<ul>
  {#each d.roots as root (root.path)}
    <li>
      <label>
        <input type="checkbox" bind:checked={root.checked} {disabled} />
        <span class="text">
          <span class="name">{label(root.path)}{#if !root.available}<small>{t('roots.unavailable')}</small>{/if}</span>
          {#if root.path !== label(root.path)}<span class="path">{root.path}</span>{/if}
        </span>
      </label>
    </li>
  {:else}
    <li class="none">{t('roots.none')}</li>
  {/each}
</ul>
<form onsubmit={add}>
  <input type="text" bind:value={path} placeholder={t('roots.otherPlaceholder')} {disabled} aria-label={t('roots.otherLabel')} />
  <button type="submit" disabled={disabled || checking || !path.trim()}>{t('roots.add')}</button>
</form>
{#if error}<p class="error">{error}</p>{/if}

<style>
  .all {
    display: flex;
    gap: 16px;
    margin-bottom: 4px;
  }
  .link {
    background: none;
    border: 0;
    padding: 0;
    color: var(--muted);
    cursor: pointer;
    font-size: var(--fs-sm);
  }
  .link:hover:not(:disabled) {
    color: var(--strong);
    text-decoration: underline;
  }
  ul {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
  }
  li {
    padding: 6px 0;
    border-bottom: 1px solid var(--line);
  }
  label {
    display: flex;
    align-items: baseline;
    gap: 8px;
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .name {
    color: var(--strong);
  }
  /* The path as written in config.json, right under the name it reads as. */
  .path {
    font-size: var(--fs-sm);
    color: var(--faint);
    overflow-wrap: anywhere;
  }
  small {
    margin-left: 8px;
    font-size: var(--fs-sm);
    color: var(--warn);
  }
  .none {
    color: var(--muted);
  }
  form {
    display: flex;
    gap: 8px;
  }
  form input {
    flex: 1;
    min-width: 0;
  }
  .error {
    color: var(--warn);
    margin: 6px 0 0;
  }
</style>
