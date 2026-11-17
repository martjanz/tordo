// How the duplicates of Revisar read.
import { formatName, size } from './format.js'
import { formatNumber, t } from './i18n/index.svelte.js'

// kindOf makes a kind whose label, help and empty text are read through t
// when used, so they follow the language.
const kindOf = (kind, count) => ({
  kind,
  count,
  get label() {
    return t(`dupes.${kind}.label`)
  },
  get help() {
    return t(`dupes.${kind}.help`)
  },
  get empty() {
    return t(`dupes.${kind}.empty`)
  },
})

export const KINDS = [kindOf('exact', 'exact'), kindOf('formats', 'formats'), kindOf('suspected', 'suspect')]

// fileQuality reads a file's format: "FLAC 24 bits / 96 kHz", "MP3 320 kbps".
// What the file does not say is left out.
export function fileQuality(f) {
  const name = formatName(f.format)
  if (f.lossless) {
    const khz = f.sampleRate ? `${formatNumber(f.sampleRate / 1000, { maximumFractionDigits: 1 })} kHz` : ''
    const parts = [f.bitDepth ? `${f.bitDepth} bits` : '', khz].filter(Boolean)
    return [name, parts.join(' / ')].filter(Boolean).join(' ')
  }
  return f.bitrate ? `${name} ${f.bitrate} kbps` : name
}

// groupLine sums a group up: "3 archivos · 21 MB", plus the space that
// deleting the copies would recover when they are exact.
export function groupLine(g) {
  const parts = [t('count.files', { n: g.files.length }), size(g.sizeBytes)]
  if (g.kind === 'exact') parts.push(t('dupes.recoverable', { size: size(g.recoverable) }))
  return parts.join(' · ')
}
