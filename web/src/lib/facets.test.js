import { describe, expect, it } from 'vitest'
import { chips, inside, parse, toSearch, valueLabel, withFacet, withOrder, withView } from './facets.js'

describe('parse and toSearch', () => {
  it('round-trips queries, leaving defaults out', () => {
    const q = parse('?decada=1980&anio=1986&pais=AR&orden=titulo&x=1')
    expect(q).toEqual({ view: 'albumes', facets: { decada: '1980', anio: '1986', pais: 'AR' }, order: 'titulo', dir: 'asc' })
    expect(toSearch(q)).toBe('?decada=1980&anio=1986&pais=AR&orden=titulo')
    expect(toSearch(parse(''))).toBe('')
    expect(toSearch(parse('?orden=anio&dir=desc'))).toBe('')
    expect(toSearch(parse('?dir=asc'))).toBe('?dir=asc')
  })
  it('knows each tab and its orders', () => {
    expect(parse('?ver=artistas')).toEqual({ view: 'artistas', facets: {}, order: 'titulo', dir: 'asc' })
    expect(parse('?ver=pistas&orden=cantidad').order).toBe('artista')
    expect(parse('?ver=otra').view).toBe('albumes')
    expect(toSearch(parse('?ver=artistas&orden=cantidad'))).toBe('?ver=artistas&orden=cantidad')
  })
})

describe('changing the query', () => {
  const base = parse('?decada=1980&anio=1986&genero=Rock&estilo=Pop+Rock')
  it('keeps years inside the decade and styles with their genre', () => {
    expect(withFacet(base, 'decada', '1990').facets).toEqual({ decada: '1990', genero: 'Rock', estilo: 'Pop Rock' })
    expect(withFacet(base, 'decada', null).facets).toEqual({ genero: 'Rock', estilo: 'Pop Rock' })
    expect(withFacet(parse(''), 'anio', '1975').facets).toEqual({ anio: '1975', decada: '1970' })
    expect(withFacet(base, 'genero', 'Latin').facets).toEqual({ decada: '1980', anio: '1986', genero: 'Latin' })
    expect(withFacet(base, 'estilo', null).facets).toEqual({ decada: '1980', anio: '1986', genero: 'Rock' })
  })
  it('switches tabs and orders', () => {
    const q = withView(withOrder(base, 'titulo'), 'pistas')
    expect([q.view, q.order, q.dir, q.facets.genero]).toEqual(['pistas', 'artista', 'asc', 'Rock'])
    expect(withOrder(base, 'agregado').dir).toBe('desc')
  })
})

describe('labels', () => {
  it('names values', () => {
    expect(valueLabel('decada', '1980')).toBe('1980s')
    expect(valueLabel('pais', 'UY')).toBe('Uruguay')
    expect(valueLabel('formato', 'flac')).toBe('FLAC')
    expect(valueLabel('calidad', 'sin-perdida')).toBe('Sin pérdida')
    expect(valueLabel('estado', 'no-reproducible')).toBe('No reproducible')
    expect(valueLabel('artista', 'charly-garcia', { label: 'Charly García' })).toBe('Charly García')
    expect(valueLabel('artista', 'x')).toBe('x')
  })
  it('lists chips and nested values', () => {
    const counts = { artista: [{ value: 'a1', label: 'Soda Stereo', count: 2 }], anio: [{ value: '1986' }, { value: '1995' }],
      estilo: [{ value: 'Pop Rock' }] }
    const q = parse('?decada=1980&artista=a1')
    expect(chips(q, counts)).toEqual([
      { name: 'decada', value: '1980', label: '1980s' },
      { name: 'artista', value: 'a1', label: 'Soda Stereo' },
    ])
    expect(inside('decada', q, counts)).toEqual([{ value: '1986' }])
    expect(inside('genero', q, counts)).toEqual([])
    expect(inside('genero', parse('?genero=Rock'), counts)).toEqual([{ value: 'Pop Rock' }])
  })
})

describe('my data facets', () => {
  it('keeps megusta, puntaje and the escuchado order', () => {
    const q = parse('?megusta=si&puntaje=4&orden=escuchado')
    expect(q.facets).toEqual({ megusta: 'si', puntaje: '4' })
    expect(q.order).toBe('escuchado')
    expect(toSearch(q)).toBe('?megusta=si&puntaje=4&orden=escuchado')
    expect(parse('?ver=pistas&orden=escuchado').order).toBe('escuchado')
  })
  it('labels them', () => {
    expect(valueLabel('puntaje', '4')).toBe('★★★★ o más')
    expect(valueLabel('megusta', 'si')).toBe('Me gusta')
  })
})
