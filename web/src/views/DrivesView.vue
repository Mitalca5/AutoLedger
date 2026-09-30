<script setup lang="ts">
import { t } from '@/i18n'
import { distanceUnit } from '@/units'
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { usePreferencesStore } from '@/stores/preferences'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import SelectAllToggle from '@/components/SelectAllToggle.vue'
import DrivesToolbar from '@/components/drives/DrivesToolbar.vue'
import DriveBulkActions from '@/components/drives/DriveBulkActions.vue'
import DriveCard from '@/components/drives/DriveCard.vue'
import DrivesPagination from '@/components/drives/DrivesPagination.vue'
import TripGroupsPanel from '@/components/drives/TripGroupsPanel.vue'
import TripSuggestions from '@/components/drives/TripSuggestions.vue'
import ToQualifyFilter from '@/components/drives/ToQualifyFilter.vue'
import DriveCostModal from '@/components/drives/DriveCostModal.vue'
import DriveGroupModal from '@/components/drives/DriveGroupModal.vue'
import TripEditModal from '@/components/drives/TripEditModal.vue'
import AddToTripModal from '@/components/drives/AddToTripModal.vue'
import ManualDriveModal from '@/components/drives/ManualDriveModal.vue'
import CSVImportModal from '@/components/CSVImportModal.vue'
import { downloadCsv } from '@/utils/csv'
import { formatAmount } from '@/currency'
import { Receipt, Layers, List, RotateCcw, Plus, UploadCloud } from 'lucide-vue-next'
import {
  driveCsvHeaders,
  applyBatchTag,
  buildTripCostDrive,
  buildSuggestionCostDrive,
  filterTrips,
  suggestionTripId,
  formatTripDates,
  currentYearMonth,
  driveCsvRows,
  monthRange,
  selectionSummary,
  toggleTag,
} from '@/utils/drives'

// The page owns the drives list, the filters, the selection and which modal is open; the toolbar, the cards,
// the pagination and the modals are components that report back to it.
const router = useRouter()
const vehicleStore = useVehicleStore()
const prefs = usePreferencesStore()
const { showConfirm, showAlert } = useConfirm()
const vehicleId = computed(() => vehicleStore.activeVehicle?.id ?? '')
const drives = ref<any[]>([])
const total = ref(0)
const page = ref(1)

// Page limit with persistent storage
const savedLimit = Number(localStorage.getItem('drives_limit'))
const limit = ref(savedLimit === 20 || savedLimit === 50 || savedLimit === 100 ? savedLimit : 20)
const totalPages = computed(() => Math.ceil(total.value / limit.value) || 1)
const selectedTag = ref('')
const unqualifiedOnly = ref(false)
const hasTollOnly = ref(false)
const tollSource = ref('')
const unqualifiedCount = ref(0)
const loading = ref(true)

// Manual drive and CSV import modals
const showManualDriveModal = ref(false)
const driveToEdit = ref<any | null>(null)
const showCSVImportModal = ref(false)

function openManualDriveModal(drive?: any) {
  driveToEdit.value = drive || null
  showManualDriveModal.value = true
}

function openCSVImportModal() {
  showCSVImportModal.value = true
}

function onDriveSaved() {
  loadDrives()
}

function onCSVImported() {
  loadDrives()
}

async function handleDeleteManualDrive(drive: any) {
  if (!vehicleStore.activeVehicle || !drive.is_manual) return
  const ok = await showConfirm({
    title: t('drives.drivesView.deleteDriveTitle'),
    message: t('drives.drivesView.deleteDriveMessage'),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDrive(vehicleStore.activeVehicle.id, drive.id)
    await loadDrives()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Period / month and address filters (edited in the toolbar)
const periodMode = ref<'ALL' | 'MONTH' | 'CUSTOM'>('ALL')
const selectedMonth = ref(currentYearMonth())
const customFrom = ref('')
const customTo = ref('')
const searchQuery = ref('')

// The trips and the suggestions are narrowed client-side by the same period and search as the drives
const tripFilter = computed(() => {
  if (periodMode.value === 'MONTH' && selectedMonth.value) return { ...monthRange(selectedMonth.value), q: searchQuery.value }
  if (periodMode.value === 'CUSTOM') return { from: customFrom.value || undefined, to: customTo.value || undefined, q: searchQuery.value }
  return { q: searchQuery.value }
})
const filteredTrips = computed(() => filterTrips(tripGroups.value, tripFilter.value))
const filteredSuggestions = computed(() => filterTrips(tripSuggestions.value, tripFilter.value))
const shownTrips = computed(() => (tripQualifyOnly.value ? filteredSuggestions.value : filteredTrips.value))
const hasFilters = computed(() => periodMode.value !== 'ALL' || !!searchQuery.value.trim())

function onFiltersChange() {
  if (viewMode.value === 'TRIPS') return
  page.value = 1
  loadDrives()
}

// Page sizing & navigation
function setLimit(newLimit: number) {
  limit.value = newLimit
  localStorage.setItem('drives_limit', newLimit.toString())
  page.value = 1
  loadDrives()
}

function goToPage(targetPage: number) {
  const p = Math.max(1, Math.min(totalPages.value, targetPage))
  if (p !== page.value) {
    page.value = p
    loadDrives()
    const scrollContainer = document.querySelector('main')?.parentElement
    if (scrollContainer) {
      scrollContainer.scrollTo({ top: 0, behavior: 'smooth' })
    } else {
      window.scrollTo({ top: 0, behavior: 'smooth' })
    }
  }
}

function resetAllFilters() {
  searchQuery.value = ''
  selectedTag.value = ''
  unqualifiedOnly.value = false
  hasTollOnly.value = false
  tollSource.value = ''
  periodMode.value = 'ALL'
  customFrom.value = ''
  customTo.value = ''
  page.value = 1
  loadDrives()
}

// Multi-selection for trip grouping & tolls & carpooling. Drives are kept by id so that the selection
// survives pagination and filters.
const selectedDrives = ref<Record<string, any>>({})
const selectedDriveIds = computed(() => Object.keys(selectedDrives.value))
const selectedList = computed(() =>
  Object.values(selectedDrives.value).sort((a: any, b: any) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())
)
const selectedOffPage = computed(() => selectedDriveIds.value.filter((id) => !drives.value.some((d) => d.id === id)).length)
const allPageSelected = computed(() => drives.value.length > 0 && drives.value.every((d) => selectedDrives.value[d.id]))
const somePageSelected = computed(() => !allPageSelected.value && drives.value.some((d) => selectedDrives.value[d.id]))

// Unified selection summary metrics (same as a Voyage)
const selectedSummaryMetrics = computed(() => selectionSummary(selectedList.value, vehicleStore.currency))

function clearSelection() {
  selectedDrives.value = {}
}

function toggleSelectDrive(d: any) {
  const next = { ...selectedDrives.value }
  if (next[d.id]) {
    delete next[d.id]
  } else {
    next[d.id] = d
  }
  selectedDrives.value = next
}

// Selects or unselects the drives of the current page, keeping selections made on other pages
function selectAll() {
  const next = { ...selectedDrives.value }
  const select = !allPageSelected.value
  for (const d of drives.value) {
    if (select) next[d.id] = d
    else delete next[d.id]
  }
  selectedDrives.value = next
}

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
    showAlert(t('drives.drivesView.tagsUpdated', { count: ids.length }), t('common.success'), 'success')
  } catch (err: any) {
    showAlert(t('drives.drivesView.batchTagError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Export selected drives to CSV
function exportSelectedDrives() {
  if (!selectedList.value.length) return
  downloadCsv(`${t('drives.drivesView.csvFileName')}_${new Date().toISOString().slice(0, 10)}.csv`, driveCsvHeaders(vehicleStore.currency), driveCsvRows(selectedList.value))
}

async function loadDrives(silent = false) {
  if (!vehicleStore.activeVehicle) {
    loading.value = false
    return
  }
  if (!silent) loading.value = true
  try {
    let fromStr: string | undefined
    let toStr: string | undefined

    if (periodMode.value === 'MONTH' && selectedMonth.value) {
      const range = monthRange(selectedMonth.value)
      fromStr = range.from
      toStr = range.to
    } else if (periodMode.value === 'CUSTOM') {
      if (customFrom.value) fromStr = customFrom.value
      if (customTo.value) toStr = customTo.value
    }

    const res = await api.getDrives(vehicleStore.activeVehicle.id, {
      tag: selectedTag.value,
      page: page.value,
      limit: limit.value,
      unqualified: unqualifiedOnly.value,
      hasToll: hasTollOnly.value,
      tollSource: hasTollOnly.value ? tollSource.value : '',
      from: fromStr,
      to: toStr,
      q: searchQuery.value.trim() || undefined,
    })
    drives.value = res.drives
    total.value = res.total
    unqualifiedCount.value = res.unqualified_count || 0
  } catch (err) {
    console.error('Failed to load drives', err)
  } finally {
    loading.value = false
  }
}

// View mode: drives list or trip groups ("voyages")
const viewMode = ref<'DRIVES' | 'TRIPS'>('DRIVES')
const tripGroups = ref<any[]>([])
const tripSuggestions = ref<any[]>([])
const suggestionBusyKey = ref<string | null>(null)
const tripQualifyOnly = ref(false)
const loadingTrips = ref(false)
const expandedTripId = ref<string | null>(null)
const tripDrives = ref<any[]>([])

// Modals
const showTripEditModal = ref(false)
const tripBeingEdited = ref<any | null>(null)
const showAddToTripModal = ref(false)
const showGroupModal = ref(false)
const showCostModal = ref(false)
const selectedCostDrive = ref<any | null>(null)
const costTripDriveIds = ref<string[]>([])
const costTripId = ref<string | null>(null)
// Detected trip whose breakdown is open before any trip group exists for it
const costSuggestion = ref<any | null>(null)
const tripLegs = ref<any[]>([])
const costStartWithToll = ref(false)
const bulkApplyingToll = ref(false)

watch(
  () => [vehicleStore.activeVehicle?.id, selectedTag.value, unqualifiedOnly.value, hasTollOnly.value, tollSource.value],
  () => {
    page.value = 1
    loadDrives()
    if (viewMode.value === 'TRIPS') loadTripGroups()
  }
)

// New data arrived (synchronization): reload in place, keeping the page, the filters and the selection
watch(
  () => vehicleStore.lastSyncTimestamp,
  () => {
    loadDrives(true)
    if (viewMode.value === 'TRIPS') loadTripGroups(true)
  }
)

watch(
  () => vehicleStore.activeVehicle?.id,
  () => clearSelection()
)

// A work/personal filter left on would hide drives with no way to clear it once the classification is off
watch(
  () => prefs.proPersoEnabled,
  (enabled) => {
    if (!enabled) selectedTag.value = ''
  },
  { immediate: true }
)

onMounted(() => {
  loadDrives()
})

async function markNoToll(d: any) {
  if (!vehicleStore.activeVehicle) return
  try {
    await api.setDriveTollReview(vehicleStore.activeVehicle.id, d.id, true)
    d.toll_reviewed_at = new Date().toISOString()
    d.needs_toll_qualification = false
    unqualifiedCount.value = Math.max(0, unqualifiedCount.value - 1)
    if (unqualifiedOnly.value) {
      drives.value = drives.value.filter((x) => x.id !== d.id)
      total.value = Math.max(0, total.value - 1)
    }
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

function openTollEntry(d: any) {
  openCostModal(d, true)
}

// ----- Trip groups ("voyages") -----
async function loadTripGroups(silent = false) {
  if (!vehicleStore.activeVehicle) return
  if (!silent) loadingTrips.value = true
  try {
    const vehicleId = vehicleStore.activeVehicle.id
    const [groups, suggestions] = await Promise.all([api.getTripGroups(vehicleId), api.getTripSuggestions(vehicleId).catch(() => [])])
    tripGroups.value = groups
    tripSuggestions.value = suggestions
  } catch (err) {
    console.error('Failed to load trip groups', err)
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
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
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
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    suggestionBusyKey.value = null
  }
}

function switchView(mode: 'DRIVES' | 'TRIPS') {
  viewMode.value = mode
  if (mode === 'TRIPS') loadTripGroups()
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
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// The edit form replaces the detail, like the edit of a carpool
function editTripFromCostModal(virtual: any) {
  const tg = tripGroups.value.find((g) => g.id === virtual.id)
  if (!tg) return
  showCostModal.value = false
  openTripEdit(tg)
}

// Its legs may have changed: the trips, the drives (their trip badge) and the open leg list are reloaded
async function onTripSaved() {
  expandedTripId.value = null
  await loadTripGroups()
  loadDrives()
}

function openTripEdit(tg: any) {
  tripBeingEdited.value = tg
  showTripEditModal.value = true
}

async function removeDriveFromTrip(tg: any, driveId: string) {
  if (!vehicleStore.activeVehicle) return
  const remaining = (tg.drive_ids || []).filter((id: string) => id !== driveId)
  if (!remaining.length) {
    showAlert(t('drives.drivesView.tripNeedsDrive'), t('drives.drivesView.actionImpossible'), 'warning')
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
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
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
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

async function openAddToTrip() {
  await loadTripGroups()
  showAddToTripModal.value = true
}

async function onAddedToTrip() {
  clearSelection()
  await loadTripGroups()
  loadDrives()
}

function onGroupCreated() {
  clearSelection()
  loadDrives()
}

async function toggleDriveTag(drive: any, tagToToggle: string) {
  if (!vehicleStore.activeVehicle) return
  const currentTags = toggleTag(drive.tags, tagToToggle)
  try {
    await api.updateDriveTags(vehicleStore.activeVehicle.id, drive.id, currentTags)
    drive.tags = currentTags
  } catch (err: any) {
    showAlert(t('drives.drivesView.tagUpdateError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

async function handleCarpoolSelectedDrives() {
  if (!vehicleStore.activeVehicle || !selectedDriveIds.value.length) return
  // Each selected drive becomes a leg of the carpool, in chronological order
  const ids = selectedList.value.map((d: any) => d.id)
  clearSelection()
  router.push({ path: '/carpools', query: { new_drive_ids: ids.join(',') } })
}

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
    showAlert(t('drives.drivesView.detailsLoadError', { message: err.message }), t('shell.confirm.error'), 'danger')
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
    showAlert(t('drives.drivesView.detailsLoadError', { message: err.message }), t('shell.confirm.error'), 'danger')
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
    showAlert(
      t('drives.drivesView.autoTollResult', { created: r.created, updated: r.updated, skipped, manual: r.skipped_manual, trip: r.skipped_trip_group, noPrice: r.skipped_no_price, noGps: r.skipped_no_gps }) + (r.failed ? t('drives.drivesView.autoTollFailed', { failed: r.failed }) : '') + '.',
      t('drives.drivesView.autoTollShort'),
      r.failed ? 'warning' : 'success'
    )
    await loadDrives()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    bulkApplyingToll.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Drives informational banner when no telemetry is linked -->
    <div v-if="vehicleStore.activeVehicle && !vehicleStore.hasTeslaMate && total === 0" class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-sky-500/30 bg-sky-500/10 p-4 text-sm text-sky-200">
      <span>{{ $t('drives.drivesView.noTelemetryBanner') }}</span>
      <div v-if="vehicleStore.canEdit" class="flex items-center gap-2">
        <button
          @click="openCSVImportModal"
          class="rounded-lg bg-sky-500/20 px-2.5 py-1 text-xs font-semibold text-sky-300 hover:bg-sky-500/30 transition-colors"
        >
          {{ $t('drives.drivesView.importCsv') }}
        </button>
        <button
          @click="openManualDriveModal()"
          class="rounded-lg bg-sky-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-sky-500 transition-colors"
        >
          {{ $t('drives.drivesView.newDrive') }}
        </button>
      </div>
    </div>

    <!-- Header & Filter Tabs -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <h2 class="text-2xl font-bold tracking-tight text-white">{{ $t('drives.drivesView.drivesAndTrips') }}</h2>
        <p class="text-sm text-slate-400">
          {{ $t('drives.drivesView.drivesActualEnergyAndTolls', { unit: distanceUnit(), total }) }}
        </p>
        <div class="flex items-center gap-1 mt-2 bg-slate-900 border border-slate-800 p-1 rounded-xl w-fit">
          <button
            @click="switchView('DRIVES')"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-colors"
            :class="viewMode === 'DRIVES' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
          >
            <List class="w-3.5 h-3.5" /> {{ $t('drives.drivesView.drives') }}
          </button>
          <button
            @click="switchView('TRIPS')"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-colors"
            :class="viewMode === 'TRIPS' ? 'bg-indigo-500/20 text-indigo-400 border border-indigo-500/30' : 'text-slate-400 hover:text-white'"
          >
            <Layers class="w-3.5 h-3.5" /> {{ $t('drives.drivesView.trips') }}
          </button>
        </div>
      </div>

      <!-- Actions & Tag Filters container -->
      <div class="flex flex-wrap items-center gap-3 self-start sm:self-center">
        <div v-if="vehicleStore.canEdit" class="flex items-center gap-2">
          <button
            @click="openCSVImportModal"
            class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 px-3.5 py-2 text-xs font-semibold text-slate-200 hover:border-slate-600 hover:bg-slate-700 transition-colors"
          >
            <UploadCloud class="w-4 h-4 text-sky-400" />
            <span>{{ $t('drives.drivesView.importCsv') }}</span>
          </button>
          <button
            @click="openManualDriveModal()"
            class="flex items-center gap-1.5 rounded-xl bg-rose-600 hover:bg-rose-500 px-3.5 py-2 text-xs font-semibold text-white shadow-lg shadow-rose-900/30 transition-all"
          >
            <Plus class="w-4 h-4" />
            <span>{{ $t('drives.drivesView.newDrive') }}</span>
          </button>
        </div>

        <!-- Tag Filters -->
        <div v-if="viewMode === 'DRIVES'" class="flex items-center gap-2 bg-slate-900 border border-slate-800 p-1 rounded-xl flex-wrap">
        <ToQualifyFilter
          :count="unqualifiedCount"
          :active="unqualifiedOnly"
          :title="$t('drives.drivesView.motorwayTypeDrivesWithNo')"
          @toggle="unqualifiedOnly = !unqualifiedOnly"
        />
        <button
          @click="hasTollOnly = !hasTollOnly"
          class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors flex items-center gap-1.5"
          :class="hasTollOnly ? 'bg-cyan-500/20 text-cyan-400 border border-cyan-500/30' : 'text-cyan-400/80 hover:text-cyan-300'"
          :title="$t('drives.drivesView.drivesWithATollExpense')"
        >
          <Receipt class="w-3.5 h-3.5" />
          {{ $t('drives.drivesView.withToll') }}
        </button>
        <template v-if="hasTollOnly">
          <label for="drives-toll-source" class="sr-only">{{ $t('drives.drivesView.tollSource') }}</label>
          <select
            id="drives-toll-source"
            v-model="tollSource"
            class="bg-slate-800 border border-slate-700 rounded-lg px-2 py-1 text-xs text-white focus:outline-none focus:border-cyan-500"
          >
            <option value="">{{ $t('drives.drivesView.all') }}</option>
            <option value="AUTO_TOLL">{{ $t('drives.drivesView.auto') }}</option>
            <option value="MANUAL">{{ $t('drives.drivesView.manual') }}</option>
          </select>
        </template>
        <template v-if="prefs.proPersoEnabled">
          <button
            @click="selectedTag = ''"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
            :class="selectedTag === '' ? 'bg-rose-500/20 text-rose-400 border border-rose-500/30' : 'text-slate-400 hover:text-white'"
          >
            {{ $t('drives.drivesView.all') }}
          </button>
          <button
            @click="selectedTag = 'Pro'"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
            :class="selectedTag === 'Pro' ? 'bg-blue-500/20 text-blue-400 border border-blue-500/30' : 'text-slate-400 hover:text-white'"
          >
            {{ $t('drives.drivesView.work') }}
          </button>
          <button
            @click="selectedTag = 'Perso'"
            class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
            :class="selectedTag === 'Perso' ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'text-slate-400 hover:text-white'"
          >
            {{ $t('drives.drivesView.personal') }}
          </button>
        </template>
      </div>

      <!-- Same slot for the trips: the queue of detected trips to qualify -->
      <div
        v-else-if="tripSuggestions.length > 0 || tripQualifyOnly"
        class="flex items-center gap-2 bg-slate-900 border border-slate-800 p-1 rounded-xl self-start sm:self-auto flex-wrap"
      >
        <ToQualifyFilter
          :count="tripSuggestions.length"
          :active="tripQualifyOnly"
          :title="$t('drives.tripSuggestions.toQualifyHint')"
          @toggle="tripQualifyOnly = !tripQualifyOnly"
        />
      </div>
      </div>
    </div>

    <!-- Filters & Navigation Toolbar (Drives Mode) -->
    <DrivesToolbar
      v-model:period-mode="periodMode"
      v-model:selected-month="selectedMonth"
      v-model:custom-from="customFrom"
      v-model:custom-to="customTo"
      v-model:search-query="searchQuery"
      :mode="viewMode"
      :total="viewMode === 'DRIVES' ? total : shownTrips.length"
      :loading="viewMode === 'DRIVES' ? loading : loadingTrips"
      :drives="viewMode === 'DRIVES' ? drives : shownTrips"
      @change="onFiltersChange"
    />

    <!-- Sticky Bulk Selection Bar -->
    <DriveBulkActions
      :selected-drive-ids="selectedDriveIds"
      :selected-off-page="selectedOffPage"
      :selected-summary-metrics="selectedSummaryMetrics"
      :bulk-applying-toll="bulkApplyingToll"
      @clear="clearSelection"
      @carpool="handleCarpoolSelectedDrives"
      @group="showGroupModal = true"
      @bulk-toll="handleBulkApplyToll"
      @tag="handleBatchTag"
      @export="exportSelectedDrives"
      @add-to-trip="openAddToTrip"
    />

    <template v-if="viewMode === 'DRIVES'">
    <!-- SKELETON LOADING STATE -->
    <div v-if="loading" class="space-y-3 animate-pulse">
      <div v-for="i in 5" :key="i" class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex items-center justify-between gap-4">
        <div class="flex items-start gap-3 w-2/3">
          <div class="w-5 h-5 bg-slate-800 rounded mt-1"></div>
          <div class="space-y-2.5 w-full">
            <div class="flex items-center gap-2">
              <div class="h-3 w-24 bg-slate-800 rounded"></div>
              <div class="h-4 w-16 bg-slate-800 rounded-full"></div>
              <div class="h-3 w-16 bg-slate-800 rounded"></div>
            </div>
            <div class="h-4 w-4/5 bg-slate-800/80 rounded"></div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <div class="h-8 w-24 bg-slate-800 rounded-xl"></div>
          <div class="h-7 w-12 bg-slate-800 rounded-lg"></div>
          <div class="h-7 w-12 bg-slate-800 rounded-lg"></div>
        </div>
      </div>
    </div>

    <!-- EMPTY STATE -->
    <div v-else-if="!drives.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400 space-y-3">
      <p>{{ $t('drives.drivesView.noDriveFoundForThis') }}</p>
      <button
        v-if="searchQuery || periodMode !== 'ALL' || selectedTag || unqualifiedOnly || hasTollOnly"
        @click="resetAllFilters"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-xl border border-slate-700 transition-colors"
      >
        <RotateCcw class="w-3.5 h-3.5" />
        {{ $t('drives.drivesView.resetTheFilters') }}
      </button>
    </div>

    <!-- REAL DRIVES LIST -->
    <div v-else class="space-y-3">
      <!-- Select all toggle & Total info -->
      <div class="flex items-center justify-between text-xs text-slate-400 px-2">
        <SelectAllToggle
          v-if="vehicleStore.canEdit"
          :checked="allPageSelected"
          :indeterminate="somePageSelected"
          :label="allPageSelected ? $t('drives.drivesView.deselectPage') : $t('drives.drivesView.selectPage')"
          @toggle="selectAll"
        />
        <span v-else></span>
        <span>{{ $t('drives.drivesView.pageOf', { page, totalPages }) }}</span>
      </div>

      <DriveCard
        v-for="d in drives"
        :key="d.id"
        :d="d"
        :selected="selectedDriveIds.includes(d.id)"
        @open="openCostModal"
        @toggle="toggleSelectDrive"
        @toll-entry="openTollEntry"
        @no-toll="markNoToll"
        @edit="openManualDriveModal"
        @delete="handleDeleteManualDrive"
      />

      <DrivesPagination
        :page="page"
        :limit="limit"
        :total="total"
        :total-pages="totalPages"
        @go-to-page="goToPage"
        @set-limit="setLimit"
      />
    </div>
    </template>

    <!-- TRIP GROUPS ("VOYAGES") -->
    <template v-else>
    <TripSuggestions
      v-if="tripQualifyOnly"
      :suggestions="filteredSuggestions"
      :busy-key="suggestionBusyKey"
      @open="openSuggestionCostModal"
      @create="createTripFromSuggestion"
      @dismiss="dismissTripSuggestion"
    />
    <TripGroupsPanel
      v-else
      :loading-trips="loadingTrips"
      :trip-groups="filteredTrips"
      :has-filters="hasFilters"
      :expanded-trip-id="expandedTripId"
      :trip-drives="tripDrives"
      @open-cost="openTripCostModal"
      @toggle-details="toggleTripDetails"
      @edit="openTripEdit"
      @delete="handleDeleteTrip"
      @remove-drive="removeDriveFromTrip"
    />
    </template>

    <DriveCostModal
      v-model:open="showCostModal"
      v-model:drive="selectedCostDrive"
      :vehicle-id="vehicleId"
      :trip-drive-ids="costTripDriveIds"
      :trip-legs="tripLegs"
      :start-with-toll-entry="costStartWithToll"
      :refresh-drive="refreshCostDrive"
      @toggle-tag="toggleDriveTag"
      @edit-trip="editTripFromCostModal"
      @create-trip="createTripFromCostModal"
      @dismiss-trip="dismissTripFromCostModal"
    />

    <DriveGroupModal
      v-model:open="showGroupModal"
      :vehicle-id="vehicleId"
      :selected-drive-ids="selectedDriveIds"
      :selected-list="selectedList"
      @saved="onGroupCreated"
    />

    <TripEditModal
      v-model:open="showTripEditModal"
      :vehicle-id="vehicleId"
      :trip="tripBeingEdited"
      @saved="onTripSaved"
    />

    <AddToTripModal
      v-model:open="showAddToTripModal"
      :vehicle-id="vehicleId"
      :trip-groups="tripGroups"
      :selected-drive-ids="selectedDriveIds"
      @saved="onAddedToTrip"
    />

    <ManualDriveModal
      v-model:open="showManualDriveModal"
      :vehicle-id="vehicleId"
      :drive="driveToEdit"
      @saved="onDriveSaved"
    />

    <CSVImportModal
      v-model:open="showCSVImportModal"
      :vehicle-id="vehicleId"
      default-type="DRIVES"
      @imported="onCSVImported"
    />
  </div>
</template>
