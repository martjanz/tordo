// Reading /api/status: {readOnly, scan, identify (from M2), counts}.
import { formatNumber, t } from './i18n/index.svelte.js'

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
  if (lost) return { text: t('status.summary.lost'), tone: 'down' }
  if (!st) return { text: '', tone: '' }
  if (st.preparing && !st.scan?.running && st.identify?.state !== 'running') return { text: t('status.summary.preparing'), tone: 'busy' }
  if (st.scan?.running) {
    const { files = 0, read = 0, toRead = 0 } = st.scan
    const text = {
      walk: t('status.summary.walk', { files: num(files) }),
      read: t('status.summary.read', { read: num(read), toRead: num(toRead), percent: percent(read, toRead) }),
      group: t('status.summary.group'),
    }[scanPhase(st.scan)]
    return { text, tone: 'busy' }
  }
  const id = st.identify
  if (id?.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      return { text: t('status.summary.enriching', { done: id.enriched, total: id.toEnrich }), tone: 'busy' }
    }
    return { text: t('status.summary.identifying', { done: id.identified, total: id.toIdentify }), tone: 'busy' }
  }
  if (st.readOnly) return { text: t('status.summary.readOnly'), tone: 'warn' }
  if (st.scan?.lastError) return { text: t('status.summary.scanFailed'), tone: 'warn' }
  if (st.scan?.skipped > 0) {
    return { text: t('status.summary.skipped', { n: st.scan.skipped }), tone: 'warn' }
  }
  if (id?.state === 'offline') return { text: t('status.summary.offline'), tone: 'warn' }
  return { text: t('status.summary.upToDate'), tone: 'ok' }
}

// STEPS are the steps from the music folders to a complete catalog, as the
// status panel lists them.
const STEPS = ['walk', 'read', 'group', 'identify', 'enrich']

// panel is what the status panel shows: a title and tone (as summary's), the
// steps with the current one, a progress bar, the catalog's figures and some
// notes. progress.percent is null for a step with no known end.
export function panel(st, lost = false) {
  const { text, tone } = summary(st, lost)
  const out = { title: text, tone, steps: null, progress: null, facts: [], notes: [] }
  if (lost) return { ...out, title: t('status.summary.lost'), notes: [t('status.panel.lost')] }
  if (!st) return out
  const steps = (current, state = 'busy') =>
    STEPS.map((step, i) => ({ label: t('status.step.' + step), state: current == null || i < current ? 'done' : i === current ? state : 'todo' }))
  const bar = (label, done, total, eta = '') => ({ label, value: t('status.panel.of', { done: num(done), total: num(total) }), percent: percent(done, total), eta })
  const scan = st.scan ?? {}
  const id = st.identify ?? {}

  if (st.preparing && (scan.running || id.state === 'running')) {
    out.notes.push(t('status.panel.preparingToo'))
  }
  if (st.preparing && !scan.running && id.state !== 'running') {
    out.progress = { label: t('status.panel.loadingAlbums'), value: '', percent: null, eta: '' }
    out.notes.push(t('status.panel.preparingOnce'))
  } else if (scan.running) {
    out.title = t('status.panel.scanning')
    switch (scanPhase(scan)) {
      case 'walk':
        out.steps = steps(0)
        out.progress = { label: t('status.panel.filesFound'), value: num(scan.files), percent: null, eta: '' }
        out.notes.push(t('status.panel.afterWalk'))
        break
      case 'group':
        out.steps = steps(2)
        out.progress = { label: t('status.panel.grouping'), value: '', percent: null, eta: '' }
        break
      default:
        out.steps = steps(1)
        out.progress = bar(t('status.panel.filesRead'), scan.read, scan.toRead, scan.etaSeconds > 0 ? timeLeft(scan.etaSeconds) : '')
        out.notes.push(t('status.panel.whileReading'))
    }
  } else if (id.state === 'running') {
    if (id.toEnrich > 0 && id.identified >= id.toIdentify) {
      out.title = t('status.panel.enriching')
      out.steps = steps(4)
      out.progress = bar(t('status.panel.albumsCompleted'), id.enriched, id.toEnrich)
      out.notes.push(t('status.panel.enrichNote'))
    } else {
      out.title = t('status.panel.identifying')
      out.steps = steps(3)
      out.progress = bar(t('status.panel.albumsSearched'), id.identified, id.toIdentify)
      out.notes.push(t('status.panel.identifyNote'))
    }
  } else if (st.readOnly) {
    out.notes.push(t('status.panel.readOnly'))
  } else if (scan.lastError) {
    out.error = scan.lastError
  } else if (id.state === 'offline') {
    out.steps = steps(3, 'warn')
    out.notes.push(t('status.panel.offline'))
  } else {
    out.steps = steps(null)
  }

  if (!scan.running && scan.skipped > 0) {
    const paths = scan.skippedPaths ?? []
    out.notes.push(t('status.panel.unreadable'), ...paths)
    if (scan.skipped > paths.length) out.notes.push(t('status.panel.moreSkipped', { n: scan.skipped - paths.length }))
  }

  const c = st.counts ?? {}
  if (c.albums > 0 || c.tracks > 0) out.facts.push({ label: t('status.fact.albums'), value: num(c.albums) }, { label: t('status.fact.tracks'), value: num(c.tracks) })
  if (c.identified > 0) out.facts.push({ label: t('status.fact.identified'), value: num(c.identified) })
  if (c.unidentified > 0) {
    const fact = { label: t('status.fact.unidentified'), value: num(c.unidentified) }
    out.facts.push(pageBlocked('revisar', st) ? fact : { ...fact, href: '/review' })
  }
  if (c.unreadable > 0) out.facts.push({ label: t('status.fact.unreadable'), value: num(c.unreadable) })

  if (st.loudness?.state === 'running' && st.loudness.pending > 0) {
    out.notes.push(t('status.panel.loudness', { n: st.loudness.pending }))
  }
  return out
}

// timeLeft words an estimate in seconds: minutes rounded up, then hours.
function timeLeft(secs) {
  if (secs < 60) return t('status.left.lessThanMinute')
  if (secs < 90 * 60) return t('status.left.minutes', { n: Math.ceil(secs / 60) })
  return t('status.left.hours', { n: Math.round(secs / 3600) })
}

// scanPhase is what a running scan is doing: "walk" (finding the files),
// "read" (reading them) or "group" (building the albums once all are read,
// also when there was nothing to read).
function scanPhase(scan) {
  const { read = 0, toRead = 0, grouping = false } = scan ?? {}
  if (grouping) return 'group'
  return toRead === 0 ? 'walk' : read >= toRead ? 'group' : 'read'
}

const num = (v) => formatNumber(v ?? 0)

// percent rounds down: 100 % only once everything is done.
const percent = (done = 0, total = 0) => (total > 0 ? Math.floor((done * 100) / total) : 0)

// scanMessage explains a running scan to a page that has nothing to show
// yet: what it is doing, and when the music will show up (the scan regroups
// the catalog every 30 s while it reads, the pages reload every 20 s).
export function scanMessage(scan) {
  const { files = 0, read = 0, toRead = 0 } = scan ?? {}
  switch (scanPhase(scan)) {
    case 'walk':
      return {
        title: t('status.scan.walkTitle', { files: num(files) }),
        detail: t('status.scan.walkDetail'),
      }
    case 'group':
      return { title: t('status.scan.groupTitle'), detail: t('status.scan.groupDetail') }
    default:
      return {
        title: t('status.scan.readTitle', { read: num(read), toRead: num(toRead), percent: percent(read, toRead) }),
        detail: t('status.scan.readDetail'),
      }
  }
}

// loadingMessage is what a page says while its data is on its way: what the
// server is doing, so a long wait does not look stuck.
export function loadingMessage(st, lost = false) {
  if (lost) return { title: t('status.summary.lost'), detail: t('status.loading.retry') }
  const running = !!st?.scan?.running || st?.identify?.state === 'running'
  const detail = running ? summary(st).text : ''
  if (st?.preparing) {
    return { title: t('status.loading.preparing'), detail: detail || t('status.panel.preparingOnce') }
  }
  return { title: t('status.loading.loading'), detail }
}

// listening reports whether listen mode is on: the server blocks curation
// and the UI hides it.
export function listening(st) {
  return !!st?.listenMode
}

// pageBlocked tells whether a page is out of reach in the current status
// (Revisar is curation, so listen mode closes it).
export function pageBlocked(page, st) {
  return page === 'revisar' && listening(st)
}
