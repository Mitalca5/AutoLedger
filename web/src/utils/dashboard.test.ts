import { describe, expect, it } from 'vitest'
import { formatAmount } from '@/currency'
import {
  buildLeaseSummary,
  buildMonthBreakdown,
  computeMonthFixedVariable,
  currentMonthStats,
  filterMonthsByRange,
  formatMonthName,
  nextDueReminder,
  reminderProgress,
  sortRemindersByUrgency,
  summarizeUrgentReminders,
  groupRemindersByDue,
} from './dashboard'

describe('filterMonthsByRange', () => {
  const list = Array.from({ length: 14 }, (_, i) => ({ month: i + 1 }))

  it('keeps everything for ALL and the last N months otherwise', () => {
    expect(filterMonthsByRange(list, 'ALL')).toHaveLength(14)
    expect(filterMonthsByRange(list, '1Y').map((m) => m.month)[0]).toBe(3)
    expect(filterMonthsByRange(list, '6M')).toHaveLength(6)
    expect(filterMonthsByRange(list, '3M').map((m) => m.month)).toEqual([12, 13, 14])
    expect(filterMonthsByRange(list, '1M')).toEqual([{ month: 14 }])
  })

  it('does not fail on a short or empty list', () => {
    expect(filterMonthsByRange([{ month: 1 }], '1Y')).toHaveLength(1)
    expect(filterMonthsByRange([], '3M')).toEqual([])
  })
})

describe('formatMonthName', () => {
  it('capitalizes the French month name', () => {
    expect(formatMonthName('2026-05')).toBe('Mai 2026')
    expect(formatMonthName('2025-12')).toBe('Décembre 2025')
  })

  it('returns blanks and malformed input as they are', () => {
    expect(formatMonthName('')).toBe('')
    expect(formatMonthName('2026')).toBe('2026')
  })
})

describe('currentMonthStats', () => {
  const list = [
    { month: '2026-04', distance_km: 800, cost_per_km: 0.3, total: 300 },
    { month: '2026-05', distance_km: 900, cost_per_km: 0.4, total: 400 },
  ]

  it('picks the current month with the previous distance', () => {
    const s = currentMonthStats(list, new Date('2026-05-15T10:00:00Z'))!
    expect(s.month).toBe('2026-05')
    expect(s.prevDistance).toBe(800)
    expect(s.total).toBe(400)
    expect(s.raw).toBe(list[1])
  })

  it('falls back to the latest month when the current one has no row', () => {
    expect(currentMonthStats(list, new Date('2026-09-01T10:00:00Z'))!.month).toBe('2026-05')
  })

  it('has no previous distance for the first month and nothing without data', () => {
    expect(currentMonthStats(list, new Date('2026-04-10T10:00:00Z'))!.prevDistance).toBeNull()
    expect(currentMonthStats([], new Date())).toBeNull()
    expect(currentMonthStats(undefined, new Date())).toBeNull()
  })
})

describe('summarizeUrgentReminders', () => {
  const r = (title: string, status: string) => ({ title, status })

  it('lists overdue and due-soon reminders and flags overdue ones', () => {
    const s = summarizeUrgentReminders([r('A', 'OVERDUE'), r('B', 'DUE_SOON'), r('C', 'OK')])
    expect(s.urgent.map((x) => x.title)).toEqual(['A', 'B'])
    expect(s.hasOverdue).toBe(true)
    expect(s.summary).toBe('A, B')
  })

  it('names two and counts the others', () => {
    const s = summarizeUrgentReminders([r('A', 'OVERDUE'), r('B', 'DUE_SOON'), r('C', 'DUE_SOON'), r('D', 'DUE_SOON')])
    expect(s.summary).toBe('A, B et 2 autre(s)')
  })

  it('is empty when nothing is urgent', () => {
    expect(summarizeUrgentReminders([r('A', 'OK')])).toEqual({ urgent: [], hasOverdue: false, summary: '' })
  })
})

describe('buildLeaseSummary', () => {
  const lease = {
    acquisition_type: 'LOA',
    contract_start_date: '2025-03-01T00:00:00Z',
    contract_end_date: '2028-03-01T00:00:00Z',
    contract_duration_months: 36,
    lease_km_driven: 14500,
    lease_km_allowance_to_date: 12500,
    lease_km_allowance_total: 45000,
    lease_km_allowance_per_year: 15000,
    lease_monthly_rent: 389,
  }
  const now = new Date('2026-06-15T10:00:00Z')

  it('applies to LOA and LLD contracts with a duration only', () => {
    expect(buildLeaseSummary(null, now)).toBeNull()
    expect(buildLeaseSummary({ acquisition_type: 'CASH' }, now)).toBeNull()
    expect(buildLeaseSummary({ acquisition_type: 'LOA' }, now)).toBeNull()
    expect(buildLeaseSummary(lease, now)).not.toBeNull()
    expect(buildLeaseSummary({ ...lease, acquisition_type: 'LLD' }, now)).not.toBeNull()
  })

  it('tracks progress through the contract', () => {
    const s = buildLeaseSummary(lease, now)!
    expect(s.totalMonths).toBe(36)
    expect(s.elapsedMonths).toBeGreaterThan(14)
    expect(s.elapsedMonths).toBeLessThan(17)
    expect(s.remainingMonths).toBeGreaterThan(19)
    expect(s.durationProgressPct).toBeGreaterThan(40)
    expect(s.durationProgressPct).toBeLessThan(50)
    expect(s.isEnded).toBe(false)
    expect(s.status.label).toBe('En cours')
  })

  it('compares mileage with the allowance to date', () => {
    const s = buildLeaseSummary(lease, now)!
    expect(s.hasMileageAllowance).toBe(true)
    expect(s.kmDiff).toBe(2000)
    expect(s.mileageProgressPct).toBe(32)
    expect(s.mileageColor).toContain('danger')
    expect(s.contractualPaceKmMonth).toBe(1250)
  })

  it('warns when the mileage is close to the allowance but not over it', () => {
    const s = buildLeaseSummary({ ...lease, lease_km_driven: 12000 }, now)!
    expect(s.mileageColor).toContain('warning')
    expect(buildLeaseSummary({ ...lease, lease_km_driven: 9000 }, now)!.mileageColor).toContain('success')
  })

  it('flags an ended contract, an approaching end, an unstarted contract and an exercised option', () => {
    expect(buildLeaseSummary(lease, new Date('2029-01-01T00:00:00Z'))!.status.label).toBe('Terminé')
    expect(buildLeaseSummary(lease, new Date('2028-01-15T00:00:00Z'))!.status.label).toBe('Échéance proche')
    const early = buildLeaseSummary(lease, new Date('2025-01-01T00:00:00Z'))!
    expect(early.isNotStarted).toBe(true)
    expect(early.durationProgressPct).toBe(0)
    expect(buildLeaseSummary({ ...lease, option_exercised_date: '2026-05-01' }, now)!.status.label).toBe("Option d'achat levée")
  })

  it('derives the end date from the duration when none is set', () => {
    const s = buildLeaseSummary({ ...lease, contract_end_date: null }, now)!
    expect(s.endDate!.getFullYear()).toBe(2028)
  })
})

describe('buildMonthBreakdown', () => {
  const month = {
    month: '2026-03', distance_km: 1000, tracked_distance_km: 900, smoothed_km: 100, cost_per_km: 0.5,
    energy: 50, smoothed_energy: 5, tolls: 10, tires: 400, tires_amortized: 20, maintenance: 0, maintenance_amortized: 15,
    insurance: 60, financing: 300, financing_amortized: 310, other: 5, total: 825,
  }

  it('totals the smoothed cost in the economic view and what was paid in the cash view', () => {
    const eco = buildMonthBreakdown(month, 'economic')
    expect(eco.economicTotal).toBe(50 + 10 + 20 + 15 + 60 + 310 + 5)
    expect(eco.cashTotal).toBe(825)
    expect(eco.activeTotal).toBe(eco.economicTotal)
    const cash = buildMonthBreakdown(month, 'cash')
    expect(cash.activeTotal).toBe(825)
    expect(cash.items.find((i) => i.key === 'tires')!.displayAmount).toBe(400)
    expect(eco.items.find((i) => i.key === 'tires')!.displayAmount).toBe(20)
  })

  it('gives each item its share and cost per km', () => {
    const eco = buildMonthBreakdown(month, 'economic')
    const energy = eco.items.find((i) => i.key === 'energy')!
    expect(energy.costPerKm).toBeCloseTo(0.05)
    expect(energy.sharePct).toBeCloseTo((50 / eco.economicTotal) * 100)
    expect(eco.items.reduce((s, i) => s + i.sharePct, 0)).toBeCloseTo(100)
  })

  it('explains estimates, cash outlays and smoothing in the item notes', () => {
    const eco = buildMonthBreakdown(month, 'economic')
    expect(eco.items.find((i) => i.key === 'energy')!.note).toContain(`${formatAmount(5, 'EUR')} estimés`)
    expect(eco.items.find((i) => i.key === 'tires')!.note).toContain(`${formatAmount(400, 'EUR')} décaissés`)
    expect(eco.items.find((i) => i.key === 'financing')!.note).toContain(`Lissé : ${formatAmount(310, 'EUR')}`)
    expect(eco.items.find((i) => i.key === 'tolls')!.note).toBeNull()
  })

  it('writes the notes in the vehicle currency', () => {
    const usd = buildMonthBreakdown(month, 'economic', 'USD')
    expect(usd.items.find((i) => i.key === 'energy')!.note).toContain(formatAmount(5, 'USD'))
    expect(usd.items.find((i) => i.key === 'energy')!.note).not.toContain('€')
  })

  it('has zero cost per km and shares without distance or spending', () => {
    const empty = buildMonthBreakdown({ month: '2026-01' }, 'economic')
    expect(empty.economicTotal).toBe(0)
    expect(empty.costPerKm).toBe(0)
    expect(empty.items.every((i) => i.sharePct === 0 && i.costPerKm === 0)).toBe(true)
    expect(empty.fixedVar.totalAmount).toBe(0)
    expect(empty.fixedVar.fixedPct).toBe(0)
    expect(empty.fixedVar.variablePct).toBe(0)
  })

  it('computes fixed vs variable ratio on the monthly breakdown', () => {
    const eco = buildMonthBreakdown(month, 'economic')
    expect(eco.fixedVar.variableAmount).toBe(95)
    expect(eco.fixedVar.fixedAmount).toBe(375)
    expect(eco.fixedVar.totalAmount).toBe(470)
    expect(eco.fixedVar.fixedPct).toBe(80)
    expect(eco.fixedVar.variablePct).toBe(20)

    const cash = buildMonthBreakdown(month, 'cash')
    expect(cash.fixedVar.variableAmount).toBe(460)
    expect(cash.fixedVar.fixedAmount).toBe(365)
    expect(cash.fixedVar.totalAmount).toBe(825)
    expect(cash.fixedVar.fixedPct).toBe(44)
    expect(cash.fixedVar.variablePct).toBe(56)
  })
})

describe('computeMonthFixedVariable', () => {
  it('returns zeroes for empty or null month', () => {
    const res = computeMonthFixedVariable(null)
    expect(res).toEqual({ fixedAmount: 0, variableAmount: 0, totalAmount: 0, fixedPct: 0, variablePct: 0 })
    expect(computeMonthFixedVariable({})).toEqual({ fixedAmount: 0, variableAmount: 0, totalAmount: 0, fixedPct: 0, variablePct: 0 })
  })

  it('splits variable and fixed costs in economic mode', () => {
    const m = {
      energy: 80,
      tolls: 20,
      tires_amortized: 30,
      maintenance_amortized: 20,
      insurance: 50,
      financing_amortized: 200,
      other: 0,
    }
    const res = computeMonthFixedVariable(m, 'economic')
    expect(res.variableAmount).toBe(150)
    expect(res.fixedAmount).toBe(250)
    expect(res.totalAmount).toBe(400)
    expect(res.fixedPct).toBe(63) // 250 / 400 = 62.5 -> 63
    expect(res.variablePct).toBe(37) // 100 - 63 = 37
  })

  it('splits variable and fixed costs in cash mode', () => {
    const m = {
      energy: 80,
      tolls: 20,
      tires: 0,
      maintenance: 0,
      insurance: 50,
      financing: 200,
      other: 10,
    }
    const res = computeMonthFixedVariable(m, 'cash')
    expect(res.variableAmount).toBe(100)
    expect(res.fixedAmount).toBe(260)
    expect(res.totalAmount).toBe(360)
    expect(res.fixedPct + res.variablePct).toBe(100)
  })
})

describe('reminderProgress / nextDueReminder', () => {
  const base = { status: 'OK', interval_km: 10000, interval_months: 12, remaining_km: 8000, remaining_days: 300 }

  it('takes the closer of the mileage and calendar due points', () => {
    expect(reminderProgress({ ...base, remaining_km: 2000, remaining_days: 300 })).toBeCloseTo(0.8)
    expect(reminderProgress({ ...base, remaining_km: 8000, remaining_days: 30 })).toBeCloseTo(1 - 30 / (12 * 30.4375))
  })

  it('works with a single criterion and clamps to 0..1', () => {
    expect(reminderProgress({ status: 'OK', interval_km: 10000, remaining_km: 4000 })).toBeCloseTo(0.6)
    expect(reminderProgress({ status: 'OK', interval_km: 10000, remaining_km: 20000 })).toBe(0)
    expect(reminderProgress({ status: 'DUE_SOON', interval_km: 10000, remaining_km: -5 })).toBe(1)
  })

  it('measures a fixed date over the year leading to it, ignoring any months interval', () => {
    expect(reminderProgress({ status: 'OK', scheduled_date: '2026-11-01', remaining_days: 365.25 })).toBe(0)
    expect(reminderProgress({ status: 'OK', scheduled_date: '2026-11-01', remaining_days: 30 })).toBeCloseTo(1 - 30 / 365.25)
    expect(reminderProgress({ status: 'DUE_SOON', scheduled_date: '2026-11-01', remaining_days: 0 })).toBe(1)
  })

  it('has no progress without a schedule', () => {
    expect(reminderProgress({ status: 'OK' })).toBeNull()
    expect(reminderProgress({ status: 'OK', interval_km: 0, remaining_km: 5 })).toBeNull()
  })

  it('picks the reminder closest to its due point and skips overdue and unscheduled ones', () => {
    const list = [
      { title: 'late', ...base, status: 'OVERDUE', remaining_km: -10 },
      { title: 'none', status: 'OK' },
      { title: 'far', ...base, remaining_km: 9000 },
      { title: 'near', ...base, status: 'DUE_SOON', remaining_km: 500 },
    ]
    expect(nextDueReminder(list)!.reminder.title).toBe('near')
  })

  it('returns null when nothing qualifies', () => {
    expect(nextDueReminder([{ ...base, status: 'OVERDUE' }, { status: 'OK' }])).toBeNull()
    expect(nextDueReminder([])).toBeNull()
  })
})

describe('sortRemindersByUrgency', () => {
  const sched = { interval_km: 10000, interval_months: 12, remaining_days: 300 }

  it('puts overdue first, then due soon, then up to date, without mutating the input', () => {
    const list = [
      { title: 'ok', status: 'OK', ...sched, remaining_km: 9000 },
      { title: 'soon', status: 'DUE_SOON', ...sched, remaining_km: 500 },
      { title: 'late', status: 'OVERDUE', ...sched, remaining_km: -50 },
    ]
    expect(sortRemindersByUrgency(list).map((r) => r.title)).toEqual(['late', 'soon', 'ok'])
    expect(list[0].title).toBe('ok')
  })

  it('orders a status by progress and sends unscheduled reminders last', () => {
    const list = [
      { title: 'none', status: 'OK' },
      { title: 'far', status: 'OK', ...sched, remaining_km: 9000 },
      { title: 'near', status: 'OK', ...sched, remaining_km: 3000 },
    ]
    expect(sortRemindersByUrgency(list).map((r) => r.title)).toEqual(['near', 'far', 'none'])
  })
})

describe('groupRemindersByDue', () => {
  const base = { interval_km: 10000, interval_months: null, scheduled_date: null, remaining_km: null, due_date: null }
  it('splits by closeness, orders by due date and drops empty groups', () => {
    const groups = groupRemindersByDue([
      { id: 1, status: 'OK', ...base, due_date: '2027-03-01' },
      { id: 2, status: 'OK', ...base, due_date: '2026-12-01' },
      { id: 3, status: 'OVERDUE', ...base, due_date: '2026-09-01' },
      { id: 4, status: 'OK', ...base, interval_km: 0 },
      { id: 5, status: 'OK', ...base, due_date: null, remaining_km: 500 },
    ])
    expect(groups.map((g) => g.key)).toEqual(['overdue', 'later', 'unscheduled'])
    expect(groups[1].items.map((r) => r.id)).toEqual([2, 1, 5])
  })
})
