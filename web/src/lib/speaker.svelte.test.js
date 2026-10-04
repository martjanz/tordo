import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./api.js', () => ({
  api: { jukeboxState: vi.fn(() => Promise.resolve(null)), jukeboxNotice: vi.fn(() => Promise.resolve(null)) },
}))
vi.mock('./app.svelte.js', () => ({ notify: vi.fn() }))
vi.mock('./player.svelte.js', () => ({
  player: {
    queue: { list: [], order: [], pos: -1, shuffle: false, repeat: 'off', station: null },
    time: 0,
    duration: 0,
    playing: false,
    seekable: true,
    volume: 1,
    muted: false,
    blocked: false,
  },
  unblock: vi.fn(),
  toggle: vi.fn(),
  next: vi.fn(),
}))

import { api } from './api.js'
import { notify } from './app.svelte.js'
import * as P from './player.svelte.js'
import { disableSpeaker, enableSpeaker, resumeSpeaker, speaker } from './speaker.svelte.js'

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
    for (const fn of this.listeners[type] ?? []) fn({ data: JSON.stringify(data) })
  }
  close() {
    this.closed = true
    this.readyState = 2
  }
}

const store = new Map()
vi.stubGlobal('localStorage', {
  getItem: (k) => store.get(k) ?? null,
  setItem: (k, v) => store.set(k, String(v)),
  clear: () => store.clear(),
})

let sources
const connect = vi.fn((url) => {
  const s = new FakeSource(url)
  sources.push(s)
  return s
})
const last = () => sources[sources.length - 1]

beforeEach(() => {
  vi.useFakeTimers()
  sources = []
  connect.mockClear()
  vi.clearAllMocks()
  localStorage.clear()
  P.player.blocked = false
})
afterEach(() => {
  disableSpeaker()
  vi.useRealTimers()
})

describe('enableSpeaker', () => {
  it('connects, remembers, and publishes once the hello arrives', async () => {
    enableSpeaker(connect)
    expect(connect).toHaveBeenCalledWith('/api/jukebox/speaker')
    expect(localStorage.getItem('tordo.audio-compu')).toBe('1')
    expect(speaker.on).toBe(true)
    expect(speaker.connected).toBe(false)

    last().emit('hello', { token: 't1' })
    expect(speaker.connected).toBe(true)
    expect(api.jukeboxState).toHaveBeenCalledWith('t1', expect.objectContaining({ playing: false }))
  })
})

describe('commands', () => {
  it('runs a host command on the player', async () => {
    enableSpeaker(connect)
    last().emit('hello', { token: 't1' })
    last().emit('command', { command: 'next', role: 'host', session: 0, remote: 'r1' })
    await vi.advanceTimersByTimeAsync(0)
    expect(P.next).toHaveBeenCalled()
  })

  it('tells a guest that only the host can do it', async () => {
    enableSpeaker(connect)
    last().emit('hello', { token: 't1' })
    last().emit('command', { command: 'next', role: 'guest', session: 5, remote: 'r1' })
    await vi.advanceTimersByTimeAsync(0)
    expect(P.next).not.toHaveBeenCalled()
    expect(api.jukeboxNotice).toHaveBeenCalledWith('t1', 'r1', 'Solo el anfitrión puede hacer esto.')
  })
})

describe('replaced', () => {
  it('turns the speaker off and says why', () => {
    enableSpeaker(connect)
    last().emit('hello', { token: 't1' })
    last().emit('replaced', {})
    expect(speaker.on).toBe(false)
    expect(speaker.connected).toBe(false)
    expect(localStorage.getItem('tordo.audio-compu')).toBe('')
    expect(last().closed).toBe(true)
    expect(notify).toHaveBeenCalledWith('Otro navegador tomó el audio de esta compu.')
  })
})

describe('errors', () => {
  it('retries a closed source after 1 s, then 2 s', async () => {
    enableSpeaker(connect)
    const first = last()
    first.readyState = 2
    first.emit('error', {})
    expect(speaker.connected).toBe(false)
    expect(connect).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(999)
    expect(connect).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(connect).toHaveBeenCalledTimes(2)

    last().readyState = 2
    last().emit('error', {})
    await vi.advanceTimersByTimeAsync(1999)
    expect(connect).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1)
    expect(connect).toHaveBeenCalledTimes(3)
  })

  it('does nothing while the browser is reconnecting by itself', async () => {
    enableSpeaker(connect)
    last().readyState = 0
    last().emit('error', {})
    expect(speaker.connected).toBe(false)
    await vi.advanceTimersByTimeAsync(60000)
    expect(connect).toHaveBeenCalledTimes(1)
  })
})

describe('resumeSpeaker', () => {
  it('does not connect when nothing is remembered', () => {
    resumeSpeaker(connect)
    expect(connect).not.toHaveBeenCalled()
    expect(speaker.on).toBe(false)
  })

  it('reconnects when remembered, with the player blocked', () => {
    localStorage.setItem('tordo.audio-compu', '1')
    resumeSpeaker(connect)
    expect(connect).toHaveBeenCalledWith('/api/jukebox/speaker')
    expect(speaker.on).toBe(true)
    expect(P.player.blocked).toBe(true)
  })
})

describe('disableSpeaker', () => {
  it('closes the source and stops publishing', async () => {
    enableSpeaker(connect)
    last().emit('hello', { token: 't1' })
    P.player.playing = true
    api.jukeboxState.mockClear()
    disableSpeaker()
    expect(last().closed).toBe(true)
    expect(speaker.on).toBe(false)
    expect(localStorage.getItem('tordo.audio-compu')).toBe('')
    await vi.advanceTimersByTimeAsync(5000)
    expect(api.jukeboxState).not.toHaveBeenCalled()
    P.player.playing = false
  })
})

describe('publishing and tokens', () => {
  const rejection = (status) => Object.assign(new Error('x'), { status })

  it('a 409 on the current token disables the speaker', async () => {
    api.jukeboxState.mockRejectedValueOnce(rejection(409))
    enableSpeaker(connect)
    last().emit('hello', { token: 't1' })
    await vi.advanceTimersByTimeAsync(0)
    expect(speaker.on).toBe(false)
    expect(localStorage.getItem('tordo.audio-compu')).toBe('')
  })

  it('does not publish while disconnected', async () => {
    enableSpeaker(connect)
    last().emit('hello', { token: 't1' })
    last().readyState = 0
    last().emit('error', {})
    P.player.playing = true
    api.jukeboxState.mockClear()
    await vi.advanceTimersByTimeAsync(5000)
    expect(api.jukeboxState).not.toHaveBeenCalled()
    expect(speaker.on).toBe(true)
    P.player.playing = false
  })

  it('a 409 for a stale token after a new hello does not disable', async () => {
    let fail
    api.jukeboxState.mockImplementationOnce(() => new Promise((_, rej) => (fail = rej)))
    enableSpeaker(connect)
    last().emit('hello', { token: 't1' })
    last().emit('hello', { token: 't2' })
    fail(rejection(409))
    await vi.advanceTimersByTimeAsync(0)
    expect(speaker.on).toBe(true)
  })
})
