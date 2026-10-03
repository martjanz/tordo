import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./api.js', () => ({ api: { status: vi.fn() } }))

import { api } from './api.js'
import { app, settingsSaved, watchStatus } from './app.svelte.js'

describe('settingsSaved', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('clears setupPending at once and ignores an older, slower poll', async () => {
    let slow
    api.status.mockImplementationOnce(() => new Promise((r) => (slow = r)))
    watchStatus()
    app.status = { setupPending: true }

    api.status.mockResolvedValueOnce({ setupPending: false, scanning: false })
    settingsSaved()
    expect(app.status.setupPending).toBe(false)
    await vi.advanceTimersByTimeAsync(0)
    expect(app.status.scanning).toBe(false)

    slow({ setupPending: true, scanning: true })
    await vi.advanceTimersByTimeAsync(0)
    expect(app.status).toEqual({ setupPending: false, scanning: false })
  })
})
