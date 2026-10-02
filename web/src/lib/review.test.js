import { describe, expect, it } from 'vitest'
import { aliasLabel, candidateLine, genreChoice, genreLinks, genreOptions, identification, percent, searchURL, worthAccepting } from './review.js'

describe('review', () => {
  it('reads scores and candidates', () => {
    expect(percent(0.873)).toBe('87 %')
    expect(percent(undefined)).toBe('0 %')
    expect(candidateLine({ artist: 'Soda Stereo', year: 1986, country: 'AR', tracks: 9 })).toBe('Soda Stereo · 1986 · Argentina · 9 pistas')
    expect(candidateLine({ artist: 'X', year: 0, country: '' })).toBe('X')
  })
  it('says where the data comes from', () => {
    expect(identification({ status: 'auto', confidence: 0.92 })).toBe('Identificado en MusicBrainz (92 %)')
    expect(identification({ status: 'ignored' })).toBe('No está en MusicBrainz')
    expect(identification({ status: '' })).toBe('Todavía sin buscar en MusicBrainz')
  })
  it('links genres and styles', () => {
    const gs = [
      { genre: 'Rock', style: 'Pop Rock' },
      { genre: 'Rock', style: 'New Wave' },
      { genre: 'Latin', source: 'tag' },
    ]
    expect(genreLinks(gs)).toEqual([
      { label: 'Rock', facets: { genero: 'Rock' } },
      { label: 'Latin', facets: { genero: 'Latin' } },
      { label: 'Pop Rock', facets: { genero: 'Rock', estilo: 'Pop Rock' } },
      { label: 'New Wave', facets: { genero: 'Rock', estilo: 'New Wave' } },
    ])
    expect(genreLinks(undefined)).toEqual([])
  })
  it('picks the albums worth accepting', () => {
    const q = [{ id: 1, candidates: [{ score: 0.78 }] }, { id: 2, candidates: [{ score: 0.5 }] }, { id: 3, candidates: [] }]
    expect(worthAccepting(q).map((a) => a.id)).toEqual([1])
  })
  it('maps raw genres', () => {
    const opts = genreOptions([{ name: 'Folk, World, & Country', styles: ['Folk'] }, { name: 'Latin', styles: ['Samba'] }])
    expect(opts.map((o) => o.label)).toEqual([
      'No es un género (ignorarlo)',
      'Folk, World, & Country',
      'Folk, World, & Country › Folk',
      'Latin',
      'Latin › Samba',
    ])
    expect(genreChoice(opts[0].value)).toEqual({ genre: '', style: '' })
    expect(genreChoice(opts[2].value)).toEqual({ genre: 'Folk, World, & Country', style: 'Folk' })
    expect(genreChoice(opts[3].value)).toEqual({ genre: 'Latin', style: '' })
    expect(aliasLabel({ raw: 'cueca', genre: '' })).toBe('ignorado')
    expect(aliasLabel({ raw: 'cueca', genre: 'Latin', style: 'Samba' })).toBe('Latin › Samba')
  })
  it('looks albums up on MusicBrainz', () => {
    expect(searchURL({ title: 'Signos (MB)', tagTitle: 'Signos', artist: 'Soda Stereo' })).toBe(
      'https://musicbrainz.org/search?type=release&query=Signos%20Soda%20Stereo',
    )
  })
})
