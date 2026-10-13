<script setup lang="ts">
import { computed } from 'vue'
import { intlLocale, t } from '@/i18n'
import { distanceUnit, formatDistanceValue, perDistance } from '@/units'
import { apiMessageText } from '@/services/apiError'
import { downloadCsv } from '@/utils/csv'
import { comparisonSavings } from '@/utils/comparisonSavings'
import { sideKey, type TrackedSide } from '@/utils/comparisonSide'
import { costRowValues } from '@/utils/comparisonForm'
import ComparisonCharts from '@/components/comparison/ComparisonCharts.vue'
import ComparisonWhatIf from '@/components/comparison/ComparisonWhatIf.vue'
import type { ComparisonForm } from '@/utils/comparisonForm'
import { ArrowLeft, Download, Info, Pencil, Printer, TrendingDown, TrendingUp } from 'lucide-vue-next'

// The outcome of one scenario: verdict, savings, cost breakdown, charts and the CSV / print exports.
const props = defineProps<{ result: any | null; scenario: any | null; side: TrackedSide; currency: string; referenceVehicleId?: string }>()
const emit = defineEmits<{ back: []; edit: []; 'save-as-new': [form: ComparisonForm] }>()

const sk = (key: string) => sideKey(key, props.side)

function fmtMoney(v: number | null | undefined, digits = 0): string {
  return Number(v || 0).toLocaleString(intlLocale(), {
    style: 'currency',
    currency: props.currency,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

function fmtKm(v: number): string {
  return formatDistanceValue(Number(v || 0))
}

const savings = computed<number>(() => Number(props.result?.tracked_savings || 0))
// A tracked vehicle bought without a depreciation would have it at zero while the other side has one: the total saving would
// count depreciation on one side only
const depreciationMissing = computed(() => props.result?.tracked_depreciation_known === false)

const verdict = computed(() => {
  if (!props.result) return ''
  if (depreciationMissing.value) return t('comparison.depreciationMissing.verdict')
  const n = props.result.years_count
  const abs = fmtMoney(Math.abs(savings.value))
  if (Math.abs(savings.value) < 1) return t('comparison.verdict.same', n)
  return savings.value > 0
    ? t(sk('comparison.verdict.less'), { count: n, amount: abs })
    : t(sk('comparison.verdict.more'), { count: n, amount: abs })
})

// What the verdict does not say: the saving on energy alone and on the whole cost, each as an amount, a share of the
// combustion cost and per month.
interface SavingsCard { key: 'energy' | 'total'; amount: number; positive: boolean; percent: string | null; perMonth: string | null }

const savingsCards = computed<SavingsCard[]>(() => {
  const r = props.result
  if (!r) return []
  return (depreciationMissing.value ? (['energy'] as const) : (['energy', 'total'] as const)).map(key => {
    const s = comparisonSavings(r.tracked[key], r.ice[key], r.years_count, r.annual_km)
    return {
      key,
      amount: Math.abs(s.amount),
      positive: s.amount >= 0,
      percent: s.percent !== null ? Math.abs(s.percent).toLocaleString(intlLocale(), { maximumFractionDigits: 1 }) : null,
      perMonth: s.perMonth !== null ? fmtMoney(Math.abs(s.perMonth)) : null,
    }
  })
})

function breakEvenSentence(be: number | null | undefined, keys: { none: string; now: string; after: string }): string {
  if (be === undefined || be === null) return t(keys.none)
  if (be === 0) return t(sk(keys.now))
  return t(keys.after, { years: Number(be).toLocaleString(intlLocale()) })
}

const breakEvenText = computed(() => {
  const r = props.result
  if (!r) return ''
  const outlay = breakEvenSentence(r.break_even_year, { none: 'comparison.breakEven.notReached', now: 'comparison.breakEven.immediate', after: 'comparison.breakEven.after' })
  const hasResale = Number(r.tracked?.depreciation) > 0 || Number(r.ice?.depreciation) > 0
  if (!hasResale) return outlay
  const net = breakEvenSentence(r.break_even_net_year, { none: 'comparison.breakEven.netNotReached', now: 'comparison.breakEven.netImmediate', after: 'comparison.breakEven.netAfter' })
  return `${outlay} ${net}`
})

// What the comparison leaves out on both sides, stated under the verdict, in print and in the CSV
const exclusionsNote = computed(() => t('comparison.exclusions.note'))

const costRows = computed(() => costRowValues(props.result).map((r) => ({ label: t(`comparison.rows.${r.key}`), tracked: r.tracked, ice: r.ice })))

function exportResultCsv() {
  const r = props.result
  if (!r) return
  const name = (props.scenario?.name || t('comparison.csv.defaultName')).replace(/[^\w-]+/g, '-')
  const rows: (string | number)[][] = costRows.value.map((row) => [row.label, Number(row.tracked).toFixed(2), Number(row.ice).toFixed(2)])
  rows.push([t('comparison.csv.total'), Number(r.tracked.total).toFixed(2), Number(r.ice.total).toFixed(2)])
  rows.push([t('comparison.csv.perMonth'), Number(r.tracked.per_month).toFixed(2), Number(r.ice.per_month).toFixed(2)])
  rows.push([t('comparison.csv.costPerKm', { unit: distanceUnit() }), perDistance(r.tracked.cost_per_km).toFixed(3), perDistance(r.ice.cost_per_km).toFixed(3)])
  if (!depreciationMissing.value) rows.push([t(sk('comparison.csv.gap')), Number(r.tracked_savings).toFixed(2), ''])
  rows.push([exclusionsNote.value, '', ''])
  rows.push(['', '', ''])
  rows.push(t(sk('comparison.csv.cumulativeHeader')).split(','))
  for (const p of r.cumulative) rows.push([p.year, Number(p.tracked).toFixed(2), Number(p.ice).toFixed(2)])
  downloadCsv(`${t('comparison.csv.filePrefix')}-${name}`, t(sk('comparison.csv.header'), { cur: props.currency }).split(','), rows)
}

function printResult() {
  window.print()
}

</script>

<template>
  <div class="space-y-5 print-area">
    <div class="flex items-center justify-between gap-3 no-print">
      <button class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5" @click="emit('back')">
        <ArrowLeft class="w-4 h-4" /> {{ $t('comparison.comparisonView.myComparisons') }}
      </button>
      <div v-if="result" class="flex items-center gap-2">
        <button type="button" class="btn btn-secondary" @click="exportResultCsv">
          <Download class="w-4 h-4" /> {{ $t('comparison.comparisonView.csv') }}
        </button>
        <button type="button" class="btn btn-secondary" @click="printResult">
          <Printer class="w-4 h-4" /> {{ $t('comparison.comparisonView.printPdf') }}
        </button>
      </div>
    </div>

    <div v-if="!result" class="text-sm text-slate-400">{{ $t('comparison.comparisonView.calculating') }}</div>
    <template v-else>
      <div
        class="rounded-2xl border p-5 flex items-center gap-4"
        :class="depreciationMissing ? 'bg-slate-900 border-slate-700' : savings >= 0 ? 'bg-success-500/10 border-success-500/30' : 'bg-warning-500/10 border-warning-500/30'"
      >
        <component :is="savings >= 0 ? TrendingDown : TrendingUp" class="w-8 h-8 shrink-0" :class="depreciationMissing ? 'text-slate-400' : savings >= 0 ? 'text-success-400' : 'text-warning-400'" />
        <div>
          <div class="text-lg font-bold text-white">{{ verdict }}</div>
          <div class="text-xs text-slate-400 mt-0.5">{{ scenario?.name }} · {{ $t('comparison.comparisonView.kmPerYear', { unit: distanceUnit(), km: fmtKm(result.annual_km) }) }}</div>
          <div class="text-xs text-slate-400 mt-1" data-testid="comparison-exclusions">{{ exclusionsNote }}</div>
        </div>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div v-if="depreciationMissing" class="order-last rounded-2xl border border-slate-700 bg-slate-900 p-4 text-sm text-slate-300" data-testid="savings-total-missing">
          {{ $t('comparison.depreciationMissing.card') }}
          <router-link to="/vehicles" class="font-semibold text-sky-400 underline">{{ $t('dashboard.tcoMetricsGrid.depreciationMissingAction') }}</router-link>
        </div>
        <div v-for="card in savingsCards" :key="card.key" class="bg-slate-900 border border-slate-800 rounded-2xl p-4" :data-testid="`savings-${card.key}`">
          <h2 class="text-sm font-semibold text-slate-300">{{ $t(`comparison.savings.${card.key}Title`) }}</h2>
          <div class="text-2xl font-bold mt-1" :class="card.positive ? 'text-success-400' : 'text-warning-400'">
            {{ fmtMoney(card.amount) }}
            <span class="text-sm font-medium">{{ $t(card.positive ? 'comparison.savings.saved' : 'comparison.savings.extra') }}</span>
          </div>
          <div class="text-xs text-slate-400 mt-1">
            <template v-if="card.percent !== null">{{ $t(card.positive ? 'comparison.savings.less' : 'comparison.savings.more', { percent: card.percent }) }}</template>
            <template v-if="card.percent !== null && card.perMonth !== null"> · </template>
            <template v-if="card.perMonth !== null">{{ $t('comparison.savings.month', { amount: card.perMonth }) }}</template>
          </div>
          <div class="text-xs text-slate-500 mt-1">{{ $t(`comparison.savings.${card.key}Hint`) }}</div>
        </div>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
          <div class="flex items-center justify-between mb-2">
            <h2 class="text-sm font-semibold text-info-300">{{ $t(sk('comparison.comparisonView.electric')) }}</h2>
            <span class="text-xs px-2 py-0.5 rounded-full border" :class="result.mode === 'RETROSPECTIVE' ? 'border-success-500/40 text-success-300' : 'border-slate-600 text-slate-400'">
              {{ result.mode === 'RETROSPECTIVE' ? $t('comparison.comparisonView.projected') : $t('comparison.comparisonView.estimated') }}
            </span>
          </div>
          <div class="text-2xl font-bold text-white">{{ fmtMoney(result.tracked.total) }}</div>
          <div class="text-xs text-slate-400 mt-1">{{ $t('comparison.comparisonView.monthKm', { unit: distanceUnit(), per_month: fmtMoney(result.tracked.per_month), cost_per_km: fmtMoney(perDistance(result.tracked.cost_per_km), 3) }) }}</div>
        </div>
        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
          <div class="flex items-center justify-between mb-2">
            <h2 class="text-sm font-semibold text-warning-300">{{ $t('comparison.comparisonView.combustion') }}</h2>
            <span class="text-xs px-2 py-0.5 rounded-full border border-slate-600 text-slate-400">{{ $t('comparison.comparisonView.estimated') }}</span>
          </div>
          <div class="text-2xl font-bold text-white">{{ fmtMoney(result.ice.total) }}</div>
          <div class="text-xs text-slate-400 mt-1">{{ $t('comparison.comparisonView.monthKm', { unit: distanceUnit(), per_month: fmtMoney(result.ice.per_month), cost_per_km: fmtMoney(perDistance(result.ice.cost_per_km), 3) }) }}</div>
        </div>
      </div>

      <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4 overflow-x-auto">
        <table class="w-full text-sm">
          <caption class="sr-only">{{ $t('comparison.comparisonView.costByCategoryOverYear', { years_count: result.years_count }) }}</caption>
          <thead>
            <tr class="text-xs text-slate-400 text-right">
              <th scope="col" class="text-left font-semibold pb-2">{{ $t('comparison.comparisonView.category') }}</th>
              <th scope="col" class="font-semibold pb-2">{{ $t(sk('comparison.comparisonView.electric')) }}</th>
              <th scope="col" class="font-semibold pb-2">{{ $t('comparison.comparisonView.combustion') }}</th>
            </tr>
          </thead>
          <tbody class="text-slate-200">
            <tr v-for="row in costRows" :key="row.label" class="border-t border-slate-800 text-right">
              <th scope="row" class="text-left font-normal py-1.5">{{ row.label }}</th>
              <td>{{ fmtMoney(row.tracked) }}</td>
              <td>{{ fmtMoney(row.ice) }}</td>
            </tr>
            <tr class="border-t border-slate-700 text-right font-semibold text-white">
              <th scope="row" class="text-left py-1.5">{{ $t('comparison.comparisonView.total') }}</th>
              <td>{{ fmtMoney(result.tracked.total) }}</td>
              <td>{{ fmtMoney(result.ice.total) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <ComparisonWhatIf
        v-if="scenario?.mode === 'PROJECTION'"
        :scenario="scenario"
        :result="result"
        :currency="currency"
        :reference-vehicle-id="referenceVehicleId"
        @save-as-new="emit('save-as-new', $event)"
      />

      <ComparisonCharts :result="result" :side="side" :currency="currency" :cost-rows="costRows" :break-even-text="breakEvenText" />

      <div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-4">
        <h2 class="text-xs font-semibold text-slate-300 mb-2 flex items-center gap-1.5"><Info class="w-3.5 h-3.5" /> {{ $t('comparison.comparisonView.assumptions') }}</h2>
        <ul class="list-disc pl-4 text-xs text-slate-400 space-y-1">
          <li v-for="(a, i) in result.assumptions" :key="i">{{ apiMessageText(a) }}</li>
        </ul>
      </div>

      <div class="flex gap-2 no-print">
        <button class="btn btn-lg btn-secondary" @click="emit('edit')">
          <Pencil class="w-4 h-4" /> {{ $t('comparison.comparisonView.editTheAssumptions') }}
        </button>
      </div>
    </template>
  </div>
</template>

<style>
@media print {
  aside,
  header,
  nav,
  .no-print {
    display: none !important;
  }
  .h-screen,
  .overflow-y-auto,
  .overflow-hidden {
    height: auto !important;
    overflow: visible !important;
  }
  .print-area,
  .print-area * {
    color: #111827 !important;
    background: transparent !important;
    border-color: #d1d5db !important;
  }
  body,
  html {
    background: #ffffff !important;
  }
}
</style>
