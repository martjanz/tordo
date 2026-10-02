import { describe, expect, it } from 'vitest'
import { albumHref, appLink, artistHref, exploreHref, folderHref, listHref, resolve, searchHref } from './router.js'

describe('resolve', () => {
  it.each([
    ['/', '', { page: 'explorar' }],
    ['/explorar', '?decada=1980', { page: 'explorar' }],
    ['/carpetas', '', { page: 'carpetas', path: '' }],
    ['/carpetas', '?ruta=..%2Fmusica%2FLatinoam%C3%A9rica%20600', { page: 'carpetas', path: '../musica/Latinoamérica 600' }],
    ['/listas', '', { page: 'listas' }],
    ['/listas/12', '', { page: 'lista', id: 12 }],
    ['/listas/0', '', { page: 'notfound' }],
    ['/listas/x', '', { page: 'notfound' }],
    ['/album/12', '', { page: 'album', id: 12 }],
    ['/album/0', '', { page: 'notfound' }],
    ['/album/abc', '', { page: 'notfound' }],
    ['/artista/charly-garcia', '', { page: 'artista', key: 'charly-garcia' }],
    ['/artista/%E0%A4%A', '', { page: 'notfound' }],
    ['/buscar', '?q=soda', { page: 'buscar', q: 'soda' }],
    ['/revisar', '', { page: 'revisar', tab: 'sin-identificar' }],
    ['/revisar/generos', '', { page: 'revisar', tab: 'generos' }],
    ['/otra', '', { page: 'notfound' }],
  ])('%s%s', (path, search, want) => {
    expect(resolve(path, search)).toEqual(want)
  })
})

describe('hrefs', () => {
  it('round-trips folder paths', () => {
    const p = '../musica/[LA-10] Soda Stereo - Signos & más'
    const url = new URL(folderHref(p), 'http://127.0.0.1')
    expect(resolve(url.pathname, url.search)).toEqual({ page: 'carpetas', path: p })
    expect(folderHref('')).toBe('/carpetas')
    expect(albumHref(7)).toBe('/album/7')
  })
  it('round-trips artist keys', () => {
    const key = 'sui-generis/x?'
    const url = new URL(artistHref(key), 'http://127.0.0.1')
    expect(resolve(url.pathname, url.search)).toEqual({ page: 'artista', key })
  })
  it('links Explorar and the search', () => {
    expect(exploreHref()).toBe('/explorar')
    expect(exploreHref({ pais: 'AR', genero: 'Folk, World, & Country' })).toBe('/explorar?pais=AR&genero=Folk%2C+World%2C+%26+Country')
    expect(exploreHref({ artista: 'x' }, 'pistas')).toBe('/explorar?ver=pistas&artista=x')
    expect(searchHref(' soda stereo ')).toBe('/buscar?q=soda%20stereo')
  })
})

function anchor(href, attrs = {}) {
  return { href, target: attrs.target ?? '', hasAttribute: (name) => name in attrs }
}

const click = { button: 0, defaultPrevented: false }
const origin = 'http://127.0.0.1:4747'

describe('appLink', () => {
  it('keeps links to the app inside the app', () => {
    expect(appLink(click, anchor('http://127.0.0.1:4747/carpetas?ruta=x'), origin)).toBe('/carpetas?ruta=x')
    expect(appLink(click, anchor('/album/1'), origin)).toBe('/album/1')
  })
  it('leaves the rest to the browser', () => {
    expect(appLink(click, null, origin)).toBeNull()
    expect(appLink(click, anchor('https://musicbrainz.org/'), origin)).toBeNull()
    expect(appLink(click, anchor('/api/covers/album/1'), origin)).toBeNull()
    expect(appLink(click, anchor('/stream/1'), origin)).toBeNull()
    expect(appLink(click, anchor('/carpetas', { target: '_blank' }), origin)).toBeNull()
    expect(appLink(click, anchor('/carpetas', { download: '' }), origin)).toBeNull()
    expect(appLink({ ...click, ctrlKey: true }, anchor('/carpetas'), origin)).toBeNull()
    expect(appLink({ ...click, button: 1 }, anchor('/carpetas'), origin)).toBeNull()
    expect(appLink({ ...click, defaultPrevented: true }, anchor('/carpetas'), origin)).toBeNull()
  })
})

describe('listHref', () => {
  it('links a playlist', () => {
    expect(listHref(12)).toBe('/listas/12')
  })
})
