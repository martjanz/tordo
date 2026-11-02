// Reading /api/status: {readOnly, scan, identify (from M2), counts}.

// busy reports whether a scan or an identification run is going on.
export function busy(st) {
  return !!st && (!!st.scan?.running || st.identify?.state === 'running')
}

// progress sums up what a run has changed so far: when it moves, the pages
// have new data to show.
export function progress(st) {
  if (!st) return ''
  return [st.scan?.read ?? 0, st.scan?.albums ?? 0, st.identify?.identified ?? 0, st.identify?.enriched ?? 0].join('|')
}

// summary is the short text of the status indicator, and its tone: "busy",
// "warn", "ok" (all quiet) or "down" (the last poll did not reach the server,
// which outweighs whatever status it had before).
export function summary(st, lost = false) {
  if (lost) return { text: 'Sin conexión con Tordo', tone: 'down' }
  if (!st) return { text: '', tone: '' }
  if (st.scan?.running) {
    const { files = 0, read = 0, toRead = 0 } = st.scan
    const text = {
      walk: `Buscando archivos · ${num(files)}`,
      read: `Leyendo archivos · ${num(read)}/${num(toRead)} (${percent(read, toRead)} %)`,
      group: 'Armando discos',
    }[scanPhase(st.scan)]
    return { text, tone: 'busy' }
  }
  const id = st.identify
  if (id?.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      return { text: `Completando datos · ${id.enriched}/${id.toEnrich}`, tone: 'busy' }
    }
    return { text: `Identificando · ${id.identified}/${id.toIdentify}`, tone: 'busy' }
  }
  if (st.readOnly) return { text: 'Modo consulta', tone: 'warn' }
  if (st.scan?.lastError) return { text: 'El último escaneo falló', tone: 'warn' }
  if (id?.state === 'offline') return { text: 'Sin conexión con MusicBrainz', tone: 'warn' }
  return { text: 'Al día', tone: 'ok' }
}

// details is the longer text of the status indicator, for its tooltip: what
// is going on, how far along, and what comes next.
export function details(st, lost = false) {
  if (lost) return 'No se puede contactar a Tordo; se reintenta cada 3 segundos.'
  if (!st) return ''
  const scan = st.scan
  if (scan?.running) {
    const then = 'Al terminar, Tordo identifica los álbumes en MusicBrainz.'
    const { files = 0, read = 0, toRead = 0, albums = 0, tracks = 0, etaSeconds = 0 } = scan
    switch (scanPhase(scan)) {
      case 'walk':
        return lines('Escaneo, paso 1 de 3: buscando archivos de música en las carpetas.', `${num(files)} encontrados hasta ahora.`, then)
      case 'group':
        return lines('Escaneo, paso 3 de 3: armando los discos con lo leído.', then)
      default:
        return lines(
          'Escaneo, paso 2 de 3: leyendo los tags y datos técnicos de cada archivo.',
          `${num(read)} de ${num(toRead)} (${percent(read, toRead)} %)${etaSeconds > 0 ? ` · faltan ${timeLeft(etaSeconds)}` : ''}.`,
          albums > 0
            ? `Ya hay ${num(albums)} discos y ${num(tracks)} pistas; aparecen más cada medio minuto.`
            : 'Los primeros discos aparecen en menos de un minuto.',
          then,
        )
    }
  }
  const id = st.identify
  if (id?.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      return lines(`Completando datos de los álbumes identificados: ${num(id.enriched)} de ${num(id.toEnrich)}.`, 'Títulos, año original, país, tapas y géneros.')
    }
    return lines(
      `Identificando álbumes en MusicBrainz: ${num(id.identified)} de ${num(id.toIdentify)}.`,
      'Una consulta por segundo, como pide MusicBrainz; si se corta, sigue donde quedó.',
      'Lo que no se identifique solo queda en Revisar → Sin identificar.',
    )
  }
  if (st.readOnly) return 'La carpeta de Tordo no se puede modificar: se puede explorar y escuchar, pero no se guardan cambios.'
  if (scan?.lastError) return `El último escaneo falló: ${scan.lastError}`
  if (id?.state === 'offline') return 'Sin conexión con MusicBrainz: la identificación se pausa y se reintenta sola.'
  return `Catálogo al día: ${num(st.counts?.albums)} discos, ${num(st.counts?.tracks)} pistas.`
}

const lines = (...ls) => ls.join('\n')

// timeLeft words an estimate in seconds: minutes rounded up, then hours.
function timeLeft(secs) {
  if (secs < 60) return 'menos de un minuto'
  if (secs < 90 * 60) return `unos ${Math.ceil(secs / 60)} min`
  return `unas ${Math.round(secs / 3600)} h`
}

// scanPhase is what a running scan is doing: "walk" (finding the files),
// "read" (reading them) or "group" (building the albums once all are read).
function scanPhase(scan) {
  const { read = 0, toRead = 0 } = scan ?? {}
  return toRead === 0 ? 'walk' : read >= toRead ? 'group' : 'read'
}

const num = (v) => (v ?? 0).toLocaleString('es-AR')

// percent rounds down: 100 % only once everything is done.
const percent = (done, total) => Math.floor((done * 100) / total)

// scanMessage explains a running scan to a page that has nothing to show
// yet: what it is doing, and when the music will show up (the scan regroups
// the catalog every 30 s while it reads, the pages reload every 20 s).
export function scanMessage(scan) {
  const { files = 0, read = 0, toRead = 0 } = scan ?? {}
  switch (scanPhase(scan)) {
    case 'walk':
      return {
        title: `Buscando archivos de música… ${num(files)} hasta ahora.`,
        detail: 'Después se leen sus tags y los discos van apareciendo acá.',
      }
    case 'group':
      return { title: 'Armando los discos…', detail: 'Ya se leyeron todos los archivos; en un momento aparecen acá.' }
    default:
      return {
        title: `Leyendo los archivos: ${num(read)} de ${num(toRead)} (${percent(read, toRead)} %).`,
        detail: 'Los discos aparecen acá a medida que se leen, los primeros en menos de un minuto. La página se actualiza sola.',
      }
  }
}

// listening reports whether listen mode is on: the server blocks curation
// and the UI hides it.
export function listening(st) {
  return !!st?.listenMode
}

// pageBlocked tells whether a page is out of reach in the current status
// (Revisar is curation, so listen mode closes it).
export function pageBlocked(page, st) {
  return page === 'revisar' && listening(st)
}
