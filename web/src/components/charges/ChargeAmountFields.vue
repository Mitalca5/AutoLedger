<script setup lang="ts">
import { computed, ref } from 'vue'
import NumberInput from '@/components/NumberInput.vue'
import { useChargeAmounts } from '@/composables/useChargeAmounts'
import { currencySymbol, formatAmount } from '@/currency'

// Energy, price per kWh and total cost of a charge, shared by the quick entry sheet and the charge modal.
const props = withDefaults(
  defineProps<{
    currency: string
    /** Last tariff used on the vehicle: starts the price field and is shown as the source of a calculated cost. */
    suggestedPrice?: number
    variant?: 'quick' | 'form'
    idPrefix?: string
    required?: boolean
    /** The cost of the session under the vehicle's tariff: replaces the suggested price until the user types one. */
    tariff?: { cost: number; plan: string } | null
    /** The energy is fixed (a synchronised charge): only the price and the cost are edited. */
    hideEnergy?: boolean
  }>(),
  { variant: 'form', idPrefix: 'charge', required: false, hideEnergy: false },
)
const kwh = defineModel<string>('kwh', { required: true })
const cost = defineModel<string>('cost', { required: true })

const { price, driver, onPriceInput, onCostInput, setFree } = useChargeAmounts(kwh, cost, props.suggestedPrice, computed(() => props.tariff?.cost ?? null))

const kwhInput = ref<InstanceType<typeof NumberInput> | null>(null)
defineExpose({ focus: () => kwhInput.value?.focus() })

const quick = computed(() => props.variant === 'quick')
const inputClass = computed(() => (quick.value ? 'quick-input' : 'field'))
const labelClass = computed(() => (quick.value ? 'quick-label' : 'block text-xs font-semibold text-slate-300 mb-1'))
const followsPlan = computed(() => driver.value === 'tariff' && !!props.tariff && cost.value !== '')
const followsTariff = computed(() => !followsPlan.value && driver.value === 'tariff' && props.suggestedPrice !== undefined && cost.value !== '')
</script>

<template>
  <div class="space-y-4">
    <div v-if="!hideEnergy">
      <label :for="`${idPrefix}-kwh`" :class="labelClass">{{ $t('quickadd.quickChargeForm.energyAddedKwh') }}</label>
      <NumberInput text :id="`${idPrefix}-kwh`" v-model="kwh" :min="quick ? '0' : '0.001'" :required="required" ref="kwhInput" :class="inputClass" />
    </div>

    <div>
      <label :for="`${idPrefix}-price`" :class="labelClass">{{ $t('quickadd.quickChargeForm.pricePerKwh', { cur: currencySymbol(currency) }) }}</label>
      <NumberInput text :id="`${idPrefix}-price`" v-model="price" min="0" :class="inputClass" @input="onPriceInput" />
    </div>

    <div>
      <div class="flex items-center justify-between" :class="quick ? '' : 'mb-1'">
        <label :for="`${idPrefix}-cost`" :class="[labelClass, quick ? '' : '!mb-0']">{{ $t('quickadd.quickChargeForm.cost', { cur: currencySymbol(currency) }) }}</label>
        <slot name="cost-action" />
      </div>
      <div class="flex gap-2">
        <NumberInput text :id="`${idPrefix}-cost`" v-model="cost" min="0" :required="required" :class="[inputClass, 'min-w-0']" @input="onCostInput" />
        <button type="button" :class="quick ? 'quick-chip shrink-0 border-slate-700 bg-slate-800 text-slate-200 hover:bg-slate-700' : 'btn btn-secondary shrink-0'" @click="setFree">{{ $t('quickadd.quickChargeForm.free') }}</button>
      </div>
      <p class="mt-1.5 min-h-4 text-xs text-slate-400" aria-live="polite">
        <template v-if="followsPlan">{{ $t('quickadd.quickChargeForm.calculatedWithTariff', { plan: tariff!.plan }) }}</template>
        <template v-else-if="followsTariff">{{ $t('quickadd.quickChargeForm.calculatedAtTheLastRate', { pricePerKwh: formatAmount(suggestedPrice!, currency, 4) }) }}</template>
      </p>
    </div>
  </div>
</template>
