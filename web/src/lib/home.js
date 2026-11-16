// Inicio's sections and their order, drawn from the same seed as the
// server's rotating rows: the same seed (a scan ending, say) keeps the page
// as it is; a new one (a visit, the refresh button) reshuffles it.
import { shuffled } from './queue.js'

// seededRandom returns a Math.random-like function (mulberry32).
export function seededRandom(seed) {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

// homeSections lists Inicio's sections in a seeded order: the stations
// always, the fixed rows that have albums and the rotating rows.
export function homeSections(data, seed) {
  const sections = [{ key: 'stations' }]
  for (const key of ['continue', 'likedTracks', 'liked']) {
    if (data[key]?.length) sections.push({ key })
  }
  for (const row of data.rotating ?? []) {
    if (row.albums?.length) sections.push({ key: `${row.kind}:${row.value}`, row })
  }
  return shuffled(sections, seededRandom(seed))
}
