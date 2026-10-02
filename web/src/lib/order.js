// move returns a copy of list with list[from] moved to index to (clamped).
export function move(list, from, to) {
  const out = [...list]
  if (from < 0 || from >= out.length) return out
  const [item] = out.splice(from, 1)
  out.splice(Math.min(Math.max(to, 0), out.length), 0, item)
  return out
}
