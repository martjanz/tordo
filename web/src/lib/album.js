// How an album's page header reads.
import { count, country, length, quality } from './format.js'
import { genreLinks } from './review.js'
import { exploreHref } from './router.js'

// albumYear is the original year, with the edition's when it differs:
// "1986 (edición 2007)".
export function albumYear(album) {
  const y = album.originalYear || album.year
  if (album.originalYear && album.year && album.year !== album.originalYear) return `${y} (edición ${album.year})`
  return y ? String(y) : ''
}

// albumMeta is the header's one meta line: year, tracks, length, country,
// genres, formats and the tracks the browser cannot play, as groups of
// {label, href?} items. The page joins groups with " · " and the items of
// a group (the genres) with ", "; an item with an href links Explorar.
export function albumMeta(album) {
  const tracks = album.tracks ?? []
  const groups = [albumYear(album), count(tracks.length, 'pista', 'pistas'), length(album.durationMs)]
    .filter(Boolean)
    .map((label) => [{ label }])
  if (album.country) groups.push([{ label: country(album.country), href: exploreHref({ pais: album.country }) }])
  const genres = genreLinks(album.genres).map((g) => ({ label: g.label, href: exploreHref(g.facets) }))
  if (genres.length) groups.push(genres)
  for (const f of new Set(tracks.map(quality))) if (f) groups.push([{ label: f }])
  const unplayable = tracks.filter((t) => !t.playable).length
  if (unplayable) groups.push([{ label: `${count(unplayable, 'pista necesita', 'pistas necesitan')} ffmpeg` }])
  return groups
}
