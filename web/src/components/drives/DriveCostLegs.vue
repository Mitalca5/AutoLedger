<script setup lang="ts">
import { ChevronRight, Layers } from 'lucide-vue-next'
import { formatDayTime } from '@/utils/dates'
import { formatDistance } from '@/units'
import { formatAmount } from '@/currency'

// The legs of a trip, as rows that open the cost breakdown of the leg
defineProps<{ tripLegs: any[]; vehicleCurrency: string }>()
const emit = defineEmits<{ open: [leg: any] }>()
const formatDate = formatDayTime
</script>

<template>
  <div class="space-y-1.5">
    <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
      <Layers class="w-3.5 h-3.5 text-indigo-400" />
      {{ $t('drives.driveCostModal.tripLegs', { count: tripLegs.length }) }}
    </h4>
    <button
      v-for="leg in tripLegs"
      :key="leg.id"
      type="button"
      @click="emit('open', leg)"
      class="w-full text-left flex items-center justify-between gap-3 bg-slate-800/40 hover:bg-slate-800/70 border border-slate-800 hover:border-slate-700 rounded-xl px-3 py-2 transition-colors"
    >
      <div class="min-w-0">
        <div class="text-xs text-slate-400">{{ formatDate(leg.start_time) }}</div>
        <div class="text-xs text-slate-200 truncate">
          {{ (leg.start_address || $t('drives.driveCostModal.start')).split(',')[0] }} → {{ (leg.end_address || $t('drives.driveCostModal.end')).split(',')[0] }}
        </div>
      </div>
      <div class="flex items-center gap-3 shrink-0">
        <span class="text-xs font-bold text-rose-400">{{ formatDistance(leg.distance_km) }}</span>
        <span class="text-xs font-mono font-bold text-white">{{ formatAmount(leg.costs?.total_cost || 0, vehicleCurrency) }}</span>
        <ChevronRight class="w-4 h-4 text-slate-400" />
      </div>
    </button>
  </div>
</template>
