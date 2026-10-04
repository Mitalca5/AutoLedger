import { describe, expect, it } from 'vitest'
import { formatAmount } from '@/currency'
import {
  canCharge,
  canLinkTeslaMate,
  canRefuel,
  isElectricOnly,
  isFuelOnly,
  emptyOwnership,
  emptyVehicleForm,
  hasTeslaMate,
  isLeaseType,
  isOwnedPhase,
  isPurchaseType,
  leasePreview,
  loanPreview,
  nextTeslaMateCarId,
  nullIfEmpty,
  ownershipFormFrom,
  ownershipPayload,
  ownershipStepError,
  ownershipSummary,
  toDateInput,
  vehicleFormFrom,
  defaultDriver,
} from './vehicles'

const form = (over: Record<string, any> = {}) => ({ ...emptyOwnership(), ...over })

describe('nextTeslaMateCarId', () => {
  it('returns 1 when no vehicles exist or list is empty', () => {
    expect(nextTeslaMateCarId()).toBe(1)
    expect(nextTeslaMateCarId(null)).toBe(1)
    expect(nextTeslaMateCarId([])).toBe(1)
  })

  it('ignores null, undefined or non-positive IDs', () => {
    expect(nextTeslaMateCarId([{ teslamate_car_id: null }, { teslamate_car_id: undefined }, { teslamate_car_id: 0 }, { teslamate_car_id: -1 }])).toBe(1)
  })

  it('increments from max existing ID', () => {
    expect(nextTeslaMateCarId([{ teslamate_car_id: 1 }])).toBe(2)
    expect(nextTeslaMateCarId([{ teslamate_car_id: 1 }, { teslamate_car_id: 2 }])).toBe(3)
    expect(nextTeslaMateCarId([{ teslamate_car_id: 2 }, { teslamate_car_id: 5 }])).toBe(6)
  })
})

describe('vehicle form', () => {
  it('starts blank, electric, with no TeslaMate settings', () => {
    const f = emptyVehicleForm()
    expect(f.name).toBe('')
    expect(f.powertrain).toBe('EV')
    expect(f.teslamate_auth_type).toBe('NONE')
    expect(f.estimated_kwh_100km).toBeNull()
    expect(f.teslamate_car_id).toBe(1)
  })

  it('auto-increments teslamate_car_id when existing vehicles are provided', () => {
    const f1 = emptyVehicleForm([{ teslamate_car_id: 1 }])
    expect(f1.teslamate_car_id).toBe(2)

    const f2 = emptyVehicleForm([{ teslamate_car_id: 1 }, { teslamate_car_id: 2 }])
    expect(f2.teslamate_car_id).toBe(3)
  })

  it('fills an existing vehicle, rounding the odometer and never restoring secrets', () => {
    const f = vehicleFormFrom({ name: 'M3', current_odometer: 1234.6, teslamate_api_url: 'http://x', teslamate_auth_type: 'BASIC', teslamate_basic_user: 'me', estimated_kwh_100km: 15 })
    expect(f.current_odometer).toBe(1235)
    expect(f.teslamate_api_key).toBe('')
    expect(f.teslamate_basic_pass).toBe('')
    expect(f.teslamate_basic_user).toBe('me')
    expect(f.estimated_price_per_kwh).toBeNull()
  })

  it('falls back to defaults for a bare vehicle', () => {
    const f = vehicleFormFrom({ name: 'Clio' })
    expect(f.powertrain).toBe('EV')
    expect(f.current_odometer).toBe(0)
    expect(f.teslamate_car_id).toBe(1)
    expect(f.teslamate_auth_type).toBe('NONE')
  })
})

describe('hasTeslaMate', () => {
  it('needs an electric vehicle with a teslamateapi URL', () => {
    expect(hasTeslaMate({ powertrain: 'EV', teslamate_api_url: 'http://tm:8080' })).toBe(true)
    expect(hasTeslaMate({ teslamate_api_url: 'http://tm:8080' })).toBe(true)
  })

  it('is off without a usable URL', () => {
    expect(hasTeslaMate({ powertrain: 'EV' })).toBe(false)
    expect(hasTeslaMate({ powertrain: 'EV', teslamate_api_url: null })).toBe(false)
    expect(hasTeslaMate({ powertrain: 'EV', teslamate_api_url: '   ' })).toBe(false)
  })

  it('is off for a combustion vehicle and when there is no vehicle', () => {
    expect(hasTeslaMate({ powertrain: 'ICE', teslamate_api_url: 'http://tm:8080' })).toBe(false)
    expect(hasTeslaMate(null)).toBe(false)
    expect(hasTeslaMate(undefined)).toBe(false)
  })
})

describe('helpers', () => {
  it('turns empty inputs into null but keeps zero and false', () => {
    expect(nullIfEmpty('')).toBeNull()
    expect(nullIfEmpty(undefined)).toBeNull()
    expect(nullIfEmpty(0)).toBe(0)
    expect(nullIfEmpty('abc')).toBe('abc')
  })

  it('formats dates for date inputs', () => {
    expect(toDateInput('2026-05-02T10:00:00Z')).toBe('2026-05-02')
    expect(toDateInput(null)).toBe('')
    expect(toDateInput(undefined)).toBe('')
  })
})

describe('contract form', () => {
  it('starts a new contract at the vehicle odometer', () => {
    expect(ownershipFormFrom(null, { current_odometer: 1500.4 }).start_odometer).toBe(1500)
    expect(ownershipFormFrom(null, {}).start_odometer).toBeNull()
  })

  it('fills the saved contract with date inputs and defaults for missing fields', () => {
    const f = ownershipFormFrom({ acquisition_type: 'LOAN', start_date: '2024-03-01T00:00:00Z', purchase_price: 45000, option_exercised_date: null }, {})
    expect(f.start_date).toBe('2024-03-01')
    expect(f.option_exercised_date).toBe('')
    expect(f.purchase_price).toBe(45000)
    expect(f.lease_includes_tires).toBe(false)
  })

  it('sends empty numeric fields as null and keeps booleans', () => {
    const payload = ownershipPayload(form({ purchase_price: '' as any, loan_rate_pct: 4.5, lease_includes_tires: true, end_date: '' }))
    expect(payload.purchase_price).toBeNull()
    expect(payload.end_date).toBeNull()
    expect(payload.loan_rate_pct).toBe(4.5)
    expect(payload.lease_includes_tires).toBe(true)
    expect(payload.lease_includes_insurance).toBe(false)
  })
})

describe('contract types', () => {
  it('tells purchases from leases', () => {
    expect(isPurchaseType({ acquisition_type: 'CASH' })).toBe(true)
    expect(isPurchaseType({ acquisition_type: 'LOAN' })).toBe(true)
    expect(isPurchaseType({ acquisition_type: 'LOA' })).toBe(false)
    expect(isLeaseType({ acquisition_type: 'LLD' })).toBe(true)
    expect(isLeaseType({ acquisition_type: 'CASH' })).toBe(false)
  })

  it('owns the car once bought, or once a LOA purchase option is exercised', () => {
    expect(isOwnedPhase({ acquisition_type: 'CASH', option_exercised_date: '' })).toBe(true)
    expect(isOwnedPhase({ acquisition_type: 'LOA', option_exercised_date: '' })).toBe(false)
    expect(isOwnedPhase({ acquisition_type: 'LOA', option_exercised_date: '2026-05-01' })).toBe(true)
    expect(isOwnedPhase({ acquisition_type: 'LLD', option_exercised_date: '2026-05-01' })).toBe(false)
  })
})

describe('loanPreview', () => {
  it('needs an amount and a duration', () => {
    expect(loanPreview(form())).toBeNull()
    expect(loanPreview(form({ loan_amount: 10000 }))).toBeNull()
  })

  it('splits a zero-rate loan evenly', () => {
    const p = loanPreview(form({ loan_amount: 12000, loan_duration_months: 24, loan_rate_pct: 0 }))!
    expect(p.payment).toBe(500)
    expect(p.totalInterest).toBe(0)
    expect(p.totalCost).toBe(0)
  })

  it('computes the annuity, the interest and the total with insurance and fees', () => {
    const p = loanPreview(form({ loan_amount: 30000, loan_duration_months: 60, loan_rate_pct: 4.5, loan_insurance_monthly: 20, loan_fees: 300 }))!
    expect(p.payment).toBeCloseTo(559.3, 1)
    expect(p.totalInterest).toBeCloseTo(p.payment * 60 - 30000, 6)
    expect(p.totalCost).toBeCloseTo(p.totalInterest + 20 * 60 + 300, 6)
  })
})

describe('leasePreview', () => {
  it('needs a rent and a duration', () => {
    expect(leasePreview(form())).toBeNull()
    expect(leasePreview(form({ lease_monthly_rent: 300 }))).toBeNull()
  })

  it('adds the down payment and fees to the rents and derives the allowance', () => {
    const p = leasePreview(form({ lease_monthly_rent: 300, lease_duration_months: 36, lease_down_payment: 2000, lease_fees: 100, lease_end_fees_estimate: 400, lease_km_allowance_per_year: 15000 }))!
    expect(p.total).toBe(300 * 36 + 2000 + 100 + 400)
    expect(p.perMonth).toBeCloseTo(p.total / 36)
    expect(p.totalKm).toBe(45000)
  })
})

describe('ownershipSummary', () => {
  it('describes a purchase and a lease, and nothing without a contract', () => {
    expect(ownershipSummary(null)).toBeNull()
    expect(ownershipSummary({ acquisition_type: 'LOAN', purchase_price: 45000 })).toContain('Achat à crédit')
    expect(ownershipSummary({ acquisition_type: 'CASH', purchase_price: 45000 })).toContain('Achat comptant')
    expect(ownershipSummary({ acquisition_type: 'LOA', lease_monthly_rent: 389, lease_duration_months: 36 })).toBe(`LOA • ${formatAmount(389, 'EUR', 0)}/mois sur 36 mois`)
    expect(ownershipSummary({ acquisition_type: 'CASH', purchase_price: 45000 }, 'USD')).toContain(formatAmount(45000, 'USD', 0))
  })
})

describe('ownershipStepError', () => {
  it('needs a start date on step 1', () => {
    expect(ownershipStepError(form({ start_date: '' }), 1)).toContain('date de début')
    expect(ownershipStepError(form({ acquisition_type: '' }), 1)).toContain("mode d'acquisition")
    expect(ownershipStepError(form(), 1)).toBeNull()
  })

  it('needs a purchase price for a purchase', () => {
    expect(ownershipStepError(form({ acquisition_type: 'CASH', purchase_price: null }), 2)).toContain("prix d'achat")
    expect(ownershipStepError(form({ acquisition_type: 'CASH', purchase_price: 0 }), 2)).toContain("prix d'achat")
    expect(ownershipStepError(form({ acquisition_type: 'CASH', purchase_price: 30000 }), 2)).toBeNull()
  })

  it('needs an amount and a duration for a loan', () => {
    const base = { acquisition_type: 'LOAN', purchase_price: 30000 }
    expect(ownershipStepError(form(base), 2)).toContain('montant emprunté')
    expect(ownershipStepError(form({ ...base, loan_amount: 20000 }), 2)).toContain('durée du crédit')
    expect(ownershipStepError(form({ ...base, loan_amount: 20000, loan_duration_months: 48 }), 2)).toBeNull()
  })

  it('needs a rent and a duration for a lease, and accepts a zero rent', () => {
    expect(ownershipStepError(form({ acquisition_type: 'LOA' }), 2)).toContain('loyer mensuel')
    expect(ownershipStepError(form({ acquisition_type: 'LLD', lease_monthly_rent: 0 }), 2)).toContain('durée de la location')
    expect(ownershipStepError(form({ acquisition_type: 'LLD', lease_monthly_rent: 0, lease_duration_months: 24 }), 2)).toBeNull()
    expect(ownershipStepError(form({ acquisition_type: 'LOA', lease_monthly_rent: -5, lease_duration_months: 24 }), 2)).toContain('loyer mensuel')
  })

  it('has nothing to check on the last step', () => {
    expect(ownershipStepError(form({ acquisition_type: 'CASH' }), 3)).toBeNull()
  })
})

describe('powertrain capabilities', () => {
  it('maps each powertrain to what it records', () => {
    expect([canCharge('EV'), canRefuel('EV'), canLinkTeslaMate('EV')]).toEqual([true, false, true])
    expect([canCharge('ICE'), canRefuel('ICE'), canLinkTeslaMate('ICE')]).toEqual([false, true, false])
    expect([canCharge(undefined), canRefuel(undefined)]).toEqual([true, false])
    expect([canCharge('PHEV'), canRefuel('PHEV'), canLinkTeslaMate('PHEV')]).toEqual([true, true, true])
    expect([canCharge('REEV'), canRefuel('REEV')]).toEqual([true, true])
    expect(['EV', 'ICE', 'PHEV'].map((p) => [isElectricOnly(p), isFuelOnly(p)])).toEqual([[true, false], [false, true], [false, false]])
  })
})

describe('default driver', () => {
  const owner = { id: 'o', name: 'Rémy', is_default: true }
  const guest = { id: 'g', name: 'Léa', is_default: false }

  it('picks the chosen default driver', () => {
    expect(defaultDriver([owner, guest], 'g')).toBe(guest)
  })

  it('falls back to the person the server flags as default', () => {
    expect(defaultDriver([guest, owner], null)).toBe(owner)
    expect(defaultDriver([guest, owner], 'zzz')).toBe(owner)
  })

  it('has no default driver without people', () => {
    expect(defaultDriver([guest], null)).toBeUndefined()
    expect(defaultDriver([], 'o')).toBeUndefined()
  })
})
