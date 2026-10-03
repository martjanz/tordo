// Who is using Tordo: this computer (no PIN) or a device on the network
// that needs a session.
import { api, onUnauthorized } from './api.js'

export const auth = $state({ checked: false, local: true, session: false, pinRequired: false, device: '' })

// checkAuth reads the state from the server. Without a connection it tries
// again in 5 s.
export async function checkAuth() {
  try {
    const a = await api.auth()
    auth.local = a.local
    auth.session = a.session
    auth.device = a.device
    auth.pinRequired = a.pinRequired
    auth.checked = true
  } catch {
    setTimeout(checkAuth, 5000)
  }
}

onUnauthorized(() => {
  auth.pinRequired = true
})
