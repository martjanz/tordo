<script>
  // A row of Inicio: a shelf of albums, with a station made from them.
  import BotonEstacion from './BotonEstacion.svelte'
  import Estante from './Estante.svelte'
  import TarjetaAlbum from './TarjetaAlbum.svelte'

  let { title, href = '', station = null, albums } = $props()
</script>

<Estante {title} {href} actions={station ? estacion : null}>
  {#each albums as album (album.id)}
    <div><TarjetaAlbum {album} /></div>
  {/each}
</Estante>

{#snippet estacion()}
  <BotonEstacion {station} label="Estación de «{title}»" compact />
{/snippet}
