// Calls to the Go server. Errors carry the HTTP status and the server's text.

export class ApiError extends Error {
  constructor(status, message, retryAfter = 0) {
    super(message)
    this.status = status
    this.retryAfter = retryAfter // seconds, from Retry-After
  }
}

let unauthorized = null

// onUnauthorized registers what to do when the server answers 401 (the
// session ended or a PIN is needed). Login's own 401 means a wrong PIN and
// does not count.
export function onUnauthorized(fn) {
  unauthorized = fn
}

async function call(method, path, body) {
  const init = { method, headers: {} }
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  let res
  try {
    res = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'No se pudo contactar a Tordo. ¿Sigue abierto?')
  }
  if (!res.ok) {
    const text = (await res.text()).trim()
    if (res.status === 401 && path !== '/api/auth/login') unauthorized?.()
    const retryAfter = Number(res.headers?.get('Retry-After')) || 0
    throw new ApiError(res.status, text || `Error ${res.status}`, retryAfter)
  }
  if (res.status === 204 || res.status === 202) return null
  return res.json()
}

const get = (path) => call('GET', path)
const post = (path, body) => call('POST', path, body ?? {})
const put = (path, body) => call('PUT', path, body)
const del = (path) => call('DELETE', path)

// playQuery writes what to play as /api/play's query: {album}, {folder},
// {artist}, {tracks: [ids]}, plus shuffle.
export function playQuery({ album, folder, artist, tracks, playlist, liked, shuffle }) {
  const params = new URLSearchParams()
  if (album != null) params.set('album', album)
  if (folder != null) params.set('folder', folder)
  if (artist != null) params.set('artist', artist)
  for (const id of tracks ?? []) params.append('track', id)
  if (playlist != null) params.set('playlist', playlist)
  if (liked) params.set('liked', '1')
  if (shuffle) params.set('shuffle', '1')
  return params.toString()
}

// stationDef is a station as the server takes it: without its saved id and
// dates.
const stationDef = ({ id, createdAt, updatedAt, ...def }) => def // eslint-disable-line no-unused-vars

// stationQuery writes /api/station/next's query: the station's definition
// and the ids of the tracks to leave out.
export function stationQuery(station, recent = []) {
  const params = new URLSearchParams({ def: JSON.stringify(stationDef(station)) })
  if (recent.length) params.set('recientes', recent.join(','))
  return params.toString()
}

export const api = {
  status: () => get('/api/status'),
  scan: () => post('/api/scan'),
  album: (id) => get(`/api/albums/${id}`),
  folder: (path) => get(path ? `/api/folders?${new URLSearchParams({ path })}` : '/api/folders'),
  folderTree: (path) => get(path ? `/api/folders/tree?${new URLSearchParams({ path })}` : '/api/folders/tree'),
  folderTreeSearch: (q) => get(`/api/folders/tree/search?${new URLSearchParams({ q })}`),
  collections: () => get('/api/collections'),
  play: (what) => get(`/api/play?${playQuery(what)}`),
  // search is Explorar's query string ("?decada=1980"); from and limit page it.
  explore: (search, from = 0, limit = 120) => {
    const params = new URLSearchParams(search)
    if (from) params.set('desde', from)
    params.set('limite', limit)
    return get(`/api/explore?${params}`)
  },
  artist: (key) => get(`/api/artists/${encodeURIComponent(key)}`),
  find: (q, limit = 6) => get(`/api/search?${new URLSearchParams({ q, limite: limit })}`),
  setCollection: (path, collection) => put('/api/folders/collection', { path, collection }),
  identify: () => post('/api/identify'),
  unidentified: () => get('/api/review/unidentified'),
  // identification is {releaseId} (an MBID or a musicbrainz.org URL) or {ignore: true}.
  setIdentification: (id, identification) => put(`/api/albums/${id}/identification`, identification),
  resetIdentification: (id) => del(`/api/albums/${id}/identification`),
  genres: () => get('/api/genres'),
  unmappedGenres: () => get('/api/genres/unmapped'),
  genreAliases: () => get('/api/genres/aliases'),
  setGenreAlias: (alias) => put('/api/genres/aliases', alias),
  deleteGenreAlias: (raw) => del(`/api/genres/aliases?${new URLSearchParams({ raw })}`),
  marks: () => get('/api/marks'),
  // ref is {kind, id} for tracks and albums, {kind, key} for artists and collections.
  like: ({ kind, id, key }, liked) => put('/api/likes', { kind, ...(key != null ? { key } : { id }), liked }),
  rate: ({ kind, id }, stars) => put('/api/ratings', { kind, id, stars }),
  recordPlay: ({ id, listenedMs, device }) => post('/api/plays', { id, listenedMs, device }),
  liked: () => get('/api/liked'),
  gains: (ids) => {
    const params = new URLSearchParams()
    for (const id of ids) params.append('track', id)
    return get(`/api/gains?${params}`)
  },
  stationNext: (station, recent) => get(`/api/station/next?${stationQuery(station, recent)}`),
  stations: () => get('/api/stations'),
  createStation: (station) => post('/api/stations', stationDef(station)),
  updateStation: (id, station) => put(`/api/stations/${id}`, stationDef(station)),
  deleteStation: (id) => del(`/api/stations/${id}`),
  home: (seed) => get(`/api/home?${new URLSearchParams({ semilla: seed })}`),
  stats: (period, limit) => get(`/api/stats?${new URLSearchParams({ periodo: period, limite: limit })}`),
  playlists: () => get('/api/playlists'),
  playlist: (id) => get(`/api/playlists/${id}`),
  createPlaylist: (name, add) => post('/api/playlists', add ? { name, add } : { name }),
  renamePlaylist: (id, name) => put(`/api/playlists/${id}`, { name }),
  deletePlaylist: (id) => del(`/api/playlists/${id}`),
  // what is {album}, {folder}, {artist}, {tracks: [ids]} or {playlist}.
  addToPlaylist: (id, what) => post(`/api/playlists/${id}/items`, what),
  removeFromPlaylist: (id, itemId) => del(`/api/playlists/${id}/items/${itemId}`),
  reorderPlaylist: (id, itemIds) => put(`/api/playlists/${id}/order`, { items: itemIds }),
  config: () => get('/api/config'),
  saveConfig: (body) => put('/api/config', body),
  checkRoot: (path) => post('/api/config/root', { path }),
  checkToken: (token) => post('/api/config/token', { token }),
  lan: () => get('/api/lan'),
  sessions: () => get('/api/sessions'),
  closeSession: (id) => del(`/api/sessions/${id}`),
  jukeboxState: (token, state) => post('/api/jukebox/state', { token, state }),
  jukeboxNotice: (token, remote, text) => post('/api/jukebox/notice', { token, remote, text }),
  jukeboxCommand: (remote, command, args) => post('/api/jukebox/command', { remote, command, args }),
  setSessionHost: (id, host) => put(`/api/sessions/${id}/host`, { host }),
  auth: () => get('/api/auth'),
  login: (pin, device) => post('/api/auth/login', { pin, device }),
  logout: () => post('/api/auth/logout'),
  duplicates: (tipo) => get(`/api/review/duplicates?tipo=${encodeURIComponent(tipo)}`),
  toConvert: () => get('/api/review/formats'),
}

// lanQrURL is the QR picture of an address of this computer on the network.
export function lanQrURL(url) {
  return `/api/lan/qr?url=${encodeURIComponent(url)}`
}

// releaseURL links a MusicBrainz release.
export function releaseURL(id) {
  return `https://musicbrainz.org/release/${id}`
}

// artistMBURL links a MusicBrainz artist.
export function artistMBURL(id) {
  return `https://musicbrainz.org/artist/${id}`
}

// Picture URLs. size is one of the server's thumbnail sizes (160, 320,
// 640); without it, the original picture.
export function albumCoverURL(id, size) {
  return `/api/covers/album/${id}` + (size ? `?size=${size}` : '')
}

export function trackCoverURL(id, size) {
  return `/api/covers/track/${id}` + (size ? `?size=${size}` : '')
}

// itemCoverURL is the picture of a queued track: its album's, or its own
// when it is a loose track.
export function itemCoverURL(item, size) {
  return item.albumId ? albumCoverURL(item.albumId, size) : trackCoverURL(item.id, size)
}

export function streamURL(id, transcode = false) {
  return `/stream/${id}` + (transcode ? '?transcode=1' : '')
}
