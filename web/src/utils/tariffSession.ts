import { toNumber } from '@/utils/quickAdd'

export interface SessionPriceRequest {
  vehicle_id: string
  start_time: string
  end_time: string
  kwh: number
}

const instant = (local: string): Date | null => {
  if (!local) return null
  const d = new Date(local)
  return Number.isNaN(d.getTime()) ? null : d
}

/**
 * The tariff query of a charge typed in a form (local date-time strings), or null while it cannot be priced: no
 * energy yet or no valid start. An end that is missing or not after the start prices the session at its start.
 * The server reads the instants in its own timezone, so a session crossing midnight or a time zone change is
 * priced hour by hour.
 */
export function sessionPriceRequest(vehicleId: string, start: string, end: string, kwh: string): SessionPriceRequest | null {
  const energy = toNumber(kwh)
  const from = instant(start)
  if (!vehicleId || !from || energy === null || energy <= 0) return null
  const to = instant(end)
  return {
    vehicle_id: vehicleId,
    start_time: from.toISOString(),
    end_time: (to && to > from ? to : from).toISOString(),
    kwh: energy,
  }
}
