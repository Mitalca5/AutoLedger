import { ref } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { formatAmount } from '@/currency'
import { formatTripDates } from '@/utils/drives'

// Trip groups ("voyages") of the active vehicle: the list, the detected suggestions and their actions.
// loadDrives reloads the drive list, whose trip badges change with the trips.
export function useTripGroups({ loadDrives }: { loadDrives: () => unknown }) {
  const vehicleStore = useVehicleStore()
  const { showConfirm, showAlert } = useConfirm()

  const tripGroups = ref<any[]>([])
  const tripSuggestions = ref<any[]>([])
  const suggestionBusyKey = ref<string | null>(null)
  const loadingTrips = ref(false)
  const tripsLoadError = ref<string | null>(null)
  const expandedTripId = ref<string | null>(null)
  const tripDrives = ref<any[]>([])

  // ----- Trip groups ("voyages") -----
  async function loadTripGroups(silent = false) {
    if (!vehicleStore.activeVehicle) return
    if (!silent) loadingTrips.value = true
    tripsLoadError.value = null
    try {
      const vehicleId = vehicleStore.activeVehicle.id
      const [groups, suggestions] = await Promise.all([api.getTripGroups(vehicleId), api.getTripSuggestions(vehicleId).catch(() => [])])
      tripGroups.value = groups
      tripSuggestions.value = suggestions
    } catch (err) {
      console.error('Failed to load trip groups', err)
      tripsLoadError.value = (err as Error)?.message ?? ''
    } finally {
      loadingTrips.value = false
    }
  }

  async function dismissTripSuggestion(s: any) {
    if (!vehicleStore.activeVehicle) return
    suggestionBusyKey.value = s.drive_ids[0]
    try {
      await api.dismissTripSuggestion(vehicleStore.activeVehicle.id, s.drive_ids)
      tripSuggestions.value = tripSuggestions.value.filter((x) => x.drive_ids[0] !== s.drive_ids[0])
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    } finally {
      suggestionBusyKey.value = null
    }
  }

  function suggestionName(s: any) {
    const route = [s.start_address, s.end_address].filter(Boolean).join(' → ')
    return route || formatTripDates({ start_time: s.start_time, end_time: s.end_time })
  }

  async function createTripFromSuggestion(s: any) {
    if (!vehicleStore.activeVehicle) return
    suggestionBusyKey.value = s.drive_ids[0]
    try {
      await api.createTripGroup(vehicleStore.activeVehicle.id, { name: suggestionName(s), drive_ids: s.drive_ids })
      await loadTripGroups()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    } finally {
      suggestionBusyKey.value = null
    }
  }

  async function toggleTripDetails(tg: any) {
    if (expandedTripId.value === tg.id) {
      expandedTripId.value = null
      return
    }
    expandedTripId.value = tg.id
    tripDrives.value = []
    try {
      const res = await api.getDrives(vehicleStore.activeVehicle!.id, { tripGroupId: tg.id, limit: 200 })
      tripDrives.value = [...res.drives].sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  async function removeDriveFromTrip(tg: any, driveId: string) {
    if (!vehicleStore.activeVehicle) return
    const remaining = (tg.drive_ids || []).filter((id: string) => id !== driveId)
    if (!remaining.length) {
      void showAlert(t('drives.drivesView.tripNeedsDrive'), t('drives.drivesView.actionImpossible'), 'warning')
      return
    }
    try {
      await api.updateTripGroup(vehicleStore.activeVehicle.id, tg.id, { name: tg.name, notes: tg.notes, drive_ids: remaining })
      await loadTripGroups()
      const updated = tripGroups.value.find((g) => g.id === tg.id)
      expandedTripId.value = null
      if (updated) await toggleTripDetails(updated)
      loadDrives()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  async function handleDeleteTrip(tg: any) {
    if (!vehicleStore.activeVehicle) return
    const ok = await showConfirm({
      title: t('drives.drivesView.deleteTripTitle'),
      message: t('drives.drivesView.deleteTripMessage', { name: tg.name }),
      confirmText: t('drives.drivesView.deleteTripTitle'),
      type: 'danger',
    })
    if (!ok) return

    let deleteExpenses = false
    if (tg.expense_count > 0) {
      deleteExpenses = await showConfirm({
        title: t('drives.drivesView.tripCostsTitle'),
        message: t('drives.drivesView.tripCostsMessage', { count: tg.expense_count, total: formatAmount(Number(tg.expenses_total), vehicleStore.currency) }),
        confirmText: t('drives.drivesView.deleteCostsToo'),
        cancelText: t('drives.drivesView.keepCostsUnlinked'),
        type: 'warning',
      })
    }
    try {
      await api.deleteTripGroup(vehicleStore.activeVehicle.id, tg.id, deleteExpenses)
      await loadTripGroups()
      loadDrives()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  return {
    tripGroups,
    tripSuggestions,
    suggestionBusyKey,
    loadingTrips,
    tripsLoadError,
    expandedTripId,
    tripDrives,
    loadTripGroups,
    dismissTripSuggestion,
    suggestionName,
    createTripFromSuggestion,
    toggleTripDetails,
    removeDriveFromTrip,
    handleDeleteTrip,
  }
}
