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

// Setting a title on a placeholder, like a typographic sleeve: heavy
// capitals bottom left, in Inter like the Sonando view. Sizes are shares of
// the picture's width (CSS cqi).
export const TITLE_MAX = 24
export const TITLE_MIN = 8
export const TITLE_WIDTH = 91 // the line: the width less 4.5% margins
export const TITLE_HEIGHT = 40 // the block stays in the bottom 40%
export const TITLE_LEADING = 0.84 // 200 on 237; accents may touch the line above
// Advance widths of Inter at weight 800, -0.025em tracking, in em: capitals,
// digits and common punctuation, measured in the browser. Accented letters
// take their base letter's width; anything else WIDE.
const WIDTHS = {
  A: 0.745, B: 0.64, C: 0.719, D: 0.698, E: 0.585, F: 0.56, G: 0.727, H: 0.724, I: 0.261, J: 0.565, K: 0.713, L: 0.54,
  M: 0.918, N: 0.741, O: 0.748, P: 0.627, Q: 0.757, R: 0.637, S: 0.635, T: 0.652, U: 0.702, V: 0.745, W: 1.034,
  X: 0.736, Y: 0.727, Z: 0.654, 0: 0.667, 1: 0.416, 2: 0.613, 3: 0.632, 4: 0.663, 5: 0.609, 6: 0.637, 7: 0.563,
  8: 0.639, 9: 0.637, ' ': 0.194, '.': 0.328, ',': 0.328, "'": 0.329, '’': 0.306, '!': 0.333, '?': 0.554, '(': 0.357,
  ')': 0.357, '&': 0.658, '-': 0.446,
}
const WIDE = 0.8
const SLACK = 1.05 // layout and font loading differ a little from the table

// textWidth: how wide text is set in capitals, in em.
export function textWidth(text) {
  let w = 0
  for (const ch of String(text ?? '').toUpperCase()) w += WIDTHS[ch] ?? WIDTHS[ch.normalize('NFD')[0]] ?? WIDE
  return w * SLACK
}

// titleLines wraps a title, upper-cased, greedily at size.
export function titleLines(name, size) {
  const words = String(name ?? '').trim().toUpperCase().split(/\s+/).filter(Boolean)
  const lines = []
  for (const w of words) {
    const last = lines[lines.length - 1]
    if (last != null && textWidth(`${last} ${w}`) * size <= TITLE_WIDTH) lines[lines.length - 1] = `${last} ${w}`
    else lines.push(w)
  }
  return lines
}

// titleSize: the largest size, in half steps, at which every word fits on
// a line and the wrapped title within TITLE_HEIGHT; TITLE_MIN at least.
export function titleSize(name) {
  const words = String(name ?? '').trim().split(/\s+/)
  const widest = Math.max(...words.map(textWidth))
  for (let size = TITLE_MAX; size > TITLE_MIN; size -= 0.5) {
    if (size * widest > TITLE_WIDTH) continue
    if (titleLines(name, size).length * size * TITLE_LEADING <= TITLE_HEIGHT) return size
  }
  return TITLE_MIN
}
