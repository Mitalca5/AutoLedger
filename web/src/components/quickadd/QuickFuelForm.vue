<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { api } from '@/services/api'
import QuickFormShell from './QuickFormShell.vue'
import { currencySymbol, formatAmount } from '@/currency'
import { buildFuelPayload, isQueued, toLocalDateInput, toNumber } from '@/utils/quickAdd'
import { distanceUnit } from '@/units'

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
      <div>
        <label for="qf-date" class="quick-label">{{ $t('common.date') }}</label>
        <input id="qf-date" v-model="form.date" type="date" class="quick-input" />
      </div>
      <div>
        <label for="qf-odometer" class="quick-label">{{ $t('quickadd.quickFuelForm.odometerKm', { unit: distanceUnit() }) }}</label>
        <DistanceInput text id="qf-odometer" v-model="form.odometer" inputmode="numeric" min="0" class="quick-input" />
      </div>
      <div>
        <label for="qf-notes" class="quick-label">{{ $t('common.notes') }}</label>
        <input id="qf-notes" v-model="form.notes" maxlength="200" autocomplete="off" class="quick-input" />
      </div>
    </div>
  </QuickFormShell>
</template>
