<script setup lang="ts">
import { t } from '@/i18n'
import { computed, ref, watch } from 'vue'
import { defaultDriver } from '@/utils/vehicles'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { usePreferencesStore } from '@/stores/preferences'
import { api, type VehiclePerson } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { Layers, MapPin, ExternalLink, X, Users, Coins, AlertTriangle, Pencil, ArrowLeft, User } from 'lucide-vue-next'
import { teslamateDriveUrl as buildTeslamateDriveUrl } from '@/utils/drives'
import { formatDayTime } from '@/utils/dates'
import { buildDriveBreakdown } from '@/utils/costBreakdown'
import { formatAmount } from '@/currency'
import CostDonut from '@/components/costs/CostDonut.vue'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit, formatDistance, kmToDisplayDistance, speedUnit } from '@/units'
import { formatCostPerDistance } from '@/utils/costPerDistance'
import DriveCostLegs from '@/components/drives/DriveCostLegs.vue'
import DriveCostCarpools from '@/components/drives/DriveCostCarpools.vue'
import DriveCostRows from '@/components/drives/DriveCostRows.vue'
import DriveCostTolls from '@/components/drives/DriveCostTolls.vue'

// Cost breakdown of a drive, or of a trip group (drive.is_trip_group, whose drives are tripDriveIds), with its
// expenses (edit, delete, add a toll) and the toll detection. A detected trip that is not created yet
// (drive.is_suggestion) is shown read-only, with a button to create it. refreshDrive reloads the drives list and returns the
// refreshed drive so the breakdown follows the server-side costs.
const props = defineProps<{
  vehicleId: string
  tripDriveIds: string[]
  tripLegs: any[]
  startWithTollEntry: boolean
  // Set when the drive was opened from another detail (a carpool): the header then offers a way back to it
  backLabel?: string
  refreshDrive: (driveId: string) => Promise<any | null>
}>()
const emit = defineEmits<{
  'toggle-tag': [drive: any, tag: string]
  'create-trip': [suggestion: any]
  'dismiss-trip': [suggestion: any]
  'edit-trip': [trip: any]
  back: []
}>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const selectedCostDrive = defineModel<any | null>('drive', { required: true })
const router = useRouter()
const vehicleStore = useVehicleStore()
const prefs = usePreferencesStore()
const { showAlert } = useConfirm()
const formatDate = formatDayTime
const breakdown = computed(() => buildDriveBreakdown(selectedCostDrive.value?.costs, Number(selectedCostDrive.value?.distance_km) || 0))
const vehicleCurrency = computed(() => vehicleStore.currency)
const teslamateDriveUrl = (d: any) => buildTeslamateDriveUrl(vehicleStore.activeVehicle, d)

// A trip shows its legs and carpools; opening a leg keeps the trip to come back to
const parentTrip = ref<any | null>(null)
const tripCarpools = ref<any[]>([])

async function loadTripCarpools(tripId: string) {
  tripCarpools.value = []
  if (!props.vehicleId) return
  try {
    const res = await api.getCarpools(props.vehicleId)
    tripCarpools.value = (res.trips || []).filter((c: any) => c.trip_group_id === tripId)
  } catch {
    tripCarpools.value = []
  }
}


function openLeg(leg: any) {
  parentTrip.value = selectedCostDrive.value
  selectedCostDrive.value = leg
}

async function backToTrip() {
  const trip = parentTrip.value
  if (!trip) return
  parentTrip.value = null
  selectedCostDrive.value = trip
  const refreshed = await props.refreshDrive(trip.id)
  if (refreshed) selectedCostDrive.value = refreshed
}

function openCarpools() {
  open.value = false
  router.push({ path: '/carpools' })
}

watch(open, (isOpen) => {
  const drive = selectedCostDrive.value
  if (!isOpen) parentTrip.value = null
  if (!isOpen || !drive) return
  if (drive.is_trip_group) {
    if (!drive.is_suggestion) loadTripCarpools(drive.id)
    return
  }
  loadVehiclePeople()
})


const vehiclePeople = ref<VehiclePerson[]>([])
const defaultDriverOption = computed(() => {
  const d = defaultDriver(vehiclePeople.value, vehicleStore.activeVehicle?.default_driver_id)
  return d ? t('drives.defaultDriver', { name: d.name }) : t('drives.unassignedDriver')
})

async function loadVehiclePeople() {
  if (!props.vehicleId) return
  try {
    vehiclePeople.value = await api.getVehiclePeople(props.vehicleId)
  } catch (err) {
    console.error('Failed to load vehicle people', err)
  }
}

async function handleDriverChange(event: Event) {
  const val = (event.target as HTMLSelectElement).value
  const driverId = val.trim() ? val.trim() : null
  if (!props.vehicleId || !selectedCostDrive.value?.id) return
  try {
    await api.updateDriveDriver(props.vehicleId, selectedCostDrive.value.id, driverId)
    const refreshed = await props.refreshDrive(selectedCostDrive.value.id)
    if (refreshed) {
      selectedCostDrive.value = refreshed
    }
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <div
    v-if="open && selectedCostDrive"
    class="fixed inset-0 z-modal bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-3xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <!-- Header -->
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2.5 min-w-0 pr-2">
          <button
            v-if="parentTrip || backLabel"
            type="button"
            @click="parentTrip ? backToTrip() : emit('back')"
            class="tap p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0"
            :title="parentTrip ? $t('drives.driveCostModal.backToTrip') : backLabel" :aria-label="parentTrip ? $t('drives.driveCostModal.backToTrip') : backLabel"
          >
            <ArrowLeft class="w-4 h-4" />
          </button>
          <div class="p-2 rounded-xl shrink-0" :class="selectedCostDrive.is_trip_group ? 'bg-indigo-500/10 text-indigo-400' : 'bg-success-500/10 text-success-400'">
            <component :is="selectedCostDrive.is_trip_group ? Layers : Coins" class="w-5 h-5" />
          </div>
          <div class="min-w-0 truncate">
            <h3 class="text-base font-bold text-white truncate">
              {{ selectedCostDrive.is_trip_group ? $t('drives.driveCostModal.tripTitle', { name: selectedCostDrive.trip_group_name }) : $t('drives.driveCostModal.driveTitle') }}
            </h3>
            <p class="text-xs text-slate-400">{{ formatDate(selectedCostDrive.start_time) }}</p>
          </div>
        </div>
        <a
          v-if="teslamateDriveUrl(selectedCostDrive)"
          :href="teslamateDriveUrl(selectedCostDrive)!"
          target="_blank"
          rel="noopener noreferrer"
          class="btn btn-secondary ml-auto mr-1 shrink-0"
          :title="$t('drives.driveCostModal.openThisDriveInThe')"
        >
          <ExternalLink class="w-3.5 h-3.5 text-info-400" />
          <span class="hidden sm:inline">TeslaMate</span>
        </a>
        <button @click="open = false" class="tap p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors shrink-0" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">

      <!-- Trip Summary Route -->
      <div class="bg-slate-800/60 border border-slate-700/60 p-3.5 rounded-2xl space-y-2">
        <div class="text-sm font-semibold text-white flex items-center gap-2">
          <MapPin class="w-4 h-4 text-rose-400 shrink-0" />
          <span class="truncate">{{ selectedCostDrive.start_address || $t('drives.driveCostModal.start') }}</span>
          <span class="text-slate-400">→</span>
          <span class="truncate">{{ selectedCostDrive.end_address || $t('drives.driveCostModal.end') }}</span>
        </div>
        <div class="flex items-center gap-3 text-xs text-slate-300 flex-wrap">
          <span class="font-bold text-rose-400">{{ formatDistance(selectedCostDrive.distance_km, 1) }}</span>
          <span v-if="selectedCostDrive.duration_min" class="text-slate-400">{{ $t('drives.driveCostModal.min', { duration_min: selectedCostDrive.duration_min }) }}</span>
          <span v-if="selectedCostDrive.drives_count" class="text-indigo-400 font-semibold">{{ $t('drives.driveCostModal.legs', { drives_count: selectedCostDrive.drives_count }) }}</span>
          <span v-if="selectedCostDrive.speed_avg" class="text-slate-400">{{ $t('drives.driveCostModal.kmHAvg', { speed: speedUnit(), speed_avg: Math.round(kmToDisplayDistance(selectedCostDrive.speed_avg)) }) }}</span>
          <span v-if="selectedCostDrive.costs?.electricity_kwh" class="text-info-400 font-mono">{{ $t('drives.driveCostModal.kwh', { electricity_kwh: selectedCostDrive.costs.electricity_kwh }) }}</span>
        </div>

        <!-- Tag qualification (only for individual drives) -->
        <div v-if="prefs.proPersoEnabled && !selectedCostDrive.is_trip_group && vehicleStore.canEdit" class="pt-2 border-t border-slate-700/60 flex items-center justify-between gap-2">
          <span class="text-xs text-slate-400">{{ $t('drives.driveCostModal.classification') }}</span>
          <div class="flex items-center gap-1.5">
            <button
              type="button"
              @click="emit('toggle-tag', selectedCostDrive, 'Pro')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                selectedCostDrive.tags?.includes('Pro')
                  ? 'bg-blue-500/20 text-blue-400 border-blue-500/40 shadow-sm'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              {{ $t('drives.driveCostModal.work') }}
            </button>
            <button
              type="button"
              @click="emit('toggle-tag', selectedCostDrive, 'Perso')"
              class="px-2.5 py-1 text-xs font-semibold rounded-lg border transition-all"
              :class="
                selectedCostDrive.tags?.includes('Perso')
                  ? 'bg-success-500/20 text-success-400 border-success-500/40 shadow-sm'
                  : 'bg-slate-800 text-slate-400 border-slate-700 hover:text-white'
              "
            >
              {{ $t('drives.driveCostModal.personal') }}
            </button>
          </div>
        </div>

        <!-- Driver attribution (only for individual drives) -->
        <div v-if="!selectedCostDrive.is_trip_group && vehicleStore.canEdit" class="pt-2 border-t border-slate-700/60 flex items-center justify-between gap-2">
          <label for="drive-driver-select" class="text-xs text-slate-400 flex items-center gap-1.5 cursor-pointer">
            <User class="w-3.5 h-3.5 text-purple-400" />
            {{ $t('drives.driverLabel') }}
          </label>
          <select
            id="drive-driver-select"
            :value="selectedCostDrive.driver_id || ''"
            @change="handleDriverChange($event)"
            class="field focus:border-purple-500"
          >
            <option value="">{{ defaultDriverOption }}</option>
            <option v-for="p in vehiclePeople" :key="p.id" :value="p.id">
              {{ p.name }}
            </option>
          </select>
        </div>
      </div>

      <p v-if="selectedCostDrive.is_suggestion" class="text-xs text-indigo-300 bg-indigo-500/10 border border-indigo-500/20 rounded-xl px-3 py-2">
        {{ $t('drives.driveCostModal.suggestionNotice') }}
      </p>

      <p v-if="selectedCostDrive.costs?.has_estimates" class="text-xs text-warning-400/90 bg-warning-500/10 border border-warning-500/20 rounded-xl px-3 py-2 flex items-start gap-2">
        <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
        <span>{{ $t('drives.driveCostModal.someItemsUseADefault') }}</span>
      </p>

      <DriveCostLegs v-if="selectedCostDrive.is_trip_group && tripLegs.length" :trip-legs="tripLegs" :vehicle-currency="vehicleCurrency" @open="openLeg" />

      <DriveCostCarpools v-if="selectedCostDrive.is_trip_group && tripCarpools.length" :trip-carpools="tripCarpools" :vehicle-currency="vehicleCurrency" @open="openCarpools" />

      <!-- Same layout as the monthly detail: donut on the left, itemized costs on the right -->
      <div class="grid grid-cols-1 md:grid-cols-5 gap-6 items-start">
      <div class="md:col-span-2 bg-slate-800/30 border border-slate-800 rounded-xl p-4 flex flex-col items-center justify-center">
        <h4 class="text-xs font-bold text-white mb-2 self-start">{{ $t('drives.driveCostModal.breakdownTitle') }}</h4>
        <div class="w-full h-56 sm:h-64 relative">
          <CostDonut
            :items="breakdown.items"
            :empty-label="$t('drives.driveCostModal.noCost')"
            :chart-label="$t('drives.driveCostModal.breakdownAria')"
            :currency="vehicleCurrency"
          />
        </div>
      </div>

      <!-- Cost Breakdown List -->
      <div class="md:col-span-3 space-y-2.5">
        <DriveCostRows :drive="selectedCostDrive" :breakdown="breakdown" :currency="vehicleCurrency" />

        <DriveCostTolls
          v-model:drive="selectedCostDrive"
          :vehicle-id="vehicleId"
          :trip-drive-ids="tripDriveIds"
          :start-with-toll-entry="startWithTollEntry"
          :breakdown="breakdown"
          :refresh-drive="refreshDrive"
        />

        <!-- Grand Total Card -->
        <div class="bg-gradient-to-r from-slate-800 to-slate-800/80 border border-success-500/30 p-4 rounded-2xl flex items-center justify-between shadow-lg">
          <div>
            <span class="text-xs font-semibold text-success-400 uppercase tracking-wider">{{ $t('drives.driveCostModal.totalCostPrice') }}</span>
            <div class="text-2xl font-black text-white">
              {{ formatAmount(selectedCostDrive.costs?.total_cost || 0, vehicleCurrency) }}
            </div>
          </div>
          <div class="text-right">
            <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">{{ $t('drives.driveCostModal.costPerKilometre', { unit: distanceUnit() }) }}</span>
            <div class="text-lg font-extrabold text-success-400 font-mono">
              {{ formatCostPerDistance(selectedCostDrive.costs?.cost_per_km, vehicleCurrency, 3, true) }}
            </div>
          </div>
        </div>
      </div>
      </div>

      </div>

      <!-- Footer Actions -->
      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <button
          @click="open = false"
          class="btn btn-lg btn-secondary"
        >
          {{ $t('common.close') }}
        </button>
        <div v-if="selectedCostDrive.is_suggestion && vehicleStore.canEdit" class="flex items-center gap-2">
          <button
            type="button"
            @click="emit('dismiss-trip', selectedCostDrive)"
            class="btn btn-lg btn-secondary"
            :title="$t('drives.tripSuggestions.dismissTitle')"
          >
            {{ $t('drives.tripSuggestions.dismiss') }}
          </button>
          <button
            type="button"
            @click="emit('create-trip', selectedCostDrive)"
            class="btn btn-lg btn-primary"
          >
            <Layers class="w-4 h-4" />
            <span>{{ $t('drives.tripSuggestions.create') }}</span>
          </button>
        </div>
        <div v-else-if="!selectedCostDrive.is_suggestion" class="flex items-center gap-2">
          <button
            v-if="selectedCostDrive.is_trip_group && vehicleStore.canEdit"
            type="button"
            @click="emit('edit-trip', selectedCostDrive)"
            class="btn btn-lg btn-secondary"
          >
            <Pencil class="w-4 h-4" />
            <span>{{ $t('common.edit') }}</span>
          </button>
          <button
            @click="open = false; router.push({ path: '/carpools', query: selectedCostDrive.is_trip_group ? { new_trip_group_id: selectedCostDrive.id } : { new_drive_id: selectedCostDrive.id } })"
            class="btn btn-lg btn-primary"
          >
            <Users class="w-4 h-4" />
            <span>{{ $t('drives.driveCostModal.shareAsACarpool') }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
