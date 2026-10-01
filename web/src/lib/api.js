// Calls to the Go server. Errors carry the HTTP status and the server's text.

export class ApiError extends Error {
  constructor(status, message) {
    super(message)
    this.status = status
  }
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
    throw new ApiError(res.status, text || `Error ${res.status}`)
  }
  if (res.status === 204 || res.status === 202) return null
  return res.json()
}

const get = (path) => call('GET', path)
const post = (path, body) => call('POST', path, body ?? {})

// playQuery writes what to play as /api/play's query: {album}, {folder},
// {tracks: [ids]}, plus shuffle.
export function playQuery({ album, folder, tracks, shuffle }) {
  const params = new URLSearchParams()
  if (album != null) params.set('album', album)
  if (folder != null) params.set('folder', folder)
  for (const id of tracks ?? []) params.append('track', id)
  if (shuffle) params.set('shuffle', '1')
  return params.toString()
}

export const api = {
  status: () => get('/api/status'),
  scan: () => post('/api/scan'),
  album: (id) => get(`/api/albums/${id}`),
  folder: (path) => get(path ? `/api/folders?${new URLSearchParams({ path })}` : '/api/folders'),
  collections: () => get('/api/collections'),
  play: (what) => get(`/api/play?${playQuery(what)}`),
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
