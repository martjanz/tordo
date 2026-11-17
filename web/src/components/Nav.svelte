<script>
  import { listenMode } from '../lib/app.svelte.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { player } from '../lib/playback.svelte.js'
  import { rockola, rockolaAvailable, setRockola } from '../lib/rockola.svelte.js'
  import { reactivate, speaker } from '../lib/speaker.svelte.js'
  import Busqueda from './Busqueda.svelte'
  import Estado from './Estado.svelte'
  import Icono from './Icono.svelte'

  // bare shows only the brand (the first-use assistant).
  let { page, bare = false } = $props()

  // A single list belongs to Listas, the statistics to Inicio.
  const isActive = (s) =>
    page === s.page ||
    (s.page === 'listas' && page === 'lista') ||
    (s.page === 'inicio' && page === 'estadisticas')

  // The sections of the app (label: the message key).
  const SECTIONS = [
    { page: 'inicio', href: '/', label: 'nav.home', icon: 'home' },
    { page: 'explorar', href: '/explore', label: 'nav.explore', icon: 'explore' },
    { page: 'carpetas', href: '/folders', label: 'nav.folders', icon: 'folder' },
    { page: 'listas', href: '/lists', label: 'nav.lists', icon: 'lists' },
    { page: 'revisar', href: '/review', label: 'nav.review', icon: 'review' },
  ]
  const sections = $derived(SECTIONS.filter((s) => !(s.page === 'revisar' && listenMode())))
</script>

<header>
  <nav>
    <a class="brand" class:alone={bare} href="/">
      <svg viewBox="0 0 32 32" width="20" height="20" aria-hidden="true">
        <circle cx="16" cy="16" r="10.5" fill="none" stroke="currentColor" stroke-width="2.5" />
        <circle cx="16" cy="16" r="3" fill="currentColor" />
      </svg>
      {t('app.name')}
    </a>
    {#if !bare}
      <span class="sections">
        {#each sections as s (s.page)}
          <a href={s.href} class:active={isActive(s)}>{t(s.label)}</a>
        {/each}
      </span>
      <span class="spacer"></span>
      {#if listenMode()}<a class="modo" href="/settings" title={t('nav.listenTitle')}>{t('nav.listenMode')}</a>{/if}
      <Busqueda />
      {#if speaker.on}
        {#if player.blocked}
          <button class="chip" type="button" onclick={reactivate}>{t('nav.reactivate')}</button>
        {:else if !speaker.connected}
          <a class="chip" href="/settings">{t('nav.reconnecting')}</a>
        {:else}
          <a class="chip" href="/settings">{t('nav.thisComputer')}</a>
        {/if}
      {/if}
      {#if !speaker.on && (rockolaAvailable() || rockola.on)}
        <button class="chip toggle" class:on={rockola.on} type="button" aria-pressed={rockola.on}
          title={t('nav.rockolaTitle')} onclick={() => setRockola(!rockola.on)}>{t('nav.rockola')}</button>
      {/if}
      <Estado />
      <a class="gear help" href="/help" class:active={page === 'ayuda'} aria-label={t('nav.help')} title={t('nav.helpTitle')}>?</a>
      <a class="gear" href="/settings" class:active={page === 'ajustes'} aria-label={t('nav.settings')} title={t('nav.settings')}>
        <Icono name="settings" size={18} />
      </a>
    {/if}
  </nav>
</header>

<!-- On phones the sections go to tabs at the bottom, under the player. -->
{#if !bare}
  <nav class="tabs" aria-label={t('nav.sections')}>
    {#each sections as s (s.page)}
      <a href={s.href} class:active={isActive(s)}>
        <Icono name={s.icon} size={20} />
        <span>{t(s.label)}</span>
      </a>
    {/each}
  </nav>
{/if}

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
  .brand.alone {
    border-right: none;
    padding-right: 0;
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
  .modo {
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--accent);
    border: 1px solid var(--accent);
    border-radius: 999px;
    padding: 2px 8px;
    white-space: nowrap;
  }
  .chip {
    font: inherit;
    font-size: 11px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--accent);
    background: none;
    border: 1px solid var(--accent);
    border-radius: 999px;
    padding: 2px 8px;
    white-space: nowrap;
    cursor: pointer;
  }
  .chip.toggle.on {
    background: var(--accent);
    color: var(--bg);
  }
  .gear {
    display: inline-flex;
    color: var(--muted);
  }
  .help {
    font-size: 16px;
    font-weight: 700;
    width: 18px;
    justify-content: center;
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
    .help {
      display: none; /* phones reach the help from Ajustes */
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
