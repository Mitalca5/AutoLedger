<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api, type MaintenanceReminder } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { X, Bell, Sparkles } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { reminderPresets, type ReminderPreset } from '@/utils/expenses'
import { todayIso } from '@/utils/dates'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'

// Creates a maintenance reminder, or edits `editing`. `preset` pre-fills a new one from a suggestion.
const props = defineProps<{ vehicleId: string; editing: MaintenanceReminder | null; preset: ReminderPreset | null; currentOdometer: number }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()

const editingReminderId = computed(() => props.editing?.id ?? null)

const reminderForm = ref({
  title: '',
  category: 'MAINTENANCE',
  interval_km: '' as number | '',
  interval_months: '' as number | '',
  last_service_odometer: '' as number | '',
  last_service_date: todayIso(),
  lead_km: 1000,
  lead_days: 15,
  webhook_enabled: true,
})

function applyReminderPreset(preset: ReminderPreset) {
  reminderForm.value.title = preset.title
  reminderForm.value.category = preset.category
  reminderForm.value.interval_km = preset.interval_km
  reminderForm.value.interval_months = preset.interval_months
  reminderForm.value.lead_km = preset.lead_km
  reminderForm.value.lead_days = preset.lead_days
}

watch(open, (isOpen) => {
  if (!isOpen) return
  const r = props.editing
  if (!r) {
    const currentOdo = props.currentOdometer ? Math.round(props.currentOdometer) : ''
    reminderForm.value = {
      title: '',
      category: 'MAINTENANCE',
      interval_km: 10000,
      interval_months: 12,
      last_service_odometer: currentOdo,
      last_service_date: todayIso(),
      lead_km: 1000,
      lead_days: 15,
      webhook_enabled: true,
    }
    if (props.preset) applyReminderPreset(props.preset)
  } else {
    reminderForm.value = {
      title: r.title,
      category: r.category,
      interval_km: r.interval_km ?? '',
      interval_months: r.interval_months ?? '',
      last_service_odometer: r.last_service_odometer !== null && r.last_service_odometer !== undefined ? Math.round(r.last_service_odometer) : '',
      last_service_date: r.last_service_date ? r.last_service_date.substring(0, 10) : todayIso(),
      lead_km: r.lead_km,
      lead_days: r.lead_days,
      webhook_enabled: r.webhook_enabled,
    }
  }
})

async function handleSaveReminder() {
  if (!props.vehicleId) return
  if (!reminderForm.value.title.trim()) {
    showAlert(t('expenses.reminderModal.titleRequired'), t('common.requiredField'), 'warning')
    return
  }
  if (!reminderForm.value.interval_km && !reminderForm.value.interval_months) {
    showAlert(t('expenses.reminderModal.intervalRequired', { unit: distanceUnit() }), t('common.requiredField'), 'warning')
    return
  }
  try {
    const payload = {
      title: reminderForm.value.title.trim(),
      category: reminderForm.value.category,
      interval_km: reminderForm.value.interval_km !== '' ? Number(reminderForm.value.interval_km) : null,
      interval_months: reminderForm.value.interval_months !== '' ? Number(reminderForm.value.interval_months) : null,
      last_service_odometer: reminderForm.value.last_service_odometer !== '' ? Number(reminderForm.value.last_service_odometer) : null,
      last_service_date: reminderForm.value.last_service_date ? new Date(reminderForm.value.last_service_date).toISOString() : null,
      lead_km: Number(reminderForm.value.lead_km || 0),
      lead_days: Number(reminderForm.value.lead_days || 0),
      webhook_enabled: reminderForm.value.webhook_enabled,
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
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="reminder-modal-form" @submit.prevent="handleSaveReminder" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <!-- Preset chips (only when adding new) -->
        <div v-if="!editingReminderId" class="space-y-1.5">
          <span class="block text-xs font-semibold text-slate-300">{{ $t('expenses.reminderModal.quickTemplates') }}</span>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="preset in reminderPresets()"
              :key="preset.title"
              type="button"
              @click="applyReminderPreset(preset)"
              class="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium rounded-lg border border-slate-700 transition-colors flex items-center gap-1"
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
              :placeholder="$t('expenses.reminderModal.eGTireRotation')"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
            />
          </div>
          <div>
            <label for="reminder-form-category" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.category') }}</label>
            <select
              id="reminder-form-category"
              v-model="reminderForm.category"
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
            >
              <option value="MAINTENANCE">{{ $t('expenses.reminderModal.maintenance') }}</option>
              <option value="TIRES">{{ $t('expenses.reminderModal.tires') }}</option>
            </select>
          </div>
        </div>

        <!-- Periodicities -->
        <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
          <span class="block text-xs font-semibold text-slate-200">{{ $t('expenses.reminderModal.frequencyAtLeastOneOf') }}</span>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="reminder-form-interval-km" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.intervalInKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput whole text
                id="reminder-form-interval-km"
                v-model="reminderForm.interval_km"
                min="500"
                step="500"
                :placeholder="$t('expenses.reminderModal.eG10000EmptyIgnored')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
            <div>
              <label for="reminder-form-interval-months" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.intervalInMonths') }}</label>
              <input
                id="reminder-form-interval-months"
                v-model="reminderForm.interval_months"
                type="number"
                min="1"
                max="120"
                :placeholder="$t('expenses.reminderModal.eG12EmptyIgnored')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
          </div>
        </div>

        <!-- Last service date & odometer -->
        <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
          <span class="block text-xs font-semibold text-slate-200">{{ $t('expenses.reminderModal.startingPointLastMaintenance') }}</span>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label for="reminder-form-last-odo" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.reminderModal.odometerAtLastMaintenanceKm', { unit: distanceUnit() }) }}</label>
              <DistanceInput text
                id="reminder-form-last-odo"
                v-model="reminderForm.last_service_odometer"
                min="0"
                :placeholder="$t('common.example', { value: '45000' })"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
            <div>
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
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
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
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
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
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="reminder-modal-form" class="px-4 py-2 bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ editingReminderId ? $t('expenses.update') : $t('expenses.reminderModal.create') }}
        </button>
      </div>
    </div>
  </div>
</template>
