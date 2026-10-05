<script setup lang="ts">
withDefaults(defineProps<{ rows?: number; label?: string }>(), { rows: 3 })
</script>

<template>
  <div class="space-y-3 animate-pulse" role="status" aria-busy="true">
    <span class="sr-only">{{ label ?? $t('common.loading') }}</span>
    <div
      v-for="i in rows"
      :key="i"
      class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex items-center justify-between gap-4 min-h-[5.25rem]"
      aria-hidden="true"
    >
      <div class="flex items-center gap-3 w-2/3">
        <div class="w-9 h-9 bg-slate-800 rounded-xl shrink-0"></div>
        <div class="space-y-2.5 w-full">
          <div class="h-3.5 w-1/2 bg-slate-800 rounded"></div>
          <div class="h-3 w-3/4 bg-slate-800/70 rounded"></div>
        </div>
      </div>
      <div class="h-5 w-16 bg-slate-800 rounded"></div>
    </div>
  </div>
</template>
