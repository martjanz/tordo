// State shared by the whole app: the server's status and the notices.
import { api } from './api.js'
import { busy, progress } from './status.js'

export const app = $state({
  status: null,
  lost: false, // the last poll did not reach the server
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

async function poll() {
  clearTimeout(timer)
  try {
    const st = await api.status()
    const now = busy(st)
    const moved = progress(st) !== lastProgress
    if ((wasBusy && !now) || (now && moved && Date.now() - lastRefresh > 20000)) {
      app.generation++
      lastRefresh = Date.now()
    }
    wasBusy = now
    lastProgress = progress(st)
    app.status = st
    app.lost = false
  } catch {
    app.lost = true
  }
  timer = setTimeout(poll, wasBusy || app.lost ? 3000 : 30000)
}

// watchStatus starts polling /api/status: every 3 s while something runs or
// the server is out of reach, every 30 s otherwise.
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
