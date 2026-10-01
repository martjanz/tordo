import { describe, expect, it } from 'vitest'
import { albumCoverURL, itemCoverURL, playQuery, streamURL } from './api.js'

describe('playQuery', () => {
  it('writes what to play', () => {
    expect(playQuery({ album: 7 })).toBe('album=7')
    expect(playQuery({ folder: '../musica/Brasil', shuffle: true })).toBe('folder=..%2Fmusica%2FBrasil&shuffle=1')
    expect(playQuery({ tracks: [3, 1] })).toBe('track=3&track=1')
  })
})

describe('urls', () => {
  it('points at covers and audio', () => {
    expect(albumCoverURL(5, 320)).toBe('/api/covers/album/5?size=320')
    expect(albumCoverURL(5)).toBe('/api/covers/album/5')
    expect(itemCoverURL({ id: 9, albumId: 5 }, 160)).toBe('/api/covers/album/5?size=160')
    expect(itemCoverURL({ id: 9, albumId: 0 }, 160)).toBe('/api/covers/track/9?size=160')
    expect(streamURL(9)).toBe('/stream/9')
    expect(streamURL(9, true)).toBe('/stream/9?transcode=1')
  })
})
