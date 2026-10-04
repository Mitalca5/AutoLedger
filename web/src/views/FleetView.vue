<script setup lang="ts">
import PageHeader from '@/components/PageHeader.vue'
import { Gauge as PageIcon } from 'lucide-vue-next'
import { ref, onMounted, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { Car, Zap, Gauge, Users, RefreshCw, Award, TrendingDown, ArrowUpRight } from 'lucide-vue-next'
import { Chart, registerables } from 'chart.js'
import { api, type FleetSummaryResponse } from '@/services/api'
import { formatAmount } from '@/currency'
import { budgetUsage, parseBudgetInput } from '@/utils/fleetBudget'
import { useConfirm } from '@/composables/useConfirm'
import { formatDistance, currentDistanceUnit, perDistance, formatPerDistanceValue } from '@/units'
import { t } from '@/i18n'

Chart.register(...registerables)

const loading = ref(false)
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
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    savingBudget.value = false
  }
}

function submitBudget() {
  const amount = parseBudgetInput(budgetInput.value)
  if (amount === null) {
    showAlert(t('fleet.budget.invalid'), t('shell.confirm.error'), 'danger')
    return
  }
  void saveBudget(amount)
}

async function loadFleetSummary() {
  loading.value = true
  try {
    const data = await api.getFleetSummary()
    summary.value = data
    await nextTick()
    renderChart()
  } catch (err) {
    console.error('Failed to load fleet summary', err)
  } finally {
    loading.value = false
  }
}

// Find the most economical vehicle by energy cost / 100km
const mostEconomicalVehicleId = computed(() => {
  if (!summary.value?.vehicles.length) return null
  const withCost = summary.value.vehicles.filter((v) => v.energy_cost_per_100km > 0)
  if (!withCost.length) return null
  return withCost.reduce((prev, curr) => (curr.energy_cost_per_100km < prev.energy_cost_per_100km ? curr : prev)).vehicle_id
})

// Ranking by full cost per km, only among vehicles whose figures are complete enough to be compared
const rankedVehicles = computed(() =>
  [...(summary.value?.vehicles ?? [])].sort((a, b) => {
    if (a.comparable !== b.comparable) return a.comparable ? -1 : 1
    return a.full_cost_per_km - b.full_cost_per_km
  }),
)
const cheapestPerKmId = computed(() => {
  const first = rankedVehicles.value[0]
  return first?.comparable && rankedVehicles.value.filter((v) => v.comparable).length > 1 ? first.vehicle_id : null
})

function completenessClass(pct: number): string {
  if (pct >= 80) return 'text-success-400'
  if (pct >= 60) return 'text-warning-400'
  return 'text-rose-400'
}

function energyPer100(v: { kwh_per_100km: number | null; liters_per_100km: number | null }): string {
  const parts: string[] = []
  if (v.kwh_per_100km != null) parts.push(t('fleet.compare.kwh100', { value: formatPerDistanceValue(v.kwh_per_100km), unit: distanceUnitLabel.value }))
  if (v.liters_per_100km != null) parts.push(t('fleet.compare.l100', { value: formatPerDistanceValue(v.liters_per_100km), unit: distanceUnitLabel.value }))
  return parts.length ? parts.join(' + ') : '—'
}

function renderChart() {
  if (!chartCanvas.value || !summary.value?.monthly_costs.length) return

  if (chartInstance) {
    chartInstance.destroy()
    chartInstance = null
  }

  const months = summary.value.monthly_costs.map((m) => m.month)
  const energyCosts = summary.value.monthly_costs.map((m) => m.energy_cost)
  const otherCosts = summary.value.monthly_costs.map((m) => m.other_cost)

  chartInstance = new Chart(chartCanvas.value, {
    type: 'bar',
    data: {
      labels: months,
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
              const curr = summary.value?.currency || 'EUR'
              return `${context.dataset.label}: ${formatAmount(val, curr)}`
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
              return formatAmount(Number(value), summary.value?.currency || 'EUR', 0)
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
        class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-sm font-medium transition-colors border border-slate-700/60 shadow-sm"
      >
        <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        {{ t('fleet.refresh') }}
      </button>
    
      </template>
    </PageHeader>

    <!-- Monthly budget -->
    <div v-if="summary" class="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.budget.title') }}</span>
        <div v-if="!editingBudget" class="flex items-center gap-3 text-xs font-semibold">
          <button type="button" @click="startEditBudget" class="text-rose-400 hover:text-rose-300">
            {{ summary.monthly_budget ? t('fleet.budget.edit') : t('fleet.budget.set') }}
          </button>
          <button v-if="summary.monthly_budget" type="button" :disabled="savingBudget" @click="saveBudget(null)" class="text-slate-400 hover:text-white">
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
          class="min-w-0 flex-1 bg-slate-800 text-slate-100 rounded-xl px-3 py-2 text-sm border border-slate-700"
        />
        <button type="submit" :disabled="savingBudget" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl disabled:opacity-50">{{ t('common.save') }}</button>
        <button type="button" @click="editingBudget = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl">{{ t('common.cancel') }}</button>
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
          <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.kpi.monthCost') }}</span>
          <div class="p-2 bg-success-500/10 text-success-400 rounded-lg">
            <TrendingDown class="w-4 h-4" />
          </div>
        </div>
        <p class="text-2xl font-bold text-white mt-2">
          {{ formatAmount(summary.current_month_cost, summary.currency) }}
        </p>
        <p class="text-xs text-slate-400 mt-1">{{ t('fleet.kpi.currentMonthTotal') }}</p>
      </div>

      <div class="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.kpi.monthDistance') }}</span>
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
          <span class="text-xs font-medium text-slate-400 uppercase tracking-wider">{{ t('fleet.kpi.monthEnergy') }}</span>
          <div class="p-2 bg-warning-500/10 text-warning-400 rounded-lg">
            <Zap class="w-4 h-4" />
          </div>
        </div>
        <p class="text-2xl font-bold text-white mt-2">
          {{ summary.current_month_energy_kwh.toFixed(1) }} <span class="text-sm font-normal text-slate-400">kWh</span>
        </p>
        <p class="text-xs text-slate-400 mt-1">{{ t('fleet.kpi.energyDelivered') }}</p>
      </div>
    </div>

    <!-- Vehicle Comparison Grid -->
    <div v-if="summary" class="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
      <div class="flex items-center justify-between border-b border-slate-800/80 pb-3">
        <h2 class="text-base font-semibold text-white flex items-center gap-2">
          <Car class="w-4 h-4 text-rose-400" />
          {{ t('fleet.vehicles.comparisonTitle') }}
        </h2>
        <span class="text-xs text-slate-400">{{ t('fleet.vehicles.efficiencyMetricHint') }}</span>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div
          v-for="v in summary.vehicles"
          :key="v.vehicle_id"
          class="relative p-4 rounded-xl bg-slate-950/60 border transition-all"
          :class="[
            v.vehicle_id === mostEconomicalVehicleId
              ? 'border-success-500/40 bg-success-950/10 shadow-sm shadow-success-500/10'
              : 'border-slate-800/80 hover:border-slate-700'
          ]"
        >
          <!-- Economical Badge -->
          <div
            v-if="v.vehicle_id === mostEconomicalVehicleId && v.energy_cost_per_100km > 0"
            class="absolute top-3 right-3 flex items-center gap-1 px-2 py-0.5 rounded-full bg-success-500/20 text-success-400 border border-success-500/30 text-xs font-semibold tracking-wide uppercase"
          >
            <Award class="w-3 h-3" />
            {{ t('fleet.vehicles.mostEconomical') }}
          </div>

          <div class="pr-20">
            <h3 class="text-sm font-bold text-white tracking-tight truncate">{{ v.name }}</h3>
            <p class="text-xs text-slate-400 mt-0.5 truncate">{{ v.make }} {{ v.model }}</p>
          </div>

          <div class="grid grid-cols-2 gap-3 mt-4 pt-3 border-t border-slate-800/60">
            <div>
              <span class="text-xs text-slate-400 block">{{ t('fleet.vehicles.energyCostPer100') }}</span>
              <span class="text-sm font-bold block mt-0.5" :class="v.energy_cost_per_100km > 0 ? 'text-success-400' : 'text-slate-400'">
                <template v-if="v.energy_cost_per_100km > 0">
                  {{ formatAmount(perDistance(v.energy_cost_per_100km), v.currency) }} / 100 {{ distanceUnitLabel }}
                </template>
                <template v-else>
                  —
                </template>
              </span>
            </div>

            <div>
              <span class="text-xs text-slate-400 block">{{ t('fleet.vehicles.currentMonthCost') }}</span>
              <span class="text-sm font-semibold text-white block mt-0.5">
                {{ formatAmount(v.month_cost, v.currency) }}
              </span>
            </div>

            <div>
              <span class="text-xs text-slate-400 block">{{ t('fleet.vehicles.currentMonthDistance') }}</span>
              <span class="text-xs font-medium text-slate-300 block mt-0.5">
                {{ formatDistance(v.month_distance_km) }}
              </span>
            </div>

            <div>
              <span class="text-xs text-slate-400 block">{{ t('fleet.vehicles.odometer') }}</span>
              <span class="text-xs font-medium text-slate-300 block mt-0.5">
                {{ formatDistance(v.current_odometer) }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Cost per distance comparison -->
    <div v-if="summary && summary.vehicles.length > 1" class="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
      <div class="flex items-center justify-between border-b border-slate-800/80 pb-3">
        <h2 class="text-base font-semibold text-white flex items-center gap-2">
          <TrendingDown class="w-4 h-4 text-rose-400" />
          {{ t('fleet.compare.title') }}
        </h2>
        <span class="text-xs text-slate-400">{{ t('fleet.compare.hint') }}</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-xs uppercase tracking-wider text-slate-400">
              <th class="py-2 pr-4 font-medium">{{ t('fleet.compare.vehicle') }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.runningPerDistance', { unit: distanceUnitLabel }) }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.fullPerDistance', { unit: distanceUnitLabel }) }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.energy') }}</th>
              <th class="py-2 pr-4 font-medium text-right">{{ t('fleet.compare.annual') }}</th>
              <th class="py-2 font-medium text-right">{{ t('fleet.compare.completeness') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in rankedVehicles" :key="v.vehicle_id" class="border-t border-slate-800/60" :class="{ 'opacity-70': !v.comparable }">
              <td class="py-2 pr-4">
                <span class="font-semibold text-white">{{ v.name }}</span>
                <span v-if="v.vehicle_id === cheapestPerKmId" class="ml-2 inline-flex items-center gap-1 text-xs font-semibold uppercase text-success-400">
                  <Award class="w-3 h-3" />{{ t('fleet.compare.cheapest') }}
                </span>
                <span v-else-if="!v.comparable" class="ml-2 text-xs uppercase text-slate-400">{{ t('fleet.compare.indicative') }}</span>
              </td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ v.running_cost_per_km > 0 ? formatAmount(perDistance(v.running_cost_per_km), v.currency) : '—' }}</td>
              <td class="py-2 pr-4 text-right tabular-nums font-semibold text-white">{{ v.full_cost_per_km > 0 ? formatAmount(perDistance(v.full_cost_per_km), v.currency) : '—' }}</td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ energyPer100(v) }}</td>
              <td class="py-2 pr-4 text-right tabular-nums">{{ v.annual_cost != null ? formatAmount(v.annual_cost, v.currency) : '—' }}</td>
              <td class="py-2 text-right tabular-nums font-semibold" :class="completenessClass(v.completeness_pct)">{{ v.completeness_pct }}%</td>
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
          <span class="text-xs text-slate-400">{{ t('fleet.chart.stackedBreakdown') }}</span>
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
            :key="member.driver_id || 'unassigned'"
            class="space-y-1.5"
          >
            <div class="flex items-center justify-between text-xs">
              <span class="font-medium text-slate-200 truncate">{{ member.driver_name }}</span>
              <span class="text-slate-400 shrink-0 font-mono">
                {{ formatDistance(member.distance_km) }} ({{ member.percentage.toFixed(1) }}%)
              </span>
            </div>
            <div class="h-2 w-full bg-slate-800 rounded-full overflow-hidden">
              <div
                class="h-full bg-gradient-to-r from-blue-500 to-indigo-500 rounded-full transition-all duration-500"
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
