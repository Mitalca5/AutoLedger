<script setup lang="ts">
import { computed } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { Users, X, MapPin, Navigation, Pencil, RotateCw, CheckCircle2, Sparkles, ChevronRight, Zap, Disc, Wrench, Shield, Receipt } from 'lucide-vue-next'
import CostDonut from '@/components/costs/CostDonut.vue'
import CostItemRow from '@/components/costs/CostItemRow.vue'
import { buildCarpoolBreakdown } from '@/utils/costBreakdown'
import { carpoolCoverage, formatDate, stopNames } from '@/utils/carpool'
import { formatAmount } from '@/currency'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistance, perDistance } from '@/units'

// Detail of a carpool trip, laid out like the drive and trip cost breakdown: summary, legs, cost split, passengers,
// total. A leg made from a drive opens the cost detail of that drive, like the legs of a trip. The form to change the
// carpool is the edit modal, reached from the footer.
const props = defineProps<{ trip: any | null; recalculating: boolean }>()
const emit = defineEmits<{ edit: [trip: any]; recalculate: [trip: any]; 'open-drive': [driveId: string] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const vehicleStore = useVehicleStore()
// Carpool amounts are in the vehicle's own currency
const fmt = (v: number) => formatAmount(Number(v || 0), vehicleStore.currency)

const breakdown = computed(() => buildCarpoolBreakdown(props.trip))
const coverage = computed(() => carpoolCoverage(props.trip))
// Positive when the passengers paid more than their share
const balance = computed(() => coverage.value.paid - coverage.value.fair)
// Same icons, colors and labels as the cost rows of the drive detail
const ROWS = {
  energy: { icon: Zap, tone: 'sky', label: 'drives.driveCostModal.electricEnergy' },
  tires: { icon: Disc, tone: 'emerald', label: 'drives.driveCostModal.tireWear' },
  maintenance: { icon: Wrench, tone: 'pink', label: 'drives.driveCostModal.maintenanceProvision' },
  insurance: { icon: Shield, tone: 'purple', label: 'drives.driveCostModal.insuranceShareFixedCost' },
  tolls: { icon: Receipt, tone: 'amber', label: 'drives.driveCostModal.tollsAndRoadCosts' },
  other: { icon: Receipt, tone: 'slate', label: 'dashboard.breakdown.other' },
} as const
const stops = computed(() => stopNames(props.trip?.legs || []))
</script>

<template>
  <div
    v-if="open && trip"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-3xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Header -->
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2.5 min-w-0 pr-2">
          <div class="p-2 rounded-xl shrink-0 bg-rose-500/10 text-rose-400">
            <Users class="w-5 h-5" />
          </div>
          <div class="min-w-0 truncate">
            <h3 class="text-base font-bold text-white truncate">{{ trip.title }}</h3>
            <p class="text-xs text-slate-400">{{ formatDate(trip.date) }}</p>
          </div>
        </div>
        <button @click="open = false" class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0" :title="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <!-- Summary route -->
        <div class="bg-slate-800/60 border border-slate-700/60 p-3.5 rounded-2xl space-y-2">
          <div class="text-sm font-semibold text-white flex items-center gap-2">
            <MapPin class="w-4 h-4 text-rose-400 shrink-0" />
            <span class="truncate">{{ stops[0] }}</span>
            <span class="text-slate-500">→</span>
            <span class="truncate">{{ stops[stops.length - 1] }}</span>
          </div>
          <div class="flex items-center gap-3 text-xs text-slate-300 flex-wrap">
            <span class="font-bold text-rose-400">{{ formatDistance(trip.distance_km, 1) }}</span>
            <span class="text-indigo-400 font-semibold">{{ $t('carpool.carpoolTripList.legs', { length: trip.legs.length }) }}</span>
            <span class="text-blue-400 font-semibold">{{ $t('carpool.carpoolTripList.passengers', { length: trip.passengers?.length || 0 }) }}</span>
          </div>
          <p v-if="trip.notes" class="text-xs text-slate-400 pt-2 border-t border-slate-700/60">{{ trip.notes }}</p>
        </div>

        <!-- Legs, as rows -->
        <div class="space-y-1.5">
          <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
            <Navigation class="w-3.5 h-3.5 text-indigo-400" />
            {{ $t('carpool.carpoolTripList.legs', { length: trip.legs.length }) }}
          </h4>
          <component
            :is="leg.drive_id ? 'button' : 'div'"
            v-for="(leg, i) in trip.legs"
            :key="leg.id"
            :type="leg.drive_id ? 'button' : undefined"
            class="w-full text-left flex items-center justify-between gap-3 bg-slate-800/40 border border-slate-800 rounded-xl px-3 py-2 transition-colors"
            :class="leg.drive_id ? 'hover:bg-slate-800/70 hover:border-slate-700' : ''"
            @click="leg.drive_id && emit('open-drive', leg.drive_id)"
          >
            <div class="min-w-0">
              <div class="text-xs text-slate-200 truncate">{{ stops[Number(i)] }} → {{ stops[Number(i) + 1] }}</div>
              <div class="text-[11px] text-slate-400">
                {{ $t('carpool.carpoolDetailModal.onBoard', { count: 1 + leg.passenger_seats }) }} · {{ $t('carpool.carpoolDetailModal.perPerson', { amount: fmt(leg.cost_per_person) }) }}
              </div>
            </div>
            <div class="flex items-center gap-3 shrink-0">
              <span class="text-[11px] font-bold text-rose-400">{{ formatDistance(leg.distance_km, 1) }}</span>
              <span class="text-xs font-mono font-bold text-white">{{ fmt(leg.total_cost) }}</span>
              <ChevronRight v-if="leg.drive_id" class="w-4 h-4 text-slate-500" />
            </div>
          </component>
        </div>

        <!-- Same layout as the drive detail: donut on the left, itemized costs on the right -->
        <div class="grid grid-cols-1 md:grid-cols-5 gap-6 items-start">
          <div class="md:col-span-2 bg-slate-800/30 border border-slate-800 rounded-xl p-4 flex flex-col items-center justify-center">
            <h4 class="text-xs font-bold text-white mb-2 self-start">{{ $t('drives.driveCostModal.breakdownTitle') }}</h4>
            <div class="w-full h-56 sm:h-64 relative">
              <CostDonut
                :items="breakdown.items"
                :empty-label="$t('drives.driveCostModal.noCost')"
                :chart-label="$t('drives.driveCostModal.breakdownAria')"
                :currency="vehicleStore.currency"
              />
            </div>
          </div>

          <div class="md:col-span-3 space-y-2.5">
            <template v-for="item in breakdown.items" :key="item.key">
              <CostItemRow
                v-if="item.amount > 0 || item.key !== 'other'"
                :icon="ROWS[item.key as keyof typeof ROWS].icon"
                :tone="ROWS[item.key as keyof typeof ROWS].tone"
                :label="$t(ROWS[item.key as keyof typeof ROWS].label)"
                :amount="item.amount"
                :share-pct="item.sharePct"
                :cost-per-km="item.costPerKm"
                :currency="vehicleStore.currency"
              />
            </template>
          </div>
        </div>

        <!-- Passengers -->
        <div class="space-y-1.5">
          <h4 class="text-xs font-bold text-white">
            <span class="flex items-center gap-1.5">
              <Users class="w-3.5 h-3.5 text-blue-400" />
              {{ $t('carpool.carpoolTripList.passengers', { length: trip.passengers?.length || 0 }) }}
            </span>
          </h4>
          <div v-for="p in trip.passengers" :key="p.id" class="bg-slate-800/40 border border-slate-800 rounded-xl px-3 py-2 text-xs space-y-1">
            <div class="flex items-center justify-between gap-2">
              <span class="font-semibold text-slate-200 truncate">{{ p.passenger_name }}</span>
              <span class="font-bold text-emerald-400 shrink-0">+{{ fmt(p.amount_paid) }}</span>
            </div>
            <div class="text-[11px] text-slate-400 truncate">
              {{ stops[p.board_stop_index] }} → {{ stops[p.alight_stop_index] }} • {{ $t('carpool.carpoolTripList.seats', p.seats) }}
            </div>
            <div v-if="trip.passengers.length > 1" class="flex items-center justify-between text-[11px]">
              <span class="text-slate-400">{{ $t('carpool.carpoolTripList.share', { cost_share: fmt(p.cost_share) }) }}</span>
              <span :class="p.balance >= 0 ? 'text-emerald-400' : 'text-amber-400'">
                {{ p.balance >= 0 ? $t('carpool.above', { amount: fmt(p.balance) }) : $t('carpool.below', { amount: fmt(-p.balance) }) }}
              </span>
            </div>
          </div>
        </div>

        <!-- Balance: the total of the drive detail, then who paid what and what is left, each figure once -->
        <div class="bg-gradient-to-r from-slate-800 to-slate-800/80 border p-4 rounded-2xl space-y-3 shadow-lg" :class="trip.net_cost <= 0 ? 'border-emerald-500/40' : 'border-emerald-500/30'">
          <div class="flex items-center justify-between">
            <div>
              <span class="text-xs font-semibold text-emerald-400 uppercase tracking-wider">{{ $t('drives.driveCostModal.totalCostPrice') }}</span>
              <div class="text-2xl font-black text-white">{{ fmt(trip.total_cost) }}</div>
            </div>
            <div class="text-right">
              <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('drives.driveCostModal.costPerKilometre', { unit: distanceUnit() }) }}</span>
              <div class="text-lg font-extrabold text-emerald-400 font-mono">
                {{ formatAmount(perDistance(trip.distance_km > 0 ? trip.total_cost / trip.distance_km : 0), vehicleStore.currency, 3) }}<span class="text-xs font-normal text-slate-400">/{{ distanceUnit() }}</span>
              </div>
            </div>
          </div>

          <!-- What the passengers paid, against what they owed (the tick) -->
          <div class="pt-3 border-t border-slate-700/60 space-y-2">
            <div class="flex items-baseline justify-between gap-2">
              <span class="text-xs font-semibold text-slate-300">{{ $t('carpool.carpoolDetailModal.paidByPassengers') }}</span>
              <span class="text-sm font-bold font-mono" :class="coverage.status === 'below' ? 'text-amber-400' : 'text-emerald-400'">{{ coverage.paidPct.toFixed(0) }}%</span>
            </div>
            <div
              class="relative h-2.5 rounded-full bg-slate-950 border border-slate-700/60"
              role="progressbar"
              :aria-valuenow="Math.round(coverage.paidPct)"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-label="$t('carpool.carpoolDetailModal.paidByPassengers')"
            >
              <div
                class="absolute inset-y-0 left-0 rounded-full transition-all"
                :class="coverage.status === 'below' ? 'bg-amber-500' : 'bg-emerald-500'"
                :style="{ width: Math.min(100, coverage.paidPct) + '%' }"
              ></div>
              <div
                class="absolute -top-1 -bottom-1 w-0.5 rounded bg-white"
                :style="{ left: `calc(${Math.min(100, coverage.fairPct)}% - 1px)` }"
                :title="$t('carpool.carpoolDetailModal.fairShare', { percent: coverage.fairPct.toFixed(0) })"
              ></div>
            </div>
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-x-3 gap-y-0.5 text-[11px] text-slate-400">
              <span>
                {{ $t('carpool.carpoolDetailModal.received', { amount: fmt(coverage.paid) }) }} ·
                {{ $t('carpool.carpoolDetailModal.fairShareAmount', { percent: coverage.fairPct.toFixed(0), amount: fmt(coverage.fair) }) }}
              </span>
              <span v-if="coverage.status !== 'fair'" :class="balance > 0 ? 'text-emerald-400' : 'text-amber-400'">
                {{ balance > 0 ? $t('carpool.above', { amount: fmt(balance) }) : $t('carpool.carpoolSummaryGrid.below', { amount: fmt(-balance) }) }}
              </span>
            </div>
          </div>

          <div class="pt-3 border-t border-slate-700/60 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs text-slate-300">
            <div class="flex items-center gap-2">
              <CheckCircle2 v-if="trip.net_cost <= 0" class="w-4 h-4 text-emerald-400 shrink-0" />
              <Sparkles v-else class="w-4 h-4 text-amber-400 shrink-0" />
              <span>{{ $t('carpool.carpoolTripList.driverSShare') }} <strong>{{ fmt(trip.driver_cost_share) }}</strong></span>
            </div>
            <div>
              <template v-if="trip.net_cost > 0">
                {{ $t('carpool.carpoolTripList.leftToTheDriver') }} <strong class="text-white text-sm">{{ fmt(trip.net_cost) }}</strong>
              </template>
              <span v-else class="font-bold text-emerald-400 text-sm">{{ $t('carpool.carpoolTripList.netSurplus', { net_cost: fmt(Math.abs(trip.net_cost)) }) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer Actions -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <button @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.close') }}
        </button>
        <div v-if="vehicleStore.canEdit" class="flex items-center gap-2">
          <button
            type="button"
            @click="emit('recalculate', trip)"
            :disabled="recalculating"
            class="px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors disabled:opacity-50"
            :title="$t('carpool.carpoolTripList.recalculateTheActualCostsOf')"
          >
            <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': recalculating }" />
          </button>
          <button
            type="button"
            @click="emit('edit', trip)"
            class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-bold rounded-xl flex items-center gap-2 shadow-lg shadow-rose-600/25 transition-all"
          >
            <Pencil class="w-4 h-4" />
            <span>{{ $t('common.edit') }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
