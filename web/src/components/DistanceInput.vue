<script setup lang="ts">
import { computed } from 'vue'
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

const shown = computed(() => {
  const v = model.value
  if (v === lastEmitted) return lastRaw
  if (v === '' || v === null || v === undefined || Number.isNaN(Number(v))) return ''
  const factor = 10 ** props.digits
  return Math.round(toDisplay(Number(v)) * factor) / factor
})

function onInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  let value: number | string
  if (raw === '' || Number.isNaN(Number(raw))) {
    value = props.text ? raw : ''
  } else {
    const converted = toApi(Number(raw))
    // Enough precision for a round trip; integer API fields get a whole number
    const rounded = props.whole ? Math.round(converted) : Math.round(converted * 10000) / 10000
    value = props.text ? String(rounded) : rounded
  }
  lastRaw = raw
  lastEmitted = value
  model.value = value
}
</script>

<template>
  <input :id="id" type="number" :value="shown" :min="displayMin" :max="displayMax" @input="onInput" />
</template>
