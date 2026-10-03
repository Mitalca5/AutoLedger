<script setup lang="ts">
import { t } from '@/i18n'
import { ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { Layers, X } from 'lucide-vue-next'
import DrivePicker from '@/components/drives/DrivePicker.vue'
import { toDateInputString } from '@/utils/carpool'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

// Edits a trip group: its name, its notes and its legs (the drives ticked in the picker, around the date of the trip).
const props = defineProps<{ vehicleId: string; trip: any | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const form = ref({ id: '', name: '', notes: '' })
const selectedDriveIds = ref<string[]>([])
const anchorDate = ref('')
// Changes each time the modal opens so the picker starts again from the trip
const session = ref(0)
const saving = ref(false)

watch(open, (isOpen) => {
  if (!isOpen || !props.trip) return
  form.value = { id: props.trip.id, name: props.trip.name, notes: props.trip.notes || '' }
  selectedDriveIds.value = [...(props.trip.drive_ids || [])]
  anchorDate.value = props.trip.start_time ? toDateInputString(props.trip.start_time) : ''
  session.value++
})

function toggleDrive(driveId: string) {
  const i = selectedDriveIds.value.indexOf(driveId)
  if (i > -1) selectedDriveIds.value.splice(i, 1)
  else selectedDriveIds.value.push(driveId)
}

async function handleSave() {
  if (!props.vehicleId || !form.value.name.trim()) return
  if (!selectedDriveIds.value.length) {
    showAlert(t('drives.drivesView.tripNeedsDrive'), t('drives.drivesView.actionImpossible'), 'warning')
    return
  }
  saving.value = true
  try {
    await api.updateTripGroup(props.vehicleId, form.value.id, {
      name: form.value.name,
      notes: form.value.notes || null,
      drive_ids: selectedDriveIds.value,
    })
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-xl w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Layers class="w-5 h-5 text-indigo-400" />
          {{ $t('drives.tripEditModal.editTheTrip') }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :title="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="trip-edit-form" @submit.prevent="handleSave" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <div>
          <label for="trip-edit-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('drives.tripEditModal.name') }}</label>
          <input id="trip-edit-name" v-model="form.name" required class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
        </div>
        <div>
          <label for="trip-edit-notes" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('common.notes') }}</label>
          <input id="trip-edit-notes" v-model="form.notes" class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white focus:outline-none focus:border-rose-500" />
        </div>

        <div class="space-y-1.5">
          <div class="flex items-center justify-between text-xs">
            <span class="text-slate-400">{{ $t('drives.tripEditModal.tickTheLegs') }}</span>
            <span class="text-indigo-300 font-semibold">{{ $t('drives.tripEditModal.legsSelected', { count: selectedDriveIds.length }) }}</span>
          </div>
          <DrivePicker
            :key="session"
            :vehicle-id="vehicleId"
            :selected-ids="selectedDriveIds"
            :anchor-date="anchorDate"
            @toggle="toggleDrive"
          />
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="trip-edit-form" :disabled="saving" class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl transition-colors disabled:opacity-50">
          {{ $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
