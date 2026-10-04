<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { Users, TrendingUp, CreditCard, Receipt } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistanceValue, perDistance } from '@/units'

// Totals of all the carpool trips of the vehicle
defineProps<{ summary: any }>()

const vehicleStore = useVehicleStore()
// Carpool amounts are in the vehicle's own currency
const fmt = (v: number) => formatAmount(Number(v || 0), vehicleStore.currency)
</script>

<template>
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-3 sm:gap-4">
    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">{{ $t('carpool.carpoolSummaryGrid.carpooledTrips') }}</span>
        <div class="p-2 bg-slate-800 rounded-xl text-slate-300"><Users class="w-4 h-4" /></div>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-white">{{ summary.total_trips }}</span>
        <span class="text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.trips') }}</span>
      </div>
      <div class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.kmShared', { unit: distanceUnit(), total_distance_km: formatDistanceValue(summary.total_distance_km || 0) }) }}</div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">{{ $t('carpool.carpoolSummaryGrid.passengersCarried') }}</span>
        <div class="p-2 bg-blue-500/10 rounded-xl text-blue-400"><Users class="w-4 h-4" /></div>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-blue-400">{{ summary.total_passengers }}</span>
        <span class="text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.people') }}</span>
      </div>
      <div class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.fairShareDue', { total_passengers_share: fmt(summary.total_passengers_share) }) }}</div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">{{ $t('carpool.carpoolSummaryGrid.totalReceivedFromPassengers') }}</span>
        <div class="p-2 bg-success-500/10 rounded-xl text-success-400"><CreditCard class="w-4 h-4" /></div>
      </div>
      <div class="mt-2"><span class="text-2xl font-bold text-success-400">{{ fmt(summary.total_revenue) }}</span></div>
      <div class="mt-1 text-xs" :class="summary.total_revenue >= summary.total_passengers_share ? 'text-success-500/80' : 'text-warning-400'">
        {{ summary.total_revenue >= summary.total_passengers_share ? $t('carpool.carpoolSummaryGrid.covered') : $t('carpool.carpoolSummaryGrid.below', { amount: fmt(summary.total_passengers_share - summary.total_revenue) }) }}
      </div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">{{ $t('carpool.carpoolSummaryGrid.costRecoveryRate') }}</span>
        <div class="p-2 bg-rose-500/10 rounded-xl text-rose-400"><TrendingUp class="w-4 h-4" /></div>
      </div>
      <div class="mt-2"><span class="text-2xl font-bold text-rose-400">{{ summary.coverage_rate_pct || 0 }} %</span></div>
      <div class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.ofTheTotalActualCost', { total_real_cost: fmt(summary.total_real_cost) }) }}</div>
    </div>

    <div class="bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-slate-400">{{ $t('carpool.carpoolSummaryGrid.driverSNetCost') }}</span>
        <div class="p-2 bg-warning-500/10 rounded-xl text-warning-400"><Receipt class="w-4 h-4" /></div>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-warning-400">{{ formatAmount(perDistance(Number(summary.net_cost_per_km || 0)), vehicleStore.currency, 3) }}</span>
        <span class="text-xs text-slate-400">/ {{ distanceUnit() }}</span>
      </div>
      <div class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.driverSFairShare', { total_driver_share: fmt(summary.total_driver_share) }) }}</div>
    </div>
  </div>
</template>
