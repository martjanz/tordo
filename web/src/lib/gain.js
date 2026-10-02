// The normalization gain of a queued track. Entries carry trackGain and
// albumGain (dB, from the server) when known. Only attenuates.

export const UNKNOWN_DB = -6 // a track not measured yet: typical for modern masters

// gainDb is the gain of the entry at position pos of the play order: the
// album's when the queue plays that album in order (shuffle off and a
// neighbor in play order from the same album), else the track's.
export function gainDb(q, pos) {
  const byKey = new Map(q.list.map((e) => [e.key, e]))
  const entry = byKey.get(q.order[pos])
  if (!entry) return 0
  const albumOf = (p) => (p >= 0 && p < q.order.length ? byKey.get(q.order[p])?.albumId : undefined)
  const inAlbum = !q.shuffle && entry.albumId && (albumOf(pos - 1) === entry.albumId || albumOf(pos + 1) === entry.albumId)
  if (inAlbum && entry.albumGain != null) return entry.albumGain
  return entry.trackGain ?? UNKNOWN_DB
}

// factor turns a gain into a volume multiplier, never above 1.
export function factor(db) {
  return Math.min(1, Math.pow(10, db / 20))
}

// missing lists the ids of up to count entries from position from of the
// play order whose track gain is not known yet, or that belong to an album
// whose gain is not (each id once).
export function missing(q, from, count) {
  const byKey = new Map(q.list.map((e) => [e.key, e]))
  const ids = new Set()
  for (let p = Math.max(0, from); p < q.order.length && ids.size < count; p++) {
    const e = byKey.get(q.order[p])
    if (e && (e.trackGain == null || (e.albumId && e.albumGain == null))) ids.add(e.id)
  }
  return [...ids]
}

// withGains sets the gains the server answered ({id: {trackGain, albumGain}})
// on every entry of those tracks.
export function withGains(q, gains) {
  return { ...q, list: q.list.map((e) => (gains[e.id] ? { ...e, ...gains[e.id] } : e)) }
}
