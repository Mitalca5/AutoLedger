import { describe, expect, it } from 'vitest'
import { chargeDeletable, chargeFieldAccess, chargeOrigin, chargeOriginKey } from './chargeProvenance'

describe('chargeOrigin', () => {
  it('uses the reported origin', () => {
    expect(chargeOrigin({ origin: 'WEBHOOK', is_manual: true })).toBe('WEBHOOK')
    expect(chargeOrigin({ origin: 'csv' })).toBe('CSV')
  })
  it('falls back on is_manual for a charge without origin', () => {
    expect(chargeOrigin({ is_manual: true })).toBe('MANUAL')
    expect(chargeOrigin({ is_manual: false })).toBe('TESLAMATE')
    expect(chargeOrigin({ origin: 'SOMETHING' })).toBe('TESLAMATE')
  })
  it('names the badge key after the origin', () => {
    expect(chargeOriginKey({ origin: 'WEBHOOK' })).toBe('expenses.chargeOrigin.WEBHOOK')
  })
})

describe('chargeFieldAccess', () => {
  it('locks the reported facts of a TeslaMate charge only', () => {
    expect(chargeFieldAccess({ origin: 'TESLAMATE' }).sourceLocked).toBe(true)
    expect(chargeFieldAccess({ origin: 'WEBHOOK', is_manual: true }).sourceLocked).toBe(false)
    expect(chargeFieldAccess({ origin: 'CSV' }).sourceLocked).toBe(false)
    expect(chargeFieldAccess({ origin: 'MANUAL' }).sourceLocked).toBe(false)
  })
  it('leaves a new charge fully editable', () => {
    expect(chargeFieldAccess(null).sourceLocked).toBe(false)
  })
})

describe('chargeDeletable', () => {
  it('keeps synchronised charges', () => {
    expect(chargeDeletable({ origin: 'TESLAMATE' })).toBe(false)
    expect(chargeDeletable({ origin: 'WEBHOOK' })).toBe(true)
    expect(chargeDeletable({ origin: 'CSV' })).toBe(true)
  })
})
