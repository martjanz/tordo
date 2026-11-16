import { describe, expect, it } from 'vitest'
import { albumMeta, albumYear } from './album.js'

const flac = { codec: 'flac', lossless: true, bitDepth: 16, sampleRate: 44100, playable: true }
const mp3 = { codec: 'mp3', bitrate: 320, playable: true }

// text reads the meta line as the page shows it.
const text = (groups) => groups.map((g) => g.map((i) => i.label).join(', ')).join(' · ')

describe('album header', () => {
  it('reads the original year, with the edition when it differs', () => {
    expect(albumYear({ originalYear: 1986, year: 2007 })).toBe('1986 (edición 2007)')
    expect(albumYear({ originalYear: 1986, year: 1986 })).toBe('1986')
    expect(albumYear({ year: 2001 })).toBe('2001')
    expect(albumYear({ originalYear: 0, year: 0 })).toBe('')
  })

  it('puts facts, country, genres and formats in one line', () => {
    const album = {
      originalYear: 1986,
      year: 1986,
      durationMs: 41 * 60000,
      country: 'AR',
      genres: [{ genre: 'Rock', style: 'New Wave' }],
      tracks: [flac, flac, { ...mp3, playable: false }],
    }
    const groups = albumMeta(album)
    expect(text(groups)).toBe('1986 · 3 pistas · 41 min · Argentina · Rock, New Wave · FLAC 16/44,1 · MP3 320 · 1 pista necesita ffmpeg')
    expect(groups[3][0].href).toBe('/explorar?pais=AR')
    expect(groups[4].map((i) => i.href)).toEqual(['/explorar?genero=Rock', '/explorar?genero=Rock&estilo=New+Wave'])
    expect(groups[0][0].href).toBeUndefined()
  })

  it('leaves out what an album does not have', () => {
    const groups = albumMeta({ durationMs: 60000, country: '', genres: [], tracks: [mp3] })
    expect(text(groups)).toBe('1 pista · 1 min · MP3 320')
  })
})
