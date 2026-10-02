import { describe, expect, it } from 'vitest'
import { enterHref, isShortcut, move, options, searchable, total, trackHref } from './search.js'

const result = {
  artists: [{ key: 'charly-garcia', name: 'Charly García' }],
  albums: [{ id: 3, title: 'Clics modernos' }],
  tracks: [{ id: 9, albumId: 3 }, { id: 10, albumId: 0, dir: '../musica/youtube' }],
  folders: [{ path: '../musica/Latinoamérica 600' }],
  totals: { artists: 1, albums: 4, tracks: 2, folders: 1 },
}

describe('search', () => {
  it('asks only with enough letters or digits', () => {
    expect(searchable('so')).toBe(true)
    expect(searchable('s!')).toBe(false)
    expect(searchable('  ')).toBe(false)
    expect(searchable(undefined)).toBe(false)
  })
  it('lists options in keyboard order', () => {
    expect(options(result).map((o) => o.href)).toEqual([
      '/artista/charly-garcia',
      '/album/3',
      '/album/3',
      '/carpetas?ruta=..%2Fmusica%2Fyoutube',
      '/carpetas?ruta=..%2Fmusica%2FLatinoam%C3%A9rica%20600',
    ])
    expect(options(null)).toEqual([])
    expect(total(result)).toBe(8)
    expect(trackHref(result.tracks[1])).toBe('/carpetas?ruta=..%2Fmusica%2Fyoutube')
  })
  it('moves the selection and follows Enter', () => {
    expect(move(-1, 'ArrowDown', 3)).toBe(0)
    expect(move(2, 'ArrowDown', 3)).toBe(-1)
    expect(move(-1, 'ArrowUp', 3)).toBe(2)
    expect(move(0, 'ArrowDown', 0)).toBe(-1)
    const opts = options(result)
    expect(enterHref(1, opts, 'cl')).toBe('/album/3')
    expect(enterHref(-1, opts, 'clics')).toBe('/buscar?q=clics')
    expect(enterHref(-1, opts, 'c')).toBeNull()
  })
  it('opens with Ctrl+K or ⌘K', () => {
    expect(isShortcut({ ctrlKey: true, key: 'k' })).toBe(true)
    expect(isShortcut({ metaKey: true, key: 'K' })).toBe(true)
    expect(isShortcut({ ctrlKey: true, shiftKey: true, key: 'k' })).toBe(false)
    expect(isShortcut({ key: 'k' })).toBe(false)
  })
})
