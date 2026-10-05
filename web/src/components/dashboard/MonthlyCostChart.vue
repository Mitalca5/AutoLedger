<script setup lang="ts">
import { t } from '@/i18n'
import { formatMonthLabel } from '@/utils/dates'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { computeMonthFixedVariable, filterMonthsByRange } from '@/utils/dashboard'
import { usePreferencesStore } from '@/stores/preferences'
import { currencySymbol, formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'

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

function renderChart() {
  if (!monthlyChartRef.value || !props.monthlyCosts) return
  if (monthlyChartInstance) monthlyChartInstance.destroy()

  const filteredList = filteredMonthlyCosts.value
  const filteredLabels = filteredList.map((m: any) => formatMonthLabel(m.month))
  const energyData = filteredList.map((m: any) => m.energy)
  const tollsData = filteredList.map((m: any) => m.tolls)
  const tiresData = filteredList.map((m: any) => m.tires || 0)
  const maintData = filteredList.map((m: any) => m.maintenance)
  const insuranceData = filteredList.map((m: any) => m.insurance || 0)
  const otherData = filteredList.map((m: any) => m.other || 0)
  const financingData = filteredList.map((m: any) => m.financing || 0)

  monthlyChartInstance = new Chart(monthlyChartRef.value, {
    type: 'bar',
    data: {
      labels: filteredLabels,
      datasets: [
        { label: `${t('dashboard.donut.energy')} (${currencySymbol(currency.value)})`, data: energyData, backgroundColor: '#38bdf8', borderRadius: 4 },
        { label: `${t('dashboard.donut.tolls')} (${currencySymbol(currency.value)})`, data: tollsData, backgroundColor: '#f59e0b', borderRadius: 4 },
        { label: `${t('dashboard.monthlyCostChart.tires')} (${currencySymbol(currency.value)})`, data: tiresData, backgroundColor: '#10b981', borderRadius: 4 },
        { label: `${t('dashboard.monthlyCostChart.maintenance')} (${currencySymbol(currency.value)})`, data: maintData, backgroundColor: '#ec4899', borderRadius: 4 },
        { label: `${t('dashboard.donut.insurance')} (${currencySymbol(currency.value)})`, data: insuranceData, backgroundColor: '#a855f7', borderRadius: 4 },
        { label: `${t('dashboard.monthlyCostChart.financing')} (${currencySymbol(currency.value)})`, data: financingData, backgroundColor: '#f97316', borderRadius: 4 },
        { label: `${t('dashboard.donut.other')} (${currencySymbol(currency.value)})`, data: otherData, backgroundColor: '#64748b', borderRadius: 4 },
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
              const total = items.reduce((sum, item) => sum + (Number(item.raw) || 0), 0)
              const lines = [t('dashboard.monthlyCostChart.monthTotal', { total: formatAmount(total, currency.value) })]
              const dataIndex = items[0]?.dataIndex
              if (dataIndex !== undefined) {
                const monthItem = filteredList[dataIndex]
                const fv = computeMonthFixedVariable(monthItem, 'economic')
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
          grid: { color: '#1e293b' },
          ticks: { color: '#64748b', callback: (v) => formatAmount(Number(v), currency.value) },
        },
      },
    },
  })
}

onMounted(renderChart)
watch([() => props.monthlyCosts, monthlyChartRange], renderChart, { flush: 'post' })
onUnmounted(() => {
  if (monthlyChartInstance) monthlyChartInstance.destroy()
})
</script>

<template>
  <div class="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-4">
      <h3 class="text-sm font-bold text-white flex items-center gap-2">
        <span>{{ $t('dashboard.monthlyCostChart.monthlyExpenseTrend', { cur: currencySymbol(currency) }) }}</span>
      </h3>
    </div>
    <div class="h-64 sm:h-72">
      <canvas ref="monthlyChartRef" role="img" :aria-label="$t('dashboard.monthlyCostChart.monthlyCostTrendByCategory')"></canvas>
    </div>
  </div>
</template>
