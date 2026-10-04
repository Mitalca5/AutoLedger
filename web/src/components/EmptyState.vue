<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{ icon?: Component; title?: string }>()
</script>

<template>
  <div class="rounded-2xl border border-slate-800 bg-slate-900 p-8 text-center">
    <div v-if="icon" class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-2xl border border-slate-700 bg-slate-800 text-slate-400">
      <component :is="icon" class="h-6 w-6" />
    </div>
    <h3 v-if="title" class="text-base font-bold text-white">{{ title }}</h3>
    <div class="mx-auto max-w-md text-sm text-slate-400" :class="title ? 'mt-1' : ''">
      <slot />
    </div>
    <div v-if="$slots.actions" class="mt-4 space-y-3">
      <slot name="actions" />
    </div>
  </div>
</template>
