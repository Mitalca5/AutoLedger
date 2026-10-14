<script setup lang="ts">
import { t } from '@/i18n'
import { formatMonthLabel } from '@/utils/dates'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { computeMonthFixedVariable, filterMonthsByRange, monthlyCostSeries, type MonthDetailMode } from '@/utils/dashboard'
import { usePreferencesStore } from '@/stores/preferences'
import { currencySymbol, formatAmount, formatAxisAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import ChartTable from '@/components/charts/ChartTable.vue'

Chart.register(...registerables)

// Stacked monthly costs per item; a click on a month opens its detail
const props = defineProps<{ monthlyCosts: any[] | null }>()
const emit = defineEmits<{ 'open-month': [month: any] }>()
const vehicleStore = useVehicleStore()
const currency = computed(() => vehicleStore.currency)

const monthlyChartRef = ref<HTMLCanvasElement | null>(null)
const prefs = usePreferencesStore()
const monthlyChartRange = computed(() => prefs.dashboardRange)
const filteredMonthlyCosts = computed(() => filterMonthsByRange(props.monthlyCosts || [], monthlyChartRange.value))
let monthlyChartInstance: Chart | null = null

// Economic: each cost spread over the period it covers, so a one-off purchase does not crush the other months.
// Cash: what was paid in the month.
const mode = ref<MonthDetailMode>('economic')
const seriesLabel = (key: string) =>
  ({
    energy: t('dashboard.donut.energy'),
    tolls: t('dashboard.donut.tolls'),
    tires: t('dashboard.monthlyCostChart.tires'),
    maintenance: t('dashboard.monthlyCostChart.maintenance'),
    insurance: t('dashboard.donut.insurance'),
    financing: t('dashboard.monthlyCostChart.financing'),
    other: t('dashboard.donut.other'),
  })[key] ?? key
// The same figures as a table for screen readers: one column per cost item, the month total last
const tableData = computed(() => {
  const { series, totals } = monthlyCostSeries(filteredMonthlyCosts.value, mode.value)
  return {
    headers: [t('dashboard.monthlyCostChart.month'), ...series.map((s) => seriesLabel(s.key)), t('dashboard.monthlyCostChart.total')],
    rows: filteredMonthlyCosts.value.map((m: any, i: number) => [
      formatMonthLabel(m.month),
      ...series.map((s) => formatAmount(s.data[i], currency.value)),
      formatAmount(totals[i], currency.value),
    ]),
  }
})

function renderChart() {
  if (!monthlyChartRef.value || !props.monthlyCosts) return
  if (monthlyChartInstance) monthlyChartInstance.destroy()

  const filteredList = filteredMonthlyCosts.value
  const filteredLabels = filteredList.map((m: any) => formatMonthLabel(m.month))
  const { series, totals } = monthlyCostSeries(filteredList, mode.value)

  monthlyChartInstance = new Chart(monthlyChartRef.value, {
    type: 'bar',
    data: {
      labels: filteredLabels,
      datasets: series.map((s) => ({
        label: `${seriesLabel(s.key)} (${currencySymbol(currency.value)})`,
        data: s.data,
        backgroundColor: s.color,
        borderRadius: 4,
      })),
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: {
        mode: 'index',
        intersect: false,
      },
      onClick: (event: any) => {
        if (!monthlyChartInstance) return
        const points = monthlyChartInstance.getElementsAtEventForMode(
          event.native || event,
          'index',
          { intersect: false },
          true
        )
        if (points && points.length > 0) {
          const dataIndex = points[0].index
          const monthItem = filteredList[dataIndex]
          if (monthItem) {
            emit('open-month', monthItem)
          }
        }
      },
      onHover: (event: any, elements: any[]) => {
        const canvas = monthlyChartRef.value
        if (canvas) {
          canvas.style.cursor = elements && elements.length > 0 ? 'pointer' : 'default'
        }
      },
      plugins: {
        legend: { position: 'top', labels: { color: '#94a3b8', font: { size: 11 } } },
        tooltip: {
          callbacks: {
            label: (context) => {
              return `${context.dataset.label} : ${formatAmount(Number(context.raw || 0), currency.value)}`
            },
            footer: (items) => {
              const dataIndex = items[0]?.dataIndex
              const total = dataIndex !== undefined ? totals[dataIndex] : items.reduce((sum, item) => sum + (Number(item.raw) || 0), 0)
              const lines = [t('dashboard.monthlyCostChart.monthTotal', { total: formatAmount(total, currency.value) })]
              if (dataIndex !== undefined) {
                const monthItem = filteredList[dataIndex]
                const fv = computeMonthFixedVariable(monthItem, mode.value)
                if (fv.totalAmount > 0) {
                  lines.push(t('dashboard.monthlyCostChart.fixedVariableTooltip', { fixedPct: fv.fixedPct, variablePct: fv.variablePct }))
                }
              }
              return lines
            },
          },
        },
      },
      scales: {
        x: { stacked: true, grid: { color: '#1e293b' }, ticks: { color: '#64748b' } },
        y: {
          stacked: true,
          beginAtZero: true,
          grid: { color: '#1e293b' },
          ticks: { color: '#64748b', callback: (v) => formatAxisAmount(Number(v), currency.value) },
        },
      },
    },
  })
}

onMounted(renderChart)
watch([() => props.monthlyCosts, monthlyChartRange, mode], renderChart, { flush: 'post' })
onUnmounted(() => {
  if (monthlyChartInstance) monthlyChartInstance.destroy()
})
</script>

<template>
  <div class="lg:col-span-2 flex flex-col bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
      <h3 class="text-sm font-bold text-white flex items-center gap-2">
        <span>{{ $t('dashboard.monthlyCostChart.monthlyExpenseTrend', { cur: currencySymbol(currency) }) }}</span>
      </h3>
      <div class="flex items-center gap-1 bg-slate-800/60 rounded-lg p-0.5 self-start" role="group" :aria-label="$t('dashboard.monthlyCostChart.view')">
        <button
          v-for="opt in (['economic', 'cash'] as const)"
          :key="opt"
          type="button"
          :aria-pressed="mode === opt"
          :class="['tap-text justify-center px-2.5 text-xs font-semibold rounded-md transition-colors pointer-coarse:min-w-11', mode === opt ? 'bg-rose-600 text-white' : 'text-slate-400 hover:text-white']"
          @click="mode = opt"
        >
          {{ opt === 'economic' ? $t('dashboard.monthlyCostChart.economic') : $t('dashboard.monthlyCostChart.cash') }}
        </button>
      </div>
    </div>
    <p class="-mt-2 mb-3 text-xs text-slate-400">{{ mode === 'economic' ? $t('dashboard.monthlyCostChart.economicHint') : $t('dashboard.monthlyCostChart.cashHint') }}</p>
    <div class="relative min-h-52 flex-1 sm:min-h-72">
      <canvas ref="monthlyChartRef" role="img" :aria-label="`${$t('dashboard.monthlyCostChart.monthlyCostTrendByCategory')} · ${mode === 'economic' ? $t('dashboard.monthlyCostChart.economic') : $t('dashboard.monthlyCostChart.cash')}`"></canvas>
    </div>
    <ChartTable :caption="$t('dashboard.monthlyCostChart.monthlyCostTrendByCategory')" :headers="tableData.headers" :rows="tableData.rows" />
  </div>
</template>
