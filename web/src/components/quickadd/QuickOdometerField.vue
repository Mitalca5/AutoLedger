<script setup lang="ts">
import { computed, watch } from 'vue'
import DistanceInput from '@/components/DistanceInput.vue'
import { distanceUnit, formatDistance } from '@/units'
import { checkOdometer } from '@/utils/quickAdd'

const props = defineProps<{ id: string; last: number | null | undefined }>()
const model = defineModel<string>({ required: true })
// Set once the user chose to keep a reading lower than the last one; typing again resets it.
const confirmed = defineModel<boolean>('confirmed', { default: false })

const below = computed(() => checkOdometer(model.value, props.last) === 'below')
const lastLabel = computed(() => formatDistance(Math.round(props.last ?? 0)))

watch(model, () => (confirmed.value = false))

function fix() {
  model.value = String(Math.round(props.last ?? 0))
}
</script>

<template>
  <div>
    <label :for="id" class="quick-label">{{ $t('quickadd.odometerField.label', { unit: distanceUnit() }) }}</label>
    <DistanceInput text :id="id" v-model="model" inputmode="numeric" min="0" class="quick-input" />
    <div v-if="below && !confirmed" role="alert" class="mt-2 space-y-2 rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-2.5 text-xs text-amber-200">
      <p>{{ $t('quickadd.odometerField.below', { last: lastLabel }) }}</p>
      <div class="flex gap-2">
        <button type="button" class="quick-chip min-h-11 flex-1 border-amber-500/40 bg-amber-500/15 text-amber-100" @click="fix">{{ $t('quickadd.odometerField.fix') }}</button>
        <button type="button" class="quick-chip min-h-11 flex-1 border-slate-700 bg-slate-800 text-slate-200" @click="confirmed = true">{{ $t('quickadd.odometerField.keep') }}</button>
      </div>
    </div>
    <p v-else-if="below" class="mt-1.5 text-[11px] text-slate-400" role="status">{{ $t('quickadd.odometerField.kept', { last: lastLabel }) }}</p>
  </div>
</template>
