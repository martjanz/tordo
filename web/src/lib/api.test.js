import { afterEach, describe, expect, it, vi } from 'vitest'
import { albumCoverURL, api, itemCoverURL, playQuery, releaseURL, stationQuery, streamURL } from './api.js'

describe('playQuery', () => {
  it('writes what to play', () => {
    expect(playQuery({ album: 7 })).toBe('album=7')
    expect(playQuery({ folder: '../musica/Brasil', shuffle: true })).toBe('folder=..%2Fmusica%2FBrasil&shuffle=1')
    expect(playQuery({ tracks: [3, 1] })).toBe('track=3&track=1')
    expect(playQuery({ artist: 'charly-garcia' })).toBe('artist=charly-garcia')
    expect(playQuery({ playlist: 3 })).toBe('playlist=3')
    expect(playQuery({ liked: true })).toBe('liked=1')
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

describe('calls', () => {
  afterEach(() => vi.unstubAllGlobals())

  function recorder() {
    const calls = []
    vi.stubGlobal('fetch', async (url, init) => {
      calls.push([init.method, url, init.body ?? null])
      return { ok: true, status: init.method === 'DELETE' ? 204 : 200, json: async () => ({}) }
    })
    return calls
  }

  it('asks the server', async () => {
    const calls = recorder()
    await api.explore('?decada=1980&ver=pistas', 120, 60)
    await api.explore('')
    await api.artist('sui generis/x')
    await api.find('soda st')
    await api.setCollection('../musica/x', false)
    await api.setIdentification(4, { releaseId: 'abc' })
    await api.deleteGenreAlias('Música uruguaya')
    await api.folderTree('')
    await api.folderTree('../musica/a b')
    await api.folderTreeSearch('zitar')
    expect(calls).toEqual([
      ['GET', '/api/explore?decada=1980&ver=pistas&desde=120&limite=60', null],
      ['GET', '/api/explore?limite=120', null],
      ['GET', '/api/artists/sui%20generis%2Fx', null],
      ['GET', '/api/search?q=soda+st&limite=6', null],
      ['PUT', '/api/folders/collection', '{"path":"../musica/x","collection":false}'],
      ['PUT', '/api/albums/4/identification', '{"releaseId":"abc"}'],
      ['DELETE', '/api/genres/aliases?raw=M%C3%BAsica+uruguaya', null],
      ['GET', '/api/folders/tree', null],
      ['GET', '/api/folders/tree?path=..%2Fmusica%2Fa+b', null],
      ['GET', '/api/folders/tree/search?q=zitar', null],
    ])
    expect(releaseURL('abc')).toBe('https://musicbrainz.org/release/abc')
  })
})

describe('stations', () => {
  afterEach(() => vi.unstubAllGlobals())
  const st = { name: 'Rock', source: { view: 'pistas', facets: { genero: 'Rock' } }, mode: 'pistas', artistGap: 3, avoidDays: 7 }

  it('writes the station query without the id', () => {
    const q = new URLSearchParams(stationQuery({ ...st, id: 4, createdAt: 'x' }, [1, 2, 3]))
    expect(JSON.parse(q.get('def'))).toEqual(st)
    expect(q.get('recientes')).toBe('1,2,3')
    expect(new URLSearchParams(stationQuery(st, [])).has('recientes')).toBe(false)
    expect(new URLSearchParams(stationQuery(st)).has('recientes')).toBe(false)
  })

  it('asks the server', async () => {
    const calls = []
    vi.stubGlobal('fetch', async (url, init) => {
      calls.push([init.method, url, init.body ?? null])
      return { ok: true, status: init.method === 'DELETE' ? 204 : 200, json: async () => ({}) }
    })
    await api.stations()
    await api.createStation({ ...st, id: 9 })
    await api.updateStation(9, { ...st, id: 9 })
    await api.deleteStation(9)
    await api.home(42)
    await api.stats('anio', 10)
    await api.stationNext(st, [4, 5])
    expect(calls.map((c) => c.slice(0, 2))).toEqual([
      ['GET', '/api/stations'],
      ['POST', '/api/stations'],
      ['PUT', '/api/stations/9'],
      ['DELETE', '/api/stations/9'],
      ['GET', '/api/home?semilla=42'],
      ['GET', '/api/stats?periodo=anio&limite=10'],
      ['GET', '/api/station/next?' + stationQuery(st, [4, 5])],
    ])
    expect(JSON.parse(calls[1][2])).toEqual(st)
    expect(JSON.parse(calls[2][2])).toEqual(st)
  })
})
