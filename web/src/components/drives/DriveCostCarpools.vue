<script setup lang="ts">
import { Users } from 'lucide-vue-next'
import { formatAmount } from '@/currency'

// The carpools shared on a trip; a click opens the carpools page
defineProps<{ tripCarpools: any[]; vehicleCurrency: string }>()
const emit = defineEmits<{ open: [] }>()
</script>

<template>
  <div class="space-y-1.5">
    <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
      <Users class="w-3.5 h-3.5 text-rose-400" />
      {{ $t('drives.driveCostModal.tripCarpools', { count: tripCarpools.length }) }}
    </h4>
    <button
      v-for="c in tripCarpools"
      :key="c.id"
      type="button"
      @click="emit('open')"
      class="w-full text-left flex items-center justify-between gap-3 bg-rose-500/5 hover:bg-rose-500/10 border border-rose-500/20 rounded-xl px-3 py-2 transition-colors"
    >
      <div class="min-w-0">
        <div class="text-xs font-semibold text-white truncate">{{ c.title }}</div>
        <div class="text-xs text-slate-400">
          {{ $t('drives.driveCostModal.carpoolLegsPassengers', { legs: c.legs?.length || 1, passengers: c.passengers?.length || 0 }) }}
        </div>
      </div>
      <div class="text-right shrink-0">
        <div class="text-xs font-mono font-bold text-success-400">+{{ formatAmount(Number(c.total_revenue || 0), vehicleCurrency) }}</div>
        <div class="text-xs text-slate-400">{{ $t('drives.driveCostModal.carpoolNetCost', { amount: formatAmount(Number(c.net_cost || 0), vehicleCurrency) }) }}</div>
      </div>
    </button>
  </div>
</template>
