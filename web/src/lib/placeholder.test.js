import { describe, expect, it } from 'vitest'
import { hashName, initials, placeholderBackground, titleSize } from './placeholder.js'

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

describe('titleSize', () => {
  it('sets short titles big, one word per line', () => {
    expect(titleSize('Signos')).toBe('short')
    expect(titleSize('Piano bar')).toBe('short')
    expect(titleSize('Toda la colección')).toBe('medium') // "colección" is 9 letters
  })

  it('steps down for long words and many words', () => {
    expect(titleSize('Clics modernos')).toBe('medium')
    expect(titleSize('Los años de la peste')).toBe('long')
    expect(titleSize('(What’s the Story) Morning Glory?')).toBe('long')
  })

  it('takes an empty name', () => {
    expect(titleSize('')).toBe('short')
    expect(titleSize(undefined)).toBe('short')
  })
})
