// Placeholders for pictures that are missing or still loading: a muted
// gradient picked from the name, the same on every device and with no
// network, and the name's initials to set on it.

// hashName is a 32-bit FNV-1a hash of the name, case and surrounding
// spaces ignored.
export function hashName(name) {
  const s = String(name ?? '').trim().toLowerCase()
  let h = 0x811c9dc5
  for (const ch of s) {
    h ^= ch.codePointAt(0)
    h = Math.imul(h, 0x01000193)
  }
  return h >>> 0
}

// placeholderBackground is a CSS gradient for the name: two dark, low-
// saturation colours a little apart in hue, at one of a few angles.
export function placeholderBackground(name) {
  const h = hashName(name)
  const hue = h % 360
  const hue2 = (hue + 25 + ((h >>> 9) % 30)) % 360
  const sat = 22 + ((h >>> 15) % 14) // 22–35%
  const angle = [135, 160, 200, 225][(h >>> 20) % 4]
  return `linear-gradient(${angle}deg, hsl(${hue} ${sat}% 32%), hsl(${hue2} ${sat + 4}% 18%))`
}

// initials: the first letter (or digit) of the first and last words, upper-
// cased. "" when the name has none.
export function initials(name) {
  const words = String(name ?? '').match(/[\p{L}\p{N}][\p{L}\p{N}'’.-]*/gu) ?? []
  if (!words.length) return ''
  const first = words[0][0]
  const last = words.length > 1 ? words[words.length - 1][0] : ''
  return (first + last).toUpperCase()
}

// titleSize: how big to set a title on a placeholder. "short" goes in big
// type, one word per line; "medium" a step down; "long" smaller, run on.
export function titleSize(name) {
  const s = String(name ?? '').trim()
  const words = s ? s.split(/\s+/) : []
  const longest = Math.max(0, ...words.map((w) => w.length))
  if (s.length > 28 || words.length > 4) return 'long'
  if (longest > 7 || words.length > 3) return 'medium'
  return 'short'
}
