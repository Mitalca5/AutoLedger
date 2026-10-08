import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { api } from '@/services/api'
import { useReminders } from './useReminders'

vi.mock('@/services/api', () => ({
  api: { getReminders: vi.fn(), deleteReminder: vi.fn(), getVehicleWebhook: vi.fn() },
}))
const store = reactive<{ activeVehicle: { id: string } | null }>({ activeVehicle: { id: 'v1' } })
vi.mock('@/stores/vehicle', () => ({ useVehicleStore: () => store }))
const showConfirm = vi.fn()
const showAlert = vi.fn()
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ showConfirm, showAlert }) }))

const reminder = (id: string, status: string, extra: Record<string, unknown> = {}) => ({ id, title: `r${id}`, status, interval_km: 10000, ...extra })

function setup(tabActive = true) {
  const options = { onLoadError: vi.fn(), onExpenseLogged: vi.fn(), isRemindersTabActive: () => tabActive }
  return { options, r: useReminders(options) }
}

describe('useReminders', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    store.activeVehicle = { id: 'v1' }
  })

  it('groups the reminders by urgency and schedule', async () => {
    vi.mocked(api.getReminders).mockResolvedValue([
      reminder('1', 'OVERDUE'),
      reminder('2', 'DUE_SOON'),
      reminder('3', 'OK'),
      reminder('4', 'OK', { interval_km: 0 }),
      reminder('5', 'DUE_SOON', { interval_km: 0 }),
    ] as any)
    const { r } = setup()
    await r.loadReminders()
    expect(r.overdueReminders.value.map((x) => x.id)).toEqual(['1'])
    expect(r.dueSoonReminders.value.map((x) => x.id)).toEqual(['2', '5'])
    expect(r.okReminders.value.map((x) => x.id)).toEqual(['3'])
    expect(r.unscheduledReminders.value.map((x) => x.id)).toEqual(['4', '5'])
    expect(r.urgentRemindersCount.value).toBe(3)
    expect(r.loadingReminders.value).toBe(false)
  })

  it('does nothing without an active vehicle', async () => {
    store.activeVehicle = null
    const { r } = setup()
    await r.loadReminders()
    await r.handleDeleteReminder(reminder('1', 'OK') as any)
    await r.openWebhookModal()
    expect(api.getReminders).not.toHaveBeenCalled()
    expect(showConfirm).not.toHaveBeenCalled()
    expect(r.showWebhookModal.value).toBe(false)
  })

  it('reports a load failure only while the reminders tab is on screen', async () => {
    vi.mocked(api.getReminders).mockRejectedValue(new Error('boom'))
    const visible = setup(true)
    await visible.r.loadReminders()
    expect(visible.options.onLoadError).toHaveBeenCalledWith('boom')
    expect(visible.r.loadingReminders.value).toBe(false)

    const hidden = setup(false)
    await hidden.r.loadReminders()
    expect(hidden.options.onLoadError).not.toHaveBeenCalled()

    vi.mocked(api.getReminders).mockRejectedValue({})
    await visible.r.loadReminders()
    expect(visible.options.onLoadError).toHaveBeenLastCalledWith('')
  })

  it('opens the editor empty, with a preset, or on an existing reminder', () => {
    const { r } = setup()
    const existing = reminder('1', 'OK') as any
    r.openEditReminderModal(existing)
    expect(r.showReminderModal.value).toBe(true)
    expect(r.editingReminder.value).toEqual(existing)
    expect(r.reminderPreset.value).toBeNull()

    const preset = { title: 'Tires' } as any
    r.openAddReminderModal(preset)
    expect(r.editingReminder.value).toBeNull()
    expect(r.reminderPreset.value).toEqual(preset)

    r.openAddReminderModal()
    expect(r.reminderPreset.value).toBeNull()
  })

  it('deletes a reminder after confirmation and reloads the list', async () => {
    showConfirm.mockResolvedValue(true)
    vi.mocked(api.deleteReminder).mockResolvedValue(undefined as any)
    vi.mocked(api.getReminders).mockResolvedValue([] as any)
    const { r } = setup()
    await r.handleDeleteReminder(reminder('7', 'OK') as any)
    expect(api.deleteReminder).toHaveBeenCalledWith('v1', '7')
    expect(showAlert).toHaveBeenCalledWith(expect.any(String), expect.any(String), 'success')
    expect(api.getReminders).toHaveBeenCalledTimes(1)
  })

  it('keeps the reminder when the confirmation is refused', async () => {
    showConfirm.mockResolvedValue(false)
    const { r } = setup()
    await r.handleDeleteReminder(reminder('7', 'OK') as any)
    expect(api.deleteReminder).not.toHaveBeenCalled()
  })

  it('shows an error dialog when the deletion fails', async () => {
    showConfirm.mockResolvedValue(true)
    vi.mocked(api.deleteReminder).mockRejectedValue(new Error('nope'))
    const { r } = setup()
    await r.handleDeleteReminder(reminder('7', 'OK') as any)
    expect(showAlert).toHaveBeenCalledWith(expect.any(String), expect.any(String), 'danger')
    expect(api.getReminders).not.toHaveBeenCalled()
  })

  it('reloads after a completion and refreshes the expenses only when one was logged', async () => {
    vi.mocked(api.getReminders).mockResolvedValue([] as any)
    const { r, options } = setup()
    const target = reminder('3', 'DUE_SOON') as any
    r.openCompleteReminder(target)
    expect(r.completingReminder.value).toEqual(target)
    expect(r.showCompleteReminderModal.value).toBe(true)

    await r.onReminderCompleted(false)
    expect(options.onExpenseLogged).not.toHaveBeenCalled()
    await r.onReminderCompleted(true)
    expect(options.onExpenseLogged).toHaveBeenCalledTimes(1)
    expect(api.getReminders).toHaveBeenCalledTimes(2)
  })

  it('opens the webhook modal with the saved configuration, or without it when loading fails', async () => {
    vi.mocked(api.getVehicleWebhook).mockResolvedValue({ url: 'https://x' } as any)
    const { r } = setup()
    await r.openWebhookModal()
    expect(r.vehicleWebhook.value).toEqual({ url: 'https://x' })
    expect(r.showWebhookModal.value).toBe(true)

    r.showWebhookModal.value = false
    vi.mocked(api.getVehicleWebhook).mockRejectedValue(new Error('offline'))
    await r.openWebhookModal()
    expect(r.showWebhookModal.value).toBe(true)
  })
})
