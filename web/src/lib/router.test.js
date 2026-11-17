import { describe, expect, it } from 'vitest'
import { albumHref, appLink, artistHref, duplicatesHref, exploreHref, folderHref, listHref, resolve, searchHref, statsHref } from './router.js'

describe('resolve', () => {
  it.each([
    ['/', '', { page: 'inicio' }],
    ['/', '?country=AR', { page: 'explorar' }],
    ['/', '?order=title', { page: 'explorar' }],
    ['/', '?view=tracks', { page: 'explorar' }],
    ['/', '?dir=asc', { page: 'explorar' }],
    ['/', '?foo=1', { page: 'inicio' }],
    ['/settings', '', { page: 'ajustes' }],
    ['/help', '', { page: 'ayuda' }],
    ['/welcome', '', { page: 'bienvenida' }],
    ['/review/duplicates', '', { page: 'revisar', tab: 'duplicados', kind: 'exact' }],
    ['/review/duplicates', '?kind=formats', { page: 'revisar', tab: 'duplicados', kind: 'formats' }],
    ['/review/duplicates', '?kind=suspected', { page: 'revisar', tab: 'duplicados', kind: 'suspected' }],
    ['/review/duplicates', '?kind=otra', { page: 'revisar', tab: 'duplicados', kind: 'exact' }],
    ['/review/formats', '', { page: 'revisar', tab: 'formatos' }],
    ['/stats', '', { page: 'estadisticas', period: 'month' }],
    ['/stats', '?period=year', { page: 'estadisticas', period: 'year' }],
    ['/stats', '?period=week', { page: 'estadisticas', period: 'week' }],
    ['/stats', '?period=all', { page: 'estadisticas', period: 'all' }],
    ['/stats', '?period=x', { page: 'estadisticas', period: 'month' }],
    ['/explore', '?decade=1980', { page: 'explorar' }],
    ['/folders', '', { page: 'carpetas', path: '' }],
    ['/folders', '?path=..%2Fmusica%2FLatinoam%C3%A9rica%20600', { page: 'carpetas', path: '../musica/Latinoamérica 600' }],
    ['/lists', '', { page: 'listas' }],
    ['/lists/12', '', { page: 'lista', id: 12 }],
    ['/lists/0', '', { page: 'notfound' }],
    ['/lists/x', '', { page: 'notfound' }],
    ['/album/12', '', { page: 'album', id: 12 }],
    ['/album/0', '', { page: 'notfound' }],
    ['/album/abc', '', { page: 'notfound' }],
    ['/artist/charly-garcia', '', { page: 'artista', key: 'charly-garcia' }],
    ['/artist/%E0%A4%A', '', { page: 'notfound' }],
    ['/search', '?q=soda', { page: 'buscar', q: 'soda' }],
    ['/review', '', { page: 'revisar', tab: 'sin-identificar' }],
    ['/review/genres', '', { page: 'revisar', tab: 'generos' }],
    ['/otra', '', { page: 'notfound' }],
    ['/explorar', '', { page: 'notfound' }],
    ['/carpetas', '?ruta=x', { page: 'notfound' }],
    ['/estadisticas', '', { page: 'notfound' }],
    ['/revisar/duplicados', '', { page: 'notfound' }],
  ])('%s%s', (path, search, want) => {
    expect(resolve(path, search)).toEqual(want)
  })
})

describe('hrefs', () => {
  it('round-trips folder paths', () => {
    const p = '../musica/[LA-10] Soda Stereo - Signos & más'
    const url = new URL(folderHref(p), 'http://127.0.0.1')
    expect(resolve(url.pathname, url.search)).toEqual({ page: 'carpetas', path: p })
    expect(folderHref('')).toBe('/folders')
    expect(albumHref(7)).toBe('/album/7')
  })
  it('round-trips artist keys', () => {
    const key = 'sui-generis/x?'
    const url = new URL(artistHref(key), 'http://127.0.0.1')
    expect(resolve(url.pathname, url.search)).toEqual({ page: 'artista', key })
  })
  it('links Explorar and the search', () => {
    expect(exploreHref()).toBe('/explore')
    expect(exploreHref({ country: 'AR', genre: 'Folk, World, & Country' })).toBe('/explore?country=AR&genre=Folk%2C+World%2C+%26+Country')
    expect(exploreHref({ artist: 'x' }, 'tracks')).toBe('/explore?view=tracks&artist=x')
    expect(searchHref(' soda stereo ')).toBe('/search?q=soda%20stereo')
  })
})

function anchor(href, attrs = {}) {
  return { href, target: attrs.target ?? '', hasAttribute: (name) => name in attrs }
}

const click = { button: 0, defaultPrevented: false }
const origin = 'http://127.0.0.1:4747'

describe('appLink', () => {
  it('keeps links to the app inside the app', () => {
    expect(appLink(click, anchor('http://127.0.0.1:4747/folders?path=x'), origin)).toBe('/folders?path=x')
    expect(appLink(click, anchor('/album/1'), origin)).toBe('/album/1')
  })
  it('leaves the rest to the browser', () => {
    expect(appLink(click, null, origin)).toBeNull()
    expect(appLink(click, anchor('https://musicbrainz.org/'), origin)).toBeNull()
    expect(appLink(click, anchor('/api/covers/album/1'), origin)).toBeNull()
    expect(appLink(click, anchor('/stream/1'), origin)).toBeNull()
    expect(appLink(click, anchor('/folders', { target: '_blank' }), origin)).toBeNull()
    expect(appLink(click, anchor('/folders', { download: '' }), origin)).toBeNull()
    expect(appLink({ ...click, ctrlKey: true }, anchor('/folders'), origin)).toBeNull()
    expect(appLink({ ...click, button: 1 }, anchor('/folders'), origin)).toBeNull()
    expect(appLink({ ...click, defaultPrevented: true }, anchor('/folders'), origin)).toBeNull()
  })
})

describe('listHref', () => {
  it('links a playlist', () => {
    expect(listHref(12)).toBe('/lists/12')
  })
})

describe('statsHref', () => {
  it('leaves the default period out', () => {
    expect(statsHref('month')).toBe('/stats')
    expect(statsHref()).toBe('/stats')
    expect(statsHref('year')).toBe('/stats?period=year')
  })
})

describe('duplicatesHref', () => {
  it('leaves the default kind out', () => {
    expect(duplicatesHref('exact')).toBe('/review/duplicates')
    expect(duplicatesHref()).toBe('/review/duplicates')
    expect(duplicatesHref('formats')).toBe('/review/duplicates?kind=formats')
  })
})
