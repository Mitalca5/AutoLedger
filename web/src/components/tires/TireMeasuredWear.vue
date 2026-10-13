<script setup lang="ts">
import { formatNumber, formatPercent } from '@/utils/numbers'
import TireWearBar from '@/components/tires/TireWearBar.vue'
import TireWearComparison from '@/components/tires/TireWearComparison.vue'

// Wear measured from tread depth, shown next to the theoretical wear by distance
defineProps<{ stat: any }>()
</script>

<template>
  <div v-if="stat.logs_count > 0" class="space-y-1.5">
    <div class="flex items-center justify-between text-xs text-slate-400">
      <span>{{ $t('tires.measuredWear.label') }} · {{ $t('tires.measuredWear.value', { depth: formatNumber(Number(stat.current_depth_mm), 1), pct: formatPercent(Number(stat.wear_percentage)) }) }}</span>
    </div>
    <TireWearBar :pct="stat.wear_percentage" :condition="stat.condition" />
    <TireWearComparison :stat="stat" />
    <p v-if="stat.wear_rate_source === 'measured' && stat.wear_rate_confidence < 0.5" class="text-xs text-slate-400">
      {{ $t('tires.measuredWear.lowConfidence') }}
    </p>
  </div>
  <p v-else class="text-xs text-slate-400">{{ $t('tires.measuredWear.none') }}</p>
</template>
