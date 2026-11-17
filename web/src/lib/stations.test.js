import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from './i18n/index.svelte.js'
import * as Q from './queue.js'
import * as S from './stations.js'

const items = (n) => Array.from({ length: n }, (_, i) => ({ id: i + 1, title: `t${i + 1}` }))
const withStation = (n, pos) => ({ ...Q.jump(Q.replace(Q.empty(), items(n)), pos), station: S.LIBRARY })

describe('station', () => {
  it('joins a source and rules', () => {
    expect(S.station('X', { liked: true })).toEqual({ name: 'X', source: { liked: true }, mode: 'tracks', artistGap: 3, avoidDays: 7 })
    expect(S.station('X', {}, { mode: 'albums', artistGap: 0, avoidDays: 0 })).toEqual({ name: 'X', source: {}, mode: 'albums', artistGap: 0, avoidDays: 0 })
  })
  it('has the built-ins', () => {
    expect(S.LIKED).toEqual({ name: 'Tus me gusta', source: { liked: true } })
    expect(S.FAVORITE_TRACKS).toEqual({ name: 'Tus canciones favoritas', source: { view: 'tracks', facets: { liked: 'yes' } } })
    expect(S.PREFERRED_ALBUMS).toEqual({ name: 'Álbumes preferidos', source: { view: 'albums', facets: { liked: 'yes' } } })
    expect(S.LIBRARY).toEqual({ name: 'Toda la colección', source: {} })
    expect([S.LOW_WATER, S.RECENT, S.KEEP_PLAYED]).toEqual([3, 100, 100])
  })
})

describe('exploreStation', () => {
  it('names the station after the applied facets', () => {
    const st = S.exploreStation({ view: 'artists', facets: { country: 'AR', genre: 'Rock' } })
    expect(st).toEqual({ name: 'Argentina · Rock', source: { view: 'albums', facets: { country: 'AR', genre: 'Rock' } } })
  })
  it('keeps the view and uses facet labels', () => {
    const st = S.exploreStation({ view: 'tracks', facets: { decade: '1980', liked: 'yes' } })
    expect(st.name).toBe('1980s · Me gusta')
    expect(st.source).toEqual({ view: 'tracks', facets: { decade: '1980', liked: 'yes' } })
  })
  it('names the liked facet after the tab', () => {
    expect(S.exploreStation({ view: 'albums', facets: { liked: 'yes' } }).name).toBe('Preferidos')
  })
  it('is the whole collection without facets', () => {
    expect(S.exploreStation({ view: 'albums', facets: {} })).toEqual({ name: 'Toda la colección', source: { view: 'albums', facets: {} } })
  })
  it('uses the counts for labels when given', () => {
    const counts = { artist: [{ value: 'soda-stereo', label: 'Soda Stereo', count: 3 }] }
    expect(S.exploreStation({ view: 'albums', facets: { artist: 'soda-stereo' } }, counts).name).toBe('Soda Stereo')
  })
})

describe('albumStation', () => {
  it('prefers the style', () => {
    expect(S.albumStation({ genres: [{ genre: 'Rock', style: 'Pop Rock' }] })).toEqual({
      name: 'Pop Rock',
      source: { view: 'tracks', facets: { style: 'Pop Rock' } },
    })
  })
  it('then the genre', () => {
    expect(S.albumStation({ genres: [{ genre: 'Jazz' }], artistKey: 'x', year: 1990 })).toEqual({
      name: 'Jazz',
      source: { view: 'tracks', facets: { genre: 'Jazz' } },
    })
  })
  it('then the artist', () => {
    expect(S.albumStation({ artist: 'Soda Stereo', artistKey: 'soda-stereo', year: 1990 })).toEqual({
      name: 'Soda Stereo',
      source: { view: 'tracks', facets: { artist: 'soda-stereo' } },
    })
  })
  it('then the decade', () => {
    expect(S.albumStation({ year: 1986 })).toEqual({ name: 'Los 80', source: { view: 'tracks', facets: { decade: '1980' } } })
  })
  it('is null with nothing', () => {
    expect(S.albumStation({})).toBe(null)
    expect(S.albumStation({ genres: [], artistKey: '', year: 0 })).toBe(null)
  })
})

describe('other sources', () => {
  it('artist, folder, playlist', () => {
    expect(S.artistStation({ key: 'k', name: 'Ka' })).toEqual({ name: 'Ka', source: { view: 'tracks', facets: { artist: 'k' } } })
    expect(S.folderStation({ name: 'Rock', path: '../m/Rock' })).toEqual({ name: 'Rock', source: { folder: '../m/Rock' } })
    expect(S.playlistStation({ id: 3, name: 'Mi lista' })).toEqual({ name: 'Mi lista', source: { playlist: 3 } })
  })
})

describe('needsMore', () => {
  it('asks when fewer than LOW_WATER remain after the current', () => {
    expect(S.needsMore(withStation(10, 7))).toBe(true) // 2 left
    expect(S.needsMore(withStation(10, 6))).toBe(false) // 3 left
    expect(S.needsMore(withStation(10, 5))).toBe(false)
    expect(S.needsMore({ ...withStation(10, 9), station: null })).toBe(false)
  })
})

describe('recentIds', () => {
  it('lists the last entries, oldest first', () => {
    expect(S.recentIds(withStation(5, 1))).toEqual([1, 2, 3, 4, 5])
    expect(S.recentIds(withStation(150, 10))).toEqual(Array.from({ length: 100 }, (_, i) => i + 51))
    expect(S.recentIds(Q.empty())).toEqual([])
  })
})

describe('summary', () => {
  it('reads the rules', () => {
    expect(S.summary(S.station('X', {}))).toBe('Pistas · sin repetir artista en 3 · evita lo escuchado en 7 días')
    expect(S.summary({ mode: 'albums', artistGap: 0, avoidDays: 0 })).toBe('Álbumes')
    expect(S.summary({ mode: 'tracks', artistGap: 1, avoidDays: 1 })).toBe('Pistas · sin repetir artista en 1 · evita lo escuchado en 1 día')
    expect(S.summary({ mode: 'tracks', artistGap: 0, avoidDays: 30 })).toBe('Pistas · evita lo escuchado en 30 días')
  })
})

describe('rowTitle', () => {
  it.each([
    [{ kind: 'decade', value: '1980' }, 'Los 80'],
    [{ kind: 'decade', value: '1990' }, 'Los 90'],
    [{ kind: 'decade', value: '2000' }, 'Los 2000'],
    [{ kind: 'decade', value: '2010' }, 'Los 2010'],
    [{ kind: 'decade', value: '1960' }, 'Los 60'],
    [{ kind: 'decade', value: '1900' }, 'Los 1900'],
    [{ kind: 'country', value: 'AR', label: 'AR' }, 'Argentina'],
    [{ kind: 'genre', value: 'Rock' }, 'Rock'],
    [{ kind: 'style', value: 'Pop Rock' }, 'Pop Rock'],
    [{ kind: 'collection', value: 'x', label: 'Latinoamérica 600' }, 'Latinoamérica 600'],
    [{ kind: 'never' }, 'Nunca escuchados'],
    [{ kind: 'year-ago' }, 'Sin escuchar hace un año'],
    [{ kind: 'random' }, 'Álbumes al azar'],
    [{ kind: 'recent' }, 'Agregados recientemente'],
  ])('%j', (row, want) => {
    expect(S.rowTitle(row)).toBe(want)
  })
})

describe('other languages', () => {
  afterEach(() => setLocale('es'))

  it.each([
    ['es', 1985, 'Los 80'],
    ['en', 1985, 'The 80s'],
    ['pt-BR', 1985, 'Anos 80'],
    ['es', 2003, 'Los 2000'],
    ['en', 2003, 'The 2000s'],
    ['pt-BR', 2003, 'Anos 2000'],
  ])('names the decade station in %s for %d', (l, year, want) => {
    setLocale(l)
    expect(S.albumStation({ year }).name).toBe(want)
  })
  it('names the built-ins and summarizes in the current language', () => {
    setLocale('en')
    expect(S.LIKED.name).toBe('Your likes')
    expect(S.likedName()).toBe('Your likes')
    expect(S.LIBRARY.name).toBe('The whole collection')
    expect(S.summary({ mode: 'albums', artistGap: 2, avoidDays: 1 })).toBe('Albums · no artist repeats within 2 · avoids what was played in the last 1 day')
    expect(S.rowTitle({ kind: 'never' })).toBe('Never played')
    setLocale('pt-BR')
    expect(S.PREFERRED_ALBUMS.name).toBe('Álbuns preferidos')
  })
  it('keeps a getter for the name', () => {
    expect(Object.getOwnPropertyDescriptor(S.LIKED, 'name').get).toBeTypeOf('function')
    expect(S.LIKED.source).toEqual({ liked: true })
  })
})
