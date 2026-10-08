import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const api = {
  getVehicles: vi.fn(),
  syncVehicle: vi.fn(),
  getSyncStatus: vi.fn(),
}
vi.mock('@/services/api', () => ({ api }))

const storage = new Map<string, string>()
const doc = {
  documentElement: {},
  visibilityState: 'visible',
  listeners: new Map<string, () => void>(),
  addEventListener(name: string, fn: () => void) { this.listeners.set(name, fn) },
  removeEventListener(name: string) { this.listeners.delete(name) },
}

async function freshStore() {
  vi.resetModules()
  setActivePinia(createPinia())
  const { useVehicleStore } = await import('./vehicle')
  return useVehicleStore()
}

const tesla = { id: 'v1', role: 'EDITOR', powertrain: 'ELECTRIC', currency: 'USD', teslamate_api_url: 'http://tm' }

beforeEach(() => {
  storage.clear()
  doc.visibilityState = 'visible'
  doc.listeners.clear()
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => storage.get(k) ?? null,
    setItem: (k: string, v: string) => void storage.set(k, v),
  })
  vi.stubGlobal('document', doc)
  Object.values(api).forEach((fn) => fn.mockReset())
  vi.spyOn(console, 'error').mockImplementation(() => {})
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('vehicle selection', () => {
  it('derives role, currency and fallbacks from the active vehicle', async () => {
    const store = await freshStore()
    expect(store.activeVehicle).toBeNull()
    expect(store.userRole).toBe('OWNER')
    expect(store.currency).toBe('EUR')

    api.getVehicles.mockResolvedValue([tesla, { id: 'v2' }])
    await store.fetchVehicles()
    expect(store.activeVehicle.id).toBe('v1')
    expect(storage.get('teslacost_active_vehicle')).toBe('v1')
    expect(store.userRole).toBe('EDITOR')
    expect(store.isEditor && store.canEdit && !store.isOwner && !store.isViewer).toBe(true)
    expect(store.currency).toBe('USD')
    expect(store.isInitialized).toBe(true)
    expect(store.isLoading).toBe(false)

    store.setActiveVehicle('v2')
    expect(store.activeVehicle.id).toBe('v2')
    expect(store.userRole).toBe('OWNER')
  })

  it('keeps a stored vehicle that still exists and replaces one that vanished', async () => {
    storage.set('teslacost_active_vehicle', 'v2')
    let store = await freshStore()
    api.getVehicles.mockResolvedValue([tesla, { id: 'v2' }])
    await store.fetchVehicles()
    expect(store.activeVehicleId).toBe('v2')

    storage.set('teslacost_active_vehicle', 'gone')
    store = await freshStore()
    await store.fetchVehicles()
    expect(store.activeVehicleId).toBe('v1')
  })

  it('still ends the loading state when the list cannot be fetched', async () => {
    const store = await freshStore()
    api.getVehicles.mockRejectedValue(new Error('down'))
    await store.fetchVehicles()
    expect(store.vehicles).toEqual([])
    expect(store.isLoading).toBe(false)
    expect(store.isInitialized).toBe(true)
  })
})

describe('synchronization', () => {
  async function storeWithVehicle() {
    const store = await freshStore()
    api.getVehicles.mockResolvedValue([tesla])
    await store.fetchVehicles()
    return store
  }

  it('does nothing without a vehicle', async () => {
    const store = await freshStore()
    await store.syncActiveVehicle()
    await store.resumeRunningSync()
    expect(api.syncVehicle).not.toHaveBeenCalled()
    expect(api.getSyncStatus).not.toHaveBeenCalled()
  })

  it('follows a running job until it succeeds, then reloads the vehicles', async () => {
    vi.useFakeTimers()
    const store = await storeWithVehicle()
    api.syncVehicle.mockResolvedValue({ status: 'RUNNING' })
    api.getSyncStatus.mockResolvedValueOnce({ status: 'RUNNING' }).mockResolvedValueOnce({ status: 'SUCCEEDED', result: { drives: 3 } })
    const done = store.syncActiveVehicle()
    await vi.advanceTimersByTimeAsync(5000)
    await done
    expect(store.syncResult).toEqual({ drives: 3 })
    expect(store.syncError).toBeNull()
    expect(store.isSyncing).toBe(false)
    expect(api.getVehicles).toHaveBeenCalledTimes(2)

    store.clearSyncStatus()
    expect(store.syncResult).toBeNull()
  })

  it('reports a failed job with its message', async () => {
    const store = await storeWithVehicle()
    api.syncVehicle.mockResolvedValue({ status: 'FAILED', error: 'teslamate unreachable' })
    await store.syncActiveVehicle()
    expect(store.syncError).toContain('teslamate unreachable')
    expect(store.syncResult).toBeNull()
    expect(store.isSyncing).toBe(false)
  })

  it('reports a request failure and gives up on a job that takes too long', async () => {
    vi.useFakeTimers()
    const store = await storeWithVehicle()
    api.syncVehicle.mockRejectedValueOnce(new Error('network'))
    await store.syncActiveVehicle()
    expect(store.syncError).toBe('network')

    api.syncVehicle.mockResolvedValue({ status: 'RUNNING' })
    api.getSyncStatus.mockResolvedValue({ status: 'RUNNING' })
    const done = store.syncActiveVehicle()
    await vi.advanceTimersByTimeAsync(21 * 60 * 1000)
    await done
    expect(store.syncError).toBeTruthy()
    expect(store.isSyncing).toBe(false)
  })

  it('ignores a second sync while one is in progress', async () => {
    vi.useFakeTimers()
    const store = await storeWithVehicle()
    api.syncVehicle.mockResolvedValue({ status: 'RUNNING' })
    api.getSyncStatus.mockResolvedValue({ status: 'SUCCEEDED', result: {} })
    const first = store.syncActiveVehicle()
    await vi.advanceTimersByTimeAsync(0)
    await store.syncActiveVehicle()
    await vi.advanceTimersByTimeAsync(2000)
    await first
    expect(api.syncVehicle).toHaveBeenCalledTimes(1)
  })

  it('resumes a job already running on the server, and nothing otherwise', async () => {
    vi.useFakeTimers()
    const store = await storeWithVehicle()
    api.getSyncStatus.mockResolvedValueOnce({ status: 'SUCCEEDED' })
    await store.resumeRunningSync()
    expect(store.isSyncing).toBe(false)

    api.getSyncStatus.mockRejectedValueOnce(new Error('offline'))
    await store.resumeRunningSync()

    api.getSyncStatus.mockResolvedValueOnce({ status: 'RUNNING' }).mockResolvedValue({ status: 'SUCCEEDED', result: { ok: 1 } })
    const done = store.resumeRunningSync()
    await vi.advanceTimersByTimeAsync(2000)
    await done
    expect(store.syncResult).toEqual({ ok: 1 })
  })
})

describe('automatic refresh', () => {
  it('reloads the vehicles when a background synchronization finished since the last look', async () => {
    vi.useFakeTimers()
    const store = await freshStore()
    api.getVehicles.mockResolvedValue([tesla])
    await store.fetchVehicles()
    api.getVehicles.mockClear()

    api.getSyncStatus
      .mockResolvedValueOnce({ status: 'SUCCEEDED', finished_at: 'a' })
      .mockResolvedValueOnce({ status: 'SUCCEEDED', finished_at: 'a' })
      .mockResolvedValueOnce({ status: 'SUCCEEDED', finished_at: 'b' })
    store.startAutoRefresh()
    store.startAutoRefresh() // idempotent
    await vi.advanceTimersByTimeAsync(60_000)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getVehicles).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getVehicles).toHaveBeenCalledTimes(1)

    store.stopAutoRefresh()
    await vi.advanceTimersByTimeAsync(120_000)
    expect(api.getSyncStatus).toHaveBeenCalledTimes(3)
  })

  it('does not poll while the tab is hidden, and follows a job found running', async () => {
    vi.useFakeTimers()
    const store = await freshStore()
    api.getVehicles.mockResolvedValue([tesla])
    await store.fetchVehicles()
    store.startAutoRefresh()
    doc.visibilityState = 'hidden'
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getSyncStatus).not.toHaveBeenCalled()

    doc.visibilityState = 'visible'
    api.getSyncStatus.mockResolvedValueOnce({ status: 'RUNNING' }).mockResolvedValue({ status: 'SUCCEEDED', result: {} })
    await vi.advanceTimersByTimeAsync(60_000)
    await vi.advanceTimersByTimeAsync(2000)
    expect(store.syncResult).toEqual({})
    store.stopAutoRefresh()
  })

  it('skips the check for a vehicle without TeslaMate and survives a failing status call', async () => {
    vi.useFakeTimers()
    const store = await freshStore()
    api.getVehicles.mockResolvedValue([{ id: 'v9', powertrain: 'ICE' }])
    await store.fetchVehicles()
    store.startAutoRefresh()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getSyncStatus).not.toHaveBeenCalled()
    store.stopAutoRefresh()

    api.getVehicles.mockResolvedValue([tesla])
    await store.fetchVehicles()
    api.getSyncStatus.mockRejectedValue(new Error('busy'))
    store.startAutoRefresh()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api.getSyncStatus).toHaveBeenCalledTimes(1)
    store.stopAutoRefresh()
  })

  it('reloads on return after a long absence and only checks after a short one', async () => {
    vi.useFakeTimers()
    const store = await freshStore()
    api.getVehicles.mockResolvedValue([tesla])
    await store.fetchVehicles()
    api.getVehicles.mockClear()
    api.getSyncStatus.mockResolvedValue({ status: 'SUCCEEDED', finished_at: 'a' })
    store.startAutoRefresh()
    const onChange = doc.listeners.get('visibilitychange')!

    doc.visibilityState = 'hidden'
    onChange()
    vi.setSystemTime(Date.now() + 10 * 60 * 1000)
    doc.visibilityState = 'visible'
    onChange()
    await vi.advanceTimersByTimeAsync(0)
    expect(api.getVehicles).toHaveBeenCalledTimes(1)

    doc.visibilityState = 'hidden'
    onChange()
    doc.visibilityState = 'visible'
    onChange()
    await vi.advanceTimersByTimeAsync(0)
    expect(api.getVehicles).toHaveBeenCalledTimes(1)
    expect(api.getSyncStatus).toHaveBeenCalledTimes(1)

    store.stopAutoRefresh()
    expect(doc.listeners.has('visibilitychange')).toBe(false)
  })
})
