import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/services/api'
import { t } from '@/i18n'
import { apiErrorMessage } from '@/services/apiError'
import { hasTeslaMate as vehicleHasTeslaMate } from '@/utils/vehicles'
import { checkSyncProgress, POLL_INTERVAL_MS, STALE_AFTER_HIDDEN_MS, type SyncSeen } from '@/utils/syncWatch'

export const useVehicleStore = defineStore('vehicle', () => {
  const vehicles = ref<any[]>([])
  const activeVehicleId = ref<string | null>(localStorage.getItem('teslacost_active_vehicle'))
  const isInitialized = ref(false)
  const isLoading = ref(false)
  const isSyncing = ref(false)
  const syncResult = ref<any | null>(null)
  const syncError = ref<string | null>(null)
  const lastSyncTimestamp = ref<number>(Date.now())

  const activeVehicle = computed(() => {
    if (!vehicles.value.length) return null
    return vehicles.value.find((v) => v.id === activeVehicleId.value) || vehicles.value[0]
  })

  const userRole = computed<'OWNER' | 'EDITOR' | 'VIEWER'>(() => activeVehicle.value?.role || 'OWNER')
  const isOwner = computed(() => userRole.value === 'OWNER')
  const isEditor = computed(() => userRole.value === 'EDITOR')
  const isViewer = computed(() => userRole.value === 'VIEWER')
  const canEdit = computed(() => isOwner.value || isEditor.value)
  // Combustion vehicles are tracked manually (fuel fill-ups) and have no TeslaMate link
  const isIce = computed(() => activeVehicle.value?.powertrain === 'ICE')
  // TeslaMate-fed data (drives, battery, temperature, synchronization) only exists for a vehicle linked to a teslamateapi
  const hasTeslaMate = computed(() => vehicleHasTeslaMate(activeVehicle.value))
  // Every amount of a vehicle is stored and shown in the currency chosen at its creation
  const currency = computed<string>(() => activeVehicle.value?.currency || 'EUR')

  async function fetchVehicles() {
    isLoading.value = true
    try {
      const list = await api.getVehicles()
      vehicles.value = list
      if (list.length > 0 && (!activeVehicleId.value || !list.some((v) => v.id === activeVehicleId.value))) {
        setActiveVehicle(list[0].id)
      }
      lastSyncTimestamp.value = Date.now()
    } catch (err) {
      console.error('Failed to fetch vehicles', err)
    } finally {
      isLoading.value = false
      isInitialized.value = true
    }
  }

  function setActiveVehicle(id: string) {
    activeVehicleId.value = id
    localStorage.setItem('teslacost_active_vehicle', id)
  }

  const SYNC_POLL_INTERVAL_MS = 1500
  const SYNC_POLL_MAX_MS = 20 * 60 * 1000
  const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

  // Follows a background synchronization job until it finishes
  async function followSyncJob(vehicleId: string, job: any) {
    isSyncing.value = true
    syncResult.value = null
    syncError.value = null
    const startedAt = Date.now()
    try {
      while (job?.status === 'RUNNING') {
        if (Date.now() - startedAt > SYNC_POLL_MAX_MS) {
          throw new Error(t('shell.sync.takesLonger'))
        }
        await sleep(SYNC_POLL_INTERVAL_MS)
        job = await api.getSyncStatus(vehicleId)
      }
      if (job?.status === 'FAILED') {
        syncError.value = apiErrorMessage({ error: job.error, code: job.error_code, params: job.error_params }, t('shell.sync.unknownError'))
      } else if (job?.status === 'SUCCEEDED') {
        syncResult.value = job.result
        await fetchVehicles()
        lastSyncTimestamp.value = Date.now()
      }
    } catch (err: any) {
      syncError.value = err.message || t('shell.sync.unknownError')
    } finally {
      isSyncing.value = false
    }
  }

  async function syncActiveVehicle() {
    if (!activeVehicle.value || isSyncing.value) return
    const vehicleId = activeVehicle.value.id
    try {
      const job = await api.syncVehicle(vehicleId)
      await followSyncJob(vehicleId, job)
    } catch (err: any) {
      syncError.value = err.message || t('shell.sync.unknownError')
    }
  }

  // Resumes the progress indicator when a synchronization (manual or scheduled) is already running
  async function resumeRunningSync() {
    if (!activeVehicle.value || isSyncing.value) return
    try {
      const job = await api.getSyncStatus(activeVehicle.value.id)
      if (job?.status === 'RUNNING') {
        await followSyncJob(activeVehicle.value.id, job)
      }
    } catch {
      // No sync information: nothing to resume
    }
  }

  // ----- Automatic refresh -----
  // The server synchronizes on its own schedule; the pages reload on lastSyncTimestamp, so it is bumped when new data
  // arrived, without the user having to leave the page and come back.
  let seenSync: SyncSeen | null = null
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let hiddenSince: number | null = null

  async function checkForNewData() {
    const vehicle = activeVehicle.value
    if (!vehicle || !hasTeslaMate.value || isSyncing.value) return
    try {
      const job = await api.getSyncStatus(vehicle.id)
      if (job?.status === 'RUNNING') {
        await followSyncJob(vehicle.id, job)
        seenSync = null
        return
      }
      const { refresh, seen } = checkSyncProgress(seenSync, vehicle.id, job)
      seenSync = seen
      if (refresh) await fetchVehicles()
    } catch {
      // Offline or server busy: try again at the next tick
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === 'hidden') {
      hiddenSince = Date.now()
      return
    }
    const wasAwayLong = hiddenSince !== null && Date.now() - hiddenSince >= STALE_AFTER_HIDDEN_MS
    hiddenSince = null
    if (wasAwayLong) void fetchVehicles()
    else void checkForNewData()
  }

  function startAutoRefresh() {
    if (pollTimer) return
    pollTimer = setInterval(() => {
      if (document.visibilityState === 'visible') void checkForNewData()
    }, POLL_INTERVAL_MS)
    document.addEventListener('visibilitychange', onVisibilityChange)
  }

  function stopAutoRefresh() {
    if (pollTimer) clearInterval(pollTimer)
    pollTimer = null
    seenSync = null
    hiddenSince = null
    document.removeEventListener('visibilitychange', onVisibilityChange)
  }

  function clearSyncStatus() {
    syncResult.value = null
    syncError.value = null
  }

  return {
    vehicles,
    activeVehicleId,
    activeVehicle,
    userRole,
    isOwner,
    isEditor,
    isViewer,
    canEdit,
    isIce,
    hasTeslaMate,
    currency,
    isSyncing,
    syncResult,
    syncError,
    lastSyncTimestamp,
    isInitialized,
    isLoading,
    fetchVehicles,
    setActiveVehicle,
    syncActiveVehicle,
    resumeRunningSync,
    startAutoRefresh,
    stopAutoRefresh,
    clearSyncStatus,
  }
})
