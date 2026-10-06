<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { Ruler, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { type TireLogForm, validateTreadDepth } from '@/utils/tires'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

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
  if (!isOpen) return
  submitted.value = false
  newLogForm.value = { ...props.initialForm }
})

const { pending: submitting, run: runOnce } = useSubmit()

const submitted = ref(false)
const depthError = computed(() => (submitted.value ? validateTreadDepth(newLogForm.value.depth_mm) : null))

async function handleAddLogAction() {
  if (!props.vehicleId || !props.selectedTire) return
  submitted.value = true
  if (validateTreadDepth(newLogForm.value.depth_mm)) {
    document.getElementById('tire-new-log-depth-mm')?.focus()
    return
  }
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
const handleAddLog = () => runOnce(handleAddLogAction)
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
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
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
            min="0.1"
            max="20"
            :aria-invalid="depthError ? 'true' : undefined"
            aria-describedby="tire-new-log-depth-error"
            class="field font-bold"
          />
          <p v-if="depthError" id="tire-new-log-depth-error" class="mt-1 text-xs text-danger-400">{{ $t(depthError) }}</p>
        </div>
        <div>
          <label for="tire-new-log-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireLogModal.currentOdometerKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput id="tire-new-log-odometer"
            v-model="newLogForm.odometer"
            class="field"
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
            class="field"
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
        <button :disabled="submitting"
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
