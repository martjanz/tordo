import { describe, expect, it } from 'vitest'
import * as Q from './queue.js'
import { factor, gainDb, missing, UNKNOWN_DB, withGains } from './gain.js'

const mk = (id, albumId, extra = {}) => ({ id, albumId, ...extra })
const queue = (items, shuffle = false) => Q.replace({ ...Q.empty(), shuffle }, items, 0, () => 0)

describe('gainDb', () => {
  it('uses the album gain when the album plays in order', () => {
    const q = queue([mk(1, 7, { trackGain: -8, albumGain: -5 }), mk(2, 7, { trackGain: -9, albumGain: -5 })])
    expect(gainDb(q, 0)).toBe(-5)
    expect(gainDb(q, 1)).toBe(-5)
  })
  it('uses the track gain for a lone track between others', () => {
    const q = queue([mk(1, 3), mk(2, 7, { trackGain: -8, albumGain: -5 }), mk(3, 4)])
    expect(gainDb(q, 1)).toBe(-8)
  })
  it('uses the track gain with shuffle on', () => {
    const q = queue([mk(1, 7, { trackGain: -8, albumGain: -5 }), mk(2, 7, { trackGain: -9, albumGain: -5 })], true)
    expect(gainDb(q, 0)).toBe(-8)
  })
  it('falls back to UNKNOWN_DB and to the track gain', () => {
    const q = queue([mk(1, 7), mk(2, 7, { trackGain: -9 })])
    expect(gainDb(q, 0)).toBe(UNKNOWN_DB)
    expect(gainDb(q, 1)).toBe(-9)
  })
  it('is 0 for a missing position', () => {
    expect(gainDb(Q.empty(), 0)).toBe(0)
  })
})

describe('factor', () => {
  it('only attenuates', () => {
    expect(factor(0)).toBe(1)
    expect(factor(6)).toBe(1)
    expect(factor(-6)).toBeCloseTo(0.501, 3)
  })
})

describe('missing', () => {
  it('skips known gains and duplicates, and respects count', () => {
    const q = queue([mk(1, 1, { trackGain: -3 }), mk(2, 1), mk(2, 1), mk(3, 1), mk(4, 1)])
    expect(missing(q, 0, 2)).toEqual([2, 3])
    expect(missing(q, 3, 5)).toEqual([3, 4])
  })
})

describe('withGains', () => {
  it('updates every entry of those tracks', () => {
    const q = queue([mk(1, 1), mk(2, 1), mk(1, 1)])
    const r = withGains(q, { 1: { trackGain: -4, albumGain: -3 } })
    expect(r.list.map((e) => e.trackGain)).toEqual([-4, undefined, -4])
    expect(r.list[1]).toBe(q.list[1])
  })
})
