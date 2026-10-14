<script setup lang="ts">
// The figures of a list under its filters: a count, then one tinted chip per measure.
// The same strip is used by the lists of the app so a total always reads the same way.
export interface SummaryChip {
  label: string
  value: string
  tone?: 'sky' | 'amber' | 'emerald' | 'slate'
}
defineProps<{ chips: SummaryChip[] }>()

const tones = {
  sky: 'bg-sky-500/10 border-sky-500/20 text-sky-300',
  amber: 'bg-amber-500/10 border-amber-500/20 text-amber-300',
  emerald: 'bg-emerald-500/10 border-emerald-500/20 text-emerald-300',
  slate: 'bg-slate-800/70 border-slate-700/70 text-slate-200',
}
</script>

<template>
  <div class="flex flex-wrap items-center gap-2 px-1 text-xs text-slate-400">
    <span v-if="$slots.default" class="font-medium text-slate-300"><slot /></span>
    <span
      v-for="c in chips"
      :key="c.label"
      class="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5"
      :class="tones[c.tone ?? 'slate']"
    >
      <span class="text-slate-400">{{ c.label }}</span>
      <strong class="font-bold">{{ c.value }}</strong>
    </span>
  </div>
</template>
