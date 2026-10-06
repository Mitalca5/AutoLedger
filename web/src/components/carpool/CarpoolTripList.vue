<script setup lang="ts">
import { computed } from 'vue'
import { formatDistance } from '@/units'
import { useVehicleStore } from '@/stores/vehicle'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import SelectAllToggle from '@/components/SelectAllToggle.vue'
import { Users, Trash2, Edit2, Calendar, Navigation, RotateCw, Download, ChevronRight } from 'lucide-vue-next'
import { formatDate } from '@/utils/carpool'
import { formatAmount } from '@/currency'

// The carpool trips as compact cards (the legs, passengers and costs are in the detail, opened by a click), and the
// selection actions (recalculate, export)
const props = defineProps<{ trips: any[]; selectedTripIds: string[]; recalculating: boolean }>()
const emit = defineEmits<{
  clear: []
  open: [trip: any]
  'batch-recalculate': []
  export: []
  'toggle-all': []
  toggle: [tripId: string]
  recalculate: [trip: any]
  edit: [trip: any]
  delete: [trip: any]
}>()
const vehicleStore = useVehicleStore()
// Carpool amounts are in the vehicle's own currency
const fmt = (v: number) => formatAmount(Number(v || 0), vehicleStore.currency)

const isAllSelected = computed(() => props.trips.length > 0 && props.selectedTripIds.length === props.trips.length)
</script>

<template>
  <div class="space-y-4">
    <!-- Sticky Bulk Selection Bar -->
    <BulkSelectionBar
      v-if="vehicleStore.canEdit"
      :count="selectedTripIds.length"
      noun="carpool"
      @clear="emit('clear')"
    >
      <button
        type="button"
        @click="emit('batch-recalculate')"
        :disabled="recalculating"
        class="px-3.5 py-1.5 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shadow-md shadow-rose-600/20 disabled:opacity-50 transition-all"
        :title="$t('carpool.carpoolTripList.recalculateTheActualCostsOf2')"
      >
        <RotateCw class="w-3.5 h-3.5" :class="{ 'animate-spin': recalculating }" />
        <span>{{ $t('carpool.carpoolTripList.recalculateActualCosts', { length: selectedTripIds.length }) }}</span>
      </button>

      <button
        type="button"
        @click="emit('export')"
        class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors border border-slate-700/60"
        :title="$t('carpool.carpoolTripList.exportTheSelectionAsCsv')"
      >
        <Download class="w-3.5 h-3.5 text-slate-300" />
        <span>{{ $t('carpool.carpoolTripList.exportCsv') }}</span>
      </button>
    </BulkSelectionBar>

    <!-- Header row: Select all checkbox & Total info -->
    <div v-if="vehicleStore.canEdit" class="flex items-center justify-between text-xs text-slate-400 px-2">
      <SelectAllToggle
        :checked="isAllSelected"
        :indeterminate="selectedTripIds.length > 0 && !isAllSelected"
        :label="isAllSelected ? $t('common.deselectAll') : $t('common.selectAll')"
        @toggle="emit('toggle-all')"
      />
      <span>{{ $t('carpool.carpoolTripList.carpoolSInTotal', { length: trips.length }) }}</span>
    </div>

    <div
      v-for="trip in trips"
      :key="trip.id"
      v-clickable
      @click="emit('open', trip)"
      class="bg-slate-900 border rounded-2xl p-4 transition-all shadow-sm space-y-3 cursor-pointer"
      :class="selectedTripIds.includes(trip.id) ? 'border-rose-500/50 bg-rose-500/[0.02]' : 'border-slate-800 hover:border-slate-700'"
    >
      <!-- Trip Header -->
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div class="flex items-start gap-3 min-w-0 flex-1">
          <!-- Select Checkbox -->
          <label
            v-if="vehicleStore.canEdit"
            :for="'carpool-select-' + trip.id"
            @click.stop
            class="mt-0.5 -ml-1 p-1 shrink-0 flex items-center cursor-pointer"
            :title="$t('carpool.carpoolTripList.selectThisCarpool')"
          >
            <span class="sr-only">{{ $t('carpool.carpoolTripList.selectThisCarpool') }}</span>
            <input
              :id="'carpool-select-' + trip.id"
              type="checkbox"
              :checked="selectedTripIds.includes(trip.id)"
              @change="emit('toggle', trip.id)"
              class="select-box"
            />
          </label>

          <div class="space-y-1 min-w-0 flex-1">
            <div class="flex items-center gap-2.5 flex-wrap min-w-0">
              <h3 class="text-base font-bold text-white truncate max-w-sm sm:max-w-md" :title="trip.title">{{ trip.title }}</h3>
              <span class="text-xs bg-slate-800 text-slate-300 px-2.5 py-0.5 rounded-full border border-slate-700/60 font-medium shrink-0">{{ formatDistance(trip.distance_km, 1) }}</span>
              <span
                class="text-xs px-2.5 py-0.5 rounded-full font-semibold border shrink-0"
                :class="trip.net_cost <= 0 ? 'bg-success-500/15 text-success-400 border-success-500/30' : 'bg-rose-500/10 text-rose-400 border-rose-500/20'"
              >
                {{ trip.net_cost <= 0 ? $t('carpool.carpoolTripList.fullyRecovered') : $t('carpool.carpoolTripList.recovered', { percent: trip.total_cost > 0 ? Math.min(100, Math.round((trip.total_revenue / trip.total_cost) * 100)) : 0 }) }}
              </span>
            </div>
            <div class="flex items-center gap-2 text-xs text-slate-400 min-w-0">
              <Calendar class="w-3.5 h-3.5 shrink-0" />
              <span class="shrink-0">{{ formatDate(trip.date) }}</span>
              <span v-if="trip.notes" class="text-slate-400 truncate">• {{ trip.notes }}</span>
            </div>
          </div>
        </div>
        <div v-if="vehicleStore.canEdit" @click.stop class="flex items-center gap-1 sm:gap-2 shrink-0 self-end sm:self-auto">
          <button
            @click="emit('recalculate', trip)"
            :disabled="recalculating"
            class="p-2 text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 rounded-lg transition-colors"
            :title="$t('carpool.carpoolTripList.recalculateTheActualCostsOf')" :aria-label="$t('carpool.carpoolTripList.recalculateTheActualCostsOf')"
          >
            <RotateCw class="w-4 h-4" />
          </button>
          <button @click="emit('edit', trip)" class="p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-lg transition-colors" :title="$t('common.edit')" :aria-label="$t('common.edit')">
            <Edit2 class="w-4 h-4" />
          </button>
          <button @click="emit('delete', trip)" class="p-2 text-slate-400 hover:text-danger-400 hover:bg-danger-500/10 rounded-lg transition-colors" :title="$t('common.delete')" :aria-label="$t('common.delete')">
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- One line: what it covers and what it cost; the rest is in the detail -->
      <div class="flex items-center justify-between gap-3 text-xs text-slate-400 flex-wrap">
        <div class="flex items-center gap-x-3 gap-y-1 flex-wrap">
          <span class="flex items-center gap-1.5"><Navigation class="w-3.5 h-3.5 text-indigo-400" />{{ $t('carpool.carpoolTripList.legs', { length: trip.legs.length }) }}</span>
          <span class="flex items-center gap-1.5"><Users class="w-3.5 h-3.5 text-blue-400" />{{ $t('carpool.carpoolTripList.passengers', { length: trip.passengers?.length || 0 }) }}</span>
          <span>{{ $t('carpool.carpoolTripList.actualCost') }} <strong class="text-slate-200">{{ fmt(trip.total_cost) }}</strong></span>
          <span class="text-success-400 font-semibold">+{{ fmt(trip.total_revenue) }}</span>
        </div>
        <div class="flex items-center gap-2">
          <span v-if="trip.net_cost > 0" class="font-semibold text-slate-200">
            {{ $t('carpool.carpoolTripList.leftToTheDriver') }} {{ fmt(trip.net_cost) }}
          </span>
          <span v-else class="font-semibold text-success-400">{{ $t('carpool.carpoolTripList.netSurplus', { net_cost: fmt(Math.abs(trip.net_cost)) }) }}</span>
          <ChevronRight class="w-4 h-4 text-slate-400" />
        </div>
      </div>
    </div>
  </div>
</template>
