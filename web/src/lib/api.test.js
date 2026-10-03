import { afterEach, describe, expect, it, vi } from 'vitest'
import { albumCoverURL, api, ApiError, itemCoverURL, lanQrURL, onUnauthorized, playQuery, releaseURL, stationQuery, streamURL } from './api.js'

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

describe('config, lan and auth', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    onUnauthorized(null)
  })

  it('asks the server', async () => {
    const calls = []
    vi.stubGlobal('fetch', async (url, init) => {
      calls.push([init.method, url, init.body ?? null])
      return { ok: true, status: init.method === 'DELETE' ? 204 : 200, json: async () => ({}) }
    })
    await api.config()
    await api.saveConfig({ roots: ['../m'] })
    await api.checkRoot('../m')
    await api.checkToken('abc')
    await api.lan()
    await api.sessions()
    await api.closeSession('s 1')
    await api.auth()
    await api.login('123456', 'Celu')
    await api.logout()
    await api.duplicates('formatos')
    await api.toConvert()
    expect(calls).toEqual([
      ['GET', '/api/config', null],
      ['PUT', '/api/config', '{"roots":["../m"]}'],
      ['POST', '/api/config/root', '{"path":"../m"}'],
      ['POST', '/api/config/token', '{"token":"abc"}'],
      ['GET', '/api/lan', null],
      ['GET', '/api/sessions', null],
      ['DELETE', '/api/sessions/s 1', null],
      ['GET', '/api/auth', null],
      ['POST', '/api/auth/login', '{"pin":"123456","device":"Celu"}'],
      ['POST', '/api/auth/logout', '{}'],
      ['GET', '/api/review/duplicates?tipo=formatos', null],
      ['GET', '/api/review/formats', null],
    ])
  })

  it('encodes the QR address', () => {
    expect(lanQrURL('http://192.168.1.5:4747/')).toBe('/api/lan/qr?url=http%3A%2F%2F192.168.1.5%3A4747%2F')
  })

  function failWith(status, headers = {}) {
    vi.stubGlobal('fetch', async () => ({
      ok: false,
      status,
      headers: { get: (k) => headers[k] ?? null },
      text: async () => 'no',
    }))
  }

  it('tells the handler about a 401, except from login', async () => {
    let n = 0
    onUnauthorized(() => n++)
    failWith(401)
    await expect(api.sessions()).rejects.toMatchObject({ status: 401 })
    expect(n).toBe(1)
    await expect(api.login('1', '')).rejects.toMatchObject({ status: 401 })
    expect(n).toBe(1)
    failWith(403)
    await expect(api.sessions()).rejects.toBeInstanceOf(ApiError)
    expect(n).toBe(1)
  })

  it('reads Retry-After', async () => {
    failWith(429, { 'Retry-After': '30' })
    await expect(api.login('1', '')).rejects.toMatchObject({ status: 429, retryAfter: 30 })
    failWith(500)
    await expect(api.status()).rejects.toMatchObject({ retryAfter: 0 })
  })
})
