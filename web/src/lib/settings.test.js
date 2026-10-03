import { describe, expect, it } from 'vitest'
import { addRoot, body, changed, draft, pinProblem, portProblem, savedNotice } from './settings.js'

const config = {
  setupPending: false,
  readOnly: false,
  roots: [{ path: '../musica', available: true }],
  suggested: ['../musica-ordenar'],
  ignore: ['*.tmp', 'Samples'],
  hasToken: true,
  tokenHint: '…a1b2',
  normalize: true,
  lan: { enabled: false, port: 4748, hasPin: false },
}

describe('draft', () => {
  it('checks the suggested folders on first use only', () => {
    expect(draft(config).roots.map((r) => r.checked)).toEqual([true, false])
    expect(draft({ ...config, setupPending: true }).roots.map((r) => r.checked)).toEqual([true, true])
  })
  it('puts the ignore patterns one per line and keeps the PIN empty', () => {
    const d = draft(config)
    expect(d.ignore).toBe('*.tmp\nSamples')
    expect(d.token).toBeNull()
    expect(d.normalize).toBe(true)
    expect(d.lan).toEqual({ enabled: false, port: 4748, pin: '' })
  })
})

describe('body', () => {
  it('keeps the token and the PIN unless changed', () => {
    const d = draft(config)
    expect(body(d)).toEqual({
      roots: ['../musica'],
      ignore: ['*.tmp', 'Samples'],
      token: null,
      normalize: true,
      lan: { enabled: false, port: 4748, pin: null },
    })
    expect(changed(config, d)).toBe(false)
    expect(body({ ...d, token: ' eyJ ' }).token).toBe('eyJ')
    expect(body({ ...d, token: '' }).token).toBe('')
  })
  it('trims the ignore lines and drops the empty ones', () => {
    const d = { ...draft(config), ignore: '  *.tmp \n\n   \nSamples\n' }
    expect(body(d).ignore).toEqual(['*.tmp', 'Samples'])
    expect(body({ ...d, ignore: '' }).ignore).toEqual([])
  })
  it('sends the port as a number and a new PIN', () => {
    const d = draft(config)
    expect(body({ ...d, lan: { ...d.lan, port: '5000', pin: '123456' } }).lan).toEqual({ enabled: false, port: 5000, pin: '123456' })
  })
})

describe('addRoot', () => {
  it('adds folders, or checks the ones already listed', () => {
    let d = addRoot(draft(config), { path: '../otras', available: true })
    d = addRoot(d, { path: '../musica-ordenar', available: true })
    expect(body(d).roots).toEqual(['../musica', '../musica-ordenar', '../otras'])
  })
})

describe('changed', () => {
  it('sees each setting', () => {
    const d = draft(config)
    expect(changed(config, addRoot(d, { path: '../otras', available: true }))).toBe(true)
    expect(changed(config, { ...d, token: '' })).toBe(true)
    expect(changed(config, { ...d, ignore: '*.tmp' })).toBe(true)
    expect(changed(config, { ...d, normalize: false })).toBe(true)
    expect(changed(config, { ...d, lan: { ...d.lan, enabled: true } })).toBe(true)
    expect(changed(config, { ...d, lan: { ...d.lan, port: '5000' } })).toBe(true)
    expect(changed(config, { ...d, lan: { ...d.lan, pin: '123456' } })).toBe(true)
  })
  it('ignores whitespace around the ignore lines and a port typed as text', () => {
    const d = draft(config)
    expect(changed(config, { ...d, ignore: ' *.tmp \n\nSamples ' })).toBe(false)
    expect(changed(config, { ...d, lan: { ...d.lan, port: '4748' } })).toBe(false)
  })
})

describe('savedNotice', () => {
  it('says what saving makes the server do', () => {
    const d = draft(config)
    expect(savedNotice({ ...config, setupPending: true }, d)).toBe('Ajustes guardados. Se escanean las carpetas.')
    expect(savedNotice(config, addRoot(d, { path: '../otras', available: true }))).toBe('Ajustes guardados. Se escanean las carpetas.')
    expect(savedNotice(config, { ...d, ignore: '*.tmp' })).toBe('Ajustes guardados. Se escanean las carpetas.')
    expect(savedNotice(config, { ...d, token: 'nuevo' })).toBe('Ajustes guardados. Se actualizan los géneros con el token nuevo.')
    expect(savedNotice(config, { ...d, normalize: false })).toBe('Ajustes guardados.')
    expect(savedNotice(config, { ...d, lan: { ...d.lan, enabled: true } })).toBe('Ajustes guardados.')
  })
})

describe('pinProblem', () => {
  it('asks for a PIN when the network is on and none is saved', () => {
    expect(pinProblem('', false, true)).toBe('Elegí un PIN para usar Tordo desde la red.')
    expect(pinProblem('', true, true)).toBe('')
    expect(pinProblem('', false, false)).toBe('')
  })
  it('wants 6 to 12 digits', () => {
    const msg = 'El PIN tiene que tener entre 6 y 12 dígitos, solo números.'
    expect(pinProblem('12345', true, true)).toBe(msg)
    expect(pinProblem('1234567890123', false, false)).toBe(msg)
    expect(pinProblem('12345a', true, true)).toBe(msg)
    expect(pinProblem('123456', false, true)).toBe('')
    expect(pinProblem('123456789012', false, true)).toBe('')
  })
})

describe('portProblem', () => {
  it('wants a port from 1024 to 65535', () => {
    const msg = 'El puerto tiene que estar entre 1024 y 65535.'
    expect(portProblem(4748)).toBe('')
    expect(portProblem('1024')).toBe('')
    expect(portProblem(65535)).toBe('')
    expect(portProblem(1023)).toBe(msg)
    expect(portProblem(65536)).toBe(msg)
    expect(portProblem('')).toBe(msg)
    expect(portProblem('abc')).toBe(msg)
    expect(portProblem('80.5')).toBe(msg)
  })
})
