import { describe, expect, it } from 'vitest'
import { discHeaders } from './discs.js'

describe('discHeaders', () => {
  const t = (id, disc, albumId = 'a1') => ({ id, disc, albumId })

  it('marks where each disc starts in a multi-disc album', () => {
    const tracks = [t(1, 1), t(2, 1), t(3, 2), t(4, 2), t(5, 3)]
    expect(discHeaders(tracks, true)).toEqual([1, null, 2, null, 3])
  })
  it('treats a missing disc number as disc 1', () => {
    expect(discHeaders([t(1, 0), t(2, undefined), t(3, 2)], true)).toEqual([1, null, 2])
  })
  it('has no headers for a single-disc album', () => {
    expect(discHeaders([t(1, 1), t(2, 1)], true)).toEqual([null, null])
    expect(discHeaders([t(1, 0), t(2, 0)], true)).toEqual([null, null])
  })
  it('has no headers in a mixed-album list, even with disc 2 tracks', () => {
    const tracks = [t(1, 1, 'a1'), t(2, 2, 'a2'), t(3, 1, 'a3')]
    expect(discHeaders(tracks, false)).toEqual([null, null, null])
  })
  it('is off unless asked for', () => {
    expect(discHeaders([t(1, 1), t(2, 2)])).toEqual([null, null])
  })
  it('handles an empty list', () => {
    expect(discHeaders([], true)).toEqual([])
  })
})
