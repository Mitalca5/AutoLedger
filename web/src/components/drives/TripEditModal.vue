<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import { t } from '@/i18n'
import { ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { Layers } from 'lucide-vue-next'
import DrivePicker from '@/components/drives/DrivePicker.vue'
import { toDateInputString } from '@/utils/carpool'

// Edits a trip group: its name, its notes and its legs (the drives ticked in the picker, around the date of the trip).
const props = defineProps<{ vehicleId: string; trip: any | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
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
  <ModalShell
    v-model:open="open"
    :title="$t('drives.tripEditModal.editTheTrip')"
    :icon="Layers"
    icon-class="text-sky-400"
  >
    <form id="trip-edit-form" @submit.prevent="handleSave" class="space-y-4">
      <div>
        <label for="trip-edit-name" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('drives.tripEditModal.name') }}</label>
        <input id="trip-edit-name" v-model="form.name" required class="field" />
      </div>
      <div>
        <label for="trip-edit-notes" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('common.notes') }}</label>
        <input id="trip-edit-notes" v-model="form.notes" class="field" />
      </div>

      <div class="space-y-1.5">
        <div class="flex items-center justify-between text-xs">
          <span class="text-slate-400">{{ $t('drives.tripEditModal.tickTheLegs') }}</span>
          <span class="text-sky-300 font-semibold">{{ $t('drives.tripEditModal.legsSelected', { count: selectedDriveIds.length }) }}</span>
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
    <template #footer>
      <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
        {{ $t('common.cancel') }}
      </button>
      <button type="submit" form="trip-edit-form" :disabled="saving" class="btn btn-lg btn-primary">
        {{ $t('common.save') }}
      </button>
    </template>
  </ModalShell>
</template>