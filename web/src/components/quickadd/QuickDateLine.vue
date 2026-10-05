<script setup lang="ts">
import { computed, ref } from 'vue'
import { intlLocale, t } from '@/i18n'
import { describeQuickDate } from '@/utils/quickAdd'

const props = defineProps<{ id: string; withTime?: boolean }>()
const model = defineModel<string>({ required: true })
const editing = ref(false)

const label = computed(() => describeQuickDate(model.value, !!props.withTime, new Date(), intlLocale(), t('quickadd.dateLine.today')))
</script>

<template>
  <div v-if="editing">
    <label :for="id" class="quick-label">{{ $t('quickadd.dateLine.label') }}</label>
    <input :id="id" v-model="model" :type="withTime ? 'datetime-local' : 'date'" class="quick-input" />
  </div>
  <div v-else class="flex min-h-12 items-center justify-between gap-3 rounded-xl border border-slate-800 bg-slate-900/60 px-4">
    <p class="text-sm text-slate-300">
      <span class="mr-2 text-xs text-slate-400">{{ $t('quickadd.dateLine.label') }}</span>
      <span class="font-semibold text-white">{{ label }}</span>
    </p>
    <button type="button" class="tap-text rounded-lg px-2 text-xs font-semibold text-rose-300 hover:text-rose-200" @click="editing = true">{{ $t('quickadd.dateLine.edit') }}</button>
  </div>
</template>
