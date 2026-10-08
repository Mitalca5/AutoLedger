import { ref, watch, type Ref } from 'vue'
import { toNumber, totalFromUnitPrice, unitPriceText } from '@/utils/quickAdd'

// The field edited last decides the other: a unit price gives the cost for the energy, a cost gives the price paid.
// 'tariff' is the state before any edit, where the price is only a suggestion that follows the energy.
export type AmountDriver = 'tariff' | 'price' | 'cost'

/** Keeps a charge's energy, unit price and total cost consistent. `kwh` and `cost` are the caller's form fields. */
export function useChargeAmounts(kwh: Ref<string>, cost: Ref<string>, suggestedPrice?: number, tariff?: Ref<number | null>) {
  const hasCost = cost.value !== ''
  const driver = ref<AmountDriver>(hasCost ? 'cost' : 'tariff')
  const price = ref(hasCost ? unitPriceText(toNumber(cost.value), toNumber(kwh.value), 4) : suggestedPrice ? String(suggestedPrice) : '')

  // The vehicle's tariff priced the session: it replaces the suggestion until the user types a price or a cost
  if (tariff) {
    watch(tariff, (total) => {
      if (driver.value !== 'tariff') return
      if (total === null) {
        // Only the remembered suggestion can be reused; the previous plan's effective rate cannot.
        price.value = suggestedPrice !== undefined ? String(suggestedPrice) : ''
        const fallback = totalFromUnitPrice(toNumber(kwh.value), toNumber(price.value))
        cost.value = fallback === null ? '' : fallback.toFixed(2)
        return
      }
      cost.value = total.toFixed(2)
      price.value = unitPriceText(total, toNumber(kwh.value), 4)
    }, { immediate: true, flush: 'sync' })
  }

  watch(kwh, (value) => {
    const energy = toNumber(value)
    if (driver.value === 'cost') {
      price.value = unitPriceText(toNumber(cost.value), energy, 4)
      return
    }
    if (driver.value === 'tariff' && tariff?.value != null) return
    const total = totalFromUnitPrice(energy, toNumber(price.value))
    cost.value = total === null ? '' : total.toFixed(2)
  })

  function onPriceInput() {
    driver.value = 'price'
    const total = totalFromUnitPrice(toNumber(kwh.value), toNumber(price.value))
    cost.value = total === null ? '' : total.toFixed(2)
  }

  function onCostInput() {
    driver.value = 'cost'
    price.value = unitPriceText(toNumber(cost.value), toNumber(kwh.value), 4)
  }

  // Costs applied by another UI (such as the public calculator) are explicit too.
  function setCost(total: number) {
    driver.value = 'cost'
    cost.value = total.toFixed(2)
    price.value = unitPriceText(total, toNumber(kwh.value), 4)
  }

  function setFree() {
    driver.value = 'cost'
    cost.value = '0'
    price.value = '0'
  }

  return { price, driver, onPriceInput, onCostInput, setCost, setFree }
}
