<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch, onMounted } from 'vue'
import { Gauge, Edit2, Trash2 } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { api } from '@/services/api'
import { distanceUnit, formatDistance, formatDistanceValue } from '@/units'

const props = defineProps<{
  vehicle: any
  canEdit: boolean
}>()

const vehicleStore = useVehicleStore()
const { showConfirm, showAlert } = useConfirm()

const readings = ref<any[]>([])
const loading = ref(false)
const editingId = ref<string | null>(null)
const form = ref({
  date: new Date().toISOString().substring(0, 10),
  odometer: '',
  notes: '',
})

async function load() {
  loading.value = true
  try {
    readings.value = await api.getOdometerCheckpoints(props.vehicle.id)
  } catch (err: any) {
    showAlert(t('manual.odometerReadingsPanel.loadError', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    loading.value = false
  }
}

function resetForm() {
  editingId.value = null
  form.value = {
    date: new Date().toISOString().substring(0, 10),
    odometer: props.vehicle?.current_odometer ? String(Math.round(props.vehicle.current_odometer)) : '',
    notes: '',
  }
}

function startEdit(r: any) {
  editingId.value = r.id
  form.value = {
    date: new Date(r.date).toISOString().substring(0, 10),
    odometer: String(Math.round(r.odometer)),
    notes: r.notes || '',
  }
}

async function save() {
  const odo = Number(form.value.odometer)
  if (form.value.odometer === '' || Number.isNaN(odo) || odo < 0) {
    showAlert(t('manual.odometerReadingsPanel.validOdometer'), t('common.requiredField'), 'warning')
    return
  }
  try {
    const payload = {
      date: form.value.date,
      odometer: odo,
      notes: form.value.notes.trim() || undefined,
    }
    if (editingId.value) {
      await api.updateOdometerCheckpoint(props.vehicle.id, editingId.value, payload)
    } else {
      await api.createOdometerCheckpoint(props.vehicle.id, payload)
    }
    resetForm()
    await load()
    // A reading raises the odometer of a combustion vehicle and reshapes the monthly smoothing
    await vehicleStore.fetchVehicles()
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

async function remove(r: any) {
  const ok = await showConfirm({
    title: t('manual.odometerReadingsPanel.deleteTitle'),
    message: t('manual.odometerReadingsPanel.deleteMessage', { unit: distanceUnit(), km: formatDistanceValue(r.odometer), date: new Date(r.date).toLocaleDateString(intlLocale()) }),
    confirmText: t('common.delete'),
    type: 'danger',
  })
  if (!ok) return
  try {
    await api.deleteOdometerCheckpoint(props.vehicle.id, r.id)
    await load()
    vehicleStore.lastSyncTimestamp = Date.now()
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}

watch(() => props.vehicle?.id, () => {
  resetForm()
  load()
})
onMounted(() => {
  resetForm()
  load()
})
</script>

<template>
  <div class="space-y-5">
    <p class="text-xs text-slate-400">
      {{ $t('manual.odometerReadingsPanel.odometerReadingsSpreadTheUntracked') }}
    </p>

    <form v-if="canEdit" class="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-4" @submit.prevent="save">
      <div class="flex items-center justify-between">
        <span class="text-xs font-bold text-cyan-400 uppercase tracking-wider">
          {{ editingId ? $t('manual.odometerReadingsPanel.edit') : $t('manual.odometerReadingsPanel.new') }}
        </span>
        <button v-if="editingId" type="button" class="text-xs text-slate-400 hover:text-white" @click="resetForm">
          {{ $t('manual.odometerReadingsPanel.cancelTheEdit') }}
        </button>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label for="checkpoint-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.odometerReadingsPanel.readingDate') }}</label>
          <AppDatePicker id="checkpoint-date" v-model="form.date" required size="sm" />
        </div>
        <div>
          <label for="checkpoint-odometer" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.odometerReadingsPanel.mileageKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput text
            id="checkpoint-odometer"
            v-model="form.odometer"
            step="1"
            min="0"
            max="2000000"
            required
            :placeholder="$t('common.example', { value: '45000' })"
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-cyan-500"
          />
        </div>
        <div>
          <label for="checkpoint-notes" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('manual.odometerReadingsPanel.reasonEvent') }}</label>
          <input
            id="checkpoint-notes"
            v-model="form.notes"
            type="text"
            :placeholder="$t('manual.odometerReadingsPanel.eGRoadworthinessInspection')"
            class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-cyan-500"
          />
        </div>
      </div>

      <div class="flex justify-end pt-1">
        <button type="submit" class="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 shadow-lg shadow-cyan-600/20">
          <span>{{ editingId ? $t('manual.odometerReadingsPanel.update') : $t('manual.odometerReadingsPanel.add') }}</span>
        </button>
      </div>
    </form>

    <div class="space-y-3">
      <div class="flex items-center justify-between">
        <h4 class="text-xs font-bold text-white uppercase tracking-wider">{{ $t('manual.odometerReadingsPanel.historyOfRecordedReadings') }}</h4>
        <span class="text-xs text-slate-400">{{ $t('manual.odometerReadingsPanel.readingS', { length: readings.length }) }}</span>
      </div>

      <div v-if="loading" class="py-8 text-center text-xs text-slate-400">{{ $t('manual.odometerReadingsPanel.loadingTheReadings') }}</div>

      <div v-else-if="readings.length === 0" class="py-8 text-center bg-slate-950/40 rounded-xl border border-slate-800 text-xs text-slate-400">
        {{ $t('manual.odometerReadingsPanel.noManualReadingYet') }}
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="r in readings"
          :key="r.id"
          class="bg-slate-950/70 border border-slate-800/80 rounded-xl p-3.5 flex items-center justify-between gap-3"
        >
          <div class="flex items-center gap-3 min-w-0">
            <div class="p-2 bg-slate-800/80 text-cyan-400 rounded-lg shrink-0">
              <Gauge class="w-4 h-4" />
            </div>
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="text-sm font-bold text-white font-mono">{{ formatDistance(r.odometer) }}</span>
                <span v-if="r.source === 'HA'" class="text-xs font-semibold uppercase tracking-wider px-1.5 py-0.5 rounded-md bg-cyan-500/15 text-cyan-300 border border-cyan-500/30" :title="$t('manual.odometerReadingsPanel.fromHomeAssistantHint')">{{ $t('manual.odometerReadingsPanel.fromHomeAssistant') }}</span>
                <span class="text-xs text-slate-400">
                  {{ $t('manual.odometerReadingsPanel.onDate', { date: new Date(r.date).toLocaleDateString(intlLocale(), { day: 'numeric', month: 'short', year: 'numeric' }) }) }}
                </span>
              </div>
              <p v-if="r.notes" class="text-xs text-slate-400 truncate mt-0.5">{{ r.notes }}</p>
            </div>
          </div>

          <div v-if="canEdit" class="flex items-center gap-1.5 shrink-0">
            <button v-if="r.source !== 'HA'" type="button" class="p-1.5 text-slate-400 hover:text-cyan-400 hover:bg-slate-800 rounded-lg transition-colors" :aria-label="$t('manual.odometerReadingsPanel.editReading', { unit: distanceUnit(), km: formatDistanceValue(r.odometer) })" @click="startEdit(r)">
              <Edit2 class="w-4 h-4" />
            </button>
            <button type="button" class="p-1.5 text-slate-400 hover:text-danger-400 hover:bg-slate-800 rounded-lg transition-colors" :aria-label="$t('manual.odometerReadingsPanel.deleteReading', { unit: distanceUnit(), km: formatDistanceValue(r.odometer) })" @click="remove(r)">
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
