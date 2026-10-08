<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import NumberInput from '@/components/NumberInput.vue'
import { Receipt, Radar, Plus, Pencil, Trash2, Save, X, ChevronDown } from 'lucide-vue-next'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { mergeExpensesByDrive, tollApplyStatusLabel } from '@/utils/drives'
import { currencySymbol, formatAmount } from '@/currency'
import { formatPercent } from '@/utils/numbers'
import { distanceUnit, perDistance } from '@/units'

// The tolls and road costs of a drive or of a trip: the expenses attached to it (edit, delete, add one), the GPS toll
// detection and its estimate. It loads what it shows when it appears and again when another drive replaces the one
// shown (a leg of the trip, or the trip again); refreshDrive reloads the drive so the costs above follow.
const props = defineProps<{
  vehicleId: string
  tripDriveIds: string[]
  startWithTollEntry: boolean
  breakdown: any
  refreshDrive: (driveId: string) => Promise<any | null>
}>()
const selectedCostDrive = defineModel<any | null>('drive', { required: true })
const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()
const vehicleCurrency = computed(() => vehicleStore.currency)

// Expense edition inside the cost modal
const editingExpenseId = ref<string | null>(null)
const expenseEditForm = ref({ type: 'TOLL', amount: '' as number | string, notes: '' })

const driveExpenses = ref<any[]>([])
const loadingExpenses = ref(false)
const showAddTollInline = ref(false)
const inlineTollAmount = ref<number | ''>('')
const inlineTollType = ref('TOLL')
const inlineTollNotes = ref('')
const addingToll = ref(false)
const tollDetection = ref<any | null>(null)
const tollDetectionLoading = ref(false)
const tollDetectionError = ref('')
const showTollSegments = ref(false)
const tollDetectionEstimatedTotal = computed(() => {
  const priced = (tollDetection.value?.segments || []).filter((s: any) => s.estimated_price != null)
  if (!priced.length) return null
  return priced.reduce((sum: number, s: any) => sum + s.estimated_price, 0)
})

// Expenses of a trip group: those of its drives, each listed once with its full share across the trip
async function loadTripExpenses() {
  if (!props.vehicleId) return
  loadingExpenses.value = true
  try {
    const expPromises = props.tripDriveIds.map((id) => api.getDriveExpensesForDrive(props.vehicleId, id).catch(() => []))
    const expResults = await Promise.all(expPromises)
    driveExpenses.value = mergeExpensesByDrive(expResults.flat())
  } finally {
    loadingExpenses.value = false
  }
}

async function loadTollDetection(drive: any) {
  tollDetection.value = null
  tollDetectionError.value = ''
  showTollSegments.value = false
  if (!props.vehicleId || drive.is_trip_group) return
  try {
    tollDetection.value = await api.getTollDetection(props.vehicleId, drive.id)
  } catch (err) {
    console.error('Failed to load toll detection', err)
  }
}

const existingTollExpense = computed(() => driveExpenses.value.find((e: any) => e.type === 'TOLL'))
const canApplyTollEstimate = computed(
  () =>
    vehicleStore.canEdit &&
    tollDetectionEstimatedTotal.value != null &&
    (!existingTollExpense.value || existingTollExpense.value.source === 'AUTO_TOLL') &&
    !existingTollExpense.value?.trip_group_id
)
// The detection needs the GPS trace of a TeslaMate drive; a trip or a manual drive has none
const canDetectTolls = computed(() => !selectedCostDrive.value?.is_trip_group && !!selectedCostDrive.value?.teslamate_drive_id)
// Nothing to apply when the toll already recorded is that same estimate
const estimateMatchesExistingToll = computed(
  () => existingTollExpense.value?.source === 'AUTO_TOLL' && Math.abs(Number(existingTollExpense.value.amount) - (tollDetectionEstimatedTotal.value ?? Number.NaN)) < 0.005
)
const applyingToll = ref(false)

async function handleApplyTollEstimate() {
  if (!props.vehicleId || !selectedCostDrive.value) return
  applyingToll.value = true
  try {
    const res = await api.applyTollEstimate(props.vehicleId, selectedCostDrive.value.id)
    if (res.status === 'created' || res.status === 'updated') {
      await refreshCostModal()
    } else {
      showAlert(tollApplyStatusLabel(res.status), t('drives.driveCostModal.tollNotApplied'), 'warning')
    }
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    applyingToll.value = false
  }
}

async function handleDetectTolls() {
  if (!props.vehicleId || !selectedCostDrive.value) return
  tollDetectionLoading.value = true
  tollDetectionError.value = ''
  try {
    tollDetection.value = await api.detectTolls(props.vehicleId, selectedCostDrive.value.id)
  } catch (err: any) {
    tollDetectionError.value = err.message || t('drives.driveCostModal.detectionFailed')
  } finally {
    tollDetectionLoading.value = false
  }
}

async function loadDriveExpenses(driveId: string) {
  if (!props.vehicleId) return
  loadingExpenses.value = true
  try {
    driveExpenses.value = await api.getDriveExpensesForDrive(props.vehicleId, driveId)
  } catch (err) {
    console.error('Failed to load drive expenses', err)
    driveExpenses.value = []
  } finally {
    loadingExpenses.value = false
  }
}

async function handleAddTollToDrive() {
  if (!props.vehicleId || !selectedCostDrive.value) return
  if (!inlineTollAmount.value || Number(inlineTollAmount.value) <= 0) {
    showAlert(t('drives.driveCostModal.enterValidAmount'), t('drives.driveCostModal.invalidAmount'), 'warning')
    return
  }

  addingToll.value = true
  try {
    const amountNum = Number(inlineTollAmount.value)
    const target = selectedCostDrive.value.is_trip_group ? { trip_group_id: selectedCostDrive.value.id } : { drive_id: selectedCostDrive.value.id }
    await api.createDriveExpense(props.vehicleId, {
      ...target,
      type: inlineTollType.value,
      amount: amountNum,
      currency: vehicleCurrency.value,
      date: selectedCostDrive.value.start_time,
      notes: inlineTollNotes.value || t('drives.driveCostModal.addedFromDrive'),
    })

    await refreshCostModal()
    showAddTollInline.value = false
    inlineTollAmount.value = ''
    inlineTollNotes.value = ''
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    addingToll.value = false
  }
}

// Reloads the drive costs (computed server-side, with trip group allocation) and its expenses
async function refreshCostModal() {
  if (!selectedCostDrive.value) return
  const driveId = selectedCostDrive.value.id
  const reloadExpenses = selectedCostDrive.value.is_trip_group ? loadTripExpenses() : loadDriveExpenses(driveId)
  const [, refreshed] = await Promise.all([reloadExpenses, props.refreshDrive(driveId)])
  if (refreshed) selectedCostDrive.value = refreshed
}

function startEditExpense(exp: any) {
  editingExpenseId.value = exp.id
  expenseEditForm.value = { type: exp.type, amount: exp.amount, notes: exp.notes || '' }
}

async function handleSaveExpenseEdit(exp: any) {
  if (!props.vehicleId) return
  const amount = Number(expenseEditForm.value.amount)
  if (!amount || amount <= 0) {
    showAlert(t('drives.driveCostModal.enterValidAmount'), t('drives.driveCostModal.invalidAmount'), 'warning')
    return
  }
  try {
    await api.updateDriveExpense(props.vehicleId, exp.id, {
      type: expenseEditForm.value.type,
      amount,
      currency: exp.currency,
      fx_rate: exp.fx_rate ?? null,
      date: exp.date,
      notes: expenseEditForm.value.notes || null,
      // Keep the current link: single drive or trip group
      drive_id: exp.drive_id ?? null,
      trip_group_id: exp.trip_group_id ?? null,
    })
    editingExpenseId.value = null
    await refreshCostModal()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

async function handleDeleteExpense(exp: any) {
  if (!props.vehicleId) return
  const scope = exp.trip_group_id ? t('drives.driveCostModal.tripScope', { name: exp.trip_group_name }) : ''
  const ok = await showConfirm({
    title: t('drives.driveCostModal.deleteCostTitle'),
    message: t('drives.driveCostModal.deleteCostMessage', { amount: formatAmount(Number(exp.amount), exp.currency || vehicleCurrency.value), scope }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteDriveExpense(props.vehicleId, exp.id)
    await refreshCostModal()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

function showFor(drive: any, opening: boolean) {
  editingExpenseId.value = null
  if (drive.is_trip_group) {
    showAddTollInline.value = false
    loadTripExpenses()
    return
  }
  showAddTollInline.value = opening ? props.startWithTollEntry : false
  if (opening) {
    inlineTollAmount.value = ''
    inlineTollNotes.value = ''
  }
  loadDriveExpenses(drive.id)
  loadTollDetection(drive)
}

onMounted(() => showFor(selectedCostDrive.value, true))
watch(
  () => selectedCostDrive.value?.id,
  (id, previous) => {
    if (id && previous && id !== previous) showFor(selectedCostDrive.value, false)
  },
)
</script>

<template>
  <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl space-y-2">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <div class="p-2 bg-warning-500/10 text-warning-400 rounded-lg">
          <Receipt class="w-4 h-4" />
        </div>
        <div>
          <div class="text-xs font-semibold text-white">{{ $t('drives.driveCostModal.tollsAndRoadCosts') }}</div>
          <div class="text-xs text-slate-400">
            {{ $t('drives.driveCostModal.costSAssigned', { length: driveExpenses.length }) }}
          </div>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <div class="text-right">
          <div class="text-sm font-bold text-warning-400 font-mono">{{ formatAmount(selectedCostDrive.costs?.tolls_cost || 0, vehicleCurrency) }}</div>
          <div class="text-xs text-slate-400 font-normal font-sans">({{ formatPercent(breakdown.byKey.tolls.sharePct) }}) · <span class="text-success-400">{{ formatAmount(perDistance(breakdown.byKey.tolls.costPerKm), vehicleCurrency, 3) }}/{{ distanceUnit() }}</span></div>
        </div>
        <button
          v-if="canDetectTolls"
          type="button"
          @click="handleDetectTolls"
          :disabled="tollDetectionLoading"
          class="tap p-1 bg-cyan-600/20 hover:bg-cyan-600/40 text-cyan-300 rounded-lg text-xs disabled:opacity-50"
          :title="tollDetectionLoading ? $t('drives.driveCostModal.detecting') : tollDetection ? $t('drives.driveCostModal.redetect') : $t('drives.driveCostModal.detectTolls')"
          :aria-label="$t('drives.driveCostModal.detectTolls')"
        >
          <Radar class="w-3.5 h-3.5" :class="{ 'animate-pulse': tollDetectionLoading }" />
        </button>
        <button
          v-if="!selectedCostDrive.is_suggestion"
          @click="showAddTollInline = !showAddTollInline"
          class="tap p-1 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg text-xs"
          :title="$t('drives.driveCostModal.addATollOrParking')" :aria-label="$t('drives.driveCostModal.addATollOrParking')"
        >
          <Plus class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>

    <!-- List of attached expenses -->
    <div v-if="driveExpenses.length" class="space-y-1 pt-1 border-t border-slate-700/50">
      <div v-for="exp in driveExpenses" :key="exp.id" class="text-xs text-slate-300 pl-9">
        <div v-if="editingExpenseId !== exp.id" class="flex items-center justify-between gap-2">
          <span>
            {{ exp.type === 'TOLL' ? $t('drives.driveCostModal.toll') : exp.type }}
            <span v-if="exp.source === 'AUTO_TOLL'" class="text-[9px] px-1.5 py-0.5 rounded bg-cyan-500/10 text-cyan-400 font-medium" :title="$t('drives.driveCostModal.calculatedAutomaticallyFromTheGps')">{{ $t('drives.driveCostModal.auto') }}</span>
            <span v-if="exp.notes" class="text-slate-400">({{ exp.notes }})</span>
            <span v-if="exp.trip_group_id && !selectedCostDrive.is_trip_group" class="text-indigo-400"> {{ $t('drives.driveCostModal.shareOfATripCosting', { amount: formatAmount(exp.amount, exp.currency || vehicleCurrency) }) }}</span>
          </span>
          <span class="flex items-center gap-1.5">
            <span class="font-mono text-warning-400">{{ formatAmount(exp.allocated_amount ?? exp.amount, vehicleCurrency) }}</span>
            <button v-if="!selectedCostDrive.is_suggestion" @click="startEditExpense(exp)" class="p-0.5 text-slate-400 hover:text-warning-400" :title="$t('drives.driveCostModal.editThisCost')" :aria-label="$t('drives.driveCostModal.editThisCost')">
              <Pencil class="w-3 h-3" />
            </button>
            <button v-if="!selectedCostDrive.is_suggestion" @click="handleDeleteExpense(exp)" class="p-0.5 text-slate-400 hover:text-danger-400" :title="$t('drives.driveCostModal.deleteThisCost')" :aria-label="$t('drives.driveCostModal.deleteThisCost')">
              <Trash2 class="w-3 h-3" />
            </button>
          </span>
        </div>
        <div v-else class="grid grid-cols-12 gap-1.5 items-center py-1">
          <label :for="`drive-expense-type-${exp.id}`" class="sr-only">{{ $t('drives.driveCostModal.costType') }}</label>
          <select :id="`drive-expense-type-${exp.id}`" v-model="expenseEditForm.type" class="field col-span-3">
            <option value="TOLL">{{ $t('drives.driveCostModal.toll') }}</option>
            <option value="PARKING">{{ $t('drives.driveCostModal.parking') }}</option>
            <option value="FERRY">{{ $t('drives.driveCostModal.ferry') }}</option>
            <option value="OTHER">{{ $t('drives.driveCostModal.other') }}</option>
          </select>
          <label :for="`drive-expense-amount-${exp.id}`" class="sr-only">{{ $t('drives.driveCostModal.totalAmount') }}</label>
          <NumberInput text :id="`drive-expense-amount-${exp.id}`" v-model="expenseEditForm.amount" min="0.01" class="field col-span-3" />
          <label :for="`drive-expense-notes-${exp.id}`" class="sr-only">{{ $t('common.notes') }}</label>
          <input :id="`drive-expense-notes-${exp.id}`" v-model="expenseEditForm.notes" :placeholder="$t('common.notes')" class="field col-span-4" />
          <button @click="handleSaveExpenseEdit(exp)" class="col-span-1 p-1 text-success-400 hover:text-success-300" :title="$t('common.save')" :aria-label="$t('common.save')">
            <Save class="w-3.5 h-3.5" />
          </button>
          <button @click="editingExpenseId = null" class="col-span-1 p-1 text-slate-400 hover:text-white" :title="$t('common.cancel')" :aria-label="$t('common.cancel')">
            <X class="w-3.5 h-3.5" />
          </button>
          <p v-if="exp.trip_group_id" class="col-span-12 text-xs text-indigo-300/80">{{ $t('drives.driveCostModal.totalAmountOfTheTrip') }}</p>
        </div>
      </div>
    </div>

    <!-- GPS toll detection, folded into the tolls: what was detected and its estimate, with the segments on demand -->
    <div
      v-if="canDetectTolls && (tollDetectionError || tollDetection)"
      class="pt-1 space-y-1 text-xs pl-9"
      :class="driveExpenses.length ? '' : 'border-t border-slate-700/50'"
    >
      <p v-if="tollDetectionError" class="text-danger-400">{{ tollDetectionError }}</p>
      <template v-else-if="tollDetection?.segments?.length">
        <div class="flex items-center justify-between gap-2 text-slate-300">
          <button
            type="button"
            @click="showTollSegments = !showTollSegments"
            :aria-expanded="showTollSegments"
            class="flex items-center gap-1 text-cyan-400 hover:text-cyan-300"
          >
            <Radar class="w-3 h-3" />
            {{ $t('drives.driveCostModal.detectedGates', tollDetection.segments.length) }}
            <ChevronDown class="w-3 h-3 transition-transform" :class="{ 'rotate-180': showTollSegments }" />
          </button>
          <span v-if="tollDetectionEstimatedTotal != null" class="flex items-center gap-2 shrink-0">
            <span class="text-slate-400">{{ $t('drives.driveCostModal.totalEstimate') }}</span>
            <span class="text-warning-400 font-mono font-semibold" :title="$t('drives.driveCostModal.class1LightVehicle')">{{ formatAmount(tollDetectionEstimatedTotal, 'EUR') }}</span>
            <button
              v-if="canApplyTollEstimate && !estimateMatchesExistingToll"
              type="button"
              @click="handleApplyTollEstimate"
              :disabled="applyingToll"
              class="btn btn-primary"
            >
              {{ applyingToll ? '...' : existingTollExpense ? $t('drives.driveCostModal.update') : $t('drives.drivesView.apply') }}
            </button>
          </span>
        </div>
        <div v-if="showTollSegments" class="space-y-1">
          <div v-for="(seg, idx) in tollDetection.segments" :key="idx" class="text-slate-300 flex items-center justify-between gap-2">
            <span v-if="seg.type === 'close' && seg.exit">
              {{ seg.operator ? `${seg.operator}${$t('drives.driveCostModal.operatorSeparator')}` : '' }}{{ seg.entry }} → {{ seg.exit }}
            </span>
            <span v-else-if="seg.type === 'close'">{{ $t('drives.driveCostModal.entryDetectedExitNotIdentified', { entry: seg.entry }) }}</span>
            <span v-else>{{ $t('drives.driveCostModal.tollGate', { entry: seg.entry }) }}</span>
            <span v-if="seg.estimated_price != null" class="text-warning-400 font-mono shrink-0">{{ formatAmount(seg.estimated_price, 'EUR') }}</span>
          </div>
        </div>
      </template>
      <p v-else class="text-slate-400">{{ $t('drives.driveCostModal.noTollDetectedOnThis') }}</p>
    </div>

    <!-- Inline add toll form -->
    <div v-if="showAddTollInline" class="p-3 bg-slate-900 border border-slate-700 rounded-xl space-y-2 mt-2">
      <div class="text-xs font-bold text-white">{{ $t('drives.driveCostModal.addATollParkingFee') }}</div>
      <div class="grid grid-cols-2 gap-2">
        <label for="drive-inline-toll-type" class="sr-only">{{ $t('drives.driveCostModal.costType') }}</label>
        <select id="drive-inline-toll-type"
          v-model="inlineTollType"
          class="field"
        >
          <option value="TOLL">{{ $t('drives.driveCostModal.toll') }}</option>
          <option value="PARKING">{{ $t('drives.driveCostModal.parking') }}</option>
          <option value="FERRY">{{ $t('drives.driveCostModal.ferry') }}</option>
        </select>
        <label for="drive-inline-toll-amount" class="sr-only">{{ $t('drives.driveCostModal.amount', { cur: currencySymbol(vehicleCurrency) }) }}</label>
        <NumberInput text id="drive-inline-toll-amount"
          v-model="inlineTollAmount"
          :placeholder="$t('drives.driveCostModal.amount', { cur: currencySymbol(vehicleCurrency) })"
          class="field"
        />
      </div>
      <label for="drive-inline-toll-notes" class="sr-only">{{ $t('drives.driveCostModal.notesEGA6Beaune') }}</label>
      <input id="drive-inline-toll-notes"
        v-model="inlineTollNotes"
        type="text"
        :placeholder="$t('drives.driveCostModal.notesEGA6Beaune')"
        class="field"
      />
      <div class="flex justify-end gap-2">
        <button
          @click="showAddTollInline = false"
          class="px-2.5 py-1 text-xs text-slate-400 hover:text-white"
        >
          {{ $t('common.cancel') }}
        </button>
        <button
          @click="handleAddTollToDrive"
          :disabled="addingToll"
          class="px-3 py-1 bg-warning-600 hover:bg-warning-500 text-white font-semibold text-xs rounded-lg disabled:opacity-50"
        >
          {{ addingToll ? $t('drives.driveCostModal.saving') : $t('drives.driveCostModal.confirm') }}
        </button>
      </div>
    </div>
  </div>
</template>
