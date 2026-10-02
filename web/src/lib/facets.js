// Explorar's query: tab, facets and order, as they appear in the URL.
import { country, formatName } from './format.js'

export const VIEWS = [
  { value: 'albumes', label: 'Álbumes' },
  { value: 'artistas', label: 'Artistas' },
  { value: 'pistas', label: 'Pistas' },
]

// FACETS lists the facets of the bar, in order. The year lives inside the
// decade menu and the style inside the genre menu; `more` facets go in the
// "Más" menu.
export const FACETS = [
  { name: 'decada', label: 'Década' },
  { name: 'artista', label: 'Artista' },
  { name: 'pais', label: 'País' },
  { name: 'genero', label: 'Género' },
  { name: 'coleccion', label: 'Colección' },
  { name: 'carpeta', label: 'Carpeta', more: true },
  { name: 'calidad', label: 'Calidad', more: true },
  { name: 'formato', label: 'Formato', more: true },
  { name: 'estado', label: 'Estado', more: true },
]

// NAMES are the facets in URL order, the year and style included.
const NAMES = ['decada', 'anio', 'artista', 'pais', 'genero', 'estilo', 'coleccion', 'carpeta', 'calidad', 'formato', 'estado']

// ORDERS lists each tab's orders, its default first (as the server).
export const ORDERS = {
  albumes: [
    { value: 'anio', label: 'Año' },
    { value: 'titulo', label: 'Título' },
    { value: 'artista', label: 'Artista' },
    { value: 'agregado', label: 'Agregado' },
  ],
  artistas: [
    { value: 'titulo', label: 'Nombre' },
    { value: 'cantidad', label: 'Cantidad de álbumes' },
    { value: 'agregado', label: 'Agregado' },
  ],
  pistas: [
    { value: 'artista', label: 'Artista' },
    { value: 'anio', label: 'Año' },
    { value: 'titulo', label: 'Título' },
    { value: 'agregado', label: 'Agregado' },
  ],
}

export const LABELS = {
  calidad: { 'sin-perdida': 'Sin pérdida', 'con-perdida': 'Con pérdida' },
  estado: { 'sin-identificar': 'Sin identificar', 'no-reproducible': 'No reproducible' },
}

// defaultDir is the natural direction of an order: A to Z for names,
// newest or largest first otherwise.
export function defaultDir(order) {
  return order === 'titulo' || order === 'artista' ? 'asc' : 'desc'
}

function viewOf(v) {
  return ORDERS[v] ? v : 'albumes'
}

// parse reads a query string ("?decada=1970&orden=titulo"). Unknown
// parameters are dropped; the server validates the values.
export function parse(search) {
  const params = new URLSearchParams(search)
  const view = viewOf(params.get('ver'))
  const facets = {}
  for (const name of NAMES) {
    const v = params.get(name)
    if (v) facets[name] = v
  }
  const orders = ORDERS[view]
  const order = orders.some((o) => o.value === params.get('orden')) ? params.get('orden') : orders[0].value
  const dir = ['asc', 'desc'].includes(params.get('dir')) ? params.get('dir') : defaultDir(order)
  return { view, facets, order, dir }
}

// toSearch writes a query back, in a stable order and leaving defaults out,
// so that equal queries give equal URLs.
export function toSearch({ view, facets, order, dir }) {
  const params = new URLSearchParams()
  view = viewOf(view)
  if (view !== 'albumes') params.set('ver', view)
  for (const name of NAMES) {
    if (facets[name]) params.set(name, facets[name])
  }
  const natural = ORDERS[view][0].value
  if (order && order !== natural) params.set('orden', order)
  if (dir && dir !== defaultDir(order || natural)) params.set('dir', dir)
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
  if (name === 'decada' && facets.anio && (!facets.decada || !facets.anio.startsWith(facets.decada.slice(0, 3)))) {
    delete facets.anio
  }
  if (name === 'anio' && value && !facets.decada) {
    facets.decada = String(Math.floor(Number(value) / 10) * 10)
  }
  if (name === 'genero') delete facets.estilo
  return { ...query, facets }
}

// withView switches tabs, keeping the facets, with the tab's default order.
export function withView(query, view) {
  view = viewOf(view)
  const order = ORDERS[view][0].value
  return { ...query, view, order, dir: defaultDir(order) }
}

// withOrder changes the order, resetting the direction to its natural one.
export function withOrder(query, order) {
  return { ...query, order, dir: defaultDir(order) }
}

// valueLabel is how a facet value reads in menus and chips. counted is the
// server's {value, label, count} when known.
export function valueLabel(name, value, counted) {
  switch (name) {
    case 'decada':
      return `${value}s`
    case 'pais':
      return country(value)
    case 'formato':
      return formatName(value)
    case 'calidad':
    case 'estado':
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
    out.push({ name, value, label: valueLabel(name, value, counted) })
  }
  return out
}

// inside lists the values of a facet nested in a chosen one: the years of
// the chosen decade, the styles of the chosen genre.
export function inside(name, query, counts = {}) {
  if (name === 'decada' && query.facets.decada) {
    return (counts.anio ?? []).filter((v) => v.value.startsWith(query.facets.decada.slice(0, 3)))
  }
  if (name === 'genero' && query.facets.genero) return counts.estilo ?? []
  return []
}
