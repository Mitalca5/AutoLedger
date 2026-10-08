<script setup lang="ts">
import NumberInput from '@/components/NumberInput.vue'
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api, type MaintenanceReminder } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useMaintenanceChoices } from '@/composables/useMaintenanceChoices'
import { useVehicleStore } from '@/stores/vehicle'
import { X, CheckCircle2 } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { todayIso } from '@/utils/dates'
import { maintenanceStartPoint } from '@/utils/expenses'
import { currencySymbol } from '@/currency'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'
import { useSubmit } from '@/composables/useSubmit'

// Marks a reminder as done, optionally logging a new maintenance expense or linking an existing one. saved carries whether an expense was logged.
const props = defineProps<{ vehicleId: string; reminder: MaintenanceReminder | null; currentOdometer: number }>()
const emit = defineEmits<{ saved: [expenseLogged: boolean] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const vehicleStore = useVehicleStore()

const completingReminder = computed(() => props.reminder)

type ExpenseMode = 'none' | 'create' | 'existing'

const { options: maintenanceOptions, load: loadMaintenanceChoices, label: maintenanceOptionLabel, find: findMaintenance } = useMaintenanceChoices(() => props.vehicleId)

// Linking an existing maintenance takes its day and odometer as the date of the work.
function onMaintenanceChosen() {
  const m = findMaintenance(completeForm.value.maintenance_id)
  if (!m) return
  const start = maintenanceStartPoint(m)
  completeForm.value.service_date = start.date
  if (start.odometer !== '') completeForm.value.service_odometer = start.odometer
}

const completeForm = ref({
  service_date: todayIso(),
  service_odometer: '' as number | '',
  expense_mode: 'none' as ExpenseMode,
  maintenance_id: '',
  expense_amount: '',
  expense_description: '',
})

watch(open, (isOpen) => {
  if (!isOpen || !props.reminder) return
  void loadMaintenanceChoices()
  const currentOdo = props.currentOdometer ? Math.round(props.currentOdometer) : ''
  completeForm.value = {
    service_date: todayIso(),
    service_odometer: currentOdo,
    expense_mode: 'none' as ExpenseMode,
    maintenance_id: '',
    expense_amount: '',
    expense_description: t('expenses.completeReminderModal.expenseDescription', { title: props.reminder.title }),
  }
})

const { pending: submitting, run: runOnce } = useSubmit()

async function handleCompleteReminderAction() {
  const reminder = props.reminder
  if (!props.vehicleId || !reminder) return
  try {
    // The expense is recorded first so the reminder can point at it.
    let maintenanceId: string | undefined
    if (completeForm.value.expense_mode === 'create' && Number(completeForm.value.expense_amount) > 0) {
      const created = await api.createMaintenance(props.vehicleId, {
        category: reminder.category === 'TIRES' ? 'TIRES' : 'MAINTENANCE',
        amount: Number(completeForm.value.expense_amount),
        currency: vehicleStore.currency,
        fx_rate: null,
        date: new Date(completeForm.value.service_date).toISOString(),
        description: completeForm.value.expense_description || reminder.title,
        odometer: completeForm.value.service_odometer ? Number(completeForm.value.service_odometer) : null,
        is_recurring: false,
      })
      maintenanceId = created?.id
    } else if (completeForm.value.expense_mode === 'existing' && completeForm.value.maintenance_id) {
      maintenanceId = completeForm.value.maintenance_id
    }
    await api.completeReminder(props.vehicleId, reminder.id, {
      completed_date: completeForm.value.service_date,
      completed_odometer: completeForm.value.service_odometer !== '' ? Number(completeForm.value.service_odometer) : undefined,
      maintenance_id: maintenanceId,
    })

    showAlert(t('expenses.completeReminderModal.done'), t('common.success'), 'success')
    open.value = false
    emit('saved', completeForm.value.expense_mode === 'create')
  } catch (err: any) {
    showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
  }
}
const handleCompleteReminder = () => runOnce(handleCompleteReminderAction)
</script>

<template>
  <div
    v-if="open"
    class="fixed inset-0 z-modal bg-black/75 backdrop-blur-sm flex items-center justify-center p-3 sm:p-4 overflow-y-auto"
    @click.self="open = false"
  >
    <div v-dialog class="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full max-h-[calc(100dvh-2rem)] flex flex-col shadow-2xl overflow-hidden my-auto">
      <div class="px-5 py-4 border-b border-slate-800/80 flex items-center justify-between shrink-0 bg-slate-900/95">
        <h3 class="text-base font-bold text-white flex items-center gap-2">
          <CheckCircle2 class="w-5 h-5 text-success-400" />
          {{ $t('expenses.completeReminderModal.confirmCompletion', { title: completingReminder?.title }) }}
        </h3>
        <button @click="open = false" class="tap text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors" :aria-label="$t('common.close')">
          <X class="w-5 h-5" />
        </button>
      </div>

      <form id="complete-reminder-form" @submit.prevent="handleCompleteReminder" class="p-5 overflow-y-auto flex-1 overscroll-contain space-y-4">
        <p class="text-xs text-slate-400">
          {{ $t('expenses.completeReminderModal.confirmingThisWorkResetsThe') }}
        </p>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="complete-form-date" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.completeReminderModal.workDate') }}</label>
            <AppDatePicker
              id="complete-form-date"
              v-model="completeForm.service_date"
              required
              size="sm"
            />
          </div>
          <div>
            <label for="complete-form-odo" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.completeReminderModal.odometerAtTheWorkKm', { unit: distanceUnit() }) }}</label>
            <DistanceInput text
              id="complete-form-odo"
              v-model="completeForm.service_odometer"
              min="0"
              required
              class="field"
            />
          </div>
        </div>

        <!-- Option to log an expense or link an existing maintenance -->
        <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
          <div>
            <label for="complete-form-expense-mode" class="block text-xs font-semibold text-slate-200 mb-1">{{ $t('expenses.completeReminderModal.expenseMode') }}</label>
            <select id="complete-form-expense-mode" v-model="completeForm.expense_mode" class="field">
              <option value="none">{{ $t('expenses.completeReminderModal.modeNone') }}</option>
              <option value="create">{{ $t('expenses.completeReminderModal.modeCreate') }}</option>
              <option v-if="maintenanceOptions.length" value="existing">{{ $t('expenses.completeReminderModal.modeExisting') }}</option>
            </select>
          </div>

          <div v-if="completeForm.expense_mode === 'existing'" class="space-y-1">
            <label for="complete-form-maintenance" class="block text-xs font-semibold text-slate-300">{{ $t('expenses.completeReminderModal.existingMaintenance') }}</label>
            <select id="complete-form-maintenance" v-model="completeForm.maintenance_id" class="field" required @change="onMaintenanceChosen">
              <option value="" disabled>{{ $t('expenses.completeReminderModal.chooseMaintenance') }}</option>
              <option v-for="m in maintenanceOptions" :key="m.id" :value="m.id">{{ maintenanceOptionLabel(m) }}</option>
            </select>
            <p class="text-xs text-slate-400">{{ $t('expenses.completeReminderModal.existingHint') }}</p>
          </div>

          <div v-if="completeForm.expense_mode === 'create'" class="space-y-3 pt-1">
            <div>
              <label for="complete-form-expense-amount" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.completeReminderModal.invoiceCost', { cur: currencySymbol(vehicleStore.currency) }) }}</label>
              <NumberInput text
                id="complete-form-expense-amount"
                v-model="completeForm.expense_amount"
                min="0"
                :placeholder="$t('expenses.completeReminderModal.000IfFree')"
                class="field"
              />
            </div>
            <div>
              <label for="complete-form-expense-desc" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.completeReminderModal.expenseLabel') }}</label>
              <input
                id="complete-form-expense-desc"
                v-model="completeForm.expense_description"
                :placeholder="$t('expenses.completeReminderModal.eGWorkshopServiceTire')"
                class="field"
              />
            </div>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="btn btn-lg btn-secondary">
          {{ $t('common.cancel') }}
        </button>
        <button :disabled="submitting" type="submit" form="complete-reminder-form" class="px-4 py-2 bg-success-600 hover:bg-success-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ $t('expenses.completeReminderModal.confirmTheMaintenance') }}
        </button>
      </div>
    </div>
  </div>
</template>
