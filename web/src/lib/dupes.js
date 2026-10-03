// How the duplicates of Revisar read.
import { formatName, size } from './format.js'

export const KINDS = [
  { kind: 'exactas', label: 'Copias idénticas', count: 'exact' },
  { kind: 'formatos', label: 'Mismo tema en varios formatos', count: 'formats' },
  { kind: 'sospechas', label: 'Sospechas', count: 'suspect' },
]

// fileQuality reads a file's format: "FLAC 24 bits / 96 kHz", "MP3 320 kbps".
// What the file does not say is left out.
export function fileQuality(f) {
  const name = formatName(f.format)
  if (f.lossless) {
    const khz = f.sampleRate ? `${(f.sampleRate / 1000).toLocaleString('es', { maximumFractionDigits: 1 })} kHz` : ''
    const parts = [f.bitDepth ? `${f.bitDepth} bits` : '', khz].filter(Boolean)
    return [name, parts.join(' / ')].filter(Boolean).join(' ')
  }
  return f.bitrate ? `${name} ${f.bitrate} kbps` : name
}

// groupLine sums a group up: "3 archivos · 21 MB", plus the space that
// deleting the copies would recover when they are exact.
export function groupLine(g) {
  const parts = [`${g.files.length} archivos`, size(g.sizeBytes)]
  if (g.kind === 'exacta') parts.push(`${size(g.recoverable)} recuperables`)
  return parts.join(' · ')
}
