import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

function fakeStorage(initial: Record<string, string> = {}) {
  const data = { ...initial }
  return {
    getItem: (k: string) => (k in data ? data[k] : null),
    setItem: (k: string, v: string) => {
      data[k] = v
    },
  }
}

describe('preferences store', () => {
  beforeEach(() => {
    vi.resetModules()
    setActivePinia(createPinia())
  })

  it('has the work/personal classification on by default', async () => {
    vi.stubGlobal('localStorage', fakeStorage())
    const { usePreferencesStore } = await import('./preferences')
    expect(usePreferencesStore().proPersoEnabled).toBe(true)
  })

  it('remembers turning it off across reloads', async () => {
    const storage = fakeStorage()
    vi.stubGlobal('localStorage', storage)
    const { usePreferencesStore } = await import('./preferences')
    usePreferencesStore().setProPersoEnabled(false)
    expect(usePreferencesStore().proPersoEnabled).toBe(false)

    setActivePinia(createPinia())
    vi.resetModules()
    const reloaded = await import('./preferences')
    expect(reloaded.usePreferencesStore().proPersoEnabled).toBe(false)
  })

  it('works where no storage is available', async () => {
    vi.stubGlobal('localStorage', undefined)
    const { usePreferencesStore } = await import('./preferences')
    const prefs = usePreferencesStore()
    expect(prefs.proPersoEnabled).toBe(true)
    prefs.setProPersoEnabled(false)
    expect(prefs.proPersoEnabled).toBe(false)
  })

  it('shows the last year of the dashboard charts by default', async () => {
    vi.stubGlobal('localStorage', fakeStorage())
    const { usePreferencesStore } = await import('./preferences')
    expect(usePreferencesStore().dashboardRange).toBe('1Y')
  })

  it('remembers the dashboard period across reloads', async () => {
    const storage = fakeStorage()
    vi.stubGlobal('localStorage', storage)
    const { usePreferencesStore } = await import('./preferences')
    usePreferencesStore().setDashboardRange('6M')

    setActivePinia(createPinia())
    vi.resetModules()
    const reloaded = await import('./preferences')
    expect(reloaded.usePreferencesStore().dashboardRange).toBe('6M')
  })

  it('ignores a stored dashboard period that no longer exists', async () => {
    vi.stubGlobal('localStorage', fakeStorage({ teslacost_dashboard_range: '10Y' }))
    const { usePreferencesStore } = await import('./preferences')
    expect(usePreferencesStore().dashboardRange).toBe('1Y')
  })
})
