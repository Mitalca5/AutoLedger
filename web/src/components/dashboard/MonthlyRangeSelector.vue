<script setup lang="ts">
import { computed } from 'vue'
import { monthlyRangeOptions } from '@/utils/dashboard'

// The period selector shared by the dashboard's monthly charts (energy efficiency, expenses, mileage and cost).
// `custom` is set while a range picked elsewhere on the page applies: no preset is highlighted then.
// `ascending` lists the shortest period first.
// `pick` fires on every click, even on the preset that is already selected.
const props = defineProps<{ label?: string; custom?: boolean; ascending?: boolean }>()
const emit = defineEmits<{ pick: [key: import('@/utils/dashboard').MonthlyRangeKey] }>()
const options = computed(() => (props.ascending ? [...monthlyRangeOptions].reverse() : [...monthlyRangeOptions]))
const range = defineModel<import('@/utils/dashboard').MonthlyRangeKey>({ required: true })
</script>

<template>
  <div class="flex items-center gap-1 bg-slate-800/60 rounded-lg p-0.5" role="group" :aria-label="label">
    <button
      v-for="opt in options"
      :key="opt.key"
      type="button"
      @click="range = opt.key; emit('pick', opt.key)"
      :aria-pressed="!custom && range === opt.key"
      :class="[
        'tap-text px-2.5 text-xs font-semibold rounded-md transition-colors',
        !custom && range === opt.key ? 'bg-rose-600 text-white' : 'text-slate-400 hover:text-white',
      ]"
    >
      {{ $t(opt.labelKey) }}
    </button>
  </div>
</template>
