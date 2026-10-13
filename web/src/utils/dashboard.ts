import { intlLocale, t } from '@/i18n'
import { COST_COLORS } from '@/utils/costBreakdown'
import { formatAmount } from '@/currency'
import { distanceUnit, formatDistanceValue } from '@/units'
export const monthlyRangeOptions = [
  { key: 'ALL', labelKey: 'dashboard.range.all' },
  { key: '1Y', labelKey: 'dashboard.range.oneYear' },
  { key: '6M', labelKey: 'dashboard.range.sixMonths' },
  { key: '3M', labelKey: 'dashboard.range.threeMonths' },
  { key: '1M', labelKey: 'dashboard.range.oneMonth' },
] as const
export type MonthlyRangeKey = (typeof monthlyRangeOptions)[number]['key']

const monthlyRangeMonths: Record<MonthlyRangeKey, number | null> = {
  ALL: null,
  '1Y': 12,
  '6M': 6,
  '3M': 3,
  '1M': 1,
}

/** Number of calendar months of a range, the current one included; 0 for the whole history. */
export function rangeToMonths(range: MonthlyRangeKey): number {
  return monthlyRangeMonths[range] ?? 0
}

/** The last N months of the monthly costs for a chart range. */
export function filterMonthsByRange<T>(list: T[], range: MonthlyRangeKey): T[] {
  const months = monthlyRangeMonths[range]
  return months ? list.slice(-months) : list
}

export function formatMonthName(monthStr: string) {
  if (!monthStr) return ''
  const parts = monthStr.split('-')
  if (parts.length < 2) return monthStr
  const year = Number.parseInt(parts[0], 10)
  const month = Number.parseInt(parts[1], 10) - 1
  const d = new Date(year, month, 1)
  const formatted = d.toLocaleDateString(intlLocale(), { month: 'long', year: 'numeric' })
  return formatted.charAt(0).toUpperCase() + formatted.slice(1)
}

/**
 * The current month (or the latest one when it has no row yet) with the previous month's distance. The server names the
 * month in the reporting timezone; the clock of the browser is only the fallback.
 */
export function currentMonthStats(monthlyCosts: any[] | undefined, now = new Date(), currentMonth?: string) {
  if (!monthlyCosts?.length) return null
  const key = currentMonth || now.toISOString().substring(0, 7) // YYYY-MM
  const list = monthlyCosts
  const current = list.find((m: any) => m.month === key) || list[list.length - 1]
  const prevIdx = list.indexOf(current) - 1
  const prev = prevIdx >= 0 ? list[prevIdx] : null
  const fixedVar = computeMonthFixedVariable(current, 'economic')

  return {
    raw: current,
    month: current.month,
    distance_km: current.distance_km || 0,
    cost_per_km: current.cost_per_km ?? null,
    total: current.total || 0,
    // The part of the total due later in the month; the rest was spent so far
    scheduled: current.scheduled || 0,
    prevDistance: prev ? prev.distance_km : null,
    fixedVar,
  }
}

/** Reminders that are overdue or due soon, with a short sentence naming the first two. */
export function summarizeUrgentReminders(reminders: { title: string; status: string }[]) {
  const urgent = reminders.filter((r) => r.status === 'OVERDUE' || r.status === 'DUE_SOON')
  const titles = urgent.map((r) => r.title)
  const summary = !titles.length ? '' : titles.length <= 2 ? titles.join(', ') : t('dashboard.urgentReminders.andOthers', { titles: titles.slice(0, 2).join(', '), count: titles.length - 2 })
  return { urgent, hasOverdue: reminders.some((r) => r.status === 'OVERDUE'), summary }
}

interface ScheduledReminder {
  status: string
  interval_km?: number | null
  interval_months?: number | null
  scheduled_date?: string | null
  remaining_km?: number | null
  remaining_days?: number | null
}

const DAYS_PER_MONTH = 30.4375
const DAYS_PER_YEAR = 365.25

/** How far a reminder is through its interval, 0 (just done) to 1 (due): the closer of its mileage and calendar due points (a fixed date counts a year before it). */
export function reminderProgress(r: ScheduledReminder): number | null {
  const parts: number[] = []
  if ((r.interval_km ?? 0) > 0 && r.remaining_km != null) parts.push(1 - r.remaining_km / (r.interval_km as number))
  if (r.scheduled_date && r.remaining_days != null) parts.push(1 - r.remaining_days / DAYS_PER_YEAR)
  else if ((r.interval_months ?? 0) > 0 && r.remaining_days != null) parts.push(1 - r.remaining_days / ((r.interval_months as number) * DAYS_PER_MONTH))
  if (!parts.length) return null
  return Math.min(1, Math.max(0, Math.max(...parts)))
}

const STATUS_RANK: Record<string, number> = { OVERDUE: 0, DUE_SOON: 1, OK: 2 }

/** Reminders by urgency: overdue, then due soon, then up to date; inside a status the one furthest through its interval first, unscheduled ones last. */
export function sortRemindersByUrgency<T extends ScheduledReminder>(reminders: T[]): T[] {
  return [...reminders].sort((a, b) => {
    const rank = (STATUS_RANK[a.status] ?? 3) - (STATUS_RANK[b.status] ?? 3)
    if (rank !== 0) return rank
    return (reminderProgress(b) ?? -1) - (reminderProgress(a) ?? -1)
  })
}

export type ReminderGroupKey = 'overdue' | 'soon' | 'later' | 'unscheduled'

const dueKey = (r: ScheduledReminder & { due_date?: string | null }): string => (r.due_date ?? '').slice(0, 10) || '9999-99-99'

/** Reminders split by how close they are, each group ordered by due date (reminders only due by mileage after the dated ones, by remaining distance); empty groups are left out. */
export function groupRemindersByDue<T extends ScheduledReminder & { due_date?: string | null; remaining_km?: number | null; interval_km?: number | null; interval_months?: number | null; scheduled_date?: string | null }>(
  reminders: T[],
): { key: ReminderGroupKey; items: T[] }[] {
  const buckets: Record<ReminderGroupKey, T[]> = { overdue: [], soon: [], later: [], unscheduled: [] }
  for (const r of reminders) {
    if (r.status === 'OVERDUE') buckets.overdue.push(r)
    else if (r.status === 'DUE_SOON') buckets.soon.push(r)
    else if ((r.interval_km ?? 0) > 0 || (r.interval_months ?? 0) > 0 || r.scheduled_date) buckets.later.push(r)
    else buckets.unscheduled.push(r)
  }
  const byDue = (a: T, b: T) => {
    const k = dueKey(a).localeCompare(dueKey(b))
    return k !== 0 ? k : (a.remaining_km ?? Infinity) - (b.remaining_km ?? Infinity)
  }
  const order: ReminderGroupKey[] = ['overdue', 'soon', 'later', 'unscheduled']
  return order.filter((key) => buckets[key].length).map((key) => ({ key, items: buckets[key].sort(byDue) }))
}

/** The scheduled reminder that comes due first and is not overdue yet (overdue ones have their own banner). */
export function nextDueReminder<T extends ScheduledReminder>(reminders: T[]): { reminder: T; progress: number } | null {
  let best: { reminder: T; progress: number } | null = null
  for (const reminder of reminders) {
    if (reminder.status === 'OVERDUE') continue
    const progress = reminderProgress(reminder)
    if (progress !== null && (!best || progress > best.progress)) best = { reminder, progress }
  }
  return best
}

/**
 * What the follow-up zone of the dashboard asks for: data left to complete in the TCO, and a maintenance coming due
 * (or, with no schedule planned yet, an invitation to plan one).
 */
export function followUpItems(completeness: { is_complete?: boolean } | null | undefined, reminders: ScheduledReminder[], hasSchedule: boolean): { data: boolean; maintenance: 'soon' | 'plan' | null } {
  return {
    data: !!completeness && !completeness.is_complete,
    maintenance: nextDueReminder(reminders) ? 'soon' : !hasSchedule ? 'plan' : null,
  }
}

/** How many things the follow-up zone waits for. */
export function countFollowUpItems(completeness: { is_complete?: boolean } | null | undefined, reminders: ScheduledReminder[], hasSchedule: boolean): number {
  const items = followUpItems(completeness, reminders, hasSchedule)
  return (items.data ? 1 : 0) + (items.maintenance ? 1 : 0)
}

/** Follow-up figures of a lease (LOA / LLD) contract: elapsed time, mileage against the allowance, status. */
export function buildLeaseSummary(tco: any, now = new Date()) {
  if (!tco || !['LOA', 'LLD'].includes(tco.acquisition_type)) {
    return null
  }
  const c = tco
  const startDateStr = c.contract_start_date
  const endDateStr = c.contract_end_date
  if (!endDateStr && !c.contract_duration_months && !c.lease_duration_months) {
    return null
  }

  const start = startDateStr ? new Date(startDateStr) : null
  const durationMonths = c.contract_duration_months || c.lease_duration_months || 0
  let end = endDateStr ? new Date(endDateStr) : null
  if (!end && start && durationMonths > 0) {
    end = new Date(start.getFullYear(), start.getMonth() + durationMonths, start.getDate())
  }

  let totalMonths = durationMonths
  if (!totalMonths && start && end) {
    totalMonths = Math.max(1, Math.round((end.getTime() - start.getTime()) / (1000 * 3600 * 24 * 30.4375)))
  }

  let elapsedMonths = 0
  let remainingMonths = 0
  let durationProgressPct = 0
  let isEnded = false
  let isNotStarted = false

  if (start && end) {
    const totalMs = end.getTime() - start.getTime()
    const elapsedMs = now.getTime() - start.getTime()
    if (elapsedMs < 0) {
      isNotStarted = true
      durationProgressPct = 0
      remainingMonths = totalMonths
    } else if (now.getTime() >= end.getTime()) {
      isEnded = true
      durationProgressPct = 100
      elapsedMonths = totalMonths
      remainingMonths = 0
    } else {
      durationProgressPct = Math.min(100, Math.max(0, Math.round((elapsedMs / totalMs) * 100)))
      elapsedMonths = Math.max(0, Math.round(elapsedMs / (1000 * 3600 * 24 * 30.4375)))
      remainingMonths = Math.max(0, Math.round((end.getTime() - now.getTime()) / (1000 * 3600 * 24 * 30.4375)))
    }
  } else if (totalMonths > 0) {
    durationProgressPct = 50
  }

  // Mileage
  const kmDriven = c.lease_km_driven || 0
  const kmAllowanceToDate = c.lease_km_allowance_to_date || 0
  const kmAllowanceTotal = c.lease_km_allowance_total || 0
  const hasMileageAllowance = kmAllowanceToDate > 0 || kmAllowanceTotal > 0

  let mileageProgressPct = 0
  let kmDiff = 0
  let actualPaceKmMonth = 0
  let contractualPaceKmMonth = 0

  if (hasMileageAllowance) {
    if (kmAllowanceTotal > 0) {
      mileageProgressPct = Math.min(100, Math.max(0, Math.round((kmDriven / kmAllowanceTotal) * 100)))
    } else if (kmAllowanceToDate > 0) {
      mileageProgressPct = Math.min(100, Math.max(0, Math.round((kmDriven / kmAllowanceToDate) * 100)))
    }

    kmDiff = Math.round(kmDriven - kmAllowanceToDate)

    if (elapsedMonths > 0) {
      actualPaceKmMonth = Math.round(kmDriven / elapsedMonths)
    }
    if (c.lease_km_allowance_per_year) {
      contractualPaceKmMonth = Math.round(c.lease_km_allowance_per_year / 12)
    } else if (totalMonths > 0 && kmAllowanceTotal > 0) {
      contractualPaceKmMonth = Math.round(kmAllowanceTotal / totalMonths)
    }
  }

  // Status
  let status = { label: t('dashboard.lease.inProgress'), class: 'bg-indigo-500/15 text-indigo-300 border-indigo-500/30' }
  if (c.option_exercised_date) {
    status = { label: t('dashboard.lease.optionExercised'), class: 'bg-success-500/15 text-success-400 border-success-500/30' }
  } else if (isEnded) {
    status = { label: t('dashboard.lease.finished'), class: 'bg-slate-800 text-slate-400 border-slate-700' }
  } else if (remainingMonths <= 3 && !isNotStarted) {
    status = { label: t('dashboard.lease.endingSoon'), class: 'bg-warning-500/15 text-warning-300 border-warning-500/30' }
  }

  // Color for mileage bar
  let mileageColor = 'bg-gradient-to-r from-success-500 to-teal-500'
  if (kmAllowanceToDate > 0 && kmDriven > kmAllowanceToDate) {
    mileageColor = 'bg-gradient-to-r from-danger-500 to-red-500'
  } else if (kmAllowanceToDate > 0 && kmDriven / kmAllowanceToDate > 0.92) {
    mileageColor = 'bg-gradient-to-r from-warning-500 to-orange-500'
  }

  return {
    acquisitionType: c.acquisition_type,
    startDate: start,
    endDate: end,
    totalMonths,
    elapsedMonths,
    remainingMonths,
    durationProgressPct,
    isEnded,
    isNotStarted,
    status,
    // Mileage
    hasMileageAllowance,
    kmDriven,
    kmAllowanceToDate,
    kmAllowanceTotal,
    mileageProgressPct,
    kmDiff,
    actualPaceKmMonth,
    contractualPaceKmMonth,
    mileageColor,
    // Financials
    monthlyRent: c.lease_monthly_rent,
    downPayment: c.lease_down_payment,
    purchaseOptionPrice: c.lease_purchase_option_price,
    excessKmCost: c.lease_excess_km_cost || 0,
    excessKmProjected: c.lease_excess_km_projected || 0,
    excessKmPrice: c.lease_excess_km_price,
    // Included services
    includesMaintenance: c.lease_includes_maintenance,
    includesInsurance: c.lease_includes_insurance,
    includesTires: c.lease_includes_tires,
  }
}

export type MonthDetailMode = 'economic' | 'cash'

export interface MonthFixedVariable {
  fixedAmount: number
  variableAmount: number
  totalAmount: number
  fixedPct: number
  variablePct: number
}

/** Computes the fixed vs variable cost breakdown for a month item.
 * Variable: energy, tolls, tires, maintenance.
 * Fixed: insurance, financing, other (taxes, subscriptions, accessories).
 */
export function computeMonthFixedVariable(m: any, mode: MonthDetailMode = 'economic'): MonthFixedVariable {
  if (!m) {
    return { fixedAmount: 0, variableAmount: 0, totalAmount: 0, fixedPct: 0, variablePct: 0 }
  }

  const isEco = mode === 'economic'
  const energy = Number(m.energy) || 0
  const tolls = Number(m.tolls) || 0
  const tires = isEco ? (Number(m.tires_amortized) || 0) : (Number(m.tires) || 0)
  const maintenance = isEco ? (Number(m.maintenance_amortized) || 0) : (Number(m.maintenance) || 0)

  const insurance = Number(m.insurance) || 0
  const financing = isEco ? (Number(m.financing_amortized) || Number(m.financing) || 0) : (Number(m.financing) || 0)
  const other = Number(m.other) || 0

  const variableAmount = energy + tolls + tires + maintenance
  const fixedAmount = insurance + financing + other
  const totalAmount = variableAmount + fixedAmount

  const fixedPct = totalAmount > 0 ? Math.round((fixedAmount / totalAmount) * 100) : 0
  const variablePct = totalAmount > 0 ? 100 - fixedPct : 0

  return {
    fixedAmount,
    variableAmount,
    totalAmount,
    fixedPct,
    variablePct,
  }
}

/** Number of calendar months of the rolling window the headline figures cover. */
export const ROLLING_MONTHS = 12

/**
 * Cash paid, economic cost and distance over the last calendar months of the monthly costs (the current month
 * included), so a recent change in usage, tariff or insurance shows. With less history the window is what exists.
 * The sums are those of the bars of the monthly charts over the same months.
 */
export function rollingWindowStats(monthlyCosts: any[] | undefined, months = ROLLING_MONTHS) {
  if (!monthlyCosts?.length) return null
  const window = monthlyCosts.slice(-months)
  let cash = 0
  let economic = 0
  let distanceKm = 0
  let estimatedEnergy = 0
  for (const m of window) {
    const b = buildMonthBreakdown(m, 'economic')
    cash += b.cashTotal
    economic += b.economicTotal
    distanceKm += m.distance_km || 0
    estimatedEnergy += m.smoothed_energy || 0
  }
  return {
    months: window.length,
    fromMonth: window[0].month as string,
    toMonth: window[window.length - 1].month as string,
    cash,
    economic,
    distanceKm,
    estimatedEnergy,
    // Unknown, not zero, without a distance or without any cost over the window
    economicPerKm: distanceKm > 0 && economic > 0 ? economic / distanceKm : null,
  }
}

/** Cost items of one month, in the economic view (smoothed) or the cash view (what was paid), with their share and cost per km. */
export function buildMonthBreakdown(m: any, mode: MonthDetailMode, currency = 'EUR') {
  const money = (v: number) => formatAmount(v, currency)
  const dist = m.distance_km || 0

  const items = [
    {
      key: 'energy',
      label: t('dashboard.breakdown.energy'),
      subLabel: '',
      color: COST_COLORS.energy,
      amount: m.energy || 0,
      cashAmount: m.energy || 0,
      note: m.smoothed_energy > 0 ? t('dashboard.breakdown.energyNote', { amount: money(m.smoothed_energy) }) : null,
    },
    {
      key: 'tolls',
      label: t('dashboard.breakdown.tolls'),
      subLabel: '',
      color: COST_COLORS.tolls,
      amount: m.tolls || 0,
      cashAmount: m.tolls || 0,
      note: null,
    },
    {
      key: 'tires',
      label: t('dashboard.breakdown.tires'),
      subLabel: t('dashboard.breakdown.tiresSub'),
      color: COST_COLORS.tires,
      amount: m.tires_amortized || 0,
      cashAmount: m.tires || 0,
      note: (m.tires || 0) > 0
        ? t('dashboard.breakdown.tiresCashNote', { amount: money(Number(m.tires)) })
        : (m.tires_amortized > 0 ? t('dashboard.breakdown.tiresAmortizedNote', { unit: distanceUnit(), km: formatDistanceValue(dist), zero: money(0) }) : null),
    },
    {
      key: 'maintenance',
      label: t('dashboard.breakdown.maintenance'),
      subLabel: t('dashboard.breakdown.smoothed'),
      color: COST_COLORS.maintenance,
      amount: m.maintenance_amortized || 0,
      cashAmount: m.maintenance || 0,
      note: (m.maintenance || 0) > 0
        ? t('dashboard.breakdown.maintenanceBilledNote', { amount: money(Number(m.maintenance)) })
        : (m.maintenance_amortized > 0 ? t('dashboard.breakdown.maintenanceSmoothedNote') : null),
    },
    {
      key: 'insurance',
      label: t('dashboard.breakdown.insurance'),
      subLabel: '',
      color: COST_COLORS.insurance,
      amount: m.insurance || 0,
      cashAmount: m.insurance || 0,
      note: null,
    },
    {
      key: 'financing',
      label: t('dashboard.breakdown.financing'),
      subLabel: t('dashboard.breakdown.smoothed'),
      color: COST_COLORS.financing,
      amount: m.financing_amortized || m.financing || 0,
      cashAmount: m.financing || 0,
      note: (m.financing_amortized > 0 && Math.abs(m.financing_amortized - (m.financing || 0)) > 0.01)
        ? t('dashboard.breakdown.financingNote', { smoothed: money(m.financing_amortized), paid: money(m.financing || 0) })
        : null,
    },
    {
      key: 'other',
      label: t('dashboard.breakdown.other'),
      subLabel: '',
      color: COST_COLORS.other,
      amount: m.other || 0,
      cashAmount: m.other || 0,
      note: null,
    },
  ]

  const economicTotal = items.reduce((sum, it) => sum + it.amount, 0)
  const cashTotal = typeof m.total === 'number' ? m.total : items.reduce((sum, it) => sum + it.cashAmount, 0)
  const activeTotal = mode === 'economic' ? economicTotal : cashTotal

  const itemsWithStats = items.map((it) => {
    const displayAmount = mode === 'economic' ? it.amount : it.cashAmount
    const costPerKm = dist > 0 ? displayAmount / dist : 0
    const sharePct = activeTotal > 0 ? (displayAmount / activeTotal) * 100 : 0
    return {
      ...it,
      displayAmount,
      costPerKm,
      sharePct,
    }
  })

  return {
    month: m.month,
    distanceKm: dist,
    trackedDistanceKm: m.tracked_distance_km || 0,
    smoothedKm: m.smoothed_km || 0,
    costPerKm: m.cost_per_km ?? (dist > 0 && economicTotal > 0 ? economicTotal / dist : null),
    economicTotal,
    cashTotal,
    activeTotal,
    items: itemsWithStats,
    fixedVar: computeMonthFixedVariable(m, mode),
  }
}

/** Below this distance a month's cost per distance is mostly the fixed costs of a few kilometres: it is not plotted. */
export const MIN_PLOTTED_DISTANCE_KM = 50

/** A month's cost per km for the trend chart, or null when the month is too short to mean anything. */
export function plottedCostPerKm(m: { distance_km?: number | null; cost_per_km?: number | null }): number | null {
  if ((m.distance_km || 0) < MIN_PLOTTED_DISTANCE_KM) return null
  return m.cost_per_km == null ? null : m.cost_per_km
}

/** The stacked monthly bars: one series per cost item, in the economic view (spread) or the cash view (what was paid). */
export function monthlyCostSeries(months: any[], mode: MonthDetailMode) {
  const rows = months.map((m) => buildMonthBreakdown(m, mode))
  const keys = rows[0]?.items.map((it) => it.key) ?? []
  return {
    series: keys.map((key, i) => ({
      key,
      color: rows[0].items[i].color,
      data: rows.map((r) => r.items[i].displayAmount),
    })),
    totals: rows.map((r) => r.activeTotal),
  }
}
