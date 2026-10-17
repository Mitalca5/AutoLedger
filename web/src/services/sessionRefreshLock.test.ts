import { afterEach, expect, it, vi } from 'vitest'
import { coordinateRefresh, refreshVersion } from './sessionRefreshLock'

afterEach(() => vi.unstubAllGlobals())

function browser(storage = true) {
  let pending = Promise.resolve<unknown>(undefined)
  const values = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => { if (!storage) throw new Error('blocked'); return values.get(key) ?? null },
    setItem: (key: string, value: string) => { if (!storage) throw new Error('blocked'); values.set(key, value) },
  })
  vi.stubGlobal('navigator', {
    locks: {
      request: (_name: string, callback: () => Promise<boolean>) => {
        const result = pending.then(callback)
        pending = result.catch(() => undefined)
        return result
      },
    },
  })
}

it('shares one successful refresh between independent tab modules', async () => {
  browser()
  const first = await import('./sessionFetch')
  vi.resetModules()
  const second = await import('./sessionFetch')
  let refreshed = false
  let refreshes = 0
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url === '/api/auth/refresh') {
      refreshes++
      await new Promise(resolve => setTimeout(resolve, 5))
      refreshed = true
      return new Response(null, { status: 200 })
    }
    return new Response(null, { status: refreshed ? 200 : 401 })
  }))
  const responses = await Promise.all([
    first.fetchWithSessionRefresh('/api/vehicles'),
    second.fetchWithSessionRefresh('/api/auth/me'),
  ])
  expect(responses.map(r => r.status)).toEqual([200, 200])
  expect(refreshes).toBe(1)
})

it('serializes requests even if browser storage is unavailable', async () => {
  browser(false)
  let running = 0
  let maximum = 0
  const refresh = vi.fn(async () => {
    maximum = Math.max(maximum, ++running)
    await new Promise(resolve => setTimeout(resolve, 5))
    running--
    return true
  })
  await Promise.all([coordinateRefresh(null, refresh), coordinateRefresh(null, refresh)])
  expect(refresh).toHaveBeenCalledTimes(2)
  expect(maximum).toBe(1)
})

it('does not announce a failed refresh as successful', async () => {
  browser()
  const refresh = vi.fn().mockResolvedValue(false)
  expect(await coordinateRefresh(refreshVersion(), refresh)).toBe(false)
  expect(refreshVersion()).toBeNull()
})

it('refreshes normally when no browser lock manager is available', async () => {
  vi.stubGlobal('navigator', {})
  const refresh = vi.fn().mockResolvedValue(true)
  expect(await coordinateRefresh(null, refresh)).toBe(true)
  expect(refresh).toHaveBeenCalledTimes(1)
})
