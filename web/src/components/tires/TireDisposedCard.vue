<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistanceValue } from '@/units'
import { formatPercent } from '@/utils/numbers'

// A tire that was disposed of: its cost stays in the TCO
defineProps<{ t: any; selected: boolean; dismount?: { date: string; odometer: number | null } | null }>()
const emit = defineEmits<{ open: [stat: any]; toggle: [tireId: string] }>()
const vehicleStore = useVehicleStore()
const day = (iso: string) => new Date(iso).toLocaleDateString(intlLocale(), { dateStyle: 'medium', timeZone: 'UTC' })
</script>

<template>
  <div
    v-clickable
    @click="emit('open', t)"
    class="bg-slate-900/60 border border-slate-800 hover:border-rose-500/40 cursor-pointer rounded-2xl p-4 space-y-2 transition-all group"
    :class="{ 'ring-2 ring-rose-500/50 border-rose-500/60': selected }"
  >
    <div class="flex items-start justify-between">
      <div>
        <h4 class="text-sm font-bold text-slate-300 group-hover:text-rose-300 transition-colors flex items-center gap-1.5">
          <label :for="'disposed-select-' + t.tire.id" @click.stop class="cursor-pointer flex items-center" :title="$t('tires.tireDisposedCard.selectForABulkAction')">
            <input
              :id="'disposed-select-' + t.tire.id"
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
        {{ $t('tires.tireDisposedCard.scrapped') }}
      </span>
    </div>
    <div class="text-xs text-slate-400">
      {{ $t('tires.tireDisposedCard.kmDrivenWear', { unit: distanceUnit(), total_distance_km: formatDistanceValue(t.total_distance_km), life_progress_pct: formatPercent(Number(t.life_progress_pct)), purchase_price: formatAmount(Number(t.tire.purchase_price || 0), vehicleStore.currency) }) }}
    </div>
    <div v-if="dismount" class="text-xs text-slate-500">
      {{ dismount.odometer
        ? $t('tires.tireDisposedCard.scrappedOnAt', { date: day(dismount.date), odometer: formatDistanceValue(dismount.odometer), unit: distanceUnit() })
        : $t('tires.tireDisposedCard.scrappedOn', { date: day(dismount.date) }) }}
    </div>
  </div>
</template>
