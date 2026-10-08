import { ref, type Ref } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { buildSuggestionCostDrive, buildTripCostDrive, suggestionTripId } from '@/utils/drives'

interface CostModalDeps {
  loadDrives: () => unknown
  tripGroups: Ref<any[]>
  tripSuggestions: Ref<any[]>
  loadTripGroups: () => unknown
  suggestionName: (s: any) => string
  createTripFromSuggestion: (s: any) => unknown
  dismissTripSuggestion: (s: any) => unknown
}

// The cost breakdown modal of a drive, a trip or a detected trip suggestion, and the reload that keeps it in
// step with the server-side costs.
export function useDriveCostModal({
  loadDrives,
  tripGroups,
  tripSuggestions,
  loadTripGroups,
  suggestionName,
  createTripFromSuggestion,
  dismissTripSuggestion,
}: CostModalDeps) {
  const vehicleStore = useVehicleStore()
  const { showAlert } = useConfirm()

  const showCostModal = ref(false)
  const selectedCostDrive = ref<any | null>(null)
  const costTripDriveIds = ref<string[]>([])
  const costTripId = ref<string | null>(null)
  // Detected trip whose breakdown is open before any trip group exists for it
  const costSuggestion = ref<any | null>(null)
  const tripLegs = ref<any[]>([])
  const costStartWithToll = ref(false)

  // Cost breakdown modal
  async function fetchTripLegs(tripId: string) {
    const res = await api.getDrives(vehicleStore.activeVehicle!.id, { tripGroupId: tripId, limit: 200 })
    return [...res.drives].sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
  }

  // A suggestion is not a trip group yet: its drives are found by the period it covers
  async function fetchSuggestionLegs(s: any) {
    const res = await api.getDrives(vehicleStore.activeVehicle!.id, { from: s.start_time, to: s.end_time, limit: 200 })
    const ids = new Set<string>(s.drive_ids)
    return res.drives.filter((d: any) => ids.has(d.id)).sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
  }

  async function openSuggestionCostModal(s: any) {
    if (!vehicleStore.activeVehicle) return
    try {
      const legs = await fetchSuggestionLegs(s)
      selectedCostDrive.value = buildSuggestionCostDrive(s, legs, suggestionName(s))
      costTripDriveIds.value = legs.map((d: any) => d.id)
      costTripId.value = null
      costSuggestion.value = s
      tripLegs.value = legs
      showCostModal.value = true
    } catch (err: any) {
      void showAlert(t('drives.drivesView.detailsLoadError', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  // The modal only knows the trip it shows: find the suggestion it was built from
  function suggestionOf(virtual: any) {
    return tripSuggestions.value.find((x) => suggestionTripId(x) === virtual.id)
  }

  async function createTripFromCostModal(virtual: any) {
    const s = suggestionOf(virtual)
    showCostModal.value = false
    if (s) await createTripFromSuggestion(s)
  }

  async function dismissTripFromCostModal(virtual: any) {
    const s = suggestionOf(virtual)
    showCostModal.value = false
    if (s) await dismissTripSuggestion(s)
  }

  async function openTripCostModal(tg: any) {
    if (!vehicleStore.activeVehicle) return
    try {
      const tgDrives = await fetchTripLegs(tg.id)
      costSuggestion.value = null
      selectedCostDrive.value = buildTripCostDrive(tg, tgDrives)
      costTripDriveIds.value = tgDrives.map((d: any) => d.id)
      costTripId.value = tg.id
      tripLegs.value = tgDrives
      showCostModal.value = true
    } catch (err: any) {
      void showAlert(t('drives.drivesView.detailsLoadError', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  function openCostModal(drive: any, startWithToll = false) {
    selectedCostDrive.value = drive
    costStartWithToll.value = startWithToll
    costTripId.value = null
    costSuggestion.value = null
    tripLegs.value = []
    showCostModal.value = true
  }

  // Reloads the costs and returns the refreshed drive or trip, so the open cost breakdown follows the server-side costs.
  // Inside a trip, its legs are reloaded too since they are what the trip total and each leg are built from.
  async function refreshCostDrive(id: string) {
    const suggestion = costSuggestion.value
    if (suggestion) {
      const legs = await fetchSuggestionLegs(suggestion)
      tripLegs.value = legs
      costTripDriveIds.value = legs.map((d: any) => d.id)
      if (id === suggestionTripId(suggestion)) return buildSuggestionCostDrive(suggestion, legs, suggestionName(suggestion))
      return legs.find((d: any) => d.id === id) ?? null
    }
    const tripId = costTripId.value
    if (tripId) {
      const [, legs] = await Promise.all([loadTripGroups(), fetchTripLegs(tripId)])
      tripLegs.value = legs
      costTripDriveIds.value = legs.map((d: any) => d.id)
      const tg = tripGroups.value.find((g) => g.id === tripId)
      if (id === tripId) return tg ? buildTripCostDrive(tg, legs) : null
      const leg = legs.find((d: any) => d.id === id)
      if (leg) return leg
    }
    // Fetched by id: after a toll is added, the drive may have left the filtered list (e.g. "to qualify")
    const [, fresh] = await Promise.all([loadDrives(), api.getDrives(vehicleStore.activeVehicle!.id, { driveId: id, limit: 1 })])
    return fresh.drives[0] ?? null
  }

  return {
    showCostModal,
    selectedCostDrive,
    costTripDriveIds,
    tripLegs,
    costStartWithToll,
    openSuggestionCostModal,
    createTripFromCostModal,
    dismissTripFromCostModal,
    openTripCostModal,
    openCostModal,
    refreshCostDrive,
  }
}
