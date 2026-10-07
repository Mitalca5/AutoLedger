<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import LoadError from '@/components/LoadError.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import { Scale as PageIcon } from 'lucide-vue-next'
import { intlLocale, t, te } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { distanceUnit, formatDistanceValue, perDistance } from '@/units'
import { apiMessageText } from '@/services/apiError'
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Plus, Trash2, Pencil, ArrowLeft, ArrowRight, Info, TrendingDown, TrendingUp, ChevronDown, Download, Printer, GitCompare } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import { api } from '@/services/api'
import { downloadCsv } from '@/utils/csv'
import { currencySymbol } from '@/currency'
import { scenarioSide, sideKey } from '@/utils/comparisonSide'
import ComparisonCompare from '@/components/comparison/ComparisonCompare.vue'

Chart.register(...registerables)

const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()

type Mode = 'RETROSPECTIVE' | 'PROJECTION'

const scenarios = ref<any[]>([])
const loading = ref(false)
const loadError = ref<string | null>(null)
const saving = ref(false)
const defaults = ref<any | null>(null)

// 'list' | 'edit' (steps 1-2) | 'result'
const view = ref<'list' | 'edit' | 'result' | 'compare'>('list')
const step = ref(1)
const editingId = ref<string | null>(null)
const currentScenario = ref<any | null>(null)
const sk = (key: string) => sideKey(key, scenarioSide(currentScenario.value, vehicleStore.vehicles))
const result = ref<any | null>(null)
const formError = ref('')

const fuelTypes = computed<any[]>(() => defaults.value?.ice || [])

// The server names each fuel in English; the catalog has the current language's name.
const fuelLabel = (f: { fuel_type: string; label: string }) => (te(`comparison.fuelTypes.${f.fuel_type}`) ? t(`comparison.fuelTypes.${f.fuel_type}`) : f.label)

// Electric and plug-in hybrid vehicles use their recorded energy costs.
const canCompareTrackedVehicle = computed(() => !!vehicleStore.activeVehicle && vehicleStore.canCharge)

function emptyForm() {
  return {
    mode: (canCompareTrackedVehicle.value ? 'RETROSPECTIVE' : 'PROJECTION') as Mode,
    name: '',
    annual_km: 12000,
    years: 5,
    ice: {
      fuel_type: 'SP95_E10',
      l_per_100km: 6.5,
      fuel_price: 1.75,
      purchase_price: 0,
      resale_value: 0,
      maintenance_yearly: 700,
      insurance_yearly: 650,
      tax_yearly: 0,
    },
    options: {
      fuel_inflation_pct: 0,
      electricity_inflation_pct: 0,
      cost_inflation_pct: 0,
      ev_incentives: 0,
    },
    ev: {
      kwh_per_100km: 16,
      eur_per_kwh: 0.2,
      purchase_price: 0,
      resale_value: 0,
      maintenance_yearly: 300,
      insurance_yearly: 800,
      tax_yearly: 0,
    },
  }
}

const form = reactive(emptyForm())
const showAdvanced = ref(false)

const isRetro = computed(() => form.mode === 'RETROSPECTIVE')

const iceFields = [
  { key: 'purchase_price', label: 'comparison.fields.purchasePrice' },
  { key: 'resale_value', label: 'comparison.fields.resaleValue' },
  { key: 'maintenance_yearly', label: 'comparison.fields.maintenanceYearly' },
  { key: 'insurance_yearly', label: 'comparison.fields.insuranceYearly' },
  { key: 'tax_yearly', label: 'comparison.fields.taxYearly' },
] as const

const evFields = [
  { key: 'purchase_price', label: 'comparison.fields.purchasePriceNet' },
  { key: 'resale_value', label: 'comparison.fields.resaleValue' },
  { key: 'maintenance_yearly', label: 'comparison.fields.maintenanceYearly' },
  { key: 'insurance_yearly', label: 'comparison.fields.insuranceYearly' },
] as const

// Amounts are in the active vehicle's currency: the tracked side comes from its own data, and the
// hypothetical figures are typed in by the same user in the currency they think in
const currency = computed(() => vehicleStore.currency)
const currencySign = computed(() => currencySymbol(currency.value))

function fmtMoney(v: number | null | undefined, digits = 0): string {
  return Number(v || 0).toLocaleString(intlLocale(), {
    style: 'currency',
    currency: currency.value,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

function fmtKm(v: number): string {
  return formatDistanceValue(Number(v || 0))
}

async function loadScenarios() {
  loading.value = true
  loadError.value = null
  try {
    scenarios.value = await api.getComparisonScenarios()
  } catch (err) {
    console.error('Failed to load comparisons', err)
    loadError.value = (err as Error)?.message ?? ''
  } finally {
    loading.value = false
  }
}

async function loadDefaults() {
  // A tracked combustion vehicle prefills the ICE side of a projection with its measured consumption and fuel price
  const vehicleId = isRetro.value || vehicleStore.fuelOnly ? vehicleStore.activeVehicle?.id : undefined
  try {
    defaults.value = await api.getComparisonDefaults(vehicleId)
  } catch (err) {
    console.error('Failed to load comparison defaults', err)
  }
}

function applyFuelDefaults() {
  const d = fuelTypes.value.find((f) => f.fuel_type === form.ice.fuel_type)
  if (!d) return
  form.ice.l_per_100km = d.l_per_100km
  form.ice.fuel_price = d.fuel_price
}

async function startNew() {
  Object.assign(form, emptyForm())
  showAdvanced.value = false
  editingId.value = null
  formError.value = ''
  step.value = 1
  view.value = 'edit'
  await loadDefaults()
  applyDefaultsToForm()
}

function applyDefaultsToForm() {
  const d = defaults.value
  if (!d) return
  form.annual_km = d.annual_km
  form.ice.maintenance_yearly = d.maintenance_yearly
  form.ice.insurance_yearly = d.insurance_yearly
  if (d.ev_kwh_per_100km) form.ev.kwh_per_100km = d.ev_kwh_per_100km
  if (d.ev_eur_per_kwh) form.ev.eur_per_kwh = d.ev_eur_per_kwh
  applyFuelDefaults()
  if (d.ice_l_per_100km) form.ice.l_per_100km = d.ice_l_per_100km
  if (d.ice_fuel_price) form.ice.fuel_price = d.ice_fuel_price
}

async function onModeChange() {
  await loadDefaults()
  applyDefaultsToForm()
}

function editScenario(sc: any) {
  Object.assign(form, emptyForm(), {
    mode: sc.mode,
    name: sc.name,
    annual_km: sc.annual_km,
    years: sc.years,
    ice: { ...sc.ice },
    ev: sc.ev ? { ...sc.ev } : emptyForm().ev,
    options: { ...emptyForm().options, ...(sc.options || {}) },
  })
  showAdvanced.value = Object.values(form.options).some((v) => Number(v) !== 0)
  editingId.value = sc.id
  formError.value = ''
  step.value = 1
  view.value = 'edit'
  loadDefaults()
}

function buildPayload() {
  const payload: any = {
    name: form.name,
    mode: form.mode,
    annual_km: Number(form.annual_km),
    years: Number(form.years),
    ice: {
      ...form.ice,
      l_per_100km: Number(form.ice.l_per_100km),
      fuel_price: Number(form.ice.fuel_price),
    },
    options: {
      fuel_inflation_pct: Number(form.options.fuel_inflation_pct) || 0,
      electricity_inflation_pct: Number(form.options.electricity_inflation_pct) || 0,
      cost_inflation_pct: Number(form.options.cost_inflation_pct) || 0,
      // Incentives only apply when the electric vehicle is described by the user
      ev_incentives: isRetro.value ? 0 : Number(form.options.ev_incentives) || 0,
    },
  }
  if (isRetro.value) {
    payload.vehicle_id = vehicleStore.activeVehicle?.id
  } else {
    payload.ev = {
      ...form.ev,
      kwh_per_100km: Number(form.ev.kwh_per_100km),
      eur_per_kwh: Number(form.ev.eur_per_kwh),
    }
  }
  for (const side of [payload.ice, payload.ev]) {
    if (!side) continue
    for (const k of ['purchase_price', 'resale_value', 'maintenance_yearly', 'insurance_yearly', 'tax_yearly']) {
      side[k] = Number(side[k] || 0)
    }
  }
  return payload
}

function stepError(): string {
  if (step.value === 1) {
    if (!form.name.trim()) return t('comparison.errors.name')
    if (!(Number(form.annual_km) > 0)) return t('comparison.errors.annualKm')
    if (!(Number(form.years) >= 1 && Number(form.years) <= 15)) return t('comparison.errors.years')
    if (isRetro.value && !canCompareTrackedVehicle.value) return t('comparison.errors.trackedNeeded')
  }
  if (step.value === 2) {
    if (!(Number(form.ice.l_per_100km) > 0)) return t('comparison.errors.iceConsumption')
    if (!(Number(form.ice.purchase_price) > 0)) return t('comparison.errors.icePrice')
    if (!isRetro.value && !(Number(form.ev.purchase_price) > 0)) return t('comparison.errors.evPrice')
  }
  return ''
}

async function nextStep() {
  formError.value = stepError()
  if (formError.value) return
  if (step.value < 2) {
    step.value += 1
    return
  }
  await saveAndCompute()
}

function prevStep() {
  formError.value = ''
  if (step.value > 1) step.value -= 1
  else view.value = 'list'
}

async function saveAndCompute() {
  saving.value = true
  try {
    const payload = buildPayload()
    const saved = editingId.value
      ? await api.updateComparisonScenario(editingId.value, payload)
      : await api.createComparisonScenario(payload)
    await loadScenarios()
    await openResult(saved)
  } catch (err: any) {
    formError.value = err?.message || t('comparison.errors.saveFailed')
  } finally {
    saving.value = false
  }
}

async function openResult(sc: any) {
  currentScenario.value = sc
  result.value = null
  view.value = 'result'
  try {
    result.value = await api.getComparisonResult(sc.id)
  } catch (err: any) {
    await showAlert(err?.message || t('comparison.errors.calcFailed'), t('comparison.comparisonView.title'), 'danger')
    view.value = 'list'
    return
  }
  await nextTick()
  renderChart()
}

async function removeScenario(sc: any) {
  const ok = await showConfirm({
    title: t('comparison.comparisonView.deleteTitle'),
    message: t('comparison.comparisonView.deleteMessage', { name: sc.name }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteComparisonScenario(sc.id)
    await loadScenarios()
  } catch (err: any) {
    await showAlert(err?.message || t('comparison.errors.deleteFailed'), t('comparison.comparisonView.deletion'), 'danger')
  }
}

// --- Result presentation ---

const savings = computed<number>(() => Number(result.value?.ev_savings || 0))

const verdict = computed(() => {
  if (!result.value) return ''
  const n = result.value.years_count
  const abs = fmtMoney(Math.abs(savings.value))
  if (Math.abs(savings.value) < 1) return t('comparison.verdict.same', n)
  return savings.value > 0
    ? t(sk('comparison.verdict.less'), { count: n, amount: abs })
    : t(sk('comparison.verdict.more'), { count: n, amount: abs })
})

const breakEvenText = computed(() => {
  if (!result.value) return ''
  const be = result.value.break_even_year
  if (be === undefined || be === null) return t('comparison.breakEven.notReached')
  if (be === 0) return t(sk('comparison.breakEven.immediate'))
  return t('comparison.breakEven.after', { years: Number(be).toLocaleString(intlLocale()) })
})

const costRows = computed(() => {
  const r = result.value
  if (!r) return []
  return [
    { label: t('comparison.rows.energy'), ev: r.ev.energy, ice: r.ice.energy },
    { label: t('comparison.rows.maintenance'), ev: r.ev.maintenance, ice: r.ice.maintenance },
    { label: t('comparison.rows.insurance'), ev: r.ev.insurance, ice: r.ice.insurance },
    { label: t('comparison.rows.tax'), ev: r.ev.tax, ice: r.ice.tax },
    { label: t('comparison.rows.depreciation'), ev: r.ev.depreciation, ice: r.ice.depreciation },
  ]
})

const chartRef = ref<HTMLCanvasElement | null>(null)
const barRef = ref<HTMLCanvasElement | null>(null)
const tornadoRef = ref<HTMLCanvasElement | null>(null)
let charts: Chart[] = []

function destroyChart() {
  charts.forEach((c) => c.destroy())
  charts = []
}

const axisStyle = { ticks: { color: '#94a3b8' }, grid: { color: '#1e293b' } }
const eurAxis = { ticks: { color: '#94a3b8', callback: (v: any) => fmtMoney(Number(v)) }, grid: { color: '#1e293b' } }

function renderChart() {
  destroyChart()
  if (!result.value) return
  const r = result.value

  if (chartRef.value) {
    const points = r.cumulative as { year: number; ev: number; ice: number }[]
    charts.push(new Chart(chartRef.value, {
      type: 'line',
      data: {
        labels: points.map((p) => (p.year === 0 ? t('comparison.chart.purchase') : t('comparison.chart.year', { year: p.year }))),
        datasets: [
          { label: t(sk('comparison.electric')), data: points.map((p) => p.ev), borderColor: '#38bdf8', backgroundColor: '#38bdf8', tension: 0.15 },
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
        scales: { x: axisStyle, y: eurAxis },
      },
    }))
  }

  if (barRef.value) {
    const colors = ['#38bdf8', '#a78bfa', '#34d399', '#94a3b8', '#f472b6']
    charts.push(new Chart(barRef.value, {
      type: 'bar',
      data: {
        labels: [t(sk('comparison.electric')), t('comparison.combustion')],
        datasets: costRows.value.map((row, i) => ({
          label: row.label,
          data: [row.ev, row.ice],
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
        scales: { x: { ...axisStyle, stacked: true }, y: { ...eurAxis, stacked: true } },
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
        scales: { x: eurAxis, y: axisStyle },
      },
    }))
  }
}

// --- Compare several scenarios ---

const selectedIds = ref<string[]>([])
const compareItems = ref<{ scenario: any; result: any }[]>([])
const MAX_COMPARE = 3

function toggleSelected(id: string) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((s) => s !== id)
  } else if (selectedIds.value.length < MAX_COMPARE) {
    selectedIds.value = [...selectedIds.value, id]
  }
}

async function openCompare() {
  const chosen = scenarios.value.filter((s) => selectedIds.value.includes(s.id))
  try {
    const results = await Promise.all(chosen.map((s) => api.getComparisonResult(s.id)))
    compareItems.value = chosen.map((scenario, i) => ({ scenario, result: results[i] }))
    view.value = 'compare'
  } catch (err: any) {
    await showAlert(err?.message || t('comparison.errors.compareFailed'), t('comparison.comparisonView.title'), 'danger')
  }
}

// --- Export ---

function exportResultCsv() {
  const r = result.value
  if (!r) return
  const name = (currentScenario.value?.name || t('comparison.csv.defaultName')).replace(/[^\w-]+/g, '-')
  const rows: (string | number)[][] = costRows.value.map((row) => [row.label, Number(row.ev).toFixed(2), Number(row.ice).toFixed(2)])
  rows.push([t('comparison.csv.total'), Number(r.ev.total).toFixed(2), Number(r.ice.total).toFixed(2)])
  rows.push([t('comparison.csv.perMonth'), Number(r.ev.per_month).toFixed(2), Number(r.ice.per_month).toFixed(2)])
  rows.push([t('comparison.csv.costPerKm', { unit: distanceUnit() }), perDistance(r.ev.cost_per_km).toFixed(3), perDistance(r.ice.cost_per_km).toFixed(3)])
  rows.push([t(sk('comparison.csv.gap')), Number(r.ev_savings).toFixed(2), ''])
  rows.push(['', '', ''])
  rows.push(t(sk('comparison.csv.cumulativeHeader')).split(','))
  for (const p of r.cumulative) rows.push([p.year, Number(p.ev).toFixed(2), Number(p.ice).toFixed(2)])
  downloadCsv(`${t('comparison.csv.filePrefix')}-${name}`, t(sk('comparison.csv.header'), { cur: currency.value }).split(','), rows)
}

function printResult() {
  window.print()
}

watch(view, (v) => {
  if (v !== 'result') destroyChart()
})

onMounted(async () => {
  await loadScenarios()
})

onBeforeUnmount(destroyChart)
</script>

<template>
  <div class="space-y-6">
    <PageHeader :title="$t('comparison.comparisonView.electricCombustionComparison')" :icon="PageIcon">
      {{ $t('comparison.comparisonView.forInformationOnlyNoneOf') }}
      <template #actions>
      <button
        v-if="view === 'list'"
        class="btn btn-lg btn-primary"
        @click="startNew"
      >
        <Plus class="w-4 h-4" /> {{ $t('comparison.comparisonView.newComparison') }}
      </button>
    
      </template>
    </PageHeader>

    <!-- Scenario list -->
    <div v-if="view === 'list'">
      <div v-if="loading" class="text-sm text-slate-400">{{ $t('comparison.comparisonView.loading') }}</div>
      <LoadError v-else-if="loadError !== null" :message="loadError" @retry="loadScenarios" />
      <EmptyState v-else-if="scenarios.length === 0">
        {{ $t('comparison.comparisonView.noComparisonYetCreateOne') }}
      </EmptyState>
      <div v-else class="space-y-3">
      <div v-if="scenarios.length >= 2" class="flex items-center justify-between gap-3 text-xs text-slate-400">
        <span>{{ $t('comparison.comparisonView.tick2Or3Comparisons') }}</span>
        <button
          type="button"
          :disabled="selectedIds.length < 2"
          class="bg-slate-800 hover:bg-slate-700 disabled:opacity-40 text-slate-200 border border-slate-700 font-semibold px-3 py-2 rounded-xl flex items-center gap-2"
          @click="openCompare"
        >
          <GitCompare class="w-4 h-4" /> {{ $t('comparison.comparisonView.compare', { length: selectedIds.length }) }}
        </button>
      </div>
      <ul class="grid gap-3 md:grid-cols-2">
        <li v-for="sc in scenarios" :key="sc.id" class="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex items-center justify-between gap-3">
          <input
            :id="`cmp-select-${sc.id}`"
            type="checkbox"
            class="rounded border-slate-600 bg-slate-800 shrink-0"
            :checked="selectedIds.includes(sc.id)"
            :disabled="!selectedIds.includes(sc.id) && selectedIds.length >= MAX_COMPARE"
            :aria-label="$t('comparison.comparisonView.selectFor', { name: sc.name })"
            @change="toggleSelected(sc.id)"
          />
          <button class="text-left min-w-0 flex-1" @click="openResult(sc)">
            <div class="text-sm font-semibold text-white truncate">{{ sc.name }}</div>
            <div class="text-xs text-slate-400">
              {{ sc.mode === 'RETROSPECTIVE' ? $t('comparison.comparisonView.trackedVehicle') : $t('comparison.comparisonView.projection') }} · {{ $t('comparison.comparisonView.scenarioUsage', { unit: distanceUnit(), km: fmtKm(sc.annual_km), years: sc.years }) }}
            </div>
          </button>
          <div class="flex items-center gap-1.5 shrink-0">
            <button :aria-label="$t('comparison.comparisonView.editScenario', { name: sc.name })" class="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300" @click="editScenario(sc)">
              <Pencil class="w-4 h-4" />
            </button>
            <button :aria-label="$t('comparison.comparisonView.deleteScenario', { name: sc.name })" class="p-2 rounded-lg bg-slate-800 hover:bg-red-900/60 text-red-300" @click="removeScenario(sc)">
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </li>
      </ul>
      </div>
    </div>

    <!-- Compare several scenarios -->
    <div v-else-if="view === 'compare'" class="space-y-5">
      <button class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5 no-print" @click="view = 'list'">
        <ArrowLeft class="w-4 h-4" /> {{ $t('comparison.comparisonView.myComparisons') }}
      </button>
      <ComparisonCompare :items="compareItems" />
    </div>

    <!-- Editor (steps 1 and 2) -->
    <form v-else-if="view === 'edit'" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 space-y-5" @submit.prevent="nextStep">
      <div class="text-xs text-slate-400">{{ $t('comparison.comparisonView.stepOf', { step, name: step === 1 ? $t('comparison.comparisonView.usage') : isRetro ? $t('comparison.comparisonView.equivalentIce') : $t('comparison.comparisonView.comparedVehicles') }) }}</div>

      <template v-if="step === 1">
        <div>
          <label for="cmp-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.comparisonName') }}</label>
          <input id="cmp-name" v-model="form.name" maxlength="100" :placeholder="$t('comparison.comparisonView.eGComparedWithA')" class="field" />
        </div>
        <div>
          <label for="cmp-mode" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.comparisonType') }}</label>
          <select id="cmp-mode" v-model="form.mode" :disabled="!!editingId" class="field" @change="onModeChange">
            <option value="RETROSPECTIVE" :disabled="!canCompareTrackedVehicle">
              {{ $t('comparison.comparisonView.myTrackedEv') }}{{ vehicleStore.activeVehicle ? ` (${vehicleStore.activeVehicle.name})` : '' }} {{ $t('comparison.comparisonView.vsIce') }}
            </option>
            <option value="PROJECTION">{{ $t('comparison.comparisonView.projectionICompareTwoVehicles') }}</option>
          </select>
          <p v-if="isRetro" class="text-xs text-slate-400 mt-1">{{ $t('comparison.comparisonView.theElectricVehicleSCosts') }}</p>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="cmp-km" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.kilometresPerYear', { unit: distanceUnit() }) }}</label>
            <DistanceInput id="cmp-km" v-model="form.annual_km" :digits="0" min="1" step="any" class="field" />
            <p v-if="defaults && isRetro" class="text-xs text-slate-400 mt-1">
              {{ defaults.annual_km_from_data ? $t('comparison.comparisonView.fromHistory') : $t('comparison.comparisonView.defaultValue') }}
            </p>
          </div>
          <div>
            <label for="cmp-years" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.durationYears') }}</label>
            <input id="cmp-years" v-model.number="form.years" type="number" min="1" max="15" class="field" />
          </div>
        </div>
      </template>

      <template v-else>
        <div class="space-y-3">
          <h2 class="text-sm font-semibold text-warning-300">{{ $t('comparison.comparisonView.equivalentCombustionVehicle') }}</h2>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="cmp-fuel" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.fuel') }}</label>
              <select id="cmp-fuel" v-model="form.ice.fuel_type" class="field" @change="applyFuelDefaults">
                <option v-for="f in fuelTypes" :key="f.fuel_type" :value="f.fuel_type">{{ fuelLabel(f) }}</option>
              </select>
            </div>
            <div>
              <label for="cmp-ice-l100" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.consumptionL100Km', { unit: distanceUnit() }) }}</label>
              <DistanceInput kind="per-distance" id="cmp-ice-l100" v-model="form.ice.l_per_100km" min="0.1" step="any" class="field" />
            </div>
            <div>
              <label for="cmp-ice-price" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.fuelPriceL', { cur: currencySign }) }}</label>
              <NumberInput id="cmp-ice-price" v-model="form.ice.fuel_price" min="0" class="field" />
            </div>
          </div>
          <p class="text-xs text-slate-400 flex items-center gap-1"><Info class="w-3 h-3" /> {{ defaults?.source ? apiMessageText(defaults.source) : $t('comparison.comparisonView.indicative') }}</p>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div v-for="f in iceFields" :key="f.key">
              <label :for="`cmp-ice-${f.key}`" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t(f.label, { cur: currencySign }) }}</label>
              <NumberInput :id="`cmp-ice-${f.key}`" v-model="form.ice[f.key]" min="0" class="field" />
            </div>
          </div>
        </div>

        <div v-if="!isRetro" class="space-y-3 pt-2 border-t border-slate-800">
          <h2 class="text-sm font-semibold text-info-300">{{ $t('comparison.comparisonView.electricVehicle') }}</h2>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="cmp-ev-kwh" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.consumptionKwh100Km', { unit: distanceUnit() }) }}</label>
              <DistanceInput kind="per-distance" id="cmp-ev-kwh" v-model="form.ev.kwh_per_100km" min="0.1" step="any" class="field" />
            </div>
            <div>
              <label for="cmp-ev-price" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.averageElectricityPriceKwh', { cur: currencySign }) }}</label>
              <NumberInput id="cmp-ev-price" v-model="form.ev.eur_per_kwh" min="0" class="field" />
            </div>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div v-for="f in evFields" :key="f.key">
              <label :for="`cmp-ev-${f.key}`" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t(f.label, { cur: currencySign }) }}</label>
              <NumberInput :id="`cmp-ev-${f.key}`" v-model="form.ev[f.key]" min="0" class="field" />
            </div>
          </div>
        </div>
      </template>

      <div v-if="step === 2" class="pt-2 border-t border-slate-800">
        <button
          type="button"
          class="flex items-center gap-1.5 text-xs font-semibold text-slate-300 hover:text-white"
          :aria-expanded="showAdvanced"
          aria-controls="cmp-advanced"
          @click="showAdvanced = !showAdvanced"
        >
          <ChevronDown class="w-4 h-4 transition-transform" :class="showAdvanced ? 'rotate-180' : ''" /> {{ $t('comparison.comparisonView.fineTuneInflationGrants') }}
        </button>
        <div v-show="showAdvanced" id="cmp-advanced" class="mt-3 space-y-3">
          <p class="text-xs text-slate-400">{{ $t('comparison.comparisonView.averageYearlyPriceChangeApplied') }}</p>
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div>
              <label for="cmp-infl-fuel" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.fuelYear') }}</label>
              <NumberInput id="cmp-infl-fuel" v-model="form.options.fuel_inflation_pct" min="-10" max="30" class="field" />
            </div>
            <div>
              <label for="cmp-infl-elec" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.electricityYear') }}</label>
              <NumberInput id="cmp-infl-elec" v-model="form.options.electricity_inflation_pct" min="-10" max="30" class="field" />
            </div>
            <div>
              <label for="cmp-infl-cost" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.maintenanceInsuranceTaxesYear') }}</label>
              <NumberInput id="cmp-infl-cost" v-model="form.options.cost_inflation_pct" min="-10" max="30" class="field" />
            </div>
          </div>
          <div v-if="!isRetro">
            <label for="cmp-ev-incentives" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('comparison.comparisonView.electricPurchaseGrantsDeductedFrom', { cur: currencySign }) }}</label>
            <NumberInput id="cmp-ev-incentives" v-model="form.options.ev_incentives" min="0" class="field sm:w-1/2" />
          </div>
        </div>
      </div>

      <p v-if="formError" class="text-xs text-red-300" role="alert">{{ formError }}</p>

      <div class="flex items-center justify-between pt-1">
        <button type="button" class="btn btn-lg btn-secondary" @click="prevStep">
          <ArrowLeft class="w-4 h-4" /> {{ step === 1 ? $t('common.cancel') : $t('comparison.comparisonView.back') }}
        </button>
        <button type="submit" :disabled="saving" class="btn btn-lg btn-primary">
          {{ step === 2 ? (saving ? $t('comparison.comparisonView.calculating') : $t('comparison.comparisonView.viewResult')) : $t('comparison.comparisonView.next') }} <ArrowRight class="w-4 h-4" />
        </button>
      </div>
    </form>

    <!-- Result -->
    <div v-else-if="view === 'result'" class="space-y-5 print-area">
      <div class="flex items-center justify-between gap-3 no-print">
        <button class="text-xs text-slate-400 hover:text-white flex items-center gap-1.5" @click="view = 'list'">
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
          :class="savings >= 0 ? 'bg-success-500/10 border-success-500/30' : 'bg-warning-500/10 border-warning-500/30'"
        >
          <component :is="savings >= 0 ? TrendingDown : TrendingUp" class="w-8 h-8 shrink-0" :class="savings >= 0 ? 'text-success-400' : 'text-warning-400'" />
          <div>
            <div class="text-lg font-bold text-white">{{ verdict }}</div>
            <div class="text-xs text-slate-400 mt-0.5">{{ currentScenario?.name }} · {{ $t('comparison.comparisonView.kmPerYear', { unit: distanceUnit(), km: fmtKm(result.annual_km) }) }}</div>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
            <div class="flex items-center justify-between mb-2">
              <h2 class="text-sm font-semibold text-info-300">{{ $t(sk('comparison.comparisonView.electric')) }}</h2>
              <span class="text-xs px-2 py-0.5 rounded-full border" :class="result.mode === 'RETROSPECTIVE' ? 'border-success-500/40 text-success-300' : 'border-slate-600 text-slate-400'">
                {{ result.mode === 'RETROSPECTIVE' ? $t('comparison.comparisonView.actual') : $t('comparison.comparisonView.estimated') }}
              </span>
            </div>
            <div class="text-2xl font-bold text-white">{{ fmtMoney(result.ev.total) }}</div>
            <div class="text-xs text-slate-400 mt-1">{{ $t('comparison.comparisonView.monthKm', { unit: distanceUnit(), per_month: fmtMoney(result.ev.per_month), cost_per_km: fmtMoney(perDistance(result.ev.cost_per_km), 3) }) }}</div>
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
                <td>{{ fmtMoney(row.ev) }}</td>
                <td>{{ fmtMoney(row.ice) }}</td>
              </tr>
              <tr class="border-t border-slate-700 text-right font-semibold text-white">
                <th scope="row" class="text-left py-1.5">{{ $t('comparison.comparisonView.total') }}</th>
                <td>{{ fmtMoney(result.ev.total) }}</td>
                <td>{{ fmtMoney(result.ice.total) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
          <h2 class="text-sm font-semibold text-white mb-3">{{ $t('comparison.comparisonView.costBreakdownOverThePeriod') }}</h2>
          <div class="h-64"><canvas ref="barRef" :aria-label="$t(sk('comparison.comparisonView.costByCategoryElectricAnd'))" role="img"></canvas></div>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
          <h2 class="text-sm font-semibold text-white mb-1">{{ $t('comparison.comparisonView.cumulativeCost') }}</h2>
          <p class="text-xs text-slate-400 mb-3">{{ breakEvenText }}</p>
          <div class="h-64"><canvas ref="chartRef" :aria-label="$t(sk('comparison.comparisonView.cumulativeCostElectricAndCombustion'))" role="img"></canvas></div>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-2xl p-4">
          <h2 class="text-sm font-semibold text-white mb-1">{{ $t('comparison.comparisonView.sensitivity') }}</h2>
          <p class="text-xs text-slate-400 mb-3">{{ $t(sk('comparison.comparisonView.effectOnTheElectricSaving')) }}</p>
          <div class="h-48 mb-3"><canvas ref="tornadoRef" :aria-label="$t('comparison.comparisonView.sensitivityOfTheGapTo')" role="img"></canvas></div>
          <ul class="text-xs text-slate-300 space-y-1">
            <li v-for="s in result.sensitivity" :key="s.label.code" class="flex justify-between">
              <span>{{ apiMessageText(s.label) }}</span>
              <span>{{ s.ev_savings >= 0 ? $t(sk('comparison.comparisonView.evLess'), { amount: fmtMoney(s.ev_savings) }) : $t(sk('comparison.comparisonView.evMore'), { amount: fmtMoney(-s.ev_savings) }) }}</span>
            </li>
          </ul>
        </div>

        <div class="bg-slate-900/60 border border-slate-800 rounded-2xl p-4">
          <h2 class="text-xs font-semibold text-slate-300 mb-2 flex items-center gap-1.5"><Info class="w-3.5 h-3.5" /> {{ $t('comparison.comparisonView.assumptions') }}</h2>
          <ul class="list-disc pl-4 text-xs text-slate-400 space-y-1">
            <li v-for="(a, i) in result.assumptions" :key="i">{{ apiMessageText(a) }}</li>
          </ul>
        </div>

        <div class="flex gap-2 no-print">
          <button class="btn btn-lg btn-secondary" @click="editScenario(currentScenario)">
            <Pencil class="w-4 h-4" /> {{ $t('comparison.comparisonView.editTheAssumptions') }}
          </button>
        </div>
      </template>
    </div>
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
