<script setup lang="ts">
import { computed, ref } from 'vue'
import CostDonut from '@/components/costs/CostDonut.vue'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { formatPercent } from '@/utils/numbers'
import { donutRows, type DonutMode } from '@/utils/donutBreakdown'

// Share of each cost item: in the full cost of ownership, or in what was actually paid. The amounts are listed under
// the ring, largest first, so no figure is hidden behind a tooltip.
const props = defineProps<{ tco: any | null }>()
const vehicleStore = useVehicleStore()

const mode = ref<DonutMode>('full')
const modes: DonutMode[] = ['full', 'cash']
const rows = computed(() =>
  donutRows(props.tco, mode.value).map((r) => ({
    ...r,
    label: r.key === 'tires' && mode.value === 'cash' ? 'dashboard.donut.tiresCash' : `dashboard.donut.${r.key}`,
  })),
)
</script>

<template>
  <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
    <div class="flex items-center justify-between gap-2 mb-1">
      <h3 class="text-sm font-bold text-white">{{ $t(mode === 'full' ? 'dashboard.costBreakdownDonut.fullCostBreakdown' : 'dashboard.donut.cashTitle') }}</h3>
      <div class="inline-flex rounded-lg bg-slate-800 p-0.5 text-xs" role="group">
        <button
          v-for="m in modes"
          :key="m"
          type="button"
          :aria-pressed="mode === m"
          class="px-2.5 py-1 rounded-md font-medium transition-colors"
          :class="mode === m ? 'bg-slate-600 text-white' : 'text-slate-400 hover:text-white'"
          @click="mode = m"
        >
          {{ $t(m === 'full' ? 'dashboard.donut.modeFull' : 'dashboard.donut.modeCash') }}
        </button>
      </div>
    </div>
    <p class="text-xs text-slate-400 mb-4">{{ $t(mode === 'full' ? 'dashboard.donut.hintFull' : 'dashboard.donut.hintCash') }}</p>
    <p v-if="mode === 'full' && tco && tco.depreciation_known === false" class="text-xs font-medium text-warning-400 -mt-2 mb-3">{{ $t('dashboard.costBreakdownDonut.depreciationMissing') }}</p>
    <div v-if="tco" class="mx-auto h-36 w-36 sm:h-44 sm:w-44">
      <CostDonut
        :items="rows.map((r) => ({ label: $t(r.label), color: r.color, amount: r.amount }))"
        :currency="vehicleStore.currency"
        :show-legend="false"
        :empty-label="$t('dashboard.costBreakdownDonut.noCost')"
        :chart-label="$t('dashboard.costBreakdownDonut.fullCostBreakdownByCategory')"
      />
    </div>
    <ul class="mt-4 space-y-1.5 text-xs">
      <li v-for="r in rows" :key="r.key" class="flex items-center justify-between gap-3">
        <span class="flex min-w-0 items-center gap-2 text-slate-300">
          <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: r.color }" aria-hidden="true"></span>
          <span class="truncate">{{ $t(r.label) }}</span>
        </span>
        <span class="shrink-0 tabular-nums text-slate-400">
          <span class="font-semibold text-white">{{ formatAmount(r.amount, vehicleStore.currency, 0) }}</span>
          · {{ formatPercent(r.sharePct, 0) }}
        </span>
      </li>
    </ul>
  </div>
</template>
