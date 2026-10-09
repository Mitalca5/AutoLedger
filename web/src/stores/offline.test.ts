import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const mocks = vi.hoisted(() => ({ list: vi.fn(), remove: vi.fn(), enqueue: vi.fn() }))
vi.mock('@/services/offlineQueue', () => ({ listQueuedMutations: mocks.list, removeQueuedMutation: mocks.remove, enqueueMutation: mocks.enqueue }))
vi.mock('@/stores/vehicle', () => ({ useVehicleStore: () => ({ lastSyncTimestamp: 0 }) }))
import { useOfflineStore } from './offline'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  vi.stubGlobal('navigator', { onLine: true })
  mocks.list.mockResolvedValue([{ id: 'queued-1', method: 'POST', endpoint: '/vehicles/v1/fuel', body: '{}', label: 'Fill-up', createdAt: 1 }])
})
afterEach(() => vi.unstubAllGlobals())

it('renews the session and removes a queued record only after a successful retry', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response('{}', { status: 401 }))
    .mockResolvedValueOnce(new Response('{}', { status: 200 }))
    .mockResolvedValueOnce(new Response('{}', { status: 201 }))
  mocks.remove.mockImplementation(async () => mocks.list.mockResolvedValue([]))
  vi.stubGlobal('fetch', fetchMock)
  const store = useOfflineStore()
  await store.flush()
  expect(fetchMock.mock.calls.map(([url]) => url)).toEqual(['/api/vehicles/v1/fuel', '/api/auth/refresh', '/api/vehicles/v1/fuel'])
  expect(fetchMock.mock.calls[2]).toEqual(fetchMock.mock.calls[0])
  expect(mocks.remove).toHaveBeenCalledWith('queued-1')
  expect(store.pendingCount).toBe(0)
})

it('retains the queued record if the refresh session is no longer valid', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 401 })))
  const store = useOfflineStore()
  await store.flush()
  expect(mocks.remove).not.toHaveBeenCalled()
  expect(store.pendingCount).toBe(1)
  expect(store.failures).toEqual([])
})
