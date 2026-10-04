import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./player.svelte.js', () => {
  const f = () => vi.fn()
  return {
    player: { queue: { order: [] } },
    play: f(), enqueue: f(), playStation: f(), stopStation: f(), setStation: f(), toggle: f(), next: f(), prev: f(),
    jumpTo: f(), moveTo: f(), removeAt: f(), clearUpcoming: f(), seek: f(), setVolume: f(), toggleMute: f(),
    toggleShuffle: f(), cycleRepeat: f(),
  }
})
vi.mock('./rockola.svelte.js', () => ({
  rockola: { on: false, role: 'guest', session: 3, view: null },
  send: vi.fn(),
}))

import * as B from './playback.svelte.js'
import * as P from './player.svelte.js'
import { rockola, send } from './rockola.svelte.js'

beforeEach(() => {
  vi.clearAllMocks()
  Object.assign(rockola, { on: false, role: 'guest', session: 3, view: null })
})

describe('playback, local', () => {
  it('forwards every function to the player with the same arguments', () => {
    B.play({ album: 1 }, { shuffle: true })
    expect(P.play).toHaveBeenCalledWith({ album: 1 }, { shuffle: true })
    B.enqueue({ album: 1 }, { next: true })
    expect(P.enqueue).toHaveBeenCalledWith({ album: 1 }, { next: true })
    B.playStation('s')
    expect(P.playStation).toHaveBeenCalledWith('s')
    B.setStation('s')
    expect(P.setStation).toHaveBeenCalledWith('s')
    B.jumpTo(2)
    expect(P.jumpTo).toHaveBeenCalledWith(2)
    B.moveTo(1, 3)
    expect(P.moveTo).toHaveBeenCalledWith(1, 3)
    B.removeAt(4)
    expect(P.removeAt).toHaveBeenCalledWith(4)
    B.seek(9)
    expect(P.seek).toHaveBeenCalledWith(9)
    B.setVolume(0.5)
    expect(P.setVolume).toHaveBeenCalledWith(0.5)
    for (const n of ['stopStation', 'toggle', 'next', 'prev', 'clearUpcoming', 'toggleMute', 'toggleShuffle', 'cycleRepeat']) {
      B[n]()
      expect(P[n]).toHaveBeenCalled()
    }
    expect(send).not.toHaveBeenCalled()
  })

  it('shows the player, is not remote and can everything', () => {
    expect(B.shown()).toBe(P.player)
    expect(B.remote()).toBe(false)
    expect(B.can('next')).toBe(true)
  })
})

describe('playback, Rockola', () => {
  const view = { queue: { order: ['a', 'b', 'c', 'd', 'e'] } }
  beforeEach(() => Object.assign(rockola, { on: true, view }))

  it('shows the view', () => {
    expect(B.shown()).toBe(view)
    expect(B.remote()).toBe(true)
  })

  it('play as host sends play', () => {
    rockola.role = 'host'
    B.play({ album: 1 }, { startId: 5 })
    expect(send).toHaveBeenCalledWith('play', { what: { album: 1 }, shuffle: false, startId: 5 })
  })

  it('play as guest sends enqueue', () => {
    B.play({ album: 1 }, { startId: 5 })
    expect(send).toHaveBeenCalledWith('enqueue', { what: { tracks: [5] }, shuffle: false, startIfIdle: true })
    B.play({ folder: 'x' }, { shuffle: true })
    expect(send).toHaveBeenLastCalledWith('enqueue', { what: { folder: 'x' }, shuffle: true, startIfIdle: true })
  })

  it('enqueue, stations, seek and volume send their commands', () => {
    B.enqueue({ album: 1 }, { next: true })
    expect(send).toHaveBeenLastCalledWith('enqueue', { what: { album: 1 }, next: true })
    B.playStation('s')
    expect(send).toHaveBeenLastCalledWith('playStation', { station: 's' })
    B.setStation('s')
    expect(send).toHaveBeenLastCalledWith('setStation', { station: 's' })
    B.seek(7)
    expect(send).toHaveBeenLastCalledWith('seek', { seconds: 7 })
    B.setVolume(0.3)
    expect(send).toHaveBeenLastCalledWith('setVolume', { volume: 0.3 })
    B.next()
    expect(send).toHaveBeenLastCalledWith('next')
    expect(P.next).not.toHaveBeenCalled()
  })

  it('positions become keys of the view', () => {
    B.jumpTo(2)
    expect(send).toHaveBeenLastCalledWith('jumpTo', { key: 'c' })
    B.removeAt(1)
    expect(send).toHaveBeenLastCalledWith('removeAt', { key: 'b' })
    B.moveTo(2, 4)
    expect(send).toHaveBeenLastCalledWith('moveTo', { key: 'c', toKey: 'e' })
  })

  it('can follows the role', () => {
    expect(B.can('next')).toBe(false)
    expect(B.can('enqueue')).toBe(true)
    rockola.role = 'host'
    expect(B.can('next')).toBe(true)
  })

  it('a guest can only remove what they added', () => {
    expect(B.can('removeAt', { addedBy: 3 })).toBe(true)
    expect(B.can('removeAt', { addedBy: 4 })).toBe(false)
    expect(B.can('removeAt', {})).toBe(false)
    rockola.role = 'host'
    expect(B.can('removeAt', { addedBy: 4 })).toBe(true)
  })
})
