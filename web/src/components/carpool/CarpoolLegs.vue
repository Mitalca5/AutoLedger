<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import DistanceInput from '@/components/DistanceInput.vue'
import { Navigation, Plus, Trash2, Calculator, RotateCw } from 'lucide-vue-next'
import { COST_FIELDS, euros, type LegForm } from '@/utils/carpool'
import { currencySymbol, formatAmount } from '@/currency'
import { distanceUnit, formatDistanceValue } from '@/units'

// The legs of a carpool with their actual costs. Estimating, adding and removing a leg belong to the form, so they are
// asked of it; the fields of a leg are edited in place.
const props = defineProps<{
  legs: LegForm[]
  sourceMode: 'DRIVES' | 'MANUAL'
  isEditing: boolean
  submitted: boolean
  estimating: boolean
  live: any
  liveDistance: number
  currency: string
}>()
const emit = defineEmits<{ recalculate: []; 'add-leg': []; estimate: [index: number]; remove: [index: number] }>()
const fmt = (v: number) => formatAmount(Number(v || 0), props.currency)
</script>

<template>
  <div class="space-y-2">
    <div class="flex items-center justify-between gap-2 flex-wrap">
      <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
        <Navigation class="w-4 h-4 text-indigo-400" />
        {{ $t('carpool.carpoolTripModal.legsAndActualCostsKm', { unit: distanceUnit(), liveDistance: formatDistanceValue(liveDistance, 1), total: fmt(euros(live.total)) }) }}
      </h4>
      <div class="flex items-center gap-2">
        <button
          v-if="isEditing && legs.length > 0"
          type="button"
          @click="emit('recalculate')"
          :disabled="estimating"
          class="text-xs text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1.5 px-2.5 py-1 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 rounded-lg transition-colors disabled:opacity-50"
          :title="$t('carpool.carpoolTripModal.updateTheActualCostsWith')"
        >
          <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': estimating }" />
          <span>{{ $t('carpool.carpoolTripModal.recalculateTheCosts') }}</span>
        </button>
        <button
          v-if="sourceMode === 'MANUAL'"
          type="button"
          @click="emit('add-leg')"
          class="text-xs text-indigo-400 hover:text-indigo-300 font-semibold flex items-center gap-1"
        >
          <Plus class="w-3.5 h-3.5" /> {{ $t('carpool.carpoolTripModal.addALeg') }}
        </button>
      </div>
    </div>
    <p v-if="!legs.length" role="status" :class="['text-xs', submitted ? 'text-danger-400' : 'text-slate-400']">{{ $t('carpool.carpoolTripModal.selectAtLeastOneDrive') }}</p>

    <div v-for="(leg, i) in legs" :key="i" class="bg-slate-950/50 border border-slate-800 rounded-xl p-3 space-y-2">
      <div class="flex flex-wrap items-center gap-2 text-xs">
        <span class="font-bold text-indigo-300">{{ $t('carpool.carpoolTripModal.leg', { i: i + 1 }) }}</span>
        <label :for="`leg-start-${i}`" class="sr-only">{{ $t('carpool.carpoolTripModal.startOfLeg', { i: i + 1 }) }}</label>
        <input
          :id="`leg-start-${i}`"
          v-model="leg.start_label"
          :placeholder="$t('carpool.carpoolTripModal.start')"
          class="field w-36!"
        />
        <span class="text-slate-400">→</span>
        <label :for="`leg-end-${i}`" class="sr-only">{{ $t('carpool.carpoolTripModal.endOfLeg', { i: i + 1 }) }}</label>
        <input
          :id="`leg-end-${i}`"
          v-model="leg.end_label"
          :placeholder="$t('carpool.carpoolTripModal.destination')"
          class="field w-36!"
        />
        <label :for="`leg-distance-${i}`" class="text-slate-400">{{ distanceUnit() }}</label>
        <DistanceInput
          :id="`leg-distance-${i}`"
          v-model="leg.distance_km"
          step="0.1"
          min="0"
          :readonly="!!leg.drive_id"
          class="field w-20!"
        />
        <button
          v-if="!leg.drive_id"
          type="button"
          @click="emit('estimate', i)"
          class="flex items-center gap-1 text-xs text-indigo-400 hover:text-indigo-300"
          :title="$t('carpool.carpoolTripModal.estimateElectricityTiresMaintenanceAnd')"
        >
          <Calculator class="w-3.5 h-3.5" /> {{ $t('carpool.carpoolTripModal.estimate') }}
        </button>
        <span class="ml-auto text-slate-300">
          {{ $t('carpool.carpoolTripModal.onBoard', { legDetails: fmt(euros(live.legDetails[i]?.total || 0)), legDetails2: 1 + (live.legDetails[i]?.seats || 0) }) }}
          <strong>{{ $t('carpool.carpoolTripModal.person', { legDetails: fmt(euros(live.legDetails[i]?.perPerson || 0)) }) }}</strong>
        </span>
        <button v-if="sourceMode === 'MANUAL' && legs.length > 1" type="button" @click="emit('remove', i)" class="text-slate-400 hover:text-danger-400" :title="$t('carpool.carpoolTripModal.deleteTheLeg')">
          <Trash2 class="w-3.5 h-3.5" />
        </button>
      </div>
      <div class="grid grid-cols-3 sm:grid-cols-6 gap-2">
        <div v-for="f in COST_FIELDS" :key="f.key">
          <label :for="`leg-${f.key}-${i}`" class="block text-xs text-slate-400 mb-0.5">{{ $t(`carpool.costFields.${f.label}`) }} ({{ currencySymbol(currency) }})</label>
          <NumberInput
            :id="`leg-${f.key}-${i}`"
            v-model="(leg as any)[f.key]"
            min="0"
            class="field"
          />
        </div>
      </div>
    </div>
  </div>
</template>
