<script setup lang="ts">
import type { Component } from 'vue'

// What an empty list looks like everywhere: a title saying what is missing, a sentence on what goes
// here (default slot), one primary action and the other ways to fill it (`actions` slot). `compact` is
// for a list inside a panel: no icon, tighter padding.
defineProps<{ icon?: Component; title?: string; compact?: boolean }>()
</script>

<template>
  <div class="rounded-2xl border border-slate-800 text-center" :class="compact ? 'bg-slate-950/40 px-4 py-6' : 'bg-slate-900 p-8'">
    <div v-if="icon && !compact" class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl border border-slate-700 bg-slate-800 text-slate-400">
      <component :is="icon" class="h-6 w-6" />
    </div>
    <h3 v-if="title" class="font-bold text-white" :class="compact ? 'text-sm' : 'text-base'">{{ title }}</h3>
    <div v-if="$slots.default" class="mx-auto max-w-md text-sm text-slate-400" :class="title ? 'mt-1' : ''">
      <slot />
    </div>
    <div v-if="$slots.actions" class="mt-4 space-y-3">
      <slot name="actions" />
    </div>
  </div>
</template>
