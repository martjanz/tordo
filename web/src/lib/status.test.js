import { describe, expect, it } from 'vitest'
import { busy, listening, loadingMessage, pageBlocked, panel, progress, scanMessage, summary } from './status.js'

describe('summary', () => {
  it.each([
    [null, { text: '', tone: '' }],
    [{ scan: { running: true, files: 12345 } }, { text: 'Buscando archivos · 12.345', tone: 'busy' }],
    [
      { scan: { running: true, files: 246993, read: 6680, toRead: 246993 } },
      { text: 'Leyendo archivos · 6.680/246.993 (2 %)', tone: 'busy' },
    ],
    [{ scan: { running: true, files: 90, read: 90, toRead: 90 } }, { text: 'Armando discos', tone: 'busy' }],
    [{ scan: { running: true, files: 90, grouping: true } }, { text: 'Armando discos', tone: 'busy' }],
    [{ scan: {}, identify: { state: 'running', identified: 3, toIdentify: 10 } }, { text: 'Identificando · 3/10', tone: 'busy' }],
    [
      { scan: {}, identify: { state: 'running', identified: 10, toIdentify: 10, enriched: 2, toEnrich: 5 } },
      { text: 'Completando datos · 2/5', tone: 'busy' },
    ],
    [{ readOnly: true, scan: {} }, { text: 'Modo consulta', tone: 'warn' }],
    [{ scan: { lastError: 'disco desconectado' } }, { text: 'El último escaneo falló', tone: 'warn' }],
    [{ scan: { skipped: 1 } }, { text: '1 carpeta sin leer', tone: 'warn' }],
    [{ scan: { skipped: 3 } }, { text: '3 carpetas sin leer', tone: 'warn' }],
    [{ scan: { lastError: 'disco desconectado', skipped: 3 } }, { text: 'El último escaneo falló', tone: 'warn' }],
    [{ scan: {}, identify: { state: 'offline' } }, { text: 'Sin conexión con MusicBrainz', tone: 'warn' }],
    [{ scan: {} }, { text: 'Al día', tone: 'ok' }],
    [{ scan: {}, preparing: true }, { text: 'Preparando el catálogo', tone: 'busy' }],
    [{ readOnly: true, scan: {}, preparing: true }, { text: 'Preparando el catálogo', tone: 'busy' }],
    [{ scan: { running: true, files: 5 }, preparing: true }, { text: 'Buscando archivos · 5', tone: 'busy' }],
  ])('%j', (st, want) => {
    expect(summary(st)).toEqual(want)
  })

  it('reports a lost server over any status it last had', () => {
    const down = { text: 'Sin conexión con Tordo', tone: 'down' }
    expect(summary(null, true)).toEqual(down)
    expect(summary({ scan: {} }, true)).toEqual(down)
    expect(summary({ scan: { running: true, files: 3 } }, true)).toEqual(down)
  })
})

describe('busy and progress', () => {
  it('follows scans and identification runs', () => {
    expect(busy({ scan: { running: true } })).toBe(true)
    expect(busy({ scan: {}, identify: { state: 'running' } })).toBe(true)
    expect(busy({ scan: {}, identify: { state: 'idle' } })).toBe(false)
    expect(progress({ scan: { read: 3, albums: 1 } })).toBe('3|1|0|0')
  })
})

describe('listening and curation', () => {
  it('reads the listen mode flag', () => {
    expect(listening(null)).toBe(false)
    expect(listening({})).toBe(false)
    expect(listening({ listenMode: true })).toBe(true)
  })
  it('blocks the review page only while listening', () => {
    expect(pageBlocked('revisar', { listenMode: true })).toBe(true)
    expect(pageBlocked('revisar', { listenMode: false })).toBe(false)
    expect(pageBlocked('explorar', { listenMode: true })).toBe(false)
    expect(pageBlocked('revisar', null)).toBe(false)
  })
})

describe('scanMessage', () => {
  it('tells what the scan is doing and when the music shows up', () => {
    expect(scanMessage({ running: true, files: 12345 })).toEqual({
      title: 'Buscando archivos de música… 12.345 hasta ahora.',
      detail: 'Después se leen sus tags y los discos van apareciendo acá.',
    })
    expect(scanMessage({ running: true, files: 246993, read: 6680, toRead: 246993 })).toEqual({
      title: 'Leyendo los archivos: 6.680 de 246.993 (2 %).',
      detail: 'Los discos aparecen acá a medida que se leen, los primeros en menos de un minuto. La página se actualiza sola.',
    })
    expect(scanMessage({ running: true, files: 10, read: 10, toRead: 10 })).toEqual({
      title: 'Armando los discos…',
      detail: 'Ya se leyeron todos los archivos; en un momento aparecen acá.',
    })
  })
})

describe('panel', () => {
  const steps = (current, tone = 'busy') =>
    ['Buscar archivos', 'Leer tags', 'Armar discos', 'Identificar', 'Completar datos'].map((label, i) => ({
      label,
      state: current == null || i < current ? 'done' : i === current ? tone : 'todo',
    }))
  const counts = { albums: 1200, tracks: 15000, unreadable: 0, identified: 0, unidentified: 0 }

  it('walks a scan through its steps', () => {
    const walk = panel({ scan: { running: true, files: 12345 }, counts: { albums: 0, tracks: 0 } })
    expect(walk.title).toBe('Escaneando la música')
    expect(walk.tone).toBe('busy')
    expect(walk.steps).toEqual(steps(0))
    expect(walk.progress).toEqual({ label: 'Archivos encontrados', value: '12.345', percent: null, eta: '' })
    expect(walk.facts).toEqual([])

    const read = panel({ scan: { running: true, files: 246993, read: 6680, toRead: 246993, etaSeconds: 3 * 3600 + 600 }, counts })
    expect(read.steps).toEqual(steps(1))
    expect(read.progress).toEqual({ label: 'Archivos leídos', value: '6.680 de 246.993', percent: 2, eta: 'faltan unas 3 h' })
    expect(read.facts).toEqual([
      { label: 'Discos', value: '1.200' },
      { label: 'Pistas', value: '15.000' },
    ])
    expect(read.notes).toEqual(['Los discos aparecen a medida que se leen: el catálogo se rearma cada medio minuto.'])

    const group = panel({ scan: { running: true, read: 90, toRead: 90 }, counts })
    expect(group.steps).toEqual(steps(2))
    expect(group.progress).toEqual({ label: 'Armando los discos con lo leído', value: '', percent: null, eta: '' })
  })

  it('follows identification and enrichment', () => {
    const id = panel({ scan: {}, identify: { state: 'running', identified: 3, toIdentify: 10 }, counts })
    expect(id.title).toBe('Identificando en MusicBrainz')
    expect(id.steps).toEqual(steps(3))
    expect(id.progress).toEqual({ label: 'Álbumes buscados', value: '3 de 10', percent: 30, eta: '' })
    expect(id.notes).toEqual(['Una consulta por segundo, como pide MusicBrainz; si se corta, sigue donde quedó.'])

    const en = panel({ scan: {}, identify: { state: 'running', identified: 10, toIdentify: 10, enriched: 2, toEnrich: 5 }, counts })
    expect(en.title).toBe('Completando datos')
    expect(en.steps).toEqual(steps(4))
    expect(en.progress).toEqual({ label: 'Álbumes completados', value: '2 de 5', percent: 40, eta: '' })
  })

  it('sums up a quiet catalog, with links to what needs a look', () => {
    const ok = panel({ scan: {}, identify: { state: 'idle' }, counts: { ...counts, identified: 1100, unidentified: 90, unreadable: 3 } })
    expect(ok.title).toBe('Al día')
    expect(ok.tone).toBe('ok')
    expect(ok.steps).toEqual(steps(null))
    expect(ok.progress).toBeNull()
    expect(ok.facts).toEqual([
      { label: 'Discos', value: '1.200' },
      { label: 'Pistas', value: '15.000' },
      { label: 'Identificados', value: '1.100' },
      { label: 'Sin identificar', value: '90', href: '/revisar' },
      { label: 'Ilegibles', value: '3' },
    ])
    // Listen mode closes Revisar: no link there.
    expect(panel({ scan: {}, listenMode: true, counts: { ...counts, unidentified: 90 } }).facts.find((f) => f.label === 'Sin identificar')).toEqual({ label: 'Sin identificar', value: '90' })
  })

  it('explains the warnings', () => {
    const off = panel({ scan: {}, identify: { state: 'offline' }, counts })
    expect(off.title).toBe('Sin conexión con MusicBrainz')
    expect(off.tone).toBe('warn')
    expect(off.steps).toEqual(steps(3, 'warn'))
    expect(off.notes).toEqual(['La identificación se pausa y se reintenta sola. Todo lo demás funciona igual.'])

    const failed = panel({ scan: { lastError: 'disco desconectado' }, counts })
    expect(failed.title).toBe('El último escaneo falló')
    expect(failed.error).toBe('disco desconectado')

    const ro = panel({ readOnly: true, scan: {}, counts })
    expect(ro.title).toBe('Modo consulta')
    expect(ro.notes).toEqual(['La carpeta de Tordo no se puede modificar: se puede explorar y escuchar, pero no se guardan cambios.'])

    const lost = panel({ scan: { running: true } }, true)
    expect(lost).toMatchObject({ title: 'Sin conexión con Tordo', tone: 'down', steps: null, progress: null, facts: [] })
    expect(lost.notes).toEqual(['No se puede contactar a Tordo; se reintenta cada 3 segundos.'])
  })

  it('mentions the loudness measuring going on in the background', () => {
    const p = panel({ scan: {}, loudness: { state: 'running', pending: 4200 }, counts })
    expect(p.notes).toContain('Midiendo el volumen de las pistas para normalizarlo: faltan 4.200.')
  })

  it.each([
    [30, 'falta menos de un minuto'],
    [61, 'faltan unos 2 min'],
    [89 * 60, 'faltan unos 89 min'],
    [90 * 60, 'faltan unas 2 h'],
  ])('time left %i s', (eta, want) => {
    expect(panel({ scan: { running: true, read: 1, toRead: 2, etaSeconds: eta } }).progress.eta).toBe(want)
  })
})

describe('panel with unread folders', () => {
  it('names them and says what happens to their files', () => {
    const { notes, title, tone } = panel({ scan: { skipped: 3, skippedPaths: ['../music/a ', '../music/b'] } })
    expect(title).toBe('3 carpetas sin leer')
    expect(tone).toBe('warn')
    expect(notes).toEqual([
      'No se pudieron leer: lo que tenían sigue en el catálogo tal como estaba.',
      '../music/a ',
      '../music/b',
      'Y 1 más.',
    ])
  })
})

describe('loadingMessage', () => {
  it('says only that it loads when nothing is known', () => {
    expect(loadingMessage(null)).toEqual({ title: 'Cargando…', detail: '' })
    expect(loadingMessage({ scan: {} })).toEqual({ title: 'Cargando…', detail: '' })
  })

  it('tells the catalog is being prepared, with what else is going on', () => {
    expect(loadingMessage({ scan: {}, preparing: true })).toEqual({
      title: 'Preparando el catálogo…',
      detail: 'Se hace una vez al abrir Tordo; con una colección grande puede tardar.',
    })
    expect(loadingMessage({ scan: { running: true, files: 12345 }, preparing: true })).toEqual({
      title: 'Preparando el catálogo…',
      detail: 'Buscando archivos · 12.345',
    })
  })

  it('shows a run while the page waits', () => {
    expect(loadingMessage({ scan: { running: true, files: 90, read: 9, toRead: 90 } })).toEqual({
      title: 'Cargando…',
      detail: 'Leyendo archivos · 9/90 (10 %)',
    })
  })

  it('tells when the server is out of reach', () => {
    expect(loadingMessage({ scan: {}, preparing: true }, true)).toEqual({
      title: 'Sin conexión con Tordo',
      detail: 'Se reintenta cada 3 segundos.',
    })
  })
})

describe('panel while preparing', () => {
  it('shows an open-ended bar and works without counts', () => {
    const p = panel({ scan: {}, preparing: true })
    expect(p.title).toBe('Preparando el catálogo')
    expect(p.tone).toBe('busy')
    expect(p.progress.percent).toBeNull()
    expect(p.facts).toEqual([])
  })

  it('keeps a running scan as the main thing and adds a note', () => {
    const p = panel({ scan: { running: true, files: 5 }, preparing: true })
    expect(p.title).toBe('Escaneando la música')
    expect(p.notes).toContain('También se está preparando el catálogo para mostrarlo.')
  })
})
