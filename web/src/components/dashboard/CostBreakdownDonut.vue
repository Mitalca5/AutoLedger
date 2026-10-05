<script setup lang="ts">
import { t } from '@/i18n'
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'

Chart.register(...registerables)

// Share of each cost item in the full cost of ownership
const props = defineProps<{ tco: any | null }>()

const donutChartRef = ref<HTMLCanvasElement | null>(null)
let donutChartInstance: Chart | null = null

function renderChart() {
  if (!donutChartRef.value || !props.tco) return
  if (donutChartInstance) donutChartInstance.destroy()
  const tco = props.tco

  donutChartInstance = new Chart(donutChartRef.value, {
    type: 'doughnut',
    data: {
      labels: ['energy', 'tolls', 'tires', 'maintenance', 'insurance', 'financing', 'depreciation', 'other'].map((k) => t(`dashboard.donut.${k}`)),
      datasets: [
        {
          data: [
            tco.energy_cost || 0,
            tco.tolls_cost || 0,
            tco.tires_amortized_cost || 0,
            (tco.maintenance_cost || 0) + (tco.repair_cost || 0),
            tco.insurance_cost || 0,
            Math.max(0, tco.financing_full_cost || 0),
            tco.depreciation_cost || 0,
            (tco.subscription_cost || 0) + (tco.tax_cost || 0) + (tco.other_cost || 0),
          ],
          backgroundColor: ['#38bdf8', '#f59e0b', '#10b981', '#ec4899', '#a855f7', '#f97316', '#e11d48', '#64748b'],
          borderWidth: 0,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { position: 'bottom', labels: { color: '#94a3b8', font: { size: 11 } } },
      },
      cutout: '72%',
    },
  })
}

onMounted(renderChart)
watch(() => props.tco, renderChart, { flush: 'post' })
onUnmounted(() => {
  if (donutChartInstance) donutChartInstance.destroy()
})
</script>

<template>
  <div class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
    <h3 class="text-sm font-bold text-white mb-4">{{ $t('dashboard.costBreakdownDonut.fullCostBreakdown') }}</h3>
    <div class="h-48 sm:h-64">
      <canvas ref="donutChartRef" role="img" :aria-label="$t('dashboard.costBreakdownDonut.fullCostBreakdownByCategory')"></canvas>
    </div>
  </div>
</template>
