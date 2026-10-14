import { intlLocale, t } from '@/i18n'
import { distanceUnit, formatDistance, kmToDisplayDistance, perDistance } from '@/units'
import { formatAmount } from '@/currency'
/** Drive of the "to qualify" toll queue: highway-like (speed heuristic or a toll found by the GPS detection), no toll
 * attached, not reviewed. The server decides (needs_toll_qualification), with the same rule as the queue filter. */
export function needsTollQualification(d: any) {
  return d.needs_toll_qualification === true
}

/** Link to the drive in the TeslaMate Grafana "Drive Details" dashboard (standard TeslaMate dashboard uid),
 * when the vehicle has a Grafana URL configured. The time range is padded so the whole drive is visible. */
export function teslamateDriveUrl(vehicle: any, d: any): string | null {
  if (!vehicle?.teslamate_grafana_url || !d?.teslamate_drive_id || d.is_trip_group) return null
  const params = new URLSearchParams({
    orgId: '1',
    from: String(new Date(d.start_time).getTime() - 60_000),
    to: String(new Date(d.end_time).getTime() + 60_000),
    'var-car_id': String(vehicle.teslamate_car_id || 1),
    'var-drive_id': String(d.teslamate_drive_id),
  })
  return `${vehicle.teslamate_grafana_url}/d/zm7wN6Zgz/drive-details?${params.toString()}`
}

export function tollApplyStatusLabel(status: string) {
  switch (status) {
    case 'skipped_manual':
      return t('drives.toll.skippedManual')
    case 'skipped_trip_group':
      return t('drives.toll.skippedTripGroup')
    case 'skipped_no_price':
      return t('drives.toll.skippedNoPrice')
    case 'skipped_no_gps':
      return t('drives.toll.skippedNoGps')
    default:
      return t('drives.toll.failed')
  }
}

export function formatTripDates(tg: any) {
  if (!tg.start_time) return t('drives.noDrive')
  const start = new Date(tg.start_time).toLocaleDateString(intlLocale(), { day: '2-digit', month: 'short', year: 'numeric' })
  const end = tg.end_time ? new Date(tg.end_time).toLocaleDateString(intlLocale(), { day: '2-digit', month: 'short', year: 'numeric' }) : start
  return start === end ? start : `${start} → ${end}`
}

// ----- Month navigation (YYYY-MM strings) -----

export function currentYearMonth(now = new Date()) {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

/** The month `delta` months after (or before, when negative) the given YYYY-MM. */
export function shiftMonth(yearMonth: string, delta: number) {
  const [y, m] = yearMonth.split('-').map(Number)
  const d = new Date(y, m - 1 + delta, 1)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
}

/** First and last day (YYYY-MM-DD) of a YYYY-MM month. */
export function monthRange(yearMonth: string) {
  const [y, m] = yearMonth.split('-').map(Number)
  const lastDay = new Date(y, m, 0).getDate()
  const mm = String(m).padStart(2, '0')
  return { from: `${y}-${mm}-01`, to: `${y}-${mm}-${String(lastDay).padStart(2, '0')}` }
}

export type PeriodMode = 'ALL' | 'DAY' | 'WEEK' | 'MONTH' | 'YEAR' | 'CUSTOM'
/** The modes that step with the arrows: one day, one ISO week (Monday to Sunday), one month, one year. */
export type PeriodUnit = 'DAY' | 'WEEK' | 'MONTH' | 'YEAR'

const dayOf = (iso: string) => {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}
const isoOf = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

export function todayIso(now: Date = new Date()) {
  return isoOf(now)
}

/** The from/to days of the unit containing `anchor` (YYYY-MM-DD). */
export function unitRange(unit: PeriodUnit, anchor: string) {
  const a = dayOf(anchor)
  if (unit === 'DAY') return { from: anchor, to: anchor }
  if (unit === 'WEEK') {
    const monday = new Date(a.getFullYear(), a.getMonth(), a.getDate() - ((a.getDay() + 6) % 7))
    return { from: isoOf(monday), to: isoOf(new Date(monday.getFullYear(), monday.getMonth(), monday.getDate() + 6)) }
  }
  if (unit === 'MONTH') return monthRange(anchor.slice(0, 7))
  return { from: `${a.getFullYear()}-01-01`, to: `${a.getFullYear()}-12-31` }
}

/** The anchor one unit before (-1) or after (+1) `anchor`. A month or a year keeps the day, clamped to the target month. */
export function shiftAnchor(unit: PeriodUnit, anchor: string, dir: -1 | 1) {
  const a = dayOf(anchor)
  if (unit === 'DAY') return isoOf(new Date(a.getFullYear(), a.getMonth(), a.getDate() + dir))
  if (unit === 'WEEK') return isoOf(new Date(a.getFullYear(), a.getMonth(), a.getDate() + 7 * dir))
  if (unit === 'MONTH') {
    const last = new Date(a.getFullYear(), a.getMonth() + dir + 1, 0).getDate()
    return isoOf(new Date(a.getFullYear(), a.getMonth() + dir, Math.min(a.getDate(), last)))
  }
  const last = new Date(a.getFullYear() + dir, a.getMonth() + 1, 0).getDate()
  return isoOf(new Date(a.getFullYear() + dir, a.getMonth(), Math.min(a.getDate(), last)))
}

/** The from/to days narrowing the list for a mode, or an empty object for "all". */
export function periodBounds(f: { mode: PeriodMode; anchor: string; month: string; from: string; to: string }): { from?: string; to?: string } {
  if (f.mode === 'ALL') return {}
  if (f.mode === 'CUSTOM') return { from: f.from || undefined, to: f.to || undefined }
  if (f.mode === 'MONTH') return f.month ? monthRange(f.month) : {}
  return unitRange(f.mode, f.anchor)
}

export type PeriodPreset = 'lastMonth' | 'last30' | 'last90' | 'thisYear'

const isoDay = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`

/** The from/to days (YYYY-MM-DD) of a ready-made period, relative to `today`. */
export function presetRange(preset: PeriodPreset, today: Date = new Date()) {
  const y = today.getFullYear()
  const m = today.getMonth()
  if (preset === 'lastMonth') return { from: isoDay(new Date(y, m - 1, 1)), to: isoDay(new Date(y, m, 0)) }
  if (preset === 'thisYear') return { from: `${y}-01-01`, to: isoDay(today) }
  const days = preset === 'last30' ? 29 : 89
  return { from: isoDay(new Date(y, m, today.getDate() - days)), to: isoDay(today) }
}

export function formatMonthLabel(yearMonth: string) {
  if (!yearMonth) return ''
  const [y, m] = yearMonth.split('-').map(Number)
  const str = new Date(y, m - 1, 1).toLocaleDateString(intlLocale(), { month: 'long', year: 'numeric' })
  return str.charAt(0).toUpperCase() + str.slice(1)
}

/** Page buttons to show: every page when there are few, otherwise the ends and a window around the current one. */
export function paginationPages(current: number, totalPages: number): (number | string)[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, i) => i + 1)
  }
  if (current <= 4) {
    return [1, 2, 3, 4, 5, '...', totalPages]
  }
  if (current >= totalPages - 3) {
    return [1, '...', totalPages - 4, totalPages - 3, totalPages - 2, totalPages - 1, totalPages]
  }
  return [1, '...', current - 1, current, current + 1, '...', totalPages]
}

// ----- Selection and tags -----

/** Tags after toggling one: Pro and Perso exclude each other. */
export function toggleTag(tags: string[] | undefined, tag: string): string[] {
  let current = [...(tags || [])]
  const idx = current.indexOf(tag)
  if (idx > -1) {
    current.splice(idx, 1)
  } else {
    if (tag === 'Pro') {
      current = current.filter((t) => t !== 'Perso')
    } else if (tag === 'Perso') {
      current = current.filter((t) => t !== 'Pro')
    }
    current.push(tag)
  }
  return current
}

/** Tags after applying a batch tag: Pro or Perso replaces the other, null clears both. */
export function applyBatchTag(tags: string[] | undefined, tag: 'Pro' | 'Perso' | null): string[] {
  let current = [...(tags || [])]
  if (tag === 'Pro') {
    current = current.filter((t) => t !== 'Perso')
    if (!current.includes('Pro')) current.push('Pro')
  } else if (tag === 'Perso') {
    current = current.filter((t) => t !== 'Pro')
    if (!current.includes('Perso')) current.push('Perso')
  } else {
    current = current.filter((t) => t !== 'Pro' && t !== 'Perso')
  }
  return current
}

/** Sum of the drive costs, in currency units like each `costs.total_cost` the API sends (not cents). */
export function sumDriveCosts(list: any[]): number {
  return list.reduce((s, d) => s + (Number(d.costs?.total_cost) || 0), 0)
}

export function selectionSummary(list: any[], currency: string): string {
  if (!list.length) return ''
  const totalKm = list.reduce((s, d) => s + (Number(d.distance_km) || 0), 0)
  const totalKwh = list.reduce((s, d) => s + (Number(d.costs?.electricity_kwh) || 0), 0)
  const totalCost = sumDriveCosts(list)
  return `${formatDistance(totalKm)} • ${Math.round(totalKwh)} kWh • ${formatAmount(totalCost, currency)}`
}

export const driveCsvHeaders = (currency: string) => t('drives.csvHeaders', { unit: distanceUnit(), cur: currency }).split(',')

/** One CSV row per drive, distances and figures per km in the account's unit like the headers. */
export function driveCsvRows(list: any[]) {
  return list.map((d) => [
    d.id,
    d.start_time ? new Date(d.start_time).toISOString().slice(0, 16) : '',
    d.start_address || '',
    d.end_address || '',
    Math.round(kmToDisplayDistance(d.distance_km || 0) * 100) / 100,
    d.duration_min || 0,
    Math.round(perDistance(d.consumption_kwh_100km || 0) * 10) / 10,
    d.costs?.electricity_kwh || 0,
    (d.costs?.total_cost || 0).toFixed(2),
    perDistance(d.costs?.cost_per_km || 0).toFixed(3),
    (d.tags || []).join(', '),
  ])
}

/**
 * Expenses of several drives, each listed once. An expense attached to a trip group comes back once per drive,
 * with that drive's share as allocated_amount: the shares are summed so the row shows what the trip as a whole bears.
 */
export function mergeExpensesByDrive<T extends { id: string; allocated_amount?: number | null }>(items: T[]): T[] {
  const map = new Map<string, T>()
  for (const e of items) {
    const seen = map.get(e.id)
    if (!seen) {
      map.set(e.id, { ...e })
    } else if (e.allocated_amount != null) {
      seen.allocated_amount = (seen.allocated_amount ?? 0) + e.allocated_amount
    }
  }
  return Array.from(map.values())
}

/** The source shown for a whole trip: the most cautious one among its drives (first of `cautious` found), else the common one. */
function pickSource(tgDrives: any[], field: string, cautious: string[] = []): string | undefined {
  const values = tgDrives.map((d) => d.costs?.[field]).filter(Boolean)
  return cautious.find((c) => values.includes(c)) ?? values[0]
}

export interface TripListFilter {
  from?: string // YYYY-MM-DD, inclusive
  to?: string // YYYY-MM-DD, inclusive
  q?: string
}

/**
 * Trip groups or trip suggestions narrowed like the drives list: start time inside the period (same day-boundary
 * rule as the API, in UTC) and the query found in the name, notes or addresses.
 */
export function filterTrips<T extends { start_time: string; name?: string; notes?: string | null; start_address?: string; end_address?: string }>(
  trips: T[],
  { from, to, q }: TripListFilter
): T[] {
  const fromMs = from ? Date.parse(`${from}T00:00:00Z`) : null
  const toMs = to ? Date.parse(`${to}T23:59:59Z`) : null
  const needle = (q || '').trim().toLowerCase()
  return trips.filter((tr) => {
    const start = Date.parse(tr.start_time)
    if (fromMs != null && start < fromMs) return false
    if (toMs != null && start > toMs) return false
    if (!needle) return true
    return [tr.name, tr.notes, tr.start_address, tr.end_address].some((field) => field?.toLowerCase().includes(needle))
  })
}

/** Id a trip suggestion goes by in the cost modal, before any trip group exists for it. */
export function suggestionTripId(s: { drive_ids: string[] }) {
  return `suggestion:${s.drive_ids[0]}`
}

/** A trip suggestion presented like a trip group for the cost modal, from the drives it is made of (with their costs). */
export function buildSuggestionCostDrive(s: any, legs: any[], name: string) {
  const tolls = legs.reduce((sum, d) => sum + (Number(d.costs?.tolls_cost) || 0), 0)
  return {
    ...buildTripCostDrive({ id: suggestionTripId(s), name, start_time: s.start_time, tolls_total: tolls }, legs),
    is_suggestion: true,
  }
}

/** A trip group presented like a drive for the cost modal: distances and costs summed over its drives. */
export function buildTripCostDrive(tg: any, tgDrives: any[]) {
  const totalKm = tgDrives.reduce((s, d) => s + (Number(d.distance_km) || 0), 0)
  const totalDuration = tgDrives.reduce((s, d) => s + (Number(d.duration_min) || 0), 0)
  const totalKwh = tgDrives.reduce((s, d) => s + (Number(d.costs?.electricity_kwh) || 0), 0)
  const elecCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.electricity_cost) || 0), 0)
  const tiresCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.tires_cost) || 0), 0)
  const maintCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.maintenance_cost) || 0), 0)
  const insurCost = tgDrives.reduce((s, d) => s + (Number(d.costs?.insurance_cost) || 0), 0)
  // tolls_total covers expenses attached to a drive of the trip as well as those attached to the trip itself
  const tollsCost = Number(tg.tolls_total ?? tg.expenses_total ?? 0)
  const totalCost = elecCost + tiresCost + maintCost + insurCost + tollsCost
  const costPerKm = totalKm > 0 ? totalCost / totalKm : 0

  const firstDrive = tgDrives[0]
  const lastDrive = tgDrives[tgDrives.length - 1]

  return {
    id: tg.id,
    is_trip_group: true,
    start_time: tg.start_time || firstDrive?.start_time || tg.created_at,
    start_address: firstDrive ? (firstDrive.start_address || t('drives.driveCostModal.start')).split(',')[0] : t('drives.driveCostModal.start'),
    end_address: lastDrive ? (lastDrive.end_address || t('drives.driveCostModal.end')).split(',')[0] : t('drives.driveCostModal.end'),
    distance_km: Math.round(totalKm),
    duration_min: totalDuration,
    tags: [],
    trip_group_name: tg.name,
    drives_count: tgDrives.length,
    costs: {
      electricity_kwh: Math.round(totalKwh),
      electricity_rate: totalKwh > 0 ? elecCost / totalKwh : 0,
      electricity_cost: elecCost,
      tires_rate: totalKm > 0 ? tiresCost / totalKm : 0,
      tires_cost: tiresCost,
      maintenance_rate: totalKm > 0 ? maintCost / totalKm : 0,
      maintenance_cost: maintCost,
      insurance_rate: totalKm > 0 ? insurCost / totalKm : 0,
      insurance_cost: insurCost,
      tolls_cost: tollsCost,
      total_cost: totalCost,
      cost_per_km: costPerKm,
      has_estimates: tgDrives.some((d) => d.costs?.has_estimates),
      // Without the sources, the modal would treat every rate as unknown (e.g. insurance "not entered")
      energy_source: pickSource(tgDrives, 'energy_source', ['DEFAULT', 'CONSUMPTION']),
      electricity_rate_source: pickSource(tgDrives, 'electricity_rate_source', ['UNKNOWN']),
      tires_rate_source: pickSource(tgDrives, 'tires_rate_source', ['UNKNOWN']),
      maintenance_rate_source: pickSource(tgDrives, 'maintenance_rate_source', ['UNKNOWN']),
      insurance_source: pickSource(tgDrives, 'insurance_source', ['NONE', 'INSUFFICIENT_DISTANCE', 'RECORDED_EXPENSES']),
    },
  }
}

/** How many filters narrow the list. On a phone they sit behind a button, which shows this number. */
export function countActiveDriveFilters(f: { mode: 'DRIVES' | 'TRIPS'; periodMode: string; unqualifiedOnly: boolean; hasTollOnly: boolean; selectedTag: string; tripQualifyOnly: boolean }): number {
  const period = f.periodMode !== 'ALL' ? 1 : 0
  if (f.mode === 'TRIPS') return period + (f.tripQualifyOnly ? 1 : 0)
  return period + (f.unqualifiedOnly ? 1 : 0) + (f.hasTollOnly ? 1 : 0) + (f.selectedTag ? 1 : 0)
}
