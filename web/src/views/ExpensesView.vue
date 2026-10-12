<script setup lang="ts">
import LoadError from '@/components/LoadError.vue'
import TabBar from '@/components/TabBar.vue'
import PageHeader from '@/components/PageHeader.vue'
import { t } from '@/i18n'
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { useExpensesTabs, type ExpensesTab } from '@/composables/useExpensesTabs'
import { useExpenseDeletions } from '@/composables/useExpenseDeletions'
import { useDocumentPreview } from '@/composables/useDocumentPreview'
import { useReminders } from '@/composables/useReminders'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import DocumentPreviewModal from '@/components/expenses/DocumentPreviewModal.vue'
import TollsPanel from '@/components/expenses/TollsPanel.vue'
import MaintenancePanel from '@/components/expenses/MaintenancePanel.vue'
import RemindersPanel from '@/components/expenses/RemindersPanel.vue'
import ChargesPanel from '@/components/expenses/ChargesPanel.vue'
import FuelLogsPanel from '@/components/manual/FuelLogsPanel.vue'
import EnergyEfficiencyPanel from '@/components/dashboard/EnergyEfficiencyPanel.vue'
import EstimatedEnergyPanel from '@/components/manual/EstimatedEnergyPanel.vue'
import ExpensesHeaderActions from '@/components/expenses/ExpensesHeaderActions.vue'
import DocumentsPanel from '@/components/expenses/DocumentsPanel.vue'
import TollModal from '@/components/expenses/TollModal.vue'
import MaintenanceModal from '@/components/expenses/MaintenanceModal.vue'
import ChargeModal from '@/components/expenses/ChargeModal.vue'
import ChargingPlacesModal from '@/components/expenses/ChargingPlacesModal.vue'
import UploadDocumentModal from '@/components/expenses/UploadDocumentModal.vue'
import ReminderModal from '@/components/expenses/ReminderModal.vue'
import CompleteReminderModal from '@/components/expenses/CompleteReminderModal.vue'
import WebhookModal from '@/components/expenses/WebhookModal.vue'
import CSVImportModal from '@/components/CSVImportModal.vue'
import QualifyChargesModal from '@/components/expenses/QualifyChargesModal.vue'
import { Zap, Eye } from 'lucide-vue-next'

// The page owns the lists, the active tab and which modal is open; each modal owns its form and its API
// call and reports back with "saved".
const router = useRouter()
const route = useRoute()
const vehicleStore = useVehicleStore()
const { showAlert } = useConfirm()

const vehicleId = computed(() => vehicleStore.activeVehicle?.id ?? '')
const currentOdometer = computed(() => vehicleStore.activeVehicle?.current_odometer || 0)
const { previewDoc, loadingDocId, closeDocPreview, viewOrDownloadDocument } = useDocumentPreview(() => vehicleStore.activeVehicle?.id)

const { activeTab, pageTitle, pageSubtitle, pageIcon, tabs } = useExpensesTabs(() => ({
  urgentReminders: urgentRemindersCount.value,
  overdueReminders: overdueReminders.value.length,
  chargesWithoutCost: chargesWithoutCost.value,
  documents: documents.value.length,
}))

const driveExpenses = ref<any[]>([])
const maintenanceExpenses = ref<any[]>([])
const charges = ref<any[]>([])
const chargesWithoutCost = ref(0)
const chargesTotal = ref(0)
const chargesPage = ref(1)
const loadingMoreCharges = ref(false)
const missingCostOnly = ref(false)
const loading = ref(false)
const loadError = ref<string | null>(null)

// Maintenance reminders & webhook
const {
  reminders,
  loadingReminders,
  showReminderModal,
  editingReminder,
  reminderPreset,
  showCompleteReminderModal,
  completingReminder,
  showWebhookModal,
  vehicleWebhook,
  overdueReminders,
  dueSoonReminders,
  okReminders,
  unscheduledReminders,
  urgentRemindersCount,
  loadReminders,
  openAddReminderModal,
  openEditReminderModal,
  handleDeleteReminder,
  openCompleteReminder,
  onReminderCompleted,
  openWebhookModal,
} = useReminders({
  isRemindersTabActive: () => activeTab.value === 'REMINDERS',
  onLoadError: (message) => {
    loadError.value = message
  },
  onExpenseLogged: async () => {
    if (vehicleStore.activeVehicle) maintenanceExpenses.value = await api.getMaintenance(vehicleStore.activeVehicle.id)
  },
})

// Documents & Invoices
const documents = ref<ExpenseDocumentHeader[]>([])

// Modals, with the item being edited (null when adding)
const showAddTollModal = ref(false)
const editingToll = ref<any | null>(null)
const showAddMaintModal = ref(false)
const editingMaint = ref<any | null>(null)
const showChargeModal = ref(false)
const showPlacesModal = ref(false)
const editingCharge = ref<any | null>(null)
const showUploadDocModal = ref(false)

async function ensureDocumentsLoaded() {
  if (!vehicleStore.activeVehicle) return
  try {
    documents.value = await api.getDocuments(vehicleStore.activeVehicle.id)
  } catch (err) {
    console.error('Failed to load documents', err)
  }
}

function onDocumentAdded(doc: ExpenseDocumentHeader) {
  documents.value.unshift(doc)
}

const pendingChargesCount = ref(0)
const showQualifyModal = ref(false)

async function checkPendingCharges() {
  try {
    const res = await api.getPendingCharges()
    pendingChargesCount.value = res.pending_charges?.length || 0
  } catch {
    pendingChargesCount.value = 0
  }
}

async function onChargesAssigned() {
  await loadData()
}

// The fill-ups panel loads its own data when mounted: remounting it shows what an import added
const fuelReloadKey = ref(0)
function onImported() {
  fuelReloadKey.value++
  loadData()
}

async function loadData() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  loadError.value = null
  try {
    if (activeTab.value === 'TOLLS') {
      driveExpenses.value = await api.getDriveExpenses(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'MAINTENANCE' || activeTab.value === 'FIXED') {
      maintenanceExpenses.value = await api.getMaintenance(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'REMINDERS') {
      await loadReminders()
    } else if (activeTab.value === 'CHARGES' && vehicleStore.canCharge) {
      chargesPage.value = 1
      const res = await api.getCharges(vehicleStore.activeVehicle.id, { missingCost: missingCostOnly.value })
      charges.value = res.charges
      chargesTotal.value = res.total || 0
      chargesWithoutCost.value = res.charges_without_cost || 0
      checkPendingCharges()
    } else if (activeTab.value === 'DOCUMENTS') {
      documents.value = await api.getDocuments(vehicleStore.activeVehicle.id)
    }
    ensureDocumentsLoaded()
    if (activeTab.value !== 'REMINDERS') {
      loadReminders()
    }
  } catch (err) {
    console.error('Failed to load expenses', err)
    loadError.value = (err as Error)?.message ?? ''
  } finally {
    loading.value = false
  }
}

const { handleDeleteToll, handleDeleteMaint, handleDeleteCharge, handleDeleteDocument } = useExpenseDeletions(documents, loadData)

watch(
  () => [vehicleStore.activeVehicle?.id, activeTab.value, vehicleStore.lastSyncTimestamp],
  () => {
    loadData()
  }
)

// The quick entry sheet sends maintenance entries here: the form opens on arrival and the parameter is dropped.
watch(
  () => [route.query.add, vehicleStore.canEdit] as const,
  ([add, canEdit]) => {
    if (add !== 'maintenance') return
    const { add: _drop, ...rest } = route.query
    router.replace({ query: rest })
    // Once mounted, so the modal sees its `open` go from false to true and loads its defaults
    if (canEdit) nextTick(openAddMaintModal)
  },
  { immediate: true },
)

onMounted(() => {
  loadData()
  loadReminders()
})

// Tolls & parking
function openAddTollModal() {
  editingToll.value = null
  showAddTollModal.value = true
  ensureDocumentsLoaded()
}

function openEditTollModal(e: any) {
  editingToll.value = e
  showAddTollModal.value = true
  ensureDocumentsLoaded()
}

// Maintenance & fixed expenses
const newExpenseCategory = ref('MAINTENANCE')
function openAddMaintModal() {
  editingMaint.value = null
  newExpenseCategory.value = activeTab.value === 'FIXED' ? 'INSURANCE' : 'MAINTENANCE'
  showAddMaintModal.value = true
  ensureDocumentsLoaded()
}

function openEditMaintModal(m: any) {
  editingMaint.value = m
  showAddMaintModal.value = true
  ensureDocumentsLoaded()
}

// Charges: manual entry and cost completion
// Next page of charges, so that older charges can also be completed or corrected
async function loadMoreCharges() {
  if (!vehicleStore.activeVehicle) return
  loadingMoreCharges.value = true
  try {
    const res = await api.getCharges(vehicleStore.activeVehicle.id, { page: chargesPage.value + 1, missingCost: missingCostOnly.value })
    chargesPage.value += 1
    charges.value = [...charges.value, ...res.charges]
    chargesTotal.value = res.total || 0
  } catch (err: any) {
    void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    loadingMoreCharges.value = false
  }
}

function toggleMissingCostFilter() {
  missingCostOnly.value = !missingCostOnly.value
  loadData()
}

const showCSVImportModal = ref(false)

function openCSVImportModal() {
  showCSVImportModal.value = true
}

function openAddChargeModal() {
  editingCharge.value = null
  showChargeModal.value = true
  ensureDocumentsLoaded()
}

function openEditChargeModal(c: any) {
  editingCharge.value = c
  showChargeModal.value = true
  ensureDocumentsLoaded()
}

// Documents
function openUploadDocumentModal() {
  showUploadDocModal.value = true
}

</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <PageHeader :title="pageTitle" :icon="pageIcon">
      {{ pageSubtitle }}
      <template #actions>
        <ExpensesHeaderActions
          :active-tab="activeTab"
          @add-toll="openAddTollModal"
          @add-maintenance="openAddMaintModal"
          @open-webhook="openWebhookModal"
          @add-reminder="openAddReminderModal()"
          @import-csv="openCSVImportModal"
          @add-charge="openAddChargeModal"
          @upload-document="openUploadDocumentModal"
        />
      </template>
    </PageHeader>

    <!-- Viewer mode banner -->
    <div
      v-if="!vehicleStore.canEdit"
      class="bg-slate-900 border border-slate-800 p-3.5 rounded-2xl flex items-center gap-3 text-xs text-slate-400"
    >
      <Eye class="w-4 h-4 text-slate-400 shrink-0" />
      <span>{{ $t('expenses.expensesView.youAreViewingThisVehicle') }} <strong>{{ $t('expenses.expensesView.readOnly') }}</strong>{{ $t('expenses.expensesView.modeAdditionsAndChangesAre') }}</span>
    </div>

    <TabBar :model-value="activeTab" :tabs="tabs" :label="pageTitle" id-prefix="expenses-tab" @update:model-value="activeTab = $event as ExpensesTab" />

    <LoadError v-if="loadError !== null" :message="loadError" @retry="loadData" />

    <!-- Content -->
    <TollsPanel
      v-if="activeTab === 'TOLLS'"
      :drive-expenses="driveExpenses"
      :loading="loading"
      @add="openAddTollModal"
      @edit="openEditTollModal"
      @delete="handleDeleteToll"
      @view-document="viewOrDownloadDocument"
    />

    <MaintenancePanel
      v-if="activeTab === 'MAINTENANCE' || activeTab === 'FIXED'"
      :kind="activeTab === 'FIXED' ? 'fixed' : 'service'"
      :vehicle-id="vehicleId"
      :maintenance-expenses="maintenanceExpenses"
      :loading="loading"
      @add="openAddMaintModal"
      @edit="openEditMaintModal"
      @delete="handleDeleteMaint"
      @view-document="viewOrDownloadDocument"
    />

    <RemindersPanel
      v-if="activeTab === 'REMINDERS'"
      :reminders="reminders"
      :overdue-reminders="overdueReminders"
      :due-soon-reminders="dueSoonReminders"
      :ok-reminders="okReminders"
      :unscheduled-reminders="unscheduledReminders"
      :loading-reminders="loadingReminders"
      :vehicle-webhook="vehicleWebhook"
      @open-webhook="openWebhookModal"
      @add="openAddReminderModal"
      @edit="openEditReminderModal"
      @complete="openCompleteReminder"
      @delete="handleDeleteReminder"
      @reload="loadReminders"
    />

    <div
      v-if="activeTab === 'CHARGES' && vehicleStore.canCharge && pendingChargesCount > 0"
      class="p-4 bg-warning-500/10 border border-warning-500/30 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs"
    >
      <div class="flex items-center gap-2.5 text-warning-300">
        <Zap class="w-4 h-4 text-warning-400 shrink-0" />
        <span>
          <strong>{{ pendingChargesCount }} {{ $t('pendingCharges.bannerCount', { count: pendingChargesCount }) }}</strong>
          — {{ $t('pendingCharges.bannerDescription') }}
        </span>
      </div>
      <button
        @click="showQualifyModal = true"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-warning-600 hover:bg-warning-500 text-white font-semibold rounded-xl transition-colors shrink-0"
      >
        {{ $t('pendingCharges.qualifyButton') }}
      </button>
    </div>

    <EnergyEfficiencyPanel
      v-if="activeTab === 'EFFICIENCY' && vehicleStore.activeVehicle"
      :vehicle-id="vehicleStore.activeVehicle.id"
      :grafana-url="vehicleStore.activeVehicle.teslamate_grafana_url"
      :sync-key="vehicleStore.lastSyncTimestamp"
      show-empty
    />

    <EstimatedEnergyPanel
      v-if="activeTab === 'ESTIMATE' && vehicleStore.activeVehicle"
      :vehicle="vehicleStore.activeVehicle"
      :can-edit="vehicleStore.canEdit"
    />

    <FuelLogsPanel
      v-if="activeTab === 'FUEL' && vehicleStore.canRefuel && vehicleStore.activeVehicle"
      :key="fuelReloadKey"
      :vehicle-id="vehicleStore.activeVehicle.id"
      :can-edit="vehicleStore.canEdit"
    />

    <ChargesPanel
      v-if="activeTab === 'CHARGES' && vehicleStore.canCharge"
      :charges="charges"
      :charges-total="chargesTotal"
      :charges-without-cost="chargesWithoutCost"
      :loading="loading"
      :loading-more-charges="loadingMoreCharges"
      :missing-cost-only="missingCostOnly"
      @toggle-missing-cost="toggleMissingCostFilter"
      @import-csv="openCSVImportModal"
      @manage-places="showPlacesModal = true"
      @load-more="loadMoreCharges"
      @edit="openEditChargeModal"
      @delete="handleDeleteCharge"
      @view-document="viewOrDownloadDocument"
    />

    <DocumentsPanel
      v-if="activeTab === 'DOCUMENTS'"
      :documents="documents"
      :loading="loading"
      :loading-doc-id="loadingDocId"
      @upload="openUploadDocumentModal"
      @delete="handleDeleteDocument"
      @view-document="viewOrDownloadDocument"
    />

    <TollModal
      v-model:open="showAddTollModal"
      :vehicle-id="vehicleId"
      :editing="editingToll"
      :documents="documents"
      @saved="loadData"
      @document-added="onDocumentAdded"
      @view-document="viewOrDownloadDocument"
    />

    <MaintenanceModal
      v-model:open="showAddMaintModal"
      :vehicle-id="vehicleId"
      :editing="editingMaint"
      :documents="documents"
      :maintenance-expenses="maintenanceExpenses"
      :current-odometer="currentOdometer"
      :default-category="newExpenseCategory"
      @saved="loadData"
      @document-added="onDocumentAdded"
      @view-document="viewOrDownloadDocument"
    />

    <ChargingPlacesModal v-model:open="showPlacesModal" :vehicle-id="vehicleId" :can-edit="vehicleStore.canEdit" @changed="loadData" />

    <ChargeModal
      v-model:open="showChargeModal"
      :vehicle-id="vehicleId"
      :editing="editingCharge"
      :documents="documents"
      :current-odometer="currentOdometer"
      @saved="loadData"
      @document-added="onDocumentAdded"
      @view-document="viewOrDownloadDocument"
    />

    <UploadDocumentModal
      v-model:open="showUploadDocModal"
      :vehicle-id="vehicleId"
      @document-added="onDocumentAdded"
    />

    <ReminderModal
      v-model:open="showReminderModal"
      :vehicle-id="vehicleId"
      :editing="editingReminder"
      :preset="reminderPreset"
      :current-odometer="currentOdometer"
      @saved="loadReminders"
    />

    <CompleteReminderModal
      v-model:open="showCompleteReminderModal"
      :vehicle-id="vehicleId"
      :reminder="completingReminder"
      :current-odometer="currentOdometer"
      @saved="onReminderCompleted"
    />

    <WebhookModal
      v-model:open="showWebhookModal"
      :vehicle-id="vehicleId"
      v-model:webhook="vehicleWebhook"
    />

    <!-- Modal: Document In-App Preview (Modular Component) -->
    <DocumentPreviewModal :preview-doc="previewDoc" @close="closeDocPreview" />

    <CSVImportModal
      v-model:open="showCSVImportModal"
      :vehicle-id="vehicleId"
      :default-type="activeTab === 'FUEL' ? 'FUEL' : 'CHARGES'"
      @imported="onImported"
    />

    <QualifyChargesModal
      v-model:open="showQualifyModal"
      @assigned="onChargesAssigned"
    />
  </div>
</template>
