<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { computed } from 'vue'
import { ArrowRight, Calendar, PieChart } from 'lucide-vue-next'
import { currentMonthStats as buildCurrentMonthStats } from '@/utils/dashboard'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { distanceUnit, formatDistanceValue, perDistance } from '@/units'

// Highlight of the current month; opens its cost detail
const props = defineProps<{ monthlyCosts: any[] | undefined }>()
const vehicleStore = useVehicleStore()
const emit = defineEmits<{ 'open-month': [month: any] }>()
const currentMonthStats = computed(() => buildCurrentMonthStats(props.monthlyCosts))
</script>

<template>
  <div v-if="currentMonthStats" class="bg-slate-900/80 border border-indigo-500/30 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div class="flex items-center gap-3 min-w-0 flex-1">
      <div class="p-2.5 bg-indigo-500/10 text-indigo-400 rounded-xl shrink-0">
        <Calendar class="w-5 h-5" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 flex-wrap">
          <span class="text-xs font-semibold text-indigo-400 uppercase tracking-wider">{{ $t('dashboard.currentMonthBanner.activityThisMonth', { month: currentMonthStats.month }) }}</span>
        </div>
        <div class="text-base sm:text-lg font-bold text-white flex items-center gap-x-3 gap-y-0.5 mt-0.5 flex-wrap">
          <span>{{ $t('dashboard.currentMonthBanner.kmDriven', { unit: distanceUnit(), distance_km: formatDistanceValue(currentMonthStats.distance_km) }) }}</span>
          <span class="text-slate-400 hidden sm:inline">•</span>
          <span class="text-success-400">{{ formatAmount(perDistance(currentMonthStats.cost_per_km > 0 ? currentMonthStats.cost_per_km : 0), vehicleStore.currency, 3) }}/{{ distanceUnit() }}</span>
          <span class="text-slate-400 hidden sm:inline">•</span>
          <span class="text-slate-300">{{ $t('dashboard.currentMonthBanner.spent', { value: formatAmount(currentMonthStats.total, vehicleStore.currency) }) }}</span>
          <template v-if="currentMonthStats.fixedVar && currentMonthStats.fixedVar.totalAmount > 0">
            <span class="text-slate-400 hidden sm:inline">•</span>
            <span class="text-xs font-semibold text-slate-300 bg-slate-800/80 px-2 py-0.5 rounded-lg border border-slate-700/60">
              {{ $t('dashboard.currentMonthBanner.fixedVariableRatio', { fixedPct: currentMonthStats.fixedVar.fixedPct, variablePct: currentMonthStats.fixedVar.variablePct }) }}
            </span>
          </template>
        </div>
      </div>
    </div>
    <div class="flex items-center gap-2 self-start sm:self-auto">
      <button
        type="button"
        @click="emit('open-month', currentMonthStats.raw)"
        class="px-3 py-1.5 bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 border border-indigo-500/30 rounded-xl text-xs font-semibold flex items-center gap-1.5 transition-colors"
      >
        <PieChart class="w-3.5 h-3.5" />
        <span>{{ $t('dashboard.currentMonthBanner.viewTheBreakdown') }}</span>
      </button>
      <router-link
        to="/drives"
        class="tap-text hidden sm:inline-flex text-xs font-semibold text-slate-400 hover:text-white gap-1 px-2"
      >
        {{ $t('dashboard.currentMonthBanner.drives') }} <ArrowRight class="w-3.5 h-3.5" />
      </router-link>
    </div>
  </div>
</template>
