<script setup lang="ts">
import ModalShell from '@/components/ModalShell.vue'
import { intlLocale, t } from '@/i18n'
import { formatDistance } from '@/units'
import { ref, watch } from 'vue'
import { Copy } from 'lucide-vue-next'
import { api } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { defaultTargetTireIds, getTireSelectLabel } from '@/utils/tires'

// source is the tire whose history is copied when the modal opens; it can be changed from inside the modal.
const props = defineProps<{ vehicleId: string; tires: any[]; source: any | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { showAlert } = useConfirm()

const copyHistorySourceTire = ref<any | null>(null)
const copyHistoryTargetTireIds = ref<string[]>([])
const copyHistoryOptions = ref({
  copy_sessions: true,
  copy_logs: true,
  adapt_position: true,
})
const copyingHistory = ref(false)

function updateCopyHistoryTargets() {
  if (!copyHistorySourceTire.value) return
  copyHistoryTargetTireIds.value = defaultTargetTireIds(props.tires, copyHistorySourceTire.value)
}

function onSourceTireChange(sourceId: string) {
  const found = props.tires.find((t) => t.tire.id === sourceId)
  if (found) {
    copyHistorySourceTire.value = found.tire
    updateCopyHistoryTargets()
  }
}

watch(open, (isOpen) => {
  if (!isOpen) return
  copyHistorySourceTire.value = props.source
  updateCopyHistoryTargets()
  copyHistoryOptions.value = {
    copy_sessions: true,
    copy_logs: true,
    adapt_position: true,
  }
})

function toggleCopyHistoryTargetTire(id: string) {
  if (copyHistoryTargetTireIds.value.includes(id)) {
    copyHistoryTargetTireIds.value = copyHistoryTargetTireIds.value.filter((x) => x !== id)
  } else {
    copyHistoryTargetTireIds.value.push(id)
  }
}

async function handleCopyHistorySubmit() {
  if (!props.vehicleId || !copyHistorySourceTire.value || !copyHistoryTargetTireIds.value.length) return
  copyingHistory.value = true
  try {
    await api.copyTireHistory(props.vehicleId, copyHistorySourceTire.value.id, {
      target_tire_ids: copyHistoryTargetTireIds.value,
      copy_sessions: copyHistoryOptions.value.copy_sessions,
      copy_logs: copyHistoryOptions.value.copy_logs,
      adapt_position: copyHistoryOptions.value.adapt_position,
    })
    open.value = false
    showAlert(t('tires.tireCopyHistoryModal.copied', { count: copyHistoryTargetTireIds.value.length }), t('common.success'), 'success')
    emit('saved')
  } catch (err: any) {
    showAlert(t('tires.tireCopyHistoryModal.error', { message: err.message }), t('shell.confirm.error'), 'danger')
  } finally {
    copyingHistory.value = false
  }
}
</script>

<template>
  <ModalShell
    v-if="copyHistorySourceTire"
    v-model:open="open"
    :title="$t('tires.tireCopyHistoryModal.copyTheFullHistory')"
    :icon="Copy"
    icon-class="text-sky-400"
    nested
    body-class="space-y-4 text-xs"
    footer-class="items-center justify-end gap-2"
  >
    <!-- Source selection -->
    <div class="bg-slate-950/60 p-3 rounded-xl border border-slate-800 space-y-2">
      <label for="copy-history-source-select" class="text-xs text-sky-400 font-semibold uppercase tracking-wider block">{{ $t('tires.tireCopyHistoryModal.sourceTireToClone') }}</label>
      <select
        id="copy-history-source-select"
        :value="copyHistorySourceTire.id"
        @change="onSourceTireChange(($event.target as HTMLSelectElement).value)"
        class="field cursor-pointer"
      >
        <option v-for="t in tires" :key="t.tire.id" :value="t.tire.id">
          {{ getTireSelectLabel(t) }}
        </option>
      </select>
    </div>

    <!-- Options -->
    <div class="space-y-2 bg-slate-950/40 p-3 rounded-xl border border-slate-800/80">
      <div class="font-semibold text-slate-300">{{ $t('tires.tireCopyHistoryModal.dataToReplicate') }}</div>
      <label class="flex items-start gap-2.5 cursor-pointer text-slate-200">
        <input
          type="checkbox"
          v-model="copyHistoryOptions.copy_sessions"
          class="rounded accent-rose-500 w-4 h-4 mt-0.5"
        />
        <div>
          <span class="font-medium text-white">{{ $t('tires.tireCopyHistoryModal.fittingAndRemovalSessions') }}</span>
          <span class="block text-xs text-slate-400">{{ $t('tires.tireCopyHistoryModal.copiesThePeriodsDatesOdometers') }}</span>
        </div>
      </label>

      <label class="flex items-start gap-2.5 cursor-pointer text-slate-200">
        <input
          type="checkbox"
          v-model="copyHistoryOptions.adapt_position"
          :disabled="!copyHistoryOptions.copy_sessions"
          class="rounded accent-rose-500 w-4 h-4 mt-0.5"
        />
        <div>
          <span class="font-medium text-white">{{ $t('tires.tireCopyHistoryModal.adaptTheFittingPositionTo') }}</span>
          <span class="block text-xs text-slate-400">{{ $t('tires.tireCopyHistoryModal.whenEnabledEachTargetTire') }}</span>
        </div>
      </label>

      <label class="flex items-start gap-2.5 cursor-pointer text-slate-200">
        <input
          type="checkbox"
          v-model="copyHistoryOptions.copy_logs"
          class="rounded accent-rose-500 w-4 h-4 mt-0.5"
        />
        <div>
          <span class="font-medium text-white">{{ $t('tires.tireCopyHistoryModal.wearAndTreadMeasurementsLogs') }}</span>
          <span class="block text-xs text-slate-400">{{ $t('tires.tireCopyHistoryModal.copiesTheTreadDepthReadings') }}</span>
        </div>
      </label>
    </div>

    <!-- Target tires list -->
    <div>
      <div class="flex items-center justify-between mb-2">
        <span class="font-semibold text-slate-300">{{ $t('tires.tireCopyHistoryModal.applyToTheTargetTires') }}</span>
        <div class="flex items-center gap-2 text-xs">
          <button
            type="button"
            @click="copyHistoryTargetTireIds = tires.filter(x => x.tire.id !== copyHistorySourceTire?.id).map(x => x.tire.id)"
            class="text-sky-400 hover:text-sky-300 font-semibold"
          >
            {{ $t('tires.tireCopyHistoryModal.tickAll') }}
          </button>
          <span class="text-slate-400">|</span>
          <button
            type="button"
            @click="copyHistoryTargetTireIds = []"
            class="text-slate-400 hover:text-slate-200"
          >
            {{ $t('tires.tireCopyHistoryModal.untickAll') }}
          </button>
        </div>
      </div>

      <div class="space-y-1.5 max-h-48 overflow-y-auto p-1 bg-slate-950/40 rounded-xl border border-slate-800/60">
        <label
          v-for="t in tires.filter(x => x.tire.id !== copyHistorySourceTire?.id)"
          :key="t.tire.id"
          class="flex items-center gap-2.5 p-2 rounded-lg hover:bg-slate-800/50 cursor-pointer text-slate-200"
        >
          <input
            type="checkbox"
            :checked="copyHistoryTargetTireIds.includes(t.tire.id)"
            @change="toggleCopyHistoryTargetTire(t.tire.id)"
            class="rounded accent-rose-500 w-4 h-4"
          />
          <div class="min-w-0 flex-1">
            <div class="font-bold truncate text-white flex items-center justify-between gap-2">
              <span class="truncate">{{ t.tire.brand }} {{ t.tire.model }}</span>
              <span class="text-xs font-normal text-sky-300 shrink-0">
                {{ formatDistance(t.total_distance_km ?? t.tire.accumulated_distance_km ?? 0) }} • {{ (t.sessions?.length || 0) }} {{ (t.sessions?.length || 0) > 1 ? 'sessions' : 'session' }}
              </span>
            </div>
            <div class="text-xs text-slate-400 truncate">
              {{ t.tire.dimension }} — {{ t.tire.current_position === 'STORAGE' ? $t('tires.inStorage') : t.tire.current_position === 'DISPOSED' ? $t('tires.scrapped') : $t('tires.wheel', { position: t.tire.current_position }) }}
              <span v-if="t.tire.dot_code" class="text-slate-400">{{ $t('tires.tireCopyHistoryModal.dot', { dot_code: t.tire.dot_code }) }}</span>
            </div>
          </div>
        </label>
      </div>
    </div>
    <template #footer>
      <button
        type="button"
        @click="open = false"
        class="btn btn-lg btn-secondary"
      >
        {{ $t('common.cancel') }}
      </button>
      <button
        type="button"
        @click="handleCopyHistorySubmit()"
        :disabled="copyingHistory || copyHistoryTargetTireIds.length === 0"
        class="btn btn-lg btn-primary"
      >
        <Copy class="w-4 h-4" />
        <span>{{ copyingHistory ? $t('tires.tireCopyHistoryModal.copying') : $t('tires.tireCopyHistoryModal.copyTo', { count: copyHistoryTargetTireIds.length }) }}</span>
      </button>
    </template>
  </ModalShell>
</template>