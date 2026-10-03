// State shared by the whole app: the server's status and the notices.
import { api } from './api.js'
import { busy, progress } from './status.js'

export const app = $state({
  status: null,
  // generation grows each time a scan or an identification run ends, and
  // every 20 s while a long one makes progress: pages read it to load their
  // data again.
  generation: 0,
  notices: [],
})

let timer
let wasBusy = false
let lastProgress = ''
let lastRefresh = Date.now() // pages load their data when they open
let polls = 0

// settingsSaved updates what the pages know right after saving the settings
// (the first-use assistant is done) and asks for the new status, so the scan
// and the normalization show at once.
export function settingsSaved() {
  if (app.status) app.status = { ...app.status, setupPending: false }
  app.generation++
  poll()
}

// Only the latest poll applies its result: an earlier, slower one could
// otherwise land after settingsSaved() with the old setupPending:true and
// bounce the assistant back.
async function poll() {
  const id = ++polls
  clearTimeout(timer)
  try {
    const st = await api.status()
    if (id !== polls) return
    const now = busy(st)
    const moved = progress(st) !== lastProgress
    if ((wasBusy && !now) || (now && moved && Date.now() - lastRefresh > 20000)) {
      app.generation++
      lastRefresh = Date.now()
    }
    wasBusy = now
    lastProgress = progress(st)
    app.status = st
  } catch {
    // The next poll tries again.
  }
  timer = setTimeout(poll, wasBusy ? 3000 : 30000)
}

// watchStatus starts polling /api/status: every 3 s while something runs,
// every 30 s otherwise.
export function watchStatus() {
  poll()
}

let seq = 0

// notify shows a notice for 6 seconds.
export function notify(text) {
  const id = ++seq
  app.notices.push({ id, text })
  setTimeout(() => {
    app.notices = app.notices.filter((n) => n.id !== id)
  }, 6000)
}
