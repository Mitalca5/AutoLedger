import { ref, watch, type Ref } from 'vue'
import { t } from '@/i18n'
import { api } from '@/services/api'
import { useVehicleStore } from '@/stores/vehicle'
import { useConfirm } from '@/composables/useConfirm'
import {
  copiedSessionFromSession,
  emptySessionForm,
  formatDate,
  sessionFormFromCopy,
  sessionFormFromSession,
  type SessionForm,
  type TireLogForm,
} from '@/utils/tires'
import { todayIso, toIsoDay } from '@/utils/dates'

// The tire whose history modal is open, with its mount sessions and tread depth logs, and the forms
// that edit them. The modals own their API calls; this owns what they are opened with.
export function useTireHistory(options: {
  tires: Ref<any[]>
  loadTires: () => Promise<void>
  selectedTireIds: Ref<string[]>
}) {
  const { tires, loadTires, selectedTireIds } = options
  const vehicleStore = useVehicleStore()
  const { showConfirm, showAlert } = useConfirm()

  const showHistoryModal = ref(false)
  const showSessionModal = ref(false)
  const showDuplicateSessionModal = ref(false)
  const showLogModal = ref(false)

  // Tire shown in the history modal, with its stats, mount sessions and tread depth logs
  const selectedTire = ref<any | null>(null)
  const selectedTireStats = ref<any | null>(null)
  const tireSessions = ref<any[]>([])
  const tireLogs = ref<any[]>([])

  // Mount session form (add / edit / paste) and the session kept by the "copy" button
  const editingSessionId = ref<string | null>(null)
  const sessionInitialForm = ref<SessionForm>(emptySessionForm())
  const copiedSession = ref<SessionForm | null>(null)
  const sessionToDuplicate = ref<any | null>(null)

  // Tread depth log form
  const editingLogId = ref<string | null>(null)
  const logInitialForm = ref<TireLogForm>({ depth_mm: 6.5, odometer: 0, notes: '', date: todayIso() })

  let historyRequest = 0
  watch(showHistoryModal, (open) => { if (!open) historyRequest++ }, { flush: 'sync' })
  watch(() => vehicleStore.activeVehicle?.id, () => {
    historyRequest++
    showHistoryModal.value = false
  }, { flush: 'sync' })

  async function openHistoryModal(tire: any) {
    const vehicleId = vehicleStore.activeVehicle?.id
    if (!vehicleId) return
    const request = ++historyRequest
    const isCurrent = () => request === historyRequest && vehicleStore.activeVehicle?.id === vehicleId

    if (selectedTire.value?.id !== tire.tire.id) {
      tireSessions.value = []
      tireLogs.value = []
    }
    selectedTire.value = tire.tire
    selectedTireStats.value = tire
    try {
      const res = await api.getTireHistory(vehicleId, tire.tire.id)
      if (!isCurrent()) return
      selectedTire.value = res.tire
      selectedTireStats.value = res.stats
      tireSessions.value = res.sessions || []
      tireLogs.value = res.logs || []
      showHistoryModal.value = true
    } catch (err: any) {
      if (!isCurrent()) return
      void showAlert(t('tires.tiresView.loadError', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  function openTimelineTire(tireId: string) {
    const found = tires.value.find((x) => x.tire.id === tireId)
    if (found) void openHistoryModal(found)
  }

  /** Reload the list, then the open history so it shows the change. */
  async function refreshAfterHistoryChange() {
    await loadTires()
    if (showHistoryModal.value && selectedTire.value) await openHistoryModal({ tire: selectedTire.value })
  }

  /** Reload the open history and the list after a session or reading was added, changed or removed. */
  async function reloadHistoryAndList() {
    await openHistoryModal({ tire: selectedTire.value })
    await loadTires()
  }

  // Dispose (worn out, damaged, sold) keeps history and cost; delete removes an erroneous entry
  async function onTireDisposed(tireId: string) {
    showHistoryModal.value = false
    selectedTireIds.value = selectedTireIds.value.filter((id) => id !== tireId)
    await loadTires()
  }

  async function handleDeleteTire(tire: any) {
    if (!vehicleStore.activeVehicle) return
    const ok = await showConfirm({
      title: t('tires.tiresView.deleteTireTitle'),
      message: t('tires.tiresView.deleteTireMessage', { brand: tire.brand, model: tire.model, dimension: tire.dimension }),
      confirmText: t('tires.tiresView.deleteTireConfirm'),
      type: 'danger',
    })
    if (!ok) return

    try {
      await api.deleteTire(vehicleStore.activeVehicle.id, tire.id)
      showHistoryModal.value = false
      selectedTireIds.value = selectedTireIds.value.filter((id) => id !== tire.id)
      await loadTires()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  // Mount sessions
  function openAddSessionModal() {
    editingSessionId.value = null
    sessionInitialForm.value = emptySessionForm()
    showSessionModal.value = true
  }

  function openEditSessionModal(s: any) {
    editingSessionId.value = s.id
    sessionInitialForm.value = sessionFormFromSession(s)
    showSessionModal.value = true
  }

  async function handleDeleteSession(session: any) {
    if (!vehicleStore.activeVehicle || !selectedTire.value) return
    const ok = await showConfirm({
      title: t('tires.tiresView.deleteSessionTitle'),
      message: t('tires.tiresView.deleteSessionMessage'),
      confirmText: t('common.delete'),
      type: 'danger',
    })
    if (!ok) return

    try {
      await api.deleteTireSession(vehicleStore.activeVehicle.id, selectedTire.value.id, session.id)
      await reloadHistoryAndList()
    } catch (err: any) {
      void showAlert(t('common.deleteError', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  // Copy a session, paste it on another tire, or duplicate it onto several
  function copySession(s: any) {
    copiedSession.value = copiedSessionFromSession(s)
  }

  function pasteSessionToCurrentTire() {
    if (!copiedSession.value) return
    editingSessionId.value = null
    sessionInitialForm.value = sessionFormFromCopy(copiedSession.value, selectedTire.value)
    showSessionModal.value = true
  }

  function openDuplicateSessionModal(s: any) {
    sessionToDuplicate.value = s
    showDuplicateSessionModal.value = true
  }

  // Tread depth measurements
  function openLogModal(tire: any, log?: any) {
    selectedTire.value = tire.tire
    editingLogId.value = log ? log.id : null
    logInitialForm.value = log
      ? { depth_mm: log.depth_mm, odometer: Math.round(log.odometer), notes: log.notes || '', date: toIsoDay(log.date) }
      : {
          depth_mm: tire.current_depth_mm || 6.5,
          odometer: Math.round(vehicleStore.activeVehicle?.current_odometer || 0),
          notes: '',
          date: todayIso(),
        }
    showLogModal.value = true
  }

  function editLog(log: any) {
    openLogModal(selectedTireStats.value || { tire: selectedTire.value }, log)
  }

  async function handleDeleteLog(l: any) {
    if (!vehicleStore.activeVehicle || !selectedTire.value) return
    const ok = await showConfirm({
      title: t('tires.tiresView.deleteReadingTitle'),
      message: t('tires.tiresView.deleteReadingMessage', { depth: l.depth_mm, date: formatDate(l.date) }),
      confirmText: t('common.delete'),
      type: 'danger',
    })
    if (!ok) return

    try {
      await api.deleteTireLog(vehicleStore.activeVehicle.id, selectedTire.value.id, l.id)
      await reloadHistoryAndList()
    } catch (err: any) {
      void showAlert(t('common.errorWithMessage', { message: err.message }), t('shell.confirm.error'), 'danger')
    }
  }

  return {
    showHistoryModal,
    showSessionModal,
    showDuplicateSessionModal,
    showLogModal,
    selectedTire,
    selectedTireStats,
    tireSessions,
    tireLogs,
    editingSessionId,
    sessionInitialForm,
    copiedSession,
    sessionToDuplicate,
    editingLogId,
    logInitialForm,
    openHistoryModal,
    openTimelineTire,
    refreshAfterHistoryChange,
    reloadHistoryAndList,
    onTireDisposed,
    handleDeleteTire,
    openAddSessionModal,
    openEditSessionModal,
    handleDeleteSession,
    copySession,
    pasteSessionToCurrentTire,
    openDuplicateSessionModal,
    openLogModal,
    editLog,
    handleDeleteLog,
  }
}
