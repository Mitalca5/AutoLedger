import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const updateLanguage = vi.fn()
const getMe = vi.fn()
const login = vi.fn()

vi.mock('@/services/api', () => ({ api: { updateLanguage: (l: string) => updateLanguage(l), getMe: () => getMe(), login: (c: unknown) => login(c) } }))
vi.mock('@/i18n', () => ({ currentLocale: () => 'fr' }))
vi.mock('@/units', () => ({ setDistanceUnit: vi.fn() }))

describe('account language sync', () => {
  beforeEach(() => {
    vi.resetModules()
    setActivePinia(createPinia())
    updateLanguage.mockReset().mockResolvedValue({})
    getMe.mockReset()
    login.mockReset()
  })

  it('pushes the UI language when the account copy differs', async () => {
    const { syncAccountLanguage } = await import('./auth')
    syncAccountLanguage({ language: 'en' })
    expect(updateLanguage).toHaveBeenCalledWith('fr')
  })

  it('does nothing when the copies already match or there is no user', async () => {
    const { syncAccountLanguage } = await import('./auth')
    syncAccountLanguage({ language: 'fr' })
    syncAccountLanguage(null)
    expect(updateLanguage).not.toHaveBeenCalled()
  })

  it('syncs on session restore and on login', async () => {
    const { useAuthStore } = await import('./auth')
    const store = useAuthStore()
    getMe.mockResolvedValue({ language: 'en' })
    await store.init()
    expect(updateLanguage).toHaveBeenCalledTimes(1)
    login.mockResolvedValue({ user: { language: 'en' } })
    await store.login({ email: 'a@b.c', password: 'x' })
    expect(updateLanguage).toHaveBeenCalledTimes(2)
  })

  it('ignores a failing language update', async () => {
    updateLanguage.mockRejectedValue(new Error('offline'))
    const { syncAccountLanguage } = await import('./auth')
    expect(() => syncAccountLanguage({ language: 'en' })).not.toThrow()
  })
})
