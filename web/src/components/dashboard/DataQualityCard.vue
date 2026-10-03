<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { formatDistance } from '@/units'
import { canCharge, canRefuel } from '@/utils/vehicles'
import { ref, computed } from 'vue'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { AlertTriangle, CheckCircle2 } from 'lucide-vue-next'

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
</script>

<template>
  <div
    v-if="tco?.completeness"
    class="p-4 rounded-2xl space-y-2 border"
    :class="tco.completeness.is_complete ? 'bg-emerald-500/5 border-emerald-500/20' : 'bg-amber-500/10 border-amber-500/30'"
  >
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
      <div class="flex items-center gap-2 text-sm font-bold" :class="tco.completeness.is_complete ? 'text-emerald-400' : 'text-amber-400'">
        <CheckCircle2 v-if="tco.completeness.is_complete" class="w-4 h-4 text-emerald-400" />
        <AlertTriangle v-else class="w-4 h-4" />
        <span>{{ $t('dashboard.dataQualityCard.tcoComplete', { score_pct: tco.completeness.score_pct }) }}</span>
        <span
          v-if="['MANUAL', 'SEMI_AUTO'].includes(vehicleStore.activeVehicle?.telemetry_mode)"
          class="ml-1 text-[11px] font-semibold px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700"
        >
          {{ $t('dashboard.dataQualityCard.manualTrackingNotice') }}
        </span>
      </div>
      <div class="w-full sm:w-48 h-2 bg-slate-800 rounded-full overflow-hidden">
        <div
          class="h-full rounded-full"
          :class="tco.completeness.score_pct >= 90 ? 'bg-emerald-500' : tco.completeness.score_pct >= 60 ? 'bg-amber-500' : 'bg-rose-500'"
          :style="{ width: tco.completeness.score_pct + '%' }"
        ></div>
      </div>
    </div>
    <div class="flex flex-wrap gap-2 pt-1">
      <router-link
        v-if="tco.completeness.charges_without_cost > 0"
        to="/expenses"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ $t('dashboard.dataQualityCard.completeTheCharges') }}
      </router-link>
      <router-link
        v-if="tco.completeness.unqualified_drives > 0"
        to="/drives"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ $t('dashboard.dataQualityCard.qualifyTheDrives') }}
      </router-link>
      <router-link
        v-if="tco.completeness.insurance_missing"
        to="/expenses"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ $t('dashboard.dataQualityCard.enterTheInsurance') }}
      </router-link>
      <router-link
        v-if="tco.completeness.acquisition_missing"
        to="/vehicles"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ $t('dashboard.dataQualityCard.enterTheAcquisition') }}
      </router-link>
      <router-link
        v-if="tco.completeness.start_odometer_missing"
        to="/vehicles"
        :title="$t('dashboard.dataQualityCard.enterTheStartOdometerHint')"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ $t('dashboard.dataQualityCard.enterTheStartOdometer') }}
      </router-link>
      <router-link
        v-if="canRefuel(tco.powertrain) && !tco.fuel_fill_ups"
        to="/manual?tab=FUEL"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ $t('dashboard.dataQualityCard.enterAFillUp') }}
      </router-link>
      <router-link
        v-if="canCharge(tco.powertrain) && tco.completeness.untracked_distance_km > 0 && !tco.estimated_energy_cost"
        to="/manual?tab=ENERGY"
        :title="$t('dashboard.dataQualityCard.enterAverageConsumptionHint')"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ $t('dashboard.dataQualityCard.enterAverageConsumption') }}
      </router-link>
      <button
        v-if="tco.completeness.odometer_gaps > 0 || tco.completeness.odometer_anomalies > 0"
        @click="toggleDataQuality"
        class="text-xs font-semibold px-2.5 py-1 rounded-lg bg-amber-500/20 text-amber-300 hover:bg-amber-500/30"
      >
        {{ showDataQuality ? $t('dashboard.dataQualityCard.hideIssues') : $t('dashboard.dataQualityCard.showIssues') }}
      </button>
    </div>
    <div v-if="showDataQuality && dataQuality" class="pt-2 max-h-64 overflow-y-auto space-y-1">
      <div
        v-for="issue in dataQuality.issues"
        :key="issue.type + issue.drive_id"
        class="text-[11px] text-amber-100/90 flex items-center justify-between gap-3 bg-slate-950/40 rounded-lg px-2.5 py-1.5"
      >
        <span>{{ new Date(issue.date).toLocaleString(intlLocale(), { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) }} — {{ issueLabels[issue.type]?.() || issue.type }}</span>
        <span class="font-mono">{{ issue.km > 0 ? '+' : '' }}{{ formatDistance(issue.km, 1) }}</span>
      </div>
    </div>
  </div>
</template>
