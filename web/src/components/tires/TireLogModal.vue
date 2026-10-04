<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { Ruler, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { type TireLogForm } from '@/utils/tires'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'

// Adds or edits (editingLogId set) a tread depth measurement of selectedTire; initialForm seeds the fields when the modal opens
const props = defineProps<{ vehicleId: string; selectedTire: any | null; editingLogId: string | null; initialForm: TireLogForm }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const newLogForm = ref<TireLogForm>({
  depth_mm: 6.5,
  odometer: 0,
  notes: '',
  date: todayIso(),
})

watch(open, (isOpen) => {
  if (isOpen) newLogForm.value = { ...props.initialForm }
})

async function handleAddLog() {
  if (!props.vehicleId || !props.selectedTire) return
  try {
    const payload = {
      depth_mm: Number(newLogForm.value.depth_mm),
      odometer: Number(newLogForm.value.odometer),
      notes: newLogForm.value.notes ? newLogForm.value.notes : null,
      date: new Date(newLogForm.value.date).toISOString(),
    }
    if (props.editingLogId) {
      await api.updateTireLog(props.vehicleId, props.selectedTire.id, props.editingLogId, payload)
    } else {
      await api.addTireLog(props.vehicleId, props.selectedTire.id, payload)
    }
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-sm w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Ruler class="w-4 h-4 text-success-400" />
          {{ editingLogId ? $t('tires.tireLogModal.edit') : $t('tires.tireLogModal.new') }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
        <div>
          <label for="tire-new-log-depth-mm" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireLogModal.measuredDepthMm') }}</label>
          <input id="tire-new-log-depth-mm"
            v-model.number="newLogForm.depth_mm"
            type="number"
            step="0.1"
            min="1.0"
            max="10.0"
            class="w-full bg-slate-800 text-slate-100 font-bold rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500 text-sm"
          />
        </div>
        <div>
          <label for="tire-new-log-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireLogModal.currentOdometerKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput id="tire-new-log-odometer"
            v-model="newLogForm.odometer"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700 focus:outline-none focus:border-rose-500"
          />
        </div>
        <div>
          <label for="tire-new-log-date" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireLogModal.readingDate') }}</label>
          <AppDatePicker id="tire-new-log-date" v-model="newLogForm.date" size="sm" required />
        </div>
        <div>
          <label for="tire-new-log-notes" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireLogModal.notesOptional') }}</label>
          <input id="tire-new-log-notes"
            v-model="newLogForm.notes"
            type="text"
            :placeholder="$t('tires.tireLogModal.eGCheckBeforeThe')"
            class="w-full bg-slate-800 text-slate-100 rounded-xl px-3 py-2 border border-slate-700"
          />
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors"
        >
          {{ $t('common.cancel') }}
        </button>
        <button
          type="button"
          @click="handleAddLog"
          class="bg-success-600 hover:bg-success-500 text-white text-xs font-semibold px-4 py-2 rounded-xl transition-colors"
        >
          {{ $t('tires.tireLogModal.saveTheReading') }}
        </button>
      </div>
    </div>
  </div>
</template>
