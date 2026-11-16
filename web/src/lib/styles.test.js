import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// The type scale lives in app.css: components pick a size from it and never
// write their own, so a screen keeps to a few sizes.

const src = join(import.meta.dirname, '..')

function svelteFiles(dir) {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = join(dir, e.name)
    if (e.isDirectory()) return svelteFiles(p)
    return e.name.endsWith('.svelte') ? [p] : []
  })
}

const files = svelteFiles(src).map((p) => ({ name: p.slice(src.length + 1), text: readFileSync(p, 'utf8') }))
const appCss = readFileSync(join(src, 'app.css'), 'utf8')

// font-size: 13px, font: 600 13px/1.2 ..., font-size: clamp(12px, ...)
const literal = /\bfont(?:-size)?\s*:[^;}]*\d(?:px|rem|em|pt)\b/g

describe('type scale', () => {
  it('components take their font sizes from app.css', () => {
    const found = files.flatMap((f) => (f.text.match(literal) ?? []).map((m) => `${f.name}: ${m}`))
    expect(found).toEqual([])
  })

  it('every size a component uses is defined', () => {
    const defined = new Set([...appCss.matchAll(/(--fs-[\w-]+)\s*:/g)].map((m) => m[1]))
    const used = files.flatMap((f) => [...f.text.matchAll(/var\((--fs-[\w-]+)\)/g)].map((m) => `${f.name}: ${m[1]}`))
    expect(used.filter((u) => !defined.has(u.split(': ')[1]))).toEqual([])
  })

  it('app.css itself sets sizes only through the scale', () => {
    expect(appCss.match(literal) ?? []).toEqual([])
  })

  it('has no size under 12px', () => {
    const values = [...appCss.matchAll(/--fs-[\w-]+\s*:\s*([^;]+);/g)].map((m) => m[1])
    expect(values.length).toBeGreaterThan(0)
    // A clamp() is never smaller than its first argument.
    const smallest = values.map((v) => Number(v.replace(/^clamp\(/, '').match(/^(\d+)px/)?.[1]))
    expect(smallest.filter((n) => !(n >= 12))).toEqual([])
  })
})
