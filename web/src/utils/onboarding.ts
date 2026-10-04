import { canLinkTeslaMate } from './vehicles'

export type Powertrain = 'EV' | 'ICE' | 'PHEV' | 'REEV'
export type TeslaMateAuthType = 'BEARER' | 'BASIC' | 'NONE'

export interface TeslaMateForm {
  enabled: boolean
  url: string
  authType: TeslaMateAuthType
  apiKey: string
  user: string
  pass: string
}

export interface OnboardingVehicleForm {
  name: string
  make: string
  model: string
  vin: string
  powertrain: Powertrain
  odometer: number | string
  currency?: string
  teslamate: TeslaMateForm
}

export function emptyTeslaMateForm(): TeslaMateForm {
  return { enabled: false, url: '', authType: 'NONE', apiKey: '', user: '', pass: '' }
}

// Only electric vehicles can use TeslaMate.
export function supportsTeslaMate(powertrain: Powertrain): boolean {
  return canLinkTeslaMate(powertrain)
}

export function teslaMateCredentials(tm: Pick<TeslaMateForm, 'authType' | 'apiKey' | 'user' | 'pass'>) {
  if (tm.authType === 'BEARER') return { teslamate_api_key: tm.apiKey }
  if (tm.authType === 'BASIC') return { teslamate_basic_user: tm.user, teslamate_basic_pass: tm.pass }
  return {}
}

// telemetry_mode is derived by the server from the connected sources, never sent by the client.
export function buildVehiclePayload(form: OnboardingVehicleForm): Record<string, unknown> {
  const tm = form.teslamate
  const withTeslaMate = supportsTeslaMate(form.powertrain) && tm.enabled && tm.url.trim() !== ''
  const payload: Record<string, unknown> = {
    name: form.name,
    powertrain: form.powertrain,
    make: form.make,
    model: form.model,
    vin: form.vin || undefined,
    current_odometer: Number(form.odometer) || 0,
    teslamate_auth_type: withTeslaMate ? tm.authType : 'NONE',
  }
  if (form.currency) payload.currency = form.currency
  if (withTeslaMate) {
    payload.teslamate_api_url = tm.url
    payload.teslamate_car_id = 1
    Object.assign(payload, teslaMateCredentials(tm))
  }
  return payload
}
