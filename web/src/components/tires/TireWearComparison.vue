<script setup lang="ts">
import { computed } from 'vue'
import { Info } from 'lucide-vue-next'
import { distanceUnit, formatDistanceValue } from '@/units'
import { formatPercent } from '@/utils/numbers'
import { MIN_CONFIDENCE_FOR_TONE, tireWearComparison, tireWearComparisonTone } from '@/utils/tireWearComparison'

const props = defineProps<{ stat: any }>()
const comparison = computed(() => tireWearComparison(props.stat))
const thin = computed(() => !(Number(props.stat?.wear_rate_confidence) >= MIN_CONFIDENCE_FOR_TONE))
const delta = computed(() => {
  const d = comparison.value?.delta_pct ?? 0
  const pct = formatPercent(Math.abs(d), 0)
  return d === 0 ? null : `${d > 0 ? '+' : '−'}${pct}`
})
// The pill takes the tone of the gap: amber when the tire wears out sooner, green when later, neutral otherwise
const pillClass = computed(() => {
  const tone = comparison.value ? tireWearComparisonTone(comparison.value, Number(props.stat?.wear_rate_confidence)) : ''
  if (tone === 'text-warning-400') return 'border-warning-500/30 bg-warning-500/10 text-warning-300'
  if (tone === 'text-success-400') return 'border-success-500/30 bg-success-500/10 text-success-300'
  return 'border-slate-700 bg-slate-800/60 text-slate-300'
})
</script>

<template>
  <div v-if="comparison" class="mt-2.5 space-y-2.5 rounded-xl border border-slate-800 bg-slate-950/60 p-3 text-xs" data-testid="tire-wear-comparison">
    <div class="flex flex-wrap items-center justify-between gap-x-2 gap-y-1.5">
      <p class="min-w-0 font-semibold text-slate-200">{{ $t('tires.wearComparison.title') }}</p>
      <span class="shrink-0 rounded-full border px-2 py-0.5 font-semibold" :class="pillClass">
        {{ delta === null ? $t('tires.wearComparison.same') : $t('tires.wearComparison.lifespanGap', { gap: delta }) }}
      </span>
    </div>
    <dl class="grid grid-cols-2 gap-3">
      <div class="min-w-0">
        <dt class="text-slate-400">{{ $t('tires.wearComparison.measured') }}</dt>
        <dd class="text-sm font-semibold text-white">
          {{ formatDistanceValue(comparison.measured_lifespan_km) }} <span class="text-xs font-medium text-slate-400">{{ distanceUnit() }}</span>
        </dd>
      </div>
      <div class="min-w-0">
        <dt class="text-slate-400">{{ $t('tires.wearComparison.expected') }}</dt>
        <dd class="text-sm font-semibold text-white">
          {{ formatDistanceValue(comparison.expected_lifespan_km) }} <span class="text-xs font-medium text-slate-400">{{ distanceUnit() }}</span>
        </dd>
      </div>
    </dl>
    <p class="flex items-start gap-1.5 text-[11px] leading-snug text-slate-500">
      <Info class="mt-px h-3 w-3 shrink-0" aria-hidden="true" />
      <span>
        {{ $t(`tires.wearComparison.${comparison.reference}`) }}
        <template v-if="thin"> {{ $t('tires.wearComparison.thin') }}</template>
      </span>
    </p>
  </div>
</template>
