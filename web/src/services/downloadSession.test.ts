import { afterEach, expect, it, vi } from 'vitest'
import { api } from './api'

afterEach(() => vi.unstubAllGlobals())

it.each(['export', 'service-book', 'document'])('renews an expired session before downloading %s', async (kind) => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(new Response('{"error":"expired"}', { status: 401, headers: { 'content-type': 'application/json' } }))
    .mockResolvedValueOnce(new Response('{}', { status: 200 }))
    .mockResolvedValueOnce(new Response('file contents', { headers: { 'content-disposition': 'attachment; filename="file.pdf"' } }))
  vi.stubGlobal('fetch', fetchMock)
  const result = kind === 'export' ? await api.downloadExport('v1', { type: 'drives', format: 'csv' })
    : kind === 'service-book' ? await api.downloadServiceBook('v1', {}) : await api.downloadDocumentBlob('v1', 'd1')
  expect(result.filename).toBe('file.pdf')
  expect(await result.blob.text()).toBe('file contents')
  expect(fetchMock.mock.calls[1][0]).toBe('/api/auth/refresh')
  expect(fetchMock.mock.calls[2]).toEqual(fetchMock.mock.calls[0])
})
