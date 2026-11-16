// How the duplicates of Revisar read.
import { formatName, size } from './format.js'
import { formatNumber } from './i18n/index.svelte.js'

export const KINDS = [
  {
    kind: 'exact', label: 'Copias idénticas', count: 'exact',
    help: 'El mismo audio en varios archivos, aunque los tags difieran.',
    empty: 'No hay copias idénticas.',
  },
  {
    kind: 'formats', label: 'Mismo tema en varios formatos', count: 'formats',
    help: 'La misma grabación del mismo álbum en archivos distintos. MEJOR marca la de más calidad.',
    empty: 'No hay temas repetidos en varios formatos.',
  },
  {
    kind: 'suspected', label: 'Sospechas', count: 'suspect',
    help: 'Mismo artista, título y duración en álbumes distintos. Suele ser legítimo (un tema en su disco y en un compilado).',
    empty: 'No hay sospechas de duplicado.',
  },
]

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
  const parts = [`${g.files.length} archivos`, size(g.sizeBytes)]
  if (g.kind === 'exact') parts.push(`${size(g.recoverable)} recuperables`)
  return parts.join(' · ')
}
