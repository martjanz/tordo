// "Audio de esta compu": this browser tab plays the Rockola. It listens to the
// server's commands, runs them on the player and publishes the player's state
// so the phones can see it.
import { api } from './api.js'
import { notify } from './app.svelte.js'
import { run, snapshot } from './jukebox.js'
import * as P from './player.svelte.js'
import { current } from './queue.js'

const KEY = 'tordo.audio-compu'
const PUBLISH_DELAY = 250 // ms: state changes are batched
const MAX_BACKOFF = 30000

export const speaker = $state({ on: false, connected: false })

let source = null
let token = ''
let backoff = 1000
let retry = null
let pending = null
let ticker = null

function remember(on) {
  try {
    localStorage.setItem(KEY, on ? '1' : '')
  } catch {
    // storage unavailable: the switch just is not remembered
  }
}

function remembered() {
  try {
    return localStorage.getItem(KEY) === '1'
  } catch {
    return false
  }
}

// publish sends the whole state. A 409 means another tab took over.
function publish() {
  clearTimeout(pending)
  pending = null
  if (!speaker.on || !token) return
  const sent = token
  api.jukeboxState(sent, snapshot(P.player)).catch((e) => {
    // only the current connection's token counts: a late answer for an old one is not news
    if (e?.status === 409 && sent === token) disableSpeaker()
  })
}

function schedulePublish() {
  if (pending !== null) return
  pending = setTimeout(publish, PUBLISH_DELAY)
}

// Everything but the time moves the state; the time goes out every second
// while playing.
$effect.root(() => {
  $effect(() => {
    if (!speaker.on) return
    // time is read as a constant so the effect does not follow every tick
    const p = P.player
    JSON.stringify(snapshot({ queue: p.queue, time: 0, duration: p.duration, playing: p.playing, seekable: p.seekable, volume: p.volume, muted: p.muted, blocked: p.blocked }))
    schedulePublish()
  })
})

function open(connect) {
  const es = connect('/api/jukebox/speaker')
  source = es
  const data = (e) => JSON.parse(e.data)
  es.addEventListener('hello', (e) => {
    token = data(e).token
    speaker.connected = true
    backoff = 1000
    publish()
  })
  es.addEventListener('command', (e) => {
    const cmd = data(e)
    run(cmd, P, (text) => api.jukeboxNotice(token, cmd.remote, text).catch(() => {}))
  })
  es.addEventListener('replaced', () => {
    disableSpeaker()
    notify('Otro navegador tomó el audio de esta compu.')
  })
  es.addEventListener('error', () => {
    if (source !== es) return
    speaker.connected = false
    token = '' // the server forgot this speaker: nothing to publish until the next hello
    if (es.readyState !== 2 || !speaker.on) return // 0: the browser is reconnecting by itself
    es.close()
    retry = setTimeout(() => {
      retry = null
      if (speaker.on) open(connect)
    }, backoff)
    backoff = Math.min(backoff * 2, MAX_BACKOFF)
  })
}

function start(connect) {
  stop()
  speaker.on = true
  backoff = 1000
  ticker = setInterval(() => {
    if (P.player.playing) publish()
  }, 1000)
  open(connect)
}

function stop() {
  clearTimeout(retry)
  clearTimeout(pending)
  clearInterval(ticker)
  retry = pending = ticker = null
  source?.close()
  source = null
  token = ''
  speaker.on = speaker.connected = false
}

// enableSpeaker is the switch turned on (a click: the browser allows audio).
export function enableSpeaker(connect = (u) => new EventSource(u)) {
  remember(true)
  start(connect)
}

export function disableSpeaker() {
  remember(false)
  stop()
}

// resumeSpeaker reconnects at start when it was on. The browser has not had
// a gesture yet, so the player starts blocked until "Reactivar audio".
export function resumeSpeaker(connect = (u) => new EventSource(u)) {
  if (!remembered()) return
  P.player.blocked = true
  start(connect)
}

// reactivate is the gesture that lets the browser play again.
export function reactivate() {
  P.unblock()
  if (current(P.player.queue) && !P.player.playing) P.toggle()
}
