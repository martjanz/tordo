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

// Form controls look the same everywhere: app.css gives text fields, selects,
// textareas and checkboxes their dark look, and components only size and place
// them. The player's range sliders are drawn by Reproductor itself.

// Each CSS rule in a component's <style>: [selector, body].
function rules(text) {
  const style = text.match(/<style>([\s\S]*?)<\/style>/)?.[1] ?? ''
  return [...style.replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => [m[1].trim(), m[2]])
}

// A selector that targets an input, select or textarea element.
const control = /(?:^|[\s,>+~(])(?:input|select|textarea)\b/
const look = /(?:^|;|\s)(?:background(?:-color)?|border(?:-color|-style|-width)?|border-radius|appearance|accent-color)\s*:/

describe('form controls', () => {
  it('app.css styles text fields, selects and textareas', () => {
    for (const sel of ["[type='search']", "[type='password']", "[type='number']", 'select', 'textarea']) {
      expect(appCss).toContain(sel)
    }
    expect(appCss).toMatch(/::placeholder\s*\{[^}]*var\(--faint\)/)
  })

  it('selects draw their own chevron', () => {
    expect(appCss).toMatch(/select\s*\{[^}]*appearance:\s*none/)
    expect(appCss).toMatch(/select\s*\{[^}]*background-image:\s*url\("data:image\/svg\+xml/)
  })

  it('checkboxes and radios take the accent', () => {
    expect(appCss).toMatch(/\[type='checkbox'\][^{]*\{[^}]*accent-color:\s*var\(--accent\)/)
    expect(appCss).toMatch(/\[type='radio'\]/)
  })

  it('components do not restyle how a control looks', () => {
    const found = files.flatMap((f) =>
      rules(f.text)
        .filter(([sel, body]) => control.test(sel) && !sel.includes("[type='range']") && look.test(body))
        .map(([sel]) => `${f.name}: ${sel}`),
    )
    expect(found).toEqual([])
  })
})

describe('checkbox rows', () => {
  it('app.css owns the .check row, not each component', () => {
    expect(appCss).toMatch(/\.check\s*\{/)
    const found = files.flatMap((f) => rules(f.text).filter(([sel]) => /(?:^|[\s,])\.check\b/.test(sel)).map(([sel]) => `${f.name}: ${sel}`))
    expect(found).toEqual([])
  })
})
