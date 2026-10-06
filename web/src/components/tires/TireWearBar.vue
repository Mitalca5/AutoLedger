<script setup lang="ts">
import { computed } from 'vue'
import { t } from '@/i18n'
import { WEAR_TONE_BG, wearTone } from '@/utils/tires'

const props = withDefaults(defineProps<{ pct: number | string; condition?: string; thick?: boolean }>(), { thick: false })

const value = computed(() => Math.max(0, Number(props.pct) || 0))
const bg = computed(() => WEAR_TONE_BG[wearTone(value.value, props.condition)])
</script>

<template>
  <div
    class="w-full bg-slate-800 rounded-full overflow-hidden"
    :class="thick ? 'h-2.5' : 'h-2'"
    role="progressbar"
    :aria-label="t('tires.wearBar.label')"
    aria-valuemin="0"
    aria-valuemax="100"
    :aria-valuenow="Math.min(100, Math.round(value))"
  >
    <div class="h-full rounded-full transition-all" :class="bg" :style="{ width: `${Math.min(100, value)}%` }"></div>
  </div>
</template>
