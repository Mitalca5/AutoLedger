<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { computed } from 'vue'
import { Coins, Gauge, Calendar, FileText, CheckCircle2 } from 'lucide-vue-next'
import { buildLeaseSummary } from '@/utils/dashboard'
import { formatAmount } from '@/currency'
import { useVehicleStore } from '@/stores/vehicle'
import { distanceUnit, formatDistance, formatDistanceValue } from '@/units'

// Follow-up of a LOA / LLD contract: duration, mileage against the allowance, costs and included services
const props = defineProps<{ tco: any | null }>()
const vehicleStore = useVehicleStore()
const currency = computed(() => vehicleStore.currency)
const leaseContract = computed(() => buildLeaseSummary(props.tco))
</script>

<template>
  <div
    v-if="leaseContract"
    class="bg-gradient-to-br from-slate-900/90 to-indigo-950/20 border border-indigo-500/20 p-4 sm:p-5 rounded-2xl shadow-sm space-y-4"
  >
    <!-- Card Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800/80">
      <div class="flex items-center gap-2.5 flex-wrap">
        <span class="px-2.5 py-0.5 rounded-full text-xs font-bold uppercase tracking-wider bg-indigo-500/15 text-indigo-400 border border-indigo-500/30 flex items-center gap-1.5">
          <FileText class="w-3.5 h-3.5" />
          {{ $t('dashboard.leaseContractCard.contract', { acquisitionType: leaseContract.acquisitionType }) }}
        </span>
        <span
          class="px-2.5 py-0.5 rounded-full text-xs font-semibold border"
          :class="leaseContract.status.class"
        >
          {{ leaseContract.status.label }}
        </span>
      </div>

      <!-- Monthly rent / downpayment -->
      <div v-if="leaseContract.monthlyRent" class="self-start sm:self-auto text-left sm:text-right">
        <span class="text-base sm:text-lg font-extrabold text-white">
          {{ formatAmount(Number(leaseContract.monthlyRent), currency) }}
        </span>
        <span class="text-xs text-slate-400 font-normal"> {{ $t('dashboard.leaseContractCard.month') }}</span>
        <span v-if="leaseContract.downPayment && leaseContract.downPayment > 0" class="block text-xs text-slate-400">
          {{ $t('dashboard.leaseContractCard.downPayment', { downPayment: formatAmount(Number(leaseContract.downPayment), currency, 0) }) }}
        </span>
      </div>
    </div>

    <!-- Progress Bars Section: 1 col on mobile, 2 cols on desktop if mileage allowance configured -->
    <div
      class="grid gap-4"
      :class="leaseContract.hasMileageAllowance ? 'grid-cols-1 lg:grid-cols-2' : 'grid-cols-1'"
    >
      <!-- Bar 1: Contract Duration Progress -->
      <div class="bg-slate-950/50 border border-slate-800/80 rounded-xl p-3.5 space-y-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="font-semibold text-slate-300 flex items-center gap-1.5">
            <Calendar class="w-3.5 h-3.5 text-indigo-400 shrink-0" />
            {{ $t('dashboard.leaseContractCard.contractDuration') }}
          </span>
          <span class="font-bold text-indigo-300">
            {{ $t('dashboard.leaseContractCard.months', { elapsedMonths: leaseContract.elapsedMonths, totalMonths: leaseContract.totalMonths }) }}
            <span class="text-slate-400 font-normal">({{ leaseContract.durationProgressPct }} %)</span>
          </span>
        </div>

        <!-- Progress Bar Track -->
        <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
          <div
            class="h-full bg-gradient-to-r from-indigo-500 to-violet-500 rounded-full transition-all duration-500"
            :style="{ width: `${leaseContract.durationProgressPct}%` }"
          ></div>
        </div>

        <!-- Sub-info: start date, remaining, end date -->
        <div class="flex items-center justify-between text-xs text-slate-400 flex-wrap gap-1">
          <span v-if="leaseContract.startDate">
            {{ $t('dashboard.leaseContractCard.start', { startDate: leaseContract.startDate.toLocaleDateString(intlLocale(), { day: 'numeric', month: 'short', year: 'numeric' }) }) }}
          </span>
          <span class="font-medium text-slate-300">
            {{ leaseContract.isEnded ? $t('dashboard.leaseContractCard.ended') : leaseContract.isNotStarted ? $t('dashboard.leaseContractCard.upcoming') : $t('dashboard.leaseContractCard.monthsLeft', { count: leaseContract.remainingMonths }) }}
          </span>
          <span v-if="leaseContract.endDate">
            {{ $t('dashboard.leaseContractCard.end', { endDate: leaseContract.endDate.toLocaleDateString(intlLocale(), { day: 'numeric', month: 'short', year: 'numeric' }) }) }}
          </span>
        </div>
      </div>

      <!-- Bar 2: Mileage Allowance Progress (if configured) -->
      <div v-if="leaseContract.hasMileageAllowance" class="bg-slate-950/50 border border-slate-800/80 rounded-xl p-3.5 space-y-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="font-semibold text-slate-300 flex items-center gap-1.5">
            <Gauge class="w-3.5 h-3.5 text-emerald-400 shrink-0" />
            {{ $t('dashboard.leaseContractCard.mileageAllowance') }}
          </span>
          <span class="font-bold text-slate-200">
            {{ formatDistance(leaseContract.kmDriven) }}
            <span v-if="leaseContract.kmAllowanceTotal" class="text-slate-400 font-normal">
              / {{ formatDistance(leaseContract.kmAllowanceTotal) }}
            </span>
            <span v-else class="text-slate-400 font-normal">
              {{ $t('dashboard.leaseContractCard.kmToDate', { unit: distanceUnit(), kmAllowanceToDate: formatDistanceValue(leaseContract.kmAllowanceToDate) }) }}
            </span>
          </span>
        </div>

        <!-- Progress Bar Track -->
        <div class="w-full bg-slate-800 h-2.5 rounded-full overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-500"
            :class="leaseContract.mileageColor"
            :style="{ width: `${Math.min(100, leaseContract.mileageProgressPct)}%` }"
          ></div>
        </div>

        <!-- Sub-info: Pace & Diff -->
        <div class="flex items-center justify-between text-xs flex-wrap gap-1">
          <span class="text-slate-400">
            {{ $t('dashboard.leaseContractCard.pace') }} <strong>{{ $t('dashboard.leaseContractCard.kmMonth', { unit: distanceUnit(), actualPaceKmMonth: formatDistanceValue(leaseContract.actualPaceKmMonth) }) }}</strong>
            <template v-if="leaseContract.contractualPaceKmMonth"> {{ $t('dashboard.leaseContractCard.plannedKmMonth', { unit: distanceUnit(), contractualPaceKmMonth: formatDistanceValue(leaseContract.contractualPaceKmMonth) }) }}</template>
          </span>
          <span
            v-if="leaseContract.kmDiff !== undefined"
            :class="leaseContract.kmDiff > 0 ? 'text-rose-400 font-semibold' : 'text-emerald-400 font-semibold'"
          >
            {{ leaseContract.kmDiff > 0 ? $t('dashboard.leaseContractCard.kmOver', { unit: distanceUnit(), km: formatDistanceValue(leaseContract.kmDiff) }) : $t('dashboard.leaseContractCard.kmAhead', { unit: distanceUnit(), km: formatDistanceValue(Math.abs(leaseContract.kmDiff)) }) }}
          </span>
        </div>
      </div>
    </div>

    <!-- Footer: Buyout option, penalties, included services -->
    <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-800/60 text-xs text-slate-400">
      <div class="flex items-center gap-2 flex-wrap">
        <!-- Purchase option (LOA) -->
        <span
          v-if="leaseContract.purchaseOptionPrice"
          class="px-2.5 py-1 rounded-lg bg-slate-800 text-slate-300 font-medium border border-slate-700/60 flex items-center gap-1.5"
        >
          <Coins class="w-3.5 h-3.5 text-amber-400" />
          {{ $t('dashboard.leaseContractCard.finalPurchaseOption') }} <strong class="text-white">{{ formatAmount(Number(leaseContract.purchaseOptionPrice), currency, 0) }}</strong>
        </span>

        <!-- Included Services Badges -->
        <span
          v-if="leaseContract.includesMaintenance"
          class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-medium flex items-center gap-1"
        >
          <CheckCircle2 class="w-3 h-3" /> {{ $t('dashboard.leaseContractCard.maintenanceIncluded') }}
        </span>
        <span
          v-if="leaseContract.includesInsurance"
          class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-medium flex items-center gap-1"
        >
          <CheckCircle2 class="w-3 h-3" /> {{ $t('dashboard.leaseContractCard.insuranceIncluded') }}
        </span>
        <span
          v-if="leaseContract.includesTires"
          class="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-medium flex items-center gap-1"
        >
          <CheckCircle2 class="w-3 h-3" /> {{ $t('dashboard.leaseContractCard.tiresIncluded') }}
        </span>
      </div>

      <!-- Penalty Warnings if applicable -->
      <div class="flex items-center gap-3">
        <span v-if="leaseContract.excessKmCost > 0" class="text-rose-400 font-semibold">
          {{ $t('dashboard.leaseContractCard.overageToDate', { value: formatAmount(leaseContract.excessKmCost, currency) }) }}
        </span>
        <span v-if="leaseContract.excessKmProjected > 0" class="text-amber-400 font-semibold">
          {{ $t('dashboard.leaseContractCard.estimatedEndOfContractPenalty', { value: formatAmount(leaseContract.excessKmProjected, currency) }) }}
        </span>
      </div>
    </div>
  </div>
</template>
