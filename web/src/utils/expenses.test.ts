import { describe, expect, it } from 'vitest'
import {
  reminderPresets,
  countUnlistedDrives,
  currencyPayload,
  findCloseCandidate,
  formatFileSize,
  filterMaintenance,
  hasReminderSchedule,
  maintenanceTotal,
  maintenanceYears,
  toLocalDateTimeInput,
} from './expenses'

describe('currencyPayload', () => {
  it('drops the rate for the vehicle\'s own currency whatever the form holds', () => {
    expect(currencyPayload({ currency: 'EUR', fx_rate: '1.2' }, 'EUR')).toEqual({ currency: 'EUR', fx_rate: null })
  })

  it('drops the rate for a non-EUR vehicle currency too', () => {
    expect(currencyPayload({ currency: 'USD', fx_rate: '1.2' }, 'USD')).toEqual({ currency: 'USD', fx_rate: null })
  })

  it('carries the rate of a foreign currency as a number', () => {
    expect(currencyPayload({ currency: 'CHF', fx_rate: '1.05' }, 'EUR')).toEqual({ currency: 'CHF', fx_rate: 1.05 })
  })

  it('leaves the rate null when a foreign currency has none yet', () => {
    expect(currencyPayload({ currency: 'USD', fx_rate: '' }, 'EUR')).toEqual({ currency: 'USD', fx_rate: null })
  })
})

describe('toLocalDateTimeInput', () => {
  it('formats local time for datetime-local inputs, zero padded', () => {
    expect(toLocalDateTimeInput(new Date(2026, 0, 5, 7, 3))).toBe('2026-01-05T07:03')
    expect(toLocalDateTimeInput(new Date(2026, 11, 25, 18, 45))).toBe('2026-12-25T18:45')
  })
})

describe('formatFileSize', () => {
  it('scales to the largest unit', () => {
    expect(formatFileSize(512)).toBe('512.0 o')
    expect(formatFileSize(2048)).toBe('2.0 Ko')
    expect(formatFileSize(1548576)).toBe('1.5 Mo')
  })

  it('shows zero for empty or invalid sizes', () => {
    expect(formatFileSize(0)).toBe('0 o')
    expect(formatFileSize(-3)).toBe('0 o')
  })
})

describe('countUnlistedDrives', () => {
  it('counts the selected drives missing from the listed ones', () => {
    expect(countUnlistedDrives(['a', 'b', 'x'], [{ id: 'a' }, { id: 'b' }, { id: 'c' }])).toBe(1)
    expect(countUnlistedDrives([], [{ id: 'a' }])).toBe(0)
  })
})

describe('findCloseCandidate', () => {
  const maints = [
    { id: 'm1', date: '2026-03-01T00:00:00Z', amortization_mode: 'NONE' },
    { id: 'm2', date: '2026-02-01T00:00:00Z', amortization_mode: 'DISTANCE' },
    { id: 'm3', date: '2026-06-01T00:00:00Z', amortization_mode: 'HYBRID' },
  ]

  it('takes the first amortized maintenance dated on or before the form date', () => {
    expect(findCloseCandidate(maints, null, '2026-04-01')?.id).toBe('m2')
    expect(findCloseCandidate(maints, null, '2026-06-01')?.id).toBe('m2')
  })

  it('ignores the maintenance being edited and unamortized ones', () => {
    expect(findCloseCandidate(maints, 'm2', '2026-04-01')).toBeNull()
  })

  it('finds nothing when every candidate is later than the form date, or the list is empty', () => {
    expect(findCloseCandidate(maints, null, '2026-01-01')).toBeNull()
    expect(findCloseCandidate([], null, '2026-04-01')).toBeNull()
    expect(findCloseCandidate(null, null, '2026-04-01')).toBeNull()
  })
})

describe('reminderPresets', () => {
  it('gives every preset a title and at least one interval', () => {
    for (const p of reminderPresets()) {
      expect(p.title).not.toBe('')
      expect(p.interval_km !== '' || p.interval_months !== '').toBe(true)
    }
  })
})

describe('hasReminderSchedule', () => {
  it('needs a mileage or a calendar interval to have a due point', () => {
    expect(hasReminderSchedule({})).toBe(false)
    expect(hasReminderSchedule({ interval_km: 0, interval_months: null })).toBe(false)
    expect(hasReminderSchedule({ interval_km: 15000 })).toBe(true)
    expect(hasReminderSchedule({ interval_months: 12 })).toBe(true)
    expect(hasReminderSchedule({ interval_km: 15000, interval_months: 12 })).toBe(true)
  })
})

describe('maintenance list helpers', () => {
  const items = [
    { category: 'MAINTENANCE', date: '2026-03-10T00:00:00Z', amount: 100, currency: 'EUR' },
    { category: 'INSURANCE', date: '2026-01-05T00:00:00Z', amount: '50.5', currency: 'EUR' },
    { category: 'MAINTENANCE', date: '2025-11-20T00:00:00Z', amount: 20, currency: 'USD', fx_rate: 0.9 },
  ]

  it('filters by category and by year, an empty value meaning any', () => {
    expect(filterMaintenance(items, { category: '', year: '' })).toHaveLength(3)
    expect(filterMaintenance(items, { category: 'MAINTENANCE', year: '' })).toHaveLength(2)
    expect(filterMaintenance(items, { category: '', year: '2026' })).toHaveLength(2)
    expect(filterMaintenance(items, { category: 'MAINTENANCE', year: '2025' })).toEqual([items[2]])
  })

  it('lists the years present, latest first', () => {
    expect(maintenanceYears(items)).toEqual([2026, 2025])
    expect(maintenanceYears([])).toEqual([])
  })

  it('totals in the vehicle currency, converting foreign amounts with their rate', () => {
    expect(maintenanceTotal(items, 'EUR')).toBeCloseTo(168.5)
    expect(maintenanceTotal([{ category: 'OTHER', date: '2026-01-01', amount: 10, currency: 'USD', fx_rate: null }], 'EUR')).toBe(0)
    expect(maintenanceTotal([], 'EUR')).toBe(0)
  })
})
