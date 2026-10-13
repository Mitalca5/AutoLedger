import { computed, ref, watch } from 'vue'
import { isUnusualEnergyPrice, type EnergyPriceReference } from '@/utils/energyPriceWarning'

// Confirmation is limited to this value and reference. Changing either asks again.
export function useEnergyPriceWarning(price: () => number, reference: () => EnergyPriceReference | undefined, context: () => string) {
  const confirmed = ref(false)
  const unusual = computed(() => isUnusualEnergyPrice(price(), reference()))
  watch(() => [price(), reference()?.count, reference()?.median, context()], () => { confirmed.value = false }, { flush: 'sync' })
  return {
    unusual,
    needsConfirmation: computed(() => unusual.value && !confirmed.value),
    confirm: () => { confirmed.value = true },
  }
}
