import { describe, expect, it } from 'vitest'
import {
  hashName,
  initials,
  placeholderBackground,
  TITLE_HEIGHT,
  TITLE_LEADING,
  TITLE_MAX,
  TITLE_MIN,
  TITLE_WIDTH,
  titleLines,
  titleSize,
  textWidth,
} from './placeholder.js'

describe('hashName', () => {
  it('is deterministic and spreads names apart', () => {
    expect(hashName('Abbey Road')).toBe(hashName('Abbey Road'))
    expect(hashName('Abbey Road')).not.toBe(hashName('Let It Be'))
    expect(hashName('')).toBe(hashName(undefined))
  })

  it('ignores case and surrounding spaces', () => {
    expect(hashName('  Abbey Road ')).toBe(hashName('abbey road'))
  })

  it('is a non-negative integer', () => {
    for (const n of ['', 'a', 'Soda Stereo', 'Ñandú', '🎵']) {
      const h = hashName(n)
      expect(Number.isInteger(h)).toBe(true)
      expect(h).toBeGreaterThanOrEqual(0)
    }
  })
})

describe('placeholderBackground', () => {
  it('is a muted gradient picked from the name', () => {
    const bg = placeholderBackground('Abbey Road')
    expect(bg).toMatch(/^linear-gradient\(\d+deg, hsl\(\d+ \d+% \d+%\), hsl\(\d+ \d+% \d+%\)\)$/)
    expect(bg).toBe(placeholderBackground('Abbey Road'))
    expect(bg).not.toBe(placeholderBackground('Let It Be'))
  })

  it('stays dark and low in saturation, so white text reads on it', () => {
    for (const n of ['a', 'b', 'Soda Stereo', 'Charly García', 'Kind of Blue', '1999']) {
      const parts = [...placeholderBackground(n).matchAll(/hsl\((\d+) (\d+)% (\d+)%\)/g)]
      expect(parts.length).toBe(2)
      for (const [, h, s, l] of parts) {
        expect(Number(h)).toBeLessThan(360)
        expect(Number(s)).toBeLessThanOrEqual(40)
        expect(Number(l)).toBeLessThanOrEqual(34)
      }
    }
  })

  it('gives different hues to many names', () => {
    const names = ['Abbey Road', 'Let It Be', 'Revolver', 'Help!', 'Rubber Soul', 'Kind of Blue', 'Blue Train', 'Bitches Brew']
    const hues = new Set(names.map((n) => placeholderBackground(n).match(/hsl\((\d+)/)[1]))
    expect(hues.size).toBeGreaterThanOrEqual(6)
  })
})

describe('initials', () => {
  it('takes the first letters of the first and last words', () => {
    expect(initials('Charly García')).toBe('CG')
    expect(initials('Los Fabulosos Cadillacs')).toBe('LC')
    expect(initials('Soda')).toBe('S')
  })

  it('skips punctuation and keeps accents and digits', () => {
    expect(initials('(What’s the Story) Morning Glory?')).toBe('WG')
    expect(initials('Ñandú azul')).toBe('ÑA')
    expect(initials('1999')).toBe('1')
    expect(initials('  ')).toBe('')
    expect(initials(undefined)).toBe('')
  })

  it('upper-cases', () => {
    expect(initials('el mató a un policía motorizado')).toBe('EM')
  })
})

describe('textWidth', () => {
  it('measures capitals, wide and narrow, accents as their letter', () => {
    expect(textWidth('WM')).toBeGreaterThan(textWidth('IL') * 2)
    expect(textWidth('religión')).toBe(textWidth('RELIGION'))
    expect(textWidth('')).toBe(0)
    expect(textWidth('★')).toBeGreaterThan(0)
  })
})

describe('titleSize', () => {
  // Sizes are shares of the picture's width (cqi): a line has TITLE_WIDTH
  // to fill and the block TITLE_HEIGHT to stand in, at TITLE_LEADING.
  const widest = (name) => Math.max(...name.split(/\s+/).map(textWidth))

  it('sets very short titles at the largest size', () => {
    expect(titleSize('Relax')).toBe(TITLE_MAX)
    expect(titleSize('')).toBe(TITLE_MAX)
    expect(titleSize(undefined)).toBe(TITLE_MAX)
  })

  it('sets a title like the mock-up at about 15% of the width', () => {
    const size = titleSize('Parte de la religión')
    expect(size).toBeGreaterThanOrEqual(14)
    expect(size).toBeLessThanOrEqual(17)
    // Three lines; CSS balances them as PARTE / DE LA / RELIGIÓN.
    expect(titleLines('Parte de la religión', size)).toHaveLength(3)
  })

  it('shrinks as the title grows', () => {
    const names = ['Relax', 'Piano bar', 'Parte de la religión', '(What’s the Story) Morning Glory?', 'Sgt. Pepper’s Lonely Hearts Club Band']
    const sizes = names.map(titleSize)
    for (let i = 1; i < sizes.length; i++) expect(sizes[i]).toBeLessThanOrEqual(sizes[i - 1])
    expect(sizes.at(-1)).toBeLessThan(sizes[0] * 0.6)
  })

  it('keeps every word whole on its line and the block within its height', () => {
    for (const n of ['Locura', 'Artaud', 'Kamikaze', 'Clics modernos', 'Wham!', 'Toda la colección', 'Desenchufado', '(What’s the Story) Morning Glory?']) {
      const size = titleSize(n)
      expect(size * widest(n)).toBeLessThanOrEqual(TITLE_WIDTH + 0.01)
      expect(titleLines(n, size).length * size * TITLE_LEADING).toBeLessThanOrEqual(TITLE_HEIGHT + 0.01)
    }
  })

  it('never goes under the floor', () => {
    expect(titleSize('Supercalifragilisticexpialidocious')).toBe(TITLE_MIN)
    expect(titleSize('a '.repeat(80))).toBe(TITLE_MIN)
  })
})
