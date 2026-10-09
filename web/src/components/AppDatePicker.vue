<script setup lang="ts">
import { computed } from 'vue'
import { VueDatePicker } from '@vuepic/vue-datepicker'
import { enGB, fr } from 'date-fns/locale'
import { format } from 'date-fns'
import { currentLocale, t } from '@/i18n'

const props = withDefaults(
  defineProps<{
    modelValue?: string | Date | null | { month: number; year: number }
    enableTimePicker?: boolean
    monthPicker?: boolean
    placeholder?: string
    disabled?: boolean
    required?: boolean
    clearable?: boolean
    id?: string
    size?: 'xs' | 'sm' | 'md'
  }>(),
  {
    modelValue: '',
    enableTimePicker: false,
    monthPicker: false,
    placeholder: '',
    disabled: false,
    required: false,
    clearable: undefined,
    id: undefined,
    size: 'sm',
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'change', value: string): void
}>()

const dateLocale = computed(() => (currentLocale() === 'fr' ? fr : enGB))

const internalValue = computed({
  get: () => {
    if (!props.modelValue) return null
    if (props.monthPicker && typeof props.modelValue === 'string') {
      const parts = props.modelValue.split('-')
      if (parts.length === 2) {
        const y = Number(parts[0])
        const m = Number(parts[1])
        if (!Number.isNaN(y) && !Number.isNaN(m)) {
          return { year: y, month: m - 1 }
        }
      }
    }
    return props.modelValue
  },
  set: (val: any) => {
    if (!val) {
      emit('update:modelValue', '')
      emit('change', '')
      return
    }
    if (props.monthPicker) {
      if (typeof val === 'object' && 'year' in val && 'month' in val) {
        const str = `${val.year}-${String(val.month + 1).padStart(2, '0')}`
        emit('update:modelValue', str)
        emit('change', str)
        return
      }
      if (val instanceof Date && !Number.isNaN(val.getTime())) {
        const str = `${val.getFullYear()}-${String(val.getMonth() + 1).padStart(2, '0')}`
        emit('update:modelValue', str)
        emit('change', str)
        return
      }
    }
    const stringVal = String(val)
    emit('update:modelValue', stringVal)
    emit('change', stringVal)
  },
})

const isClearable = computed(() => {
  if (props.clearable !== undefined) return props.clearable
  return !props.required
})

const timeConfig = computed(() => ({
  enableTimePicker: props.enableTimePicker,
}))

const displayFormat = computed(() => {
  if (props.monthPicker) return 'MMMM yyyy'
  return props.enableTimePicker ? 'dd/MM/yyyy HH:mm' : 'dd/MM/yyyy'
})

const formatsConfig = computed(() => {
  if (props.monthPicker) {
    return {
      input: (d: Date) => {
        const str = format(d, 'MMMM yyyy', { locale: dateLocale.value })
        return str.charAt(0).toUpperCase() + str.slice(1)
      },
      preview: (d: Date) => {
        const str = format(d, 'MMMM yyyy', { locale: dateLocale.value })
        return str.charAt(0).toUpperCase() + str.slice(1)
      },
    }
  }
  const fmt = props.enableTimePicker ? 'dd/MM/yyyy HH:mm' : 'dd/MM/yyyy'
  return {
    input: fmt,
    preview: fmt,
  }
})

const effectivePlaceholder = computed(() => {
  if (props.placeholder) return props.placeholder
  if (props.monthPicker) return t('shell.datePicker.selectMonth')
  return props.enableTimePicker ? t('shell.datePicker.dateTimePlaceholder') : t('shell.datePicker.dayPlaceholder')
})

const actionRowConfig = computed(() => ({
  showNow: true,
  nowBtnLabel: t('shell.datePicker.today'),
  selectBtnLabel: t('shell.datePicker.confirm'),
  cancelBtnLabel: t('common.cancel'),
}))
</script>

<template>
  <div :class="['app-datepicker-wrapper', `size-${size}`, { 'is-disabled': disabled }]">
    <VueDatePicker
      :uid="id"
      v-model="internalValue"
      :dark="true"
      :locale="dateLocale"
      :month-picker="monthPicker"
      :time-config="timeConfig"
      :auto-apply="!enableTimePicker"
      :close-on-auto-apply="!enableTimePicker"
      :model-type="monthPicker ? undefined : (enableTimePicker ? 'yyyy-MM-dd\'T\'HH:mm' : 'yyyy-MM-dd')"
      :format="displayFormat"
      :formats="formatsConfig"
      :placeholder="effectivePlaceholder"
      :disabled="disabled"
      :input-attrs="{ clearable: isClearable }"
      teleport="body"
      :action-row="actionRowConfig"
    />
  </div>
</template>

<style scoped>
.app-datepicker-wrapper {
  width: 100%;
}

.is-disabled {
  pointer-events: none;
}

.is-disabled :deep(.dp--input),
:deep(.dp--disabled) {
  background-color: color-mix(in oklab, var(--color-slate-900) 90%, transparent) !important;
  border-color: var(--color-slate-800) !important;
  color: var(--color-slate-500) !important;
  cursor: not-allowed !important;
  opacity: 0.6 !important;
  box-shadow: none !important;
}

.is-disabled :deep(.dp--input-icon),
:deep(.dp--disabled .dp--input-icon) {
  color: var(--color-slate-600) !important;
}

.size-xs :deep(.dp--input) {
  font-size: 0.75rem !important;
  line-height: 1rem !important;
  padding-top: 0.375rem !important;
  padding-bottom: 0.375rem !important;
  padding-left: 1.85rem !important;
  padding-right: 0.375rem !important;
}

.size-xs :deep(.dp--input-icon) {
  padding-left: 0.5rem !important;
}

.size-sm :deep(.dp--input) {
  font-size: 0.875rem !important;
  line-height: 1.25rem !important;
  padding-top: 0.5rem !important;
  padding-bottom: 0.5rem !important;
}

.size-md :deep(.dp--input) {
  font-size: 1rem !important;
  line-height: 1.5rem !important;
  padding-top: 0.625rem !important;
  padding-bottom: 0.625rem !important;
}
</style>
