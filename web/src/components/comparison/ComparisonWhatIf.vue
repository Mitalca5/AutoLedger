<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { intlLocale, t } from '@/i18n'
import { api } from '@/services/api'
import { distanceUnit, formatDistanceValue } from '@/units'
import DistanceInput from '@/components/DistanceInput.vue'
import NumberInput from '@/components/NumberInput.vue'
import { formatAmount } from '@/currency'
import { perVolume, volumeUnitLabel } from '@/units'
import { useComparisonPriceHistory } from '@/composables/useComparisonPriceHistory'
import { useEnergyPriceWarning } from '@/composables/useEnergyPriceWarning'
import { HIGH_PRICE_MULTIPLIER } from '@/utils/energyPriceWarning'
import type { ComparisonForm } from '@/utils/comparisonForm'
import {
  isWhatIfDirty,
  isWhatIfValid,
  whatIfBase,
  whatIfBounds,
  whatIfRanges,
  whatIfFigures,
  whatIfForm,
  whatIfPayload,
  type WhatIfInputs,
  type WhatIfInputLimits,
  type WhatIfKey,
} from '@/utils/comparisonWhatIf'
import { RotateCcw, SlidersHorizontal } from 'lucide-vue-next'

// Explore other annual distance, fuel and electricity prices on a projection without editing the saved scenario:
// the explored result comes from the server's comparison engine, nothing is stored and recorded costs are untouched.
const props = defineProps<{ scenario: any; result: any; currency: string; referenceVehicleId?: string }>()
const emit = defineEmits<{ 'save-as-new': [form: ComparisonForm] }>()

const DEBOUNCE_MS = 250

const base = computed(() => whatIfBase(props.scenario))
const limits = computed<WhatIfInputLimits>(() => props.result.input_limits)
const bounds = computed(() => whatIfBounds(base.value, limits.value))
const ranges = computed(() => whatIfRanges(limits.value))
const explored = reactive<WhatIfInputs>({ ...base.value })
const history = useComparisonPriceHistory(() => props.referenceVehicleId, () => props.scenario?.ice?.fuel_type || '', () => props.currency)
const warningContext = () => `${props.referenceVehicleId}:${props.scenario?.id}:${props.scenario?.ice?.fuel_type}:${props.currency}`
const fuelWarning = useEnergyPriceWarning(() => explored.fuel_price, () => history.value?.fuel, warningContext)
const electricityWarning = useEnergyPriceWarning(() => explored.eur_per_kwh, () => history.value?.electricity, warningContext)
const needsConfirmation = computed(() => fuelWarning.needsConfirmation.value || electricityWarning.needsConfirmation.value)
const preview = ref<any | null>(null)
const loading = ref(false)
const failed = ref(false)

const dirty = computed(() => isWhatIfDirty(base.value, explored))
const valid = computed(() => isWhatIfValid(explored, limits.value))
const originalFigures = computed(() => whatIfFigures(props.result))
const exploredFigures = computed(() => whatIfFigures(preview.value))

let timer: ReturnType<typeof setTimeout> | undefined
let sequence = 0

function reset() {
  Object.assign(explored, base.value)
}

// A new scenario or a recomputed result starts again from its own values.
watch(base, reset)

async function refresh() {
  const current = ++sequence
  loading.value = true
  failed.value = false
  try {
    const res = await api.previewComparison(whatIfPayload(props.scenario, explored))
    if (current === sequence) preview.value = res
  } catch (err) {
    console.error('Failed to preview the what-if', err)
    if (current === sequence) failed.value = true
  } finally {
    if (current === sequence) loading.value = false
  }
}

watch(
  () => [JSON.stringify(explored), needsConfirmation.value],
  () => {
    clearTimeout(timer)
    // A pending confirmation drops the previous result: it was computed for another price.
    if (!dirty.value || needsConfirmation.value) {
      sequence += 1
      preview.value = null
      loading.value = false
      failed.value = false
      return
    }
    if (!valid.value) return
    timer = setTimeout(refresh, DEBOUNCE_MS)
  },
)

onBeforeUnmount(() => {
  clearTimeout(timer)
  sequence += 1
})

function saveAsNew() {
  if (!valid.value || needsConfirmation.value) return
  const form = whatIfForm(props.scenario, explored)
  form.name = t('comparison.whatIf.copyName', { name: props.scenario?.name || '' }).slice(0, 100)
  emit('save-as-new', form)
}

function fmtMoney(v: number): string {
  return Number(v || 0).toLocaleString(intlLocale(), { style: 'currency', currency: props.currency, maximumFractionDigits: 0 })
}

function fmtSigned(v: number): string {
  return `${v > 0 ? '+' : v < 0 ? '−' : ''}${fmtMoney(Math.abs(v))}`
}

function fmtYears(v: number | null): string {
  return v === null ? t('comparison.whatIf.notReached') : t('comparison.whatIf.years', { years: Number(v).toLocaleString(intlLocale(), { maximumFractionDigits: 1 }) })
}

const fields = computed(() => [
  { key: 'annual_km' as WhatIfKey, id: 'wi-km', label: t('comparison.whatIf.annualKm', { unit: distanceUnit() }) },
  { key: 'fuel_price' as WhatIfKey, id: 'wi-fuel', label: t('comparison.whatIf.fuelPrice') },
  { key: 'eur_per_kwh' as WhatIfKey, id: 'wi-elec', label: t('comparison.whatIf.electricityPrice') },
])

const originalHint = (key: WhatIfKey) => {
  const v = base.value[key]
  return key === 'annual_km' ? `${formatDistanceValue(v)} ${distanceUnit()}` : v.toLocaleString(intlLocale(), { maximumFractionDigits: 3 })
}

function warningFor(key: WhatIfKey) {
  return key === 'fuel_price' ? fuelWarning : key === 'eur_per_kwh' ? electricityWarning : null
}

function historicalMedian(key: WhatIfKey): string {
  const median = key === 'fuel_price' ? history.value?.fuel.median : history.value?.electricity.median
  return formatAmount(key === 'fuel_price' ? perVolume(median || 0) : median || 0, props.currency)
}

const rows = computed(() => {
  const o = originalFigures.value
  const e = exploredFigures.value
  if (!o) return []
  return [
    { key: 'energy', label: t('comparison.whatIf.energySavings'), original: fmtSigned(o.energySavings), explored: e ? fmtSigned(e.energySavings) : '', delta: e ? fmtSigned(e.energySavings - o.energySavings) : '' },
    { key: 'total', label: t('comparison.whatIf.totalSavings'), original: fmtSigned(o.totalSavings), explored: e ? fmtSigned(e.totalSavings) : '', delta: e ? fmtSigned(e.totalSavings - o.totalSavings) : '' },
    { key: 'breakEven', label: t('comparison.whatIf.breakEven'), original: fmtYears(o.breakEvenYear), explored: e ? fmtYears(e.breakEvenYear) : '', delta: '' },
  ]
})
</script>

<template>
  <!-- Older cached results without API bounds cannot safely drive the what-if controls. -->
  <section v-if="limits" class="bg-slate-900 border border-slate-800 rounded-2xl p-4 no-print" data-testid="what-if">
    <div class="flex items-center justify-between gap-3 mb-3">
      <h2 class="text-sm font-semibold text-slate-300 flex items-center gap-1.5">
        <SlidersHorizontal class="w-4 h-4 text-slate-400" /> {{ $t('comparison.whatIf.title') }}
      </h2>
      <button type="button" class="btn btn-secondary" :disabled="!dirty" @click="reset">
        <RotateCcw class="w-4 h-4" /> {{ $t('comparison.whatIf.reset') }}
      </button>
    </div>
    <p class="text-xs text-slate-400 mb-4">{{ $t('comparison.whatIf.intro') }}</p>

    <div class="space-y-4">
      <div v-for="f in fields" :key="f.key" class="grid gap-2 sm:grid-cols-[minmax(0,12rem)_minmax(0,1fr)_7rem] sm:items-center">
        <label :for="f.id" class="text-xs font-semibold text-slate-300">{{ f.label }}</label>
        <input
          v-model.number="explored[f.key]"
          type="range"
          class="w-full accent-rose-500"
          :min="bounds[f.key].min"
          :max="bounds[f.key].max"
          :step="bounds[f.key].step"
          :aria-label="f.label"
        />
        <div>
          <DistanceInput v-if="f.key === 'annual_km'" :id="f.id" v-model="explored.annual_km" :digits="0" :min="ranges.annual_km.min" :max="ranges.annual_km.max" class="field" />
          <DistanceInput v-else-if="f.key === 'fuel_price'" :id="f.id" v-model="explored.fuel_price" kind="per-volume" :digits="3" :min="ranges.fuel_price.min" :max="ranges.fuel_price.max" class="field" />
          <NumberInput v-else :id="f.id" v-model="explored.eur_per_kwh" :min="ranges.eur_per_kwh.min" :max="ranges.eur_per_kwh.max" class="field" />
          <div class="text-[11px] text-slate-500 mt-0.5">{{ $t('comparison.whatIf.original', { value: originalHint(f.key) }) }}</div>
        </div>
        <div class="sm:col-span-3 empty:hidden" aria-live="polite">
          <div v-if="valid && warningFor(f.key)?.needsConfirmation.value" class="rounded-lg border border-warning-500/40 p-3 text-xs text-warning-300">
            <p>{{ $t('comparison.priceWarning.high', { factor: HIGH_PRICE_MULTIPLIER, median: historicalMedian(f.key), unit: f.key === 'fuel_price' ? volumeUnitLabel() : 'kWh' }) }}</p>
            <button type="button" class="btn btn-secondary mt-2" @click="warningFor(f.key)?.confirm()">{{ $t('comparison.priceWarning.keep') }}</button>
          </div>
        </div>
      </div>
    </div>

    <div class="overflow-x-auto mt-5" aria-live="polite" :aria-busy="loading">
      <table class="w-full text-sm">
        <caption class="sr-only">{{ $t('comparison.whatIf.tableCaption') }}</caption>
        <thead>
          <tr class="text-xs text-slate-400 text-right">
            <th scope="col" class="text-left font-semibold pb-2"><span class="sr-only">{{ $t('comparison.comparisonView.category') }}</span></th>
            <th scope="col" class="font-semibold pb-2">{{ $t('comparison.whatIf.originalColumn') }}</th>
            <th scope="col" class="font-semibold pb-2">{{ $t('comparison.whatIf.exploredColumn') }}</th>
            <th scope="col" class="font-semibold pb-2">{{ $t('comparison.whatIf.gapColumn') }}</th>
          </tr>
        </thead>
        <tbody class="text-slate-200" :class="loading ? 'opacity-60' : ''">
          <tr v-for="row in rows" :key="row.key" class="border-t border-slate-800 text-right">
            <th scope="row" class="text-left font-normal py-1.5">{{ row.label }}</th>
            <td>{{ row.original }}</td>
            <td class="font-semibold text-white">{{ dirty ? row.explored || '…' : '—' }}</td>
            <td class="text-slate-400">{{ dirty ? row.delta : '' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-xs text-slate-500 mt-2">{{ $t('comparison.whatIf.distinct') }}</p>
    <p v-if="!valid" class="text-xs text-red-300 mt-2" role="alert">{{ $t('comparison.whatIf.invalid') }}</p>
    <p v-else-if="failed" class="text-xs text-red-300 mt-2" role="alert">{{ $t('comparison.whatIf.failed') }}</p>

    <div class="flex justify-end mt-3">
      <button type="button" class="btn btn-secondary" :disabled="!dirty || !valid || needsConfirmation" @click="saveAsNew">
        {{ $t('comparison.whatIf.saveAsNew') }}
      </button>
    </div>
  </section>
</template>
