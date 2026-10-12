<script setup lang="ts">
import { computed } from 'vue'
import { Info } from 'lucide-vue-next'
import { t } from '@/i18n'

// Definition of a cost or consumption term (locales glossary.json), shown on hover and read by screen readers.
const props = defineProps<{ term: string }>()
const label = computed(() => t(`glossary.${props.term}.label`))
const definition = computed(() => t(`glossary.${props.term}.definition`))
</script>

<template>
  <span
    class="inline-flex align-middle text-slate-500 hover:text-slate-300 focus-visible:text-slate-300"
    tabindex="0"
    role="note"
    :title="definition"
    :aria-label="`${label}: ${definition}`"
  >
    <Info class="h-3.5 w-3.5" aria-hidden="true" />
  </span>
</template>
