import { computed, onScopeDispose, ref, watch } from 'vue'
import { api } from '@/services/api'
import { comparablePriceHistory, type ComparisonPriceHistory } from '@/utils/energyPriceWarning'

export function useComparisonPriceHistory(vehicleId: () => string | undefined, fuelType: () => string, currency: () => string) {
  const history = ref<ComparisonPriceHistory | null>(null)
  let sequence = 0
  watch(() => [vehicleId(), fuelType(), currency()], async () => {
    const current = ++sequence
    history.value = null
    const id = vehicleId()
    if (!id) return
    try {
      const result = await api.getComparisonPriceHistory(id, fuelType())
      if (current === sequence) history.value = result
    } catch {
      // An unavailable advisory reference must not prevent exploring a valid scenario.
    }
  }, { immediate: true })
  onScopeDispose(() => { sequence += 1 })
  return computed(() => comparablePriceHistory(history.value, currency()))
}
