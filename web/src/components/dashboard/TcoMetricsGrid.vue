<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { canCharge, canRefuel, isFuelOnly } from '@/utils/vehicles'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { Coins, Zap, Receipt, TrendingUp } from 'lucide-vue-next'
import { distanceUnit, formatDistanceValue, formatPerDistanceValue, perDistance } from '@/units'

defineProps<{ tco: any | null }>()
const vehicleStore = useVehicleStore()
// Every TCO figure is in the vehicle's own currency
const money = (v: number, digits = 2) => formatAmount(v || 0, vehicleStore.currency, digits)
// A cost per km from the API, per the account's distance unit
// A headline amount drops its cents from 1 000 on: they are noise next to the digits that matter
const headline = (v: number) => money(v, Math.abs(v || 0) >= 1000 ? 0 : 2)
const perUnit = (v: number) => money(perDistance(v || 0), 3)
</script>

<template>
  <div class="grid grid-cols-2 lg:grid-cols-5 gap-3 sm:gap-4">
    <!-- Cost per km -->
    <div class="col-span-2 lg:col-span-2 bg-gradient-to-br from-slate-900 to-emerald-950/30 border border-emerald-500/20 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('dashboard.tcoMetricsGrid.fullCostPerKm', { unit: distanceUnit() }) }}</span>
        <div class="p-2 bg-success-500/10 text-success-400 rounded-xl">
          <TrendingUp class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-success-400">
        {{ perUnit(tco?.full_cost_per_km) }}<span class="text-base font-normal text-slate-400">/{{ distanceUnit() }}</span>
      </div>
      <div class="mt-2 space-y-0.5">
        <p class="text-xs text-slate-400">
          {{ $t('dashboard.tcoMetricsGrid.directRunningCostKm', { unit: distanceUnit(), usage_cost_per_km: perUnit(tco?.usage_cost_per_km) }) }}
        </p>
        <p class="text-xs text-slate-400">
          {{ $t('dashboard.tcoMetricsGrid.overKm', { unit: distanceUnit(), distance_basis_km: formatDistanceValue(tco?.distance_basis_km || 0) }) }}
          <template v-if="tco?.depreciation_cost_per_km"> {{ $t('dashboard.tcoMetricsGrid.depreciationKm', { unit: distanceUnit(), value: perUnit(tco.depreciation_cost_per_km) }) }}</template>
        </p>
      </div>
    </div>

    <!-- Total Cost -->
    <div class="col-span-2 lg:col-span-1 bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('dashboard.tcoMetricsGrid.totalCost') }}</span>
        <div class="p-2 bg-rose-500/10 text-rose-400 rounded-xl">
          <Coins class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-white">
        {{ headline(tco?.total_cost) }}
      </div>
      <div class="mt-2 space-y-0.5">
        <p class="text-xs text-slate-300">
          {{ $t('dashboard.tcoMetricsGrid.fullCost', { full_cost: money(tco?.full_cost) }) }}
          <span v-if="tco?.depreciation_cost" class="text-slate-400 text-xs"> {{ $t('dashboard.tcoMetricsGrid.includingDepreciation', { depreciation_cost: money(tco.depreciation_cost, 0) }) }}</span>
        </p>
        <p v-if="tco?.carpool_revenue" class="text-xs text-success-400">
          {{ $t('dashboard.tcoMetricsGrid.netOfCarpooling', { full_cost_net: money(tco.full_cost_net) }) }}
        </p>
      </div>
    </div>

    <!-- Energy Cost -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ isFuelOnly(tco?.powertrain) ? $t('dashboard.tcoMetricsGrid.fuel') : $t('dashboard.tcoMetricsGrid.energy') }}</span>
        <div class="p-2 bg-info-500/10 text-info-400 rounded-xl">
          <Zap class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-info-400">
        {{ headline(tco?.energy_cost) }}
      </div>
      <div class="mt-2 space-y-0.5">
        <p v-if="canRefuel(tco?.powertrain)" class="text-xs text-slate-400">
          {{ perUnit(tco?.energy_cost_per_km) }}/{{ distanceUnit() }} • {{ Math.round(tco?.total_liters || 0).toLocaleString(intlLocale()) }} L<template v-if="tco?.consumption_l_100km"> • {{ formatPerDistanceValue(tco.consumption_l_100km, 2) }} L/100 {{ distanceUnit() }}</template><template v-if="tco?.avg_cost_per_liter"> • {{ money(tco.avg_cost_per_liter, 3) }}/L</template>
        </p>
        <p v-if="canCharge(tco?.powertrain) && canRefuel(tco?.powertrain) && tco?.total_kwh_added" class="text-xs text-slate-400">
          {{ $t('dashboard.tcoMetricsGrid.kwhOnly', { total_kwh_added: Math.round(tco.total_kwh_added).toLocaleString(intlLocale()) }) }}
        </p>
        <p v-else-if="!canRefuel(tco?.powertrain)" class="text-xs text-slate-400">
          {{ $t('dashboard.tcoMetricsGrid.kmKwh', { unit: distanceUnit(), energy_cost_per_km: perUnit(tco?.energy_cost_per_km), total_kwh_added: Math.round(tco?.total_kwh_added || 0).toLocaleString(intlLocale()) }) }}
        </p>
        <p v-if="tco?.completeness?.charges_without_cost" class="text-xs text-warning-400">
          {{ $t('dashboard.tcoMetricsGrid.chargeSWithoutACost', { charges_without_cost: tco.completeness.charges_without_cost }) }}
        </p>
      </div>
    </div>

    <!-- Tolls & Parkings -->
    <div class="bg-gradient-to-br from-slate-900 to-slate-900/50 border border-slate-800 p-4 sm:p-5 rounded-2xl shadow-sm">
      <div class="flex items-center justify-between mb-3">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('dashboard.tcoMetricsGrid.tollsAndParking') }}</span>
        <div class="p-2 bg-warning-500/10 text-warning-400 rounded-xl">
          <Receipt class="w-5 h-5" />
        </div>
      </div>
      <div class="text-2xl sm:text-3xl font-extrabold text-warning-400">
        {{ headline(tco?.tolls_cost) }}
      </div>
      <div class="mt-2">
        <p class="text-xs text-slate-400">{{ perUnit(tco?.tolls_cost_per_km) }}/{{ distanceUnit() }}</p>
      </div>
    </div>
  </div>
</template>
