// When a play counts (spec §7.3): 4 minutes, or half the track, of actual
// listening. Only forward progress in small steps counts, so seeking ahead
// or a stall does not. Pure: the player keeps the state.

export const MIN_MS = 4 * 60 * 1000
const MAX_STEP = 2 // seconds between two time updates that still count as listening

export function counts(listenedMs, durationMs) {
  return listenedMs >= MIN_MS || (durationMs > 0 && listenedMs >= durationMs / 2)
}

// begin starts a pass over entry: a new play to count.
export function begin(entry) {
  return { id: entry?.id ?? null, durationMs: entry?.durationMs ?? 0, listenedMs: 0, last: null, sent: false }
}

// tick takes the element's current time, in seconds.
export function tick(t, seconds) {
  if (t.id == null) return t
  const step = t.last == null ? 0 : seconds - t.last
  const listenedMs = step > 0 && step <= MAX_STEP ? t.listenedMs + step * 1000 : t.listenedMs
  return { ...t, listenedMs, last: seconds }
}

// due tells when the pass should be recorded: once.
export function due(t) {
  return t.id != null && !t.sent && counts(t.listenedMs, t.durationMs)
}

export function sent(t) {
  return { ...t, sent: true }
}
