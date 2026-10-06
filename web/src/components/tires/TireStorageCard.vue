<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { formatDistance } from '@/units'
import { getSeasonIcon, WEAR_TONE_TEXT, wearTone } from '@/utils/tires'
import { formatPercent } from '@/utils/numbers'
import TireWearBar from '@/components/tires/TireWearBar.vue'

// A tire kept in the garage
defineProps<{ t: any; selected: boolean }>()
const emit = defineEmits<{ open: [stat: any]; toggle: [tireId: string] }>()
</script>

<template>
  <div
    v-clickable
    @click="emit('open', t)"
    class="bg-slate-900 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-2xl p-4 space-y-3 shadow-sm transition-all group"
    :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selected }"
  >
    <div class="flex items-start justify-between">
      <div>
        <div class="flex items-center gap-1.5 text-xs font-semibold">
          <component :is="getSeasonIcon(t.tire.season).icon" class="w-3.5 h-3.5" :class="getSeasonIcon(t.tire.season).color" />
          <span class="text-slate-300">{{ getSeasonIcon(t.tire.season).label }}</span>
        </div>
        <h4 class="text-sm font-bold text-white group-hover:text-rose-300 transition-colors mt-1 flex items-center gap-1.5">
          <label :for="'storage-select-' + t.tire.id" @click.stop class="cursor-pointer flex items-center" :title="$t('tires.tireStorageCard.selectForABulkAction')">
            <input
              :id="'storage-select-' + t.tire.id"
              type="checkbox"
              :checked="selected"
              @change="emit('toggle', t.tire.id)"
              class="select-box"
            />
          </label>
          {{ t.tire.brand }} {{ t.tire.model }}
        </h4>
        <div class="text-xs text-slate-400 font-mono">{{ t.tire.dimension }}</div>
      </div>
      <span class="bg-slate-800 text-slate-400 text-xs px-2 py-0.5 rounded-full border border-slate-700 font-medium">
        {{ $t('tires.tireStorageCard.inStorage') }}
      </span>
    </div>

    <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80 text-center text-xs">
      <div>
        <div class="text-xs text-slate-400">{{ $t('tires.tireStorageCard.totalDriven') }}</div>
        <div class="font-bold text-white">{{ formatDistance(t.total_distance_km) }}</div>
      </div>
      <div>
        <div class="text-xs text-slate-400">{{ $t('tires.tireStorageCard.estimatedWear') }}</div>
        <div class="font-bold" :class="WEAR_TONE_TEXT[wearTone(t.life_progress_pct, t.condition)]">{{ formatPercent(Number(t.life_progress_pct)) }}</div>
      </div>
    </div>

    <TireWearBar :pct="t.life_progress_pct" :condition="t.condition" />
  </div>
</template>
