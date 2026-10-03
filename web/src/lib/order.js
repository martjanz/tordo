// move returns a copy of list with list[from] moved to index to (clamped).
export function move(list, from, to) {
  const out = [...list]
  if (from < 0 || from >= out.length) return out
  const [item] = out.splice(from, 1)
  out.splice(Math.min(Math.max(to, 0), out.length), 0, item)
  return out
}

// dropIndex is where a dragged row lands: how many of the other rows (their
// boxes when the drag began) lie above its center, moved by dy.
export function dropIndex(rects, from, dy) {
  const center = rects[from].top + rects[from].height / 2 + dy
  let to = 0
  rects.forEach((r, j) => {
    if (j !== from && r.top + r.height / 2 < center) to++
  })
  return to
}

// dragShift is how far row i moves while the drag ({from, to, h}) takes
// another row past it.
export function dragShift(drag, i) {
  if (!drag) return 0
  const { from, to, h } = drag
  if (from < to && i > from && i <= to) return -h
  if (from > to && i >= to && i < from) return h
  return 0
}
