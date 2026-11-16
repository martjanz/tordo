import { describe, expect, it } from 'vitest'
import { chips, inside, parse, toSearch, valueLabel, withFacet, withOrder, withView } from './facets.js'

describe('parse and toSearch', () => {
  it('round-trips queries, leaving defaults out', () => {
    const q = parse('?decade=1980&year=1986&country=AR&order=title&x=1')
    expect(q).toEqual({ view: 'albums', facets: { decade: '1980', year: '1986', country: 'AR' }, order: 'title', dir: 'asc' })
    expect(toSearch(q)).toBe('?decade=1980&year=1986&country=AR&order=title')
    expect(toSearch(parse(''))).toBe('')
    expect(toSearch(parse('?order=year&dir=desc'))).toBe('')
    expect(toSearch(parse('?dir=asc'))).toBe('?dir=asc')
  })
  it('knows each tab and its orders', () => {
    expect(parse('?view=artists')).toEqual({ view: 'artists', facets: {}, order: 'title', dir: 'asc' })
    expect(parse('?view=tracks&order=count').order).toBe('artist')
    expect(parse('?view=otra').view).toBe('albums')
    expect(toSearch(parse('?view=artists&order=count'))).toBe('?view=artists&order=count')
  })
})

describe('the random order', () => {
  it('carries its seed in the URL, and only with that order', () => {
    expect(parse('?order=random&seed=42')).toEqual({ view: 'albums', facets: {}, order: 'random', dir: 'desc', seed: '42' })
    expect(toSearch(parse('?order=random&seed=42'))).toBe('?order=random&seed=42')
    expect(toSearch(parse('?order=random'))).toBe('?order=random')
    expect(toSearch(parse('?order=title&seed=42'))).toBe('?order=title')
    // The server echoes it as a number.
    expect(toSearch({ view: 'tracks', facets: {}, order: 'random', dir: 'desc', seed: 42 })).toBe('?view=tracks&order=random&seed=42')
  })
  it('is on every tab', () => {
    for (const view of ['albums', 'artists', 'tracks']) expect(parse(`?view=${view}&order=random`).order).toBe('random')
  })
  it('draws a new seed when chosen, and drops it for another order or tab', () => {
    const a = withOrder(parse(''), 'random')
    expect(Number(a.seed)).toBeGreaterThan(0)
    expect(Number(a.seed)).toBeLessThanOrEqual(2 ** 31)
    const seeds = new Set(Array.from({ length: 10 }, () => withOrder(a, 'random').seed))
    expect(seeds.size).toBeGreaterThan(1)
    expect(withOrder(a, 'title')).not.toHaveProperty('seed')
    expect(withView(a, 'tracks')).not.toHaveProperty('seed')
  })
})

describe('changing the query', () => {
  const base = parse('?decade=1980&year=1986&genre=Rock&style=Pop+Rock')
  it('keeps years inside the decade and styles with their genre', () => {
    expect(withFacet(base, 'decade', '1990').facets).toEqual({ decade: '1990', genre: 'Rock', style: 'Pop Rock' })
    expect(withFacet(base, 'decade', null).facets).toEqual({ genre: 'Rock', style: 'Pop Rock' })
    expect(withFacet(parse(''), 'year', '1975').facets).toEqual({ year: '1975', decade: '1970' })
    expect(withFacet(base, 'genre', 'Latin').facets).toEqual({ decade: '1980', year: '1986', genre: 'Latin' })
    expect(withFacet(base, 'style', null).facets).toEqual({ decade: '1980', year: '1986', genre: 'Rock' })
  })
  it('switches tabs and orders', () => {
    const q = withView(withOrder(base, 'title'), 'tracks')
    expect([q.view, q.order, q.dir, q.facets.genre]).toEqual(['tracks', 'artist', 'asc', 'Rock'])
    expect(withOrder(base, 'added').dir).toBe('desc')
  })
})

describe('labels', () => {
  it('names values', () => {
    expect(valueLabel('decade', '1980')).toBe('1980s')
    expect(valueLabel('country', 'UY')).toBe('Uruguay')
    expect(valueLabel('format', 'flac')).toBe('FLAC')
    expect(valueLabel('quality', 'lossless')).toBe('Sin pérdida')
    expect(valueLabel('state', 'unplayable')).toBe('No reproducible')
    expect(valueLabel('state', 'duplicated')).toBe('Con duplicados')
    expect(valueLabel('artist', 'charly-garcia', { label: 'Charly García' })).toBe('Charly García')
    expect(valueLabel('artist', 'x')).toBe('x')
  })
  it('lists chips and nested values', () => {
    const counts = { artist: [{ value: 'a1', label: 'Soda Stereo', count: 2 }], year: [{ value: '1986' }, { value: '1995' }],
      style: [{ value: 'Pop Rock' }] }
    const q = parse('?decade=1980&artist=a1')
    expect(chips(q, counts)).toEqual([
      { name: 'decade', value: '1980', label: '1980s' },
      { name: 'artist', value: 'a1', label: 'Soda Stereo' },
    ])
    expect(inside('decade', q, counts)).toEqual([{ value: '1986' }])
    expect(inside('genre', q, counts)).toEqual([])
    expect(inside('genre', parse('?genre=Rock'), counts)).toEqual([{ value: 'Pop Rock' }])
  })
})

describe('my data facets', () => {
  it('keeps liked, rating and the plays order', () => {
    const q = parse('?liked=yes&rating=4&order=plays')
    expect(q.facets).toEqual({ liked: 'yes', rating: '4' })
    expect(q.order).toBe('plays')
    expect(toSearch(q)).toBe('?liked=yes&rating=4&order=plays')
    expect(parse('?view=tracks&order=plays').order).toBe('plays')
  })
  it('labels them', () => {
    expect(valueLabel('rating', '4')).toBe('★★★★ o más')
    expect(valueLabel('liked', 'yes')).toBe('Me gusta')
  })
  it('calls the liked facet Preferidos on the albums tab only', async () => {
    const F = await import('./facets.js')
    expect(F.facetLabel({ name: 'liked', label: 'Me gusta' }, 'albums')).toBe('Preferidos')
    expect(F.facetLabel({ name: 'liked', label: 'Me gusta' }, 'tracks')).toBe('Me gusta')
    expect(F.facetLabel({ name: 'liked', label: 'Me gusta' }, 'artists')).toBe('Me gusta')
    expect(F.facetLabel({ name: 'country', label: 'País' }, 'albums')).toBe('País')
    expect(valueLabel('liked', 'yes', undefined, 'albums')).toBe('Preferidos')
    expect(valueLabel('liked', 'yes', undefined, 'tracks')).toBe('Me gusta')
    expect(chips({ view: 'albums', facets: { liked: 'yes' } })[0].label).toBe('Preferidos')
    expect(chips({ view: 'tracks', facets: { liked: 'yes' } })[0].label).toBe('Me gusta')
  })
})

describe('listened', () => {
  it('is a facet with its labels', async () => {
    const F = await import('./facets.js')
    const i = F.FACETS.findIndex((f) => f.name === 'rating')
    expect(F.FACETS[i + 1]).toEqual({ name: 'listened', label: 'Escucha' })
    expect(F.valueLabel('listened', 'never')).toBe('Nunca escuchado')
    expect(F.valueLabel('listened', 'year-ago')).toBe('Sin escuchar hace un año')
    const q = F.parse('?listened=never')
    expect(q.facets).toEqual({ listened: 'never' })
    expect(F.toSearch(q)).toBe('?listened=never')
  })
})
