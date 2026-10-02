import { describe, expect, it } from 'vitest'
import { begin, counts, due, sent, tick } from './plays.js'

// listen advances the pass by seconds of 1 s steps.
function listen(t, seconds, from = 0) {
  for (let s = from; s <= from + seconds; s++) t = tick(t, s)
  return t
}

describe('plays', () => {
  it('counts a 6 minute track at 3 minutes, not before', () => {
    const entry = { id: 1, durationMs: 6 * 60 * 1000 }
    expect(due(listen(begin(entry), 179))).toBe(false)
    expect(due(listen(begin(entry), 180))).toBe(true)
  })
  it('counts a 10 minute track at 4 minutes', () => {
    const entry = { id: 1, durationMs: 10 * 60 * 1000 }
    expect(due(listen(begin(entry), 239))).toBe(false)
    expect(due(listen(begin(entry), 240))).toBe(true)
  })
  it('counts() follows the rule', () => {
    expect(counts(240000, 0)).toBe(true)
    expect(counts(100000, 0)).toBe(false)
    expect(counts(50000, 100000)).toBe(true)
  })
  it('ignores seeking ahead and going back', () => {
    let t = tick(tick(begin({ id: 1, durationMs: 100000 }), 0), 120)
    expect(t.listenedMs).toBe(0)
    t = tick(t, 119)
    expect(t.listenedMs).toBe(0)
    t = tick(t, 120)
    expect(t.listenedMs).toBe(1000)
  })
  it('is due once', () => {
    let t = listen(begin({ id: 1, durationMs: 10000 }), 6)
    expect(due(t)).toBe(true)
    t = sent(t)
    expect(due(t)).toBe(false)
  })
  it('never counts without an entry', () => {
    const t = listen(begin(null), 600)
    expect(due(t)).toBe(false)
  })
})
