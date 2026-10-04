<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  title: string
  icon: Component
}>()
</script>

<template>
  <header class="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
    <div class="min-w-0">
      <div class="flex items-center gap-3">
        <div class="shrink-0 rounded-xl border border-rose-500/20 bg-rose-500/10 p-2 text-rose-400">
          <component :is="icon" class="h-6 w-6" aria-hidden="true" />
        </div>
        <h2 class="text-2xl font-bold tracking-tight text-white">{{ title }}</h2>
      </div>
      <p v-if="$slots.default" class="mt-1 text-sm text-slate-400"><slot /></p>
      <slot name="below" />
    </div>
    <div v-if="$slots.actions" class="flex flex-wrap items-center gap-2 self-start sm:self-center">
      <slot name="actions" />
    </div>
  </header>
</template>
