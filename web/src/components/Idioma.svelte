<script>
  import { LOCALES, LOCALE_NAMES, t } from '../lib/i18n/index.svelte.js'

  // value is '' for automatic, or a locale; onchange gets the new one.
  let { value, onchange, disabled = false } = $props()
</script>

<select
  {value}
  {disabled}
  aria-label={t('settings.language.title')}
  onchange={(e) => onchange(e.currentTarget.value)}
>
  <option value="">{t('settings.language.auto')}</option>
  {#each LOCALES as l (l)}
    <option value={l}>{LOCALE_NAMES[l]}</option>
  {/each}
</select>
