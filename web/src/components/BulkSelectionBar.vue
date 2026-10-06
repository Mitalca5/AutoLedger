<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { X } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    count: number
    // What is selected: picks the wording of the counter
    noun?: 'item' | 'tire' | 'carpool' | 'drive'
    offScreenCount?: number
    metricsSummary?: string
    disabled?: boolean
  }>(),
  {
    noun: 'item',
    offScreenCount: 0,
    disabled: false,
  }
)

const emit = defineEmits<{
  (e: 'clear'): void
}>()

function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.count > 0 && !props.disabled) {
    emit('clear')
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
})
</script>

<template>
  <div
    v-if="count > 0"
    class="sticky top-16 z-30 bg-slate-800/95 backdrop-blur-md border border-slate-700/80 p-3 rounded-2xl flex flex-wrap items-center justify-between gap-3 shadow-2xl transition-all"
  >
    <!-- Left info & summary -->
    <div class="flex items-center gap-2.5 flex-wrap min-w-0" aria-live="polite">
      <span class="px-2.5 py-0.5 bg-rose-500/20 text-rose-400 font-bold text-xs rounded-lg border border-rose-500/30">
        {{ count }}
      </span>
      <span class="text-xs sm:text-sm font-semibold text-slate-200">
        {{ $t(`shell.bulkSelectionBar.selected.${noun}`, count) }}
      </span>

      <!-- Optional metrics preview (e.g. unified trip metrics) -->
      <span
        v-if="metricsSummary"
        class="hidden sm:inline-flex items-center text-xs text-slate-400 border-l border-slate-700/80 pl-2.5 ml-0.5 font-medium"
      >
        {{ metricsSummary }}
      </span>

      <!-- Off-page indicator -->
      <span v-if="offScreenCount > 0" class="text-xs text-slate-400">
        {{ $t('shell.bulkSelectionBar.outOfView', { offScreenCount }) }}
      </span>
    </div>

    <!-- Right contextual actions + Clear button -->
    <div class="flex items-center gap-2 flex-wrap shrink-0">
      <slot />

      <button
        type="button"
        @click="emit('clear')"
        :disabled="disabled"
        class="btn btn-secondary"
        :title="$t('shell.bulkSelectionBar.clearTheSelectionEsc')"
      >
        <X class="w-3.5 h-3.5" />
        <span>{{ $t('common.cancel') }}</span>
      </button>
    </div>
  </div>
</template>
