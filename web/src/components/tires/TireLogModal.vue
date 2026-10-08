<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { Ruler } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { type TireLogForm, validateTreadDepth } from '@/utils/tires'
import { todayIso } from '@/utils/dates'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

// Adds or edits (editingLogId set) a tread depth measurement of selectedTire; initialForm seeds the fields when the modal opens
const props = defineProps<{ vehicleId: string; selectedTire: any | null; editingLogId: string | null; initialForm: TireLogForm }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
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
  <ModalShell
    v-model:open="open"
    :title="editingLogId ? $t('tires.tireLogModal.edit') : $t('tires.tireLogModal.new')"
    :icon="Ruler"
    icon-class="text-success-400"
    size="sm"
    body-class="space-y-4 text-xs"
    footer-class="items-center justify-end gap-2"
  >
    <div>
      <label for="tire-new-log-depth-mm" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireLogModal.measuredDepthMm') }}</label>
      <NumberInput id="tire-new-log-depth-mm"
        v-model="newLogForm.depth_mm"
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
    <template #footer>
      <button
        type="button"
        @click="open = false"
        class="btn btn-lg btn-secondary"
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
    </template>
  </ModalShell>
</template>