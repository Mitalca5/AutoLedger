<script setup lang="ts">
import { ref } from 'vue'
import { ChevronDown } from 'lucide-vue-next'

// The informational cards of the dashboard (data quality, next maintenance) in one disclosure. Open on a wide screen,
// closed on a phone, where the charts are what the page is for; the count and the items say what is waiting.
defineProps<{ count: number; items?: string[] }>()
const open = ref(typeof window === 'undefined' || typeof window.matchMedia !== 'function' || window.matchMedia('(min-width: 640px)').matches)
</script>

<template>
  <section>
    <button
      type="button"
      class="flex w-full items-center justify-between gap-2 rounded-xl px-1 py-1.5 text-left text-xs font-semibold uppercase tracking-wider text-slate-400 hover:text-slate-200"
      :aria-expanded="open"
      aria-controls="dashboard-follow-up"
      @click="open = !open"
    >
      <span class="flex flex-wrap items-center gap-x-2 gap-y-0.5">
        <span class="whitespace-nowrap">{{ $t('dashboard.followUp.title') }}</span>
        <span v-if="count > 0" class="rounded-full bg-slate-800 px-2 py-0.5 text-xs font-bold text-slate-200">{{ count }}</span>
        <span v-if="items?.length" class="text-xs font-medium normal-case tracking-normal text-slate-400">{{ items.join(' · ') }}</span>
      </span>
      <ChevronDown class="h-4 w-4 transition-transform" :class="open ? 'rotate-180' : ''" aria-hidden="true" />
    </button>
    <div v-show="open" id="dashboard-follow-up" class="mt-2 space-y-4">
      <slot />
    </div>
  </section>
</template>
