import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// Fails when user-visible Spanish is left in the code instead of the message
// files: any accented Spanish letter or inverted mark in a string literal, a
// template or markup text. Comments do not count.
const SPANISH = /[áéíóúñ¿¡ÁÉÍÓÚÑ]/

const SRC = fileURLToPath(new URL('../../', import.meta.url))

// PENDING lists the files not migrated yet, one path per line. The list can
// only shrink: a file listed here that no longer has Spanish text fails.
const PENDING = [
  'components/ArbolCarpetas.svelte',
  'components/AudioDeEstaCompu.svelte',
  'components/BarraFacetas.svelte',
  'components/ContenidoCarpeta.svelte',
  'components/Dispositivos.svelte',
  'components/Duplicados.svelte',
  'components/GenerosPorMapear.svelte',
  'components/Identificar.svelte',
  'components/ListaCarpeta.svelte',
  'components/Raices.svelte',
  'components/RedLocal.svelte',
  'components/SinIdentificar.svelte',
  'components/TokenDiscogs.svelte',
  'lib/ayuda.js',
  'pages/Ajustes.svelte',
  'pages/Album.svelte',
  'pages/Artista.svelte',
  'pages/Ayuda.svelte',
  'pages/Bienvenida.svelte',
  'pages/Buscar.svelte',
  'pages/Carpetas.svelte',
  'pages/Entrar.svelte',
  'pages/Estadisticas.svelte',
  'pages/Explorar.svelte',
  'pages/Inicio.svelte',
  'pages/Lista.svelte',
  'pages/Listas.svelte',
  'pages/Revisar.svelte',
]

// ALLOW lists the accepted exceptions: a line of the file that contains text.
// Each one needs a reason.
const ALLOW = [
  // { file: 'lib/example.js', text: 'Latinoamérica', reason: 'a proper name, not interface text' },
]

// stripJS removes // and /* */ comments from code, leaving strings, templates
// and regular expressions alone.
export function stripJS(code) {
  let out = ''
  let i = 0
  while (i < code.length) {
    const c = code[i]
    const next = code[i + 1]
    if (c === '/' && next === '/') {
      while (i < code.length && code[i] !== '\n') i++
    } else if (c === '/' && next === '*') {
      const end = code.indexOf('*/', i + 2)
      const stop = end < 0 ? code.length : end + 2
      out += code.slice(i, stop).replace(/[^\n]/g, '')
      i = stop
    } else if (c === "'" || c === '"' || c === '`') {
      let j = i + 1
      while (j < code.length && code[j] !== c) {
        if (code[j] === '\\') j++
        else if (c !== '`' && code[j] === '\n') break
        j++
      }
      out += code.slice(i, j + 1)
      i = j + 1
    } else {
      out += c
      i++
    }
  }
  return out
}

// stripSvelte keeps what a component shows: its script without comments and
// its markup without HTML comments (styles are dropped).
export function stripSvelte(source) {
  const noStyle = source.replace(/<style[\s\S]*?<\/style>/g, '')
  const withScripts = noStyle.replace(/(<script[^>]*>)([\s\S]*?)(<\/script>)/g, (_, open, body, close) => open + stripJS(body) + close)
  return withScripts.replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ''))
}

function sources(dir = SRC, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    const rel = path.relative(SRC, full).split(path.sep).join('/')
    if (entry.isDirectory()) {
      if (rel !== 'lib/i18n') sources(full, out)
    } else if (/\.(js|svelte)$/.test(entry.name) && !entry.name.endsWith('.test.js') && entry.name !== 'test-setup.js') {
      out.push(rel)
    }
  }
  return out
}

// spanishLines returns the lines of a source file that hold Spanish text and
// are not allowed.
function spanishLines(rel) {
  const source = fs.readFileSync(path.join(SRC, rel), 'utf8')
  const code = rel.endsWith('.svelte') ? stripSvelte(source) : stripJS(source)
  const allowed = ALLOW.filter((a) => a.file === rel)
  return code
    .split('\n')
    .map((text, i) => ({ line: i + 1, text: text.trim() }))
    .filter((l) => SPANISH.test(l.text) && !allowed.some((a) => l.text.includes(a.text)))
}

describe('the comment stripper', () => {
  it('drops comments but keeps strings with comment marks', () => {
    expect(stripJS("const a = 'x' // año\n/* ñ */ const b = 'https://x.org/é'")).toBe("const a = 'x' \n const b = 'https://x.org/é'")
  })
  it('drops HTML comments and styles from components', () => {
    const out = stripSvelte('<!-- ñ -->\n<p>hola</p>\n<style>/* é */ p { color: red }</style>\n<script>\n  // á\n  let a = "í"\n</script>')
    expect(out).not.toMatch(/[ñéá]/)
    expect(out).toContain('"í"')
  })
})

describe('leftover Spanish', () => {
  const files = sources()

  it('lists existing files only', () => {
    for (const rel of PENDING) expect(files, rel).toContain(rel)
  })

  it('is not in any file that was migrated', () => {
    const found = files.filter((rel) => !PENDING.includes(rel)).flatMap((rel) => spanishLines(rel).map((l) => `${rel}:${l.line}: ${l.text}`))
    expect(found).toEqual([])
  })

  it('is still in every pending file (the list only shrinks)', () => {
    const clean = PENDING.filter((rel) => files.includes(rel) && spanishLines(rel).length === 0)
    expect(clean, 'remove these files from PENDING').toEqual([])
  })

  it('has no stale exceptions', () => {
    for (const a of ALLOW) {
      expect(a.reason, `${a.file}: reason`).toBeTruthy()
      const source = fs.readFileSync(path.join(SRC, a.file), 'utf8')
      expect(source, `${a.file}: ${a.text}`).toContain(a.text)
    }
  })
})
