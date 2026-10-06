<script setup lang="ts">
import { formatNumber, formatPercent } from '@/utils/numbers'
import { intlLocale } from '@/i18n'
import { Disc } from 'lucide-vue-next'
import { getConditionBadge } from '@/utils/tires'
import TireWearBar from '@/components/tires/TireWearBar.vue'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { distanceUnit, formatDistance, formatDistanceValue, perDistance } from '@/units'
import { formatCostPerDistance } from '@/utils/costPerDistance'

// One wheel of the chassis view: the mounted tire with its wear, or a placeholder when the wheel is empty
defineProps<{ pos: string; label: string; stat: any | null; selected: boolean; canMount?: boolean }>()
const emit = defineEmits<{ open: [stat: any]; toggle: [tireId: string]; mount: [] }>()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div
    v-if="stat"
    v-clickable
    @click="emit('open', stat)"
    class="bg-slate-900 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-2xl sm:rounded-3xl p-3.5 sm:p-5 space-y-3 sm:space-y-4 shadow-sm transition-all group"
    :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selected }"
  >
    <div class="flex items-start justify-between">
      <div>
        <label :for="'chassis-select-' + pos.toLowerCase() + '-' + stat.tire.id" @click.stop class="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-danger-400 hover:text-danger-300 cursor-pointer" :title="selected ? $t('tires.tireWheelCard.removeFromSelection') : $t('tires.tireWheelCard.selectForBulk')">
          <input
            :id="'chassis-select-' + pos.toLowerCase() + '-' + stat.tire.id"
            type="checkbox"
            :checked="selected"
            @change="emit('toggle', stat.tire.id)"
            class="select-box"
          />
          <span>{{ label }} ({{ pos }})</span>
        </label>
        <h3 class="text-sm sm:text-base font-bold text-white group-hover:text-rose-300 transition-colors mt-0.5">{{ stat.tire.brand }} {{ stat.tire.model }}</h3>
        <div class="text-xs text-slate-400 font-mono">{{ stat.tire.dimension }}</div>
      </div>
      <span
        class="px-2.5 py-1 rounded-full text-xs font-semibold border"
        :class="getConditionBadge(stat.condition).class"
      >
        {{ getConditionBadge(stat.condition).label }}
      </span>
    </div>

    <!-- Metrics Row -->
    <div class="grid grid-cols-2 gap-2 bg-slate-950/60 p-2 sm:p-3 rounded-xl sm:rounded-2xl border border-slate-800/80 text-center">
      <div>
        <div class="text-xs text-slate-400 uppercase">{{ $t('tires.tireWheelCard.totalDriven') }}</div>
        <div class="text-sm font-bold text-slate-200">{{ formatDistance(stat.total_distance_km) }}</div>
      </div>
      <div>
        <div class="text-xs text-slate-400 uppercase">{{ $t('tires.tireWheelCard.costKm', { unit: distanceUnit() }) }}</div>
        <div class="text-sm font-bold text-warning-400" :title="Number(stat.tire.purchase_price) > 0 ? undefined : $t('tires.costPerKmUnset')">{{ formatCostPerDistance(stat.cost_per_km, vehicleStore.currency, 4) }}</div>
      </div>
    </div>

    <!-- Lifespan progress bar -->
    <div class="space-y-1.5">
      <div class="flex items-center justify-between text-xs text-slate-400">
        <span>{{ $t('tires.tireWheelCard.estimatedLifespanWearKm', { unit: distanceUnit(), estimated_lifespan_km: formatDistanceValue(stat.estimated_lifespan_km) }) }}</span>
        <span class="font-bold text-slate-200">{{ formatPercent(Number(stat.life_progress_pct)) }}</span>
      </div>
      <TireWearBar :pct="stat.life_progress_pct" :condition="stat.condition" />
    </div>
  </div>
  <div v-else class="bg-slate-900/40 border border-dashed border-slate-800 rounded-2xl sm:rounded-3xl p-4 sm:p-8 text-center text-slate-400 flex flex-col items-center justify-center space-y-2">
    <Disc class="w-8 h-8 opacity-30" />
    <span>{{ $t('tires.tireWheelCard.noTireFittedAtThe', { label, pos }) }}</span>
    <button v-if="canMount" type="button" @click="emit('mount')" class="text-xs font-semibold text-rose-400 hover:text-rose-300 underline">
      {{ $t('tires.tireWheelCard.fitATire') }}
    </button>
  </div>
</template>
