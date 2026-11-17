// Saved stations written before the English vocabulary (the queue's running
// station in localStorage). Mirrors upgradeStationDef in internal/store/vocab.go.

const VIEWS = { pistas: 'tracks', albumes: 'albums', artistas: 'artists' }

const FACET_NAMES = {
  decada: 'decade', anio: 'year', artista: 'artist', pais: 'country', genero: 'genre', estilo: 'style',
  coleccion: 'collection', carpeta: 'folder', calidad: 'quality', formato: 'format', estado: 'state',
  megusta: 'liked', puntaje: 'rating', escucha: 'listened',
}

// Keyed by the new facet name.
const FACET_VALUES = {
  quality: { 'sin-perdida': 'lossless', 'con-perdida': 'lossy' },
  state: { 'sin-identificar': 'unidentified', 'no-reproducible': 'unplayable', 'con-duplicados': 'duplicated' },
  liked: { si: 'yes' },
  listened: { nunca: 'never', 'hace-un-anio': 'year-ago' },
}

const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v)

// upgradeStation maps a station's mode, source view and facets from the
// Spanish words to the English ones, keeping every other field. A station
// already in the new words comes back equal; something that is not an object
// gives null. The argument is not modified.
export function upgradeStation(station) {
  if (!isObject(station)) return null
  const out = { ...station }
  if (typeof out.mode === 'string' && Object.hasOwn(VIEWS, out.mode)) out.mode = VIEWS[out.mode]
  if (isObject(out.source)) {
    const source = { ...out.source }
    if (typeof source.view === 'string' && Object.hasOwn(VIEWS, source.view)) source.view = VIEWS[source.view]
    if (isObject(source.facets)) {
      const facets = {}
      for (const [name, value] of Object.entries(source.facets)) {
        const newName = Object.hasOwn(FACET_NAMES, name) ? FACET_NAMES[name] : name
        const words = FACET_VALUES[newName]
        facets[newName] = typeof value === 'string' && words && Object.hasOwn(words, value) ? words[value] : value
      }
      source.facets = facets
    }
    out.source = source
  }
  return out
}
