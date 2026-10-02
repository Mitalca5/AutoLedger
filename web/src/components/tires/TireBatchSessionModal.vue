<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Check, History, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { useOdometerPrefill } from '@/composables/useOdometerPrefill'
import { distanceUnit } from '@/units'

// Records the same past session on several tires stored in the garage
const props = defineProps<{ vehicleId: string; storageTires: any[]; selectedTireIds: string[]; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const batchSessionTireIds = ref<string[]>([])
const savingBatchSession = ref(false)
const batchSessionForm = ref({
  mounted_date: todayIso(),
  mounted_odometer: 0,
  dismounted_date: todayIso(),
  dismounted_odometer: 0,
  distance_km: 0,
  notes: '',
  position: 'STORAGE',
})

const mountedPrefill = useOdometerPrefill({
  vehicleId: () => props.vehicleId,
  enabled: () => open.value,
  date: () => batchSessionForm.value.mounted_date,
  current: () => batchSessionForm.value.mounted_odometer,
  fill: (km) => {
    batchSessionForm.value.mounted_odometer = km
    onBatchOdometerChange()
  },
})
const dismountedPrefill = useOdometerPrefill({
  vehicleId: () => props.vehicleId,
  enabled: () => open.value,
  date: () => batchSessionForm.value.dismounted_date,
  current: () => batchSessionForm.value.dismounted_odometer,
  fill: (km) => {
    batchSessionForm.value.dismounted_odometer = km
    onBatchOdometerChange()
  },
})

watch(open, (isOpen) => {
  if (!isOpen) return
  const storageIds = props.storageTires.map((t) => t.tire.id)
  const selectedStorage = props.selectedTireIds.filter((id) => storageIds.includes(id))
  batchSessionTireIds.value = selectedStorage.length > 0 ? [...selectedStorage] : [...storageIds]

  const curOdo = Math.round(props.currentOdometer || 0)
  batchSessionForm.value = {
    mounted_date: todayIso(),
    mounted_odometer: curOdo,
    dismounted_date: todayIso(),
    dismounted_odometer: curOdo,
    distance_km: 0,
    notes: '',
    position: 'STORAGE',
  }
  mountedPrefill.reset(curOdo)
  dismountedPrefill.reset(curOdo)
})

function toggleBatchSessionTire(id: string) {
  if (batchSessionTireIds.value.includes(id)) {
    batchSessionTireIds.value = batchSessionTireIds.value.filter((x) => x !== id)
  } else {
    batchSessionTireIds.value.push(id)
  }
}

function selectAllBatchSessionTires() {
  batchSessionTireIds.value = props.storageTires.map((t) => t.tire.id)
}

function deselectAllBatchSessionTires() {
  batchSessionTireIds.value = []
}

function onBatchOdometerChange() {
  const mount = Number(batchSessionForm.value.mounted_odometer) || 0
  const dismount = Number(batchSessionForm.value.dismounted_odometer) || 0
  if (dismount > mount) {
    batchSessionForm.value.distance_km = dismount - mount
  }
}

async function handleSaveBatchSession() {
  if (!props.vehicleId || batchSessionTireIds.value.length === 0) return
  if (!batchSessionForm.value.mounted_date || !batchSessionForm.value.dismounted_date) {
    showAlert(t('tires.tireBatchSessionModal.datesRequired'), t('tires.tireBatchSessionModal.datesRequiredTitle'), 'warning')
    return
  }

  savingBatchSession.value = true
  try {
    const payload: any = {
      position: 'STORAGE',
      mounted_date: new Date(batchSessionForm.value.mounted_date).toISOString(),
      mounted_odometer: Number(batchSessionForm.value.mounted_odometer) || 0,
      dismounted_date: new Date(batchSessionForm.value.dismounted_date).toISOString(),
      dismounted_odometer: Number(batchSessionForm.value.dismounted_odometer) || 0,
      distance_km: Number(batchSessionForm.value.distance_km) || 0,
      notes: batchSessionForm.value.notes ? batchSessionForm.value.notes : null,
    }

    if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
      payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
    }

    for (const tireId of batchSessionTireIds.value) {
      await api.createTireSession(props.vehicleId, tireId, payload)
    }

    open.value = false
    emit('saved')
    showAlert(t('tires.tireBatchSessionModal.saved', { count: batchSessionTireIds.value.length }), t('common.success'), 'success')
  } catch (err: any) {
    showAlert(t('tires.tireBatchSessionModal.error', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    savingBatchSession.value = false
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2">
          <History class="w-5 h-5 text-rose-400" />
          <h3 class="text-base font-bold text-white">
            {{ $t('tires.tireBatchSessionModal.addAPastSessionOn') }}
          </h3>
        </div>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
        <!-- Tire selection from storage -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <span class="font-semibold text-slate-300">
              {{ $t('tires.tireBatchSessionModal.garageTiresConcerned', { length: batchSessionTireIds.length, length2: storageTires.length }) }}
            </span>
            <div class="flex items-center gap-2 text-[11px]">
              <button type="button" @click="selectAllBatchSessionTires()" class="text-rose-400 hover:text-rose-300 font-semibold">
                {{ $t('tires.tireBatchSessionModal.tickAll') }}
              </button>
              <span class="text-slate-600">|</span>
              <button type="button" @click="deselectAllBatchSessionTires()" class="text-slate-400 hover:text-slate-200">
                {{ $t('tires.tireBatchSessionModal.untickAll') }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 max-h-40 overflow-y-auto p-1 bg-slate-950/60 rounded-xl border border-slate-800/80">
            <label
              v-for="t in storageTires"
              :key="t.tire.id"
              class="flex items-center gap-2.5 p-2 rounded-lg hover:bg-slate-800/50 cursor-pointer text-slate-200"
            >
              <input
                type="checkbox"
                :checked="batchSessionTireIds.includes(t.tire.id)"
                @change="toggleBatchSessionTire(t.tire.id)"
                class="rounded accent-rose-500 w-4 h-4"
              />
              <div class="min-w-0 flex-1">
                <div class="font-bold truncate text-white">{{ t.tire.brand }} {{ t.tire.model }}</div>
                <div class="text-[10px] text-slate-400 truncate">{{ t.tire.dimension }}</div>
              </div>
            </label>
          </div>
        </div>

        <!-- Dates & Odometers -->
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="batch-session-mounted-date" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.fittingDate') }}</label>
            <AppDatePicker
              id="batch-session-mounted-date"
              v-model="batchSessionForm.mounted_date"
              size="sm"
              :clearable="true"
            />
          </div>
          <div>
            <label for="batch-session-mounted-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.odometerAtFittingKm', { unit: distanceUnit() }) }}</label>
            <DistanceInput
              id="batch-session-mounted-odometer"
              v-model="batchSessionForm.mounted_odometer"
              @input="onBatchOdometerChange"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
            />
          </div>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="batch-session-dismounted-date" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.removalDate') }}</label>
            <AppDatePicker
              id="batch-session-dismounted-date"
              v-model="batchSessionForm.dismounted_date"
              size="sm"
              :clearable="true"
            />
          </div>
          <div>
            <label for="batch-session-dismounted-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.odometerAtRemovalKm', { unit: distanceUnit() }) }}</label>
            <DistanceInput
              id="batch-session-dismounted-odometer"
              v-model="batchSessionForm.dismounted_odometer"
              @input="onBatchOdometerChange"
              class="w-full bg-slate-800 text-slate-100 rounded-xl px-2.5 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
            />
          </div>
        </div>

        <div>
          <label for="batch-session-distance-km" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.sessionDistanceKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput
            id="batch-session-distance-km"
            v-model="batchSessionForm.distance_km"
            :placeholder="$t('tires.tireBatchSessionModal.calculatedFromTheOdometersOr')"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
          />
        </div>

        <div>
          <label for="batch-session-notes" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.commentNotes') }}</label>
          <input
            id="batch-session-notes"
            v-model="batchSessionForm.notes"
            type="text"
            :placeholder="$t('tires.tireBatchSessionModal.eGWinterSeason2023')"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:border-rose-500 focus:outline-none"
          />
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
        >
          {{ $t('common.cancel') }}
        </button>
        <button
          type="button"
          @click="handleSaveBatchSession()"
          :disabled="savingBatchSession || batchSessionTireIds.length === 0"
          class="px-4 py-2 bg-rose-600 hover:bg-rose-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl shadow-lg shadow-rose-600/20 transition-all flex items-center gap-1.5"
        >
          <Check class="w-4 h-4" />
          <span>{{ savingBatchSession ? $t('tires.tireBatchSessionModal.saving') : $t('tires.tireBatchSessionModal.applyTo', { count: batchSessionTireIds.length }) }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
