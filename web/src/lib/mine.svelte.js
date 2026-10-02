// The user's marks (likes and ratings), shared by the whole app. Changes
// are optimistic: shown at once, rolled back if the server refuses.
import { api } from './api.js'
import { notify } from './app.svelte.js'
import * as M from './marks.js'

export const mine = $state({ marks: M.empty(), loaded: false })

// loadMine reads the marks; on an error the last ones stay.
export async function loadMine() {
  try {
    mine.marks = M.fromServer(await api.marks())
    mine.loaded = true
  } catch {
    // keep what we have
  }
}

// change applies next, sends the request, and puts back the previous
// marks when it fails.
async function change(next, request) {
  const before = mine.marks
  mine.marks = next
  try {
    await request()
  } catch (e) {
    mine.marks = before
    notify(e.message)
  }
}

// ref is the track or album id, or the artist key or collection path.
export function toggleLike(kind, ref) {
  const liked = !M.isLiked(mine.marks, kind, ref)
  const target = kind === 'artist' || kind === 'collection' ? { kind, key: ref } : { kind, id: ref }
  return change(M.withLike(mine.marks, kind, ref, liked), () => api.like(target, liked))
}

export function rate(kind, id, stars) {
  return change(M.withRating(mine.marks, kind, id, stars), () => api.rate({ kind, id }, stars))
}
