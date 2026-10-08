import { describe, expect, it } from 'vitest'
import { setDistanceUnit } from '@/units'
import {
  applyBatchTag,
  buildSuggestionCostDrive,
  countActiveDriveFilters,
  buildTripCostDrive,
  currentYearMonth,
  driveCsvHeaders,
  driveCsvRows,
  filterTrips,
  formatMonthLabel,
  formatTripDates,
  monthRange,
  needsTollQualification,
  paginationPages,
  selectionSummary,
  shiftMonth,
  suggestionTripId,
  teslamateDriveUrl,
  toggleTag,
  tollApplyStatusLabel,
  mergeExpensesByDrive,
} from './drives'

describe('needsTollQualification', () => {
  it('follows the flag computed by the server', () => {
    expect(needsTollQualification({ needs_toll_qualification: true })).toBe(true)
    expect(needsTollQualification({ needs_toll_qualification: false })).toBe(false)
    expect(needsTollQualification({})).toBe(false)
  })
})

describe('teslamateDriveUrl', () => {
  const vehicle = { teslamate_grafana_url: 'http://grafana.local', teslamate_car_id: 2 }
  const drive = { teslamate_drive_id: 77, start_time: '2026-05-01T08:00:00Z', end_time: '2026-05-01T09:00:00Z' }

  it('builds the Drive Details link with a one minute margin around the drive', () => {
    const url = new URL(teslamateDriveUrl(vehicle, drive)!)
    expect(url.origin + url.pathname).toBe('http://grafana.local/d/zm7wN6Zgz/drive-details')
    expect(url.searchParams.get('var-drive_id')).toBe('77')
    expect(url.searchParams.get('var-car_id')).toBe('2')
    expect(Number(url.searchParams.get('from'))).toBe(new Date(drive.start_time).getTime() - 60_000)
    expect(Number(url.searchParams.get('to'))).toBe(new Date(drive.end_time).getTime() + 60_000)
  })

  it('has no link without Grafana, without a TeslaMate drive, or for a trip group', () => {
    expect(teslamateDriveUrl({}, drive)).toBeNull()
    expect(teslamateDriveUrl(vehicle, { ...drive, teslamate_drive_id: null })).toBeNull()
    expect(teslamateDriveUrl(vehicle, { ...drive, is_trip_group: true })).toBeNull()
    expect(teslamateDriveUrl(null, drive)).toBeNull()
  })
})

describe('tollApplyStatusLabel', () => {
  it('explains each skipped status and falls back to a generic message', () => {
    expect(tollApplyStatusLabel('skipped_manual')).toContain('manuellement')
    expect(tollApplyStatusLabel('skipped_trip_group')).toContain('voyage')
    expect(tollApplyStatusLabel('skipped_no_price')).toContain('tarif')
    expect(tollApplyStatusLabel('skipped_no_gps')).toContain('GPS')
    expect(tollApplyStatusLabel('whatever')).toContain('pas pu être appliqué')
  })
})

describe('formatTripDates', () => {
  it('shows one date for a single-day trip and a range otherwise', () => {
    expect(formatTripDates({})).toBe('Aucun trajet')
    const one = formatTripDates({ start_time: '2026-05-02T08:00:00Z', end_time: '2026-05-02T18:00:00Z' })
    expect(one).not.toContain('→')
    expect(formatTripDates({ start_time: '2026-05-02T08:00:00Z', end_time: '2026-05-04T18:00:00Z' })).toContain(' → ')
    expect(formatTripDates({ start_time: '2026-05-02T08:00:00Z' })).not.toContain('→')
  })
})

describe('month helpers', () => {
  it('formats the current month as YYYY-MM', () => {
    expect(currentYearMonth(new Date(2026, 8, 19))).toBe('2026-09')
    expect(currentYearMonth(new Date(2026, 0, 2))).toBe('2026-01')
  })

  it('shifts across year boundaries', () => {
    expect(shiftMonth('2026-01', -1)).toBe('2025-12')
    expect(shiftMonth('2025-12', 1)).toBe('2026-01')
    expect(shiftMonth('2026-05', 0)).toBe('2026-05')
  })

  it('gives the first and last day, including leap Februaries', () => {
    expect(monthRange('2026-05')).toEqual({ from: '2026-05-01', to: '2026-05-31' })
    expect(monthRange('2026-02')).toEqual({ from: '2026-02-01', to: '2026-02-28' })
    expect(monthRange('2028-02').to).toBe('2028-02-29')
  })

  it('capitalizes the month label', () => {
    expect(formatMonthLabel('2026-05')).toBe('Mai 2026')
    expect(formatMonthLabel('')).toBe('')
  })
})

describe('paginationPages', () => {
  it('lists every page when there are seven or fewer', () => {
    expect(paginationPages(1, 1)).toEqual([1])
    expect(paginationPages(3, 7)).toEqual([1, 2, 3, 4, 5, 6, 7])
  })

  it('collapses the middle for many pages', () => {
    expect(paginationPages(1, 20)).toEqual([1, 2, 3, 4, 5, '...', 20])
    expect(paginationPages(4, 20)).toEqual([1, 2, 3, 4, 5, '...', 20])
    expect(paginationPages(10, 20)).toEqual([1, '...', 9, 10, 11, '...', 20])
    expect(paginationPages(17, 20)).toEqual([1, '...', 16, 17, 18, 19, 20])
    expect(paginationPages(20, 20)).toEqual([1, '...', 16, 17, 18, 19, 20])
  })
})

describe('tags', () => {
  it('toggles a tag on and off', () => {
    expect(toggleTag([], 'Pro')).toEqual(['Pro'])
    expect(toggleTag(['Pro'], 'Pro')).toEqual([])
    expect(toggleTag(undefined, 'Perso')).toEqual(['Perso'])
  })

  it('keeps Pro and Perso exclusive and leaves other tags alone', () => {
    expect(toggleTag(['Perso', 'Vacances'], 'Pro')).toEqual(['Vacances', 'Pro'])
    expect(toggleTag(['Pro'], 'Perso')).toEqual(['Perso'])
  })

  it('applies a batch tag without duplicating it, or clears both', () => {
    expect(applyBatchTag(['Pro'], 'Pro')).toEqual(['Pro'])
    expect(applyBatchTag(['Perso', 'X'], 'Pro')).toEqual(['X', 'Pro'])
    expect(applyBatchTag(['Pro', 'Perso', 'X'], null)).toEqual(['X'])
  })
})

describe('selectionSummary and CSV rows', () => {
  const list = [
    { id: 'a', distance_km: 100.4, costs: { electricity_kwh: 18.2, total_cost: 5.5, cost_per_km: 0.05 }, tags: ['Pro', 'X'], start_time: '2026-05-01T08:00:00Z', start_address: 'Rue "A", Paris', end_address: 'Lyon' },
    { id: 'b', distance_km: 50, costs: { electricity_kwh: 9, total_cost: 2.25 } },
  ]

  it('sums distance, energy and cost', () => {
    expect(selectionSummary(list, 'EUR')).toBe(
      `${Math.round(150.4).toLocaleString('fr-FR')} km • 27 kWh • ${new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR' }).format(7.75)}`
    )
    expect(selectionSummary([], 'EUR')).toBe('')
  })

  it('formats the total in the vehicle\'s own currency', () => {
    expect(selectionSummary(list, 'USD')).toContain(new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'USD' }).format(7.75))
  })

  it('escapes quotes and joins tags in the CSV rows', () => {
    const rows = driveCsvRows(list)
    expect(rows[0][1]).toBe('2026-05-01T08:00')
    expect(rows[0][2]).toBe('"Rue ""A"", Paris"')
    expect(rows[0][8]).toBe('5.50')
    expect(rows[0][9]).toBe('0.050')
    expect(rows[0][10]).toBe('"Pro, X"')
    expect(rows[1][1]).toBe('')
    expect(rows[1][10]).toBe('""')
  })
  it('names the vehicle currency in the cost columns of the CSV header', () => {
    const headers = driveCsvHeaders('USD')
    expect(headers).toHaveLength(driveCsvRows(list)[0].length)
    expect(headers[8]).toMatch(/_USD$/)
    expect(headers.join(',')).not.toContain('EUR')
  })

  it('writes distances and figures per km in the account\'s unit', () => {
    setDistanceUnit('mi')
    try {
      const [row] = driveCsvRows([{ id: 'x', distance_km: 160.9344, consumption_kwh_100km: 16.09344, costs: { cost_per_km: 0.1 } }])
      expect(row[4]).toBe(100)
      expect(row[6]).toBe(25.9)
      expect(row[9]).toBe('0.161')
      expect(driveCsvHeaders('USD')[4]).toBe('Distance_mi')
    } finally {
      setDistanceUnit(null)
    }
  })
})

describe('mergeExpensesByDrive', () => {
  it('lists each expense once, in order', () => {
    const merged = mergeExpensesByDrive([{ id: 'a', allocated_amount: 1 }, { id: 'b', allocated_amount: 2 }, { id: 'a', allocated_amount: 3 }])
    expect(merged.map((e) => e.id)).toEqual(['a', 'b'])
  })

  it('sums the shares of a trip group expense returned once per drive', () => {
    const merged = mergeExpensesByDrive([
      { id: 'toll', amount: 11.9, allocated_amount: 7.53 },
      { id: 'own', amount: 5.6, allocated_amount: 5.6 },
      { id: 'toll', amount: 11.9, allocated_amount: 4.37 },
    ])
    expect(merged.map((e) => e.allocated_amount)).toEqual([11.9, 5.6])
    expect(merged.reduce((s, e) => s + (e.allocated_amount ?? 0), 0)).toBeCloseTo(17.5, 2)
  })

  it('does not mutate its input', () => {
    const input = [{ id: 'a', allocated_amount: 1 }, { id: 'a', allocated_amount: 2 }]
    mergeExpensesByDrive(input)
    expect(input[0].allocated_amount).toBe(1)
  })
})

describe('buildTripCostDrive', () => {
  const drives = [
    { start_time: '2026-05-02T08:00:00Z', start_address: 'Paris, France', end_address: 'Dijon, France', distance_km: 300, duration_min: 200, costs: { electricity_kwh: 50, electricity_cost: 11, tires_cost: 3, maintenance_cost: 2, insurance_cost: 1, has_estimates: false } },
    { start_time: '2026-05-03T08:00:00Z', start_address: 'Dijon, France', end_address: 'Lyon, France', distance_km: 200, duration_min: 150, costs: { electricity_kwh: 30, electricity_cost: 7, tires_cost: 2, maintenance_cost: 1, insurance_cost: 1, has_estimates: true } },
  ]

  it('sums the drives and adds the trip expenses to the total', () => {
    const t = buildTripCostDrive({ id: 'tg1', name: 'Alpes', expenses_total: 20 }, drives)
    expect(t.is_trip_group).toBe(true)
    expect(t.trip_group_name).toBe('Alpes')
    expect(t.drives_count).toBe(2)
    expect(t.distance_km).toBe(500)
    expect(t.duration_min).toBe(350)
    expect(t.start_address).toBe('Paris')
    expect(t.end_address).toBe('Lyon')
    expect(t.costs.electricity_cost).toBe(18)
    expect(t.costs.tolls_cost).toBe(20)
    expect(t.costs.total_cost).toBe(18 + 5 + 3 + 2 + 20)
    expect(t.costs.cost_per_km).toBeCloseTo(48 / 500)
    expect(t.costs.electricity_rate).toBeCloseTo(18 / 80)
    expect(t.costs.has_estimates).toBe(true)
  })

  it('keeps the rate sources so the modal does not report the insurance as not entered', () => {
    const withSources = (insurance: string, tires: string) => ({ ...drives[0], costs: { ...drives[0].costs, insurance_source: insurance, tires_rate_source: tires, electricity_rate_source: 'HISTORY', maintenance_rate_source: 'DEFAULT' } })
    const t = buildTripCostDrive({ id: 'tg1', name: 'Alpes' }, [withSources('RECORDED_EXPENSES', 'MOUNTED_TIRES'), withSources('RECORDED_EXPENSES', 'MOUNTED_TIRES')])
    expect(t.costs.insurance_source).toBe('RECORDED_EXPENSES')
    expect(t.costs.tires_rate_source).toBe('MOUNTED_TIRES')
    expect(t.costs.electricity_rate_source).toBe('HISTORY')
    expect(t.costs.maintenance_rate_source).toBe('DEFAULT')
  })

  it('shows the most cautious source when the drives of a trip differ', () => {
    const one = (insurance: string, tires: string) => ({ ...drives[0], costs: { ...drives[0].costs, insurance_source: insurance, tires_rate_source: tires } })
    const t = buildTripCostDrive({ id: 'tg1', name: 'Alpes' }, [one('RECORDED_EXPENSES', 'MOUNTED_TIRES'), one('INSUFFICIENT_DISTANCE', 'DEFAULT')])
    expect(t.costs.insurance_source).toBe('INSUFFICIENT_DISTANCE')
    expect(t.costs.tires_rate_source).toBe('DEFAULT')
  })

  it('counts the tolls entered on a drive of the trip, not only those attached to the trip itself', () => {
    const t = buildTripCostDrive({ id: 'tg1', name: 'Alpes', expenses_total: 0, tolls_total: 10.3 }, drives)
    expect(t.costs.tolls_cost).toBe(10.3)
    expect(t.costs.total_cost).toBeCloseTo(18 + 5 + 3 + 2 + 10.3)
  })

  it('falls back to default rates and placeholders for an empty trip', () => {
    const t = buildTripCostDrive({ id: 'tg2', name: 'Vide', created_at: '2026-05-06' }, [])
    expect(t.start_time).toBe('2026-05-06')
    expect(t.start_address).toBe('Départ')
    expect(t.end_address).toBe('Arrivée')
    expect(t.costs.electricity_rate).toBe(0.22)
    expect(t.costs.tires_rate).toBe(0.02)
    expect(t.costs.maintenance_rate).toBe(0.015)
    expect(t.costs.cost_per_km).toBe(0)
  })
})

describe('filterTrips', () => {
  const trips = [
    { id: 'a', name: 'Annecy → Lyon', start_time: '2026-09-13T16:00:00Z' },
    { id: 'b', name: 'Valloire', notes: 'week-end ski', start_time: '2026-09-17T16:41:00Z' },
    { id: 'c', name: '', start_address: 'Grenoble', end_address: 'Valence', start_time: '2026-08-30T09:00:00Z' },
  ]

  it('keeps everything without a filter', () => {
    expect(filterTrips(trips, {})).toHaveLength(3)
  })

  it('applies both bounds of the period on whole days', () => {
    expect(filterTrips(trips, { from: '2026-09-13', to: '2026-09-13' }).map((t) => t.id)).toEqual(['a'])
    expect(filterTrips(trips, { from: '2026-09-01' }).map((t) => t.id)).toEqual(['a', 'b'])
    expect(filterTrips(trips, { to: '2026-09-12' }).map((t) => t.id)).toEqual(['c'])
  })

  it('searches name, notes and addresses without case or blanks', () => {
    expect(filterTrips(trips, { q: ' LYON ' }).map((t) => t.id)).toEqual(['a'])
    expect(filterTrips(trips, { q: 'ski' }).map((t) => t.id)).toEqual(['b'])
    expect(filterTrips(trips, { q: 'valence' }).map((t) => t.id)).toEqual(['c'])
    expect(filterTrips(trips, { q: 'nowhere' })).toEqual([])
  })
})

describe('buildSuggestionCostDrive', () => {
  const legs = [
    { id: 'd1', start_time: '2026-09-17T16:00:00Z', start_address: 'A, France', end_address: 'B, France', distance_km: 80, costs: { electricity_cost: 8, tolls_cost: 7.53, tires_cost: 1, maintenance_cost: 1, insurance_cost: 0 } },
    { id: 'd2', start_time: '2026-09-17T18:00:00Z', start_address: 'B, France', end_address: 'C, France', distance_km: 70, costs: { electricity_cost: 7, tolls_cost: 4.37, tires_cost: 1, maintenance_cost: 1, insurance_cost: 0 } },
  ]
  const s = { drive_ids: ['d1', 'd2'], start_time: '2026-09-17T16:00:00Z' }

  it('is a read-only trip whose tolls are those already attached to its drives', () => {
    const trip = buildSuggestionCostDrive(s, legs, 'A → C')
    expect(trip.id).toBe(suggestionTripId(s))
    expect(trip.is_trip_group).toBe(true)
    expect(trip.is_suggestion).toBe(true)
    expect(trip.trip_group_name).toBe('A → C')
    expect(trip.costs.tolls_cost).toBeCloseTo(11.9, 2)
    expect(trip.distance_km).toBe(150)
  })
})

describe('countActiveDriveFilters', () => {
  const none = { mode: 'DRIVES' as const, periodMode: 'ALL', unqualifiedOnly: false, hasTollOnly: false, selectedTag: '', tripQualifyOnly: false }
  it('is zero when nothing narrows the list', () => {
    expect(countActiveDriveFilters(none)).toBe(0)
  })
  it('counts the period, the toll filters and the tag of the drives list', () => {
    expect(countActiveDriveFilters({ ...none, periodMode: 'MONTH', unqualifiedOnly: true, hasTollOnly: true, selectedTag: 'Pro' })).toBe(4)
  })
  it('counts only the filters of the trips list in trip mode', () => {
    expect(countActiveDriveFilters({ ...none, mode: 'TRIPS', hasTollOnly: true, selectedTag: 'Pro', tripQualifyOnly: true, periodMode: 'CUSTOM' })).toBe(2)
  })
})
