import { describe, expect, it } from 'vitest'
import { isHelpKey, SECTIONS } from './ayuda.js'
import { resolve } from './router.js'

const key = (over = {}) => ({ key: '?', ctrlKey: false, metaKey: false, altKey: false, target: { tagName: 'BODY' }, ...over })

describe('isHelpKey', () => {
  it('accepts a plain ?', () => {
    expect(isHelpKey(key())).toBe(true)
    expect(isHelpKey(key({ shiftKey: true }))).toBe(true) // ? is Shift+/ on most layouts
  })
  it('ignores other keys and modifiers', () => {
    expect(isHelpKey(key({ key: '/' }))).toBe(false)
    expect(isHelpKey(key({ ctrlKey: true }))).toBe(false)
    expect(isHelpKey(key({ metaKey: true }))).toBe(false)
    expect(isHelpKey(key({ altKey: true }))).toBe(false)
  })
  it('ignores typing', () => {
    for (const tagName of ['INPUT', 'TEXTAREA', 'SELECT']) expect(isHelpKey(key({ target: { tagName } }))).toBe(false)
    expect(isHelpKey(key({ target: { tagName: 'DIV', isContentEditable: true } }))).toBe(false)
  })
  it('works without a target', () => {
    expect(isHelpKey({ key: '?' })).toBe(true)
  })
})

describe('SECTIONS', () => {
  it('has unique ids and a title on every section', () => {
    const ids = SECTIONS.map((s) => s.id)
    expect(new Set(ids).size).toBe(ids.length)
    for (const s of SECTIONS) {
      expect(s.id).toMatch(/^[a-z-]+$/)
      expect(s.title).toBeTruthy()
      expect(Array.isArray(s.items)).toBe(true)
      expect(s.items.length).toBeGreaterThan(0)
    }
  })
  it('has a term or a text on every item', () => {
    for (const s of SECTIONS)
      for (const i of s.items) {
        expect(typeof i.text).toBe('string')
        expect(i.text).toBeTruthy()
        if (i.term !== undefined) expect(i.term).toBeTruthy()
        if (i.keys !== undefined) expect(i.keys.every((k) => typeof k === 'string' && k)).toBe(true)
      }
  })
  it('links only to pages that exist', () => {
    const links = SECTIONS.flatMap((s) => s.items).filter((i) => i.href)
    expect(links.length).toBeGreaterThan(0)
    for (const { href } of links) {
      const url = new URL(href, 'http://x')
      expect(resolve(url.pathname, url.search).page, href).not.toBe('notfound')
    }
  })
  it('marks only Revisar as blocked in listen mode', () => {
    const blocked = SECTIONS.flatMap((s) => s.items).filter((i) => i.blockedInListen)
    expect(blocked.map((i) => i.href)).toEqual(['/revisar'])
  })
})
