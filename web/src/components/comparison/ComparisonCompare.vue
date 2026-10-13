<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { distanceUnit, formatDistanceValue } from '@/units'
import { ref, computed, watch, onMounted, onBeforeUnmount, onScopeDispose, nextTick } from 'vue'
import { api } from '@/services/api'
import DistanceInput from '@/components/DistanceInput.vue'
import NumberInput from '@/components/NumberInput.vue'
import { breakEvenDelta } from '@/utils/comparisonBreakEven'
import {
  ENERGY_PRICE_PCT_MAX,
  ENERGY_PRICE_PCT_MIN,
  OWN_BASIS,
  basisPayload,
  cumulativeAdvantage,
  electricityPriceApplies,
  gapAgainst,
  isBasisValid,
  isOwnBasis,
  type CompareBasis,
} from '@/utils/comparisonCompareBasis'
import { Chart, registerables } from 'chart.js'
import { downloadCsv } from '@/utils/csv'
import { comparisonCsvRows } from '@/utils/comparisonCsv'
import { useVehicleStore } from '@/stores/vehicle'
import { scenarioSide, sideKey } from '@/utils/comparisonSide'
import { comparisonCurrency, canCompareCurrencies } from '@/utils/comparisonCurrency'

Chart.register(...registerables)

const props = defineProps<{
  items: { scenario: any; result: any }[]
}>()

const vehicleStore = useVehicleStore()

// --- Common basis: the same distance, period and energy price change for every scenario, never saved ---

const DEBOUNCE_MS = 250
const commonBasis = ref(false)
const basis = ref<CompareBasis>({ ...OWN_BASIS })
const referenceId = ref('')
const horizon = ref(0)
const previews = ref<Record<string, any> | null>(null)
const loadingBasis = ref(false)
const basisFailed = ref(false)
let basisTimer: ReturnType<typeof setTimeout> | undefined
let basisSeq = 0

// Limits travel with the results; results from an older server have none and keep the saved basis only.
const limits = computed(() => {
  const l = props.items[0]?.result?.input_limits
  return l && l.years_max !== undefined ? { annual_km_max: Number(l.annual_km_max), years_max: Number(l.years_max) } : null
})
const basisValid = computed(() => !!limits.value && isBasisValid(basis.value, limits.value))
const basisActive = computed(() => commonBasis.value && !isOwnBasis(basis.value))

function startCommonBasis() {
  const first = props.items.find((i) => i.scenario.id === referenceId.value) ?? props.items[0]
  basis.value = { annual_km: Number(first?.result?.annual_km) || null, years: Number(first?.result?.years_count) || null, energy_price_pct: 0 }
}

watch(commonBasis, (on) => {
  if (on) startCommonBasis()
  else basis.value = { ...OWN_BASIS }
})

async function refreshPreviews() {
  const seq = ++basisSeq
  if (!basisActive.value || !basisValid.value) {
    previews.value = null
    loadingBasis.value = false
    basisFailed.value = false
    return
  }
  loadingBasis.value = true
  basisFailed.value = false
  try {
    const results = await Promise.all(props.items.map(({ scenario, result }) => api.previewComparison(basisPayload(scenario, result, basis.value))))
    if (seq !== basisSeq) return
    previews.value = Object.fromEntries(props.items.map((i, k) => [i.scenario.id, results[k]]))
  } catch {
    if (seq !== basisSeq) return
    previews.value = null
    basisFailed.value = true
  } finally {
    if (seq === basisSeq) loadingBasis.value = false
  }
}

watch(
  [() => props.items, basis, commonBasis],
  () => {
    clearTimeout(basisTimer)
    basisTimer = setTimeout(refreshPreviews, DEBOUNCE_MS)
  },
  { deep: true }
)
onScopeDispose(() => {
  clearTimeout(basisTimer)
  basisSeq++
})

// What is shown: the results on the common basis once computed, the saved ones otherwise.
const shown = computed(() => props.items.map((i) => ({ scenario: i.scenario, result: (basisActive.value && previews.value?.[i.scenario.id]) || i.result })))
const onBasis = computed(() => basisActive.value && previews.value !== null)
// Scenarios whose electricity could not follow the energy price change
const electricityKept = computed(() =>
  Number(basis.value.energy_price_pct) === 0 ? [] : props.items.filter((i) => !electricityPriceApplies(i.scenario, i.result)).map((i) => i.scenario.name)
)

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
  shown.value.map(({ scenario, result }) => ({
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
    result,
  }))
)

// Gaps against the reference scenario, when one is chosen
const reference = computed(() => rows.value.find((r) => r.id === referenceId.value) ?? null)
function gapOf(row: { id: string; result: any }) {
  if (!reference.value || row.id === reference.value.id) return null
  return gapAgainst(reference.value.result, row.result)
}
function fmtSigned(v: number): string {
  return `${v > 0 ? '+' : v < 0 ? '−' : ''}${fmtMoney(Math.abs(v), commonCurrency.value || 'EUR')}`
}
function fmtPct(v: number | null): string {
  return v === null ? '' : ` (${v > 0 ? '+' : v < 0 ? '−' : ''}${Math.abs(v).toLocaleString(intlLocale(), { maximumFractionDigits: 0 })} %)`
}
function fmtBreakEvenGap(g: NonNullable<ReturnType<typeof gapOf>>): string {
  const d = g.breakEven
  if (d.kind === 'reached') return t('comparison.whatIf.breakEvenReached')
  if (d.kind === 'lost') return t('comparison.whatIf.breakEvenLost')
  if (d.kind === 'none') return ''
  const sign = d.years > 0 ? '+' : d.years < 0 ? '−' : ''
  return `${sign}${t('comparison.compare.years', { years: Math.abs(d.years).toLocaleString(intlLocale(), { maximumFractionDigits: 1 }) })}`
}

// The visible horizon of the cumulative chart, up to the longest period compared
const maxYears = computed(() => Math.max(0, ...rows.value.map((r) => Number(r.years) || 0)))
const visibleYears = computed(() => (horizon.value >= 1 && horizon.value <= maxYears.value ? horizon.value : maxYears.value))
const horizonOptions = computed(() => Array.from({ length: maxYears.value }, (_, i) => i + 1))

const canvas = ref<HTMLCanvasElement | null>(null)
const curveCanvas = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null
let curveChart: Chart | null = null

const CURVE_COLORS = ['#38bdf8', '#f59e0b', '#a78bfa']

function renderCurves() {
  curveChart?.destroy()
  const currency = commonCurrency.value
  if (!curveCanvas.value || !currency) return
  curveChart = new Chart(curveCanvas.value, {
    type: 'line',
    data: {
      datasets: rows.value.map((r, i) => ({
        label: r.name,
        data: cumulativeAdvantage(r.result).filter((p) => p.year <= visibleYears.value).map((p) => ({ x: p.year, y: p.advantage })),
        borderColor: CURVE_COLORS[i % CURVE_COLORS.length],
        backgroundColor: CURVE_COLORS[i % CURVE_COLORS.length],
        pointRadius: 2,
        tension: 0,
      })),
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { labels: { color: '#94a3b8', boxWidth: 12 } },
        tooltip: { callbacks: { label: (ctx) => ` ${ctx.dataset.label} : ${fmtMoney(Number(ctx.parsed.y), currency)}` } },
      },
      scales: {
        x: { type: 'linear', min: 0, max: visibleYears.value, ticks: { color: '#94a3b8', precision: 0 }, grid: { color: '#1e293b' }, title: { display: true, text: t('comparison.comparisonCompare.yearsAxis'), color: '#94a3b8' } },
        y: { ticks: { color: '#94a3b8', callback: (v) => fmtMoney(Number(v), currency) }, grid: { color: '#1e293b' } },
      },
    },
  })
}

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
    comparisonCsvRows(rows.value)
  )
}

async function renderAll() {
  await nextTick()
  render()
  renderCurves()
}

onMounted(renderAll)
watch([rows, visibleYears], renderAll)
onBeforeUnmount(() => {
  chart?.destroy()
  curveChart?.destroy()
})
</script>

<template>
  <p v-if="!commonCurrency" class="text-warning-300">{{ $t('comparison.currency.same') }}</p>
  <div v-else class="space-y-5 print-area">
    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4 no-print" data-testid="compare-basis">
      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label for="cmp-reference" class="text-xs font-semibold text-slate-300">{{ $t('comparison.comparisonCompare.reference') }}</label>
          <select id="cmp-reference" v-model="referenceId" class="field mt-1">
            <option value="">{{ $t('comparison.comparisonCompare.referenceNone') }}</option>
            <option v-for="i in items" :key="i.scenario.id" :value="i.scenario.id">{{ i.scenario.name }}</option>
          </select>
        </div>
        <div v-if="limits">
          <label class="flex items-center gap-2 text-xs font-semibold text-slate-300 mt-5">
            <input v-model="commonBasis" type="checkbox" data-testid="compare-common-basis" />
            {{ $t('comparison.comparisonCompare.commonBasis') }}
          </label>
        </div>
      </div>
      <div v-if="commonBasis && limits" class="grid gap-3 sm:grid-cols-3 mt-3">
        <div>
          <label for="cmp-km" class="text-xs font-semibold text-slate-300">{{ $t('comparison.whatIf.annualKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput id="cmp-km" v-model="basis.annual_km" :digits="0" :min="1" :max="limits.annual_km_max" class="field" />
        </div>
        <div>
          <label for="cmp-years" class="text-xs font-semibold text-slate-300">{{ $t('comparison.whatIf.years_field') }}</label>
          <NumberInput id="cmp-years" v-model="basis.years" :min="1" :max="limits.years_max" :step="1" class="field" />
        </div>
        <div>
          <label for="cmp-energy" class="text-xs font-semibold text-slate-300">{{ $t('comparison.comparisonCompare.energyPrices') }}</label>
          <NumberInput id="cmp-energy" v-model="basis.energy_price_pct" :min="ENERGY_PRICE_PCT_MIN" :max="ENERGY_PRICE_PCT_MAX" :step="5" class="field" />
        </div>
      </div>
      <p v-if="commonBasis" class="text-xs text-slate-400 mt-2" aria-live="polite">
        <template v-if="!basisValid">{{ $t('comparison.comparisonCompare.basisInvalid') }}</template>
        <template v-else-if="basisFailed">{{ $t('comparison.comparisonCompare.basisFailed') }}</template>
        <template v-else-if="loadingBasis">{{ $t('comparison.comparisonCompare.basisLoading') }}</template>
        <template v-else-if="onBasis">{{ $t('comparison.comparisonCompare.basisApplied') }}</template>
        <template v-else>{{ $t('comparison.comparisonCompare.basisHint') }}</template>
      </p>
      <p v-if="electricityKept.length" class="text-xs text-warning-300 mt-1">{{ $t('comparison.comparisonCompare.electricityKept', { names: electricityKept.join(', ') }) }}</p>
    </div>

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
            <th v-if="reference" scope="col" class="font-semibold pb-2">{{ $t('comparison.comparisonCompare.gapVsReference') }}</th>
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
            <td v-if="reference" data-testid="compare-gap">
              <template v-if="gapOf(r)">
                {{ fmtSigned(gapOf(r)!.savings) }}{{ fmtPct(gapOf(r)!.savingsPct) }}
                <div class="text-xs text-slate-400">{{ gapOf(r)!.savings >= 0 ? $t('comparison.comparisonCompare.savesMore') : $t('comparison.comparisonCompare.savesLess') }}</div>
                <div v-if="fmtBreakEvenGap(gapOf(r)!)" class="text-xs text-slate-400">{{ $t('comparison.comparisonCompare.breakEven') }} {{ fmtBreakEvenGap(gapOf(r)!) }}</div>
              </template>
              <span v-else class="text-xs text-slate-400">{{ $t('comparison.comparisonCompare.referenceBadge') }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
      <div class="flex items-center justify-between gap-3 mb-1">
        <h2 class="text-sm font-semibold text-white">{{ $t('comparison.comparisonCompare.advantageTitle') }}</h2>
        <div class="flex items-center gap-2 no-print">
          <label for="cmp-horizon" class="text-xs text-slate-400">{{ $t('comparison.comparisonCompare.horizon') }}</label>
          <select id="cmp-horizon" v-model.number="horizon" class="field w-auto">
            <option :value="0">{{ $t('comparison.comparisonCompare.horizonAll') }}</option>
            <option v-for="y in horizonOptions" :key="y" :value="y">{{ $t('comparison.compare.years', { years: y }) }}</option>
          </select>
        </div>
      </div>
      <p class="text-xs text-slate-400 mb-3">{{ $t('comparison.comparisonCompare.advantageHint') }}</p>
      <div class="h-72"><canvas ref="curveCanvas" :aria-label="$t('comparison.comparisonCompare.advantageTitle')"></canvas></div>
    </div>

    <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
      <h2 class="text-sm font-semibold text-white mb-3">{{ $t('comparison.comparisonCompare.totalCostPerScenario') }}</h2>
      <div class="h-72"><canvas ref="canvas" :aria-label="$t(sk('comparison.comparisonCompare.totalElectricAndCombustionCost'))"></canvas></div>
    </div>
  </div>
</template>
