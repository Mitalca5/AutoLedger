<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { reactive, ref } from 'vue'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { currencySymbol, formatAmount } from '@/currency'
import {
  buildPendingCostPayload,
  costFromTariff,
  isQueued,
  loadMemory,
  rememberCharge,
  type PendingCharge,
} from '@/utils/quickAdd'

const props = defineProps<{
  vehicleId: string
  charges: PendingCharge[]
  total: number
}>()

const emit = defineEmits<{ completed: [result: { id: string; queued: boolean; message: string }] }>()
const vehicleStore = useVehicleStore()

const memory = loadMemory(props.vehicleId)
const costs = reactive<Record<string, string>>({})
const busyId = ref<string | null>(null)
const errors = reactive<Record<string, string>>({})

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleString(intlLocale(), { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
const fmtKwh = (v: number) => v.toLocaleString(intlLocale(), { maximumFractionDigits: 1 })

// Cost the last known tariff would give for this charge, offered as a one-tap suggestion.
function suggestion(c: PendingCharge): string | null {
  const cost = costFromTariff(c.kwh_added, memory.pricePerKwh)
  return cost === null ? null : cost.toFixed(2)
}

async function complete(c: PendingCharge) {
  errors[c.id] = ''
  let payload: ReturnType<typeof buildPendingCostPayload>
  try {
    payload = buildPendingCostPayload(c, costs[c.id] ?? '', vehicleStore.currency)
  } catch (err: any) {
    errors[c.id] = err.message
    return
  }
  busyId.value = c.id
  try {
    const result = await api.updateCharge(props.vehicleId, c.id, payload)
    rememberCharge(props.vehicleId, { kwh: c.kwh_added, cost: payload.cost, address: null })
    emit('completed', { id: c.id, queued: isQueued(result), message: t('quickadd.quickPendingCosts.costAdded', { cost: formatAmount(payload.cost, vehicleStore.currency) }) })
  } catch (err: any) {
    errors[c.id] = err?.message || t('quickadd.quickPendingCosts.saveFailed')
  } finally {
    busyId.value = null
  }
}
</script>

<template>
  <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-4 pb-[max(1rem,env(safe-area-inset-bottom))]">
    <p v-if="charges.length === 0" class="rounded-xl border border-slate-800 bg-slate-800/40 p-4 text-sm text-slate-300">
      {{ total > 0 ? $t('common.loading') : $t('quickadd.quickPendingCosts.allHaveCost') }}
    </p>

    <template v-else>
      <p class="mb-3 text-xs text-slate-400">
        {{ $t('quickadd.quickPendingCosts.teslamateChargesWithoutARate') }}
      </p>
      <ul class="space-y-3">
        <li v-for="c in charges" :key="c.id" class="rounded-xl border border-slate-800 bg-slate-800/40 p-3">
          <div class="flex items-baseline justify-between gap-2 text-sm">
            <span class="min-w-0 truncate text-slate-200">
              {{ fmtDate(c.date) }}<template v-if="c.address"> · {{ c.address }}</template>
            </span>
            <span class="shrink-0 font-semibold text-info-300">{{ $t('quickadd.quickPendingCosts.kwh', { kwh_added: fmtKwh(c.kwh_added) }) }}</span>
          </div>

          <form class="mt-2 flex gap-2" novalidate @submit.prevent="complete(c)">
            <label :for="`qp-cost-${c.id}`" class="sr-only">{{ $t('quickadd.quickPendingCosts.costOfTheChargeOf', { date: fmtDate(c.date), cur: currencySymbol(vehicleStore.currency) }) }}</label>
            <input
              :id="`qp-cost-${c.id}`"
              v-model="costs[c.id]"
              type="number"
              inputmode="decimal"
              step="any"
              min="0"
              :placeholder="$t('quickadd.quickPendingCosts.cost', { cur: currencySymbol(vehicleStore.currency) })"
              class="quick-input min-w-0"
            />
            <button
              type="submit"
              :disabled="busyId === c.id"
              class="min-h-12 shrink-0 rounded-xl bg-rose-600 px-4 text-sm font-bold text-white hover:bg-rose-500 disabled:opacity-60"
            >
              {{ busyId === c.id ? '…' : $t('quickadd.quickPendingCosts.confirm') }}
            </button>
          </form>

          <button
            v-if="suggestion(c)"
            type="button"
            class="mt-2 min-h-11 rounded-lg px-1 text-xs font-semibold text-indigo-300 hover:text-indigo-200"
            @click="costs[c.id] = suggestion(c)!"
          >
            {{ $t('quickadd.quickPendingCosts.applyTheLastRate', { c: formatAmount(Number(suggestion(c)), vehicleStore.currency) }) }}
          </button>
          <p v-if="errors[c.id]" role="alert" class="mt-1 text-xs text-danger-300">{{ errors[c.id] }}</p>
        </li>
      </ul>
      <p v-if="total > charges.length" class="mt-3 text-center text-xs text-slate-400">
        {{ $t('quickadd.quickPendingCosts.otherSToCompleteFrom', { total: total - charges.length }) }}
      </p>
    </template>
  </div>
</template>
