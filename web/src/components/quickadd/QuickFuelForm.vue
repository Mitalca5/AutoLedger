<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { api } from '@/services/api'
import QuickFormShell from './QuickFormShell.vue'
import QuickOdometerField from './QuickOdometerField.vue'
import QuickDateLine from './QuickDateLine.vue'
import { currencySymbol, formatAmount } from '@/currency'
import { buildFuelPayload, checkOdometer, isQueued, toLocalDateInput, toNumber, totalFromUnitPrice, unitPriceText } from '@/utils/quickAdd'
import { formatDistance } from '@/units'

const props = defineProps<{ vehicle: any }>()
const currency: string = props.vehicle.currency || 'EUR'
const emit = defineEmits<{ saved: [result: { queued: boolean; message: string }] }>()

const form = reactive({
  date: toLocalDateInput(new Date()),
  amount: '',
  liters: '',
  price: '',
  fullTank: true,
  odometer: '',
  notes: '',
})

const saving = ref(false)
const error = ref('')
const showDetails = ref(false)
const odometerConfirmed = ref(false)
watch(odometerConfirmed, () => (error.value = ''))
const amountInput = ref<HTMLInputElement | null>(null)

onMounted(() => amountInput.value?.focus())

// The field edited last decides the other: a price gives the amount for the litres, an amount gives the price paid.
const driver = ref<'amount' | 'price'>('amount')

function sync() {
  const liters = toNumber(form.liters)
  if (driver.value === 'price') {
    const amount = totalFromUnitPrice(liters, toNumber(form.price))
    form.amount = amount === null ? '' : amount.toFixed(2)
  } else {
    form.price = unitPriceText(toNumber(form.amount), liters, 3)
  }
}

function onPriceInput() {
  driver.value = 'price'
  sync()
}

function onAmountInput() {
  driver.value = 'amount'
  sync()
}

async function submit() {
  error.value = ''
  if (checkOdometer(form.odometer, props.vehicle.current_odometer) === 'below' && !odometerConfirmed.value) {
    showDetails.value = true
    error.value = t('quickadd.odometerField.confirmRequired', { last: formatDistance(Math.round(props.vehicle.current_odometer)) })
    return
  }
  let payload: ReturnType<typeof buildFuelPayload>
  try {
    payload = buildFuelPayload({ ...form })
  } catch (err: any) {
    error.value = err.message
    return
  }
  saving.value = true
  try {
    const result = await api.createFuelLog(props.vehicle.id, payload, currency)
    emit('saved', { queued: isQueued(result), message: t('quickadd.quickFuelForm.message', { amount: formatAmount(payload.amount, currency) }) })
  } catch (err: any) {
    error.value = err?.message || t('quickadd.quickFuelForm.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <QuickFormShell :submit-label="$t('quickadd.quickFuelForm.saveTheFillUp')" :saving="saving" :error="error" @submit="submit">
    <div>
      <label for="qf-amount" class="quick-label">{{ $t('quickadd.quickFuelForm.amount', { cur: currencySymbol(currency) }) }}</label>
      <NumberInput text id="qf-amount" ref="amountInput" v-model="form.amount" min="0" class="quick-input" @input="onAmountInput" />
    </div>

    <div>
      <label for="qf-liters" class="quick-label">{{ $t('quickadd.quickFuelForm.quantityL') }}</label>
      <NumberInput text id="qf-liters" v-model="form.liters" min="0" class="quick-input" @input="sync" />
    </div>

    <div>
      <label for="qf-price" class="quick-label">{{ $t('quickadd.quickFuelForm.pricePerLiter', { cur: currencySymbol(currency) }) }}</label>
      <NumberInput text id="qf-price" v-model="form.price" min="0" class="quick-input" @input="onPriceInput" />
    </div>

    <label class="flex min-h-12 cursor-pointer items-center justify-between gap-3 rounded-xl border border-slate-700 bg-slate-800 px-4 text-sm font-semibold text-white">
      {{ $t('quickadd.quickFuelForm.fullTank') }}
      <input v-model="form.fullTank" type="checkbox" class="h-6 w-6 accent-rose-500" />
    </label>

    <QuickDateLine id="qf-date" v-model="form.date" />

    <button
      type="button"
      class="flex min-h-11 w-full items-center justify-between rounded-xl px-1 text-sm font-semibold text-slate-300"
      :aria-expanded="showDetails"
      aria-controls="qf-details"
      @click="showDetails = !showDetails"
    >
      {{ $t('quickadd.quickFuelForm.moreDetails') }}
      <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': showDetails }" aria-hidden="true" />
    </button>

    <div v-show="showDetails" id="qf-details" class="space-y-4">
      <QuickOdometerField id="qf-odometer" v-model="form.odometer" v-model:confirmed="odometerConfirmed" :last="vehicle.current_odometer" />
      <div>
        <label for="qf-notes" class="quick-label">{{ $t('common.notes') }}</label>
        <input id="qf-notes" v-model="form.notes" maxlength="200" autocomplete="off" class="quick-input" />
      </div>
    </div>
  </QuickFormShell>
</template>
