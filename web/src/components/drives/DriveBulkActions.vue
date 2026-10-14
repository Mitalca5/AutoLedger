<script setup lang="ts">
import { computed } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import { t } from '@/i18n'
import type { VehiclePerson } from '@/services/api'
import { defaultDriver } from '@/utils/vehicles'
import { usePreferencesStore } from '@/stores/preferences'
import BulkSelectionBar from '@/components/BulkSelectionBar.vue'
import { Receipt, Layers, Users, Plus, Download } from 'lucide-vue-next'

// Actions on the drives selected in the list; the page runs them.
const props = defineProps<{ selectedDriveIds: string[]; selectedOffPage: number; selectedSummaryMetrics: string; bulkApplyingToll: boolean; people: VehiclePerson[] }>()
const emit = defineEmits<{
  clear: []
  carpool: []
  group: []
  'bulk-toll': []
  tag: [tag: 'Pro' | 'Perso']
  driver: [driverId: string | null]
  export: []
  'add-to-trip': []
}>()
const vehicleStore = useVehicleStore()
const prefs = usePreferencesStore()

// "" is the placeholder; DEFAULT_DRIVER clears the driver so the vehicle's default applies
const DEFAULT_DRIVER = '__default__'
const defaultDriverLabel = computed(() => {
  const d = defaultDriver(props.people, vehicleStore.activeVehicle?.default_driver_id)
  return d ? t('drives.defaultDriver', { name: d.name }) : t('drives.unassignedDriver')
})
function onDriverChange(e: Event) {
  const el = e.target as HTMLSelectElement
  const value = el.value
  el.value = ''
  if (!value) return
  emit('driver', value === DEFAULT_DRIVER ? null : value)
}
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
    <!-- The one primary action: complete the tolls of the selection -->
    <button
      type="button"
      @click="emit('bulk-toll')"
      :disabled="bulkApplyingToll"
      class="btn btn-primary"
      :title="$t('drives.driveBulkActions.detectsTheTollsAndRecords')"
    >
      <Receipt class="w-3.5 h-3.5" />
      <span>{{ bulkApplyingToll ? $t('drives.driveBulkActions.applying') : $t('drives.driveBulkActions.autoToll', { count: selectedDriveIds.length }) }}</span>
    </button>

    <button type="button" @click="emit('carpool')" class="btn btn-secondary">
      <Users class="w-3.5 h-3.5" />
      <span>{{ selectedDriveIds.length > 1 ? $t('drives.driveBulkActions.carpoolMany', { count: selectedDriveIds.length }) : $t('drives.driveBulkActions.carpool') }}</span>
    </button>

    <button type="button" @click="emit('group')" class="btn btn-secondary">
      <Layers class="w-3.5 h-3.5" />
      <span>{{ $t('drives.driveBulkActions.mergeToll') }}</span>
    </button>

    <!-- Occasional actions -->
    <template #more>
      <label v-if="people.length" class="block px-3 pt-1.5 pb-1 text-xs font-semibold text-slate-400">
        {{ $t('drives.driveBulkActions.driver') }}
        <select value="" @change="onDriverChange" class="field mt-1 w-full font-normal" :title="$t('drives.driveBulkActions.changeDriver')">
          <option value="" disabled>{{ $t('drives.driveBulkActions.chooseDriver') }}</option>
          <option :value="DEFAULT_DRIVER">{{ defaultDriverLabel }}</option>
          <option v-for="p in people" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </label>

      <template v-if="prefs.proPersoEnabled">
        <button type="button" role="menuitem" class="menu-item" :title="$t('drives.driveBulkActions.markTheSelectionAsWork')" @click="emit('tag', 'Pro')">
          <span class="h-2 w-2 rounded-full bg-blue-400"></span>{{ $t('drives.driveBulkActions.work') }}
        </button>
        <button type="button" role="menuitem" class="menu-item" :title="$t('drives.driveBulkActions.markTheSelectionAsPersonal')" @click="emit('tag', 'Perso')">
          <span class="h-2 w-2 rounded-full bg-success-400"></span>{{ $t('drives.driveBulkActions.personal') }}
        </button>
      </template>

      <button type="button" role="menuitem" class="menu-item" :title="$t('drives.driveBulkActions.exportTheSelectionAsCsv')" @click="emit('export')">
        <Download class="w-3.5 h-3.5 text-slate-400" />{{ $t('drives.driveBulkActions.export') }}
      </button>
      <button type="button" role="menuitem" class="menu-item" @click="emit('add-to-trip')">
        <Plus class="w-3.5 h-3.5 text-slate-400" />{{ $t('drives.driveBulkActions.addToTrip') }}
      </button>
    </template>
  </BulkSelectionBar>
</template>
