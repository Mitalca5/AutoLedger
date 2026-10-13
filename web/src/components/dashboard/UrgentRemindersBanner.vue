<script setup lang="ts">
import { computed } from 'vue'
import type { MaintenanceReminder } from '@/services/api'
import { ArrowRight, AlertTriangle } from 'lucide-vue-next'
import { summarizeUrgentReminders } from '@/utils/dashboard'

// Alert about the overdue maintenance reminders; the one coming due next has its own quieter card (NextDueCard)
const props = defineProps<{ reminders: MaintenanceReminder[] }>()
const summary = computed(() => summarizeUrgentReminders(props.reminders.filter((r) => r.status === 'OVERDUE')))
const urgentReminders = computed(() => summary.value.urgent)
const urgentRemindersSummary = computed(() => summary.value.summary)
</script>

<template>
  <div
    v-if="urgentReminders.length > 0"
    class="p-4 rounded-2xl border bg-danger-500/10 border-danger-500/30 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
  >
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-xl flex items-center justify-center shrink-0 bg-rose-500/20 text-rose-400">
        <AlertTriangle class="w-5 h-5" aria-hidden="true" />
      </div>
      <div>
        <div class="text-sm font-bold text-white flex items-center gap-2">
          <span>{{ $t('dashboard.urgentRemindersBanner.overdue') }}</span>
          <span class="px-2 py-0.5 text-xs rounded-full font-bold bg-rose-500/20 text-rose-400">{{ urgentReminders.length }}</span>
        </div>
        <p class="text-xs text-slate-300 mt-0.5">
          {{ urgentRemindersSummary }}
        </p>
      </div>
    </div>
    <router-link
      to="/maintenance?tab=REMINDERS"
      class="px-3.5 py-1.5 text-xs font-semibold rounded-xl shrink-0 transition-colors inline-flex items-center gap-1.5 self-start sm:self-auto shadow-sm bg-danger-600 hover:bg-danger-500 text-white"
    >
      {{ $t('dashboard.urgentRemindersBanner.viewTheReminders') }}
      <ArrowRight class="w-3.5 h-3.5" />
    </router-link>
  </div>
</template>
