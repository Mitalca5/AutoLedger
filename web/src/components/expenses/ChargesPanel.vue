<script setup lang="ts">
import EmptySourceHints from '@/components/EmptySourceHints.vue'
import { useRouter } from 'vue-router'
import { useVehicleStore } from '@/stores/vehicle'
import { Zap, Pencil, Trash2, AlertTriangle, Paperclip } from 'lucide-vue-next'
import { formatDate } from '@/utils/expenses'
import { formatAmount } from '@/currency'
import { distanceUnit, formatPerDistanceValue, perDistance } from '@/units'

defineProps<{
  charges: any[]
  chargesTotal: number
  chargesWithoutCost: number
  loading: boolean
  loadingMoreCharges: boolean
  missingCostOnly: boolean
}>()
const emit = defineEmits<{
  'toggle-missing-cost': []
  'import-csv': []
  'load-more': []
  edit: [charge: any]
  delete: [charge: any]
  'view-document': [docId: string | null | undefined, filename?: string | null, download?: boolean]
}>()
const router = useRouter()
const vehicleStore = useVehicleStore()
</script>

<template>
  <div class="space-y-3">
    <!-- Energy estimate banner if configured -->
    <div
      v-if="vehicleStore.activeVehicle?.estimated_kwh_100km && vehicleStore.activeVehicle?.estimated_price_per_kwh"
      class="p-3 bg-info-500/10 border border-info-500/20 rounded-2xl flex items-center justify-between text-xs text-info-300"
    >
      <div class="flex items-center gap-2">
        <Zap class="w-4 h-4 shrink-0 text-info-400" />
        <span>
          {{ $t('expenses.chargesPanel.energyEstimateActive') }}
          <strong>{{ $t('expenses.chargesPanel.kwh100km', { unit: distanceUnit(), estimated_kwh_100km: formatPerDistanceValue(Number(vehicleStore.activeVehicle.estimated_kwh_100km)) }) }}</strong> {{ $t('expenses.chargesPanel.at') }}
          <strong>{{ $t('expenses.chargesPanel.kwh3', { estimated_price_per_kwh: formatAmount(Number(vehicleStore.activeVehicle.estimated_price_per_kwh), vehicleStore.currency, 4) }) }}</strong>
          {{ $t('expenses.chargesPanel.automaticallyIncludedInTheTco') }}
        </span>
      </div>
      <button
        @click="router.push('/vehicles')"
        class="shrink-0 font-medium underline hover:text-info-200 transition-colors ml-2"
      >
        {{ $t('common.edit') }}
      </button>
    </div>

    <button
      v-if="chargesWithoutCost > 0 || missingCostOnly"
      @click="emit('toggle-missing-cost')"
      class="w-full p-3 rounded-2xl text-left text-xs font-semibold flex items-center gap-2 border transition-colors"
      :class="missingCostOnly ? 'bg-warning-500/20 border-warning-500/40 text-warning-300' : 'bg-warning-500/10 border-warning-500/20 text-warning-400 hover:bg-warning-500/15'"
    >
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span v-if="missingCostOnly">{{ $t('expenses.chargesPanel.showingOnlyTheChargesWithout') }}</span>
      <span v-else>{{ $t('expenses.chargesPanel.chargeSWithoutACost', { chargesWithoutCost }) }}</span>
    </button>
    <div v-if="loading" class="text-center py-12 text-slate-400">{{ $t('common.loading') }}</div>
    <div v-else-if="!charges.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400">
      <p class="mb-3">{{ $t('expenses.chargesPanel.empty') }}</p>
      <EmptySourceHints quick-kind="CHARGE" @import-csv="emit('import-csv')" />
    </div>
    <div v-else class="space-y-3">
      <div
        v-for="c in charges"
        :key="c.id"
        class="bg-slate-900 border p-4 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3"
        :class="c.cost === null ? 'border-warning-500/40' : 'border-slate-800'"
      >
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-xs px-2 py-0.5 rounded-full font-bold bg-info-500/10 text-info-400 border border-info-500/20 shrink-0">
              {{ $t('expenses.chargesPanel.kwh2', { kwh_added: c.kwh_added }) }}
            </span>
            <span class="text-xs text-slate-400 shrink-0">{{ formatDate(c.date) }}</span>
            <span v-if="c.is_manual" class="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700 shrink-0">{{ $t('expenses.chargesPanel.manual') }}</span>
            <span v-else-if="c.cost_source === 'MANUAL'" class="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700 shrink-0">{{ $t('expenses.chargesPanel.correctedCost') }}</span>
            <button
              v-if="c.document_id"
              @click="emit('view-document', c.document_id, c.document_filename, false)"
              class="text-xs px-2 py-0.5 rounded-lg bg-indigo-500/10 hover:bg-indigo-500/20 text-indigo-400 border border-indigo-500/20 flex items-center gap-1 transition-colors max-w-[200px] truncate"
              :title="$t('expenses.chargesPanel.viewTheReceipt')"
            >
              <Paperclip class="w-3 h-3 shrink-0" />
              <span class="truncate">{{ c.document_filename || $t('expenses.invoice') }}</span>
            </button>
          </div>
          <p class="text-sm text-slate-300 mt-1 truncate" :title="c.address">{{ c.address || $t('expenses.chargesPanel.unknownPlace') }}</p>
        </div>
        <div class="flex items-center justify-between sm:justify-end gap-3 shrink-0">
          <div class="text-left sm:text-right">
            <template v-if="c.cost !== null">
              <span class="text-lg font-extrabold text-info-400">{{ formatAmount(c.cost, c.currency || vehicleStore.currency) }}</span>
              <p v-if="c.kwh_added > 0" class="text-xs text-slate-400">
                {{ $t('expenses.chargesPanel.kwh', { cost: formatAmount(c.cost / c.kwh_added, c.currency || vehicleStore.currency, 3) }) }}
              </p>
            </template>
            <span v-else class="text-xs font-bold text-warning-400 flex items-center gap-1">
              <AlertTriangle class="w-3.5 h-3.5" /> {{ $t('expenses.chargesPanel.missingCost') }}
            </span>
          </div>
          <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
            <button
              @click="emit('edit', c)"
              class="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-info-400 rounded-xl transition-colors border border-slate-700/60"
              :title="c.is_manual ? $t('expenses.chargesPanel.editThisCharge') : $t('expenses.chargesPanel.fixCost')"
            >
              <Pencil class="w-3.5 h-3.5" />
            </button>
            <button
              v-if="c.is_manual"
              @click="emit('delete', c)"
              class="p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
              :title="$t('expenses.chargesPanel.deleteThisCharge')"
            >
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
      <div class="flex items-center justify-between text-xs text-slate-400 px-1">
        <span>{{ $t('expenses.chargesPanel.chargeSShownOutOf', { length: charges.length, chargesTotal }) }}</span>
        <button
          v-if="charges.length < chargesTotal"
          @click="emit('load-more')"
          :disabled="loadingMoreCharges"
          class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold rounded-xl border border-slate-700 disabled:opacity-50"
        >
          {{ loadingMoreCharges ? $t('common.loading') : $t('expenses.chargesPanel.loadMore') }}
        </button>
      </div>
    </div>
  </div>
</template>
