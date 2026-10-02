// The user's marks (likes and ratings), as the client holds them. Pure:
// marks are never mutated, every change returns a new object.
//
// marks: { tracks: {id: {liked, likedAt, rating}}, albums: {id: {…}},
//          artists: {key: true}, collections: {path: true} }

export function empty() {
  return { tracks: {}, albums: {}, artists: {}, collections: {} }
}

const set = (list) => Object.fromEntries((list ?? []).map((k) => [k, true]))

// fromServer reads /api/marks (artists and collections come as arrays).
export function fromServer(json) {
  return {
    tracks: { ...(json?.tracks ?? {}) },
    albums: { ...(json?.albums ?? {}) },
    artists: set(json?.artists),
    collections: set(json?.collections),
  }
}

// kinds that hold a mark per id; the others only hold a liked flag.
const KEYED = { track: 'tracks', album: 'albums' }
const FLAGS = { artist: 'artists', collection: 'collections' }

export function isLiked(m, kind, ref) {
  if (KEYED[kind]) return !!m[KEYED[kind]][ref]?.liked
  if (FLAGS[kind]) return !!m[FLAGS[kind]][ref]
  return false
}

export function ratingOf(m, kind, id) {
  return KEYED[kind] ? (m[KEYED[kind]][id]?.rating ?? 0) : 0
}

// put stores a mark under its id, dropping it when nothing is left.
function put(m, field, id, mark) {
  const next = { ...m[field] }
  if (mark.liked || mark.rating) next[id] = mark
  else delete next[id]
  return { ...m, [field]: next }
}

export function withLike(m, kind, ref, liked) {
  if (FLAGS[kind]) {
    const field = FLAGS[kind]
    const next = { ...m[field] }
    if (liked) next[ref] = true
    else delete next[ref]
    return { ...m, [field]: next }
  }
  const field = KEYED[kind]
  if (!field) return m
  const { liked: _l, likedAt: _a, ...rest } = m[field][ref] ?? {}
  return put(m, field, ref, liked ? { ...rest, liked: true } : rest)
}

export function withRating(m, kind, id, stars) {
  const field = KEYED[kind]
  if (!field) return m
  const { rating: _r, ...rest } = m[field][id] ?? {}
  return put(m, field, id, stars ? { ...rest, rating: stars } : rest)
}
