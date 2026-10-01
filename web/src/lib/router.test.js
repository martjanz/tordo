import { describe, expect, it } from 'vitest'
import { albumHref, appLink, folderHref, resolve } from './router.js'

describe('resolve', () => {
  it.each([
    ['/', '', { page: 'carpetas', path: '' }],
    ['/carpetas', '', { page: 'carpetas', path: '' }],
    ['/carpetas', '?ruta=..%2Fmusica%2FLatinoam%C3%A9rica%20600', { page: 'carpetas', path: '../musica/Latinoamérica 600' }],
    ['/listas', '', { page: 'listas' }],
    ['/album/12', '', { page: 'album', id: 12 }],
    ['/album/0', '', { page: 'notfound' }],
    ['/album/abc', '', { page: 'notfound' }],
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
