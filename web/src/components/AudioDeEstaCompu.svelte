<script>
  import { t } from '../lib/i18n/index.svelte.js'
  import { disableSpeaker, enableSpeaker, speaker } from '../lib/speaker.svelte.js'
</script>

<!-- Acts at once: it is not part of Ajustes' Guardar form. -->
<label class="check">
  <input type="checkbox" checked={speaker.on} onchange={(e) => (e.currentTarget.checked ? enableSpeaker() : disableSpeaker())} />
  <span>
    {t('settings.computerAudio')}
    <small>{t('settings.computerAudioHelp')}</small>
  </span>
</label>
