import { describe, expect, it } from 'vitest'
import * as M from './marks.js'

const server = {
  tracks: { 1: { liked: true, likedAt: 5, rating: 4 }, 2: { rating: 3 } },
  albums: { 9: { liked: true } },
  artists: ['soda'],
  collections: ['../musica/Rock'],
}

describe('fromServer', () => {
  it('turns key arrays into objects', () => {
    const m = M.fromServer(server)
    expect(m.artists).toEqual({ soda: true })
    expect(m.collections).toEqual({ '../musica/Rock': true })
    expect(m.tracks[1].rating).toBe(4)
    expect(M.fromServer(null)).toEqual(M.empty())
    expect(M.fromServer({})).toEqual(M.empty())
  })
})

describe('queries', () => {
  const m = M.fromServer(server)
  it('isLiked and ratingOf for every kind', () => {
    expect(M.isLiked(m, 'track', 1)).toBe(true)
    expect(M.isLiked(m, 'track', 2)).toBe(false)
    expect(M.isLiked(m, 'track', 99)).toBe(false)
    expect(M.isLiked(m, 'album', 9)).toBe(true)
    expect(M.isLiked(m, 'artist', 'soda')).toBe(true)
    expect(M.isLiked(m, 'artist', 'otro')).toBe(false)
    expect(M.isLiked(m, 'collection', '../musica/Rock')).toBe(true)
    expect(M.isLiked(m, 'collection', 'x')).toBe(false)
    expect(M.ratingOf(m, 'track', 1)).toBe(4)
    expect(M.ratingOf(m, 'track', 2)).toBe(3)
    expect(M.ratingOf(m, 'track', 99)).toBe(0)
    expect(M.ratingOf(m, 'album', 9)).toBe(0)
  })
})

describe('changes', () => {
  it('withLike returns a new object', () => {
    const m = M.fromServer(server)
    const before = JSON.stringify(m)
    const a = M.withLike(m, 'track', 2, true)
    expect(JSON.stringify(m)).toBe(before)
    expect(a).not.toBe(m)
    expect(M.isLiked(a, 'track', 2)).toBe(true)
    expect(M.ratingOf(a, 'track', 2)).toBe(3)
    expect(M.isLiked(M.withLike(m, 'artist', 'x', true), 'artist', 'x')).toBe(true)
    expect(M.isLiked(M.withLike(m, 'artist', 'soda', false), 'artist', 'soda')).toBe(false)
    expect(M.isLiked(M.withLike(m, 'collection', 'c', true), 'collection', 'c')).toBe(true)
    expect(M.isLiked(M.withLike(m, 'album', 3, true), 'album', 3)).toBe(true)
  })
  it('unlike keeps the rating', () => {
    const m = M.withLike(M.fromServer(server), 'track', 1, false)
    expect(M.isLiked(m, 'track', 1)).toBe(false)
    expect(M.ratingOf(m, 'track', 1)).toBe(4)
  })
  it('withRating sets and clears', () => {
    const m = M.fromServer(server)
    const a = M.withRating(m, 'track', 1, 2)
    expect(M.ratingOf(a, 'track', 1)).toBe(2)
    expect(M.isLiked(a, 'track', 1)).toBe(true)
    expect(M.ratingOf(m, 'track', 1)).toBe(4)
    expect(M.ratingOf(M.withRating(m, 'album', 5, 5), 'album', 5)).toBe(5)
  })
  it('rating 0 on an unliked track removes the entry', () => {
    const m = M.withRating(M.fromServer(server), 'track', 2, 0)
    expect(2 in m.tracks).toBe(false)
    const keep = M.withRating(M.fromServer(server), 'track', 1, 0)
    expect(keep.tracks[1].liked).toBe(true)
    expect(M.ratingOf(keep, 'track', 1)).toBe(0)
  })
})
