import { describe, expect, it } from 'vitest'
import {
  buildVehiclePayload,
  emptyTeslaMateForm,
  supportsTeslaMate,
  teslaMateCredentials,
  type OnboardingVehicleForm,
} from './onboarding'

function form(overrides: Partial<OnboardingVehicleForm> = {}): OnboardingVehicleForm {
  return {
    name: 'Model 3',
    make: 'Tesla',
    model: 'Model 3',
    vin: '',
    powertrain: 'EV',
    odometer: '15000',
    teslamate: emptyTeslaMateForm(),
    ...overrides,
  }
}

describe('emptyTeslaMateForm', () => {
  it('does not preselect TeslaMate', () => {
    expect(emptyTeslaMateForm().enabled).toBe(false)
  })
})

describe('supportsTeslaMate', () => {
  it('only applies to electric vehicles', () => {
    expect(supportsTeslaMate('EV')).toBe(true)
    expect(supportsTeslaMate('ICE')).toBe(false)
  })
})

describe('teslaMateCredentials', () => {
  const base = { apiKey: 'k', user: 'u', pass: 'p' }
  it('sends the token for BEARER', () => {
    expect(teslaMateCredentials({ ...base, authType: 'BEARER' })).toEqual({ teslamate_api_key: 'k' })
  })
  it('sends user and password for BASIC', () => {
    expect(teslaMateCredentials({ ...base, authType: 'BASIC' })).toEqual({
      teslamate_basic_user: 'u',
      teslamate_basic_pass: 'p',
    })
  })
  it('sends nothing without authentication', () => {
    expect(teslaMateCredentials({ ...base, authType: 'NONE' })).toEqual({})
  })
})

describe('buildVehiclePayload', () => {
  it('never sends telemetry_mode', () => {
    expect(buildVehiclePayload(form())).not.toHaveProperty('telemetry_mode')
  })

  it('creates a source-less vehicle by default', () => {
    const p = buildVehiclePayload(form({ vin: '' }))
    expect(p).toMatchObject({ name: 'Model 3', powertrain: 'EV', current_odometer: 15000, teslamate_auth_type: 'NONE' })
    expect(p.vin).toBeUndefined()
    expect(p).not.toHaveProperty('teslamate_api_url')
  })

  it('falls back to a zero odometer when the field is empty or invalid', () => {
    expect(buildVehiclePayload(form({ odometer: '' })).current_odometer).toBe(0)
    expect(buildVehiclePayload(form({ odometer: 'abc' })).current_odometer).toBe(0)
  })

  it('adds the TeslaMate connection only when enabled with a URL', () => {
    const tm = { ...emptyTeslaMateForm(), enabled: true, url: 'http://tm:8080', authType: 'BEARER' as const, apiKey: 'k' }
    expect(buildVehiclePayload(form({ teslamate: tm }))).toMatchObject({
      teslamate_api_url: 'http://tm:8080',
      teslamate_car_id: 1,
      teslamate_auth_type: 'BEARER',
      teslamate_api_key: 'k',
    })
    expect(buildVehiclePayload(form({ teslamate: { ...tm, url: '  ' } }))).not.toHaveProperty('teslamate_api_url')
    expect(buildVehiclePayload(form({ teslamate: { ...tm, enabled: false } }))).not.toHaveProperty('teslamate_api_url')
  })

  it('ignores a TeslaMate form on a combustion vehicle', () => {
    const tm = { ...emptyTeslaMateForm(), enabled: true, url: 'http://tm:8080' }
    const p = buildVehiclePayload(form({ powertrain: 'ICE', teslamate: tm }))
    expect(p).not.toHaveProperty('teslamate_api_url')
    expect(p.teslamate_auth_type).toBe('NONE')
  })

  it('keeps the VIN when given', () => {
    expect(buildVehiclePayload(form({ vin: 'VIN123' })).vin).toBe('VIN123')
  })

  it('sends the chosen currency, and none when unset', () => {
    expect(buildVehiclePayload(form({ currency: 'GBP' })).currency).toBe('GBP')
    expect(buildVehiclePayload(form())).not.toHaveProperty('currency')
  })
})
