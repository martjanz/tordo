<script>
  // An artist card: a round picture (one of their album covers), name and
  // how many albums.
  import { albumCoverURL } from '../lib/api.js'
  import { t } from '../lib/i18n/index.svelte.js'
  import { artistHref } from '../lib/router.js'
  import Tapa from './Tapa.svelte'

  let { artist } = $props()
</script>

<a class="card" href={artistHref(artist.key)}>
  <Tapa srcs={artist.cover ? [albumCoverURL(artist.cover, 320)] : []} icon="user" round alt={artist.name} />
  <span class="name">{artist.name}</span>
  <span class="sub">{t('count.albums', { n: artist.albums })}</span>
</a>

<style>
  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
    min-width: 0;
    text-align: center;
  }
  .name {
    margin-top: 6px;
    max-width: 100%;
    color: var(--strong);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    font-size: var(--fs-sm);
    color: var(--muted);
  }
</style>
