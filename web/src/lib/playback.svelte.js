// What the components call to play: the local player, or, in Rockola mode,
// commands to the computer. Components never import the player directly.
import { allowed } from './jukebox.js'
import * as P from './player.svelte.js'
import { rockola, send } from './rockola.svelte.js'

export { player } from './player.svelte.js'

// shown is what the bar displays: the computer's state in Rockola mode.
export const shown = () => (rockola.on ? rockola.view : P.player)
export const remote = () => rockola.on

// can says whether the current role may run a command (an entry is only
// needed for removeAt: a guest may remove what they added).
export function can(command, entry = null) {
  if (!rockola.on) return true
  if (command === 'removeAt' && rockola.role !== 'host') {
    return entry?.addedBy != null && entry.addedBy === rockola.session
  }
  return allowed(rockola.role, command)
}

const order = () => shown()?.queue?.order ?? []

export function play(what, opts = {}) {
  if (!rockola.on) return P.play(what, opts)
  const { shuffle, startId } = opts
  if (rockola.role === 'host') {
    return send('play', { what, shuffle: !!shuffle, startId: startId ?? null })
  }
  // A guest cannot take over the player: what they pick goes to the queue.
  return send('enqueue', {
    what: startId != null ? { tracks: [startId] } : what,
    shuffle: !!shuffle && startId == null,
    startIfIdle: true,
  })
}

export function enqueue(what, opts = {}) {
  if (!rockola.on) return P.enqueue(what, opts)
  return send('enqueue', { what, next: !!opts.next })
}

export const playStation = (st) => (rockola.on ? send('playStation', { station: st }) : P.playStation(st))
export const setStation = (st) => (rockola.on ? send('setStation', { station: st }) : P.setStation(st))
export const jumpTo = (pos) => (rockola.on ? send('jumpTo', { key: order()[pos] }) : P.jumpTo(pos))
export const removeAt = (pos) => (rockola.on ? send('removeAt', { key: order()[pos] }) : P.removeAt(pos))
export const moveTo = (from, to) => {
  if (!rockola.on) return P.moveTo(from, to)
  const o = order()
  return send('moveTo', { key: o[from], toKey: o[to] })
}
export const seek = (seconds) => (rockola.on ? send('seek', { seconds }) : P.seek(seconds))
export const setVolume = (v) => (rockola.on ? send('setVolume', { volume: v }) : P.setVolume(v))

export const stopStation = () => (rockola.on ? send('stopStation') : P.stopStation())
export const toggle = () => (rockola.on ? send('toggle') : P.toggle())
export const next = () => (rockola.on ? send('next') : P.next())
export const prev = () => (rockola.on ? send('prev') : P.prev())
export const clearUpcoming = () => (rockola.on ? send('clearUpcoming') : P.clearUpcoming())
export const toggleMute = () => (rockola.on ? send('toggleMute') : P.toggleMute())
export const toggleShuffle = () => (rockola.on ? send('toggleShuffle') : P.toggleShuffle())
export const cycleRepeat = () => (rockola.on ? send('cycleRepeat') : P.cycleRepeat())
