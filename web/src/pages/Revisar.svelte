<script>
  // Revisar: what needs the user's judgment. Duplicados and Formatos a
  // convertir come with M5.
  import GenerosPorMapear from '../components/GenerosPorMapear.svelte'
  import SinIdentificar from '../components/SinIdentificar.svelte'

  let { tab } = $props()

  let counts = $state({})
</script>

<h1>Revisar</h1>
<nav class="tabs">
  <a href="/revisar" class:on={tab === 'sin-identificar'}>Sin identificar{counts.unidentified != null ? ` (${counts.unidentified})` : ''}</a>
  <a href="/revisar/generos" class:on={tab === 'generos'}>Géneros por mapear{counts.genres != null ? ` (${counts.genres})` : ''}</a>
</nav>

{#if tab === 'generos'}
  <GenerosPorMapear onload={(n) => (counts.genres = n)} />
{:else}
  <SinIdentificar onload={(n) => (counts.unidentified = n)} />
{/if}

<style>
  .tabs {
    display: flex;
    gap: 20px;
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
