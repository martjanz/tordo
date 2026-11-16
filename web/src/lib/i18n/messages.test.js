import { describe, expect, it } from 'vitest'
import en from './en.js'
import es from './es.js'
import ptBR from './pt-BR.js'

const isPlural = (v) => v && typeof v === 'object' && 'other' in v
const PLURAL_KEYS = ['zero', 'one', 'two', 'few', 'many']
const looksPlural = (v) => v && typeof v === 'object' && PLURAL_KEYS.some((k) => k in v)

// flatten returns { 'a.b.c': leaf } where a leaf is a string or a plural object.
function flatten(obj, prefix = '', out = {}) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (typeof v === 'string' || isPlural(v) || looksPlural(v)) out[key] = v
    else flatten(v, key, out)
  }
  return out
}

const placeholders = (leaf) => {
  if (leaf === undefined) return '' // a missing key is reported by the keys test
  const strs = typeof leaf === 'string' ? [leaf] : Object.values(leaf)
  const set = new Set()
  for (const s of strs) for (const m of s.matchAll(/\{(\w+)\}/g)) set.add(m[1])
  return [...set].sort().join(',')
}

const files = { en: flatten(en), es: flatten(es), 'pt-BR': flatten(ptBR) }

describe('message files', () => {
  it.each(['es', 'pt-BR'])('%s has the same keys as en', (l) => {
    expect(Object.keys(files[l]).sort()).toEqual(Object.keys(files.en).sort())
  })
  it.each(['es', 'pt-BR'])('%s has the same placeholders as en', (l) => {
    for (const [k, v] of Object.entries(files.en)) {
      expect(placeholders(files[l][k]), `${l}:${k}`).toBe(placeholders(v))
    }
  })
  it.each(['en', 'es', 'pt-BR'])('%s plurals have one and other; no empty strings', (l) => {
    for (const [k, v] of Object.entries(files[l])) {
      if (typeof v === 'string') {
        expect(v, `${l}:${k}`).not.toBe('')
      } else {
        expect(typeof v.other, `${l}:${k}.other (plural objects need other)`).toBe('string')
        expect(typeof v.one, `${l}:${k}.one`).toBe('string')
        expect(typeof v.other, `${l}:${k}.other`).toBe('string')
        for (const s of Object.values(v)) expect(s, `${l}:${k}`).not.toBe('')
      }
    }
  })
})
