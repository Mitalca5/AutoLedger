import { watch } from 'vue'
import { api } from '@/services/api'

// An odometer field is filled for the user while it is empty, or still holds what was filled last.
export function canPrefillOdometer(current: number | string | null | undefined, lastFilled: number | null): boolean {
  if (current === '' || current === null || current === undefined) return true
  const n = Number(current)
  return !n || (lastFilled !== null && n === lastFilled)
}

interface Options {
  vehicleId: () => string | undefined
  // The form is shown and the date field applies (a mounted tire with no removal has no removal date).
  enabled: () => boolean
  date: () => string | undefined
  current: () => number | string | null | undefined
  fill: (km: number) => void
}

// Prefills an odometer field with the estimate for the date next to it (a linear spread between the known
// readings, as in the monthly mileage), never replacing what the user typed.
export function useOdometerPrefill(opts: Options) {
  let lastFilled: number | null = null
  let request = 0

  watch(
    () => [opts.enabled(), opts.date(), opts.vehicleId()] as const,
    async ([enabled, date, vehicleId]) => {
      if (!enabled || !date || !vehicleId) return
      if (!canPrefillOdometer(opts.current(), lastFilled)) return
      const mine = ++request
      try {
        const res = await api.getOdometerEstimate(vehicleId, date)
        if (mine !== request || typeof res?.odometer !== 'number' || res.odometer <= 0) return
        if (!canPrefillOdometer(opts.current(), lastFilled)) return
        lastFilled = res.odometer
        opts.fill(res.odometer)
      } catch {
        // The field stays as it is: the estimate is only a convenience.
      }
    },
    { flush: 'post' },
  )

  // Call when the form is seeded; a seeded value counts as filled for the user when it is `seed`.
  return {
    reset(seed: number | null = null) {
      lastFilled = seed
      request++
    },
  }
}
