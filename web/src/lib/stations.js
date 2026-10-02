// Stations: endless queues the server fills in batches. Pure helpers to
// define them, to know when to ask for more and to describe them.
import { country } from './format.js'
import { NAMES, valueLabel } from './facets.js'

export const DEFAULT_RULES = { mode: 'pistas', artistGap: 3, avoidDays: 7 }
export const LOW_WATER = 3 // ask for more when fewer than this remain after the current track
export const RECENT = 100 // ids sent as recientes
export const KEEP_PLAYED = 100 // played entries kept while a station plays

export function station(name, source, rules = DEFAULT_RULES) {
  return { name, source, ...rules }
}

export const LIKED = { name: 'Tus me gusta', source: { liked: true } }
export const LIBRARY = { name: 'Toda la colección', source: {} }

// decadeName: "1980" → "Los 80", "2000" → "Los 2000".
function decadeName(decade) {
  const n = Number(decade)
  return n >= 1900 && n < 2000 ? `Los ${n - 1900}` : `Los ${decade}`
}

// exploreStation turns Explorar's query into a station. counts are the
// server's facet counts, when known, to label values (artists, genres).
export function exploreStation(query, counts = {}) {
  const facets = { ...query.facets }
  const labels = NAMES.filter((n) => facets[n]).map((n) =>
    valueLabel(n, facets[n], (counts[n] ?? []).find((v) => v.value === facets[n])),
  )
  const view = query.view === 'pistas' ? 'pistas' : 'albumes'
  return { name: labels.length ? labels.join(' · ') : LIBRARY.name, source: { view, facets } }
}

// albumStation is the station of an album: not the album on loop but its
// style, else its genre, else its artist, else its decade (album ids change
// with rescans; facets are stable). null when it has none of them.
export function albumStation(album) {
  const genres = album.genres ?? []
  const style = genres.find((g) => g.style)?.style
  if (style) return { name: style, source: { view: 'pistas', facets: { estilo: style } } }
  const genre = genres.find((g) => g.genre)?.genre
  if (genre) return { name: genre, source: { view: 'pistas', facets: { genero: genre } } }
  if (album.artistKey) {
    return { name: album.artist || album.artistKey, source: { view: 'pistas', facets: { artista: album.artistKey } } }
  }
  if (album.year > 0) {
    const decade = String(Math.floor(album.year / 10) * 10)
    return { name: decadeName(decade), source: { view: 'pistas', facets: { decada: decade } } }
  }
  return null
}

export function artistStation(artist) {
  return { name: artist.name, source: { view: 'pistas', facets: { artista: artist.key } } }
}

export function folderStation(folder) {
  return { name: folder.name, source: { folder: folder.path } }
}

export function playlistStation(list) {
  return { name: list.name, source: { playlist: list.id } }
}

// needsMore: a station is playing and fewer than LOW_WATER entries remain
// after the current one.
export function needsMore(q) {
  return !!q.station && q.order.length - (q.pos + 1) < LOW_WATER
}

// recentIds are the ids of the last RECENT entries up to the end of the
// play order, oldest first.
export function recentIds(q) {
  const byKey = new Map(q.list.map((e) => [e.key, e]))
  return q.order
    .slice(-RECENT)
    .map((k) => byKey.get(k)?.id)
    .filter((id) => id != null)
}

const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`

// summary reads a station's rules; rules at 0 are left out.
export function summary(st) {
  const parts = [st.mode === 'albumes' ? 'Álbumes' : 'Pistas']
  if (st.artistGap > 0) parts.push(`sin repetir artista en ${st.artistGap}`)
  if (st.avoidDays > 0) parts.push(`evita lo escuchado en ${plural(st.avoidDays, 'día', 'días')}`)
  return parts.join(' · ')
}

// rowTitle is the title of one of Inicio's rotating rows.
export function rowTitle(row) {
  switch (row.kind) {
    case 'decada':
      return decadeName(row.value)
    case 'pais':
      return country(row.value)
    case 'coleccion':
      return row.label || row.value
    case 'nunca':
      return 'Nunca escuchados'
    case 'hace-un-anio':
      return 'Sin escuchar hace un año'
    case 'azar':
      return 'Álbumes al azar'
  }
  return row.value
}
