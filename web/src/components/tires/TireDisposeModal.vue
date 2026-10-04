<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Archive, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { getLastDismountInfo, isMountedPosition } from '@/utils/tires'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { useOdometerPrefill } from '@/composables/useOdometerPrefill'
import { distanceUnit } from '@/units'

// Dispose (worn out, damaged, sold) keeps history and cost; deleting a tire removes an erroneous entry
const props = defineProps<{ vehicleId: string; selectedTire: any | null; tires: any[]; currentOdometer: number }>()
const emit = defineEmits<{ saved: [tireId: string] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
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

async function handleDisposeTire() {
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
</script>

<template>
  <div
    v-if="open && selectedTire"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-sm w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Archive class="w-4 h-4 text-warning-400" />
          {{ $t('tires.tireDisposeModal.scrap') }}
        </h3>
        <button type="button" @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-4 h-4" />
        </button>
      </div>

      <form id="tire-dispose-modal-form" @submit.prevent="handleDisposeTire" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
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
          <DistanceInput id="tire-dispose-odometer" v-model="disposeForm.odometer" min="0" required class="w-full bg-slate-800 text-slate-100 text-xs rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500" />
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="tire-dispose-modal-form" class="bg-warning-600 hover:bg-warning-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors">
          {{ $t('tires.tireDisposeModal.scrap') }}
        </button>
      </div>
    </div>
  </div>
</template>
