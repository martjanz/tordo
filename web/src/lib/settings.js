// The settings as the pages edit them: a draft made from GET /api/config,
// and the body of PUT /api/config.

// draft is what the page edits. On first use the suggested folders come
// checked (the assistant proposes every sibling folder); later they are
// offered unchecked. token is null while the saved one is kept; the PIN is
// '' while the saved one is kept.
export function draft(config) {
  const roots = [
    ...config.roots.map((r) => ({ ...r, checked: true })),
    ...config.suggested.map((path) => ({ path, available: true, checked: config.setupPending })),
  ]
  return {
    roots,
    ignore: (config.ignore ?? []).join('\n'),
    token: null,
    normalize: config.normalize,
    lan: { enabled: config.lan.enabled, port: config.lan.port, pin: '' },
  }
}

// addRoot adds a checked folder, or checks it when it is already listed.
export function addRoot(d, root) {
  const known = d.roots.find((r) => r.path === root.path)
  const roots = known
    ? d.roots.map((r) => (r.path === root.path ? { ...r, checked: true } : r))
    : [...d.roots, { ...root, checked: true }]
  return { ...d, roots }
}

const lines = (text) =>
  text
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)

// body is the PUT /api/config request for a draft: null keeps the token
// (or the PIN), "" removes the token.
export function body(d) {
  return {
    roots: d.roots.filter((r) => r.checked).map((r) => r.path),
    ignore: lines(d.ignore),
    token: d.token === null ? null : d.token.trim(),
    normalize: d.normalize,
    lan: { enabled: d.lan.enabled, port: Number(d.lan.port), pin: d.lan.pin === '' ? null : d.lan.pin },
  }
}

const same = (a, b) => a.length === b.length && a.every((v, i) => v === b[i])

function scanChanged(config, b) {
  return !same(b.roots, config.roots.map((r) => r.path)) || !same(b.ignore, config.ignore ?? [])
}

// changed reports whether saving the draft would change the configuration.
export function changed(config, d) {
  const b = body(d)
  return (
    scanChanged(config, b) ||
    b.token !== null ||
    b.normalize !== config.normalize ||
    b.lan.enabled !== config.lan.enabled ||
    b.lan.port !== config.lan.port ||
    b.lan.pin !== null
  )
}

// savedNotice is what saving the draft makes the server do, said after
// saving: on first use, or when the folders or the ignored patterns change,
// it scans; a new token updates the genres; the rest takes effect quietly.
export function savedNotice(config, d) {
  const b = body(d)
  if (config.setupPending || scanChanged(config, b)) return 'Ajustes guardados. Se escanean las carpetas.'
  if (b.token !== null) return 'Ajustes guardados. Se actualizan los géneros con el token nuevo.'
  return 'Ajustes guardados.'
}

// pinProblem is what is wrong with the PIN typed ('' keeps the saved one),
// or '' when it is fine.
export function pinProblem(pin, hasPin, enabled) {
  if (enabled && !hasPin && pin === '') return 'Elegí un PIN para usar Tordo desde la red.'
  if (pin !== '' && !/^[0-9]{6,12}$/.test(pin)) return 'El PIN tiene que tener entre 6 y 12 dígitos, solo números.'
  return ''
}

// portProblem is what is wrong with the port typed, or ''.
export function portProblem(port) {
  const n = Number(port)
  if (String(port).trim() === '' || !Number.isInteger(n) || n < 1024 || n > 65535) return 'El puerto tiene que estar entre 1024 y 65535.'
  return ''
}
