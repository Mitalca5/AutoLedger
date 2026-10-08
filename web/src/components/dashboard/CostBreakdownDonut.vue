<script setup lang="ts">
import { t } from '@/i18n'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'

import { DONUT_KEYS, donutAmounts, type DonutMode } from '@/utils/donutBreakdown'

Chart.register(...registerables)

// Share of each cost item: in the full cost of ownership, or in what was actually paid
const props = defineProps<{ tco: any | null }>()

const donutChartRef = ref<HTMLCanvasElement | null>(null)
let donutChartInstance: Chart | null = null
const mode = ref<DonutMode>('full')
const modes: DonutMode[] = ['full', 'cash']
const keys = computed(() => DONUT_KEYS.filter((k) => mode.value === 'full' || k !== 'depreciation'))

function renderChart() {
  if (!donutChartRef.value || !props.tco) return
  if (donutChartInstance) donutChartInstance.destroy()
  const tco = props.tco

  donutChartInstance = new Chart(donutChartRef.value, {
    type: 'doughnut',
    data: {
      labels: keys.value.map((k) => t(`dashboard.donut.${k === 'tires' && mode.value === 'cash' ? 'tiresCash' : k}`)),
      datasets: [
        {
          data: donutAmounts(tco, mode.value).filter((_, i) => mode.value === 'full' || DONUT_KEYS[i] !== 'depreciation'),
          backgroundColor: ['#38bdf8', '#f59e0b', '#10b981', '#ec4899', '#a855f7', '#f97316', '#e11d48', '#64748b'].filter((_, i) => mode.value === 'full' || DONUT_KEYS[i] !== 'depreciation'),
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
watch([() => props.tco, mode], renderChart, { flush: 'post' })
onUnmounted(() => {
  if (donutChartInstance) donutChartInstance.destroy()
})
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
    <p class="text-xs text-slate-500 mb-4">{{ $t(mode === 'full' ? 'dashboard.donut.hintFull' : 'dashboard.donut.hintCash') }}</p>
    <div class="h-48 sm:h-64">
      <canvas ref="donutChartRef" :aria-label="$t('dashboard.costBreakdownDonut.fullCostBreakdownByCategory')"></canvas>
    </div>
  </div>
</template>
