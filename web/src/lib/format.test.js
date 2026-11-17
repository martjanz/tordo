import { afterEach, describe, expect, it } from 'vitest'
import { setLocale, t } from './i18n/index.svelte.js'
import { artistLabel, codecName, country, duration, folderName, formatName, length, quality, size, since, years } from './format.js'

describe('durations', () => {
  it('reads track lengths and totals', () => {
    expect(duration(187000)).toBe('3:07')
    expect(duration(3723000)).toBe('1:02:03')
    expect(duration(0)).toBe('0:00')
    expect(duration(undefined)).toBe('0:00')
    expect(length(48 * 60000)).toBe('48 min')
    expect(length(72 * 60000)).toBe('1 h 12 min')
    expect(length(120 * 60000)).toBe('2 h')
  })
})

describe('quality', () => {
  it.each([
    [{ format: 'flac', codec: 'flac', lossless: true, bitDepth: 24, sampleRate: 96000 }, 'FLAC 24/96'],
    [{ format: 'wav', codec: 'pcm', lossless: true, bitDepth: 16, sampleRate: 44100 }, 'WAV 16/44,1'],
    [{ format: 'mp3', codec: 'mp3', bitrate: 320 }, 'MP3 320'],
    [{ format: 'asf', codec: 'wma', bitrate: 128 }, 'WMA 128'],
    [{ format: 'mp4', codec: 'alac', lossless: true }, 'ALAC'],
    [{ format: 'ogg', codec: 'opus', bitrate: 0 }, 'Opus'],
    [{ format: 'ape', codec: 'ape', lossless: true }, 'APE'],
  ])('%j', (t, want) => {
    expect(quality(t)).toBe(want)
  })
  it('names unknown codecs as they come', () => {
    expect(codecName({ format: 'wv', codec: 'wv2' })).toBe('WV2')
  })
})

describe('folderName', () => {
  it('takes the last part', () => {
    expect(folderName('../musica/Latinoamérica 600')).toBe('Latinoamérica 600')
    expect(folderName('../../../Music')).toBe('Music')
    expect(folderName('')).toBe('')
  })
})

describe('names', () => {
  it('names countries and formats', () => {
    expect(country('AR')).toBe('Argentina')
    expect(country('BR')).toBe('Brasil')
    expect(country('')).toBe('')
    expect(country('ZZZ')).toBe('ZZZ')
    expect(formatName('flac')).toBe('FLAC')
    expect(formatName('wma')).toBe('WMA')
    expect(formatName('aiff')).toBe('AIFF')
    expect(formatName('xyz')).toBe('XYZ')
  })
  it('reads year spans', () => {
    expect(years(1982, 2007)).toBe('1982–2007')
    expect(years(1990, 1990)).toBe('1990')
    expect(years(0, 0)).toBe('')
  })
})

describe('size', () => {
  it.each([
    [512, '512 B'], [84_000, '84 KB'], [7_200_000, '7,2 MB'], [1_400_000_000, '1,4 GB'],
    [3_000_000_000, '3,0 GB'], [410_000_000, '410 MB'], [9_960_000, '10 MB'], [999_600, '1,0 MB'],
    [30_100_000_000, '30 GB'], [0, '0 B'],
  ])('%d', (n, want) => expect(size(n)).toBe(want))
  it('is empty without a value', () => expect(size(undefined)).toBe(''))
})

describe('since', () => {
  const now = Date.UTC(2026, 9, 2, 12, 0, 0)
  it.each([
    [now - 5_000, 'recién'],
    [now - 60_000, 'hace 1 min'],
    [now - 25 * 60_000, 'hace 25 min'],
    [now - 3600_000, 'hace 1 h'],
    [now - 5 * 3600_000, 'hace 5 h'],
    [now - 86_400_000, 'ayer'],
    [now - 3 * 86_400_000, 'hace 3 días'],
    [now + 5_000, 'recién'],
  ])('%d', (ms, want) => expect(since(ms, now)).toBe(want))
})

describe('other languages', () => {
  afterEach(() => setLocale('es'))
  const now = Date.UTC(2026, 9, 2, 12, 0, 0)

  it('formats in English', () => {
    setLocale('en')
    expect(size(7_200_000)).toBe('7.2 MB')
    expect(since(now - 3 * 86_400_000, now)).toBe('3 days ago')
    expect(since(now, now)).toBe('just now')
    expect(since(now - 25 * 60_000, now)).toBe('25 min ago')
    expect(since(now - 5 * 3600_000, now)).toBe('5 h ago')
    expect(since(now - 86_400_000, now)).toBe('yesterday')
    expect(country('AR')).toBe('Argentina')
    expect(country('DE')).toBe('Germany')
    expect(length(72 * 60000)).toBe('1 h 12 min')
    expect(length(120 * 60000)).toBe('2 h')
    expect(t('count.tracks', { n: 1 })).toBe('1 track')
    expect(t('count.tracks', { n: 1234 })).toBe('1,234 tracks')
    expect(quality({ format: 'wav', codec: 'pcm', lossless: true, bitDepth: 16, sampleRate: 44100 })).toBe('WAV 16/44.1')
  })
  it('formats in Spanish', () => {
    setLocale('es')
    expect(size(7_200_000)).toBe('7,2 MB')
    expect(t('count.tracks', { n: 1 })).toBe('1 pista')
    expect(t('count.albums', { n: 0 })).toBe('0 álbumes')
    expect(t('count.tracks', { n: 1234 })).toBe('1.234 pistas')
    expect(country('DE')).toBe('Alemania')
  })
  it('formats in Portuguese', () => {
    setLocale('pt-BR')
    expect(country('DE')).toBe('Alemanha')
    expect(size(7_200_000)).toBe('7,2 MB')
    expect(since(now - 3 * 86_400_000, now)).toBe('há 3 dias')
    expect(since(now, now)).toBe('agora')
    expect(since(now - 86_400_000, now)).toBe('ontem')
  })
})

describe('artistLabel', () => {
  afterEach(() => setLocale('es'))
  it('names the compilations artist in the current language', () => {
    expect(artistLabel('Various Artists')).toBe('Varios artistas')
    setLocale('en')
    expect(artistLabel('Various Artists')).toBe('Various Artists')
    setLocale('pt-BR')
    expect(artistLabel('Various Artists')).toBe('Vários artistas')
  })
  it('leaves other names as they are', () => {
    expect(artistLabel('Charly García')).toBe('Charly García')
    expect(artistLabel('')).toBe('')
    expect(artistLabel(undefined)).toBe(undefined)
  })
})
