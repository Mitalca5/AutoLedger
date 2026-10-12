<script setup lang="ts">
import { computed } from 'vue'
import { VueDatePicker } from '@vuepic/vue-datepicker'
import { enGB, fr } from 'date-fns/locale'
import { currentLocale, t } from '@/i18n'

// Two days picked on one calendar: v-model is [from, to] as YYYY-MM-DD, or null when no range is chosen.
const props = withDefaults(
  defineProps<{
    modelValue?: [string, string] | null
    placeholder?: string
    disabled?: boolean
    id?: string
    /** Latest selectable day (YYYY-MM-DD); defaults to no limit. */
    max?: string
  }>(),
  { modelValue: null, placeholder: '', disabled: false, id: undefined, max: undefined },
)
const emit = defineEmits<{ 'update:modelValue': [value: [string, string] | null] }>()

const dateLocale = computed(() => (currentLocale() === 'fr' ? fr : enGB))

const internal = computed({
  get: () => (props.modelValue ? [...props.modelValue] : null),
  set: (val: unknown) => {
    if (Array.isArray(val) && val.length === 2 && val[0] && val[1]) {
      emit('update:modelValue', [String(val[0]), String(val[1])])
      return
    }
    emit('update:modelValue', null)
  },
})

const maxDate = computed(() => (props.max ? new Date(`${props.max}T00:00:00`) : undefined))
const actionRow = computed(() => ({
  selectBtnLabel: t('shell.datePicker.confirm'),
  cancelBtnLabel: t('common.cancel'),
}))
</script>

<template>
  <div :class="['app-daterange-wrapper', { 'is-disabled': disabled }]">
    <VueDatePicker
      :uid="id"
      v-model="internal"
      range
      :dark="true"
      :locale="dateLocale"
      model-type="yyyy-MM-dd"
      :time-config="{ enableTimePicker: false }"
      :max-date="maxDate"
      :formats="{ input: 'dd/MM/yyyy', preview: 'dd/MM/yyyy' }"
      :placeholder="placeholder || $t('shell.datePicker.rangePlaceholder')"
      :disabled="disabled"
      :input-attrs="{ clearable: true }"
      teleport="body"
      :action-row="actionRow"
    />
  </div>
</template>

<style scoped>
.app-daterange-wrapper {
  min-width: 13rem;
}

.app-daterange-wrapper :deep(.dp--input) {
  font-size: 0.75rem !important;
  line-height: 1rem !important;
  padding-top: 0.375rem !important;
  padding-bottom: 0.375rem !important;
  padding-left: 1.85rem !important;
  padding-right: 0.375rem !important;
}

.app-daterange-wrapper :deep(.dp--input-icon) {
  padding-left: 0.5rem !important;
}

.is-disabled {
  pointer-events: none;
  opacity: 0.6;
}
</style>
