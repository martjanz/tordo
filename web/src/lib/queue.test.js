import { describe, expect, it } from 'vitest'
import * as Q from './queue.js'

const items = (...ids) => ids.map((id) => ({ id, title: `t${id}` }))
const ids = (q) => q.order.map((k) => q.list.find((e) => e.key === k).id)
const cur = (q) => Q.current(q)?.id ?? null
// A rand that always returns 0: a fixed, repeatable shuffle.
const first = () => 0

describe('replace', () => {
  it('starts a new queue at a track', () => {
    const q = Q.replace(Q.empty(), items(1, 2, 3), 1)
    expect(ids(q)).toEqual([1, 2, 3])
    expect(cur(q)).toBe(2)
  })
  it('with shuffle, plays the chosen track first and shuffles the rest', () => {
    const q = Q.replace({ ...Q.empty(), shuffle: true }, items(1, 2, 3, 4), 2, first)
    expect(cur(q)).toBe(3)
    expect(q.pos).toBe(0)
    expect(ids(q).slice(1).sort()).toEqual([1, 2, 4])
    expect(q.list.map((e) => e.id)).toEqual([1, 2, 3, 4])
  })
  it('empties on no items and caps at MAX', () => {
    expect(Q.replace(Q.replace(Q.empty(), items(1)), []).pos).toBe(-1)
    const many = Array.from({ length: Q.MAX + 10 }, (_, i) => ({ id: i }))
    expect(Q.replace(Q.empty(), many).list.length).toBe(Q.MAX)
  })
})

describe('adding', () => {
  it('appends at the end and starts an empty queue', () => {
    let q = Q.append(Q.empty(), items(1))
    expect(cur(q)).toBe(1)
    q = Q.append(q, items(2, 3))
    expect(ids(q)).toEqual([1, 2, 3])
    expect(cur(q)).toBe(1)
  })
  it('plays next right after the current track, also without shuffle later', () => {
    let q = Q.replace(Q.empty(), items(1, 2, 3), 0)
    q = Q.playNext(q, items(9))
    expect(ids(q)).toEqual([1, 9, 2, 3])
    expect(q.list.map((e) => e.id)).toEqual([1, 9, 2, 3])
    q = Q.setShuffle(Q.setShuffle(q, true, first), false)
    expect(ids(q)).toEqual([1, 9, 2, 3])
  })
  it('keeps duplicates apart', () => {
    const q = Q.append(Q.replace(Q.empty(), items(1)), items(1))
    expect(new Set(q.order).size).toBe(2)
  })
})

describe('moving', () => {
  const q = Q.replace(Q.empty(), items(1, 2, 3), 0)
  it('goes forward and stops at the end', () => {
    expect(cur(Q.next(q))).toBe(2)
    expect(Q.next(Q.jump(q, 2))).toBeNull()
    expect(Q.peek(Q.jump(q, 2))).toBeNull()
  })
  it('repeats all and one', () => {
    const all = { ...Q.jump(q, 2), repeat: 'all' }
    expect(cur(Q.next(all))).toBe(1)
    expect(Q.peek(all).id).toBe(1)
    const one = { ...q, repeat: 'one' }
    expect(cur(Q.next(one, true))).toBe(1) // the track ended: again
    expect(cur(Q.next(one))).toBe(2) // the user asked for the next one
    expect(Q.peek(one).id).toBe(1)
  })
  it('goes back', () => {
    expect(cur(Q.prev(Q.jump(q, 2)))).toBe(2)
    expect(cur(Q.prev(q))).toBe(1)
    expect(cur(Q.prev({ ...q, repeat: 'all' }))).toBe(3)
  })
  it('cycles repeat modes', () => {
    expect(Q.cycleRepeat(q).repeat).toBe('all')
    expect(Q.cycleRepeat(Q.cycleRepeat(Q.cycleRepeat(q))).repeat).toBe('off')
  })
})

describe('reordering', () => {
  const q = Q.jump(Q.replace(Q.empty(), items(1, 2, 3, 4, 5), 0), 1) // playing 2
  const listIds = (q) => q.list.map((e) => e.id)
  it('moves an upcoming track, in the play order and the list', () => {
    const m = Q.move(q, 4, 2)
    expect(ids(m)).toEqual([1, 2, 5, 3, 4])
    expect(listIds(m)).toEqual([1, 2, 5, 3, 4])
    expect(cur(m)).toBe(2)
    expect(ids(Q.move(q, 2, 4))).toEqual([1, 2, 4, 5, 3])
  })
  it('keeps the move after shuffle is turned off and on again', () => {
    const m = Q.setShuffle(Q.setShuffle(Q.move(q, 4, 2), true, first), false)
    expect(ids(m)).toEqual([1, 2, 5, 3, 4])
  })
  it('only moves within what is still to play', () => {
    expect(ids(Q.move(q, 4, 0))).toEqual([1, 2, 5, 3, 4]) // clamped after the current one
    expect(ids(Q.move(q, 2, 99))).toEqual([1, 2, 4, 5, 3])
    expect(Q.move(q, 1, 3)).toBe(q) // the current track stays
    expect(Q.move(q, 0, 3)).toBe(q) // and what was played
    expect(Q.move(q, 9, 2)).toBe(q)
    expect(Q.move(q, 3, 3)).toBe(q)
  })
  it('with shuffle on, moves only in the play order', () => {
    const s = Q.setShuffle(q, true, first)
    const m = Q.move(s, 4, 2)
    expect(ids(m)).toEqual([...ids(s).slice(0, 2), ids(s)[4], ...ids(s).slice(2, 4)])
    expect(listIds(m)).toEqual([1, 2, 3, 4, 5])
  })
  it('tells apart the same track queued twice', () => {
    const d = Q.append(Q.replace(Q.empty(), items(1, 2), 0), items(1))
    const m = Q.move(d, 2, 1)
    expect(ids(m)).toEqual([1, 1, 2])
    expect(m.order).toEqual([d.order[0], d.order[2], d.order[1]])
  })
})

describe('removing', () => {
  const q = Q.jump(Q.replace(Q.empty(), items(1, 2, 3, 4), 0), 1)
  it('keeps the current track when another goes', () => {
    expect(cur(Q.remove(q, 0))).toBe(2)
    expect(cur(Q.remove(q, 3))).toBe(2)
  })
  it('moves on when the current one goes', () => {
    expect(cur(Q.remove(q, 1))).toBe(3)
    const last = Q.jump(q, 3)
    expect(cur(Q.remove(last, 3))).toBe(3)
    expect(Q.remove(Q.replace(Q.empty(), items(1)), 0).pos).toBe(-1)
  })
  it('clears what is still to play', () => {
    const c = Q.clearUpcoming(q)
    expect(ids(c)).toEqual([1, 2])
    expect(cur(c)).toBe(2)
    expect(Q.upcoming(c)).toEqual([])
  })
  it('lists what comes next', () => {
    expect(Q.upcoming(q).map((u) => [u.entry.id, u.pos])).toEqual([
      [3, 2],
      [4, 3],
    ])
  })
})

describe('shuffle', () => {
  it('shuffles only what is still to play, and comes back to the list order', () => {
    const q = Q.jump(Q.replace(Q.empty(), items(1, 2, 3, 4, 5), 0), 1)
    const s = Q.setShuffle(q, true, () => 0.99)
    expect(ids(s).slice(0, 2)).toEqual([1, 2])
    expect(ids(s).slice(2).sort()).toEqual([3, 4, 5])
    expect(cur(s)).toBe(2)
    const back = Q.setShuffle(Q.jump(s, 4), false)
    expect(ids(back)).toEqual([1, 2, 3, 4, 5])
    expect(cur(back)).toBe(Q.current(Q.jump(s, 4)).id)
  })
})

describe('restore', () => {
  it('round-trips and survives garbage', () => {
    const q = Q.cycleRepeat(Q.replace(Q.empty(), items(1, 2), 1))
    expect(Q.restore(JSON.stringify(q))).toEqual(q)
    expect(Q.restore('no es json')).toEqual(Q.empty())
    expect(Q.restore('{"list":[],"order":[3]}')).toEqual(Q.empty())
    expect(Q.restore(null)).toEqual(Q.empty())
    const r = Q.restore(JSON.stringify({ ...q, repeat: 'x', pos: 9, nextKey: 0 }))
    expect([r.repeat, r.pos, r.nextKey]).toEqual(['off', 0, 3])
  })
})

describe('peekPos', () => {
  it('is the play-order position peek returns', () => {
    const q = Q.replace(Q.empty(), items(1, 2, 3), 1)
    expect(Q.peekPos(q)).toBe(2)
    const end = Q.jump(q, 2)
    expect(Q.peekPos(end)).toBe(-1)
    expect(Q.peekPos({ ...end, repeat: 'all' })).toBe(0)
    expect(Q.peekPos({ ...end, repeat: 'one' })).toBe(2)
    expect(Q.peekPos(Q.empty())).toBe(-1)
  })
})

describe('station', () => {
  const st = { name: 'Rock', source: {} }
  it('starts without one and restore keeps an object', () => {
    expect(Q.empty().station).toBe(null)
    const q = { ...Q.replace(Q.empty(), items(1, 2)), station: st }
    expect(Q.restore(JSON.stringify(q)).station).toEqual(st)
    expect(Q.restore(JSON.stringify({ ...q, station: 'x' })).station).toBe(null)
    expect(Q.restore(JSON.stringify({ ...q, station: undefined })).station).toBe(null)
  })
})

describe('trimPlayed', () => {
  const many = (n) => items(...Array.from({ length: n }, (_, i) => i + 1))
  it('drops what was played before the last keep entries', () => {
    const q = Q.jump(Q.replace(Q.empty(), many(200)), 150)
    const t = Q.trimPlayed(q, 100)
    expect(t.pos).toBe(100)
    expect(t.order.length).toBe(150)
    expect(t.list.length).toBe(150)
    expect(ids(t)[0]).toBe(51)
    expect(cur(t)).toBe(151)
  })
  it('works with shuffle on', () => {
    const q = Q.jump(Q.replace({ ...Q.empty(), shuffle: true }, many(200), 0, () => 0.3), 150)
    const gone = q.order.slice(0, 50)
    const t = Q.trimPlayed(q, 100)
    expect(t.pos).toBe(100)
    expect(t.order).toEqual(q.order.slice(50))
    expect(t.list.length).toBe(150)
    expect(t.list.some((e) => gone.includes(e.key))).toBe(false)
    expect(cur(t)).toBe(cur(q))
  })
  it('leaves a short history alone', () => {
    const q = Q.jump(Q.replace(Q.empty(), many(20)), 10)
    expect(Q.trimPlayed(q, 100)).toBe(q)
  })
})

describe('shuffled', () => {
  it('returns a permutation and leaves the input untouched', () => {
    const input = [1, 2, 3, 4]
    const rands = [0.7, 0.2, 0.9]
    let n = 0
    const out = Q.shuffled(input, () => rands[n++ % rands.length])
    expect(input).toEqual([1, 2, 3, 4])
    expect([...out].sort()).toEqual([1, 2, 3, 4])
    expect(out).not.toEqual(input)
  })
  it('of nothing is nothing', () => {
    expect(Q.shuffled([])).toEqual([])
  })
})
