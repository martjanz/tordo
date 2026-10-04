import { describe, it, expect, vi } from 'vitest'
import { ROLES, KEEP_PLAYED, KEEP_NEXT, allowed, cleanArgs, snapshot, run } from './jukebox.js'

// The Go table's keys (internal/jukebox/jukebox.go, commands).
const GO_COMMANDS = [
  'enqueue', 'toggle', 'removeAt',
  'play', 'playStation', 'stopStation', 'setStation',
  'next', 'prev', 'jumpTo', 'moveTo', 'clearUpcoming',
  'seek', 'setVolume', 'toggleMute', 'toggleShuffle', 'cycleRepeat',
]
const GUEST = ['enqueue', 'toggle', 'removeAt']

describe('ROLES / allowed', () => {
  it('ignores names inherited from the prototype', () => {
    expect(allowed('host', 'toString')).toBe(false)
    expect(cleanArgs('toString', {})).toBeNull()
  })

  it('has the same keys as the Go table', () => {
    expect(Object.keys(ROLES).sort()).toEqual([...GO_COMMANDS].sort())
    for (const c of GUEST) expect(ROLES[c]).toBe('guest')
    for (const c of GO_COMMANDS.filter((c) => !GUEST.includes(c))) expect(ROLES[c]).toBe('host')
  })

  it('lets both roles send guest commands', () => {
    for (const c of GUEST) {
      expect(allowed('guest', c)).toBe(true)
      expect(allowed('host', c)).toBe(true)
    }
  })

  it('keeps host commands for the host', () => {
    expect(allowed('guest', 'next')).toBe(false)
    expect(allowed('host', 'next')).toBe(true)
  })

  it('refuses unknown commands', () => {
    expect(allowed('host', 'nope')).toBe(false)
    expect(allowed('guest', 'nope')).toBe(false)
  })
})

describe('cleanArgs', () => {
  it('requires a valid what for play', () => {
    expect(cleanArgs('play', {})).toBeNull()
    expect(cleanArgs('play', { what: { evil: 1 } })).toBeNull()
    expect(cleanArgs('play', { what: { album: 3 } })).toEqual({ what: { album: 3 }, shuffle: false, startId: null })
  })

  it('drops unknown keys and bad tracks', () => {
    expect(cleanArgs('enqueue', { what: { album: 3, evil: 1 } }).what).toEqual({ album: 3 })
    expect(cleanArgs('enqueue', { what: { tracks: [1, 'x'], album: 2 } }).what).toEqual({ album: 2 })
    expect(cleanArgs('enqueue', { what: { tracks: [1, 2] }, next: true })).toEqual({
      what: { tracks: [1, 2] }, next: true, shuffle: false, startIfIdle: false,
    })
  })

  it('validates numbers and keys', () => {
    expect(cleanArgs('seek', { seconds: -1 })).toBeNull()
    expect(cleanArgs('seek', { seconds: 12 })).toEqual({ seconds: 12 })
    expect(cleanArgs('setVolume', { volume: 1.5 })).toBeNull()
    expect(cleanArgs('setVolume', { volume: 0.5 })).toEqual({ volume: 0.5 })
    expect(cleanArgs('jumpTo', { key: 0 })).toBeNull()
    expect(cleanArgs('jumpTo', { key: 4 })).toEqual({ key: 4 })
    expect(cleanArgs('moveTo', { key: 1 })).toBeNull()
    expect(cleanArgs('moveTo', { key: 1, toKey: 2 })).toEqual({ key: 1, toKey: 2 })
  })

  it('needs a station object', () => {
    expect(cleanArgs('playStation', {})).toBeNull()
    expect(cleanArgs('setStation', { station: { id: 1 } })).toEqual({ station: { id: 1 } })
  })

  it('ignores junk on argument-less commands and rejects unknown ones', () => {
    expect(cleanArgs('toggle', 'junk')).toEqual({})
    expect(cleanArgs('toggle', { a: 1 })).toEqual({})
    expect(cleanArgs('nope', {})).toBeNull()
  })
})

describe('snapshot', () => {
  const mk = (n, pos) => ({
    queue: {
      list: Array.from({ length: n }, (_, i) => ({ key: i + 1 })),
      order: Array.from({ length: n }, (_, i) => i + 1),
      pos,
      shuffle: true,
      repeat: 'all',
      station: null,
    },
    time: 5, duration: 200, playing: true, seekable: true, volume: 0.7, muted: false, blocked: true,
  })

  it('trims around the current entry', () => {
    const s = snapshot(mk(300, 150))
    expect(KEEP_PLAYED).toBe(10)
    expect(KEEP_NEXT).toBe(100)
    expect(s.queue.order).toHaveLength(111)
    expect(s.queue.pos).toBe(10)
    expect(s.queue.upcoming).toBe(149)
    expect(s.queue.list).toHaveLength(111)
    expect(s.queue.list.every((e) => s.queue.order.includes(e.key))).toBe(true)
    expect(s.queue.order[s.queue.pos]).toBe(151)
  })

  it('shows the head when nothing is current', () => {
    const s = snapshot(mk(300, -1))
    expect(s.queue.order).toHaveLength(100)
    expect(s.queue.pos).toBe(-1)
    expect(s.queue.upcoming).toBe(300)
  })

  it('carries the player state', () => {
    const s = snapshot(mk(3, 0))
    expect(s).toMatchObject({ time: 5, duration: 200, playing: true, seekable: true, volume: 0.7, muted: false, blocked: true })
    expect(s.queue).toMatchObject({ shuffle: true, repeat: 'all', station: null, upcoming: 2 })
  })
})

describe('run', () => {
  function fakeP() {
    const entries = [
      { key: 11, addedBy: 7 },
      { key: 12, addedBy: 8 },
      { key: 13, addedBy: null },
    ]
    const P = {
      posOfKey: (k) => entries.findIndex((e) => e.key === k),
      entryOfKey: (k) => entries.find((e) => e.key === k) || null,
    }
    for (const f of [
      'play', 'enqueue', 'playStation', 'setStation', 'stopStation', 'toggle', 'next', 'prev', 'jumpTo',
      'moveTo', 'removeAt', 'clearUpcoming', 'seek', 'setVolume', 'toggleMute', 'toggleShuffle', 'cycleRepeat',
    ]) P[f] = vi.fn()
    return P
  }

  it('refuses host commands from a guest', async () => {
    const P = fakeP()
    const say = vi.fn()
    await run({ command: 'next', args: {}, session: 7, role: 'guest' }, P, say)
    expect(say).toHaveBeenCalledWith('Solo el anfitrión puede hacer esto.')
    expect(P.next).not.toHaveBeenCalled()
  })

  it('runs host commands for the host', async () => {
    const P = fakeP()
    await run({ command: 'next', args: {}, session: 0, role: 'host' }, P, vi.fn())
    expect(P.next).toHaveBeenCalledTimes(1)
  })

  it('passes addedBy for a guest enqueue and null for the computer', async () => {
    const P = fakeP()
    const say = vi.fn()
    await run({ command: 'enqueue', args: { what: { album: 3 }, next: true }, session: 7, role: 'guest' }, P, say)
    expect(P.enqueue).toHaveBeenCalledWith({ album: 3 }, { next: true, shuffle: false, startIfIdle: false, addedBy: 7, say })
    await run({ command: 'enqueue', args: { what: { album: 3 } }, session: 0, role: 'host' }, P, say)
    expect(P.enqueue).toHaveBeenLastCalledWith({ album: 3 }, { next: false, shuffle: false, startIfIdle: false, addedBy: null, say })
  })

  it('lets a guest remove only what they added', async () => {
    const P = fakeP()
    const say = vi.fn()
    await run({ command: 'removeAt', args: { key: 11 }, session: 7, role: 'guest' }, P, say)
    expect(P.removeAt).toHaveBeenCalledWith(0)
    await run({ command: 'removeAt', args: { key: 11 }, session: 8, role: 'guest' }, P, say)
    expect(say).toHaveBeenCalledWith('Solo podés quitar lo que agregaste.')
    expect(P.removeAt).toHaveBeenCalledTimes(1)
  })

  it('lets the host remove anything', async () => {
    const P = fakeP()
    await run({ command: 'removeAt', args: { key: 12 }, session: 0, role: 'host' }, P, vi.fn())
    expect(P.removeAt).toHaveBeenCalledWith(1)
  })

  it('ignores unknown keys', async () => {
    const P = fakeP()
    const say = vi.fn()
    await run({ command: 'jumpTo', args: { key: 99 }, session: 0, role: 'host' }, P, say)
    await run({ command: 'removeAt', args: { key: 99 }, session: 0, role: 'host' }, P, say)
    expect(P.jumpTo).not.toHaveBeenCalled()
    expect(P.removeAt).not.toHaveBeenCalled()
    expect(say).not.toHaveBeenCalled()
  })

  it('jumps to a known key', async () => {
    const P = fakeP()
    await run({ command: 'jumpTo', args: { key: 13 }, session: 0, role: 'host' }, P, vi.fn())
    expect(P.jumpTo).toHaveBeenCalledWith(2)
  })

  it('moves by keys for the host only', async () => {
    const P = fakeP()
    const say = vi.fn()
    await run({ command: 'moveTo', args: { key: 13, toKey: 11 }, session: 0, role: 'host' }, P, say)
    expect(P.moveTo).toHaveBeenCalledWith(2, 0)
    await run({ command: 'moveTo', args: { key: 13, toKey: 11 }, session: 7, role: 'guest' }, P, say)
    expect(P.moveTo).toHaveBeenCalledTimes(1)
    expect(say).toHaveBeenCalledWith('Solo el anfitrión puede hacer esto.')
  })

  it('does nothing with invalid arguments', async () => {
    const P = fakeP()
    const say = vi.fn()
    await run({ command: 'seek', args: { seconds: -3 }, session: 0, role: 'host' }, P, say)
    await run({ command: 'play', args: {}, session: 0, role: 'host' }, P, say)
    await run({ command: 'moveTo', args: { key: 11 }, session: 0, role: 'host' }, P, say)
    for (const f of ['seek', 'play', 'moveTo']) expect(P[f]).not.toHaveBeenCalled()
    expect(say).not.toHaveBeenCalled()
  })

  it('forwards seek, volume and plain commands', async () => {
    const P = fakeP()
    await run({ command: 'seek', args: { seconds: 30 }, session: 0, role: 'host' }, P, vi.fn())
    await run({ command: 'setVolume', args: { volume: 0.4 }, session: 0, role: 'host' }, P, vi.fn())
    await run({ command: 'toggle', args: {}, session: 7, role: 'guest' }, P, vi.fn())
    expect(P.seek).toHaveBeenCalledWith(30)
    expect(P.setVolume).toHaveBeenCalledWith(0.4)
    expect(P.toggle).toHaveBeenCalledTimes(1)
  })
})
