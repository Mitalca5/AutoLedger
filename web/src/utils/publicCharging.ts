import type { PublicChargingCalculationRequest, PublicChargingRates } from '@/services/api'

type NumericInput = number | ''
export interface PublicChargingFields {
  connectionFee: NumericInput
  costPerKwh: NumericInput
  costPerMinute: NumericInput
  idleFeePerMinute: NumericInput
  idleGraceMinutes: NumericInput
}

/** The API accepts monetary values in currency units; it converts them to cents internally. */
export function publicChargingRates(fields: PublicChargingFields): PublicChargingRates {
  return {
    connection_fee: Number(fields.connectionFee),
    price_per_kwh: Number(fields.costPerKwh),
    price_per_minute: Number(fields.costPerMinute),
    idle_fee_per_minute: Number(fields.idleFeePerMinute),
    idle_grace_minutes: Number(fields.idleGraceMinutes),
  }
}

export function publicChargingRequest(fields: PublicChargingFields, kwh: NumericInput, chargingMinutes: NumericInput, idleMinutes: NumericInput): PublicChargingCalculationRequest {
  return {
    ...publicChargingRates(fields),
    kwh: Number(kwh),
    charging_minutes: Number(chargingMinutes),
    total_plugged_minutes: Number(chargingMinutes) + Number(idleMinutes),
  }
}
