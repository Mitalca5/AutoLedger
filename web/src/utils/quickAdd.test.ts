import { describe, expect, it } from 'vitest'
import {
  buildChargePayload,
  buildExpensePayload,
  buildFuelPayload,
  buildPendingCostPayload,
  costFromTariff,
  effectivePricePerKwh,
  isQueued,
  loadMemory,
  rememberCharge,
  toLocalDateInput,
  toLocalDateTimeInput,
  toNumber,
  type StorageLike,
} from './quickAdd'

function memoryStorage(initial: Record<string, string> = {}): StorageLike & { data: Record<string, string> } {
  const data = { ...initial }
  return {
    data,
    getItem: (k) => (k in data ? data[k] : null),
    setItem: (k, v) => {
      data[k] = v
    },
  }
}

describe('toNumber', () => {
  it('accepts a decimal comma as typed on a French keyboard', () => {
    expect(toNumber('12,5')).toBe(12.5)
    expect(toNumber(' 0,2276 ')).toBe(0.2276)
  })

  it('treats empty and invalid input as missing rather than zero', () => {
    expect(toNumber('')).toBeNull()
    expect(toNumber('   ')).toBeNull()
    expect(toNumber('abc')).toBeNull()
    expect(toNumber(undefined)).toBeNull()
    expect(toNumber(NaN)).toBeNull()
  })

  it('keeps a genuine zero', () => {
    expect(toNumber('0')).toBe(0)
    expect(toNumber(0)).toBe(0)
  })
})

describe('tariff helpers', () => {
  it('computes the cost of a charge at a known tariff, to the cent', () => {
    expect(costFromTariff(30, 0.3858)).toBe(11.57)
    expect(costFromTariff(32.4, 0.2276)).toBe(7.37)
  })

  it('gives no suggestion without a usable energy or tariff', () => {
    expect(costFromTariff(null, 0.3)).toBeNull()
    expect(costFromTariff(0, 0.3)).toBeNull()
    expect(costFromTariff(10, undefined)).toBeNull()
    expect(costFromTariff(10, 0)).toBeNull()
  })

  it('derives the price actually paid per kWh', () => {
    expect(effectivePricePerKwh(40, 12)).toBe(0.3)
    expect(effectivePricePerKwh(40, 0)).toBeNull() // free charge: no price to learn
    expect(effectivePricePerKwh(0, 5)).toBeNull()
    expect(effectivePricePerKwh(null, 5)).toBeNull()
  })
})

describe('local date formatting', () => {
  it('uses the local calendar day, not the UTC one', () => {
    const justAfterMidnight = new Date(2026, 8, 19, 0, 30) // 19 Sept, local
    expect(toLocalDateInput(justAfterMidnight)).toBe('2026-09-19')
    expect(toLocalDateTimeInput(justAfterMidnight)).toBe('2026-09-19T00:30')
  })

  it('zero pads month, day, hour and minute', () => {
    expect(toLocalDateTimeInput(new Date(2026, 0, 5, 7, 4))).toBe('2026-01-05T07:04')
  })
})

describe('buildChargePayload', () => {
  const base = { date: '2026-09-19T10:30', kwh: '30', cost: '11,57', address: '', odometer: '42123', notes: '', documentId: null }

  it('builds the API payload with euros and empty optionals as null', () => {
    const p = buildChargePayload(base, 'EUR')
    expect(p).toMatchObject({
      kwh_added: 30,
      cost: 11.57,
      currency: 'EUR',
      fx_rate: null,
      address: null,
      odometer: 42123,
      notes: null,
      document_id: null,
    })
    expect(new Date(p.date).getTime()).toBe(new Date('2026-09-19T10:30').getTime())
  })

  it('carries the vehicle\'s own currency, never a hard-coded EUR', () => {
    expect(buildChargePayload(base, 'USD').currency).toBe('USD')
  })

  it('requires a positive energy', () => {
    expect(() => buildChargePayload({ ...base, kwh: '' }, 'EUR')).toThrow(/énergie/i)
    expect(() => buildChargePayload({ ...base, kwh: '0' }, 'EUR')).toThrow(/énergie/i)
  })

  it('requires a cost but accepts 0 for a free charge', () => {
    expect(() => buildChargePayload({ ...base, cost: '' }, 'EUR')).toThrow(/coût/i)
    expect(() => buildChargePayload({ ...base, cost: '-1' }, 'EUR')).toThrow(/coût/i)
    expect(buildChargePayload({ ...base, cost: '0' }, 'EUR').cost).toBe(0)
  })

  it('drops a non positive odometer and trims text fields', () => {
    const p = buildChargePayload({ ...base, odometer: '0', address: '  Borne A ', notes: ' ', documentId: 'doc-1' }, 'EUR')
    expect(p.odometer).toBeNull()
    expect(p.address).toBe('Borne A')
    expect(p.notes).toBeNull()
    expect(p.document_id).toBe('doc-1')
  })
})

describe('buildFuelPayload', () => {
  const base = { date: '2026-09-19', amount: '62,4', liters: '38,5', fullTank: true, odometer: '', notes: '' }

  it('builds the fill-up with the optional fields left out when empty', () => {
    expect(buildFuelPayload(base)).toEqual({
      date: '2026-09-19',
      amount: 62.4,
      liters: 38.5,
      odometer: undefined,
      is_full_tank: true,
      notes: undefined,
    })
  })

  it('requires an amount', () => {
    expect(() => buildFuelPayload({ ...base, amount: '' })).toThrow(/montant/i)
    expect(() => buildFuelPayload({ ...base, amount: '0' })).toThrow(/montant/i)
  })

  it('accepts an amount alone and keeps a partial fill-up flag', () => {
    const p = buildFuelPayload({ ...base, liters: '', fullTank: false })
    expect(p.liters).toBeUndefined()
    expect(p.is_full_tank).toBe(false)
  })
})

describe('buildExpensePayload', () => {
  it('builds a toll payload', () => {
    const p = buildExpensePayload({ type: 'TOLL', date: '2026-09-19T10:30', amount: '8,4', notes: '', documentId: null }, 'EUR')
    expect(p).toMatchObject({ type: 'TOLL', amount: 8.4, currency: 'EUR', fx_rate: null, notes: '', document_id: null })
  })

  it('requires a positive amount', () => {
    expect(() => buildExpensePayload({ type: 'PARKING', date: '2026-09-19T10:30', amount: '0', notes: '', documentId: null }, 'EUR')).toThrow(/montant/i)
  })
})

describe('buildPendingCostPayload', () => {
  const charge = {
    id: 'c1',
    date: '2026-09-18T18:30:00Z',
    kwh_added: 32.4,
    address: 'Supercharger Lyon',
    odometer: 41000,
    notes: 'note existante',
    document_id: 'doc-9',
  }

  it('sends back the existing notes, attachment and TeslaMate values so the update wipes nothing', () => {
    expect(buildPendingCostPayload(charge, '12,5', 'EUR')).toEqual({
      date: '2026-09-18T18:30:00Z',
      kwh_added: 32.4,
      cost: 12.5,
      currency: 'EUR',
      fx_rate: null,
      address: 'Supercharger Lyon',
      odometer: 41000,
      notes: 'note existante',
      document_id: 'doc-9',
    })
  })

  it('normalises missing optionals to null', () => {
    const p = buildPendingCostPayload({ id: 'c2', date: '2026-09-18T18:30:00Z', kwh_added: 10 }, '0', 'EUR')
    expect(p).toMatchObject({ cost: 0, address: null, odometer: null, notes: null, document_id: null })
  })

  it('rejects a missing cost', () => {
    expect(() => buildPendingCostPayload(charge, '', 'EUR')).toThrow(/coût/i)
  })
})

describe('tariff memory', () => {
  it('starts empty', () => {
    expect(loadMemory('v1', memoryStorage())).toEqual({})
  })

  it('remembers the price per kWh and the place of a charge, per vehicle', () => {
    const storage = memoryStorage()
    rememberCharge('v1', { kwh: 40, cost: 12, address: 'Domicile' }, storage)
    expect(loadMemory('v1', storage)).toEqual({ pricePerKwh: 0.3, address: 'Domicile' })
    expect(loadMemory('v2', storage)).toEqual({})
  })

  it('keeps the previous tariff after a free charge and the previous place when none is given', () => {
    const storage = memoryStorage()
    rememberCharge('v1', { kwh: 40, cost: 12, address: 'Domicile' }, storage)
    rememberCharge('v1', { kwh: 25, cost: 0, address: null }, storage)
    expect(loadMemory('v1', storage)).toEqual({ pricePerKwh: 0.3, address: 'Domicile' })
  })

  it('reads a memory saved under the previous key and rewrites it under the new one', () => {
    const storage = memoryStorage({ 'teslacost.quickadd.v1': '{"pricePerKwh":0.25,"address":"Home"}' })
    expect(loadMemory('v1', storage)).toEqual({ pricePerKwh: 0.25, address: 'Home' })
    rememberCharge('v1', { kwh: 10, cost: 3, address: null }, storage)
    expect(JSON.parse(storage.data['autoledger.quickadd.v1'])).toEqual({ pricePerKwh: 0.3, address: 'Home' })
  })

  it('ignores corrupted or invalid stored values', () => {
    expect(loadMemory('v1', memoryStorage({ 'teslacost.quickadd.v1': '{not json' }))).toEqual({})
    expect(loadMemory('v1', memoryStorage({ 'teslacost.quickadd.v1': '{"pricePerKwh":-2,"address":"  "}' }))).toEqual({})
  })

  it('does not fail when storage is unavailable or refuses writes', () => {
    expect(loadMemory('v1', null)).toEqual({})
    expect(() => rememberCharge('v1', { kwh: 10, cost: 3, address: null }, null)).not.toThrow()
    const full: StorageLike = {
      getItem: () => null,
      setItem: () => {
        throw new Error('QuotaExceededError')
      },
    }
    expect(() => rememberCharge('v1', { kwh: 10, cost: 3, address: null }, full)).not.toThrow()
  })
})

describe('isQueued', () => {
  it('recognises the answer of a mutation stored for later', () => {
    expect(isQueued({ queued: true })).toBe(true)
  })

  it('does not mistake a server response for a queued one', () => {
    expect(isQueued({ id: 'x' })).toBe(false)
    expect(isQueued({ queued: false })).toBe(false)
    expect(isQueued(null)).toBe(false)
    expect(isQueued(undefined)).toBe(false)
  })
})
