<script>
  import { disableSpeaker, enableSpeaker, speaker } from '../lib/speaker.svelte.js'
</script>

<!-- Acts at once: it is not part of Ajustes' Guardar form. -->
<label class="check">
  <input type="checkbox" checked={speaker.on} onchange={(e) => (e.currentTarget.checked ? enableSpeaker() : disableSpeaker())} />
  <span>
    Usar el audio de esta compu para tordo
    <small>Los teléfonos en modo Rockola eligen qué suena acá. Mantené esta pestaña abierta.</small>
  </span>
</label>
