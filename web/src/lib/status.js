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
  if (st.preparing && !st.scan?.running && st.identify?.state !== 'running') return { text: 'Preparando el catálogo', tone: 'busy' }
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
  if (st.scan?.skipped > 0) {
    return { text: st.scan.skipped === 1 ? '1 carpeta sin leer' : `${num(st.scan.skipped)} carpetas sin leer`, tone: 'warn' }
  }
  if (id?.state === 'offline') return { text: 'Sin conexión con MusicBrainz', tone: 'warn' }
  return { text: 'Al día', tone: 'ok' }
}

// STEPS are the steps from the music folders to a complete catalog, as the
// status panel lists them.
const STEPS = ['Buscar archivos', 'Leer tags', 'Armar discos', 'Identificar', 'Completar datos']

// panel is what the status panel shows: a title and tone (as summary's), the
// steps with the current one, a progress bar, the catalog's figures and some
// notes. progress.percent is null for a step with no known end.
export function panel(st, lost = false) {
  const { text, tone } = summary(st, lost)
  const out = { title: text, tone, steps: null, progress: null, facts: [], notes: [] }
  if (lost) return { ...out, title: 'Sin conexión con Tordo', notes: ['No se puede contactar a Tordo; se reintenta cada 3 segundos.'] }
  if (!st) return out
  const steps = (current, state = 'busy') =>
    STEPS.map((label, i) => ({ label, state: current == null || i < current ? 'done' : i === current ? state : 'todo' }))
  const bar = (label, done, total, eta = '') => ({ label, value: `${num(done)} de ${num(total)}`, percent: percent(done, total), eta })
  const scan = st.scan ?? {}
  const id = st.identify ?? {}

  if (st.preparing && (scan.running || id.state === 'running')) {
    out.notes.push('También se está preparando el catálogo para mostrarlo.')
  }
  if (st.preparing && !scan.running && id.state !== 'running') {
    out.progress = { label: 'Cargando discos y pistas', value: '', percent: null, eta: '' }
    out.notes.push('Se hace una vez al abrir Tordo; con una colección grande puede tardar.')
  } else if (scan.running) {
    out.title = 'Escaneando la música'
    switch (scanPhase(scan)) {
      case 'walk':
        out.steps = steps(0)
        out.progress = { label: 'Archivos encontrados', value: num(scan.files), percent: null, eta: '' }
        out.notes.push('Después se leen sus tags y se arman los discos.')
        break
      case 'group':
        out.steps = steps(2)
        out.progress = { label: 'Armando los discos con lo leído', value: '', percent: null, eta: '' }
        break
      default:
        out.steps = steps(1)
        out.progress = bar('Archivos leídos', scan.read, scan.toRead, scan.etaSeconds > 0 ? timeLeft(scan.etaSeconds) : '')
        out.notes.push('Los discos aparecen a medida que se leen: el catálogo se rearma cada medio minuto.')
    }
  } else if (id.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      out.title = 'Completando datos'
      out.steps = steps(4)
      out.progress = bar('Álbumes completados', id.enriched, id.toEnrich)
      out.notes.push('Títulos de MusicBrainz, año original, país, tapas y géneros.')
    } else {
      out.title = 'Identificando en MusicBrainz'
      out.steps = steps(3)
      out.progress = bar('Álbumes buscados', id.identified, id.toIdentify)
      out.notes.push('Una consulta por segundo, como pide MusicBrainz; si se corta, sigue donde quedó.')
    }
  } else if (st.readOnly) {
    out.notes.push('La carpeta de Tordo no se puede modificar: se puede explorar y escuchar, pero no se guardan cambios.')
  } else if (scan.lastError) {
    out.error = scan.lastError
  } else if (id.state === 'offline') {
    out.steps = steps(3, 'warn')
    out.notes.push('La identificación se pausa y se reintenta sola. Todo lo demás funciona igual.')
  } else {
    out.steps = steps(null)
  }

  if (!scan.running && scan.skipped > 0) {
    const paths = scan.skippedPaths ?? []
    out.notes.push('No se pudieron leer: lo que tenían sigue en el catálogo tal como estaba.', ...paths)
    if (scan.skipped > paths.length) out.notes.push(`Y ${num(scan.skipped - paths.length)} más.`)
  }

  const c = st.counts ?? {}
  if (c.albums > 0 || c.tracks > 0) out.facts.push({ label: 'Discos', value: num(c.albums) }, { label: 'Pistas', value: num(c.tracks) })
  if (c.identified > 0) out.facts.push({ label: 'Identificados', value: num(c.identified) })
  if (c.unidentified > 0) {
    const fact = { label: 'Sin identificar', value: num(c.unidentified) }
    out.facts.push(pageBlocked('revisar', st) ? fact : { ...fact, href: '/revisar' })
  }
  if (c.unreadable > 0) out.facts.push({ label: 'Ilegibles', value: num(c.unreadable) })

  if (st.loudness?.state === 'running' && st.loudness.pending > 0) {
    out.notes.push(`Midiendo el volumen de las pistas para normalizarlo: faltan ${num(st.loudness.pending)}.`)
  }
  return out
}

// timeLeft words an estimate in seconds: minutes rounded up, then hours.
function timeLeft(secs) {
  if (secs < 60) return 'falta menos de un minuto'
  if (secs < 90 * 60) return `faltan unos ${Math.ceil(secs / 60)} min`
  return `faltan unas ${Math.round(secs / 3600)} h`
}

// scanPhase is what a running scan is doing: "walk" (finding the files),
// "read" (reading them) or "group" (building the albums once all are read,
// also when there was nothing to read).
function scanPhase(scan) {
  const { read = 0, toRead = 0, grouping = false } = scan ?? {}
  if (grouping) return 'group'
  return toRead === 0 ? 'walk' : read >= toRead ? 'group' : 'read'
}

const num = (v) => (v ?? 0).toLocaleString('es-AR')

// percent rounds down: 100 % only once everything is done.
const percent = (done = 0, total = 0) => (total > 0 ? Math.floor((done * 100) / total) : 0)

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

// loadingMessage is what a page says while its data is on its way: what the
// server is doing, so a long wait does not look stuck.
export function loadingMessage(st, lost = false) {
  if (lost) return { title: 'Sin conexión con Tordo', detail: 'Se reintenta cada 3 segundos.' }
  const running = !!st?.scan?.running || st?.identify?.state === 'running'
  const detail = running ? summary(st).text : ''
  if (st?.preparing) {
    return { title: 'Preparando el catálogo…', detail: detail || 'Se hace una vez al abrir Tordo; con una colección grande puede tardar.' }
  }
  return { title: 'Cargando…', detail }
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
