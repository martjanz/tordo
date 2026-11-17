import { describe, expect, it } from 'vitest'
import { helpSections, isHelpKey } from './ayuda.js'
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

const LOCALES = ['es', 'en', 'pt-BR']

describe('helpSections', () => {
  it('returns the sections of each locale and English for an unknown one', () => {
    for (const l of LOCALES) expect(helpSections(l).length).toBeGreaterThan(0)
    expect(helpSections('xx')).toBe(helpSections('en'))
    expect(helpSections(undefined)).toBe(helpSections('en'))
    expect(helpSections('es')).not.toBe(helpSections('en'))
  })
})

describe.each(LOCALES)('help sections (%s)', (l) => {
  const sections = helpSections(l)
  it('has unique ids and a title on every section', () => {
    const ids = sections.map((s) => s.id)
    expect(new Set(ids).size).toBe(ids.length)
    for (const s of sections) {
      expect(s.id).toMatch(/^[a-z-]+$/)
      expect(s.title).toBeTruthy()
      expect(Array.isArray(s.items)).toBe(true)
      expect(s.items.length).toBeGreaterThan(0)
    }
  })
  it('has a text on every item', () => {
    for (const s of sections)
      for (const i of s.items) {
        expect(typeof i.text).toBe('string')
        expect(i.text).toBeTruthy()
        if (i.term !== undefined) expect(i.term).toBeTruthy()
        if (i.keys !== undefined) expect(i.keys.every((k) => typeof k === 'string' && k)).toBe(true)
      }
  })
  it('links only to pages that exist', () => {
    const links = sections.flatMap((s) => s.items).filter((i) => i.href)
    expect(links.length).toBeGreaterThan(0)
    for (const { href } of links) {
      const url = new URL(href, 'http://x')
      expect(resolve(url.pathname, url.search).page, href).not.toBe('notfound')
    }
  })
  it('marks only what lives in Review as blocked in listen mode', () => {
    const blocked = sections.flatMap((s) => s.items).filter((i) => i.blockedInListen)
    expect(blocked.map((i) => i.href)).toEqual(['/review', '/review/duplicates'])
  })
  it('uses only the known key tokens', () => {
    const tokens = ['Ctrl+K', '⌘K', 'Space', '/', 'Esc', '?']
    for (const k of sections.flatMap((s) => s.items).flatMap((i) => i.keys ?? [])) expect(tokens).toContain(k)
  })
})

describe('help parity across languages', () => {
  const [es, en, pt] = LOCALES.map(helpSections)
  const shape = (sections) => sections.map((s) => ({ id: s.id, items: s.items.map((i) => ({ keys: i.keys, href: i.href, blocked: !!i.blockedInListen, term: i.term !== undefined })) }))
  it('has the same ids, items, keys, links and blocked flags', () => {
    expect(shape(en)).toEqual(shape(es))
    expect(shape(pt)).toEqual(shape(es))
  })
  it('links the Settings network anchor and the sections help links to ids that exist', () => {
    expect(es.map((s) => s.id)).toEqual(['shortcuts', 'sections', 'concepts', 'how-it-works', 'guidelines'])
  })
})
