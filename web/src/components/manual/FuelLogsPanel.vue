<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import { intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, computed, watch, onMounted } from 'vue'
import { Fuel, Plus, Pencil, Trash2, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { currencySymbol, formatAmount } from '@/currency'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistance, formatPerDistanceValue, perDistance } from '@/units'
import LoadError from '@/components/LoadError.vue'

const props = defineProps<{
  vehicleId: string
  canEdit: boolean
}>()

const vehicleStore = useVehicleStore()
const currency = computed(() => vehicleStore.currency)
const { showConfirm, showAlert } = useConfirm()

const stats = ref<any | null>(null)
const loading = ref(false)
const loadError = ref<string | null>(null)
const saving = ref(false)
const showForm = ref(false)
useEscapeToClose(showForm, () => (showForm.value = false))
const editingId = ref<string | null>(null)
const formError = ref('')

const fuelTypes = [
  { value: '', label: '', labelKey: 'manual.fuelLogsPanel.unspecified' },
  { value: 'SP95_E10', label: '', labelKey: 'comparison.fuelTypes.SP95_E10' },
  { value: 'SP98', label: '', labelKey: 'comparison.fuelTypes.SP98' },
  { value: 'DIESEL', label: '', labelKey: 'manual.fuelLogsPanel.diesel' },
  { value: 'E85', label: '', labelKey: 'comparison.fuelTypes.E85' },
  { value: 'GPL', label: '', labelKey: 'comparison.fuelTypes.GPL' },
]

function emptyForm() {
  return {
    date: new Date().toISOString().substring(0, 10),
    odometer: null as number | null,
    amount: null as number | null,
    liters: null as number | null,
    price_per_liter: null as number | null,
    fuel_type: '',
    is_full_tank: true,
    notes: '',
  }
}

const form = ref(emptyForm())

// Newest first for display; the API sends them oldest first
const logs = computed<any[]>(() => [...(stats.value?.logs || [])].reverse())

// Any two of amount / liters / price per liter determine the third (the server derives the missing one)
const derivedHint = computed(() => {
  const { amount, liters, price_per_liter: price } = form.value
  if (amount && liters && !price) return t('manual.fuelLogsPanel.derivedPrice', { value: formatAmount(amount / liters, currency.value, 3) })
  if (amount && price && !liters) return t('manual.fuelLogsPanel.derivedQuantity', { value: (amount / price).toLocaleString(intlLocale(), { maximumFractionDigits: 2 }) })
  if (liters && price && !amount) return t('manual.fuelLogsPanel.derivedAmount', { value: formatAmount(liters * price, currency.value) })
  return ''
})

function fmtMoney(v: number | null | undefined, digits = 2): string {
  return formatAmount(Number(v || 0), currency.value, digits)
}

function fmtNum(v: number | null | undefined, digits = 1): string {
  return Number(v).toLocaleString(intlLocale(), { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

function fmtDate(d: string): string {
  return new Date(d).toLocaleDateString(intlLocale(), { day: '2-digit', month: 'short', year: 'numeric' })
}

async function load() {
  loading.value = true
  loadError.value = null
  try {
    stats.value = await api.getFuelLogs(props.vehicleId)
  } catch (err: any) {
    console.error('Failed to load fuel logs', err)
    loadError.value = err?.message ?? ''
  } finally {
    loading.value = false
  }
}

function openAdd() {
  editingId.value = null
  form.value = emptyForm()
  formError.value = ''
  showForm.value = true
}

function openEdit(log: any) {
  editingId.value = log.id
  form.value = {
    date: String(log.date).substring(0, 10),
    odometer: log.odometer ?? null,
    amount: log.amount,
    liters: log.liters ?? null,
    price_per_liter: log.price_per_liter ?? null,
    fuel_type: log.fuel_type || '',
    is_full_tank: log.is_full_tank,
    notes: log.notes || '',
  }
  formError.value = ''
  showForm.value = true
}

function hasOdometer(v: unknown): boolean {
  return v !== null && v !== undefined && String(v) !== ''
}

function payload() {
  const f = form.value
  return {
    date: f.date,
    odometer: hasOdometer(f.odometer) ? Number(f.odometer) : undefined,
    amount: f.amount ? Number(f.amount) : undefined,
    liters: f.liters ? Number(f.liters) : undefined,
    price_per_liter: f.price_per_liter ? Number(f.price_per_liter) : undefined,
    fuel_type: f.fuel_type || undefined,
    is_full_tank: f.is_full_tank,
    notes: f.notes || undefined,
  }
}

async function save() {
  formError.value = ''
  if (!form.value.date) {
    formError.value = t('manual.fuelLogsPanel.dateRequired')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await api.updateFuelLog(props.vehicleId, editingId.value, payload())
    } else {
      await api.createFuelLog(props.vehicleId, payload(), currency.value)
    }
    showForm.value = false
    await load()
  } catch (err: any) {
    formError.value = err?.message || t('manual.fuelLogsPanel.saveFailed')
  } finally {
    saving.value = false
  }
}

async function remove(log: any) {
  const ok = await showConfirm({
    title: t('manual.fuelLogsPanel.deleteTitle'),
    message: t('manual.fuelLogsPanel.deleteMessage', { date: fmtDate(log.date), amount: fmtMoney(log.amount) }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteFuelLog(props.vehicleId, log.id)
    await load()
  } catch (err: any) {
    await showAlert(err?.message || t('manual.fuelLogsPanel.deleteFailed'), t('shell.confirm.error'), 'danger')
  }
}

watch(() => props.vehicleId, load)
onMounted(load)
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2 text-sm font-semibold text-white">
        <Fuel class="w-4 h-4 text-warning-400" /> {{ $t('manual.fuelLogsPanel.fuelFillUps') }}
      </div>
      <button
        v-if="canEdit"
        type="button"
        class="btn btn-lg btn-primary"
        @click="openAdd"
      >
        <Plus class="w-3.5 h-3.5" /> {{ $t('manual.fuelLogsPanel.newFillUp') }}
      </button>
    </div>

    <div v-if="stats && stats.fill_ups > 0" class="grid grid-cols-2 md:grid-cols-4 gap-3">
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-3">
        <div class="text-xs text-slate-400">{{ $t('manual.fuelLogsPanel.fillUps') }}</div>
        <div class="text-lg font-bold text-white">{{ stats.fill_ups }}</div>
      </div>
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-3">
        <div class="text-xs text-slate-400">{{ $t('manual.fuelLogsPanel.totalSpent') }}</div>
        <div class="text-lg font-bold text-white">{{ fmtMoney(stats.total_cost, 0) }}</div>
      </div>
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-3">
        <div class="text-xs text-slate-400">{{ $t('manual.fuelLogsPanel.averagePricePerLitre') }}</div>
        <div class="text-lg font-bold text-white">{{ stats.avg_price_per_liter ? `${fmtMoney(stats.avg_price_per_liter, 3)}/L` : '—' }}</div>
      </div>
      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-3">
        <div class="text-xs text-slate-400">{{ $t('manual.fuelLogsPanel.averageConsumption') }}</div>
        <div class="text-lg font-bold text-white">{{ stats.consumption_l_100km ? `${formatPerDistanceValue(stats.consumption_l_100km, 2)} L/100 ${distanceUnit()}` : $t('manual.fuelLogsPanel.notMeasurable') }}</div>
      </div>
    </div>
    <p v-if="stats && stats.unmeasurable_segments > 0" class="text-xs text-warning-400">
      {{ $t('manual.fuelLogsPanel.intervalSBetweenFullTanks', { unmeasurable_segments: stats.unmeasurable_segments }) }}
    </p>
    <p v-if="stats && stats.fill_ups_without_mileage > 0" class="text-xs text-slate-400">
      {{ $t('manual.fuelLogsPanel.fillUpSWithoutMileage', { fill_ups_without_mileage: stats.fill_ups_without_mileage }) }}
      <template v-if="stats.estimated_segments > 0">{{ $t('manual.fuelLogsPanel.theConsumptionOfIntervalS', { estimated_segments: stats.estimated_segments }) }}</template>
    </p>
    <p v-if="stats && stats.fill_ups > 0 && !stats.consumption_l_100km && stats.unmeasurable_segments === 0" class="text-xs text-slate-400">
      {{ $t('manual.fuelLogsPanel.consumptionIsCalculatedBetweenTwo') }}
    </p>

    <div v-if="loading && !stats" class="text-sm text-slate-400">{{ $t('manual.fuelLogsPanel.loading') }}</div>
    <LoadError v-else-if="loadError !== null" :message="loadError" @retry="load" />
    <EmptyState v-else-if="logs.length === 0">
      {{ $t('manual.fuelLogsPanel.noFillUpRecordedEnter', { unit: distanceUnit() }) }}
    </EmptyState>
    <ul v-else class="space-y-2">
      <li v-for="log in logs" :key="log.id" class="bg-slate-900 border border-slate-800 rounded-2xl p-3 flex items-center justify-between gap-3">
        <div class="min-w-0">
          <div class="text-sm font-semibold text-white flex flex-wrap items-center gap-2">
            {{ fmtDate(log.date) }}
            <template v-if="log.odometer != null"> · {{ formatDistance(log.odometer) }}</template>
            <template v-else-if="log.odometer_estimated != null"> · ≈ {{ formatDistance(log.odometer_estimated) }} <span class="text-xs px-2 py-0.5 rounded-full border border-slate-600 text-slate-400">{{ $t('manual.fuelLogsPanel.estimated') }}</span></template>
            <span v-if="!log.is_full_tank" class="text-xs px-2 py-0.5 rounded-full border border-slate-600 text-slate-400">{{ $t('manual.fuelLogsPanel.partial') }}</span>
          </div>
          <div class="text-xs text-slate-400 mt-0.5">
            {{ fmtMoney(log.amount) }}
            <template v-if="log.liters"> · {{ fmtNum(log.liters, 2) }} L</template>
            <template v-if="log.price_per_liter"> · {{ fmtMoney(log.price_per_liter, 3) }}/L</template>
            <template v-if="log.consumption_l_100km"> · <span class="text-success-300">{{ log.segment_estimated ? '≈ ' : '' }}{{ formatPerDistanceValue(log.consumption_l_100km, 2) }} L/100 {{ distanceUnit() }}</span></template>
            <template v-if="log.cost_per_km"> · {{ fmtMoney(perDistance(log.cost_per_km), 3) }}/{{ distanceUnit() }}</template>
          </div>
        </div>
        <div v-if="canEdit" class="flex items-center gap-1.5 shrink-0">
          <button type="button" :aria-label="$t('manual.fuelLogsPanel.editFillUp', { date: fmtDate(log.date) })" class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300" @click="openEdit(log)">
            <Pencil class="w-4 h-4" />
          </button>
          <button type="button" :aria-label="$t('manual.fuelLogsPanel.deleteFillUp', { date: fmtDate(log.date) })" class="p-2 rounded-lg bg-slate-800 hover:bg-red-900/60 text-red-300" @click="remove(log)">
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </li>
    </ul>

    <!-- Add / edit modal -->
    <div v-if="showForm" class="fixed inset-0 z-50 bg-black/60 flex items-center justify-center p-4 overflow-y-auto">
      <form v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full p-5 space-y-3.5 my-auto shadow-2xl" @submit.prevent="save">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-bold text-white">{{ editingId ? $t('manual.fuelLogsPanel.edit') : $t('manual.fuelLogsPanel.new') }}</h3>
          <button type="button" :aria-label="$t('common.close')" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800" @click="showForm = false">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="fuel-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('common.date') }}</label>
            <AppDatePicker id="fuel-date" v-model="form.date" required size="sm" />
          </div>
          <div>
            <label for="fuel-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.fuelLogsPanel.mileageKmOptional', { unit: distanceUnit() }) }}</label>
            <DistanceInput id="fuel-odometer" v-model="form.odometer" inputmode="numeric" min="0" step="1" class="field" />
          </div>
        </div>

        <div class="grid grid-cols-3 gap-3">
          <div>
            <label for="fuel-amount" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.fuelLogsPanel.amount', { cur: currencySymbol(currency) }) }}</label>
            <input id="fuel-amount" v-model.number="form.amount" type="number" inputmode="decimal" min="0.01" step="0.01" class="field" />
          </div>
          <div>
            <label for="fuel-liters" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.fuelLogsPanel.litres') }}</label>
            <input id="fuel-liters" v-model.number="form.liters" type="number" inputmode="decimal" min="0.01" step="0.01" class="field" />
          </div>
          <div>
            <label for="fuel-price" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.fuelLogsPanel.priceL', { cur: currencySymbol(currency) }) }}</label>
            <input id="fuel-price" v-model.number="form.price_per_liter" type="number" inputmode="decimal" min="0.001" step="0.001" class="field" />
          </div>
        </div>
        <p class="text-xs text-slate-400 -mt-1.5">
          {{ $t('manual.fuelLogsPanel.enterTheAmountOrThe', { derivedHint }) }}
        </p>
        <p v-if="!hasOdometer(form.odometer)" class="text-xs text-slate-400 -mt-2">
          {{ $t('manual.fuelLogsPanel.withoutAMileageItIs') }}
        </p>

        <div class="grid grid-cols-2 gap-3 items-end">
          <div>
            <label for="fuel-type" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.fuelLogsPanel.fuel') }}</label>
            <select id="fuel-type" v-model="form.fuel_type" class="field">
              <option v-for="t in fuelTypes" :key="t.value" :value="t.value">{{ t.labelKey ? $t(t.labelKey) : t.label }}</option>
            </select>
          </div>
          <label for="fuel-full" class="flex items-center gap-2 text-xs text-slate-300 pb-2">
            <input id="fuel-full" v-model="form.is_full_tank" type="checkbox" class="rounded border-slate-600 bg-slate-800" />
            {{ $t('manual.fuelLogsPanel.fullTank') }}
          </label>
        </div>

        <div>
          <label for="fuel-notes" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.fuelLogsPanel.notesOptional') }}</label>
          <input id="fuel-notes" v-model="form.notes" maxlength="200" class="field" />
        </div>

        <p v-if="formError" class="text-xs text-red-300" role="alert">{{ formError }}</p>

        <div class="flex justify-end gap-2 pt-1">
          <button type="button" class="btn btn-lg btn-secondary" @click="showForm = false">{{ $t('common.cancel') }}</button>
          <button type="submit" :disabled="saving" class="btn btn-lg btn-primary">
            {{ saving ? $t('manual.fuelLogsPanel.saving') : $t('common.save') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
