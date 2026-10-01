<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick, onBeforeUnmount } from 'vue'
import { LayoutGrid, Car, Zap, Gauge, Users, RefreshCw, Award, TrendingDown, ArrowUpRight } from 'lucide-vue-next'
import { Chart, registerables } from 'chart.js'
import { api, type FleetSummaryResponse } from '@/services/api'
import { formatMoney, formatAmount } from '@/currency'
import { formatDistance, currentDistanceUnit, perDistance } from '@/units'
import { t } from '@/i18n'

Chart.register(...registerables)

const loading = ref(false)
const summary = ref<FleetSummaryResponse | null>(null)
const chartCanvas = ref<HTMLCanvasElement | null>(null)
let chartInstance: Chart | null = null

const distanceUnitLabel = computed(() => currentDistanceUnit())

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

function renderChart() {
  if (!chartCanvas.value || !summary.value?.monthly_costs.length) return

  if (chartInstance) {
    chartInstance.destroy()
    chartInstance = null
  }

  const months = summary.value.monthly_costs.map((m) => m.month)
  const energyCosts = summary.value.monthly_costs.map((m) => m.energy_cost / 100)
  const otherCosts = summary.value.monthly_costs.map((m) => m.other_cost / 100)

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
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div>
        <div class="flex items-center gap-2">
          <div class="p-2 bg-rose-500/10 text-rose-400 rounded-lg">
            <LayoutGrid class="w-6 h-6" />
          </div>
          <h1 class="text-2xl font-bold tracking-tight text-white">
            {{ t('fleet.title') }}
          </h1>
        </div>
        <p class="text-sm text-slate-400 mt-1">
          {{ t('fleet.subtitle') }}
        </p>
      </div>

      <button
        @click="loadFleetSummary"
        :disabled="loading"
        class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-sm font-medium transition-colors border border-slate-700/60 shadow-sm"
      >
        <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
        {{ t('fleet.refresh') }}
      </button>
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
          <div class="p-2 bg-emerald-500/10 text-emerald-400 rounded-lg">
            <TrendingDown class="w-4 h-4" />
          </div>
        </div>
        <p class="text-2xl font-bold text-white mt-2">
          {{ formatMoney(summary.current_month_cost, summary.currency) }}
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
          <div class="p-2 bg-amber-500/10 text-amber-400 rounded-lg">
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
              ? 'border-emerald-500/40 bg-emerald-950/10 shadow-sm shadow-emerald-500/10'
              : 'border-slate-800/80 hover:border-slate-700'
          ]"
        >
          <!-- Economical Badge -->
          <div
            v-if="v.vehicle_id === mostEconomicalVehicleId && v.energy_cost_per_100km > 0"
            class="absolute top-3 right-3 flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 text-[10px] font-semibold tracking-wide uppercase"
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
              <span class="text-[11px] text-slate-400 block">{{ t('fleet.vehicles.energyCostPer100') }}</span>
              <span class="text-sm font-bold block mt-0.5" :class="v.energy_cost_per_100km > 0 ? 'text-emerald-400' : 'text-slate-500'">
                <template v-if="v.energy_cost_per_100km > 0">
                  {{ formatAmount(perDistance(v.energy_cost_per_100km), v.currency) }} / 100 {{ distanceUnitLabel }}
                </template>
                <template v-else>
                  —
                </template>
              </span>
            </div>

            <div>
              <span class="text-[11px] text-slate-400 block">{{ t('fleet.vehicles.currentMonthCost') }}</span>
              <span class="text-sm font-semibold text-white block mt-0.5">
                {{ formatMoney(v.month_cost, v.currency) }}
              </span>
            </div>

            <div>
              <span class="text-[11px] text-slate-400 block">{{ t('fleet.vehicles.currentMonthDistance') }}</span>
              <span class="text-xs font-medium text-slate-300 block mt-0.5">
                {{ formatDistance(v.month_distance_km) }}
              </span>
            </div>

            <div>
              <span class="text-[11px] text-slate-400 block">{{ t('fleet.vehicles.odometer') }}</span>
              <span class="text-xs font-medium text-slate-300 block mt-0.5">
                {{ formatDistance(v.current_odometer) }}
              </span>
            </div>
          </div>
        </div>
      </div>
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
        <div v-else class="text-center py-8 text-xs text-slate-500">
          {{ t('fleet.drivers.noData') }}
        </div>
      </div>
    </div>
  </div>
</template>
