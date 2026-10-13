<script setup lang="ts">
import { computed } from 'vue'
import { distanceUnit, formatDistanceValue, perDistance } from '@/units'
import { formatNumber, formatPercent } from '@/utils/numbers'
import { tireWearComparison, tireWearComparisonTone } from '@/utils/tireWearComparison'

const props = defineProps<{ stat: any }>()
const comparison = computed(() => tireWearComparison(props.stat))
const deltaKey = computed(() => comparison.value?.delta_pct === 0 ? 'same' : (comparison.value?.delta_pct || 0) > 0 ? 'faster' : 'slower')
// The API stores mm per 10,000 km; convert the denominator for accounts displaying miles.
const rate = (value: number) => formatNumber(perDistance(value), 2)
</script>

<template>
  <div v-if="comparison" class="border-t border-slate-800 pt-2 space-y-2 text-xs" data-testid="tire-wear-comparison">
    <p class="font-semibold text-slate-300">{{ $t('tires.wearComparison.title') }}</p>
    <dl class="grid grid-cols-2 gap-3">
      <div class="min-w-0">
        <dt class="text-slate-400">{{ $t('tires.wearComparison.measured') }}</dt>
        <dd class="font-semibold text-slate-200">{{ $t('tires.wearComparison.rate', { value: rate(comparison.measured_rate_per_10k_km), distance: formatNumber(10000, 0), unit: distanceUnit() }) }}</dd>
        <dd class="text-slate-400">{{ $t('tires.wearComparison.lifespan', { value: formatDistanceValue(comparison.measured_lifespan_km), unit: distanceUnit() }) }}</dd>
      </div>
      <div class="min-w-0">
        <dt class="text-slate-400">{{ $t('tires.wearComparison.expected') }}</dt>
        <dd class="font-semibold text-slate-200">{{ $t('tires.wearComparison.rate', { value: rate(comparison.expected_rate_per_10k_km), distance: formatNumber(10000, 0), unit: distanceUnit() }) }}</dd>
        <dd class="text-slate-400">{{ $t('tires.wearComparison.lifespan', { value: formatDistanceValue(comparison.expected_lifespan_km), unit: distanceUnit() }) }}</dd>
      </div>
    </dl>
    <p :class="tireWearComparisonTone(comparison.delta_pct)">{{ $t(`tires.wearComparison.${deltaKey}`, { pct: formatPercent(Math.abs(comparison.delta_pct)) }) }}</p>
    <p class="text-slate-400">{{ $t(`tires.wearComparison.${comparison.reference}`) }}</p>
    <p class="text-slate-400">{{ $t('tires.wearComparison.confidence', { pct: formatPercent(Number(stat.wear_rate_confidence) * 100) }) }}</p>
  </div>
</template>
