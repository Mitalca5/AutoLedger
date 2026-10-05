<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { formatDistance } from '@/units'
import { canRefuel, isElectricOnly } from '@/utils/vehicles'
import { ref, computed } from 'vue'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { AlertTriangle, CheckCircle2, ChevronDown } from 'lucide-vue-next'

// TCO completeness score with the reasons it is not 100 %, and the odometer inconsistencies on demand
const props = defineProps<{ tco: any | null; vehicleId: string }>()
const vehicleStore = useVehicleStore()

const dataQuality = ref<any | null>(null)
const showDataQuality = ref(false)

async function toggleDataQuality() {
  showDataQuality.value = !showDataQuality.value
  if (showDataQuality.value && props.vehicleId) {
    try {
      dataQuality.value = await api.getDataQuality(props.vehicleId)
    } catch (err) {
      console.error('Failed to load data quality', err)
    }
  }
}

const issueLabels: Record<string, () => string> = {
  ODOMETER_GAP: () => t('dashboard.dataQualityCard.odometerGap'),
  ODOMETER_REGRESSION: () => t('dashboard.dataQualityCard.odometerRegression'),
  DISTANCE_MISMATCH: () => t('dashboard.dataQualityCard.distanceMismatch'),
}

const RING_LENGTH = 2 * Math.PI * 15.5

// Below half the card turns amber: that is when the figures above are really unreliable. Above, it is a to-do, not an alert.
const score = computed(() => props.tco?.completeness?.score_pct ?? 0)
const low = computed(() => score.value < 50)
const ringColor = computed(() => (low.value ? 'stroke-amber-400' : score.value >= 90 ? 'stroke-emerald-400' : 'stroke-sky-400'))

interface Action {
  key: string
  to: string
  label: string
  hint?: string
}

const actions = computed<Action[]>(() => {
  const c = props.tco?.completeness
  if (!c) return []
  const list: (Action & { show: boolean })[] = [
    { key: 'charges', to: '/expenses', label: t('dashboard.dataQualityCard.completeTheCharges'), show: c.charges_without_cost > 0 },
    { key: 'drives', to: '/drives', label: t('dashboard.dataQualityCard.qualifyTheDrives'), show: c.unqualified_drives > 0 },
    { key: 'insurance', to: '/expenses', label: t('dashboard.dataQualityCard.enterTheInsurance'), show: !!c.insurance_missing },
    { key: 'acquisition', to: '/vehicles', label: t('dashboard.dataQualityCard.enterTheAcquisition'), show: !!c.acquisition_missing },
    {
      key: 'startOdometer',
      to: '/vehicles',
      label: t('dashboard.dataQualityCard.enterTheStartOdometer'),
      hint: t('dashboard.dataQualityCard.enterTheStartOdometerHint'),
      show: !!c.start_odometer_missing,
    },
    { key: 'fillUp', to: '/manual?tab=FUEL', label: t('dashboard.dataQualityCard.enterAFillUp'), show: canRefuel(props.tco.powertrain) && !props.tco.fuel_fill_ups },
    {
      key: 'consumption',
      to: '/manual?tab=ENERGY',
      label: t('dashboard.dataQualityCard.enterAverageConsumption'),
      hint: t('dashboard.dataQualityCard.enterAverageConsumptionHint'),
      show: isElectricOnly(props.tco.powertrain) && c.untracked_distance_km > 0 && !props.tco.estimated_energy_cost,
    },
  ]
  return list.filter((a) => a.show)
})
const hasOdometerIssues = computed(() => (props.tco?.completeness?.odometer_gaps ?? 0) > 0 || (props.tco?.completeness?.odometer_anomalies ?? 0) > 0)
const todoCount = computed(() => actions.value.length + (hasOdometerIssues.value ? 1 : 0))
const showTodo = ref(false)
</script>

<template>
  <div
    v-if="tco?.completeness"
    class="p-4 rounded-2xl space-y-2 border"
    :class="tco.completeness.is_complete ? 'bg-emerald-500/5 border-emerald-500/20' : low ? 'bg-amber-500/10 border-amber-500/30' : 'bg-slate-900 border-slate-800'"
  >
    <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
      <div class="flex items-center gap-2.5 text-sm font-bold min-w-0" :class="tco.completeness.is_complete ? 'text-emerald-400' : low ? 'text-amber-400' : 'text-slate-200'">
        <CheckCircle2 v-if="tco.completeness.is_complete" class="w-5 h-5 shrink-0 text-emerald-400" />
        <svg v-else class="w-7 h-7 shrink-0 -rotate-90" viewBox="0 0 36 36" fill="none" aria-hidden="true">
          <circle cx="18" cy="18" r="15.5" stroke-width="3.5" class="stroke-slate-800" />
          <circle cx="18" cy="18" r="15.5" stroke-width="3.5" stroke-linecap="round" :stroke-dasharray="RING_LENGTH" :stroke-dashoffset="RING_LENGTH * (1 - score / 100)" :class="ringColor" />
        </svg>
        <AlertTriangle v-if="low && !tco.completeness.is_complete" class="w-4 h-4 shrink-0" />
        <span>{{ $t('dashboard.dataQualityCard.tcoComplete', { score_pct: tco.completeness.score_pct }) }}</span>
        <span
          v-if="['MANUAL', 'SEMI_AUTO'].includes(vehicleStore.activeVehicle?.telemetry_mode)"
          class="ml-1 text-[11px] font-semibold px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700"
        >
          {{ $t('dashboard.dataQualityCard.manualTrackingNotice') }}
        </span>
      </div>
      <button
        v-if="todoCount > 0"
        type="button"
        class="tap-text px-3 text-xs font-semibold rounded-xl gap-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition-colors"
        :aria-expanded="showTodo"
        @click="showTodo = !showTodo"
      >
        {{ $t('dashboard.dataQualityCard.complete', { count: todoCount }) }}
        <ChevronDown class="w-3.5 h-3.5 transition-transform" :class="{ 'rotate-180': showTodo }" aria-hidden="true" />
      </button>
    </div>
    <div v-if="showTodo" class="flex flex-wrap gap-2 pt-1">
      <router-link
        v-for="action in actions"
        :key="action.key"
        :to="action.to"
        :title="action.hint"
        class="tap-text text-xs font-semibold px-3 rounded-lg bg-slate-800 text-slate-200 border border-slate-700 hover:bg-slate-700"
      >
        {{ action.label }}
      </router-link>
      <button
        v-if="hasOdometerIssues"
        type="button"
        class="tap-text text-xs font-semibold px-3 rounded-lg bg-slate-800 text-slate-200 border border-slate-700 hover:bg-slate-700"
        @click="toggleDataQuality"
      >
        {{ showDataQuality ? $t('dashboard.dataQualityCard.hideIssues') : $t('dashboard.dataQualityCard.showIssues') }}
      </button>
    </div>
    <div v-if="showDataQuality && dataQuality" class="pt-2 max-h-64 overflow-y-auto space-y-1">
      <div
        v-for="issue in dataQuality.issues"
        :key="issue.type + issue.drive_id"
        class="text-xs text-warning-100/90 flex items-center justify-between gap-3 bg-slate-950/40 rounded-lg px-2.5 py-1.5"
      >
        <span>{{ new Date(issue.date).toLocaleString(intlLocale(), { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) }} — {{ issueLabels[issue.type]?.() || issue.type }}</span>
        <span class="font-mono">{{ issue.km > 0 ? '+' : '' }}{{ formatDistance(issue.km, 1) }}</span>
      </div>
    </div>
  </div>
</template>
