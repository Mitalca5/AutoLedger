import { describe, expect, it } from 'vitest'
import {
  reminderPresets,
  countUnlistedDrives,
  currencyPayload,
  defaultAmortizationMode,
  recentDescriptions,
  findCloseCandidate,
  formatFileSize,
  expensesOfKind,
  filterMaintenance,
  isFixedCost,
  groupChargesByMonth,
  hasReminderSchedule,
  reminderDueTile,
  nextOccurrenceDate,
  reminderDaysLabel,
  reminderDaysOver,
  formatCalendarDay,
  maintenanceStartPoint,
  maintenanceTotal,
  maintenanceYears,
  sortMaintenanceByDate,
  toLocalDateTimeInput,
  categoryStyle,
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
  it('gives every preset a title and a due point: an interval or a fixed day', () => {
    for (const p of reminderPresets()) {
      expect(p.title).not.toBe('')
      expect(p.interval_km !== '' || p.interval_months !== '' || (!!p.scheduled_month && !!p.scheduled_day)).toBe(true)
    }
  })

  it('offers seasonal tire changes on a fixed day', () => {
    const seasonal = reminderPresets().filter((p) => p.scheduled_month)
    expect(seasonal.map((p) => [p.scheduled_month, p.scheduled_day])).toEqual([[11, 1], [4, 1]])
    expect(seasonal.every((p) => p.category === 'TIRES')).toBe(true)
  })
})

describe('nextOccurrenceDate', () => {
  it('stays in the current year while the day has not passed, today included', () => {
    expect(nextOccurrenceDate(11, 1, new Date(2026, 9, 5))).toBe('2026-11-01')
    expect(nextOccurrenceDate(11, 1, new Date(2026, 10, 1))).toBe('2026-11-01')
  })

  it('rolls over to next year once it has passed', () => {
    expect(nextOccurrenceDate(4, 1, new Date(2026, 9, 5))).toBe('2027-04-01')
    expect(nextOccurrenceDate(11, 1, new Date(2026, 10, 2))).toBe('2027-11-01')
  })
})

describe('reminder calendar labels', () => {
  it('reads a fixed date reached today as today, not overdue', () => {
    expect(reminderDaysOver({ remaining_days: 0, scheduled_date: '2026-11-01' })).toBe(false)
    expect(reminderDaysLabel({ remaining_days: 0, scheduled_date: '2026-11-01' })).toBe("Aujourd'hui")
  })

  it('counts an interval reaching its due day as over, like the backend status', () => {
    expect(reminderDaysOver({ remaining_days: 0 })).toBe(true)
    expect(reminderDaysOver({ remaining_days: -1, scheduled_date: '2026-11-01' })).toBe(true)
    expect(reminderDaysOver({ remaining_days: 3, scheduled_date: '2026-11-01' })).toBe(false)
  })

  it('shows a calendar day without timezone drift', () => {
    expect(formatCalendarDay('2026-11-01T00:00:00Z', false)).toMatch(/1/)
    expect(formatCalendarDay('2026-11-01T00:00:00Z')).toMatch(/2026/)
  })
})

describe('hasReminderSchedule', () => {
  it('needs a mileage or a calendar interval to have a due point', () => {
    expect(hasReminderSchedule({})).toBe(false)
    expect(hasReminderSchedule({ interval_km: 0, interval_months: null })).toBe(false)
    expect(hasReminderSchedule({ interval_km: 15000 })).toBe(true)
    expect(hasReminderSchedule({ interval_months: 12 })).toBe(true)
    expect(hasReminderSchedule({ interval_km: 15000, interval_months: 12 })).toBe(true)
    expect(hasReminderSchedule({ scheduled_date: '2026-11-01T00:00:00Z' })).toBe(true)
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

describe('maintenance form defaults', () => {
  it('spreads only accessories over the distance by default', () => {
    expect(defaultAmortizationMode('ACCESSORY')).toBe('DISTANCE')
    expect(defaultAmortizationMode('MAINTENANCE')).toBe('NONE')
    expect(defaultAmortizationMode('INSURANCE')).toBe('NONE')
  })

  it('suggests the latest distinct descriptions, newest first and capped', () => {
    const items = [
      { description: 'Vidange', date: '2025-01-01' },
      { description: 'Pneus', date: '2026-02-01' },
      { description: ' Vidange ', date: '2026-03-01' },
      { description: '', date: '2026-04-01' },
      { description: null, date: '2026-05-01' },
    ]
    expect(recentDescriptions(items)).toEqual(['Vidange', 'Pneus'])
    expect(recentDescriptions(items, 1)).toEqual(['Vidange'])
  })
})

describe('groupChargesByMonth', () => {
  const at = (y: number, m: number, d: number) => new Date(y, m - 1, d, 12).toISOString()
  const charges = [
    { date: at(2026, 3, 20), kwh_added: 10, cost: 4, currency: 'EUR' },
    { date: at(2026, 3, 2), kwh_added: 30, cost: 12, currency: 'EUR' },
    { date: at(2026, 3, 1), kwh_added: 5, cost: null, currency: 'EUR' },
    { date: at(2026, 2, 27), kwh_added: 20, cost: 10, currency: 'USD', fx_rate: 0.5 },
  ]

  it('groups by month in order and totals energy and cost', () => {
    const months = groupChargesByMonth(charges, 'EUR')
    expect(months.map((m) => m.key)).toEqual(['2026-03', '2026-02'])
    expect(months[0].charges).toHaveLength(3)
    expect(months[0].kwh).toBe(45)
    expect(months[0].cost).toBe(16)
    expect(months[0].withoutCost).toBe(1)
  })

  it('averages the price over the charges that have a cost, converting foreign amounts', () => {
    const [march, february] = groupChargesByMonth(charges, 'EUR')
    expect(march.pricePerKwh).toBeCloseTo(16 / 40)
    expect(february.cost).toBe(5)
    expect(february.pricePerKwh).toBeCloseTo(0.25)
  })

  it('has no average price when nothing has a cost, and no month for no charge', () => {
    expect(groupChargesByMonth([{ date: at(2026, 1, 5), kwh_added: 8, cost: null }], 'EUR')[0].pricePerKwh).toBeNull()
    expect(groupChargesByMonth([], 'EUR')).toEqual([])
  })
})

describe('reminder maintenance link helpers', () => {
  it('sorts recorded maintenance from the most recent', () => {
    const items = [
      { id: 'a', date: '2026-01-05T00:00:00Z' },
      { id: 'c', date: '2026-09-01T00:00:00Z' },
      { id: 'b', date: '2026-03-10T00:00:00Z' },
    ]
    expect(sortMaintenanceByDate(items).map((m) => m.id)).toEqual(['c', 'b', 'a'])
    expect(items[0].id).toBe('a')
  })

  it('takes the day and the rounded odometer as starting point', () => {
    expect(maintenanceStartPoint({ id: 'a', date: '2026-03-10T00:00:00Z', odometer: 24000.4 })).toEqual({ date: '2026-03-10', odometer: 24000 })
  })

  it('leaves the odometer empty when the maintenance has none', () => {
    expect(maintenanceStartPoint({ id: 'a', date: '2026-03-10T00:00:00Z', odometer: null })).toEqual({ date: '2026-03-10', odometer: '' })
    expect(maintenanceStartPoint({ id: 'a', date: '2026-03-10T00:00:00Z' }).odometer).toBe('')
    expect(maintenanceStartPoint({ id: 'a', date: '2026-03-10T00:00:00Z', odometer: 0 }).odometer).toBe(0)
  })
})

describe('fixed costs', () => {
  const items = [
    { id: 1, category: 'MAINTENANCE' },
    { id: 2, category: 'INSURANCE' },
    { id: 3, category: 'REPAIR' },
    { id: 4, category: 'TAX' },
    { id: 5, category: 'SUBSCRIPTION' },
    { id: 6, category: 'FINANCING' },
    { id: 7, category: 'ACCESSORY' },
    { id: 8, category: 'OTHER' },
  ]

  it('recognises insurance, tax, subscription and financing', () => {
    expect(items.filter((m) => isFixedCost(m.category)).map((m) => m.id)).toEqual([2, 4, 5, 6])
  })

  it('splits the expenses between servicing and fixed costs', () => {
    expect(expensesOfKind(items, 'service').map((m) => m.id)).toEqual([1, 3, 7, 8])
    expect(expensesOfKind(items, 'fixed').map((m) => m.id)).toEqual([2, 4, 5, 6])
  })
})

describe('reminderDueTile', () => {
  it('prefers the due date, then the due mileage', () => {
    expect(reminderDueTile({ due_date: '2026-11-01T00:00:00Z', due_odometer: 90000 })).toMatchObject({ kind: 'date', day: 1 })
    expect(reminderDueTile({ due_odometer: 90000 })).toEqual({ kind: 'km', km: 90000 })
    expect(reminderDueTile({})).toBeNull()
  })
})

describe('categoryStyle', () => {
  it('gives each known category its own colour and falls back to neutral', () => {
    const known = ['INSURANCE', 'SUBSCRIPTION', 'TAX', 'FINANCING', 'MAINTENANCE', 'REPAIR', 'ACCESSORY'].map((c) => categoryStyle(c).accent)
    expect(new Set(known).size).toBe(known.length)
    expect(categoryStyle('OTHER').accent).toBe('border-l-slate-600')
  })
})
