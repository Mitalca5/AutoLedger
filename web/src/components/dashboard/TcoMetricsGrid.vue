<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { computed, ref } from 'vue'
import { canCharge, canRefuel, isFuelOnly } from '@/utils/vehicles'
import { useVehicleStore } from '@/stores/vehicle'
import { formatAmount, formatAmountOrUnknown } from '@/currency'
import GlossaryTip from '@/components/GlossaryTip.vue'
import { currentMonthStats as buildCurrentMonthStats, formatMonthName, rollingWindowStats } from '@/utils/dashboard'
import { formatCostPerDistance } from '@/utils/costPerDistance'
import { Coins, Zap, Calendar, TrendingUp, ChevronDown } from 'lucide-vue-next'
import { distanceUnit, formatDistanceValue, formatFuelConsumptionValue, fuelConsumptionUnit, litresToDisplayVolume, perDistance, perVolume, volumeUnitLabel } from '@/units'

// The four figures the page opens on, each with the period it covers; the other totals sit in the details below
const props = defineProps<{ tco: any | null }>()
const emit = defineEmits<{ 'open-month': [month: any] }>()
const vehicleStore = useVehicleStore()
// Every TCO figure is in the vehicle's own currency
const money = (v: number, digits = 2) => formatAmount(v || 0, vehicleStore.currency, digits)
// A headline amount drops its cents from 1 000 on: they are noise next to the digits that matter
const headline = (v: number) => money(v, Math.abs(v || 0) >= 1000 ? 0 : 2)
const detailsRef = ref<HTMLDetailsElement | null>(null)
const showDetails = () => {
  if (detailsRef.value) detailsRef.value.open = true
  detailsRef.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
}
// The cost per 100 distance units lives in the energy summary: one definition, not a second figure on this card
const showEnergyPanel = () => {
  const panel = document.getElementById('energy-efficiency')
  if (panel instanceof HTMLDetailsElement) panel.open = true
  panel?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
// The last calendar months first: a recent change shows there, the since-purchase figures stay as secondary text
const rolling = computed(() => rollingWindowStats(props.tco?.monthly_costs))
const month = computed(() => buildCurrentMonthStats(props.tco?.monthly_costs, new Date(), props.tco?.current_month))
// A hybrid's energy is its charges and its fill-ups: the label names both
const energyLabel = computed(() => {
  const powertrain = props.tco?.powertrain
  if (isFuelOnly(powertrain)) return t('dashboard.tcoMetricsGrid.fuel')
  return canRefuel(powertrain) ? t('dashboard.tcoMetricsGrid.energyHybrid') : t('dashboard.tcoMetricsGrid.energy')
})
// Unknown (no distance yet) reads as a dash, not as 0,00
const perUnit = (v: number | null | undefined) => formatAmountOrUnknown(v == null ? null : perDistance(v), vehicleStore.currency, 3)
// Economic cost per km over the window; the since-purchase full cost when the monthly rows are missing
const costPerKm = computed(() => (rolling.value ? rolling.value.economicPerKm : (props.tco?.full_cost_per_km ?? null)))
</script>

<template>
  <section class="space-y-3">
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
      <!-- Economic cost per km -->
      <div class="relative flex flex-col bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm transition-colors hover:border-slate-600">
        <div class="flex min-h-8 items-start justify-between gap-2 mb-2">
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">
            {{ rolling ? $t('dashboard.tcoMetricsGrid.economicPerKm', { unit: distanceUnit() }) : $t('dashboard.tcoMetricsGrid.fullCostPerKm', { unit: distanceUnit() }) }}
            <span class="relative z-10"><GlossaryTip term="economicCost" /></span>
            <span v-if="rolling" class="block font-medium normal-case tracking-normal text-slate-400">{{ $t('dashboard.tcoMetricsGrid.windowMonths', { months: rolling.months }, rolling.months) }}</span>
          </span>
          <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-success-500/15 text-success-400"><TrendingUp class="w-4 h-4" aria-hidden="true" /></span>
        </div>
        <div class="text-3xl font-extrabold text-success-400">
          {{ perUnit(costPerKm) }}<span class="text-sm font-normal text-slate-300">/{{ distanceUnit() }}</span>
        </div>
        <p v-if="tco && tco.depreciation_known === false" class="mt-2 text-xs font-medium text-warning-400">
          {{ $t('dashboard.tcoMetricsGrid.depreciationMissing') }}
          <router-link to="/vehicles" class="relative z-10 underline">{{ $t('dashboard.tcoMetricsGrid.depreciationMissingAction') }}</router-link>
        </p>
        <p v-else-if="rolling && tco?.full_cost_per_km" class="mt-2 text-xs text-slate-300">
          {{ $t('dashboard.tcoMetricsGrid.sincePurchasePerKm', { amount: perUnit(tco.full_cost_per_km), unit: distanceUnit() }) }}
        </p>
        <button type="button" class="tap-text mt-auto self-start pt-1 text-xs font-semibold text-sky-400 underline after:absolute after:inset-0 after:content-['']" @click="showDetails">
          {{ $t('dashboard.tcoMetricsGrid.details') }}
        </button>
      </div>

      <!-- Cash paid -->
      <div class="relative flex flex-col bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm transition-colors hover:border-slate-600">
        <div class="flex min-h-8 items-start justify-between gap-2 mb-2">
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">
            {{ rolling ? $t('dashboard.tcoMetricsGrid.cashPaid') : $t('dashboard.tcoMetricsGrid.totalCost') }}
            <span class="relative z-10"><GlossaryTip term="cashPaid" /></span>
            <span v-if="rolling" class="block font-medium normal-case tracking-normal text-slate-400">{{ $t('dashboard.tcoMetricsGrid.windowMonths', { months: rolling.months }, rolling.months) }}</span>
          </span>
          <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-rose-500/15 text-rose-400"><Coins class="w-4 h-4" aria-hidden="true" /></span>
        </div>
        <div class="text-2xl font-extrabold text-white">{{ headline(rolling ? rolling.cash : tco?.total_cost) }}</div>
        <p v-if="rolling && (rolling.estimatedEnergy || tco?.estimated_energy_cost)" class="mt-2 text-xs text-slate-300">
          {{ $t('dashboard.tcoMetricsGrid.ofWhichEstimatedEnergy', { amount: money(rolling.estimatedEnergy, 0) }) }}
          <span class="relative z-10"><GlossaryTip term="estimatedEnergy" /></span>
        </p>
        <p v-else-if="tco?.estimated_energy_cost" class="mt-2 text-xs text-slate-300">
          {{ $t('dashboard.tcoMetricsGrid.ofWhichEstimatedEnergy', { amount: money(tco.estimated_energy_cost, 0) }) }}
          <span class="relative z-10"><GlossaryTip term="estimatedEnergy" /></span>
        </p>
        <router-link to="/expenses" class="tap-text mt-auto self-start pt-1 text-xs font-semibold text-sky-400 underline after:absolute after:inset-0 after:content-['']">
          {{ $t('dashboard.tcoMetricsGrid.viewExpenses') }}
        </router-link>
      </div>

      <!-- Current month -->
      <div class="relative flex flex-col bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm transition-colors hover:border-slate-600">
        <div class="flex min-h-8 items-start justify-between gap-2 mb-2">
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">
            {{ month ? $t('dashboard.tcoMetricsGrid.currentMonth', { month: formatMonthName(month.month) }) : $t('dashboard.tcoMetricsGrid.currentMonthShort') }}
          </span>
          <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-violet-500/15 text-violet-400"><Calendar class="w-4 h-4" aria-hidden="true" /></span>
        </div>
        <template v-if="month">
          <div class="text-2xl font-extrabold text-white">{{ headline(month.total - month.scheduled) }}</div>
          <p class="mt-1 text-xs text-slate-300">{{ $t('dashboard.tcoMetricsGrid.spentSoFar') }}</p>
          <p v-if="month.scheduled > 0" class="mt-0.5 text-xs text-slate-300">
            {{ $t('dashboard.tcoMetricsGrid.alreadyScheduled', { amount: money(month.scheduled) }) }}
          </p>
          <p class="mt-2 text-xs text-slate-300">
            {{ $t('dashboard.tcoMetricsGrid.monthDistance', { unit: distanceUnit(), distance_km: formatDistanceValue(month.distance_km) }) }}
            · {{ formatCostPerDistance(month.cost_per_km, vehicleStore.currency, 3, true) }}
          </p>
          <button type="button" class="tap-text mt-auto self-start pt-1 text-xs font-semibold text-sky-400 underline after:absolute after:inset-0 after:content-['']" @click="emit('open-month', month.raw)">
            {{ $t('dashboard.tcoMetricsGrid.viewTheBreakdown') }}
          </button>
        </template>
        <div v-else class="text-2xl font-extrabold text-slate-400">—</div>
      </div>

      <!-- Energy -->
      <div class="relative flex flex-col bg-slate-900 border border-slate-800 p-4 rounded-2xl shadow-sm transition-colors hover:border-slate-600">
        <div class="flex min-h-8 items-start justify-between gap-2 mb-2">
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ energyLabel }}</span>
          <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-sky-500/15 text-sky-400"><Zap class="w-4 h-4" aria-hidden="true" /></span>
        </div>
        <div class="text-2xl font-extrabold text-sky-400">{{ headline(tco?.energy_cost) }}</div>
        <p class="mt-2 text-xs text-slate-300">{{ perUnit(tco?.energy_cost_per_km) }}/{{ distanceUnit() }}</p>
        <button v-if="canCharge(tco?.powertrain)" type="button" class="tap-text mt-auto self-start pt-1 text-xs font-semibold text-sky-400 underline after:absolute after:inset-0 after:content-['']" @click="showEnergyPanel">
          {{ $t('dashboard.tcoMetricsGrid.energyPanelLink') }}
        </button>
      </div>
    </div>

    <details ref="detailsRef" class="rounded-2xl border border-slate-800 bg-slate-900/60 px-4 py-2 text-xs text-slate-300 group">
      <summary class="flex cursor-pointer list-none items-center justify-between gap-2 font-semibold text-slate-300">
        {{ $t('dashboard.tcoMetricsGrid.details') }}
        <ChevronDown class="h-4 w-4 transition-transform group-open:rotate-180" aria-hidden="true" />
      </summary>
      <dl class="mt-3 grid grid-cols-1 gap-x-6 gap-y-1.5 sm:grid-cols-2">
        <div class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.fullCostLabel') }}</dt>
          <dd class="font-semibold text-white">{{ money(tco?.full_cost) }}</dd>
        </div>
        <div v-if="tco?.depreciation_cost" class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.depreciationLabel') }}</dt>
          <dd class="font-semibold text-white">{{ money(tco.depreciation_cost, 0) }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.totalCost') }}</dt>
          <dd class="font-semibold text-white">{{ money(tco?.total_cost, 0) }}</dd>
        </div>
        <div v-if="rolling" class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.economicWindowLabel', { months: rolling.months }, rolling.months) }}</dt>
          <dd class="font-semibold text-white">{{ money(rolling.economic, 0) }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.runningCostLabel', { unit: distanceUnit() }) }} <GlossaryTip term="runningCost" /></dt>
          <dd class="font-semibold text-white">{{ perUnit(tco?.usage_cost_per_km) }}/{{ distanceUnit() }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.distanceBasis') }}</dt>
          <dd class="font-semibold text-white">{{ formatDistanceValue(tco?.distance_basis_km || 0) }} {{ distanceUnit() }}</dd>
        </div>
        <div class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.tollsAndParking') }}</dt>
          <dd class="font-semibold text-white">{{ money(tco?.tolls_cost, 0) }} · {{ perUnit(tco?.tolls_cost_per_km) }}/{{ distanceUnit() }}</dd>
        </div>
        <div v-if="tco?.carpool_revenue" class="flex justify-between gap-3">
          <dt class="text-success-400">{{ $t('dashboard.tcoMetricsGrid.netOfCarpoolingLabel') }}</dt>
          <dd class="font-semibold text-success-400">{{ money(tco.full_cost_net) }}</dd>
        </div>
        <template v-if="canRefuel(tco?.powertrain)">
          <div v-if="tco?.total_liters" class="flex justify-between gap-3">
            <dt class="text-slate-400">{{ volumeUnitLabel() }}</dt>
            <dd class="font-semibold text-white">{{ Math.round(litresToDisplayVolume(tco.total_liters)).toLocaleString(intlLocale()) }}</dd>
          </div>
          <div v-if="tco?.consumption_l_100km" class="flex justify-between gap-3">
            <dt class="text-slate-400">{{ fuelConsumptionUnit() }}</dt>
            <dd class="font-semibold text-white">{{ formatFuelConsumptionValue(tco.consumption_l_100km, 2) }}</dd>
          </div>
          <div v-if="tco?.avg_cost_per_liter" class="flex justify-between gap-3">
            <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.avgPrice') }}</dt>
            <dd class="font-semibold text-white">{{ money(perVolume(tco.avg_cost_per_liter), 3) }}/{{ volumeUnitLabel() }}</dd>
          </div>
        </template>
        <div v-if="canCharge(tco?.powertrain) && tco?.total_kwh_added" class="flex justify-between gap-3">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.kwhLabel') }}</dt>
          <dd class="font-semibold text-white">{{ Math.round(tco.total_kwh_added).toLocaleString(intlLocale()) }} kWh</dd>
        </div>
        <div v-if="canCharge(tco?.powertrain) && canRefuel(tco?.powertrain)" class="flex justify-between gap-3 sm:col-span-2">
          <dt class="text-slate-400">{{ $t('dashboard.tcoMetricsGrid.hybridSplitLabel') }}</dt>
          <dd class="font-semibold text-white">{{ $t('dashboard.tcoMetricsGrid.hybridSplit', { electricity: money((tco?.energy_cost || 0) - (tco?.fuel_energy_cost || 0), 0), fuel: money(tco?.fuel_energy_cost || 0, 0) }) }}</dd>
        </div>
      </dl>
      <p v-if="tco?.completeness?.charges_without_cost" class="mt-2 text-xs text-warning-400">
        {{ $t('dashboard.tcoMetricsGrid.chargeSWithoutACost', { charges_without_cost: tco.completeness.charges_without_cost }) }}
      </p>
    </details>
  </section>
</template>
