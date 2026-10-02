// The logic behind Carpetas' detail list: entries, sorting, relative bar.
import { codecName, quality } from './format.js'

export const DEFAULT_SORT = { column: null, desc: false }

// entries turns a /api/folders response into the rows of the detail list.
export function entries(folder) {
  const folders = (folder.children ?? []).map((c) => ({
    key: 'd:' + c.path,
    type: 'folder',
    name: c.name,
    path: c.path,
    tracks: c.trackCount,
    durationMs: c.durationMs,
    format: c.codec ? codecName({ codec: c.codec }) : '',
    sizeBytes: c.sizeBytes,
    album: c.album,
    collection: c.collection,
    playable: c.trackCount > 0,
  }))
  const files = (folder.files ?? []).map((f) => {
    const audio = f.kind === 'audio'
    return {
      key: 'f:' + f.id,
      type: audio ? 'audio' : 'other',
      name: f.name,
      id: f.id,
      tracks: 0,
      durationMs: audio ? f.durationMs : 0,
      format: audio ? quality(f) : '',
      sizeBytes: f.sizeBytes,
      album: 0,
      collection: false,
      playable: audio ? !!f.playable : false,
      track: !!f.track,
    }
  })
  return { folders, files }
}

const collator = new Intl.Collator('es', { numeric: true, sensitivity: 'base' })

const KEYS = {
  name: (e) => e.name,
  format: (e) => e.format,
  size: (e) => e.sizeBytes,
  tracks: (e) => e.tracks,
  duration: (e) => e.durationMs,
}

// sortEntries returns a sorted copy; ties keep their order (also when descending).
export function sortEntries(list, sort) {
  const key = KEYS[sort?.column]
  if (!key) return [...list]
  const text = sort.column === 'name' || sort.column === 'format'
  const dir = sort.desc ? -1 : 1
  return list
    .map((e, i) => [e, i])
    .sort(([a, i], [b, j]) => {
      const x = key(a)
      const y = key(b)
      const c = text ? collator.compare(x ?? '', y ?? '') : (x ?? 0) - (y ?? 0)
      return c * dir || i - j
    })
    .map(([e]) => e)
}

// nextSort is what a click on a column header does.
export function nextSort(sort, column) {
  if (sort.column === column) return { column, desc: !sort.desc }
  return { column, desc: column !== 'name' && column !== 'format' }
}

// bar is the percentage of the size bar, relative to the open folder.
export function bar(sizeBytes, totalBytes) {
  if (totalBytes <= 0 || !sizeBytes) return 0
  return Math.min(100, Math.max(1, Math.round((sizeBytes / totalBytes) * 100)))
}

// playableIds are the ids of the playable audio entries, in order.
export function playableIds(files) {
  return files.filter((e) => e.type === 'audio' && e.playable).map((e) => e.id)
}
