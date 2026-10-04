// The Rockola's commands, shared by the computer (which runs them) and the
// phones (which send them). Pure: the player is passed in.

// ROLES is the lowest role that may send each command (as the server's table).
export const ROLES = {
  enqueue: 'guest', toggle: 'guest', removeAt: 'guest',
  play: 'host', playStation: 'host', stopStation: 'host', setStation: 'host',
  next: 'host', prev: 'host', jumpTo: 'host', moveTo: 'host', clearUpcoming: 'host',
  seek: 'host', setVolume: 'host', toggleMute: 'host', toggleShuffle: 'host', cycleRepeat: 'host',
}

export const KEEP_PLAYED = 10 // entries already played that the phones see
export const KEEP_NEXT = 100 // entries to come that the phones see

export function allowed(role, command) {
  const need = Object.hasOwn(ROLES, command) ? ROLES[command] : null
  return !!need && (need === 'guest' || role === 'host')
}

const isObj = (v) => v !== null && typeof v === 'object' && !Array.isArray(v)
const isId = (v) => Number.isInteger(v) && v > 0
const isNum = (v) => typeof v === 'number' && Number.isFinite(v)

// cleanWhat keeps what /api/play understands: {album}, {folder}, {artist},
// {tracks}, {playlist}, {liked}; null when there is nothing valid.
function cleanWhat(w) {
  if (!isObj(w)) return null
  const out = {}
  if (isId(w.album)) out.album = w.album
  if (typeof w.folder === 'string') out.folder = w.folder
  if (typeof w.artist === 'string') out.artist = w.artist
  if (Array.isArray(w.tracks) && w.tracks.length <= 5000 && w.tracks.every(isId)) out.tracks = w.tracks
  if (isId(w.playlist)) out.playlist = w.playlist
  if (w.liked === true) out.liked = true
  return Object.keys(out).length ? out : null
}

export function cleanArgs(command, args) {
  if (!Object.hasOwn(ROLES, command)) return null
  const a = isObj(args) ? args : {}
  switch (command) {
    case 'play': {
      const what = cleanWhat(a.what)
      return what && { what, shuffle: a.shuffle === true, startId: isId(a.startId) ? a.startId : null }
    }
    case 'enqueue': {
      const what = cleanWhat(a.what)
      return what && { what, next: a.next === true, shuffle: a.shuffle === true, startIfIdle: a.startIfIdle === true }
    }
    case 'playStation':
    case 'setStation':
      return isObj(a.station) ? { station: a.station } : null
    case 'jumpTo':
    case 'removeAt':
      return isId(a.key) ? { key: a.key } : null
    case 'moveTo':
      return isId(a.key) && isId(a.toKey) ? { key: a.key, toKey: a.toKey } : null
    case 'seek':
      return isNum(a.seconds) && a.seconds >= 0 ? { seconds: a.seconds } : null
    case 'setVolume':
      return isNum(a.volume) && a.volume >= 0 && a.volume <= 1 ? { volume: a.volume } : null
    default:
      return {}
  }
}

// snapshot is what the computer publishes: the player's state with the queue
// trimmed around the current entry. upcoming counts every entry still to come.
export function snapshot(p) {
  const q = p.queue
  const from = q.pos < 0 ? 0 : Math.max(0, q.pos - KEEP_PLAYED)
  const to = q.pos < 0 ? KEEP_NEXT : q.pos + 1 + KEEP_NEXT
  const order = q.order.slice(from, to)
  const keys = new Set(order)
  return {
    queue: {
      list: q.list.filter((e) => keys.has(e.key)),
      order,
      pos: q.pos < 0 ? -1 : q.pos - from,
      shuffle: q.shuffle,
      repeat: q.repeat,
      station: q.station,
      upcoming: q.pos < 0 ? q.order.length : q.order.length - q.pos - 1,
    },
    time: p.time,
    duration: p.duration,
    playing: p.playing,
    seekable: p.seekable,
    volume: p.volume,
    muted: p.muted,
    blocked: p.blocked,
  }
}

// run executes a phone's command on the player P, checking role and
// arguments again; say sends a notice back to that phone.
export async function run({ command, args, session, role }, P, say) {
  if (!allowed(role, command)) return say('Solo el anfitrión puede hacer esto.')
  const a = cleanArgs(command, args)
  if (!a) return
  switch (command) {
    case 'play':
      return P.play(a.what, { shuffle: a.shuffle, startId: a.startId, say })
    case 'enqueue':
      return P.enqueue(a.what, { next: a.next, shuffle: a.shuffle, startIfIdle: a.startIfIdle, addedBy: session || null, say })
    case 'playStation':
      return P.playStation(a.station, { say })
    case 'setStation':
      return P.setStation(a.station)
    case 'jumpTo': {
      const pos = P.posOfKey(a.key)
      if (pos >= 0) P.jumpTo(pos)
      return
    }
    case 'moveTo': {
      const from = P.posOfKey(a.key)
      const to = P.posOfKey(a.toKey)
      if (from >= 0 && to >= 0) P.moveTo(from, to)
      return
    }
    case 'removeAt': {
      const entry = P.entryOfKey(a.key)
      if (!entry) return
      if (role !== 'host' && entry.addedBy !== session) return say('Solo podés quitar lo que agregaste.')
      return P.removeAt(P.posOfKey(a.key))
    }
    case 'seek':
      return P.seek(a.seconds)
    case 'setVolume':
      return P.setVolume(a.volume)
    default:
      return P[command]()
  }
}
