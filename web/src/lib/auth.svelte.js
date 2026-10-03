// Who is using Tordo: this computer (no PIN) or a device on the network
// that needs a session.
import { api, onUnauthorized } from './api.js'

export const auth = $state({ checked: false, local: true, session: false, pinRequired: false, device: '' })

let retry

// checkAuth reads the state from the server. Without a connection it tries
// again in 5 s (one pending retry at a time).
export async function checkAuth() {
  clearTimeout(retry)
  try {
    const a = await api.auth()
    auth.local = a.local
    auth.session = a.session
    auth.device = a.device
    auth.pinRequired = a.pinRequired
    auth.checked = true
  } catch {
    retry = setTimeout(checkAuth, 5000)
  }
}

onUnauthorized(() => {
  auth.pinRequired = true
})
