import { ref, watch, onBeforeUnmount, type Ref } from 'vue'
import { api } from '@/services/api'
import type { SessionPriceRequest } from '@/utils/tariffSession'

export interface SessionTariffCost {
  cost: number
  plan: string
}

/** The cost of a session under the vehicle's tariff, refreshed (debounced) when the query changes; null when no tariff prices it. */
export function useSessionTariffCost(query: () => SessionPriceRequest | null, enabled: Ref<boolean>) {
  const result = ref<SessionTariffCost | null>(null)
  let timer: ReturnType<typeof setTimeout> | undefined
  let latest = 0

  watch(
    () => (enabled.value ? JSON.stringify(query()) : 'null'),
    (key) => {
      clearTimeout(timer)
      const request: SessionPriceRequest | null = JSON.parse(key)
      const seq = ++latest
      if (!request) {
        result.value = null
        return
      }
      timer = setTimeout(async () => {
        try {
          const res = await api.calculateSessionCost(request)
          if (seq === latest) result.value = res.plan ? { cost: res.cost, plan: res.plan } : null
        } catch {
          if (seq === latest) result.value = null
        }
      }, 300)
    },
    { immediate: true },
  )
  onBeforeUnmount(() => clearTimeout(timer))
  return result
}
