<script setup lang="ts">
import { computed } from 'vue'
import { TrendingDown, TrendingUp } from 'lucide-vue-next'
import { intlLocale, t } from '@/i18n'
import { distanceUnit, perDistance } from '@/units'
import { comparisonSavings } from '@/utils/comparisonSavings'

const props = defineProps<{
  result: {
    tracked: { energy: number; total: number }
    ice: { energy: number; total: number }
    years_count: number
    annual_km: number
  }
  currency: string
}>()

const cards = computed(() => (['energy', 'total'] as const).map(key => ({
  key,
  ...comparisonSavings(props.result.tracked[key], props.result.ice[key], props.result.years_count, props.result.annual_km),
})))
const otherCosts = computed(() => cards.value[0]!.amount - cards.value[1]!.amount)
function money(value: number, digits = 2) {
  return Math.abs(value).toLocaleString(intlLocale(), {
    style: 'currency', currency: props.currency, minimumFractionDigits: digits, maximumFractionDigits: digits,
  })
}
function percentage(value: number) {
  return Math.abs(value).toLocaleString(intlLocale(), { maximumFractionDigits: 1 })
}
</script>

<template>
  <div class="space-y-3">
    <div class="grid gap-4 md:grid-cols-2">
      <section v-for="card in cards" :key="card.key" class="rounded-2xl border p-5"
        :class="card.amount >= 0 ? 'bg-success-500/10 border-success-500/30' : 'bg-warning-500/10 border-warning-500/30'">
        <h2 class="flex items-center gap-2 text-sm font-semibold"
          :class="card.amount >= 0 ? 'text-success-300' : 'text-warning-300'">
          <component :is="card.amount >= 0 ? TrendingDown : TrendingUp" class="w-5 h-5 shrink-0" aria-hidden="true" />
          {{ t(`comparison.savings.${card.key}${card.amount < 0 ? 'Extra' : ''}`) }}
        </h2>
        <div class="text-3xl font-bold text-white mt-2">{{ money(card.amount) }}</div>
        <p v-if="card.percent !== null" class="text-sm text-slate-300 mt-1">
          {{ t(card.amount < 0 ? 'comparison.savings.more' : 'comparison.savings.less', { percent: percentage(card.percent) }) }}
        </p>
        <p class="text-xs text-slate-400 mt-2">{{ t(`comparison.savings.${card.key}Help`) }}</p>
        <div class="flex flex-wrap gap-x-3 gap-y-1 text-xs text-slate-300 mt-3">
          <span v-if="card.perMonth !== null">{{ t('comparison.savings.month', { amount: money(card.perMonth) }) }}</span>
          <span v-if="card.perKm !== null">{{ money(perDistance(card.perKm), 3) }}/{{ distanceUnit() }}</span>
        </div>
      </section>
    </div>
    <p v-if="Math.abs(otherCosts) >= 0.01" class="text-xs text-slate-400">
      {{ t(otherCosts > 0 ? 'comparison.savings.reduced' : 'comparison.savings.increased', { amount: money(otherCosts) }) }}
    </p>
  </div>
</template>
