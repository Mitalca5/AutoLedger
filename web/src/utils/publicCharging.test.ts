import { describe, expect, it } from 'vitest'
import { publicChargingRates, publicChargingRequest } from './publicCharging'

const fields = { connectionFee: 1, costPerKwh: 0.25, costPerMinute: 0.01, idleFeePerMinute: 0.1, idleGraceMinutes: 5 }

describe('public charging API payloads', () => {
  it('sends currency units and includes idle time in the total plugged duration', () => {
    expect(publicChargingRequest(fields, 20, 60, 15)).toEqual({
      connection_fee: 1, price_per_kwh: 0.25, price_per_minute: 0.01,
      idle_fee_per_minute: 0.1, idle_grace_minutes: 5,
      kwh: 20, charging_minutes: 60, total_plugged_minutes: 75,
    })
  })

  it('uses the same rates for presets and treats empty fields as zero fees', () => {
    const empty = { connectionFee: '', costPerKwh: '', costPerMinute: '', idleFeePerMinute: '', idleGraceMinutes: '' } as const
    expect(publicChargingRates(empty)).toEqual({
      connection_fee: 0, price_per_kwh: 0, price_per_minute: 0, idle_fee_per_minute: 0, idle_grace_minutes: 0,
    })
    expect(publicChargingRequest(empty, 10, '', '').total_plugged_minutes).toBe(0)
  })
})
