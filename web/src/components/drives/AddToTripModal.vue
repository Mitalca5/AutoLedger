<script setup lang="ts">
import { t } from '@/i18n'
import { ref, watch } from 'vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { X, Plus } from 'lucide-vue-next'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

// Adds the selected drives to an existing trip group.
const props = defineProps<{ vehicleId: string; tripGroups: any[]; selectedDriveIds: string[] }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const addToTripId = ref('')

watch(open, (isOpen) => {
  if (isOpen) addToTripId.value = props.tripGroups[0]?.id || ''
})

async function handleAddToTrip() {
  const tg = props.tripGroups.find((g) => g.id === addToTripId.value)
  if (!props.vehicleId || !tg) return
  const driveIds = Array.from(new Set([...(tg.drive_ids || []), ...props.selectedDriveIds]))
  try {
    await api.updateTripGroup(props.vehicleId, tg.id, { name: tg.name, notes: tg.notes, drive_ids: driveIds })
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
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2 truncate pr-2">
          <Plus class="w-5 h-5 text-indigo-400 shrink-0" />
          <span class="truncate">{{ $t('drives.addToTripModal.addDriveSToA', { length: selectedDriveIds.length }) }}</span>
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors shrink-0" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="add-to-trip-form" @submit.prevent="handleAddToTrip" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <p v-if="!tripGroups.length" class="text-xs text-slate-400">{{ $t('drives.addToTripModal.noExistingTripUseMerge') }}</p>
        <div v-else>
          <label for="add-to-trip" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('drives.addToTripModal.trip') }}</label>
          <select id="add-to-trip" v-model="addToTripId" class="field">
            <option v-for="tg in tripGroups" :key="tg.id" :value="tg.id">{{ $t('drives.addToTripModal.drives', { name: tg.name, length: tg.drive_ids.length }) }}</option>
          </select>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="add-to-trip-form" :disabled="!tripGroups.length" :title="!tripGroups.length ? $t('drives.addToTripModal.noTrip') : undefined" class="btn btn-lg btn-primary">
          {{ $t('drives.addToTripModal.add') }}
        </button>
      </div>
    </div>
  </div>
</template>
