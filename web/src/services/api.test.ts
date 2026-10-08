import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const queue = vi.fn()
vi.mock('@/stores/offline', () => ({ useOfflineStore: () => ({ queue }) }))

import { api } from './api'

function jsonResponse(body: unknown, status = 200, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json', ...headers } })
}

const fetchMock = vi.fn()
const location = { pathname: '/dashboard', href: '' }

beforeEach(() => {
  fetchMock.mockReset()
  queue.mockReset()
  location.pathname = '/dashboard'
  location.href = ''
  vi.stubGlobal('fetch', fetchMock)
  vi.stubGlobal('window', { location })
  vi.stubGlobal('navigator', { onLine: true })
})

afterEach(() => vi.unstubAllGlobals())

describe('request', () => {
  it('sends credentials and a JSON content type, and returns the parsed body', async () => {
    fetchMock.mockResolvedValue(jsonResponse([{ id: 'v1' }]))
    expect(await api.getVehicles()).toEqual([{ id: 'v1' }])
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/vehicles')
    expect(init.credentials).toBe('include')
    expect(init.headers['Content-Type']).toBe('application/json')
    expect(init.headers['Idempotency-Key']).toBeUndefined()
  })

  it('leaves the content type to the browser for a multipart body', async () => {
    fetchMock.mockImplementation(async () => jsonResponse({ id: 'd1' }))
    await api.uploadDocument('v1', new File(['x'], 'a.pdf'), 'invoice')
    const [, init] = fetchMock.mock.calls[0]
    expect(init.body).toBeInstanceOf(FormData)
    expect((init.body as FormData).get('description')).toBe('invoice')
    expect(init.headers['Content-Type']).toBeUndefined()

    await api.uploadDocument('v1', new File(['x'], 'b.pdf'))
    expect((fetchMock.mock.calls[1][1].body as FormData).has('description')).toBe(false)
  })

  it('throws the API error message on a failed response', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'vehicle not found' }, 404))
    await expect(api.getVehicles()).rejects.toThrow('vehicle not found')
  })

  it('rejects a response that is not JSON, with a different message when it is a failure', async () => {
    fetchMock.mockResolvedValueOnce(new Response('<html>', { status: 200, headers: { 'content-type': 'text/html' } }))
    const invalid = await api.getVehicles().catch((e: Error) => e.message)
    fetchMock.mockResolvedValueOnce(new Response('<html>', { status: 502, headers: { 'content-type': 'text/html' } }))
    const notReady = await api.getVehicles().catch((e: Error) => e.message)
    expect(invalid).toBeTruthy()
    expect(notReady).toContain('502')
    expect(invalid).not.toBe(notReady)
  })

  it('refreshes the session on a 401 and replays the request', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'expired' }, 401))
      .mockResolvedValueOnce(jsonResponse({}, 200)) // /auth/refresh
      .mockResolvedValueOnce(jsonResponse([{ id: 'v1' }]))
    expect(await api.getVehicles()).toEqual([{ id: 'v1' }])
    expect(fetchMock.mock.calls[1][0]).toBe('/api/auth/refresh')
    expect(fetchMock.mock.calls[2][0]).toBe('/api/vehicles')
  })

  it('shares one refresh between concurrent 401s', async () => {
    let refreshes = 0
    const seen = new Set<string>()
    fetchMock.mockImplementation(async (url: string) => {
      if (url === '/api/auth/refresh') {
        refreshes++
        await new Promise((resolve) => setTimeout(resolve, 10))
        return jsonResponse({})
      }
      if (!seen.has(url)) {
        seen.add(url)
        return jsonResponse({ error: 'expired' }, 401)
      }
      return jsonResponse([])
    })
    await Promise.all([api.getVehicles(), api.getTires('v1')])
    expect(refreshes).toBe(1)
  })

  it.each([
    ['/login', '/login'],
    ['/dashboard', '/login'],
  ])('sends the user to the login page when the refresh fails (from %s)', async (from, expected) => {
    location.pathname = from
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'expired' }, 401))
      .mockResolvedValueOnce(jsonResponse({}, 401)) // refresh refused
      .mockResolvedValueOnce(jsonResponse({ error: 'expired' }, 401))
    await expect(api.getVehicles()).rejects.toThrow()
    expect(location.href).toBe(from === '/login' ? '' : expected)
  })

  it('treats a network error during the refresh as a failed refresh', async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ error: 'expired' }, 401))
      .mockRejectedValueOnce(new TypeError('offline'))
      .mockResolvedValueOnce(jsonResponse({ error: 'expired' }, 401))
    await expect(api.getVehicles()).rejects.toThrow()
    expect(location.href).toBe('/login')
  })

  it('does not try to refresh a failed login', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: 'bad credentials' }, 401))
    await expect(api.login({ email: 'a', password: 'b' })).rejects.toThrow('bad credentials')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})

describe('offline mutations', () => {
  it('queues a flagged mutation made while offline, with an idempotency key', async () => {
    vi.stubGlobal('navigator', { onLine: false })
    const result = await api.createCharge('v1', { kwh_added: 12 })
    expect(result).toEqual({ queued: true })
    expect(fetchMock).not.toHaveBeenCalled()
    const queued = queue.mock.calls[0][0]
    expect(queued).toMatchObject({ method: 'POST', endpoint: '/vehicles/v1/charges', body: JSON.stringify({ kwh_added: 12 }) })
    expect(queued.id).toBeTruthy()
  })

  it('queues it when the network drops mid-request, but rethrows other failures', async () => {
    fetchMock.mockRejectedValueOnce(new TypeError('network'))
    expect(await api.updateCharge('v1', 'c1', {})).toEqual({ queued: true })
    expect(queue.mock.calls[0][0].method).toBe('PUT')

    fetchMock.mockRejectedValueOnce(new Error('abort'))
    await expect(api.updateCharge('v1', 'c1', {})).rejects.toThrow('abort')

    fetchMock.mockRejectedValueOnce(new TypeError('network'))
    await expect(api.getCharges('v1')).rejects.toThrow('network')
  })

  it('sends the idempotency key with an online flagged mutation', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: 'c1' }))
    await api.createCharge('v1', { kwh_added: 3 })
    expect(fetchMock.mock.calls[0][1].headers['Idempotency-Key']).toBeTruthy()
  })
})

describe('downloads', () => {
  it('takes the file name from Content-Disposition, or falls back', async () => {
    fetchMock.mockResolvedValueOnce(new Response('csv', { headers: { 'content-disposition': 'attachment; filename="drives.csv"' } }))
    const named = await api.downloadExport('v1', { type: 'drives', format: 'csv', from: '2026-01-01', to: '2026-02-01', tag: 'work', rates: 'x' })
    expect(named.filename).toBe('drives.csv')
    expect(fetchMock.mock.calls[0][0]).toBe('/api/vehicles/v1/export?type=drives&format=csv&from=2026-01-01&to=2026-02-01&tag=work&rates=x')

    fetchMock.mockResolvedValueOnce(new Response('csv'))
    expect((await api.downloadExport('v1', { type: 'drives', format: 'xlsx' })).filename).toBe('export.xlsx')

    fetchMock.mockResolvedValueOnce(new Response('pdf'))
    const book = await api.downloadServiceBook('v1', { from: 'a', to: 'b', attachments: true })
    expect(book.filename).toBe('service-book.pdf')
    expect(fetchMock.mock.calls[2][0]).toBe('/api/vehicles/v1/service-book?from=a&to=b&attachments=1')
  })

  it('keeps the server error message of a failed export when there is one', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'no data in range' }, 400))
    await expect(api.downloadExport('v1', { type: 'drives', format: 'csv' })).rejects.toThrow('no data in range')
    fetchMock.mockResolvedValueOnce(new Response('oops', { status: 500 }))
    await expect(api.downloadExport('v1', { type: 'drives', format: 'csv' })).rejects.toThrow()
  })

  it('downloads a document with its name, and reports a failure', async () => {
    fetchMock.mockResolvedValueOnce(new Response('pdf', { headers: { 'content-disposition': 'attachment; filename=inv.pdf' } }))
    expect((await api.downloadDocumentBlob('v1', 'd1')).filename).toBe('inv.pdf')
    fetchMock.mockResolvedValueOnce(new Response('pdf'))
    expect((await api.downloadDocumentBlob('v1', 'd1')).filename).toBe('document')
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: 'gone' }, 404))
    await expect(api.downloadDocumentBlob('v1', 'd1')).rejects.toThrow('gone')
    fetchMock.mockResolvedValueOnce(new Response('x', { status: 500 }))
    await expect(api.downloadDocumentBlob('v1', 'd1')).rejects.toThrow()
  })
})

describe('query building', () => {
  beforeEach(() => fetchMock.mockImplementation(async () => jsonResponse({})))
  const lastUrl = () => fetchMock.mock.calls.at(-1)![0] as string

  it('only sends the drive filters that are set', async () => {
    await api.getDrives('v1')
    expect(lastUrl()).toBe('/api/vehicles/v1/drives?')
    await api.getDrives('v1', {
      tag: 'work', tripGroupId: 't1', driveId: 'd1', unqualified: true, hasToll: true, tollSource: 'manual',
      page: 2, limit: 10, from: '2026-01-01', to: '2026-02-01', q: 'paris',
    })
    expect(lastUrl()).toBe(
      '/api/vehicles/v1/drives?tag=work&trip_group_id=t1&drive_id=d1&unqualified=true&has_toll=true&toll_source=manual&page=2&limit=10&from=2026-01-01&to=2026-02-01&q=paris'
    )
  })

  it('defaults the charge pagination and filters on missing cost', async () => {
    await api.getCharges('v1')
    expect(lastUrl()).toBe('/api/vehicles/v1/charges?page=1&limit=50')
    await api.getCharges('v1', { page: 3, limit: 5, missingCost: true })
    expect(lastUrl()).toBe('/api/vehicles/v1/charges?page=3&limit=5&missing_cost=true')
  })

  it('skips undefined and NaN residual value options', async () => {
    await api.getResidualValue('v1', { expected_km: 100000, km_share: Number.NaN, health_weight: undefined })
    expect(lastUrl()).toBe('/api/vehicles/v1/residual-value?expected_km=100000')
  })

  it('builds the comparison defaults and carpool estimate queries', async () => {
    await api.getComparisonDefaults()
    expect(lastUrl()).toBe('/api/comparison-scenarios/defaults')
    await api.getComparisonDefaults('v1', 'USD')
    expect(lastUrl()).toBe('/api/comparison-scenarios/defaults?vehicle_id=v1&currency=USD')

    await api.estimateCarpoolCosts('v1', { drive_id: 'd1', trip_group_id: 't1', drive_ids: ['a', 'b'], distance_km: 0 })
    expect(lastUrl()).toBe('/api/vehicles/v1/carpools/estimate?drive_id=d1&trip_group_id=t1&drive_ids=a%2Cb&distance_km=0')
    await api.estimateCarpoolCosts('v1', { drive_ids: [] })
    expect(lastUrl()).toBe('/api/vehicles/v1/carpools/estimate?')
  })

  it('recalculates only the given carpool trips', async () => {
    await api.recalculateCarpools('v1', ['a'])
    expect(JSON.parse(fetchMock.mock.calls.at(-1)![1].body)).toEqual({ trip_ids: ['a'] })
    await api.recalculateCarpools('v1', [])
    expect(JSON.parse(fetchMock.mock.calls.at(-1)![1].body)).toEqual({})
    await api.recalculateCarpools('v1')
    expect(JSON.parse(fetchMock.mock.calls.at(-1)![1].body)).toEqual({})
  })

  it('encodes the odometer dates and trip deletion flag', async () => {
    await api.getOdometerAt('v1', '2026-01-01T10:00:00+01:00')
    expect(lastUrl()).toBe('/api/vehicles/v1/odometer-at?date=2026-01-01T10%3A00%3A00%2B01%3A00')
    await api.getOdometerEstimate('v1', '2026-01-01')
    expect(lastUrl()).toBe('/api/vehicles/v1/odometer-estimate?date=2026-01-01')
    await api.deleteTripGroup('v1', 'g1')
    expect(lastUrl()).toBe('/api/vehicles/v1/trip-groups/g1?delete_expenses=false')
    await api.deleteTripGroup('v1', 'g1', true)
    expect(lastUrl()).toBe('/api/vehicles/v1/trip-groups/g1?delete_expenses=true')
  })
})

describe('endpoint contract', () => {
  beforeEach(() => fetchMock.mockImplementation(async () => jsonResponse({})))

  const cases: Array<[string, () => Promise<unknown>, string, string, unknown?]> = [
    ['login', () => api.login({ email: 'e' }), 'POST', '/auth/login', { email: 'e' }],
    ['register', () => api.register({ email: 'e' }), 'POST', '/auth/register', { email: 'e' }],
    ['refresh', () => api.refresh(), 'POST', '/auth/refresh'],
    ['logout', () => api.logout(), 'POST', '/auth/logout'],
    ['getMe', () => api.getMe(), 'GET', '/auth/me'],
    ['getSessions', () => api.getSessions(), 'GET', '/auth/sessions'],
    ['revokeSession', () => api.revokeSession('s1'), 'DELETE', '/auth/sessions/s1'],
    ['logoutAll', () => api.logoutAll(), 'POST', '/auth/logout-all'],
    ['changePassword', () => api.changePassword({ current_password: 'a', new_password: 'b' }), 'POST', '/auth/password', { current_password: 'a', new_password: 'b' }],
    ['updateLanguage', () => api.updateLanguage('en'), 'PUT', '/auth/language', { language: 'en' }],
    ['updateDistanceUnit', () => api.updateDistanceUnit('mi'), 'PUT', '/auth/distance-unit', { distance_unit: 'mi' }],
    ['getAuthConfig', () => api.getAuthConfig(), 'GET', '/auth/config'],
    ['createVehicle', () => api.createVehicle({ name: 'n' }), 'POST', '/vehicles', { name: 'n' }],
    ['updateVehicle', () => api.updateVehicle('v1', { name: 'n' }), 'PUT', '/vehicles/v1', { name: 'n' }],
    ['deleteVehicle', () => api.deleteVehicle('v1'), 'DELETE', '/vehicles/v1'],
    ['testTeslaMate', () => api.testTeslaMate('v1'), 'POST', '/vehicles/v1/teslamate/test'],
    ['testTeslaMateRaw', () => api.testTeslaMateRaw({ url: 'u' }), 'POST', '/vehicles/test-connection', { url: 'u' }],
    ['syncVehicle', () => api.syncVehicle('v1'), 'POST', '/vehicles/v1/sync'],
    ['getDataSources', () => api.getDataSources('v1'), 'GET', '/vehicles/v1/data-sources'],
    ['getSyncStatus', () => api.getSyncStatus('v1'), 'GET', '/vehicles/v1/sync'],
    ['getOwnership', () => api.getOwnership('v1'), 'GET', '/vehicles/v1/ownership'],
    ['saveOwnership', () => api.saveOwnership('v1', { a: 1 }), 'PUT', '/vehicles/v1/ownership', { a: 1 }],
    ['deleteOwnership', () => api.deleteOwnership('v1'), 'DELETE', '/vehicles/v1/ownership'],
    ['updateEstimatedEnergy', () => api.updateEstimatedEnergy('v1', { estimated_kwh_100km: 15 }), 'PUT', '/vehicles/v1/estimated-energy', { estimated_kwh_100km: 15 }],
    ['getDataQuality', () => api.getDataQuality('v1'), 'GET', '/vehicles/v1/data-quality'],
    ['getVehicleMembers', () => api.getVehicleMembers('v1'), 'GET', '/vehicles/v1/members'],
    ['addVehicleMember', () => api.addVehicleMember('v1', { email: 'e', role: 'VIEWER' }), 'POST', '/vehicles/v1/members', { email: 'e', role: 'VIEWER' }],
    ['updateVehicleMemberRole', () => api.updateVehicleMemberRole('v1', 'm1', { role: 'EDITOR' }), 'PUT', '/vehicles/v1/members/m1', { role: 'EDITOR' }],
    ['removeVehicleMember', () => api.removeVehicleMember('v1', 'm1'), 'DELETE', '/vehicles/v1/members/m1'],
    ['getVehiclePeople', () => api.getVehiclePeople('v1'), 'GET', '/vehicles/v1/people'],
    ['createVehiclePerson', () => api.createVehiclePerson('v1', 'Ann'), 'POST', '/vehicles/v1/people', { name: 'Ann' }],
    ['renameVehiclePerson', () => api.renameVehiclePerson('v1', 'p1', 'Bo'), 'PUT', '/vehicles/v1/people/p1', { name: 'Bo' }],
    ['linkVehiclePerson', () => api.linkVehiclePerson('v1', 'p1', 'u1'), 'PUT', '/vehicles/v1/people/p1/link', { user_id: 'u1' }],
    ['setDefaultVehiclePerson', () => api.setDefaultVehiclePerson('v1', 'p1'), 'PUT', '/vehicles/v1/people/p1/default'],
    ['deleteVehiclePerson', () => api.deleteVehiclePerson('v1', 'p1'), 'DELETE', '/vehicles/v1/people/p1'],
    ['getFuelLogs', () => api.getFuelLogs('v1'), 'GET', '/vehicles/v1/fuel-logs'],
    ['createFuelLog', () => api.createFuelLog('v1', { amount: 40 }, 'EUR'), 'POST', '/vehicles/v1/fuel-logs', { amount: 40 }],
    ['updateFuelLog', () => api.updateFuelLog('v1', 'f1', { amount: 1 }), 'PUT', '/vehicles/v1/fuel-logs/f1', { amount: 1 }],
    ['deleteFuelLog', () => api.deleteFuelLog('v1', 'f1'), 'DELETE', '/vehicles/v1/fuel-logs/f1'],
    ['getOdometerCheckpoints', () => api.getOdometerCheckpoints('v1'), 'GET', '/vehicles/v1/odometer-checkpoints'],
    ['createOdometerCheckpoint', () => api.createOdometerCheckpoint('v1', { o: 1 }), 'POST', '/vehicles/v1/odometer-checkpoints', { o: 1 }],
    ['updateOdometerCheckpoint', () => api.updateOdometerCheckpoint('v1', 'c1', { o: 2 }), 'PUT', '/vehicles/v1/odometer-checkpoints/c1', { o: 2 }],
    ['deleteOdometerCheckpoint', () => api.deleteOdometerCheckpoint('v1', 'c1'), 'DELETE', '/vehicles/v1/odometer-checkpoints/c1'],
    ['getAddressBackfill', () => api.getAddressBackfill('v1'), 'GET', '/vehicles/v1/drives/address-backfill'],
    ['resolveDriveAddresses', () => api.resolveDriveAddresses('v1'), 'POST', '/vehicles/v1/drives/resolve-addresses'],
    ['createDrive', () => api.createDrive('v1', { d: 1 }), 'POST', '/vehicles/v1/drives', { d: 1 }],
    ['updateDrive', () => api.updateDrive('v1', 'd1', { d: 2 }), 'PUT', '/vehicles/v1/drives/d1', { d: 2 }],
    ['deleteDrive', () => api.deleteDrive('v1', 'd1'), 'DELETE', '/vehicles/v1/drives/d1'],
    ['updateDriveDriver', () => api.updateDriveDriver('v1', 'd1', null), 'PUT', '/vehicles/v1/drives/d1/driver', { driver_id: null }],
    ['listImportBatches', () => api.listImportBatches('v1'), 'GET', '/vehicles/v1/import/batches'],
    ['undoImportBatch', () => api.undoImportBatch('v1', 'b1'), 'DELETE', '/vehicles/v1/import/batches/b1'],
    ['listImportProfiles', () => api.listImportProfiles(), 'GET', '/import-profiles'],
    ['deleteImportProfile', () => api.deleteImportProfile('p1'), 'DELETE', '/import-profiles/p1'],
    ['updateDriveTags', () => api.updateDriveTags('v1', 'd1', ['a']), 'PATCH', '/vehicles/v1/drives/d1/tags', { tags: ['a'] }],
    ['setDriveTollReview', () => api.setDriveTollReview('v1', 'd1', true), 'PATCH', '/vehicles/v1/drives/d1/toll-review', { reviewed: true }],
    ['getTollDetection', () => api.getTollDetection('v1', 'd1'), 'GET', '/vehicles/v1/drives/d1/toll-detection'],
    ['applyTollEstimate', () => api.applyTollEstimate('v1', 'd1'), 'POST', '/vehicles/v1/drives/d1/apply-toll-estimate'],
    ['applyTollEstimatesBulk', () => api.applyTollEstimatesBulk('v1', ['d1']), 'POST', '/vehicles/v1/drives/apply-toll-estimates', { drive_ids: ['d1'] }],
    ['detectTolls', () => api.detectTolls('v1', 'd1'), 'POST', '/vehicles/v1/drives/d1/detect-tolls'],
    ['createTripGroup', () => api.createTripGroup('v1', { name: 'n', drive_ids: ['d1'] }), 'POST', '/vehicles/v1/trip-groups', { name: 'n', drive_ids: ['d1'] }],
    ['getTripGroups', () => api.getTripGroups('v1'), 'GET', '/vehicles/v1/trip-groups'],
    ['getTripSuggestions', () => api.getTripSuggestions('v1'), 'GET', '/vehicles/v1/trip-suggestions'],
    ['dismissTripSuggestion', () => api.dismissTripSuggestion('v1', ['d1']), 'POST', '/vehicles/v1/trip-suggestions/dismiss', { drive_ids: ['d1'] }],
    ['updateTripGroup', () => api.updateTripGroup('v1', 'g1', { name: 'n' }), 'PUT', '/vehicles/v1/trip-groups/g1', { name: 'n' }],
    ['getTires', () => api.getTires('v1'), 'GET', '/vehicles/v1/tires'],
    ['createTire', () => api.createTire('v1', { t: 1 }), 'POST', '/vehicles/v1/tires', { t: 1 }],
    ['batchCreateTires', () => api.batchCreateTires('v1', { t: 1 }), 'POST', '/vehicles/v1/tires/batch', { t: 1 }],
    ['updateTire', () => api.updateTire('v1', 't1', { t: 2 }), 'PUT', '/vehicles/v1/tires/t1', { t: 2 }],
    ['quickRotateTires', () => api.quickRotateTires('v1', { mode: 'swap', odometer: 5 }), 'POST', '/vehicles/v1/tires/quick-rotate', { mode: 'swap', odometer: 5 }],
    ['getTireHistory', () => api.getTireHistory('v1', 't1'), 'GET', '/vehicles/v1/tires/t1/history'],
    ['createTireSession', () => api.createTireSession('v1', 't1', { s: 1 }), 'POST', '/vehicles/v1/tires/t1/sessions', { s: 1 }],
    ['updateTireSession', () => api.updateTireSession('v1', 't1', 's1', { s: 2 }), 'PUT', '/vehicles/v1/tires/t1/sessions/s1', { s: 2 }],
    ['deleteTireSession', () => api.deleteTireSession('v1', 't1', 's1'), 'DELETE', '/vehicles/v1/tires/t1/sessions/s1'],
    ['batchUpdateTires', () => api.batchUpdateTires('v1', { b: 1 }), 'PATCH', '/vehicles/v1/tires/batch', { b: 1 }],
    ['deleteTire', () => api.deleteTire('v1', 't1'), 'DELETE', '/vehicles/v1/tires/t1'],
    ['disposeTire', () => api.disposeTire('v1', 't1', { date: 'd' }), 'POST', '/vehicles/v1/tires/t1/dispose', { date: 'd' }],
    ['batchDisposeTires', () => api.batchDisposeTires('v1', { tire_ids: ['t1'], date: 'd' }), 'POST', '/vehicles/v1/tires/batch-dispose', { tire_ids: ['t1'], date: 'd' }],
    ['copyTireHistory', () => api.copyTireHistory('v1', 't1', { target_tire_ids: ['t2'] }), 'POST', '/vehicles/v1/tires/t1/copy-history', { target_tire_ids: ['t2'] }],
    ['updateTireLog', () => api.updateTireLog('v1', 't1', 'l1', { l: 1 }), 'PUT', '/vehicles/v1/tires/t1/logs/l1', { l: 1 }],
    ['deleteTireLog', () => api.deleteTireLog('v1', 't1', 'l1'), 'DELETE', '/vehicles/v1/tires/t1/logs/l1'],
    ['addTireLog', () => api.addTireLog('v1', 't1', { l: 1 }), 'POST', '/vehicles/v1/tires/t1/logs', { l: 1 }],
    ['rotateTires', () => api.rotateTires('v1', { r: 1 }), 'POST', '/vehicles/v1/tire-rotations', { r: 1 }],
    ['getDriveExpenses', () => api.getDriveExpenses('v1'), 'GET', '/vehicles/v1/expenses'],
    ['createDriveExpense', () => api.createDriveExpense('v1', { amount: 3 }), 'POST', '/vehicles/v1/expenses', { amount: 3 }],
    ['createDriveExpense (currency)', () => api.createDriveExpense('v1', { amount: 3, currency: 'USD' }), 'POST', '/vehicles/v1/expenses', { amount: 3, currency: 'USD' }],
    ['updateDriveExpense', () => api.updateDriveExpense('v1', 'e1', { e: 1 }), 'PUT', '/vehicles/v1/expenses/e1', { e: 1 }],
    ['deleteDriveExpense', () => api.deleteDriveExpense('v1', 'e1'), 'DELETE', '/vehicles/v1/expenses/e1'],
    ['getMaintenance', () => api.getMaintenance('v1'), 'GET', '/vehicles/v1/maintenance'],
    ['createMaintenance', () => api.createMaintenance('v1', { description: 'oil' }), 'POST', '/vehicles/v1/maintenance', { description: 'oil' }],
    ['updateMaintenance', () => api.updateMaintenance('v1', 'm1', { m: 1 }), 'PUT', '/vehicles/v1/maintenance/m1', { m: 1 }],
    ['deleteMaintenance', () => api.deleteMaintenance('v1', 'm1'), 'DELETE', '/vehicles/v1/maintenance/m1'],
    ['createCharge', () => api.createCharge('v1', { kwh_added: 5 }), 'POST', '/vehicles/v1/charges', { kwh_added: 5 }],
    ['deleteCharge', () => api.deleteCharge('v1', 'c1'), 'DELETE', '/vehicles/v1/charges/c1'],
    ['getDriveExpensesForDrive', () => api.getDriveExpensesForDrive('v1', 'd1'), 'GET', '/vehicles/v1/drives/d1/expenses'],
    ['getTCO', () => api.getTCO('v1'), 'GET', '/vehicles/v1/tco'],
    ['getBatteryHealth', () => api.getBatteryHealth('v1'), 'GET', '/vehicles/v1/battery-health'],
    ['saveBatteryReading', () => api.saveBatteryReading('v1', { date: 'd' }), 'POST', '/vehicles/v1/battery-health', { date: 'd' }],
    ['deleteBatteryReading', () => api.deleteBatteryReading('v1', '2026-01-01'), 'DELETE', '/vehicles/v1/battery-health/2026-01-01'],
    ['getEnergyStats', () => api.getEnergyStats('v1'), 'GET', '/vehicles/v1/energy-stats'],
    ['getComparisonScenarios', () => api.getComparisonScenarios(), 'GET', '/comparison-scenarios'],
    ['createComparisonScenario', () => api.createComparisonScenario({ s: 1 }), 'POST', '/comparison-scenarios', { s: 1 }],
    ['updateComparisonScenario', () => api.updateComparisonScenario('s1', { s: 2 }), 'PUT', '/comparison-scenarios/s1', { s: 2 }],
    ['deleteComparisonScenario', () => api.deleteComparisonScenario('s1'), 'DELETE', '/comparison-scenarios/s1'],
    ['getComparisonResult', () => api.getComparisonResult('s1'), 'GET', '/comparison-scenarios/s1/result'],
    ['getCarpools', () => api.getCarpools('v1'), 'GET', '/vehicles/v1/carpools'],
    ['getCarpool', () => api.getCarpool('v1', 'c1'), 'GET', '/vehicles/v1/carpools/c1'],
    ['createCarpool', () => api.createCarpool('v1', { c: 1 }), 'POST', '/vehicles/v1/carpools', { c: 1 }],
    ['updateCarpool', () => api.updateCarpool('v1', 'c1', { c: 2 }), 'PUT', '/vehicles/v1/carpools/c1', { c: 2 }],
    ['deleteCarpool', () => api.deleteCarpool('v1', 'c1'), 'DELETE', '/vehicles/v1/carpools/c1'],
    ['getDocuments', () => api.getDocuments('v1'), 'GET', '/vehicles/v1/documents'],
    ['deleteDocument', () => api.deleteDocument('v1', 'd1'), 'DELETE', '/vehicles/v1/documents/d1'],
    ['getReminders', () => api.getReminders('v1'), 'GET', '/vehicles/v1/reminders'],
    ['createReminder', () => api.createReminder('v1', { r: 1 }), 'POST', '/vehicles/v1/reminders', { r: 1 }],
    ['updateReminder', () => api.updateReminder('v1', 'r1', { r: 2 }), 'PUT', '/vehicles/v1/reminders/r1', { r: 2 }],
    ['deleteReminder', () => api.deleteReminder('v1', 'r1'), 'DELETE', '/vehicles/v1/reminders/r1'],
    ['completeReminder', () => api.completeReminder('v1', 'r1', { completed_date: 'd' }), 'POST', '/vehicles/v1/reminders/r1/complete', { completed_date: 'd' }],
    ['getReminderTemplates', () => api.getReminderTemplates(), 'GET', '/reminder-templates/'],
    ['createReminderTemplate', () => api.createReminderTemplate({ name: 'n', from_vehicle_id: 'v1' }), 'POST', '/reminder-templates/', { name: 'n', from_vehicle_id: 'v1' }],
    ['deleteReminderTemplate', () => api.deleteReminderTemplate('t1'), 'DELETE', '/reminder-templates/t1'],
    ['applyReminderTemplate', () => api.applyReminderTemplate('v1', 't1'), 'POST', '/vehicles/v1/reminders/apply-template', { template_id: 't1' }],
    ['getVehicleWebhook', () => api.getVehicleWebhook('v1'), 'GET', '/vehicles/v1/webhook'],
    ['saveVehicleWebhook', () => api.saveVehicleWebhook('v1', { url: 'u' }), 'PUT', '/vehicles/v1/webhook', { url: 'u' }],
    ['deleteVehicleWebhook', () => api.deleteVehicleWebhook('v1'), 'DELETE', '/vehicles/v1/webhook'],
    ['testVehicleWebhook', () => api.testVehicleWebhook('v1', { url: 'u' }), 'POST', '/vehicles/v1/webhook/test', { url: 'u' }],
    ['getAPITokens', () => api.getAPITokens(), 'GET', '/auth/tokens'],
    ['createAPIToken', () => api.createAPIToken({ name: 'ha' }), 'POST', '/auth/tokens', { name: 'ha' }],
    ['revokeAPIToken', () => api.revokeAPIToken('k1'), 'DELETE', '/auth/tokens/k1'],
    ['getMileageRates', () => api.getMileageRates(), 'GET', '/mileage-rates/'],
    ['createMileageRate', () => api.createMileageRate({ label: 'l', year: 2026, from_km: 0, to_km: null, rate_per_km: 0.5 }), 'POST', '/mileage-rates/', { label: 'l', year: 2026, from_km: 0, to_km: null, rate_per_km: 0.5 }],
    ['deleteMileageRate', () => api.deleteMileageRate('r1'), 'DELETE', '/mileage-rates/r1'],
    ['getTariffPlans', () => api.getTariffPlans(), 'GET', '/tariffs/plans'],
    ['createTariffPlan', () => api.createTariffPlan({ name: 'n' }), 'POST', '/tariffs/plans', { name: 'n' }],
    ['updateTariffPlan', () => api.updateTariffPlan('p1', { name: 'm' }), 'PUT', '/tariffs/plans/p1', { name: 'm' }],
    ['deleteTariffPlan', () => api.deleteTariffPlan('p1'), 'DELETE', '/tariffs/plans/p1'],
    ['calculateSessionCost', () => api.calculateSessionCost({ start_time: 's', end_time: 'e', kwh: 5 }), 'POST', '/tariffs/calculate-session', { start_time: 's', end_time: 'e', kwh: 5 }],
    ['getPublicPresets', () => api.getPublicPresets(), 'GET', '/tariffs/public-presets'],
    ['createPublicPreset', () => api.createPublicPreset({ name: 'n' }), 'POST', '/tariffs/public-presets', { name: 'n' }],
    ['deletePublicPreset', () => api.deletePublicPreset('p1'), 'DELETE', '/tariffs/public-presets/p1'],
    ['getPendingCharges', () => api.getPendingCharges(), 'GET', '/pending-charges'],
    ['assignPendingCharge', () => api.assignPendingCharge('c1', { vehicle_id: 'v1' }), 'POST', '/pending-charges/c1/assign', { vehicle_id: 'v1' }],
    ['deletePendingCharge', () => api.deletePendingCharge('c1'), 'DELETE', '/pending-charges/c1'],
    ['getFleetSummary', () => api.getFleetSummary(), 'GET', '/fleet/summary'],
    ['setFleetBudget', () => api.setFleetBudget(null), 'PUT', '/fleet/budget', { amount: null }],
  ]

  it.each(cases)('%s', async (_name, call, method, path, body) => {
    await call()
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe(`/api${path}`)
    expect(init.method ?? 'GET').toBe(method)
    if (body !== undefined) expect(JSON.parse(init.body)).toEqual(body)
    else expect(init.body).toBeUndefined()
  })
})
