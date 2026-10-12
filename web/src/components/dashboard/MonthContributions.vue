<script setup lang="ts">
import { ref, watch } from 'vue'
import { api } from '@/services/api'
import { apiErrorMessage } from '@/services/apiError'
import { formatAmount } from '@/currency'
import { formatDayTime } from '@/utils/dates'
import { contributionRoute, recurrenceLabel } from '@/utils/monthContributions'
import { distanceUnit, formatDistanceValue } from '@/units'
import { formatNumber } from '@/utils/numbers'
import { t } from '@/i18n'

// Entries behind one category total of a month: the recorded ledger entries and, for energy, the estimate.
const props = defineProps<{ vehicleId: string; month: string; category: string; currency: string }>()

const data = ref<any | null>(null)
const loading = ref(false)
const error = ref('')
let sequence = 0

async function load() {
  const current = ++sequence
  loading.value = true
  error.value = ''
  try {
    const result = await api.getMonthlyContributions(props.vehicleId, props.month, props.category)
    if (current !== sequence) return
    data.value = result
  } catch (e: any) {
    if (current !== sequence) return
    data.value = null
    error.value = apiErrorMessage(e, t('dashboard.monthDetailModal.contributions.failed'))
  } finally {
    if (current === sequence) loading.value = false
  }
}

watch(() => [props.vehicleId, props.month, props.category], load, { immediate: true })
</script>

<template>
  <div class="mt-1.5 ml-5 pl-3 border-l border-slate-700/60 space-y-1.5 text-xs" aria-live="polite">
    <p v-if="loading" class="text-slate-400">{{ $t('dashboard.monthDetailModal.contributions.loading') }}</p>
    <p v-else-if="error" class="text-danger-400" role="alert">{{ error }}</p>
    <template v-else-if="data">
      <p v-if="!data.items.length && !data.estimate" class="text-slate-400">{{ $t('dashboard.monthDetailModal.contributions.empty') }}</p>
      <ul v-else class="space-y-1">
        <li v-for="item in data.items" :key="`${item.source_id}-${item.date}`" class="flex items-center justify-between gap-3">
          <div class="min-w-0">
            <router-link :to="contributionRoute(item.source_table)" class="text-sky-300 hover:text-sky-200 truncate">
              {{ item.label || formatDayTime(item.date) }}
            </router-link>
            <span class="text-slate-400"> · {{ formatDayTime(item.date) }}</span>
            <span v-if="item.recurring && item.interval_months > 0" class="text-slate-400"> · {{ $t(recurrenceLabel(item.interval_months).key, recurrenceLabel(item.interval_months).params) }}</span>
          </div>
          <span class="font-semibold text-white shrink-0">{{ formatAmount(item.amount, currency) }}</span>
        </li>
        <li v-if="data.estimate" class="flex items-center justify-between gap-3 text-slate-300">
          <span class="min-w-0">
            {{ $t('dashboard.monthDetailModal.contributions.estimate', {
              km: formatDistanceValue(data.estimate.km),
              unit: distanceUnit(),
              kwh: formatNumber(data.estimate.kwh, 1),
            }) }}
          </span>
          <span class="font-semibold text-white shrink-0">{{ formatAmount(data.estimate.amount, currency) }}</span>
        </li>
      </ul>
      <p v-if="category === 'INSURANCE'" class="text-slate-400">{{ $t('dashboard.monthDetailModal.contributions.insuranceNote') }}</p>
    </template>
  </div>
</template>
