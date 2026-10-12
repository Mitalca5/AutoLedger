<script setup lang="ts">
import GlossaryTip from '@/components/GlossaryTip.vue'
import LoadError from '@/components/LoadError.vue'
import ListSkeleton from '@/components/ListSkeleton.vue'
import { formatNumber, formatPercent } from '@/utils/numbers'
import PageHeader from '@/components/PageHeader.vue'
import { Gauge as PageIcon } from 'lucide-vue-next'
import { ref, onMounted, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { Car, Zap, Gauge, Users, RefreshCw, Award, TrendingDown } from 'lucide-vue-next'
import { Chart, registerables } from 'chart.js'
import { api, type FleetSummaryResponse } from '@/services/api'
import { formatAmount, formatAmountOrUnknown } from '@/currency'
import { budgetUsage, parseBudgetInput } from '@/utils/fleetBudget'
import { cheapestPerKmIds, rankFleetVehicles } from '@/utils/fleetRanking'
import { useConfirm } from '@/composables/useConfirm'
import { formatDistance, currentDistanceUnit, perDistance, formatPerDistanceValue, formatFuelConsumptionValue, fuelConsumptionUnit } from '@/units'
import { t } from '@/i18n'
import { formatMonthLabel } from '@/utils/dates'
import { formatMonthName } from '@/utils/dashboard'

Chart.register(...registerables)

const loading = ref(false)
const loadError = ref<string | null>(null)
const summary = ref<FleetSummaryResponse | null>(null)
const chartCanvas = ref<HTMLCanvasElement | null>(null)
let chartInstance: Chart | null = null

const distanceUnitLabel = computed(() => currentDistanceUnit())
const { showAlert } = useConfirm()

const editingBudget = ref(false)
const budgetInput = ref('')
const savingBudget = ref(false)
const usage = computed(() => (summary.value?.monthly_budget ? budgetUsage(summary.value.current_month_cost, summary.value.monthly_budget) : null))

function startEditBudget() {
  budgetInput.value = summary.value?.monthly_budget ? String(summary.value.monthly_budget) : ''
  editingBudget.value = true
}

async function saveBudget(amount: number | null) {
  if (!summary.value) return
  savingBudget.value = true
  try {
    const res = await api.setFleetBudget(amount)
    summary.value.monthly_budget = res.monthly_budget
    editingBudget.value = false
  } catch (err: any) {
    void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    savingBudget.value = false
  }
}

function submitBudget() {
  const amount = parseBudgetInput(budgetInput.value)
  if (amount === null) {
    void showAlert(t('fleet.budget.invalid'), t('shell.confirm.error'), 'danger')
    return
  }
  void saveBudget(amount)
}

async function loadFleetSummary() {
  loading.value = true
  loadError.value = null
  try {
    const data = await api.getFleetSummary()
    summary.value = data
    await nextTick()
    renderChart()
  } catch (err) {
    console.error('Failed to load fleet summary', err)
    loadError.value = (err as Error)?.message ?? ''
  } finally {
    loading.value = false
  }
}

// Vehicles in a currency other than the fleet one are not part of its totals, budget or chart
const otherCurrencyCount = computed(() => (summary.value?.totals ?? []).filter((x) => x.currency !== summary.value?.currency).reduce((n, x) => n + x.vehicle_count, 0))
const otherTotals = computed(() => (summary.value?.totals ?? []).filter((x) => x.currency !== summary.value?.currency))

// Ranking by full cost per km inside each currency, only among vehicles whose figures are complete enough to be compared
const rankedVehicles = computed(() => rankFleetVehicles(summary.value?.vehicles ?? [], summary.value?.currency ?? ''))
const cheapestPerKmIdSet = computed(() => cheapestPerKmIds(rankedVehicles.value))

function completenessClass(pct: number): string {
  if (pct >= 80) return 'text-success-400'
  if (pct >= 60) return 'text-warning-400'
  return 'text-rose-400'
}

// The energy cost of 100 distance units: a known zero (free charging) is shown, an unknown figure is a dash
function energyCost100(v: { energy_cost_per_100km: number | null; currency: string }): string {
  return v.energy_cost_per_100km == null ? '—' : formatAmount(perDistance(v.energy_cost_per_100km), v.currency)
}

function energyPer100(v: { kwh_per_100km: number | null; liters_per_100km: number | null }): string {
  const parts: string[] = []
  if (v.kwh_per_100km != null) parts.push(t('fleet.compare.kwh100', { value: formatPerDistanceValue(v.kwh_per_100km), unit: distanceUnitLabel.value }))
  if (v.liters_per_100km != null) parts.push(t('fleet.compare.l100', { value: formatFuelConsumptionValue(v.liters_per_100km), unit: fuelConsumptionUnit() }))
  return parts.length ? parts.join(' + ') : '—'
}

function renderChart() {
  if (!chartCanvas.value || !summary.value?.monthly_costs.length) return

  if (chartInstance) {
    chartInstance.destroy()
    chartInstance = null
  }

  const currency = summary.value.currency
  const months = summary.value.monthly_costs.map((m) => m.month)
  const energyCosts = summary.value.monthly_costs.map((m) => m.energy_cost)
  const otherCosts = summary.value.monthly_costs.map((m) => m.other_cost)

  chartInstance = new Chart(chartCanvas.value, {
    type: 'bar',
    data: {
      labels: months.map(formatMonthLabel),
      datasets: [
        {
          label: t('fleet.chart.energyCost'),
          data: energyCosts,
          backgroundColor: 'rgba(59, 130, 246, 0.75)',
          borderRadius: 4,
          stack: 'combined',
        },
        {
          label: t('fleet.chart.otherCost'),
          data: otherCosts,
          backgroundColor: 'rgba(245, 158, 11, 0.75)',
          borderRadius: 4,
          stack: 'combined',
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: {
        mode: 'index',
        intersect: false,
      },
      plugins: {
        legend: {
          labels: {
            color: '#94a3b8',
          },
        },
        tooltip: {
          callbacks: {
            label(context) {
              const val = context.raw as number
              return `${context.dataset.label}: ${formatAmount(val, currency)}`
            },
          },
        },
      },
      scales: {
        x: {
          stacked: true,
          grid: { color: 'rgba(51, 65, 85, 0.3)' },
          ticks: { color: '#94a3b8' },
        },
        y: {
          stacked: true,
          grid: { color: 'rgba(51, 65, 85, 0.3)' },
          ticks: {
            color: '#94a3b8',
            callback(value) {
              return formatAmount(Number(value), currency, 0)
            },
          },
        },
      },
    },
  })
}

onMounted(() => {
  loadFleetSummary()
})

onBeforeUnmount(() => {
  if (chartInstance) {
    chartInstance.destroy()
    chartInstance = null
  }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <PageHeader :title="t('fleet.title')" :icon="PageIcon">
      {{ t('fleet.subtitle') }}
      <template #actions>
      <button
        @click="loadFleetSummary"
        :disabled="loading"
        class="btn btn-secondary"
      >
        <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        {{ t('fleet.refresh') }}
      </button>
    
      </template>
    </PageHeader>

    <LoadError v-if="loadError !== null" :message="loadError" @retry="loadFleetSummary" />
    <ListSkeleton v-else-if="loading && !summary" :rows="3" />

    <!-- Monthly budget -->
    <div v-if="summary" class="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.budget.title') }}</span>
        <div v-if="!editingBudget" class="flex items-center gap-3 text-xs font-semibold">
          <button type="button" @click="startEditBudget" class="tap-text text-rose-400 hover:text-rose-300">
            {{ summary.monthly_budget ? t('fleet.budget.edit') : t('fleet.budget.set') }}
          </button>
          <button v-if="summary.monthly_budget" type="button" :disabled="savingBudget" @click="saveBudget(null)" class="tap-text text-slate-400 hover:text-white">
            {{ t('fleet.budget.remove') }}
          </button>
        </div>
      </div>
      <form v-if="editingBudget" class="flex flex-wrap items-center gap-2" @submit.prevent="submitBudget">
        <input
          v-model="budgetInput"
          type="text"
          inputmode="decimal"
          :aria-label="t('fleet.budget.title')"
          :placeholder="t('fleet.budget.placeholder', { currency: summary.currency })"
          class="field flex-1"
        />
        <button type="submit" :disabled="savingBudget" class="btn btn-lg btn-primary">{{ t('common.save') }}</button>
        <button type="button" @click="editingBudget = false" class="btn btn-lg btn-secondary">{{ t('common.cancel') }}</button>
      </form>
      <template v-if="usage && summary.monthly_budget && !editingBudget">
        <div class="h-2 rounded-full bg-slate-800 overflow-hidden" role="progressbar" :aria-valuenow="Math.round(usage.percent)" aria-valuemin="0" aria-valuemax="100">
          <div
            class="h-full rounded-full"
            :class="usage.status === 'over' ? 'bg-rose-500' : usage.status === 'warning' ? 'bg-warning-400' : 'bg-success-500'"
            :style="{ width: `${Math.min(100, usage.percent)}%` }"
          ></div>
        </div>
        <p class="text-xs text-slate-300 break-words">
          {{ t('fleet.budget.spent', { spent: formatAmount(summary.current_month_cost, summary.currency), budget: formatAmount(summary.monthly_budget, summary.currency), percent: Math.round(usage.percent) }) }}
          <span :class="usage.status === 'over' ? 'text-rose-400' : 'text-slate-400'">
            · {{ usage.status === 'over' ? t('fleet.budget.over', { amount: formatAmount(-usage.remaining, summary.currency) }) : t('fleet.budget.remaining', { amount: formatAmount(usage.remaining, summary.currency) }) }}
          </span>
        </p>
      </template>
      <p v-else-if="!editingBudget" class="text-xs text-slate-400">{{ t('fleet.budget.none') }}</p>
      <p v-if="otherCurrencyCount > 0" class="text-xs text-slate-400" data-testid="fleet-budget-other-currency">
        {{ t('fleet.budget.otherCurrency', { currency: summary.currency, count: otherCurrencyCount }, otherCurrencyCount) }}
      </p>
    </div>

    <!-- KPI Grid -->
    <div v-if="summary" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.kpi.vehicles') }}</span>
          <div class="p-2 bg-rose-500/10 text-rose-400 rounded-lg">
            <Car class="w-4 h-4" />
          </div>
        </div>
        <p class="text-2xl font-bold text-white mt-2">{{ summary.total_vehicles }}</p>
        <p class="text-xs text-slate-400 mt-1">{{ t('fleet.kpi.activeVehicles') }}</p>
      </div>

      <div class="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.kpi.monthCost', { month: formatMonthName(summary.current_month) }) }}</span>
          <div class="p-2 bg-success-500/10 text-success-400 rounded-lg">
            <TrendingDown class="w-4 h-4" />
          </div>
        </div>
        <p class="text-2xl font-bold text-white mt-2">
          {{ formatAmount(summary.current_month_cost, summary.currency) }}
        </p>
        <p v-for="tot in otherTotals" :key="tot.currency" class="text-lg font-bold text-white tabular-nums" data-testid="fleet-currency-total">
          {{ formatAmount(tot.current_month_cost, tot.currency) }}
        </p>
        <p class="text-xs text-slate-400 mt-1">{{ otherTotals.length ? t('fleet.kpi.perCurrency') : t('fleet.kpi.currentMonthTotal') }}</p>
      </div>

      <div class="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.kpi.monthDistance', { month: formatMonthName(summary.current_month) }) }}</span>
          <div class="p-2 bg-blue-500/10 text-blue-400 rounded-lg">
            <Gauge class="w-4 h-4" />
          </div>
        </div>
        <p class="text-2xl font-bold text-white mt-2">
          {{ formatDistance(summary.current_month_distance_km) }}
        </p>
        <p class="text-xs text-slate-400 mt-1">{{ t('fleet.kpi.fleetTravelled') }}</p>
      </div>

      <div class="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.kpi.monthEnergy', { month: formatMonthName(summary.current_month) }) }}</span>
          <div class="p-2 bg-warning-500/10 text-warning-400 rounded-lg">
            <Zap class="w-4 h-4" />
          </div>
        </div>
        <p class="text-2xl font-bold text-white mt-2">
          {{ formatNumber(summary.current_month_energy_kwh) }} <span class="text-sm font-normal text-slate-400">kWh</span>
        </p>
        <p class="text-xs text-slate-400 mt-1">{{ t('fleet.kpi.energyDelivered') }}</p>
      </div>
    </div>

    <!-- Cost per distance comparison -->
    <div v-if="summary && summary.vehicles.length > 0" class="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
      <div class="flex flex-col gap-1 border-b border-slate-800/80 pb-3 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
        <h2 class="text-base font-semibold text-white flex items-center gap-2">
          <TrendingDown class="w-4 h-4 text-rose-400" />
          {{ t('fleet.compare.title') }}
        </h2>
        <span class="text-xs text-slate-400">{{ t('fleet.compare.hint') }}</span>
      </div>
      <ul class="space-y-2 md:hidden">
        <li
          v-for="v in rankedVehicles"
          :key="v.vehicle_id"
          class="rounded-xl border border-slate-800 bg-slate-950/60 p-3"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="truncate font-semibold text-white">{{ v.name }}</p>
              <p v-if="cheapestPerKmIdSet.has(v.vehicle_id)" class="mt-0.5 inline-flex items-center gap-1 text-xs font-semibold uppercase text-success-400">
                <Award class="w-3 h-3" />{{ t('fleet.compare.cheapest') }}
              </p>
              <p v-else-if="!v.comparable" class="mt-0.5 text-xs uppercase text-slate-300">{{ v.depreciation_known === false ? t('fleet.compare.depreciationMissing') : t('fleet.compare.indicative') }}</p>
            </div>
            <div class="shrink-0 text-right">
              <p class="text-lg font-bold tabular-nums text-white">{{ formatAmountOrUnknown(v.full_cost_per_km == null ? null : perDistance(v.full_cost_per_km), v.currency) }}</p>
              <p class="text-xs text-slate-400">{{ t('fleet.compare.fullPerDistance', { unit: distanceUnitLabel }) }}</p>
            </div>
          </div>
          <dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1.5 border-t border-slate-800/60 pt-2 text-xs">
            <div>
              <dt class="text-slate-400">{{ t('fleet.compare.runningPerDistance', { unit: distanceUnitLabel }) }} <GlossaryTip term="cashPaid" /></dt>
              <dd class="tabular-nums text-slate-200">{{ formatAmountOrUnknown(v.running_cost_per_km == null ? null : perDistance(v.running_cost_per_km), v.currency) }}</dd>
            </div>
            <div>
              <dt class="text-slate-400">{{ t('fleet.compare.energyCost', { unit: distanceUnitLabel }) }}</dt>
              <dd class="tabular-nums text-slate-200">{{ energyCost100(v) }}</dd>
            </div>
            <div>
              <dt class="text-slate-400">{{ t('fleet.compare.energy') }} <GlossaryTip term="atThePlug" /></dt>
              <dd class="tabular-nums text-slate-200">{{ energyPer100(v) }}</dd>
            </div>
            <div>
              <dt class="text-slate-400">{{ t('fleet.compare.monthDistance') }}</dt>
              <dd class="tabular-nums text-slate-200">{{ formatDistance(v.month_distance_km) }}</dd>
            </div>
            <div>
              <dt class="text-slate-400">{{ t('fleet.compare.monthCost') }}</dt>
              <dd class="tabular-nums text-slate-200">{{ formatAmount(v.month_cost, v.currency) }}</dd>
            </div>
            <div>
              <dt class="text-slate-400">{{ t('fleet.compare.annual') }}</dt>
              <dd class="tabular-nums text-slate-200">{{ v.annual_cost != null ? formatAmount(v.annual_cost, v.currency) : '—' }}</dd>
            </div>
            <div>
              <dt class="text-slate-400">{{ t('fleet.compare.completeness') }}</dt>
              <dd class="font-semibold tabular-nums" :class="completenessClass(v.completeness_pct)">{{ formatPercent(v.completeness_pct, 0) }}</dd>
            </div>
          </dl>
        </li>
      </ul>
      <div class="hidden overflow-x-auto md:block">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-xs uppercase tracking-wider text-slate-400">
              <th class="py-2 pr-4 font-medium">{{ t('fleet.compare.vehicle') }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.runningPerDistance', { unit: distanceUnitLabel }) }} <GlossaryTip term="cashPaid" /></th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.fullPerDistance', { unit: distanceUnitLabel }) }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.energyCost', { unit: distanceUnitLabel }) }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.energy') }} <GlossaryTip term="atThePlug" /></th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.monthDistance') }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.monthCost') }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.annual') }}</th>
              <th class="py-2 font-medium text-right">{{ t('fleet.compare.completeness') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in rankedVehicles" :key="v.vehicle_id" class="border-t border-slate-800/60">
              <td class="py-2 pr-4">
                <span class="font-semibold text-white">{{ v.name }}</span>
                <span v-if="cheapestPerKmIdSet.has(v.vehicle_id)" class="ml-2 inline-flex items-center gap-1 text-xs font-semibold uppercase text-success-400">
                  <Award class="w-3 h-3" />{{ t('fleet.compare.cheapest') }}
                </span>
                <span v-else-if="!v.comparable" class="ml-2 text-xs uppercase text-slate-300">{{ v.depreciation_known === false ? t('fleet.compare.depreciationMissing') : t('fleet.compare.indicative') }}</span>
              </td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ formatAmountOrUnknown(v.running_cost_per_km == null ? null : perDistance(v.running_cost_per_km), v.currency) }}</td>
              <td class="py-2 pr-4 text-right tabular-nums font-semibold text-white">{{ formatAmountOrUnknown(v.full_cost_per_km == null ? null : perDistance(v.full_cost_per_km), v.currency) }}</td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ energyCost100(v) }}</td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ energyPer100(v) }}</td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ formatDistance(v.month_distance_km) }}</td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ formatAmount(v.month_cost, v.currency) }}</td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ v.annual_cost != null ? formatAmount(v.annual_cost, v.currency) : '—' }}</td>
              <td class="py-2 text-right tabular-nums font-semibold" :class="completenessClass(v.completeness_pct)">{{ formatPercent(v.completeness_pct, 0) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="text-xs text-slate-400">{{ t('fleet.compare.footnote') }}</p>
    </div>

    <!-- Charts & Driver Share Row -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6" v-if="summary">
      <!-- Monthly Cost Chart -->
      <div class="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
        <div class="flex items-center justify-between border-b border-slate-800/80 pb-3">
          <h2 class="text-base font-semibold text-white">
            {{ t('fleet.chart.monthlyCostHistory') }}
          </h2>
          <span class="text-xs text-slate-400">{{ otherCurrencyCount > 0 ? t('fleet.chart.fleetCurrencyOnly', { currency: summary.currency }) : t('fleet.chart.stackedBreakdown') }}</span>
        </div>
        <div class="h-64 relative">
          <canvas ref="chartCanvas"></canvas>
        </div>
      </div>

      <!-- Driver Distance Distribution -->
      <div class="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
        <div class="border-b border-slate-800/80 pb-3">
          <h2 class="text-base font-semibold text-white flex items-center gap-2">
            <Users class="w-4 h-4 text-blue-400" />
            {{ t('fleet.drivers.title') }}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{{ t('fleet.drivers.subtitle') }}</p>
        </div>

        <div v-if="summary.member_km_shares.length" class="space-y-3">
          <div
            v-for="member in summary.member_km_shares"
            :key="member.person_id"
            class="space-y-1.5"
          >
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-200 truncate">{{ member.display_name }}</span>
              <span class="text-slate-400 shrink-0 font-mono">
                {{ formatDistance(member.distance_km) }} ({{ formatPercent(member.percentage) }})
              </span>
            </div>
            <div class="h-2 w-full bg-slate-800 rounded-full overflow-hidden" role="progressbar" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(Math.min(100, Math.max(0, member.percentage)))" :aria-label="member.display_name">
              <div
                class="h-full bg-gradient-to-r from-blue-500 to-sky-500 rounded-full transition-all duration-500"
                :style="{ width: `${Math.min(100, Math.max(0, member.percentage))}%` }"
              ></div>
            </div>
          </div>
        </div>
        <div v-else class="text-center py-8 text-xs text-slate-400">
          {{ t('fleet.drivers.noData') }}
        </div>
      </div>
    </div>
  </div>
</template>
