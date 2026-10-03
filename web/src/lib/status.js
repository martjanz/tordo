// Reading /api/status: {readOnly, scan, identify (from M2), counts}.

// busy reports whether a scan or an identification run is going on.
export function busy(st) {
  return !!st && (!!st.scan?.running || st.identify?.state === 'running')
}

// progress sums up what a run has changed so far: when it moves, the pages
// have new data to show.
export function progress(st) {
  if (!st) return ''
  return [st.scan?.read ?? 0, st.scan?.albums ?? 0, st.identify?.identified ?? 0, st.identify?.enriched ?? 0].join('|')
}

// summary is the short text of the status indicator, and its tone: "busy",
// "warn", "ok" (all quiet) or "down" (the last poll did not reach the server,
// which outweighs whatever status it had before).
export function summary(st, lost = false) {
  if (lost) return { text: 'Sin conexión con Tordo', tone: 'down' }
  if (!st) return { text: '', tone: '' }
  if (st.scan?.running) {
    const { files = 0, read = 0, toRead = 0 } = st.scan
    const detail = toRead > 0 ? `${read}/${toRead} leídos` : `${files} archivos`
    return { text: `Escaneando · ${detail}`, tone: 'busy' }
  }
  const id = st.identify
  if (id?.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      return { text: `Completando datos · ${id.enriched}/${id.toEnrich}`, tone: 'busy' }
    }
    return { text: `Identificando · ${id.identified}/${id.toIdentify}`, tone: 'busy' }
  }
  if (st.readOnly) return { text: 'Modo consulta', tone: 'warn' }
  if (st.scan?.lastError) return { text: 'El último escaneo falló', tone: 'warn' }
  if (id?.state === 'offline') return { text: 'Sin conexión con MusicBrainz', tone: 'warn' }
  return { text: 'Al día', tone: 'ok' }
}
