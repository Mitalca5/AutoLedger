import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { api } from '@/services/api'
import { currentLocale } from '@/i18n'
import { setDistanceUnit } from '@/units'

// The access/refresh tokens live in HttpOnly cookies set by the API — this store never
// holds a token value, only whether the current cookie-backed session is valid.
type AuthStatus = 'unknown' | 'authenticated' | 'unauthenticated'

// Reminder and sync-failure webhooks are built outside any request, so the server keeps its own copy
// of the language. The UI's language is the source: push it whenever the account's copy differs.
export function syncAccountLanguage(user: { language?: string } | null | undefined) {
  const locale = currentLocale()
  if (user && user.language !== locale) api.updateLanguage(locale).catch(() => {})
}

export const useAuthStore = defineStore('auth', () => {
  const status = ref<AuthStatus>('unknown')
  const user = ref<any | null>(null)
  const isAuthenticated = computed(() => status.value === 'authenticated')

  // Resolves the session status once by asking the API (cookies are sent automatically).
  // Safe to call multiple times: only the first call (per page load) actually hits the network.
  async function init() {
    if (status.value !== 'unknown') return
    try {
      user.value = await api.getMe()
      setDistanceUnit(user.value?.distance_unit)
      status.value = 'authenticated'
      syncAccountLanguage(user.value)
    } catch {
      user.value = null
      status.value = 'unauthenticated'
    }
  }

  async function login(credentials: { email: string; password: string }) {
    const res = await api.login(credentials)
    user.value = res.user
    setDistanceUnit(user.value?.distance_unit)
    status.value = 'authenticated'
    syncAccountLanguage(user.value)
  }

  async function register(payload: { email: string; password: string }) {
    const res = await api.register(payload)
    user.value = res.user
    setDistanceUnit(user.value?.distance_unit)
    status.value = 'authenticated'
    syncAccountLanguage(user.value)
  }

  async function logout() {
    try {
      await api.logout()
    } catch {
      // Ignorer les erreurs réseau lors de la déconnexion
    }
    user.value = null
    status.value = 'unauthenticated'
    setDistanceUnit(null)
  }

  return {
    status,
    user,
    isAuthenticated,
    init,
    login,
    register,
    logout,
  }
})
