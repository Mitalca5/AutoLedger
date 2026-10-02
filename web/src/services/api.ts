import { t } from '@/i18n'
import { formatAmount } from '@/currency'
// AutoLedger API Service
import { newIdempotencyKey } from '@/services/offlineQueue'
import { apiErrorMessage } from '@/services/apiError'
import { csvImportForm, type CSVExecuteResult, type CSVImportBatch, type CSVImportOptions, type CSVImportProfile, type CSVPreviewResult } from '@/services/csvImport'

const BASE_URL = '/api'

// The access and refresh tokens live exclusively in HttpOnly cookies set by the API
// (see internal/handlers/auth_handler.go) — JS never reads or stores them, which removes
// them as an XSS exfiltration target compared to localStorage.
let refreshPromise: Promise<boolean> | null = null

function getHeaders(body?: any): HeadersInit {
  const headers: Record<string, string> = {}
  if (!(body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }
  return headers
}

async function refreshAccessToken(): Promise<boolean> {
  if (refreshPromise) {
    return refreshPromise
  }

  refreshPromise = (async () => {
    try {
      const res = await fetch(`${BASE_URL}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
      })
      return res.ok
    } catch {
      return false
    } finally {
      refreshPromise = null
    }
  })()

  return refreshPromise
}

export interface QueuedResult {
  queued: true
}

export interface AuthConfig {
  registration_enabled: boolean
  needs_onboarding: boolean
  user_count: number
  oidc_enabled: boolean
  oidc_provider_name: string
}

export interface ExpenseDocumentHeader {
  id: string
  vehicle_id: string
  filename: string
  mime_type: string
  file_size: number
  description?: string | null
  linked_expenses_count: number
  created_at: string
}

async function request<T>(endpoint: string, options: RequestInit = {}, offlineLabel?: string): Promise<T> {
  const idempotencyKey = offlineLabel ? newIdempotencyKey() : undefined
  const queueForLater = async () => {
    const { useOfflineStore } = await import('@/stores/offline')
    await useOfflineStore().queue({
      id: idempotencyKey!,
      method: options.method || 'POST',
      endpoint,
      body: typeof options.body === 'string' ? options.body : undefined,
      label: offlineLabel!,
      createdAt: Date.now(),
    })
    return { queued: true } as T
  }

  if (offlineLabel && !navigator.onLine) {
    return queueForLater()
  }

  let res: Response
  try {
    res = await fetch(`${BASE_URL}${endpoint}`, {
      ...options,
      credentials: options.credentials || 'include',
      headers: {
        ...getHeaders(options.body),
        ...(idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : {}),
        ...(options.headers || {}),
      },
    })
  } catch (err) {
    // Network failure: mutations flagged for offline use are kept and replayed later
    if (offlineLabel && err instanceof TypeError) {
      return queueForLater()
    }
    throw err
  }

  // Intercept 401 Unauthorized for token refresh
  if (res.status === 401) {
    const isAuthEndpoint =
      endpoint.startsWith('/auth/login') ||
      endpoint.startsWith('/auth/register') ||
      endpoint.startsWith('/auth/refresh') ||
      endpoint.startsWith('/auth/config')

    if (!isAuthEndpoint) {
      const refreshed = await refreshAccessToken()
      if (refreshed) {
        // The new access token cookie is already set by the browser; just retry.
        return request<T>(endpoint, options, offlineLabel)
      }

      // Refresh failed -> session is over, redirect to login
      if (
        window.location.pathname !== '/login' &&
        window.location.pathname !== '/register' &&
        window.location.pathname !== '/onboarding'
      ) {
        window.location.href = '/login'
      }
    }
  }

  let data: any
  const contentType = res.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    data = await res.json()
  } else {
    throw new Error(
      res.ok
        ? t('shell.api.invalidResponse')
        : t('shell.api.notReady', { status: res.status })
    )
  }

  if (!res.ok) {
    throw new Error(apiErrorMessage(data, t('shell.api.requestFailed', { status: res.status })))
  }

  return data as T
}

export const api = {
  // Auth
  login: (credentials: any) => request<any>('/auth/login', { method: 'POST', body: JSON.stringify(credentials) }),
  register: (payload: any) => request<any>('/auth/register', { method: 'POST', body: JSON.stringify(payload) }),
  refresh: () => request<any>('/auth/refresh', { method: 'POST' }),
  logout: () => request<any>('/auth/logout', { method: 'POST' }),
  getMe: () => request<any>('/auth/me'),
  getSessions: () => request<any[]>('/auth/sessions'),
  revokeSession: (id: string) => request<any>(`/auth/sessions/${id}`, { method: 'DELETE' }),
  logoutAll: () => request<any>('/auth/logout-all', { method: 'POST' }),
  changePassword: (data: { current_password: string; new_password: string }) =>
    request<any>('/auth/password', { method: 'POST', body: JSON.stringify(data) }),
  updateLanguage: (language: string) => request<any>('/auth/language', { method: 'PUT', body: JSON.stringify({ language }) }),
  updateDistanceUnit: (distance_unit: string) => request<any>('/auth/distance-unit', { method: 'PUT', body: JSON.stringify({ distance_unit }) }),
  getAuthConfig: () => request<AuthConfig>('/auth/config'),

  // Vehicles
  getVehicles: () => request<any[]>('/vehicles'),
  createVehicle: (data: any) => request<any>('/vehicles', { method: 'POST', body: JSON.stringify(data) }),
  updateVehicle: (id: string, data: any) => request<any>(`/vehicles/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteVehicle: (id: string) => request<any>(`/vehicles/${id}`, { method: 'DELETE' }),
  testTeslaMate: (id: string) => request<any>(`/vehicles/${id}/teslamate/test`, { method: 'POST' }),
  testTeslaMateRaw: (payload: any) =>
    request<any>('/vehicles/test-connection', { method: 'POST', body: JSON.stringify(payload) }),
  syncVehicle: (id: string) => request<any>(`/vehicles/${id}/sync`, { method: 'POST' }),
  getDataSources: (id: string) => request<{ teslamate_configured: boolean; activity: any[] }>(`/vehicles/${id}/data-sources`),
  getSyncStatus: (id: string) => request<any>(`/vehicles/${id}/sync`),
  getOwnership: (id: string) => request<any>(`/vehicles/${id}/ownership`),
  saveOwnership: (id: string, data: any) =>
    request<any>(`/vehicles/${id}/ownership`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteOwnership: (id: string) => request<any>(`/vehicles/${id}/ownership`, { method: 'DELETE' }),
  updateEstimatedEnergy: (id: string, data: { estimated_kwh_100km?: number | null; estimated_price_per_kwh?: number | null }) =>
    request<any>(`/vehicles/${id}/estimated-energy`, { method: 'PUT', body: JSON.stringify(data) }),
  getOdometerAt: (vehicleId: string, date: string) =>
    request<{ odometer: number; source: string }>(`/vehicles/${vehicleId}/odometer-at?date=${encodeURIComponent(date)}`),
  getOdometerEstimate: (vehicleId: string, date: string) =>
    request<{ odometer: number | null; source: string }>(`/vehicles/${vehicleId}/odometer-estimate?date=${encodeURIComponent(date)}`),
  getDataQuality: (vehicleId: string) => request<any>(`/vehicles/${vehicleId}/data-quality`),

  // Vehicle Members
  getVehicleMembers: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/members`),
  addVehicleMember: (vehicleId: string, payload: { email: string; role: string }) =>
    request<any>(`/vehicles/${vehicleId}/members`, { method: 'POST', body: JSON.stringify(payload) }),
  updateVehicleMemberRole: (vehicleId: string, memberId: string, payload: { role: string }) =>
    request<any>(`/vehicles/${vehicleId}/members/${memberId}`, { method: 'PUT', body: JSON.stringify(payload) }),
  removeVehicleMember: (vehicleId: string, memberId: string) =>
    request<any>(`/vehicles/${vehicleId}/members/${memberId}`, { method: 'DELETE' }),

  // Fuel fill-ups (combustion vehicles); the list comes with consumption figures per segment and global stats
  getFuelLogs: (vehicleId: string) => request<any>(`/vehicles/${vehicleId}/fuel-logs`),
  // A fill-up is always in the vehicle's own currency, which the payload does not carry
  createFuelLog: (vehicleId: string, data: any, currency: string) =>
    request<any>(`/vehicles/${vehicleId}/fuel-logs`, { method: 'POST', body: JSON.stringify(data) }, t('shell.api.fillUp', { amount: formatAmount(Number(data.amount), currency) })),
  updateFuelLog: (vehicleId: string, fuelLogId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/fuel-logs/${fuelLogId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteFuelLog: (vehicleId: string, fuelLogId: string) =>
    request<any>(`/vehicles/${vehicleId}/fuel-logs/${fuelLogId}`, { method: 'DELETE' }),

  // Odometer Checkpoints
  getOdometerCheckpoints: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/odometer-checkpoints`),
  createOdometerCheckpoint: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/odometer-checkpoints`, { method: 'POST', body: JSON.stringify(data) }),
  updateOdometerCheckpoint: (vehicleId: string, checkpointId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/odometer-checkpoints/${checkpointId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteOdometerCheckpoint: (vehicleId: string, checkpointId: string) =>
    request<any>(`/vehicles/${vehicleId}/odometer-checkpoints/${checkpointId}`, { method: 'DELETE' }),

  // Drives
  getDrives: (
    vehicleId: string,
    params?: {
      tag?: string
      page?: number
      limit?: number
      unqualified?: boolean
      hasToll?: boolean
      tollSource?: string
      tripGroupId?: string
      driveId?: string
      from?: string
      to?: string
      q?: string
    }
  ) => {
    const q = new URLSearchParams()
    if (params?.tag) q.set('tag', params.tag)
    if (params?.tripGroupId) q.set('trip_group_id', params.tripGroupId)
    if (params?.driveId) q.set('drive_id', params.driveId)
    if (params?.unqualified) q.set('unqualified', 'true')
    if (params?.hasToll) q.set('has_toll', 'true')
    if (params?.tollSource) q.set('toll_source', params.tollSource)
    if (params?.page) q.set('page', params.page.toString())
    if (params?.limit) q.set('limit', params.limit.toString())
    if (params?.from) q.set('from', params.from)
    if (params?.to) q.set('to', params.to)
    if (params?.q) q.set('q', params.q)
    return request<any>(`/vehicles/${vehicleId}/drives?${q.toString()}`)
  },
  createDrive: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/drives`, { method: 'POST', body: JSON.stringify(data) }),
  updateDrive: (vehicleId: string, driveId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/drives/${driveId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteDrive: (vehicleId: string, driveId: string) =>
    request<void>(`/vehicles/${vehicleId}/drives/${driveId}`, { method: 'DELETE' }),
  updateDriveDriver: (vehicleId: string, driveId: string, driverId: string | null) =>
    request<{ success: boolean }>(`/vehicles/${vehicleId}/drives/${driveId}/driver`, {
      method: 'PUT',
      body: JSON.stringify({ driver_id: driverId }),
    }),
  previewCSVImport: (vehicleId: string, file: File, opts: CSVImportOptions) =>
    request<CSVPreviewResult>(`/vehicles/${vehicleId}/import/preview`, { method: 'POST', body: csvImportForm(file, opts) }),
  executeCSVImport: (vehicleId: string, file: File, opts: CSVImportOptions) =>
    request<CSVExecuteResult>(`/vehicles/${vehicleId}/import/execute`, { method: 'POST', body: csvImportForm(file, opts) }),
  listImportBatches: (vehicleId: string) => request<CSVImportBatch[]>(`/vehicles/${vehicleId}/import/batches`),
  undoImportBatch: (vehicleId: string, batchId: string) =>
    request<{ removed: number }>(`/vehicles/${vehicleId}/import/batches/${batchId}`, { method: 'DELETE' }),
  listImportProfiles: () => request<CSVImportProfile[]>('/import-profiles'),
  saveImportProfile: (profile: Omit<CSVImportProfile, 'id'>) =>
    request<CSVImportProfile>('/import-profiles', { method: 'POST', body: JSON.stringify(profile) }),
  deleteImportProfile: (id: string) => request<void>(`/import-profiles/${id}`, { method: 'DELETE' }),
  updateDriveTags: (vehicleId: string, driveId: string, tags: string[]) =>
    request<any>(`/vehicles/${vehicleId}/drives/${driveId}/tags`, { method: 'PATCH', body: JSON.stringify({ tags }) }),
  setDriveTollReview: (vehicleId: string, driveId: string, reviewed: boolean) =>
    request<any>(
      `/vehicles/${vehicleId}/drives/${driveId}/toll-review`,
      { method: 'PATCH', body: JSON.stringify({ reviewed }) },
      t('shell.api.driveNoToll')
    ),
  getTollDetection: (vehicleId: string, driveId: string) =>
    request<any>(`/vehicles/${vehicleId}/drives/${driveId}/toll-detection`),
  applyTollEstimate: (vehicleId: string, driveId: string) =>
    request<any>(`/vehicles/${vehicleId}/drives/${driveId}/apply-toll-estimate`, { method: 'POST' }),
  applyTollEstimatesBulk: (vehicleId: string, driveIds: string[]) =>
    request<any>(`/vehicles/${vehicleId}/drives/apply-toll-estimates`, { method: 'POST', body: JSON.stringify({ drive_ids: driveIds }) }),
  detectTolls: (vehicleId: string, driveId: string) =>
    request<any>(`/vehicles/${vehicleId}/drives/${driveId}/detect-tolls`, { method: 'POST' }),
  createTripGroup: (vehicleId: string, payload: { name: string; notes?: string; drive_ids: string[] }) =>
    request<any>(`/vehicles/${vehicleId}/trip-groups`, { method: 'POST', body: JSON.stringify(payload) }),
  getTripGroups: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/trip-groups`),
  getTripSuggestions: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/trip-suggestions`),
  dismissTripSuggestion: (vehicleId: string, driveIds: string[]) =>
    request<any>(
      `/vehicles/${vehicleId}/trip-suggestions/dismiss`,
      { method: 'POST', body: JSON.stringify({ drive_ids: driveIds }) },
      t('shell.api.tripSuggestionDismiss')
    ),
  updateTripGroup: (vehicleId: string, groupId: string, payload: { name: string; notes?: string | null; drive_ids?: string[] }) =>
    request<any>(`/vehicles/${vehicleId}/trip-groups/${groupId}`, { method: 'PUT', body: JSON.stringify(payload) }),
  deleteTripGroup: (vehicleId: string, groupId: string, deleteExpenses = false) =>
    request<any>(`/vehicles/${vehicleId}/trip-groups/${groupId}?delete_expenses=${deleteExpenses}`, { method: 'DELETE' }),

  // Tires
  getTires: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/tires`),
  createTire: (vehicleId: string, data: any) => request<any>(`/vehicles/${vehicleId}/tires`, { method: 'POST', body: JSON.stringify(data) }),
  batchCreateTires: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/batch`, { method: 'POST', body: JSON.stringify(data) }),
  updateTire: (vehicleId: string, tireId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}`, { method: 'PUT', body: JSON.stringify(data) }),
  quickRotateTires: (vehicleId: string, data: { mode: string; odometer: number; swap_with_pack_tire_ids?: string[] }) =>
    request<any>(`/vehicles/${vehicleId}/tires/quick-rotate`, { method: 'POST', body: JSON.stringify(data) }),
  getTireHistory: (vehicleId: string, tireId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/history`),
  createTireSession: (vehicleId: string, tireId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/sessions`, { method: 'POST', body: JSON.stringify(data) }),
  updateTireSession: (vehicleId: string, tireId: string, sessionId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/sessions/${sessionId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTireSession: (vehicleId: string, tireId: string, sessionId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/sessions/${sessionId}`, { method: 'DELETE' }),
  batchUpdateTires: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/batch`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteTire: (vehicleId: string, tireId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}`, { method: 'DELETE' }),
  disposeTire: (vehicleId: string, tireId: string, data: { date: string; odometer?: number | null }) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/dispose`, { method: 'POST', body: JSON.stringify(data) }),
  batchDisposeTires: (vehicleId: string, data: { tire_ids: string[]; date: string; odometer?: number | null }) =>
    request<any>(`/vehicles/${vehicleId}/tires/batch-dispose`, { method: 'POST', body: JSON.stringify(data) }),
  copyTireHistory: (vehicleId: string, tireId: string, data: { target_tire_ids: string[]; copy_sessions?: boolean; copy_logs?: boolean; adapt_position?: boolean }) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/copy-history`, { method: 'POST', body: JSON.stringify(data) }),
  updateTireLog: (vehicleId: string, tireId: string, logId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/logs/${logId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTireLog: (vehicleId: string, tireId: string, logId: string) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/logs/${logId}`, { method: 'DELETE' }),
  addTireLog: (vehicleId: string, tireId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tires/${tireId}/logs`, { method: 'POST', body: JSON.stringify(data) }),
  rotateTires: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/tire-rotations`, { method: 'POST', body: JSON.stringify(data) }),

  // Expenses & Charges
  getDriveExpenses: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/expenses`),
  createDriveExpense: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/expenses`, { method: 'POST', body: JSON.stringify(data) }, t('shell.api.toll', { amount: formatAmount(Number(data.amount), data.currency || 'EUR') })),
  updateDriveExpense: (vehicleId: string, expenseId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/expenses/${expenseId}`, { method: 'PUT', body: JSON.stringify(data) }, t('shell.api.tollEdit')),
  deleteDriveExpense: (vehicleId: string, expenseId: string) =>
    request<void>(`/vehicles/${vehicleId}/expenses/${expenseId}`, { method: 'DELETE' }),
  getMaintenance: (vehicleId: string) => request<any[]>(`/vehicles/${vehicleId}/maintenance`),
  createMaintenance: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/maintenance`, { method: 'POST', body: JSON.stringify(data) }, t('shell.api.expense', { description: data.description })),
  updateMaintenance: (vehicleId: string, maintenanceId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/maintenance/${maintenanceId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteMaintenance: (vehicleId: string, maintenanceId: string) =>
    request<void>(`/vehicles/${vehicleId}/maintenance/${maintenanceId}`, { method: 'DELETE' }),
  getCharges: (vehicleId: string, params: { page?: number; limit?: number; missingCost?: boolean } = {}) => {
    const q = new URLSearchParams({ page: String(params.page || 1), limit: String(params.limit || 50) })
    if (params.missingCost) q.set('missing_cost', 'true')
    return request<any>(`/vehicles/${vehicleId}/charges?${q.toString()}`)
  },
  createCharge: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/charges`, { method: 'POST', body: JSON.stringify(data) }, t('shell.api.charge', { kwh: data.kwh_added })),
  updateCharge: (vehicleId: string, chargeId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/charges/${chargeId}`, { method: 'PUT', body: JSON.stringify(data) }, t('shell.api.chargeCost')),
  deleteCharge: (vehicleId: string, chargeId: string) =>
    request<any>(`/vehicles/${vehicleId}/charges/${chargeId}`, { method: 'DELETE' }),

  getDriveExpensesForDrive: (vehicleId: string, driveId: string) =>
    request<any[]>(`/vehicles/${vehicleId}/drives/${driveId}/expenses`),

  // TCO Analytics
  getTCO: (vehicleId: string) => request<any>(`/vehicles/${vehicleId}/tco`),
  getEnergyStats: (vehicleId: string) => request<any>(`/vehicles/${vehicleId}/energy-stats`),

  // EV vs ICE cost comparison (informational)
  getComparisonScenarios: () => request<any[]>('/comparison-scenarios'),
  createComparisonScenario: (data: any) =>
    request<any>('/comparison-scenarios', { method: 'POST', body: JSON.stringify(data) }),
  updateComparisonScenario: (scenarioId: string, data: any) =>
    request<any>(`/comparison-scenarios/${scenarioId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteComparisonScenario: (scenarioId: string) =>
    request<any>(`/comparison-scenarios/${scenarioId}`, { method: 'DELETE' }),
  getComparisonResult: (scenarioId: string) => request<any>(`/comparison-scenarios/${scenarioId}/result`),
  getComparisonDefaults: (vehicleId?: string) =>
    request<any>(`/comparison-scenarios/defaults${vehicleId ? `?vehicle_id=${encodeURIComponent(vehicleId)}` : ''}`),

  // Carpooling
  getCarpools: (vehicleId: string) => request<{ trips: any[]; summary: any }>(`/vehicles/${vehicleId}/carpools`),
  getCarpool: (vehicleId: string, id: string) => request<any>(`/vehicles/${vehicleId}/carpools/${id}`),
  createCarpool: (vehicleId: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/carpools`, { method: 'POST', body: JSON.stringify(data) }),
  updateCarpool: (vehicleId: string, id: string, data: any) =>
    request<any>(`/vehicles/${vehicleId}/carpools/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteCarpool: (vehicleId: string, id: string) =>
    request<any>(`/vehicles/${vehicleId}/carpools/${id}`, { method: 'DELETE' }),
  recalculateCarpools: (vehicleId: string, tripIds?: string[]) =>
    request<{ updated_count: number; trips: any[] }>(`/vehicles/${vehicleId}/carpools/recalculate`, {
      method: 'POST',
      body: JSON.stringify(tripIds && tripIds.length > 0 ? { trip_ids: tripIds } : {}),
    }),
  estimateCarpoolCosts: (vehicleId: string, params: { drive_id?: string; trip_group_id?: string; drive_ids?: string[]; distance_km?: number }) => {
    const q = new URLSearchParams()
    if (params.drive_id) q.set('drive_id', params.drive_id)
    if (params.trip_group_id) q.set('trip_group_id', params.trip_group_id)
    if (params.drive_ids && params.drive_ids.length > 0) q.set('drive_ids', params.drive_ids.join(','))
    if (params.distance_km !== undefined) q.set('distance_km', params.distance_km.toString())
    return request<any>(`/vehicles/${vehicleId}/carpools/estimate?${q.toString()}`)
  },

  // Documents & Invoices
  getDocuments: (vehicleId: string) =>
    request<ExpenseDocumentHeader[]>(`/vehicles/${vehicleId}/documents`),

  uploadDocument: async (vehicleId: string, file: File, description?: string): Promise<ExpenseDocumentHeader> => {
    const formData = new FormData()
    formData.append('file', file)
    if (description) {
      formData.append('description', description)
    }
    return request<ExpenseDocumentHeader>(`/vehicles/${vehicleId}/documents`, {
      method: 'POST',
      body: formData,
    })
  },

  deleteDocument: (vehicleId: string, docId: string) =>
    request<{ message: string }>(`/vehicles/${vehicleId}/documents/${docId}`, { method: 'DELETE' }),

  downloadDocumentBlob: async (vehicleId: string, docId: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await fetch(`${BASE_URL}/vehicles/${vehicleId}/documents/${docId}`, {
      credentials: 'include',
    })
    if (!res.ok) {
      let errorMsg = t('shell.api.documentLoadFailed')
      try {
        const errorData = await res.json()
        if (errorData && errorData.error) {
          errorMsg = errorData.error
        }
      } catch {
        // Non-JSON response
      }
      throw new Error(errorMsg)
    }
    const contentDisposition = res.headers.get('content-disposition') || ''
    let filename = 'document'
    const match = contentDisposition.match(/filename="?([^";]+)"?/)
    if (match && match[1]) {
      filename = match[1]
    }
    const blob = await res.blob()
    return { blob, filename }
  },

  // Maintenance Reminders & Webhooks
  getReminders: (vehicleId: string) =>
    request<MaintenanceReminder[]>(`/vehicles/${vehicleId}/reminders`),
  createReminder: (vehicleId: string, data: any) =>
    request<MaintenanceReminder>(`/vehicles/${vehicleId}/reminders`, { method: 'POST', body: JSON.stringify(data) }, t('shell.api.reminderCreated')),
  updateReminder: (vehicleId: string, reminderId: string, data: any) =>
    request<MaintenanceReminder>(`/vehicles/${vehicleId}/reminders/${reminderId}`, { method: 'PUT', body: JSON.stringify(data) }, t('shell.api.reminderUpdated')),
  deleteReminder: (vehicleId: string, reminderId: string) =>
    request<{ success: boolean }>(`/vehicles/${vehicleId}/reminders/${reminderId}`, { method: 'DELETE' }, t('shell.api.reminderDeleted')),
  completeReminder: (vehicleId: string, reminderId: string, data: { completed_date: string; completed_odometer?: number }) =>
    request<MaintenanceReminder>(`/vehicles/${vehicleId}/reminders/${reminderId}/complete`, { method: 'POST', body: JSON.stringify(data) }, t('shell.api.maintenanceDone')),
  getVehicleWebhook: (vehicleId: string) =>
    request<VehicleWebhook | null>(`/vehicles/${vehicleId}/webhook`),
  saveVehicleWebhook: (vehicleId: string, data: any) =>
    request<VehicleWebhook>(`/vehicles/${vehicleId}/webhook`, { method: 'PUT', body: JSON.stringify(data) }, t('shell.api.webhookSaved')),
  deleteVehicleWebhook: (vehicleId: string) =>
    request<{ success: boolean }>(`/vehicles/${vehicleId}/webhook`, { method: 'DELETE' }, t('shell.api.webhookDeleted')),
  testVehicleWebhook: (vehicleId: string, data: any) =>
    request<{ success: boolean; message: string }>(`/vehicles/${vehicleId}/webhook/test`, { method: 'POST', body: JSON.stringify(data) }, t('shell.api.webhookTest')),

  // API Tokens (Home Assistant & Automations)
  getAPITokens: () => request<{ tokens: APITokenInfo[] }>('/auth/tokens'),
  createAPIToken: (data: { name: string; expires_at?: string | null }) =>
    request<APITokenCreatedResponse>('/auth/tokens', { method: 'POST', body: JSON.stringify(data) }),
  revokeAPIToken: (tokenId: string) =>
    request<{ success: boolean }>(`/auth/tokens/${tokenId}`, { method: 'DELETE' }),

  // Tariff Plans & Public Charging
  getTariffPlans: () => request<{ plans: TariffPlan[] }>('/tariffs/plans'),
  createTariffPlan: (data: Partial<TariffPlan>) =>
    request<TariffPlan>('/tariffs/plans', { method: 'POST', body: JSON.stringify(data) }),
  updateTariffPlan: (id: string, data: Partial<TariffPlan>) =>
    request<TariffPlan>(`/tariffs/plans/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteTariffPlan: (id: string) =>
    request<{ success: boolean }>(`/tariffs/plans/${id}`, { method: 'DELETE' }),
  calculateSessionCost: (data: { plan_id?: string; start_time: string; end_time: string; kwh: number }) =>
    request<{ cost: number }>('/tariffs/calculate-session', { method: 'POST', body: JSON.stringify(data) }),

  getPublicPresets: () => request<{ presets: PublicChargingPreset[] }>('/tariffs/public-presets'),
  createPublicPreset: (data: Partial<PublicChargingPreset>) =>
    request<PublicChargingPreset>('/tariffs/public-presets', { method: 'POST', body: JSON.stringify(data) }),
  deletePublicPreset: (id: string) =>
    request<{ success: boolean }>(`/tariffs/public-presets/${id}`, { method: 'DELETE' }),
  calculatePublicCharge: (data: PublicChargingCalculationRequest) =>
    request<PublicChargingBreakdown>('/tariffs/calculate-public', { method: 'POST', body: JSON.stringify(data) }),

  // Pending Charges ("Recharges à qualifier")
  getPendingCharges: () => request<{ pending_charges: PendingCharge[] }>('/pending-charges'),
  assignPendingCharge: (id: string, data: AssignPendingChargeRequest) =>
    request<any>(`/pending-charges/${id}/assign`, { method: 'POST', body: JSON.stringify(data) }),
  deletePendingCharge: (id: string) =>
    request<{ success: boolean }>(`/pending-charges/${id}`, { method: 'DELETE' }),

  // Household Fleet Summary
  getFleetSummary: () => request<FleetSummaryResponse>('/fleet/summary'),
}

export interface MaintenanceReminder {
  id: string
  vehicle_id: string
  title: string
  category: string
  interval_km?: number | null
  interval_months?: number | null
  last_service_odometer?: number | null
  last_service_date?: string | null
  lead_km: number
  lead_days: number
  webhook_enabled: boolean
  last_notified_at?: string | null
  last_notified_odometer?: number | null
  created_at: string
  updated_at: string
  status: 'OK' | 'DUE_SOON' | 'OVERDUE'
  remaining_km?: number | null
  remaining_days?: number | null
  due_odometer?: number | null
  due_date?: string | null
}

export interface VehicleWebhook {
  id: string
  vehicle_id: string
  url: string
  type: string
  enabled: boolean
  created_at: string
  updated_at: string
}

// API Tokens
export interface APITokenInfo {
  id: string
  name: string
  token_prefix: string
  last_used_at: string | null
  expires_at: string | null
  created_at: string
}

export interface APITokenCreatedResponse {
  token: string
  info: APITokenInfo
}

// Tariffs & Public Charging
export interface TimeWindow {
  start_time: string
  end_time: string
  days_of_week?: number[]
}

export interface TariffPlan {
  id: string
  user_id: string
  name: string
  plan_type: 'FLAT' | 'TIME_OF_USE'
  currency: string
  flat_rate_cents?: number | null
  peak_rate_cents?: number | null
  offpeak_rate_cents?: number | null
  time_windows?: TimeWindow[] | null
  is_default: boolean
  created_at: string
  updated_at: string
}

export interface PublicChargingPreset {
  id: string
  user_id: string
  name: string
  network?: string | null
  currency: string
  connection_fee_cents?: number | null
  cost_per_kwh_cents?: number | null
  cost_per_minute_cents?: number | null
  idle_fee_per_minute_cents?: number | null
  idle_grace_minutes?: number | null
  created_at: string
}

export interface PublicChargingCalculationRequest {
  connection_fee_cents?: number | null
  cost_per_kwh_cents?: number | null
  cost_per_minute_cents?: number | null
  idle_fee_per_minute_cents?: number | null
  idle_grace_minutes?: number | null
  kwh: number
  charging_minutes: number
  idle_minutes?: number | null
}

export interface PublicChargingBreakdown {
  connection_fee_cents: number
  energy_cost_cents: number
  duration_cost_cents: number
  idle_cost_cents: number
  total_cost_cents: number
  summary: string
}

// Pending Charges ("Recharges à qualifier")
export interface PendingCharge {
  id: string
  user_id: string
  source: string
  charger_name?: string | null
  start_time: string
  end_time: string
  energy_kwh: number
  location: string
  raw_data?: Record<string, any> | null
  created_at: string
}

export interface AssignPendingChargeRequest {
  vehicle_id: string
  driver_id?: string | null
}

// Household Fleet Dashboard
export interface FleetMonthlyCost {
  month: string
  total_cost: number
  energy_cost: number
  other_cost: number
  distance_km: number
  by_vehicle: Record<string, number>
}

export interface VehicleFleetMetric {
  vehicle_id: string
  name: string
  make: string
  model: string
  currency: string
  current_odometer: number
  month_distance_km: number
  month_cost: number
  energy_cost_per_100km: number
}

export interface MemberKmShare {
  driver_id: string | null
  driver_name: string
  distance_km: number
  percentage: number
}

export interface FleetSummaryResponse {
  total_vehicles: number
  currency: string
  current_month_cost: number
  current_month_distance_km: number
  current_month_energy_kwh: number
  monthly_costs: FleetMonthlyCost[]
  vehicles: VehicleFleetMetric[]
  member_km_shares: MemberKmShare[]
}
