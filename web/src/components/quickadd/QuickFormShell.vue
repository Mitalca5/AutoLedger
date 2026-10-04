<script setup lang="ts">
// Scrollable fields with a submit button pinned at the bottom of the sheet, within thumb reach.
defineProps<{
  submitLabel: string
  saving?: boolean
  error?: string
}>()

defineEmits<{ submit: [] }>()
</script>

<template>
  <form class="flex min-h-0 flex-1 flex-col" novalidate @submit.prevent="$emit('submit')">
    <div class="min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain px-4 py-4">
      <slot />
    </div>
    <div class="shrink-0 space-y-2 border-t border-slate-800 px-4 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
      <p v-if="error" role="alert" class="rounded-lg border border-danger-500/30 bg-danger-500/10 px-3 py-2 text-xs text-danger-300">
        {{ error }}
      </p>
      <button
        type="submit"
        :disabled="saving"
        class="min-h-14 w-full rounded-xl bg-rose-600 text-base font-bold text-white shadow-lg shadow-rose-600/20 transition-colors hover:bg-rose-500 disabled:opacity-60"
      >
        {{ saving ? $t('quickadd.quickFormShell.saving') : submitLabel }}
      </button>
    </div>
  </form>
</template>
