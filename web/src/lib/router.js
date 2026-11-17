// Routes of the app. Pure: nav.svelte.js keeps the current route.
import { NAMES as FACET_NAMES } from './facets.js'

// Links of the form "/?country=AR" open Explorar: "/" with any of
// these parameters is Explorar, without them it is Inicio.
const EXPLORAR_PARAMS = ['view', 'order', 'dir', ...FACET_NAMES]

const DUPLICATE_KINDS = ['exact', 'formats', 'suspected']

const PERIODS = ['week', 'month', 'year', 'all']

// resolve maps a pathname and query to the page to show and its parameters.
export function resolve(pathname, search = '') {
  const params = new URLSearchParams(search)
  if (pathname === '/') return EXPLORAR_PARAMS.some((p) => params.has(p)) ? { page: 'explorar' } : { page: 'inicio' }
  if (pathname === '/explore') return { page: 'explorar' }
  if (pathname === '/stats') {
    const period = params.get('period')
    return { page: 'estadisticas', period: PERIODS.includes(period) ? period : 'month' }
  }
  if (pathname === '/folders') return { page: 'carpetas', path: params.get('path') ?? '' }
  if (pathname === '/lists') return { page: 'listas' }
  if (pathname === '/search') return { page: 'buscar', q: params.get('q') ?? '' }
  if (pathname === '/review') return { page: 'revisar', tab: 'sin-identificar' }
  if (pathname === '/settings') return { page: 'ajustes' }
  if (pathname === '/help') return { page: 'ayuda' }
  if (pathname === '/welcome') return { page: 'bienvenida' }
  if (pathname === '/review/duplicates') {
    const kind = params.get('kind')
    return { page: 'revisar', tab: 'duplicados', kind: DUPLICATE_KINDS.includes(kind) ? kind : 'exact' }
  }
  if (pathname === '/review/formats') return { page: 'revisar', tab: 'formatos' }
  if (pathname === '/review/genres') return { page: 'revisar', tab: 'generos' }
  let m = pathname.match(/^\/lists\/([1-9][0-9]*)$/)
  if (m) return { page: 'lista', id: Number(m[1]) }
  m = pathname.match(/^\/album\/([1-9][0-9]*)$/)
  if (m) return { page: 'album', id: Number(m[1]) }
  m = pathname.match(/^\/artist\/([^/]+)$/)
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
  return path ? `/folders?path=${encodeURIComponent(path)}` : '/folders'
}

export function albumHref(id) {
  return `/album/${id}`
}

export function listHref(id) {
  return `/lists/${id}`
}

export function artistHref(key) {
  return `/artist/${encodeURIComponent(key)}`
}

// exploreHref links Explorar with some facets: exploreHref({ country: 'AR' }).
export function exploreHref(facets = {}, view = '') {
  const params = new URLSearchParams()
  if (view && view !== 'albums') params.set('view', view)
  for (const [k, v] of Object.entries(facets)) if (v) params.set(k, v)
  const s = params.toString()
  return s ? `/explore?${s}` : '/explore'
}

export function searchHref(q) {
  return `/search?q=${encodeURIComponent(q.trim())}`
}

// statsHref links the statistics page; "month" is the default period.
export function statsHref(period) {
  return period && period !== 'month' ? `/stats?period=${encodeURIComponent(period)}` : '/stats'
}

// duplicatesHref links the duplicates tab of Revisar; "exact" is the default kind.
export function duplicatesHref(kind) {
  return kind && kind !== 'exact' ? `/review/duplicates?kind=${encodeURIComponent(kind)}` : '/review/duplicates'
}
