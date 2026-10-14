<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { X, MoreHorizontal } from 'lucide-vue-next'

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

// The "More" popover holds the occasional actions, so the bar keeps one primary action and a few secondary ones
const moreOpen = ref(false)
const moreRoot = ref<HTMLElement | null>(null)

function onKeyDown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (moreOpen.value) {
    moreOpen.value = false
    return
  }
  if (props.count > 0 && !props.disabled) emit('clear')
}

function closeOnAction(e: MouseEvent) {
  if ((e.target as HTMLElement).closest('button')) moreOpen.value = false
}

function onDocumentClick(e: MouseEvent) {
  if (moreOpen.value && moreRoot.value && !moreRoot.value.contains(e.target as Node)) moreOpen.value = false
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
  document.addEventListener('click', onDocumentClick)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeyDown)
  document.removeEventListener('click', onDocumentClick)
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

      <div v-if="$slots.more" ref="moreRoot" class="relative">
        <button
          type="button"
          class="btn btn-secondary"
          :aria-expanded="moreOpen"
          aria-haspopup="menu"
          @click="moreOpen = !moreOpen"
        >
          <MoreHorizontal class="w-3.5 h-3.5" />
          <span>{{ $t('shell.bulkSelectionBar.more') }}</span>
        </button>
        <div
          v-if="moreOpen"
          role="menu"
          class="absolute right-0 top-full z-40 mt-2 w-64 space-y-0.5 rounded-xl border border-slate-700 bg-slate-800 p-1.5 shadow-2xl"
          @click="closeOnAction"
          @change="moreOpen = false"
        >
          <slot name="more" />
        </div>
      </div>

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
