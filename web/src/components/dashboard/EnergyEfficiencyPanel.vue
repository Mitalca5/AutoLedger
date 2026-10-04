<script setup lang="ts">
import { t } from '@/i18n'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { Chart, registerables } from 'chart.js'
import { Gauge } from 'lucide-vue-next'
import { api } from '@/services/api'
import EnergyBatterySection from './EnergyBatterySection.vue'
import EnergyTemperatureSection from './EnergyTemperatureSection.vue'
import CostDonut from '@/components/costs/CostDonut.vue'
import { AXIS_TEXT, GRID_COLOR, fmt, fmtMoney, fmtPercent, perUnit, mergeAcDcClasses, type ChargeClass, type EnergyStats } from './energyStats'
import { filterMonthsByRange, type MonthlyRangeKey } from '@/utils/dashboard'
import MonthlyRangeSelector from './MonthlyRangeSelector.vue'
import { currencySymbol, formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { distanceUnit } from '@/units'

Chart.register(...registerables)

const props = defineProps<{
  vehicleId: string
  grafanaUrl?: string | null
  // Bumped after a synchronization so the figures follow the imported drives and charges
  syncKey?: number
}>()

const vehicleStore = useVehicleStore()
const currency = computed(() => vehicleStore.currency)

const stats = ref<EnergyStats | null>(null)
const failed = ref(false)
const range = ref<MonthlyRangeKey>('1Y')

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

const basis = computed(() => stats.value?.basis)
const showConsumption = computed(() => basis.value?.consumption !== 'unavailable')
const showCost = computed(() => basis.value?.cost !== 'unavailable')
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
      y: { grid: { color: GRID_COLOR }, ticks: { color: AXIS_TEXT, callback: (v: any) => `${v}` }, title: { display: true, text: unit, color: AXIS_TEXT } },
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
  const labels = months.map((m) => m.month)

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
            backgroundColor: 'rgba(99, 102, 241, 0.35)',
            borderRadius: 4,
          },
          {
            type: 'line',
            label: t('dashboard.energyEfficiencyPanel.threeMonthAverage'),
            data: months.map((m) => perUnit(m.cost_per_100km_trailing) ?? null),
            borderColor: '#f59e0b',
            backgroundColor: '#f59e0b',
            tension: 0.25,
            spanGaps: true,
            pointRadius: 3,
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
  <section v-if="hasData" class="space-y-4 rounded-2xl border border-slate-800 bg-slate-900 p-5 shadow-sm" aria-labelledby="energy-efficiency-title">
    <div class="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
      <div>
        <h3 id="energy-efficiency-title" class="flex items-center gap-2 text-sm font-bold text-white">
          <Gauge class="h-4 w-4 text-info-400" aria-hidden="true" />
          {{ $t('dashboard.energyEfficiencyPanel.energyEfficiency') }}
        </h3>
        <p class="mt-0.5 text-xs text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.whatTheCarUsesAnd') }}</p>
      </div>
      <MonthlyRangeSelector v-model="range" class="self-start" :label="$t('dashboard.energyEfficiencyPanel.period')" />
    </div>

    <dl class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-xs font-semibold uppercase tracking-wide text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.actualConsumption') }}</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ fmt(perUnit(stats?.summary.consumption_kwh_100km), 1) }} <span class="text-xs font-medium text-slate-400">kWh/100 {{ distanceUnit() }}</span></dd>
        <p class="mt-0.5 text-xs text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.energyUsedWhileDrivingMeasured') }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-xs font-semibold uppercase tracking-wide text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.energyCost') }}</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ formatAmount(perUnit(stats?.summary.cost_per_100km) || 0, currency) }} <span class="text-xs font-medium text-slate-400">/100 {{ distanceUnit() }}</span></dd>
        <p class="mt-0.5 text-xs text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.thatIsKwhOnAverage', { price_per_kwh: fmtMoney(stats?.summary.price_per_kwh, currency, 3) }) }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-xs font-semibold uppercase tracking-wide text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.chargingEfficiency') }}</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ fmtPercent(stats?.summary.charge_efficiency) }}</dd>
        <p class="mt-0.5 text-xs text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.energyStoredInTheBattery') }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-950/60 p-3">
        <dt class="text-xs font-semibold uppercase tracking-wide text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.fullCharge') }}</dt>
        <dd class="mt-1 text-xl font-bold text-white">{{ formatAmount(stats?.summary.cost_per_full_charge || 0, currency) }} <span class="text-xs font-medium text-slate-400">(0 → 100 %)</span></dd>
        <p class="mt-0.5 text-xs text-slate-400">{{ $t('dashboard.energyEfficiencyPanel.extrapolatedFromTheChargesWhose') }}</p>
      </div>
    </dl>

    <p v-if="(stats?.summary.sessions_without_cost ?? 0) > 0" class="rounded-lg border border-warning-500/20 bg-warning-500/10 px-3 py-2 text-xs text-warning-300">
      {{ $t('dashboard.energyEfficiencyPanel.chargeSWithoutAKnown', { unit: distanceUnit(), sessions_without_cost: stats?.summary.sessions_without_cost }) }}
      <router-link to="/expenses?tab=CHARGES" class="font-semibold underline">{{ $t('dashboard.energyEfficiencyPanel.completeThem') }}</router-link>
    </p>

    <p v-if="hasDerived" class="rounded-lg border border-info-500/20 bg-info-500/10 px-3 py-2 text-xs text-info-300">
      {{ $t('dashboard.energyEfficiencyPanel.derivedNote') }}
    </p>

    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div v-if="showConsumption">
        <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.energyEfficiencyPanel.monthlyConsumption') }}</h4>
        <div class="h-56"><canvas ref="consumptionRef" role="img" :aria-label="$t('dashboard.energyEfficiencyPanel.monthlyConsumptionInKwhPer', { unit: distanceUnit() })"></canvas></div>
      </div>
      <div v-if="showCost">
        <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.energyEfficiencyPanel.energyCostPer100Km', { unit: distanceUnit() }) }}</h4>
        <div class="h-56"><canvas ref="costRef" role="img" :aria-label="$t('dashboard.energyEfficiencyPanel.monthlyEnergyCostPer100', { unit: distanceUnit() })"></canvas></div>
      </div>
    </div>

    <!-- A table is not clipped by sr-only (its width ignores the 1px), so the class goes on a wrapper -->
    <div class="sr-only">
      <table>
        <caption>{{ $t('dashboard.energyEfficiencyPanel.consumptionAndEnergyCostPer') }}</caption>
        <thead>
          <tr><th>{{ $t('dashboard.energyEfficiencyPanel.month') }}</th><th>kWh/100 {{ distanceUnit() }}</th><th>{{ currencySymbol(currency) }}/100 {{ distanceUnit() }}</th><th>{{ $t('dashboard.energyEfficiencyPanel.3MonthAverage100Km', { unit: distanceUnit(), cur: currencySymbol(currency) }) }}</th></tr>
        </thead>
        <tbody>
          <tr v-for="m in visibleMonths" :key="m.month">
            <td>{{ m.month }}</td>
            <td>{{ fmt(perUnit(m.consumption_kwh_100km), 1) }}</td>
            <td>{{ fmt(perUnit(m.cost_per_100km), 2) }}</td>
            <td>{{ fmt(perUnit(m.cost_per_100km_trailing), 2) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="acDc.classes.length > 0">
      <h4 class="mb-2 text-xs font-bold text-slate-200">{{ $t('dashboard.energyEfficiencyPanel.howTheCarIsCharged') }}</h4>
      <div class="grid grid-cols-1 md:grid-cols-5 gap-4 items-start">
        <div class="md:col-span-2 bg-slate-950/60 border border-slate-800 rounded-xl p-3 flex flex-col items-center justify-center">
          <div class="w-full h-40 relative">
            <CostDonut
              :items="donutItems"
              unit="kWh"
              :empty-label="$t('dashboard.energyEfficiencyPanel.noCharge')"
              :chart-label="$t('dashboard.energyEfficiencyPanel.howTheCarIsCharged')"
            />
          </div>
        </div>

        <ul class="md:col-span-3 space-y-2">
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

    <template v-if="stats">
      <EnergyTemperatureSection v-if="basis?.temperature !== 'unavailable'" :stats="stats" />
      <EnergyBatterySection v-if="basis?.battery !== 'unavailable'" :stats="stats" :grafana-url="grafanaUrl" />
    </template>
  </section>
  <p v-else-if="failed" role="alert" class="rounded-xl border border-slate-800 bg-slate-900 p-3 text-xs text-slate-400">
    {{ $t('dashboard.energyEfficiencyPanel.theEnergyEfficiencyStatisticsCould') }}
  </p>
</template>
