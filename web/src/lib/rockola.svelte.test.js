import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./api.js', async (orig) => {
  const real = await orig()
  return { ...real, api: { jukeboxCommand: vi.fn(() => Promise.resolve(null)) } }
})
vi.mock('./app.svelte.js', () => ({ notify: vi.fn(), app: { status: { jukebox: { speaker: true } } } }))
vi.mock('./auth.svelte.js', () => ({ checkAuth: vi.fn() }))
vi.mock('./player.svelte.js', () => ({ player: { playing: false }, toggle: vi.fn(), setMediaKeys: vi.fn() }))

import { ApiError, api } from './api.js'
import { msg, setLocale, text } from './i18n/index.svelte.js'
import { notify } from './app.svelte.js'
import { checkAuth } from './auth.svelte.js'
import * as P from './player.svelte.js'
import { rockola, rockolaAvailable, send, setRockola } from './rockola.svelte.js'

class FakeSource {
  constructor(url) {
    this.url = url
    this.listeners = {}
    this.readyState = 1
    this.closed = false
  }
  addEventListener(type, fn) {
    ;(this.listeners[type] ??= []).push(fn)
  }
  emit(type, data) {
    for (const fn of this.listeners[type] ?? []) fn({ data: JSON.stringify(data ?? {}) })
  }
  close() {
    this.closed = true
    this.readyState = 2
  }
}

let sources
const connect = vi.fn((url) => {
  const s = new FakeSource(url)
  sources.push(s)
  return s
})
const last = () => sources[sources.length - 1]
const hello = (role = 'guest') => last().emit('hello', { remote: 'r1', session: 7, role })

beforeEach(() => {
  setRockola(false)
  sources = []
  vi.clearAllMocks()
  P.player.playing = false
})

describe('rockola', () => {
  it('is available when the status says there is a speaker', () => {
    expect(rockolaAvailable()).toBe(true)
  })

  it('pauses a playing local player and connects to the events', () => {
    P.player.playing = true
    setRockola(true, connect)
    expect(P.toggle).toHaveBeenCalled()
    expect(rockola.on).toBe(true)
    expect(last().url).toBe('/api/jukebox/events')
  })

  it('turns the media keys off on enter and on again on leave', () => {
    setRockola(true, connect)
    expect(P.setMediaKeys).toHaveBeenLastCalledWith(false)
    setRockola(false)
    expect(P.setMediaKeys).toHaveBeenLastCalledWith(true)
  })

  it('does not toggle a paused player', () => {
    setRockola(true, connect)
    expect(P.toggle).not.toHaveBeenCalled()
  })

  it('fills rockola from hello, state and role', () => {
    setRockola(true, connect)
    hello()
    expect(rockola).toMatchObject({ connected: true, remote: 'r1', session: 7, role: 'guest' })
    last().emit('state', { playing: true })
    expect(rockola.view).toEqual({ playing: true })
    last().emit('role', { role: 'host' })
    expect(rockola.role).toBe('host')
  })

  it('shows notices', () => {
    setRockola(true, connect)
    last().emit('notice', { notice: 'texto' })
    expect(notify).toHaveBeenCalledWith('texto')
  })

  it('translates a notice key in the own language of the phone', () => {
    setLocale('en')
    try {
      setRockola(true, connect)
      last().emit('notice', { notice: { key: 'queue.next' } })
      expect(notify).toHaveBeenCalledWith({ key: 'queue.next' })
      expect(text(notify.mock.calls[0][0])).toBe('Playing next.') // what notify stores on this phone
    } finally {
      setLocale('es')
    }
  })

  it('turns off with "no está disponible" on a speaker-off right after hello', () => {
    setRockola(true, connect)
    const s = last()
    hello()
    s.emit('speaker-off')
    expect(notify).toHaveBeenCalledWith(msg('error.no_speaker'))
    expect(rockola.on).toBe(false)
    expect(s.closed).toBe(true)
  })

  it('turns off with "se perdió" on a later speaker-off', () => {
    setRockola(true, connect)
    hello()
    last().emit('state', {})
    last().emit('speaker-off')
    expect(notify).toHaveBeenCalledWith(msg('rockola.lost'))
    expect(rockola.on).toBe(false)
    expect(rockola.view).toBe(null)
  })

  it('marks disconnected while the browser reconnects', () => {
    setRockola(true, connect)
    hello()
    last().readyState = 0
    last().emit('error')
    expect(rockola.connected).toBe(false)
    expect(rockola.on).toBe(true)
  })

  it('turns off and rechecks the session when the source is closed', () => {
    setRockola(true, connect)
    hello()
    last().readyState = 2
    last().emit('error')
    expect(notify).toHaveBeenCalledWith(msg('rockola.lost'))
    expect(rockola.on).toBe(false)
    expect(checkAuth).toHaveBeenCalled()
  })
})

describe('send', () => {
  beforeEach(() => {
    setRockola(true, connect)
    hello('host')
  })

  it('posts the command with the remote id', async () => {
    await send('seek', { seconds: 3 })
    expect(api.jukeboxCommand).toHaveBeenCalledWith('r1', 'seek', { seconds: 3 })
  })

  it('on 409 notifies and turns off', async () => {
    api.jukeboxCommand.mockRejectedValueOnce(new ApiError(409, 'sin rockola'))
    await send('toggle')
    expect(notify).toHaveBeenCalledWith('sin rockola')
    expect(rockola.on).toBe(false)
  })

  it('on 403 notifies and stays on', async () => {
    api.jukeboxCommand.mockRejectedValueOnce(new ApiError(403, 'Solo el anfitrión puede hacer esto.'))
    await send('next')
    expect(notify).toHaveBeenCalledWith('Solo el anfitrión puede hacer esto.')
    expect(rockola.on).toBe(true)
  })

  it('on 404 does nothing', async () => {
    api.jukeboxCommand.mockRejectedValueOnce(new ApiError(404, 'x'))
    await send('next')
    expect(notify).not.toHaveBeenCalled()
    expect(rockola.on).toBe(true)
  })
})
