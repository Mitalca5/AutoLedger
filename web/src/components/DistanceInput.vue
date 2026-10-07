<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { t } from '@/i18n'
import { formatDecimalInput, parseDecimal } from '@/utils/numbers'
import { displayDistanceToKm, kmToDisplayDistance, perDistance, perDistanceToPerKm } from '@/units'

// A number input whose model stays in the API's unit (km, or a figure per km) while the user reads and types
// in the account's distance unit. Other attributes (id, class, min, step, placeholder...) reach the <input>.
const model = defineModel<number | string | null | undefined>()
const props = withDefaults(
  defineProps<{
    // The id its <label for> points at
    id?: string
    // 'distance': an odometer or a length; 'per-distance': a figure per km (kWh/100 km, a price per km)
    kind?: 'distance' | 'per-distance'
    // Decimals shown in the field
    digits?: number
    // The API field is an integer: the converted value is rounded to a whole number
    whole?: boolean
    // The form keeps the field as text (v-model without .number): the converted value is emitted as a string
    text?: boolean
    // Bounds expressed in the API's unit (km or per km), converted to the display unit for the <input>
    min?: number | string
    max?: number | string
  }>(),
  { kind: 'distance', digits: 1, whole: false, text: false },
)

const toDisplay = (v: number) => (props.kind === 'distance' ? kmToDisplayDistance(v) : perDistance(v))
const toApi = (v: number) => (props.kind === 'distance' ? displayDistanceToKm(v) : perDistanceToPerKm(v))

const displayMin = computed(() => {
  if (props.min === undefined || props.min === '') return undefined
  const v = toDisplay(Number(props.min))
  if (Number.isNaN(v)) return undefined
  const factor = 10 ** props.digits
  return props.whole ? Math.round(v) : Math.round(v * factor) / factor
})

const displayMax = computed(() => {
  if (props.max === undefined || props.max === '') return undefined
  const v = toDisplay(Number(props.max))
  if (Number.isNaN(v)) return undefined
  const factor = 10 ** props.digits
  return props.whole ? Math.round(v) : Math.round(v * factor) / factor
})

// While the model is the value this field just emitted, the text stays as typed (rounding it would
// rewrite the field under the cursor); any other value (loaded, reset) is shown rounded.
let lastRaw = ''
let lastEmitted: unknown = Symbol('none')

const input = ref<HTMLInputElement | null>(null)

const shown = computed(() => {
  const v = model.value
  if (v === lastEmitted) return lastRaw
  if (v === '' || v === null || v === undefined || Number.isNaN(Number(v))) return ''
  const factor = 10 ** props.digits
  return formatDecimalInput(Math.round(toDisplay(Number(v)) * factor) / factor)
})

function problem(raw: string): string {
  if (raw.trim() === '') return ''
  const n = parseDecimal(raw)
  if (Number.isNaN(n)) return t('common.numberInvalid')
  if (displayMin.value !== undefined && n < displayMin.value) return t('common.numberMin', { min: formatDecimalInput(displayMin.value) })
  if (displayMax.value !== undefined && n > displayMax.value) return t('common.numberMax', { max: formatDecimalInput(displayMax.value) })
  return ''
}

const validate = () => input.value?.setCustomValidity(problem(input.value.value))
onMounted(validate)
watch([shown, displayMin, displayMax], () => input.value && void Promise.resolve().then(validate))

function onInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  const parsed = parseDecimal(raw)
  let value: number | string
  if (Number.isNaN(parsed)) {
    value = props.text ? raw : ''
  } else {
    const converted = toApi(parsed)
    // Enough precision for a round trip; integer API fields get a whole number
    const rounded = props.whole ? Math.round(converted) : Math.round(converted * 10000) / 10000
    value = props.text ? String(rounded) : rounded
  }
  lastRaw = raw
  lastEmitted = value
  model.value = value
  validate()
}
</script>

<template>
  <input :id="id" ref="input" type="text" inputmode="decimal" autocomplete="off" :value="shown" @input="onInput" />
</template>
