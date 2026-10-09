<script setup lang="ts">
import { computed } from 'vue'
import type { MaintenanceReminder } from '@/services/api'
import { CalendarClock, ArrowRight } from 'lucide-vue-next'
import { nextDueReminder } from '@/utils/dashboard'
import { hasReminderSchedule, reminderDaysLabel } from '@/utils/expenses'
import { distanceUnit, formatDistanceValue } from '@/units'

// The maintenance that comes due next; overdue ones are left to the urgent banner, so the card hides when only those remain.
const props = defineProps<{ reminders: MaintenanceReminder[]; canEdit: boolean }>()
const next = computed(() => nextDueReminder(props.reminders))
const hasSchedule = computed(() => props.reminders.some(hasReminderSchedule))
const soon = computed(() => next.value?.reminder.status === 'DUE_SOON')
</script>

<template>
  <div
    v-if="next || !hasSchedule"
    class="p-4 rounded-2xl border bg-slate-900 flex sm:flex-row sm:items-center justify-between gap-3"
    :class="[soon ? 'border-warning-500/30' : 'border-slate-800', next ? 'flex-col' : 'items-center']"
  >
    <div class="flex items-center gap-3 min-w-0 flex-1">
      <div
        class="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
        :class="soon ? 'bg-warning-500/20 text-warning-400' : 'bg-sky-500/10 text-sky-400'"
      >
        <CalendarClock class="w-5 h-5" />
      </div>
      <div v-if="next" class="min-w-0 flex-1 space-y-1.5">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span class="text-xs text-slate-400">{{ $t('dashboard.nextDueCard.title') }}</span>
          <span class="text-sm font-bold text-white truncate">{{ next.reminder.title }}</span>
          <span v-if="next.reminder.remaining_km != null" class="text-xs font-semibold text-slate-300">
            {{
              next.reminder.remaining_km <= 0
                ? $t('expenses.remindersPanel.kmOver', { unit: distanceUnit(), km: formatDistanceValue(Math.abs(Math.round(next.reminder.remaining_km))) })
                : $t('expenses.remindersPanel.kmLeft', { unit: distanceUnit(), km: formatDistanceValue(next.reminder.remaining_km) })
            }}
          </span>
          <span v-if="next.reminder.remaining_days != null" class="text-xs font-semibold text-slate-300">
            {{ reminderDaysLabel({ remaining_days: next.reminder.remaining_days, scheduled_date: next.reminder.scheduled_date }) }}
          </span>
        </div>
        <div class="h-1.5 rounded-full bg-slate-800 overflow-hidden" role="progressbar" :aria-valuenow="Math.round(next.progress * 100)" aria-valuemin="0" aria-valuemax="100">
          <div class="h-full rounded-full transition-all" :class="soon ? 'bg-warning-400' : 'bg-sky-500'" :style="{ width: `${Math.round(next.progress * 100)}%` }"></div>
        </div>
      </div>
      <p v-else class="text-sm text-slate-300">{{ $t('dashboard.nextDueCard.empty') }}</p>
    </div>
    <router-link
      v-if="next || canEdit"
      to="/maintenance?tab=REMINDERS"
      :class="{ 'self-start': next }"
      class="btn btn-lg btn-secondary tap-text shrink-0 sm:self-auto"
    >
      {{ next ? $t('dashboard.nextDueCard.viewReminders') : $t('dashboard.nextDueCard.create') }}
      <ArrowRight class="w-3.5 h-3.5" />
    </router-link>
  </div>
</template>
