<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import { intlLocale } from '@/i18n'
import { distanceUnit, formatDistance, perDistance } from '@/units'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { Layers, X, Users, Coins, Pencil, Trash2 } from 'lucide-vue-next'
import { formatTripDates } from '@/utils/drives'
import { formatDayTime } from '@/utils/dates'
import { formatAmount } from '@/currency'

// The trip groups ("voyages") list; a trip can be expanded to show its drives.
defineProps<{ loadingTrips: boolean; tripGroups: any[]; hasFilters: boolean; expandedTripId: string | null; tripDrives: any[] }>()
const emit = defineEmits<{
  'open-cost': [trip: any]
  'toggle-details': [trip: any]
  edit: [trip: any]
  delete: [trip: any]
  'remove-drive': [trip: any, driveId: string]
}>()
const router = useRouter()
const vehicleStore = useVehicleStore()
const formatDate = formatDayTime
</script>

<template>
  <div class="space-y-3">
    <div v-if="loadingTrips" class="text-center py-12 text-slate-400">{{ $t('common.loading') }}</div>
    <EmptyState v-else-if="!tripGroups.length">
      {{ hasFilters ? $t('drives.tripGroupsPanel.noTripMatchesTheFilters') : $t('drives.tripGroupsPanel.noTripsSelectSeveralDrives') }}
    </EmptyState>
    <template v-else>
    <template
      v-for="tg in tripGroups"
      :key="tg.id"
    >
      <div
        @click="emit('open-cost', tg)"
        class="bg-slate-900 border border-slate-800 hover:border-slate-700/90 p-4 rounded-2xl transition-all flex flex-col lg:flex-row lg:items-center justify-between gap-4 cursor-pointer group"
      >
        <div class="flex items-start gap-3 min-w-0 flex-1">
          <!-- Icon indicator -->
          <div class="mt-1 p-2 rounded-xl bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 shrink-0">
            <Layers class="w-4 h-4" />
          </div>

          <!-- Voyage Details (matching Drive details structure) -->
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2 flex-wrap mb-1.5">
              <span class="text-xs font-semibold text-slate-400 shrink-0">{{ formatTripDates(tg) }}</span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-bold bg-slate-800 text-slate-200 border border-slate-700/60 shrink-0">
                {{ formatDistance(tg.distance_km) }}
              </span>
              <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/30 shrink-0">
                {{ $t('drives.tripGroupsPanel.legS', { length: tg.drive_ids?.length || 0 }) }}
              </span>
              <span v-if="tg.carpool_count" class="text-xs px-2.5 py-0.5 rounded-full font-semibold bg-rose-500/10 text-rose-300 border border-rose-500/30 shrink-0">
                {{ $t('drives.tripGroupsPanel.carpoolS', { carpool_count: tg.carpool_count }) }}
              </span>
            </div>

            <!-- Voyage Title & Notes (matching Route address line) -->
            <div class="text-sm text-slate-200 flex items-center gap-2 flex-wrap min-w-0">
              <span class="font-bold text-white truncate max-w-sm sm:max-w-md" :title="tg.name">{{ tg.name }}</span>
              <span v-if="tg.notes" class="text-xs text-slate-400 truncate max-w-xs">• {{ tg.notes }}</span>
            </div>
          </div>
        </div>

        <!-- Right Side: Cost Badge & Actions (matching Drive right-side) -->
        <div class="flex items-center gap-2 sm:gap-2.5 self-start lg:self-auto flex-wrap justify-start lg:justify-end shrink-0" @click.stop>
          <!-- Real Cost Badge -->
          <div
            class="px-3 py-1.5 bg-slate-800/80 border border-slate-700/70 rounded-xl flex items-center gap-2 text-left shadow-sm"
            :title="$t('drives.tripGroupsPanel.consolidatedCostOfTheTrip')"
          >
            <div class="p-1 rounded-lg bg-success-500/10 text-success-400">
              <Coins class="w-3.5 h-3.5" />
            </div>
            <div>
              <div class="text-xs font-extrabold text-white flex items-center gap-1.5">
                <span>{{ Number(tg.tolls_total || 0) > 0 ? $t('drives.tripGroupsPanel.costsAmount', { amount: formatAmount(Number(tg.tolls_total), vehicleStore.currency) }) : $t('drives.tripGroupsPanel.costDetail') }}</span>
                <span v-if="tg.distance_km > 0 && tg.tolls_total" class="text-xs font-normal text-success-400 font-mono">
                  {{ formatAmount(perDistance(Number(tg.tolls_total) / tg.distance_km), vehicleStore.currency, 3) }}/{{ distanceUnit() }}
                </span>
              </div>
            </div>
          </div>

          <!-- Details toggle button -->
          <button
            @click="emit('toggle-details', tg)"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-white transition-colors"
          >
            {{ expandedTripId === tg.id ? $t('drives.tripGroupsPanel.hide') : $t('drives.tripGroupsPanel.legs') }}
          </button>

          <!-- Quick Carpool Button -->
          <button
            v-if="vehicleStore.canEdit"
            @click="router.push({ path: '/carpools', query: { new_trip_group_id: tg.id } })"
            class="px-2.5 py-1 text-xs font-semibold rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-rose-400 hover:border-rose-500/40 flex items-center gap-1.5 transition-all"
            :title="$t('drives.tripGroupsPanel.carpoolThisTrip')"
          >
            <Users class="w-3.5 h-3.5 text-rose-500" />
            <span class="hidden md:inline">{{ $t('drives.tripGroupsPanel.carpool') }}</span>
          </button>

          <!-- Edit & Delete -->
          <template v-if="vehicleStore.canEdit">
            <button
              @click="emit('edit', tg)"
              class="tap p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-indigo-400 rounded-lg border border-slate-700/60 transition-colors"
              :title="$t('drives.tripGroupsPanel.renameTheTrip')"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('delete', tg)"
              class="tap p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-lg border border-slate-700/60 transition-colors"
              :title="$t('drives.tripGroupsPanel.deleteTheTrip')"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </template>
        </div>
      </div>
      <!-- Expanded drives drawer -->
      <div v-if="expandedTripId === tg.id" class="space-y-1.5 bg-slate-950/40 border border-slate-800/80 rounded-2xl p-3 -mt-1 ml-4 mr-4">
        <div v-for="d in tripDrives" :key="d.id" class="flex items-center justify-between gap-3 text-xs text-slate-300 bg-slate-800/40 rounded-lg px-2.5 py-1.5 min-w-0">
          <span class="truncate min-w-0 flex-1">
            {{ formatDate(d.start_time) }}{{ $t('drives.tripGroupsPanel.dateSeparator') }}{{ (d.start_address || $t('drives.driveCostModal.start')).split(',')[0] }} → {{ (d.end_address || $t('drives.driveCostModal.end')).split(',')[0] }}
            <span class="text-slate-400">({{ formatDistance(d.distance_km, 1) }})</span>
          </span>
          <button v-if="vehicleStore.canEdit" @click="emit('remove-drive', tg, d.id)" class="tap text-slate-400 hover:text-danger-400 shrink-0 p-1" :title="$t('drives.tripGroupsPanel.removeThisDriveFromThe')">
            <X class="w-3.5 h-3.5" />
          </button>
        </div>
        <p v-if="!tripDrives.length" class="text-xs text-slate-400">{{ $t('drives.tripGroupsPanel.loadingTheDrives') }}</p>
      </div>
    </template>
    </template>
  </div>
</template>
