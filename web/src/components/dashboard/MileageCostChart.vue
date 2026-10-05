<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { formatMonthLabel } from '@/utils/dates'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Activity, PieChart } from 'lucide-vue-next'
import { filterMonthsByRange } from '@/utils/dashboard'
import { usePreferencesStore } from '@/stores/preferences'
import { currencySymbol, formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { distanceUnit, formatDistanceValue, kmToDisplayDistance, perDistance } from '@/units'

Chart.register(...registerables)

// Monthly distance and average cost per km on two axes; a click on a month opens its detail
const props = defineProps<{ monthlyCosts: any[] | null }>()
const emit = defineEmits<{ 'open-month': [month: any] }>()
const vehicleStore = useVehicleStore()

const mileageChartRef = ref<HTMLCanvasElement | null>(null)
const prefs = usePreferencesStore()
const mileageChartRange = computed(() => prefs.dashboardRange)
const filteredMileageCosts = computed(() => filterMonthsByRange(props.monthlyCosts || [], mileageChartRange.value))
let mileageChartInstance: Chart | null = null

function renderChart() {
  if (!mileageChartRef.value || !props.monthlyCosts) return
  if (mileageChartInstance) mileageChartInstance.destroy()

  const filteredMileageList = filteredMileageCosts.value
  const mileageLabels = filteredMileageList.map((m: any) => formatMonthLabel(m.month))
  const distanceData = filteredMileageList.map((m: any) => Math.round(kmToDisplayDistance(m.distance_km || 0)))
  const costPerKmData = filteredMileageList.map((m: any) => perDistance(m.cost_per_km || 0))

  mileageChartInstance = new Chart(mileageChartRef.value, {
    type: 'bar',
    data: {
      labels: mileageLabels,
      datasets: [
        {
          type: 'bar',
          label: t('dashboard.mileageCostChart.kmDriven', { unit: distanceUnit() }),
          data: distanceData,
          backgroundColor: 'rgba(99, 102, 241, 0.65)',
          hoverBackgroundColor: 'rgba(99, 102, 241, 0.9)',
          borderRadius: 6,
          yAxisID: 'yDistance',
        },
        {
          type: 'line',
          label: t('dashboard.mileageCostChart.averageCostPerKm', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) }),
          data: costPerKmData,
          borderColor: '#10b981',
          backgroundColor: '#10b981',
          borderWidth: 2.5,
          pointBackgroundColor: '#10b981',
          pointRadius: 4,
          pointHoverRadius: 6,
          tension: 0.3,
          yAxisID: 'yCost',
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: {
        mode: 'index',
        intersect: false,
      },
      onClick: (event: any) => {
        if (!mileageChartInstance) return
        const points = mileageChartInstance.getElementsAtEventForMode(
          event.native || event,
          'index',
          { intersect: false },
          true
        )
        if (points && points.length > 0) {
          const dataIndex = points[0].index
          const monthItem = filteredMileageList[dataIndex]
          if (monthItem) {
            emit('open-month', monthItem)
          }
        }
      },
      onHover: (event: any, elements: any[]) => {
        const canvas = mileageChartRef.value
        if (canvas) {
          canvas.style.cursor = elements && elements.length > 0 ? 'pointer' : 'default'
        }
      },
      plugins: {
        legend: { position: 'top', labels: { color: '#94a3b8', font: { size: 11 } } },
        tooltip: {
          callbacks: {
            label: (context) => {
              if (context.dataset.yAxisID === 'yDistance') {
                const dist = Number(context.raw).toLocaleString(intlLocale())
                return t('dashboard.mileageCostChart.tooltipDistance', { unit: distanceUnit(), distance: dist })
              }
              return t('dashboard.mileageCostChart.tooltipCostPerKm', { unit: distanceUnit(), cost: formatAmount(Number(context.raw), vehicleStore.currency, 3) })
            },
          },
        },
      },
      scales: {
        x: { grid: { color: '#1e293b' }, ticks: { color: '#64748b' } },
        yDistance: {
          type: 'linear',
          position: 'left',
          grid: { color: '#1e293b' },
          ticks: {
            color: '#818cf8',
            callback: (v) => `${Number(v).toLocaleString(intlLocale())} ${distanceUnit()}`,
          },
          title: { display: true, text: t('dashboard.mileageCostChart.axisDistance', { unit: distanceUnit() }), color: '#818cf8', font: { size: 11 } },
        },
        yCost: {
          type: 'linear',
          position: 'right',
          grid: { display: false },
          ticks: {
            color: '#10b981',
            callback: (v) => formatAmount(Number(v), vehicleStore.currency, 3),
          },
          title: { display: true, text: t('dashboard.mileageCostChart.axisCostPerKm', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) }), color: '#10b981', font: { size: 11 } },
        },
      },
    },
  })
}

onMounted(renderChart)
watch([() => props.monthlyCosts, mileageChartRange], renderChart, { flush: 'post' })
onUnmounted(() => {
  if (mileageChartInstance) mileageChartInstance.destroy()
})
</script>

<template>
  <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm space-y-3">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
      <div>
        <h3 class="text-sm font-bold text-white flex items-center gap-2">
          <Activity class="w-4 h-4 text-indigo-400" />
          <span>{{ $t('dashboard.mileageCostChart.monthlyMileageAndCostPer2', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) }) }}</span>
        </h3>
        <p class="text-xs text-slate-400 mt-0.5">{{ $t('dashboard.mileageCostChart.clickABarOfThe') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2.5 self-start sm:self-auto">
        <button
          v-if="filteredMileageCosts.length"
          type="button"
          @click="emit('open-month', filteredMileageCosts[filteredMileageCosts.length - 1])"
          class="px-2.5 py-1 bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-colors"
        >
          <PieChart class="w-3.5 h-3.5" />
          <span>{{ $t('dashboard.mileageCostChart.lastMonthSDetail') }}</span>
        </button>
        <div class="flex items-center gap-3 text-xs">
          <span class="flex items-center gap-1.5 text-indigo-300">
            <span class="w-3 h-3 rounded bg-indigo-500/80 inline-block"></span>
            {{ $t('dashboard.mileageCostChart.distanceKm', { unit: distanceUnit() }) }}
          </span>
          <span class="flex items-center gap-1.5 text-success-400">
            <span class="w-3 h-1 rounded bg-success-400 inline-block"></span>
            {{ $t('dashboard.mileageCostChart.costKm', { unit: distanceUnit(), cur: currencySymbol(vehicleStore.currency) }) }}
          </span>
        </div>
      </div>
    </div>

    <div class="h-52 sm:h-72">
      <canvas ref="mileageChartRef" role="img" :aria-label="$t('dashboard.mileageCostChart.monthlyMileageAndCostPer', { unit: distanceUnit() })"></canvas>
    </div>
  </div>
</template>
