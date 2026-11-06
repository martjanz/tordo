import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('./api.js', () => ({ api: { status: vi.fn() } }))

import { api } from './api.js'
import { app, settingsSaved, stopStatus, watchStatus } from './app.svelte.js'

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

describe('stopStatus', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('makes no more requests, even for a poll in flight', async () => {
    api.status.mockReset()
    let slow
    api.status.mockImplementationOnce(() => new Promise((r) => (slow = r)))
    watchStatus()
    stopStatus()
    slow({ setupPending: false, scanning: true })
    await vi.advanceTimersByTimeAsync(120000)
    expect(api.status).toHaveBeenCalledTimes(1)
    expect(app.status?.scanning).not.toBe(true)
  })
})

describe('polling', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => {
    stopStatus()
    vi.useRealTimers()
  })

  it('asks every second while the catalog is being prepared, then relaxes', async () => {
    api.status.mockReset()
    api.status.mockResolvedValue({ scan: {}, preparing: true })
    watchStatus()
    await vi.advanceTimersByTimeAsync(0)
    expect(api.status).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.status).toHaveBeenCalledTimes(2)

    api.status.mockResolvedValue({ scan: {} })
    await vi.advanceTimersByTimeAsync(1000)
    expect(api.status).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(10000)
    expect(api.status).toHaveBeenCalledTimes(3)
  })
})
