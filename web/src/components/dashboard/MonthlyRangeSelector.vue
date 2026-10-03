<script setup lang="ts">
import { monthlyRangeOptions } from '@/utils/dashboard'

// The period selector shared by the dashboard's monthly charts (energy efficiency, expenses, mileage and cost).
defineProps<{ label?: string }>()
const range = defineModel<import('@/utils/dashboard').MonthlyRangeKey>({ required: true })
</script>

<template>
  <div class="flex items-center gap-1 bg-slate-800/60 rounded-lg p-0.5" role="group" :aria-label="label">
    <button
      v-for="opt in monthlyRangeOptions"
      :key="opt.key"
      type="button"
      @click="range = opt.key"
      :aria-pressed="range === opt.key"
      :class="[
        'px-2.5 py-1 text-xs font-semibold rounded-md transition-colors',
        range === opt.key ? 'bg-indigo-500 text-white' : 'text-slate-400 hover:text-white',
      ]"
    >
      {{ $t(opt.labelKey) }}
    </button>
  </div>
</template>
