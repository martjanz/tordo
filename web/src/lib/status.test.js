import { describe, expect, it } from 'vitest'
import { busy, progress, summary } from './status.js'

describe('summary', () => {
  it.each([
    [null, { text: '', tone: '' }],
    [{ scan: { running: true, files: 120 } }, { text: 'Escaneando · 120 archivos', tone: 'busy' }],
    [{ scan: { running: true, files: 120, read: 30, toRead: 90 } }, { text: 'Escaneando · 30/90 leídos', tone: 'busy' }],
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
