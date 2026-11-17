// Stations: endless queues the server fills in batches. Pure helpers to
// define them, to know when to ask for more and to describe them.
import { country } from './format.js'
import { NAMES, valueLabel } from './facets.js'
import { t } from './i18n/index.svelte.js'

export const DEFAULT_RULES = { mode: 'tracks', artistGap: 3, avoidDays: 7 }
export const LOW_WATER = 3 // ask for more when fewer than this remain after the current track
export const RECENT = 100 // ids sent as recent
export const KEEP_PLAYED = 100 // played entries kept while a station plays

export function station(name, source, rules = DEFAULT_RULES) {
  return { name, source, ...rules }
}

// The built-ins are named in the language of the moment they are read: name
// is a getter over the *Name functions.
export const likedName = () => t('station.liked')
export const favoriteTracksName = () => t('station.favoriteTracks')
export const preferredAlbumsName = () => t('station.preferredAlbums')
export const libraryName = () => t('station.library')

export const LIKED = {
  get name() {
    return likedName()
  },
  source: { liked: true },
}
// Favourite (♥) tracks and Preferidos albums, as stations from Explorar's
// "liked" facet: the track mark on tracks, the album mark on albums.
export const FAVORITE_TRACKS = {
  get name() {
    return favoriteTracksName()
  },
  source: { view: 'tracks', facets: { liked: 'yes' } },
}
export const PREFERRED_ALBUMS = {
  get name() {
    return preferredAlbumsName()
  },
  source: { view: 'albums', facets: { liked: 'yes' } },
}
export const LIBRARY = {
  get name() {
    return libraryName()
  },
  source: {},
}

// decadeName: "1980" → "Los 80", "2000" → "Los 2000" (in the current language).
function decadeName(decade) {
  const n = Number(decade)
  return t('station.decade', { d: n > 1900 && n < 2000 ? n - 1900 : decade })
}

// exploreStation turns Explorar's query into a station. counts are the
// server's facet counts, when known, to label values (artists, genres).
export function exploreStation(query, counts = {}) {
  const facets = { ...query.facets }
  const labels = NAMES.filter((n) => facets[n]).map((n) =>
    valueLabel(n, facets[n], (counts[n] ?? []).find((v) => v.value === facets[n]), query.view),
  )
  const view = query.view === 'tracks' ? 'tracks' : 'albums'
  return { name: labels.length ? labels.join(' · ') : libraryName(), source: { view, facets } }
}

// albumStation is the station of an album: not the album on loop but its
// style, else its genre, else its artist, else its decade (album ids change
// with rescans; facets are stable). null when it has none of them.
export function albumStation(album) {
  const genres = album.genres ?? []
  const style = genres.find((g) => g.style)?.style
  if (style) return { name: style, source: { view: 'tracks', facets: { style } } }
  const genre = genres.find((g) => g.genre)?.genre
  if (genre) return { name: genre, source: { view: 'tracks', facets: { genre } } }
  if (album.artistKey) {
    return { name: album.artist || album.artistKey, source: { view: 'tracks', facets: { artist: album.artistKey } } }
  }
  if (album.year > 0) {
    const decade = String(Math.floor(album.year / 10) * 10)
    return { name: decadeName(decade), source: { view: 'tracks', facets: { decade } } }
  }
  return null
}

export function artistStation(artist) {
  return { name: artist.name, source: { view: 'tracks', facets: { artist: artist.key } } }
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

// summary reads a station's rules; rules at 0 are left out.
export function summary(st) {
  const parts = [t(st.mode === 'albums' ? 'facets.view.albums' : 'facets.view.tracks')]
  if (st.artistGap > 0) parts.push(t('station.artistGap', { n: st.artistGap }))
  if (st.avoidDays > 0) parts.push(t('station.avoidDays', { n: st.avoidDays }))
  return parts.join(' · ')
}

// rowTitle is the title of one of Inicio's rotating rows.
export function rowTitle(row) {
  switch (row.kind) {
    case 'decade':
      return decadeName(row.value)
    case 'country':
      return country(row.value)
    case 'collection':
      return row.label || row.value
    case 'never':
      return t('station.row.never')
    case 'year-ago':
      return t('station.row.yearAgo')
    case 'random':
      return t('station.row.random')
    case 'recent':
      return t('station.row.recent')
  }
  return row.value
}
