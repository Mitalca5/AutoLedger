<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Coins, Zap, Receipt, Disc, Wrench, Briefcase, ArrowRight, Activity, Shield, X, ChevronLeft, ChevronRight, PieChart, Info, SlidersHorizontal } from 'lucide-vue-next'
import { buildMonthBreakdown, formatMonthName, type MonthDetailMode } from '@/utils/dashboard'
import CostDonut from '@/components/costs/CostDonut.vue'
import { currencySymbol, formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistance, formatDistanceValue, perDistance } from '@/units'

// Cost detail of one month with a donut chart and the itemized list; ← / → move between months, Esc closes.
// The modal is open while a month is selected.
const props = defineProps<{ monthlyCosts: any[] }>()
const selectedMonth = defineModel<any | null>('month', { required: true })
const vehicleStore = useVehicleStore()
const currency = computed(() => vehicleStore.currency)

const monthDetailViewMode = ref<MonthDetailMode>('economic')

const ITEM_ICONS: Record<string, any> = {
  energy: Zap,
  tolls: Receipt,
  tires: Disc,
  maintenance: Wrench,
  insurance: Shield,
  financing: Briefcase,
  other: Coins,
}

const selectedMonthIndex = computed(() => {
  if (!selectedMonth.value) return -1
  return props.monthlyCosts.findIndex((m: any) => m.month === selectedMonth.value.month)
})

const hasPrevMonth = computed(() => selectedMonthIndex.value > 0)
const hasNextMonth = computed(() => selectedMonthIndex.value >= 0 && selectedMonthIndex.value < props.monthlyCosts.length - 1)

function selectPrevMonth() {
  if (hasPrevMonth.value) {
    selectedMonth.value = props.monthlyCosts[selectedMonthIndex.value - 1]
  }
}

function selectNextMonth() {
  if (hasNextMonth.value) {
    selectedMonth.value = props.monthlyCosts[selectedMonthIndex.value + 1]
  }
}

function closeMonthDetail() {
  selectedMonth.value = null
}

const selectedMonthBreakdown = computed(() => {
  if (!selectedMonth.value) return null
  const breakdown = buildMonthBreakdown(selectedMonth.value, monthDetailViewMode.value, currency.value)
  return { ...breakdown, items: breakdown.items.map((it) => ({ ...it, icon: ITEM_ICONS[it.key] })) }
})

function onKeydown(e: KeyboardEvent) {
  if (!selectedMonth.value) return
  if (e.key === 'ArrowLeft') {
    selectPrevMonth()
  } else if (e.key === 'ArrowRight') {
    selectNextMonth()
  }
}

useEscapeToClose(() => !!selectedMonth.value, closeMonthDetail)

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div
    v-if="selectedMonth && selectedMonthBreakdown"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="closeMonthDetail"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-3xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Header with Month Title, Prev/Next Navigation, and Close Button -->
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between gap-2 shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2 sm:gap-3 min-w-0 pr-2">
          <div class="p-2 bg-indigo-500/10 text-indigo-400 rounded-xl shrink-0">
            <PieChart class="w-5 h-5" />
          </div>
          <div class="min-w-0 truncate">
            <h3 class="text-base sm:text-lg font-bold text-white flex items-center gap-2 truncate">
              <span class="truncate">{{ $t('dashboard.monthDetailModal.costDetails', { value: formatMonthName(selectedMonthBreakdown.month) }) }}</span>
            </h3>
            <p class="text-xs text-slate-400 truncate">
              {{ $t('dashboard.monthDetailModal.fullBreakdownOfTheExpense', { unit: distanceUnit() }) }}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-1 sm:gap-2 shrink-0">
          <div class="flex items-center bg-slate-800/80 rounded-xl border border-slate-700/60 p-0.5">
            <button
              type="button"
              :disabled="!hasPrevMonth"
              @click="selectPrevMonth"
              class="p-1.5 text-slate-400 hover:text-white disabled:opacity-30 disabled:hover:text-slate-400 rounded-lg hover:bg-slate-700/50 transition-colors"
              :title="$t('dashboard.monthDetailModal.previousMonth')"
            >
              <ChevronLeft class="w-4 h-4" />
            </button>
            <button
              type="button"
              :disabled="!hasNextMonth"
              @click="selectNextMonth"
              class="p-1.5 text-slate-400 hover:text-white disabled:opacity-30 disabled:hover:text-slate-400 rounded-lg hover:bg-slate-700/50 transition-colors"
              :title="$t('dashboard.monthDetailModal.nextMonth')"
            >
              <ChevronRight class="w-4 h-4" />
            </button>
          </div>
          <button
            type="button"
            @click="closeMonthDetail"
            class="p-2 text-slate-400 hover:text-white rounded-xl hover:bg-slate-800 transition-colors"
            :title="$t('dashboard.monthDetailModal.closeEsc')"
          >
            <X class="w-5 h-5" />
          </button>
        </div>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

      <!-- 4 KPI Summary Cards for the Month -->
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5 sm:gap-3">
        <div class="bg-slate-800/50 border border-slate-700/50 p-3 rounded-xl">
          <span class="text-xs font-medium text-slate-400 block">{{ $t('dashboard.monthDetailModal.totalDistance') }}</span>
          <div class="text-base sm:text-lg font-extrabold text-white mt-0.5">
            {{ formatDistanceValue(selectedMonthBreakdown.distanceKm) }} <span class="text-xs font-normal text-slate-400">{{ distanceUnit() }}</span>
          </div>
          <span v-if="selectedMonthBreakdown.smoothedKm > 0" class="text-xs text-slate-400 block truncate">
            {{ $t('dashboard.monthDetailModal.ofWhichKmSmoothed', { unit: distanceUnit(), smoothedKm: formatDistanceValue(selectedMonthBreakdown.smoothedKm) }) }}
          </span>
          <span v-else class="text-xs text-slate-400 block truncate">{{ $t('dashboard.monthDetailModal.100GpsDrives') }}</span>
        </div>

        <div class="bg-emerald-500/5 border border-emerald-500/30 p-3 rounded-xl">
          <span class="text-xs font-medium text-emerald-400 block">{{ $t('dashboard.monthDetailModal.costPerKilometre', { unit: distanceUnit() }) }}</span>
          <div class="text-base sm:text-lg font-extrabold text-emerald-400 mt-0.5">
            {{ formatAmount(perDistance(selectedMonthBreakdown.costPerKm), currency, 3) }}<span class="text-xs font-normal text-emerald-500/80">/{{ distanceUnit() }}</span>
          </div>
          <span class="text-xs text-emerald-400/70 block">{{ $t('dashboard.monthDetailModal.actualCostPrice') }}</span>
        </div>

        <div class="bg-slate-800/50 border border-slate-700/50 p-3 rounded-xl">
          <span class="text-xs font-medium text-slate-400 block">{{ $t('dashboard.monthDetailModal.calculatedRunningCost') }}</span>
          <div class="text-base sm:text-lg font-extrabold text-white mt-0.5">
            {{ formatAmount(selectedMonthBreakdown.economicTotal, currency) }}
          </div>
          <span class="text-xs text-slate-400 block">{{ $t('dashboard.monthDetailModal.costPerKmBasis', { unit: distanceUnit() }) }}</span>
        </div>

        <div class="bg-slate-800/50 border border-slate-700/50 p-3 rounded-xl">
          <span class="text-xs font-medium text-slate-400 block">{{ $t('dashboard.monthDetailModal.totalPaidOutCash') }}</span>
          <div class="text-base sm:text-lg font-extrabold text-white mt-0.5">
            {{ formatAmount(selectedMonthBreakdown.cashTotal, currency) }}
          </div>
          <span class="text-xs text-slate-400 block">{{ $t('dashboard.monthDetailModal.paymentsThisMonth') }}</span>
        </div>
      </div>

      <!-- Fixed vs Variable Cost Structure -->
      <div v-if="selectedMonthBreakdown.fixedVar && selectedMonthBreakdown.fixedVar.totalAmount > 0" class="p-3.5 bg-slate-800/40 border border-slate-700/50 rounded-xl space-y-2">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs">
          <span class="font-semibold text-white flex items-center gap-1.5">
            <SlidersHorizontal class="w-3.5 h-3.5 text-indigo-400" />
            <span>{{ $t('dashboard.monthDetailModal.fixedVsVariable') }}</span>
          </span>
          <div class="flex items-center gap-3 text-xs flex-wrap">
            <span class="flex items-center gap-1.5 text-purple-300">
              <span class="w-2 h-2 rounded-full bg-purple-500"></span>
              <span>{{ $t('dashboard.monthDetailModal.fixedCosts') }} : <strong class="text-white">{{ selectedMonthBreakdown.fixedVar.fixedPct }}%</strong> ({{ formatAmount(selectedMonthBreakdown.fixedVar.fixedAmount, currency) }})</span>
            </span>
            <span class="text-slate-400">•</span>
            <span class="flex items-center gap-1.5 text-sky-300">
              <span class="w-2 h-2 rounded-full bg-sky-500"></span>
              <span>{{ $t('dashboard.monthDetailModal.variableCosts') }} : <strong class="text-white">{{ selectedMonthBreakdown.fixedVar.variablePct }}%</strong> ({{ formatAmount(selectedMonthBreakdown.fixedVar.variableAmount, currency) }})</span>
            </span>
          </div>
        </div>

        <!-- Bicolor bar -->
        <div class="w-full h-2.5 bg-slate-800 rounded-full overflow-hidden flex" role="progressbar" :aria-label="$t('dashboard.monthDetailModal.fixedVsVariable')">
          <div
            class="bg-gradient-to-r from-purple-500 to-indigo-500 h-full transition-all duration-300"
            :style="{ width: `${selectedMonthBreakdown.fixedVar.fixedPct}%` }"
            :title="`${$t('dashboard.monthDetailModal.fixedCosts')}: ${selectedMonthBreakdown.fixedVar.fixedPct}%`"
          ></div>
          <div
            class="bg-gradient-to-r from-sky-500 to-teal-400 h-full transition-all duration-300"
            :style="{ width: `${selectedMonthBreakdown.fixedVar.variablePct}%` }"
            :title="`${$t('dashboard.monthDetailModal.variableCosts')}: ${selectedMonthBreakdown.fixedVar.variablePct}%`"
          ></div>
        </div>

        <div class="flex items-center justify-between text-xs text-slate-400 flex-wrap gap-1">
          <span>{{ $t('dashboard.monthDetailModal.fixedCostsSub') }}</span>
          <span>{{ $t('dashboard.monthDetailModal.variableCostsSub') }}</span>
        </div>
      </div>

      <!-- Toggle View Mode: Economic Cost vs Cash-Flow -->
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 p-2.5 bg-slate-800/40 border border-slate-700/50 rounded-xl text-xs">
        <span class="text-slate-300 font-medium flex items-center gap-1.5">
          <Info class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
          <span>{{ $t('dashboard.monthDetailModal.breakdownCalculationMode') }}</span>
        </span>
        <div class="flex items-center gap-1 bg-slate-900/80 p-0.5 rounded-lg border border-slate-700/70">
          <button
            type="button"
            @click="monthDetailViewMode = 'economic'"
            :class="[
              'px-2.5 py-1 text-xs font-semibold rounded-md transition-colors',
              monthDetailViewMode === 'economic' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-white'
            ]"
          >
            {{ $t('dashboard.monthDetailModal.costPriceKm', { unit: distanceUnit(), cur: currencySymbol(currency) }) }}
          </button>
          <button
            type="button"
            @click="monthDetailViewMode = 'cash'"
            :class="[
              'px-2.5 py-1 text-xs font-semibold rounded-md transition-colors',
              monthDetailViewMode === 'cash' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-white'
            ]"
          >
            {{ $t('dashboard.monthDetailModal.cashExpenses', { cur: currencySymbol(currency) }) }}
          </button>
        </div>
      </div>

      <!-- Content: Donut Chart (Left) + Itemized Breakdown List (Right) -->
      <div class="grid grid-cols-1 md:grid-cols-5 gap-6 items-start">
        <!-- Left (2 cols): Donut Chart -->
        <div class="md:col-span-2 bg-slate-800/30 border border-slate-800 rounded-xl p-4 flex flex-col items-center justify-center">
          <h4 class="text-xs font-bold text-white mb-2 self-start flex items-center gap-1.5">
            <PieChart class="w-3.5 h-3.5 text-indigo-400" />
            <span>{{ $t('dashboard.monthDetailModal.breakdownOfTheMonth') }}</span>
          </h4>
          <div class="w-full h-56 sm:h-64 relative">
            <CostDonut
              :items="selectedMonthBreakdown.items.map((it) => ({ label: it.label, color: it.color, amount: it.displayAmount }))"
              :empty-label="$t('dashboard.monthDetailModal.noExpense')"
              :chart-label="$t('dashboard.monthDetailModal.costBreakdownOfTheSelected')"
              :currency="currency"
            />
          </div>
        </div>

        <!-- Right (3 cols): Itemized Table / List -->
        <div class="md:col-span-3 space-y-2">
          <h4 class="text-xs font-bold text-white mb-2 flex items-center justify-between">
            <span>{{ $t('dashboard.monthDetailModal.figuresByExpenseCategory') }}</span>
            <span class="text-xs text-slate-400 font-normal">
              {{ monthDetailViewMode === 'economic' ? $t('dashboard.monthDetailModal.smoothingIncluded') : $t('dashboard.monthDetailModal.cashAmounts') }}
            </span>
          </h4>

          <div class="space-y-1.5 max-h-72 sm:max-h-80 overflow-y-auto pr-1">
            <div
              v-for="item in selectedMonthBreakdown.items"
              :key="item.key"
              class="p-2.5 bg-slate-800/50 border border-slate-700/40 rounded-xl flex items-center justify-between gap-3 text-xs hover:border-slate-600 transition-colors"
            >
              <div class="flex items-center gap-2.5 min-w-0">
                <span
                  class="w-2.5 h-2.5 rounded-full shrink-0"
                  :style="{ backgroundColor: item.color }"
                ></span>
                <component :is="item.icon" class="w-4 h-4 shrink-0 text-slate-400" />
                <div class="min-w-0">
                  <div class="flex items-center gap-1.5">
                    <span class="font-semibold text-white truncate">{{ item.label }}</span>
                    <span v-if="item.subLabel" class="text-xs px-1.5 py-0.2 bg-slate-700 text-slate-300 rounded font-normal">
                      {{ item.subLabel }}
                    </span>
                  </div>
                  <p v-if="item.note" class="text-xs text-slate-400 truncate">
                    {{ item.note }}
                  </p>
                </div>
              </div>

              <div class="text-right shrink-0">
                <div class="flex items-baseline justify-end gap-2">
                  <span class="font-bold text-white text-xs sm:text-sm">
                    {{ formatAmount(item.displayAmount, currency) }}
                  </span>
                  <span class="text-xs text-slate-400 font-medium">
                    ({{ item.sharePct.toFixed(1) }}%)
                  </span>
                </div>
                <div class="text-xs text-emerald-400 font-medium">
                  {{ formatAmount(perDistance(item.costPerKm), currency, 3) }}/{{ distanceUnit() }}
                </div>
              </div>
            </div>
          </div>

          <!-- Total Row -->
          <div class="p-3 bg-indigo-500/10 border border-indigo-500/30 rounded-xl flex items-center justify-between text-xs mt-2">
            <div class="font-bold text-white flex items-center gap-2">
              <span>{{ $t('dashboard.monthDetailModal.monthTotal') }}</span>
              <span class="text-xs text-slate-400 font-normal">({{ formatDistance(selectedMonthBreakdown.distanceKm) }})</span>
            </div>
            <div class="text-right">
              <div class="font-extrabold text-white text-sm sm:text-base">
                {{ formatAmount(selectedMonthBreakdown.activeTotal, currency) }}
              </div>
              <div class="text-xs font-bold text-emerald-400">
                {{ formatAmount(perDistance(selectedMonthBreakdown.distanceKm > 0 ? selectedMonthBreakdown.activeTotal / selectedMonthBreakdown.distanceKm : 0), currency, 3) }}/{{ distanceUnit() }}
              </div>
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- Footer with Quick Links and Close Button -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-3 shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2">
          <router-link
            to="/drives"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white rounded-xl text-xs font-semibold border border-slate-700 transition-colors flex items-center gap-1.5"
          >
            <Activity class="w-3.5 h-3.5 text-indigo-400" />
            <span>{{ $t('dashboard.monthDetailModal.vehicleDrives') }}</span>
          </router-link>
          <router-link
            to="/expenses"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white rounded-xl text-xs font-semibold border border-slate-700 transition-colors flex items-center gap-1.5"
          >
            <Receipt class="w-3.5 h-3.5 text-amber-400" />
            <span>{{ $t('dashboard.monthDetailModal.expensesAndInvoices') }}</span>
          </router-link>
        </div>

        <button
          type="button"
          @click="closeMonthDetail"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors self-end sm:self-auto"
        >
          {{ $t('common.close') }}
        </button>
      </div>
    </div>
  </div>
</template>
