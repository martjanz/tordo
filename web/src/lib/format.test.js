import { describe, expect, it } from 'vitest'
import { codecName, count, country, duration, folderName, formatName, length, quality, size, years } from './format.js'

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

describe('count', () => {
  it('agrees in number', () => {
    expect(count(1, 'pista', 'pistas')).toBe('1 pista')
    expect(count(12, 'pista', 'pistas')).toBe('12 pistas')
    expect(count(0, 'álbum', 'álbumes')).toBe('0 álbumes')
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
