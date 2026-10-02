// The folder tree state both Carpetas layouts share. It lives at module level
// so it survives navigating between folders and leaving the page.
import { api } from './api.js'
import { loadOpen, reveal, saveOpen } from './tree.js'

function storage() {
  try {
    return localStorage
  } catch {
    return null
  }
}

export const tree = $state({ levels: {}, open: storage() ? loadOpen(storage()) : {}, generation: -1 })

// syncGeneration forgets the loaded levels when a scan ended (open branches stay).
export function syncGeneration(generation) {
  if (tree.generation === generation) return
  tree.generation = generation
  tree.levels = {}
}

export async function loadLevel(key) {
  tree.levels[key] = await api.folderTree(key)
}

// revealPath loads what is needed to show target, opens its branches and saves them.
export async function revealPath(target) {
  const r = await reveal(tree.levels, target, api.folderTree)
  for (const p of r.chain) tree.open[p] = true
  const s = storage()
  if (s) saveOpen(s, tree.open)
  return r
}

export function setOpen(path, on) {
  if (on) tree.open[path] = true
  else delete tree.open[path]
  const s = storage()
  if (s) saveOpen(s, tree.open)
}
