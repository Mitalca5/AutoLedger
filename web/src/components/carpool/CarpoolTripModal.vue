<script setup lang="ts">
import { t } from '@/i18n'
import { computed, ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { Users, X, Lock } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import DrivePicker from '@/components/drives/DrivePicker.vue'
import { COST_FIELDS, allocate, cents, clampPassengerStops as clampStops, earliestSelectedDriveDate, emptyLeg, estimateTitle, legsFromEstimate, legsFromTrip, newPassenger, passengersFromTrip, remapPassengerStops, stopNames, toDateInputString, type LegForm, type PassengerForm } from '@/utils/carpool'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { formatAmount } from '@/currency'

import CarpoolLegs from '@/components/carpool/CarpoolLegs.vue'
import CarpoolPassengers from '@/components/carpool/CarpoolPassengers.vue'
import CarpoolSimulation from '@/components/carpool/CarpoolSimulation.vue'

// Creates a carpool trip, or edits `editing`. A new trip can start from drives or a trip group (createOptions).
// openToken changes every time the page asks to open the modal, so the form is initialised again even if it is already open.
const props = defineProps<{
  vehicleId: string
  editing: any | null
  createOptions: { driveIds?: string[]; tripGroupId?: string }
  openToken: number
}>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const editingTripId = computed<string | null>(() => props.editing?.id ?? null)
const modalSubmitting = ref(false)
const estimating = ref(false)
const vehicleStore = useVehicleStore()
// Carpool amounts are in the vehicle's own currency
const fmt = (v: number) => formatAmount(Number(v || 0), vehicleStore.currency)
// Trips are built from TeslaMate drives when the vehicle has them, from a typed distance otherwise
const defaultSourceMode = (): 'DRIVES' | 'MANUAL' => (vehicleStore.hasTeslaMate ? 'DRIVES' : 'MANUAL')
const sourceMode = ref<'DRIVES' | 'MANUAL'>(defaultSourceMode())
// Drives listed by the picker (it reports them: the earliest selected one dates a carpool without an estimate date)
const recentDrives = ref<any[]>([])
// Date the picker opens on: the carpool's own date, or none for the latest drives
const pickerAnchor = ref('')
const selectedDriveIds = ref<string[]>([])
const titleTouched = ref(false)
const currentRates = ref<any>(null)
const expandedPassengerIndex = ref<number | null>(null)

const isDateDisabled = computed(() => sourceMode.value === 'DRIVES')

const form = ref({
  title: '',
  date: toDateInputString(new Date()),
  trip_group_id: null as string | null,
  notes: '',
  legs: [] as LegForm[],
  passengers: [] as PassengerForm[],
})

// ---------- Live form computations ----------
const stops = computed(() => stopNames(form.value.legs))
const live = computed(() => allocate(form.value.legs, form.value.passengers))
const liveDistance = computed(() => form.value.legs.reduce((s, l) => s + (Number(l.distance_km) || 0), 0))
const liveRevenue = computed(() => form.value.passengers.reduce((s, p) => s + cents(p.amount_paid), 0))
const liveNet = computed(() => live.value.total - liveRevenue.value)
const liveCoverage = computed(() => (live.value.total > 0 ? Math.min(100, Math.round((liveRevenue.value / live.value.total) * 1000) / 10) : 0))

// After the drives are chosen (from a trip group or the drives page), the picker moves to their date
function anchorPickerOnSelection() {
  if (selectedDriveIds.value.length && form.value.date) pickerAnchor.value = form.value.date
}

function clampPassengerStops(previousLegCount: number) {
  clampStops(form.value.passengers, form.value.legs.length, previousLegCount)
}

// ---------- Estimation ----------
function applyEstimate(est: any) {
  currentRates.value = est
  const previousLegs = form.value.legs
  const previousLegCount = previousLegs.length
  form.value.legs = legsFromEstimate(est)
  remapPassengerStops(previousLegs, form.value.legs, form.value.passengers)
  clampPassengerStops(previousLegCount === 0 ? 0 : -1)
  if (est.start_date) {
    form.value.date = toDateInputString(est.start_date)
  }
  if (!titleTouched.value && form.value.legs.length) {
    form.value.title = estimateTitle(stops.value, form.value.legs.length)
  }
}

async function estimateFromDrives() {
  if (!props.vehicleId) return
  if (!selectedDriveIds.value.length) {
    form.value.legs = []
    form.value.date = toDateInputString(new Date())
    return
  }
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(props.vehicleId, { drive_ids: selectedDriveIds.value })
    applyEstimate(est)
    if (est.start_date) {
      form.value.date = toDateInputString(est.start_date)
    } else {
      const first = earliestSelectedDriveDate(recentDrives.value, selectedDriveIds.value)
      if (first) form.value.date = first
    }
  } catch (err: any) {
    showAlert(t('carpool.carpoolTripModal.estimateError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    estimating.value = false
  }
}

async function toggleDrive(driveId: string) {
  const idx = selectedDriveIds.value.indexOf(driveId)
  if (idx > -1) selectedDriveIds.value.splice(idx, 1)
  else selectedDriveIds.value.push(driveId)
  await estimateFromDrives()
}

async function estimateManualLeg(index: number) {
  const leg = form.value.legs[index]
  if (!props.vehicleId || !(Number(leg.distance_km) > 0)) {
    showAlert(t('carpool.carpoolTripModal.legDistanceFirst'), t('common.requiredField'), 'warning')
    return
  }
  estimating.value = true
  try {
    const est = await api.estimateCarpoolCosts(props.vehicleId, { distance_km: Number(leg.distance_km) })
    currentRates.value = est
    const estimated = est.legs?.[0] || est
    leg.electricity_cost = estimated.electricity_cost
    leg.tires_cost = estimated.tires_cost
    leg.maintenance_cost = estimated.maintenance_cost
    leg.insurance_cost = estimated.insurance_cost
  } catch (err: any) {
    showAlert(t('carpool.carpoolTripModal.estimateError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    estimating.value = false
  }
}

function addManualLeg() {
  const previous = form.value.legs.length
  const leg = emptyLeg()
  if (previous) leg.start_label = form.value.legs[previous - 1].end_label
  form.value.legs.push(leg)
  clampPassengerStops(previous)
}

function removeLeg(index: number) {
  if (form.value.legs.length <= 1) return
  const previous = form.value.legs.length
  form.value.legs.splice(index, 1)
  clampPassengerStops(previous)
}

function switchSource(mode: 'DRIVES' | 'MANUAL') {
  if (sourceMode.value === mode) return
  sourceMode.value = mode
  if (mode === 'MANUAL') {
    selectedDriveIds.value = []
    form.value.legs = form.value.legs.map((l) => ({ ...l, drive_id: null }))
    if (!form.value.legs.length) addManualLeg()
  }
}

// ---------- Initialisation ----------
function resetForm() {
  titleTouched.value = false
  currentRates.value = null
  expandedPassengerIndex.value = null
  selectedDriveIds.value = []
  pickerAnchor.value = ''
  sourceMode.value = defaultSourceMode()
  form.value = {
    title: '',
    date: toDateInputString(new Date()),
    trip_group_id: null,
    notes: '',
    legs: [],
    passengers: [],
  }
  form.value.passengers.push({ ...newPassenger(0, form.value.legs.length), passenger_name: t('carpool.passenger', { n: 1 }) })
}

async function initCreate(options: { driveIds?: string[]; tripGroupId?: string }) {
  resetForm()
  if (sourceMode.value === 'MANUAL') {
    addManualLeg()
  }
  if (!props.vehicleId) return

  if (options.tripGroupId) {
    form.value.trip_group_id = options.tripGroupId
    estimating.value = true
    try {
      const est = await api.estimateCarpoolCosts(props.vehicleId, { trip_group_id: options.tripGroupId })
      selectedDriveIds.value = (est.legs || []).map((l: any) => l.drive_id).filter(Boolean)
      applyEstimate(est)
      const groups = await api.getTripGroups(props.vehicleId)
      const group = (groups || []).find((g: any) => g.id === options.tripGroupId)
      if (group?.name) {
        form.value.title = group.name
        titleTouched.value = true
      }
      if (est.start_date) {
        form.value.date = toDateInputString(est.start_date)
      } else if (group?.start_time) {
        form.value.date = toDateInputString(group.start_time)
      }
      anchorPickerOnSelection()
    } catch (err: any) {
      showAlert(t('carpool.carpoolTripModal.estimateError', { message: err.message }), t('shell.confirm.error'), 'danger')
    } finally {
      estimating.value = false
    }
  } else if (options.driveIds?.length) {
    selectedDriveIds.value = [...options.driveIds]
    await estimateFromDrives()
    anchorPickerOnSelection()
  }
}

function initEdit(trip: any) {
  resetForm()
  titleTouched.value = true
  const legs = legsFromTrip(trip)
  sourceMode.value = legs.some((l) => l.drive_id) ? 'DRIVES' : 'MANUAL'
  selectedDriveIds.value = legs.map((l) => l.drive_id).filter((id): id is string => !!id)
  form.value = {
    title: trip.title,
    date: toDateInputString(trip.date),
    trip_group_id: trip.trip_group_id || null,
    notes: trip.notes || '',
    legs,
    passengers: passengersFromTrip(trip, legs.length),
  }
  if (!form.value.passengers.length) form.value.passengers.push(newPassenger(0, legs.length))
  pickerAnchor.value = form.value.date
}

watch([open, () => props.openToken], ([isOpen]) => {
  if (!isOpen) return
  submitted.value = false
  if (props.editing) initEdit(props.editing)
  else initCreate(props.createOptions)
})

const submitted = ref(false)
const titleError = computed(() => (!form.value.title.trim() ? t('carpool.carpoolTripModal.titleRequired') : ''))
const legError = computed(() => (!form.value.legs.length ? t('carpool.carpoolTripModal.legRequired') : ''))

async function handleSave() {
  if (!props.vehicleId) return
  submitted.value = true
  if (titleError.value) {
    document.getElementById('carpool-title')?.focus()
    return
  }
  if (legError.value) return
  modalSubmitting.value = true
  try {
    const payload = {
      title: form.value.title,
      date: new Date(form.value.date).toISOString(),
      trip_group_id: form.value.trip_group_id,
      notes: form.value.notes || null,
      legs: form.value.legs.map((l) => ({
        ...l,
        distance_km: Number(l.distance_km) || 0,
        ...Object.fromEntries(COST_FIELDS.map((f) => [f.key, Number(l[f.key]) || 0])),
      })),
      passengers: form.value.passengers.map((p) => ({
        ...p,
        seats: Number(p.seats) || 1,
        amount_paid: Number(p.amount_paid) || 0,
        notes: p.notes || null,
      })),
    }
    if (editingTripId.value) {
      await api.updateCarpool(props.vehicleId, editingTripId.value, payload)
    } else {
      await api.createCarpool(props.vehicleId, payload)
    }
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('carpool.carpoolTripModal.saveError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    modalSubmitting.value = false
  }
}

async function handleModalRecalculate() {
  if (!props.vehicleId) return
  if (sourceMode.value === 'DRIVES') {
    await estimateFromDrives()
  } else {
    for (let i = 0; i < form.value.legs.length; i++) {
      if (Number(form.value.legs[i].distance_km) > 0) {
        await estimateManualLeg(i)
      }
    }
  }
  showAlert(t('carpool.carpoolTripModal.reestimated'), t('carpool.carpoolTripModal.reestimatedTitle'), 'success')
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-modal bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-4xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Users class="w-5 h-5 text-rose-400" />
          {{ editingTripId ? $t('carpool.carpoolTripModal.edit') : $t('carpool.carpoolView.newCarpool') }}
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-5">

      <!-- Source -->
      <div class="space-y-3">
        <div v-if="vehicleStore.hasTeslaMate || sourceMode === 'DRIVES'" class="flex items-center gap-1 bg-slate-950 border border-slate-800 p-1 rounded-xl w-fit text-xs font-semibold">
          <button
            @click="switchSource('DRIVES')"
            class="px-3 py-1.5 rounded-lg"
            :class="sourceMode === 'DRIVES' ? 'bg-rose-600 text-white' : 'text-slate-400 hover:text-white'"
          >
            {{ $t('carpool.carpoolTripModal.teslamateDrives') }}
          </button>
          <button
            @click="switchSource('MANUAL')"
            class="px-3 py-1.5 rounded-lg"
            :class="sourceMode === 'MANUAL' ? 'bg-rose-600 text-white' : 'text-slate-400 hover:text-white'"
          >
            {{ $t('carpool.carpoolTripModal.manualEntry') }}
          </button>
        </div>

        <div v-if="sourceMode === 'DRIVES'" class="space-y-1.5">
          <div class="flex items-center justify-between text-xs">
            <span class="text-slate-400">{{ $t('carpool.carpoolTripModal.tickTheDrivesThatMake') }}</span>
            <span class="text-rose-400 font-semibold">{{ $t('carpool.carpoolTripModal.legsSelected', { count: selectedDriveIds.length }) }}{{ estimating ? $t('carpool.carpoolTripModal.estimating') : '' }}</span>
          </div>
          <DrivePicker
            :key="openToken"
            :vehicle-id="vehicleId"
            :selected-ids="selectedDriveIds"
            :anchor-date="pickerAnchor"
            @toggle="toggleDrive"
            @loaded="recentDrives = $event"
          />
        </div>
      </div>

      <!-- Title & date -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div class="sm:col-span-2">
          <label for="carpool-title" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('carpool.carpoolTripModal.title') }}</label>
          <input
            id="carpool-title"
            v-model="form.title"
            @input="titleTouched = true"
            :placeholder="$t('carpool.carpoolTripModal.eGAnnecyValence')"
            :aria-invalid="submitted && !!titleError"
            :aria-describedby="submitted && titleError ? 'carpool-title-error' : undefined"
            class="field"
          />
          <p v-if="submitted && titleError" id="carpool-title-error" class="text-xs text-danger-400 mt-1">{{ titleError }}</p>
        </div>
        <div>
          <div class="flex items-center justify-between mb-1">
            <label for="carpool-date" class="block text-xs font-semibold text-slate-400">{{ $t('common.date') }}</label>
            <span v-if="isDateDisabled" class="text-xs text-slate-400 flex items-center gap-1 font-normal" :title="$t('carpool.carpoolTripModal.theDateIsAutomaticallyLinked')">
              <Lock class="w-3 h-3 text-slate-400" />
              {{ $t('carpool.carpoolTripModal.tripDate') }}
            </span>
          </div>
          <AppDatePicker
            id="carpool-date"
            v-model="form.date"
            :disabled="isDateDisabled"
            required
          />
        </div>
      </div>

      <!-- Legs -->
      <CarpoolLegs
        :legs="form.legs"
        :source-mode="sourceMode"
        :is-editing="!!editingTripId"
        :submitted="submitted"
        :estimating="estimating"
        :live="live"
        :live-distance="liveDistance"
        :currency="vehicleStore.currency"
        @recalculate="handleModalRecalculate"
        @add-leg="addManualLeg"
        @estimate="estimateManualLeg"
        @remove="removeLeg"
      />

      <!-- Passengers -->
      <CarpoolPassengers
        v-model:passengers="form.passengers"
        v-model:expanded="expandedPassengerIndex"
        :legs="form.legs"
        :stops="stops"
        :live="live"
        :currency="vehicleStore.currency"
      />

      <!-- Simulation -->
      <CarpoolSimulation :live="live" :live-revenue="liveRevenue" :live-coverage="liveCoverage" :live-net="liveNet" :currency="vehicleStore.currency" />

      <div>
        <label for="carpool-notes" class="block text-xs font-semibold text-slate-400 mb-1">{{ $t('carpool.carpoolTripModal.notesOptional') }}</label>
        <input id="carpool-notes" v-model="form.notes" class="field" />
      </div>

      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="btn btn-lg btn-secondary"
        >
          {{ $t('common.cancel') }}
        </button>
        <button
          type="button"
          @click="handleSave"
          :disabled="modalSubmitting || estimating"
          class="btn btn-lg btn-primary"
        >
          {{ modalSubmitting ? $t('carpool.carpoolTripModal.saving') : $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
