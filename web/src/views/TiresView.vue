<script setup lang="ts">
import TabBar, { type TabItem } from '@/components/TabBar.vue'
import PageHeader from '@/components/PageHeader.vue'
import { useTireHistory } from '@/composables/useTireHistory'
import { useTireList } from '@/composables/useTireList'
import { useTireSelection } from '@/composables/useTireSelection'
import LoadError from '@/components/LoadError.vue'
import ListSkeleton from '@/components/ListSkeleton.vue'
import { Disc as PageIcon } from 'lucide-vue-next'
import { t } from '@/i18n'
import { ref, onMounted, computed, watch } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import SelectAllToggle from '@/components/SelectAllToggle.vue'
import TireQuickRotationBar from '@/components/tires/TireQuickRotationBar.vue'
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
import { Archive, Copy, Disc, History, Package, Pencil, Plus, Snowflake } from 'lucide-vue-next'
import { getLastDismountInfo } from '@/utils/tires'

// The page owns the tire list, the selection and which modal is open; each modal owns its form and
// its API call and reports back with "saved".
const vehicleStore = useVehicleStore()
const { showAlert } = useConfirm()
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
const showBatchSessionModal = ref(false)
const showCopyHistoryModal = ref(false)
const showBatchDisposeModal = ref(false)
const showPackSwapModal = ref(false)
const showTireEditModal = ref(false)
const showDisposeModal = ref(false)

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

const {
  showHistoryModal,
  showSessionModal,
  showDuplicateSessionModal,
  showLogModal,
  selectedTire,
  selectedTireStats,
  tireSessions,
  tireLogs,
  editingSessionId,
  sessionInitialForm,
  copiedSession,
  sessionToDuplicate,
  editingLogId,
  logInitialForm,
  openHistoryModal,
  openTimelineTire,
  refreshAfterHistoryChange,
  reloadHistoryAndList,
  onTireDisposed,
  handleDeleteTire,
  openAddSessionModal,
  openEditSessionModal,
  handleDeleteSession,
  copySession,
  pasteSessionToCurrentTire,
  openDuplicateSessionModal,
  openLogModal,
  editLog,
  handleDeleteLog,
} = useTireHistory({ tires, loadTires, selectedTireIds })

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

// The odometer the vehicle was acquired at: the timeline starts there rather than at 0 km. Best effort, the
// timeline falls back to 0 when the vehicle has no ownership record.
const startOdometer = ref<number | null>(null)
async function loadStartOdometer() {
  const id = vehicleStore.activeVehicle?.id
  startOdometer.value = null
  if (!id) return
  try {
    const ownership = await api.getOwnership(id)
    if (vehicleStore.activeVehicle?.id === id) startOdometer.value = Number(ownership?.start_odometer) || null
  } catch {
    // no ownership record: the axis starts at 0
  }
}
watch(() => vehicleStore.activeVehicle?.id, loadStartOdometer)

onMounted(() => {
  loadTires()
  loadStartOdometer()
})

function openPackSwapModal() {
  if (!vehicleStore.activeVehicle) return
  if (storageTires.value.length === 0) {
    void showAlert(t('tires.tiresView.changeSetNeedsStorage'), t('tires.tiresView.changeSet'), 'info')
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

// Edit one or several tires
function openTireEdit(ids: string[]) {
  tireEditIds.value = [...ids]
  showTireEditModal.value = true
}

async function onTireEdited() {
  selectedTireIds.value = []
  await refreshAfterHistoryChange()
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

async function onSessionSaved() {
  await reloadHistoryAndList()
}

async function onLogSaved() {
  await refreshAfterHistoryChange()
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Actions -->
    <PageHeader :title="$t('shell.nav.tires')" :icon="PageIcon">
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

    <TabBar :model-value="activeTab" :tabs="tabs" :label="$t('shell.nav.tires')" id-prefix="tires-tab" @update:model-value="activeTab = $event as 'chassis' | 'storage' | 'disposed'" />

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
        :start-odometer="startOdometer"
        @select-tire="openTimelineTire"
      />
      <TireQuickRotationBar v-if="vehicleStore.canEdit" :has-mounted-tires="hasMountedTires" @rotated="loadTires" />

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
              <Copy class="w-3.5 h-3.5 text-sky-400" />
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
