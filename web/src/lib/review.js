// How identifications read: statuses, candidates, confidence.
import { country } from './format.js'
import { t } from './i18n/index.svelte.js'

// percent reads a score: 0.873 → "87 %".
export function percent(score) {
  return `${Math.round((score ?? 0) * 100)} %`
}

// candidateLine sums up a MusicBrainz candidate: "Soda Stereo · 1986 ·
// Argentina · 9 pistas".
export function candidateLine(c) {
  return [c.artist, c.year || '', country(c.country), c.tracks ? t('count.tracks', { n: c.tracks }) : ''].filter(Boolean).join(' · ')
}

// identification tells where an album's data comes from.
export function identification(album) {
  switch (album.status) {
    case 'auto':
      return t('review.status.auto', { percent: percent(album.confidence) })
    case 'manual':
      return t('review.status.manual')
    case 'unmatched':
      return t('review.status.unmatched')
    case 'ignored':
      return t('review.status.ignored')
    case 'selection':
      return t('review.status.selection')
  }
  return t('review.status.none')
}

// genreLinks lists an album's genres for links to Explorar: each genre once,
// then each style with its genre.
export function genreLinks(genres) {
  const out = []
  const seen = new Set()
  for (const g of genres ?? []) {
    if (!seen.has(g.genre)) {
      seen.add(g.genre)
      out.push({ label: g.genre, facets: { genre: g.genre } })
    }
  }
  for (const g of genres ?? []) {
    if (g.style) out.push({ label: g.style, facets: { genre: g.genre, style: g.style } })
  }
  return out
}

// worthAccepting lists the albums of the queue whose best candidate scores
// at least min: likely right, missed for a reissue's year or another
// edition's durations.
export function worthAccepting(albums, min = 0.7) {
  return albums.filter((a) => (a.candidates?.[0]?.score ?? 0) >= min)
}

// searchURL looks an album up on musicbrainz.org, to paste the right release.
export function searchURL(album) {
  const q = [album.tagTitle || album.title, album.tagArtist || album.artist].filter(Boolean).join(' ')
  return `https://musicbrainz.org/search?type=release&query=${encodeURIComponent(q)}`
}

// genreOptions lists what a raw genre can map to: each genre, then its
// styles ("Latin › Samba"), and "not a genre" first. value encodes the
// choice: genreChoice reads it back.
export function genreOptions(taxonomy) {
  const out = [{ value: '|', label: t('review.notGenre') }]
  for (const g of taxonomy ?? []) {
    out.push({ value: `${g.name}|`, label: g.name })
    for (const s of g.styles) out.push({ value: `${g.name}|${s}`, label: `${g.name} › ${s}` })
  }
  return out
}

export function genreChoice(value) {
  const i = value.lastIndexOf('|')
  return { genre: value.slice(0, i), style: value.slice(i + 1) }
}

// aliasLabel reads a user's mapping: "Folk, World, & Country › Folk", or
// "ignorado".
export function aliasLabel(a) {
  if (!a.genre) return t('review.ignored')
  return a.style ? `${a.genre} › ${a.style}` : a.genre
}
