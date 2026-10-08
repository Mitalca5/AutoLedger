<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Archive } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { getLastDismountInfo, isMountedPosition } from '@/utils/tires'
import { todayIso } from '@/utils/dates'
import { useOdometerPrefill } from '@/composables/useOdometerPrefill'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

// Dispose (worn out, damaged, sold) keeps history and cost; deleting a tire removes an erroneous entry
const props = defineProps<{ vehicleId: string; selectedTire: any | null; tires: any[]; currentOdometer: number }>()
const emit = defineEmits<{ saved: [tireId: string] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const disposeForm = ref({ date: todayIso(), odometer: 0 as number | string })

const odometerPrefill = useOdometerPrefill({
  vehicleId: () => props.vehicleId,
  enabled: () => open.value && isMountedPosition(props.selectedTire?.current_position),
  date: () => disposeForm.value.date,
  current: () => disposeForm.value.odometer,
  fill: (km) => {
    disposeForm.value.odometer = km
  },
})

watch(open, (isOpen) => {
  if (!isOpen) return
  const t = props.selectedTire
  const isMounted = t && isMountedPosition(t.current_position)
  const lastDismount = t ? getLastDismountInfo(props.tires, t.id) : null

  disposeForm.value = {
    date: (!isMounted && lastDismount?.date) ? lastDismount.date : todayIso(),
    odometer: isMounted ? Math.round(props.currentOdometer || 0) : (lastDismount?.odometer || ''),
  }
  odometerPrefill.reset(isMounted ? Math.round(props.currentOdometer || 0) : null)
})

const { pending: submitting, run: runOnce } = useSubmit()

async function handleDisposeTireAction() {
  if (!props.vehicleId || !props.selectedTire) return
  const tireId = props.selectedTire.id
  try {
    await api.disposeTire(props.vehicleId, tireId, {
      date: new Date(disposeForm.value.date).toISOString(),
      odometer: disposeForm.value.odometer === '' ? null : Number(disposeForm.value.odometer),
    })
    open.value = false
    emit('saved', tireId)
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleDisposeTire = () => runOnce(handleDisposeTireAction)
</script>

<template>
  <ModalShell
    v-if="selectedTire"
    v-model:open="open"
    :title="$t('tires.tireDisposeModal.scrap')"
    :icon="Archive"
    icon-class="text-warning-400"
    size="sm"
  >
    <form id="tire-dispose-modal-form" @submit.prevent="handleDisposeTire" class="space-y-4">
      <p class="text-xs text-slate-300 font-semibold">
        {{ selectedTire.brand }} {{ selectedTire.model }}
      </p>
      <div>
        <label for="tire-dispose-date" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('common.date') }}</label>
        <AppDatePicker
          id="tire-dispose-date"
          v-model="disposeForm.date"
          required
          size="xs"
        />
      </div>
      <div v-if="['FL', 'FR', 'RL', 'RR'].includes(selectedTire.current_position)">
        <label for="tire-dispose-odometer" class="block text-xs text-slate-400 mb-1 font-semibold">{{ $t('tires.tireDisposeModal.odometerAtRemovalKm', { unit: distanceUnit() }) }}</label>
        <DistanceInput id="tire-dispose-odometer" v-model="disposeForm.odometer" min="0" required class="field" />
      </div>
    </form>
    <template #footer>
      <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
        {{ $t('common.cancel') }}
      </button>
      <button :disabled="submitting" type="submit" form="tire-dispose-modal-form" class="bg-warning-600 hover:bg-warning-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors">
        {{ $t('tires.tireDisposeModal.scrap') }}
      </button>
    </template>
  </ModalShell>
</template>