<script setup lang="ts">
import { formatPercent } from '@/utils/numbers'
import { intlLocale } from '@/i18n'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { Briefcase, User } from 'lucide-vue-next'
import { distanceUnit, formatDistanceValue } from '@/units'
import { formatNumber } from '@/utils/numbers'

// Pro / Perso split of the drives
defineProps<{ tco: any | null }>()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div v-if="tco?.tag_breakdown?.length" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
    <h3 class="text-sm font-bold text-white mb-3">{{ $t('dashboard.tagBreakdown.breakdownByDriveCategoryWork') }}</h3>
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <div
        v-for="item in tco.tag_breakdown"
        :key="item.tag"
        class="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex items-center justify-between"
      >
        <div>
          <div class="flex items-center gap-2">
            <Briefcase v-if="item.tag === 'Pro'" class="w-4 h-4 text-blue-400" />
            <User v-else class="w-4 h-4 text-success-400" />
            <span class="text-sm font-bold text-white">{{ item.tag || $t('dashboard.tagBreakdown.untagged') }}</span>
          </div>
          <p class="text-xs text-slate-400 mt-1">{{ $t('dashboard.tagBreakdown.kmKwh', { unit: distanceUnit(), distance_km: formatDistanceValue(item.distance_km, 1), energy_kwh: formatNumber(item.energy_kwh, 1) }) }}</p>
          <p v-if="item.tolls_amount" class="text-xs text-warning-400">{{ $t('dashboard.tagBreakdown.ofTollsAndParking', { value: formatAmount(item.tolls_amount, vehicleStore.currency) }) }}</p>
        </div>
        <div class="text-right">
          <span class="text-lg font-extrabold text-white">{{ formatPercent(item.percentage, 0) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
