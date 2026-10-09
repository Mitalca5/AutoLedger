<script setup lang="ts">
import { euros } from '@/utils/carpool'
import { formatAmount } from '@/currency'

// The totals of the carpool as the form stands: real cost, shares, what was received, what is left to the driver
const props = defineProps<{ live: any; liveRevenue: number; liveCoverage: number; liveNet: number; currency: string }>()
const fmt = (v: number) => formatAmount(Number(v || 0), props.currency)
</script>

<template>
  <div class="bg-slate-950/80 border border-slate-800 rounded-2xl p-4 grid grid-cols-2 sm:grid-cols-5 gap-3 text-xs">
    <div>
      <div class="text-slate-400">{{ $t('carpool.carpoolTripModal.actualCost') }}</div>
      <div class="text-sm font-bold text-white">{{ fmt(euros(live.total)) }}</div>
    </div>
    <div>
      <div class="text-slate-400">{{ $t('carpool.carpoolTripModal.passengersShares') }}</div>
      <div class="text-sm font-bold text-blue-400">{{ fmt(euros(live.passengersShare)) }}</div>
    </div>
    <div>
      <div class="text-slate-400">{{ $t('carpool.carpoolTripModal.driverSShare') }}</div>
      <div class="text-sm font-bold text-warning-400">{{ fmt(euros(live.driverShare)) }}</div>
    </div>
    <div>
      <div class="text-slate-400">{{ $t('carpool.carpoolTripModal.received') }}</div>
      <div class="text-sm font-bold text-success-400">{{ fmt(euros(liveRevenue)) }} ({{ liveCoverage }} %)</div>
    </div>
    <div>
      <div class="text-slate-400">{{ liveNet > 0 ? $t('carpool.carpoolTripModal.leftToDriver') : $t('carpool.carpoolTripModal.surplus') }}</div>
      <div class="text-sm font-bold" :class="liveNet > 0 ? 'text-white' : 'text-success-400'">{{ fmt(euros(Math.abs(liveNet))) }}</div>
    </div>
  </div>
</template>
