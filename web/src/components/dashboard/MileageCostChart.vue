<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { formatMonthLabel } from '@/utils/dates'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Activity, PieChart } from 'lucide-vue-next'
import { filterMonthsByRange, MIN_PLOTTED_DISTANCE_KM, plottedCostPerKm } from '@/utils/dashboard'
import { usePreferencesStore } from '@/stores/preferences'
import { currencySymbol, formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import ChartTable from '@/components/charts/ChartTable.vue'
import { distanceUnit, formatDistanceValue, kmToDisplayDistance, perDistance } from '@/units'

Chart.register(...registerables)

// Monthly distance and cost per distance, each on its own chart and axis; a click on a month opens its detail
const props = defineProps<{ monthlyCosts: any[] | null }>()
const emit = defineEmits<{ 'open-month': [month: any] }>()
const vehicleStore = useVehicleStore()

const distanceRef = ref<HTMLCanvasElement | null>(null)
const costRef = ref<HTMLCanvasElement | null>(null)
const prefs = usePreferencesStore()
const range = computed(() => prefs.dashboardRange)
const months = computed(() => filterMonthsByRange(props.monthlyCosts || [], range.value))
let distanceChart: Chart | null = null
let costChart: Chart | null = null

// The same figures as a table for screen readers
const tableRows = computed(() =>
  months.value.map((m: any) => {
    const cost = plottedCostPerKm(m)
    return [
      formatMonthLabel(m.month),
      formatDistanceValue(m.distance_km || 0),
      cost == null ? '—' : formatAmount(perDistance(cost), vehicleStore.currency, 3),
    ]
  }),
)

const AXIS = '#64748b'
const GRID = '#1e293b'

function baseOptions(canvas: () => HTMLCanvasElement | null, chart: () => Chart | null, list: any[], tooltipLabel: (v: number) => string, tick: (v: number) => string) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    onClick: (event: any) => {
      const points = chart()?.getElementsAtEventForMode(event.native || event, 'index', { intersect: false }, true)
      const month = points?.length ? list[points[0].index] : null
      if (month) emit('open-month', month)
    },
    onHover: (_event: any, elements: any[]) => {
      const el = canvas()
      if (el) el.style.cursor = elements?.length ? 'pointer' : 'default'
    },
    plugins: {
      legend: { display: false },
      tooltip: { callbacks: { label: (context: any) => tooltipLabel(Number(context.raw)) } },
    },
    scales: {
      x: { grid: { color: GRID }, ticks: { color: AXIS } },
      y: { beginAtZero: true, grid: { color: GRID }, ticks: { color: AXIS, callback: (v: any) => tick(Number(v)) } },
    },
  }
}

function renderCharts() {
  distanceChart?.destroy()
  costChart?.destroy()
  distanceChart = null
  costChart = null
  if (!props.monthlyCosts) return
  const list = months.value
  const labels = list.map((m: any) => formatMonthLabel(m.month))

  if (distanceRef.value) {
    distanceChart = new Chart(distanceRef.value, {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            label: t('dashboard.mileageCostChart.kmDriven', { unit: distanceUnit() }),
            data: list.map((m: any) => Math.round(kmToDisplayDistance(m.distance_km || 0))),
            backgroundColor: 'rgba(56, 189, 248, 0.7)',
            hoverBackgroundColor: 'rgba(56, 189, 248, 0.95)',
            borderRadius: 6,
          },
        ],
      },
      options: baseOptions(
        () => distanceRef.value,
        () => distanceChart,
        list,
        (v) => t('dashboard.mileageCostChart.tooltipDistance', { unit: distanceUnit(), distance: v.toLocaleString(intlLocale()) }),
        (v) => v.toLocaleString(intlLocale()),
      ),
    })
  }

  if (costRef.value) {
    costChart = new Chart(costRef.value, {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: t('dashboard.mileageCostChart.averageCostPerKm', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) }),
            // A month under the minimum distance, or without a known cost, is a gap rather than a point
            data: list.map((m: any) => {
              const v = plottedCostPerKm(m)
              return v == null ? null : perDistance(v)
            }),
            borderColor: '#38bdf8',
            backgroundColor: '#38bdf8',
            borderWidth: 2.5,
            pointRadius: 4,
            pointHoverRadius: 6,
            tension: 0.3,
            spanGaps: false,
          },
        ],
      },
      options: baseOptions(
        () => costRef.value,
        () => costChart,
        list,
        (v) => t('dashboard.mileageCostChart.tooltipCostPerKm', { unit: distanceUnit(), cost: formatAmount(v, vehicleStore.currency, 3) }),
        (v) => formatAmount(v, vehicleStore.currency, 2),
      ),
    })
  }
}

onMounted(renderCharts)
watch([() => props.monthlyCosts, range], renderCharts, { flush: 'post' })
onUnmounted(() => {
  distanceChart?.destroy()
  costChart?.destroy()
})
</script>

<template>
  <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm space-y-3">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
      <div>
        <h3 class="text-sm font-bold text-white flex items-center gap-2">
          <Activity class="w-4 h-4 text-slate-400" aria-hidden="true" />
          <span>{{ $t('dashboard.mileageCostChart.monthlyMileageAndCostPer', { unit: distanceUnit() }) }}</span>
        </h3>
        <p class="text-xs text-slate-400 mt-0.5">{{ $t('dashboard.mileageCostChart.clickABarOfThe') }}</p>
      </div>
      <button
        v-if="months.length"
        type="button"
        class="self-start sm:self-auto px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-colors"
        @click="emit('open-month', months[months.length - 1])"
      >
        <PieChart class="w-3.5 h-3.5" aria-hidden="true" />
        <span>{{ $t('dashboard.mileageCostChart.lastMonthSDetail') }}</span>
      </button>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div>
        <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.mileageCostChart.distanceKm', { unit: distanceUnit() }) }}</h4>
        <div class="h-40 sm:h-52">
          <canvas ref="distanceRef" role="img" :aria-label="$t('dashboard.mileageCostChart.distanceKm', { unit: distanceUnit() })"></canvas>
        </div>
      </div>
      <div>
        <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.mileageCostChart.costKm', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) }) }}</h4>
        <div class="h-40 sm:h-52">
          <canvas ref="costRef" role="img" :aria-label="$t('dashboard.mileageCostChart.costKm', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) })"></canvas>
        </div>
        <p class="mt-1 text-xs text-slate-400">{{ $t('dashboard.mileageCostChart.minDistanceNote', { distance: formatDistanceValue(MIN_PLOTTED_DISTANCE_KM), unit: distanceUnit() }) }}</p>
      </div>
    </div>
    <ChartTable
      :caption="$t('dashboard.mileageCostChart.monthlyMileageAndCostPer', { unit: distanceUnit() })"
      :headers="[$t('dashboard.monthlyCostChart.month'), $t('dashboard.mileageCostChart.distanceKm', { unit: distanceUnit() }), $t('dashboard.mileageCostChart.costKm', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) })]"
      :rows="tableRows"
    />
  </div>
</template>
