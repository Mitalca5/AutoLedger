<script setup lang="ts">
import { computed, ref } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import type { MaintenanceReminder } from '@/services/api'
import { Pencil, Trash2, CheckCircle2, Radio, CircleDashed, ChevronDown } from 'lucide-vue-next'
import { formatDate, formatCalendarDay, hasReminderSchedule, reminderDaysLabel, reminderDaysOver, reminderDueTile } from '@/utils/expenses'
import { distanceUnit, formatDistanceValue } from '@/units'

// One reminder as a compact line (when it is due, how far, the done button); the details open on demand.
const props = defineProps<{ reminder: MaintenanceReminder }>()
const emit = defineEmits<{
  edit: [reminder: MaintenanceReminder]
  complete: [reminder: MaintenanceReminder]
  delete: [reminder: MaintenanceReminder]
}>()
const vehicleStore = useVehicleStore()
const expanded = ref(false)
const r = computed(() => props.reminder)
const tile = computed(() => reminderDueTile(r.value))
</script>

<template>
  <div
    class="bg-slate-900 border rounded-2xl transition-colors"
    :class="r.status === 'OVERDUE' ? 'border-danger-500/40 bg-danger-500/5' : r.status === 'DUE_SOON' ? 'border-warning-500/40 bg-warning-500/5' : 'border-slate-800'"
  >
    <div class="flex items-center gap-2 p-3">
      <button
        type="button"
        class="flex-1 min-w-0 flex items-center gap-3 text-left rounded-xl"
        :aria-expanded="expanded"
        @click="expanded = !expanded"
      >
        <span
          class="w-14 shrink-0 rounded-xl border py-1.5 text-center leading-tight"
          :class="r.status === 'OVERDUE' ? 'bg-rose-500/15 border-rose-500/30 text-rose-200' : r.status === 'DUE_SOON' ? 'bg-warning-500/15 border-warning-500/30 text-warning-200' : 'bg-slate-800/60 border-slate-700 text-slate-200'"
        >
          <template v-if="tile?.kind === 'date'">
            <span class="block text-[0.65rem] font-semibold uppercase tracking-wide opacity-80">{{ tile.month }}</span>
            <span class="block text-lg font-extrabold">{{ tile.day }}</span>
          </template>
          <template v-else-if="tile?.kind === 'km'">
            <span class="block text-xs font-extrabold">{{ formatDistanceValue(tile.km) }}</span>
            <span class="block text-[0.65rem] font-semibold uppercase opacity-80">{{ distanceUnit() }}</span>
          </template>
          <CircleDashed v-else class="w-5 h-5 mx-auto text-slate-400" />
        </span>
        <span class="min-w-0 flex-1 space-y-1">
          <span class="flex items-center gap-2 flex-wrap">
            <span class="text-sm font-bold text-white">{{ r.title }}</span>
            <span class="text-xs px-2 py-0.5 rounded-lg bg-slate-800 text-slate-300 border border-slate-700">
              {{ r.category === 'TIRES' ? $t('expenses.remindersPanel.tires') : $t('expenses.remindersPanel.maintenance') }}
            </span>
            <span
              v-if="r.webhook_enabled"
              class="text-xs px-2 py-0.5 rounded-full bg-violet-500/10 text-violet-400 border border-violet-500/20 flex items-center gap-1"
              :title="$t('expenses.remindersPanel.webhookNotificationEnabledForThis')"
            >
              <Radio class="w-2.5 h-2.5" />
              {{ $t('expenses.remindersPanel.webhook') }}
            </span>
          </span>
          <span class="flex items-center gap-2 flex-wrap text-xs font-semibold">
            <span
              v-if="r.remaining_km !== null && r.remaining_km !== undefined"
              class="px-2 py-0.5 rounded-lg"
              :class="r.remaining_km <= 0 ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' : r.remaining_km <= r.lead_km ? 'bg-warning-500/20 text-warning-300 border border-warning-500/30' : 'text-slate-300 bg-slate-800 border border-slate-700'"
            >
              {{ r.remaining_km <= 0 ? $t('expenses.remindersPanel.kmOver', { unit: distanceUnit(), km: formatDistanceValue(Math.abs(Math.round(r.remaining_km))) }) : $t('expenses.remindersPanel.kmLeft', { unit: distanceUnit(), km: formatDistanceValue(r.remaining_km) }) }}
            </span>
            <span
              v-if="r.remaining_days !== null && r.remaining_days !== undefined"
              class="px-2 py-0.5 rounded-lg"
              :class="reminderDaysOver({ remaining_days: r.remaining_days, scheduled_date: r.scheduled_date }) ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' : r.remaining_days <= r.lead_days ? 'bg-warning-500/20 text-warning-300 border border-warning-500/30' : 'text-slate-300 bg-slate-800 border border-slate-700'"
            >
              {{ reminderDaysLabel({ remaining_days: r.remaining_days, scheduled_date: r.scheduled_date }) }}
            </span>
            <span v-if="!hasReminderSchedule(r)" class="text-slate-400 font-medium">{{ $t('expenses.remindersPanel.withoutSchedule') }}</span>
          </span>
        </span>
        <ChevronDown class="w-4 h-4 shrink-0 text-slate-400 transition-transform" :class="expanded ? 'rotate-180' : ''" aria-hidden="true" />
      </button>
      <button
        v-if="vehicleStore.canEdit"
        type="button"
        class="px-2.5 py-1.5 bg-success-600/20 hover:bg-success-600/30 text-success-300 text-xs font-semibold rounded-xl border border-success-500/30 flex items-center gap-1.5 transition-colors shrink-0"
        :title="$t('expenses.remindersPanel.markThisMaintenanceAsDone')"
        @click="emit('complete', r)"
      >
        <CheckCircle2 class="w-3.5 h-3.5 text-success-400" />
        <span class="hidden sm:inline">{{ $t('expenses.remindersPanel.markDone') }}</span>
        <span class="sm:hidden sr-only">{{ $t('expenses.remindersPanel.markDone') }}</span>
      </button>
    </div>

    <div v-if="expanded" class="px-3 pb-3 space-y-3">
      <!-- Card details grid -->
      <div class="grid grid-cols-1 sm:grid-cols-[repeat(auto-fit,minmax(11rem,1fr))] gap-2.5 pt-1 text-xs text-slate-300">
        <div v-if="r.interval_km" class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
          <span class="text-xs text-slate-400 block mb-0.5">{{ $t('expenses.remindersPanel.mileageDue') }}</span>
          <span class="font-medium text-white">
            {{ $t('expenses.remindersPanel.everyKm', { unit: distanceUnit(), interval_km: formatDistanceValue(r.interval_km) }) }}
            <span v-if="r.observed_interval_km" class="text-slate-400 block text-xs">
              {{ $t('expenses.remindersPanel.observedInterval', { km: formatDistanceValue(r.observed_interval_km), unit: distanceUnit(), months: r.observed_interval_months ?? '–' }) }}
            </span>
            <span v-if="r.due_odometer" class="text-slate-400 block text-xs">
              {{ $t('expenses.remindersPanel.dueAtKm', { unit: distanceUnit(), due_odometer: formatDistanceValue(r.due_odometer) }) }}
            </span>
          </span>
        </div>

        <div v-if="r.scheduled_date" class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
          <span class="text-xs text-slate-400 block mb-0.5">{{ $t('expenses.remindersPanel.calendarDueDate') }}</span>
          <span class="font-medium text-white">
            {{ r.repeat_yearly ? $t('expenses.remindersPanel.everyYearOn', { date: formatCalendarDay(r.scheduled_date, false) }) : $t('expenses.remindersPanel.onDate', { date: formatCalendarDay(r.scheduled_date) }) }}
            <span v-if="r.due_date && r.repeat_yearly" class="text-slate-400 block text-xs">
              {{ $t('expenses.remindersPanel.due', { due_date: formatDate(r.due_date) }) }}
            </span>
          </span>
        </div>

        <div v-else-if="r.interval_months" class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
          <span class="text-xs text-slate-400 block mb-0.5">{{ $t('expenses.remindersPanel.calendarDueDate') }}</span>
          <span class="font-medium text-white">
            {{ $t('expenses.remindersPanel.everyMonths', { interval_months: r.interval_months }) }}
            <span v-if="r.due_date" class="text-slate-400 block text-xs">
              {{ $t('expenses.remindersPanel.due', { due_date: formatDate(r.due_date) }) }}
            </span>
          </span>
        </div>

        <div class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
          <span class="text-xs text-slate-400 block mb-0.5">{{ $t('expenses.remindersPanel.lastCompleted') }}</span>
          <span class="font-medium text-white">
            {{ r.last_service_date ? formatDate(r.last_service_date) : $t('expenses.remindersPanel.notEntered') }}
            <span v-if="r.last_service_odometer" class="text-slate-400 block text-xs">
              {{ $t('common.atKm', { unit: distanceUnit(), km: formatDistanceValue(r.last_service_odometer) }) }}
            </span>
          </span>
          <span v-if="r.maintenance" class="mt-1 block text-xs text-violet-300">
            {{ $t('expenses.remindersPanel.basedOnMaintenance', { date: formatDate(r.maintenance.date) }) }}
          </span>
        </div>
      </div>

      <!-- Card footer -->
      <div class="flex items-center justify-between pt-2 border-t border-slate-800/80">
        <div class="text-xs text-slate-400">
          <span v-if="r.last_notified_at">
            {{ $t('expenses.remindersPanel.lastWebhookAlert', { last_notified_at: formatDate(r.last_notified_at) }) }}
          </span>
          <span v-else>
            {{ $t('expenses.remindersPanel.earlyAlertKmDBefore', { unit: distanceUnit(), lead_km: formatDistanceValue(r.lead_km), lead_days: r.lead_days }) }}
          </span>
        </div>

        <div v-if="vehicleStore.canEdit" class="flex items-center gap-1.5">
          <button
            @click="emit('edit', r)"
            class="tap p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-violet-400 rounded-xl transition-colors border border-slate-700/60"
            :title="$t('expenses.remindersPanel.editThisReminder')"
          >
            <Pencil class="w-3.5 h-3.5" />
          </button>
          <button
            @click="emit('delete', r)"
            class="tap p-1.5 bg-slate-800 hover:bg-rose-900/40 text-slate-400 hover:text-rose-400 rounded-xl transition-colors border border-slate-700/60"
            :title="$t('expenses.remindersPanel.deleteThisReminder')"
          >
            <Trash2 class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
