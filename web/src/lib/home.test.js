import { describe, expect, it } from 'vitest'
import { homeSections, seededRandom } from './home.js'

const data = {
  continue: [{ id: 1 }],
  likedTracks: [],
  liked: [{ id: 2 }],
  rotating: [
    { kind: 'decade', value: '1980', albums: [{ id: 3 }] },
    { kind: 'random', value: '', albums: [] },
    { kind: 'recent', value: '', albums: [{ id: 4 }] },
  ],
}
const keys = (sections) => sections.map((s) => s.key)

describe('seededRandom', () => {
  it('gives the same numbers for the same seed, in [0, 1)', () => {
    const a = seededRandom(42)
    const b = seededRandom(42)
    for (let i = 0; i < 20; i++) {
      const x = a()
      expect(x).toBe(b())
      expect(x).toBeGreaterThanOrEqual(0)
      expect(x).toBeLessThan(1)
    }
    expect(seededRandom(1)()).not.toBe(seededRandom(2)())
  })
})

describe('homeSections', () => {
  it('holds the stations, the non-empty fixed rows and the rotating rows with albums', () => {
    expect(keys(homeSections(data, 7)).sort()).toEqual(['continue', 'decade:1980', 'liked', 'recent:', 'stations'])
  })

  it('keeps the order for a seed and varies it across seeds', () => {
    expect(keys(homeSections(data, 7))).toEqual(keys(homeSections(data, 7)))
    const orders = new Set()
    for (let seed = 1; seed <= 50; seed++) orders.add(keys(homeSections(data, seed)).join(','))
    expect(orders.size).toBeGreaterThan(10)
  })

  it('tolerates missing rows', () => {
    expect(keys(homeSections({}, 1))).toEqual(['stations'])
  })
})
