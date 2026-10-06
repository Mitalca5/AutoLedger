<script setup lang="ts">
import { formatNumber, formatPercent } from '@/utils/numbers'
import { t } from '@/i18n'
import { computed, ref, watch } from 'vue'
import { defaultDriver } from '@/utils/vehicles'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { usePreferencesStore } from '@/stores/preferences'
import { api, type VehiclePerson } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { Receipt, Layers, MapPin, ExternalLink, Zap, X, Users, Coins, Shield, Wrench, Disc, Plus, AlertTriangle, Pencil, Trash2, Save, ArrowLeft, ChevronRight, ChevronDown, Radar, User } from 'lucide-vue-next'
import { teslamateDriveUrl as buildTeslamateDriveUrl, tollApplyStatusLabel, mergeExpensesByDrive } from '@/utils/drives'
import { formatDayTime } from '@/utils/dates'
import { buildDriveBreakdown } from '@/utils/costBreakdown'
import { currencySymbol, formatAmount } from '@/currency'
import CostDonut from '@/components/costs/CostDonut.vue'
import CostItemRow from '@/components/costs/CostItemRow.vue'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistance, kmToDisplayDistance, perDistance, speedUnit } from '@/units'
import { formatCostPerDistance } from '@/utils/costPerDistance'

// Cost breakdown of a drive, or of a trip group (drive.is_trip_group, whose drives are tripDriveIds), with its
// expenses (edit, delete, add a toll) and the toll detection. A detected trip that is not created yet
// (drive.is_suggestion) is shown read-only, with a button to create it. refreshDrive reloads the drives list and returns the
// refreshed drive so the breakdown follows the server-side costs.
const props = defineProps<{
  vehicleId: string
  tripDriveIds: string[]
  tripLegs: any[]
  startWithTollEntry: boolean
  // Set when the drive was opened from another detail (a carpool): the header then offers a way back to it
  backLabel?: string
  refreshDrive: (driveId: string) => Promise<any | null>
}>()
const emit = defineEmits<{
  'toggle-tag': [drive: any, tag: string]
  'create-trip': [suggestion: any]
  'dismiss-trip': [suggestion: any]
  'edit-trip': [trip: any]
  back: []
}>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const selectedCostDrive = defineModel<any | null>('drive', { required: true })
const router = useRouter()
const vehicleStore = useVehicleStore()
const prefs = usePreferencesStore()
const { showConfirm, showAlert } = useConfirm()
const formatDate = formatDayTime
const breakdown = computed(() => buildDriveBreakdown(selectedCostDrive.value?.costs, Number(selectedCostDrive.value?.distance_km) || 0))
const vehicleCurrency = computed(() => vehicleStore.currency)
const teslamateDriveUrl = (d: any) => buildTeslamateDriveUrl(vehicleStore.activeVehicle, d)

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

// A trip shows its legs and carpools; opening a leg keeps the trip to come back to
const parentTrip = ref<any | null>(null)
const tripCarpools = ref<any[]>([])

async function loadTripCarpools(tripId: string) {
  tripCarpools.value = []
  if (!props.vehicleId) return
  try {
    const res = await api.getCarpools(props.vehicleId)
    tripCarpools.value = (res.trips || []).filter((c: any) => c.trip_group_id === tripId)
  } catch {
    tripCarpools.value = []
  }
}

function openLeg(leg: any) {
  parentTrip.value = selectedCostDrive.value
  selectedCostDrive.value = leg
  editingExpenseId.value = null
  showAddTollInline.value = false
  loadDriveExpenses(leg.id)
  loadTollDetection(leg)
}

async function backToTrip() {
  const trip = parentTrip.value
  if (!trip) return
  parentTrip.value = null
  selectedCostDrive.value = trip
  editingExpenseId.value = null
  showAddTollInline.value = false
  const [, refreshed] = await Promise.all([loadTripExpenses(), props.refreshDrive(trip.id)])
  if (refreshed) selectedCostDrive.value = refreshed
}

watch(open, (isOpen) => {
  const drive = selectedCostDrive.value
  if (!isOpen) parentTrip.value = null
  if (!isOpen || !drive) return
  editingExpenseId.value = null
  if (drive.is_trip_group) {
    showAddTollInline.value = false
    loadTripExpenses()
    if (!drive.is_suggestion) loadTripCarpools(drive.id)
    return
  }
  showAddTollInline.value = props.startWithTollEntry
  inlineTollAmount.value = ''
  inlineTollNotes.value = ''
  loadDriveExpenses(drive.id)
  loadTollDetection(drive)
  loadVehiclePeople()
})

const vehiclePeople = ref<VehiclePerson[]>([])
const defaultDriverOption = computed(() => {
  const d = defaultDriver(vehiclePeople.value, vehicleStore.activeVehicle?.default_driver_id)
  return d ? t('drives.defaultDriver', { name: d.name }) : t('drives.unassignedDriver')
})

async function loadVehiclePeople() {
  if (!props.vehicleId) return
  try {
    vehiclePeople.value = await api.getVehiclePeople(props.vehicleId)
  } catch (err) {
    console.error('Failed to load vehicle people', err)
  }
}

async function handleDriverChange(event: Event) {
  const val = (event.target as HTMLSelectElement).value
  const driverId = val.trim() ? val.trim() : null
  if (!props.vehicleId || !selectedCostDrive.value?.id) return
  try {
    await api.updateDriveDriver(props.vehicleId, selectedCostDrive.value.id, driverId)
    const refreshed = await props.refreshDrive(selectedCostDrive.value.id)
    if (refreshed) {
      selectedCostDrive.value = refreshed
    }
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

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
  () => existingTollExpense.value?.source === 'AUTO_TOLL' && Math.abs(Number(existingTollExpense.value.amount) - (tollDetectionEstimatedTotal.value ?? NaN)) < 0.005
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
</script>

<template>
  <div
    v-if="open && selectedCostDrive"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-3xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Header -->
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2.5 min-w-0 pr-2">
          <button
            v-if="parentTrip || backLabel"
            type="button"
            @click="parentTrip ? backToTrip() : emit('back')"
            class="tap p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0"
            :title="parentTrip ? $t('drives.driveCostModal.backToTrip') : backLabel" :aria-label="parentTrip ? $t('drives.driveCostModal.backToTrip') : backLabel"
          >
            <ArrowLeft class="w-4 h-4" />
          </button>
          <div class="p-2 rounded-xl shrink-0" :class="selectedCostDrive.is_trip_group ? 'bg-indigo-500/10 text-indigo-400' : 'bg-success-500/10 text-success-400'">
            <component :is="selectedCostDrive.is_trip_group ? Layers : Coins" class="w-5 h-5" />
          </div>
          <div class="min-w-0 truncate">
            <h3 class="text-base font-bold text-white truncate">
              {{ selectedCostDrive.is_trip_group ? $t('drives.driveCostModal.tripTitle', { name: selectedCostDrive.trip_group_name }) : $t('drives.driveCostModal.driveTitle') }}
            </h3>
            <p class="text-xs text-slate-400">{{ formatDate(selectedCostDrive.start_time) }}</p>
          </div>
        </div>
        <a
          v-if="teslamateDriveUrl(selectedCostDrive)"
          :href="teslamateDriveUrl(selectedCostDrive)!"
          target="_blank"
          rel="noopener noreferrer"
          class="ml-auto mr-1 px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold rounded-lg flex items-center gap-1.5 shrink-0"
          :title="$t('drives.driveCostModal.openThisDriveInThe')"
        >
          <ExternalLink class="w-3.5 h-3.5 text-info-400" />
          <span class="hidden sm:inline">TeslaMate</span>
        </a>
        <button @click="open = false" class="tap p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">

      <!-- Trip Summary Route -->
      <div class="bg-slate-800/60 border border-slate-700/60 p-3.5 rounded-2xl space-y-2">
        <div class="text-sm font-semibold text-white flex items-center gap-2">
          <MapPin class="w-4 h-4 text-rose-400 shrink-0" />
          <span class="truncate">{{ selectedCostDrive.start_address || $t('drives.driveCostModal.start') }}</span>
          <span class="text-slate-400">→</span>
          <span class="truncate">{{ selectedCostDrive.end_address || $t('drives.driveCostModal.end') }}</span>
        </div>
        <div class="flex items-center gap-3 text-xs text-slate-300 flex-wrap">
          <span class="font-bold text-rose-400">{{ formatDistance(selectedCostDrive.distance_km, 1) }}</span>
          <span v-if="selectedCostDrive.duration_min" class="text-slate-400">{{ $t('drives.driveCostModal.min', { duration_min: selectedCostDrive.duration_min }) }}</span>
          <span v-if="selectedCostDrive.drives_count" class="text-indigo-400 font-semibold">{{ $t('drives.driveCostModal.legs', { drives_count: selectedCostDrive.drives_count }) }}</span>
          <span v-if="selectedCostDrive.speed_avg" class="text-slate-400">{{ $t('drives.driveCostModal.kmHAvg', { speed: speedUnit(), speed_avg: Math.round(kmToDisplayDistance(selectedCostDrive.speed_avg)) }) }}</span>
          <span v-if="selectedCostDrive.costs?.electricity_kwh" class="text-info-400 font-mono">{{ $t('drives.driveCostModal.kwh', { electricity_kwh: selectedCostDrive.costs.electricity_kwh }) }}</span>
        </div>

        <!-- Tag qualification (only for individual drives) -->
        <div v-if="prefs.proPersoEnabled && !selectedCostDrive.is_trip_group && vehicleStore.canEdit" class="pt-2 border-t border-slate-700/60 flex items-center justify-between gap-2">
          <span class="text-xs text-slate-400">{{ $t('drives.driveCostModal.classification') }}</span>
          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="emit('toggle-tag', selectedCostDrive, 'Pro')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                selectedCostDrive.tags?.includes('Pro')
                  ? 'bg-blue-500/20 text-blue-400 border-blue-500/40 shadow-sm'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              {{ $t('drives.driveCostModal.work') }}
            </button>
            <button
              type="button"
              @click="emit('toggle-tag', selectedCostDrive, 'Perso')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                selectedCostDrive.tags?.includes('Perso')
                  ? 'bg-success-500/20 text-success-400 border-success-500/40 shadow-sm'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              {{ $t('drives.driveCostModal.personal') }}
            </button>
          </div>
        </div>

        <!-- Driver attribution (only for individual drives) -->
        <div v-if="!selectedCostDrive.is_trip_group && vehicleStore.canEdit" class="pt-2 border-t border-slate-700/60 flex items-center justify-between gap-2">
          <label for="drive-driver-select" class="text-xs text-slate-400 flex items-center gap-1.5 cursor-pointer">
            <User class="w-3.5 h-3.5 text-purple-400" />
            {{ $t('drives.driverLabel') }}
          </label>
          <select
            id="drive-driver-select"
            :value="selectedCostDrive.driver_id || ''"
            @change="handleDriverChange($event)"
            class="field focus:border-purple-500"
          >
            <option value="">{{ defaultDriverOption }}</option>
            <option v-for="p in vehiclePeople" :key="p.id" :value="p.id">
              {{ p.name }}
            </option>
          </select>
        </div>
      </div>

      <p v-if="selectedCostDrive.is_suggestion" class="text-xs text-indigo-300 bg-indigo-500/10 border border-indigo-500/20 rounded-xl px-3 py-2">
        {{ $t('drives.driveCostModal.suggestionNotice') }}
      </p>

      <p v-if="selectedCostDrive.costs?.has_estimates" class="text-xs text-warning-400/90 bg-warning-500/10 border border-warning-500/20 rounded-xl px-3 py-2 flex items-start gap-2">
        <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
        <span>{{ $t('drives.driveCostModal.someItemsUseADefault') }}</span>
      </p>

      <!-- Legs of the trip, as drive rows -->
      <div v-if="selectedCostDrive.is_trip_group && tripLegs.length" class="space-y-1.5">
        <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
          <Layers class="w-3.5 h-3.5 text-indigo-400" />
          {{ $t('drives.driveCostModal.tripLegs', { count: tripLegs.length }) }}
        </h4>
        <button
          v-for="leg in tripLegs"
          :key="leg.id"
          type="button"
          @click="openLeg(leg)"
          class="w-full text-left flex items-center justify-between gap-3 bg-slate-800/40 hover:bg-slate-800/70 border border-slate-800 hover:border-slate-700 rounded-xl px-3 py-2 transition-colors"
        >
          <div class="min-w-0">
            <div class="text-xs text-slate-400">{{ formatDate(leg.start_time) }}</div>
            <div class="text-xs text-slate-200 truncate">
              {{ (leg.start_address || $t('drives.driveCostModal.start')).split(',')[0] }} → {{ (leg.end_address || $t('drives.driveCostModal.end')).split(',')[0] }}
            </div>
          </div>
          <div class="flex items-center gap-3 shrink-0">
            <span class="text-xs font-bold text-rose-400">{{ formatDistance(leg.distance_km) }}</span>
            <span class="text-xs font-mono font-bold text-white">{{ formatAmount(leg.costs?.total_cost || 0, vehicleCurrency) }}</span>
            <ChevronRight class="w-4 h-4 text-slate-400" />
          </div>
        </button>
      </div>

      <!-- Carpools of the trip -->
      <div v-if="selectedCostDrive.is_trip_group && tripCarpools.length" class="space-y-1.5">
        <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
          <Users class="w-3.5 h-3.5 text-rose-400" />
          {{ $t('drives.driveCostModal.tripCarpools', { count: tripCarpools.length }) }}
        </h4>
        <button
          v-for="c in tripCarpools"
          :key="c.id"
          type="button"
          @click="open = false; router.push({ path: '/carpools' })"
          class="w-full text-left flex items-center justify-between gap-3 bg-rose-500/5 hover:bg-rose-500/10 border border-rose-500/20 rounded-xl px-3 py-2 transition-colors"
        >
          <div class="min-w-0">
            <div class="text-xs font-semibold text-white truncate">{{ c.title }}</div>
            <div class="text-xs text-slate-400">
              {{ $t('drives.driveCostModal.carpoolLegsPassengers', { legs: c.legs?.length || 1, passengers: c.passengers?.length || 0 }) }}
            </div>
          </div>
          <div class="text-right shrink-0">
            <div class="text-xs font-mono font-bold text-success-400">+{{ formatAmount(Number(c.total_revenue || 0), vehicleCurrency) }}</div>
            <div class="text-xs text-slate-400">{{ $t('drives.driveCostModal.carpoolNetCost', { amount: formatAmount(Number(c.net_cost || 0), vehicleCurrency) }) }}</div>
          </div>
        </button>
      </div>

      <!-- Same layout as the monthly detail: donut on the left, itemized costs on the right -->
      <div class="grid grid-cols-1 md:grid-cols-5 gap-6 items-start">
      <div class="md:col-span-2 bg-slate-800/30 border border-slate-800 rounded-xl p-4 flex flex-col items-center justify-center">
        <h4 class="text-xs font-bold text-white mb-2 self-start">{{ $t('drives.driveCostModal.breakdownTitle') }}</h4>
        <div class="w-full h-56 sm:h-64 relative">
          <CostDonut
            :items="breakdown.items"
            :empty-label="$t('drives.driveCostModal.noCost')"
            :chart-label="$t('drives.driveCostModal.breakdownAria')"
            :currency="vehicleCurrency"
          />
        </div>
      </div>

      <!-- Cost Breakdown List -->
      <div class="md:col-span-3 space-y-2.5">
        <!-- 1. Électricité -->
        <CostItemRow
          :icon="Zap"
          tone="sky"
          :label="$t('drives.driveCostModal.electricEnergy')"
          :sub="$t('drives.driveCostModal.kwhKwh', { electricity_kwh: selectedCostDrive.costs?.electricity_kwh || 0, electricity_rate: formatAmount(selectedCostDrive.costs?.electricity_rate || 0.22, vehicleCurrency, 3) })"
          :amount="selectedCostDrive.costs?.electricity_cost || 0"
          :share-pct="breakdown.byKey.energy.sharePct"
          :cost-per-km="breakdown.byKey.energy.costPerKm"
          :currency="vehicleCurrency"
        >
          <template #badge>
            <span v-if="selectedCostDrive.costs?.energy_source === 'DEFAULT' || selectedCostDrive.costs?.electricity_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium">{{ $t('drives.driveCostModal.estimate') }}</span>
          </template>
        </CostItemRow>

        <!-- 2. Pneus -->
        <CostItemRow
          :icon="Disc"
          tone="emerald"
          :label="$t('drives.driveCostModal.tireWear')"
          :sub="`${formatDistance(selectedCostDrive.distance_km, 1)} × ${formatAmount(perDistance(selectedCostDrive.costs?.tires_rate || 0.02), vehicleCurrency, 3)}/${distanceUnit()}`"
          :amount="selectedCostDrive.costs?.tires_cost || 0"
          :share-pct="breakdown.byKey.tires.sharePct"
          :cost-per-km="breakdown.byKey.tires.costPerKm"
          :currency="vehicleCurrency"
        >
          <template #badge>
            <span v-if="selectedCostDrive.costs?.tires_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium">{{ $t('drives.driveCostModal.estimate') }}</span>
            <span v-else-if="selectedCostDrive.costs?.tires_rate_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-success-500/10 text-success-400 font-medium">{{ $t('drives.driveCostModal.includedInTheLease2') }}</span>
          </template>
        </CostItemRow>

        <!-- 3. Entretien -->
        <CostItemRow
          :icon="Wrench"
          tone="pink"
          :label="$t('drives.driveCostModal.maintenanceProvision')"
          :sub="`${formatDistance(selectedCostDrive.distance_km, 1)} × ${formatAmount(perDistance(selectedCostDrive.costs?.maintenance_rate || 0.015), vehicleCurrency, 3)}/${distanceUnit()}`"
          :amount="selectedCostDrive.costs?.maintenance_cost || 0"
          :share-pct="breakdown.byKey.maintenance.sharePct"
          :cost-per-km="breakdown.byKey.maintenance.costPerKm"
          :currency="vehicleCurrency"
        >
          <template #badge>
            <span v-if="selectedCostDrive.costs?.maintenance_rate_source === 'DEFAULT'" class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium">{{ $t('drives.driveCostModal.estimate') }}</span>
            <span v-else-if="selectedCostDrive.costs?.maintenance_rate_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-success-500/10 text-success-400 font-medium">{{ $t('drives.driveCostModal.includedInTheLease2') }}</span>
          </template>
        </CostItemRow>

        <!-- 4. Assurance -->
        <CostItemRow
          :icon="Shield"
          tone="purple"
          :label="$t('drives.driveCostModal.insuranceShareFixedCost')"
          :sub="`${formatDistance(selectedCostDrive.distance_km, 1)} × ${formatAmount(perDistance(selectedCostDrive.costs?.insurance_rate || 0), vehicleCurrency, 3)}/${distanceUnit()}`"
          :amount="selectedCostDrive.costs?.insurance_cost || 0"
          :share-pct="breakdown.byKey.insurance.sharePct"
          :cost-per-km="breakdown.byKey.insurance.costPerKm"
          :currency="vehicleCurrency"
        >
          <template #badge>
            <span
              v-if="selectedCostDrive.costs?.insurance_source === 'RECORDED_EXPENSES'"
              class="text-[9px] px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 font-medium"
              :title="$t('drives.driveCostModal.premiumsPaidOverTheLast')"
            >
              {{ $t('drives.driveCostModal.actualPremiums') }}
            </span>
            <span v-else-if="selectedCostDrive.costs?.insurance_source === 'INCLUDED_IN_LEASE'" class="text-[9px] px-1.5 py-0.5 rounded bg-success-500/10 text-success-400 font-medium">
              {{ $t('drives.driveCostModal.includedInTheLease') }}
            </span>
            <span
              v-else-if="selectedCostDrive.costs?.insurance_source === 'INSUFFICIENT_DISTANCE'"
              class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium"
              :title="$t('drives.driveCostModal.lessThan500KmDriven', { min: formatDistance(500) })"
            >
              {{ $t('drives.driveCostModal.notEnoughKm') }}
            </span>
            <span v-else class="text-[9px] px-1.5 py-0.5 rounded bg-warning-500/10 text-warning-400 font-medium" :title="$t('drives.driveCostModal.noInsurancePremiumRecordedIn')">
              {{ $t('drives.driveCostModal.notEntered') }}
            </span>
          </template>
        </CostItemRow>

        <!-- 5. Péages & Frais de route -->
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
                <input :id="`drive-expense-amount-${exp.id}`" v-model="expenseEditForm.amount" type="number" step="0.01" min="0.01" class="field col-span-3" />
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
                    class="px-2 py-0.5 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-lg disabled:opacity-50"
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
              <input id="drive-inline-toll-amount"
                v-model="inlineTollAmount"
                type="number"
                step="0.01"
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

        <!-- Grand Total Card -->
        <div class="bg-gradient-to-r from-slate-800 to-slate-800/80 border border-success-500/30 p-4 rounded-2xl flex items-center justify-between shadow-lg">
          <div>
            <span class="text-xs font-semibold text-success-400 uppercase tracking-wider">{{ $t('drives.driveCostModal.totalCostPrice') }}</span>
            <div class="text-2xl font-black text-white">
              {{ formatAmount(selectedCostDrive.costs?.total_cost || 0, vehicleCurrency) }}
            </div>
          </div>
          <div class="text-right">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('drives.driveCostModal.costPerKilometre', { unit: distanceUnit() }) }}</span>
            <div class="text-lg font-extrabold text-success-400 font-mono">
              {{ formatCostPerDistance(selectedCostDrive.costs?.cost_per_km, vehicleCurrency, 3, true) }}
            </div>
          </div>
        </div>
      </div>
      </div>

      </div>

      <!-- Footer Actions -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <button
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
        >
          {{ $t('common.close') }}
        </button>
        <div v-if="selectedCostDrive.is_suggestion && vehicleStore.canEdit" class="flex items-center gap-2">
          <button
            type="button"
            @click="emit('dismiss-trip', selectedCostDrive)"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
            :title="$t('drives.tripSuggestions.dismissTitle')"
          >
            {{ $t('drives.tripSuggestions.dismiss') }}
          </button>
          <button
            type="button"
            @click="emit('create-trip', selectedCostDrive)"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/25 transition-all"
          >
            <Layers class="w-4 h-4" />
            <span>{{ $t('drives.tripSuggestions.create') }}</span>
          </button>
        </div>
        <div v-else-if="!selectedCostDrive.is_suggestion" class="flex items-center gap-2">
          <button
            v-if="selectedCostDrive.is_trip_group && vehicleStore.canEdit"
            type="button"
            @click="emit('edit-trip', selectedCostDrive)"
            class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-2 transition-colors"
          >
            <Pencil class="w-4 h-4" />
            <span>{{ $t('common.edit') }}</span>
          </button>
          <button
            @click="open = false; router.push({ path: '/carpools', query: selectedCostDrive.is_trip_group ? { new_trip_group_id: selectedCostDrive.id } : { new_drive_id: selectedCostDrive.id } })"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/25 transition-all"
          >
            <Users class="w-4 h-4" />
            <span>{{ $t('drives.driveCostModal.shareAsACarpool') }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
