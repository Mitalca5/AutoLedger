<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, nextTick, ref, watch } from 'vue'
import { api, type MaintenanceReminder } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useMaintenanceChoices } from '@/composables/useMaintenanceChoices'
import { X, Bell, Sparkles } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { reminderPresets, nextOccurrenceDate, maintenanceStartPoint, type ReminderPreset } from '@/utils/expenses'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

// Creates a maintenance reminder, or edits `editing`. `preset` pre-fills a new one from a suggestion.
const props = defineProps<{ vehicleId: string; editing: MaintenanceReminder | null; preset: ReminderPreset | null; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const editingReminderId = computed(() => props.editing?.id ?? null)

const scheduleMode = ref<'interval' | 'date'>('interval')

const { options: maintenanceOptions, load: loadMaintenanceChoices, label: maintenanceOptionLabel, find: findMaintenance } = useMaintenanceChoices(() => props.vehicleId)

// Basing a reminder on a recorded maintenance takes that maintenance's day and odometer as starting point.
function onMaintenanceChosen() {
  const m = findMaintenance(reminderForm.value.maintenance_id)
  if (!m) return
  const start = maintenanceStartPoint(m)
  reminderForm.value.last_service_date = start.date
  if (start.odometer !== '') reminderForm.value.last_service_odometer = start.odometer
}

const reminderForm = ref({
  title: '',
  category: 'MAINTENANCE',
  interval_km: '' as number | '',
  interval_months: '' as number | '',
  scheduled_date: '',
  repeat_yearly: true,
  last_service_odometer: '' as number | '',
  last_service_date: todayIso(),
  lead_km: 1000,
  lead_days: 15,
  webhook_enabled: true,
  maintenance_id: '',
})

function applyReminderPreset(preset: ReminderPreset) {
  reminderForm.value.title = preset.title
  reminderForm.value.category = preset.category
  reminderForm.value.interval_km = preset.interval_km
  reminderForm.value.interval_months = preset.interval_months
  if (preset.scheduled_month && preset.scheduled_day) {
    scheduleMode.value = 'date'
    reminderForm.value.scheduled_date = nextOccurrenceDate(preset.scheduled_month, preset.scheduled_day)
    reminderForm.value.repeat_yearly = true
    reminderForm.value.last_service_date = ''
  } else {
    scheduleMode.value = 'interval'
    reminderForm.value.scheduled_date = ''
    if (!reminderForm.value.last_service_date) reminderForm.value.last_service_date = todayIso()
  }
  reminderForm.value.lead_km = preset.lead_km
  reminderForm.value.lead_days = preset.lead_days
}

watch(open, (isOpen) => {
  if (!isOpen) return
  submitted.value = false
  const r = props.editing
  if (!r) {
    const currentOdo = props.currentOdometer ? Math.round(props.currentOdometer) : ''
    reminderForm.value = {
      title: '',
      category: 'MAINTENANCE',
      interval_km: 10000,
      interval_months: 12,
      scheduled_date: '',
      repeat_yearly: true,
      last_service_odometer: currentOdo,
      last_service_date: todayIso(),
      lead_km: 1000,
      lead_days: 15,
      webhook_enabled: true,
      maintenance_id: '',
    }
    scheduleMode.value = 'interval'
    if (props.preset) applyReminderPreset(props.preset)
  } else {
    scheduleMode.value = r.scheduled_date ? 'date' : 'interval'
    reminderForm.value = {
      title: r.title,
      category: r.category,
      interval_km: r.interval_km ?? '',
      interval_months: r.interval_months ?? '',
      scheduled_date: r.scheduled_date ? r.scheduled_date.substring(0, 10) : '',
      repeat_yearly: r.repeat_yearly ?? false,
      last_service_odometer: r.last_service_odometer !== null && r.last_service_odometer !== undefined ? Math.round(r.last_service_odometer) : '',
      last_service_date: r.last_service_date ? r.last_service_date.substring(0, 10) : r.scheduled_date ? '' : todayIso(),
      lead_km: r.lead_km,
      lead_days: r.lead_days,
      webhook_enabled: r.webhook_enabled,
      maintenance_id: r.maintenance_id ?? '',
    }
  }
  loadMaintenanceChoices()
})

// A fixed-date reminder is settled by a service date close to its occurrence, so a new one starts without a last service date.
function setScheduleMode(mode: 'interval' | 'date') {
  scheduleMode.value = mode
  if (editingReminderId.value) return
  if (mode === 'date') reminderForm.value.last_service_date = ''
  else if (!reminderForm.value.last_service_date) reminderForm.value.last_service_date = todayIso()
}

const { pending: submitting, run: runOnce } = useSubmit()

const submitted = ref(false)
const errors = computed(() => {
  const byDate = scheduleMode.value === 'date'
  return {
    title: !reminderForm.value.title.trim() ? t('expenses.reminderModal.titleRequired') : '',
    date: byDate && !reminderForm.value.scheduled_date ? t('expenses.reminderModal.dateRequired') : '',
    interval: !byDate && !reminderForm.value.interval_km && !reminderForm.value.interval_months
      ? t('expenses.reminderModal.intervalRequired', { unit: distanceUnit() })
      : '',
  }
})
const visibleErrors = computed(() => (submitted.value ? errors.value : { title: '', date: '', interval: '' }))

async function handleSaveReminderAction() {
  if (!props.vehicleId) return
  submitted.value = true
  const byDate = scheduleMode.value === 'date'
  if (errors.value.title || errors.value.date || errors.value.interval) {
    await nextTick()
    document.querySelector<HTMLElement>('[data-reminder-form] [aria-invalid="true"]')?.focus()
    return
  }
  try {
    const payload = {
      title: reminderForm.value.title.trim(),
      category: reminderForm.value.category,
      interval_km: reminderForm.value.interval_km !== '' ? Number(reminderForm.value.interval_km) : null,
      interval_months: !byDate && reminderForm.value.interval_months !== '' ? Number(reminderForm.value.interval_months) : null,
      scheduled_date: byDate ? reminderForm.value.scheduled_date : null,
      repeat_yearly: byDate && reminderForm.value.repeat_yearly,
      last_service_odometer: reminderForm.value.last_service_odometer !== '' ? Number(reminderForm.value.last_service_odometer) : null,
      last_service_date: reminderForm.value.last_service_date ? new Date(reminderForm.value.last_service_date).toISOString() : null,
      lead_km: Number(reminderForm.value.lead_km || 0),
      lead_days: Number(reminderForm.value.lead_days || 0),
      webhook_enabled: reminderForm.value.webhook_enabled,
      maintenance_id: reminderForm.value.maintenance_id || null,
    }
    if (editingReminderId.value) {
      await api.updateReminder(props.vehicleId, editingReminderId.value, payload)
      showAlert(t('expenses.reminderModal.updated'), t('common.success'), 'success')
    } else {
      await api.createReminder(props.vehicleId, payload)
      showAlert(t('expenses.reminderModal.created'), t('common.success'), 'success')
    }
    open.value = false
    emit('saved')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleSaveReminder = () => runOnce(handleSaveReminderAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-[60] bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <Bell class="w-5 h-5 text-violet-400" />
          {{ editingReminderId ? $t('expenses.reminderModal.edit') : $t('expenses.reminderModal.new') }}
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="reminder-modal-form" data-reminder-form novalidate @submit.prevent="handleSaveReminder" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <!-- Preset chips (only when adding new) -->
        <div v-if="!editingReminderId" class="space-y-1.5">
          <span class="block text-xs font-semibold text-slate-300">{{ $t('expenses.reminderModal.quickTemplates') }}</span>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="preset in reminderPresets()"
              :key="preset.title"
              type="button"
              @click="applyReminderPreset(preset)"
              class="btn btn-secondary"
            >
              <Sparkles class="w-3 h-3 text-violet-400" />
              {{ preset.title }}
            </button>
          </div>
        </div>

        <!-- Title & Category -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div class="sm:col-span-2">
            <label for="reminder-form-title" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.maintenanceTitle') }}</label>
            <input
              id="reminder-form-title"
              v-model="reminderForm.title"
              type="text"
              required
              :aria-invalid="!!visibleErrors.title"
              :aria-describedby="visibleErrors.title ? 'reminder-form-title-error' : undefined"
              :placeholder="$t('expenses.reminderModal.eGTireRotation')"
              class="field"
            />
            <p v-if="visibleErrors.title" id="reminder-form-title-error" class="text-xs text-danger-400 mt-1">{{ visibleErrors.title }}</p>
          </div>
          <div>
            <label for="reminder-form-category" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.category') }}</label>
            <select
              id="reminder-form-category"
              v-model="reminderForm.category"
              class="field"
            >
              <option value="MAINTENANCE">{{ $t('expenses.reminderModal.maintenance') }}</option>
              <option value="TIRES">{{ $t('expenses.reminderModal.tires') }}</option>
            </select>
          </div>
        </div>

        <!-- Periodicities -->
        <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
          <span class="block text-xs font-semibold text-slate-200">{{ scheduleMode === 'date' ? $t('expenses.reminderModal.scheduleTitleDate') : $t('expenses.reminderModal.frequencyAtLeastOneOf') }}</span>
          <div role="group" :aria-label="$t('expenses.reminderModal.scheduleMode')" class="flex gap-1 rounded-xl border border-slate-800 bg-slate-950 p-1">
            <button
              v-for="mode in (['interval', 'date'] as const)"
              :key="mode"
              type="button"
              :aria-pressed="scheduleMode === mode"
              class="tap-text flex-1 whitespace-nowrap rounded-lg px-2 text-xs font-semibold transition-colors"
              :class="scheduleMode === mode ? 'bg-rose-500/15 text-rose-200' : 'text-slate-400 hover:text-white'"
              @click="setScheduleMode(mode)"
            >
              {{ mode === 'interval' ? $t('expenses.reminderModal.modeInterval') : $t('expenses.reminderModal.modeDate') }}
            </button>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="reminder-form-interval-km" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.intervalInKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput whole text
                id="reminder-form-interval-km"
                v-model="reminderForm.interval_km"
                min="500"
                step="500"
                :aria-invalid="!!visibleErrors.interval"
                :aria-describedby="visibleErrors.interval ? 'reminder-form-interval-error' : undefined"
                :placeholder="$t('expenses.reminderModal.eG10000EmptyIgnored')"
                class="field"
              />
              <p v-if="visibleErrors.interval" id="reminder-form-interval-error" class="text-xs text-danger-400 mt-1">{{ visibleErrors.interval }}</p>
            </div>
            <div v-if="scheduleMode === 'interval'">
              <label for="reminder-form-interval-months" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.intervalInMonths') }}</label>
              <input
                id="reminder-form-interval-months"
                v-model="reminderForm.interval_months"
                type="number"
                min="1"
                max="120"
                :placeholder="$t('expenses.reminderModal.eG12EmptyIgnored')"
                class="field"
              />
            </div>
            <div v-else>
              <label for="reminder-form-scheduled-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.scheduledDate') }}</label>
              <AppDatePicker id="reminder-form-scheduled-date" v-model="reminderForm.scheduled_date" size="sm" :required="true" />
              <p v-if="visibleErrors.date" class="text-xs text-danger-400 mt-1">{{ visibleErrors.date }}</p>
            </div>
          </div>
          <div v-if="scheduleMode === 'date'" class="flex items-center gap-2">
            <input
              id="reminder-form-repeat-yearly"
              v-model="reminderForm.repeat_yearly"
              type="checkbox"
              class="rounded border-slate-700 bg-slate-800 text-violet-600 focus:ring-violet-500"
            />
            <label for="reminder-form-repeat-yearly" class="text-xs text-slate-300 cursor-pointer">{{ $t('expenses.reminderModal.repeatYearly') }}</label>
          </div>
        </div>

        <!-- Previous maintenance this reminder is based on -->
        <div v-if="maintenanceOptions.length || reminderForm.maintenance_id" class="space-y-1">
          <label for="reminder-form-maintenance" class="block text-xs font-semibold text-slate-300">{{ $t('expenses.reminderModal.basedOn') }}</label>
          <select id="reminder-form-maintenance" v-model="reminderForm.maintenance_id" class="field" @change="onMaintenanceChosen">
            <option value="">{{ $t('expenses.reminderModal.basedOnNone') }}</option>
            <option v-for="m in maintenanceOptions" :key="m.id" :value="m.id">{{ maintenanceOptionLabel(m) }}</option>
          </select>
          <p v-if="reminderForm.maintenance_id" class="text-xs text-slate-400">{{ $t('expenses.reminderModal.basedOnHint') }}</p>
        </div>

        <!-- Last service date & odometer -->
        <div v-if="scheduleMode === 'interval' || reminderForm.interval_km" class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
          <span class="block text-xs font-semibold text-slate-200">{{ $t('expenses.reminderModal.startingPointLastMaintenance') }}</span>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="reminder-form-last-odo" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.odometerAtLastMaintenanceKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput text
                id="reminder-form-last-odo"
                v-model="reminderForm.last_service_odometer"
                min="0"
                :placeholder="$t('common.example', { value: '45000' })"
                class="field"
              />
            </div>
            <div v-if="scheduleMode === 'interval'">
              <label for="reminder-form-last-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.dateOfLastMaintenance') }}</label>
              <AppDatePicker
                id="reminder-form-last-date"
                v-model="reminderForm.last_service_date"
                size="sm"
                :clearable="true"
              />
            </div>
          </div>
        </div>

        <!-- Alert lead thresholds -->
        <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
          <span class="block text-xs font-semibold text-slate-200">{{ $t('expenses.reminderModal.alertLeadThreshold') }}</span>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="reminder-form-lead-km" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.alertBeforeKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput whole
                id="reminder-form-lead-km"
                v-model="reminderForm.lead_km"
                min="0"
                step="100"
                placeholder="1000"
                class="field"
              />
            </div>
            <div>
              <label for="reminder-form-lead-days" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.alertBeforeDays') }}</label>
              <input
                id="reminder-form-lead-days"
                v-model.number="reminderForm.lead_days"
                type="number"
                min="0"
                placeholder="15"
                class="field"
              />
            </div>
          </div>
        </div>

        <!-- Webhook notification toggle -->
        <div class="flex items-center gap-2 pt-1">
          <input
            id="reminder-form-webhook-toggle"
            v-model="reminderForm.webhook_enabled"
            type="checkbox"
            class="rounded border-slate-700 bg-slate-800 text-violet-600 focus:ring-violet-500"
          />
          <label for="reminder-form-webhook-toggle" class="text-xs text-slate-300 cursor-pointer">
            {{ $t('expenses.reminderModal.sendAnAutomaticWebhookNotification') }}
          </label>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting" type="submit" form="reminder-modal-form" class="btn btn-lg btn-primary">
          {{ editingReminderId ? $t('expenses.update') : $t('expenses.reminderModal.create') }}
        </button>
      </div>
    </div>
  </div>
</template>
