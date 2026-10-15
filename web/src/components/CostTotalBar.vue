<script setup lang="ts">
/**
 * The "total cost + cost per distance" bar closing a cost breakdown.
 * `strong` brightens the border (the trip costs the driver nothing); the default slot adds
 * detail below the headline figures, separated by the bar's own spacing.
 */
withDefaults(defineProps<{
  /** Total cost, already formatted with its currency. */
  total: string
  /** Cost per distance unit, already formatted (with the unit when it should show). */
  perDistance: string
  /** Label for the per-distance figure, e.g. "Cost per km". */
  perDistanceLabel: string
  totalLabel: string
  strong?: boolean
}>(), { strong: false })
</script>

<template>
  <div
    class="bg-gradient-to-r from-slate-800 to-slate-800/80 border p-4 rounded-2xl space-y-3 shadow-lg"
    :class="strong ? 'border-success-500/40' : 'border-success-500/30'"
  >
    <div class="flex items-center justify-between gap-3">
      <div class="min-w-0">
        <span class="text-xs font-semibold text-success-400 uppercase tracking-wider">{{ totalLabel }}</span>
        <div class="text-2xl font-black text-white" data-testid="cost-total">{{ total }}</div>
      </div>
      <div class="text-right shrink-0">
        <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ perDistanceLabel }}</span>
        <div class="text-lg font-extrabold text-success-400 font-mono" data-testid="cost-per-distance">{{ perDistance }}</div>
      </div>
    </div>
    <slot />
  </div>
</template>
