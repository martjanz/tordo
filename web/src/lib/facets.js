// Explorar's query: tab, facets and order, as they appear in the URL.
import { country, formatName } from './format.js'

export const VIEWS = [
  { value: 'albums', label: 'Álbumes' },
  { value: 'artists', label: 'Artistas' },
  { value: 'tracks', label: 'Pistas' },
]

// FACETS lists the facets of the bar, in order. The year lives inside the
// decade menu and the style inside the genre menu; `more` facets go in the
// "Más" menu.
export const FACETS = [
  { name: 'decade', label: 'Década' },
  { name: 'artist', label: 'Artista' },
  { name: 'country', label: 'País' },
  { name: 'genre', label: 'Género' },
  { name: 'collection', label: 'Colección' },
  { name: 'liked', label: 'Me gusta' },
  { name: 'rating', label: 'Puntaje' },
  { name: 'listened', label: 'Escucha' },
  { name: 'folder', label: 'Carpeta', more: true },
  { name: 'quality', label: 'Calidad', more: true },
  { name: 'format', label: 'Formato', more: true },
  { name: 'state', label: 'Estado', more: true },
]

// facetLabel is a facet's name in the bar. On the albums tab the "liked"
// facet (same URL) reads Preferidos: albums are a shortlist, not favourites.
export function facetLabel(facet, view) {
  return facet.name === 'liked' && view === 'albums' ? 'Preferidos' : facet.label
}

// NAMES are the facets in URL order, the year and style included.
export const NAMES = ['decade', 'year', 'artist', 'country', 'genre', 'style', 'collection', 'folder', 'quality', 'format', 'state', 'liked', 'rating', 'listened']

// ORDERS lists each tab's orders, its default first (as the server).
export const ORDERS = {
  albums: [
    { value: 'year', label: 'Año' },
    { value: 'title', label: 'Título' },
    { value: 'artist', label: 'Artista' },
    { value: 'added', label: 'Agregado' },
    { value: 'plays', label: 'Más escuchado' },
    { value: 'random', label: 'Aleatorio' },
  ],
  artists: [
    { value: 'title', label: 'Nombre' },
    { value: 'count', label: 'Cantidad de álbumes' },
    { value: 'added', label: 'Agregado' },
    { value: 'plays', label: 'Más escuchado' },
    { value: 'random', label: 'Aleatorio' },
  ],
  tracks: [
    { value: 'artist', label: 'Artista' },
    { value: 'year', label: 'Año' },
    { value: 'title', label: 'Título' },
    { value: 'added', label: 'Agregado' },
    { value: 'plays', label: 'Más escuchado' },
    { value: 'random', label: 'Aleatorio' },
  ],
}

export const LABELS = {
  quality: { lossless: 'Sin pérdida', lossy: 'Con pérdida' },
  state: { unidentified: 'Sin identificar', unplayable: 'No reproducible', duplicated: 'Con duplicados' },
  liked: { yes: 'Me gusta' },
  listened: { never: 'Nunca escuchado', 'year-ago': 'Sin escuchar hace un año' },
  rating: { 5: '★★★★★', 4: '★★★★ o más', 3: '★★★ o más', 2: '★★ o más', 1: '★ o más' },
}

// defaultDir is the natural direction of an order: A to Z for names,
// newest or largest first otherwise.
export function defaultDir(order) {
  return order === 'title' || order === 'artist' ? 'asc' : 'desc'
}

function viewOf(v) {
  return ORDERS[v] ? v : 'albums'
}

// parse reads a query string ("?decade=1970&order=title"). Unknown
// parameters are dropped; the server validates the values.
export function parse(search) {
  const params = new URLSearchParams(search)
  const view = viewOf(params.get('view'))
  const facets = {}
  for (const name of NAMES) {
    const v = params.get(name)
    if (v) facets[name] = v
  }
  const orders = ORDERS[view]
  const order = orders.some((o) => o.value === params.get('order')) ? params.get('order') : orders[0].value
  const dir = ['asc', 'desc'].includes(params.get('dir')) ? params.get('dir') : defaultDir(order)
  const q = { view, facets, order, dir }
  // The random order's seed; without one the server draws it.
  if (order === 'random' && params.get('seed')) q.seed = params.get('seed')
  return q
}

// toSearch writes a query back, in a stable order and leaving defaults out,
// so that equal queries give equal URLs.
export function toSearch({ view, facets, order, dir, seed }) {
  const params = new URLSearchParams()
  view = viewOf(view)
  if (view !== 'albums') params.set('view', view)
  for (const name of NAMES) {
    if (facets[name]) params.set(name, facets[name])
  }
  const natural = ORDERS[view][0].value
  if (order && order !== natural) params.set('order', order)
  if (dir && dir !== defaultDir(order || natural)) params.set('dir', dir)
  if (order === 'random' && seed) params.set('seed', String(seed))
  const s = params.toString()
  return s ? `?${s}` : ''
}

// withFacet sets (or, with a null value, clears) one facet. A year outside
// the chosen decade is dropped, and clearing the decade clears the year; a
// style goes with its genre.
export function withFacet(query, name, value) {
  const facets = { ...query.facets }
  if (value == null || value === '') delete facets[name]
  else facets[name] = String(value)
  if (name === 'decade' && facets.year && (!facets.decade || !facets.year.startsWith(facets.decade.slice(0, 3)))) {
    delete facets.year
  }
  if (name === 'year' && value && !facets.decade) {
    facets.decade = String(Math.floor(Number(value) / 10) * 10)
  }
  if (name === 'genre') delete facets.style
  return { ...query, facets }
}

// withView switches tabs, keeping the facets, with the tab's default order.
export function withView(query, view) {
  view = viewOf(view)
  return withOrder({ ...query, view }, ORDERS[view][0].value)
}

// withOrder changes the order, resetting the direction to its natural one.
// The random order gets a new seed each time: choosing it again reshuffles.
export function withOrder(query, order) {
  const { seed, ...rest } = query
  const q = { ...rest, order, dir: defaultDir(order) }
  if (order === 'random') q.seed = String(Math.floor(Math.random() * 2 ** 31) + 1)
  return q
}

// valueLabel is how a facet value reads in menus and chips. counted is the
// server's {value, label, count} when known; view is Explorar's tab.
export function valueLabel(name, value, counted, view) {
  switch (name) {
    case 'decade':
      return `${value}s`
    case 'country':
      return country(value)
    case 'format':
      return formatName(value)
    case 'liked':
      return view === 'albums' ? 'Preferidos' : LABELS.liked[value] ?? value
    case 'quality':
    case 'state':
    case 'rating':
    case 'listened':
      return LABELS[name][value] ?? value
  }
  return counted?.label || value
}

// chips lists the applied facets, labelled with the server's counts.
export function chips(query, counts = {}) {
  const out = []
  for (const name of NAMES) {
    const value = query.facets[name]
    if (!value) continue
    const counted = (counts[name] ?? []).find((v) => v.value === value)
    out.push({ name, value, label: valueLabel(name, value, counted, query.view) })
  }
  return out
}

// inside lists the values of a facet nested in a chosen one: the years of
// the chosen decade, the styles of the chosen genre.
export function inside(name, query, counts = {}) {
  if (name === 'decade' && query.facets.decade) {
    return (counts.year ?? []).filter((v) => v.value.startsWith(query.facets.decade.slice(0, 3)))
  }
  if (name === 'genre' && query.facets.genre) return counts.style ?? []
  return []
}
