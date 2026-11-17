<script>
  // A row of Inicio: a title, its actions and the albums in one line that
  // scrolls sideways.
  import { t } from '../lib/i18n/index.svelte.js'
  import BotonEstacion from './BotonEstacion.svelte'
  import TarjetaAlbum from './TarjetaAlbum.svelte'

  let { title, href = '', station = null, albums } = $props()
</script>

<section>
  <div class="head">
    <h2>{title}</h2>
    {#if href}<a class="all" href={href}>{t('common.seeAll')}</a>{/if}
    <span class="spacer"></span>
    {#if station}<BotonEstacion {station} />{/if}
  </div>
  <div class="strip">
    {#each albums as album (album.id)}
      <div class="cell"><TarjetaAlbum {album} /></div>
    {/each}
  </div>
</section>

<style>
  section {
    margin-bottom: 28px;
  }
  .head {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin-bottom: 12px;
  }
  .head h2 {
    margin: 0;
  }
  .head :global(button) {
    align-self: center;
  }
  .all {
    font-size: 13px;
    color: var(--muted);
  }
  .spacer {
    flex: 1;
  }
  .strip {
    display: flex;
    gap: 16px;
    overflow-x: auto;
    scroll-snap-type: x proximity;
    padding-bottom: 8px;
  }
  .cell {
    flex: 0 0 168px;
    min-width: 0;
    scroll-snap-align: start;
  }
  @media (max-width: 640px) {
    .strip {
      gap: 12px;
    }
    .cell {
      flex-basis: 140px;
    }
  }
</style>
