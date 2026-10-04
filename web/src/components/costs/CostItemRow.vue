<script setup lang="ts">
import type { Component } from 'vue'
import { distanceUnit, perDistance } from '@/units'
import { formatAmount } from '@/currency'

// One cost of a breakdown: an icon, its label (with badges and a formula line if any), its amount, its share of the
// total and its cost per km. Shared by the drive, trip and carpool details.
type Tone = 'sky' | 'emerald' | 'pink' | 'purple' | 'amber' | 'slate'
defineProps<{ icon: Component; tone: Tone; label: string; sub?: string; amount: number; sharePct: number; costPerKm: number; currency: string }>()

// Full class names, so Tailwind sees them
const TONES: Record<Tone, { box: string; text: string }> = {
  sky: { box: 'bg-info-500/10 text-info-400', text: 'text-info-400' },
  emerald: { box: 'bg-success-500/10 text-success-400', text: 'text-success-400' },
  pink: { box: 'bg-pink-500/10 text-pink-400', text: 'text-pink-400' },
  purple: { box: 'bg-purple-500/10 text-purple-400', text: 'text-purple-400' },
  amber: { box: 'bg-warning-500/10 text-warning-400', text: 'text-warning-400' },
  slate: { box: 'bg-slate-500/10 text-slate-300', text: 'text-slate-300' },
}
</script>

<template>
  <div class="bg-slate-800/40 border border-slate-800 p-3 rounded-xl flex items-center justify-between">
    <div class="flex items-center gap-3">
      <div class="p-2 rounded-lg" :class="TONES[tone].box">
        <component :is="icon" class="w-4 h-4" />
      </div>
      <div>
        <div class="text-xs font-semibold text-white flex items-center gap-1.5">
          {{ label }}
          <slot name="badge" />
        </div>
        <div v-if="sub" class="text-xs text-slate-400 font-mono">{{ sub }}</div>
      </div>
    </div>
    <div class="text-right">
      <div class="text-sm font-bold font-mono" :class="TONES[tone].text">{{ formatAmount(amount, currency) }}</div>
      <div class="text-xs text-slate-400 font-normal font-sans">({{ sharePct.toFixed(1) }}%) · <span class="text-success-400">{{ formatAmount(perDistance(costPerKm), currency, 3) }}/{{ distanceUnit() }}</span></div>
    </div>
  </div>
</template>
