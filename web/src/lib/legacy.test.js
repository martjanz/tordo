import { describe, expect, it } from 'vitest'
import { upgradeStation } from './legacy.js'

// The same fixture as Go's TestUpgradeStationDef (internal/store/vocab_test.go).
const old = { name: 'x', source: { view: 'pistas', facets: { megusta: 'si', calidad: 'sin-perdida', decada: '1980' } }, mode: 'albumes', artistGap: 3 }
const now = { name: 'x', source: { view: 'tracks', facets: { liked: 'yes', quality: 'lossless', decade: '1980' } }, mode: 'albums', artistGap: 3 }

describe('upgradeStation', () => {
  it('maps the old words to the new ones, keeping the rest', () => {
    expect(upgradeStation(old)).toEqual(now)
  })
  it('maps facet values by the new facet name', () => {
    expect(
      upgradeStation({ source: { view: 'artistas', facets: { estado: 'con-duplicados', escucha: 'hace-un-anio', puntaje: '4', pais: 'AR' } } }),
    ).toEqual({ source: { view: 'artists', facets: { state: 'duplicated', listened: 'year-ago', rating: '4', country: 'AR' } } })
    expect(upgradeStation({ source: { facets: { calidad: 'con-perdida', estado: 'sin-identificar', nunca: 'x', escucha: 'nunca' } } })).toEqual({
      source: { facets: { quality: 'lossy', state: 'unidentified', nunca: 'x', listened: 'never' } },
    })
    expect(upgradeStation({ source: { facets: { estado: 'no-reproducible' } } }).source.facets).toEqual({ state: 'unplayable' })
  })
  it('leaves new definitions as they are', () => {
    expect(upgradeStation(now)).toEqual(now)
    for (const s of [
      { source: { view: 'tracks', facets: { liked: 'yes', listened: 'year-ago' } }, mode: 'tracks' },
      { source: { folder: 'Brasil' } },
      { source: { facets: { genre: 'si', artist: 'nunca' } } },
      {},
    ]) {
      expect(upgradeStation(s)).toEqual(s)
    }
  })
  it('does not change its argument', () => {
    const copy = structuredClone(old)
    upgradeStation(old)
    expect(old).toEqual(copy)
  })
  it('gives null for what it cannot read', () => {
    expect(upgradeStation(null)).toBeNull()
    expect(upgradeStation(undefined)).toBeNull()
    expect(upgradeStation('x')).toBeNull()
    expect(upgradeStation([])).toBeNull()
  })
})
