<script setup lang="ts">
import { t } from '@/i18n'
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ChevronDown, Wrench } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useQuickAddStore } from '@/stores/quickAdd'
import QuickFormShell from './QuickFormShell.vue'
import QuickPhotoField from './QuickPhotoField.vue'
import QuickDateLine from './QuickDateLine.vue'
import { currencySymbol, formatAmount } from '@/currency'
import { buildExpensePayload, isQueued, toLocalDateTimeInput, type ExpenseType } from '@/utils/quickAdd'

const props = defineProps<{ vehicle: any }>()
const currency: string = props.vehicle.currency || 'EUR'
const emit = defineEmits<{ saved: [result: { queued: boolean; message: string }] }>()

const types: { value: ExpenseType; label: string }[] = [
  { value: 'TOLL', label: 'quickadd.quickExpenseForm.toll' },
  { value: 'PARKING', label: 'quickadd.quickExpenseForm.parking' },
  { value: 'FERRY', label: 'quickadd.quickExpenseForm.ferry' },
  { value: 'OTHER', label: 'quickadd.quickExpenseForm.other' },
]

const form = reactive({
  type: 'TOLL' as ExpenseType,
  date: toLocalDateTimeInput(new Date()),
  amount: '',
  notes: '',
  documentId: null as string | null,
  documentFilename: null as string | null,
})

const router = useRouter()
const quickAdd = useQuickAddStore()

// A maintenance carries a category, a coverage and documents: the full form opens with today's date and the detected odometer.
function openMaintenance() {
  quickAdd.close()
  router.push({ path: '/expenses', query: { tab: 'MAINTENANCE', add: 'maintenance' } })
}

const saving = ref(false)
const error = ref('')
const showDetails = ref(false)
const amountInput = ref<HTMLInputElement | null>(null)

onMounted(() => amountInput.value?.focus())

async function submit() {
  error.value = ''
  let payload: ReturnType<typeof buildExpensePayload>
  try {
    payload = buildExpensePayload({ ...form }, currency)
  } catch (err: any) {
    error.value = err.message
    return
  }
  saving.value = true
  try {
    const result = await api.createDriveExpense(props.vehicle.id, payload)
    const label = t(types.find((type) => type.value === payload.type)?.label ?? 'quickadd.quickAddSheet.tabExpense')
    emit('saved', { queued: isQueued(result), message: t('quickadd.quickExpenseForm.message', { label, amount: formatAmount(payload.amount, currency) }) })
  } catch (err: any) {
    error.value = err?.message || t('quickadd.quickExpenseForm.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <QuickFormShell :submit-label="$t('quickadd.quickExpenseForm.saveTheExpense')" :saving="saving" :error="error" @submit="submit">
    <div role="radiogroup" :aria-label="$t('quickadd.quickExpenseForm.expenseType')" class="grid grid-cols-4 gap-2">
      <button
        v-for="t in types"
        :key="t.value"
        type="button"
        role="radio"
        :aria-checked="form.type === t.value"
        class="quick-chip"
        :class="form.type === t.value ? 'border-rose-500/50 bg-rose-500/15 text-rose-200' : 'border-slate-700 bg-slate-800 text-slate-300 hover:bg-slate-700'"
        @click="form.type = t.value"
      >
        {{ $t(t.label) }}
      </button>
    </div>

    <button
      type="button"
      class="flex min-h-11 w-full items-center justify-center gap-2 rounded-xl border border-dashed border-slate-600 px-3 text-sm font-semibold text-slate-300 transition-colors hover:border-slate-500 hover:bg-slate-800"
      @click="openMaintenance"
    >
      <Wrench class="h-4 w-4" aria-hidden="true" />
      {{ $t('quickadd.quickExpenseForm.maintenance') }}
    </button>

    <div>
      <label for="qe-amount" class="quick-label">{{ $t('quickadd.quickExpenseForm.amount', { cur: currencySymbol(currency) }) }}</label>
      <input id="qe-amount" ref="amountInput" v-model="form.amount" type="number" inputmode="decimal" step="any" min="0" class="quick-input" />
    </div>

    <QuickDateLine id="qe-date" v-model="form.date" with-time />

    <button
      type="button"
      class="flex min-h-11 w-full items-center justify-between rounded-xl px-1 text-sm font-semibold text-slate-300"
      :aria-expanded="showDetails"
      aria-controls="qe-details"
      @click="showDetails = !showDetails"
    >
      {{ $t('quickadd.quickExpenseForm.moreDetails') }}
      <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': showDetails }" aria-hidden="true" />
    </button>

    <div v-show="showDetails" id="qe-details" class="space-y-4">
      <div>
        <label for="qe-notes" class="quick-label">{{ $t('common.notes') }}</label>
        <input id="qe-notes" v-model="form.notes" autocomplete="off" class="quick-input" />
      </div>
    </div>

    <QuickPhotoField
      v-model:document-id="form.documentId"
      v-model:filename="form.documentFilename"
      :vehicle-id="vehicle.id"
    />
  </QuickFormShell>
</template>
