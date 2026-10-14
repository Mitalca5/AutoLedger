import { ref, watch, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { t } from '@/i18n'
import { api, type VehiclePerson } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { downloadCsv } from '@/utils/csv'
import { applyBatchTag, driveCsvHeaders, driveCsvRows } from '@/utils/drives'

// What the bulk bar does with the selected drives: tag, driver, export, carpool and toll estimates.
export function useDriveBulkActions(options: {
  drives: Ref<any[]>
  selectedDrives: Readonly<Ref<Record<string, any>>>
  selectedDriveIds: Readonly<Ref<string[]>>
  selectedList: Readonly<Ref<any[]>>
  clearSelection: () => void
  loadDrives: () => Promise<void>
}) {
  const { drives, selectedDrives, selectedDriveIds, selectedList, clearSelection, loadDrives } = options
  const router = useRouter()
  const vehicleStore = useVehicleStore()
  const { showConfirm, showAlert } = useConfirm()
  const bulkApplyingToll = ref(false)

  // Batch tagging
  async function handleBatchTag(tag: 'Pro' | 'Perso' | null) {
    if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
    const ids = [...selectedDriveIds.value]
    try {
      for (const id of ids) {
        const d = selectedDrives.value[id] || drives.value.find((x) => x.id === id)
        const currentTags = applyBatchTag(d?.tags, tag)
        await api.updateDriveTags(vehicleStore.activeVehicle.id, id, currentTags)
        if (d) d.tags = currentTags
        const listed = drives.value.find((x) => x.id === id)
        if (listed) listed.tags = currentTags
      }
      void showAlert(t('drives.drivesView.tagsUpdated', { count: ids.length }), t('common.success'), 'success')
    } catch (err: any) {
      void showAlert(t('drives.drivesView.batchTagError', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  // Batch driver: a person of the vehicle, or null for "the vehicle's default driver"
  const vehiclePeople = ref<VehiclePerson[]>([])
  async function loadVehiclePeople() {
    const id = vehicleStore.activeVehicle?.id
    vehiclePeople.value = []
    if (!id) return
    try {
      vehiclePeople.value = await api.getVehiclePeople(id)
    } catch {
      vehiclePeople.value = []
    }
  }
  watch(() => vehicleStore.activeVehicle?.id, loadVehiclePeople, { immediate: true })

  async function handleBatchDriver(driverId: string | null) {
    if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
    const ids = [...selectedDriveIds.value]
    const person = vehiclePeople.value.find((p) => p.id === driverId)
    try {
      await api.updateDrivesDriver(vehicleStore.activeVehicle.id, ids, driverId)
      for (const id of ids) {
        for (const d of [selectedDrives.value[id], drives.value.find((x) => x.id === id)]) {
          if (d) {
            d.driver_id = driverId
            d.driver_name = person?.name ?? null
          }
        }
      }
      void showAlert(t('drives.drivesView.driverUpdated', { count: ids.length }), t('common.success'), 'success')
      await loadDrives()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  // Export selected drives to CSV
  function exportSelectedDrives() {
    if (!selectedList.value.length) return
    downloadCsv(`${t('drives.drivesView.csvFileName')}_${new Date().toISOString().slice(0, 10)}.csv`, driveCsvHeaders(vehicleStore.currency), driveCsvRows(selectedList.value))
  }

  async function handleCarpoolSelectedDrives() {
    if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
    // Each selected drive becomes a leg of the carpool, in chronological order
    const ids = selectedList.value.map((d: any) => d.id)
    clearSelection()
    void router.push({ path: '/carpools', query: { new_drive_ids: ids.join(',') } })
  }

  async function handleBulkApplyToll() {
    if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
    const ok = await showConfirm({
      title: t('drives.drivesView.autoTollTitle'),
      message: t('drives.drivesView.autoTollMessage', { count: selectedDriveIds.value.length }),
      confirmText: t('drives.drivesView.apply'),
      type: 'info',
    })
    if (!ok) return
    bulkApplyingToll.value = true
    try {
      const r = await api.applyTollEstimatesBulk(vehicleStore.activeVehicle.id, selectedDriveIds.value)
      const skipped = r.skipped_manual + r.skipped_trip_group + r.skipped_no_price + r.skipped_no_gps
      void showAlert(
        t('drives.drivesView.autoTollResult', { created: r.created, updated: r.updated, skipped, manual: r.skipped_manual, trip: r.skipped_trip_group, noPrice: r.skipped_no_price, noGps: r.skipped_no_gps }) + (r.failed ? t('drives.drivesView.autoTollFailed', { failed: r.failed }) : '') + '.',
        t('drives.drivesView.autoTollShort'),
        r.failed ? 'warning' : 'info'
      )
      await loadDrives()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    } finally {
      bulkApplyingToll.value = false
    }
  }

  return { bulkApplyingToll, vehiclePeople, handleBatchDriver, handleBatchTag, exportSelectedDrives, handleCarpoolSelectedDrives, handleBulkApplyToll }
}
