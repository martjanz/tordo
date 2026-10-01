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
