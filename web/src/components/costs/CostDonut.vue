<script setup lang="ts">
import { formatNumber, formatPercent } from '@/utils/numbers'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { formatAmount } from '@/currency'

Chart.register(...registerables)

// Doughnut of cost items, shared by the monthly detail and the drive detail. Items without an amount are left out;
// with none left, a neutral ring is drawn with emptyLabel. Money items pass the vehicle's currency; a non-money
// chart (the AC/DC energy split, in kWh) passes unit instead.
const props = withDefaults(
  defineProps<{
    items: { label: string; color: string; amount: number }[]
    chartLabel: string
    emptyLabel?: string
    showLegend?: boolean
    cutout?: string
    unit?: string
    currency?: string
  }>(),
  { emptyLabel: '', showLegend: true, cutout: '68%', unit: '', currency: 'EUR' },
)

const canvasRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null
const active = computed(() => props.items.filter((it) => it.amount > 0.005))

function render() {
  chart?.destroy()
  chart = null
  if (!canvasRef.value) return
  const items = active.value
  const total = items.reduce((sum, it) => sum + it.amount, 0)
  chart = new Chart(canvasRef.value, {
    type: 'doughnut',
    data: {
      labels: items.length ? items.map((it) => it.label) : [props.emptyLabel],
      datasets: [{ data: items.length ? items.map((it) => it.amount) : [1], backgroundColor: items.length ? items.map((it) => it.color) : ['#334155'], borderWidth: 0 }],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: props.showLegend, position: 'bottom', labels: { color: '#94a3b8', font: { size: 11 }, boxWidth: 10, padding: 10 } },
        tooltip: {
          callbacks: {
            label: (ctx) => {
              if (!items.length) return ' ' + props.emptyLabel
              const value = Number(ctx.raw || 0)
              const pct = formatPercent(total > 0 ? (value / total) * 100 : 0)
              const formatted = props.unit ? `${formatNumber(value, 2)} ${props.unit}` : formatAmount(value, props.currency)
              return ` ${ctx.label} : ${formatted} (${pct})`
            },
          },
        },
      },
      cutout: props.cutout,
    },
  })
}

onMounted(render)
watch(() => [props.items, props.showLegend], render, { flush: 'post', deep: true })
onUnmounted(() => chart?.destroy())
</script>

<template>
  <canvas ref="canvasRef" :aria-label="chartLabel"></canvas>
</template>
