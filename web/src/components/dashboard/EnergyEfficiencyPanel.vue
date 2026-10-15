<script setup lang="ts">
import GlossaryTip from '@/components/GlossaryTip.vue'
import KpiCard from '@/components/KpiCard.vue'
import EmptyState from '@/components/EmptyState.vue'
import { t } from '@/i18n'
import { formatMonthLabel } from '@/utils/dates'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { ChevronDown, Gauge } from 'lucide-vue-next'
import { api } from '@/services/api'
import EnergyBatterySection from './EnergyBatterySection.vue'
import EnergyTemperatureSection from './EnergyTemperatureSection.vue'
import CostDonut from '@/components/costs/CostDonut.vue'
import { AXIS_TEXT, GRID_COLOR, fmt, fmtMoney, fmtPercent, perUnit, mergeAcDcClasses, type ChargeClass, type EnergyStats } from './energyStats'
import { filterMonthsByRange, rangeToMonths } from '@/utils/dashboard'
import { categoryColor, placeCategoryRows, type PlaceStats } from '@/utils/chargingInsights'
import { placeCategoryKey } from '@/utils/chargingPlaces'
import { usePreferencesStore } from '@/stores/preferences'
import { currencySymbol, formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { distanceUnit } from '@/units'

Chart.register(...registerables)

const props = defineProps<{
  vehicleId: string
  grafanaUrl?: string | null
  // Bumped after a synchronization so the figures follow the imported drives and charges
  syncKey?: number
  // Dashboard: link to the energy page. Energy page: explain why the panel is empty.
  showLink?: boolean
  showEmpty?: boolean
  // Dashboard: a closed disclosure with the headline figures; the charts, charging habits, temperature and battery stay on the energy page
  summary?: boolean
}>()

const vehicleStore = useVehicleStore()
const currency = computed(() => vehicleStore.currency)

const stats = ref<EnergyStats | null>(null)
// Where the energy was charged, over the same period: the figures of the place view of the charges tab
const placeStats = ref<PlaceStats | null>(null)
const placeRows = computed(() => (placeStats.value ? placeCategoryRows(placeStats.value) : []))
const placeDonutItems = computed(() => placeRows.value.map((r) => ({ label: placeLabel(r.category), color: categoryColor(r.category), amount: r.kwh })))
const placeTotalKwh = computed(() => placeRows.value.reduce((sum, r) => sum + r.kwh, 0))
const placeLabel = (category: string) => (category ? t(placeCategoryKey(category)) : t('expenses.chargingInsights.unclassified'))
const failed = ref(false)
const prefs = usePreferencesStore()
const range = computed(() => prefs.dashboardRange)

const consumptionRef = ref<HTMLCanvasElement | null>(null)
const costRef = ref<HTMLCanvasElement | null>(null)
let consumptionChart: Chart | null = null
let costChart: Chart | null = null

const classLabel = (c: ChargeClass['class']) => ({
  label: t(`dashboard.energyEfficiencyPanel.chargeClass.${c}.label`),
  hint: t(`dashboard.energyEfficiencyPanel.chargeClass.${c}.hint`),
})
const CLASS_COLOR: Record<'AC' | 'DC', string> = { AC: '#38bdf8', DC: '#f59e0b' }
const acDc = computed(() => mergeAcDcClasses(stats.value?.charge_classes ?? []))
const donutItems = computed(() =>
  acDc.value.classes.map((c) => ({ label: classLabel(c.class).label, color: CLASS_COLOR[c.class as 'AC' | 'DC'], amount: c.kwh_added })),
)

const visibleMonths = computed(() => filterMonthsByRange(stats.value?.months ?? [], range.value))

// A line or bar chart of one value, or of an estimate repeated every month, tells nothing the figures above do not already say.
const hasTrend = (values: (number | null | undefined)[]) => new Set(values.filter((v): v is number => typeof v === 'number').map((v) => v.toFixed(2))).size >= 2
const consumptionTrend = computed(() => hasTrend(visibleMonths.value.map((m) => m.consumption_kwh_100km)))
const costTrend = computed(() => hasTrend(visibleMonths.value.map((m) => m.cost_per_100km)))

const basis = computed(() => stats.value?.basis)
const showConsumption = computed(() => basis.value?.consumption !== 'unavailable' && consumptionTrend.value)
const showCost = computed(() => basis.value?.cost !== 'unavailable' && costTrend.value)
const hasDerived = computed(() => basis.value?.consumption === 'derived' || basis.value?.cost === 'derived')

const hasData = computed(() => (stats.value?.months.length ?? 0) > 0)

const totalKwh = computed(() => acDc.value.classes.reduce((sum, c) => sum + c.kwh_added, 0))

const share = (c: ChargeClass) => (totalKwh.value > 0 ? Math.round((c.kwh_added / totalKwh.value) * 100) : 0)

async function load() {
  failed.value = false
  try {
    stats.value = await api.getEnergyStats(props.vehicleId)
  } catch (err) {
    console.error('Failed to load energy statistics', err)
    stats.value = null
    failed.value = true
  }
}


function baseOptions(unit: string) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: { position: 'top' as const, labels: { color: AXIS_TEXT, font: { size: 11 } } },
      tooltip: {
        callbacks: {
          label: (ctx: any) => (ctx.raw === null || ctx.raw === undefined ? '' : `${ctx.dataset.label} : ${fmt(Number(ctx.raw), 2)} ${unit}`),
        },
      },
    },
    scales: {
      x: { grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT } },
      y: { beginAtZero: true, grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT, callback: (v: any) => `${v}` }, title: { display: true, text: unit, color: AXIS_TEXT } },
    },
  }
}

function drawCharts() {
  consumptionChart?.destroy()
  costChart?.destroy()
  consumptionChart = null
  costChart = null
  const months = visibleMonths.value
  if (months.length === 0) return
  const labels = months.map((m) => formatMonthLabel(m.month))

  if (consumptionRef.value) {
    consumptionChart = new Chart(consumptionRef.value, {
      type: 'line',
      data: {
        labels,
        datasets: [
          {
            label: t('dashboard.energyEfficiencyPanel.consumption'),
            data: months.map((m) => perUnit(m.consumption_kwh_100km) ?? null),
            borderColor: '#38bdf8',
            backgroundColor: '#38bdf8',
            tension: 0.25,
            spanGaps: true,
            pointRadius: 3,
          },
        ],
      },
      options: baseOptions(`kWh/100 ${distanceUnit()}`),
    })
  }

  if (costRef.value) {
    costChart = new Chart(costRef.value, {
      type: 'bar',
      data: {
        labels,
        datasets: [
          {
            type: 'bar',
            label: t('dashboard.energyEfficiencyPanel.month'),
            data: months.map((m) => perUnit(m.cost_per_100km) ?? null),
            backgroundColor: 'rgba(56, 189, 248, 0.7)',
            borderRadius: 4,
          },
          {
            type: 'line',
            label: t('dashboard.energyEfficiencyPanel.threeMonthAverage'),
            data: months.map((m) => perUnit(m.cost_per_100km_trailing) ?? null),
            borderColor: '#f59e0b',
            backgroundColor: '#f59e0b',
            borderWidth: 2,
            tension: 0.25,
            spanGaps: true,
            pointRadius: 2,
          },
        ],
      },
      options: baseOptions(`${currencySymbol(currency.value)}/100 ${distanceUnit()}`),
    })
  }
}

watch(
  () => [props.vehicleId, props.syncKey],
  () => load(),
  { immediate: true },
)

async function loadPlaces() {
  try {
    placeStats.value = await api.getChargingPlaceStats(props.vehicleId, { months: rangeToMonths(range.value) })
  } catch (err) {
    console.error('Failed to load charging place statistics', err)
    placeStats.value = null
  }
}
watch(
  () => [props.vehicleId, props.syncKey, range.value],
  () => loadPlaces(),
  { immediate: true },
)

watch([stats, range], async () => {
  await nextTick()
  drawCharts()
})

onBeforeUnmount(() => {
  consumptionChart?.destroy()
  costChart?.destroy()
})
</script>

<template>
  <EmptyState v-if="!hasData && showEmpty" :icon="Gauge" :title="$t('dashboard.energyEfficiencyPanel.noData')">
    {{ $t('dashboard.energyEfficiencyPanel.noDataHint') }}
  </EmptyState>
  <component
    :is="summary ? 'details' : 'section'"
    v-else-if="hasData"
    id="energy-efficiency"
    class="group scroll-mt-20 rounded-2xl border border-slate-800 bg-slate-900 p-5 shadow-sm"
    :class="summary ? 'open:space-y-4' : 'space-y-4'"
    aria-labelledby="energy-efficiency-title"
  >
    <component :is="summary ? 'summary' : 'div'" class="flex flex-col justify-between gap-2 sm:flex-row sm:items-center" :class="summary ? 'cursor-pointer list-none' : ''">
      <div>
        <h3 id="energy-efficiency-title" class="flex items-center gap-2 text-sm font-bold text-white">
          <Gauge class="h-4 w-4 text-info-400" aria-hidden="true" />
          {{ $t('dashboard.energyEfficiencyPanel.energyEfficiency') }}
        </h3>
        <p class="mt-0.5 text-xs text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.whatTheCarUsesAnd') }}</p>
      </div>
      <router-link v-if="showLink && !summary" to="/energy" class="tap-text text-xs font-semibold text-sky-400 underline">
        {{ $t('dashboard.energyEfficiencyPanel.seeEnergy') }}
      </router-link>
      <span v-if="summary" class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs font-semibold text-slate-300">
        <span>{{ fmt(perUnit(stats?.summary.consumption_kwh_100km), 1) }} kWh/100 {{ distanceUnit() }}</span>
        <span>{{ fmtMoney(perUnit(stats?.summary.cost_per_100km), currency) }}/100 {{ distanceUnit() }}</span>
        <ChevronDown class="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden="true" />
      </span>
    </component>

    <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <KpiCard
        compact
        :label="$t('dashboard.energyEfficiencyPanel.actualConsumption')"
        term="atTheWheel"
        :value="fmt(perUnit(stats?.summary.consumption_kwh_100km), 1)"
        :unit="` kWh/100 ${distanceUnit()}`"
      >
        <p class="mt-0.5 hidden text-xs text-slate-400 sm:block">{{ $t('dashboard.energyEfficiencyPanel.energyUsedWhileDrivingMeasured') }}</p>
      </KpiCard>
      <KpiCard
        compact
        :label="$t('dashboard.energyEfficiencyPanel.energyCost')"
        :value="fmtMoney(perUnit(stats?.summary.cost_per_100km), currency)"
        :unit="` /100 ${distanceUnit()}`"
      >
        <p class="mt-0.5 hidden text-xs text-slate-400 sm:block">{{ $t('dashboard.energyEfficiencyPanel.thatIsKwhOnAverage', { price_per_kwh: fmtMoney(stats?.summary.price_per_kwh, currency, 3) }) }}</p>
      </KpiCard>
      <KpiCard compact :label="$t('dashboard.energyEfficiencyPanel.chargingEfficiency')" :value="fmtPercent(stats?.summary.charge_efficiency)">
        <p v-if="stats?.summary.charge_efficiency == null" class="mt-0.5 text-xs text-slate-400">
          {{ $t('dashboard.energyEfficiencyPanel.chargingEfficiencyEmpty') }}
          <span class="block">
            <router-link to="/energy?tab=CHARGES" class="tap-text font-semibold text-sky-400 underline">{{ $t('dashboard.energyEfficiencyPanel.chargingEfficiencyAction') }}</router-link>
          </span>
        </p>
        <p v-else class="mt-0.5 hidden text-xs text-slate-400 sm:block">{{ $t('dashboard.energyEfficiencyPanel.energyStoredInTheBattery') }}</p>
      </KpiCard>
      <KpiCard
        compact
        :label="$t('dashboard.energyEfficiencyPanel.fullCharge')"
        :value="fmtMoney(stats?.summary.cost_per_full_charge ?? undefined, currency)"
        unit=" (0 → 100 %)"
      >
        <p class="mt-0.5 hidden text-xs text-slate-400 sm:block">{{ $t('dashboard.energyEfficiencyPanel.extrapolatedFromTheChargesWhose') }}</p>
      </KpiCard>
    </div>

    <p v-if="(stats?.summary.sessions_without_cost ?? 0) > 0" class="rounded-lg border border-warning-500/20 bg-warning-500/10 px-3 py-2 text-xs text-warning-300">
      {{ $t('dashboard.energyEfficiencyPanel.chargeSWithoutAKnown', { unit: distanceUnit(), sessions_without_cost: stats?.summary.sessions_without_cost }) }}
      <router-link to="/energy?tab=CHARGES" class="font-semibold underline">{{ $t('dashboard.energyEfficiencyPanel.completeThem') }}</router-link>
    </p>

    <p v-if="hasDerived" class="rounded-lg border border-info-500/20 bg-info-500/10 px-3 py-2 text-xs text-info-300">
      {{ $t('dashboard.energyEfficiencyPanel.derivedNote') }}
    </p>

    <div v-if="!summary && (showConsumption || showCost)" class="grid grid-cols-1 gap-4" :class="{ 'lg:grid-cols-2': showConsumption && showCost }">
      <div v-if="showConsumption">
        <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.energyEfficiencyPanel.monthlyConsumption') }}</h4>
        <div class="h-44 sm:h-56"><canvas ref="consumptionRef" :aria-label="$t('dashboard.energyEfficiencyPanel.monthlyConsumptionInKwhPer', { unit: distanceUnit() })"></canvas></div>
      </div>
      <div v-if="showCost">
        <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.energyEfficiencyPanel.energyCostPer100Km', { unit: distanceUnit() }) }}</h4>
        <div class="h-44 sm:h-56"><canvas ref="costRef" :aria-label="$t('dashboard.energyEfficiencyPanel.monthlyEnergyCostPer100', { unit: distanceUnit() })"></canvas></div>
      </div>
    </div>

    <!-- A table is not clipped by sr-only (its width ignores the 1px), so the class goes on a wrapper -->
    <div v-if="!summary" class="sr-only">
      <table>
        <caption>{{ $t('dashboard.energyEfficiencyPanel.consumptionAndEnergyCostPer') }}</caption>
        <thead>
          <tr><th>{{ $t('dashboard.energyEfficiencyPanel.month') }}</th><th>kWh/100 {{ distanceUnit() }}</th><th>{{ currencySymbol(currency) }}/100 {{ distanceUnit() }}</th><th>{{ $t('dashboard.energyEfficiencyPanel.3MonthAverage100Km', { unit: distanceUnit(), cur: currencySymbol(currency) }) }}</th></tr>
        </thead>
        <tbody>
          <tr v-for="m in visibleMonths" :key="m.month">
            <td>{{ formatMonthLabel(m.month) }}</td>
            <td>{{ fmt(perUnit(m.consumption_kwh_100km), 1) }}</td>
            <td>{{ fmt(perUnit(m.cost_per_100km), 2) }}</td>
            <td>{{ fmt(perUnit(m.cost_per_100km_trailing), 2) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="placeRows.length > 0">
      <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.energyEfficiencyPanel.byPlace') }}</h4>
      <div class="grid grid-cols-1 items-center gap-4 md:grid-cols-5">
        <div v-if="placeRows.length > 1" class="flex flex-col items-center justify-center rounded-xl border border-slate-800 bg-slate-950/60 p-3 md:col-span-2">
          <div class="relative h-40 w-full">
            <CostDonut
              :items="placeDonutItems"
              unit="kWh"
              :show-legend="false"
              :empty-label="$t('dashboard.energyEfficiencyPanel.noCharge')"
              :chart-label="$t('dashboard.energyEfficiencyPanel.byPlace')"
            />
            <div class="pointer-events-none absolute inset-0 flex items-center justify-center text-center" aria-hidden="true">
              <span class="text-base font-bold leading-tight text-white">{{ fmt(placeTotalKwh, 0) }}<span class="block text-xs font-medium text-slate-400">kWh</span></span>
            </div>
          </div>
        </div>
        <ul :class="placeRows.length > 1 ? 'md:col-span-3' : 'md:col-span-5'" class="space-y-1.5 text-xs">
          <li v-for="r in placeRows" :key="r.category || 'unclassified'" class="rounded-xl border border-slate-800 bg-slate-950/60 px-3 py-2">
            <div class="flex flex-col gap-0.5 sm:flex-row sm:flex-wrap sm:items-baseline sm:justify-between sm:gap-x-3">
              <span class="flex items-center gap-2 font-semibold text-white">
                <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: categoryColor(r.category) }" aria-hidden="true"></span>
                {{ placeLabel(r.category) }}
              </span>
              <span class="text-slate-300">
                {{ $t('dashboard.energyEfficiencyPanel.placeShare', { share: Math.round(r.sharePct), kwh: fmt(r.kwh, 0) }) }}
                · {{ r.pricePerKwh === null ? $t('expenses.chargingInsights.noPrice') : $t('expenses.chargingInsights.pricePerKwh', { price: fmtMoney(r.pricePerKwh, currency, 3) }) }}
              </span>
            </div>
            <p v-if="r.unknownSessions > 0" class="mt-0.5 text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.placeUnknown', { count: r.unknownSessions }) }}</p>
          </li>
        </ul>
      </div>
      <router-link to="/energy?tab=CHARGES" class="tap-text mt-1 inline-block text-xs font-semibold text-sky-400 underline">
        {{ $t('dashboard.energyEfficiencyPanel.placeLink') }}
      </router-link>
    </div>

    <div v-if="!summary && acDc.classes.length > 0">
      <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.energyEfficiencyPanel.howTheCarIsCharged') }}</h4>
      <div class="grid grid-cols-1 md:grid-cols-5 gap-4 items-start">
        <div v-if="acDc.classes.length > 1" class="md:col-span-2 bg-slate-950/60 border border-slate-800 rounded-xl p-3 flex flex-col items-center justify-center">
          <div class="w-full h-40 relative">
            <CostDonut
              :items="donutItems"
              unit="kWh"
              :empty-label="$t('dashboard.energyEfficiencyPanel.noCharge')"
              :chart-label="$t('dashboard.energyEfficiencyPanel.howTheCarIsCharged')"
            />
          </div>
        </div>

        <ul :class="acDc.classes.length > 1 ? 'md:col-span-3' : 'md:col-span-5'" class="space-y-2">
          <li v-for="c in acDc.classes" :key="c.class" class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
            <div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
              <span class="text-sm font-semibold text-white flex items-center gap-1.5">
                <span class="w-2.5 h-2.5 rounded-full shrink-0" :style="{ backgroundColor: CLASS_COLOR[c.class as 'AC' | 'DC'] }"></span>
                {{ classLabel(c.class).label }}
              </span>
              <span class="text-xs text-slate-300">{{ $t('dashboard.energyEfficiencyPanel.ofTheEnergyChargeS', { value: share(c), sessions: c.sessions }) }}</span>
            </div>
            <p class="mt-2 text-xs text-slate-400">
              {{ $t('dashboard.energyEfficiencyPanel.kwhKwhEfficiency', { value: fmt(c.kwh_added, 0), value2: fmtMoney(c.price_per_kwh, currency, 3), value3: fmtPercent(c.charge_efficiency) }) }}<template v-if="c.cost_per_full_charge !== undefined"> · 0 → 100 % : {{ formatAmount(c.cost_per_full_charge, currency) }}</template>
            </p>
          </li>
        </ul>
      </div>
      <p v-if="acDc.unknownSessions > 0" class="mt-2 text-xs text-slate-400">
        {{ $t('dashboard.energyEfficiencyPanel.unknownDurationSessions', { count: acDc.unknownSessions }) }}
      </p>
    </div>

    <template v-if="stats && !summary">
      <EnergyTemperatureSection v-if="basis?.temperature !== 'unavailable'" :stats="stats" />
      <EnergyBatterySection v-if="basis?.battery !== 'unavailable'" :stats="stats" :grafana-url="grafanaUrl" />
    </template>

    <router-link v-if="summary" to="/energy" class="tap-text inline-block text-xs font-semibold text-sky-400 underline">
      {{ $t('dashboard.energyEfficiencyPanel.seeEnergy') }}
    </router-link>
  </component>
  <p v-else-if="failed" role="alert" class="rounded-xl border border-slate-800 bg-slate-900 p-3 text-xs text-slate-400">
    {{ $t('dashboard.energyEfficiencyPanel.theEnergyEfficiencyStatisticsCould') }}
  </p>
</template>
