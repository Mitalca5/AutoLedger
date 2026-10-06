<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { ref, watch } from 'vue'
import { ClipboardPaste, X } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { emptySessionForm, formatDate, type SessionForm } from '@/utils/tires'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { useOdometerPrefill } from '@/composables/useOdometerPrefill'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

// Adds or edits (editingSessionId set) a mount session of selectedTire; initialForm seeds the fields when the modal opens
const props = defineProps<{
  vehicleId: string
  selectedTire: any | null
  copiedSession: SessionForm | null
  editingSessionId: string | null
  initialForm: SessionForm
}>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const sessionForm = ref<SessionForm>(emptySessionForm())

const mountedPrefill = useOdometerPrefill({
  vehicleId: () => props.vehicleId,
  enabled: () => open.value,
  date: () => sessionForm.value.mounted_date,
  current: () => sessionForm.value.mounted_odometer,
  fill: (km) => {
    sessionForm.value.mounted_odometer = km
    onSessionOdometerChange()
  },
})
const dismountedPrefill = useOdometerPrefill({
  vehicleId: () => props.vehicleId,
  enabled: () => open.value && sessionForm.value.is_dismounted,
  date: () => sessionForm.value.dismounted_date,
  current: () => sessionForm.value.dismounted_odometer,
  fill: (km) => {
    sessionForm.value.dismounted_odometer = km
    onSessionOdometerChange()
  },
})

watch(open, (isOpen) => {
  if (!isOpen) return
  sessionForm.value = { ...props.initialForm }
  // A stored reading counts as filled for the user: changing the date recomputes it, as for a new session.
  const stored = sessionForm.value
  mountedPrefill.reset(props.editingSessionId ? stored.mounted_odometer || null : null)
  dismountedPrefill.reset(props.editingSessionId ? stored.dismounted_odometer || null : null)
})

function applyCopiedSessionToForm() {
  if (!props.copiedSession) return
  sessionForm.value = {
    ...sessionForm.value,
    ...props.copiedSession,
    position: sessionForm.value.position || props.copiedSession.position,
  }
}

function onSessionOdometerChange() {
  const mount = Number(sessionForm.value.mounted_odometer) || 0
  const dismount = Number(sessionForm.value.dismounted_odometer) || 0
  if (dismount > mount) {
    sessionForm.value.distance_km = dismount - mount
  }
}

const { pending: submitting, run: runOnce } = useSubmit()

async function handleSaveSessionAction() {
  if (!props.vehicleId || !props.selectedTire) return

  try {
    const payload: any = {
      position: sessionForm.value.position,
      mounted_date: new Date(sessionForm.value.mounted_date).toISOString(),
      mounted_odometer: Number(sessionForm.value.mounted_odometer),
      distance_km: Number(sessionForm.value.distance_km),
      notes: sessionForm.value.notes ? sessionForm.value.notes : null,
    }

    if (sessionForm.value.is_dismounted) {
      payload.dismounted_date = new Date(sessionForm.value.dismounted_date).toISOString()
      payload.dismounted_odometer = Number(sessionForm.value.dismounted_odometer)
      if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
        payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
      }
    } else {
      payload.dismounted_date = null
      payload.dismounted_odometer = null
    }

    if (props.editingSessionId) {
      await api.updateTireSession(props.vehicleId, props.selectedTire.id, props.editingSessionId, payload)
    } else {
      await api.createTireSession(props.vehicleId, props.selectedTire.id, payload)
    }

    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleSaveSession = () => runOnce(handleSaveSessionAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white">
          {{ editingSessionId ? $t('tires.tireSessionModal.edit') : $t('tires.tireSessionModal.add') }}
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
        <!-- Quick paste banner if session copied -->
        <button
          v-if="copiedSession && !editingSessionId"
          type="button"
          @click="applyCopiedSessionToForm()"
          class="w-full px-3 py-2 bg-indigo-950/40 border border-indigo-800/60 rounded-xl text-indigo-300 hover:text-white text-xs flex items-center justify-center gap-2 transition-colors font-semibold"
        >
          <ClipboardPaste class="w-4 h-4 text-indigo-400" />
          <span>{{ $t('tires.tireSessionModal.pasteTheDataOfThe', { mounted_date: copiedSession.mounted_date ? formatDate(copiedSession.mounted_date) : '' }) }}</span>
        </button>

        <div>
          <label for="tire-session-position" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireSessionModal.positionUsed') }}</label>
          <select id="tire-session-position"
            v-model="sessionForm.position"
            class="field"
          >
            <option value="FL">{{ $t('tires.tireSessionModal.frontLeftFl') }}</option>
            <option value="FR">{{ $t('tires.tireSessionModal.frontRightFr') }}</option>
            <option value="RL">{{ $t('tires.tireSessionModal.rearLeftRl') }}</option>
            <option value="RR">{{ $t('tires.tireSessionModal.rearRightRr') }}</option>
            <option value="STORAGE">{{ $t('tires.tireSessionModal.inStorageUnspecified') }}</option>
          </select>
        </div>

        <div class="grid grid-cols-2 gap-2">
          <div>
            <label for="tire-session-mounted-date" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireSessionModal.fittingDate') }}</label>
            <AppDatePicker
              id="tire-session-mounted-date"
              v-model="sessionForm.mounted_date"
              size="xs"
              :clearable="true"
            />
          </div>
          <div>
            <label for="tire-session-mounted-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireSessionModal.odometerAtFittingKm', { unit: distanceUnit() }) }}</label>
            <DistanceInput id="tire-session-mounted-odometer"
              v-model="sessionForm.mounted_odometer"
              @input="onSessionOdometerChange"
              class="field"
            />
          </div>
        </div>

        <div class="pt-1">
          <label class="flex items-center gap-2 cursor-pointer text-slate-300">
            <input type="checkbox" v-model="sessionForm.is_dismounted" class="rounded accent-rose-500" />
            <span>{{ $t('tires.tireSessionModal.thisSessionIsOverTire') }}</span>
          </label>
        </div>

        <div v-if="sessionForm.is_dismounted" class="grid grid-cols-2 gap-2 bg-slate-950/60 p-3 rounded-xl border border-slate-800">
          <div>
            <label for="tire-session-dismounted-date" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireSessionModal.removalDate') }}</label>
            <AppDatePicker
              id="tire-session-dismounted-date"
              v-model="sessionForm.dismounted_date"
              size="xs"
              :clearable="true"
            />
          </div>
          <div>
            <label for="tire-session-dismounted-odometer" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireSessionModal.odometerAtRemovalKm', { unit: distanceUnit() }) }}</label>
            <DistanceInput id="tire-session-dismounted-odometer"
              v-model="sessionForm.dismounted_odometer"
              @input="onSessionOdometerChange"
              class="field"
            />
          </div>
        </div>

        <div>
          <label for="tire-session-distance-km" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireSessionModal.sessionDistanceKm', { unit: distanceUnit() }) }}</label>
          <DistanceInput id="tire-session-distance-km"
            v-model="sessionForm.distance_km"
            :placeholder="$t('tires.tireSessionModal.calculatedOrForced')"
            class="field"
          />
        </div>

        <div>
          <label for="tire-session-notes" class="block text-slate-400 mb-1 font-semibold">{{ $t('tires.tireSessionModal.commentNotes') }}</label>
          <input id="tire-session-notes"
            v-model="sessionForm.notes"
            type="text"
            :placeholder="$t('tires.tireSessionModal.eGWinterSeason2024')"
            class="field"
          />
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="btn btn-lg btn-secondary"
        >
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting"
          type="button"
          @click="handleSaveSession"
          class="btn btn-lg btn-primary"
        >
          {{ $t('common.save') }}
        </button>
      </div>
    </div>
  </div>
</template>
