<script setup lang="ts">
import { t } from '@/i18n'
import DistanceInput from '@/components/DistanceInput.vue'
import { computed, ref, watch } from 'vue'
import { api, type MaintenanceReminder } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { X, CheckCircle2 } from 'lucide-vue-next'
import AppDatePicker from '@/components/AppDatePicker.vue'
import { todayIso } from '@/utils/dates'
import { currencySymbol } from '@/currency'
import { useEscapeToClose } from '@/composables/useEscapeToClose'
import { distanceUnit } from '@/units'

// Marks a reminder as done, optionally logging the maintenance expense. saved carries whether an expense was logged.
const props = defineProps<{ vehicleId: string; reminder: MaintenanceReminder | null; currentOdometer: number }>()
const emit = defineEmits<{ saved: [expenseLogged: boolean] }>()
const open = defineModel<boolean>('open', { required: true })
useEscapeToClose(open, () => (open.value = false))
const { showAlert } = useConfirm()
const vehicleStore = useVehicleStore()

const completingReminder = computed(() => props.reminder)

const completeForm = ref({
  service_date: todayIso(),
  service_odometer: '' as number | '',
  log_expense: false,
  expense_amount: '',
  expense_description: '',
})

watch(open, (isOpen) => {
  if (!isOpen || !props.reminder) return
  const currentOdo = props.currentOdometer ? Math.round(props.currentOdometer) : ''
  completeForm.value = {
    service_date: todayIso(),
    service_odometer: currentOdo,
    log_expense: false,
    expense_amount: '',
    expense_description: t('expenses.completeReminderModal.expenseDescription', { title: props.reminder.title }),
  }
})

async function handleCompleteReminder() {
  const reminder = props.reminder
  if (!props.vehicleId || !reminder) return
  try {
    const payload = {
      completed_date: completeForm.value.service_date,
      completed_odometer: completeForm.value.service_odometer !== '' ? Number(completeForm.value.service_odometer) : undefined,
    }
    await api.completeReminder(props.vehicleId, reminder.id, payload)

    if (completeForm.value.log_expense && Number(completeForm.value.expense_amount) > 0) {
      await api.createMaintenance(props.vehicleId, {
        category: reminder.category === 'TIRES' ? 'TIRES' : 'MAINTENANCE',
        amount: Number(completeForm.value.expense_amount),
        currency: vehicleStore.currency,
        fx_rate: null,
        date: new Date(completeForm.value.service_date).toISOString(),
        description: completeForm.value.expense_description || reminder.title,
        odometer: completeForm.value.service_odometer ? Number(completeForm.value.service_odometer) : null,
        is_recurring: false,
      })
    }

    showAlert(t('expenses.completeReminderModal.done'), t('common.success'), 'success')
    open.value = false
    emit('saved', completeForm.value.log_expense)
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
          <CheckCircle2 class="w-5 h-5 text-emerald-400" />
          {{ $t('expenses.completeReminderModal.confirmCompletion', { title: completingReminder?.title }) }}
        </h3>
        <button @click="open = false" class="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition-colors">
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
              class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
            />
          </div>
        </div>

        <!-- Option to log an expense -->
        <div class="p-3.5 bg-slate-800/40 rounded-xl border border-slate-800 space-y-3">
          <div class="flex items-center gap-2">
            <input
              id="complete-form-log-expense"
              v-model="completeForm.log_expense"
              type="checkbox"
              class="rounded border-slate-700 bg-slate-800 text-emerald-600 focus:ring-emerald-500"
            />
            <label for="complete-form-log-expense" class="text-xs font-semibold text-slate-200 cursor-pointer">
              {{ $t('expenses.completeReminderModal.alsoRecordAMaintenanceExpense') }}
            </label>
          </div>

          <div v-if="completeForm.log_expense" class="space-y-3 pt-1">
            <div>
              <label for="complete-form-expense-amount" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.completeReminderModal.invoiceCost', { cur: currencySymbol(vehicleStore.currency) }) }}</label>
              <input
                id="complete-form-expense-amount"
                v-model="completeForm.expense_amount"
                type="number"
                step="0.01"
                min="0"
                :placeholder="$t('expenses.completeReminderModal.000IfFree')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
            <div>
              <label for="complete-form-expense-desc" class="block text-xs font-semibold text-slate-300 mb-1">{{ $t('expenses.completeReminderModal.expenseLabel') }}</label>
              <input
                id="complete-form-expense-desc"
                v-model="completeForm.expense_description"
                :placeholder="$t('expenses.completeReminderModal.eGWorkshopServiceTire')"
                class="w-full bg-slate-800 border border-slate-700 rounded-xl px-3 py-2 text-sm text-white"
              />
            </div>
          </div>
        </div>
      </form>

      <div class="px-5 py-3.5 border-t border-slate-800/80 flex justify-end gap-2 shrink-0 bg-slate-900/95">
        <button type="button" @click="open = false" class="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold rounded-xl transition-colors">
          {{ $t('common.cancel') }}
        </button>
        <button type="submit" form="complete-reminder-form" class="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-xl transition-colors">
          {{ $t('expenses.completeReminderModal.confirmTheMaintenance') }}
        </button>
      </div>
    </div>
  </div>
</template>
