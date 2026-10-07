<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { api } from '@/services/api'
import QuickFormShell from './QuickFormShell.vue'
import QuickPhotoField from './QuickPhotoField.vue'
import QuickOdometerField from './QuickOdometerField.vue'
import QuickDateLine from './QuickDateLine.vue'
import { currencySymbol, formatAmount } from '@/currency'
import { formatDistance } from '@/units'
import { formatNumber } from '@/utils/numbers'
import {
  buildChargePayload,
  checkOdometer,
  costFromTariff,
  isQueued,
  loadMemory,
  rememberCharge,
  totalFromUnitPrice,
  unitPriceText,
  toLocalDateTimeInput,
  toNumber,
} from '@/utils/quickAdd'

const props = defineProps<{ vehicle: any }>()
const currency: string = props.vehicle.currency || 'EUR'
const emit = defineEmits<{ saved: [result: { queued: boolean; message: string }] }>()

const memory = loadMemory(props.vehicle.id)
const odometer = props.vehicle.current_odometer ? String(Math.round(props.vehicle.current_odometer)) : ''

const form = reactive({
  date: toLocalDateTimeInput(new Date()),
  kwh: '',
  price: memory.pricePerKwh ? String(memory.pricePerKwh) : '',
  cost: '',
  address: memory.address ?? '',
  odometer,
  notes: '',
  documentId: null as string | null,
  documentFilename: null as string | null,
})

const saving = ref(false)
const error = ref('')
const showDetails = ref(false)
const odometerConfirmed = ref(false)
watch(odometerConfirmed, () => (error.value = ''))
const kwhInput = ref<HTMLInputElement | null>(null)

onMounted(() => kwhInput.value?.focus())

// The price per kWh starts at the last tariff used on this vehicle. The field edited last decides the other:
// a price gives the cost for the energy, a cost gives the price paid.
type Driver = 'tariff' | 'price' | 'cost'
const driver = ref<Driver>('tariff')

watch(
  () => form.kwh,
  (value) => {
    const kwh = toNumber(value)
    if (driver.value === 'cost') {
      form.price = unitPriceText(toNumber(form.cost), kwh, 4)
      return
    }
    const cost = totalFromUnitPrice(kwh, toNumber(form.price))
    form.cost = cost === null ? '' : cost.toFixed(2)
  },
)

function onPriceInput() {
  driver.value = 'price'
  const cost = totalFromUnitPrice(toNumber(form.kwh), toNumber(form.price))
  form.cost = cost === null ? '' : cost.toFixed(2)
}

function onCostInput() {
  driver.value = 'cost'
  form.price = unitPriceText(toNumber(form.cost), toNumber(form.kwh), 4)
}

const fmtPrice = (v: number) => formatAmount(v, currency, 4)
const followsTariff = computed(() => driver.value === 'tariff' && memory.pricePerKwh !== undefined && form.cost !== '')

function setFree() {
  driver.value = 'cost'
  form.cost = '0'
  form.price = '0'
}

async function submit() {
  error.value = ''
  if (checkOdometer(form.odometer, props.vehicle.current_odometer) === 'below' && !odometerConfirmed.value) {
    showDetails.value = true
    error.value = t('quickadd.odometerField.confirmRequired', { last: formatDistance(Math.round(props.vehicle.current_odometer)) })
    return
  }
  let payload: ReturnType<typeof buildChargePayload>
  try {
    payload = buildChargePayload({ ...form }, currency)
  } catch (err: any) {
    error.value = err.message
    return
  }
  saving.value = true
  try {
    const result = await api.createCharge(props.vehicle.id, payload)
    rememberCharge(props.vehicle.id, { kwh: payload.kwh_added, cost: payload.cost, address: payload.address })
    emit('saved', { queued: isQueued(result), message: t('quickadd.quickChargeForm.message', { kwh: formatNumber(payload.kwh_added, 2) }) })
  } catch (err: any) {
    error.value = err?.message || t('quickadd.quickChargeForm.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <QuickFormShell :submit-label="$t('quickadd.quickChargeForm.saveTheCharge')" :saving="saving" :error="error" @submit="submit">
    <p class="text-xs text-slate-400">{{ $t('quickadd.quickChargeForm.chargeOutsideTeslamateThirdParty') }}</p>

    <div>
      <label for="qc-kwh" class="quick-label">{{ $t('quickadd.quickChargeForm.energyAddedKwh') }}</label>
      <NumberInput text id="qc-kwh" ref="kwhInput" v-model="form.kwh" min="0" class="quick-input" />
    </div>

    <div>
      <label for="qc-price" class="quick-label">{{ $t('quickadd.quickChargeForm.pricePerKwh', { cur: currencySymbol(currency) }) }}</label>
      <NumberInput text id="qc-price" v-model="form.price" min="0" class="quick-input" @input="onPriceInput" />
    </div>

    <div>
      <label for="qc-cost" class="quick-label">{{ $t('quickadd.quickChargeForm.cost', { cur: currencySymbol(currency) }) }}</label>
      <div class="flex gap-2">
        <NumberInput text id="qc-cost" v-model="form.cost" min="0" class="quick-input min-w-0" @input="onCostInput" />
        <button type="button" class="quick-chip shrink-0 border-slate-700 bg-slate-800 text-slate-200 hover:bg-slate-700" @click="setFree">{{ $t('quickadd.quickChargeForm.free') }}</button>
      </div>
      <p class="mt-1.5 min-h-4 text-xs text-slate-400" aria-live="polite">
        <template v-if="followsTariff">{{ $t('quickadd.quickChargeForm.calculatedAtTheLastRate', { pricePerKwh: fmtPrice(memory.pricePerKwh!) }) }}</template>
        <template v-else-if="toNumber(form.cost) === 0">{{ $t('expenses.chargeModal.freeChargeHelp') }}</template>
      </p>
    </div>

    <QuickDateLine id="qc-date" v-model="form.date" with-time />

    <button
      type="button"
      class="flex min-h-11 w-full items-center justify-between rounded-xl px-1 text-sm font-semibold text-slate-300"
      :aria-expanded="showDetails"
      aria-controls="qc-details"
      @click="showDetails = !showDetails"
    >
      {{ $t('quickadd.quickChargeForm.moreDetails') }}
      <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': showDetails }" aria-hidden="true" />
    </button>

    <div v-show="showDetails" id="qc-details" class="space-y-4">
      <div>
        <label for="qc-address" class="quick-label">{{ $t('quickadd.quickChargeForm.place') }}</label>
        <input id="qc-address" v-model="form.address" :placeholder="$t('quickadd.quickChargeForm.chargerHome')" autocomplete="off" class="quick-input" />
      </div>
      <QuickOdometerField id="qc-odometer" v-model="form.odometer" v-model:confirmed="odometerConfirmed" :last="vehicle.current_odometer" />
      <div>
        <label for="qc-notes" class="quick-label">{{ $t('common.notes') }}</label>
        <input id="qc-notes" v-model="form.notes" autocomplete="off" class="quick-input" />
      </div>
    </div>

    <QuickPhotoField
      v-model:document-id="form.documentId"
      v-model:filename="form.documentFilename"
      :vehicle-id="vehicle.id"
    />
  </QuickFormShell>
</template>
