<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import { Receipt as PageIcon } from 'lucide-vue-next'
import { t } from '@/i18n'
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { useDocumentPreview } from '@/composables/useDocumentPreview'
import { api, type ExpenseDocumentHeader, type MaintenanceReminder, type VehicleWebhook } from '@/services/api'
import DocumentPreviewModal from '@/components/expenses/DocumentPreviewModal.vue'
import TollsPanel from '@/components/expenses/TollsPanel.vue'
import MaintenancePanel from '@/components/expenses/MaintenancePanel.vue'
import RemindersPanel from '@/components/expenses/RemindersPanel.vue'
import ChargesPanel from '@/components/expenses/ChargesPanel.vue'
import DocumentsPanel from '@/components/expenses/DocumentsPanel.vue'
import TollModal from '@/components/expenses/TollModal.vue'
import MaintenanceModal from '@/components/expenses/MaintenanceModal.vue'
import ChargeModal from '@/components/expenses/ChargeModal.vue'
import UploadDocumentModal from '@/components/expenses/UploadDocumentModal.vue'
import ReminderModal from '@/components/expenses/ReminderModal.vue'
import CompleteReminderModal from '@/components/expenses/CompleteReminderModal.vue'
import WebhookModal from '@/components/expenses/WebhookModal.vue'
import CSVImportModal from '@/components/CSVImportModal.vue'
import QualifyChargesModal from '@/components/expenses/QualifyChargesModal.vue'
import { Receipt, Plus, Wrench, Zap, Navigation, Paperclip, Eye, Bell, Radio, UploadCloud } from 'lucide-vue-next'
import type { ReminderPreset } from '@/utils/expenses'
import { formatAmount } from '@/currency'

// The page owns the lists, the active tab and which modal is open; each modal owns its form and its API
// call and reports back with "saved".
const router = useRouter()
const route = useRoute()
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()

const vehicleId = computed(() => vehicleStore.activeVehicle?.id ?? '')
const currentOdometer = computed(() => vehicleStore.activeVehicle?.current_odometer || 0)
const { previewDoc, loadingDocId, closeDocPreview, viewOrDownloadDocument } = useDocumentPreview(() => vehicleStore.activeVehicle?.id)

const validTabs = ['TOLLS', 'MAINTENANCE', 'REMINDERS', 'CHARGES', 'DOCUMENTS'] as const
type TabType = typeof validTabs[number]

const initialTab = (route.query.tab as string)?.toUpperCase()
const activeTab = ref<TabType>(validTabs.includes(initialTab as TabType) ? (initialTab as TabType) : 'TOLLS')

// A fuel-only vehicle has no charges: its fill-ups live in the manual tracking page
watch([() => vehicleStore.canCharge, activeTab], ([charges, tab]) => {
  if (!charges && tab === 'CHARGES') router.replace('/manual?tab=FUEL')
}, { immediate: true })

watch(activeTab, (newTab) => {
  if (route.query.tab !== newTab) {
    router.replace({ query: { ...route.query, tab: newTab } })
  }
})

watch(() => route.query.tab, (qTab) => {
  const upper = (qTab as string)?.toUpperCase()
  if (upper && validTabs.includes(upper as TabType) && activeTab.value !== upper) {
    activeTab.value = upper as TabType
  }
})

const driveExpenses = ref<any[]>([])
const maintenanceExpenses = ref<any[]>([])
const charges = ref<any[]>([])
const chargesWithoutCost = ref(0)
const chargesTotal = ref(0)
const chargesPage = ref(1)
const loadingMoreCharges = ref(false)
const missingCostOnly = ref(false)
const loading = ref(false)

// Documents & Invoices
const documents = ref<ExpenseDocumentHeader[]>([])

// Modals, with the item being edited (null when adding)
const showAddTollModal = ref(false)
const editingToll = ref<any | null>(null)
const showAddMaintModal = ref(false)
const editingMaint = ref<any | null>(null)
const showChargeModal = ref(false)
const editingCharge = ref<any | null>(null)
const showUploadDocModal = ref(false)

// Maintenance reminders & webhook
const reminders = ref<MaintenanceReminder[]>([])
const loadingReminders = ref(false)
const showReminderModal = ref(false)
const editingReminder = ref<MaintenanceReminder | null>(null)
const reminderPreset = ref<ReminderPreset | null>(null)
const showCompleteReminderModal = ref(false)
const completingReminder = ref<MaintenanceReminder | null>(null)
const showWebhookModal = ref(false)
const vehicleWebhook = ref<VehicleWebhook | null>(null)

const overdueReminders = computed(() => reminders.value.filter((r) => r.status === 'OVERDUE'))
const dueSoonReminders = computed(() => reminders.value.filter((r) => r.status === 'DUE_SOON'))
const okReminders = computed(() => reminders.value.filter((r) => r.status === 'OK'))
const urgentRemindersCount = computed(() => overdueReminders.value.length + dueSoonReminders.value.length)

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

async function loadReminders() {
  if (!vehicleStore.activeVehicle) return
  loadingReminders.value = true
  try {
    reminders.value = await api.getReminders(vehicleStore.activeVehicle.id)
  } catch (err: any) {
    console.error('Failed to load reminders', err)
  } finally {
    loadingReminders.value = false
  }
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

async function loadData() {
  if (!vehicleStore.activeVehicle) return
  loading.value = true
  try {
    if (activeTab.value === 'TOLLS') {
      driveExpenses.value = await api.getDriveExpenses(vehicleStore.activeVehicle.id)
    } else if (activeTab.value === 'MAINTENANCE') {
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
  } finally {
    loading.value = false
  }
}

watch(
  () => route.query.tab,
  (tab) => {
    if (tab && ['TOLLS', 'MAINTENANCE', 'REMINDERS', 'CHARGES', 'DOCUMENTS'].includes(String(tab))) {
      activeTab.value = tab as any
    }
  }
)

watch(
  () => [vehicleStore.activeVehicle?.id, activeTab.value, vehicleStore.lastSyncTimestamp],
  () => {
    loadData()
  }
)

onMounted(() => {
  if (route.query.tab && ['TOLLS', 'MAINTENANCE', 'REMINDERS', 'CHARGES', 'DOCUMENTS'].includes(String(route.query.tab))) {
    activeTab.value = route.query.tab as any
  }
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

async function handleDeleteToll(e: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('expenses.expensesView.deleteExpenseTitle'),
    message: t('expenses.expensesView.deleteTollMessage', { amount: formatAmount(Number(e.amount), e.currency || vehicleStore.currency) }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDriveExpense(vehicleStore.activeVehicle.id, e.id)
    await loadData()
  } catch (err: any) {
    showAlert(t('common.deleteError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Maintenance & fixed expenses
function openAddMaintModal() {
  editingMaint.value = null
  showAddMaintModal.value = true
  ensureDocumentsLoaded()
}

function openEditMaintModal(m: any) {
  editingMaint.value = m
  showAddMaintModal.value = true
  ensureDocumentsLoaded()
}

async function handleDeleteMaint(m: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('expenses.expensesView.deleteExpenseTitle'),
    message: t('expenses.expensesView.deleteExpenseMessage', { description: m.description, amount: formatAmount(Number(m.amount), m.currency || vehicleStore.currency) }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteMaintenance(vehicleStore.activeVehicle.id, m.id)
    await loadData()
  } catch (err: any) {
    showAlert(t('common.deleteError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
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
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
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

async function handleDeleteCharge(c: any) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('expenses.expensesView.deleteChargeTitle'),
    message: t('expenses.expensesView.deleteChargeMessage', { kwh: c.kwh_added }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteCharge(vehicleStore.activeVehicle.id, c.id)
    await loadData()
  } catch (err: any) {
    showAlert(t('common.deleteError', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Documents
function openUploadDocumentModal() {
  showUploadDocModal.value = true
}

async function handleDeleteDocument(doc: ExpenseDocumentHeader) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('expenses.expensesView.deleteReceiptTitle'),
    message: t('expenses.expensesView.deleteReceiptMessage', { filename: doc.filename }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDocument(vehicleStore.activeVehicle.id, doc.id)
    documents.value = documents.value.filter((d) => d.id !== doc.id)
    showAlert(t('expenses.expensesView.receiptDeleted'), t('common.success'), 'success')
    await loadData()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

// Maintenance reminders & webhook
function openAddReminderModal(preset: ReminderPreset | null = null) {
  editingReminder.value = null
  reminderPreset.value = preset
  showReminderModal.value = true
}

function openEditReminderModal(r: MaintenanceReminder) {
  editingReminder.value = r
  reminderPreset.value = null
  showReminderModal.value = true
}

async function handleDeleteReminder(r: MaintenanceReminder) {
  if (!vehicleStore.activeVehicle) return
  const ok = await showConfirm({
    title: t('expenses.expensesView.deleteReminderTitle'),
    message: t('expenses.expensesView.deleteReminderMessage', { title: r.title }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteReminder(vehicleStore.activeVehicle.id, r.id)
    showAlert(t('expenses.expensesView.reminderDeleted'), t('common.success'), 'success')
    await loadReminders()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

function openCompleteReminder(r: MaintenanceReminder) {
  completingReminder.value = r
  showCompleteReminderModal.value = true
}

async function onReminderCompleted(expenseLogged: boolean) {
  await loadReminders()
  if (expenseLogged && vehicleStore.activeVehicle) {
    maintenanceExpenses.value = await api.getMaintenance(vehicleStore.activeVehicle.id)
  }
}

// The webhook is fetched before the modal opens, so it shows the saved configuration
async function openWebhookModal() {
  if (!vehicleStore.activeVehicle) return
  try {
    vehicleWebhook.value = await api.getVehicleWebhook(vehicleStore.activeVehicle.id)
  } catch (err: any) {
    console.error('Failed to load webhook', err)
  }
  showWebhookModal.value = true
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <PageHeader :title="$t('expenses.expensesView.expensesAndMaintenance')" :icon="PageIcon">
      {{ $t('expenses.expensesView.tollsParkingRecurringMaintenanceInsurance') }}
      <template #actions>
      <div v-if="vehicleStore.canEdit" class="flex items-center gap-2">
        <button
          v-if="activeTab === 'TOLLS'"
          @click="openAddTollModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          {{ $t('expenses.expensesView.tollParking') }}
        </button>
        <button
          v-if="activeTab === 'MAINTENANCE'"
          @click="openAddMaintModal"
          class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          {{ $t('expenses.expensesView.maintenanceFixed') }}
        </button>
        <button
          v-if="activeTab === 'REMINDERS'"
          @click="openWebhookModal"
          class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-2 border border-slate-700 transition-colors"
          :title="$t('expenses.expensesView.setUpTheWebhookTo')"
        >
          <Radio class="w-3.5 h-3.5 text-violet-400" />
          <span class="hidden sm:inline">{{ $t('expenses.expensesView.homelabWebhook') }}</span>
          <span class="sm:hidden">{{ $t('expenses.expensesView.webhook') }}</span>
        </button>
        <button
          v-if="activeTab === 'REMINDERS'"
          @click="openAddReminderModal()"
          class="px-3.5 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-violet-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          {{ $t('expenses.expensesView.newReminder') }}
        </button>
        <template v-if="activeTab === 'CHARGES' && vehicleStore.canCharge">
          <button
            @click="openCSVImportModal"
            class="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-2 border border-slate-700 transition-colors"
          >
            <UploadCloud class="w-3.5 h-3.5 text-sky-400" />
            <span>{{ $t('expenses.expensesView.importCsv') }}</span>
          </button>
          <button
            @click="openAddChargeModal"
            class="px-3.5 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/20"
          >
            <Plus class="w-3.5 h-3.5" />
            {{ $t('expenses.expensesView.addCharge') }}
          </button>
        </template>
        <button
          v-if="activeTab === 'DOCUMENTS'"
          @click="openUploadDocumentModal"
          class="px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl flex items-center gap-2 shadow-lg shadow-indigo-600/20"
        >
          <Plus class="w-3.5 h-3.5" />
          {{ $t('expenses.expensesView.addAReceipt') }}
        </button>
      </div>
    
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

    <!-- Segmented Navigation: Frais de Route vs Flotte & Entretien -->
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 pb-3">
      <div class="flex min-w-0 max-w-full flex-wrap items-center gap-3">
        <!-- Groupe 1: Route & Trajets -->
        <div class="flex max-w-full items-center overflow-x-auto bg-slate-900 border border-slate-800 rounded-2xl p-1 gap-1">
          <span class="text-xs font-bold uppercase tracking-wider text-slate-400 px-2.5 py-1 select-none flex items-center gap-1.5">
            <Navigation class="w-3 h-3 text-amber-400" />
            <span class="hidden sm:inline">{{ $t('expenses.expensesView.roadAndDrives') }}</span>
          </span>
          <button
            @click="activeTab = 'TOLLS'"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors shrink-0"
            :class="activeTab === 'TOLLS' ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30 shadow-sm' : 'text-slate-400 hover:text-white'"
          >
            <Receipt class="w-3.5 h-3.5" />
            <span>{{ $t('expenses.expensesView.tolls') }}</span>
          </button>
          <button
            v-if="vehicleStore.canCharge"
            @click="activeTab = 'CHARGES'"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors shrink-0"
            :class="activeTab === 'CHARGES' ? 'bg-sky-500/20 text-sky-300 border border-sky-500/30 shadow-sm' : 'text-slate-400 hover:text-white'"
          >
            <Zap class="w-3.5 h-3.5" />
            <span>{{ $t('expenses.expensesView.charges') }}</span>
            <span v-if="chargesWithoutCost > 0" class="px-1.5 py-0.2 text-xs font-bold rounded-full bg-amber-500/20 text-amber-300 border border-amber-500/30">
              {{ chargesWithoutCost }}
            </span>
          </button>
        </div>

        <!-- Groupe 2: Flotte & Entretien -->
        <div class="flex max-w-full items-center overflow-x-auto bg-slate-900 border border-slate-800 rounded-2xl p-1 gap-1">
          <span class="text-xs font-bold uppercase tracking-wider text-slate-400 px-2.5 py-1 select-none flex items-center gap-1.5">
            <Wrench class="w-3 h-3 text-pink-400" />
            <span class="hidden sm:inline">{{ $t('expenses.expensesView.fleetAndVehicle') }}</span>
          </span>
          <button
            @click="activeTab = 'MAINTENANCE'"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors shrink-0"
            :class="activeTab === 'MAINTENANCE' ? 'bg-pink-500/20 text-pink-300 border border-pink-500/30 shadow-sm' : 'text-slate-400 hover:text-white'"
          >
            <Wrench class="w-3.5 h-3.5" />
            <span>{{ $t('expenses.expensesView.maintenance') }}</span>
          </button>
          <button
            @click="activeTab = 'REMINDERS'"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors shrink-0 relative"
            :class="activeTab === 'REMINDERS' ? 'bg-violet-500/20 text-violet-300 border border-violet-500/30 shadow-sm' : 'text-slate-400 hover:text-white'"
          >
            <Bell class="w-3.5 h-3.5" />
            <span>{{ $t('expenses.expensesView.reminders') }}</span>
            <span
              v-if="urgentRemindersCount > 0"
              class="px-1.5 py-0.2 text-xs font-bold rounded-full"
              :class="overdueReminders.length > 0 ? 'bg-rose-500 text-white' : 'bg-amber-500 text-slate-950'"
            >
              {{ urgentRemindersCount }}
            </span>
          </button>
          <button
            @click="activeTab = 'DOCUMENTS'"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors shrink-0"
            :class="activeTab === 'DOCUMENTS' ? 'bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 shadow-sm' : 'text-slate-400 hover:text-white'"
          >
            <Paperclip class="w-3.5 h-3.5" />
            <span>{{ $t('expenses.expensesView.receipts') }}</span>
            <span v-if="documents.length > 0" class="px-1.5 py-0.2 text-xs font-medium rounded-full bg-slate-800 text-slate-400">
              {{ documents.length }}
            </span>
          </button>
        </div>
      </div>
    </div>

    <!-- Content -->
    <TollsPanel
      v-if="activeTab === 'TOLLS'"
      :drive-expenses="driveExpenses"
      :loading="loading"
      @edit="openEditTollModal"
      @delete="handleDeleteToll"
      @view-document="viewOrDownloadDocument"
    />

    <MaintenancePanel
      v-if="activeTab === 'MAINTENANCE'"
      :maintenance-expenses="maintenanceExpenses"
      :loading="loading"
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
      class="p-4 bg-amber-500/10 border border-amber-500/30 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs"
    >
      <div class="flex items-center gap-2.5 text-amber-300">
        <Zap class="w-4 h-4 text-amber-400 shrink-0" />
        <span>
          <strong>{{ pendingChargesCount }} {{ $t('pendingCharges.bannerCount', { count: pendingChargesCount }) }}</strong>
          — {{ $t('pendingCharges.bannerDescription') }}
        </span>
      </div>
      <button
        @click="showQualifyModal = true"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-amber-600 hover:bg-amber-500 text-white font-semibold rounded-xl transition-colors shrink-0"
      >
        {{ $t('pendingCharges.qualifyButton') }}
      </button>
    </div>

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
      @saved="loadData"
      @document-added="onDocumentAdded"
      @view-document="viewOrDownloadDocument"
    />

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
      default-type="CHARGES"
      @imported="loadData"
    />

    <QualifyChargesModal
      v-model:open="showQualifyModal"
      @assigned="onChargesAssigned"
    />
  </div>
</template>
