import { describe, expect, it } from 'vitest'
import { busy, details, listening, pageBlocked, progress, scanMessage, summary } from './status.js'

describe('summary', () => {
  it.each([
    [null, { text: '', tone: '' }],
    [{ scan: { running: true, files: 12345 } }, { text: 'Buscando archivos · 12.345', tone: 'busy' }],
    [
      { scan: { running: true, files: 246993, read: 6680, toRead: 246993 } },
      { text: 'Leyendo archivos · 6.680/246.993 (2 %)', tone: 'busy' },
    ],
    [{ scan: { running: true, files: 90, read: 90, toRead: 90 } }, { text: 'Armando discos', tone: 'busy' }],
    [{ scan: {}, identify: { state: 'running', identified: 3, toIdentify: 10 } }, { text: 'Identificando · 3/10', tone: 'busy' }],
    [
      { scan: {}, identify: { state: 'running', identified: 10, toIdentify: 10, enriched: 2, toEnrich: 5 } },
      { text: 'Completando datos · 2/5', tone: 'busy' },
    ],
    [{ readOnly: true, scan: {} }, { text: 'Modo consulta', tone: 'warn' }],
    [{ scan: { lastError: 'disco desconectado' } }, { text: 'El último escaneo falló', tone: 'warn' }],
    [{ scan: {}, identify: { state: 'offline' } }, { text: 'Sin conexión con MusicBrainz', tone: 'warn' }],
    [{ scan: {} }, { text: 'Al día', tone: 'ok' }],
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

describe('details', () => {
  const then = 'Al terminar, Tordo identifica los álbumes en MusicBrainz.'
  it.each([
    [{ scan: { running: true, files: 12345 } }, ['Escaneo, paso 1 de 3: buscando archivos de música en las carpetas.', '12.345 encontrados hasta ahora.', then]],
    [
      { scan: { running: true, read: 6680, toRead: 246993 } },
      ['Escaneo, paso 2 de 3: leyendo los tags y datos técnicos de cada archivo.', '6.680 de 246.993 (2 %).', 'Los primeros discos aparecen en menos de un minuto.', then],
    ],
    [
      { scan: { running: true, read: 6680, toRead: 246993, albums: 1200, tracks: 15000, etaSeconds: 3 * 3600 + 600 } },
      ['Escaneo, paso 2 de 3: leyendo los tags y datos técnicos de cada archivo.', '6.680 de 246.993 (2 %) · faltan unas 3 h.', 'Ya hay 1.200 discos y 15.000 pistas; aparecen más cada medio minuto.', then],
    ],
    [{ scan: { running: true, read: 90, toRead: 90 } }, ['Escaneo, paso 3 de 3: armando los discos con lo leído.', then]],
    [
      { scan: {}, identify: { state: 'running', identified: 3, toIdentify: 10 } },
      ['Identificando álbumes en MusicBrainz: 3 de 10.', 'Una consulta por segundo, como pide MusicBrainz; si se corta, sigue donde quedó.', 'Lo que no se identifique solo queda en Revisar → Sin identificar.'],
    ],
    [
      { scan: {}, identify: { state: 'running', identified: 10, toIdentify: 10, enriched: 2, toEnrich: 5 } },
      ['Completando datos de los álbumes identificados: 2 de 5.', 'Títulos, año original, país, tapas y géneros.'],
    ],
    [{ readOnly: true, scan: {} }, ['La carpeta de Tordo no se puede modificar: se puede explorar y escuchar, pero no se guardan cambios.']],
    [{ scan: { lastError: 'disco desconectado' } }, ['El último escaneo falló: disco desconectado']],
    [{ scan: {}, identify: { state: 'offline' } }, ['Sin conexión con MusicBrainz: la identificación se pausa y se reintenta sola.']],
    [{ scan: {}, counts: { albums: 1234, tracks: 15000 } }, ['Catálogo al día: 1.234 discos, 15.000 pistas.']],
  ])('%j', (st, want) => {
    expect(details(st)).toBe(want.join('\n'))
  })

  it.each([
    [30, 'menos de un minuto'],
    [61, 'unos 2 min'],
    [89 * 60, 'unos 89 min'],
    [90 * 60, 'unas 2 h'],
  ])('time left %i s', (eta, want) => {
    expect(details({ scan: { running: true, read: 1, toRead: 2, etaSeconds: eta } })).toContain(`faltan ${want}.`)
  })

  it('explains a lost server', () => {
    expect(details({ scan: {} }, true)).toBe('No se puede contactar a Tordo; se reintenta cada 3 segundos.')
  })
})
