<script>
  // Revisar: what needs the user's judgment.
  import { t } from '../lib/i18n/index.svelte.js'
  import Duplicados from '../components/Duplicados.svelte'
  import FormatosAConvertir from '../components/FormatosAConvertir.svelte'
  import GenerosPorMapear from '../components/GenerosPorMapear.svelte'
  import SinIdentificar from '../components/SinIdentificar.svelte'

  let { tab, kind = 'exact' } = $props()

  let counts = $state({})
</script>

<h1>{t('nav.review')}</h1>
<nav class="tabs">
  <a href="/review" class:on={tab === 'sin-identificar'}>{t('review.tab.unidentified')}{counts.unidentified != null ? ` (${counts.unidentified})` : ''}</a>
  <a href="/review/genres" class:on={tab === 'generos'}>{t('review.tab.genres')}{counts.genres != null ? ` (${counts.genres})` : ''}</a>
  <a href="/review/duplicates" class:on={tab === 'duplicados'}>{t('review.tab.duplicates')}{counts.duplicates != null ? ` (${counts.duplicates})` : ''}</a>
  <a href="/review/formats" class:on={tab === 'formatos'}>{t('review.tab.formats')}{counts.convert != null ? ` (${counts.convert})` : ''}</a>
</nav>

{#if tab === 'duplicados'}
  <Duplicados {kind} onload={(n) => (counts.duplicates = n)} />
{:else if tab === 'formatos'}
  <FormatosAConvertir onload={(n) => (counts.convert = n)} />
{:else if tab === 'generos'}
  <GenerosPorMapear onload={(n) => (counts.genres = n)} />
{:else}
  <SinIdentificar onload={(n) => (counts.unidentified = n)} />
{/if}

<style>
  .tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 20px;
    margin: 12px 0 16px;
    border-bottom: 1px solid var(--line);
  }
  .tabs a {
    padding: 6px 0;
    border-bottom: 2px solid transparent;
    color: var(--muted);
  }
  .tabs a.on {
    color: var(--strong);
    border-bottom-color: var(--accent);
  }
</style>
