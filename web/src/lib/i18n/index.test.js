import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  _setMessages,
  applyLanguage,
  formatNumber,
  has,
  locale,
  msg,
  numberLocale,
  resolveLocale,
  setLocale,
  t,
  text,
} from './index.svelte.js'

describe('resolveLocale', () => {
  it.each([
    ['en', ['es-AR'], 'en'],
    ['', ['es-MX'], 'es'],
    ['', ['pt-PT', 'en'], 'pt-BR'],
    ['', ['PT'], 'pt-BR'],
    ['', ['zh-CN'], 'en'],
    ['', [], 'en'],
    ['', undefined, 'en'],
    ['fr', ['es'], 'es'],
  ])('(%j, %j) -> %s', (setting, langs, want) => {
    expect(resolveLocale(setting, langs)).toBe(want)
  })
})

describe('t', () => {
  let restore
  beforeEach(() => {
    restore = _setMessages({
      en: {
        greet: '{name} ok',
        tracks: { one: '{n} track', other: '{n} tracks' },
        year: '{year}',
        onlyEn: 'English only',
        a: { b: 'deep' },
      },
      es: {
        greet: '{name} listo',
        tracks: { one: '{n} pista', other: '{n} pistas' },
        year: '{year}',
      },
      'pt-BR': {
        tracks: { one: '{n} faixa', other: '{n} faixas' },
      },
    })
  })
  afterEach(() => {
    restore()
    setLocale('es')
  })

  it('substitutes params', () => {
    setLocale('en')
    expect(t('greet', { name: 'Bob' })).toBe('Bob ok')
  })
  it('picks plurals and formats n', () => {
    setLocale('en')
    expect(t('tracks', { n: 1 })).toBe('1 track')
    expect(t('tracks', { n: 2 })).toBe('2 tracks')
    expect(t('tracks', { n: 1234 })).toBe('1,234 tracks')
    setLocale('es')
    expect(t('tracks', { n: 1234 })).toBe('1.234 pistas')
  })
  it('keeps other numeric params unformatted', () => {
    setLocale('en')
    expect(t('year', { year: 1985 })).toBe('1985')
  })
  it('pt-BR uses one for zero', () => {
    setLocale('pt-BR')
    expect(t('tracks', { n: 0 })).toBe('0 faixa')
  })
  it('falls back to English, then the key', () => {
    setLocale('es')
    expect(t('onlyEn')).toBe('English only')
    expect(t('nope.nothing')).toBe('nope.nothing')
    expect(t('a.b')).toBe('deep')
  })
  it('has', () => {
    setLocale('es')
    expect(has('onlyEn')).toBe(true)
    expect(has('tracks')).toBe(true)
    expect(has('nope')).toBe(false)
    expect(has('a')).toBe(false)
  })
  it('text and msg', () => {
    setLocale('en')
    expect(text('plain')).toBe('plain')
    expect(text(msg('a.b'))).toBe('deep')
    expect(text(msg('greet', { name: 'X' }))).toBe('X ok')
  })
  it('does not throw on odd params', () => {
    setLocale('en')
    expect(t('tracks')).toBe('{n} tracks')
  })
})

describe('locale', () => {
  afterEach(() => setLocale('es'))
  it('numberLocale follows the locale', () => {
    setLocale('es')
    expect(numberLocale()).toBe('es-AR')
    expect(locale()).toBe('es')
    setLocale('en')
    expect(numberLocale()).toBe('en')
    setLocale('pt-BR')
    expect(numberLocale()).toBe('pt-BR')
  })
  it('formatNumber', () => {
    setLocale('en')
    expect(formatNumber(1234567)).toBe('1,234,567')
    setLocale('es')
    expect(formatNumber(1234567)).toBe('1.234.567')
  })
  it('ignores unknown locales', () => {
    setLocale('xx')
    expect(locale()).toBe('es')
  })
})

describe('applyLanguage', () => {
  afterEach(() => {
    setLocale('es')
    delete globalThis.document
    applyLanguage('es')
  })

  it('sets the locale and the document language', () => {
    globalThis.document = { documentElement: {} }
    applyLanguage('pt-BR')
    expect(locale()).toBe('pt-BR')
    expect(document.documentElement.lang).toBe('pt-BR')
  })
  it('does nothing when the same setting comes again', () => {
    applyLanguage('en')
    setLocale('es')
    applyLanguage('en')
    expect(locale()).toBe('es')
  })
  it('resolves automatic from the browser and does not throw without document', () => {
    delete globalThis.document
    applyLanguage('')
    expect(['es', 'en', 'pt-BR']).toContain(locale())
  })
})
