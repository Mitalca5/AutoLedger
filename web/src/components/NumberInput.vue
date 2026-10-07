<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { t } from '@/i18n'
import { formatDecimalInput, parseDecimal } from '@/utils/numbers'

// A decimal field that follows the app's language rather than the browser's: "16,5" and "16.5" both work in
// French, and the value is shown with the language's separator. Other attributes (id, class, required,
// placeholder, disabled...) reach the <input>; `min` and `max` are checked through the browser's own form validation.
const model = defineModel<number | string | null | undefined>()
const props = withDefaults(
  defineProps<{
    // The id its <label for> points at
    id?: string
    // The form keeps the field as text (v-model without .number): the value is emitted as a string
    text?: boolean
    min?: number | string
    max?: number | string
  }>(),
  { text: false },
)

const input = ref<HTMLInputElement | null>(null)
const draft = ref('')
let lastEmitted: unknown = Symbol('none')

const asText = (v: unknown) => (v === '' || v === null || v === undefined || Number.isNaN(Number(v)) ? '' : formatDecimalInput(Number(v)))

watch(
  model,
  (v) => {
    if (v !== lastEmitted) draft.value = asText(v)
  },
  { immediate: true },
)

function problem(raw: string): string {
  if (raw.trim() === '') return ''
  const n = parseDecimal(raw)
  if (Number.isNaN(n)) return t('common.numberInvalid')
  if (props.min !== undefined && props.min !== '' && n < Number(props.min)) return t('common.numberMin', { min: formatDecimalInput(Number(props.min)) })
  if (props.max !== undefined && props.max !== '' && n > Number(props.max)) return t('common.numberMax', { max: formatDecimalInput(Number(props.max)) })
  return ''
}

const validate = () => input.value?.setCustomValidity(problem(draft.value))
onMounted(validate)
watch(() => [props.min, props.max], validate)

function onInput(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  draft.value = raw
  const n = parseDecimal(raw)
  const value: number | string = Number.isNaN(n) ? (props.text ? raw : '') : props.text ? String(n) : n
  lastEmitted = value
  model.value = value
  validate()
}

defineExpose({ focus: () => input.value?.focus(), select: () => input.value?.select(), el: input })
</script>

<template>
  <input :id="id" ref="input" type="text" inputmode="decimal" autocomplete="off" :value="draft" @input="onInput" />
</template>
