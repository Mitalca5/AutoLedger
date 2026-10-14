import { beforeEach, describe, expect, it, vi } from 'vitest'
import { computed, ref } from 'vue'
import { useDriveBulkActions } from './useDriveBulkActions'

const api = vi.hoisted(() => ({ updateDriveTags: vi.fn(), applyTollEstimatesBulk: vi.fn(), updateDrivesDriver: vi.fn(), getVehiclePeople: vi.fn() }))
vi.mock('@/services/api', () => ({ api }))
vi.mock('@/i18n', () => ({ t: (k: string) => k }))
const push = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
const store = vi.hoisted(() => ({ activeVehicle: { id: 'v1' } as { id: string } | null, currency: 'EUR' }))
vi.mock('@/stores/vehicle', () => ({ useVehicleStore: () => store }))
const confirm = vi.hoisted(() => ({ showConfirm: vi.fn(), showAlert: vi.fn() }))
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => confirm }))
const csv = vi.hoisted(() => ({ downloadCsv: vi.fn() }))
vi.mock('@/utils/csv', () => csv)

function setup(selected: string[] = ['d1', 'd2']) {
  const drives = ref<any[]>([
    { id: 'd1', tags: [], start_time: '2026-01-01T10:00:00Z' },
    { id: 'd2', tags: ['Pro'], start_time: '2026-01-02T10:00:00Z' },
  ])
  const selectedDrives = ref<Record<string, any>>(Object.fromEntries(selected.map((id) => [id, drives.value.find((d) => d.id === id)])))
  const selectedDriveIds = computed(() => Object.keys(selectedDrives.value))
  const selectedList = computed(() => Object.values(selectedDrives.value))
  const clearSelection = vi.fn()
  const loadDrives = vi.fn().mockResolvedValue(undefined)
  const h = useDriveBulkActions({ drives, selectedDrives, selectedDriveIds, selectedList, clearSelection, loadDrives })
  return { h, drives, clearSelection, loadDrives }
}

describe('useDriveBulkActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    store.activeVehicle = { id: 'v1' }
    confirm.showConfirm.mockResolvedValue(true)
    api.getVehiclePeople.mockResolvedValue([{ id: 'p1', name: 'Alice' }])
  })

  it('assigns the driver of every selected drive in one call and reloads', async () => {
    api.updateDrivesDriver.mockResolvedValue({ success: true, updated: 2 })
    const { h, drives, loadDrives } = setup()
    await vi.waitFor(() => expect(h.vehiclePeople.value).toHaveLength(1))
    await h.handleBatchDriver('p1')
    expect(api.updateDrivesDriver).toHaveBeenCalledWith('v1', ['d1', 'd2'], 'p1')
    expect(drives.value.map((d) => d.driver_name)).toEqual(['Alice', 'Alice'])
    expect(confirm.showAlert).toHaveBeenCalledWith('drives.drivesView.driverUpdated', 'common.success', 'success')
    expect(loadDrives).toHaveBeenCalled()
  })

  it('clears the driver with null and reports a failure', async () => {
    api.updateDrivesDriver.mockRejectedValueOnce(new Error('boom'))
    const { h, drives, loadDrives } = setup()
    await h.handleBatchDriver(null)
    expect(api.updateDrivesDriver).toHaveBeenCalledWith('v1', ['d1', 'd2'], null)
    expect(drives.value[0].driver_id).toBeUndefined()
    expect(confirm.showAlert.mock.calls[0][2]).toBe('danger')
    expect(loadDrives).not.toHaveBeenCalled()
    api.updateDrivesDriver.mockClear()
    await setup([]).h.handleBatchDriver('p1')
    expect(api.updateDrivesDriver).not.toHaveBeenCalled()
  })

  it('has no people without a vehicle or when loading them fails', async () => {
    store.activeVehicle = null
    const none = setup()
    expect(none.h.vehiclePeople.value).toEqual([])
    expect(api.getVehiclePeople).not.toHaveBeenCalled()

    store.activeVehicle = { id: 'v1' }
    api.getVehiclePeople.mockRejectedValueOnce(new Error('down'))
    const failed = setup()
    await vi.waitFor(() => expect(api.getVehiclePeople).toHaveBeenCalled())
    expect(failed.h.vehiclePeople.value).toEqual([])
  })

  it('updates a selected drive that is no longer on the loaded page', async () => {
    api.updateDrivesDriver.mockResolvedValue({ success: true, updated: 1 })
    const { h, drives, loadDrives } = setup(['d1'])
    drives.value = []
    await vi.waitFor(() => expect(h.vehiclePeople.value).toHaveLength(1))
    await h.handleBatchDriver('p1')
    expect(api.updateDrivesDriver).toHaveBeenCalledWith('v1', ['d1'], 'p1')
    expect(loadDrives).toHaveBeenCalled()
  })

  it('tags every selected drive and reports the count', async () => {
    const { h, drives } = setup()
    await h.handleBatchTag('Perso')
    expect(api.updateDriveTags).toHaveBeenCalledTimes(2)
    expect(drives.value[0].tags).toEqual(['Perso'])
    expect(confirm.showAlert).toHaveBeenCalledWith('drives.drivesView.tagsUpdated', 'common.success', 'success')
  })

  it('alerts when tagging fails and does nothing without a selection', async () => {
    api.updateDriveTags.mockRejectedValueOnce(new Error('x'))
    const { h } = setup()
    await h.handleBatchTag('Pro')
    expect(confirm.showAlert).toHaveBeenCalledWith('drives.drivesView.batchTagError', 'shell.confirm.error', 'danger')
    api.updateDriveTags.mockClear()
    await setup([]).h.handleBatchTag('Pro')
    expect(api.updateDriveTags).not.toHaveBeenCalled()
  })

  it('exports the selection only when there is one', () => {
    setup([]).h.exportSelectedDrives()
    expect(csv.downloadCsv).not.toHaveBeenCalled()
    setup().h.exportSelectedDrives()
    expect(csv.downloadCsv).toHaveBeenCalledTimes(1)
  })

  it('opens a carpool with the selected drives in chronological order', async () => {
    const { h, clearSelection } = setup()
    await h.handleCarpoolSelectedDrives()
    expect(clearSelection).toHaveBeenCalled()
    expect(push).toHaveBeenCalledWith({ path: '/carpools', query: { new_drive_ids: 'd1,d2' } })
  })

  it('applies toll estimates after confirmation and reloads', async () => {
    api.applyTollEstimatesBulk.mockResolvedValue({ created: 1, updated: 0, skipped_manual: 0, skipped_trip_group: 0, skipped_no_price: 0, skipped_no_gps: 0, failed: 0 })
    const { h, loadDrives } = setup()
    await h.handleBulkApplyToll()
    expect(api.applyTollEstimatesBulk).toHaveBeenCalledWith('v1', ['d1', 'd2'])
    expect(confirm.showAlert.mock.calls[0][2]).toBe('info')
    expect(loadDrives).toHaveBeenCalled()
    expect(h.bulkApplyingToll.value).toBe(false)
  })

  it('warns when some estimates failed, and stops when the confirmation is declined', async () => {
    api.applyTollEstimatesBulk.mockResolvedValue({ created: 0, updated: 0, skipped_manual: 1, skipped_trip_group: 0, skipped_no_price: 0, skipped_no_gps: 0, failed: 2 })
    await setup().h.handleBulkApplyToll()
    expect(confirm.showAlert.mock.calls[0][2]).toBe('warning')
    api.applyTollEstimatesBulk.mockClear()
    confirm.showConfirm.mockResolvedValue(false)
    await setup().h.handleBulkApplyToll()
    expect(api.applyTollEstimatesBulk).not.toHaveBeenCalled()
  })

  it('alerts when the bulk toll request fails', async () => {
    api.applyTollEstimatesBulk.mockRejectedValue(new Error('x'))
    const { h } = setup()
    await h.handleBulkApplyToll()
    expect(confirm.showAlert).toHaveBeenCalledWith('common.errorWithMessage', 'shell.confirm.error', 'danger')
    expect(h.bulkApplyingToll.value).toBe(false)
  })
})
