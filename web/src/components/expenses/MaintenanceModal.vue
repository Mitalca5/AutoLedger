<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, nextTick, ref, watch } from 'vue'
import { api, type ExpenseDocumentHeader } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useDocumentAttach } from '@/composables/useDocumentAttach'
import { useVehicleStore } from '@/stores/vehicle'
import { currencySymbol, formatAmount } from '@/currency'
import { ChevronDown, Wrench, X, Paperclip, FileText, Eye } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import AppDropzone from '@/components/AppDropzone.vue'
import { CURRENCIES, currencyPayload, defaultAmortizationMode, findCloseCandidate, formatDate, recentDescriptions } from '@/utils/expenses'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistanceValue } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

// Adds a maintenance / fixed expense, or edits it when `editing` is set. maintenanceExpenses lets a new one close an earlier revision.
const props = defineProps<{
  vehicleId: string
  editing: any | null
  documents: ExpenseDocumentHeader[]
  maintenanceExpenses: any[]
  currentOdometer?: number
  defaultCategory?: string
}>()
const emit = defineEmits<{
  saved: []
  'document-added': [doc: ExpenseDocumentHeader]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const { isUploadingDocument, onSelectExistingDoc, onDropzoneDirectUpload } = useDocumentAttach(
  () => props.vehicleId,
  () => props.documents,
  (doc) => emit('document-added', doc)
)

const editingMaintId = computed(() => props.editing?.id ?? null)
const vehicleStore = useVehicleStore()
const baseCurrency = computed(() => vehicleStore.currency)

const insuranceAnnualPremium = ref<number | ''>('')

// Annual premium paid monthly: amount = premium / 12, recurring every month
function applyMonthlyPremium() {
  const annual = Number(insuranceAnnualPremium.value)
  if (!annual || annual <= 0) {
    showAlert(t('expenses.maintenanceModal.enterAnnualPremium'), t('common.requiredField'), 'warning')
    return
  }
  maintForm.value.amount = (Math.round((annual / 12) * 100) / 100).toFixed(2)
  maintForm.value.is_recurring = true
  maintForm.value.recurrence_interval_months = 1
  if (!maintForm.value.description) {
    maintForm.value.description = t('expenses.maintenanceModal.premiumDescription', { amount: formatAmount(annual, baseCurrency.value) })
  }
}

const maintForm = ref({
  category: 'MAINTENANCE',
  amount: '',
  currency: baseCurrency.value,
  fx_rate: '',
  date: todayIso(),
  odometer: 0,
  is_recurring: false,
  recurrence_interval_months: 12,
  recurrence_end_date: '',
  amortization_mode: defaultAmortizationMode('MAINTENANCE'),
  coverage_km: 50000,
  coverage_months: 24,
  closes_maintenance_id: null as string | null,
  description: '',
  document_id: null as string | null,
  document_filename: null as string | null,
})

const amortizationLabel = computed(() => {
  const key = { NONE: 'immediate', DISTANCE: 'perKm', DURATION: 'byDuration', HYBRID: 'mixed' }[maintForm.value.amortization_mode] ?? 'immediate'
  return t(`expenses.maintenanceModal.${key}`, { unit: distanceUnit() })
})
const descriptionSuggestions = computed(() => recentDescriptions(props.maintenanceExpenses))

// The smoothing follows the category until the user picks a mode; it stays folded unless an edited expense uses one.
const showAdvanced = ref(false)
let amortizationTouched = false

function pickAmortization(mode: string) {
  amortizationTouched = true
  maintForm.value.amortization_mode = mode
}

watch(
  () => maintForm.value.category,
  (category) => {
    if (open.value && !props.editing && !amortizationTouched) maintForm.value.amortization_mode = defaultAmortizationMode(category)
  }
)

const detectedOdometer = ref<number | null>(null)
const detectingOdometer = ref(false)
const amountInput = ref<HTMLInputElement | null>(null)
const shouldClosePrevious = ref(false)
// Last value the form put in the odometer itself: only that value is replaced when the date changes.
let autofilledOdometer = 0

const closeCandidateMaintenance = computed(() =>
  findCloseCandidate(props.maintenanceExpenses, editingMaintId.value, maintForm.value.date)
)

async function checkOdometerForDate(dateVal: string) {
  if (!props.vehicleId || !dateVal) return
  detectingOdometer.value = true
  try {
    const res = await api.getOdometerAt(props.vehicleId, dateVal)
    if (res && typeof res.odometer === 'number' && res.source !== 'none') {
      detectedOdometer.value = res.odometer
      if (!maintForm.value.odometer || maintForm.value.odometer === autofilledOdometer) {
        maintForm.value.odometer = Math.round(res.odometer)
        autofilledOdometer = maintForm.value.odometer
      }
    } else {
      detectedOdometer.value = null
    }
  } catch {
    detectedOdometer.value = null
  } finally {
    detectingOdometer.value = false
  }
}

watch(open, (isOpen) => {
  if (!isOpen) return
  const m = props.editing
  detectedOdometer.value = null
  amortizationTouched = false
  showAdvanced.value = Boolean(m && m.amortization_mode && m.amortization_mode !== 'NONE')
  if (!m) {
    shouldClosePrevious.value = false
    autofilledOdometer = props.currentOdometer ? Math.round(props.currentOdometer) : 0
    maintForm.value = {
      category: props.defaultCategory ?? 'MAINTENANCE',
      amount: '',
      currency: baseCurrency.value,
      fx_rate: '',
      date: todayIso(),
      odometer: autofilledOdometer,
      is_recurring: false,
      recurrence_interval_months: 12,
      recurrence_end_date: '',
      amortization_mode: defaultAmortizationMode('MAINTENANCE'),
      coverage_km: 50000,
      coverage_months: 24,
      closes_maintenance_id: null,
      description: '',
      document_id: null,
      document_filename: null,
    }
    checkOdometerForDate(maintForm.value.date)
    nextTick(() => amountInput.value?.focus())
  } else {
    shouldClosePrevious.value = Boolean(m.closes_maintenance_id)
    maintForm.value = {
      category: m.category || 'MAINTENANCE',
      amount: String(m.amount),
      currency: m.currency || baseCurrency.value,
      fx_rate: m.fx_rate ? String(m.fx_rate) : '',
      date: new Date(m.date).toISOString().substring(0, 10),
      odometer: m.odometer ? Math.round(m.odometer) : 0,
      is_recurring: Boolean(m.is_recurring),
      recurrence_interval_months: m.recurrence_interval_months || 12,
      recurrence_end_date: m.recurrence_end_date ? new Date(m.recurrence_end_date).toISOString().substring(0, 10) : '',
      amortization_mode: m.amortization_mode || 'NONE',
      coverage_km: m.coverage_km ? Number(m.coverage_km) : 50000,
      coverage_months: m.coverage_months ? Number(m.coverage_months) : 24,
      closes_maintenance_id: m.closes_maintenance_id || null,
      description: m.description || '',
      document_id: m.document_id || null,
      document_filename: m.document_filename || null,
    }
  }
})

watch(() => maintForm.value.date, (newDate) => {
  if (open.value && newDate) {
    checkOdometerForDate(newDate)
  }
})

const { pending: submitting, run: runOnce } = useSubmit()

async function handleCreateMaintAction() {
  if (!props.vehicleId) return
  try {
    let closesId: string | null = null
    if (shouldClosePrevious.value && closeCandidateMaintenance.value) {
      closesId = closeCandidateMaintenance.value.id
    }

    const payload = {
      ...maintForm.value,
      ...currencyPayload(maintForm.value, baseCurrency.value),
      amount: Number(maintForm.value.amount),
      odometer: maintForm.value.odometer ? Number(maintForm.value.odometer) : null,
      coverage_km:
        maintForm.value.amortization_mode === 'DISTANCE' || maintForm.value.amortization_mode === 'HYBRID'
          ? Number(maintForm.value.coverage_km || 50000)
          : null,
      coverage_months:
        maintForm.value.amortization_mode === 'DURATION' || maintForm.value.amortization_mode === 'HYBRID'
          ? Number(maintForm.value.coverage_months || 24)
          : null,
      closes_maintenance_id: closesId,
      date: new Date(maintForm.value.date).toISOString(),
      recurrence_end_date:
        maintForm.value.is_recurring && maintForm.value.recurrence_end_date
          ? new Date(maintForm.value.recurrence_end_date).toISOString()
          : null,
      document_id: maintForm.value.document_id || null,
    }
    if (editingMaintId.value) {
      await api.updateMaintenance(props.vehicleId, editingMaintId.value, payload)
    } else {
      await api.createMaintenance(props.vehicleId, payload)
    }
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleCreateMaint = () => runOnce(handleCreateMaintAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Wrench class="w-5 h-5 text-pink-400" />
          {{ editingMaintId ? $t('expenses.maintenanceModal.edit') : $t('expenses.maintenanceModal.add') }}
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="maint-modal-form" @submit.prevent="handleCreateMaint" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <div>
          <label for="expense-maint-category" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.category') }}</label>
          <select id="expense-maint-category" v-model="maintForm.category" class="field">
            <option value="MAINTENANCE">{{ $t('expenses.maintenanceModal.maintenanceService') }}</option>
            <option value="REPAIR">{{ $t('expenses.maintenanceModal.repairClaimExcess') }}</option>
            <option value="INSURANCE">{{ $t('expenses.maintenanceModal.insurancePremium') }}</option>
            <option value="SUBSCRIPTION">{{ $t('expenses.maintenanceModal.subscriptionConnectivity') }}</option>
            <option value="TAX">{{ $t('expenses.maintenanceModal.taxRegistration') }}</option>
            <option value="FINANCING">{{ $t('expenses.maintenanceModal.otherFinancingOutsideTheVehicle') }}</option>
            <option value="ACCESSORY">{{ $t('expenses.maintenanceModal.accessory') }}</option>
            <option value="OTHER">{{ $t('expenses.maintenanceModal.other') }}</option>
          </select>
        </div>
        <!-- Insurance: annual premium paid monthly -->
        <div v-if="maintForm.category === 'INSURANCE'" class="bg-indigo-500/10 border border-indigo-500/20 rounded-xl p-3 space-y-2">
          <div class="flex items-end gap-2">
            <div class="flex-1">
              <label for="expense-insurance-annual" class="block text-xs font-semibold text-indigo-200 mb-1">{{ $t('expenses.maintenanceModal.annualPremium', { cur: currencySymbol(baseCurrency) }) }}</label>
              <input
                id="expense-insurance-annual"
                v-model="insuranceAnnualPremium"
                type="number"
                step="0.01"
                min="0"
                :placeholder="$t('common.example', { value: '850' })"
                class="field"
              />
            </div>
            <button type="button" @click="applyMonthlyPremium" class="px-3 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl">
              {{ $t('expenses.maintenanceModal.spreadMonthly') }}
            </button>
          </div>
          <p class="text-xs text-indigo-200/80">
            {{ $t('expenses.maintenanceModal.createsAMonthlyRecurringExpense') }}
          </p>
        </div>
        <p v-else-if="maintForm.category === 'FINANCING'" class="text-xs text-warning-300/90">
          {{ $t('expenses.maintenanceModal.leasePaymentsTheDownPayment') }}
        </p>

        <div>
          <label for="expense-maint-description" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.description') }}</label>
          <input id="expense-maint-description" v-model="maintForm.description" required list="maint-description-suggestions" autocomplete="off" :placeholder="$t('expenses.maintenanceModal.eGCabinFilterReplacement')" class="field" />
          <datalist id="maint-description-suggestions">
            <option v-for="d in descriptionSuggestions" :key="d" :value="d" />
          </datalist>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="expense-maint-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('common.date') }}</label>
            <AppDatePicker id="expense-maint-date" v-model="maintForm.date" required size="sm" />
          </div>
          <div>
            <label for="maint-form-amount" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.amount') }}</label>
            <div class="flex gap-1.5">
              <input id="maint-form-amount" ref="amountInput" v-model="maintForm.amount" type="number" inputmode="decimal" step="0.01" min="0.01" required class="field" />
              <label for="maint-form-currency" class="sr-only">{{ $t('expenses.maintenanceModal.currency') }}</label>
              <select id="maint-form-currency" v-model="maintForm.currency" class="field !w-16 shrink-0 !px-1.5">
                <option v-for="cur in CURRENCIES" :key="cur" :value="cur">{{ cur }}</option>
              </select>
            </div>
          </div>
        </div>
        <div v-if="maintForm.currency !== baseCurrency">
          <label for="maint-form-fx-rate" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.conversionRate1', { currency: maintForm.currency, base: baseCurrency }) }}</label>
          <input id="maint-form-fx-rate" v-model="maintForm.fx_rate" type="number" step="0.000001" min="0.000001" required class="field" />
        </div>

        <div>
          <div class="flex items-center justify-between mb-1">
            <label for="expense-maint-odometer" class="block text-xs font-semibold text-slate-300">{{ $t('expenses.maintenanceModal.odometerKm', { unit: distanceUnit() }) }}</label>
            <span v-if="detectingOdometer" class="text-xs text-slate-400">{{ $t('expenses.maintenanceModal.detectingTheMileage') }}</span>
          </div>
          <DistanceInput id="expense-maint-odometer" v-model="maintForm.odometer" class="field" />
          <div v-if="detectedOdometer !== null && detectedOdometer > 0" class="flex items-center justify-between text-xs text-success-400 mt-1">
            <span>{{ $t('expenses.maintenanceModal.mileageDetectedKm', { unit: distanceUnit(), detectedOdometer: formatDistanceValue(detectedOdometer) }) }}</span>
            <button
              type="button"
              v-if="maintForm.odometer !== Math.round(detectedOdometer)"
              @click="maintForm.odometer = Math.round(detectedOdometer)"
              class="underline hover:text-success-300 transition-colors ml-2"
            >
              {{ $t('expenses.maintenanceModal.apply') }}
            </button>
          </div>
        </div>

        <!-- Lissage du coût pour dépenses non-récurrentes -->
        <details
          v-if="!maintForm.is_recurring"
          :open="showAdvanced"
          class="group rounded-xl border border-slate-700/60 bg-slate-800/40"
          @toggle="showAdvanced = ($event.target as HTMLDetailsElement).open"
        >
          <summary class="flex min-h-11 cursor-pointer list-none items-center justify-between gap-2 px-3.5 text-xs font-semibold text-slate-200 [&::-webkit-details-marker]:hidden">
            <span>{{ $t('expenses.maintenanceModal.advancedOptions') }}</span>
            <span class="flex items-center gap-1.5 font-medium text-slate-400">
              {{ $t('expenses.maintenanceModal.smoothingSummary', { mode: amortizationLabel }) }}
              <ChevronDown class="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden="true" />
            </span>
          </summary>
          <div class="space-y-2.5 border-t border-slate-700/60 p-3.5">
          <span class="block text-xs font-semibold text-slate-200">
            {{ $t('expenses.maintenanceModal.costPerKmSmoothing', { unit: distanceUnit() }) }}
          </span>

          <div class="grid grid-cols-4 gap-1.5 pt-1">
            <button
              type="button"
              @click="pickAmortization('NONE')"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'NONE' ? 'bg-pink-500/20 text-pink-300 border-pink-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              {{ $t('expenses.maintenanceModal.immediate') }}
            </button>
            <button
              type="button"
              @click="pickAmortization('DISTANCE')"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'DISTANCE' ? 'bg-success-500/20 text-success-300 border-success-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              {{ $t('expenses.maintenanceModal.perKm', { unit: distanceUnit() }) }}
            </button>
            <button
              type="button"
              @click="pickAmortization('DURATION')"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'DURATION' ? 'bg-purple-500/20 text-purple-300 border-purple-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              {{ $t('expenses.maintenanceModal.byDuration') }}
            </button>
            <button
              type="button"
              @click="pickAmortization('HYBRID')"
              class="py-1.5 px-1 text-xs font-medium rounded-lg transition-colors text-center border"
              :class="maintForm.amortization_mode === 'HYBRID' ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40' : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-slate-200'"
            >
              {{ $t('expenses.maintenanceModal.mixed') }}
            </button>
          </div>

          <!-- Distance parameter -->
          <div v-if="maintForm.amortization_mode === 'DISTANCE' || maintForm.amortization_mode === 'HYBRID'" class="pt-1">
            <label for="maint-coverage-km" class="block text-xs text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.distanceCoveredKm', { unit: distanceUnit() }) }}</label>
            <DistanceInput
              id="maint-coverage-km"
              v-model="maintForm.coverage_km"
              min="1000"
              step="1000"
              placeholder="50000"
              class="field"
            />
          </div>

          <!-- Duration parameter -->
          <div v-if="maintForm.amortization_mode === 'DURATION' || maintForm.amortization_mode === 'HYBRID'" class="pt-1">
            <label for="maint-coverage-months" class="block text-xs text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.durationCoveredMonths') }}</label>
            <input
              id="maint-coverage-months"
              v-model.number="maintForm.coverage_months"
              type="number"
              min="1"
              max="120"
              placeholder="24"
              class="field"
            />
          </div>

          <!-- Clôture de la maintenance précédente -->
          <div v-if="closeCandidateMaintenance && maintForm.amortization_mode !== 'NONE'" class="pt-2 border-t border-slate-700/60">
            <div class="flex items-start gap-2">
              <input
                id="close-candidate"
                v-model="shouldClosePrevious"
                type="checkbox"
                class="select-box mt-0.5"
              />
              <label for="close-candidate" class="text-xs text-slate-300 leading-snug cursor-pointer">
                {{ $t('expenses.maintenanceModal.closeThePreviousServiceIn') }}
                <span class="block text-xs text-warning-400 font-normal">
                  {{ closeCandidateMaintenance.description }} ({{ formatDate(closeCandidateMaintenance.date) }} — {{ formatAmount(Number(closeCandidateMaintenance.amount), baseCurrency) }})
                </span>
              </label>
            </div>
          </div>
          </div>
        </details>

        <div class="space-y-2 pt-1">
          <div class="flex items-center gap-2">
            <input v-model="maintForm.is_recurring" type="checkbox" id="rec" class="select-box" />
            <label for="rec" class="text-xs text-slate-300 font-medium">{{ $t('expenses.maintenanceModal.recurringExpense') }}</label>
          </div>
          <div v-if="maintForm.is_recurring" class="pt-1 grid grid-cols-2 gap-3">
            <div>
              <label for="maint-form-recurrence-interval-months" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.intervalMonths') }}</label>
              <input id="maint-form-recurrence-interval-months" v-model.number="maintForm.recurrence_interval_months" type="number" min="1" max="120" required class="field" />
            </div>
            <div>
              <label for="maint-form-recurrence-end-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.maintenanceModal.endOptional') }}</label>
              <AppDatePicker id="maint-form-recurrence-end-date" v-model="maintForm.recurrence_end_date" size="sm" :clearable="true" />
            </div>
          </div>
        </div>

        <!-- Justificatif / Facture -->
        <div class="space-y-2 bg-slate-800/40 p-3 rounded-xl border border-slate-700/60">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold text-slate-300 flex items-center gap-1.5">
              <Paperclip class="w-3.5 h-3.5 text-indigo-400" />
              {{ $t('expenses.maintenanceModal.receiptInvoice') }}
            </span>
            <span v-if="maintForm.document_id" class="text-xs text-success-400 font-medium">{{ $t('expenses.maintenanceModal.linked') }}</span>
          </div>

          <div v-if="maintForm.document_id" class="flex items-center justify-between p-2.5 bg-slate-900 border border-indigo-500/30 rounded-xl">
            <div class="flex items-center gap-2 min-w-0">
              <FileText class="w-4 h-4 text-indigo-400 shrink-0" />
              <span class="text-xs text-white truncate font-medium">{{ maintForm.document_filename || $t('expenses.linkedInvoice') }}</span>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              <button
                type="button"
                @click="emit('view-document', maintForm.document_id, maintForm.document_filename, false)"
                class="tap p-1 text-slate-400 hover:text-indigo-400 rounded-lg hover:bg-slate-800"
                :title="$t('expenses.maintenanceModal.viewTheDocument')" :aria-label="$t('expenses.maintenanceModal.viewTheDocument')"
              >
                <Eye class="w-3.5 h-3.5" />
              </button>
              <button
                type="button"
                @click="maintForm.document_id = null; maintForm.document_filename = null"
                class="tap p-1 text-slate-400 hover:text-rose-400 rounded-lg hover:bg-slate-800"
                :title="$t('expenses.maintenanceModal.detachTheReceipt')" :aria-label="$t('expenses.maintenanceModal.detachTheReceipt')"
              >
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div v-else class="space-y-2.5">
            <div v-if="documents.length > 0">
              <label for="maint-existing-doc" class="block text-xs text-slate-400 mb-1">{{ $t('expenses.maintenanceModal.attachAnExistingInvoice') }}</label>
              <select
                id="maint-existing-doc"
                class="field text-slate-300"
                @change="(e: any) => onSelectExistingDoc(e.target.value, maintForm)"
              >
                <option value="">{{ $t('expenses.maintenanceModal.selectAnExistingReceipt') }}</option>
                <option v-for="d in documents" :key="d.id" :value="d.id">
                  {{ d.filename }} ({{ formatDate(d.created_at) }})
                </option>
              </select>
            </div>

            <div>
              <span class="block text-xs text-slate-400 mb-1">{{ $t('expenses.maintenanceModal.orDropANewInvoice') }}</span>
              <AppDropzone
                :model-value="null"
                :disabled="isUploadingDocument"
                :label="$t('expenses.maintenanceModal.dropTheInvoiceHereOr')"
                :helperText="$t('expenses.maintenanceModal.uploadHelper')"
                @update:model-value="(f) => onDropzoneDirectUpload(f, maintForm)"
              />
            </div>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting" type="submit" form="maint-modal-form" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ editingMaintId ? $t('expenses.update') : $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
