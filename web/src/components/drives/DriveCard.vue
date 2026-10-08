<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { usePreferencesStore } from '@/stores/preferences'
import { MapPin, Clock, Users, Coins, Pencil, Trash2 } from 'lucide-vue-next'
import CardOpenButton from '@/components/CardOpenButton.vue'
import QualifyActions from '@/components/drives/QualifyActions.vue'
import { needsTollQualification } from '@/utils/drives'
import { formatDayTime } from '@/utils/dates'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistance, perDistance } from '@/units'
import { formatCostPerDistance } from '@/utils/costPerDistance'

// One drive of the list: click opens its cost breakdown.
defineProps<{ d: any; selected: boolean }>()
const emit = defineEmits<{
  open: [drive: any]
  toggle: [drive: any]
  'toll-entry': [drive: any]
  'no-toll': [drive: any]
  edit: [drive: any]
  delete: [drive: any]
}>()
const router = useRouter()
const vehicleStore = useVehicleStore()
const prefs = usePreferencesStore()
const formatDate = formatDayTime
</script>

<template>
  <div
    class="relative bg-slate-900 border border-slate-800 hover:border-slate-700/90 p-4 rounded-2xl transition-all flex flex-col lg:flex-row lg:items-center justify-between gap-4 group"
    :class="{ 'border-rose-500/40 bg-slate-800/40 shadow-lg shadow-rose-950/20': selected }"
  >
    <CardOpenButton :label="$t('drives.driveCard.openBreakdown', { date: formatDate(d.start_time) })" @click="emit('open', d)" />
    <div class="flex items-start gap-3 min-w-0 flex-1">
      <!-- Selection checkbox -->
      <label
        v-if="vehicleStore.canEdit"
        :for="'drive-select-' + d.id"
        class="relative z-10 tap mt-0.5 -ml-1 p-1 shrink-0 flex items-center cursor-pointer"
        :title="$t('drives.driveCard.selectThisDrive')"
      >
        <span class="sr-only">{{ $t('drives.driveCard.selectThisDrive') }}</span>
        <input
          :id="'drive-select-' + d.id"
          type="checkbox"
          :checked="selected"
          @change="emit('toggle', d)"
          class="select-box"
        />
      </label>

      <!-- Drive Details -->
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 flex-wrap mb-1.5">
          <span class="text-xs font-semibold text-slate-400 shrink-0">{{ formatDate(d.start_time) }}</span>
          <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60 shrink-0">
            {{ formatDistance(d.distance_km, 1) }}
          </span>
          <span v-if="d.duration_min" class="text-xs text-slate-400 flex items-center gap-1 shrink-0">
            <Clock class="w-3 h-3" /> {{ $t('drives.driveCard.min', { duration_min: d.duration_min }) }}
          </span>
          <span
            v-if="d.consumption_kwh_100km"
            class="text-xs font-mono shrink-0"
            :class="d.energy_estimated ? 'text-slate-400' : 'text-info-400'"
            :title="d.energy_estimated ? $t('drives.driveCard.estimatedConsumption') : undefined"
          >
            {{ $t(d.energy_estimated ? 'drives.driveCard.kwh100kmEstimated' : 'drives.driveCard.kwh100km', { unit: distanceUnit(), consumption_kwh_100km: Math.round(perDistance(d.consumption_kwh_100km) * 10) / 10 }) }}
          </span>
          <!-- Clean tag pills -->
          <span
            v-if="prefs.proPersoEnabled && d.tags?.includes('Pro')"
            class="text-xs px-2 py-0.5 rounded-full font-bold bg-blue-500/20 text-blue-400 border border-blue-500/40 shrink-0"
          >
            {{ $t('drives.driveCard.work') }}
          </span>
          <span
            v-if="prefs.proPersoEnabled && d.tags?.includes('Perso')"
            class="text-xs px-2 py-0.5 rounded-full font-bold bg-success-500/20 text-success-400 border border-success-500/40 shrink-0"
          >
            {{ $t('drives.driveCard.personal') }}
          </span>
          <span
            v-if="d.is_manual"
            class="text-xs px-2 py-0.5 rounded-full font-bold bg-warning-500/20 text-warning-300 border border-warning-500/40 shrink-0"
          >
            {{ $t('drives.drivesView.manual') }}
          </span>
        </div>

        <!-- Route Address -->
        <div class="text-sm text-slate-300 flex items-center gap-1.5 flex-wrap min-w-0">
          <MapPin class="w-3.5 h-3.5 text-rose-400 shrink-0" />
          <span class="truncate max-w-[140px] sm:max-w-[220px] md:max-w-xs font-medium" :title="d.start_address">{{ d.start_address || $t('drives.driveCard.unknownStart') }}</span>
          <span class="text-slate-400 shrink-0">→</span>
          <span class="truncate max-w-[140px] sm:max-w-[220px] md:max-w-xs font-medium" :title="d.end_address">{{ d.end_address || $t('drives.driveCard.unknownEnd') }}</span>
        </div>
      </div>
    </div>

    <!-- Right Side: Cost Badge & Actions -->
    <div class="relative z-10 flex items-center gap-2 sm:gap-2.5 self-start lg:self-auto flex-wrap justify-start lg:justify-end shrink-0">
      <!-- Toll qualification: 2 taps -->
      <QualifyActions
        v-if="vehicleStore.canEdit && needsTollQualification(d)"
        :primary-label="$t('drives.driveCard.toll')"
        :primary-title="$t('drives.driveCard.enterTheTollOfThis')"
        :secondary-label="$t('drives.driveCard.noToll')"
        :secondary-title="$t('drives.driveCard.confirmThatThisDriveHas')"
        @primary="emit('toll-entry', d)"
        @secondary="emit('no-toll', d)"
      />

      <!-- Real Cost Badge -->
      <div
        class="px-3 py-1.5 bg-slate-800/80 border border-slate-700/70 rounded-xl flex items-center gap-2 text-left shadow-sm"
        :title="$t('drives.driveCard.actualCostPriceCalculatedFor')"
      >
        <div class="p-1 rounded-lg bg-success-500/10 text-success-400">
          <Coins class="w-3.5 h-3.5" />
        </div>
        <div>
          <div class="text-xs font-extrabold text-white flex items-center gap-1.5">
            <span>{{ d.costs?.has_estimates ? '~' : '' }}{{ formatAmount(d.costs?.total_cost || 0, vehicleStore.currency) }}</span>
            <span class="text-xs font-normal text-success-400 font-mono">
              {{ formatCostPerDistance(d.costs?.cost_per_km, vehicleStore.currency, 3, true) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Quick Carpool Button -->
      <button
        v-if="vehicleStore.canEdit"
        @click="router.push({ path: '/carpools', query: { new_drive_id: d.id } })"
        class="btn btn-secondary tap hover:text-rose-400 hover:border-rose-500/40"
        :title="$t('drives.driveCard.createACarpoolFromThis')"
      >
        <Users class="w-3.5 h-3.5 text-rose-500" />
        <span class="hidden md:inline">{{ $t('drives.driveCard.carpool') }}</span>
      </button>

      <!-- Manual Drive Actions -->
      <template v-if="vehicleStore.canEdit && d.is_manual">
        <button
          @click="emit('edit', d)"
          class="tap p-1.5 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-white hover:border-slate-600 transition-all"
          :title="$t('common.edit')" :aria-label="$t('common.edit')"
        >
          <Pencil class="w-3.5 h-3.5" />
        </button>
        <button
          @click="emit('delete', d)"
          class="tap p-1.5 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-400 hover:text-rose-400 hover:border-rose-500/40 transition-all"
          :title="$t('common.delete')" :aria-label="$t('common.delete')"
        >
          <Trash2 class="w-3.5 h-3.5" />
        </button>
      </template>
    </div>
  </div>
</template>
