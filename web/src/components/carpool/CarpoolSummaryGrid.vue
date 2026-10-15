<script setup lang="ts">
import { computed } from 'vue'
import KpiCard from '@/components/KpiCard.vue'
import { Users, TrendingUp, CreditCard, Receipt } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { formatPercentOrUnknown } from '@/utils/numbers'
import { distanceUnit, formatDistanceValue, perDistance } from '@/units'

// Totals of all the carpool trips of the vehicle
const props = defineProps<{ summary: any }>()

const vehicleStore = useVehicleStore()
// Carpool amounts are in the vehicle's own currency
const fmt = (v: number) => formatAmount(Number(v || 0), vehicleStore.currency)
// A vehicle with no cost yet has no recovery rate: a dash, not 0 %
const coverageRate = computed(() => (props.summary.coverage_rate_pct == null ? null : formatPercentOrUnknown(props.summary.coverage_rate_pct)))
// Revenue at least equal to the passengers' fair share
const covered = computed(() => props.summary.total_revenue >= props.summary.total_passengers_share)
</script>

<template>
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-3 sm:gap-4">
    <KpiCard
      :label="$t('carpool.carpoolSummaryGrid.carpooledTrips')"
      :icon="Users"
      :value="String(summary.total_trips)"
      :unit="` ${$t('carpool.carpoolSummaryGrid.trips', {}, summary.total_trips)}`"
    >
      <p class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.kmShared', { unit: distanceUnit(), total_distance_km: formatDistanceValue(summary.total_distance_km || 0) }) }}</p>
    </KpiCard>

    <KpiCard
      :label="$t('carpool.carpoolSummaryGrid.passengersCarried')"
      :icon="Users"
      :value="String(summary.total_passengers)"
      :unit="` ${$t('carpool.carpoolSummaryGrid.people', {}, summary.total_passengers)}`"
    >
      <p class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.fairShareDue', { total_passengers_share: fmt(summary.total_passengers_share) }) }}</p>
    </KpiCard>

    <KpiCard :label="$t('carpool.carpoolSummaryGrid.totalReceivedFromPassengers')" :icon="CreditCard" :value="fmt(summary.total_revenue)">
      <p class="mt-1 text-xs" :class="covered ? 'text-success-400' : 'text-warning-400'">
        {{ covered ? $t('carpool.carpoolSummaryGrid.covered') : $t('carpool.carpoolSummaryGrid.below', { amount: fmt(summary.total_passengers_share - summary.total_revenue) }) }}
      </p>
    </KpiCard>

    <KpiCard :label="$t('carpool.carpoolSummaryGrid.costRecoveryRate')" :icon="TrendingUp" :value="coverageRate">
      <p class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.ofTheTotalActualCost', { total_real_cost: fmt(summary.total_real_cost) }) }}</p>
    </KpiCard>

    <KpiCard
      :label="$t('carpool.carpoolSummaryGrid.driverSNetCost')"
      :icon="Receipt"
      :value="formatAmount(perDistance(Number(summary.net_cost_per_km || 0)), vehicleStore.currency, 3)"
      :unit="` / ${distanceUnit()}`"
    >
      <p class="mt-1 text-xs text-slate-400">{{ $t('carpool.carpoolSummaryGrid.driverSFairShare', { total_driver_share: fmt(summary.total_driver_share) }) }}</p>
    </KpiCard>
  </div>
</template>
