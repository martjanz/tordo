import { describe, expect, it } from 'vitest'
import { KINDS, fileQuality, groupLine } from './dupes.js'

describe('fileQuality', () => {
  it('reads lossless files with bit depth and sample rate', () => {
    expect(fileQuality({ format: 'flac', lossless: true, bitDepth: 24, sampleRate: 96000 })).toBe('FLAC 24 bits / 96 kHz')
    expect(fileQuality({ format: 'flac', lossless: true, bitDepth: 16, sampleRate: 44100 })).toBe('FLAC 16 bits / 44,1 kHz')
    expect(fileQuality({ format: 'alac', lossless: true, bitDepth: 16, sampleRate: 44100 })).toBe('ALAC 16 bits / 44,1 kHz')
  })
  it('leaves out what is unknown', () => {
    expect(fileQuality({ format: 'flac', lossless: true })).toBe('FLAC')
    expect(fileQuality({ format: 'flac', lossless: true, bitDepth: 16 })).toBe('FLAC 16 bits')
    expect(fileQuality({ format: 'flac', lossless: true, sampleRate: 48000 })).toBe('FLAC 48 kHz')
  })
  it('reads lossy files with their bitrate', () => {
    expect(fileQuality({ format: 'mp3', lossless: false, bitrate: 320 })).toBe('MP3 320 kbps')
    expect(fileQuality({ format: 'wma', lossless: false, bitrate: 128 })).toBe('WMA 128 kbps')
    expect(fileQuality({ format: 'mp3', lossless: false })).toBe('MP3')
  })
})

describe('groupLine', () => {
  it('counts files and size, and the recoverable space of exact copies', () => {
    const files = [{}, {}, {}]
    expect(groupLine({ kind: 'exacta', files, sizeBytes: 21_000_000, recoverable: 14_000_000 })).toBe('3 archivos · 21 MB · 14 MB recuperables')
    expect(groupLine({ kind: 'formatos', files: files.slice(0, 2), sizeBytes: 9_000_000, recoverable: 0 })).toBe('2 archivos · 9,0 MB')
  })
})

describe('KINDS', () => {
  it('lists the kinds with the key of their count', () => {
    expect(KINDS.map((k) => [k.kind, k.count])).toEqual([
      ['exactas', 'exact'],
      ['formatos', 'formats'],
      ['sospechas', 'suspect'],
    ])
  })
})
