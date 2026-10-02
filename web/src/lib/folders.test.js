import { describe, expect, it } from 'vitest'
import { bar, DEFAULT_SORT, entries, nextSort, playableIds, sortEntries } from './folders.js'

const folder = {
  sizeBytes: 3_000_000_000,
  children: [
    { path: 'm/RARAS', name: 'RARAS PARTITURAS', trackCount: 183, durationMs: 39_600_000, sizeBytes: 1_400_000_000, codec: 'mp3', album: 0, collection: false },
    { path: 'm/Lhasa', name: 'Lhasa de Sela', trackCount: 35, durationMs: 7_200_000, sizeBytes: 410_000_000, codec: 'flac', album: 4, collection: false },
    { path: 'm/vacia', name: 'vacía', trackCount: 0, durationMs: 0, sizeBytes: 0, codec: '', album: 0, collection: false },
  ],
  files: [
    { id: 9, name: 'suelto.mp3', kind: 'audio', sizeBytes: 7_200_000, durationMs: 187_000, format: 'mp3', codec: 'mp3', bitrate: 320, playable: true, track: true },
    { id: 10, name: 'raro.wma', kind: 'audio', sizeBytes: 5_000_000, durationMs: 100_000, format: 'asf', codec: 'wma', bitrate: 128, playable: false, track: true },
    { id: 11, name: 'folder.jpg', kind: 'image', sizeBytes: 84_000, playable: false, track: false },
  ],
}

describe('entries', () => {
  it('lists subfolders, then files', () => {
    const { folders, files } = entries(folder)
    expect(folders.map((e) => [e.type, e.name, e.format, e.playable])).toEqual([
      ['folder', 'RARAS PARTITURAS', 'MP3', true], ['folder', 'Lhasa de Sela', 'FLAC', true], ['folder', 'vacía', '', false],
    ])
    expect(files.map((e) => [e.type, e.name, e.format, e.playable])).toEqual([
      ['audio', 'suelto.mp3', 'MP3 320', true], ['audio', 'raro.wma', 'WMA 128', false], ['other', 'folder.jpg', '', false],
    ])
    expect(folders[1]).toMatchObject({ key: 'd:m/Lhasa', path: 'm/Lhasa', tracks: 35, album: 4 })
    expect(files[0]).toMatchObject({ key: 'f:9', id: 9, durationMs: 187_000, sizeBytes: 7_200_000 })
  })
  it('copes with a folder without files', () => {
    expect(entries({ children: [] })).toEqual({ folders: [], files: [] })
  })
})

describe('sorting', () => {
  const { folders, files } = entries(folder)
  it('keeps the server order by default', () => {
    expect(sortEntries(folders, DEFAULT_SORT).map((e) => e.name)).toEqual(['RARAS PARTITURAS', 'Lhasa de Sela', 'vacía'])
  })
  it('sorts by name in natural order, and back', () => {
    const named = [{ name: '[LA-10] b' }, { name: '[LA-9] a' }, { name: 'Árbol' }]
    expect(sortEntries(named, { column: 'name', desc: false }).map((e) => e.name)).toEqual(['[LA-9] a', '[LA-10] b', 'Árbol'])
    expect(sortEntries(named, { column: 'name', desc: true }).map((e) => e.name)).toEqual(['Árbol', '[LA-10] b', '[LA-9] a'])
  })
  it('sorts by size, tracks, duration and format', () => {
    expect(sortEntries(folders, { column: 'size', desc: true }).map((e) => e.name)).toEqual(['RARAS PARTITURAS', 'Lhasa de Sela', 'vacía'])
    expect(sortEntries(folders, { column: 'tracks', desc: false }).map((e) => e.tracks)).toEqual([0, 35, 183])
    expect(sortEntries(files, { column: 'duration', desc: false }).map((e) => e.name)).toEqual(['folder.jpg', 'raro.wma', 'suelto.mp3'])
    expect(sortEntries(folders, { column: 'format', desc: false }).map((e) => e.format)).toEqual(['', 'FLAC', 'MP3'])
  })
  it('is stable on ties, both ways', () => {
    const tied = [{ name: 'a', sizeBytes: 1 }, { name: 'b', sizeBytes: 1 }]
    expect(sortEntries(tied, { column: 'size', desc: true }).map((e) => e.name)).toEqual(['a', 'b'])
  })
  it('flips the same column, starts numbers biggest first', () => {
    expect(nextSort(DEFAULT_SORT, 'name')).toEqual({ column: 'name', desc: false })
    expect(nextSort({ column: 'name', desc: false }, 'name')).toEqual({ column: 'name', desc: true })
    expect(nextSort({ column: 'name', desc: false }, 'size')).toEqual({ column: 'size', desc: true })
    expect(nextSort(DEFAULT_SORT, 'format')).toEqual({ column: 'format', desc: false })
  })
})

describe('bar', () => {
  it('is relative to the folder', () => {
    expect(bar(1_400_000_000, 3_000_000_000)).toBe(47)
    expect(bar(1, 3_000_000_000)).toBe(1)
    expect(bar(0, 3_000_000_000)).toBe(0)
    expect(bar(5, 0)).toBe(0)
    expect(bar(10, 5)).toBe(100)
  })
})

describe('playableIds', () => {
  it('keeps the playable audio, in order', () => {
    expect(playableIds(entries(folder).files)).toEqual([9])
  })
})
