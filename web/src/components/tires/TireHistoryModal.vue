<script setup lang="ts">
import { intlLocale } from '@/i18n'
import { Archive, ClipboardPaste, Copy, Edit2, History, Pencil, Plus, Ruler, Shuffle, Trash2, X, Zap } from 'lucide-vue-next'
import { useVehicleStore } from '@/stores/vehicle'
import { apiMessageText } from '@/services/apiError'
import { formatAmount } from '@/currency'
import { formatDate, positionLabel, WEAR_TONE_TEXT, wearTone, type SessionForm } from '@/utils/tires'
import { formatNumber, formatPercent } from '@/utils/numbers'
import TireWearBar from '@/components/tires/TireWearBar.vue'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistance, formatDistanceValue, formatPerDistanceValue, perDistance } from '@/units'
import { formatCostPerDistance } from '@/utils/costPerDistance'

const vehicleStore = useVehicleStore()

// Full history of one tire: life KPIs, TeslaMate telemetry, mount sessions and tread depth measurements.
// It only displays: every action is emitted for the page to run.
defineProps<{
  selectedTire: any | null
  selectedTireStats: any | null
  tireSessions: any[]
  tireLogs: any[]
  copiedSession: SessionForm | null
}>()
const emit = defineEmits<{
  'edit-tire': []
  'dispose-tire': []
  'delete-tire': []
  'paste-session': []
  'add-session': []
  'copy-session': [session: any]
  'duplicate-session': [session: any]
  'edit-session': [session: any]
  'delete-session': [session: any]
  'add-log': []
  'edit-log': [log: any]
  'delete-log': [log: any]
}>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
</script>

<template>
  <div
    v-if="open && selectedTire"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-2xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Header -->
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-start justify-between shrink-0 bg-slate-900/95">
        <div>
          <div class="flex items-center gap-2">
            <h3 class="text-base font-bold text-white">{{ selectedTire.brand }} {{ selectedTire.model }}</h3>
            <span class="text-xs font-mono bg-slate-800 px-2 py-0.5 rounded text-slate-300">
              {{ selectedTire.dimension }}
            </span>
          </div>
          <div class="text-xs text-slate-400 mt-0.5">
            {{ $t('tires.tireHistoryModal.boughtOn', { purchase_date: formatDate(selectedTire.purchase_date), purchase_price: formatAmount(Number(selectedTire.purchase_price || 0), vehicleStore.currency) }) }}
          </div>
        </div>
        <div class="flex items-center gap-1.5">
          <button @click="emit('edit-tire')" class="tap p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white rounded-lg transition-colors" :title="$t('tires.tireHistoryModal.editTheTire')" :aria-label="$t('tires.tireHistoryModal.editTheTire')">
            <Pencil class="w-4 h-4" />
          </button>
          <button
            v-if="selectedTire.current_position !== 'DISPOSED'"
            @click="emit('dispose-tire')"
            class="tap p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-warning-400 rounded-lg transition-colors"
            :title="$t('tires.tireHistoryModal.scrapWornPuncturedSold')" :aria-label="$t('tires.tireHistoryModal.scrapWornPuncturedSold')"
          >
            <Archive class="w-4 h-4" />
          </button>
          <button @click="emit('delete-tire')" class="tap p-1.5 ml-1.5 bg-slate-800 hover:bg-danger-900/40 text-slate-400 hover:text-danger-400 rounded-lg transition-colors" :title="$t('tires.tireHistoryModal.deleteEntryError')" :aria-label="$t('tires.tireHistoryModal.deleteEntryError')">
            <Trash2 class="w-4 h-4" />
          </button>
          <button @click="open = false" :aria-label="$t('common.close')" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
            <X class="w-5 h-5" />
          </button>
        </div>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-6">

      <!-- Life KPI Card -->
      <div class="bg-slate-950/60 border border-slate-800 p-4 rounded-2xl space-y-3">
        <div class="flex items-center justify-between text-xs">
          <span class="text-slate-400">{{ $t('tires.tireHistoryModal.totalLifetimeMileage') }}</span>
          <span class="text-base font-bold text-white">
            {{ formatDistance(selectedTireStats?.total_distance_km || 0) }}
            <span class="text-xs text-slate-400 font-normal">{{ $t('tires.tireHistoryModal.kmEstimated', { unit: distanceUnit(), estimated_lifespan_km: formatDistanceValue(selectedTire.estimated_lifespan_km || 45000) }) }}</span>
          </span>
        </div>

        <TireWearBar thick :pct="selectedTireStats?.life_progress_pct || 0" :condition="selectedTireStats?.condition" />

        <div class="grid grid-cols-3 gap-2 text-center text-xs pt-1">
          <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
            <div class="text-xs text-slate-400">{{ $t('tires.tireHistoryModal.currentTread') }}</div>
            <div class="font-bold" :class="WEAR_TONE_TEXT[wearTone(0, selectedTireStats?.condition)]">{{ selectedTireStats?.current_depth_mm != null ? `${formatNumber(selectedTireStats.current_depth_mm, 1)} mm` : '—' }}</div>
          </div>
          <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
            <div class="text-xs text-slate-400">{{ $t('tires.tireHistoryModal.lifespanUsed') }}</div>
            <div class="font-bold text-slate-200">{{ formatPercent(Number(selectedTireStats?.life_progress_pct || 0)) }}</div>
          </div>
          <div class="bg-slate-900/80 p-2 rounded-xl border border-slate-800">
            <div class="text-xs text-slate-400">{{ $t('tires.tireHistoryModal.actualCostKm', { unit: distanceUnit() }) }}</div>
            <div class="font-bold text-warning-400" :title="Number(selectedTire?.purchase_price) > 0 ? undefined : $t('tires.costPerKmUnset')">{{ formatCostPerDistance(selectedTireStats?.cost_per_km, vehicleStore.currency, 4) }}</div>
          </div>
        </div>
      </div>

      <!-- TeslaMate Driving Telemetry & Stress Analysis Card -->
      <div v-if="vehicleStore.hasTeslaMate && selectedTireStats?.driving_stress_index > 0" class="bg-gradient-to-br from-slate-950 to-slate-900 border border-slate-800 p-4 rounded-2xl space-y-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Zap class="w-4 h-4 text-warning-400" />
            <h4 class="text-xs font-bold text-white uppercase tracking-wider">{{ $t('tires.tireHistoryModal.teslamateDrivingTelemetry') }}</h4>
          </div>
          <span
            class="px-2.5 py-0.5 rounded-full text-xs font-bold border"
            :class="
              selectedTireStats.driving_style === 'SPORT'
                ? 'bg-rose-500/15 text-rose-400 border-rose-500/30'
                : selectedTireStats.driving_style === 'ECO'
                ? 'bg-success-500/15 text-success-400 border-success-500/30'
                : 'bg-info-500/15 text-info-400 border-info-500/30'
            "
          >
            {{ selectedTireStats.driving_style === 'SPORT' ? $t('tires.tireHistoryModal.sport') : selectedTireStats.driving_style === 'ECO' ? $t('tires.tireHistoryModal.eco') : $t('tires.tireHistoryModal.balanced') }} {{ $t('tires.tireHistoryModal.index', { index: formatNumber(selectedTireStats.driving_stress_index, 2) }) }}
          </span>
        </div>

        <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-center text-xs">
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-xs text-slate-400 uppercase">{{ $t('tires.tireHistoryModal.peakAcceleration') }}</div>
            <div class="font-bold text-rose-400 text-sm mt-0.5">+{{ formatNumber(selectedTireStats.avg_power_max_kw, 0) }} kW</div>
          </div>
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-xs text-slate-400 uppercase">{{ $t('tires.tireHistoryModal.peakRegeneration') }}</div>
            <div class="font-bold text-success-400 text-sm mt-0.5">{{ formatNumber(selectedTireStats.avg_power_min_kw, 0) }} kW</div>
          </div>
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-xs text-slate-400 uppercase">{{ $t('tires.tireHistoryModal.averageConsumption') }}</div>
            <div class="font-bold text-info-400 text-sm mt-0.5">{{ $t('tires.tireHistoryModal.kwh', { unit: distanceUnit(), avg_consumption_kwh_100km: formatPerDistanceValue(Number(selectedTireStats.avg_consumption_kwh_100km)) }) }}</div>
          </div>
          <div class="bg-slate-900 p-2.5 rounded-xl border border-slate-800">
            <div class="text-xs text-slate-400 uppercase">{{ $t('tires.tireHistoryModal.adjustedLongevity') }}</div>
            <div class="font-bold text-indigo-300 text-sm mt-0.5">~{{ formatDistance(selectedTireStats.dynamic_lifespan_km || selectedTire.estimated_lifespan_km) }}</div>
          </div>
        </div>

        <p v-if="selectedTireStats.wear_explanation" class="text-xs text-slate-300 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800/80">
          {{ apiMessageText(selectedTireStats.wear_explanation) }}
        </p>
      </div>

      <!-- Timeline: Mount/Dismount Sessions -->
      <div class="space-y-3">
        <div class="flex items-center justify-between">
          <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
            <History class="w-4 h-4 text-rose-500" />
            {{ $t('tires.tireHistoryModal.historyOfFittingsRemovalsAnd') }}
          </h4>
          <div class="flex items-center gap-2">
            <button
              v-if="copiedSession"
              @click="emit('paste-session')"
              class="text-xs text-indigo-400 hover:text-indigo-300 font-semibold flex items-center gap-1 transition-colors bg-indigo-950/40 border border-indigo-800/60 px-2 py-1 rounded-lg"
              :title="$t('tires.tireHistoryModal.pasteCopied', { date: copiedSession.mounted_date ? formatDate(copiedSession.mounted_date) : '' })" :aria-label="$t('tires.tireHistoryModal.pasteCopied', { date: copiedSession.mounted_date ? formatDate(copiedSession.mounted_date) : '' })"
            >
              <ClipboardPaste class="w-3.5 h-3.5" />
              <span>{{ $t('tires.tireHistoryModal.paste') }}</span>
            </button>
            <button
              @click="emit('add-session')"
              class="text-xs text-rose-400 hover:text-rose-300 font-semibold flex items-center gap-1 transition-colors"
            >
              <Plus class="w-3.5 h-3.5" />
              {{ $t('tires.tireHistoryModal.addAPastSession') }}
            </button>
          </div>
        </div>

        <div v-if="tireSessions.length === 0" class="p-6 text-center bg-slate-950/40 rounded-2xl text-xs text-slate-400">
          {{ $t('tires.tireHistoryModal.noSessionRecordedForThis') }}
        </div>

        <div v-else class="space-y-2.5">
          <div
            v-for="s in tireSessions"
            :key="s.id"
            class="bg-slate-950/80 border border-slate-800/80 rounded-2xl p-3.5 text-xs space-y-2 hover:border-slate-700 transition-all"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span
                  class="px-2 py-0.5 rounded-md font-bold text-xs"
                  :class="s.dismounted_date ? 'bg-slate-800 text-slate-300' : 'bg-success-500/20 text-success-400 border border-success-500/30'"
                >
                  {{ s.dismounted_date ? $t('tires.tireHistoryModal.sessionOver') : $t('tires.tireHistoryModal.currentlyFitted') }}
                </span>
                <span class="font-bold text-white">{{ positionLabel(s.position) }}</span>
              </div>

              <div class="flex items-center gap-1.5">
                <button
                  @click="emit('copy-session', s)"
                  class="tap p-1 rounded transition-colors"
                  :class="copiedSession?.mounted_date === (s.mounted_date ? new Date(s.mounted_date).toISOString().substring(0, 10) : '') && copiedSession?.mounted_odometer === s.mounted_odometer ? 'text-indigo-400 bg-indigo-950/60' : 'text-slate-400 hover:text-indigo-400'"
                  :title="$t('tires.tireHistoryModal.copyThisSessionSData')" :aria-label="$t('tires.tireHistoryModal.copyThisSessionSData')"
                >
                  <Copy class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="emit('duplicate-session', s)"
                  class="tap p-1 text-slate-400 hover:text-info-400 rounded transition-colors"
                  :title="$t('tires.tireHistoryModal.duplicateToOtherTires')" :aria-label="$t('tires.tireHistoryModal.duplicateToOtherTires')"
                >
                  <Shuffle class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="emit('edit-session', s)"
                  class="tap p-1 text-slate-400 hover:text-white rounded"
                  :title="$t('tires.tireHistoryModal.editTheSession')" :aria-label="$t('tires.tireHistoryModal.editTheSession')"
                >
                  <Edit2 class="w-3.5 h-3.5" />
                </button>
                <button
                  @click="emit('delete-session', s)"
                  class="tap p-1 text-slate-400 hover:text-rose-400 rounded"
                  :title="$t('tires.tireHistoryModal.deleteTheSession')" :aria-label="$t('tires.tireHistoryModal.deleteTheSession')"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Session Details -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs text-slate-400 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800/60">
              <div>
                <div class="text-slate-400">{{ $t('tires.tireHistoryModal.fitting') }}</div>
                <div class="text-slate-200 font-medium">{{ $t('tires.tireHistoryModal.dateAtKm', { unit: distanceUnit(), date: formatDate(s.mounted_date), km: formatDistanceValue(s.mounted_odometer) }) }}</div>
              </div>
              <div>
                <div class="text-slate-400">{{ $t('tires.tireHistoryModal.removal') }}</div>
                <div class="text-slate-200 font-medium">
                  {{ s.dismounted_date ? $t('tires.tireHistoryModal.removedAt', { unit: distanceUnit(), date: formatDate(s.dismounted_date), odometer: formatDistanceValue(s.dismounted_odometer) }) : $t('tires.tireHistoryModal.currentlyOnVehicle') }}
                </div>
              </div>
            </div>

            <div class="flex items-center justify-between text-xs pt-1">
              <span v-if="s.notes" class="text-slate-400 italic">"{{ s.notes }}"</span>
              <span v-else></span>
              <span class="font-bold text-rose-400">{{ $t('tires.tireHistoryModal.kmDriven', { unit: distanceUnit(), distance_km: formatDistanceValue(s.dismounted_date ? s.distance_km : selectedTireStats?.current_run_km ?? s.distance_km) }) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Tread Logs (Mesures de gomme) -->
      <div class="space-y-2 pt-3 border-t border-slate-800">
        <div class="flex items-center justify-between">
          <h4 class="text-xs font-bold text-white flex items-center gap-1.5">
            <Ruler class="w-4 h-4 text-success-400" />
            {{ $t('tires.tireHistoryModal.treadDepthReadings', { length: tireLogs.length }) }}
          </h4>
          <button
            @click="emit('add-log')"
            class="text-xs text-success-400 hover:text-success-300 font-semibold flex items-center gap-1 transition-colors"
          >
            <Plus class="w-3.5 h-3.5" />
            {{ $t('tires.tireHistoryModal.addAReading') }}
          </button>
        </div>

        <div v-if="tireLogs.length > 0" class="grid grid-cols-2 sm:grid-cols-3 gap-2">
          <div
            v-for="l in tireLogs"
            :key="l.id"
            class="bg-slate-950/60 border border-slate-800 p-2.5 rounded-xl text-xs space-y-0.5"
          >
            <div class="flex items-center justify-between">
              <span class="font-bold text-success-400">{{ l.depth_mm }} mm</span>
              <span class="text-xs text-slate-400">{{ formatDate(l.date) }}</span>
            </div>
            <div class="flex items-center justify-between text-xs text-slate-400">
              <span>{{ $t('common.atKm', { unit: distanceUnit(), km: formatDistanceValue(l.odometer) }) }}</span>
              <span class="flex items-center gap-1">
                <button @click="emit('edit-log', l)" class="tap text-slate-400 hover:text-success-400" :title="$t('tires.tireHistoryModal.editTheReading')" :aria-label="$t('tires.tireHistoryModal.editTheReading')">
                  <Pencil class="w-3 h-3" />
                </button>
                <button @click="emit('delete-log', l)" class="tap text-slate-400 hover:text-danger-400" :title="$t('tires.tireHistoryModal.deleteTheReading')" :aria-label="$t('tires.tireHistoryModal.deleteTheReading')">
                  <Trash2 class="w-3 h-3" />
                </button>
              </span>
            </div>
          </div>
        </div>
      </div>
      </div>

      <!-- Pinned Footer -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="btn btn-lg btn-secondary"
        >
          {{ $t('common.close') }}
        </button>
      </div>
    </div>
  </div>
</template>
