// The play queue: pure functions over a plain, serializable state. Every
// queued track is an entry with its own key, so the same track can be
// queued twice. list keeps the entries as they were added (the order
// without shuffle); order lists their keys in play order; pos is the
// current entry's place in order (-1: nothing).

export const MAX = 5000 // as the server's MaxPlay: what fits in localStorage

export const REPEAT = ['off', 'all', 'one']

export function empty() {
  return { list: [], order: [], pos: -1, shuffle: false, repeat: 'off', nextKey: 1, station: null }
}

function entries(q, items) {
  let key = q.nextKey
  const out = items.map((item) => ({ ...item, key: key++ }))
  return { out, nextKey: key }
}

// shuffled returns a copy of keys in random order (Fisher–Yates; rand
// returns [0, 1)).
function shuffled(keys, rand) {
  const a = [...keys]
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(rand() * (i + 1))
    ;[a[i], a[j]] = [a[j], a[i]]
  }
  return a
}

// replace starts a new queue with items, playing items[start] first. With
// shuffle on, the rest come in random order.
export function replace(q, items, start = 0, rand = Math.random) {
  items = items.slice(0, MAX)
  const { out, nextKey } = entries(q, items)
  const keys = out.map((e) => e.key)
  if (keys.length === 0) return { ...q, list: [], order: [], pos: -1, nextKey }
  start = Math.min(Math.max(start, 0), keys.length - 1)
  let order = keys
  let pos = start
  if (q.shuffle) {
    order = [keys[start], ...shuffled(keys.filter((_, i) => i !== start), rand)]
    pos = 0
  }
  return { ...q, list: out, order, pos, nextKey }
}

// append adds items at the end of the queue ("Agregar a la cola").
export function append(q, items) {
  items = items.slice(0, Math.max(0, MAX - q.list.length))
  const { out, nextKey } = entries(q, items)
  const order = [...q.order, ...out.map((e) => e.key)]
  return { ...q, list: [...q.list, ...out], order, pos: q.pos < 0 && order.length ? 0 : q.pos, nextKey }
}

// playNext puts items right after the current track ("Reproducir a
// continuación"), in the list and in the play order.
export function playNext(q, items) {
  items = items.slice(0, Math.max(0, MAX - q.list.length))
  if (q.pos < 0) return append(q, items)
  const { out, nextKey } = entries(q, items)
  const keys = out.map((e) => e.key)
  const cur = q.order[q.pos]
  const at = q.list.findIndex((e) => e.key === cur) + 1
  const list = [...q.list.slice(0, at), ...out, ...q.list.slice(at)]
  const order = [...q.order.slice(0, q.pos + 1), ...keys, ...q.order.slice(q.pos + 1)]
  return { ...q, list, order, nextKey }
}

function byKey(q, key) {
  return q.list.find((e) => e.key === key) ?? null
}

// current is the entry playing (or paused), or null.
export function current(q) {
  return q.pos >= 0 && q.pos < q.order.length ? byKey(q, q.order[q.pos]) : null
}

// nextPos is where the queue goes after the current entry: auto is true
// when the track ended by itself (repeat one plays it again). -1: nothing
// more to play.
function nextPos(q, auto) {
  if (q.pos < 0 || q.order.length === 0) return -1
  if (auto && q.repeat === 'one') return q.pos
  if (q.pos + 1 < q.order.length) return q.pos + 1
  return q.repeat === 'off' ? -1 : 0
}

// next moves to the following entry, or returns null at the end.
export function next(q, auto = false) {
  const pos = nextPos(q, auto)
  return pos < 0 ? null : { ...q, pos }
}

// peekPos is the play-order position peek returns, -1 when none.
export function peekPos(q) {
  return nextPos(q, true)
}

// peek is the entry that will play when the current one ends (to preload
// it), or null.
export function peek(q) {
  const pos = peekPos(q)
  return pos < 0 ? null : byKey(q, q.order[pos])
}

// prev moves to the previous entry (the last one from the first with
// repeat on; the first one stays).
export function prev(q) {
  if (q.pos < 0) return q
  if (q.pos > 0) return { ...q, pos: q.pos - 1 }
  return q.repeat === 'off' ? q : { ...q, pos: q.order.length - 1 }
}

// jump plays the entry at a place of the play order.
export function jump(q, pos) {
  return pos >= 0 && pos < q.order.length ? { ...q, pos } : q
}

// remove takes the entry at a place of the play order out of the queue.
// Removing the current one makes the next one current.
export function remove(q, pos) {
  if (pos < 0 || pos >= q.order.length) return q
  const key = q.order[pos]
  const order = q.order.filter((_, i) => i !== pos)
  const list = q.list.filter((e) => e.key !== key)
  let p = q.pos
  if (pos < p) p--
  if (p >= order.length) p = order.length - 1
  return { ...q, list, order, pos: order.length ? p : -1 }
}

// move takes the entry at place from of the play order to place to; both
// are clamped to what is still to play. Without shuffle the list follows,
// so the new order survives turning shuffle on and off.
export function move(q, from, to) {
  if (from <= q.pos || from >= q.order.length) return q
  to = Math.min(Math.max(to, q.pos + 1), q.order.length - 1)
  if (to === from) return q
  const key = q.order[from]
  const order = [...q.order]
  order.splice(from, 1)
  order.splice(to, 0, key)
  if (q.shuffle) return { ...q, order }
  // Without shuffle, put it in the list right after the entry now before it.
  const list = q.list.filter((e) => e.key !== key)
  const at = list.findIndex((e) => e.key === order[to - 1]) + 1
  list.splice(at, 0, byKey(q, key))
  return { ...q, list, order }
}

// clearUpcoming keeps what was played and the current entry, and drops the
// rest.
export function clearUpcoming(q) {
  const keep = new Set(q.order.slice(0, q.pos + 1))
  return { ...q, order: q.order.slice(0, q.pos + 1), list: q.list.filter((e) => keep.has(e.key)) }
}

// trimPlayed drops the entries played before the last keep ones (in play
// order), so a station's queue does not grow forever.
export function trimPlayed(q, keep) {
  const drop = q.pos - keep
  if (drop <= 0) return q
  const gone = new Set(q.order.slice(0, drop))
  return { ...q, order: q.order.slice(drop), list: q.list.filter((e) => !gone.has(e.key)), pos: q.pos - drop }
}

// setShuffle turns shuffle on (what is still to play comes in random
// order) or off (back to the list's order, from the current entry).
export function setShuffle(q, on, rand = Math.random) {
  if (on === q.shuffle) return q
  if (on) {
    const done = q.order.slice(0, q.pos + 1)
    return { ...q, shuffle: true, order: [...done, ...shuffled(q.order.slice(q.pos + 1), rand)] }
  }
  const cur = q.order[q.pos]
  const order = q.list.map((e) => e.key)
  return { ...q, shuffle: false, order, pos: q.pos < 0 ? -1 : order.indexOf(cur) }
}

// cycleRepeat goes off → all → one → off.
export function cycleRepeat(q) {
  return { ...q, repeat: REPEAT[(REPEAT.indexOf(q.repeat) + 1) % REPEAT.length] }
}

// upcoming lists the entries after the current one, with their place in
// the play order.
export function upcoming(q) {
  return q.order.slice(q.pos + 1).map((key, i) => ({ entry: byKey(q, key), pos: q.pos + 1 + i }))
}

// restore reads a saved queue back, or an empty one when it is missing or
// broken.
export function restore(json) {
  try {
    const q = JSON.parse(json)
    if (!Array.isArray(q?.list) || !Array.isArray(q?.order)) return empty()
    const keys = new Set(q.list.map((e) => e.key))
    if (q.order.some((k) => !keys.has(k))) return empty()
    if (!REPEAT.includes(q.repeat)) q.repeat = 'off'
    q.shuffle = !!q.shuffle
    q.station = q.station && typeof q.station === 'object' && !Array.isArray(q.station) ? q.station : null
    q.pos = Number.isInteger(q.pos) && q.pos < q.order.length ? q.pos : q.order.length ? 0 : -1
    q.nextKey = Math.max(q.nextKey ?? 1, ...q.list.map((e) => e.key + 1))
    return q
  } catch {
    return empty()
  }
}
