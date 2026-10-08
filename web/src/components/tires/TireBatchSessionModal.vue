<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Check, History } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { todayIso } from '@/utils/dates'
import { useOdometerPrefill } from '@/composables/useOdometerPrefill'
import { distanceUnit } from '@/units'

// Records the same past session on several tires stored in the garage
const props = defineProps<{ vehicleId: string; storageTires: any[]; selectedTireIds: string[]; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
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
  <ModalShell
    v-model:open="open"
    :title="$t('tires.tireBatchSessionModal.addAPastSessionOn')"
    :icon="History"
    icon-class="text-rose-400"
    body-class="space-y-4 text-xs"
    footer-class="items-center justify-end gap-2"
  >
    <!-- Tire selection from storage -->
    <div>
      <div class="flex items-center justify-between mb-2">
        <span class="font-semibold text-slate-300">
          {{ $t('tires.tireBatchSessionModal.garageTiresConcerned', { length: batchSessionTireIds.length, length2: storageTires.length }) }}
        </span>
        <div class="flex items-center gap-2 text-xs">
          <button type="button" @click="selectAllBatchSessionTires()" class="text-rose-400 hover:text-rose-300 font-semibold">
            {{ $t('tires.tireBatchSessionModal.tickAll') }}
          </button>
          <span class="text-slate-400">|</span>
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
            <div class="text-xs text-slate-400 truncate">{{ t.tire.dimension }}</div>
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
          class="field"
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
          class="field"
        />
      </div>
    </div>

    <div>
      <label for="batch-session-distance-km" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.sessionDistanceKm', { unit: distanceUnit() }) }}</label>
      <DistanceInput
        id="batch-session-distance-km"
        v-model="batchSessionForm.distance_km"
        :placeholder="$t('tires.tireBatchSessionModal.calculatedFromTheOdometersOr')"
        class="field"
      />
    </div>

    <div>
      <label for="batch-session-notes" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireBatchSessionModal.commentNotes') }}</label>
      <input
        id="batch-session-notes"
        v-model="batchSessionForm.notes"
        type="text"
        :placeholder="$t('tires.tireBatchSessionModal.eGWinterSeason2023')"
        class="field"
      />
    </div>
    <template #footer>
      <button
        type="button"
        @click="open = false"
        class="btn btn-lg btn-secondary"
      >
        {{ $t('common.cancel') }}
      </button>
      <button
        type="button"
        @click="handleSaveBatchSession()"
        :disabled="savingBatchSession || batchSessionTireIds.length === 0"
        class="btn btn-lg btn-primary"
      >
        <Check class="w-4 h-4" />
        <span>{{ savingBatchSession ? $t('tires.tireBatchSessionModal.saving') : $t('tires.tireBatchSessionModal.applyTo', { count: batchSessionTireIds.length }) }}</span>
      </button>
    </template>
  </ModalShell>
</template>