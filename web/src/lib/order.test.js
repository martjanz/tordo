import { describe, expect, it } from 'vitest'
import { dragShift, dropIndex, move } from './order.js'

describe('move', () => {
  const list = ['a', 'b', 'c', 'd']
  it('moves forward and backward', () => {
    expect(move(list, 0, 2)).toEqual(['b', 'c', 'a', 'd'])
    expect(move(list, 3, 1)).toEqual(['a', 'd', 'b', 'c'])
  })
  it('keeps the same place', () => {
    expect(move(list, 1, 1)).toEqual(list)
  })
  it('clamps to', () => {
    expect(move(list, 0, 99)).toEqual(['b', 'c', 'd', 'a'])
    expect(move(list, 3, -5)).toEqual(['d', 'a', 'b', 'c'])
  })
  it('does not change the input', () => {
    move(list, 0, 2)
    expect(list).toEqual(['a', 'b', 'c', 'd'])
  })
})

describe('dropIndex', () => {
  // Four rows of 10px from y = 0.
  const rects = [0, 10, 20, 30].map((top) => ({ top, height: 10 }))
  it('counts the other rows above the dragged row center', () => {
    expect(dropIndex(rects, 0, 0)).toBe(0)
    expect(dropIndex(rects, 0, 12)).toBe(1)
    expect(dropIndex(rects, 0, 99)).toBe(3)
    expect(dropIndex(rects, 3, -16)).toBe(2)
    expect(dropIndex(rects, 3, -99)).toBe(0)
  })
})

describe('dragShift', () => {
  it('moves the rows the dragged one passes by its height', () => {
    const down = { from: 1, to: 3, h: 10 }
    expect([0, 1, 2, 3].map((i) => dragShift(down, i))).toEqual([0, 0, -10, -10])
    const up = { from: 3, to: 1, h: 10 }
    expect([0, 1, 2, 3].map((i) => dragShift(up, i))).toEqual([0, 10, 10, 0])
    expect(dragShift(null, 2)).toBe(0)
  })
})
