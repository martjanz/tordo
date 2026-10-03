<script>
  // Revisar: what needs the user's judgment.
  import Duplicados from '../components/Duplicados.svelte'
  import FormatosAConvertir from '../components/FormatosAConvertir.svelte'
  import GenerosPorMapear from '../components/GenerosPorMapear.svelte'
  import SinIdentificar from '../components/SinIdentificar.svelte'

  let { tab, kind = 'exactas' } = $props()

  let counts = $state({})
</script>

<h1>Revisar</h1>
<nav class="tabs">
  <a href="/revisar" class:on={tab === 'sin-identificar'}>Sin identificar{counts.unidentified != null ? ` (${counts.unidentified})` : ''}</a>
  <a href="/revisar/generos" class:on={tab === 'generos'}>Géneros por mapear{counts.genres != null ? ` (${counts.genres})` : ''}</a>
  <a href="/revisar/duplicados" class:on={tab === 'duplicados'}>Duplicados{counts.duplicates != null ? ` (${counts.duplicates})` : ''}</a>
  <a href="/revisar/formatos" class:on={tab === 'formatos'}>Formatos a convertir{counts.convert != null ? ` (${counts.convert})` : ''}</a>
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
