import { describe, expect, it } from 'vitest'
import { edges, pageStep, widePage } from './shelf.js'

describe('edges', () => {
  it('a row that fits has nothing past either edge', () => {
    expect(edges({ scrollLeft: 0, clientWidth: 1000, scrollWidth: 1000 })).toEqual({ start: false, end: false })
  })

  it('at the start, more past the end only', () => {
    expect(edges({ scrollLeft: 0, clientWidth: 1000, scrollWidth: 3000 })).toEqual({ start: false, end: true })
  })

  it('in the middle, more on both sides', () => {
    expect(edges({ scrollLeft: 800, clientWidth: 1000, scrollWidth: 3000 })).toEqual({ start: true, end: true })
  })

  it('at the end, more before the start only', () => {
    expect(edges({ scrollLeft: 2000, clientWidth: 1000, scrollWidth: 3000 })).toEqual({ start: true, end: false })
  })

  it('ignores the sub-pixel leftovers of a zoomed or snapped scroll', () => {
    expect(edges({ scrollLeft: 0.6, clientWidth: 1000, scrollWidth: 3000 }).start).toBe(false)
    expect(edges({ scrollLeft: 1999.4, clientWidth: 1000, scrollWidth: 3000 }).end).toBe(false)
  })

  it('reads a right-to-left scroll (negative scrollLeft) the same way', () => {
    expect(edges({ scrollLeft: -800, clientWidth: 1000, scrollWidth: 3000 })).toEqual({ start: true, end: true })
  })
})

describe('pageStep', () => {
  it('pages by whole cells, keeping one in view', () => {
    // 1000px show 5 cells of 168 + 16; a page moves 4 of them.
    expect(pageStep(1000, 184)).toBe(4 * 184)
  })

  it('moves at least one cell on a narrow row', () => {
    expect(pageStep(300, 184)).toBe(184)
  })

  it('falls back to most of the width when the cell size is unknown', () => {
    expect(pageStep(1000, 0)).toBe(800)
  })
})

describe('widePage', () => {
  it('cover pages use the whole screen', () => {
    expect(widePage('inicio')).toBe(true)
    expect(widePage('explorar')).toBe(true)
  })

  it('text pages keep a readable width', () => {
    for (const p of ['album', 'ajustes', 'ayuda', 'estadisticas', 'carpetas', 'listas', 'lista', 'artista', 'notfound', '']) {
      expect(widePage(p)).toBe(false)
    }
  })
})
