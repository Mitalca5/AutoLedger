<script setup lang="ts">
import { intlLocale, t } from '@/i18n'
import { formatDistance } from '@/units'
import { ref, watch } from 'vue'
import { Copy, X } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { defaultTargetTireIds, formatDate } from '@/utils/tires'
import { useEscapeToClose } from '@/composables/useEscapeToClose'

// Copies one mount session of the open tire onto other tires
const props = defineProps<{ vehicleId: string; selectedTire: any | null; sessionToDuplicate: any | null; tires: any[] }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const duplicateTargetTireIds = ref<string[]>([])
const duplicatingSession = ref(false)

watch(open, (isOpen) => {
  if (isOpen) duplicateTargetTireIds.value = defaultTargetTireIds(props.tires, props.selectedTire)
})

function toggleDuplicateTargetTire(id: string) {
  if (duplicateTargetTireIds.value.includes(id)) {
    duplicateTargetTireIds.value = duplicateTargetTireIds.value.filter((x) => x !== id)
  } else {
    duplicateTargetTireIds.value.push(id)
  }
}

async function handleDuplicateSessionSubmit() {
  if (!props.vehicleId || !props.sessionToDuplicate || duplicateTargetTireIds.value.length === 0) return

  duplicatingSession.value = true
  try {
    const s = props.sessionToDuplicate
    const payload: any = {
      position: s.position || 'STORAGE',
      mounted_date: new Date(s.mounted_date).toISOString(),
      mounted_odometer: Number(s.mounted_odometer) || 0,
      distance_km: Number(s.distance_km) || 0,
      notes: s.notes || null,
    }
    if (s.dismounted_date) {
      payload.dismounted_date = new Date(s.dismounted_date).toISOString()
      payload.dismounted_odometer = Number(s.dismounted_odometer) || 0
      if (payload.distance_km === 0 && payload.dismounted_odometer > payload.mounted_odometer) {
        payload.distance_km = payload.dismounted_odometer - payload.mounted_odometer
      }
    } else {
      payload.dismounted_date = null
      payload.dismounted_odometer = null
    }

    for (const targetId of duplicateTargetTireIds.value) {
      await api.createTireSession(props.vehicleId, targetId, payload)
    }

    open.value = false
    emit('saved')
    showAlert(t('tires.tireDuplicateSessionModal.duplicated', { count: duplicateTargetTireIds.value.length }), t('common.success'), 'success')
  } catch (err: any) {
    showAlert(t('tires.tireDuplicateSessionModal.error', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    duplicatingSession.value = false
  }
}
</script>

<template>
  <div
    v-if="open && sessionToDuplicate"
    class="fixed inset-0 z-[70] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-md w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <div class="flex items-center gap-2">
          <Copy class="w-5 h-5 text-indigo-400" />
          <h3 class="text-base font-bold text-white">
            {{ $t('tires.tireDuplicateSessionModal.duplicateTheSessionToOther') }}
          </h3>
        </div>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <div class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4 text-xs">
        <!-- Session recap -->
        <div class="bg-slate-950/60 p-3 rounded-xl border border-slate-800 space-y-1">
          <div class="text-slate-400">{{ $t('tires.tireDuplicateSessionModal.period') }} <strong class="text-white">{{ formatDate(sessionToDuplicate.mounted_date) }} → {{ sessionToDuplicate.dismounted_date ? formatDate(sessionToDuplicate.dismounted_date) : $t('tires.tireDuplicateSessionModal.ongoing') }}</strong></div>
          <div class="text-slate-400">{{ $t('tires.tireDuplicateSessionModal.distance') }} <strong class="text-rose-400">+{{ formatDistance(sessionToDuplicate.distance_km || 0) }}</strong></div>
          <div v-if="sessionToDuplicate.notes" class="text-slate-400 italic">"{{ sessionToDuplicate.notes }}"</div>
        </div>

        <!-- Target tires selection -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <span class="font-semibold text-slate-300">{{ $t('tires.tireDuplicateSessionModal.selectTheTargetTires') }}</span>
            <div class="flex items-center gap-2 text-[11px]">
              <button
                type="button"
                @click="duplicateTargetTireIds = tires.filter(x => x.tire.id !== selectedTire?.id).map(x => x.tire.id)"
                class="text-indigo-400 hover:text-indigo-300 font-semibold"
              >
                {{ $t('tires.tireDuplicateSessionModal.tickAll') }}
              </button>
              <span class="text-slate-600">|</span>
              <button
                type="button"
                @click="duplicateTargetTireIds = []"
                class="text-slate-400 hover:text-slate-200"
              >
                {{ $t('tires.tireDuplicateSessionModal.untickAll') }}
              </button>
            </div>
          </div>
          <div class="space-y-1.5 max-h-56 overflow-y-auto p-1 bg-slate-950/40 rounded-xl border border-slate-800/60">
            <label
              v-for="t in tires.filter(x => x.tire.id !== selectedTire?.id)"
              :key="t.tire.id"
              class="flex items-center gap-2.5 p-2 rounded-lg hover:bg-slate-800/50 cursor-pointer text-slate-200"
            >
              <input
                type="checkbox"
                :checked="duplicateTargetTireIds.includes(t.tire.id)"
                @change="toggleDuplicateTargetTire(t.tire.id)"
                class="rounded accent-indigo-500 w-4 h-4"
              />
              <div class="min-w-0 flex-1">
                <div class="font-bold truncate text-white">{{ t.tire.brand }} {{ t.tire.model }}</div>
                <div class="text-[10px] text-slate-400 truncate">{{ t.tire.dimension }} — {{ t.tire.current_position === 'STORAGE' ? $t('tires.inStorage') : $t('tires.wheel', { position: t.tire.current_position }) }}</div>
              </div>
            </label>
          </div>
        </div>
      </div>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex items-center justify-end gap-2 shrink-0 bg-slate-900/95">
        <button
          type="button"
          @click="open = false"
          class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl transition-colors"
        >
          {{ $t('common.cancel') }}
        </button>
        <button
          type="button"
          @click="handleDuplicateSessionSubmit()"
          :disabled="duplicatingSession || duplicateTargetTireIds.length === 0"
          class="px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:cursor-not-allowed text-white text-xs font-semibold rounded-xl shadow-lg shadow-indigo-600/20 transition-all flex items-center gap-1.5"
        >
          <Copy class="w-4 h-4" />
          <span>{{ duplicatingSession ? $t('tires.tireDuplicateSessionModal.duplicating') : $t('tires.tireDuplicateSessionModal.duplicateTo', { count: duplicateTargetTireIds.length }) }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
