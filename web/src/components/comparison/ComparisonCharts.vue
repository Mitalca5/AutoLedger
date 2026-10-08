<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { Chart, registerables } from 'chart.js'
import { intlLocale, t } from '@/i18n'
import { apiMessageText } from '@/services/apiError'
import { sideKey, type TrackedSide } from '@/utils/comparisonSide'

Chart.register(...registerables)

const props = defineProps<{
  result: any
  side: TrackedSide
  currency: string
  costRows: { label: string; tracked: number; ice: number }[]
  breakEvenText: string
}>()

const sk = (key: string) => sideKey(key, props.side)

function fmtMoney(v: number | null | undefined): string {
  return Number(v || 0).toLocaleString(intlLocale(), { style: 'currency', currency: props.currency, minimumFractionDigits: 0, maximumFractionDigits: 0 })
}

const chartRef = ref<HTMLCanvasElement | null>(null)
const barRef = ref<HTMLCanvasElement | null>(null)
const tornadoRef = ref<HTMLCanvasElement | null>(null)
let charts: Chart[] = []

function destroyCharts() {
  charts.forEach((c) => c.destroy())
  charts = []
}

const axisStyle = { ticks: { color: '#94a3b8' }, grid: { color: '#1e293b' } }
const moneyAxis = { ticks: { color: '#94a3b8', callback: (v: any) => fmtMoney(Number(v)) }, grid: { color: '#1e293b' } }

function render() {
  destroyCharts()
  const r = props.result
  if (!r) return

  if (chartRef.value) {
    const points = r.cumulative as { year: number; tracked: number; ice: number }[]
    charts.push(new Chart(chartRef.value, {
      type: 'line',
      data: {
        labels: points.map((p) => (p.year === 0 ? t('comparison.chart.purchase') : t('comparison.chart.year', { year: p.year }))),
        datasets: [
          { label: t(sk('comparison.electric')), data: points.map((p) => p.tracked), borderColor: '#38bdf8', backgroundColor: '#38bdf8', tension: 0.15 },
          { label: t('comparison.combustion'), data: points.map((p) => p.ice), borderColor: '#f59e0b', backgroundColor: '#f59e0b', tension: 0.15 },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          legend: { labels: { color: '#94a3b8', boxWidth: 12 } },
          tooltip: { callbacks: { label: (ctx) => ` ${ctx.dataset.label} : ${fmtMoney(Number(ctx.raw))}` } },
        },
        scales: { x: axisStyle, y: moneyAxis },
      },
    }))
  }

  if (barRef.value) {
    const colors = ['#38bdf8', '#a78bfa', '#34d399', '#94a3b8', '#f472b6']
    charts.push(new Chart(barRef.value, {
      type: 'bar',
      data: {
        labels: [t(sk('comparison.electric')), t('comparison.combustion')],
        datasets: props.costRows.map((row, i) => ({
          label: row.label,
          data: [row.tracked, row.ice],
          backgroundColor: colors[i % colors.length],
        })),
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { labels: { color: '#94a3b8', boxWidth: 12 } },
          tooltip: { callbacks: { label: (ctx) => ` ${ctx.dataset.label} : ${fmtMoney(Number(ctx.raw))}` } },
        },
        scales: { x: { ...axisStyle, stacked: true }, y: { ...moneyAxis, stacked: true } },
      },
    }))
  }

  if (tornadoRef.value) {
    const rows = (r.sensitivity as { label: any; delta_shift: number }[]).map((s) => ({ label: apiMessageText(s.label), delta_shift: s.delta_shift }))
    charts.push(new Chart(tornadoRef.value, {
      type: 'bar',
      data: {
        labels: rows.map((s) => s.label),
        datasets: [{
          label: t(sk('comparison.chart.gapLabel')),
          data: rows.map((s) => s.delta_shift),
          backgroundColor: rows.map((s) => (s.delta_shift >= 0 ? '#34d399' : '#f87171')),
          borderRadius: 4,
        }],
      },
      options: {
        indexAxis: 'y',
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: { callbacks: { label: (ctx) => ` ${Number(ctx.raw) >= 0 ? '+' : '−'}${fmtMoney(Math.abs(Number(ctx.raw)))} ${t(sk('comparison.chart.inFavor'))}` } },
        },
        scales: { x: moneyAxis, y: axisStyle },
      },
    }))
  }
}

watch(() => props.result, () => nextTick(render))
onMounted(() => nextTick(render))
onBeforeUnmount(destroyCharts)
</script>

<template>
  <div class="space-y-5">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
      <h2 class="text-sm font-semibold text-white mb-3">{{ $t('comparison.comparisonView.costBreakdownOverThePeriod') }}</h2>
      <div class="h-64"><canvas ref="barRef" :aria-label="$t(sk('comparison.comparisonView.costByCategoryElectricAnd'))"></canvas></div>
    </div>

    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
      <h2 class="text-sm font-semibold text-white mb-1">{{ $t('comparison.comparisonView.cumulativeCost') }}</h2>
      <p class="text-xs text-slate-400 mb-3">{{ breakEvenText }}</p>
      <div class="h-64"><canvas ref="chartRef" :aria-label="$t(sk('comparison.comparisonView.cumulativeCostElectricAndCombustion'))"></canvas></div>
    </div>

    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
      <h2 class="text-sm font-semibold text-white mb-1">{{ $t('comparison.comparisonView.sensitivity') }}</h2>
      <p class="text-xs text-slate-400 mb-3">{{ $t(sk('comparison.comparisonView.effectOnTheElectricSaving')) }}</p>
      <div class="h-48 mb-3"><canvas ref="tornadoRef" :aria-label="$t('comparison.comparisonView.sensitivityOfTheGapTo')"></canvas></div>
      <ul class="text-xs text-slate-300 space-y-1">
        <li v-for="s in result.sensitivity" :key="s.label.code" class="flex justify-between">
          <span>{{ apiMessageText(s.label) }}</span>
          <span>{{ s.tracked_savings >= 0 ? $t(sk('comparison.comparisonView.evLess'), { amount: fmtMoney(s.tracked_savings) }) : $t(sk('comparison.comparisonView.evMore'), { amount: fmtMoney(-s.tracked_savings) }) }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>
