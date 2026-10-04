<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import { intlLocale } from '@/i18n'
import { useVehicleStore } from '@/stores/vehicle'
import { Repeat, Pencil, Trash2, Paperclip } from 'lucide-vue-next'
import { categoryLabel, formatDate } from '@/utils/expenses'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistanceValue } from '@/units'

defineProps<{ maintenanceExpenses: any[]; loading: boolean }>()
const emit = defineEmits<{
  edit: [expense: any]
  delete: [expense: any]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div>
    <div v-if="loading" class="text-center py-12 text-slate-400">{{ $t('common.loading') }}</div>
    <EmptyState v-else-if="!maintenanceExpenses.length">
      {{ $t('expenses.maintenancePanel.noMaintenanceOrFixedExpense') }}
    </EmptyState>
    <div v-else class="space-y-3">
      <div
        v-for="m in maintenanceExpenses"
        :key="m.id"
        class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
      >
        <div class="space-y-1 min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-pink-500/10 text-pink-400 border border-pink-500/20 shrink-0">
              {{ categoryLabel(m.category) }}
            </span>
            <span class="text-xs text-slate-400 shrink-0">{{ formatDate(m.date) }}</span>
            <span v-if="m.is_recurring" class="text-xs text-slate-400 flex items-center gap-1 shrink-0">
              <Repeat class="w-3 h-3 text-pink-400" /> {{ $t('expenses.maintenancePanel.everyMonths', { recurrence_interval_months: m.recurrence_interval_months }) }}
              <template v-if="m.recurrence_end_date">{{ $t('expenses.maintenancePanel.until', { recurrence_end_date: formatDate(m.recurrence_end_date) }) }}</template>
            </span>
            <span v-else-if="m.amortization_mode === 'DISTANCE'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-success-500/10 text-success-400 border border-success-500/20 shrink-0">
              {{ $t('expenses.maintenancePanel.smoothedOverKm', { unit: distanceUnit(), coverage_km: m.coverage_km ? formatDistanceValue(m.coverage_km) : formatDistanceValue(50000) }) }}
            </span>
            <span v-else-if="m.amortization_mode === 'DURATION'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20 shrink-0">
              {{ $t('expenses.maintenancePanel.smoothedOverMonths', { coverage_months: m.coverage_months || 24 }) }}
            </span>
            <span v-else-if="m.amortization_mode === 'HYBRID'" class="text-xs px-2 py-0.5 rounded-full font-medium bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 shrink-0">
              {{ $t('expenses.maintenancePanel.mixedSmoothingKmMonths', { unit: distanceUnit(), coverage_km: m.coverage_km ? formatDistanceValue(m.coverage_km) : formatDistanceValue(50000), coverage_months: m.coverage_months || 24 }) }}
            </span>
            <span v-if="m.closes_maintenance_id" class="text-xs px-2 py-0.5 rounded-full font-medium bg-warning-500/10 text-warning-400 border border-warning-500/20 shrink-0">
              {{ $t('expenses.maintenancePanel.closesThePreviousService') }}
            </span>
            <button
              v-if="m.document_id"
              @click="emit('view-document', m.document_id, m.document_filename, false)"
              class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
              :title="$t('expenses.maintenancePanel.viewTheReceipt')"
            >
              <Paperclip class="w-3 h-3 shrink-0" />
              <span class="truncate">{{ m.document_filename || $t('expenses.invoice') }}</span>
            </button>
          </div>
          <p class="text-sm font-semibold text-slate-200 truncate">{{ m.description }}</p>
          <p v-if="m.odometer" class="text-xs text-slate-400">{{ $t('common.atKmCapitalized', { unit: distanceUnit(), km: formatDistanceValue(m.odometer) }) }}</p>
        </div>
        <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
          <div class="text-lg font-extrabold text-pink-400">
            {{ formatAmount(m.amount, m.currency || vehicleStore.currency) }}
          </div>
          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
            <button
              @click="emit('edit', m)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-pink-400 rounded-xl transition-colors border border-slate-700/60"
              :title="$t('expenses.maintenancePanel.editThisExpense')"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('delete', m)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
              :title="$t('expenses.maintenancePanel.deleteThisExpense')"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
