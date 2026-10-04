<script setup lang="ts">
import { computed } from 'vue'
import type { MaintenanceReminder } from '@/services/api'
import { ArrowRight, AlertTriangle, Clock } from 'lucide-vue-next'
import { summarizeUrgentReminders } from '@/utils/dashboard'

// Alert about maintenance reminders that are overdue or due soon
const props = defineProps<{ reminders: MaintenanceReminder[] }>()
const summary = computed(() => summarizeUrgentReminders(props.reminders))
const urgentReminders = computed(() => summary.value.urgent)
const hasOverdueReminders = computed(() => summary.value.hasOverdue)
const urgentRemindersSummary = computed(() => summary.value.summary)
</script>

<template>
  <div
    v-if="urgentReminders.length > 0"
    class="p-4 rounded-2xl border flex flex-col sm:flex-row sm:items-center justify-between gap-3 transition-colors"
    :class="hasOverdueReminders ? 'bg-danger-500/10 border-danger-500/30' : 'bg-warning-500/10 border-warning-500/30'"
  >
    <div class="flex items-center gap-3">
      <div
        class="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
        :class="hasOverdueReminders ? 'bg-rose-500/20 text-rose-400' : 'bg-warning-500/20 text-warning-400'"
      >
        <AlertTriangle v-if="hasOverdueReminders" class="w-5 h-5" />
        <Clock v-else class="w-5 h-5" />
      </div>
      <div>
        <div class="text-sm font-bold text-white flex items-center gap-2">
          <span>{{ hasOverdueReminders ? $t('dashboard.urgentRemindersBanner.overdue') : $t('dashboard.urgentRemindersBanner.dueSoon') }}</span>
          <span
            class="px-2 py-0.5 text-xs rounded-full font-bold"
            :class="hasOverdueReminders ? 'bg-rose-500/20 text-rose-400' : 'bg-warning-500/20 text-warning-400'"
          >
            {{ urgentReminders.length }}
          </span>
        </div>
        <p class="text-xs text-slate-300 mt-0.5">
          {{ urgentRemindersSummary }}
        </p>
      </div>
    </div>
    <router-link
      to="/expenses?tab=REMINDERS"
      class="px-3.5 py-1.5 text-xs font-semibold rounded-xl shrink-0 transition-colors inline-flex items-center gap-1.5 self-start sm:self-auto shadow-sm"
      :class="hasOverdueReminders ? 'bg-danger-600 hover:bg-danger-500 text-white' : 'bg-warning-600 hover:bg-warning-500 text-white'"
    >
      {{ $t('dashboard.urgentRemindersBanner.viewTheReminders') }}
      <ArrowRight class="w-3.5 h-3.5" />
    </router-link>
  </div>
</template>
