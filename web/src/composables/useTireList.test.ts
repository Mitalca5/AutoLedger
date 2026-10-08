import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { api } from '@/services/api'
import { useTireList } from './useTireList'

vi.mock('@/services/api', () => ({ api: { getTires: vi.fn() } }))
const store = reactive<{ activeVehicle: { id: string } | null }>({ activeVehicle: { id: 'v1' } })
vi.mock('@/stores/vehicle', () => ({ useVehicleStore: () => store }))

const tire = (id: string, position: string) => ({ tire: { id, current_position: position } })

describe('useTireList', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    store.activeVehicle = { id: 'v1' }
  })

  it('loads and groups the tires by position', async () => {
    vi.mocked(api.getTires).mockResolvedValue([tire('a', 'FL'), tire('b', 'STORAGE'), tire('c', 'DISPOSED')] as any)
    const list = useTireList()
    expect(list.ready.value).toBe(false)
    await list.loadTires()
    expect(list.ready.value).toBe(true)
    expect(list.mountedTires.value.FL.tire.id).toBe('a')
    expect(list.mountedTires.value.FR).toBeNull()
    expect(list.hasMountedTires.value).toBe(true)
    expect(list.storageTires.value.map((t) => t.tire.id)).toEqual(['b'])
    expect(list.disposedTires.value.map((t) => t.tire.id)).toEqual(['c'])
    expect(list.loading.value).toBe(false)
  })

  it('treats a null answer as no tires', async () => {
    vi.mocked(api.getTires).mockResolvedValue(null as any)
    const list = useTireList()
    await list.loadTires()
    expect(list.tires.value).toEqual([])
    expect(list.hasMountedTires.value).toBe(false)
  })

  it('reports a load failure and keeps the tires already loaded', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.mocked(api.getTires).mockResolvedValueOnce([tire('a', 'FL')] as any).mockRejectedValueOnce(new Error('boom'))
    const list = useTireList()
    await list.loadTires()
    await list.loadTires()
    expect(list.loadFailed.value).toBe(true)
    expect(list.loadError.value).toBe('boom')
    expect(list.tires.value).toHaveLength(1)
  })

  it('ignores an answer that arrives after the vehicle changed', async () => {
    let resolve!: (v: any) => void
    vi.mocked(api.getTires).mockReturnValue(new Promise((r) => (resolve = r)) as any)
    const list = useTireList()
    const pending = list.loadTires()
    store.activeVehicle = { id: 'v2' }
    resolve([tire('a', 'FL')])
    await pending
    expect(list.tires.value).toEqual([])
    expect(list.ready.value).toBe(false)
  })

  it('does nothing without an active vehicle', async () => {
    store.activeVehicle = null
    const list = useTireList()
    await list.loadTires()
    expect(api.getTires).not.toHaveBeenCalled()
    expect(list.vehicleId.value).toBe('')
  })
})
