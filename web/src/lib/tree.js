// Pure logic of the folder tree: visible rows, revealing a path, keyboard
// actions, open branches in storage, filter rows and narrow-screen helpers.
// Levels are { [levelKey]: Node[] } ('' is the top); open is { [path]: true }.

export function visibleRows(levels, open, key = '') {
  const out = []
  const walk = (k, depth) => {
    for (const node of levels[k] ?? []) {
      out.push({ node, depth, parent: k })
      if (open[node.path] && levels[node.path]) walk(node.path, depth + 1)
    }
  }
  walk(key, 1)
  return out
}

export function missingLevels(levels, open, key = '') {
  const out = []
  const walk = (k) => {
    for (const node of levels[k] ?? []) {
      if (!open[node.path]) continue
      if (levels[node.path]) walk(node.path)
      else if (node.hasChildren) out.push(node.path)
    }
  }
  walk(key)
  return out
}

// The folder a row starts at: its path minus the pass-through segments.
function rowTop(node) {
  const extra = node.label.split(' › ').length - 1
  return extra > 0 ? node.path.split('/').slice(0, -extra).join('/') : node.path
}

export function locate(levels, target) {
  const chain = []
  let key = ''
  for (;;) {
    const list = levels[key]
    if (!list) return { chain, node: null, missing: key }
    if (target === '') return { chain, node: null, missing: null }
    let next = null
    for (const n of list) {
      const top = rowTop(n)
      const inChain =
        n.path === target ||
        (n.path.startsWith(target + '/') && (target === top || target.startsWith(top + '/')))
      if (inChain) return { chain, node: n.path, missing: null }
      if (next === null && target.startsWith(n.path + '/')) next = n.path
    }
    if (next === null) return { chain, node: null, missing: null }
    chain.push(next)
    key = next
  }
}

export async function reveal(levels, target, load) {
  for (let i = 0; i < 64; i++) {
    const r = locate(levels, target)
    if (r.missing === null) return { chain: r.chain, node: r.node }
    levels[r.missing] = await load(r.missing)
  }
  const r = locate(levels, target)
  return { chain: r.chain, node: r.node }
}

export function keyAction(rows, cursor, key, open) {
  if (!rows.length) return null
  const i = rows.findIndex((r) => r.node.path === cursor)
  if (i < 0) {
    return ['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(key)
      ? { type: 'cursor', path: rows[0].node.path }
      : null
  }
  const row = rows[i]
  const at = (j) => ({ type: 'cursor', path: rows[j].node.path })
  switch (key) {
    case 'ArrowDown': return at(Math.min(i + 1, rows.length - 1))
    case 'ArrowUp': return at(Math.max(i - 1, 0))
    case 'Home': return at(0)
    case 'End': return at(rows.length - 1)
    case 'ArrowRight': {
      if (!row.node.hasChildren) return null
      if (!open[row.node.path]) return { type: 'open', path: row.node.path }
      const next = rows[i + 1]
      return next && next.parent === row.node.path ? at(i + 1) : null
    }
    case 'ArrowLeft': {
      if (open[row.node.path]) return { type: 'close', path: row.node.path }
      if (row.parent !== '' && rows.some((r) => r.node.path === row.parent)) {
        return { type: 'cursor', path: row.parent }
      }
      return null
    }
    case 'Enter': return { type: 'show', path: row.node.path }
    default: return null
  }
}

export function isFilterKey(event) {
  if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey) return false
  const t = event.target
  if (!t) return true
  if (['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName) || t.isContentEditable) return false
  return true
}

export const OPEN_KEY = 'tordo.carpetas.abiertas'

export function loadOpen(storage) {
  try {
    const list = JSON.parse(storage.getItem(OPEN_KEY))
    if (!Array.isArray(list) || !list.every((x) => typeof x === 'string')) return {}
    return Object.fromEntries(list.map((p) => [p, true]))
  } catch {
    return {}
  }
}

export function saveOpen(storage, open) {
  try {
    storage.setItem(OPEN_KEY, JSON.stringify(Object.keys(open).filter((k) => open[k])))
  } catch {
    // storage unavailable or full: open branches just are not remembered
  }
}

export function filterOpen(levels) {
  return Object.fromEntries(Object.keys(levels).filter((k) => k !== '').map((k) => [k, true]))
}

export function matchRows(result) {
  const out = []
  for (const path of result.matches ?? []) {
    for (const k of Object.keys(result.levels ?? {})) {
      const node = result.levels[k].find((n) => n.path === path)
      if (node) {
        out.push({ node, parent: k })
        break
      }
    }
  }
  return out
}

export function direction(from, to) {
  if (from === to) return 'none'
  return from === '' || to.startsWith(from + '/') ? 'forward' : 'back'
}

export function shortPath(path, root) {
  let p = path
  if (root && p.startsWith(root + '/')) p = p.slice(root.length + 1)
  else while (p.startsWith('../')) p = p.slice(3)
  const parts = p.split('/')
  if (parts.length > 3) return [parts[0], '…', parts[parts.length - 2], parts[parts.length - 1]].join('/')
  return parts.join('/')
}
