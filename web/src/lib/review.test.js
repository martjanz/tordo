import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from './i18n/index.svelte.js'
import { aliasLabel, candidateChip, candidateLine, confidenceLevel, folderDiffers, genreChoice, genreLinks, genreOptions, identification, percent, searchURL, worthAccepting } from './review.js'

describe('review', () => {
  it('reads scores and candidates', () => {
    expect(percent(0.873)).toBe('87 %')
    expect(percent(undefined)).toBe('0 %')
    expect(candidateLine({ artist: 'Soda Stereo', year: 1986, country: 'AR', tracks: 9 })).toBe('Soda Stereo · 1986 · Argentina · 9 pistas')
    expect(candidateLine({ artist: 'X', year: 0, country: '' })).toBe('X')
  })
  it('sums up the best candidate', () => {
    expect(candidateChip({ title: 'Signos', artist: 'Soda Stereo', year: 1986 })).toBe('Signos — Soda Stereo · 1986')
    expect(candidateChip({ title: 'Signos', artist: 'Soda Stereo', year: 0 })).toBe('Signos — Soda Stereo')
    expect(candidateChip({ title: 'Signos' })).toBe('Signos')
  })
  it('grades confidence like the bulk accept', () => {
    expect(confidenceLevel(0.7)).toBe('high')
    expect(confidenceLevel(0.93)).toBe('high')
    expect(confidenceLevel(0.69)).toBe('mid')
    expect(confidenceLevel(0.5)).toBe('mid')
    expect(confidenceLevel(0.49)).toBe('low')
    expect(confidenceLevel(undefined)).toBe('low')
  })
  it('shows the folder only when it says something the title does not', () => {
    const a = { tagTitle: 'Signos', tagArtist: 'Soda Stereo', year: 1986 }
    expect(folderDiffers({ ...a, dir: 'Musica/Signos' })).toBe(false)
    expect(folderDiffers({ ...a, dir: 'Musica/signos' })).toBe(false)
    expect(folderDiffers({ ...a, dir: 'Musica/Soda Stereo - Signos (1986)' })).toBe(false)
    expect(folderDiffers({ ...a, dir: 'Musica/1986 - Signos [FLAC]' })).toBe(true)
    expect(folderDiffers({ ...a, dir: 'Musica/CD1' })).toBe(true)
    expect(folderDiffers({ tagTitle: 'Canción Animal', dir: 'x/Cancion animal' })).toBe(false)
    expect(folderDiffers({ tagTitle: '', dir: 'x/Algo' })).toBe(true)
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
      { label: 'Rock', facets: { genre: 'Rock' } },
      { label: 'Latin', facets: { genre: 'Latin' } },
      { label: 'Pop Rock', facets: { genre: 'Rock', style: 'Pop Rock' } },
      { label: 'New Wave', facets: { genre: 'Rock', style: 'New Wave' } },
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

describe('review in other languages', () => {
  afterEach(() => setLocale('es'))

  it('speaks English', () => {
    setLocale('en')
    expect(identification({ status: 'auto', confidence: 0.873 })).toBe('Identified on MusicBrainz (87 %)')
    expect(identification({ status: 'unmatched' })).toBe('Unidentified: review the candidates')
    expect(candidateLine({ artist: 'Soda Stereo', year: 1986, country: 'AR', tracks: 9 })).toBe('Soda Stereo · 1986 · Argentina · 9 tracks')
    expect(aliasLabel({ genre: '' })).toBe('ignored')
    expect(genreOptions([])[0].label).toBe('Not a genre (ignore it)')
  })
})
