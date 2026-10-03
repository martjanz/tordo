// Routes of the app. Pure: nav.svelte.js keeps the current route.
import { NAMES as FACET_NAMES } from './facets.js'

// Old links of the form "/?pais=AR" still open Explorar: "/" with any of
// these parameters is Explorar, without them it is Inicio.
const EXPLORAR_PARAMS = ['ver', 'orden', 'dir', ...FACET_NAMES]

const PERIODS = ['semana', 'mes', 'anio', 'siempre']

// resolve maps a pathname and query to the page to show and its parameters.
export function resolve(pathname, search = '') {
  const params = new URLSearchParams(search)
  if (pathname === '/') return EXPLORAR_PARAMS.some((p) => params.has(p)) ? { page: 'explorar' } : { page: 'inicio' }
  if (pathname === '/explorar') return { page: 'explorar' }
  if (pathname === '/estadisticas') {
    const period = params.get('periodo')
    return { page: 'estadisticas', period: PERIODS.includes(period) ? period : 'mes' }
  }
  if (pathname === '/carpetas') return { page: 'carpetas', path: params.get('ruta') ?? '' }
  if (pathname === '/listas') return { page: 'listas' }
  if (pathname === '/buscar') return { page: 'buscar', q: params.get('q') ?? '' }
  if (pathname === '/revisar') return { page: 'revisar', tab: 'sin-identificar' }
  if (pathname === '/revisar/generos') return { page: 'revisar', tab: 'generos' }
  let m = pathname.match(/^\/listas\/([1-9][0-9]*)$/)
  if (m) return { page: 'lista', id: Number(m[1]) }
  m = pathname.match(/^\/album\/([1-9][0-9]*)$/)
  if (m) return { page: 'album', id: Number(m[1]) }
  m = pathname.match(/^\/artista\/([^/]+)$/)
  if (m) {
    try {
      return { page: 'artista', key: decodeURIComponent(m[1]) }
    } catch {
      return { page: 'notfound' } // a malformed escape
    }
  }
  return { page: 'notfound' }
}

// appLink returns the path + query a click on a link should navigate to
// inside the app, or null when the browser should handle it (another site,
// the API or audio, a new tab, a download, modifier keys).
export function appLink(event, anchor, origin) {
  if (!anchor || event.defaultPrevented || event.button !== 0) return null
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return null
  if (anchor.target || anchor.hasAttribute('download')) return null
  const url = new URL(anchor.href, origin)
  if (url.origin !== origin) return null
  if (url.pathname.startsWith('/api/') || url.pathname.startsWith('/stream/')) return null
  return url.pathname + url.search
}

export function folderHref(path) {
  return path ? `/carpetas?ruta=${encodeURIComponent(path)}` : '/carpetas'
}

export function albumHref(id) {
  return `/album/${id}`
}

export function listHref(id) {
  return `/listas/${id}`
}

export function artistHref(key) {
  return `/artista/${encodeURIComponent(key)}`
}

// exploreHref links Explorar with some facets: exploreHref({ pais: 'AR' }).
export function exploreHref(facets = {}, view = '') {
  const params = new URLSearchParams()
  if (view && view !== 'albumes') params.set('ver', view)
  for (const [k, v] of Object.entries(facets)) if (v) params.set(k, v)
  const s = params.toString()
  return s ? `/explorar?${s}` : '/explorar'
}

export function searchHref(q) {
  return `/buscar?q=${encodeURIComponent(q.trim())}`
}

// statsHref links the statistics page; "mes" is the default period.
export function statsHref(period) {
  return period && period !== 'mes' ? `/estadisticas?periodo=${encodeURIComponent(period)}` : '/estadisticas'
}
