import { computed, ref } from 'vue'
import { t } from '@/i18n'
import { api, type MaintenanceReminder, type VehicleWebhook } from '@/services/api'
import { useConfirm } from '@/composables/useConfirm'
import { useVehicleStore } from '@/stores/vehicle'
import { hasReminderSchedule, type ReminderPreset } from '@/utils/expenses'

export interface UseRemindersOptions {
  // Called with the error message when loading fails and the reminders tab is the one on screen
  onLoadError: (message: string) => void
  // Called after a reminder was completed together with a logged maintenance expense
  onExpenseLogged: () => Promise<void> | void
  isRemindersTabActive: () => boolean
}

// Maintenance reminders of the active vehicle, the modals that edit them, and the vehicle webhook.
export function useReminders(options: UseRemindersOptions) {
  const vehicleStore = useVehicleStore()
  const { showConfirm, showAlert } = useConfirm()

  const reminders = ref<MaintenanceReminder[]>([])
  const loadingReminders = ref(false)
  const showReminderModal = ref(false)
  const editingReminder = ref<MaintenanceReminder | null>(null)
  const reminderPreset = ref<ReminderPreset | null>(null)
  const showCompleteReminderModal = ref(false)
  const completingReminder = ref<MaintenanceReminder | null>(null)
  const showWebhookModal = ref(false)
  const vehicleWebhook = ref<VehicleWebhook | null>(null)

  const overdueReminders = computed(() => reminders.value.filter((r) => r.status === 'OVERDUE'))
  const dueSoonReminders = computed(() => reminders.value.filter((r) => r.status === 'DUE_SOON'))
  const okReminders = computed(() => reminders.value.filter((r) => r.status === 'OK' && hasReminderSchedule(r)))
  const unscheduledReminders = computed(() => reminders.value.filter((r) => !hasReminderSchedule(r)))
  const urgentRemindersCount = computed(() => overdueReminders.value.length + dueSoonReminders.value.length)

  async function loadReminders() {
    if (!vehicleStore.activeVehicle) return
    loadingReminders.value = true
    try {
      reminders.value = await api.getReminders(vehicleStore.activeVehicle.id)
    } catch (err: any) {
      console.error('Failed to load reminders', err)
      if (options.isRemindersTabActive()) options.onLoadError(err?.message ?? '')
    } finally {
      loadingReminders.value = false
    }
  }

  function openAddReminderModal(preset: ReminderPreset | null = null) {
    editingReminder.value = null
    reminderPreset.value = preset
    showReminderModal.value = true
  }

  function openEditReminderModal(r: MaintenanceReminder) {
    editingReminder.value = r
    reminderPreset.value = null
    showReminderModal.value = true
  }

  async function handleDeleteReminder(r: MaintenanceReminder) {
    if (!vehicleStore.activeVehicle) return
    const ok = await showConfirm({
      title: t('expenses.expensesView.deleteReminderTitle'),
      message: t('expenses.expensesView.deleteReminderMessage', { title: r.title }),
      confirmText: t('common.delete'),
      type: 'danger',
    })
    if (!ok) return
    try {
      await api.deleteReminder(vehicleStore.activeVehicle.id, r.id)
      void showAlert(t('expenses.expensesView.reminderDeleted'), t('common.success'), 'success')
      await loadReminders()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  function openCompleteReminder(r: MaintenanceReminder) {
    completingReminder.value = r
    showCompleteReminderModal.value = true
  }

  async function onReminderCompleted(expenseLogged: boolean) {
    await loadReminders()
    if (expenseLogged && vehicleStore.activeVehicle) await options.onExpenseLogged()
  }

  // The webhook is fetched before the modal opens, so it shows the saved configuration
  async function openWebhookModal() {
    if (!vehicleStore.activeVehicle) return
    try {
      vehicleWebhook.value = await api.getVehicleWebhook(vehicleStore.activeVehicle.id)
    } catch (err: any) {
      console.error('Failed to load webhook', err)
    }
    showWebhookModal.value = true
  }

  return {
    reminders,
    loadingReminders,
    showReminderModal,
    editingReminder,
    reminderPreset,
    showCompleteReminderModal,
    completingReminder,
    showWebhookModal,
    vehicleWebhook,
    overdueReminders,
    dueSoonReminders,
    okReminders,
    unscheduledReminders,
    urgentRemindersCount,
    loadReminders,
    openAddReminderModal,
    openEditReminderModal,
    handleDeleteReminder,
    openCompleteReminder,
    onReminderCompleted,
    openWebhookModal,
  }
}
