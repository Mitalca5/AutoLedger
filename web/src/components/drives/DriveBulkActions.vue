<script setup lang="ts">
import { useVehicleStore } from '@/stores/vehicle'
import { usePreferencesStore } from '@/stores/preferences'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import { Receipt, Layers, Users, Plus, Download } from 'lucide-vue-next'

// Actions on the drives selected in the list; the page runs them.
defineProps<{ selectedDriveIds: string[]; selectedOffPage: number; selectedSummaryMetrics: string; bulkApplyingToll: boolean }>()
const emit = defineEmits<{
  clear: []
  carpool: []
  group: []
  'bulk-toll': []
  tag: [tag: 'Pro' | 'Perso']
  export: []
  'add-to-trip': []
}>()
const vehicleStore = useVehicleStore()
const prefs = usePreferencesStore()
</script>

<template>
  <BulkSelectionBar
    v-if="vehicleStore.canEdit"
    :count="selectedDriveIds.length"
    noun="drive"
    :off-screen-count="selectedOffPage"
    :metrics-summary="selectedSummaryMetrics"
    @clear="emit('clear')"
  >
    <!-- Direct Carpool button for single or multi-drives -->
    <button
      type="button"
      @click="emit('carpool')"
      class="btn btn-primary"
    >
      <Users class="w-3.5 h-3.5" />
      <span>{{ selectedDriveIds.length > 1 ? $t('drives.driveBulkActions.carpoolMany', { count: selectedDriveIds.length }) : $t('drives.driveBulkActions.carpool') }}</span>
    </button>

    <!-- Fusion Voyage Group -->
    <button
      type="button"
      @click="emit('group')"
      class="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors"
    >
      <Layers class="w-3.5 h-3.5" />
      <span>{{ $t('drives.driveBulkActions.mergeToll') }}</span>
    </button>

    <!-- Auto toll -->
    <button
      v-if="vehicleStore.canEdit"
      type="button"
      @click="emit('bulk-toll')"
      :disabled="bulkApplyingToll"
      class="btn btn-primary"
      :title="$t('drives.driveBulkActions.detectsTheTollsAndRecords')"
    >
      <Receipt class="w-3.5 h-3.5" />
      <span>{{ bulkApplyingToll ? $t('drives.driveBulkActions.applying') : $t('drives.driveBulkActions.autoToll', { count: selectedDriveIds.length }) }}</span>
    </button>

    <!-- Batch Tag actions -->
    <button
      v-if="prefs.proPersoEnabled"
      type="button"
      @click="emit('tag', 'Pro')"
      class="px-2.5 py-1.5 bg-blue-500/20 hover:bg-blue-500/30 text-blue-300 border border-blue-500/40 text-xs font-semibold rounded-xl flex items-center gap-1 transition-colors"
      :title="$t('drives.driveBulkActions.markTheSelectionAsWork')"
    >
      <span>{{ $t('drives.driveBulkActions.work') }}</span>
    </button>

    <button
      v-if="prefs.proPersoEnabled"
      type="button"
      @click="emit('tag', 'Perso')"
      class="px-2.5 py-1.5 bg-success-500/20 hover:bg-success-500/30 text-success-300 border border-success-500/40 text-xs font-semibold rounded-xl flex items-center gap-1 transition-colors"
      :title="$t('drives.driveBulkActions.markTheSelectionAsPersonal')"
    >
      <span>{{ $t('drives.driveBulkActions.personal') }}</span>
    </button>

    <button
      type="button"
      @click="emit('export')"
      class="btn btn-secondary"
      :title="$t('drives.driveBulkActions.exportTheSelectionAsCsv')"
    >
      <Download class="w-3.5 h-3.5 text-slate-300" />
      <span>{{ $t('drives.driveBulkActions.export') }}</span>
    </button>

    <button
      type="button"
      @click="emit('add-to-trip')"
      class="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 text-slate-200 text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors"
    >
      <Plus class="w-3.5 h-3.5" />
      <span>{{ $t('drives.driveBulkActions.addToTrip') }}</span>
    </button>
  </BulkSelectionBar>
</template>
