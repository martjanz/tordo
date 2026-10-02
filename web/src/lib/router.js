// Routes of the app. Pure: nav.svelte.js keeps the current route.

// resolve maps a pathname and query to the page to show and its parameters.
export function resolve(pathname, search = '') {
  const params = new URLSearchParams(search)
  if (pathname === '/' || pathname === '/explorar') return { page: 'explorar' }
  if (pathname === '/carpetas') return { page: 'carpetas', path: params.get('ruta') ?? '' }
  if (pathname === '/listas') return { page: 'listas' }
  if (pathname === '/buscar') return { page: 'buscar', q: params.get('q') ?? '' }
  if (pathname === '/revisar') return { page: 'revisar', tab: 'sin-identificar' }
  if (pathname === '/revisar/generos') return { page: 'revisar', tab: 'generos' }
  let m = pathname.match(/^\/album\/([1-9][0-9]*)$/)
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
