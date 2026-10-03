// The player: two <audio> elements, one playing and one loading the next
// track so it takes over as soon as the current one ends (almost gapless).
// The queue and the volume live in this browser's storage: each device has
// its own, and they survive reloading the page.
import { api, itemCoverURL, streamURL } from './api.js'
import { app, notify } from './app.svelte.js'
import { deviceName } from './device.js'
import { count } from './format.js'
import { factor, gainDb, missing, withGains } from './gain.js'
import * as P from './plays.js'
import * as Q from './queue.js'
import * as S from './stations.js'

const QUEUE_KEY = 'tordo.cola'
const TIME_KEY = 'tordo.tiempo'
const VOLUME_KEY = 'tordo.volumen'
const PRELOAD_SECONDS = 30 // start loading the next track this long before the end
const RESTART_SECONDS = 3 // "previous" after this long restarts the track

function load(key, fallback) {
  try {
    return localStorage.getItem(key) ?? fallback
  } catch {
    return fallback
  }
}

function save(key, value) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Full or blocked storage: the queue just won't survive a reload.
  }
}

const saved = (() => {
  try {
    return JSON.parse(load(TIME_KEY, 'null'))
  } catch {
    return null
  }
})()

export const player = $state({
  queue: Q.restore(load(QUEUE_KEY, null)),
  playing: false,
  loading: false,
  time: 0, // seconds into the current track
  duration: 0,
  seekable: true,
  volume: Math.min(1, Math.max(0, Number(load(VOLUME_KEY, '1')) || 0)),
  muted: false,
  ffmpeg: true, // as the server last said
})

{
  // Show the restored track as it was left, without loading it yet.
  const entry = Q.current(player.queue)
  player.duration = (entry?.durationMs ?? 0) / 1000
  if (entry && saved?.key === entry.key) player.time = saved.time
}

let active = null // the <audio> playing
let standby = null // the <audio> loading the next track
let errors = 0 // tracks that failed in a row
let startAt = 0 // where to start the track being loaded
let pass = P.begin(null) // the listening that may count as a play

const canPlayCache = {}

function canPlay(mime) {
  if (!(mime in canPlayCache)) canPlayCache[mime] = !!mime && active.canPlayType(mime) !== ''
  return canPlayCache[mime]
}

// source is how an entry is asked for: as it is, or converted when the
// browser cannot play its format (Safari and Ogg, for instance).
function source(entry) {
  const convert = !entry.transcoded && !canPlay(entry.mime) && player.ffmpeg
  return { url: streamURL(entry.id, convert), seekable: !entry.transcoded && !convert }
}

function makeAudio() {
  const a = new Audio()
  a.preload = 'auto'
  const mine = (fn) => () => a === active && fn()
  a.addEventListener('timeupdate', mine(onTime))
  a.addEventListener('loadedmetadata', mine(onMetadata))
  a.addEventListener('ended', mine(onEnded))
  a.addEventListener('play', mine(onPlay))
  a.addEventListener('pause', mine(onPause))
  a.addEventListener('waiting', mine(() => (player.loading = true)))
  a.addEventListener('playing', mine(() => ((player.loading = false), (errors = 0))))
  a.addEventListener('error', mine(onError))
  return a
}

function audios() {
  if (!active) {
    active = makeAudio()
    standby = makeAudio()
    applyVolume()
  }
}

function normalizing() {
  return app.status?.normalize !== false
}

function applyVolume() {
  for (const a of [active, standby]) {
    if (a) {
      a.volume = player.volume * (normalizing() ? Number(a.dataset.gain || 1) : 1)
      a.muted = player.muted
    }
  }
}

function persist() {
  save(QUEUE_KEY, JSON.stringify(player.queue))
}

let lastTimeSave = 0

function onTime() {
  player.time = active.currentTime
  pass = P.tick(pass, active.currentTime)
  if (P.due(pass)) {
    api.recordPlay({ id: pass.id, listenedMs: Math.round(pass.listenedMs), device: deviceName(navigator.userAgent) }).catch(() => {})
    pass = P.sent(pass)
  }
  const entry = Q.current(player.queue)
  if (Date.now() - lastTimeSave > 5000 && entry) {
    lastTimeSave = Date.now()
    save(TIME_KEY, JSON.stringify({ key: entry.key, time: player.time }))
  }
  if (player.duration - player.time < PRELOAD_SECONDS) preload()
  positionState()
}

function onMetadata() {
  const d = active.duration
  if (Number.isFinite(d) && d > 0) player.duration = d
  if (startAt > 0 && player.seekable) active.currentTime = startAt
  startAt = 0
}

function setPlaybackState(state) {
  if ('mediaSession' in navigator) navigator.mediaSession.playbackState = state
}

// release empties an element so that a fetch or transcode it had under way
// is cancelled.
function release(a) {
  a.removeAttribute('src')
  a.dataset.key = ''
  a.load()
}

function onPlay() {
  player.playing = true
  setPlaybackState('playing')
}

function onPause() {
  player.playing = false
  setPlaybackState('paused')
  const entry = Q.current(player.queue)
  if (entry) save(TIME_KEY, JSON.stringify({ key: entry.key, time: active.currentTime }))
}

function onEnded() {
  const q = Q.next(player.queue, true)
  if (!q) {
    player.playing = false
    player.time = 0
    if (player.queue.station) {
      // A station ran dry before its top-up arrived: ask again and go on.
      topUp().then(() => {
        if (player.playing) return
        const more = Q.next(player.queue, true)
        if (!more) return
        player.queue = more
        persist()
        start(Q.current(more), 0, true)
      })
    }
    return
  }
  player.queue = q
  persist()
  const entry = Q.current(q)
  if (standby.dataset.key === String(entry.key) && !standby.error) {
    // The next track is loaded and usable: swap the elements.
    ;[active, standby] = [standby, active]
    release(standby)
    show(entry, active.dataset.seekable === '1')
    refreshGains()
    topUp()
    // media errors arrive through the element's error event (mine); autoplay
    // refusal is ignored here and will pause the element if the user retries.
    active.play().catch(() => {})
    return
  }
  start(entry, 0, true)
}

function onError() {
  const entry = Q.current(player.queue)
  player.loading = false
  if (!entry) return
  notify(`No se pudo reproducir «${entry.title}».`)
  if (++errors >= 3) {
    player.playing = false
    return
  }
  const q = Q.next(player.queue)
  if (!q) {
    player.playing = false
    return
  }
  player.queue = q
  persist()
  start(Q.current(q), 0, true)
}

// show updates what the page shows for entry: duration, seekability, the
// lock screen.
function show(entry, seekable) {
  pass = P.begin(entry)
  player.duration = (entry.durationMs ?? 0) / 1000
  player.seekable = seekable
  player.time = 0
  mediaSession(entry)
}

// start loads entry in the active element, from at seconds, and plays it
// when play is true.
function start(entry, at, play) {
  audios()
  const src = source(entry)
  release(standby)
  active.src = src.url
  active.dataset.key = String(entry.key)
  active.dataset.seekable = src.seekable ? '1' : ''
  show(entry, src.seekable)
  player.time = at
  startAt = at
  player.loading = play
  active.dataset.gain = String(factor(gainDb(player.queue, player.queue.pos)))
  applyVolume()
  if (play) active.play().catch(() => (player.loading = false))
  refreshGains()
  topUp()
}

let topUpBusy = false // a request for more of the station is in flight
let topUpFailed = null // the station whose failure was already announced

// topUp asks the server for more of the station when the queue runs low.
// The answer is dropped when the station changed or ended meanwhile.
async function topUp() {
  const st = player.queue.station
  if (!st || topUpBusy || !S.needsMore(player.queue)) return
  topUpBusy = true
  let res
  try {
    res = await api.stationNext(st, S.recentIds(player.queue))
  } catch (e) {
    if (e.status === 404) {
      // The station's source is gone (a deleted playlist): it ends here.
      if (player.queue.station === st) {
        notify(e.message)
        player.queue = { ...player.queue, station: null }
        persist()
      }
      return
    }
    if (topUpFailed !== st) {
      topUpFailed = st
      notify(e.message)
    }
    return // the next call tries again
  } finally {
    topUpBusy = false
  }
  if (player.queue.station !== st) return
  topUpFailed = null
  player.ffmpeg = res.ffmpeg
  if (res.items.length === 0) {
    notify('La estación se quedó sin pistas.')
    player.queue = { ...player.queue, station: null }
    persist()
    return
  }
  player.queue = Q.trimPlayed(Q.append(player.queue, res.items), S.KEEP_PLAYED)
  persist()
  refreshGains()
}

// playStation starts a station: the queue is replaced with its first batch.
export async function playStation(st) {
  let res
  try {
    res = await api.stationNext(st, [])
  } catch (e) {
    notify(e.message)
    return
  }
  player.ffmpeg = res.ffmpeg
  if (res.items.length === 0) {
    notify('Esta estación no tiene nada para sonar.')
    return
  }
  // The server's cleaned station has no id: keep the saved station's.
  const station = res.station ? { ...res.station, id: st.id } : st
  setQueue(Q.replace({ ...player.queue, shuffle: false, station }, res.items))
}

// stopStation ends the station; the queue stays and plays out.
export function stopStation() {
  player.queue = { ...player.queue, station: null }
  persist()
}

// setStation replaces the playing station (its rules or name changed, or it
// was saved and has an id); the next top-up uses it.
export function setStation(st) {
  player.queue = { ...player.queue, station: st }
  persist()
}

// refreshGains asks for the gains of the next tracks that lack one. Errors
// are ignored: the next call tries again.
async function refreshGains() {
  const ids = missing(player.queue, player.queue.pos, 10)
  if (ids.length === 0) return
  let res
  try {
    res = await api.gains(ids)
  } catch {
    return
  }
  player.queue = withGains(player.queue, res) // the live queue: it may have changed meanwhile
  persist()
  const pos = Q.peekPos(player.queue)
  const entry = Q.peek(player.queue)
  if (standby && entry && standby.dataset.key === String(entry.key) && standby !== active) {
    standby.dataset.gain = String(factor(gainDb(player.queue, pos)))
    applyVolume()
  }
}

// preload starts loading the track that comes next, once.
function preload() {
  const entry = Q.peek(player.queue)
  if (!entry || entry.key === Q.current(player.queue)?.key || standby.dataset.key === String(entry.key)) return
  const src = source(entry)
  standby.src = src.url
  standby.dataset.key = String(entry.key)
  standby.dataset.seekable = src.seekable ? '1' : ''
  standby.dataset.gain = String(factor(gainDb(player.queue, Q.peekPos(player.queue))))
  applyVolume()
  standby.load()
}

function setQueue(q, play = true) {
  player.queue = q
  persist()
  const entry = Q.current(q)
  if (entry) start(entry, 0, play)
  else refreshGains()
}

// play asks the server for what to play ({album}, {folder}, {tracks}) and
// plays it: from the track with id startId, or shuffled.
export async function play(what, { shuffle = false, startId = null } = {}) {
  let res
  try {
    res = await api.play(what) // the queue does the shuffling
  } catch (e) {
    notify(e.message)
    return
  }
  player.ffmpeg = res.ffmpeg
  if (res.skipped > 0) notify(`${count(res.skipped, 'pista no se puede', 'pistas no se pueden')} reproducir: falta ffmpeg.`)
  if (res.items.length === 0) {
    if (res.skipped === 0) notify('No hay nada para reproducir acá.')
    return
  }
  if (res.truncated > 0) notify(`Se agregaron las primeras ${count(res.items.length, 'pista', 'pistas')}.`)
  const at = startId == null ? 0 : Math.max(0, res.items.findIndex((it) => it.id === startId))
  setQueue(Q.replace({ ...player.queue, shuffle, station: null }, res.items, shuffle ? Math.floor(Math.random() * res.items.length) : at))
}

// enqueue adds what to the end of the queue, or right after the current
// track with next: true.
export async function enqueue(what, { next = false } = {}) {
  let res
  try {
    res = await api.play(what)
  } catch (e) {
    notify(e.message)
    return
  }
  player.ffmpeg = res.ffmpeg
  if (res.items.length === 0) {
    if (res.skipped === 0) notify('No hay nada para reproducir acá.')
    return
  }
  const wasEmpty = !Q.current(player.queue)
  player.queue = next ? Q.playNext(player.queue, res.items) : Q.append(player.queue, res.items)
  persist()
  if (res.truncated > 0) notify(`Se agregaron las primeras ${count(res.items.length, 'pista', 'pistas')}.`)
  else notify(next ? 'Suena a continuación.' : `${count(res.items.length, 'pista agregada', 'pistas agregadas')} a la cola.`)
  if (wasEmpty) start(Q.current(player.queue), 0, false)
  else refreshGains()
}

if (player.queue.list.length > 0) refreshGains()
topUp() // a restored station may be running low

export function toggle() {
  const entry = Q.current(player.queue)
  if (!entry) return
  audios()
  if (active.dataset.key !== String(entry.key) || active.error) {
    // after a reload or after a failure: start fresh with error counter reset
    errors = 0
    start(entry, player.time, true)
    return
  }
  if (active.paused) active.play().catch(() => {})
  else active.pause()
}

export function next() {
  const q = Q.next(player.queue)
  if (q) setQueue(q, true)
}

export function prev() {
  if (active && active.currentTime > RESTART_SECONDS && player.seekable) {
    active.currentTime = 0
    return
  }
  setQueue(Q.prev(player.queue), true)
}

export function jumpTo(pos) {
  setQueue(Q.jump(player.queue, pos), true)
}

export function removeAt(pos) {
  const wasCurrent = pos === player.queue.pos
  player.queue = Q.remove(player.queue, pos)
  persist()
  if (!wasCurrent) return topUp()
  const entry = Q.current(player.queue)
  if (entry) start(entry, 0, player.playing)
  else stop()
}

// moveTo moves an upcoming entry from one place of the play order to another.
export function moveTo(from, to) {
  player.queue = Q.move(player.queue, from, to)
  persist()
}

export function clearUpcoming() {
  player.queue = Q.clearUpcoming(player.queue)
  persist()
  if (standby) release(standby)
  topUp()
}

function stop() {
  if (active) {
    active.pause()
    release(active)
  }
  player.playing = false
  player.time = 0
  player.duration = 0
}

export function seek(seconds) {
  if (!active || !player.seekable) return
  active.currentTime = Math.max(0, Math.min(seconds, player.duration || seconds))
  player.time = active.currentTime
}

export function setVolume(v) {
  player.volume = Math.min(1, Math.max(0, v))
  player.muted = false
  save(VOLUME_KEY, String(player.volume))
  applyVolume()
}

export function toggleMute() {
  player.muted = !player.muted
  applyVolume()
}

export function toggleShuffle() {
  player.queue = Q.setShuffle(player.queue, !player.queue.shuffle)
  persist()
}

export function cycleRepeat() {
  player.queue = Q.cycleRepeat(player.queue)
  persist()
}

// mediaSession shows the track on the lock screen and in the system's
// media controls, which also work with headphones and media keys.
function mediaSession(entry) {
  if (!('mediaSession' in navigator)) return
  navigator.mediaSession.metadata = new MediaMetadata({
    title: entry.title,
    artist: entry.artist,
    album: entry.album,
    artwork: [
      { src: location.origin + itemCoverURL(entry, 320), sizes: '320x320', type: 'image/jpeg' },
      { src: location.origin + itemCoverURL(entry, 640), sizes: '640x640', type: 'image/jpeg' },
    ],
  })
}

let lastPosition = 0

function positionState() {
  if (!('mediaSession' in navigator) || !navigator.mediaSession.setPositionState) return
  if (Date.now() - lastPosition < 1000 || !(player.duration > 0)) return
  lastPosition = Date.now()
  try {
    navigator.mediaSession.setPositionState({
      duration: player.duration,
      position: Math.min(player.time, player.duration),
      playbackRate: 1,
    })
  } catch {
    // Some browsers reject a position past a duration they computed.
  }
}

if ('mediaSession' in navigator) {
  const handlers = {
    play: () => player.playing || toggle(),
    pause: () => player.playing && toggle(),
    previoustrack: prev,
    nexttrack: next,
    seekto: (d) => seek(d.seekTime),
  }
  for (const [action, fn] of Object.entries(handlers)) {
    try {
      navigator.mediaSession.setActionHandler(action, fn)
    } catch {
      // An action this browser does not support.
    }
  }
}
