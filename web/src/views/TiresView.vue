<script setup lang="ts">
import TabBar, { type TabItem } from '@/components/TabBar.vue'
import PageHeader from '@/components/PageHeader.vue'
import { useSubmit } from '@/composables/useSubmit'
import { useTireList } from '@/composables/useTireList'
import { useTireSelection } from '@/composables/useTireSelection'
import LoadError from '@/components/LoadError.vue'
import ListSkeleton from '@/components/ListSkeleton.vue'
import { Disc as PageIcon } from 'lucide-vue-next'
import { intlLocale, t } from '@/i18n'
import { ref, onMounted, computed, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import SelectAllToggle from '@/components/SelectAllToggle.vue'
import TireOdometerTimeline from '@/components/tires/TireOdometerTimeline.vue'
import TireWheelCard from '@/components/tires/TireWheelCard.vue'
import TireStorageCard from '@/components/tires/TireStorageCard.vue'
import TireDisposedCard from '@/components/tires/TireDisposedCard.vue'
import TireAddModal from '@/components/tires/TireAddModal.vue'
import TireHistoryModal from '@/components/tires/TireHistoryModal.vue'
import TireSessionModal from '@/components/tires/TireSessionModal.vue'
import TireLogModal from '@/components/tires/TireLogModal.vue'
import TirePackSwapModal from '@/components/tires/TirePackSwapModal.vue'
import TireEditModal from '@/components/tires/TireEditModal.vue'
import TireDisposeModal from '@/components/tires/TireDisposeModal.vue'
import TireBatchSessionModal from '@/components/tires/TireBatchSessionModal.vue'
import TireDuplicateSessionModal from '@/components/tires/TireDuplicateSessionModal.vue'
import TireBatchDisposeModal from '@/components/tires/TireBatchDisposeModal.vue'
import TireCopyHistoryModal from '@/components/tires/TireCopyHistoryModal.vue'
import { Archive, ArrowUpDown, Copy, Disc, History, Package, Pencil, Plus, RefreshCw, Shuffle, Snowflake } from 'lucide-vue-next'
import {
  copiedSessionFromSession,
  emptySessionForm,
  formatDate,
  sessionFormFromCopy,
  sessionFormFromSession,
  type SessionForm,
  type TireLogForm,
  getLastDismountInfo,
} from '@/utils/tires'
import { todayIso, toIsoDay } from '@/utils/dates'

// The page owns the tire list, the selection and which modal is open; each modal owns its form and
// its API call and reports back with "saved".
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const {
  tires,
  loading,
  loadError,
  loadFailed,
  ready,
  vehicleId,
  mountedTires,
  hasMountedTires,
  storageTires,
  disposedTires,
  loadTires,
} = useTireList()
const currentOdometer = computed(() => vehicleStore.activeVehicle?.current_odometer || 0)

// Active tab: 'chassis' (Montés) or 'storage' (Au garage)
const activeTab = ref<'chassis' | 'storage' | 'disposed'>('chassis')
const tabs = computed<TabItem[]>(() => [
  { key: 'chassis', label: t('tires.tiresView.tiresFittedOnTheVehicle'), icon: Disc },
  { key: 'storage', label: t('tires.tiresView.catalogueAndGarageStock', { length: storageTires.value.length }), icon: Package },
  { key: 'disposed', label: t('tires.tiresView.scrapped', { length: disposedTires.value.length }), icon: Archive, muted: disposedTires.value.length === 0 },
])

const wheels = [
  { pos: 'FL', labelKey: 'tires.wheels.FL' },
  { pos: 'FR', labelKey: 'tires.wheels.FR' },
  { pos: 'RL', labelKey: 'tires.wheels.RL' },
  { pos: 'RR', labelKey: 'tires.wheels.RR' },
]

// Modals
const showAddTireModal = ref(false)
const showHistoryModal = ref(false)
const showSessionModal = ref(false)
const showBatchSessionModal = ref(false)
const showDuplicateSessionModal = ref(false)
const showCopyHistoryModal = ref(false)
const showBatchDisposeModal = ref(false)
const showLogModal = ref(false)
const showPackSwapModal = ref(false)
const showTireEditModal = ref(false)
const showDisposeModal = ref(false)

// Tire shown in the history modal, with its stats, mount sessions and tread depth logs
const selectedTire = ref<any | null>(null)
const selectedTireStats = ref<any | null>(null)
const tireSessions = ref<any[]>([])
const tireLogs = ref<any[]>([])

// Mount session form (add / edit / paste) and the session kept by the "copy" button
const editingSessionId = ref<string | null>(null)
const sessionInitialForm = ref<SessionForm>(emptySessionForm())
const copiedSession = ref<SessionForm | null>(null)
const sessionToDuplicate = ref<any | null>(null)

// Tread depth log form
const editingLogId = ref<string | null>(null)
const logInitialForm = ref<TireLogForm>({ depth_mm: 6.5, odometer: 0, notes: '', date: todayIso() })

const tireEditIds = ref<string[]>([])
const copyHistorySource = ref<any | null>(null)

const {
  selectedTireIds,
  toggleTireSelection,
  currentTabTireIds,
  isCurrentTabAllSelected,
  isCurrentTabPartlySelected,
  toggleSelectAllCurrentTab,
  canBatchDispose,
} = useTireSelection({ tires, mountedTires, storageTires, disposedTires, activeTab })

watch(
  () => [vehicleStore.activeVehicle?.id, activeTab.value],
  () => {
    selectedTireIds.value = []
  }
)

watch(
  () => [vehicleStore.activeVehicle?.id, vehicleStore.lastSyncTimestamp],
  () => {
    loadTires()
  }
)

onMounted(() => {
  loadTires()
})

// Quick rotations
const { pending: rotating, run: runOnce } = useSubmit()
async function quickRotateAction(mode: 'FRONT_BACK' | 'CROSS') {
  if (!vehicleStore.activeVehicle) return
  const odo = Math.round(vehicleStore.activeVehicle.current_odometer || 0)

  try {
    await api.quickRotateTires(vehicleStore.activeVehicle.id, {
      mode,
      odometer: odo,
    })
    await loadTires()
  } catch (err: any) {
    showAlert(t('tires.tiresView.rotationError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleQuickRotate = (mode: 'FRONT_BACK' | 'CROSS') => runOnce(() => quickRotateAction(mode))

function openPackSwapModal() {
  if (!vehicleStore.activeVehicle) return
  if (storageTires.value.length === 0) {
    showAlert(t('tires.tiresView.changeSetNeedsStorage'), t('tires.tiresView.changeSet'), 'info')
    return
  }
  showPackSwapModal.value = true
}

function mountOnEmptyWheel() {
  if (storageTires.value.length > 0) activeTab.value = 'storage'
  else openAddModal()
}

function openAddModal() {
  showAddTireModal.value = true
}

// History modal
function openTimelineTire(tireId: string) {
  const t = tires.value.find((x) => x.tire.id === tireId)
  if (t) openHistoryModal(t)
}

async function openHistoryModal(tire: any) {
  selectedTire.value = tire.tire
  selectedTireStats.value = tire
  try {
    const res = await api.getTireHistory(vehicleStore.activeVehicle!.id, tire.tire.id)
    selectedTire.value = res.tire
    selectedTireStats.value = res.stats
    tireSessions.value = res.sessions || []
    tireLogs.value = res.logs || []
    showHistoryModal.value = true
  } catch (err: any) {
    showAlert(t('tires.tiresView.loadError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

/** Reload the list, then the open history so it shows the change. */
async function refreshAfterHistoryChange() {
  await loadTires()
  if (showHistoryModal.value && selectedTire.value) await openHistoryModal({ tire: selectedTire.value })
}

// Edit one or several tires
function openTireEdit(ids: string[]) {
  tireEditIds.value = [...ids]
  showTireEditModal.value = true
}

async function onTireEdited() {
  selectedTireIds.value = []
  await refreshAfterHistoryChange()
}

// Dispose (worn out, damaged, sold) keeps history and cost; delete removes an erroneous entry
async function onTireDisposed(tireId: string) {
  showHistoryModal.value = false
  selectedTireIds.value = selectedTireIds.value.filter((id) => id !== tireId)
  await loadTires()
}

function openBatchSessionModal() {
  if (storageTires.value.length === 0) return
  showBatchSessionModal.value = true
}

function openBatchDisposeModal() {
  if (!selectedTireIds.value.length) return
  showBatchDisposeModal.value = true
}

async function onBatchDisposed() {
  selectedTireIds.value = []
  await loadTires()
}

function openCopyHistoryModal() {
  let source: any | null
  if (selectedTireIds.value.length === 1) {
    const s = tires.value.find((t) => t.tire.id === selectedTireIds.value[0])
    source = s?.tire || tires.value[0]?.tire || null
  } else {
    source = storageTires.value[0]?.tire || tires.value[0]?.tire || null
  }
  if (!source) return
  copyHistorySource.value = source
  showCopyHistoryModal.value = true
}

async function onHistoryCopied() {
  await refreshAfterHistoryChange()
}

async function handleDeleteTire(tire: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('tires.tiresView.deleteTireTitle'),
    message: t('tires.tiresView.deleteTireMessage', { brand: tire.brand, model: tire.model, dimension: tire.dimension }),
    confirmText: t('tires.tiresView.deleteTireConfirm'),
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteTire(vehicleStore.activeVehicle.id, tire.id)
    showHistoryModal.value = false
    selectedTireIds.value = selectedTireIds.value.filter((id) => id !== tire.id)
    await loadTires()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Mount sessions
function openAddSessionModal() {
  editingSessionId.value = null
  sessionInitialForm.value = emptySessionForm()
  showSessionModal.value = true
}

function openEditSessionModal(s: any) {
  editingSessionId.value = s.id
  sessionInitialForm.value = sessionFormFromSession(s)
  showSessionModal.value = true
}

async function onSessionSaved() {
  await openHistoryModal({ tire: selectedTire.value })
  await loadTires()
}

async function handleDeleteSession(session: any) {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  const ok = await showConfirm({
    title: t('tires.tiresView.deleteSessionTitle'),
    message: t('tires.tiresView.deleteSessionMessage'),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, session.id)
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  } catch (err: any) {
    showAlert(t('common.deleteError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Copy a session, paste it on another tire, or duplicate it onto several
function copySession(s: any) {
  copiedSession.value = copiedSessionFromSession(s)
}

function pasteSessionToCurrentTire() {
  if (!copiedSession.value) return
  editingSessionId.value = null
  sessionInitialForm.value = sessionFormFromCopy(copiedSession.value, selectedTire.value)
  showSessionModal.value = true
}

function openDuplicateSessionModal(s: any) {
  sessionToDuplicate.value = s
  showDuplicateSessionModal.value = true
}

// Tread depth measurements
function openLogModal(tire: any, log?: any) {
  selectedTire.value = tire.tire
  editingLogId.value = log ? log.id : null
  logInitialForm.value = log
    ? { depth_mm: log.depth_mm, odometer: Math.round(log.odometer), notes: log.notes || '', date: toIsoDay(log.date) }
    : {
        depth_mm: tire.current_depth_mm || 6.5,
        odometer: Math.round(vehicleStore.activeVehicle?.current_odometer || 0),
        notes: '',
        date: todayIso(),
      }
  showLogModal.value = true
}

function editLog(log: any) {
  openLogModal(selectedTireStats.value || { tire: selectedTire.value }, log)
}

async function onLogSaved() {
  await refreshAfterHistoryChange()
}

async function handleDeleteLog(l: any) {
  if (!vehicleStore.activeVehicle || !selectedTire.value) return
  const ok = await showConfirm({
    title: t('tires.tiresView.deleteReadingTitle'),
    message: t('tires.tiresView.deleteReadingMessage', { depth: l.depth_mm, date: formatDate(l.date) }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return

  try {
    await api.deleteTireLog(vehicleStore.activeVehicle.id, selectedTire.value.id, l.id)
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Actions -->
    <PageHeader :title="$t('tires.tiresView.tiresAndLifeCycles')" :icon="PageIcon">
      {{ $t('tires.tiresView.wearTrackingInMmEstimated') }}
      <template #actions>
      <!-- Action Buttons -->
      <div v-if="vehicleStore.canEdit" class="flex items-center gap-2 flex-wrap">
        <button
          @click="openPackSwapModal()"
          :class="{ 'opacity-60': storageTires.length === 0 }"
          class="btn btn-lg btn-secondary"
          :title="$t('tires.tiresView.swapTheFittedSetWith')"
        >
          <Snowflake class="w-4 h-4 text-info-400" />
          <span>{{ $t('tires.tiresView.changeSet') }}</span>
        </button>

        <button
          @click="openAddModal()"
          class="btn btn-lg btn-primary"
        >
          <Plus class="w-4 h-4" />
          {{ $t('tires.tiresView.addTires') }}
        </button>
      </div>
    
      </template>
    </PageHeader>

    <!-- Viewer mode banner -->
    <div
      v-if="!vehicleStore.canEdit"
      class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex items-center gap-3 text-xs text-slate-400"
    >
      <Disc class="w-4 h-4 text-slate-400 shrink-0" />
      <span>{{ $t('tires.tiresView.youAreViewingThisVehicle') }} <strong>{{ $t('tires.tiresView.readOnly') }}</strong>{{ $t('tires.tiresView.modeChangesToTiresRotations') }}</span>
    </div>

    <TabBar :model-value="activeTab" :tabs="tabs" :label="$t('tires.tiresView.tiresAndLifeCycles')" id-prefix="tires-tab" @update:model-value="activeTab = $event as 'chassis' | 'storage' | 'disposed'" />

    <!-- Sticky Bulk Selection Bar -->
    <BulkSelectionBar
      v-if="vehicleStore.canEdit"
      :count="selectedTireIds.length"
      noun="tire"
      @clear="selectedTireIds = []"
    >
      <button
        type="button"
        @click="openTireEdit(selectedTireIds)"
        class="btn btn-primary"
      >
        <Pencil class="w-3.5 h-3.5" />
        <span>{{ $t('tires.tiresView.editInBulk') }}</span>
      </button>

      <button
        v-if="canBatchDispose"
        type="button"
        @click="openBatchDisposeModal()"
        class="btn btn-warning"
        :title="$t('tires.tiresView.scrapTheSelectedTires')"
      >
        <Archive class="w-3.5 h-3.5" />
        <span>{{ $t('tires.tiresView.scrap') }}</span>
      </button>
    </BulkSelectionBar>

    <!-- Header row: Select all toggle & Total info -->
    <div v-if="vehicleStore.canEdit && activeTab === 'disposed' && currentTabTireIds.length > 0" class="flex items-center justify-between text-xs text-slate-400 px-2">
      <SelectAllToggle
        :checked="isCurrentTabAllSelected"
        :indeterminate="isCurrentTabPartlySelected"
        :label="isCurrentTabAllSelected ? $t('tires.tiresView.deselectAll') : $t('tires.tiresView.selectAll')"
        @toggle="toggleSelectAllCurrentTab"
      />
      <span>{{ $t('tires.tiresView.tireSInThisView', { length: currentTabTireIds.length }) }}</span>
    </div>

    <LoadError v-if="loadFailed" :message="loadError" @retry="loadTires" />
    <ListSkeleton v-else-if="!ready && vehicleId" :rows="3" />

    <!-- TAB 1: CHASSIS INTERACTIF (PNEUS MONTÉS) -->
    <div v-if="ready && activeTab === 'chassis'" class="space-y-6">
      <TireOdometerTimeline
        :tires="tires"
        :current-odometer="vehicleStore.activeVehicle?.current_odometer || 0"
        @select-tire="openTimelineTire"
      />
      <div v-if="vehicleStore.canEdit" class="bg-slate-900 border border-slate-800 p-3 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
        <div class="flex items-center gap-2 text-slate-300 font-semibold">
          <RefreshCw class="w-4 h-4 text-rose-400" />
          <span>{{ $t('tires.tiresView.quickVehicleRotationsIn1') }}</span>
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <button
            @click="handleQuickRotate('FRONT_BACK')"
            :disabled="rotating || !hasMountedTires"
            :title="$t('tires.tiresView.rotateTooltip', { pairs: 'FL ⇄ RL, FR ⇄ RR' })"
            class="tap btn btn-secondary"
          >
            <ArrowUpDown class="w-3.5 h-3.5 text-blue-400" />
            {{ $t('tires.tiresView.frontRear') }}
          </button>
          <button
            @click="handleQuickRotate('CROSS')"
            :disabled="rotating || !hasMountedTires"
            :title="$t('tires.tiresView.rotateTooltip', { pairs: 'FL ⇄ RR, FR ⇄ RL' })"
            class="tap btn btn-secondary"
          >
            <Shuffle class="w-3.5 h-3.5 text-indigo-400" />
            {{ $t('tires.tiresView.crossRotation') }}
          </button>
        </div>
      </div>

      <div v-if="vehicleStore.canEdit && currentTabTireIds.length > 0" class="flex items-center justify-between text-xs text-slate-400 px-2">
        <SelectAllToggle
          :checked="isCurrentTabAllSelected"
          :indeterminate="isCurrentTabPartlySelected"
          :label="isCurrentTabAllSelected ? $t('tires.tiresView.deselectAll') : $t('tires.tiresView.selectAll')"
          @toggle="toggleSelectAllCurrentTab"
        />
        <span>{{ $t('tires.tiresView.tireSInThisView', { length: currentTabTireIds.length }) }}</span>
      </div>

      <div v-if="tires.length === 0" class="bg-slate-900/60 border border-slate-800 rounded-3xl p-10 text-center text-slate-400 space-y-3">
        <Disc class="w-10 h-10 mx-auto text-slate-400" />
        <h3 class="text-base font-bold text-white">{{ $t('tires.tiresView.noTireYet') }}</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">{{ $t('tires.tiresView.noTireYetHint') }}</p>
        <button v-if="vehicleStore.canEdit" type="button" @click="openAddModal()" class="btn btn-lg btn-primary">
          {{ $t('tires.tiresView.addTires') }}
        </button>
      </div>

      <!-- Cartes des 4 roues -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <TireWheelCard
          v-for="w in wheels"
          :key="w.pos"
          :pos="w.pos"
          :label="$t(w.labelKey)"
          :stat="mountedTires[w.pos]"
          :selected="!!mountedTires[w.pos] && selectedTireIds.includes(mountedTires[w.pos].tire.id)"
          :can-mount="vehicleStore.canEdit"
          @open="openHistoryModal"
          @toggle="toggleTireSelection"
          @mount="mountOnEmptyWheel"
        />
      </div>
    </div>

    <!-- TAB 2: CATALOGUE & STOCK AU GARAGE -->
    <div v-if="ready && activeTab === 'storage'" class="space-y-4">
      <div v-if="storageTires.length === 0" class="bg-slate-900/60 border border-slate-800 rounded-3xl p-12 text-center text-slate-400 space-y-3">
        <Package class="w-10 h-10 mx-auto text-slate-400" />
        <h3 class="text-base font-bold text-white">{{ $t('tires.tiresView.noTireInGarageStorage') }}</h3>
        <p class="text-xs text-slate-400 max-w-sm mx-auto">
          {{ $t('tires.tiresView.youCanRecordYourWinter') }}
        </p>
      </div>

      <div v-else class="space-y-4">
        <!-- Garage batch actions bar -->
        <div class="flex items-center justify-between flex-wrap gap-2 bg-slate-900/60 border border-slate-800 p-3 rounded-2xl">
          <div class="flex items-center gap-3 px-1 text-xs text-slate-400">
            <SelectAllToggle
              v-if="vehicleStore.canEdit"
              :checked="isCurrentTabAllSelected"
              :indeterminate="isCurrentTabPartlySelected"
              :label="isCurrentTabAllSelected ? $t('tires.tiresView.deselectAll') : $t('tires.tiresView.selectAll')"
              @toggle="toggleSelectAllCurrentTab"
            />
            <span class="font-semibold text-slate-300">{{ $t('tires.tiresView.tireSInGarageStorage', { length: storageTires.length }) }}</span>
          </div>
          <div class="flex items-center gap-2 flex-wrap">
            <button
              v-if="vehicleStore.canEdit"
              @click="openCopyHistoryModal()"
              class="btn btn-secondary"
              :title="$t('tires.tiresView.copyATireSWhole')"
            >
              <Copy class="w-3.5 h-3.5 text-indigo-400" />
              <span>{{ $t('tires.tiresView.copyATireSHistory') }}</span>
            </button>
            <button
              v-if="vehicleStore.canEdit"
              @click="openBatchSessionModal()"
              class="btn btn-secondary"
              :title="$t('tires.tiresView.recordAPastSessionOn')"
            >
              <History class="w-3.5 h-3.5 text-rose-400" />
              <span>{{ $t('tires.tiresView.addAPastSessionOn') }}</span>
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <TireStorageCard
            v-for="t in storageTires"
            :key="t.tire.id"
            :t="t"
            :selected="selectedTireIds.includes(t.tire.id)"
            @open="openHistoryModal"
            @toggle="toggleTireSelection"
          />
      </div>
      </div>
    </div>

    <!-- TAB 3: PNEUS MIS AU REBUT -->
    <div v-if="ready && activeTab === 'disposed' && disposedTires.length === 0" class="bg-slate-900/60 border border-slate-800 rounded-3xl p-12 text-center text-slate-400 space-y-3">
      <Archive class="w-10 h-10 mx-auto text-slate-400" />
      <h3 class="text-base font-bold text-white">{{ $t('tires.tiresView.noScrappedTire') }}</h3>
      <p class="text-xs text-slate-400 max-w-sm mx-auto">{{ $t('tires.tiresView.noScrappedTireHint') }}</p>
    </div>

    <div v-if="ready && activeTab === 'disposed' && disposedTires.length > 0" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <TireDisposedCard
        v-for="t in disposedTires"
        :key="t.tire.id"
        :t="t"
        :selected="selectedTireIds.includes(t.tire.id)"
        :dismount="getLastDismountInfo(tires, t.tire.id)"
        @open="openHistoryModal"
        @toggle="toggleTireSelection"
      />
    </div>

    <TireAddModal
      v-model:open="showAddTireModal"
      :vehicle-id="vehicleId"
      :current-odometer="currentOdometer"
      @saved="loadTires"
    />

    <TireHistoryModal
      v-model:open="showHistoryModal"
      :selected-tire="selectedTire"
      :selected-tire-stats="selectedTireStats"
      :tire-sessions="tireSessions"
      :tire-logs="tireLogs"
      :copied-session="copiedSession"
      @edit-tire="openTireEdit([selectedTire.id])"
      @dispose-tire="showDisposeModal = true"
      @delete-tire="handleDeleteTire(selectedTire)"
      @paste-session="pasteSessionToCurrentTire"
      @add-session="openAddSessionModal"
      @copy-session="copySession"
      @duplicate-session="openDuplicateSessionModal"
      @edit-session="openEditSessionModal"
      @delete-session="handleDeleteSession"
      @add-log="openLogModal(selectedTireStats)"
      @edit-log="editLog"
      @delete-log="handleDeleteLog"
    />

    <TireSessionModal
      v-model:open="showSessionModal"
      :vehicle-id="vehicleId"
      :selected-tire="selectedTire"
      :copied-session="copiedSession"
      :editing-session-id="editingSessionId"
      :initial-form="sessionInitialForm"
      @saved="onSessionSaved"
    />

    <TireLogModal
      v-model:open="showLogModal"
      :vehicle-id="vehicleId"
      :selected-tire="selectedTire"
      :editing-log-id="editingLogId"
      :initial-form="logInitialForm"
      @saved="onLogSaved"
    />

    <TirePackSwapModal
      v-model:open="showPackSwapModal"
      :vehicle-id="vehicleId"
      :storage-tires="storageTires"
      :current-odometer="currentOdometer"
      @saved="loadTires"
    />

    <TireEditModal
      v-model:open="showTireEditModal"
      :vehicle-id="vehicleId"
      :tires="tires"
      :tire-ids="tireEditIds"
      :fallback-tire="selectedTire"
      :fallback-stats="selectedTireStats"
      @saved="onTireEdited"
    />

    <TireDisposeModal
      v-model:open="showDisposeModal"
      :vehicle-id="vehicleId"
      :selected-tire="selectedTire"
      :tires="tires"
      :current-odometer="currentOdometer"
      @saved="onTireDisposed"
    />

    <TireBatchSessionModal
      v-model:open="showBatchSessionModal"
      :vehicle-id="vehicleId"
      :storage-tires="storageTires"
      :selected-tire-ids="selectedTireIds"
      :current-odometer="currentOdometer"
      @saved="loadTires"
    />

    <TireDuplicateSessionModal
      v-model:open="showDuplicateSessionModal"
      :vehicle-id="vehicleId"
      :selected-tire="selectedTire"
      :session-to-duplicate="sessionToDuplicate"
      :tires="tires"
      @saved="loadTires"
    />

    <TireBatchDisposeModal
      v-model:open="showBatchDisposeModal"
      :vehicle-id="vehicleId"
      :selected-tire-ids="selectedTireIds"
      :tires="tires"
      :current-odometer="currentOdometer"
      @saved="onBatchDisposed"
    />

    <TireCopyHistoryModal
      v-model:open="showCopyHistoryModal"
      :vehicle-id="vehicleId"
      :tires="tires"
      :source="copyHistorySource"
      @saved="onHistoryCopied"
    />
  </div>
</template>
