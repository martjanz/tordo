import { describe, it, expect } from 'vitest'
import {
  visibleRows, missingLevels, locate, reveal, keyAction, isFilterKey,
  OPEN_KEY, loadOpen, saveOpen, filterOpen, matchRows, direction, shortPath,
} from './tree.js'

const n = (path, label, hasChildren = true) => ({ path, label, name: label.split(' › ').pop(), tracks: 1, sizeBytes: 1, album: 0, collection: false, hasChildren })
const S = 'm/Soulseek/complete'
const levels = () => ({
  '': [n('m/Brasil', 'Brasil'), n(S, 'Soulseek › complete'), n('m/Uruguay', 'Uruguay', false)],
  [S]: [n(S + '/benteveo/Zitarrosa/Canta', 'benteveo › Zitarrosa › Canta', false), n(S + '/Low_ID', 'Low_ID')],
  [S + '/Low_ID']: [n(S + '/Low_ID/Lhasa', 'Lhasa', false)],
})

describe('visibleRows', () => {
  it('lists the top level when nothing is open', () => {
    const rows = visibleRows(levels(), {})
    expect(rows.map((r) => r.node.path)).toEqual(['m/Brasil', S, 'm/Uruguay'])
    expect(rows.map((r) => r.depth)).toEqual([1, 1, 1])
    expect(rows.map((r) => r.parent)).toEqual(['', '', ''])
  })
  it('lists children of open loaded rows depth first', () => {
    const rows = visibleRows(levels(), { [S]: true, [S + '/Low_ID']: true })
    expect(rows.map((r) => r.node.path)).toEqual([
      'm/Brasil', S, S + '/benteveo/Zitarrosa/Canta', S + '/Low_ID', S + '/Low_ID/Lhasa', 'm/Uruguay',
    ])
    expect(rows.map((r) => r.depth)).toEqual([1, 1, 2, 2, 3, 1])
    expect(rows[4].parent).toBe(S + '/Low_ID')
  })
  it('shows no children for an open but unloaded row', () => {
    const rows = visibleRows(levels(), { 'm/Brasil': true })
    expect(rows.map((r) => r.node.path)).toEqual(['m/Brasil', S, 'm/Uruguay'])
  })
  it('returns [] for an unknown key', () => {
    expect(visibleRows(levels(), {}, 'nope')).toEqual([])
  })
})

describe('missingLevels', () => {
  it('collects visible open rows whose level is not loaded', () => {
    expect(missingLevels(levels(), { 'm/Brasil': true, [S]: true, 'm/Uruguay': true })).toEqual(['m/Brasil'])
  })
})

describe('locate', () => {
  it('walks down the chain to a nested row', () => {
    expect(locate(levels(), S + '/Low_ID/Lhasa')).toEqual({ chain: [S, S + '/Low_ID'], node: S + '/Low_ID/Lhasa', missing: null })
  })
  it('finds a pass-through folder inside a row', () => {
    expect(locate(levels(), 'm/Soulseek')).toEqual({ chain: [], node: S, missing: null })
  })
  it('finds a middle folder of a chain', () => {
    expect(locate(levels(), S + '/benteveo/Zitarrosa')).toEqual({ chain: [S], node: S + '/benteveo/Zitarrosa/Canta', missing: null })
  })
  it('reports the unloaded level', () => {
    expect(locate(levels(), 'm/Brasil/x')).toEqual({ chain: ['m/Brasil'], node: null, missing: 'm/Brasil' })
  })
  it('does not match the root above the level', () => {
    const r = locate(levels(), 'm')
    expect(r.node).toBeNull()
    expect(r.missing).toBeNull()
  })
  it('does not match a name prefix that is not a folder', () => {
    expect(locate(levels(), 'm/Soul').node).toBeNull()
  })
  it('reports the top level missing with no levels', () => {
    expect(locate({}, 'm/Brasil').missing).toBe('')
  })
  it('handles the empty target', () => {
    expect(locate(levels(), '')).toEqual({ chain: [], node: null, missing: null })
  })
})

describe('reveal', () => {
  const mk = () => {
    const keys = []
    const load = async (key) => {
      keys.push(key)
      if (key === 'm/Brasil') return [n('m/Brasil/x', 'x', false)]
      return levels()[key]
    }
    return { keys, load }
  }
  it('loads the missing levels in order', async () => {
    const { keys, load } = mk()
    const lv = {}
    const r = await reveal(lv, S + '/Low_ID/Lhasa', load)
    expect(keys).toEqual(['', S, S + '/Low_ID'])
    expect(r).toEqual({ chain: [S, S + '/Low_ID'], node: S + '/Low_ID/Lhasa' })
  })
  it('loads nothing the second time', async () => {
    const { keys, load } = mk()
    const lv = {}
    await reveal(lv, S + '/Low_ID/Lhasa', load)
    keys.length = 0
    await reveal(lv, S + '/Low_ID/Lhasa', load)
    expect(keys).toEqual([])
  })
})

describe('keyAction', () => {
  const open = { [S]: true }
  const rows = () => visibleRows(levels(), open)
  const CANTA = S + '/benteveo/Zitarrosa/Canta'
  const LOW = S + '/Low_ID'

  it('ArrowDown moves to the next row', () => {
    expect(keyAction(rows(), 'm/Brasil', 'ArrowDown', open)).toEqual({ type: 'cursor', path: S })
  })
  it('ArrowUp stays at the first row', () => {
    expect(keyAction(rows(), 'm/Brasil', 'ArrowUp', open)).toEqual({ type: 'cursor', path: 'm/Brasil' })
  })
  it('End and Home jump to the ends', () => {
    expect(keyAction(rows(), 'm/Brasil', 'End', open)).toEqual({ type: 'cursor', path: 'm/Uruguay' })
    expect(keyAction(rows(), 'm/Uruguay', 'Home', open)).toEqual({ type: 'cursor', path: 'm/Brasil' })
  })
  it('ArrowRight opens a closed row', () => {
    expect(keyAction(rows(), 'm/Brasil', 'ArrowRight', open)).toEqual({ type: 'open', path: 'm/Brasil' })
  })
  it('ArrowRight on an open row moves to its first child', () => {
    expect(keyAction(rows(), S, 'ArrowRight', open)).toEqual({ type: 'cursor', path: CANTA })
  })
  it('ArrowRight on a leaf does nothing', () => {
    expect(keyAction(rows(), 'm/Uruguay', 'ArrowRight', open)).toBeNull()
  })
  it('ArrowRight on an open row still loading does nothing', () => {
    const o = { [S]: true, [LOW]: true }
    const lv = levels()
    delete lv[LOW]
    expect(keyAction(visibleRows(lv, o), LOW, 'ArrowRight', o)).toBeNull()
  })
  it('ArrowLeft closes an open row', () => {
    expect(keyAction(rows(), S, 'ArrowLeft', open)).toEqual({ type: 'close', path: S })
  })
  it('ArrowLeft on a child goes to the parent', () => {
    expect(keyAction(rows(), CANTA, 'ArrowLeft', open)).toEqual({ type: 'cursor', path: S })
  })
  it('ArrowLeft on a closed top row does nothing', () => {
    expect(keyAction(rows(), 'm/Brasil', 'ArrowLeft', open)).toBeNull()
  })
  it('Enter shows the row', () => {
    expect(keyAction(rows(), LOW, 'Enter', open)).toEqual({ type: 'show', path: LOW })
  })
  it('ignores other keys', () => {
    expect(keyAction(rows(), LOW, 'x', open)).toBeNull()
  })
  it('without a cursor, navigation keys select the first row', () => {
    expect(keyAction(rows(), null, 'ArrowDown', open)).toEqual({ type: 'cursor', path: 'm/Brasil' })
    expect(keyAction(rows(), null, 'Enter', open)).toBeNull()
  })
  it('returns null with no rows', () => {
    expect(keyAction([], null, 'ArrowDown', {})).toBeNull()
  })
})

describe('isFilterKey', () => {
  it('accepts / outside text fields', () => {
    expect(isFilterKey({ key: '/', target: { tagName: 'BODY' } })).toBe(true)
  })
  it('rejects modifiers', () => {
    expect(isFilterKey({ key: '/', ctrlKey: true, target: { tagName: 'BODY' } })).toBe(false)
  })
  it('rejects text fields', () => {
    expect(isFilterKey({ key: '/', target: { tagName: 'INPUT' } })).toBe(false)
  })
  it('rejects contenteditable', () => {
    expect(isFilterKey({ key: '/', target: { tagName: 'DIV', isContentEditable: true } })).toBe(false)
  })
  it('rejects other keys', () => {
    expect(isFilterKey({ key: 'a', target: { tagName: 'BODY' } })).toBe(false)
  })
})

describe('loadOpen / saveOpen', () => {
  const fake = (initial) => {
    const m = new Map(initial ? [[OPEN_KEY, initial]] : [])
    return { getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => m.set(k, v) }
  }
  it('round trips only the open entries', () => {
    const s = fake()
    saveOpen(s, { a: true, b: true, c: false })
    expect(loadOpen(s)).toEqual({ a: true, b: true })
  })
  it('returns {} for bad data', () => {
    for (const bad of ['nope', '{"a":1}', '[1]']) expect(loadOpen(fake(bad))).toEqual({})
    expect(loadOpen(fake())).toEqual({})
  })
  it('returns {} when getItem throws', () => {
    expect(loadOpen({ getItem: () => { throw new Error('x') } })).toEqual({})
  })
  it('swallows setItem errors', () => {
    expect(() => saveOpen({ setItem: () => { throw new Error('x') } }, { a: true })).not.toThrow()
  })
})

describe('filterOpen', () => {
  it('opens every level except the top', () => {
    expect(filterOpen({ '': [], a: [], b: [] })).toEqual({ a: true, b: true })
  })
})

describe('matchRows', () => {
  it('maps matches to rows in match order, skipping unknown ones', () => {
    const lv = levels()
    const r = matchRows({ levels: lv, matches: [S + '/Low_ID/Lhasa', 'm/nope', 'm/Uruguay'] })
    expect(r).toEqual([
      { node: lv[S + '/Low_ID'][0], parent: S + '/Low_ID' },
      { node: lv[''][2], parent: '' },
    ])
  })
})

describe('direction', () => {
  it('classifies moves', () => {
    expect(direction('', 'm/a')).toBe('forward')
    expect(direction('m/a', 'm/a/b')).toBe('forward')
    expect(direction('m/a/b', 'm/a')).toBe('back')
    expect(direction('m/a', 'm/ab')).toBe('back')
    expect(direction('m/a', 'm/a')).toBe('none')
  })
})

describe('shortPath', () => {
  it('collapses middle segments', () => {
    expect(shortPath('../Music/Soulseek/complete/Low_ID/Lhasa', '../Music')).toBe('Soulseek/…/Low_ID/Lhasa')
  })
  it('keeps short paths', () => {
    expect(shortPath('../Music/Uruguay/Zitarrosa', '../Music')).toBe('Uruguay/Zitarrosa')
  })
  it('strips leading ../ without a root', () => {
    expect(shortPath('../../otro/a/b', '')).toBe('otro/a/b')
  })
})
