<script setup lang="ts">
import type { Component } from 'vue'
import { computed } from 'vue'
import GlossaryTip from '@/components/GlossaryTip.vue'
import { UNKNOWN_AMOUNT } from '@/currency'

// One headline figure: label, value with its unit, the period it covers, an optional change and a
// definition tip. An unknown value (`null`) reads as a dash, never as 0. Secondary lines go in the default
// slot; `actionLabel` adds the single link or button that opens the detail behind the figure.
export interface KpiDelta {
  text: string
  tone?: 'good' | 'bad' | 'neutral'
}
const props = withDefaults(defineProps<{
  label: string
  value: string | null
  unit?: string
  period?: string
  term?: string
  icon?: Component
  tone?: 'default' | 'sky' | 'success'
  // The dense tile of a panel inside a disclosure: no icon, smaller figure, a darker background.
  compact?: boolean
  delta?: KpiDelta
  actionLabel?: string
  to?: string
}>(), { tone: 'default' })
const emit = defineEmits<{ action: [] }>()
// The app's own unknown marker, as returned by the amount and number formatters, reads as unknown too
const unknown = computed(() => props.value == null || props.value === UNKNOWN_AMOUNT)

const valueTones = { default: 'text-white', sky: 'text-sky-400', success: 'text-success-400' }
const deltaTones = { good: 'text-success-400', bad: 'text-danger-400', neutral: 'text-slate-300' }
const actionClass = "tap-text mt-auto self-start pt-1 text-xs font-semibold text-sky-400 underline after:absolute after:inset-0 after:content-['']"
</script>

<template>
  <div
    class="relative flex flex-col"
    :class="compact ? 'rounded-xl border border-slate-800 bg-slate-950/60 p-3' : 'rounded-2xl border border-slate-800 bg-slate-900 p-4 shadow-sm transition-colors hover:border-slate-600'"
  >
    <div class="flex items-start justify-between gap-2" :class="compact ? '' : 'mb-2 min-h-8'">
      <span class="text-xs font-semibold uppercase text-slate-400" :class="compact ? 'tracking-wide' : 'tracking-wider'">
        {{ label }}
        <span v-if="term" class="relative z-10"><GlossaryTip :term="term" /></span>
        <span v-if="period" class="block font-medium normal-case tracking-normal text-slate-400">{{ period }}</span>
      </span>
      <span v-if="icon && !compact" class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-slate-800 text-slate-300">
        <component :is="icon" class="h-4 w-4" aria-hidden="true" />
      </span>
    </div>
    <div class="font-extrabold tabular-nums" :class="[compact ? 'mt-1 text-xl font-bold' : 'text-2xl', unknown ? 'text-slate-400' : valueTones[tone]]">
      <template v-if="unknown">—</template>
      <template v-else>{{ value }}<span v-if="unit" class="font-normal text-slate-300" :class="compact ? 'text-xs font-medium text-slate-400' : 'text-sm'">{{ unit }}</span></template>
    </div>
    <p v-if="delta" class="mt-1 text-xs font-semibold" :class="deltaTones[delta.tone ?? 'neutral']">{{ delta.text }}</p>
    <slot />
    <router-link v-if="actionLabel && to" :to="to" :class="actionClass">{{ actionLabel }}</router-link>
    <button v-else-if="actionLabel" type="button" :class="actionClass" @click="emit('action')">{{ actionLabel }}</button>
  </div>
</template>
