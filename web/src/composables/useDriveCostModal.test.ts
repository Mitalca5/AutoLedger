import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { api } from '@/services/api'
import { useDriveCostModal } from './useDriveCostModal'

vi.mock('@/services/api', () => ({ api: { getDrives: vi.fn() } }))
vi.mock('@/stores/vehicle', () => ({ useVehicleStore: () => ({ activeVehicle: { id: 'v1' }, currency: 'EUR' }) }))
const alert = vi.fn()
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ showConfirm: vi.fn(), showAlert: alert }) }))

const legs = [
  { id: 'b', start_time: '2026-01-02T08:00:00Z', end_time: '2026-01-02T09:00:00Z', distance: 10 },
  { id: 'a', start_time: '2026-01-01T08:00:00Z', end_time: '2026-01-01T09:00:00Z', distance: 5 },
]

function setup() {
  const deps = {
    loadDrives: vi.fn(),
    tripGroups: ref<any[]>([]),
    tripSuggestions: ref<any[]>([]),
    loadTripGroups: vi.fn(),
    suggestionName: vi.fn(() => 'Paris → Lyon'),
    createTripFromSuggestion: vi.fn(),
    dismissTripSuggestion: vi.fn(),
  }
  return { deps, modal: useDriveCostModal(deps) }
}

describe('useDriveCostModal', () => {
  beforeEach(() => vi.clearAllMocks())

  it('opens a plain drive with the toll entry flag and no trip legs', () => {
    const { modal } = setup()
    const drive = { id: 'd1' }
    modal.openCostModal(drive, true)
    expect(modal.showCostModal.value).toBe(true)
    expect(modal.selectedCostDrive.value).toEqual(drive)
    expect(modal.costStartWithToll.value).toBe(true)
    expect(modal.tripLegs.value).toEqual([])
  })

  it('opens a trip with its legs sorted chronologically', async () => {
    vi.mocked(api.getDrives).mockResolvedValue({ drives: legs } as any)
    const { modal } = setup()
    await modal.openTripCostModal({ id: 't1', name: 'Trip' })
    expect(api.getDrives).toHaveBeenCalledWith('v1', { tripGroupId: 't1', limit: 200 })
    expect(modal.tripLegs.value.map((d) => d.id)).toEqual(['a', 'b'])
    expect(modal.costTripDriveIds.value).toEqual(['a', 'b'])
    expect(modal.showCostModal.value).toBe(true)
  })

  it('reports a failure to load the trip legs and stays closed', async () => {
    vi.mocked(api.getDrives).mockRejectedValue(new Error('down'))
    const { modal } = setup()
    await modal.openTripCostModal({ id: 't1' })
    expect(alert).toHaveBeenCalledTimes(1)
    expect(modal.showCostModal.value).toBe(false)
  })

  it('opens a suggestion with only the legs it lists', async () => {
    vi.mocked(api.getDrives).mockResolvedValue({ drives: legs } as any)
    const { modal } = setup()
    await modal.openSuggestionCostModal({ drive_ids: ['a'], start_time: legs[1].start_time, end_time: legs[0].end_time })
    expect(modal.tripLegs.value.map((d) => d.id)).toEqual(['a'])
    expect(modal.showCostModal.value).toBe(true)
  })

  it('refreshes a plain drive by id and reloads the list', async () => {
    vi.mocked(api.getDrives).mockResolvedValue({ drives: [{ id: 'd1' }] } as any)
    const { deps, modal } = setup()
    modal.openCostModal({ id: 'd1' })
    expect(await modal.refreshCostDrive('d1')).toEqual({ id: 'd1' })
    expect(api.getDrives).toHaveBeenCalledWith('v1', { driveId: 'd1', limit: 1 })
    expect(deps.loadDrives).toHaveBeenCalled()
  })

  it('refreshes a trip leg from the reloaded legs', async () => {
    vi.mocked(api.getDrives).mockResolvedValue({ drives: legs } as any)
    const { deps, modal } = setup()
    await modal.openTripCostModal({ id: 't1' })
    expect((await modal.refreshCostDrive('b'))?.id).toBe('b')
    expect(deps.loadTripGroups).toHaveBeenCalled()
  })

  it('closes the modal before creating or dismissing the suggestion it shows', async () => {
    vi.mocked(api.getDrives).mockResolvedValue({ drives: legs } as any)
    const { deps, modal } = setup()
    const s = { drive_ids: ['a', 'b'], start_time: legs[1].start_time, end_time: legs[0].end_time }
    deps.tripSuggestions.value = [s]
    await modal.openSuggestionCostModal(s)
    const virtual = modal.selectedCostDrive.value
    await modal.createTripFromCostModal(virtual)
    expect(modal.showCostModal.value).toBe(false)
    expect(deps.createTripFromSuggestion).toHaveBeenCalledWith(s)
    modal.showCostModal.value = true
    await modal.dismissTripFromCostModal(virtual)
    expect(deps.dismissTripSuggestion).toHaveBeenCalledWith(s)
  })
})
