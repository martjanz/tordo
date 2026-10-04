// Rockola mode on a phone (or any browser that is not the computer's speaker):
// the bar shows the computer's published state and commands go to the server.
import { ApiError, api } from './api.js'
import { app, notify } from './app.svelte.js'
import { checkAuth } from './auth.svelte.js'
import * as P from './player.svelte.js'

const initial = () => ({ on: false, connected: false, remote: null, session: 0, role: 'guest', view: null })

export const rockola = $state(initial())
export const rockolaAvailable = () => !!app.status?.jukebox?.speaker

const LOST = 'Se perdió la conexión con la rockola.'
const UNAVAILABLE = 'La rockola no está disponible: activá el audio en la compu.'

let source = null
let helloSeen = false // a hello arrived and no state yet: the first speaker-off means "not available"

const parse = (e) => {
  try {
    return JSON.parse(e.data) ?? {}
  } catch {
    return {}
  }
}

function close() {
  if (source) source.close()
  source = null
  helloSeen = false
}

export function setRockola(on, connect = (u) => new EventSource(u)) {
  if (!on) {
    close()
    Object.assign(rockola, initial())
    return
  }
  if (rockola.on) return
  if (P.player.playing) P.toggle()
  rockola.on = true
  const src = connect('/api/jukebox/events')
  source = src
  src.addEventListener('hello', (e) => {
    const d = parse(e)
    rockola.remote = d.remote
    rockola.session = d.session
    rockola.role = d.role
    rockola.connected = true
    helloSeen = true
  })
  src.addEventListener('state', (e) => {
    helloSeen = false
    rockola.view = parse(e)
  })
  src.addEventListener('notice', (e) => notify(parse(e).text))
  src.addEventListener('role', (e) => {
    rockola.role = parse(e).role
  })
  src.addEventListener('speaker-off', () => {
    notify(helloSeen ? UNAVAILABLE : LOST)
    setRockola(false)
  })
  src.addEventListener('error', () => {
    if (src.readyState === 0) {
      rockola.connected = false // the browser reconnects; the next hello brings a new remote
    } else if (src.readyState === 2) {
      notify(LOST)
      setRockola(false)
      checkAuth()
    }
  })
}

export async function send(command, args = {}) {
  try {
    await api.jukeboxCommand(rockola.remote, command, args)
  } catch (e) {
    if (!(e instanceof ApiError)) return notify(e.message)
    if (e.status === 409) {
      notify(e.message)
      setRockola(false)
    } else if (e.status !== 404) {
      notify(e.message)
    }
  }
}
