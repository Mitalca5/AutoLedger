<script setup lang="ts">
import { computed } from 'vue'
import { distanceUnit, formatDistanceValue } from '@/units'
import { formatPercent } from '@/utils/numbers'
import { tireWearComparison, tireWearComparisonTone } from '@/utils/tireWearComparison'

const props = defineProps<{ stat: any }>()
const comparison = computed(() => tireWearComparison(props.stat))
const deltaKey = computed(() => {
  const delta = comparison.value?.delta_pct ?? 0
  return delta === 0 ? 'same' : delta < 0 ? 'shorter' : 'longer'
})
const distance = (km: number) => `${formatDistanceValue(km)} ${distanceUnit()}`
</script>

<template>
  <div v-if="comparison" class="space-y-1 border-t border-slate-800 pt-2 text-xs" data-testid="tire-wear-comparison">
    <p class="font-semibold text-slate-300">{{ $t('tires.wearComparison.title') }}</p>
    <p class="text-slate-400">
      {{ $t('tires.wearComparison.lifespans', { measured: distance(comparison.measured_lifespan_km), expected: distance(comparison.expected_lifespan_km) }) }}
    </p>
    <p :class="tireWearComparisonTone(comparison, Number(stat.wear_rate_confidence))">
      {{ $t(`tires.wearComparison.${deltaKey}`, { pct: formatPercent(Math.abs(comparison.delta_pct)) }) }}
    </p>
    <p class="text-slate-400">{{ $t(`tires.wearComparison.${comparison.reference}`) }}</p>
  </div>
</template>
