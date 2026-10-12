<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { distanceUnit, formatDistanceValue, kmToDisplayDistance } from '@/units'
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { Chart, registerables } from 'chart.js'
import { downloadCsv } from '@/utils/csv'
import { useVehicleStore } from '@/stores/vehicle'
import { scenarioSide, sideKey } from '@/utils/comparisonSide'
import { comparisonCurrency, canCompareCurrencies } from '@/utils/comparisonCurrency'

Chart.register(...registerables)

const props = defineProps<{
  items: { scenario: any; result: any }[]
}>()

const vehicleStore = useVehicleStore()

// Hybrid wording only when every compared scenario tracks a hybrid.
const sk = (key: string) => sideKey(key, props.items.length > 0 && props.items.every((i) => scenarioSide(i.scenario, vehicleStore.vehicles) === 'hybrid') ? 'hybrid' : 'electric')

const commonCurrency = computed(() => canCompareCurrencies(props.items.map(i => i.scenario), vehicleStore.vehicles)
  ? comparisonCurrency(props.items[0]?.scenario, vehicleStore.vehicles) : undefined)

// Never label or plot different currencies as if their amounts were interchangeable.
function fmtMoney(v: number | null | undefined, currency: string, digits = 0): string {
  return Number(v || 0).toLocaleString(intlLocale(), { style: 'currency', currency, minimumFractionDigits: digits, maximumFractionDigits: digits })
}

function breakEven(r: any): string {
  if (r.break_even_year === undefined || r.break_even_year === null) return t('comparison.compare.notReached')
  if (r.break_even_year === 0) return t('comparison.compare.fromPurchase')
  return t('comparison.compare.years', { years: Number(r.break_even_year).toLocaleString(intlLocale()) })
}

const rows = computed(() =>
  props.items.map(({ scenario, result }) => ({
    id: scenario.id,
    name: scenario.name,
    mode: scenario.mode === 'RETROSPECTIVE' ? t('comparison.comparisonView.trackedVehicle') : t('comparison.comparisonView.projection'),
    years: result.years_count,
    km: result.annual_km,
    tracked: result.tracked.total,
    ice: result.ice.total,
    evMonth: result.tracked.per_month,
    iceMonth: result.ice.per_month,
    savings: result.tracked_savings,
    breakEven: breakEven(result),
  }))
)

const canvas = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

function render() {
  chart?.destroy()
  const currency = commonCurrency.value
  if (!canvas.value || !currency) return
  chart = new Chart(canvas.value, {
    type: 'bar',
    data: {
      labels: rows.value.map((r) => r.name),
      datasets: [
        { label: t(sk('comparison.electric')), data: rows.value.map((r) => r.tracked), backgroundColor: '#38bdf8', borderRadius: 4 },
        { label: t('comparison.combustion'), data: rows.value.map((r) => r.ice), backgroundColor: '#f59e0b', borderRadius: 4 },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { labels: { color: '#94a3b8', boxWidth: 12 } },
        tooltip: { callbacks: { label: (ctx) => ` ${ctx.dataset.label} : ${fmtMoney(Number(ctx.raw), currency)}` } },
      },
      scales: {
        x: { ticks: { color: '#94a3b8' }, grid: { color: '#1e293b' } },
        y: { ticks: { color: '#94a3b8', callback: (v) => fmtMoney(Number(v), currency) }, grid: { color: '#1e293b' } },
      },
    },
  })
}

function exportCsv() {
  downloadCsv(
    t('comparison.csv.scenariosFile'),
    t(sk('comparison.csv.scenariosHeader'), { unit: distanceUnit(), cur: commonCurrency.value }).split(','),
    rows.value.map((r) => [
      `"${r.name.replace(/"/g, '""')}"`,
      r.mode,
      r.years,
      Math.round(kmToDisplayDistance(r.km)),
      Number(r.tracked).toFixed(2),
      Number(r.ice).toFixed(2),
      Number(r.savings).toFixed(2),
      r.breakEven,
    ])
  )
}

onMounted(async () => {
  await nextTick()
  render()
})
onBeforeUnmount(() => chart?.destroy())
</script>

<template>
  <p v-if="!commonCurrency" class="text-warning-300">{{ $t('comparison.currency.same') }}</p>
  <div v-else class="space-y-5 print-area">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4 overflow-x-auto">
      <div class="flex items-center justify-between mb-3 gap-3">
        <h2 class="text-sm font-semibold text-white">{{ $t('comparison.comparisonCompare.comparisonOfScenarios', { length: rows.length }) }}</h2>
        <button type="button" class="btn btn-secondary no-print" @click="exportCsv">
          {{ $t('comparison.comparisonCompare.exportAsCsv') }}
        </button>
      </div>
      <table class="w-full text-sm">
        <caption class="sr-only">{{ $t('comparison.comparisonCompare.totalsPerScenario') }}</caption>
        <thead>
          <tr class="text-xs text-slate-400 text-right">
            <th scope="col" class="text-left font-semibold pb-2">{{ $t('comparison.comparisonCompare.scenario') }}</th>
            <th scope="col" class="font-semibold pb-2">{{ $t(sk('comparison.comparisonCompare.electric')) }}</th>
            <th scope="col" class="font-semibold pb-2">{{ $t('comparison.comparisonCompare.combustion') }}</th>
            <th scope="col" class="font-semibold pb-2">{{ $t('comparison.comparisonCompare.gap') }}</th>
            <th scope="col" class="font-semibold pb-2">{{ $t('comparison.comparisonCompare.breakEven') }}</th>
          </tr>
        </thead>
        <tbody class="text-slate-200">
          <tr v-for="r in rows" :key="r.id" class="border-t border-slate-800 text-right">
            <th scope="row" class="text-left font-normal py-2">
              <div class="font-semibold text-white">{{ r.name }}</div>
              <div class="text-xs text-slate-400">{{ r.mode }} · {{ $t('comparison.comparisonCompare.usage', { unit: distanceUnit(), km: formatDistanceValue(r.km), years: r.years }) }}</div>
            </th>
            <td>{{ fmtMoney(r.tracked, commonCurrency) }}<div class="text-xs text-slate-400">{{ $t('comparison.comparisonCompare.month2', { evMonth: fmtMoney(r.evMonth, commonCurrency) }) }}</div></td>
            <td>{{ fmtMoney(r.ice, commonCurrency) }}<div class="text-xs text-slate-400">{{ $t('comparison.comparisonCompare.month', { iceMonth: fmtMoney(r.iceMonth, commonCurrency) }) }}</div></td>
            <td :class="r.savings >= 0 ? 'text-success-400' : 'text-warning-400'">
              {{ r.savings >= 0 ? '−' : '+' }}{{ fmtMoney(Math.abs(r.savings), commonCurrency) }}
              <div class="text-xs text-slate-400">{{ r.savings >= 0 ? $t(sk('comparison.compare.saves')) : $t(sk('comparison.compare.costsMore')) }}</div>
            </td>
            <td>{{ r.breakEven }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
      <h2 class="text-sm font-semibold text-white mb-3">{{ $t('comparison.comparisonCompare.totalCostPerScenario') }}</h2>
      <div class="h-72"><canvas ref="canvas" :aria-label="$t(sk('comparison.comparisonCompare.totalElectricAndCombustionCost'))"></canvas></div>
    </div>
  </div>
</template>
