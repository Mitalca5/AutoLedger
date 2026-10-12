import { intlLocale, t } from '@/i18n'
import { distanceUnit, kmToDisplayDistance } from '@/units'
export interface LegForm {
  drive_id: string | null
  start_label: string
  end_label: string
  distance_km: number
  electricity_cost: number
  tolls_cost: number
  tires_cost: number
  maintenance_cost: number
  insurance_cost: number
  other_cost: number
}

export interface PassengerForm {
  passenger_name: string
  seats: number
  amount_paid: number
  board_stop_index: number
  alight_stop_index: number
  notes: string
}

export const COST_FIELDS: Array<{ key: keyof LegForm; label: string }> = [
  { key: 'electricity_cost', label: 'electricity' },
  { key: 'tolls_cost', label: 'tolls' },
  { key: 'tires_cost', label: 'tires' },
  { key: 'maintenance_cost', label: 'maintenance' },
  { key: 'insurance_cost', label: 'insurance' },
  { key: 'other_cost', label: 'other' },
]

export const cents = (v: number | string) => Math.round((Number(v) || 0) * 100)
export const euros = (c: number) => c / 100
export function toDateInputString(dateVal: string | Date | null | undefined): string {
  if (!dateVal) return ''
  const d = new Date(dateVal)
  if (Number.isNaN(d.getTime())) return ''
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString(intlLocale(), { day: 'numeric', month: 'short', year: 'numeric' })
}

export function formatDriveTime(dateStr: string) {
  return new Date(dateStr).toLocaleDateString(intlLocale(), { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' })
}

export function emptyLeg(): LegForm {
  return {
    drive_id: null,
    start_label: '',
    end_label: '',
    distance_km: 0,
    electricity_cost: 0,
    tolls_cost: 0,
    tires_cost: 0,
    maintenance_cost: 0,
    insurance_cost: 0,
    other_cost: 0,
  }
}

/** A passenger riding the whole trip; legCount is the number of legs the form has so far. */
export function newPassenger(index: number, legCount: number): PassengerForm {
  return {
    passenger_name: t('carpool.passenger', { n: index + 1 }),
    seats: 1,
    amount_paid: 0,
    board_stop_index: 0,
    alight_stop_index: Math.max(1, legCount),
    notes: '',
  }
}

export function legTotalCents(leg: any) {
  return COST_FIELDS.reduce((sum, f) => sum + cents(leg[f.key]), 0)
}

// Stop names: stop i starts leg i, the last stop ends the last leg
export function stopNames(legs: any[]) {
  if (!legs.length) return [t('carpool.start'), t('carpool.destination')]
  const names = legs.map((l, i) => l.start_label || (i > 0 && legs[i - 1].end_label) || (i === 0 ? t('carpool.start') : t('carpool.stop', { n: i })))
  names.push(legs[legs.length - 1].end_label || t('carpool.destination'))
  return names
}

// Same fair split as the backend: each leg cost is divided between the people on board (driver included)
export function allocate(legs: any[], passengers: any[]) {
  const shares = passengers.map(() => 0)
  const legDetails = legs.map((leg, i) => {
    const total = legTotalCents(leg)
    let seats = 0
    passengers.forEach((p) => {
      if (p.board_stop_index <= i && i < p.alight_stop_index) seats += Number(p.seats) || 1
    })
    const perPerson = Math.floor(total / (1 + seats))
    passengers.forEach((p, j) => {
      if (p.board_stop_index <= i && i < p.alight_stop_index) shares[j] += perPerson * (Number(p.seats) || 1)
    })
    return { total, seats, perPerson }
  })
  const total = legDetails.reduce((s, l) => s + l.total, 0)
  const passengersShare = shares.reduce((s, v) => s + v, 0)
  return { legDetails, shares, total, passengersShare, driverShare: total - passengersShare }
}

// Identifies a stop by the drive it starts (or ends, for the last stop), so that passengers keep their
// boarding and alighting places when drives are added or removed around them.
export function stopKeys(legs: LegForm[]) {
  const keys = legs.map((l, i) => (l.drive_id ? `start:${l.drive_id}` : `index:${i}`))
  keys.push(legs.length && legs[legs.length - 1].drive_id ? `end:${legs[legs.length - 1].drive_id}` : `index:${legs.length}`)
  return keys
}

/** Legs of an estimate as form values. */
export function legsFromEstimate(est: any): LegForm[] {
  return (est.legs || []).map((l: any) => ({
    drive_id: l.drive_id || null,
    start_label: l.start_label || '',
    end_label: l.end_label || '',
    distance_km: l.distance_km,
    electricity_cost: l.electricity_cost,
    tolls_cost: l.tolls_cost,
    tires_cost: l.tires_cost,
    maintenance_cost: l.maintenance_cost,
    insurance_cost: l.insurance_cost,
    other_cost: l.other_cost || 0,
  }))
}

/** Legs of a saved trip as form values. */
export function legsFromTrip(trip: any): LegForm[] {
  return (trip.legs || []).map((l: any) => ({
    drive_id: l.drive_id || null,
    start_label: l.start_label || '',
    end_label: l.end_label || '',
    distance_km: l.distance_km,
    electricity_cost: l.electricity_cost,
    tolls_cost: l.tolls_cost,
    tires_cost: l.tires_cost,
    maintenance_cost: l.maintenance_cost,
    insurance_cost: l.insurance_cost,
    other_cost: l.other_cost,
  }))
}

export function passengersFromTrip(trip: any, legCount: number): PassengerForm[] {
  return (trip.passengers || []).map((p: any) => ({
    passenger_name: p.passenger_name,
    seats: p.seats || 1,
    amount_paid: p.amount_paid || 0,
    board_stop_index: p.board_stop_index ?? 0,
    alight_stop_index: p.alight_stop_index ?? legCount,
    notes: p.notes || '',
  }))
}

// Keeps passengers' stops inside the current stops. A passenger who rode to the last stop (or any passenger
// entered before the legs were known) still rides to the new last stop.
export function clampPassengerStops(passengers: PassengerForm[], legCount: number, previousLegCount: number) {
  const n = legCount
  if (!n) return
  passengers.forEach((p) => {
    if (previousLegCount === 0 || (previousLegCount > 0 && p.alight_stop_index >= previousLegCount)) p.alight_stop_index = n
    p.alight_stop_index = Math.min(Math.max(p.alight_stop_index, 1), n)
    p.board_stop_index = Math.min(Math.max(p.board_stop_index, 0), p.alight_stop_index - 1)
  })
}

/** After the legs changed, moves each passenger's boarding and alighting stop to the same place in the new legs. */
export function remapPassengerStops(previousLegs: LegForm[], newLegs: LegForm[], passengers: PassengerForm[]) {
  const previousLegCount = previousLegs.length
  if (previousLegCount === 0) return
  const previousKeys = stopKeys(previousLegs)
  // Alternative key of a stop: the end of the previous drive is the same place as the start of the next one
  const previousAltKeys = previousKeys.map((_, i) => (i > 0 && previousLegs[i - 1]?.drive_id ? `end:${previousLegs[i - 1].drive_id}` : ''))
  const keys = stopKeys(newLegs)
  const altIndex = new Map<string, number>()
  newLegs.forEach((l, i) => l.drive_id && altIndex.set(`end:${l.drive_id}`, i + 1))
  const locate = (stop: number) => {
    const direct = keys.indexOf(previousKeys[stop])
    if (direct >= 0) return direct
    if (altIndex.has(previousKeys[stop])) return altIndex.get(previousKeys[stop])!
    const alt = previousAltKeys[stop] && altIndex.get(previousAltKeys[stop])
    return alt === undefined || alt === '' ? -1 : alt
  }
  passengers.forEach((p) => {
    const ridesToEnd = p.alight_stop_index >= previousLegCount
    const board = locate(p.board_stop_index)
    const alight = locate(p.alight_stop_index)
    p.board_stop_index = board >= 0 ? board : 0
    p.alight_stop_index = ridesToEnd || alight < 0 ? newLegs.length : alight
  })
}

/** Default title of a trip: first stop to last stop, with the number of legs when there are several. */
export function estimateTitle(names: string[], legCount: number) {
  return `${names[0]} → ${names[names.length - 1]}${legCount > 1 ? ` (${t('carpool.legCount', { count: legCount })})` : ''}`
}

/** Date of the earliest selected drive, used when an estimate carries no start date. */
export function earliestSelectedDriveDate(drives: any[], selectedIds: string[]): string {
  const first = drives
    .filter((d) => selectedIds.includes(d.id))
    .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime())[0]
  return first ? toDateInputString(first.start_time) : ''
}

/**
 * What the passengers paid against what they owed: paid and fair shares as a percentage of the actual cost of the
 * carpool. `status` compares the two (a cent of tolerance for rounding).
 */
export function carpoolCoverage(trip: any) {
  const total = Number(trip?.total_cost) || 0
  const paid = Number(trip?.total_revenue) || 0
  const fair = Number(trip?.passengers_cost_share) || 0
  const pct = (v: number) => (total > 0 ? (v / total) * 100 : 0)
  const status: 'below' | 'fair' | 'above' = paid < fair - 0.01 ? 'below' : paid > fair + 0.01 ? 'above' : 'fair'
  return { total, paid, fair, paidPct: pct(paid), fairPct: pct(fair), status }
}

/** Days on each side of a date of the window of drives offered to build a carpool. */
export const DRIVE_WINDOW_DAYS = 7

/** A YYYY-MM-DD date moved by a number of days (calendar arithmetic, no time zone involved). */
export function shiftDay(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  return d.toISOString().slice(0, 10)
}

/** Bounds (YYYY-MM-DD, inclusive) of the drives offered around a date. */
export function driveWindow(date: string, days = DRIVE_WINDOW_DAYS) {
  return { from: shiftDay(date, -days), to: shiftDay(date, days) }
}

/**
 * The drives to offer: those of the window, plus the selected ones known from elsewhere (an edited carpool whose
 * drives are older than the window keeps them listed), most recent first.
 */
export function pickerDrives(windowDrives: any[], known: Map<string, any>, selectedIds: string[]): any[] {
  const byId = new Map<string, any>(windowDrives.map((d) => [d.id, d]))
  for (const id of selectedIds) {
    const d = known.get(id)
    if (d && !byId.has(id)) byId.set(id, d)
  }
  return [...byId.values()].sort((a, b) => new Date(b.start_time).getTime() - new Date(a.start_time).getTime())
}

export const carpoolCsvHeaders = (currency: string) => t('carpool.csvHeaders', { unit: distanceUnit(), cur: currency }).split(',')

export function carpoolCsvRows(trips: any[]) {
  return trips.map((t) => [
    t.id,
    toDateInputString(t.date),
    t.title || '',
    Math.round(kmToDisplayDistance(t.distance_km || 0) * 100) / 100,
    t.passenger_count || (t.passengers || []).length || 0,
    (t.total_cost || 0).toFixed(2),
    (t.total_revenue || 0).toFixed(2),
    (t.net_cost || 0).toFixed(2),
    t.total_cost > 0 ? Math.min(100, Math.round((t.total_revenue / t.total_cost) * 100)) : 0,
  ])
}
