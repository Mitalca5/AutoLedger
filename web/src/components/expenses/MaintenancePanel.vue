<script setup lang="ts">
import ListSkeleton from '@/components/ListSkeleton.vue'
import EmptyState from '@/components/EmptyState.vue'
import { intlLocale } from '@/i18n'
import { useVehicleStore } from '@/stores/vehicle'
import { computed, ref } from 'vue'
import { Repeat, Pencil, Trash2, Paperclip } from 'lucide-vue-next'
import { categoryLabel, filterMaintenance, formatDate, maintenanceTotal, maintenanceYears } from '@/utils/expenses'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistanceValue } from '@/units'

const props = defineProps<{ maintenanceExpenses: any[]; loading: boolean }>()
const emit = defineEmits<{
  edit: [expense: any]
  delete: [expense: any]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const vehicleStore = useVehicleStore()

const filter = ref({ category: '', year: '' })
const categories = computed(() => [...new Set(props.maintenanceExpenses.map((m) => m.category as string))])
const years = computed(() => maintenanceYears(props.maintenanceExpenses))
const visible = computed(() => filterMaintenance(props.maintenanceExpenses, filter.value))
const total = computed(() => maintenanceTotal(visible.value, vehicleStore.currency))
</script>

<template>
  <div>
    <ListSkeleton v-if="loading" />
    <EmptyState v-else-if="!maintenanceExpenses.length">
      {{ $t('expenses.maintenancePanel.noMaintenanceOrFixedExpense') }}
    </EmptyState>
    <div v-else class="space-y-3">
      <div class="grid grid-cols-2 items-center gap-2 sm:flex sm:flex-wrap">
        <label class="sr-only" for="maint-filter-category">{{ $t('expenses.maintenancePanel.categoryFilter') }}</label>
        <select id="maint-filter-category" v-model="filter.category" class="field !w-auto">
          <option value="">{{ $t('expenses.maintenancePanel.allCategories') }}</option>
          <option v-for="c in categories" :key="c" :value="c">{{ categoryLabel(c) }}</option>
        </select>
        <label class="sr-only" for="maint-filter-year">{{ $t('expenses.maintenancePanel.yearFilter') }}</label>
        <select id="maint-filter-year" v-model="filter.year" class="field !w-auto">
          <option value="">{{ $t('expenses.maintenancePanel.allYears') }}</option>
          <option v-for="y in years" :key="y" :value="String(y)">{{ y }}</option>
        </select>
        <p class="col-span-2 text-sm text-slate-400 sm:ml-auto">
          {{ $t('expenses.maintenancePanel.total') }}
          <span class="font-extrabold text-white">{{ formatAmount(total, vehicleStore.currency) }}</span>
          <span class="text-xs"> · {{ $t('expenses.maintenancePanel.count', { n: visible.length }) }}</span>
        </p>
      </div>
      <EmptyState v-if="!visible.length">{{ $t('expenses.maintenancePanel.noMatch') }}</EmptyState>
      <div
        v-for="m in visible"
        :key="m.id"
        class="bg-slate-900 border border-slate-800 p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
      >
        <div class="space-y-1 min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-slate-800 text-slate-300 border border-slate-700 shrink-0">
              {{ categoryLabel(m.category) }}
            </span>
            <span class="text-xs text-slate-400 shrink-0">{{ formatDate(m.date) }}</span>
            <span v-if="m.odometer" class="text-xs text-slate-400 shrink-0">· {{ $t('common.atKmCapitalized', { unit: distanceUnit(), km: formatDistanceValue(m.odometer) }) }}</span>
            <span v-if="m.is_recurring" class="text-xs text-slate-400 flex items-center gap-1 shrink-0">
              <Repeat class="w-3 h-3 text-slate-400" /> {{ $t('expenses.maintenancePanel.everyMonths', { recurrence_interval_months: m.recurrence_interval_months }) }}
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
        </div>
        <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
          <div class="text-lg font-extrabold text-white">
            {{ formatAmount(m.amount, m.currency || vehicleStore.currency) }}
          </div>
          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
            <button
              @click="emit('edit', m)"
              class="tap p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white rounded-xl transition-colors border border-slate-700/60"
              :title="$t('expenses.maintenancePanel.editThisExpense')"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              @click="emit('delete', m)"
              class="tap p-1.5 bg-slate-800 hover:bg-rose-900/40 text-rose-400/80 hover:text-rose-300 focus-visible:text-rose-300 rounded-xl transition-colors border border-slate-700/60 hover:border-rose-500/40"
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
