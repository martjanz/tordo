<script>
  import Busqueda from './Busqueda.svelte'
  import Estado from './Estado.svelte'
  import Icono from './Icono.svelte'

  let { page } = $props()

  // A single list belongs to Listas, the statistics to Inicio.
  const isActive = (s) =>
    page === s.page ||
    (s.page === 'listas' && page === 'lista') ||
    (s.page === 'inicio' && page === 'estadisticas')

  // The sections of the app.
  const SECTIONS = [
    { page: 'inicio', href: '/', label: 'Inicio', icon: 'home' },
    { page: 'explorar', href: '/explorar', label: 'Explorar', icon: 'explore' },
    { page: 'carpetas', href: '/carpetas', label: 'Carpetas', icon: 'folder' },
    { page: 'listas', href: '/listas', label: 'Listas', icon: 'lists' },
    { page: 'revisar', href: '/revisar', label: 'Revisar', icon: 'review' },
  ]
</script>

<header>
  <nav>
    <a class="brand" href="/">
      <svg viewBox="0 0 32 32" width="20" height="20" aria-hidden="true">
        <circle cx="16" cy="16" r="10.5" fill="none" stroke="currentColor" stroke-width="2.5" />
        <circle cx="16" cy="16" r="3" fill="currentColor" />
      </svg>
      Tordo
    </a>
    <span class="sections">
      {#each SECTIONS as s (s.page)}
        <a href={s.href} class:active={isActive(s)}>{s.label}</a>
      {/each}
    </span>
    <span class="spacer"></span>
    <Busqueda />
    <Estado />
    <a class="gear" href="/ajustes" class:active={page === 'ajustes'} aria-label="Ajustes" title="Ajustes">
      <Icono name="settings" size={18} />
    </a>
  </nav>
</header>

<!-- On phones the sections go to tabs at the bottom, under the player. -->
<nav class="tabs" aria-label="Secciones">
  {#each SECTIONS as s (s.page)}
    <a href={s.href} class:active={isActive(s)}>
      <Icono name={s.icon} size={20} />
      <span>{s.label}</span>
    </a>
  {/each}
</nav>

<style>
  header {
    position: sticky;
    top: 0;
    z-index: 20;
    background: rgba(21, 23, 27, 0.94);
    backdrop-filter: blur(6px);
    border-bottom: 1px solid var(--line);
    margin-bottom: 16px;
  }
  header nav {
    max-width: 1400px;
    margin: 0 auto;
    padding: 12px var(--gutter);
    display: flex;
    align-items: center;
    gap: 20px;
  }
  .sections {
    display: flex;
    gap: 20px;
    font-size: 13px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-right: 20px;
    border-right: 1px solid var(--line-strong);
    color: var(--strong);
    font-size: 18px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }
  .brand svg {
    color: var(--accent);
  }
  .active {
    color: var(--strong);
  }
  .spacer {
    flex: 1;
  }
  .gear {
    display: inline-flex;
    color: var(--muted);
  }
  .gear.active {
    color: var(--strong);
  }
  .tabs {
    display: none;
  }
  @media (max-width: 640px) {
    .sections {
      display: none;
    }
    .brand {
      border-right: none;
    }
    .tabs {
      display: flex;
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 25;
      height: calc(var(--tabs-h) + env(safe-area-inset-bottom));
      padding-bottom: env(safe-area-inset-bottom);
      background: var(--surface);
      border-top: 1px solid var(--line);
    }
    .tabs a {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 2px;
      font-size: 11px;
      color: var(--faint);
    }
    .tabs a.active {
      color: var(--accent);
    }
  }
</style>
