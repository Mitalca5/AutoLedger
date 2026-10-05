<script setup lang="ts">
import { t } from '@/i18n'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { api } from '@/services/api'
import QuickFormShell from './QuickFormShell.vue'
import QuickOdometerField from './QuickOdometerField.vue'
import QuickDateLine from './QuickDateLine.vue'
import { currencySymbol, formatAmount } from '@/currency'
import { buildFuelPayload, checkOdometer, isQueued, toLocalDateInput, toNumber } from '@/utils/quickAdd'
import { formatDistance } from '@/units'

const props = defineProps<{ vehicle: any }>()
const currency: string = props.vehicle.currency || 'EUR'
const emit = defineEmits<{ saved: [result: { queued: boolean; message: string }] }>()

const form = reactive({
  date: toLocalDateInput(new Date()),
  amount: '',
  liters: '',
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

const pricePerLiter = computed(() => {
  const amount = toNumber(form.amount)
  const liters = toNumber(form.liters)
  if (amount === null || liters === null || amount <= 0 || liters <= 0) return null
  return amount / liters
})
const fmtPrice = (v: number) => formatAmount(v, currency, 3)

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
      <input id="qf-amount" ref="amountInput" v-model="form.amount" type="number" inputmode="decimal" step="any" min="0" class="quick-input" />
    </div>

    <div>
      <label for="qf-liters" class="quick-label">{{ $t('quickadd.quickFuelForm.quantityL') }}</label>
      <input id="qf-liters" v-model="form.liters" type="number" inputmode="decimal" step="any" min="0" class="quick-input" />
      <p class="mt-1.5 min-h-4 text-xs text-slate-400" aria-live="polite">
        <template v-if="pricePerLiter !== null">{{ $t('quickadd.quickFuelForm.thatIsL', { pricePerLiter: fmtPrice(pricePerLiter) }) }}</template>
      </p>
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
