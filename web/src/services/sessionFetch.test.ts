import { afterEach, expect, it, vi } from 'vitest'
import { fetchWithSessionRefresh } from './sessionFetch'

const json = (status: number) => new Response('{}', { status, headers: { 'content-type': 'application/json' } })
afterEach(() => vi.unstubAllGlobals())

it('refreshes once and preserves a mutation body and idempotency key on retry', async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(json(401)).mockResolvedValueOnce(json(200)).mockResolvedValueOnce(json(201))
  vi.stubGlobal('fetch', fetchMock)
  const options = { method: 'POST', body: '{"cost":0}', headers: { 'Idempotency-Key': 'original-key' } }
  expect((await fetchWithSessionRefresh('/api/vehicles/v1/charges', options)).status).toBe(201)
  expect(fetchMock.mock.calls[1][0]).toBe('/api/auth/refresh')
  expect(fetchMock.mock.calls[2]).toEqual(fetchMock.mock.calls[0])
})

it('stops after one retry if the refreshed request is still unauthorized', async () => {
  const fetchMock = vi.fn().mockResolvedValueOnce(json(401)).mockResolvedValueOnce(json(200)).mockResolvedValueOnce(json(401))
  vi.stubGlobal('fetch', fetchMock)
  expect((await fetchWithSessionRefresh('/api/vehicles')).status).toBe(401)
  expect(fetchMock).toHaveBeenCalledTimes(3)
})

it('does not retry when refresh fails or intercept public authentication requests', async () => {
  const fetchMock = vi.fn().mockResolvedValue(json(401))
  vi.stubGlobal('fetch', fetchMock)
  expect((await fetchWithSessionRefresh('/api/vehicles')).status).toBe(401)
  expect(fetchMock).toHaveBeenCalledTimes(2)
  fetchMock.mockClear()
  await fetchWithSessionRefresh('/api/auth/login', { method: 'POST' })
  expect(fetchMock).toHaveBeenCalledTimes(1)
})
