<script setup lang="ts">
import ListSkeleton from '@/components/ListSkeleton.vue'
import { computed, ref } from 'vue'
import { useVehicleStore } from '@/stores/vehicle'
import type { MaintenanceReminder, VehicleWebhook } from '@/services/api'
import { Plus, Pencil, Trash2, AlertTriangle, Bell, Clock, CheckCircle2, Radio, Sparkles, CircleDashed, Settings } from 'lucide-vue-next'
import { reminderPresets, formatDate, hasReminderSchedule, type ReminderPreset } from '@/utils/expenses'
import { distanceUnit, formatDistanceValue } from '@/units'
import { sortRemindersByUrgency } from '@/utils/dashboard'
import ReminderTemplatesBar from '@/components/expenses/ReminderTemplatesBar.vue'

const props = defineProps<{
  reminders: MaintenanceReminder[]
  overdueReminders: MaintenanceReminder[]
  dueSoonReminders: MaintenanceReminder[]
  okReminders: MaintenanceReminder[]
  unscheduledReminders: MaintenanceReminder[]
  loadingReminders: boolean
  vehicleWebhook: VehicleWebhook | null
}>()
const emit = defineEmits<{
  'open-webhook': []
  add: [preset: ReminderPreset | null]
  edit: [reminder: MaintenanceReminder]
  complete: [reminder: MaintenanceReminder]
  delete: [reminder: MaintenanceReminder]
  reload: []
}>()
const vehicleStore = useVehicleStore()
const showSettings = ref(false)
const sortedReminders = computed(() => sortRemindersByUrgency(props.reminders))
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <ul class="flex flex-wrap items-center gap-2 text-xs font-semibold" :aria-label="$t('expenses.remindersPanel.summary')">
        <li class="px-2.5 py-1 rounded-full bg-slate-800 text-slate-300 border border-slate-700">{{ $t('expenses.remindersPanel.totalCount', { count: reminders.length }) }}</li>
        <li v-if="overdueReminders.length" class="px-2.5 py-1 rounded-full bg-rose-500/15 text-rose-300 border border-rose-500/30 flex items-center gap-1.5">
          <AlertTriangle class="w-3 h-3" />{{ overdueReminders.length }} {{ $t('expenses.remindersPanel.overdue') }}
        </li>
        <li v-if="dueSoonReminders.length" class="px-2.5 py-1 rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30 flex items-center gap-1.5">
          <Clock class="w-3 h-3" />{{ dueSoonReminders.length }} {{ $t('expenses.remindersPanel.comingUp') }}
        </li>
        <li v-if="okReminders.length" class="px-2.5 py-1 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1.5">
          <CheckCircle2 class="w-3 h-3" />{{ okReminders.length }} {{ $t('expenses.remindersPanel.upToDate') }}
        </li>
      </ul>
      <button
        type="button"
        class="tap-text px-3 text-xs font-semibold rounded-xl gap-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition-colors"
        :aria-expanded="showSettings"
        @click="showSettings = !showSettings"
      >
        <Settings class="w-3.5 h-3.5" />
        {{ $t('expenses.remindersPanel.settings') }}
        <span v-if="vehicleWebhook?.enabled" class="w-1.5 h-1.5 rounded-full bg-emerald-400" :title="$t('expenses.remindersPanel.active', { type: vehicleWebhook.type })"></span>
      </button>
    </div>

    <div v-if="showSettings" class="space-y-4">
      <div class="p-4 bg-slate-900 border border-slate-800 rounded-2xl flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-violet-500/10 border border-violet-500/20 text-violet-400 shrink-0">
            <Radio class="w-5 h-5" />
          </div>
          <div>
            <div class="font-bold text-white flex items-center gap-2">
              <span>{{ $t('expenses.remindersPanel.homelabWebhook') }}</span>
              <span
                class="px-2 py-0.5 text-xs rounded-full font-bold border"
                :class="vehicleWebhook?.enabled ? 'bg-success-500/10 text-success-400 border-success-500/20' : 'bg-slate-800 text-slate-400 border-slate-700'"
              >
                {{ vehicleWebhook?.enabled ? $t('expenses.remindersPanel.active', { type: vehicleWebhook.type }) : $t('expenses.remindersPanel.notConfigured') }}
              </span>
            </div>
            <p class="text-slate-400 text-xs mt-0.5">
              {{ vehicleWebhook?.enabled ? $t('expenses.remindersPanel.alertsSent') : $t('expenses.remindersPanel.alertsHint') }}
            </p>
          </div>
        </div>
        <button
          v-if="vehicleStore.canEdit"
          @click="emit('open-webhook')"
          class="px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold rounded-xl border border-slate-700 shrink-0 transition-colors self-start sm:self-auto"
        >
          {{ vehicleWebhook ? $t('expenses.remindersPanel.editWebhook') : $t('expenses.remindersPanel.setUpWebhook') }}
        </button>
      </div>

      <ReminderTemplatesBar v-if="vehicleStore.canEdit" :has-reminders="reminders.length > 0" @changed="emit('reload')" />
    </div>

    <ListSkeleton v-if="loadingReminders" :label="$t('expenses.remindersPanel.loadingTheReminders')" />

    <!-- Empty state -->
    <div v-else-if="!reminders.length" class="p-8 text-center bg-slate-900 border border-slate-800 rounded-2xl text-slate-400 space-y-4">
      <div class="p-3 bg-violet-500/10 border border-violet-500/20 text-violet-400 w-12 h-12 rounded-2xl mx-auto flex items-center justify-center">
        <Bell class="w-6 h-6" />
      </div>
      <div>
        <h3 class="text-base font-bold text-white">{{ $t('expenses.remindersPanel.noMaintenanceReminderConfigured') }}</h3>
        <p class="text-xs text-slate-400 mt-1 max-w-md mx-auto">
          {{ $t('expenses.remindersPanel.trackTireWearCabinFilter') }}
        </p>
      </div>

      <div v-if="vehicleStore.canEdit" class="pt-2">
        <p class="text-xs font-semibold text-slate-300 mb-3">{{ $t('expenses.remindersPanel.addAStandardReminderIn') }}</p>
        <div class="flex flex-wrap justify-center gap-2 max-w-lg mx-auto">
          <button
            v-for="preset in reminderPresets()"
            :key="preset.title"
            type="button"
            @click="emit('add', preset)"
            class="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium rounded-xl border border-slate-700 transition-colors flex items-center gap-1.5"
          >
            <Sparkles class="w-3 h-3 text-violet-400" />
            {{ preset.title }}
          </button>
        </div>
      </div>

      <div v-if="vehicleStore.canEdit" class="pt-2">
        <button
          @click="emit('add', null)"
          class="px-4 py-2 bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold rounded-xl inline-flex items-center gap-2 shadow-lg shadow-rose-600/20"
        >
          <Plus class="w-4 h-4" />
          {{ $t('expenses.remindersPanel.createACustomReminder') }}
        </button>
      </div>
    </div>

    <!-- Reminders List -->
    <div v-else class="space-y-3">
      <div
        v-for="r in sortedReminders"
        :key="r.id"
        class="bg-slate-900 border p-4 rounded-2xl flex flex-col justify-between gap-3 transition-colors"
        :class="r.status === 'OVERDUE' ? 'border-danger-500/40 bg-danger-500/5' : r.status === 'DUE_SOON' ? 'border-warning-500/40 bg-warning-500/5' : 'border-slate-800'"
      >
        <!-- Card top -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div class="flex items-center gap-2.5 flex-wrap">
            <span
              class="text-xs px-2.5 py-0.5 rounded-full font-bold border flex items-center gap-1"
              :class="r.status === 'OVERDUE' ? 'bg-rose-500/20 text-rose-300 border-rose-500/30' : r.status === 'DUE_SOON' ? 'bg-warning-500/20 text-warning-300 border-warning-500/30' : hasReminderSchedule(r) ? 'bg-success-500/10 text-success-400 border-success-500/20' : 'bg-slate-800 text-slate-400 border-slate-700'"
            >
              <AlertTriangle v-if="r.status === 'OVERDUE'" class="w-3 h-3" />
              <Clock v-else-if="r.status === 'DUE_SOON'" class="w-3 h-3" />
              <CheckCircle2 v-else-if="hasReminderSchedule(r)" class="w-3 h-3" />
              <CircleDashed v-else class="w-3 h-3" />
              {{ r.status === 'OVERDUE' ? $t('expenses.remindersPanel.overdue') : r.status === 'DUE_SOON' ? $t('expenses.remindersPanel.dueSoon') : hasReminderSchedule(r) ? $t('expenses.remindersPanel.upToDate') : $t('expenses.remindersPanel.withoutSchedule') }}
            </span>

            <span class="text-xs px-2 py-0.5 rounded-lg bg-slate-800 text-slate-300 border border-slate-700">
              {{ r.category === 'TIRES' ? $t('expenses.remindersPanel.tires') : $t('expenses.remindersPanel.maintenance') }}
            </span>

            <h4 class="text-sm font-bold text-white">{{ r.title }}</h4>

            <span
              v-if="r.webhook_enabled"
              class="text-xs px-2 py-0.5 rounded-full bg-violet-500/10 text-violet-400 border border-violet-500/20 flex items-center gap-1"
              :title="$t('expenses.remindersPanel.webhookNotificationEnabledForThis')"
            >
              <Radio class="w-2.5 h-2.5" />
              {{ $t('expenses.remindersPanel.webhook') }}
            </span>
          </div>

          <!-- Due Badges / Urgency pill -->
          <div class="flex items-center gap-2 text-xs font-semibold">
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
              :class="r.remaining_days <= 0 ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' : r.remaining_days <= r.lead_days ? 'bg-warning-500/20 text-warning-300 border border-warning-500/30' : 'text-slate-300 bg-slate-800 border border-slate-700'"
            >
              {{ r.remaining_days <= 0 ? $t('expenses.remindersPanel.daysOver', { days: Math.abs(r.remaining_days) }) : $t('expenses.remindersPanel.daysLeft', { days: r.remaining_days }) }}
            </span>
          </div>
        </div>

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

          <div v-if="r.interval_months" class="bg-slate-800/40 p-2.5 rounded-xl border border-slate-800">
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
              @click="emit('complete', r)"
              class="px-2.5 py-1.5 bg-success-600/20 hover:bg-success-600/30 text-success-300 text-xs font-semibold rounded-xl border border-success-500/30 flex items-center gap-1.5 transition-colors"
              :title="$t('expenses.remindersPanel.markThisMaintenanceAsDone')"
            >
              <CheckCircle2 class="w-3.5 h-3.5 text-success-400" />
              <span>{{ $t('expenses.remindersPanel.markDone') }}</span>
            </button>
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
  </div>
</template>
