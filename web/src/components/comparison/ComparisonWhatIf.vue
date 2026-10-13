<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { intlLocale, t } from '@/i18n'
import { api } from '@/services/api'
import { distanceUnit, formatDistanceValue } from '@/units'
import DistanceInput from '@/components/DistanceInput.vue'
import NumberInput from '@/components/NumberInput.vue'
import { formatAmount } from '@/currency'
import { perVolume, volumeUnitLabel } from '@/units'
import { useComparisonPriceHistory } from '@/composables/useComparisonPriceHistory'
import { useEnergyPriceWarning } from '@/composables/useEnergyPriceWarning'
import { useScenarioDraft } from '@/composables/useScenarioDraft'
import { HIGH_PRICE_MULTIPLIER } from '@/utils/energyPriceWarning'
import { breakEvenDelta, breakEvenReasonKey } from '@/utils/comparisonBreakEven'
import type { ComparisonForm } from '@/utils/comparisonForm'
import {
  hasExtendedLimits,
  isRetrospective,
  isWhatIfValid,
  measuredElectricityPrice,
  whatIfBase,
  whatIfBounds,
  whatIfRanges,
  whatIfFigures,
  whatIfForm,
  whatIfPayload,
  type WhatIfInputs,
  type WhatIfInputLimits,
  type WhatIfKey,
  type WhatIfSliderKey,
} from '@/utils/comparisonWhatIf'
import { RotateCcw, SlidersHorizontal } from 'lucide-vue-next'

// Explore other annual distance, fuel and electricity prices on a projection without editing the saved scenario:
// the explored result comes from the server's comparison engine, nothing is stored and recorded costs are untouched.
const props = defineProps<{ scenario: any; result: any; currency: string; referenceVehicleId?: string }>()
const emit = defineEmits<{ 'save-as-new': [form: ComparisonForm] }>()

const DEBOUNCE_MS = 250

const retro = computed(() => isRetrospective(props.scenario))
// A retrospective scenario follows the recorded costs: only the electricity price can be tried against them.
const measuredPrice = computed(() => (retro.value ? measuredElectricityPrice(props.result) : 0))
const measured = computed(() => (retro.value ? props.result?.tracked_measured : null))
const base = computed(() => whatIfBase(props.scenario, props.result))
const limits = computed<WhatIfInputLimits>(() => props.result.input_limits)
const bounds = computed(() => whatIfBounds(base.value, limits.value))
const ranges = computed(() => whatIfRanges(limits.value))
// Both prices are advisory: an unusual one needs a confirmation before it is evaluated or saved.
const confirmationPending = ref(false)

const { draft: explored, dirty, valid, preview, loading, failed, reset } = useScenarioDraft<WhatIfInputs>({
  base: () => base.value,
  isValid: (d) => isWhatIfValid(d, limits.value),
  payload: (d) => whatIfPayload(props.scenario, d, measuredPrice.value),
  preview: (payload) => api.previewComparison(payload),
  blocked: () => confirmationPending.value,
  debounceMs: DEBOUNCE_MS,
})

const history = useComparisonPriceHistory(() => props.referenceVehicleId, () => props.scenario?.ice?.fuel_type || '', () => props.currency)
const warningContext = () => `${props.referenceVehicleId}:${props.scenario?.id}:${props.scenario?.ice?.fuel_type}:${props.currency}`
const fuelWarning = useEnergyPriceWarning(() => explored.fuel_price, () => history.value?.fuel, warningContext)
const electricityWarning = useEnergyPriceWarning(() => explored.eur_per_kwh, () => history.value?.electricity, warningContext)
const needsConfirmation = computed(() => fuelWarning.needsConfirmation.value || electricityWarning.needsConfirmation.value)
watch(needsConfirmation, (pending) => { confirmationPending.value = pending }, { immediate: true, flush: 'sync' })

// What the tracked side takes from recorded costs, shown read-only so the user sees what follows the data.
const measuredRows = computed(() => {
  const m = measured.value
  if (!m) return []
  const rows: { key: string; label: string; value: string }[] = []
  if (m.kwh_per_100km) rows.push({ key: 'kwh', label: t('comparison.whatIf.measuredKwh'), value: `${Number(m.kwh_per_100km).toLocaleString(intlLocale(), { maximumFractionDigits: 1 })} kWh/100 ${distanceUnit()}` })
  if (m.maintenance_yearly) rows.push({ key: 'maintenance', label: t('comparison.whatIf.measuredMaintenance'), value: fmtMoney(m.maintenance_yearly) })
  if (m.insurance_yearly) rows.push({ key: 'insurance', label: t('comparison.whatIf.measuredInsurance'), value: fmtMoney(m.insurance_yearly) })
  if (m.tax_yearly) rows.push({ key: 'tax', label: t('comparison.whatIf.measuredTax'), value: fmtMoney(m.tax_yearly) })
  if (m.purchase_price) rows.push({ key: 'purchase', label: t('comparison.whatIf.measuredPurchase'), value: fmtMoney(m.purchase_price) })
  return rows
})

const originalFigures = computed(() => whatIfFigures(props.result))
const exploredFigures = computed(() => whatIfFigures(preview.value))

function saveAsNew() {
  if (!valid.value || needsConfirmation.value) return
  const form = whatIfForm(props.scenario, explored, measuredPrice.value)
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
  { key: 'annual_km' as WhatIfSliderKey, id: 'wi-km', label: t('comparison.whatIf.annualKm', { unit: distanceUnit() }) },
  { key: 'fuel_price' as WhatIfSliderKey, id: 'wi-fuel', label: t('comparison.whatIf.fuelPrice') },
  { key: 'eur_per_kwh' as WhatIfSliderKey, id: 'wi-elec', label: t('comparison.whatIf.electricityPrice') },
])

// The inputs without a slider: shown once the result carries their limits, tracked-side ones only with an electric vehicle.
const showMore = computed(() => hasExtendedLimits(limits.value))
const moreFields = computed(() => {
  const money = (key: WhatIfKey, label: string) => ({ key, id: `wi-${key}`, label, step: 1 })
  const pct = (key: WhatIfKey, label: string) => ({ key, id: `wi-${key}`, label, step: 0.1 })
  return [
    { key: 'years' as WhatIfKey, id: 'wi-years', label: t('comparison.whatIf.years_field'), step: 1 },
    money('ice_purchase_price', t('comparison.whatIf.icePurchasePrice')),
    // Measured on a retrospective scenario, where only the combustion side is described by the user
    ...(retro.value ? [] : [money('tracked_purchase_price', t('comparison.whatIf.trackedPurchasePrice'))]),
    money('ice_resale_value', t('comparison.whatIf.iceResaleValue')),
    ...(retro.value ? [] : [money('tracked_resale_value', t('comparison.whatIf.trackedResaleValue'))]),
    pct('fuel_inflation_pct', t('comparison.whatIf.fuelInflation')),
    pct('electricity_inflation_pct', t('comparison.whatIf.electricityInflation')),
    pct('cost_inflation_pct', t('comparison.whatIf.costInflation')),
  ]
})

const originalHint = (key: WhatIfSliderKey) => {
  const v = base.value[key]
  return key === 'annual_km' ? `${formatDistanceValue(v)} ${distanceUnit()}` : v.toLocaleString(intlLocale(), { maximumFractionDigits: 3 })
}

function warningFor(key: WhatIfSliderKey) {
  return key === 'fuel_price' ? fuelWarning : key === 'eur_per_kwh' ? electricityWarning : null
}

function historicalMedian(key: WhatIfSliderKey): string {
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
    { key: 'breakEven', label: t('comparison.whatIf.breakEven'), original: fmtYears(o.breakEvenYear), explored: e ? fmtYears(e.breakEvenYear) : '', delta: e ? fmtBreakEvenDelta(o.breakEvenYear, e.breakEvenYear) : '' },
  ]
})

function fmtBreakEvenDelta(original: number | null, explored: number | null): string {
  const d = breakEvenDelta(original, explored)
  if (d.kind === 'reached') return t('comparison.whatIf.breakEvenReached')
  if (d.kind === 'lost') return t('comparison.whatIf.breakEvenLost')
  if (d.kind === 'none') return ''
  const sign = d.years > 0 ? '+' : d.years < 0 ? '−' : ''
  return `${sign}${t('comparison.whatIf.years', { years: Math.abs(d.years).toLocaleString(intlLocale(), { maximumFractionDigits: 1 }) })}`
}

// Why the break-even shown for the explored values (or the saved ones until something changes) is missing.
const breakEvenReasonText = computed(() => {
  const shown = dirty.value && preview.value ? preview.value : props.result
  if (!shown || (shown.break_even_year !== undefined && shown.break_even_year !== null)) return ''
  const key = breakEvenReasonKey(shown.break_even_reason)
  return key ? t(key, { years: shown.years_count }) : ''
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
          <span v-if="retro && f.key === 'eur_per_kwh'" class="inline-block mt-1 text-[11px] px-2 py-0.5 rounded-full border border-success-500/40 text-success-300" data-testid="what-if-measured-badge">
            {{ $t('comparison.whatIf.measuredBadge', { value: measuredPrice.toLocaleString(intlLocale(), { maximumFractionDigits: 3 }) }) }}
          </span>
        </div>
        <div class="sm:col-span-3 empty:hidden" aria-live="polite">
          <div v-if="valid && warningFor(f.key)?.needsConfirmation.value" class="rounded-lg border border-warning-500/40 p-3 text-xs text-warning-300">
            <p>{{ $t('comparison.priceWarning.high', { factor: HIGH_PRICE_MULTIPLIER, median: historicalMedian(f.key), unit: f.key === 'fuel_price' ? volumeUnitLabel() : 'kWh' }) }}</p>
            <button type="button" class="btn btn-secondary mt-2" @click="warningFor(f.key)?.confirm()">{{ $t('comparison.priceWarning.keep') }}</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="measuredRows.length" class="mt-4" data-testid="what-if-measured">
      <h3 class="text-xs font-semibold text-slate-300">{{ $t('comparison.whatIf.measuredTitle') }}</h3>
      <dl class="grid gap-x-4 gap-y-1 sm:grid-cols-2 mt-2 text-xs">
        <div v-for="r in measuredRows" :key="r.key" class="flex justify-between gap-2 border-b border-slate-800 py-1">
          <dt class="text-slate-400">{{ r.label }}</dt>
          <dd class="text-slate-200">{{ r.value }}</dd>
        </div>
      </dl>
    </div>

    <details v-if="showMore" class="mt-4" data-testid="what-if-more">
      <summary class="text-xs font-semibold text-slate-300 cursor-pointer">{{ $t('comparison.whatIf.more') }}</summary>
      <div class="grid gap-3 sm:grid-cols-2 mt-3">
        <div v-for="f in moreFields" :key="f.key">
          <label :for="f.id" class="text-xs font-semibold text-slate-300">{{ f.label }}</label>
          <NumberInput :id="f.id" v-model="explored[f.key]" :min="ranges[f.key].min" :max="ranges[f.key].max" :step="f.step" class="field" />
          <div class="text-[11px] text-slate-500 mt-0.5">{{ $t('comparison.whatIf.original', { value: base[f.key].toLocaleString(intlLocale(), { maximumFractionDigits: 3 }) }) }}</div>
        </div>
      </div>
    </details>

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
    <p v-if="breakEvenReasonText" class="text-xs text-slate-300 mt-2" data-testid="what-if-break-even-reason">{{ breakEvenReasonText }}</p>
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
