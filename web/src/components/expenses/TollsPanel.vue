<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { Navigation, Layers, Users, Pencil, Trash2, Paperclip } from 'lucide-vue-next'
import { formatDate } from '@/utils/expenses'
import { formatAmount } from '@/currency'
import { t } from '@/i18n'

defineProps<{ driveExpenses: any[]; loading: boolean }>()
const emit = defineEmits<{
  edit: [expense: any]
  delete: [expense: any]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const router = useRouter()
const vehicleStore = useVehicleStore()

const TYPE_LABEL_KEYS: Record<string, string> = {
  TOLL: 'expenses.tollModal.toll',
  PARKING: 'expenses.tollModal.parking',
  FERRY: 'expenses.tollModal.ferry',
  OTHER: 'expenses.tollModal.other',
}
const expenseTypeLabel = (type: string) => (TYPE_LABEL_KEYS[type] ? t(TYPE_LABEL_KEYS[type]) : type)
</script>

<template>
  <div>
    <div v-if="loading" class="text-center py-12 text-slate-400">{{ $t('common.loading') }}</div>
    <EmptyState v-else-if="!driveExpenses.length">
      {{ $t('expenses.tollsPanel.noTollOrParkingRecorded') }}
    </EmptyState>
    <div v-else class="space-y-3">
      <div
        v-for="e in driveExpenses"
        :key="e.id"
        class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
      >
        <div class="space-y-1.5 min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-warning-500/10 text-warning-400 border border-warning-500/20 shrink-0">
              {{ expenseTypeLabel(e.type) }}
            </span>
            <span class="text-xs text-slate-400 shrink-0">{{ formatDate(e.date) }}</span>
            <span v-if="e.drive_title" class="text-xs px-2.5 py-0.5 rounded-lg bg-success-500/10 text-success-400 border border-success-500/20 flex items-center gap-1 truncate max-w-xs">
              <Navigation class="w-3 h-3 shrink-0" /> <span class="truncate">{{ e.drive_title }}</span>
            </span>
            <span v-else-if="e.trip_group_name" class="text-xs px-2.5 py-0.5 rounded-lg bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 truncate max-w-xs">
              <Layers class="w-3 h-3 shrink-0" /> <span class="truncate">{{ e.trip_group_name }}</span>
            </span>
            <button
              v-if="e.document_id"
              @click="emit('view-document', e.document_id, e.document_filename, false)"
              class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
              :title="$t('expenses.tollsPanel.viewTheReceipt')"
            >
              <Paperclip class="w-3 h-3 shrink-0" />
              <span class="truncate">{{ e.document_filename || $t('expenses.invoice') }}</span>
            </button>
          </div>
          <p v-if="e.notes" class="text-sm text-slate-300">{{ e.notes }}</p>
        </div>
        <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
          <div class="text-lg font-extrabold text-warning-400">
            {{ formatAmount(e.amount, e.currency || vehicleStore.currency) }}
          </div>
          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
            <button
              v-if="e.drive_id || e.trip_group_id"
              @click="router.push({ path: '/carpools', query: e.drive_id ? { new_drive_id: e.drive_id } : { new_trip_group_id: e.trip_group_id } })"
              class="px-2.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold rounded-xl flex items-center gap-1.5 transition-colors border border-slate-700/60"
              :title="$t('expenses.tollsPanel.createACarpoolForThis')"
            >
              <Users class="w-3.5 h-3.5 text-cyan-400" />
              <span>{{ $t('expenses.tollsPanel.carpool') }}</span>
            </button>
            <button
              @click="emit('edit', e)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-warning-400 rounded-xl transition-colors border border-slate-700/60"
              :title="$t('expenses.tollsPanel.editThisToll')"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('delete', e)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
              :title="$t('expenses.tollsPanel.deleteThisToll')"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
