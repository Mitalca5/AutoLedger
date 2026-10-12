<script setup lang="ts">
import { formatPercent } from '@/utils/numbers'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { computed } from 'vue'
import { Tag } from 'lucide-vue-next'
import { distanceUnit, formatDistanceValue } from '@/units'
import { formatNumber } from '@/utils/numbers'
import { formatCostPerDistance } from '@/utils/costPerDistance'

// Split of the drives by tag: distance, energy and the running cost the drive list shows for the drives of the tag
const props = defineProps<{ tco: any | null }>()
const vehicleStore = useVehicleStore()
// A split with no tag at all would read "Untagged 100 %": nothing to show until a drive is tagged
const hasTags = computed(() => (props.tco?.tag_breakdown ?? []).some((item: any) => item.tag))
// A drive carrying several tags counts in each of them
const hasSharedDrives = computed(() => (props.tco?.tag_breakdown ?? []).some((item: any) => item.shared_drives > 0))
</script>

<template>
  <div v-if="hasTags" class="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
    <h3 class="text-sm font-bold text-white">{{ $t('dashboard.tagBreakdown.breakdownByDriveCategoryWork') }}</h3>
    <p class="mt-0.5 mb-3 text-xs text-slate-400">{{ $t('dashboard.tagBreakdown.costBasis') }}</p>
    <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <div
        v-for="item in tco.tag_breakdown"
        :key="item.tag"
        class="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex items-center justify-between gap-3"
      >
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <Tag class="w-4 h-4 shrink-0 text-slate-400" aria-hidden="true" />
            <span class="truncate text-sm font-bold text-white">{{ item.tag || $t('dashboard.tagBreakdown.untagged') }}</span>
          </div>
          <p class="text-xs text-slate-400 mt-1">{{ $t('dashboard.tagBreakdown.kmKwh', { unit: distanceUnit(), distance_km: formatDistanceValue(item.distance_km, 1), energy_kwh: formatNumber(item.energy_kwh, 1) }) }}</p>
          <p class="text-xs text-slate-300 mt-0.5">
            {{ item.cost > 0 ? formatAmount(item.cost, vehicleStore.currency) : '—' }}
            · {{ formatCostPerDistance(item.cost_per_km, vehicleStore.currency, 3, true) }}
          </p>
          <p v-if="item.tolls_amount" class="text-xs text-slate-400">{{ $t('dashboard.tagBreakdown.ofTollsAndParking', { value: formatAmount(item.tolls_amount, vehicleStore.currency) }) }}</p>
        </div>
        <div class="text-right shrink-0">
          <span class="text-lg font-extrabold text-white">{{ formatPercent(item.percentage, 0) }}</span>
        </div>
      </div>
    </div>
    <p v-if="hasSharedDrives" class="mt-3 text-xs text-slate-400">{{ $t('dashboard.tagBreakdown.sharedNote') }}</p>
  </div>
</template>
