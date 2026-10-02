// How numbers, durations and formats read in the app (Spanish).

// duration is a track's length: "3:07", "1:02:03".
export function duration(ms) {
  const s = Math.max(0, Math.round((ms ?? 0) / 1000))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = String(s % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

// length is a long total: "48 min", "1 h 12 min".
export function length(ms) {
  const min = Math.round((ms ?? 0) / 60000)
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  return min % 60 ? `${h} h ${min % 60} min` : `${h} h`
}

// count reads "1 pista", "12 pistas" (thousands with a dot: "1.234 pistas").
export function count(n, one, many) {
  return `${n.toLocaleString('es')} ${n === 1 ? one : many}`
}

const CODECS = {
  mp3: 'MP3',
  mp2: 'MP2',
  aac: 'AAC',
  alac: 'ALAC',
  flac: 'FLAC',
  opus: 'Opus',
  vorbis: 'Vorbis',
  wma: 'WMA',
  wmav1: 'WMA',
  wmapro: 'WMA Pro',
  wmalossless: 'WMA Lossless',
  ape: 'APE',
  wavpack: 'WavPack',
}

// codecName names a track's codec ("pcm" by its container: WAV, AIFF).
export function codecName(t) {
  if (t.codec?.startsWith('pcm')) return (t.format || 'PCM').toUpperCase()
  return CODECS[t.codec] ?? (t.codec || t.format || '').toUpperCase()
}

// quality is a track's format as it reads in lists: "FLAC 24/96",
// "MP3 320", "WAV 16/44,1".
export function quality(t) {
  const name = codecName(t)
  if (t.lossless) {
    if (!t.bitDepth || !t.sampleRate) return name
    const khz = (t.sampleRate / 1000).toLocaleString('es', { maximumFractionDigits: 1 })
    return `${name} ${t.bitDepth}/${khz}`
  }
  return t.bitrate ? `${name} ${t.bitrate}` : name
}

// folderName is the last part of a catalog path.
export function folderName(path) {
  return (path ?? '').split('/').filter((p) => p && p !== '..').pop() ?? ''
}

const regions = (() => {
  try {
    return new Intl.DisplayNames(['es'], { type: 'region' })
  } catch {
    return null
  }
})()

// country names an ISO 3166-1 code in Spanish: "AR" → "Argentina".
export function country(code) {
  if (!code) return ''
  try {
    return regions?.of(code) ?? code
  } catch {
    return code
  }
}

const FORMATS = { ...CODECS, wav: 'WAV', aiff: 'AIFF', ogg: 'Ogg', mp4: 'MP4' }

// formatName names a value of Explorar's Formato facet ("flac" → "FLAC").
export function formatName(key) {
  return FORMATS[key] ?? (key ?? '').toUpperCase()
}

// years reads a span of years: "1982–2007", or "1975" for one.
export function years(first, last) {
  if (!first) return ''
  return last && last !== first ? `${first}–${last}` : String(first)
}

// size reads a byte count as the Finder does (decimal units): "84 KB",
// "7,2 MB", "1,4 GB". Under 10 it keeps one decimal.
export function size(bytes) {
  if (bytes == null) return ''
  if (bytes < 1000) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  const round = (v) => (v < 10 ? Math.round(v * 10) / 10 : Math.round(v))
  let value = bytes / 1000
  let unit = 0
  // Promote with the same rounding as the output: 999,6 KB is "1,0 MB".
  while (round(value) >= 1000 && unit < units.length - 1) {
    value /= 1000
    unit++
  }
  const rounded = round(value)
  const decimals = rounded < 10 ? 1 : 0
  const text = rounded.toLocaleString('es', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
  return `${text} ${units[unit]}`
}
