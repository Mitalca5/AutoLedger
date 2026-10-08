import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/services/api'
import { useTripGroups } from './useTripGroups'

vi.mock('@/services/api', () => ({
  api: {
    getTripGroups: vi.fn(),
    getTripSuggestions: vi.fn(),
    dismissTripSuggestion: vi.fn(),
    createTripGroup: vi.fn(),
    getDrives: vi.fn(),
  },
}))

vi.mock('@/stores/vehicle', () => ({
  useVehicleStore: () => ({ activeVehicle: { id: 'v1' }, currency: 'EUR' }),
}))

const alert = vi.fn()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ showConfirm: vi.fn(), showAlert: alert }),
}))

const suggestion = { drive_ids: ['d1', 'd2'], start_time: '2026-01-01T08:00:00Z', end_time: '2026-01-02T08:00:00Z', start_address: 'Paris', end_address: 'Lyon' }

describe('useTripGroups', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads the trips and the suggestions, and tolerates failing suggestions', async () => {
    vi.mocked(api.getTripGroups).mockResolvedValue([{ id: 't1' }] as any)
    vi.mocked(api.getTripSuggestions).mockRejectedValue(new Error('nope'))
    const trips = useTripGroups({ loadDrives: vi.fn() })
    await trips.loadTripGroups()
    expect(trips.tripGroups.value).toEqual([{ id: 't1' }])
    expect(trips.tripSuggestions.value).toEqual([])
    expect(trips.loadingTrips.value).toBe(false)
    expect(trips.tripsLoadError.value).toBeNull()
  })

  it('exposes the load error message', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.mocked(api.getTripGroups).mockRejectedValue(new Error('boom'))
    vi.mocked(api.getTripSuggestions).mockResolvedValue([] as any)
    const trips = useTripGroups({ loadDrives: vi.fn() })
    await trips.loadTripGroups()
    expect(trips.tripsLoadError.value).toBe('boom')
  })

  it('drops a dismissed suggestion and releases the busy key', async () => {
    vi.mocked(api.dismissTripSuggestion).mockResolvedValue(undefined as any)
    const trips = useTripGroups({ loadDrives: vi.fn() })
    trips.tripSuggestions.value = [suggestion, { ...suggestion, drive_ids: ['d9'] }]
    await trips.dismissTripSuggestion(suggestion)
    expect(api.dismissTripSuggestion).toHaveBeenCalledWith('v1', ['d1', 'd2'])
    expect(trips.tripSuggestions.value.map((s) => s.drive_ids[0])).toEqual(['d9'])
    expect(trips.suggestionBusyKey.value).toBeNull()
  })

  it('names a suggestion after its route and creates the trip', async () => {
    vi.mocked(api.createTripGroup).mockResolvedValue({} as any)
    vi.mocked(api.getTripGroups).mockResolvedValue([] as any)
    vi.mocked(api.getTripSuggestions).mockResolvedValue([] as any)
    const trips = useTripGroups({ loadDrives: vi.fn() })
    expect(trips.suggestionName(suggestion)).toBe('Paris → Lyon')
    await trips.createTripFromSuggestion(suggestion)
    expect(api.createTripGroup).toHaveBeenCalledWith('v1', { name: 'Paris → Lyon', drive_ids: ['d1', 'd2'] })
    expect(api.getTripGroups).toHaveBeenCalled()
  })

  it('shows the legs of a trip sorted by start, and collapses on a second toggle', async () => {
    vi.mocked(api.getDrives).mockResolvedValue({ drives: [{ id: 'b', start_time: '2026-01-02' }, { id: 'a', start_time: '2026-01-01' }] } as any)
    const trips = useTripGroups({ loadDrives: vi.fn() })
    await trips.toggleTripDetails({ id: 't1' })
    expect(trips.expandedTripId.value).toBe('t1')
    expect(trips.tripDrives.value.map((d) => d.id)).toEqual(['a', 'b'])
    await trips.toggleTripDetails({ id: 't1' })
    expect(trips.expandedTripId.value).toBeNull()
  })

  it('refuses to remove the last drive of a trip', async () => {
    const trips = useTripGroups({ loadDrives: vi.fn() })
    await trips.removeDriveFromTrip({ id: 't1', drive_ids: ['d1'] }, 'd1')
    expect(alert).toHaveBeenCalledTimes(1)
  })
})
