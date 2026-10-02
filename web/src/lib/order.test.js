import { describe, expect, it } from 'vitest'
import { move } from './order.js'

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
