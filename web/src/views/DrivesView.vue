<script setup lang="ts">
import LoadError from '@/components/LoadError.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import TabBar, { type TabItem } from '@/components/TabBar.vue'
import { Navigation as PageIcon } from 'lucide-vue-next'
import { t } from '@/i18n'
import { distanceUnit } from '@/units'
import { ref, computed, onMounted, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { usePreferencesStore } from '@/stores/preferences'
import { useConfirm } from '@/composables/useConfirm'
import { useDriveSelection } from '@/composables/useDriveSelection'
import { useDriveBulkActions } from '@/composables/useDriveBulkActions'
import { useTripGroups } from '@/composables/useTripGroups'
import { useDriveCostModal } from '@/composables/useDriveCostModal'
import { api } from '@/services/api'
import SelectAllToggle from '@/components/SelectAllToggle.vue'
import DrivesToolbar from '@/components/drives/DrivesToolbar.vue'
import DriveBulkActions from '@/components/drives/DriveBulkActions.vue'
import DriveCard from '@/components/drives/DriveCard.vue'
import DrivesPagination from '@/components/drives/DrivesPagination.vue'
import AddressBackfillNotice from '@/components/drives/AddressBackfillNotice.vue'
import TripGroupsPanel from '@/components/drives/TripGroupsPanel.vue'
import TripSuggestions from '@/components/drives/TripSuggestions.vue'
import ToQualifyFilter from '@/components/drives/ToQualifyFilter.vue'
import DriveFilterGroups from '@/components/drives/DriveFilterGroups.vue'
import DriveListSkeleton from '@/components/drives/DriveListSkeleton.vue'
import DriveCostModal from '@/components/drives/DriveCostModal.vue'
import DriveGroupModal from '@/components/drives/DriveGroupModal.vue'
import TripEditModal from '@/components/drives/TripEditModal.vue'
import AddToTripModal from '@/components/drives/AddToTripModal.vue'
import ManualDriveModal from '@/components/drives/ManualDriveModal.vue'
import CSVImportModal from '@/components/CSVImportModal.vue'
import EmptySourceHints from '@/components/EmptySourceHints.vue'
import { Layers, List, RotateCcw, Plus, UploadCloud } from 'lucide-vue-next'
import {
  filterTrips,
  currentYearMonth,
  countActiveDriveFilters,
  periodBounds,
  selectionSummary,
  todayIso,
  type PeriodMode,
  toggleTag,
} from '@/utils/drives'

// The page owns the drives list, the filters, the selection and which modal is open; the toolbar, the cards,
// the pagination and the modals are components that report back to it.
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
const loadError = ref<string | null>(null)

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
    void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Period / month and address filters (edited in the toolbar)
const periodMode = ref<PeriodMode>('ALL')
const selectedMonth = ref(currentYearMonth())
// The day the DAY, WEEK and YEAR modes step from
const anchorDay = ref(todayIso())
const customFrom = ref('')
const customTo = ref('')
const searchQuery = ref('')

// The trips and the suggestions are narrowed client-side by the same period and search as the drives
const tripFilter = computed(() => {
  return { ...periodBounds({ mode: periodMode.value, anchor: anchorDay.value, month: selectedMonth.value, from: customFrom.value, to: customTo.value }), q: searchQuery.value }
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

const {
  selectedDrives,
  selectedDriveIds,
  selectedList,
  selectedOffPage,
  allPageSelected,
  somePageSelected,
  clearSelection,
  toggleSelectDrive,
  selectAll,
} = useDriveSelection(drives)

// Unified selection summary metrics (same as a Voyage)
// The page figures above already are the selection's when the whole page is selected
const selectedSummaryMetrics = computed(() =>
  selectedOffPage.value === 0 && selectedDriveIds.value.length === drives.value.length ? '' : selectionSummary(selectedList.value, vehicleStore.currency)
)

async function loadDrives(silent = false) {
  if (!vehicleStore.activeVehicle) {
    loading.value = false
    return
  }
  if (!silent) loading.value = true
  loadError.value = null
  try {
    const { from: fromStr, to: toStr } = periodBounds({ mode: periodMode.value, anchor: anchorDay.value, month: selectedMonth.value, from: customFrom.value, to: customTo.value })

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
    loadError.value = (err as Error)?.message ?? ''
  } finally {
    loading.value = false
  }
}

// View mode: drives list or trip groups ("voyages")
const viewMode = ref<'DRIVES' | 'TRIPS'>('DRIVES')
const tripQualifyOnly = ref(false)

// On a phone the filters sit behind one button, so the first drive is not pushed below the fold; a wide screen shows them
const activeFilterCount = computed(() => countActiveDriveFilters({ mode: viewMode.value, periodMode: periodMode.value, unqualifiedOnly: unqualifiedOnly.value, hasTollOnly: hasTollOnly.value, selectedTag: selectedTag.value, tripQualifyOnly: tripQualifyOnly.value }))
const {
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
} = useTripGroups({ loadDrives })

// Modals
const showTripEditModal = ref(false)
const tripBeingEdited = ref<any | null>(null)
const showAddToTripModal = ref(false)
const showGroupModal = ref(false)
const { bulkApplyingToll, vehiclePeople, handleBatchDriver, handleBatchTag, exportSelectedDrives, handleCarpoolSelectedDrives, handleBulkApplyToll } = useDriveBulkActions({
  drives,
  selectedDrives,
  selectedDriveIds,
  selectedList,
  clearSelection,
  loadDrives,
})
const {
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
} = useDriveCostModal({
  loadDrives,
  tripGroups,
  tripSuggestions,
  loadTripGroups,
  suggestionName,
  createTripFromSuggestion,
  dismissTripSuggestion,
})

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

// The qualification queue and the toll source only exist with the toll detection: leave them when it goes away
watch(
  () => vehicleStore.tollDetectionActive,
  (active) => {
    if (active) return
    unqualifiedOnly.value = false
    tollSource.value = ''
  },
  { immediate: true }
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
    void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

function openTollEntry(d: any) {
  openCostModal(d, true)
}

const viewTabs = computed<TabItem[]>(() => [
  { key: 'DRIVES', label: t('drives.drivesView.drives'), icon: List },
  { key: 'TRIPS', label: t('drives.drivesView.trips'), icon: Layers },
])

function switchView(mode: 'DRIVES' | 'TRIPS') {
  viewMode.value = mode
  if (mode === 'TRIPS') loadTripGroups()
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
    void showAlert(t('drives.drivesView.tagUpdateError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Drives informational banner when no telemetry is linked -->
    <div v-if="vehicleStore.activeVehicle && !vehicleStore.hasTeslaMate && total === 0" class="rounded-2xl border border-info-500/30 bg-info-500/10 p-4 text-sm text-info-200">
      <span>{{ $t('drives.drivesView.noTelemetryBanner') }}</span>
    </div>

    <AddressBackfillNotice v-if="vehicleId" :vehicle-id="vehicleId" :can-edit="vehicleStore.canEdit" @resolved="loadDrives(true)" />

    <!-- Header & Filter Tabs -->
    <PageHeader :title="$t('shell.nav.drives')" :icon="PageIcon">
      {{ $t('drives.drivesView.drivesActualEnergyAndTolls', { unit: distanceUnit(), total }) }}
      <template #actions>
      <!-- Actions & Tag Filters container -->
      <div class="flex flex-wrap items-center gap-3 self-start sm:self-center">
        <div v-if="vehicleStore.canEdit" class="flex items-center gap-2">
          <button
            @click="openCSVImportModal"
            class="btn btn-lg btn-secondary hover:border-slate-600"
          >
            <UploadCloud class="w-4 h-4 text-info-400" />
            <span>{{ $t('drives.drivesView.importCsv') }}</span>
          </button>
          <button
            @click="openManualDriveModal()"
            class="btn btn-lg btn-primary"
          >
            <Plus class="w-4 h-4" />
            <span>{{ $t('drives.drivesView.newDrive') }}</span>
          </button>
        </div>


      </div>
    
      </template>
    </PageHeader>

    <TabBar :model-value="viewMode" :tabs="viewTabs" :label="$t('shell.nav.drives')" id-prefix="drives-tab" @update:model-value="switchView($event as 'DRIVES' | 'TRIPS')" />

    <!-- Filters & Navigation Toolbar (Drives Mode) -->
    <DrivesToolbar
      v-model:period-mode="periodMode"
      v-model:selected-month="selectedMonth"
      v-model:anchor-day="anchorDay"
      v-model:custom-from="customFrom"
      v-model:custom-to="customTo"
      v-model:search-query="searchQuery"
      :mode="viewMode"
      :active-count="activeFilterCount"
      :total="viewMode === 'DRIVES' ? total : shownTrips.length"
      :loading="viewMode === 'DRIVES' ? loading : loadingTrips"
      :drives="viewMode === 'DRIVES' ? drives : shownTrips"
      @change="onFiltersChange"
    >
      <template #filters>
        <DriveFilterGroups
          v-if="viewMode === 'DRIVES'"
          v-model:unqualified-only="unqualifiedOnly"
          v-model:has-toll-only="hasTollOnly"
          v-model:toll-source="tollSource"
          v-model:selected-tag="selectedTag"
          :unqualified-count="unqualifiedCount"
          :toll-detection-active="vehicleStore.tollDetectionActive"
          :pro-perso-enabled="prefs.proPersoEnabled"
        />
        <!-- The trips: the queue of detected trips to qualify -->
        <div v-else-if="tripSuggestions.length > 0 || tripQualifyOnly" id="drives-filter-groups" class="flex flex-wrap items-center gap-2">
          <ToQualifyFilter
            :count="tripSuggestions.length"
            :active="tripQualifyOnly"
            :title="$t('drives.tripSuggestions.toQualifyHint')"
            @toggle="tripQualifyOnly = !tripQualifyOnly"
          />
        </div>
      </template>
    </DrivesToolbar>

    <!-- Sticky Bulk Selection Bar -->
    <DriveBulkActions
      :selected-drive-ids="selectedDriveIds"
      :selected-off-page="selectedOffPage"
      :selected-summary-metrics="selectedSummaryMetrics"
      :bulk-applying-toll="bulkApplyingToll"
      :toll-detection-active="vehicleStore.tollDetectionActive"
      :people="vehiclePeople"
      @clear="clearSelection"
      @carpool="handleCarpoolSelectedDrives"
      @group="showGroupModal = true"
      @bulk-toll="handleBulkApplyToll"
      @tag="handleBatchTag"
      @driver="handleBatchDriver"
      @export="exportSelectedDrives"
      @add-to-trip="openAddToTrip"
    />

    <template v-if="viewMode === 'DRIVES'">
    <DriveListSkeleton v-if="loading" />

    <LoadError v-else-if="loadError !== null" :message="loadError" @retry="loadDrives()" />

    <!-- EMPTY STATE -->
    <EmptyState v-else-if="!drives.length" :icon="PageIcon" :title="$t('drives.drivesView.noDriveFoundForThis')">
      <template #actions>
      <EmptySourceHints v-if="!(searchQuery || periodMode !== 'ALL' || selectedTag || unqualifiedOnly || hasTollOnly)" @import-csv="openCSVImportModal" />
      <button
        v-if="searchQuery || periodMode !== 'ALL' || selectedTag || unqualifiedOnly || hasTollOnly"
        @click="resetAllFilters"
        class="btn btn-secondary"
      >
        <RotateCcw class="w-3.5 h-3.5" />
        {{ $t('drives.drivesView.resetTheFilters') }}
      </button>
      </template>
    </EmptyState>

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
    <LoadError v-if="tripsLoadError !== null" :message="tripsLoadError" @retry="loadTripGroups()" />
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
