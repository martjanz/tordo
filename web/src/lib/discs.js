// discHeaders says, for each track, the disc number whose header goes before
// it, or null for none. Headers only make sense in one album's track list
// (enabled), and only when it spans more than one disc; a list that mixes
// albums gets none, even if some track came from a disc 2. A missing disc
// number counts as disc 1.
export function discHeaders(tracks, enabled = false) {
  const disc = (t) => t.disc || 1
  if (!enabled || new Set(tracks.map(disc)).size < 2) return tracks.map(() => null)
  return tracks.map((t, i) => (i === 0 || disc(tracks[i - 1]) !== disc(t) ? disc(t) : null))
}
